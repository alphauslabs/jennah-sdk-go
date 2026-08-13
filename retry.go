package jennah

import (
	"context"
	"time"

	agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"
	approvalv1 "github.com/alphauslabs/jennah-sdk-go/jennah/approval/v1"
	authv1 "github.com/alphauslabs/jennah-sdk-go/jennah/auth/v1"
	billingv1 "github.com/alphauslabs/jennah-sdk-go/jennah/billing/v1"
	datastorev1 "github.com/alphauslabs/jennah-sdk-go/jennah/datastore/v1"
	platformv1 "github.com/alphauslabs/jennah-sdk-go/jennah/platform/v1"
	gax "github.com/googleapis/gax-go/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// RetryPolicy configures automatic retries. The zero value is the default policy
// described on Config.Retry.
//
// The loop itself is gax's, using the AIP-4221 jittered backoff.
// This package defines which calls may be replayed and how many times.
type RetryPolicy struct {
	// Disabled turns retries off completely, including for reads.
	Disabled bool

	// MaxAttempts counts the first try. 0 uses 3; 1 means no retry.
	//
	// gax omits an attempt cap on purpose, since a deadline bounds a retry loop more
	// meaningfully than a count does. It is kept here because an agent commonly runs
	// with no deadline at all, and an uncapped loop against a genuinely unreachable
	// endpoint would then never return.
	MaxAttempts int

	// BaseBackoff is the ceiling on the wait before the second attempt, growing by
	// two from there. 0 uses 100ms.
	BaseBackoff time.Duration

	// MaxBackoff caps the growth. 0 uses 2s.
	MaxBackoff time.Duration
}

func (p RetryPolicy) attempts() int {
	if p.MaxAttempts > 0 {
		return p.MaxAttempts
	}
	return 3
}

func (p RetryPolicy) baseBackoff() time.Duration {
	if p.BaseBackoff > 0 {
		return p.BaseBackoff
	}
	return 100 * time.Millisecond
}

func (p RetryPolicy) maxBackoff() time.Duration {
	if p.MaxBackoff > 0 {
		return p.MaxBackoff
	}
	return 2 * time.Second
}

// retryableCodes defines gRPC codes eligible for retry.
//
// UNAVAILABLE is the transient one: a dropped connection, a draining instance, a
// rolling deploy. RESOURCE_EXHAUSTED is an entitlement limit and would return the
// same answer forever; DEADLINE_EXCEEDED means the caller's own budget is spent;
// ABORTED does not reach clients here, because the backend's read-write
// transactions retry it internally.
var retryableCodes = []codes.Code{codes.Unavailable}

// retryableCode reports whether err is one of retryableCodes. It backs IsTransient
// and keeps that answer identical to what the retry loop acts on.
func retryableCode(err error) bool {
	code := status.Code(err)
	for _, c := range retryableCodes {
		if code == c {
			return true
		}
	}
	return false
}

// safeToReplay classifies a method for retry, which is a question about the
// server's write semantics rather than about the network.
//
// Reads replay freely. Writes are eligible only when replaying one cannot produce
// a second effect, and for three of them that depends on the request:
//
//   - CommitMemory: vector chunks and graph nodes and edges are idempotent
//     upserts, so replaying them converges. An execution-log step is append-only,
//     so a replay returns AlreadyExists. A commit carrying a log section is
//     therefore not replayed, whatever else is in it.
//   - CommitData: safe exactly when the caller set IdempotencyKey, which is what
//     makes the server return the first attempt's receipt instead of applying the
//     operations twice.
//   - CreateApproval: safe exactly when the caller set RequestKey. Without it a
//     replay raises a second approval and mails a second set of approvers, and
//     notifications cannot be recalled.
//
// Everything else returns false. The default is no replay, so a method added to
// the API is safe until someone classifies it, rather than silently replayed.
func safeToReplay(method string, req any) bool {
	if replayableReads[method] {
		return true
	}
	switch r := req.(type) {
	case *agentv1.CommitMemoryRequest:
		return r.GetLog() == nil
	case *datastorev1.CommitDataRequest:
		return r.GetIdempotencyKey() != ""
	case *approvalv1.CreateApprovalRequest:
		return r.GetRequestKey() != ""
	}
	return false
}

// replayableReads is every method that reads without writing, so replaying one
// costs a round trip and nothing else. Enumerated rather than pattern-matched on
// the name, because "Get", "List" and "Query" are conventions the server is not
// obliged to keep, and TestEveryMethodIsClassified fails if a new method lands in
// method retry policy list.
var replayableReads = map[string]bool{
	agentv1.AgentService_GetAgent_FullMethodName:       true,
	agentv1.AgentService_ListAgents_FullMethodName:     true,
	agentv1.MemoryService_QueryMemory_FullMethodName:   true,
	agentv1.MemoryService_InspectMemory_FullMethodName: true,

	datastorev1.DatasetService_GetDataset_FullMethodName:   true,
	datastorev1.DatasetService_ListDatasets_FullMethodName: true,
	datastorev1.SchemaService_GetSchema_FullMethodName:     true,
	datastorev1.DataService_QueryData_FullMethodName:       true,

	authv1.AuthService_WhoAmI_FullMethodName:          true,
	authv1.AuthService_ListApiKeys_FullMethodName:     true,
	authv1.AuthService_ListMembers_FullMethodName:     true,
	authv1.AuthService_ListInvitations_FullMethodName: true,
	authv1.AuthService_ListPermissions_FullMethodName: true,
	authv1.AuthService_ListRoles_FullMethodName:       true,
	authv1.AuthService_GetRole_FullMethodName:         true,
	authv1.AuthService_PollDeviceLogin_FullMethodName: true,

	approvalv1.ApprovalService_GetApproval_FullMethodName:             true,
	approvalv1.ApprovalService_ListApprovals_FullMethodName:           true,
	approvalv1.ApprovalService_ListApprovers_FullMethodName:           true,
	approvalv1.ApprovalService_DescribeApprovalByToken_FullMethodName: true,

	billingv1.BillingService_GetBillingState_FullMethodName: true,

	platformv1.PlatformService_ListLocations_FullMethodName: true,

	healthpb.Health_Check_FullMethodName: true,
	healthpb.Health_List_FullMethodName:  true,
}

// conditionalReplay is the writes whose safety depends on the request, decided in
// safeToReplay. Listed so the classification test can see they were considered.
var conditionalReplay = map[string]bool{
	agentv1.MemoryService_CommitMemory_FullMethodName:        true,
	datastorev1.DataService_CommitData_FullMethodName:        true,
	approvalv1.ApprovalService_CreateApproval_FullMethodName: true,
}

// neverReplay contains methods excluded from automatic retries.
// It exists so TestEveryMethodIsClassified can prove no method was simply
// overlooked: a new RPC lands in none of these three sets and fails the test,
// which is the point at which someone has to think about its write semantics.
var neverReplay = map[string]bool{
	// Creating or destroying a resource twice is not the same as once.
	agentv1.AgentService_CreateAgent_FullMethodName:         true,
	agentv1.AgentService_DeleteAgent_FullMethodName:         true,
	datastorev1.DatasetService_CreateDataset_FullMethodName: true,
	datastorev1.DatasetService_DeleteDataset_FullMethodName: true,

	// Schema work is asynchronous and mutates the catalog; a replay races the
	// declaration already running.
	datastorev1.SchemaService_DeclareTables_FullMethodName: true,

	// Closes an edge's validity and inserts its replacement. A replay finds the
	// prior edge already closed.
	agentv1.MemoryService_SupersedeEdge_FullMethodName: true,

	// A long poll that reports a pending approval as success. Retrying inside the
	// interceptor would stack another 30-second wait inside the caller's budget;
	// ApprovalsAPI.WaitUntilDecided owns the loop instead.
	approvalv1.ApprovalService_WaitApproval_FullMethodName: true,

	// Each of these sends mail, records a decision, or moves an approval to a
	// terminal state. A notification cannot be recalled.
	approvalv1.ApprovalService_CancelApproval_FullMethodName:             true,
	approvalv1.ApprovalService_ResendApprovalNotification_FullMethodName: true,
	approvalv1.ApprovalService_SubmitApprovalDecision_FullMethodName:     true,
	approvalv1.ApprovalService_AddApprover_FullMethodName:                true,
	approvalv1.ApprovalService_RemoveApprover_FullMethodName:             true,

	// Session and credential mutations. A replayed refresh or logout can revoke the
	// token the first attempt just issued, and a replayed mint leaves a key nobody
	// holds the plaintext for.
	authv1.AuthService_StartLogin_FullMethodName:       true,
	authv1.AuthService_CompleteLogin_FullMethodName:    true,
	authv1.AuthService_ExchangeCode_FullMethodName:     true,
	authv1.AuthService_StartDeviceLogin_FullMethodName: true,
	authv1.AuthService_RefreshToken_FullMethodName:     true,
	authv1.AuthService_Logout_FullMethodName:           true,
	authv1.AuthService_CreateApiKey_FullMethodName:     true,
	authv1.AuthService_RevokeApiKey_FullMethodName:     true,

	// Membership, role and enterprise administration.
	authv1.AuthService_InviteMember_FullMethodName:     true,
	authv1.AuthService_RevokeInvitation_FullMethodName: true,
	authv1.AuthService_AcceptInvitation_FullMethodName: true,
	authv1.AuthService_ChangeMemberRole_FullMethodName: true,
	authv1.AuthService_RemoveMember_FullMethodName:     true,
	authv1.AuthService_TransferRoot_FullMethodName:     true,
	authv1.AuthService_UpdateEnterprise_FullMethodName: true,
	authv1.AuthService_CreateRole_FullMethodName:       true,
	authv1.AuthService_UpdateRole_FullMethodName:       true,
	authv1.AuthService_DeleteRole_FullMethodName:       true,

	// Commits the enterprise to a paid agreement.
	billingv1.BillingService_BindMarketplaceRegistration_FullMethodName:    true,
	billingv1.BillingService_ResolveMarketplaceRegistration_FullMethodName: true,
}

// retryInterceptor replays eligible calls on transient errors using gax.Invoke.
// Non-retryable calls bypass gax. If context expires during backoff, gax returns
// the context error.
func retryInterceptor(p RetryPolicy) grpc.UnaryClientInterceptor {
	attempts, base, max := p.attempts(), p.baseBackoff(), p.maxBackoff()
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if attempts < 2 || !safeToReplay(method, req) {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		return gax.Invoke(ctx, func(ctx context.Context, _ gax.CallSettings) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}, gax.WithRetry(func() gax.Retryer {
			// One retryer per Invoke, so the attempt budget is per call and not
			// shared across the Client's lifetime.
			return &cappedRetryer{
				left: attempts - 1,
				inner: gax.OnCodes(retryableCodes, gax.Backoff{
					Initial:    base,
					Max:        max,
					Multiplier: 2,
				}),
			}
		}))
	}
}

// cappedRetryer is gax's code-based retryer with an attempt budget, which gax
// leaves to the caller to build (see RetryPolicy.MaxAttempts).
type cappedRetryer struct {
	inner gax.Retryer
	left  int
}

func (r *cappedRetryer) Retry(err error) (time.Duration, bool) {
	if r.left <= 0 {
		return 0, false
	}
	// Ask the inner retryer first, so exhausting the budget never advances the
	// backoff for an error that was not retryable anyway.
	pause, ok := r.inner.Retry(err)
	if !ok {
		return 0, false
	}
	r.left--
	return pause, true
}
