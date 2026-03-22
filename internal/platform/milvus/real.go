package milvus

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/milvus-io/milvus-proto/go-api/v2/commonpb"
	"github.com/milvus-io/milvus-proto/go-api/v2/milvuspb"
	"github.com/milvus-io/milvus/client/v2/entity"
	milvussdk "github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/milvus-io/milvus/pkg/util/commonpbutil"
	"github.com/milvus-io/milvus/pkg/util/crypto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
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

func (c *sdkClient) GetCollectionRowCount(ctx context.Context, database, collection string) (int64, error) {
	client := c.client
	closer := func(context.Context) error { return nil }

	if database != "" {
		scopedClient, err := c.newScopedClient(ctx, database)
		if err != nil {
			return 0, err
		}
		client = scopedClient
		closer = scopedClient.Close
	}
	defer closer(ctx)

	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()

	stats, err := client.GetCollectionStats(callCtx, milvussdk.NewGetCollectionStatsOption(collection))
	if err != nil {
		return 0, err
	}

	rowCount, err := parseCollectionRowCount(stats)
	if err != nil {
		return 0, fmt.Errorf("parse row_count for collection %q: %w", collection, err)
	}
	return rowCount, nil
}

func (c *sdkClient) GetCollectionID(ctx context.Context, database, collection string) (int64, error) {
	description, err := c.DescribeCollection(ctx, database, collection)
	if err != nil {
		return 0, err
	}
	return description.ID, nil
}

func (c *sdkClient) DescribeCollection(ctx context.Context, database, collection string) (CollectionDescription, error) {
	client := c.client
	closer := func(context.Context) error { return nil }

	if database != "" {
		scopedClient, err := c.newScopedClient(ctx, database)
		if err != nil {
			return CollectionDescription{}, err
		}
		client = scopedClient
		closer = scopedClient.Close
	}
	defer closer(ctx)

	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()

	description, err := client.DescribeCollection(callCtx, milvussdk.NewDescribeCollectionOption(collection))
	if err != nil {
		return CollectionDescription{}, err
	}

	result := CollectionDescription{
		ID:   description.ID,
		Name: description.Name,
	}
	for _, field := range description.Schema.Fields {
		item := CollectionField{
			Name:         field.Name,
			DataType:     field.DataType.Name(),
			IsPrimaryKey: field.PrimaryKey,
		}
		switch field.DataType {
		case entity.FieldTypeFloatVector, entity.FieldTypeBinaryVector, entity.FieldTypeFloat16Vector, entity.FieldTypeBFloat16Vector, entity.FieldTypeSparseVector:
			item.IsVector = true
			dim, err := field.GetDim()
			if err == nil {
				item.Dimension = dim
			}
		}
		result.Fields = append(result.Fields, item)
	}
	return result, nil
}

func (c *sdkClient) GetCollectionLoadState(ctx context.Context, database, collection string) (LoadState, error) {
	client := c.client
	closer := func(context.Context) error { return nil }

	if database != "" {
		scopedClient, err := c.newScopedClient(ctx, database)
		if err != nil {
			return LoadStateUnknown, err
		}
		client = scopedClient
		closer = scopedClient.Close
	}
	defer closer(ctx)

	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()

	state, err := client.GetLoadState(callCtx, milvussdk.NewGetLoadStateOption(collection))
	if err != nil {
		return LoadStateUnknown, err
	}

	switch state.State {
	case entity.LoadStateLoaded:
		return LoadStateLoaded, nil
	case entity.LoadStateLoading:
		return LoadStateLoading, nil
	case entity.LoadStateNotLoad, entity.LoadStateUnloading:
		return LoadStateNotLoad, nil
	default:
		return LoadStateUnknown, nil
	}
}

func (c *sdkClient) Query(ctx context.Context, req QueryRequest) (QueryResult, error) {
	client := c.client
	closer := func(context.Context) error { return nil }

	if req.Database != "" {
		scopedClient, err := c.newScopedClient(ctx, req.Database)
		if err != nil {
			return QueryResult{}, err
		}
		client = scopedClient
		closer = scopedClient.Close
	}
	defer closer(ctx)

	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()

	option := milvussdk.NewQueryOption(req.Collection).
		WithFilter(req.Expr).
		WithLimit(req.Limit).
		WithOutputFields(req.OutputFields...)
	result, err := client.Query(callCtx, option)
	if err != nil {
		return QueryResult{}, err
	}
	return QueryResult{ResultCount: result.ResultCount}, nil
}

