package jennah_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
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

// authServer rejects every bearer except the one it currently considers valid,
// and mints a new one when asked to refresh. It is the smallest thing that can
// tell a client which renewed correctly from one that merely retried.
type authServer struct {
	agentv1.UnimplementedAgentServiceServer
	authv1.UnimplementedAuthServiceServer
	datastorev1.UnimplementedDataServiceServer

	mu sync.Mutex
	// valid is the access token the server accepts right now.
	valid string
	// acceptedRefresh is the refresh token the server will rotate. A refresh
	// presenting anything else is rejected, which is how a lost rotation looks.
	acceptedRefresh string
	// refreshes counts renewals actually performed.
	refreshes int
	// calls counts business-method attempts, including reissues.
	calls int
	// presented records every bearer the server was shown, in order.
	presented []string
	// refuseRefresh makes renewal fail, as a revoked or rotated-away token would.
	refuseRefresh bool
	// unavailableFirst fails the first N business attempts with UNAVAILABLE, to
	// exercise the transport retry sitting outside the credential interceptor.
	unavailableFirst int
	// alwaysReject refuses every access token, even a freshly minted one.
	alwaysReject bool
}

func (s *authServer) bearer(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	if v := md.Get("authorization"); len(v) > 0 {
		return strings.TrimPrefix(v[0], "Bearer ")
	}
	return ""
}

// check is the server's whole authentication chain: it decides before the
// operation runs, which is exactly why reissuing a rejected call is safe.
func (s *authServer) check(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	got := s.bearer(ctx)
	s.presented = append(s.presented, got)
	if s.unavailableFirst > 0 {
		s.unavailableFirst--
		return status.Error(codes.Unavailable, "try again")
	}
	if s.alwaysReject || got != s.valid {
		return status.Error(codes.Unauthenticated, "the access token is invalid or has expired")
	}
	return nil
}

func (s *authServer) ListAgents(ctx context.Context, _ *agentv1.ListAgentsRequest) (*agentv1.ListAgentsResponse, error) {
	if err := s.check(ctx); err != nil {
		return nil, err
	}
	return &agentv1.ListAgentsResponse{}, nil
}

// CommitData is the deliberately non-idempotent case: no idempotency key means
// the transport retry would refuse to replay it, and the auth retry must anyway.
func (s *authServer) CommitData(ctx context.Context, _ *datastorev1.CommitDataRequest) (*datastorev1.CommitDataResponse, error) {
	if err := s.check(ctx); err != nil {
		return nil, err
	}
	return &datastorev1.CommitDataResponse{}, nil
}

func (s *authServer) RefreshToken(_ context.Context, in *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refuseRefresh || in.GetRefreshToken() != s.acceptedRefresh {
		return nil, status.Error(codes.Unauthenticated, "the refresh token is invalid or has expired")
	}
	s.refreshes++
	// Rotate, exactly as the platform does: the presented token is dead now.
	s.valid = "at_renewed"
	s.acceptedRefresh = "rt_rotated"
	return &authv1.RefreshTokenResponse{
		AccessToken:  s.valid,
		RefreshToken: s.acceptedRefresh,
		ExpiresIn:    3600,
	}, nil
}

func (s *authServer) stats() (calls, refreshes int, presented []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls, s.refreshes, append([]string(nil), s.presented...)
}

