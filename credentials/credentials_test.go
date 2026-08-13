package credentials_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alphauslabs/jennah-sdk-go/credentials"
)

// cliWritten is a stored session exactly as the command-line client writes one:
// two-space indent, this field order, no trailing newline. It is the format
// contract between the two clients, so it is pinned here as a literal rather
// than produced by the code under test — a fixture the implementation generated
// would agree with itself no matter what it did.
const cliWritten = `{
  "endpoint": "https://jennah.alphaus.cloud",
  "access_token": "eyJhbGciOiJIUzI1NiJ9.header.signature",
  "refresh_token": "rt_opaque_value",
  "token_type": "Bearer",
  "expires_at": 1786000000
}`

// isolate points the config directory at a temp dir for one test, so nothing
// touches the developer's real session.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Resolve through the package so the test asserts against the same path the
	// code writes, rather than re-deriving the layout.
	path, err := credentials.Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if want := filepath.Join(dir, "jennah", "credentials"); path != want {
		t.Fatalf("Path = %q, want %q", path, want)
	}
	return path
}

func writeSession(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// A session written by the command-line client must load with every field
// intact, and re-saving it must reproduce the same bytes. Anything less means
// the two implementations drift the first time either one writes.
func TestLoadCLIWrittenSessionRoundTrips(t *testing.T) {
	path := isolate(t)
	writeSession(t, path, cliWritten)

	got, err := credentials.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Endpoint != "https://jennah.alphaus.cloud" {
		t.Errorf("Endpoint = %q", got.Endpoint)
	}
	if got.AccessToken != "eyJhbGciOiJIUzI1NiJ9.header.signature" {
		t.Errorf("AccessToken = %q", got.AccessToken)
	}
	if got.RefreshToken != "rt_opaque_value" {
		t.Errorf("RefreshToken = %q", got.RefreshToken)
	}
	if got.TokenType != "Bearer" {
		t.Errorf("TokenType = %q", got.TokenType)
	}
	if got.ExpiresAt != 1786000000 {
		t.Errorf("ExpiresAt = %d", got.ExpiresAt)
	}

	if err := credentials.Save(got); err != nil {
		t.Fatalf("Save: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(b) != cliWritten {
		t.Errorf("re-saved bytes differ from what the CLI writes:\n got: %q\nwant: %q", b, cliWritten)
	}
}

func TestSaveIsOwnerOnlyAndAtomic(t *testing.T) {
	path := isolate(t)
	in := &credentials.Session{AccessToken: "at", RefreshToken: "rt", TokenType: "Bearer", ExpiresAt: 42}
	if err := credentials.Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("session mode = %04o, want 0600", got)
	}
	if di, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("stat dir: %v", err)
	} else if got := di.Mode().Perm(); got != 0o700 {
		t.Errorf("config dir mode = %04o, want 0700", got)
	}

	// The write must leave nothing behind: a stray temp file next to the session
	// is a copy of the credential with no one responsible for removing it.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "credentials" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("config dir holds %v, want only the session file", names)
	}

	out, err := credentials.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if *out != *in {
		t.Errorf("round trip: got %+v, want %+v", *out, *in)
	}
}

// A reader concurrent with a write must see one whole session or the other,
// never a partial file. The rename is what guarantees it; this hammers the two
// against each other so that a future implementation writing in place would be
// caught rather than merely reviewed.
func TestConcurrentReadsNeverSeeAPartialWrite(t *testing.T) {
	isolate(t)
	first := &credentials.Session{AccessToken: "at_one", RefreshToken: "rt_one", TokenType: "Bearer"}
	second := &credentials.Session{AccessToken: "at_two_which_is_much_longer_than_the_first", RefreshToken: "rt_two", TokenType: "Bearer"}
	if err := credentials.Save(first); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			s := first
			if i%2 == 0 {
				s = second
			}
			if err := credentials.Save(s); err != nil {
				t.Errorf("Save: %v", err)
				break
			}
		}
		close(stop)
	}()

	for {
		select {
		case <-stop:
			wg.Wait()
			return
		default:
		}
		got, err := credentials.Load()
		if err != nil {
			t.Fatalf("Load observed a write in progress: %v", err)
		}
		if got.AccessToken != first.AccessToken && got.AccessToken != second.AccessToken {
			t.Fatalf("Load observed a torn session: %+v", *got)
		}
	}
}

