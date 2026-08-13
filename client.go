package jennah

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	jennahcreds "github.com/alphauslabs/jennah-sdk-go/credentials"
	agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"
	approvalv1 "github.com/alphauslabs/jennah-sdk-go/jennah/approval/v1"
	authv1 "github.com/alphauslabs/jennah-sdk-go/jennah/auth/v1"
	billingv1 "github.com/alphauslabs/jennah-sdk-go/jennah/billing/v1"
	datastorev1 "github.com/alphauslabs/jennah-sdk-go/jennah/datastore/v1"
	platformv1 "github.com/alphauslabs/jennah-sdk-go/jennah/platform/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// DefaultEndpoint is the platform's public gRPC front door. TLS terminates at
// the load balancer on 443; the backend listener holds no certificate, so
// ordinary TLS transport credentials are all a caller needs.
//
// Default gRPC endpoint. "jennah.alphaus.cloud" routes to the
// HTTP gateway, which speaks HTTP/JSON and cannot answer a gRPC call. The two
// front doors terminate on the same server behind the same auth chain, so the
// choice of hostname changes the wire protocol and nothing about authority.
const DefaultEndpoint = "jennah-grpc.alphaus.cloud:443"

// Config configures a Client.
type Config struct {
	// Endpoint is the public gRPC address in host:port form. Empty uses
	// DefaultEndpoint. Point it elsewhere only for a local server or a test
	// listener. The HTTP gateway hostname will not serve gRPC.
	Endpoint string

	// APIKey is the bearer credential sent on every request as
	// "authorization: Bearer <APIKey>": either an opaque "jennah_sk_..." API key
	// or a jennah access token. The server derives the effective enterprise from
	// this credential.
	//
	// Optional. Left empty, the client resolves a credential itself, in order:
	// the JENNAH_API_KEY environment variable, then the session stored by
	// "jnh login" (see the credentials package). A program that supplies this
	// field is unaffected by that fallback, because the first source that yields
	// a credential wins and the rest are never consulted.
	APIKey string

	// Credentials supplies the bearer per call from custom sources, such as a
	// secret manager, test double, or custom renewal implementation.
	//
	// It takes precedence over APIKey, which is itself shorthand for a source
	// holding one fixed credential. Left nil, the client builds a source from
	// APIKey or from the resolution order above.
	Credentials jennahcreds.Source

	// EnterpriseID is the caller's enterprise. It is stored for reference only:
	// the tenant boundary is derived server-side from APIKey and any
	// client-supplied enterprise is ignored. Optional.
	EnterpriseID string

	// Insecure dials without transport security. Intended for local gateways and
	// emulator/testing only; never enable it against a production endpoint.
	Insecure bool

	// TransportCredentials overrides the transport credentials. When nil, TLS is
	// used unless Insecure is set.
	TransportCredentials credentials.TransportCredentials

	// Retry configures automatic retries. The zero value retries a call up to
	// three times, 100ms apart doubling to 2s, but only when the failure is
	// UNAVAILABLE and only when replaying the call cannot produce a second effect.
	//
	// Reads always qualify. Writes qualify only when the request carries what makes
	// a replay safe: an IdempotencyKey on a data commit, a RequestKey on an
	// approval, or a memory commit with no execution-log section (vector and graph
	// writes are idempotent upserts, log steps are append-only). See safeToReplay.
	Retry RetryPolicy

	// DialOptions are extra gRPC dial options, appended after the SDK's own
	// (transport credentials, retry, and the auth interceptor). Useful for custom
	// dialers, keepalives, or interceptors.
	DialOptions []grpc.DialOption
}

