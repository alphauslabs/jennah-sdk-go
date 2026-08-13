package jennah_test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	jennah "github.com/alphauslabs/jennah-sdk-go"
	agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"
	approvalv1 "github.com/alphauslabs/jennah-sdk-go/jennah/approval/v1"
	datastorev1 "github.com/alphauslabs/jennah-sdk-go/jennah/datastore/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// pagedServer serves two pages of datasets, two pages of inspect chunks, and an
// approval that stays pending for the first `pendingRounds` waits before being
// approved. It records the page tokens it was handed so the tests can prove the
// cursor was actually threaded rather than the first page fetched twice.
type pagedServer struct {
	datastorev1.UnimplementedDatasetServiceServer
	agentv1.UnimplementedMemoryServiceServer
	approvalv1.UnimplementedApprovalServiceServer

	mu            sync.Mutex
	dsTokens      []string
	chunkTokens   []string
	waitCalls     int
	pendingRounds int
	failOnPage    int // when > 0, the call with that index fails
	dsCalls       int
}

func (p *pagedServer) ListDatasets(_ context.Context, in *datastorev1.ListDatasetsRequest) (*datastorev1.ListDatasetsResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dsCalls++
	p.dsTokens = append(p.dsTokens, in.GetPageToken())
	if p.failOnPage > 0 && p.dsCalls == p.failOnPage {
		return nil, status.Error(codes.Internal, "page blew up")
	}
	switch in.GetPageToken() {
	case "":
		return &datastorev1.ListDatasetsResponse{
			Datasets:      []*datastorev1.Dataset{{DatasetId: "ds1"}, {DatasetId: "ds2"}},
			NextPageToken: "p2",
		}, nil
	case "p2":
		return &datastorev1.ListDatasetsResponse{
			Datasets: []*datastorev1.Dataset{{DatasetId: "ds3"}},
		}, nil
	default:
		return &datastorev1.ListDatasetsResponse{}, nil
	}
}

func (p *pagedServer) InspectMemory(_ context.Context, in *agentv1.InspectMemoryRequest) (*agentv1.InspectMemoryResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	token := in.GetVectors().GetPageToken()
	p.chunkTokens = append(p.chunkTokens, token)
	if token == "" {
		return &agentv1.InspectMemoryResponse{
			Vectors:        &agentv1.VectorInspectResult{Chunks: []*agentv1.VectorChunkInfo{{ChunkId: "c1"}}},
			NextChunkToken: "c2tok",
		}, nil
	}
	return &agentv1.InspectMemoryResponse{
		Vectors: &agentv1.VectorInspectResult{Chunks: []*agentv1.VectorChunkInfo{{ChunkId: "c2"}}},
	}, nil
}

func (p *pagedServer) WaitApproval(_ context.Context, in *approvalv1.WaitApprovalRequest) (*approvalv1.WaitApprovalResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.waitCalls++
	if p.waitCalls <= p.pendingRounds {
		// The server reports a reached ceiling as a SUCCESS carrying the pending
		// approval, which is exactly the case a hand-written loop gets wrong.
		return &approvalv1.WaitApprovalResponse{
			Approval: &approvalv1.Approval{
				ApprovalId: in.GetApprovalId(),
				Status:     approvalv1.ApprovalStatus_APPROVAL_STATUS_PENDING,
			},
			TimedOut: true,
		}, nil
	}
	return &approvalv1.WaitApprovalResponse{
		Approval: &approvalv1.Approval{
			ApprovalId: in.GetApprovalId(),
			Status:     approvalv1.ApprovalStatus_APPROVAL_STATUS_APPROVED,
		},
	}, nil
}

func (p *pagedServer) waits() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.waitCalls
}

