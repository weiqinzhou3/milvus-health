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
	GetCollectionRowCount(ctx context.Context, database, collection string) (int64, error)
	Close(ctx context.Context) error
}

type ClientFactory interface {
	New(ctx context.Context, cfg Config, timeout time.Duration) (Client, error)
}
