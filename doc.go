// Package jennah is the reference Go client for the Jennah agent memory &
// context platform.
//
// It is a thin ergonomic layer over the generated gRPC stubs under
// github.com/alphauslabs/jennah-sdk-go/jennah/... A Client holds one connection
// and one credential and reaches every service the platform publishes:
//
//   - Agent workspaces and memory: Client.Spawn, Client.List, Client.Agent(id).
//     A workspace handle exposes the unified memory transport
//     (Agent.Memory.Commit / Agent.Memory.Query / Agent.Memory.Inspect) plus the
//     per-type conveniences (Agent.Logs, Agent.Vectors, Agent.Graph) that are
//     thin single-section wrappers over those same endpoints, not extra routes.
//   - Application datasets: Client.Datasets and Client.Dataset(id), whose handle
//     carries Schema and Data.
//   - Client.Auth for identity, enterprise administration, API keys, and roles;
//     Client.Approvals for human approvals; Client.Billing; Client.Platform.
//
// Agents own the bare top-level verbs because they are the platform's primary
// object and had them first; every other service is namespaced, since one
// Client.List cannot mean agents, datasets, approvals, and keys at once.
//
// The unified memory backend is why the memory conveniences are section wrappers
// and not separate services: a Commit writes any subset of log/vector/graph
// sections in one transaction, and a Query evaluates any subset over one read
// timestamp.
//
// Two things still answer codes.Unimplemented. Vector+graph fusion (QueryInput's
// Link) is not built yet. Server-side embedding is built, but is configured per
// region: a chunk or query submitted without an embedding is embedded by the
// backend where that region has an embedding endpoint, and refused with
// "supply a precomputed embedding" where it does not. A caller that cannot
// tolerate that refusal should send its own vectors.
//
// The client speaks gRPC to the platform's public gRPC endpoint
// (DefaultEndpoint, "jennah-grpc.alphaus.cloud:443"), not to the HTTP gateway on
// jennah.alphaus.cloud. Both front doors terminate on the same server behind the
// same authentication and authorization chain, so a credential gets the same
// decision on the same operation either way; what gRPC avoids is the gateway's
// JSON transcoding. TLS terminates at the load balancer on 443, so nothing but
// ordinary transport credentials is needed: leave Config.Endpoint empty.
//
// Reaching an operation here therefore grants nothing extra. An API key cannot
// hold management-class scopes, so the enterprise-administration calls under
// Client.Auth refuse a key-authenticated caller exactly as they do over HTTP.
// Transport selects which credential you may present, never what it may do.
//
//	jc, err := jennah.NewClient(jennah.Config{
//		APIKey: "jennah_sk_...", // or an access token from a sign-in flow
//	})
//	if err != nil {
//		return err
//	}
//	defer jc.Close()
//
// # Credentials
//
// Config.APIKey is optional. Left empty, the client resolves a credential
// itself, taking the first that answers and consulting no further:
//
//  1. Config.Credentials, a source the program supplies;
//  2. Config.APIKey;
//  3. the JENNAH_API_KEY environment variable;
//  4. the session stored by "jnh login".
//
// Default client configurations resolve credentials from CLI logins or environment variables.
//
// Stored session endpoints are ignored because CLI sessions record HTTP gateway hostnames.
//
// A resolved session renews itself: when the platform rejects the access token,
// the client refreshes it and reissues the call once. The reissue asks for none
// of the evidence a retry after a transport failure requires, because an
// UNAUTHENTICATED rejection is decided before the call reaches the operation, so
// there is nothing to have applied twice. Renewals are written back to the shared
// credentials file, since refreshing rotates the refresh token and a renewal kept
// in memory would strand every other client reading that file. An API key is
// never renewed; a rejection means the key was refused.
//
//	a := jc.Agent("agent-abc")
//	_, err = a.Logs.Create(ctx, &jennah.ExecutionLogStep{
//		StepId:         "step-1",
//		ThoughtProcess: "deciding which tool to call",
//	})
//
// Beyond one-to-one wrappers, the client carries the behavior a caller would
// otherwise write per project, each piece encoding a server contract rather than
// inventing policy:
//
//   - ApprovalsAPI.WaitUntilDecided loops the server's 30-second long poll until
//     the approval is terminal, because a reached ceiling comes back as a success
//     carrying a still-pending approval.
//   - All iterators (Client.All, DatasetsAPI.All, and one per inspect section)
//     page to exhaustion, so cursors never reach calling code.
//   - Retries run on gax (github.com/googleapis/gax-go/v2), replaying UNAVAILABLE
//     only where a replay cannot produce a second effect, judged from the request
//     and not merely the method. See Config.Retry.
//   - Code, IsAlreadyCommitted, IsLimitExceeded, IsUnsupported, IsDenied,
//     IsUnauthenticated, IsNotFound and IsTransient name the statuses this API
//     actually returns, so callers never match on message text.
//
// Generated messages and enums are aliased in types.go,
// so building a request needs no deep import. For anything these wrappers do not
// cover, Client.Conn returns the live connection, and stubs built on it carry the
// same credential.
package jennah

import agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"

// Fusion directions for a linked Memory.Query. The zero value is treated as
// vector-first by the backend.
//
// These three predate the faithful enum re-exports in types.go, which also
// publishes them as FusionDirection_FUSION_DIRECTION_*. They stay because
// callers already use them.
const (
	FusionUnspecified = agentv1.FusionDirection_FUSION_DIRECTION_UNSPECIFIED
	FusionVectorFirst = agentv1.FusionDirection_FUSION_DIRECTION_VECTOR_FIRST
	FusionGraphFirst  = agentv1.FusionDirection_FUSION_DIRECTION_GRAPH_FIRST
)