func newPagedClient(t *testing.T, fake *pagedServer) *jennah.Client {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	datastorev1.RegisterDatasetServiceServer(srv, fake)
	agentv1.RegisterMemoryServiceServer(srv, fake)
	approvalv1.RegisterApprovalServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()

	jc, err := jennah.NewClient(jennah.Config{
		Endpoint: "passthrough:///bufnet",
		APIKey:   "jennah_sk_testkey",
		Insecure: true,
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

// The iterator must thread the cursor and yield every page's items in order.
func TestIteratorWalksEveryPage(t *testing.T) {
	fake := &pagedServer{}
	jc := newPagedClient(t, fake)

	var got []string
	for ds, err := range jc.Datasets.All(context.Background(), nil) {
		if err != nil {
			t.Fatalf("iteration: %v", err)
		}
		got = append(got, ds.GetDatasetId())
	}
	if want := []string{"ds1", "ds2", "ds3"}; len(got) != 3 || got[0] != want[0] || got[2] != want[2] {
		t.Errorf("items = %v, want %v", got, want)
	}
	if len(fake.dsTokens) != 2 || fake.dsTokens[0] != "" || fake.dsTokens[1] != "p2" {
		t.Errorf("page tokens = %v, want [\"\" p2]", fake.dsTokens)
	}
}

// Breaking out must stop fetching. Without this, a caller who wanted the first
// result pays for the whole listing.
func TestIteratorStopsOnBreak(t *testing.T) {
	fake := &pagedServer{}
	jc := newPagedClient(t, fake)

	for range jc.Datasets.All(context.Background(), nil) {
		break
	}
	if fake.dsCalls != 1 {
		t.Errorf("pages fetched = %d, want 1", fake.dsCalls)
	}
}

// An error is yielded once, with no item, and ends the sequence.
func TestIteratorYieldsErrorOnce(t *testing.T) {
	fake := &pagedServer{failOnPage: 2}
	jc := newPagedClient(t, fake)

	var items, errs int
	for ds, err := range jc.Datasets.All(context.Background(), nil) {
		if err != nil {
			errs++
			if ds != nil {
				t.Error("an error was yielded alongside an item")
			}
			continue
		}
		items++
	}
	if items != 2 || errs != 1 {
		t.Errorf("items = %d, errors = %d, want 2 and 1", items, errs)
	}
}

// The caller's request must survive the walk unmodified, so it can drive another.
func TestIteratorDoesNotMutateTheRequest(t *testing.T) {
	jc := newPagedClient(t, &pagedServer{})

	req := &jennah.ListDatasetsRequest{PageSize: 2}
	for range jc.Datasets.All(context.Background(), req) {
	}
	if req.GetPageToken() != "" {
		t.Errorf("caller's PageToken = %q, want it untouched", req.GetPageToken())
	}
}

// The inspect iterators exist so a caller never handles the four cursors by hand.
func TestInspectIteratorPagesOneSection(t *testing.T) {
	fake := &pagedServer{}
	jc := newPagedClient(t, fake)

	var ids []string
	for c, err := range jc.Agent("a1").Vectors.AllChunks(context.Background(), nil) {
		if err != nil {
			t.Fatalf("iteration: %v", err)
		}
		ids = append(ids, c.GetChunkId())
	}
	if len(ids) != 2 || ids[0] != "c1" || ids[1] != "c2" {
		t.Errorf("chunks = %v, want [c1 c2]", ids)
	}
	if len(fake.chunkTokens) != 2 || fake.chunkTokens[1] != "c2tok" {
		t.Errorf("chunk tokens = %v, want the second call to carry c2tok", fake.chunkTokens)
	}
}

// WaitUntilDecided must keep waiting through a TimedOut response and return only a
// terminal approval.
func TestWaitUntilDecidedLoopsPastTimeouts(t *testing.T) {
	fake := &pagedServer{pendingRounds: 2}
	jc := newPagedClient(t, fake)

	got, err := jc.Approvals.WaitUntilDecided(context.Background(), "ap1")
	if err != nil {
		t.Fatalf("WaitUntilDecided: %v", err)
	}
	if got.GetStatus() != approvalv1.ApprovalStatus_APPROVAL_STATUS_APPROVED {
		t.Errorf("status = %v, want APPROVED", got.GetStatus())
	}
	if fake.waits() != 3 {
		t.Errorf("wait calls = %d, want 3", fake.waits())
	}
}

// When the caller's deadline arrives it must report that, and hand back the last
// pending approval so the caller can say what it was waiting on.
func TestWaitUntilDecidedRespectsTheDeadline(t *testing.T) {
	fake := &pagedServer{pendingRounds: 1 << 30} // never decides
	jc := newPagedClient(t, fake)

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()

	start := time.Now()
	last, err := jc.Approvals.WaitUntilDecided(ctx, "ap1")
	if err != context.DeadlineExceeded {
		t.Fatalf("error = %v, want DeadlineExceeded", err)
	}
	if last.GetStatus() != approvalv1.ApprovalStatus_APPROVAL_STATUS_PENDING {
		t.Errorf("last approval = %v, want the pending one", last)
	}
	// The poll floor has to hold, or an instantly-returning server turns the loop
	// into a hot spin: ~1.2s of budget at a 250ms floor is a handful of calls.
	if n := fake.waits(); n > 8 {
		t.Errorf("wait calls = %d in %v, want the poll floor to bound them", n, time.Since(start))
	}
}
