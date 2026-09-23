package jennah_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	jennah "github.com/alphauslabs/jennah-sdk-go"
	"github.com/alphauslabs/jennah-sdk-go/credentials"
	agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"
	authv1 "github.com/alphauslabs/jennah-sdk-go/jennah/auth/v1"
	datastorev1 "github.com/alphauslabs/jennah-sdk-go/jennah/datastore/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// The shared credential conformance suite. The cases live in
// conformance/credentials/cases.json, copied here by jennah-api's CI from the
// same revision as the generated stubs, and are the same cases every other SDK
// runs. This file only translates them onto the Go client: see
// conformance/README.md for what each op and expectation means. A failing case
// is a defect in this SDK, never a reason to edit the case.

const conformanceCases = "conformance/credentials/cases.json"

type conformanceSuite struct {
	Suite   string            `json:"suite"`
	Version int               `json:"version"`
	Source  string            `json:"source"`
	Cases   []conformanceCase `json:"cases"`
}

type conformanceCase struct {
	ID          string   `json:"id"`
	Requirement string   `json:"requirement"`
	Scenario    string   `json:"scenario"`
	Requires    []string `json:"requires"`
	Given       struct {
		Explicit string       `json:"explicit"`
		Env      string       `json:"env"`
		Session  *sessionSpec `json:"session"`
		Server   serverSpec   `json:"server"`
	} `json:"given"`
	Steps []conformanceStep `json:"steps"`
}

// sessionSpec is a stored session as a case writes it, or, under check, the
// fields the stored file must hold.
type sessionSpec struct {
	Raw          *string `json:"raw"`
	Endpoint     *string `json:"endpoint"`
	AccessToken  *string `json:"access_token"`
	RefreshToken *string `json:"refresh_token"`
	TokenType    *string `json:"token_type"`
	ExpiresIn    *int64  `json:"expires_in"`
	Expires      string  `json:"expires"`
}

type serverSpec struct {
	Accept           string       `json:"accept"`
	RejectAll        bool         `json:"reject_all"`
	UnavailableFirst int          `json:"unavailable_first"`
	Refresh          *refreshSpec `json:"refresh"`
	OnRefreshWrite   *sessionSpec `json:"on_refresh_write_session"`
}