// startAuthServer runs srv on an in-process listener and returns a dialer for it.
func startAuthServer(t *testing.T, srv *authServer) grpc.DialOption {
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

// storedClient writes a session and builds a client that resolves it, so the
// credential travels the real path: file, resolution, interceptor.
func storedClient(t *testing.T, dialer grpc.DialOption, s *credentials.Session) *jennah.Client {
	t.Helper()
	writeStoredSession(t, s)
	jc, err := jennah.NewClient(jennah.Config{
		Endpoint: "passthrough:///bufnet",
		Insecure: true,
		DialOptions: []grpc.DialOption{
			dialer,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = jc.Close() })
	return jc
}

func expiredSession() *credentials.Session {
	return &credentials.Session{
		Endpoint:     "https://jennah.alphaus.cloud",
		AccessToken:  "at_expired",
		RefreshToken: "rt_valid",
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().Add(-time.Minute).Unix(),
	}
}

// The headline case: a long-lived client's session expires, and the caller sees
// a success without ever having handled the expiry.
func TestExpiredSessionRenewsMidFlight(t *testing.T) {
	isolateCredentials(t)
	srv := &authServer{valid: "at_current", acceptedRefresh: "rt_valid"}
	jc := storedClient(t, startAuthServer(t, srv), expiredSession())

	if _, err := jc.List(context.Background(), jennah.ListInput{}); err != nil {
		t.Fatalf("List: %v", err)
	}

	calls, refreshes, presented := srv.stats()
	if refreshes != 1 {
		t.Errorf("refreshes = %d, want 1", refreshes)
	}
	if calls != 2 {
		t.Errorf("business attempts = %d, want 2 (the rejection and the reissue)", calls)
	}
	if len(presented) != 2 || presented[0] != "at_expired" || presented[1] != "at_renewed" {
		t.Errorf("presented = %v, want the expired token then the renewed one", presented)
	}
}

// A write with no idempotency key is exactly what the transport retry refuses to
// replay. The auth retry must reissue it anyway: the call was refused before it
// reached the operation, so there is nothing to have applied twice.
func TestNonReplayableWriteStillRenewsAndRetries(t *testing.T) {
	isolateCredentials(t)
	srv := &authServer{valid: "at_current", acceptedRefresh: "rt_valid"}
	jc := storedClient(t, startAuthServer(t, srv), expiredSession())

	ds := jc.Dataset("ds-1")
	_, err := ds.Data.Commit(context.Background(), &datastorev1.CommitDataRequest{})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, refreshes, _ := srv.stats(); refreshes != 1 {
		t.Errorf("refreshes = %d, want 1", refreshes)
	}
}

// The renewed session must outlive the process that renewed it. The token just
// spent is dead, so a renewal kept in memory strands every other reader of the
// file, the CLI included.
func TestRenewalIsWrittenBackToTheSharedFile(t *testing.T) {
	isolateCredentials(t)
	srv := &authServer{valid: "at_current", acceptedRefresh: "rt_valid"}
	jc := storedClient(t, startAuthServer(t, srv), expiredSession())

	if _, err := jc.List(context.Background(), jennah.ListInput{}); err != nil {
		t.Fatalf("List: %v", err)
	}

	stored, err := credentials.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if stored.AccessToken != "at_renewed" {
		t.Errorf("stored access token = %q, want the renewed one", stored.AccessToken)
	}
	if stored.RefreshToken != "rt_rotated" {
		t.Errorf("stored refresh token = %q, want the rotated one; the old one is dead", stored.RefreshToken)
	}
	if stored.Endpoint != "https://jennah.alphaus.cloud" {
		t.Errorf("the renewal clobbered an unrelated field: endpoint = %q", stored.Endpoint)
	}
	if stored.ExpiresAt <= time.Now().Unix() {
		t.Errorf("expiry was not advanced: %d", stored.ExpiresAt)
	}
}

// Concurrent calls that all expire at once must renew once. A rotation per call
// would leave all but one of them holding a token the rotation invalidated.
func TestConcurrentRejectionsRenewOnce(t *testing.T) {
	isolateCredentials(t)
	srv := &authServer{valid: "at_current", acceptedRefresh: "rt_valid"}
	jc := storedClient(t, startAuthServer(t, srv), expiredSession())

	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = jc.List(context.Background(), jennah.ListInput{})
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("call %d: %v", i, err)
		}
	}
	if _, refreshes, _ := srv.stats(); refreshes != 1 {
		t.Errorf("refreshes = %d, want 1 for %d concurrent rejections", refreshes, n)
	}
}

