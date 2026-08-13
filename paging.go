package jennah

import (
	"context"
	"iter"

	agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"
	approvalv1 "github.com/alphauslabs/jennah-sdk-go/jennah/approval/v1"
	authv1 "github.com/alphauslabs/jennah-sdk-go/jennah/auth/v1"
	datastorev1 "github.com/alphauslabs/jennah-sdk-go/jennah/datastore/v1"
	"google.golang.org/protobuf/proto"
)

// Iterators over the paged listings. Each one walks every page, so a caller
// stops thinking about cursors:
//
//	for ds, err := range jc.Datasets.All(ctx, nil) {
//		if err != nil {
//			return err
//		}
//		use(ds)
//	}
//
// The error is yielded once, with a nil item, and the sequence then ends. Break
// out of the loop whenever you like; no further page is fetched. The request you
// pass is never modified: each iterator clones it before setting a page token, so
// the same request can drive several walks.

// pageAll is the shared loop. fetch runs one page and returns its items and the
// token for the next; an empty token ends the walk.
func pageAll[T any](yield func(T, error) bool, fetch func(token string) ([]T, string, error)) {
	var token string
	for {
		items, next, err := fetch(token)
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		for _, it := range items {
			if !yield(it, nil) {
				return
			}
		}
		if next == "" {
			return
		}
		token = next
	}
}

// All walks every agent workspace the credential can reach.
func (c *Client) All(ctx context.Context, in ListInput) iter.Seq2[*agentv1.AgentInstance, error] {
	return func(yield func(*agentv1.AgentInstance, error) bool) {
		pageAll(yield, func(token string) ([]*agentv1.AgentInstance, string, error) {
			resp, err := c.agents.ListAgents(ctx, &agentv1.ListAgentsRequest{
				PageSize:  in.PageSize,
				PageToken: token,
			})
			return resp.GetAgents(), resp.GetNextPageToken(), err
		})
	}
}

// All walks every dataset the credential can reach.
func (d DatasetsAPI) All(ctx context.Context, in *datastorev1.ListDatasetsRequest) iter.Seq2[*datastorev1.Dataset, error] {
	return func(yield func(*datastorev1.Dataset, error) bool) {
		pageAll(yield, func(token string) ([]*datastorev1.Dataset, string, error) {
			req := clonePage(in, &datastorev1.ListDatasetsRequest{})
			req.PageToken = token
			resp, err := d.c.datasets.ListDatasets(ctx, req)
			return resp.GetDatasets(), resp.GetNextPageToken(), err
		})
	}
}

// All walks every approval matching the request's filters.
func (a ApprovalsAPI) All(ctx context.Context, in *approvalv1.ListApprovalsRequest) iter.Seq2[*approvalv1.Approval, error] {
	return func(yield func(*approvalv1.Approval, error) bool) {
		pageAll(yield, func(token string) ([]*approvalv1.Approval, string, error) {
			req := clonePage(in, &approvalv1.ListApprovalsRequest{})
			req.PageToken = token
			resp, err := a.c.approvals.ListApprovals(ctx, req)
			return resp.GetApprovals(), resp.GetNextPageToken(), err
		})
	}
}

// All walks every approver allowlist entry.
func (a ApproversAPI) All(ctx context.Context) iter.Seq2[*approvalv1.ApproverAllowlistEntry, error] {
	return func(yield func(*approvalv1.ApproverAllowlistEntry, error) bool) {
		pageAll(yield, func(token string) ([]*approvalv1.ApproverAllowlistEntry, string, error) {
			resp, err := a.c.approvals.ListApprovers(ctx, &approvalv1.ListApproversRequest{PageToken: token})
			return resp.GetEntries(), resp.GetNextPageToken(), err
		})
	}
}

// All walks every API key in the enterprise.
func (k KeysAPI) All(ctx context.Context) iter.Seq2[*authv1.ApiKey, error] {
	return func(yield func(*authv1.ApiKey, error) bool) {
		pageAll(yield, func(token string) ([]*authv1.ApiKey, string, error) {
			resp, err := k.c.auth.ListApiKeys(ctx, &authv1.ListApiKeysRequest{PageToken: token})
			return resp.GetKeys(), resp.GetNextPageToken(), err
		})
	}
}

// All walks every member of the enterprise.
func (m MembersAPI) All(ctx context.Context) iter.Seq2[*authv1.Member, error] {
	return func(yield func(*authv1.Member, error) bool) {
		pageAll(yield, func(token string) ([]*authv1.Member, string, error) {
			resp, err := m.c.auth.ListMembers(ctx, &authv1.ListMembersRequest{PageToken: token})
			return resp.GetMembers(), resp.GetNextPageToken(), err
		})
	}
}

