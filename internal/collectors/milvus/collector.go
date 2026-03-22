package milvus

import (
	"context"
	"fmt"
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

			rowCount, err := client.GetCollectionRowCount(ctx, database, collection)
			if err != nil {
				rowCountComplete = false
				inventory.CapabilityDegraded = true
				inventory.DegradedCapabilities = appendUnique(inventory.DegradedCapabilities, fmt.Sprintf("row_count:%s.%s", database, collection))
			} else {
				collectionInventory.RowCount = int64Ptr(rowCount)
				totalRowCount += rowCount
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
		MQType:      "unknown",
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