func TestSaveReplacesRatherThanMerges(t *testing.T) {
	path := isolate(t)
	writeSession(t, path, cliWritten)
	if err := credentials.Save(&credentials.Session{AccessToken: "only"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := credentials.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.RefreshToken != "" || got.Endpoint != "" {
		t.Errorf("previous session survived the replacement: %+v", *got)
	}
}

func TestLoadMissingIsNoSession(t *testing.T) {
	isolate(t)
	_, err := credentials.Load()
	if !errors.Is(err, credentials.ErrNoSession) {
		t.Fatalf("Load on empty machine: err = %v, want ErrNoSession", err)
	}
}

// A file that exists but will not parse must be reported, and reported
// distinctly: reading it as "not logged in" would send the holder looking for a
// login problem they do not have.
func TestLoadCorruptNamesThePath(t *testing.T) {
	path := isolate(t)
	writeSession(t, path, "{not json")

	_, err := credentials.Load()
	var corrupt *credentials.CorruptError
	if !errors.As(err, &corrupt) {
		t.Fatalf("Load on corrupt file: err = %v, want *CorruptError", err)
	}
	if corrupt.Path != path {
		t.Errorf("CorruptError.Path = %q, want %q", corrupt.Path, path)
	}
	if errors.Is(err, credentials.ErrNoSession) {
		t.Error("a corrupt session reads as ErrNoSession, so a caller cannot tell it from an empty machine")
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	path := isolate(t)
	writeSession(t, path, cliWritten)
	if err := credentials.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session still present after Delete: %v", err)
	}
	if err := credentials.Delete(); err != nil {
		t.Fatalf("Delete on an absent session: %v", err)
	}
}

func TestExpiredIsAdvisoryAndUnsetMeansUnknown(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	cases := []struct {
		name    string
		expires int64
		want    bool
	}{
		{"past", now.Add(-time.Second).Unix(), true},
		{"future", now.Add(time.Hour).Unix(), false},
		{"unset means unknown, not expired", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &credentials.Session{ExpiresAt: tc.expires}
			if got := s.Expired(now); got != tc.want {
				t.Errorf("Expired = %v, want %v", got, tc.want)
			}
		})
	}
}

// Precedence across every combination of sources. The point of each row is not
// only which credential wins but that the losing sources are irrelevant: a row
// with a broken file and a working key must succeed.
func TestResolvePrecedence(t *testing.T) {
	const (
		explicitKey = "jennah_sk_explicit"
		envKey      = "jennah_sk_from_env"
	)

	cases := []struct {
		name       string
		explicit   string
		env        string
		file       string // "" writes no file
		wantCred   string
		wantKind   credentials.Kind
		wantOrigin credentials.Origin
	}{
		{
			name: "explicit beats everything", explicit: explicitKey, env: envKey, file: cliWritten,
			wantCred: explicitKey, wantKind: credentials.KindAPIKey, wantOrigin: credentials.OriginExplicit,
		},
		{
			name: "explicit needs no file and no env", explicit: explicitKey,
			wantCred: explicitKey, wantKind: credentials.KindAPIKey, wantOrigin: credentials.OriginExplicit,
		},
		{
			name: "explicit survives a corrupt file", explicit: explicitKey, file: "{not json",
			wantCred: explicitKey, wantKind: credentials.KindAPIKey, wantOrigin: credentials.OriginExplicit,
		},
		{
			name: "env beats the file", env: envKey, file: cliWritten,
			wantCred: envKey, wantKind: credentials.KindAPIKey, wantOrigin: credentials.OriginEnvironment,
		},
		{
			name: "env survives a corrupt file", env: envKey, file: "{not json",
			wantCred: envKey, wantKind: credentials.KindAPIKey, wantOrigin: credentials.OriginEnvironment,
		},
		{
			name: "the file answers when nothing else does", file: cliWritten,
			wantCred:   "eyJhbGciOiJIUzI1NiJ9.header.signature",
			wantKind:   credentials.KindSession,
			wantOrigin: credentials.OriginFile,
		},
		{
			name: "whitespace is not a credential", explicit: "   ", env: envKey, file: cliWritten,
			wantCred: envKey, wantKind: credentials.KindAPIKey, wantOrigin: credentials.OriginEnvironment,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := isolate(t)
			t.Setenv(credentials.EnvAPIKey, tc.env)
			if tc.file != "" {
				writeSession(t, path, tc.file)
			}

			got, err := credentials.Resolve(tc.explicit)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if got.Credential != tc.wantCred {
				t.Errorf("Credential = %q, want %q", got.Credential, tc.wantCred)
			}
			if got.Kind != tc.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind, tc.wantKind)
			}
			if got.Origin != tc.wantOrigin {
				t.Errorf("Origin = %v, want %v", got.Origin, tc.wantOrigin)
			}
			// The session comes along only when the file is what answered, so a
			// caller can renew without reading the file a second time.
			if (got.Session != nil) != (tc.wantOrigin == credentials.OriginFile) {
				t.Errorf("Session attached = %v, want %v", got.Session != nil, tc.wantOrigin == credentials.OriginFile)
			}
		})
	}
}

