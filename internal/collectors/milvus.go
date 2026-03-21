package collectors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/model"
	"github.com/weiqinzhou3/milvus-health/internal/platform"
)

type DefaultMilvusInventoryCollector struct {
	Factory platform.MilvusClientFactory
}

func (c DefaultMilvusInventoryCollector) Collect(ctx context.Context, cfg *model.Config) (model.MilvusInventory, error) {
	if cfg == nil {
		return model.MilvusInventory{}, &model.AppError{Code: model.ErrCodeMilvusCollect, Message: "config is nil"}
	}
	if c.Factory == nil {
		return model.MilvusInventory{}, &model.AppError{Code: model.ErrCodeMilvusCollect, Message: "milvus client factory is nil"}
	}

	client, err := c.Factory.New(ctx, cfg.Cluster.Milvus, time.Duration(cfg.TimeoutSec)*time.Second)
	if err != nil {
		return model.MilvusInventory{}, &model.AppError{Code: model.ErrCodeMilvusConnect, Message: fmt.Sprintf("create milvus client: %v", err), Cause: err}
	}
	defer client.Close(ctx)

	if err := client.Ping(ctx); err != nil {
		return model.MilvusInventory{}, &model.AppError{Code: model.ErrCodeMilvusConnect, Message: fmt.Sprintf("ping milvus: %v", err), Cause: err}
	}

	version, err := client.GetServerVersion(ctx)
	if err != nil {
		return model.MilvusInventory{}, &model.AppError{Code: model.ErrCodeMilvusCollect, Message: fmt.Sprintf("get milvus version: %v", err), Cause: err}
	}

	inventory := model.MilvusInventory{
		Reachable:     true,
		ServerVersion: version,
	}

	databases, err := client.ListDatabases(ctx)
	switch {
	case err == nil:
		inventory.DatabaseNames = append([]string(nil), databases...)
	case errors.Is(err, platform.ErrCapabilityUnavailable):
		inventory.CapabilityDegraded = true
		inventory.DegradedCapabilities = append(inventory.DegradedCapabilities, "list_databases")
		databases = []string{"default"}
		inventory.DatabaseNames = append([]string(nil), databases...)
	default:
		return model.MilvusInventory{}, &model.AppError{Code: model.ErrCodeMilvusCollect, Message: fmt.Sprintf("list databases: %v", err), Cause: err}
	}

	if len(databases) == 0 {
		databases = []string{"default"}
	}

	for _, database := range databases {
		collections, err := client.ListCollections(ctx, database)
		if err != nil {
			return model.MilvusInventory{}, &model.AppError{Code: model.ErrCodeMilvusCollect, Message: fmt.Sprintf("list collections for database %q: %v", database, err), Cause: err}
		}
		if !contains(inventory.DatabaseNames, database) {
			inventory.DatabaseNames = append(inventory.DatabaseNames, database)
		}
		databaseInventory := model.DatabaseInventory{Name: database}
		for _, collection := range collections {
			databaseInventory.Collections = append(databaseInventory.Collections, collection.Name)
			inventory.Collections = append(inventory.Collections, model.CollectionInventory{
				Database:   collection.Database,
				Name:       collection.Name,
				ShardNum:   collection.ShardNum,
				FieldCount: collection.FieldCount,
			})
		}
		inventory.Databases = append(inventory.Databases, databaseInventory)
	}
	inventory.DatabaseCount = len(inventory.Databases)
	inventory.CollectionCount = len(inventory.Collections)

	return inventory, nil
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