// Client is a connection to the Jennah gRPC endpoint scoped to one credential.
// It is safe for concurrent use. Call Close when done.
//
// Agent workspaces are reached through Spawn, List and Agent; every other
// service hangs off one of the namespace fields below. All of them share this
// Client's single connection and credential.
type Client struct {
	// Auth is identity, enterprise administration, API keys and roles.
	Auth AuthAPI
	// Datasets creates and lists application datasets; Client.Dataset(id) opens
	// one.
	Datasets DatasetsAPI
	// Approvals raises and resolves human approvals.
	Approvals ApprovalsAPI
	// Billing reads subscription and entitlement state.
	Billing BillingAPI
	// Platform is operator topology: where tenant resources can live.
	Platform PlatformAPI

	conn         *grpc.ClientConn
	agents       agentv1.AgentServiceClient
	memory       agentv1.MemoryServiceClient
	datasets     datastorev1.DatasetServiceClient
	schema       datastorev1.SchemaServiceClient
	data         datastorev1.DataServiceClient
	auth         authv1.AuthServiceClient
	approvals    approvalv1.ApprovalServiceClient
	billing      billingv1.BillingServiceClient
	platform     platformv1.PlatformServiceClient
	health       healthpb.HealthClient
	endpoint     string
	enterpriseID string
	credential   jennahcreds.Source
}

// NewClient dials the gRPC endpoint and returns a Client. The connection is
// lazy: no network round trip happens until the first RPC (see Ping to force
// one).
//
// Resolves credentials from Config.Credentials, Config.APIKey, or default sources.
// Returns an error if no credentials are found.
func NewClient(cfg Config) (*Client, error) {
	source, err := resolveSource(cfg)
	if err != nil {
		return nil, err
	}
	// Uses Config.Endpoint or DefaultEndpoint, ignoring stored HTTP gateway endpoints.
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	var opts []grpc.DialOption
	switch {
	case cfg.TransportCredentials != nil:
		opts = append(opts, grpc.WithTransportCredentials(cfg.TransportCredentials))
	case cfg.Insecure:
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	default:
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	}
	// Wraps bearer interceptors inside retry interceptors so replayed calls fetch refreshed credentials.
	chain := []grpc.UnaryClientInterceptor{}
	if !cfg.Retry.Disabled {
		chain = append(chain, retryInterceptor(cfg.Retry))
	}
	chain = append(chain, bearerInterceptor(source))
	opts = append(opts, grpc.WithChainUnaryInterceptor(chain...))
	opts = append(opts, cfg.DialOptions...)

	conn, err := grpc.NewClient(endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("jennah: dial %q: %w", endpoint, err)
	}
	c := &Client{
		conn:         conn,
		agents:       agentv1.NewAgentServiceClient(conn),
		memory:       agentv1.NewMemoryServiceClient(conn),
		datasets:     datastorev1.NewDatasetServiceClient(conn),
		schema:       datastorev1.NewSchemaServiceClient(conn),
		data:         datastorev1.NewDataServiceClient(conn),
		auth:         authv1.NewAuthServiceClient(conn),
		approvals:    approvalv1.NewApprovalServiceClient(conn),
		billing:      billingv1.NewBillingServiceClient(conn),
		platform:     platformv1.NewPlatformServiceClient(conn),
		health:       healthpb.NewHealthClient(conn),
		endpoint:     endpoint,
		enterpriseID: cfg.EnterpriseID,
		credential:   source,
	}
	c.Auth = AuthAPI{
		c:           c,
		Session:     SessionAPI{c: c},
		Keys:        KeysAPI{c: c},
		Members:     MembersAPI{c: c},
		Invitations: InvitationsAPI{c: c},
		Roles:       RolesAPI{c: c},
		Enterprise:  EnterpriseAPI{c: c},
	}
	c.Datasets = DatasetsAPI{c: c}
	c.Approvals = ApprovalsAPI{c: c, Approvers: ApproversAPI{c: c}}
	c.Billing = BillingAPI{c: c}
	c.Platform = PlatformAPI{c: c}
	c.attachRenewer(source)
	return c, nil
}

// Endpoint returns the address this Client dials.
func (c *Client) Endpoint() string { return c.endpoint }

// CredentialInfo describes the credential a Client authenticates with, without
// exposing it. Every field is safe to log, which is the point: a program that
// behaves differently as a service credential than as a signed-in user needs to
// know which it got, and a diagnostic that answers that question must not be the
// thing that leaks the secret.
type CredentialInfo struct {
	// Kind is whether the credential is an API key or a signed-in session.
	Kind jennahcreds.Kind
	// Origin is which source it was resolved from.
	Origin jennahcreds.Origin
}

// String renders the credential for a log line: what it is and where it came
// from, never its value.
func (i CredentialInfo) String() string {
	return fmt.Sprintf("%s from %s", i.Kind, i.Origin)
}

