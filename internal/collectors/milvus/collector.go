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
	}

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
		inventory.CollectionCount += len(collections)
	}

	inventory.DatabaseCount = len(inventory.Databases)
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
