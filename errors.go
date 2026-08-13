package jennah

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Helpers for identifying error categories.
//
// Anything not covered here is read with Code, or with the status package
// directly.

// Code returns the gRPC code of err, or codes.OK when err is nil. An error from
// outside gRPC reads as codes.Unknown.
//
// Context errors are the exception, and they are mapped rather than reported as
// Unknown, because they arrive here unwrapped: the retry loop returns the caller's
// ctx.Err() when a deadline lands during a backoff, and a bare
// context.DeadlineExceeded is not a status. So a call abandoned mid-retry reads as
// DeadlineExceeded, which is what happened, rather than as Unknown.
func Code(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return status.FromContextError(err).Code()
	}
	return status.Code(err)
}

// IsAlreadyCommitted reports whether the write lost a race with itself: the row is
// already there.
//
// This is what a re-committed execution-log step returns, because log steps are
// append-only while vector chunks and graph writes are idempotent upserts. An agent
// that retries a commit after an ambiguous failure should usually treat this as
// success, since the step it wanted recorded is recorded.
func IsAlreadyCommitted(err error) bool { return Code(err) == codes.AlreadyExists }

// IsLimitExceeded reports whether an entitlement limit refused the call, meaning
// the plan's ceiling was reached rather than anything transient. Retrying returns
// the same answer; read Client.Billing.State to see which ceiling bound.
func IsLimitExceeded(err error) bool { return Code(err) == codes.ResourceExhausted }

// IsUnsupported reports whether the platform declined because the capability is not
// available, rather than because the caller did anything wrong.
//
// Two cases produce it today: vector and graph fusion, which is not built, and a
// memory write or query that left the embedding to the server in a region with no
// embedding endpoint configured. The second is fixed by sending a precomputed
// embedding.
func IsUnsupported(err error) bool { return Code(err) == codes.Unimplemented }

// IsDenied reports whether the credential was understood and refused: a permission
// it lacks, a selector that does not reach the resource, or a handler that refuses
// API-key callers outright. Presenting the same credential again cannot help.
func IsDenied(err error) bool { return Code(err) == codes.PermissionDenied }

// IsUnauthenticated reports whether the credential was rejected or missing: an
// unknown or revoked API key, or an expired access token. A token that expired is
// indicates the session should be refreshed before retrying.
func IsUnauthenticated(err error) bool { return Code(err) == codes.Unauthenticated }

// IsNotFound reports whether the named resource does not exist for this credential.
// It is also what an agent or approval outside the credential's reach returns, since
// the platform does not distinguish "absent" from "not yours" to a caller who cannot
// see it either way.
func IsNotFound(err error) bool { return Code(err) == codes.NotFound }

// IsTransient reports whether the failure is the kind the SDK retries on its own:
// a dropped connection, a draining instance, a rolling deploy. Seeing one means the
// automatic retries were exhausted, disabled, or the call was not safe to replay
// (see Config.Retry).
func IsTransient(err error) bool { return retryableCode(err) }
