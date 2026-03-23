package milvus

import (
	"context"
	"time"
)

type FakeClient struct {
	Version                     string
	VersionErr                  error
	Databases                   []string
	DatabasesErr                error
	CreateDatabaseErr           map[string]error
	DropDatabaseErr             map[string]error
	Collections                 map[string][]string
	CollectionErrs              map[string]error
	CreateCollectionErrs        map[string]map[string]error
	DropCollectionErrs          map[string]map[string]error
	CollectionIDs               map[string]map[string]int64
	CollectionIDErrs            map[string]map[string]error
	RowCounts                   map[string]map[string]int64
	RowCountErrs                map[string]map[string]error
	Descriptions                map[string]map[string]CollectionDescription
	DescriptionErrs             map[string]map[string]error
	LoadStates                  map[string]map[string]LoadState
	LoadStateErrs               map[string]map[string]error
	InsertResults               map[string]map[string]InsertResult
	InsertErrs                  map[string]map[string]error
	FlushErrs                   map[string]map[string]error
	QueryResults                map[string]map[string]QueryResult
	QueryErrs                   map[string]map[string]error
	SearchResults               map[string]map[string]SearchResult
	SearchErrs                  map[string]map[string]error
	LastQueryRequest            QueryRequest
	LastSearchRequest           SearchRequest
	LastCreateCollectionRequest CreateCollectionRequest
	LastInsertRequest           InsertRequest
	Operations                  []string
	MetricsByType               map[string]string
	MetricsErrs                 map[string]error
	Closed                      bool
}

func (f *FakeClient) GetVersion(ctx context.Context) (string, error) {
	_ = ctx
	return f.Version, f.VersionErr
}

func (f *FakeClient) ListDatabases(ctx context.Context) ([]string, error) {
	_ = ctx
	return append([]string(nil), f.Databases...), f.DatabasesErr
}

func (f *FakeClient) CreateDatabase(ctx context.Context, database string) error {
	_ = ctx
	f.Operations = append(f.Operations, "create-database:"+database)
	if err := f.CreateDatabaseErr[database]; err != nil {
		return err
	}
	return nil
}

func (f *FakeClient) DropDatabase(ctx context.Context, database string) error {
	_ = ctx
	f.Operations = append(f.Operations, "drop-database:"+database)
	if err := f.DropDatabaseErr[database]; err != nil {
		return err
	}
	return nil
}

func (f *FakeClient) ListCollections(ctx context.Context, database string) ([]string, error) {
	_ = ctx
	f.Operations = append(f.Operations, "list-collections:"+database)
	if err := f.CollectionErrs[database]; err != nil {
		return nil, err
	}
	return append([]string(nil), f.Collections[database]...), nil
}

func (f *FakeClient) CreateCollection(ctx context.Context, req CreateCollectionRequest) error {
	_ = ctx
	f.LastCreateCollectionRequest = req
	f.Operations = append(f.Operations, "create-collection:"+req.Database+"."+req.Collection)
	if err := f.CreateCollectionErrs[req.Database][req.Collection]; err != nil {
		return err
	}
	return nil
}

func (f *FakeClient) DropCollection(ctx context.Context, database, collection string) error {
	_ = ctx
	f.Operations = append(f.Operations, "drop-collection:"+database+"."+collection)
	if err := f.DropCollectionErrs[database][collection]; err != nil {
		return err
	}
	return nil
}

func (f *FakeClient) GetCollectionRowCount(ctx context.Context, database, collection string) (int64, error) {
	_ = ctx
	if err := f.RowCountErrs[database][collection]; err != nil {
		return 0, err
	}
	return f.RowCounts[database][collection], nil
}

func (f *FakeClient) GetCollectionID(ctx context.Context, database, collection string) (int64, error) {
	_ = ctx
	if err := f.CollectionIDErrs[database][collection]; err != nil {
		return 0, err
	}
	return f.CollectionIDs[database][collection], nil
}

func (f *FakeClient) DescribeCollection(ctx context.Context, database, collection string) (CollectionDescription, error) {
	_ = ctx
	if err := f.DescriptionErrs[database][collection]; err != nil {
		return CollectionDescription{}, err
	}
	return f.Descriptions[database][collection], nil
}

func (f *FakeClient) GetCollectionLoadState(ctx context.Context, database, collection string) (LoadState, error) {
	_ = ctx
	if err := f.LoadStateErrs[database][collection]; err != nil {
		return LoadStateUnknown, err
	}
	if state, ok := f.LoadStates[database][collection]; ok {
		return state, nil
	}
	return LoadStateUnknown, nil
}

func (f *FakeClient) Insert(ctx context.Context, req InsertRequest) (InsertResult, error) {
	_ = ctx
	f.LastInsertRequest = req
	f.Operations = append(f.Operations, "insert:"+req.Database+"."+req.Collection)
	if err := f.InsertErrs[req.Database][req.Collection]; err != nil {
		return InsertResult{}, err
	}
	return f.InsertResults[req.Database][req.Collection], nil
}

func (f *FakeClient) Flush(ctx context.Context, database, collection string) error {
	_ = ctx
	f.Operations = append(f.Operations, "flush:"+database+"."+collection)
	if err := f.FlushErrs[database][collection]; err != nil {
		return err
	}
	return nil
}

func (f *FakeClient) Query(ctx context.Context, req QueryRequest) (QueryResult, error) {
	_ = ctx
	f.LastQueryRequest = req
	f.Operations = append(f.Operations, "query:"+req.Database+"."+req.Collection)
	if err := f.QueryErrs[req.Database][req.Collection]; err != nil {
		return QueryResult{}, err
	}
	return f.QueryResults[req.Database][req.Collection], nil
}

func (f *FakeClient) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	_ = ctx
	f.LastSearchRequest = req
	f.Operations = append(f.Operations, "search:"+req.Database+"."+req.Collection)
	if err := f.SearchErrs[req.Database][req.Collection]; err != nil {
		return SearchResult{}, err
	}
	return f.SearchResults[req.Database][req.Collection], nil
}

func (f *FakeClient) GetMetrics(ctx context.Context, metricType string) (string, error) {
	_ = ctx
	if err := f.MetricsErrs[metricType]; err != nil {
		return "", err
	}
	return f.MetricsByType[metricType], nil
}

func (f *FakeClient) Close(ctx context.Context) error {
	_ = ctx
	f.Closed = true
	return nil
}

type FakeClientFactory struct {
	Client *FakeClient
	Err    error
}

func (f FakeClientFactory) New(ctx context.Context, cfg Config, timeout time.Duration) (Client, error) {
	_ = ctx
	_ = cfg
	_ = timeout
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Client, nil
}