func TestResolveNoCredentialAnywhere(t *testing.T) {
	isolate(t)
	t.Setenv(credentials.EnvAPIKey, "")

	_, err := credentials.Resolve("")
	if !errors.Is(err, credentials.ErrNoCredential) {
		t.Fatalf("Resolve on an empty machine: err = %v, want ErrNoCredential", err)
	}
	// The error has to be actionable: a caller seeing it has not yet chosen
	// between the ways of supplying a credential.
	for _, want := range []string{credentials.EnvAPIKey, "jnh login"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("ErrNoCredential does not mention %q: %v", want, err)
		}
	}
}

// A corrupt file is the last source, so nothing else is going to answer and it
// must surface as itself rather than as "no credential".
func TestResolveReportsCorruptFile(t *testing.T) {
	path := isolate(t)
	t.Setenv(credentials.EnvAPIKey, "")
	writeSession(t, path, "{not json")

	_, err := credentials.Resolve("")
	var corrupt *credentials.CorruptError
	if !errors.As(err, &corrupt) {
		t.Fatalf("Resolve with a corrupt file: err = %v, want *CorruptError", err)
	}
}

func TestResolveEmptyAccessTokenIsNoCredential(t *testing.T) {
	path := isolate(t)
	t.Setenv(credentials.EnvAPIKey, "")
	writeSession(t, path, `{"endpoint":"x","access_token":"","refresh_token":"rt"}`)

	_, err := credentials.Resolve("")
	if !errors.Is(err, credentials.ErrNoCredential) {
		t.Fatalf("Resolve with a tokenless session: err = %v, want ErrNoCredential", err)
	}
}

func TestStaticSource(t *testing.T) {
	key := credentials.NewStatic("jennah_sk_abc", credentials.OriginEnvironment)
	if key.Kind() != credentials.KindAPIKey {
		t.Errorf("Kind = %v, want KindAPIKey", key.Kind())
	}
	if key.Origin() != credentials.OriginEnvironment {
		t.Errorf("Origin = %v, want OriginEnvironment", key.Origin())
	}
	if key.Renewable() {
		t.Error("an API key reports itself renewable, which would send a caller looking for a refresh token it does not have")
	}
	if tok, err := key.Token(t.Context()); err != nil || tok != "jennah_sk_abc" {
		t.Errorf("Token = %q, %v", tok, err)
	}
	// Renewing a key must name the key as refused, not report an expired session.
	if _, err := key.Renew(t.Context(), "jennah_sk_abc"); !errors.Is(err, credentials.ErrKeyRefused) {
		t.Errorf("Renew on a key: err = %v, want ErrKeyRefused", err)
	}

	sess := credentials.NewStatic("eyJ.a.b", credentials.OriginFile)
	if sess.Kind() != credentials.KindSession {
		t.Errorf("Kind = %v, want KindSession", sess.Kind())
	}
	if _, err := sess.Renew(t.Context(), "eyJ.a.b"); !errors.Is(err, credentials.ErrSessionExpired) {
		t.Errorf("Renew on a non-renewing session: err = %v, want ErrSessionExpired", err)
	}
}