// Credential reports what this Client authenticates with and where that came
// from. It never returns the credential itself.
func (c *Client) Credential() CredentialInfo {
	return CredentialInfo{Kind: c.credential.Kind(), Origin: c.credential.Origin()}
}

// Conn returns the underlying connection, so a caller can build a generated stub
// for unwrapped calls over the same connection.
// connection.
//
// Stubs built on it are credentialed: the bearer is attached by a dial-time
// interceptor, not by the wrappers, so a hand-built stub authenticates exactly as
// the wrapped calls do. That is the point of exposing it. Dialing a second
// connection to reach an unwrapped RPC would mean re-implementing the credential
// plumbing and paying for a second connection.
//
// The Client owns it: it is closed by Client.Close, so do not close it directly,
// and do not retain it past the Client's lifetime.
func (c *Client) Conn() *grpc.ClientConn { return c.conn }

// Ping forces the lazy connection open and reports whether the endpoint is
// serving, using the standard gRPC health service. It is a connectivity and TLS
// check, not an authorization one: the health service is unauthenticated, so a
// successful Ping says nothing about whether the credential is accepted.
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.health.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("jennah: ping %q: %w", c.endpoint, err)
	}
	if s := resp.GetStatus(); s != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("jennah: %q reports %s, not SERVING", c.endpoint, s)
	}
	return nil
}

// EnterpriseID returns the enterprise id supplied in Config, if any. It is
// informational; the server derives the effective enterprise from the API key.
func (c *Client) EnterpriseID() string { return c.enterpriseID }

// Close releases the underlying connection.
func (c *Client) Close() error { return c.conn.Close() }

// Agent returns a handle scoped to a single agent workspace. It performs no
// network call; the workspace need not exist yet (see Spawn).
func (c *Client) Agent(agentInstanceID string) *Agent {
	a := &Agent{id: agentInstanceID, c: c}
	a.Memory = memoryAPI{a: a}
	a.Logs = logsAPI{a: a}
	a.Vectors = vectorsAPI{a: a}
	a.Graph = graphAPI{a: a}
	return a
}

// SpawnInput are the parameters for Spawn.
type SpawnInput struct {
	AgentInstanceID string // caller-chosen, unique within the enterprise
	AgentName       string
	Region          string // optional Jennah region id; empty uses the platform default
}

// Spawn creates a new agent workspace and returns a handle to it. The
// enterprise is taken from the credential, never from this call.
func (c *Client) Spawn(ctx context.Context, in SpawnInput) (*Agent, error) {
	resp, err := c.agents.CreateAgent(ctx, &agentv1.CreateAgentRequest{
		AgentInstanceId: in.AgentInstanceID,
		AgentName:       in.AgentName,
		Region:          in.Region,
	})
	if err != nil {
		return nil, err
	}
	return c.Agent(resp.GetAgent().GetAgentInstanceId()), nil
}

// ListInput are the parameters for List.
type ListInput struct {
	PageSize  int32
	PageToken string
}

// List returns a page of the caller's agent workspaces.
func (c *Client) List(ctx context.Context, in ListInput) (*agentv1.ListAgentsResponse, error) {
	return c.agents.ListAgents(ctx, &agentv1.ListAgentsRequest{
		PageSize:  in.PageSize,
		PageToken: in.PageToken,
	})
}

// resolveSource checks configured credentials, environment variables, and stored sessions.
// Unrenewable expired sessions return errors immediately during source resolution.
func resolveSource(cfg Config) (jennahcreds.Source, error) {
	if cfg.Credentials != nil {
		return cfg.Credentials, nil
	}
	resolved, err := jennahcreds.Resolve(cfg.APIKey)
	if err != nil {
		return nil, err
	}
	if resolved.Session == nil {
		// An API key: nothing to renew, and nothing that expires.
		return jennahcreds.NewStatic(resolved.Credential, resolved.Origin), nil
	}

	source := jennahcreds.NewSessionSource(resolved.Session, resolved.Origin)
	// An expired session is refused here only when nothing can renew it. Once it
	// can, an expired access token is an ordinary condition the first call
	// resolves, and refusing to construct would be refusing a client that works.
	if resolved.Session.Expired(time.Now()) && !resolved.Session.Renewable() {
		return nil, jennahcreds.ErrSessionExpired
	}
	return source, nil
}

