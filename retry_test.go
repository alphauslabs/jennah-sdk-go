package jennah_test

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	jennah "github.com/alphauslabs/jennah-sdk-go"
	agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"
	approvalv1 "github.com/alphauslabs/jennah-sdk-go/jennah/approval/v1"
	authv1 "github.com/alphauslabs/jennah-sdk-go/jennah/auth/v1"
	datastorev1 "github.com/alphauslabs/jennah-sdk-go/jennah/datastore/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// flakyServer fails the first `failures` calls to each method with UNAVAILABLE and
// then succeeds, so a test can count how many attempts the SDK actually made. When
// `always` is set it fails every attempt with that code instead.
type flakyServer struct {
	agentv1.UnimplementedAgentServiceServer
	agentv1.UnimplementedMemoryServiceServer
	datastorev1.UnimplementedDataServiceServer
	approvalv1.UnimplementedApprovalServiceServer
	authv1.UnimplementedAuthServiceServer

	mu       sync.Mutex
	failures int
	always   codes.Code
	calls    map[string]int // attempts seen per method
	auths    map[string]int // attempts that carried the bearer
}

func (f *flakyServer) attempt(ctx context.Context, method string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[method]++
	if hasTestBearer(ctx) {
		f.auths[method]++
	}
	if f.always != codes.OK {
		return status.Error(f.always, "refused")
	}
	if f.calls[method] <= f.failures {
		return status.Error(codes.Unavailable, "try again")
	}
	return nil
}

func (f *flakyServer) count(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[method]
}

func (f *flakyServer) credentialed(method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.auths[method]
}

func hasTestBearer(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}
	v := md.Get("authorization")
	return len(v) > 0 && v[0] == "Bearer jennah_sk_testkey"
}

func (f *flakyServer) ListAgents(ctx context.Context, _ *agentv1.ListAgentsRequest) (*agentv1.ListAgentsResponse, error) {
	if err := f.attempt(ctx, "ListAgents"); err != nil {
		return nil, err
	}
	return &agentv1.ListAgentsResponse{}, nil
}

func (f *flakyServer) CreateAgent(ctx context.Context, _ *agentv1.CreateAgentRequest) (*agentv1.CreateAgentResponse, error) {
	if err := f.attempt(ctx, "CreateAgent"); err != nil {
		return nil, err
	}
	return &agentv1.CreateAgentResponse{Agent: &agentv1.AgentInstance{}}, nil
}

func (f *flakyServer) CommitMemory(ctx context.Context, _ *agentv1.CommitMemoryRequest) (*agentv1.CommitMemoryResponse, error) {
	if err := f.attempt(ctx, "CommitMemory"); err != nil {
		return nil, err
	}
	return &agentv1.CommitMemoryResponse{}, nil
}

func (f *flakyServer) CommitData(ctx context.Context, _ *datastorev1.CommitDataRequest) (*datastorev1.CommitDataResponse, error) {
	if err := f.attempt(ctx, "CommitData"); err != nil {
		return nil, err
	}
	return &datastorev1.CommitDataResponse{}, nil
}

func (f *flakyServer) CreateApproval(ctx context.Context, _ *approvalv1.CreateApprovalRequest) (*approvalv1.CreateApprovalResponse, error) {
	if err := f.attempt(ctx, "CreateApproval"); err != nil {
		return nil, err
	}
	return &approvalv1.CreateApprovalResponse{}, nil
}

func (f *flakyServer) InviteMember(ctx context.Context, _ *authv1.InviteMemberRequest) (*authv1.InviteMemberResponse, error) {
	if err := f.attempt(ctx, "InviteMember"); err != nil {
		return nil, err
	}
	return &authv1.InviteMemberResponse{}, nil
}

func newFlakyClient(t *testing.T, srvFake *flakyServer, policy jennah.RetryPolicy) *jennah.Client {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	agentv1.RegisterAgentServiceServer(srv, srvFake)
	agentv1.RegisterMemoryServiceServer(srv, srvFake)
	datastorev1.RegisterDataServiceServer(srv, srvFake)
	approvalv1.RegisterApprovalServiceServer(srv, srvFake)
	authv1.RegisterAuthServiceServer(srv, srvFake)
	go func() { _ = srv.Serve(lis) }()

	jc, err := jennah.NewClient(jennah.Config{
		Endpoint: "passthrough:///bufnet",
		APIKey:   "jennah_sk_testkey",
		Insecure: true,
		Retry:    policy,
		DialOptions: []grpc.DialOption{
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return lis.DialContext(ctx)
			}),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() {
		_ = jc.Close()
		srv.Stop()
	})
	return jc
}

