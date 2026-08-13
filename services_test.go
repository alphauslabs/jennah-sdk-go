package jennah_test

import (
	"context"
	"net"
	"testing"

	jennah "github.com/alphauslabs/jennah-sdk-go"
	approvalv1 "github.com/alphauslabs/jennah-sdk-go/jennah/approval/v1"
	authv1 "github.com/alphauslabs/jennah-sdk-go/jennah/auth/v1"
	billingv1 "github.com/alphauslabs/jennah-sdk-go/jennah/billing/v1"
	datastorev1 "github.com/alphauslabs/jennah-sdk-go/jennah/datastore/v1"
	platformv1 "github.com/alphauslabs/jennah-sdk-go/jennah/platform/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// fakeServices implements every non-agent service the endpoint publishes,
// recording which methods the SDK reached and the requests it sent. Only the
// methods the tests below drive are implemented; the embedded Unimplemented
// servers cover the rest.
type fakeServices struct {
	datastorev1.UnimplementedDatasetServiceServer
	datastorev1.UnimplementedSchemaServiceServer
	datastorev1.UnimplementedDataServiceServer
	authv1.UnimplementedAuthServiceServer
	approvalv1.UnimplementedApprovalServiceServer
	billingv1.UnimplementedBillingServiceServer
	platformv1.UnimplementedPlatformServiceServer

	hits map[string]bool

	lastDeclare *datastorev1.DeclareTablesRequest
	lastCommit  *datastorev1.CommitDataRequest
	lastQuery   *datastorev1.QueryDataRequest
	lastGetDS   *datastorev1.GetDatasetRequest
	lastRefresh *authv1.RefreshTokenRequest
	lastAddAppr *approvalv1.AddApproverRequest
}

func (f *fakeServices) hit(name string) { f.hits[name] = true }

// --- datastore ---

func (f *fakeServices) CreateDataset(_ context.Context, in *datastorev1.CreateDatasetRequest) (*datastorev1.CreateDatasetResponse, error) {
	f.hit("CreateDataset")
	return &datastorev1.CreateDatasetResponse{
		Dataset:      &datastorev1.Dataset{DatasetId: in.GetDatasetId()},
		ApiKeySecret: "jennah_sk_minted",
	}, nil
}

func (f *fakeServices) ListDatasets(context.Context, *datastorev1.ListDatasetsRequest) (*datastorev1.ListDatasetsResponse, error) {
	f.hit("ListDatasets")
	return &datastorev1.ListDatasetsResponse{}, nil
}

func (f *fakeServices) GetDataset(_ context.Context, in *datastorev1.GetDatasetRequest) (*datastorev1.GetDatasetResponse, error) {
	f.hit("GetDataset")
	f.lastGetDS = in
	return &datastorev1.GetDatasetResponse{Dataset: &datastorev1.Dataset{DatasetId: in.GetDatasetId()}}, nil
}

func (f *fakeServices) DeleteDataset(context.Context, *datastorev1.DeleteDatasetRequest) (*datastorev1.DeleteDatasetResponse, error) {
	f.hit("DeleteDataset")
	return &datastorev1.DeleteDatasetResponse{}, nil
}

func (f *fakeServices) GetSchema(context.Context, *datastorev1.GetSchemaRequest) (*datastorev1.GetSchemaResponse, error) {
	f.hit("GetSchema")
	return &datastorev1.GetSchemaResponse{}, nil
}

func (f *fakeServices) DeclareTables(_ context.Context, in *datastorev1.DeclareTablesRequest) (*datastorev1.DeclareTablesResponse, error) {
	f.hit("DeclareTables")
	f.lastDeclare = in
	return &datastorev1.DeclareTablesResponse{}, nil
}

func (f *fakeServices) CommitData(_ context.Context, in *datastorev1.CommitDataRequest) (*datastorev1.CommitDataResponse, error) {
	f.hit("CommitData")
	f.lastCommit = in
	return &datastorev1.CommitDataResponse{}, nil
}

func (f *fakeServices) QueryData(_ context.Context, in *datastorev1.QueryDataRequest) (*datastorev1.QueryDataResponse, error) {
	f.hit("QueryData")
	f.lastQuery = in
	return &datastorev1.QueryDataResponse{}, nil
}

// --- auth ---

func (f *fakeServices) WhoAmI(context.Context, *authv1.WhoAmIRequest) (*authv1.WhoAmIResponse, error) {
	f.hit("WhoAmI")
	return &authv1.WhoAmIResponse{}, nil
}

func (f *fakeServices) RefreshToken(_ context.Context, in *authv1.RefreshTokenRequest) (*authv1.RefreshTokenResponse, error) {
	f.hit("RefreshToken")
	f.lastRefresh = in
	return &authv1.RefreshTokenResponse{}, nil
}

func (f *fakeServices) ListApiKeys(context.Context, *authv1.ListApiKeysRequest) (*authv1.ListApiKeysResponse, error) {
	f.hit("ListApiKeys")
	return &authv1.ListApiKeysResponse{}, nil
}

func (f *fakeServices) ListMembers(context.Context, *authv1.ListMembersRequest) (*authv1.ListMembersResponse, error) {
	f.hit("ListMembers")
	return &authv1.ListMembersResponse{}, nil
}

func (f *fakeServices) ListInvitations(context.Context, *authv1.ListInvitationsRequest) (*authv1.ListInvitationsResponse, error) {
	f.hit("ListInvitations")
	return &authv1.ListInvitationsResponse{}, nil
}

func (f *fakeServices) ListPermissions(context.Context, *authv1.ListPermissionsRequest) (*authv1.ListPermissionsResponse, error) {
	f.hit("ListPermissions")
	return &authv1.ListPermissionsResponse{}, nil
}

func (f *fakeServices) UpdateEnterprise(context.Context, *authv1.UpdateEnterpriseRequest) (*authv1.UpdateEnterpriseResponse, error) {
	f.hit("UpdateEnterprise")
	return &authv1.UpdateEnterpriseResponse{}, nil
}

// --- approvals ---

func (f *fakeServices) ListApprovals(context.Context, *approvalv1.ListApprovalsRequest) (*approvalv1.ListApprovalsResponse, error) {
	f.hit("ListApprovals")
	return &approvalv1.ListApprovalsResponse{}, nil
}

func (f *fakeServices) GetApproval(context.Context, *approvalv1.GetApprovalRequest) (*approvalv1.GetApprovalResponse, error) {
	f.hit("GetApproval")
	return &approvalv1.GetApprovalResponse{}, nil
}

func (f *fakeServices) AddApprover(_ context.Context, in *approvalv1.AddApproverRequest) (*approvalv1.AddApproverResponse, error) {
	f.hit("AddApprover")
	f.lastAddAppr = in
	return &approvalv1.AddApproverResponse{}, nil
}

// --- billing and platform ---

func (f *fakeServices) GetBillingState(context.Context, *billingv1.GetBillingStateRequest) (*billingv1.GetBillingStateResponse, error) {
	f.hit("GetBillingState")
	return &billingv1.GetBillingStateResponse{}, nil
}

func (f *fakeServices) ListLocations(context.Context, *platformv1.ListLocationsRequest) (*platformv1.ListLocationsResponse, error) {
	f.hit("ListLocations")
	return &platformv1.ListLocationsResponse{}, nil
}

// newServicesClient wires a Client to a server carrying every non-agent service.
func newServicesClient(t *testing.T) (*jennah.Client, *fakeServices) {
	t.Helper()

	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	fake := &fakeServices{hits: map[string]bool{}}
	datastorev1.RegisterDatasetServiceServer(srv, fake)
	datastorev1.RegisterSchemaServiceServer(srv, fake)
	datastorev1.RegisterDataServiceServer(srv, fake)
	authv1.RegisterAuthServiceServer(srv, fake)
	approvalv1.RegisterApprovalServiceServer(srv, fake)
	billingv1.RegisterBillingServiceServer(srv, fake)
	platformv1.RegisterPlatformServiceServer(srv, fake)
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
	return jc, fake
}

// Every service the endpoint publishes must be reachable through the ergonomic
// layer. The endpoint serves whatever the server registers, so a service the SDK
// forgets is not an absent capability, it is one the caller has to drop to the
// generated stubs for.
func TestEveryServiceIsReachable(t *testing.T) {
	jc, fake := newServicesClient(t)
	ctx := context.Background()

	// One call per service, through the namespaces a caller would use.
	if _, err := jc.Platform.Locations(ctx); err != nil { // PlatformService
		t.Fatalf("Platform.Locations: %v", err)
	}
	if _, err := jc.Billing.State(ctx); err != nil { // BillingService
		t.Fatalf("Billing.State: %v", err)
	}
	if _, err := jc.Auth.WhoAmI(ctx); err != nil { // AuthService
		t.Fatalf("Auth.WhoAmI: %v", err)
	}
	if _, err := jc.Approvals.List(ctx, nil); err != nil { // ApprovalService
		t.Fatalf("Approvals.List: %v", err)
	}
	if _, err := jc.Datasets.List(ctx, nil); err != nil { // DatasetService
		t.Fatalf("Datasets.List: %v", err)
	}
	if _, err := jc.Dataset("ds1").Schema.Get(ctx); err != nil { // SchemaService
		t.Fatalf("Schema.Get: %v", err)
	}
	if _, err := jc.Dataset("ds1").Data.Query(ctx, nil); err != nil { // DataService
		t.Fatalf("Data.Query: %v", err)
	}

	// AgentService and MemoryService are covered against their own fake in
	// client_test.go; the seven above are the rest of the published set.
	for _, want := range []string{
		"ListLocations", "GetBillingState", "WhoAmI", "ListApprovals",
		"ListDatasets", "GetSchema", "QueryData",
	} {
		if !fake.hits[want] {
			t.Errorf("%s was never reached", want)
		}
	}
}

// Each Auth sub-namespace must route to its own RPC: the grouping is for the
// caller's benefit and must not change which method is called.
func TestAuthNamespacesRoute(t *testing.T) {
	jc, fake := newServicesClient(t)
	ctx := context.Background()

	if _, err := jc.Auth.Keys.List(ctx, nil); err != nil {
		t.Fatalf("Keys.List: %v", err)
	}
	if _, err := jc.Auth.Members.List(ctx, nil); err != nil {
		t.Fatalf("Members.List: %v", err)
	}
	if _, err := jc.Auth.Invitations.List(ctx, nil); err != nil {
		t.Fatalf("Invitations.List: %v", err)
	}
	if _, err := jc.Auth.Roles.Permissions(ctx); err != nil {
		t.Fatalf("Roles.Permissions: %v", err)
	}
	if _, err := jc.Auth.Enterprise.Update(ctx, &jennah.UpdateEnterpriseRequest{Name: "n"}); err != nil {
		t.Fatalf("Enterprise.Update: %v", err)
	}
	for _, want := range []string{"ListApiKeys", "ListMembers", "ListInvitations", "ListPermissions", "UpdateEnterprise"} {
		if !fake.hits[want] {
			t.Errorf("%s was never reached", want)
		}
	}

	// Switching enterprise is a scoped refresh, not its own RPC: the enterprise id
	// has to land on RefreshToken or the caller silently stays where they were.
	if _, err := jc.Auth.Session.Refresh(ctx, "rt", "ent-9"); err != nil {
		t.Fatalf("Session.Refresh: %v", err)
	}
	if got := fake.lastRefresh; got.GetRefreshToken() != "rt" || got.GetEnterpriseId() != "ent-9" {
		t.Errorf("RefreshToken sent %+v", fake.lastRefresh)
	}
}

// The dataset handle owns the dataset id. A caller passing a request with the
// field unset (or set to something else) must still address the handle's dataset,
// or a mistyped id would write to the wrong tenant resource.
func TestDatasetHandleOwnsTheID(t *testing.T) {
	jc, fake := newServicesClient(t)
	ctx := context.Background()
	ds := jc.Dataset("ds-7")

	if _, err := ds.Schema.Declare(ctx, &jennah.DeclareTablesRequest{}); err != nil {
		t.Fatalf("Schema.Declare: %v", err)
	}
	if got := fake.lastDeclare.GetDatasetId(); got != "ds-7" {
		t.Errorf("DeclareTables dataset = %q, want ds-7", got)
	}

	if _, err := ds.Data.Commit(ctx, &jennah.CommitDataRequest{DatasetId: "ds-other"}); err != nil {
		t.Fatalf("Data.Commit: %v", err)
	}
	if got := fake.lastCommit.GetDatasetId(); got != "ds-7" {
		t.Errorf("CommitData dataset = %q, want the handle's ds-7", got)
	}

	if _, err := ds.Data.Query(ctx, nil); err != nil {
		t.Fatalf("Data.Query: %v", err)
	}
	if got := fake.lastQuery.GetDatasetId(); got != "ds-7" {
		t.Errorf("QueryData dataset = %q, want ds-7", got)
	}

	if _, err := ds.Get(ctx); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got := fake.lastGetDS.GetDatasetId(); got != "ds-7" {
		t.Errorf("GetDataset dataset = %q, want ds-7", got)
	}
	if ds.ID() != "ds-7" {
		t.Errorf("handle id = %q, want ds-7", ds.ID())
	}
}

// Create returns the response alongside the handle.
func TestDatasetCreateSurfacesTheOneTimeKey(t *testing.T) {
	jc, _ := newServicesClient(t)

	ds, resp, err := jc.Datasets.Create(context.Background(), &jennah.CreateDatasetRequest{
		DatasetId:    "ds-new",
		CreateApiKey: true,
	})
	if err != nil {
		t.Fatalf("Datasets.Create: %v", err)
	}
	if ds.ID() != "ds-new" {
		t.Errorf("handle id = %q, want ds-new", ds.ID())
	}
	if resp.GetApiKeySecret() != "jennah_sk_minted" {
		t.Errorf("ApiKeySecret = %q, want the minted key", resp.GetApiKeySecret())
	}
}

// Scalar-argument wrappers must put their argument on the field the server reads.
func TestScalarArgumentsLandOnTheRequest(t *testing.T) {
	jc, fake := newServicesClient(t)

	if _, err := jc.Approvals.Approvers.Add(context.Background(),
		jennah.AllowlistKind_ALLOWLIST_KIND_DOMAIN, "alphaus.cloud"); err != nil {
		t.Fatalf("Approvers.Add: %v", err)
	}
	got := fake.lastAddAppr
	if got.GetKind() != jennah.AllowlistKind_ALLOWLIST_KIND_DOMAIN || got.GetValue() != "alphaus.cloud" {
		t.Errorf("AddApprover sent %+v", got)
	}
}