// An API key cannot be renewed, so a rejection is the key being refused. Saying
// "session expired" would send the holder to a login that would not help.
func TestRejectedAPIKeyIsNotRenewed(t *testing.T) {
	isolateCredentials(t)
	srv := &authServer{valid: "at_current", acceptedRefresh: "rt_valid", alwaysReject: true}
	dialer := startAuthServer(t, srv)

	jc, err := jennah.NewClient(jennah.Config{
		Endpoint:    "passthrough:///bufnet",
		APIKey:      "jennah_sk_wrong",
		Insecure:    true,
		DialOptions: []grpc.DialOption{dialer, grpc.WithTransportCredentials(insecure.NewCredentials())},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	defer jc.Close()

	_, err = jc.List(context.Background(), jennah.ListInput{})
	if !errors.Is(err, credentials.ErrKeyRefused) {
		t.Errorf("err = %v, want ErrKeyRefused", err)
	}
	if errors.Is(err, credentials.ErrSessionExpired) {
		t.Error("a refused key reads as an expired session")
	}
	// The gRPC status has to survive the wrapping, or every caller branching on
	// IsUnauthenticated silently stops working.
	if !jennah.IsUnauthenticated(err) {
		t.Errorf("IsUnauthenticated = false for %v", err)
	}
	if _, refreshes, _ := srv.stats(); refreshes != 0 {
		t.Errorf("refreshes = %d, want 0: a key has no refresh token to spend", refreshes)
	}
}

// A reissue that is rejected again is the platform's answer, not an invitation
// to renew in a loop.
func TestSecondRejectionStops(t *testing.T) {
	isolateCredentials(t)
	srv := &authServer{valid: "at_current", acceptedRefresh: "rt_valid", alwaysReject: true}
	jc := storedClient(t, startAuthServer(t, srv), expiredSession())

	_, err := jc.List(context.Background(), jennah.ListInput{})
	if err == nil {
		t.Fatal("expected the second rejection to be returned")
	}
	if !jennah.IsUnauthenticated(err) {
		t.Errorf("err = %v, want an Unauthenticated status", err)
	}
	calls, refreshes, _ := srv.stats()
	if refreshes != 1 {
		t.Errorf("refreshes = %d, want exactly 1", refreshes)
	}
	if calls != 2 {
		t.Errorf("business attempts = %d, want 2: one renewal, one reissue, then stop", calls)
	}
}

// A renewal that fails because another process rotated the token first must not
// be reported as a dead session: the file now holds a working one, and adopting
// it is the whole recovery.
func TestFailedRenewalAdoptsAnotherProcessesSession(t *testing.T) {
	isolateCredentials(t)
	srv := &authServer{valid: "at_written_by_other", acceptedRefresh: "rt_unreachable", refuseRefresh: true}
	jc := storedClient(t, startAuthServer(t, srv), expiredSession())

	// Another process renews and writes back between our read and our call.
	writeStoredSession(t, &credentials.Session{
		Endpoint:     "https://jennah.alphaus.cloud",
		AccessToken:  "at_written_by_other",
		RefreshToken: "rt_written_by_other",
		TokenType:    "Bearer",
		ExpiresAt:    time.Now().Add(time.Hour).Unix(),
	})

	if _, err := jc.List(context.Background(), jennah.ListInput{}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if _, refreshes, presented := srv.stats(); refreshes != 0 || presented[len(presented)-1] != "at_written_by_other" {
		t.Errorf("refreshes = %d, presented = %v; want the other process's token adopted with no rotation spent",
			refreshes, presented)
	}
}

// A renewal that cannot be persisted is reported, not quietly used. Proceeding
// would spend the rotation and then lose it when the process exits, leaving the
// file holding a refresh token the platform has already invalidated, the exact
// state that costs a user their session.
func TestUnpersistableRenewalIsSurfaced(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: an unwritable directory is still writable")
	}
	path := isolateCredentials(t)
	srv := &authServer{valid: "at_current", acceptedRefresh: "rt_valid"}
	jc := storedClient(t, startAuthServer(t, srv), expiredSession())

	// Take away the ability to replace the file, the way a read-only mount or a
	// directory owned by someone else would.
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	_, err := jc.List(context.Background(), jennah.ListInput{})
	if err == nil {
		t.Fatal("a renewal that could not be written back was reported as success")
	}
	if !strings.Contains(err.Error(), "session") && !strings.Contains(err.Error(), "permission") {
		t.Errorf("the error does not explain what failed: %v", err)
	}
	// The failure has to be the write, not something earlier: the rotation was
	// spent, which is exactly why losing it silently would matter.
	if _, refreshes, _ := srv.stats(); refreshes != 1 {
		t.Errorf("refreshes = %d, want 1; the test is not exercising the write path", refreshes)
	}
}

// A session with nothing to renew with fails as an expired session, naming the
// login that fixes it.
func TestUnrenewableSessionReportsExpiry(t *testing.T) {
	isolateCredentials(t)
	srv := &authServer{valid: "at_current", acceptedRefresh: "rt_valid"}
	s := expiredSession()
	s.RefreshToken = ""
	s.ExpiresAt = time.Now().Add(time.Hour).Unix() // passes construction, fails on use
	jc := storedClient(t, startAuthServer(t, srv), s)

	_, err := jc.List(context.Background(), jennah.ListInput{})
	if !errors.Is(err, credentials.ErrSessionExpired) {
		t.Errorf("err = %v, want ErrSessionExpired", err)
	}
	if !strings.Contains(err.Error(), "jnh login") {
		t.Errorf("the error does not name the way out: %v", err)
	}
}

// The transport retry sits outside the credential interceptor, so a replay after
// a connection failure re-resolves the credential rather than reusing the one
// the failed attempt carried. Without that ordering, a replay would present a
// credential renewed in the meantime, or worse, one captured at dial.
func TestTransportRetryPresentsTheCurrentCredential(t *testing.T) {
	isolateCredentials(t)
	srv := &authServer{valid: "at_current", acceptedRefresh: "rt_valid", unavailableFirst: 1}
	jc := storedClient(t, startAuthServer(t, srv), expiredSession())

	if _, err := jc.List(context.Background(), jennah.ListInput{}); err != nil {
		t.Fatalf("List: %v", err)
	}

	_, refreshes, presented := srv.stats()
	if refreshes != 1 {
		t.Errorf("refreshes = %d, want 1", refreshes)
	}
	// UNAVAILABLE, then the replay is rejected, then the renewed reissue lands.
	if len(presented) != 3 || presented[len(presented)-1] != "at_renewed" {
		t.Errorf("presented = %v, want the last attempt to carry the renewed credential", presented)
	}
}
