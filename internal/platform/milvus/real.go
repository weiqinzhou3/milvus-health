package milvus

import (
	"context"
	"sort"
	"time"

	milvussdk "github.com/milvus-io/milvus/client/v2/milvusclient"
)

type SDKClientFactory struct{}

func (SDKClientFactory) New(ctx context.Context, cfg Config, timeout time.Duration) (Client, error) {
	connectCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		connectCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	client, err := milvussdk.New(connectCtx, &milvussdk.ClientConfig{
		Address:  cfg.Address,
		Username: cfg.Username,
		Password: cfg.Password,
		APIKey:   cfg.Token,
	})
	if err != nil {
		return nil, err
	}

	return &sdkClient{
		client:      client,
		baseConfig:  cfg,
		callTimeout: timeout,
	}, nil
}

type sdkClient struct {
	client      *milvussdk.Client
	baseConfig  Config
	callTimeout time.Duration
}

func (c *sdkClient) GetVersion(ctx context.Context) (string, error) {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.client.GetServerVersion(callCtx, milvussdk.NewGetServerVersionOption())
}

func (c *sdkClient) ListDatabases(ctx context.Context) ([]string, error) {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()

	databases, err := c.client.ListDatabase(callCtx, milvussdk.NewListDatabaseOption())
	if err != nil {
		return nil, err
	}
	sort.Strings(databases)
	return databases, nil
}

func (c *sdkClient) ListCollections(ctx context.Context, database string) ([]string, error) {
	client := c.client
	closer := func(context.Context) error { return nil }

	if database != "" {
		scopedClient, err := c.newScopedClient(ctx, database)
		if err != nil {
			return nil, err
		}
		client = scopedClient
		closer = scopedClient.Close
	}
	defer closer(ctx)

	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()

	collections, err := client.ListCollections(callCtx, milvussdk.NewListCollectionOption())
	if err != nil {
		return nil, err
	}
	sort.Strings(collections)
	return collections, nil
}

func (c *sdkClient) Close(ctx context.Context) error {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()
	return c.client.Close(callCtx)
}

func (c *sdkClient) newScopedClient(ctx context.Context, database string) (*milvussdk.Client, error) {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()

	return milvussdk.New(callCtx, &milvussdk.ClientConfig{
		Address:  c.baseConfig.Address,
		Username: c.baseConfig.Username,
		Password: c.baseConfig.Password,
		APIKey:   c.baseConfig.Token,
		DBName:   database,
	})
}

func (c *sdkClient) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.callTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, c.callTimeout)
}
