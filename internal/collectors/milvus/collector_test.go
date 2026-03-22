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

func TestCollector_CollectClusterInfo_PreservesConfiguredMQType(t *testing.T) {
	t.Parallel()

	cfg := testConfig()
	cfg.Dependencies.MQ.Type = "rocksmq"
	collector := collectormilvus.DefaultCollector{
		Factory: platformmilvus.FakeClientFactory{
			Client: &platformmilvus.FakeClient{Version: "v2.4.7"},
		},
	}

	info, err := collector.CollectClusterInfo(context.Background(), cfg)
	if err != nil {
		t.Fatalf("CollectClusterInfo() error = %v", err)
	}
	if info.ArchProfile != model.ArchProfileV24 {
		t.Fatalf("ArchProfile = %q, want %q", info.ArchProfile, model.ArchProfileV24)
	}
	if info.MQType != "rocksmq" {
		t.Fatalf("MQType = %q, want rocksmq", info.MQType)
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
				RowCounts: map[string]map[string]int64{
					"analytics": {"events": 7},
					"default":   {"book": 10, "movie": 11},
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
	if inventory.TotalRowCount == nil || *inventory.TotalRowCount != 28 {
		t.Fatalf("TotalRowCount = %#v, want 28", inventory.TotalRowCount)
	}
	if len(inventory.Collections) != 3 {
		t.Fatalf("Collections = %#v", inventory.Collections)
	}
	if inventory.Collections[0].RowCount == nil || *inventory.Collections[0].RowCount != 7 {
		t.Fatalf("first collection row count = %#v", inventory.Collections[0].RowCount)
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

func TestCollector_CollectInventory_DegradesWhenCollectionRowCountUnavailable(t *testing.T) {
	t.Parallel()

	collector := collectormilvus.DefaultCollector{
		Factory: platformmilvus.FakeClientFactory{
			Client: &platformmilvus.FakeClient{
				Databases: []string{"default"},
				Collections: map[string][]string{
					"default": {"book", "movie"},
				},
				RowCounts: map[string]map[string]int64{
					"default": {"book": 10},
				},
				RowCountErrs: map[string]map[string]error{
					"default": {"movie": errors.New("stats unavailable")},
				},
			},
		},
	}

	inventory, err := collector.CollectInventory(context.Background(), testConfig())
	if err != nil {
		t.Fatalf("CollectInventory() error = %v", err)
	}
	if !inventory.CapabilityDegraded {
		t.Fatal("CapabilityDegraded = false, want true")
	}
	if inventory.TotalRowCount != nil {
		t.Fatalf("TotalRowCount = %#v, want nil", inventory.TotalRowCount)
	}
	if len(inventory.DegradedCapabilities) != 1 || inventory.DegradedCapabilities[0] != "row_count:default.movie" {
		t.Fatalf("DegradedCapabilities = %#v", inventory.DegradedCapabilities)
	}
	if inventory.Collections[1].RowCount != nil {
		t.Fatalf("missing row count should render as nil, got %#v", inventory.Collections[1].RowCount)
	}
}