func flaky(failures int) *flakyServer {
	return &flakyServer{failures: failures, calls: map[string]int{}, auths: map[string]int{}}
}

// fastRetry keeps the tests quick without changing which calls are eligible.
var fastRetry = jennah.RetryPolicy{MaxAttempts: 3, BaseBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}

// A read is always replayable, so two transient failures must be absorbed, and
// every attempt must carry the credential: the retry interceptor wraps the bearer
// one, not the other way round.
func TestRetryAbsorbsTransientFailureOnReads(t *testing.T) {
	fake := flaky(2)
	jc := newFlakyClient(t, fake, fastRetry)

	if _, err := jc.List(context.Background(), jennah.ListInput{}); err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := fake.count("ListAgents"); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
	if got := fake.credentialed("ListAgents"); got != 3 {
		t.Errorf("credentialed attempts = %d, want 3", got)
	}
}

// A write with no idempotency token must be attempted exactly once, because a
// replay could create a second resource or send a second mail.
func TestRetrySkipsUnsafeWrites(t *testing.T) {
	fake := flaky(1)
	jc := newFlakyClient(t, fake, fastRetry)
	ctx := context.Background()

	if _, err := jc.Spawn(ctx, jennah.SpawnInput{AgentInstanceID: "a1"}); !jennah.IsTransient(err) {
		t.Fatalf("Spawn error = %v, want the transient failure surfaced", err)
	}
	if got := fake.count("CreateAgent"); got != 1 {
		t.Errorf("CreateAgent attempts = %d, want 1", got)
	}

	if _, err := jc.Auth.Invitations.Create(ctx, &jennah.InviteMemberRequest{Email: "a@b.c"}); err == nil {
		t.Error("InviteMember unexpectedly succeeded")
	}
	if got := fake.count("InviteMember"); got != 1 {
		t.Errorf("InviteMember attempts = %d, want 1", got)
	}
}

// The three conditional writes are eligible only when the request carries what
// makes a replay safe. This is why retry lives here rather than in a generic
// service-config policy: that policy sees the method, never the request.
func TestRetryDependsOnTheRequestForConditionalWrites(t *testing.T) {
	ctx := context.Background()

	t.Run("memory commit carrying a log step is not replayed", func(t *testing.T) {
		fake := flaky(1)
		jc := newFlakyClient(t, fake, fastRetry)
		_, err := jc.Agent("a1").Memory.Commit(ctx, jennah.CommitInput{
			Log:     &jennah.ExecutionLogStep{StepId: "s1"},
			Vectors: []*jennah.VectorChunk{{ChunkId: "c1"}},
		})
		if err == nil {
			t.Fatal("commit unexpectedly succeeded")
		}
		if got := fake.count("CommitMemory"); got != 1 {
			t.Errorf("attempts = %d, want 1: a log step is append-only", got)
		}
	})

	t.Run("memory commit without a log step is replayed", func(t *testing.T) {
		fake := flaky(1)
		jc := newFlakyClient(t, fake, fastRetry)
		if _, err := jc.Agent("a1").Vectors.Upsert(ctx, &jennah.VectorChunk{ChunkId: "c1"}); err != nil {
			t.Fatalf("Upsert: %v", err)
		}
		if got := fake.count("CommitMemory"); got != 2 {
			t.Errorf("attempts = %d, want 2: vector writes are idempotent upserts", got)
		}
	})

	t.Run("data commit is replayed only with an idempotency key", func(t *testing.T) {
		bare := flaky(1)
		if _, err := newFlakyClient(t, bare, fastRetry).Dataset("ds1").
			Data.Commit(ctx, &jennah.CommitDataRequest{}); err == nil {
			t.Fatal("commit unexpectedly succeeded")
		}
		if got := bare.count("CommitData"); got != 1 {
			t.Errorf("attempts without a key = %d, want 1", got)
		}

		keyed := flaky(1)
		if _, err := newFlakyClient(t, keyed, fastRetry).Dataset("ds1").
			Data.Commit(ctx, &jennah.CommitDataRequest{IdempotencyKey: "k1"}); err != nil {
			t.Fatalf("Commit with key: %v", err)
		}
		if got := keyed.count("CommitData"); got != 2 {
			t.Errorf("attempts with a key = %d, want 2", got)
		}
	})

	t.Run("approval is replayed only with a request key", func(t *testing.T) {
		bare := flaky(1)
		if _, err := newFlakyClient(t, bare, fastRetry).Approvals.
			Create(ctx, &jennah.CreateApprovalRequest{Title: "t"}); err == nil {
			t.Fatal("create unexpectedly succeeded")
		}
		if got := bare.count("CreateApproval"); got != 1 {
			t.Errorf("attempts without a key = %d, want 1: mail cannot be recalled", got)
		}

		keyed := flaky(1)
		if _, err := newFlakyClient(t, keyed, fastRetry).Approvals.
			Create(ctx, &jennah.CreateApprovalRequest{Title: "t", RequestKey: "rk1"}); err != nil {
			t.Fatalf("Create with key: %v", err)
		}
		if got := keyed.count("CreateApproval"); got != 2 {
			t.Errorf("attempts with a key = %d, want 2", got)
		}
	})
}

