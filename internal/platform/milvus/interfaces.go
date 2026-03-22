package milvus

import (
	"context"
	"time"
)

type Config struct {
	Address  string
	Username string
	Password string
	Token    string
}

type Client interface {
	GetVersion(ctx context.Context) (string, error)
	ListDatabases(ctx context.Context) ([]string, error)
	ListCollections(ctx context.Context, database string) ([]string, error)
	GetCollectionID(ctx context.Context, database, collection string) (int64, error)
	GetCollectionRowCount(ctx context.Context, database, collection string) (int64, error)
	DescribeCollection(ctx context.Context, database, collection string) (CollectionDescription, error)
	GetCollectionLoadState(ctx context.Context, database, collection string) (LoadState, error)
	Query(ctx context.Context, req QueryRequest) (QueryResult, error)
	Search(ctx context.Context, req SearchRequest) (SearchResult, error)
	GetMetrics(ctx context.Context, metricType string) (string, error)
	Close(ctx context.Context) error
}

type CollectionDescription struct {
	ID     int64
	Name   string
	Fields []CollectionField
}

type CollectionField struct {
	Name         string
	DataType     string
	Dimension    int64
	IsVector     bool
	IsPrimaryKey bool
}

type LoadState string

const (
	LoadStateUnknown LoadState = "unknown"
	LoadStateLoading LoadState = "loading"
	LoadStateLoaded  LoadState = "loaded"
	LoadStateNotLoad LoadState = "not_load"
)

type QueryRequest struct {
	Database     string
	Collection   string
	Expr         string
	Limit        int
	OutputFields []string
}

type QueryResult struct {
	ResultCount int
}

type SearchRequest struct {
	Database     string
	Collection   string
	Expr         string
	AnnsField    string
	TopK         int
	Vector       []float32
	OutputFields []string
}

type SearchResult struct {
	ResultCount int
}

type ClientFactory interface {
	New(ctx context.Context, cfg Config, timeout time.Duration) (Client, error)
}
