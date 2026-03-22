package milvus

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/model"
	platformmilvus "github.com/weiqinzhou3/milvus-health/internal/platform/milvus"
)

type Collector interface {
	CollectClusterInfo(ctx context.Context, cfg *model.Config) (model.ClusterInfo, error)
	CollectInventory(ctx context.Context, cfg *model.Config) (model.MilvusInventory, error)
}

type DefaultCollector struct {
	Factory platformmilvus.ClientFactory
}

func (c DefaultCollector) CollectClusterInfo(ctx context.Context, cfg *model.Config) (model.ClusterInfo, error) {
	base := clusterInfoFromConfig(cfg)
	if cfg == nil {
		return base, &model.AppError{Code: model.ErrCodeMilvusCollect, Message: "config is nil"}
	}
	client, err := c.newClient(ctx, cfg)
	if err != nil {
		return base, &model.AppError{Code: model.ErrCodeMilvusConnect, Message: fmt.Sprintf("create milvus client: %v", err), Cause: err}
	}
	defer client.Close(ctx)

	version, err := client.GetVersion(ctx)
	if err != nil {
		return base, &model.AppError{Code: model.ErrCodeMilvusConnect, Message: fmt.Sprintf("get milvus version: %v", err), Cause: err}
	}

	base.MilvusVersion = version
	base.ArchProfile = model.DetectArchProfile(version)
	return base, nil
}

func (c DefaultCollector) CollectInventory(ctx context.Context, cfg *model.Config) (model.MilvusInventory, error) {
	if cfg == nil {
		return model.MilvusInventory{}, &model.AppError{Code: model.ErrCodeMilvusCollect, Message: "config is nil"}
	}
	client, err := c.newClient(ctx, cfg)
	if err != nil {
		return model.MilvusInventory{}, &model.AppError{Code: model.ErrCodeMilvusConnect, Message: fmt.Sprintf("create milvus client: %v", err), Cause: err}
	}
	defer client.Close(ctx)

	databases, err := client.ListDatabases(ctx)
	if err != nil {
		return model.MilvusInventory{}, &model.AppError{Code: model.ErrCodeMilvusCollect, Message: fmt.Sprintf("list databases: %v", err), Cause: err}
	}

	inventory := model.MilvusInventory{
		Reachable:     true,
		DatabaseNames: append([]string(nil), databases...),
		Databases:     make([]model.DatabaseInventory, 0, len(databases)),
		Collections:   make([]model.CollectionInventory, 0),
	}

	binlogSizes, totalBinlogSize, binlogErr := c.fetchBinlogSizes(ctx, client)
	if binlogErr != nil {
		inventory.CapabilityDegraded = true
		inventory.DegradedCapabilities = appendUnique(inventory.DegradedCapabilities, "binlog_size")
	} else {
		inventory.TotalBinlogSizeBytes = int64Ptr(totalBinlogSize)
	}

	var totalRowCount int64
	rowCountComplete := true

	for _, database := range databases {
		collections, err := client.ListCollections(ctx, database)
		if err != nil {
			return model.MilvusInventory{}, &model.AppError{
				Code:    model.ErrCodeMilvusCollect,
				Message: fmt.Sprintf("list collections for database %q: %v", database, err),
				Cause:   err,
			}
		}
		inventory.Databases = append(inventory.Databases, model.DatabaseInventory{
			Name:        database,
			Collections: append([]string(nil), collections...),
		})
		for _, collection := range collections {
			collectionInventory := model.CollectionInventory{
				Database: database,
				Name:     collection,
			}

			collectionID, err := client.GetCollectionID(ctx, database, collection)
			if err != nil {
				return model.MilvusInventory{}, &model.AppError{
					Code:    model.ErrCodeMilvusCollect,
					Message: fmt.Sprintf("describe collection %q in database %q: %v", collection, database, err),
					Cause:   err,
				}
			}
			collectionInventory.CollectionID = collectionID

			rowCount, err := client.GetCollectionRowCount(ctx, database, collection)
			if err != nil {
				rowCountComplete = false
				inventory.CapabilityDegraded = true
				inventory.DegradedCapabilities = appendUnique(inventory.DegradedCapabilities, fmt.Sprintf("row_count:%s.%s", database, collection))
			} else {
				collectionInventory.RowCount = int64Ptr(rowCount)
				totalRowCount += rowCount
			}

			if binlogErr == nil {
				if binlogSize, ok := binlogSizes[collectionID]; ok {
					collectionInventory.BinlogSizeBytes = int64Ptr(binlogSize)
				} else {
					inventory.CapabilityDegraded = true
					inventory.DegradedCapabilities = appendUnique(inventory.DegradedCapabilities, fmt.Sprintf("binlog_size:%s.%s", database, collection))
				}
			}

			inventory.Collections = append(inventory.Collections, collectionInventory)
			inventory.CollectionCount++
		}
	}

	inventory.DatabaseCount = len(inventory.Databases)
	if rowCountComplete {
		inventory.TotalRowCount = int64Ptr(totalRowCount)
	}
	return inventory, nil
}

