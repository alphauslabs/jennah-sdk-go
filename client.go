package jennah

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"

	agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Config configures a Client.
type Config struct {
	// Endpoint is the Jennah gateway address in host:port form, e.g.
	// "jennah.alphaus.cloud:443". Required.
	Endpoint string

	// APIKey is the bearer credential sent on every request as
	// "authorization: Bearer <APIKey>" — an opaque "jennah_sk_..." API key or a
	// jennah access token. The server derives the effective enterprise from this
	// credential. Required.
	APIKey string

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

	// DialOptions are extra gRPC dial options, appended after the SDK's own
	// (transport credentials and the auth interceptor). Useful for custom
	// dialers, keepalives, or interceptors.
	DialOptions []grpc.DialOption
}

// Client is a connection to a Jennah gateway scoped to one credential. It is
// safe for concurrent use. Call Close when done.
type Client struct {
	conn         *grpc.ClientConn
	agents       agentv1.AgentServiceClient
	memory       agentv1.MemoryServiceClient
	enterpriseID string
}

// NewClient dials the gateway and returns a Client. The connection is lazy: no
// network round trip happens until the first RPC.
func NewClient(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("jennah: Config.Endpoint is required")
	}
	if cfg.APIKey == "" {
		return nil, errors.New("jennah: Config.APIKey is required")
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
	opts = append(opts, grpc.WithChainUnaryInterceptor(bearerInterceptor(cfg.APIKey)))
	opts = append(opts, cfg.DialOptions...)

	conn, err := grpc.NewClient(cfg.Endpoint, opts...)
	if err != nil {
		return nil, fmt.Errorf("jennah: dial %q: %w", cfg.Endpoint, err)
	}
	return &Client{
		conn:         conn,
		agents:       agentv1.NewAgentServiceClient(conn),
		memory:       agentv1.NewMemoryServiceClient(conn),
		enterpriseID: cfg.EnterpriseID,
	}, nil
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

// bearerInterceptor attaches the API key to every unary call on the metadata
// header the gateway authenticates against.
func bearerInterceptor(cred string) grpc.UnaryClientInterceptor {
	value := "Bearer " + cred
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", value)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
