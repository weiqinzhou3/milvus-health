package collectors_test

import (
	"context"
	"errors"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/collectors"
	"github.com/weiqinzhou3/milvus-health/internal/model"
	"github.com/weiqinzhou3/milvus-health/internal/platform"
)

func testConfig() *model.Config {
	return &model.Config{
		Cluster: model.ClusterConfig{
			Name: "demo",
			Milvus: model.MilvusConfig{
				URI: "127.0.0.1:19530",
			},
		},
		K8s: model.K8sConfig{Namespace: "milvus"},
		Probe: model.ProbeConfig{
			Read: model.ReadProbeConfig{MinSuccessTargets: 1},
		},
		TimeoutSec: 1,
	}
}

func TestMilvusInventoryCollector_ReturnsInventory_FromFakeClient(t *testing.T) {
	t.Parallel()

	collector := collectors.DefaultMilvusInventoryCollector{
		Factory: platform.FakeMilvusClientFactory{
			Client: &platform.FakeMilvusClient{
				Version:   "2.6.1",
				Databases: []string{"default"},
				Collections: map[string][]platform.MilvusCollection{
					"default": {{Database: "default", Name: "book", ShardNum: 2, FieldCount: 3}},
				},
			},
		},
	}

	inventory, err := collector.Collect(context.Background(), testConfig())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if !inventory.Reachable || inventory.ServerVersion != "2.6.1" {
		t.Fatalf("Collect() inventory = %#v", inventory)
	}
	if inventory.DatabaseCount != 1 || inventory.CollectionCount != 1 {
		t.Fatalf("Collect() inventory counts = %#v", inventory)
	}
	if len(inventory.Databases) != 1 || inventory.Databases[0].Collections[0] != "book" {
		t.Fatalf("Collect() databases = %#v", inventory.Databases)
	}
}

func TestMilvusInventoryCollector_ReturnsAppError_WhenClientFails(t *testing.T) {
	t.Parallel()

	collector := collectors.DefaultMilvusInventoryCollector{
		Factory: platform.FakeMilvusClientFactory{
			Client: &platform.FakeMilvusClient{
				PingErr: errors.New("dial failed"),
			},
		},
	}

	_, err := collector.Collect(context.Background(), testConfig())
	var appErr *model.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Collect() error = %T, want *model.AppError", err)
	}
	if appErr.Code != model.ErrCodeMilvusConnect {
		t.Fatalf("AppError.Code = %s, want %s", appErr.Code, model.ErrCodeMilvusConnect)
	}
}

func TestMilvusInventoryCollector_HandlesCapabilityDegrade(t *testing.T) {
	t.Parallel()

	collector := collectors.DefaultMilvusInventoryCollector{
		Factory: platform.FakeMilvusClientFactory{
			Client: &platform.FakeMilvusClient{
				Version:      "2.4.7",
				DatabasesErr: platform.ErrCapabilityUnavailable,
				Collections: map[string][]platform.MilvusCollection{
					"default": {{Database: "default", Name: "book"}},
				},
			},
		},
	}

	inventory, err := collector.Collect(context.Background(), testConfig())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if !inventory.CapabilityDegraded {
		t.Fatalf("CapabilityDegraded = false, want true")
	}
	if len(inventory.DatabaseNames) != 1 || inventory.DatabaseNames[0] != "default" {
		t.Fatalf("DatabaseNames = %#v", inventory.DatabaseNames)
	}
}
