package analyzers_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/analyzers"
	"github.com/weiqinzhou3/milvus-health/internal/model"
)

func analysisConfig() *model.Config {
	return &model.Config{
		Cluster: model.ClusterConfig{
			Name: "demo",
			Milvus: model.MilvusConfig{
				URI: "127.0.0.1:19530",
			},
		},
		K8s: model.K8sConfig{Namespace: "milvus"},
		Probe: model.ProbeConfig{
			RW: model.RWProbeConfig{Enabled: false},
		},
	}
}

func TestAnalyzer_ReturnsFailWhenRunnerCapturedMilvusFailure(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:        "demo",
				MilvusURI:   "127.0.0.1:19530",
				Namespace:   "milvus",
				ArchProfile: model.ArchProfileUnknown,
				MQType:      "unknown",
			},
		},
		Checks: []model.CheckResult{
			{Name: "milvus-connectivity", Status: model.CheckStatusFail, Message: "Milvus is unavailable"},
		},
		Failures:  []string{"get milvus version: deadline exceeded"},
		StartedAt: time.Unix(0, 0),
		EndedAt:   time.Unix(0, int64(250*time.Millisecond)),
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultFAIL {
		t.Fatalf("Result = %s, want FAIL", result.Result)
	}
	if result.Confidence != model.ConfidenceLow {
		t.Fatalf("Confidence = %s, want low", result.Confidence)
	}
}

func TestAnalyzer_ReturnsPassForSuccessfulMilvusInventory(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Inventory: model.ClusterInventory{
			Milvus: model.MilvusInventory{
				ServerVersion:   "2.6.1",
				DatabaseCount:   1,
				CollectionCount: 2,
				TotalRowCount:   int64Ptr(30),
				Databases: []model.DatabaseInventory{
					{Name: "default", Collections: []string{"book", "movie"}},
				},
				Collections: []model.CollectionInventory{
					{Database: "default", Name: "book", RowCount: int64Ptr(10)},
					{Database: "default", Name: "movie", RowCount: int64Ptr(20)},
				},
			},
		},
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:          "demo",
				MilvusURI:     "127.0.0.1:19530",
				Namespace:     "milvus",
				MilvusVersion: "2.6.1",
				ArchProfile:   model.ArchProfileV26,
				MQType:        "unknown",
			},
		},
		Checks: []model.CheckResult{
			{Name: "milvus-connectivity", Status: model.CheckStatusPass, Message: "Milvus is reachable"},
			{Name: "milvus-version", Status: model.CheckStatusPass, Message: "Milvus version collected successfully"},
			{Name: "milvus-inventory", Status: model.CheckStatusPass, Message: "Milvus inventory collected successfully"},
		},
		StartedAt: time.Unix(0, 0),
		EndedAt:   time.Unix(0, int64(500*time.Millisecond)),
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultPASS {
		t.Fatalf("Result = %s, want PASS", result.Result)
	}
	if result.Summary.DatabaseCount != 1 || result.Summary.CollectionCount != 2 {
		t.Fatalf("Summary = %#v", result.Summary)
	}
	if result.Summary.TotalRowCount == nil || *result.Summary.TotalRowCount != 30 {
		t.Fatalf("Summary.TotalRowCount = %#v, want 30", result.Summary.TotalRowCount)
	}
	if result.ElapsedMS != 500 {
		t.Fatalf("ElapsedMS = %d, want 500", result.ElapsedMS)
	}
}

func TestAnalyzer_ReturnsWarnWhenWarningsPresent(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:          "demo",
				MilvusURI:     "127.0.0.1:19530",
				Namespace:     "milvus",
				MilvusVersion: "2.4.7",
				ArchProfile:   model.ArchProfileV24,
				MQType:        "unknown",
			},
		},
		Checks: []model.CheckResult{
			{Name: "milvus-connectivity", Status: model.CheckStatusPass, Message: "Milvus is reachable"},
			{Name: "milvus-inventory", Status: model.CheckStatusWarn, Message: "partial inventory"},
		},
		Warnings:  []string{"partial inventory"},
		StartedAt: time.Unix(0, 0),
		EndedAt:   time.Unix(0, int64(100*time.Millisecond)),
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultWARN {
		t.Fatalf("Result = %s, want WARN", result.Result)
	}
	if result.Confidence != model.ConfidenceMedium {
		t.Fatalf("Confidence = %s, want medium", result.Confidence)
	}
	found := false
	for _, check := range result.Checks {
		if check.Status == model.CheckStatusWarn && strings.Contains(check.Message, "partial") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Checks = %#v", result.Checks)
	}
}

func TestAnalyzer_WarnsWhenCollectionRowCountIsPartial(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Inventory: model.ClusterInventory{
			Milvus: model.MilvusInventory{
				DatabaseCount:   1,
				CollectionCount: 2,
				Databases: []model.DatabaseInventory{
					{Name: "default", Collections: []string{"book", "movie"}},
				},
				Collections: []model.CollectionInventory{
					{Database: "default", Name: "book", RowCount: int64Ptr(10)},
					{Database: "default", Name: "movie"},
				},
			},
		},
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:          "demo",
				MilvusURI:     "127.0.0.1:19530",
				Namespace:     "milvus",
				MilvusVersion: "2.6.1",
				ArchProfile:   model.ArchProfileV26,
				MQType:        "unknown",
			},
		},
		Checks: []model.CheckResult{
			{Name: "milvus-connectivity", Status: model.CheckStatusPass, Message: "Milvus is reachable"},
			{Name: "milvus-inventory", Status: model.CheckStatusPass, Message: "Milvus inventory collected successfully"},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultWARN {
		t.Fatalf("Result = %s, want WARN", result.Result)
	}
	if result.Summary.TotalRowCount != nil {
		t.Fatalf("Summary.TotalRowCount = %#v, want nil", result.Summary.TotalRowCount)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "default.movie") {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "milvus-row-count" && check.Status == model.CheckStatusWarn {
			found = true
		}
	}
	if !found {
		t.Fatalf("Checks = %#v", result.Checks)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