// Attaches client connection to session sources for token renewal RPCs.
func (c *Client) attachRenewer(source jennahcreds.Source) {
	src, ok := source.(*jennahcreds.SessionSource)
	if !ok {
		return
	}
	src.SetRenewer(func(ctx context.Context, refreshToken string) (*jennahcreds.Renewal, error) {
		// No enterprise id: this renews the session in place. Passing one would
		// switch which enterprise the token is scoped to, which is a caller's
		// decision (Auth.Session.Refresh), never a side effect of a token expiring.
		resp, err := c.auth.RefreshToken(withRenewalCall(ctx), &authv1.RefreshTokenRequest{RefreshToken: refreshToken})
		if err != nil {
			return nil, err
		}
		return &jennahcreds.Renewal{
			AccessToken:  resp.GetAccessToken(),
			RefreshToken: resp.GetRefreshToken(),
			ExpiresAt:    time.Now().Add(time.Duration(resp.GetExpiresIn()) * time.Second).Unix(),
		}, nil
	})
}

// bearerInterceptor attaches the credential to every unary call on the metadata
// header the server authenticates against, and renews it when the platform says
// it is spent. It is per call, not per connection: gRPC sends metadata with each
// RPC, and dialing carries none of it.
//
// The credential is asked for per call too, not captured here. A client outlives
// an access token, so one that closed over a string at dial time would present a
// value that expired hours ago and could never recover, however many times the
// credential behind it had been renewed.
//
// Attaching and renewing are one interceptor rather than two because the renewal
// needs the exact credential that was rejected, in order to tell "this is spent"
// from "someone already replaced it". Split across two interceptors, the one that
// resolved the credential would be the inner one, and the value would have to be
// smuggled back out through a mutable context holder to reach the outer one.
//
// Unary RPC interceptor. Server governance
// entitlement, permission and selector enforcement runs on unary calls alone, and
// it refuses every streaming method outside its own infrastructure set. There is
// no tenant-facing stream for a stream interceptor to credential.
func bearerInterceptor(source jennahcreds.Source) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// The renewal itself carries no credential and cannot renew.
		//
		// Both halves matter. It carries none because the platform's refresh
		// method is on its unauthenticated allowlist: the refresh token in the
		// body is the credential, and attaching the spent access token it exists
		// to replace would be pointless. It cannot renew because a renewal that
		// re-entered this path would call back into the source it is already
		// inside, which is a deadlock on the first attempt and unbounded
		// recursion if the refresh token is what got rejected.
		if isRenewalCall(ctx) {
			return invoker(ctx, method, req, reply, cc, opts...)
		}

		cred, err := source.Token(ctx)
		if err != nil {
			return fmt.Errorf("jennah: resolve credential for %s: %w", method, err)
		}
		err = invoker(withBearer(ctx, cred), method, req, reply, cc, opts...)
		if status.Code(err) != codes.Unauthenticated {
			return err
		}

		// The call was refused before it reached the operation, so it had no
		// effect and reissuing it cannot produce a second one. That is why this
		// retry asks for none of the evidence safeToReplay demands of a retry
		// after a transport failure: a commit with no idempotency key is as safe
		// to reissue here as a read, and refusing to reissue it would mean a
		// token expiring mid-program breaks exactly the calls that matter most.
		renewed, rerr := source.Renew(ctx, cred)
		if rerr != nil {
			// The sentinel indicates handling for
			// it, the original status keeps Code and IsUnauthenticated working.
			return fmt.Errorf("%w: %w", rerr, err)
		}
		// Exactly once. A second rejection is the platform's answer, not an
		// invitation to renew again.
		return invoker(withBearer(ctx, renewed), method, req, reply, cc, opts...)
	}
}

func withBearer(ctx context.Context, cred string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+cred)
}

// renewalCallKey marks the one RPC the credential interceptor must keep its
// hands off: the renewal it is itself performing.
type renewalCallKey struct{}

func withRenewalCall(ctx context.Context) context.Context {
	return context.WithValue(ctx, renewalCallKey{}, true)
}

func isRenewalCall(ctx context.Context) bool {
	v, _ := ctx.Value(renewalCallKey{}).(bool)
	return v
}