// A replayable read still must not be replayed on a non-transient code: an
// entitlement limit returns the same answer however often it is asked.
func TestRetryIgnoresNonTransientCodes(t *testing.T) {
	fake := flaky(0)
	fake.always = codes.ResourceExhausted
	jc := newFlakyClient(t, fake, fastRetry)

	_, err := jc.List(context.Background(), jennah.ListInput{})
	if !jennah.IsLimitExceeded(err) {
		t.Fatalf("error = %v, want ResourceExhausted", err)
	}
	if got := fake.count("ListAgents"); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

// Disabled means disabled, including for reads.
func TestRetryDisabled(t *testing.T) {
	fake := flaky(1)
	jc := newFlakyClient(t, fake, jennah.RetryPolicy{Disabled: true})

	if _, err := jc.List(context.Background(), jennah.ListInput{}); err == nil {
		t.Fatal("List unexpectedly succeeded")
	}
	if got := fake.count("ListAgents"); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}
}

func TestErrorClassifiers(t *testing.T) {
	cases := []struct {
		code  codes.Code
		is    func(error) bool
		named string
	}{
		{codes.AlreadyExists, jennah.IsAlreadyCommitted, "IsAlreadyCommitted"},
		{codes.ResourceExhausted, jennah.IsLimitExceeded, "IsLimitExceeded"},
		{codes.Unimplemented, jennah.IsUnsupported, "IsUnsupported"},
		{codes.PermissionDenied, jennah.IsDenied, "IsDenied"},
		{codes.Unauthenticated, jennah.IsUnauthenticated, "IsUnauthenticated"},
		{codes.NotFound, jennah.IsNotFound, "IsNotFound"},
		{codes.Unavailable, jennah.IsTransient, "IsTransient"},
	}
	for _, c := range cases {
		err := status.Error(c.code, "x")
		if !c.is(err) {
			t.Errorf("%s(%v) = false, want true", c.named, c.code)
		}
		if c.is(nil) {
			t.Errorf("%s(nil) = true, want false", c.named)
		}
		if c.is(status.Error(codes.Internal, "other")) {
			t.Errorf("%s matched an unrelated code", c.named)
		}
	}
	if jennah.Code(nil) != codes.OK {
		t.Error("Code(nil) should be OK")
	}
}

// Configured attempt caps limit retries.
func TestRetryHonoursTheAttemptCap(t *testing.T) {
	fake := flaky(50) // never recovers
	jc := newFlakyClient(t, fake, fastRetry)

	_, err := jc.List(context.Background(), jennah.ListInput{})
	if !jennah.IsTransient(err) {
		t.Fatalf("error = %v, want the transient failure surfaced", err)
	}
	if got := fake.count("ListAgents"); got != 3 {
		t.Errorf("attempts = %d, want the 3 the policy allows", got)
	}
}

// A deadline reached during backoff ends the call, and gax reports the deadline
// rather than the last transport failure.
func TestRetryStopsAtTheDeadline(t *testing.T) {
	fake := flaky(50)
	jc := newFlakyClient(t, fake, jennah.RetryPolicy{
		MaxAttempts: 100,
		BaseBackoff: 300 * time.Millisecond,
		MaxBackoff:  300 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	_, err := jc.List(ctx, jennah.ListInput{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want the caller's deadline", err)
	}
	// The context error arrives unwrapped, so Code has to map it rather than call it
	// Unknown.
	if got := jennah.Code(err); got != codes.DeadlineExceeded {
		t.Errorf("Code = %v, want DeadlineExceeded", got)
	}
	if jennah.IsTransient(err) {
		t.Error("IsTransient(deadline) = true, want false")
	}
	if got := fake.count("ListAgents"); got > 2 {
		t.Errorf("attempts = %d, want the deadline to cut the loop short", got)
	}
}
