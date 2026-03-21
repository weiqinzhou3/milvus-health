package milvus_test

import (
	"context"
	"errors"
	"testing"

	collectormilvus "github.com/weiqinzhou3/milvus-health/internal/collectors/milvus"
	"github.com/weiqinzhou3/milvus-health/internal/model"
	platformmilvus "github.com/weiqinzhou3/milvus-health/internal/platform/milvus"
)

func testConfig() *model.Config {
	return &model.Config{
		Cluster: model.ClusterConfig{
			Name: "demo",
			Milvus: model.MilvusConfig{
				URI: "127.0.0.1:19530",
			},
		},
		K8s:        model.K8sConfig{Namespace: "milvus"},
		TimeoutSec: 5,
	}
}

func TestCollector_CollectClusterInfo(t *testing.T) {
	t.Parallel()

	collector := collectormilvus.DefaultCollector{
		Factory: platformmilvus.FakeClientFactory{
			Client: &platformmilvus.FakeClient{Version: "2.6.3"},
		},
	}

	info, err := collector.CollectClusterInfo(context.Background(), testConfig())
	if err != nil {
		t.Fatalf("CollectClusterInfo() error = %v", err)
	}
	if info.MilvusVersion != "2.6.3" {
		t.Fatalf("MilvusVersion = %q, want 2.6.3", info.MilvusVersion)
	}
	if info.ArchProfile != model.ArchProfileV26 {
		t.Fatalf("ArchProfile = %q, want %q", info.ArchProfile, model.ArchProfileV26)
	}
}

func TestCollector_CollectInventory(t *testing.T) {
	t.Parallel()

	collector := collectormilvus.DefaultCollector{
		Factory: platformmilvus.FakeClientFactory{
			Client: &platformmilvus.FakeClient{
				Databases: []string{"analytics", "default"},
				Collections: map[string][]string{
					"analytics": {"events"},
					"default":   {"book", "movie"},
				},
			},
		},
	}

	inventory, err := collector.CollectInventory(context.Background(), testConfig())
	if err != nil {
		t.Fatalf("CollectInventory() error = %v", err)
	}
	if inventory.DatabaseCount != 2 || inventory.CollectionCount != 3 {
		t.Fatalf("inventory counts = %#v", inventory)
	}
	if len(inventory.Databases) != 2 || inventory.Databases[1].Collections[1] != "movie" {
		t.Fatalf("Databases = %#v", inventory.Databases)
	}
}

func TestCollector_CollectClusterInfoReturnsAppErrorOnConnectFailure(t *testing.T) {
	t.Parallel()

	collector := collectormilvus.DefaultCollector{
		Factory: platformmilvus.FakeClientFactory{Err: errors.New("dial failed")},
	}

	_, err := collector.CollectClusterInfo(context.Background(), testConfig())
	var appErr *model.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("CollectClusterInfo() error = %T, want *model.AppError", err)
	}
	if appErr.Code != model.ErrCodeMilvusConnect {
		t.Fatalf("AppError.Code = %q, want %q", appErr.Code, model.ErrCodeMilvusConnect)
	}
}