// All walks every invitation, whatever its status.
func (i InvitationsAPI) All(ctx context.Context) iter.Seq2[*authv1.Invitation, error] {
	return func(yield func(*authv1.Invitation, error) bool) {
		pageAll(yield, func(token string) ([]*authv1.Invitation, string, error) {
			resp, err := i.c.auth.ListInvitations(ctx, &authv1.ListInvitationsRequest{PageToken: token})
			return resp.GetInvitations(), resp.GetNextPageToken(), err
		})
	}
}

// All walks every custom role in the enterprise.
func (r RolesAPI) All(ctx context.Context) iter.Seq2[*authv1.CustomRole, error] {
	return func(yield func(*authv1.CustomRole, error) bool) {
		pageAll(yield, func(token string) ([]*authv1.CustomRole, string, error) {
			resp, err := r.c.auth.ListRoles(ctx, &authv1.ListRolesRequest{PageToken: token})
			return resp.GetRoles(), resp.GetNextPageToken(), err
		})
	}
}

// ---------------------------------------------------------------------------
// Inspect: four cursors, four iterators
// ---------------------------------------------------------------------------

// The inspect listings page independently, one cursor per section, which is why
// Memory.Inspect hands back the whole response rather than a section's rows. These
// iterators are the other half of that: each one drives a single section to
// exhaustion, so a caller who wants one section never sees a cursor at all.
//
// Each call requests only its own section, so walking chunks does not also fetch
// nodes, edges and log steps on every page.

// AllChunks walks every stored vector chunk.
func (v vectorsAPI) AllChunks(ctx context.Context, in *agentv1.InspectVectors) iter.Seq2[*agentv1.VectorChunkInfo, error] {
	return func(yield func(*agentv1.VectorChunkInfo, error) bool) {
		pageAll(yield, func(token string) ([]*agentv1.VectorChunkInfo, string, error) {
			req := clonePage(in, &agentv1.InspectVectors{})
			req.PageToken = token
			resp, err := v.a.Memory.Inspect(ctx, InspectInput{Vectors: req})
			return resp.GetVectors().GetChunks(), resp.GetNextChunkToken(), err
		})
	}
}

// AllNodes walks every stored graph node.
func (g graphAPI) AllNodes(ctx context.Context, nodeLimit int32) iter.Seq2[*agentv1.GraphNode, error] {
	return func(yield func(*agentv1.GraphNode, error) bool) {
		pageAll(yield, func(token string) ([]*agentv1.GraphNode, string, error) {
			resp, err := g.a.Memory.Inspect(ctx, InspectInput{Graph: &agentv1.InspectGraph{
				NodeLimit:     nodeLimit,
				NodePageToken: token,
			}})
			return resp.GetGraph().GetNodes(), resp.GetNextNodeToken(), err
		})
	}
}

// AllEdges walks every stored graph edge.
func (g graphAPI) AllEdges(ctx context.Context, edgeLimit int32) iter.Seq2[*agentv1.GraphEdge, error] {
	return func(yield func(*agentv1.GraphEdge, error) bool) {
		pageAll(yield, func(token string) ([]*agentv1.GraphEdge, string, error) {
			resp, err := g.a.Memory.Inspect(ctx, InspectInput{Graph: &agentv1.InspectGraph{
				EdgeLimit:     edgeLimit,
				EdgePageToken: token,
			}})
			return resp.GetGraph().GetEdges(), resp.GetNextEdgeToken(), err
		})
	}
}

// AllSteps walks every stored execution-log step.
func (l logsAPI) AllSteps(ctx context.Context, in *agentv1.InspectLog) iter.Seq2[*agentv1.ExecutionLogStep, error] {
	return func(yield func(*agentv1.ExecutionLogStep, error) bool) {
		pageAll(yield, func(token string) ([]*agentv1.ExecutionLogStep, string, error) {
			req := clonePage(in, &agentv1.InspectLog{})
			req.PageToken = token
			resp, err := l.a.Memory.Inspect(ctx, InspectInput{Log: req})
			return resp.GetLog().GetSteps(), resp.GetNextLogToken(), err
		})
	}
}

// clonePage copies the caller's request so an iterator can set a page token
// without mutating what it was handed. A nil request yields the empty fallback.
func clonePage[T proto.Message](in, empty T) T {
	if !in.ProtoReflect().IsValid() {
		return empty
	}
	return proto.Clone(in).(T)
}
