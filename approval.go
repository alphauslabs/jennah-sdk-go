package jennah

import (
	"context"
	"time"

	approvalv1 "github.com/alphauslabs/jennah-sdk-go/jennah/approval/v1"
)

// ApprovalsAPI raises approvals and reads their outcome: the surface an agent uses
// to put a human in the loop before doing something consequential.
type ApprovalsAPI struct {
	c *Client

	// Approvers is the enterprise's approver allowlist.
	Approvers ApproversAPI
}

// Create raises an approval request and notifies its approvers.
//
// Set RequestKey to make the call idempotent: retrying with the same key returns
// the existing approval instead of raising a second one, which matters because the
// notification side effect cannot be taken back.
func (a ApprovalsAPI) Create(ctx context.Context, in *approvalv1.CreateApprovalRequest) (*approvalv1.CreateApprovalResponse, error) {
	return a.c.approvals.CreateApproval(ctx, in)
}

// Get reads one approval with its approvers and their decisions.
func (a ApprovalsAPI) Get(ctx context.Context, approvalID string) (*approvalv1.GetApprovalResponse, error) {
	return a.c.approvals.GetApproval(ctx, &approvalv1.GetApprovalRequest{ApprovalId: approvalID})
}

// List returns a page of the caller's approvals, optionally filtered by status,
// agent, or whether the caller is the one being awaited.
func (a ApprovalsAPI) List(ctx context.Context, in *approvalv1.ListApprovalsRequest) (*approvalv1.ListApprovalsResponse, error) {
	if in == nil {
		in = &approvalv1.ListApprovalsRequest{}
	}
	return a.c.approvals.ListApprovals(ctx, in)
}

// Cancel withdraws a pending approval.
func (a ApprovalsAPI) Cancel(ctx context.Context, approvalID string) (*approvalv1.CancelApprovalResponse, error) {
	return a.c.approvals.CancelApproval(ctx, &approvalv1.CancelApprovalRequest{ApprovalId: approvalID})
}

// Wait blocks server-side until the approval is terminal or the timeout elapses,
// then returns its current state. It is a long poll, not a stream, so it returns
// whatever is true when it returns: an approval still pending means the wait
// elapsed, and the caller loops.
//
// Give the context more room than TimeoutSeconds, or the client deadline fires
// first and the server-side wait is wasted.
func (a ApprovalsAPI) Wait(ctx context.Context, in *approvalv1.WaitApprovalRequest) (*approvalv1.WaitApprovalResponse, error) {
	return a.c.approvals.WaitApproval(ctx, in)
}

// Wait budgets. The server caps a single WaitApproval at 30 seconds and shaves
// 500ms off the caller's deadline, so a call gets a little more than the ceiling
// and no more polls are issued once too little budget remains for one to matter.
const (
	waitServerCeiling = 30 * time.Second
	waitCallSlack     = 5 * time.Second
	waitMinSlice      = time.Second

	// waitPollFloor is the least time one iteration may take. A healthy server
	// blocks for its ceiling, so this never fires against one; it exists because a
	// server that returns TimedOut immediately would otherwise turn this loop into
	// a hot spin against the endpoint.
	waitPollFloor = 250 * time.Millisecond
)

// WaitUntilDecided blocks until the approval is approved, rejected, cancelled or
// expired, and returns it. It is the loop every caller of Wait would otherwise
// write. A wait that reaches
// the server's ceiling comes back as a SUCCESS carrying the still-pending approval
// and TimedOut set, so code that treats a returned approval as a decision will act
// on a pending one.
//
// Bound it with the context. If ctx has no deadline this waits indefinitely, which
// is usually what an agent parked on a human decision wants. When the deadline
// arrives it returns the context's error together with the last pending approval it
// saw, so a caller can report what it was still waiting on.
func (a ApprovalsAPI) WaitUntilDecided(ctx context.Context, approvalID string) (*approvalv1.Approval, error) {
	var last *approvalv1.Approval
	for {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		// Stop before issuing a poll the remaining budget cannot outlive; the server
		// would return TimedOut immediately and the loop would spin.
		if dl, ok := ctx.Deadline(); ok && time.Until(dl) < waitMinSlice {
			<-ctx.Done()
			return last, ctx.Err()
		}

		started := time.Now()
		callCtx, cancel := context.WithTimeout(ctx, waitServerCeiling+waitCallSlack)
		resp, err := a.c.approvals.WaitApproval(callCtx, &approvalv1.WaitApprovalRequest{
			ApprovalId:     approvalID,
			TimeoutSeconds: int32(waitServerCeiling / time.Second),
		})
		cancel()
		if err != nil {
			// A parent that expired mid-call is the caller's deadline, not a fault of
			// the approval, so report it as such.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return last, ctxErr
			}
			return last, err
		}
		if !resp.GetTimedOut() {
			return resp.GetApproval(), nil
		}
		last = resp.GetApproval()

		if rest := waitPollFloor - time.Since(started); rest > 0 {
			select {
			case <-ctx.Done():
				return last, ctx.Err()
			case <-time.After(rest):
			}
		}
	}
}

// Resend re-notifies one approver of a pending approval.
func (a ApprovalsAPI) Resend(ctx context.Context, in *approvalv1.ResendApprovalNotificationRequest) (*approvalv1.ResendApprovalNotificationResponse, error) {
	return a.c.approvals.ResendApprovalNotification(ctx, in)
}

// DescribeByToken reads what an approver's link refers to, authorized by the token
// alone. It is what the decision page calls before a decision is submitted, so it
// requires no credentials.
func (a ApprovalsAPI) DescribeByToken(ctx context.Context, token string) (*approvalv1.DescribeApprovalByTokenResponse, error) {
	return a.c.approvals.DescribeApprovalByToken(ctx, &approvalv1.DescribeApprovalByTokenRequest{Token: token})
}

// SubmitDecision records an approve or reject, authorized by the approver's token.
//
// Pass the SeenDigest from DescribeByToken so a decision cannot land against a
// different payload than the approver actually read.
func (a ApprovalsAPI) SubmitDecision(ctx context.Context, in *approvalv1.SubmitApprovalDecisionRequest) (*approvalv1.SubmitApprovalDecisionResponse, error) {
	return a.c.approvals.SubmitApprovalDecision(ctx, in)
}

// ApproversAPI is the enterprise's approver allowlist: who may be named as an
// approver on an approval at all.
type ApproversAPI struct{ c *Client }

// List returns a page of allowlist entries.
func (a ApproversAPI) List(ctx context.Context, in *approvalv1.ListApproversRequest) (*approvalv1.ListApproversResponse, error) {
	if in == nil {
		in = &approvalv1.ListApproversRequest{}
	}
	return a.c.approvals.ListApprovers(ctx, in)
}

// Add allowlists an email address or a whole domain as an eligible approver.
func (a ApproversAPI) Add(ctx context.Context, kind approvalv1.AllowlistKind, value string) (*approvalv1.AddApproverResponse, error) {
	return a.c.approvals.AddApprover(ctx, &approvalv1.AddApproverRequest{Kind: kind, Value: value})
}

// Remove drops an allowlist entry.
func (a ApproversAPI) Remove(ctx context.Context, entryID string) (*approvalv1.RemoveApproverResponse, error) {
	return a.c.approvals.RemoveApprover(ctx, &approvalv1.RemoveApproverRequest{EntryId: entryID})
}
