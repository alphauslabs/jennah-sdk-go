package credentials

import (
	"context"
	"fmt"
	"sync"
)

// Renewal is what a Renewer returns: the replacement credential and how long it
// is good for.
//
// RefreshToken is part of the answer because renewal rotates. The platform mints
// a new refresh token and invalidates the one that was presented, so a caller
// that ignored this field would be left holding a token nothing will accept.
type Renewal struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64 // unix seconds
}

// Renewer exchanges a refresh token for a fresh credential.
//
// It is supplied rather than implemented here because renewal is the one part of
// a credential's life that is not shared: the command-line client renews over the
// HTTP gateway and the SDK renews over gRPC, so each provides its own while both
// use the same stored session.
type Renewer func(ctx context.Context, refreshToken string) (*Renewal, error)

// SessionSource is a Source backed by the stored session, able to renew itself
// when the platform rejects the access token it is holding.
//
// It is safe for concurrent use, and its renewals are single-flight: the calls
// of a busy client all expire at once, and a lock that let each of them renew
// independently would spend a rotation per call and leave all but one holding a
// token the rotation had already invalidated.
type SessionSource struct {
	mu      sync.Mutex
	session Session
	origin  Origin
	renew   Renewer
}

// NewSessionSource returns a Source over a stored session. The renewer may be
// supplied later with SetRenewer, because a client that renews over its own
// connection does not have one to renew over until it has finished being built.
func NewSessionSource(s *Session, origin Origin) *SessionSource {
	src := &SessionSource{origin: origin}
	if s != nil {
		src.session = *s
	}
	return src
}

// SetRenewer supplies how this source renews. It must be called before the first
// renewal, which in practice it always is: renewing takes a rejection, and a
// rejection takes a call.
func (s *SessionSource) SetRenewer(r Renewer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renew = r
}

// Token returns the access token to present.
func (s *SessionSource) Token(context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session.AccessToken, nil
}

// Kind is always KindSession: this source is a signed-in user's credential.
func (s *SessionSource) Kind() Kind { return KindSession }

// Origin reports where the session was resolved from.
func (s *SessionSource) Origin() Origin { return s.origin }

// Renewable reports whether this source has both a way to renew and something to
// renew with.
func (s *SessionSource) Renewable() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.renew != nil && s.session.RefreshToken != ""
}

// Renew obtains a fresh access token after presented was rejected.
//
// It spends a rotation only when it has to. Before renewing it checks whether
// the credential has already moved on: in this process, because a concurrent
// call renewed while this one waited for the lock, or on disk, because another
// process did. Either way the answer is that replacement, and renewing again
// would invalidate a perfectly good token to obtain a second one.
//
// A renewal that fails is not immediately fatal either: the likeliest cause is
// that another process rotated the token between this one reading it and using
// it, so the shared session is re-read once more before the session is declared
// dead.
func (s *SessionSource) Renew(ctx context.Context, presented string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Someone in this process got there first while we waited for the lock.
	if s.session.AccessToken != "" && s.session.AccessToken != presented {
		return s.session.AccessToken, nil
	}
	// Someone in another process may have too. This also picks up the freshest
	// refresh token, which is the only one a rotation will accept.
	if adopted, ok := s.adoptFromDisk(presented); ok {
		return adopted, nil
	}

	if s.renew == nil || s.session.RefreshToken == "" {
		return "", ErrSessionExpired
	}

	renewed, err := s.renew(ctx, s.session.RefreshToken)
	if err != nil {
		// A rotation we lost looks exactly like this. Check before giving up.
		if adopted, ok := s.adoptFromDisk(presented); ok {
			return adopted, nil
		}
		return "", fmt.Errorf("%w: %w", ErrSessionExpired, err)
	}
	if renewed == nil || renewed.AccessToken == "" {
		return "", ErrSessionExpired
	}

	s.session.AccessToken = renewed.AccessToken
	if renewed.RefreshToken != "" {
		s.session.RefreshToken = renewed.RefreshToken
	}
	s.session.ExpiresAt = renewed.ExpiresAt

	// Publish before relying on it. The token just spent is dead, so a renewal
	// kept in memory would leave every other reader of this file (the CLI
	// included) holding something that can never be renewed again. A write
	// failure is reported for the same reason: proceeding would lose the rotation
	// the moment this process exits.
	if s.origin == OriginFile {
		if err := Save(&s.session); err != nil {
			return "", err
		}
	}
	return s.session.AccessToken, nil
}

// adoptFromDisk takes up the stored session when it holds something other than
// the credential that was just rejected, and reports whether it did.
//
// A read failure is not propagated: this is an opportunistic check for someone
// else's renewal, and the caller has its own path for the case where nothing has
// replaced the credential.
func (s *SessionSource) adoptFromDisk(presented string) (string, bool) {
	if s.origin != OriginFile {
		return "", false
	}
	stored, err := Load()
	if err != nil || stored.AccessToken == "" || stored.AccessToken == presented {
		return "", false
	}
	s.session = *stored
	return s.session.AccessToken, true
}
