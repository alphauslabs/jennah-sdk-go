package credentials

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// EnvAPIKey is the environment variable an API key is read from when no
// credential is supplied explicitly.
const EnvAPIKey = "JENNAH_API_KEY"

// apiKeyPrefix is the reserved prefix the platform's API keys carry. The server
// branches on it to decide which credential it was handed, so it is also how a
// client can tell what it resolved without asking anyone.
const apiKeyPrefix = "jennah_sk_"

// Kind is what sort of credential was resolved.
type Kind int

const (
	// KindUnknown is the zero value, held by no resolved credential.
	KindUnknown Kind = iota
	// KindAPIKey is an opaque service credential. It does not expire and cannot
	// be renewed, so a rejection means the key itself was refused.
	KindAPIKey
	// KindSession is a signed-in user's access token, backed by a refresh token.
	// It expires, and a rejection is ordinarily just that.
	KindSession
)

func (k Kind) String() string {
	switch k {
	case KindAPIKey:
		return "api key"
	case KindSession:
		return "session"
	default:
		return "unknown"
	}
}

// Origin is where a resolved credential came from.
type Origin int

const (
	// OriginUnknown is the zero value, held by no resolved credential.
	OriginUnknown Origin = iota
	// OriginExplicit is a credential the program supplied in its configuration.
	OriginExplicit
	// OriginEnvironment is a credential read from EnvAPIKey.
	OriginEnvironment
	// OriginFile is the stored session at Path.
	OriginFile
)

func (o Origin) String() string {
	switch o {
	case OriginExplicit:
		return "explicit configuration"
	case OriginEnvironment:
		return "$" + EnvAPIKey
	case OriginFile:
		return "stored session"
	default:
		return "unknown"
	}
}

// Source supplies the bearer to present, per call rather than once.
//
// It is an interface because the credential a long-lived client holds is not a
// constant: an access token expires while the program that resolved it is still
// running, and a client that captured a string at construction could only ever
// present the value that has since gone stale. Asking per call is what lets a
// renewal take effect without rebuilding the client, and lets a program plug in
// a credential this package knows nothing about, such as one from a secret
// manager.
type Source interface {
	// Token returns the bearer for the next call.
	Token(ctx context.Context) (string, error)

	// Kind reports what sort of credential this is, so a caller can tell an
	// expired session apart from a refused key.
	Kind() Kind

	// Origin reports where the credential was resolved from.
	Origin() Origin

	// Renewable reports whether Renew can do anything at all. An API key is not
	// renewable: it has no refresh token, so a rejection is final and attempting
	// a renewal would turn a clear error into a confusing one.
	Renewable() bool

	// Renew obtains a fresh bearer after presented was rejected, and returns the
	// replacement.
	//
	// It takes the value that failed so it can tell "this credential is spent"
	// from "someone already replaced it": if the current credential no longer
	// matches presented, a concurrent caller or another process has renewed
	// already and the right answer is that replacement, not another renewal.
	//
	// A Source that is not Renewable returns an error describing the rejection.
	Renew(ctx context.Context, presented string) (string, error)
}

// Static is a Source holding one credential that never changes: an API key, or a
// session this client will not renew.
type Static struct {
	credential string
	kind       Kind
	origin     Origin
}

// NewStatic returns a Source for a credential supplied directly. The kind is
// inferred from the credential itself, on the same prefix the server branches on.
func NewStatic(credential string, origin Origin) *Static {
	return &Static{credential: credential, kind: kindOf(credential), origin: origin}
}

func (s *Static) Token(context.Context) (string, error) { return s.credential, nil }
func (s *Static) Kind() Kind                            { return s.kind }
func (s *Static) Origin() Origin                        { return s.origin }
func (s *Static) Renewable() bool                       { return false }

func (s *Static) Renew(context.Context, string) (string, error) {
	if s.kind == KindAPIKey {
		return "", ErrKeyRefused
	}
	return "", ErrSessionExpired
}

// ErrKeyRefused reports that an API key was rejected. A key does not expire, so
// this is the key itself being refused: revoked, expired at the server, or
// simply wrong. It is never a reason to attempt a renewal.
var ErrKeyRefused = errors.New("jennah: the API key was refused (check the configured key or $" + EnvAPIKey + ")")

// ErrSessionExpired reports a session that can no longer authenticate and could
// not be renewed. It names the command that re-establishes one, because that is
// the only thing the holder can do about it.
var ErrSessionExpired = errors.New("jennah: the stored session has expired (run: jnh login)")

// ErrNoCredential reports that no source yielded a credential. It names every
// way to supply one, because a caller seeing this has not chosen between them
// and is not yet in a position to know they had to.
var ErrNoCredential = errors.New("jennah: no credential found (set one explicitly, set $" +
	EnvAPIKey + ", or run: jnh login)")

// kindOf classifies a credential by the prefix the server branches on.
func kindOf(credential string) Kind {
	if strings.HasPrefix(credential, apiKeyPrefix) {
		return KindAPIKey
	}
	return KindSession
}

// Resolved is the outcome of Resolve: the credential, what it is, and where it
// came from. It carries the stored session when one was read, so a caller that
// wants to renew has what renewal needs without reading the file a second time.
type Resolved struct {
	// Credential is the bearer to present.
	Credential string
	// Kind is what sort of credential it is.
	Kind Kind
	// Origin is which source yielded it.
	Origin Origin
	// Session is the stored session Credential came from, or nil when the
	// credential did not come from the file.
	Session *Session
}

// Resolve ranks the ways a client can be credentialed and returns the first that
// yields one: the explicit credential, then EnvAPIKey, then the stored session.
//
// First match wins and later sources are not consulted. That ordering is what
// makes an explicit credential usable on a machine that has never logged in and
// has no session file, and equally makes an explicit credential override a
// session rather than compete with it. Because a later source is never read, a
// missing or unreadable session file cannot fail a caller who supplied a key.
//
// A file that exists but cannot be interpreted is reported rather than treated
// as absent: it is the last source, so nothing else is going to answer, and
// silently reporting "no credential" would send the caller looking for the wrong
// problem.
func Resolve(explicit string) (*Resolved, error) {
	if c := strings.TrimSpace(explicit); c != "" {
		return &Resolved{Credential: c, Kind: kindOf(c), Origin: OriginExplicit}, nil
	}
	if c := strings.TrimSpace(os.Getenv(EnvAPIKey)); c != "" {
		return &Resolved{Credential: c, Kind: kindOf(c), Origin: OriginEnvironment}, nil
	}

	sess, err := Load()
	switch {
	case errors.Is(err, ErrNoSession):
		return nil, ErrNoCredential
	case err != nil:
		return nil, err
	}
	if strings.TrimSpace(sess.AccessToken) == "" {
		return nil, fmt.Errorf("%w: the stored session holds no access token", ErrNoCredential)
	}
	return &Resolved{
		Credential: sess.AccessToken,
		Kind:       kindOf(sess.AccessToken),
		Origin:     OriginFile,
		Session:    sess,
	}, nil
}
