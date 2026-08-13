// Package credentials resolves the credential a client authenticates with, and
// owns the stored session that clients on one machine share.
//
// It holds a file format and a resolution order, and deliberately no transport:
// the command-line client renews a session over the HTTP gateway and the SDK
// renews it over gRPC, so renewal is per-client while the file is common. That
// split is what lets both clients read and write one session without either
// depending on the other's front door.
package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Session is a stored login, persisted as JSON at Path with owner-only
// permissions.
//
// The shape is fixed by the file the command-line client already writes: field
// names, their order, and the two-space indent are all part of the format, not
// presentation choices, because both clients read and write the same file and a
// session written by either must load in the other with every field intact.
type Session struct {
	// Endpoint is the address the tokens were obtained from. It is recorded for
	// provenance and is NOT an instruction to any client about where to connect:
	// the platform publishes more than one front door, and the address a client
	// using one recorded is unreachable for a client using another.
	Endpoint string `json:"endpoint"`

	// AccessToken is the short-lived bearer presented on each call.
	AccessToken string `json:"access_token"`

	// RefreshToken is the long-lived opaque token that mints a new AccessToken.
	// Renewing rotates it: the value that renewed is dead the moment the platform
	// answers, so a client that renews and does not write the replacement back
	// leaves this field holding a token nothing will accept again.
	RefreshToken string `json:"refresh_token"`

	// TokenType is the authorization scheme, always "Bearer" in practice.
	TokenType string `json:"token_type"`

	// ExpiresAt is when AccessToken expires, in unix seconds. It is advisory: the
	// platform's rejection is what actually decides a token is spent, and this is
	// used only to turn a certain failure into an immediate one. Zero means the
	// expiry is unknown, which is not the same as expired.
	ExpiresAt int64 `json:"expires_at"`
}

// ErrNoSession reports that no stored session exists. It is distinct from a
// CorruptError so a caller can tell "this machine is not logged in" from "this
// machine is logged in and something ate the file", which want opposite
// responses: the first falls through to another credential source, the second
// must be reported.
var ErrNoSession = errors.New("jennah: no stored session")

// CorruptError reports a stored session that exists but cannot be interpreted.
// It names the path, because the only useful response is to look at or remove
// that file.
type CorruptError struct {
	Path string
	Err  error
}

func (e *CorruptError) Error() string {
	return fmt.Sprintf("jennah: stored session at %s is unreadable: %v", e.Path, e.Err)
}

func (e *CorruptError) Unwrap() error { return e.Err }

// Path returns the stored session's location, one per user account.
//
// os.UserConfigDir is the platform's per-user configuration directory, which on
// Linux is ~/.config and honors XDG_CONFIG_HOME. Relocating the config directory
// therefore relocates the session, which is what makes an isolated test or a
// per-shell account possible without a flag for it.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("jennah: locate config directory: %w", err)
	}
	return filepath.Join(dir, "jennah", "credentials"), nil
}

// Load reads the stored session.
//
// A missing file returns ErrNoSession; a file that will not parse returns a
// *CorruptError. Every other read failure (a permission problem, an unreadable
// directory) is returned as itself, because it is neither of those and silently
// treating it as "not logged in" would hide it.
func Load() (*Session, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	return loadFrom(path)
}

func loadFrom(path string) (*Session, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSession
		}
		return nil, fmt.Errorf("jennah: read stored session %s: %w", path, err)
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, &CorruptError{Path: path, Err: err}
	}
	return &s, nil
}

// Save writes the session, replacing any previous one.
//
// The write is atomic: the content lands in a temporary file in the same
// directory, is flushed, and is then renamed over the target, so a concurrent
// reader observes either the whole previous session or the whole new one. The
// temporary name is unique per write rather than a fixed ".tmp" sibling, because
// two processes renewing at once would otherwise write through each other's
// half-finished file.
//
// The file is owner-readable only and its directory owner-accessible only.
func Save(s *Session) error {
	path, err := Path()
	if err != nil {
		return err
	}
	return saveTo(path, s)
}

func saveTo(path string, s *Session) error {
	if s == nil {
		return errors.New("jennah: save stored session: nothing to save")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("jennah: create config directory %s: %w", dir, err)
	}
	// Two spaces, this field order, and the absence of a trailing newline are the
	// format the command-line client writes. Matching them exactly keeps a load
	// and re-save through either client byte-identical, which is what lets the two
	// implementations coexist while the CLI is migrated onto this one.
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("jennah: encode stored session: %w", err)
	}

	// CreateTemp opens at 0600, which is the mode the session must end up with.
	f, err := os.CreateTemp(dir, "credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("jennah: create temporary session file in %s: %w", dir, err)
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op once the rename below has moved it

	if _, err := f.Write(b); err != nil {
		f.Close()
		return fmt.Errorf("jennah: write stored session: %w", err)
	}
	// Flush before renaming: a rename that beats its own content to disk would
	// lose a rotated refresh token on a crash, and a lost rotation is a session
	// nothing can renew again.
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("jennah: flush stored session: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("jennah: close stored session: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("jennah: replace stored session %s: %w", path, err)
	}
	return nil
}

// Delete removes the stored session. A session that is already absent is not an
// error: the caller wanted it gone and it is gone.
func Delete() error {
	path, err := Path()
	if err != nil {
		return err
	}
	return deleteAt(path)
}

func deleteAt(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("jennah: remove stored session %s: %w", path, err)
	}
	return nil
}

// Expired reports whether the access token's recorded expiry has passed.
//
// An unset expiry reads as not expired: the field is advisory, and a session
// carrying no expiry is one whose validity only the platform can answer.
func (s *Session) Expired(now time.Time) bool {
	if s == nil || s.ExpiresAt == 0 {
		return false
	}
	return now.After(time.Unix(s.ExpiresAt, 0))
}

// Renewable reports whether this session carries what renewal needs.
func (s *Session) Renewable() bool {
	return s != nil && s.RefreshToken != ""
}
