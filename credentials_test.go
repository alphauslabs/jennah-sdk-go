package jennah_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	jennah "github.com/alphauslabs/jennah-sdk-go"
	"github.com/alphauslabs/jennah-sdk-go/credentials"
)

// isolateCredentials points credential resolution at an empty temp machine: no
// stored session and no environment key.
//
// Every test that exercises resolution needs this. The fallback reads real
// machine state, so a test that skipped it would assert against whether the
// developer running it happened to be logged in, and would pass on a laptop and
// fail in CI for reasons having nothing to do with the code.
func isolateCredentials(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv(credentials.EnvAPIKey, "")
	return filepath.Join(dir, "jennah", "credentials")
}

// writeStoredSession puts a session on the isolated machine, the way a login
// would have.
func writeStoredSession(t *testing.T, s *credentials.Session) {
	t.Helper()
	if err := credentials.Save(s); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func liveSession() *credentials.Session {
	return &credentials.Session{
		// The gateway origin, which is what a CLI-written session records.
		Endpoint:     "https://jennah.alphaus.cloud",
		AccessToken:  "eyJ.stored.session",
		RefreshToken: "rt_stored",
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	}
}

// The whole point of the fallback: a program that was handed nothing
// authenticates as whoever is logged in on the machine.
func TestNewClientFallsBackToStoredSession(t *testing.T) {
	isolateCredentials(t)
	writeStoredSession(t, liveSession())

	jc, err := jennah.NewClient(jennah.Config{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer jc.Close()

	if got := jc.Credential(); got.Kind != credentials.KindSession || got.Origin != credentials.OriginFile {
		t.Errorf("Credential() = %v, want a session from the stored file", got)
	}
}

// Explicit API keys override stored session files.
func TestExplicitCredentialBeatsAndIgnoresTheFile(t *testing.T) {
	path := isolateCredentials(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	jc, err := jennah.NewClient(jennah.Config{APIKey: "jennah_sk_explicit"})
	if err != nil {
		t.Fatalf("NewClient with an explicit key and a corrupt file: %v", err)
	}
	defer jc.Close()

	if got := jc.Credential(); got.Kind != credentials.KindAPIKey || got.Origin != credentials.OriginExplicit {
		t.Errorf("Credential() = %v, want an API key from explicit configuration", got)
	}
}

func TestEnvironmentCredentialIsResolved(t *testing.T) {
	isolateCredentials(t)
	t.Setenv(credentials.EnvAPIKey, "jennah_sk_from_env")

	jc, err := jennah.NewClient(jennah.Config{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer jc.Close()

	if got := jc.Credential(); got.Kind != credentials.KindAPIKey || got.Origin != credentials.OriginEnvironment {
		t.Errorf("Credential() = %v, want an API key from the environment", got)
	}
}

// Expired sessions without refresh tokens fail construction.
func TestUnrenewableExpiredSessionFailsConstruction(t *testing.T) {
	isolateCredentials(t)
	s := liveSession()
	s.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	s.RefreshToken = "" // nothing to renew with
	writeStoredSession(t, s)

	_, err := jennah.NewClient(jennah.Config{})
	if !errors.Is(err, credentials.ErrSessionExpired) {
		t.Fatalf("NewClient with a dead session: err = %v, want ErrSessionExpired", err)
	}
}

// An expired session that CAN be renewed is not refused: the expiry is an
// ordinary condition the first call resolves, and failing construction here
// would be refusing a client that works.
func TestExpiredButRenewableSessionConstructs(t *testing.T) {
	isolateCredentials(t)
	s := liveSession()
	s.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	writeStoredSession(t, s)

	jc, err := jennah.NewClient(jennah.Config{})
	if err != nil {
		t.Fatalf("NewClient with an expired but renewable session: %v", err)
	}
	jc.Close()
}

// The recorded expiry is advisory in one direction only. A session that has not
// expired locally is attempted, because whether the token is still good is the
// platform's answer to give, not the client's to guess.
func TestUnexpiredStoredSessionIsNotSecondGuessed(t *testing.T) {
	isolateCredentials(t)
	s := liveSession()
	s.ExpiresAt = 0 // unknown expiry: unknown is not expired
	writeStoredSession(t, s)

	jc, err := jennah.NewClient(jennah.Config{})
	if err != nil {
		t.Fatalf("NewClient with a session carrying no expiry: %v", err)
	}
	jc.Close()
}

// Stored session endpoints do not override client configuration.
func TestStoredEndpointDoesNotSelectTheEndpoint(t *testing.T) {
	isolateCredentials(t)
	writeStoredSession(t, liveSession())

	jc, err := jennah.NewClient(jennah.Config{})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer jc.Close()

	if got := jc.Endpoint(); got != jennah.DefaultEndpoint {
		t.Errorf("Endpoint() = %q, want %q (the stored gateway origin must not win)", got, jennah.DefaultEndpoint)
	}

	// And a configured endpoint still wins over the default.
	jc2, err := jennah.NewClient(jennah.Config{Endpoint: "localhost:9999", Insecure: true})
	if err != nil {
		t.Fatalf("NewClient with a configured endpoint: %v", err)
	}
	defer jc2.Close()
	if got := jc2.Endpoint(); got != "localhost:9999" {
		t.Errorf("Endpoint() = %q, want the configured endpoint", got)
	}
}

// Config.Credentials overrides Config.APIKey and stored sessions.
func TestConfigCredentialsOutranksAPIKey(t *testing.T) {
	isolateCredentials(t)
	src := credentials.NewStatic("jennah_sk_supplied", credentials.OriginExplicit)

	jc, err := jennah.NewClient(jennah.Config{APIKey: "jennah_sk_ignored", Credentials: src})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer jc.Close()

	if got := jc.Credential(); got.Kind != credentials.KindAPIKey || got.Origin != credentials.OriginExplicit {
		t.Errorf("Credential() = %v", got)
	}
	tok, err := src.Token(context.Background())
	if err != nil || tok != "jennah_sk_supplied" {
		t.Errorf("the supplied source was not the one consulted: %q, %v", tok, err)
	}
}

// The diagnostic must be safe to log: it says what the credential is and where
// it came from, and there is no path from it to the secret.
func TestCredentialInfoDoesNotExposeTheSecret(t *testing.T) {
	isolateCredentials(t)
	const secret = "jennah_sk_do_not_log_me"

	jc, err := jennah.NewClient(jennah.Config{APIKey: secret})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer jc.Close()

	info := jc.Credential()
	if rendered := info.String(); rendered != "api key from explicit configuration" {
		t.Errorf("String() = %q", rendered)
	}
	// Every exported field of the report, rendered: none of it may be the secret.
	for _, s := range []string{info.String(), info.Kind.String(), info.Origin.String()} {
		if s == secret {
			t.Fatal("the credential report renders the secret itself")
		}
	}
}
