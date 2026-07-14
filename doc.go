// Package jennah is the reference Go client for the Jennah agent memory &
// context platform.
//
// It is a thin ergonomic layer over the generated gRPC stubs in
// github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1. A Client holds one
// connection and one credential; Client.Agent returns a handle scoped to a
// single agent workspace, exposing the unified memory transport
// (Agent.Memory.Commit / Agent.Memory.Query) plus the per-type conveniences
// (Agent.Logs, Agent.Vectors, Agent.Graph) that are thin single-section
// wrappers over those same two endpoints — no extra gateway routes.
//
// The unified backend is why the conveniences are section wrappers and not
// separate services: a Commit writes any subset of log/vector/graph sections in
// one transaction, and a Query evaluates any subset over one read timestamp.
// Deferred sections (server-side embedding, graph query, vector+graph fusion)
// surface as a codes.Unimplemented gRPC status until the backend enables them.
//
//	jc, err := jennah.NewClient(jennah.Config{
//		Endpoint: "jennah.alphaus.cloud:443",
//		APIKey:   "jennah_sk_...",
//	})
//	if err != nil {
//		return err
//	}
//	defer jc.Close()
//
//	a := jc.Agent("agent-abc")
//	_, err = a.Logs.Create(ctx, &jennah.ExecutionLogStep{
//		StepId:         "step-1",
//		ThoughtProcess: "deciding which tool to call",
//	})
package jennah

import agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"

// Message type aliases so callers can build requests and read results without
// importing the deep generated package path. These are the exact generated
// types; the SDK stays a thin wrapper by reusing them rather than translating.
type (
	AgentInstance    = agentv1.AgentInstance
	ExecutionLogStep = agentv1.ExecutionLogStep
	VectorChunk      = agentv1.VectorChunk
	GraphWrite       = agentv1.GraphWrite
	GraphNode        = agentv1.GraphNode
	GraphEdge        = agentv1.GraphEdge
	SemanticQuery    = agentv1.SemanticQuery
	GraphQuery       = agentv1.GraphQuery
	LogQuery         = agentv1.LogQuery

	SemanticResult = agentv1.SemanticResult
	SemanticMatch  = agentv1.SemanticMatch
	GraphResult    = agentv1.GraphResult
	LogResult      = agentv1.LogResult
	FusedResult    = agentv1.FusedResult

	CommitMemoryResponse = agentv1.CommitMemoryResponse
	QueryMemoryResponse  = agentv1.QueryMemoryResponse
	DeleteAgentResponse  = agentv1.DeleteAgentResponse

	FusionDirection = agentv1.FusionDirection
)

// Fusion directions for a linked Memory.Query. The zero value is treated as
// vector-first by the backend.
const (
	FusionUnspecified = agentv1.FusionDirection_FUSION_DIRECTION_UNSPECIFIED
	FusionVectorFirst = agentv1.FusionDirection_FUSION_DIRECTION_VECTOR_FIRST
	FusionGraphFirst  = agentv1.FusionDirection_FUSION_DIRECTION_GRAPH_FIRST
)
