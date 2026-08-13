package jennah

import (
	"context"

	datastorev1 "github.com/alphauslabs/jennah-sdk-go/jennah/datastore/v1"
)

// DatasetsAPI is the dataset collection: create one, or list what the credential
// can reach. Reach a single dataset with Client.Dataset.
type DatasetsAPI struct{ c *Client }

// Create provisions an application dataset and returns a handle to it. The
// enterprise comes from the credential, never from the request.
//
// It returns the response alongside the handle, unlike Client.Spawn, because a
// request that set CreateApiKey gets back a one-time plaintext key that cannot be
// retrieved again. A two-value signature would make dropping that secret the
// path of least resistance.
//
// Provisioning is asynchronous: the dataset can come back before its physical
// objects exist, so read the status through Dataset.Get rather than assuming a
// returned dataset is ready to write.
func (d DatasetsAPI) Create(ctx context.Context, in *datastorev1.CreateDatasetRequest) (*Dataset, *datastorev1.CreateDatasetResponse, error) {
	resp, err := d.c.datasets.CreateDataset(ctx, in)
	if err != nil {
		return nil, nil, err
	}
	return d.c.Dataset(resp.GetDataset().GetDatasetId()), resp, nil
}

// List returns a page of the caller's datasets.
func (d DatasetsAPI) List(ctx context.Context, in *datastorev1.ListDatasetsRequest) (*datastorev1.ListDatasetsResponse, error) {
	if in == nil {
		in = &datastorev1.ListDatasetsRequest{}
	}
	return d.c.datasets.ListDatasets(ctx, in)
}

// Dataset returns a handle scoped to a single dataset. It performs no network
// call; the dataset need not exist yet (see Client.Datasets.Create).
func (c *Client) Dataset(datasetID string) *Dataset {
	ds := &Dataset{id: datasetID, c: c}
	ds.Schema = schemaAPI{ds: ds}
	ds.Data = dataAPI{ds: ds}
	return ds
}

// Dataset is a handle to one application dataset: user-defined tables on the
// same backend as agent memory, reached through Schema (what the tables are) and
// Data (what is in them).
type Dataset struct {
	id string
	c  *Client

	Schema schemaAPI
	Data   dataAPI
}

// ID returns the dataset id this handle is scoped to.
func (d *Dataset) ID() string { return d.id }

// Get reads the dataset's directory record, including its provisioning status.
func (d *Dataset) Get(ctx context.Context) (*datastorev1.Dataset, error) {
	resp, err := d.c.datasets.GetDataset(ctx, &datastorev1.GetDatasetRequest{DatasetId: d.id})
	if err != nil {
		return nil, err
	}
	return resp.GetDataset(), nil
}

// Destroy deletes the dataset and drops its tables. The returned receipt reports
// what was removed.
func (d *Dataset) Destroy(ctx context.Context) (*datastorev1.DeleteDatasetResponse, error) {
	return d.c.datasets.DeleteDataset(ctx, &datastorev1.DeleteDatasetRequest{DatasetId: d.id})
}

// schemaAPI is the dataset's logical table catalog.
type schemaAPI struct{ ds *Dataset }

// Get reads the dataset's declared tables and its schema usage.
func (s schemaAPI) Get(ctx context.Context) (*datastorev1.GetSchemaResponse, error) {
	return s.ds.c.schema.GetSchema(ctx, &datastorev1.GetSchemaRequest{DatasetId: s.ds.id})
}

// Declare creates or evolves the dataset's tables. The dataset id comes from the
// handle, so leave it unset on the request.
//
// The work is asynchronous: a table can come back PENDING and become READY later,
// so a caller that writes immediately after declaring must poll Get rather than
// treat the response as completion.
func (s schemaAPI) Declare(ctx context.Context, in *datastorev1.DeclareTablesRequest) (*datastorev1.DeclareTablesResponse, error) {
	if in == nil {
		in = &datastorev1.DeclareTablesRequest{}
	}
	in.DatasetId = s.ds.id
	return s.ds.c.schema.DeclareTables(ctx, in)
}

// dataAPI is the dataset's rows.
type dataAPI struct{ ds *Dataset }

// Commit applies a transactional set of row operations. The dataset id comes from
// the handle, so leave it unset on the request.
//
// Set IdempotencyKey to make a retried commit safe: the server returns the first
// attempt's receipt instead of applying the operations twice.
func (d dataAPI) Commit(ctx context.Context, in *datastorev1.CommitDataRequest) (*datastorev1.CommitDataResponse, error) {
	if in == nil {
		in = &datastorev1.CommitDataRequest{}
	}
	in.DatasetId = d.ds.id
	return d.ds.c.data.CommitData(ctx, in)
}

// Query runs a structured read. The dataset id comes from the handle, so leave it
// unset on the request.
//
// Both sections of one request are evaluated at a single read instant, which the
// response reports as ReadTimestamp.
func (d dataAPI) Query(ctx context.Context, in *datastorev1.QueryDataRequest) (*datastorev1.QueryDataResponse, error) {
	if in == nil {
		in = &datastorev1.QueryDataRequest{}
	}
	in.DatasetId = d.ds.id
	return d.ds.c.data.QueryData(ctx, in)
}