func (c DefaultCollector) fetchBinlogSizes(ctx context.Context, client platformmilvus.Client) (map[int64]int64, int64, error) {
	response, err := client.GetMetrics(ctx, "system_info")
	if err != nil {
		return nil, 0, err
	}

	metrics, err := parseBinlogMetrics(response)
	if err != nil {
		return nil, 0, err
	}
	return metrics.CollectionBinlogSize, metrics.TotalBinlogSize, nil
}

func (c DefaultCollector) newClient(ctx context.Context, cfg *model.Config) (platformmilvus.Client, error) {
	if c.Factory == nil {
		return nil, fmt.Errorf("milvus client factory is nil")
	}
	return c.Factory.New(ctx, platformmilvus.Config{
		Address:  cfg.Cluster.Milvus.URI,
		Username: cfg.Cluster.Milvus.Username,
		Password: cfg.Cluster.Milvus.Password,
		Token:    cfg.Cluster.Milvus.Token,
	}, time.Duration(cfg.TimeoutSec)*time.Second)
}

func clusterInfoFromConfig(cfg *model.Config) model.ClusterInfo {
	if cfg == nil {
		return model.ClusterInfo{
			ArchProfile: model.ArchProfileUnknown,
			MQType:      "unknown",
		}
	}
	return model.ClusterInfo{
		Name:        cfg.Cluster.Name,
		MilvusURI:   cfg.Cluster.Milvus.URI,
		Namespace:   cfg.K8s.Namespace,
		ArchProfile: model.ArchProfileUnknown,
		MQType:      model.NormalizeMQType(cfg.Dependencies.MQ.Type),
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

type binlogMetrics struct {
	TotalBinlogSize      int64
	CollectionBinlogSize map[int64]int64
}

func parseBinlogMetrics(payload string) (*binlogMetrics, error) {
	var root any
	if err := json.Unmarshal([]byte(payload), &root); err != nil {
		return nil, fmt.Errorf("parse system_info metrics json: %w", err)
	}

	metrics, ok := findBinlogMetrics(root)
	if !ok {
		return nil, fmt.Errorf("DataCoordQuotaMetrics not found in system_info")
	}
	return metrics, nil
}

func findBinlogMetrics(node any) (*binlogMetrics, bool) {
	switch value := node.(type) {
	case map[string]any:
		if metrics, ok := parseBinlogMetricsObject(value); ok {
			return metrics, true
		}
		for _, child := range value {
			if metrics, ok := findBinlogMetrics(child); ok {
				return metrics, true
			}
		}
	case []any:
		for _, child := range value {
			if metrics, ok := findBinlogMetrics(child); ok {
				return metrics, true
			}
		}
	}
	return nil, false
}

func parseBinlogMetricsObject(node map[string]any) (*binlogMetrics, bool) {
	quotaMetrics := node
	if nestedQuotaMetrics, ok := getMapAnyAlias(node, "quota_metrics", "QuotaMetrics"); ok {
		quotaMetrics = nestedQuotaMetrics
	}

	totalRaw, hasTotal := getAnyAlias(quotaMetrics, "total_binlog_size", "TotalBinlogSize")
	collectionRaw, hasCollections := getAnyAlias(quotaMetrics, "collection_binlog_size", "CollectionBinlogSize")
	if !hasTotal && !hasCollections {
		return nil, false
	}

	var total int64
	var err error
	if hasTotal {
		total, err = parseJSONInt64(totalRaw)
		if err != nil {
			return nil, false
		}
	}

	collectionMap := make(map[int64]int64)
	if hasCollections {
		items, ok := normalizeStringKeyMap(collectionRaw)
		if !ok {
			return nil, false
		}
		for key, rawValue := range items {
			collectionID, err := strconv.ParseInt(key, 10, 64)
			if err != nil {
				return nil, false
			}
			binlogSize, err := parseJSONInt64(rawValue)
			if err != nil {
				return nil, false
			}
			collectionMap[collectionID] = binlogSize
		}
	}

	return &binlogMetrics{
		TotalBinlogSize:      total,
		CollectionBinlogSize: collectionMap,
	}, true
}

func getAnyAlias(node map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		value, ok := node[key]
		if ok {
			return value, true
		}
	}
	return nil, false
}

func getMapAnyAlias(node map[string]any, keys ...string) (map[string]any, bool) {
	value, ok := getAnyAlias(node, keys...)
	if !ok {
		return nil, false
	}
	mapped, ok := normalizeStringKeyMap(value)
	return mapped, ok
}

func normalizeStringKeyMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	default:
		return nil, false
	}
}

func parseJSONInt64(value any) (int64, error) {
	switch v := value.(type) {
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	case json.Number:
		return v.Int64()
	default:
		return 0, fmt.Errorf("unsupported int64 value type %T", value)
	}
}
