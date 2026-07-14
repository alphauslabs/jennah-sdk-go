package jennah

import (
	"context"

	agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Agent is a handle to a single agent workspace. Obtain one with Client.Agent
// or Client.Spawn. The memory sections are reached through Memory (the unified
// transport) and the per-type conveniences Logs, Vectors, and Graph.
type Agent struct {
	id string
	c  *Client

	Memory  memoryAPI
	Logs    logsAPI
	Vectors vectorsAPI
	Graph   graphAPI
}

// ID returns the agent instance id this handle is scoped to.
func (a *Agent) ID() string { return a.id }

// Get fetches the workspace's current state.
func (a *Agent) Get(ctx context.Context) (*agentv1.AgentInstance, error) {
	resp, err := a.c.agents.GetAgent(ctx, &agentv1.GetAgentRequest{AgentInstanceId: a.id})
	if err != nil {
		return nil, err
	}
	return resp.GetAgent(), nil
}

// Destroy deletes the workspace and cascades to all of its memory. The returned
// receipt reports the commit timestamp and per-memory-type row counts removed.
func (a *Agent) Destroy(ctx context.Context) (*agentv1.DeleteAgentResponse, error) {
	return a.c.agents.DeleteAgent(ctx, &agentv1.DeleteAgentRequest{AgentInstanceId: a.id})
}

// ---------------------------------------------------------------------------
// Unified memory transport
// ---------------------------------------------------------------------------

// memoryAPI is the unified memory transport for one agent: the two endpoints
// every other convenience is built on.
type memoryAPI struct{ a *Agent }

// CommitInput is an atomic multi-type write. Any subset of the sections may be
// set; all present sections are written in one transaction.
type CommitInput struct {
	Log     *agentv1.ExecutionLogStep // execution-log step
	Vectors []*agentv1.VectorChunk    // semantic chunks to upsert
	Graph   *agentv1.GraphWrite       // graph node/edge writes
}

// Commit writes the given sections atomically and returns the commit receipt.
func (m memoryAPI) Commit(ctx context.Context, in CommitInput) (*agentv1.CommitMemoryResponse, error) {
	return m.a.c.memory.CommitMemory(ctx, &agentv1.CommitMemoryRequest{
		AgentInstanceId: m.a.id,
		Log:             in.Log,
		Vectors:         in.Vectors,
		Graph:           in.Graph,
	})
}

// QueryInput is a fused, snapshot-consistent read. Any subset of the sections
// may be set; all present sections are evaluated at one read timestamp.
type QueryInput struct {
	Semantic *agentv1.SemanticQuery // semantic/ANN section
	Graph    *agentv1.GraphQuery    // graph (GQL) section
	Log      *agentv1.LogQuery      // execution-log recency section

	// Link composes the semantic and graph sections into an additional fused
	// result; requires both of those sections to be set.
	Link            bool
	FusionDirection agentv1.FusionDirection // direction when Link is set; zero = vector-first

	// AsOf reads every section at the same historical instant. Nil reads latest.
	AsOf *timestamppb.Timestamp
}

// Query evaluates the given sections over one snapshot and returns each
// requested section's result.
func (m memoryAPI) Query(ctx context.Context, in QueryInput) (*agentv1.QueryMemoryResponse, error) {
	return m.a.c.memory.QueryMemory(ctx, &agentv1.QueryMemoryRequest{
		AgentInstanceId: m.a.id,
		Semantic:        in.Semantic,
		Graph:           in.Graph,
		Log:             in.Log,
		Link:            in.Link,
		FusionDirection: in.FusionDirection,
		AsOf:            in.AsOf,
	})
}

// ---------------------------------------------------------------------------
// Per-type conveniences: thin single-section wrappers over Commit/Query
// ---------------------------------------------------------------------------

// logsAPI wraps the execution-log section.
type logsAPI struct{ a *Agent }

// Create commits a single execution-log step (the log section of Commit).
func (l logsAPI) Create(ctx context.Context, step *agentv1.ExecutionLogStep) (*agentv1.CommitMemoryResponse, error) {
	return l.a.Memory.Commit(ctx, CommitInput{Log: step})
}

// Recent returns the agent's most recent log steps (the log section of Query).
func (l logsAPI) Recent(ctx context.Context, q *agentv1.LogQuery) (*agentv1.LogResult, error) {
	resp, err := l.a.Memory.Query(ctx, QueryInput{Log: q})
	if err != nil {
		return nil, err
	}
	return resp.GetLog(), nil
}

// vectorsAPI wraps the semantic/vector section.
type vectorsAPI struct{ a *Agent }

// Upsert commits one or more vector chunks (the vector section of Commit).
func (v vectorsAPI) Upsert(ctx context.Context, chunks ...*agentv1.VectorChunk) (*agentv1.CommitMemoryResponse, error) {
	return v.a.Memory.Commit(ctx, CommitInput{Vectors: chunks})
}

// Search runs a semantic (ANN) query (the semantic section of Query).
func (v vectorsAPI) Search(ctx context.Context, q *agentv1.SemanticQuery) (*agentv1.SemanticResult, error) {
	resp, err := v.a.Memory.Query(ctx, QueryInput{Semantic: q})
	if err != nil {
		return nil, err
	}
	return resp.GetSemantic(), nil
}

// graphAPI wraps the graph section.
type graphAPI struct{ a *Agent }

// Write commits graph node/edge writes (the graph section of Commit).
func (g graphAPI) Write(ctx context.Context, w *agentv1.GraphWrite) (*agentv1.CommitMemoryResponse, error) {
	return g.a.Memory.Commit(ctx, CommitInput{Graph: w})
}

// Query runs a GQL traversal (the graph section of Query). The gateway injects
// the tenant/agent clamp into the GQL before execution.
func (g graphAPI) Query(ctx context.Context, gql string) (*agentv1.GraphResult, error) {
	resp, err := g.a.Memory.Query(ctx, QueryInput{Graph: &agentv1.GraphQuery{Gql: gql}})
	if err != nil {
		return nil, err
	}
	return resp.GetGraph(), nil
}