func (c *sdkClient) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	client := c.client
	closer := func(context.Context) error { return nil }

	if req.Database != "" {
		scopedClient, err := c.newScopedClient(ctx, req.Database)
		if err != nil {
			return SearchResult{}, err
		}
		client = scopedClient
		closer = scopedClient.Close
	}
	defer closer(ctx)

	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()

	option := milvussdk.NewSearchOption(req.Collection, req.TopK, []entity.Vector{entity.FloatVector(req.Vector)}).
		WithANNSField(req.AnnsField).
		WithFilter(req.Expr).
		WithOutputFields(req.OutputFields...)
	resultSets, err := client.Search(callCtx, option)
	if err != nil {
		return SearchResult{}, err
	}

	count := 0
	if len(resultSets) > 0 {
		count = resultSets[0].ResultCount
	}
	return SearchResult{ResultCount: count}, nil
}

func (c *sdkClient) GetMetrics(ctx context.Context, metricType string) (string, error) {
	callCtx, cancel := c.withTimeout(ctx)
	defer cancel()

	rawClient, err := newRawServiceClient(callCtx, c.baseConfig)
	if err != nil {
		return "", err
	}
	defer rawClient.Close()

	return rawClient.GetMetrics(callCtx, metricType)
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

func parseCollectionRowCount(stats map[string]string) (int64, error) {
	value, ok := stats["row_count"]
	if !ok {
		return 0, fmt.Errorf("row_count missing from collection statistics")
	}

	rowCount, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid row_count %q: %w", value, err)
	}
	if rowCount < 0 {
		return 0, fmt.Errorf("row_count must be non-negative, got %d", rowCount)
	}
	return rowCount, nil
}

type rawServiceClient struct {
	conn       *grpc.ClientConn
	service    milvuspb.MilvusServiceClient
	identifier string
	authHeader string
}

func newRawServiceClient(ctx context.Context, cfg Config) (*rawServiceClient, error) {
	conn, err := grpc.DialContext(
		ctx,
		cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(math.MaxInt32)),
	)
	if err != nil {
		return nil, err
	}

	client := &rawServiceClient{
		conn:    conn,
		service: milvuspb.NewMilvusServiceClient(conn),
	}
	if cfg.Token != "" {
		client.authHeader = crypto.Base64Encode(cfg.Token)
	} else if cfg.Username != "" || cfg.Password != "" {
		client.authHeader = crypto.Base64Encode(fmt.Sprintf("%s:%s", cfg.Username, cfg.Password))
	}

	connectCtx := client.appendMetadata(ctx)
	resp, err := client.service.Connect(connectCtx, &milvuspb.ConnectRequest{
		ClientInfo: &commonpb.ClientInfo{
			SdkType:    "milvus-health",
			SdkVersion: "dev",
			LocalTime:  time.Now().Format(time.RFC3339),
			User:       cfg.Username,
		},
	})
	if err != nil {
		conn.Close()
		return nil, err
	}
	if status := resp.GetStatus(); status != nil && status.GetErrorCode() != commonpb.ErrorCode_Success {
		conn.Close()
		return nil, fmt.Errorf("connect for metrics failed: %s", status.GetReason())
	}
	client.identifier = strconv.FormatInt(resp.GetIdentifier(), 10)
	return client, nil
}

func (c *rawServiceClient) GetMetrics(ctx context.Context, metricType string) (string, error) {
	req, err := constructMetricsRequest(metricType)
	if err != nil {
		return "", err
	}

	resp, err := c.service.GetMetrics(c.appendMetadata(ctx), req)
	if err != nil {
		return "", err
	}
	if status := resp.GetStatus(); status != nil && status.GetErrorCode() != commonpb.ErrorCode_Success {
		return "", fmt.Errorf("get metrics failed: %s", status.GetReason())
	}
	return resp.GetResponse(), nil
}

func (c *rawServiceClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *rawServiceClient) appendMetadata(ctx context.Context) context.Context {
	if c.authHeader != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", c.authHeader)
	}
	if c.identifier != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "identifier", c.identifier)
	}
	return ctx
}

func constructMetricsRequest(metricType string) (*milvuspb.GetMetricsRequest, error) {
	payload, err := json.Marshal(map[string]string{"metric_type": metricType})
	if err != nil {
		return nil, err
	}
	return &milvuspb.GetMetricsRequest{
		Base: commonpbutil.NewMsgBase(
			commonpbutil.WithMsgType(commonpb.MsgType_SystemInfo),
		),
		Request: string(payload),
	}, nil
}
