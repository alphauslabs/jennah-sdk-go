package jennah_test

import (
	"context"
	"net"
	"testing"

	jennah "github.com/alphauslabs/jennah-sdk-go"
	agentv1 "github.com/alphauslabs/jennah-sdk-go/jennah/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fakeServer records the last request and inbound authorization header for each
// service, letting the tests assert exactly what the SDK put on the wire. The
// graph query section returns Unimplemented, mirroring the current backend, so
// the passthrough of a deferred-section error can be verified.
type fakeServer struct {
	agentv1.UnimplementedAgentServiceServer
	agentv1.UnimplementedMemoryServiceServer

	lastAuth   string
	lastCreate *agentv1.CreateAgentRequest
	lastDelete *agentv1.DeleteAgentRequest
	lastCommit *agentv1.CommitMemoryRequest
	lastQuery  *agentv1.QueryMemoryRequest
}

func (f *fakeServer) recordAuth(ctx context.Context) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := md.Get("authorization"); len(v) > 0 {
			f.lastAuth = v[0]
		}
	}
}

func (f *fakeServer) CreateAgent(ctx context.Context, in *agentv1.CreateAgentRequest) (*agentv1.CreateAgentResponse, error) {
	f.recordAuth(ctx)
	f.lastCreate = in
	return &agentv1.CreateAgentResponse{Agent: &agentv1.AgentInstance{
		AgentInstanceId: in.GetAgentInstanceId(),
		AgentName:       in.GetAgentName(),
	}}, nil
}

func (f *fakeServer) DeleteAgent(ctx context.Context, in *agentv1.DeleteAgentRequest) (*agentv1.DeleteAgentResponse, error) {
	f.recordAuth(ctx)
	f.lastDelete = in
	return &agentv1.DeleteAgentResponse{ExecutionLogRows: 3}, nil
}

func (f *fakeServer) CommitMemory(ctx context.Context, in *agentv1.CommitMemoryRequest) (*agentv1.CommitMemoryResponse, error) {
	f.recordAuth(ctx)
	f.lastCommit = in
	return &agentv1.CommitMemoryResponse{VectorRows: int64(len(in.GetVectors()))}, nil
}

func (f *fakeServer) QueryMemory(ctx context.Context, in *agentv1.QueryMemoryRequest) (*agentv1.QueryMemoryResponse, error) {
	f.recordAuth(ctx)
	f.lastQuery = in
	if in.GetGraph() != nil {
		return nil, status.Error(codes.Unimplemented, "the graph query section is not yet available")
	}
	return &agentv1.QueryMemoryResponse{Semantic: &agentv1.SemanticResult{
		Matches: []*agentv1.SemanticMatch{{ChunkId: "c1", Distance: 0.1}},
	}}, nil
}

// newTestClient spins up the fake server on an in-memory listener and returns a
// Client wired to it.
func newTestClient(t *testing.T) (*jennah.Client, *fakeServer) {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	fake := &fakeServer{}
	agentv1.RegisterAgentServiceServer(srv, fake)
	agentv1.RegisterMemoryServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()

	jc, err := jennah.NewClient(jennah.Config{
		Endpoint:     "passthrough:///bufnet",
		APIKey:       "jennah_sk_testkey",
		EnterpriseID: "ent-123",
		Insecure:     true,
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
	return jc, fake
}

func TestNewClientValidation(t *testing.T) {
	if _, err := jennah.NewClient(jennah.Config{APIKey: "k"}); err == nil {
		t.Error("expected error when Endpoint is empty")
	}
	if _, err := jennah.NewClient(jennah.Config{Endpoint: "e:443"}); err == nil {
		t.Error("expected error when APIKey is empty")
	}
}

func TestBearerCredentialSent(t *testing.T) {
	jc, fake := newTestClient(t)
	if _, err := jc.Agent("a1").Destroy(context.Background()); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if fake.lastAuth != "Bearer jennah_sk_testkey" {
		t.Errorf("authorization = %q, want %q", fake.lastAuth, "Bearer jennah_sk_testkey")
	}
}

func TestSpawnAndDestroy(t *testing.T) {
	jc, fake := newTestClient(t)
	ctx := context.Background()

	a, err := jc.Spawn(ctx, jennah.SpawnInput{AgentInstanceID: "a1", AgentName: "n", Region: "us-east-1"})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if fake.lastCreate.GetAgentInstanceId() != "a1" || fake.lastCreate.GetRegion() != "us-east-1" {
		t.Errorf("CreateAgent got %+v", fake.lastCreate)
	}
	if a.ID() != "a1" {
		t.Errorf("handle id = %q, want a1", a.ID())
	}
	if _, err := a.Destroy(ctx); err != nil {
		t.Fatalf("Destroy: %v", err)
	}
	if fake.lastDelete.GetAgentInstanceId() != "a1" {
		t.Errorf("DeleteAgent id = %q, want a1", fake.lastDelete.GetAgentInstanceId())
	}
}

// Each convenience must build a request with exactly its one section set and
// route the agent id, proving they are single-section wrappers over the two
// unified endpoints.
func TestConveniencesAreSingleSection(t *testing.T) {
	jc, fake := newTestClient(t)
	ctx := context.Background()
	a := jc.Agent("a7")

	if _, err := a.Logs.Create(ctx, &jennah.ExecutionLogStep{StepId: "s1"}); err != nil {
		t.Fatalf("Logs.Create: %v", err)
	}
	if c := fake.lastCommit; c.GetAgentInstanceId() != "a7" || c.GetLog() == nil || len(c.GetVectors()) != 0 || c.GetGraph() != nil {
		t.Errorf("Logs.Create sent %+v", fake.lastCommit)
	}

	if _, err := a.Vectors.Upsert(ctx, &jennah.VectorChunk{ChunkId: "c1"}, &jennah.VectorChunk{ChunkId: "c2"}); err != nil {
		t.Fatalf("Vectors.Upsert: %v", err)
	}
	if c := fake.lastCommit; len(c.GetVectors()) != 2 || c.GetLog() != nil || c.GetGraph() != nil {
		t.Errorf("Vectors.Upsert sent %+v", fake.lastCommit)
	}

	res, err := a.Vectors.Search(ctx, &jennah.SemanticQuery{Embedding: []float32{0.1, 0.2}, Limit: 5})
	if err != nil {
		t.Fatalf("Vectors.Search: %v", err)
	}
	if q := fake.lastQuery; q.GetSemantic() == nil || q.GetGraph() != nil || q.GetLog() != nil {
		t.Errorf("Vectors.Search sent %+v", fake.lastQuery)
	}
	if len(res.GetMatches()) != 1 || res.GetMatches()[0].GetChunkId() != "c1" {
		t.Errorf("Search result = %+v", res)
	}
}

// A deferred backend section (graph query) must surface its gRPC status
// unchanged through the convenience.
func TestGraphQueryUnimplementedPassthrough(t *testing.T) {
	jc, _ := newTestClient(t)
	_, err := jc.Agent("a1").Graph.Query(context.Background(), "MATCH (n) RETURN n")
	if status.Code(err) != codes.Unimplemented {
		t.Errorf("Graph.Query code = %v, want Unimplemented", status.Code(err))
	}
}