type refreshSpec struct {
	Accept       string `json:"accept"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type conformanceStep struct {
	Op       string       `json:"op"`
	Method   string       `json:"method"`
	Endpoint string       `json:"endpoint"`
	Count    int          `json:"count"`
	Writes   int          `json:"writes"`
	Raw      string       `json:"raw"`
	Session  *sessionSpec `json:"session"`
	Expect   *expectSpec  `json:"expect"`
}

type expectSpec struct {
	OK       *bool    `json:"ok"`
	Error    []string `json:"error"`
	Not      []string `json:"not"`
	Mentions []string `json:"mentions"`

	Kind     string `json:"kind"`
	Origin   string `json:"origin"`
	Endpoint string `json:"endpoint"`

	PartialReads *int  `json:"partial_reads"`
	Identical    *bool `json:"identical"`

	Presented      *[]string       `json:"presented"`
	RefreshBearers *[]string       `json:"refresh_bearers"`
	Refreshes      *int            `json:"refreshes"`
	RefreshCalls   *int            `json:"refresh_calls"`
	Session        json.RawMessage `json:"session"`
	SessionMode    string          `json:"session_mode"`
	DirMode        string          `json:"dir_mode"`
	StrayFiles     *int            `json:"stray_files"`
}

func loadConformance(t *testing.T) conformanceSuite {
	t.Helper()
	b, err := os.ReadFile(conformanceCases)
	if err != nil {
		t.Fatalf("read the shared suite: %v", err)
	}
	// Unknown keys fail rather than being skipped: a harness that ignores what it
	// does not understand passes cases it never ran.
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var s conformanceSuite
	if err := dec.Decode(&s); err != nil {
		t.Fatalf("decode the shared suite: %v", err)
	}
	if s.Suite != "client-credentials" || s.Version != 1 {
		t.Fatalf("suite %q version %d: this harness understands client-credentials version 1", s.Suite, s.Version)
	}
	if len(s.Cases) == 0 {
		t.Fatal("the shared suite has no cases")
	}
	return s
}

func TestCredentialConformance(t *testing.T) {
	suite := loadConformance(t)
	seen := map[string]bool{}
	for _, c := range suite.Cases {
		if seen[c.ID] {
			t.Fatalf("duplicate case id %q", c.ID)
		}
		seen[c.ID] = true
		t.Run(c.ID, func(t *testing.T) { runConformanceCase(t, c) })
	}
}

// conformanceRun is one case's machine: its isolated session path, its fake
// platform, and the client under test.
type conformanceRun struct {
	t       *testing.T
	path    string
	secrets []string
	srv     *conformanceServer
	dialer  grpc.DialOption
	client  *jennah.Client
}

func runConformanceCase(t *testing.T, c conformanceCase) {
	for _, r := range c.Requires {
		switch r {
		case "unwritable_directory":
			if runtime.GOOS == "windows" || os.Geteuid() == 0 {
				t.Skip("directory permissions are not enforced for this runner")
			}
		default:
			t.Fatalf("unknown requirement %q", r)
		}
	}

	r := &conformanceRun{t: t, path: isolateCredentials(t)}
	if c.Given.Env != "" {
		t.Setenv(credentials.EnvAPIKey, c.Given.Env)
	}
	r.secrets = collectSecrets(c)
	if c.Given.Session != nil {
		r.writeSession(c.Given.Session)
	}
	r.srv = &conformanceServer{spec: c.Given.Server, write: r.writeSession}
	r.dialer = startConformanceServer(t, r.srv)

	for i, s := range c.Steps {
		r.step(i, c, s)
	}
}

func collectSecrets(c conformanceCase) []string {
	var out []string
	add := func(v string) {
		if v != "" {
			out = append(out, v)
		}
	}
	add(c.Given.Explicit)
	add(c.Given.Env)
	if s := c.Given.Session; s != nil {
		if s.AccessToken != nil {
			add(*s.AccessToken)
		}
		if s.RefreshToken != nil {
			add(*s.RefreshToken)
		}
	}
	return out
}

// writeSession puts a session on disk as another process would: the canonical
// format written directly, not through the SDK under test.
func (r *conformanceRun) writeSession(s *sessionSpec) {
	r.t.Helper()
	var b []byte
	if s.Raw != nil {
		b = []byte(*s.Raw)
	} else {
		var expiresAt int64
		if s.ExpiresIn != nil {
			expiresAt = time.Now().Unix() + *s.ExpiresIn
		}
		var err error
		b, err = json.MarshalIndent(struct {
			Endpoint     string `json:"endpoint"`
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			TokenType    string `json:"token_type"`
			ExpiresAt    int64  `json:"expires_at"`
		}{deref(s.Endpoint), deref(s.AccessToken), deref(s.RefreshToken), deref(s.TokenType), expiresAt}, "", "  ")
		if err != nil {
			r.t.Fatalf("encode session: %v", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		r.t.Fatalf("mkdir: %v", err)
	}
	tmp := r.path + ".harness"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		r.t.Fatalf("write session: %v", err)
	}
	if err := os.Rename(tmp, r.path); err != nil {
		r.t.Fatalf("replace session: %v", err)
	}
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (r *conformanceRun) step(i int, c conformanceCase, s conformanceStep) {
	t := r.t
	t.Helper()
	where := fmt.Sprintf("step %d (%s)", i, s.Op)
	ex := s.Expect
	needExpect := func() {
		if ex == nil {
			t.Fatalf("%s: missing expect", where)
		}
	}

	switch s.Op {
	case "construct":
		needExpect()
		cfg := jennah.Config{APIKey: c.Given.Explicit}
		switch s.Endpoint {
		case "":
			cfg.Endpoint = "passthrough:///bufnet"
			cfg.Insecure = true
			cfg.DialOptions = []grpc.DialOption{r.dialer, grpc.WithTransportCredentials(insecure.NewCredentials())}
		case "default":
		default:
			t.Fatalf("%s: unknown endpoint %q", where, s.Endpoint)
		}
		jc, err := jennah.NewClient(cfg)
		r.expectOutcome(where, ex, err)
		if err == nil {
			r.client = jc
			t.Cleanup(func() { _ = jc.Close() })
		}

	case "describe":
		needExpect()
		r.needClient(where)
		info := r.client.Credential()
		rendered := fmt.Sprintf("%v %+v %#v %s", info, info, info, info.String())
		for _, secret := range r.secrets {
			if strings.Contains(rendered, secret) {
				t.Errorf("%s: the credential report leaks a secret: %q", where, rendered)
			}
		}
		if ex.Kind != "" && kindName(info.Kind) != ex.Kind {
			t.Errorf("%s: kind = %s, want %s", where, kindName(info.Kind), ex.Kind)
		}
		if ex.Origin != "" && originName(info.Origin) != ex.Origin {
			t.Errorf("%s: origin = %s, want %s", where, originName(info.Origin), ex.Origin)
		}
		if ex.Endpoint != "" {
			want := ex.Endpoint
			if want == "$DEFAULT_ENDPOINT" {
				want = jennah.DefaultEndpoint
			}
			if got := r.client.Endpoint(); got != want {
				t.Errorf("%s: endpoint = %q, want %q", where, got, want)
			}
		}

	case "call":
		needExpect()
		r.needClient(where)
		r.expectOutcome(where, ex, r.call(s.Method))

	case "call_concurrently":
		needExpect()
		r.needClient(where)
		if s.Count < 2 {
			t.Fatalf("%s: count %d", where, s.Count)
		}
		r.srv.holdNext(s.Count)
		errs := make([]error, s.Count)
		var wg sync.WaitGroup
		for n := range s.Count {
			wg.Go(func() { errs[n] = r.call(s.Method) })
		}
		wg.Wait()
		for n, err := range errs {
			r.expectOutcome(fmt.Sprintf("%s call %d", where, n), ex, err)
		}

	case "write_session":
		if s.Session == nil {
			t.Fatalf("%s: missing session", where)
		}
		r.writeSession(s.Session)

	case "lock_session_directory":
		if !slices.Contains(c.Requires, "unwritable_directory") {
			t.Fatalf("%s: case does not declare requires unwritable_directory", where)
		}
		dir := filepath.Dir(r.path)
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatalf("%s: %v", where, err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	case "atomic_replace":
		needExpect()
		if ex.PartialReads == nil {
			t.Fatalf("%s: expect needs partial_reads", where)
		}
		if got := r.atomicReplace(s.Writes); got != *ex.PartialReads {
			t.Errorf("%s: partial reads = %d, want %d", where, got, *ex.PartialReads)
		}

	case "round_trip":
		needExpect()
		r.writeSession(&sessionSpec{Raw: &s.Raw})
		sess, err := credentials.Load()
		if err != nil {
			t.Fatalf("%s: load: %v", where, err)
		}
		if err := credentials.Save(sess); err != nil {
			t.Fatalf("%s: save: %v", where, err)
		}
		b, err := os.ReadFile(r.path)
		if err != nil {
			t.Fatalf("%s: read back: %v", where, err)
		}
		if identical := string(b) == s.Raw; ex.Identical == nil || identical != *ex.Identical {
			t.Errorf("%s: identical = %v\n got: %q\nwant: %q", where, identical, b, s.Raw)
		}

	case "check":
		needExpect()
		r.check(where, ex)

	default:
		t.Fatalf("%s: unknown op", where)
	}
}

func (r *conformanceRun) needClient(where string) {
	if r.client == nil {
		r.t.Fatalf("%s: no client was constructed", where)
	}
}

func (r *conformanceRun) call(method string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	switch method {
	case "read":
		_, err := r.client.List(ctx, jennah.ListInput{})
		return err
	case "write_unsafe":
		_, err := r.client.Dataset("ds-conformance").Data.Commit(ctx, &datastorev1.CommitDataRequest{})
		return err
	default:
		r.t.Fatalf("unknown method %q", method)
		return nil
	}
}

// errorCategories maps the suite's error categories onto how the Go client
// reports them.
var errorCategories = map[string]func(error) bool{
	"no_credential": func(err error) bool { return errors.Is(err, credentials.ErrNoCredential) },
	"corrupt_session": func(err error) bool {
		var ce *credentials.CorruptError
		return errors.As(err, &ce)
	},
	"session_expired":    func(err error) bool { return errors.Is(err, credentials.ErrSessionExpired) },
	"credential_refused": func(err error) bool { return errors.Is(err, credentials.ErrKeyRefused) },
	"unauthenticated":    jennah.IsUnauthenticated,
	"unavailable":        func(err error) bool { return jennah.Code(err) == codes.Unavailable },
	"failed":             func(err error) bool { return err != nil },
}

func (r *conformanceRun) expectOutcome(where string, ex *expectSpec, err error) {
	t := r.t
	t.Helper()
	if ex.OK != nil {
		if *ex.OK && err != nil {
			t.Errorf("%s: want success, got %v", where, err)
		}
		return
	}
	if len(ex.Error) == 0 {
		t.Fatalf("%s: expect names neither ok nor error", where)
	}
	if err == nil {
		t.Errorf("%s: want error %v, got success", where, ex.Error)
		return
	}
	for _, cat := range ex.Error {
		holds, ok := errorCategories[cat]
		if !ok {
			t.Fatalf("%s: unknown error category %q", where, cat)
		}
		if !holds(err) {
			t.Errorf("%s: error %q is not %s", where, err, cat)
		}
	}
	for _, cat := range ex.Not {
		holds, ok := errorCategories[cat]
		if !ok {
			t.Fatalf("%s: unknown error category %q", where, cat)
		}
		if holds(err) {
			t.Errorf("%s: error %q must not be %s", where, err, cat)
		}
	}
	for _, m := range ex.Mentions {
		if m == "$SESSION_PATH" {
			m = r.path
		}
		if !strings.Contains(err.Error(), m) {
			t.Errorf("%s: error %q does not mention %q", where, err, m)
		}
	}
}

func (r *conformanceRun) atomicReplace(writes int) int {
	if writes < 1 {
		r.t.Fatalf("atomic_replace: writes %d", writes)
	}
	token := func(n int) string {
		// Lengths vary on purpose, so a torn write cannot parse by accident.
		return fmt.Sprintf("at_%d_%s", n, strings.Repeat("x", n%37))
	}
	written := map[string]bool{}
	for n := range writes {
		written[token(n)] = true
	}

	var partial int
	var mu sync.Mutex
	done := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-done:
					return
				default:
				}
				s, err := credentials.Load()
				if errors.Is(err, credentials.ErrNoSession) {
					continue
				}
				if err != nil || !written[s.AccessToken] {
					mu.Lock()
					partial++
					mu.Unlock()
				}
			}
		})
	}
	for n := range writes {
		if err := credentials.Save(&credentials.Session{
			Endpoint: "https://jennah.alphaus.cloud", AccessToken: token(n),
			RefreshToken: "rt", TokenType: "Bearer",
		}); err != nil {
			r.t.Fatalf("atomic_replace: save %d: %v", n, err)
		}
	}
	close(done)
	wg.Wait()
	return partial
}

func (r *conformanceRun) check(where string, ex *expectSpec) {
	t := r.t
	t.Helper()
	presented, bearers, refreshes, refreshCalls := r.srv.stats()
	if ex.Presented != nil && !slices.Equal(presented, *ex.Presented) {
		t.Errorf("%s: presented = %q, want %q", where, presented, *ex.Presented)
	}
	if ex.RefreshBearers != nil && !slices.Equal(bearers, *ex.RefreshBearers) {
		t.Errorf("%s: refresh bearers = %q, want %q", where, bearers, *ex.RefreshBearers)
	}
	if ex.Refreshes != nil && refreshes != *ex.Refreshes {
		t.Errorf("%s: refreshes = %d, want %d", where, refreshes, *ex.Refreshes)
	}
	if ex.RefreshCalls != nil && refreshCalls != *ex.RefreshCalls {
		t.Errorf("%s: refresh calls = %d, want %d", where, refreshCalls, *ex.RefreshCalls)
	}
	if len(ex.Session) > 0 {
		r.checkSession(where, ex.Session)
	}
	if runtime.GOOS != "windows" {
		if ex.SessionMode != "" {
			r.checkMode(where, r.path, ex.SessionMode)
		}
		if ex.DirMode != "" {
			r.checkMode(where, filepath.Dir(r.path), ex.DirMode)
		}
	}
	if ex.StrayFiles != nil {
		entries, err := os.ReadDir(filepath.Dir(r.path))
		if err != nil {
			t.Fatalf("%s: %v", where, err)
		}
		var stray []string
		for _, e := range entries {
			if e.Name() != filepath.Base(r.path) {
				stray = append(stray, e.Name())
			}
		}
		if len(stray) != *ex.StrayFiles {
			t.Errorf("%s: stray files = %q, want %d", where, stray, *ex.StrayFiles)
		}
	}
}

func (r *conformanceRun) checkSession(where string, raw json.RawMessage) {
	t := r.t
	t.Helper()
	var absent string
	if json.Unmarshal(raw, &absent) == nil {
		if absent != "absent" {
			t.Fatalf("%s: unknown session expectation %q", where, absent)
		}
		if _, err := os.Stat(r.path); !os.IsNotExist(err) {
			t.Errorf("%s: want no stored session, stat = %v", where, err)
		}
		return
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var want sessionSpec
	if err := dec.Decode(&want); err != nil {
		t.Fatalf("%s: session expectation: %v", where, err)
	}
	got, err := credentials.Load()
	if err != nil {
		t.Fatalf("%s: load stored session: %v", where, err)
	}
	for _, f := range []struct {
		name      string
		got, want *string
	}{
		{"endpoint", &got.Endpoint, want.Endpoint},
		{"access_token", &got.AccessToken, want.AccessToken},
		{"refresh_token", &got.RefreshToken, want.RefreshToken},
		{"token_type", &got.TokenType, want.TokenType},
	} {
		if f.want != nil && *f.got != *f.want {
			t.Errorf("%s: stored %s = %q, want %q", where, f.name, *f.got, *f.want)
		}
	}
	switch want.Expires {
	case "":
	case "future":
		if got.ExpiresAt <= time.Now().Unix() {
			t.Errorf("%s: stored expires_at %d is not in the future", where, got.ExpiresAt)
		}
	default:
		t.Fatalf("%s: unknown expires expectation %q", where, want.Expires)
	}
}

func (r *conformanceRun) checkMode(where, path, want string) {
	fi, err := os.Stat(path)
	if err != nil {
		r.t.Fatalf("%s: %v", where, err)
	}
	if got := fmt.Sprintf("%04o", fi.Mode().Perm()); got != want {
		r.t.Errorf("%s: mode of %s = %s, want %s", where, path, got, want)
	}
}

func kindName(k credentials.Kind) string {
	switch k {
	case credentials.KindAPIKey:
		return "api_key"
	case credentials.KindSession:
		return "session"
	}
	return "unknown"
}

func originName(o credentials.Origin) string {
	switch o {
	case credentials.OriginExplicit:
		return "explicit"
	case credentials.OriginEnvironment:
		return "environment"
	case credentials.OriginFile:
		return "file"
	}
	return "unknown"
}

// conformanceServer is the suite's fake platform, configured by a case's
// given.server.
type conformanceServer struct {
	agentv1.UnimplementedAgentServiceServer
	authv1.UnimplementedAuthServiceServer
	datastorev1.UnimplementedDataServiceServer

	spec  serverSpec
	write func(*sessionSpec)

	mu             sync.Mutex
	presented      []string
	refreshBearers []string
	refreshes      int
	refreshCalls   int

	// held counts business attempts still to be held at the barrier, which
	// opens when the last of them arrives.
	held    int
	barrier chan struct{}
}

// holdNext makes the next n business attempts wait until all n have arrived.
func (s *conformanceServer) holdNext(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.held = n
	s.barrier = make(chan struct{})
}

func (s *conformanceServer) waitAtBarrier() {
	s.mu.Lock()
	if s.held == 0 {
		s.mu.Unlock()
		return
	}
	s.held--
	ch := s.barrier
	if s.held == 0 {
		close(ch)
	}
	s.mu.Unlock()
	<-ch
}

func incomingBearer(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	if v := md.Get("authorization"); len(v) > 0 {
		return strings.TrimPrefix(v[0], "Bearer ")
	}
	return ""
}

func (s *conformanceServer) business(ctx context.Context) error {
	s.waitAtBarrier()
	s.mu.Lock()
	defer s.mu.Unlock()
	got := incomingBearer(ctx)
	s.presented = append(s.presented, got)
	if s.spec.UnavailableFirst > 0 {
		s.spec.UnavailableFirst--
		return status.Error(codes.Unavailable, "try again")
	}
	if s.spec.RejectAll || s.spec.Accept == "" || got != s.spec.Accept {
		return status.Error(codes.Unauthenticated, "the access token is invalid or has expired")
	}
	return nil
}

func (s *conformanceServer) ListAgents(ctx context.Context, _ *agentv1.ListAgentsRequest) (*agentv1.ListAgentsResponse, error) {
	if err := s.business(ctx); err != nil {
		return nil, err
	}
	return &agentv1.ListAgentsResponse{}, nil
}

func (s *conformanceServer) CommitData(ctx context.Context, _ *datastorev1.CommitDataRequest) (*datastorev1.CommitDataResponse, error) {
	if err := s.business(ctx); err != nil {
		return nil, err
	}
	return &datastorev1.CommitDataResponse{}, nil
}

func (s *conformanceServer) RefreshToken(ctx context.Context, in *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshCalls++
	s.refreshBearers = append(s.refreshBearers, incomingBearer(ctx))
	if s.spec.OnRefreshWrite != nil {
		s.write(s.spec.OnRefreshWrite)
	}
	rf := s.spec.Refresh
	if rf == nil || in.GetRefreshToken() != rf.Accept {
		return nil, status.Error(codes.Unauthenticated, "the refresh token is invalid or has expired")
	}
	s.refreshes++
	resp := &authv1.RefreshTokenResponse{
		AccessToken:  rf.AccessToken,
		RefreshToken: rf.RefreshToken,
		ExpiresIn:    rf.ExpiresIn,
	}
	// Rotate: the refresh token just presented is dead from here on.
	s.spec.Accept = rf.AccessToken
	s.spec.Refresh = &refreshSpec{Accept: rf.RefreshToken}
	return resp, nil
}

func (s *conformanceServer) stats() (presented, refreshBearers []string, refreshes, refreshCalls int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.presented), slices.Clone(s.refreshBearers), s.refreshes, s.refreshCalls
}

func startConformanceServer(t *testing.T, srv *conformanceServer) grpc.DialOption {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	g := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(g, srv)
	authv1.RegisterAuthServiceServer(g, srv)
	datastorev1.RegisterDataServiceServer(g, srv)
	go func() { _ = g.Serve(lis) }()
	t.Cleanup(g.Stop)
	return grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	})
}
