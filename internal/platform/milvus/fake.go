package milvus

import (
	"context"
	"time"
)

type FakeClient struct {
	Version          string
	VersionErr       error
	Databases        []string
	DatabasesErr     error
	Collections      map[string][]string
	CollectionErrs   map[string]error
	CollectionIDs    map[string]map[string]int64
	CollectionIDErrs map[string]map[string]error
	RowCounts        map[string]map[string]int64
	RowCountErrs     map[string]map[string]error
	MetricsByType    map[string]string
	MetricsErrs      map[string]error
	Closed           bool
}

func (f *FakeClient) GetVersion(ctx context.Context) (string, error) {
	_ = ctx
	return f.Version, f.VersionErr
}

func (f *FakeClient) ListDatabases(ctx context.Context) ([]string, error) {
	_ = ctx
	return append([]string(nil), f.Databases...), f.DatabasesErr
}

func (f *FakeClient) ListCollections(ctx context.Context, database string) ([]string, error) {
	_ = ctx
	if err := f.CollectionErrs[database]; err != nil {
		return nil, err
	}
	return append([]string(nil), f.Collections[database]...), nil
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
