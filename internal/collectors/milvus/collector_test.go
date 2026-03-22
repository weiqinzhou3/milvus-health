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
				CollectionIDs: map[string]map[string]int64{
					"analytics": {"events": 1001},
					"default":   {"book": 1002, "movie": 1003},
				},
				RowCounts: map[string]map[string]int64{
					"analytics": {"events": 7},
					"default":   {"book": 10, "movie": 11},
				},
				MetricsByType: map[string]string{
					"system_info": `{"quota_metrics":{"total_binlog_size":2800,"collection_binlog_size":{"1001":700,"1002":1000,"1003":1100}}}`,
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
	if inventory.TotalBinlogSizeBytes == nil || *inventory.TotalBinlogSizeBytes != 2800 {
		t.Fatalf("TotalBinlogSizeBytes = %#v, want 2800", inventory.TotalBinlogSizeBytes)
	}
	if len(inventory.Collections) != 3 {
		t.Fatalf("Collections = %#v", inventory.Collections)
	}
	if inventory.Collections[0].RowCount == nil || *inventory.Collections[0].RowCount != 7 {
		t.Fatalf("first collection row count = %#v", inventory.Collections[0].RowCount)
	}
	if inventory.Collections[0].CollectionID != 1001 {
		t.Fatalf("first collection collection_id = %d, want 1001", inventory.Collections[0].CollectionID)
	}
	if inventory.Collections[0].BinlogSizeBytes == nil || *inventory.Collections[0].BinlogSizeBytes != 700 {
		t.Fatalf("first collection binlog size = %#v, want 700", inventory.Collections[0].BinlogSizeBytes)
	}
}

func TestCollector_CollectInventory_ParsesReal247StyleBinlogMetrics(t *testing.T) {
	t.Parallel()

	collector := collectormilvus.DefaultCollector{
		Factory: platformmilvus.FakeClientFactory{
			Client: &platformmilvus.FakeClient{
				Databases: []string{"default"},
				Collections: map[string][]string{
					"default": {"book", "movie"},
				},
				CollectionIDs: map[string]map[string]int64{
					"default": {"book": 451866866319598777, "movie": 451866866319598778},
				},
				RowCounts: map[string]map[string]int64{
					"default": {"book": 10, "movie": 11},
				},
				MetricsByType: map[string]string{
					"system_info": `{
						"nodes_info":[
							{
								"infos":{
									"quota_metrics":{
										"TotalBinlogSize":"4509715660",
										"CollectionBinlogSize":{
											"451866866319598777":"2254857830",
											"451866866319598778":"2254857830"
										}
									}
								}
							}
						]
					}`,
				},
			},
		},
	}

	inventory, err := collector.CollectInventory(context.Background(), testConfig())
	if err != nil {
		t.Fatalf("CollectInventory() error = %v", err)
	}
	if inventory.TotalBinlogSizeBytes == nil || *inventory.TotalBinlogSizeBytes != 4509715660 {
		t.Fatalf("TotalBinlogSizeBytes = %#v, want 4509715660", inventory.TotalBinlogSizeBytes)
	}
	if inventory.Collections[0].BinlogSizeBytes == nil || *inventory.Collections[0].BinlogSizeBytes != 2254857830 {
		t.Fatalf("Collections[0].BinlogSizeBytes = %#v, want 2254857830", inventory.Collections[0].BinlogSizeBytes)
	}
	if inventory.Collections[1].BinlogSizeBytes == nil || *inventory.Collections[1].BinlogSizeBytes != 2254857830 {
		t.Fatalf("Collections[1].BinlogSizeBytes = %#v, want 2254857830", inventory.Collections[1].BinlogSizeBytes)
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
				CollectionIDs: map[string]map[string]int64{
					"default": {"book": 1001, "movie": 1002},
				},
				RowCounts: map[string]map[string]int64{
					"default": {"book": 10},
				},
				RowCountErrs: map[string]map[string]error{
					"default": {"movie": errors.New("stats unavailable")},
				},
				MetricsByType: map[string]string{
					"system_info": `{"quota_metrics":{"total_binlog_size":1000,"collection_binlog_size":{"1001":100}}}`,
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
	if len(inventory.DegradedCapabilities) != 2 || inventory.DegradedCapabilities[0] != "row_count:default.movie" {
		t.Fatalf("DegradedCapabilities = %#v", inventory.DegradedCapabilities)
	}
	if inventory.Collections[1].RowCount != nil {
		t.Fatalf("missing row count should render as nil, got %#v", inventory.Collections[1].RowCount)
	}
	if inventory.Collections[1].BinlogSizeBytes != nil {
		t.Fatalf("missing binlog size should render as nil, got %#v", inventory.Collections[1].BinlogSizeBytes)
	}
	if got, want := inventory.DegradedCapabilities[1], "binlog_size:default.movie"; got != want {
		t.Fatalf("DegradedCapabilities[1] = %q, want %q", got, want)
	}
}

func TestCollector_CollectInventory_DegradesWhenBinlogMetricsUnavailable(t *testing.T) {
	t.Parallel()

	collector := collectormilvus.DefaultCollector{
		Factory: platformmilvus.FakeClientFactory{
			Client: &platformmilvus.FakeClient{
				Databases: []string{"default"},
				Collections: map[string][]string{
					"default": {"book"},
				},
				CollectionIDs: map[string]map[string]int64{
					"default": {"book": 1001},
				},
				RowCounts: map[string]map[string]int64{
					"default": {"book": 10},
				},
				MetricsErrs: map[string]error{
					"system_info": errors.New("metrics unavailable"),
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
	if inventory.TotalBinlogSizeBytes != nil {
		t.Fatalf("TotalBinlogSizeBytes = %#v, want nil", inventory.TotalBinlogSizeBytes)
	}
	if len(inventory.DegradedCapabilities) != 1 || inventory.DegradedCapabilities[0] != "binlog_size" {
		t.Fatalf("DegradedCapabilities = %#v", inventory.DegradedCapabilities)
	}
	if inventory.Collections[0].BinlogSizeBytes != nil {
		t.Fatalf("BinlogSizeBytes = %#v, want nil", inventory.Collections[0].BinlogSizeBytes)
	}
	if inventory.Collections[0].RowCount == nil || *inventory.Collections[0].RowCount != 10 {
		t.Fatalf("RowCount = %#v, want 10", inventory.Collections[0].RowCount)
	}
}
