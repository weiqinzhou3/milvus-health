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
		Rules: model.RulesConfig{
			ResourceWarnRatio: 0.85,
		},
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
				ServerVersion:        "2.6.1",
				DatabaseCount:        1,
				CollectionCount:      2,
				TotalRowCount:        int64Ptr(30),
				TotalBinlogSizeBytes: int64Ptr(3000),
				Databases: []model.DatabaseInventory{
					{Name: "default", Collections: []string{"book", "movie"}},
				},
				Collections: []model.CollectionInventory{
					{Database: "default", Name: "book", RowCount: int64Ptr(10), BinlogSizeBytes: int64Ptr(1000)},
					{Database: "default", Name: "movie", RowCount: int64Ptr(20), BinlogSizeBytes: int64Ptr(2000)},
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
	if result.Summary.ServiceCount != 0 || result.Summary.EndpointCount != 0 {
		t.Fatalf("K8s Summary = %#v", result.Summary)
	}
	if result.Summary.TotalRowCount == nil || *result.Summary.TotalRowCount != 30 {
		t.Fatalf("Summary.TotalRowCount = %#v, want 30", result.Summary.TotalRowCount)
	}
	if result.Summary.TotalBinlogSizeBytes == nil || *result.Summary.TotalBinlogSizeBytes != 3000 {
		t.Fatalf("Summary.TotalBinlogSizeBytes = %#v, want 3000", result.Summary.TotalBinlogSizeBytes)
	}
	if result.ElapsedMS != 500 {
		t.Fatalf("ElapsedMS = %d, want 500", result.ElapsedMS)
	}
}

func TestAnalyzer_AddsK8sWarningsForNotReadyAndRestartedPods(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Inventory: model.ClusterInventory{
			K8s: model.K8sInventory{
				Namespace:        "milvus",
				TotalPodCount:    2,
				ReadyPodCount:    1,
				NotReadyPodCount: 1,
				Pods: []model.PodStatusSummary{
					{Name: "proxy-0", Phase: "Running", Ready: false, RestartCount: 2},
					{Name: "querynode-0", Phase: "Running", Ready: true, RestartCount: 0},
				},
				Services: []model.ServiceInventory{
					{Name: "milvus", Type: "ClusterIP", Ports: []string{"19530/tcp"}},
				},
				Endpoints: []model.EndpointInventory{
					{Name: "milvus-abc", Addresses: []string{"10.0.0.1"}},
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
			{Name: "k8s-collection", Status: model.CheckStatusPass, Message: "Kubernetes inventory collected successfully"},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultWARN {
		t.Fatalf("Result = %s, want WARN", result.Result)
	}
	if result.Summary.PodCount != 2 || result.Summary.ReadyPodCount != 1 || result.Summary.NotReadyPodCount != 1 || result.Summary.ServiceCount != 1 || result.Summary.EndpointCount != 1 {
		t.Fatalf("Summary = %#v", result.Summary)
	}
	if len(result.Warnings) != 2 {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
	foundReadiness := false
	foundRestarts := false
	for _, check := range result.Checks {
		if check.Name == "k8s-pod-readiness" && check.Status == model.CheckStatusWarn {
			foundReadiness = true
		}
		if check.Name == "k8s-pod-restarts" && check.Status == model.CheckStatusWarn {
			foundRestarts = true
		}
	}
	if !foundReadiness || !foundRestarts {
		t.Fatalf("Checks = %#v", result.Checks)
	}
}

func TestAnalyzer_SkipsK8sPodHealthWhenArchUnknown(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Inventory: model.ClusterInventory{
			K8s: model.K8sInventory{
				Namespace:     "milvus",
				TotalPodCount: 1,
				Pods: []model.PodStatusSummary{
					{Name: "proxy-0", Phase: "Running", Ready: false, RestartCount: 1},
				},
			},
		},
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:        "demo",
				MilvusURI:   "127.0.0.1:19530",
				Namespace:   "milvus",
				ArchProfile: model.ArchProfileUnknown,
				MQType:      "unknown",
			},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultPASS {
		t.Fatalf("Result = %s, want PASS", result.Result)
	}
	foundSkip := false
	for _, check := range result.Checks {
		if check.Name == "k8s-pod-health" && check.Status == model.CheckStatusSkip {
			foundSkip = true
		}
	}
	if !foundSkip {
		t.Fatalf("Checks = %#v", result.Checks)
	}
}

func TestAnalyzer_WarnsWhenResourceUsageUnavailable(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Inventory: model.ClusterInventory{
			K8s: model.K8sInventory{
				Namespace:                 "milvus",
				TotalPodCount:             2,
				ReadyPodCount:             2,
				ResourceUsageAvailable:    false,
				ResourceUnavailableReason: model.MetricsUnavailableReasonNotFound,
				Pods: []model.PodStatusSummary{
					{Name: "proxy-0", Phase: "Running", Ready: true},
					{Name: "querynode-0", Phase: "Running", Ready: true},
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
			},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultWARN {
		t.Fatalf("Result = %s, want WARN", result.Result)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "k8s-resource-usage" && check.Status == model.CheckStatusWarn {
			found = true
		}
	}
	if !found {
		t.Fatalf("Checks = %#v", result.Checks)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "metrics-server not found") {
		t.Fatalf("Warnings = %#v", result.Warnings)
	}
}

func TestAnalyzer_WarnsWhenResourceUsageIsPartial(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Inventory: model.ClusterInventory{
			K8s: model.K8sInventory{
				Namespace:                "milvus",
				TotalPodCount:            2,
				ReadyPodCount:            2,
				ResourceUsageAvailable:   true,
				ResourceUsagePartial:     true,
				MetricsAvailablePodCount: 1,
				Pods: []model.PodStatusSummary{
					{Name: "proxy-0", Phase: "Running", Ready: true, CPULimitRatio: float64Ptr(0.9)},
					{Name: "querynode-0", Phase: "Running", Ready: true},
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
			},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultWARN {
		t.Fatalf("Result = %s, want WARN", result.Result)
	}
	foundPartial := false
	foundRatio := false
	for _, check := range result.Checks {
		if check.Name == "k8s-resource-usage" && check.Status == model.CheckStatusWarn {
			foundPartial = true
		}
		if check.Name == "k8s-resource-ratio" && check.Status == model.CheckStatusWarn {
			foundRatio = true
		}
	}
	if !foundPartial || !foundRatio {
		t.Fatalf("Checks = %#v", result.Checks)
	}
}

func TestAnalyzer_ReturnsWarnWhenWarningsPresent(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Checks: []model.CheckResult{
			{Name: "milvus-connectivity", Status: model.CheckStatusPass, Message: "Milvus is reachable"},
			{Name: "milvus-inventory", Status: model.CheckStatusWarn, Message: "partial inventory"},
		},
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:          "demo",
				MilvusURI:     "127.0.0.1:19530",
				Namespace:     "milvus",
				MilvusVersion: "2.4.7",
				ArchProfile:   model.ArchProfileV24,
				MQType:        "unknown",
			},
			RWProbe: model.RWProbeResult{
				Status:         model.CheckStatusPass,
				Enabled:        true,
				InsertRows:     3,
				VectorDim:      4,
				CleanupEnabled: true,
				Message:        "rw probe completed successfully",
			},
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

func TestAnalyzer_AddsRWProbeFailureEvidence(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:          "demo",
				MilvusURI:     "127.0.0.1:19530",
				Namespace:     "milvus",
				MilvusVersion: "2.6.1",
				ArchProfile:   model.ArchProfileV26,
				MQType:        "unknown",
			},
			RWProbe: model.RWProbeResult{
				Status:          model.CheckStatusFail,
				Enabled:         true,
				InsertRows:      3,
				VectorDim:       4,
				CleanupEnabled:  true,
				CleanupExecuted: true,
				Message:         "query failed: timeout; cleanup failed: drop test collection: forbidden",
				StepResults: []model.ProbeStepResult{
					{Name: "query", Success: false, Error: "timeout"},
					{Name: "cleanup", Success: false, Error: "drop test collection: forbidden"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultFAIL {
		t.Fatalf("Result = %s, want FAIL", result.Result)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name != "rw-probe" {
			continue
		}
		if check.Status != model.CheckStatusFail {
			t.Fatalf("rw-probe status = %s, want fail", check.Status)
		}
		if len(check.Evidence) != 2 {
			t.Fatalf("rw-probe evidence = %#v", check.Evidence)
		}
		found = true
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

func TestAnalyzer_DoesNotBackfillBusinessReadProbeCheckWhenMissing(t *testing.T) {
	t.Parallel()

	cfg := analysisConfig()
	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: cfg,
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:          "demo",
				MilvusURI:     "127.0.0.1:19530",
				Namespace:     "milvus",
				MilvusVersion: "2.6.1",
				ArchProfile:   model.ArchProfileV26,
			},
			BusinessReadProbe: model.BusinessReadProbeResult{
				Enabled:           false,
				Executed:          false,
				Status:            model.CheckStatusSkip,
				MinSuccessTargets: 1,
				Message:           "disabled by config",
			},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Probes.BusinessRead.Status != model.CheckStatusSkip {
		t.Fatalf("BusinessRead = %#v", result.Probes.BusinessRead)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "business-read-probe" {
			found = true
		}
	}
	if found {
		t.Fatalf("Checks = %#v, want analyzer not to backfill business-read-probe", result.Checks)
	}
}

func TestAnalyzer_ReadProbeDisabledKeepsSkipCheckAndLowersConfidence(t *testing.T) {
	t.Parallel()

	cfg := analysisConfig()
	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: cfg,
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:          "demo",
				MilvusURI:     "127.0.0.1:19530",
				Namespace:     "milvus",
				MilvusVersion: "2.6.1",
				ArchProfile:   model.ArchProfileV26,
			},
			BusinessReadProbe: model.BusinessReadProbeResult{
				Enabled:           false,
				Executed:          false,
				Status:            model.CheckStatusSkip,
				Message:           "disabled by config",
				Check:             &model.CheckResult{Name: "business-read-probe", Category: "probe", Status: model.CheckStatusSkip, Message: "disabled by config"},
				MinSuccessTargets: 1,
			},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result == model.FinalResultFAIL {
		t.Fatalf("Result = %s, want not FAIL", result.Result)
	}
	if result.Confidence != model.ConfidenceLow {
		t.Fatalf("Confidence = %s, want low", result.Confidence)
	}
	if len(result.Failures) != 0 {
		t.Fatalf("Failures = %#v, want none", result.Failures)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "business-read-probe" && check.Status == model.CheckStatusSkip && check.Message == "disabled by config" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Checks = %#v", result.Checks)
	}
}

func TestAnalyzer_SkipsRWProbeWhenDisabled(t *testing.T) {
	t.Parallel()

	cfg := analysisConfig()
	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: cfg,
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:          "demo",
				MilvusURI:     "127.0.0.1:19530",
				Namespace:     "milvus",
				MilvusVersion: "2.6.1",
				ArchProfile:   model.ArchProfileV26,
			},
			RWProbe: model.RWProbeResult{
				Status:          model.CheckStatusSkip,
				Enabled:         false,
				CleanupEnabled:  true,
				CleanupExecuted: false,
				Message:         "rw probe disabled",
			},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Probes.RW.Status != model.CheckStatusSkip {
		t.Fatalf("RW = %#v", result.Probes.RW)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "rw-probe" && check.Status == model.CheckStatusSkip && check.Message == "rw probe disabled" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Checks = %#v", result.Checks)
	}
	if len(result.Failures) != 0 {
		t.Fatalf("Failures = %#v, want none", result.Failures)
	}
}

func TestAnalyzer_FailsWhenBusinessReadProbeFailsAndKeepsEvidence(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:          "demo",
				MilvusURI:     "127.0.0.1:19530",
				Namespace:     "milvus",
				MilvusVersion: "2.6.1",
				ArchProfile:   model.ArchProfileV26,
			},
			BusinessReadProbe: model.BusinessReadProbeResult{
				Status:            model.CheckStatusFail,
				ConfiguredTargets: 1,
				SuccessfulTargets: 0,
				MinSuccessTargets: 1,
				Message:           "no read probe targets succeeded",
				Targets: []model.BusinessReadTargetResult{{
					Database:   "default",
					Collection: "book",
					Action:     model.ProbeActionQuery,
					Success:    false,
					Error:      "query failed: timeout",
				}},
				Check: &model.CheckResult{
					Name:     "business-read-probe",
					Category: "probe",
					Status:   model.CheckStatusFail,
					Message:  "no read probe targets succeeded",
					Evidence: []string{"default.book: query failed: timeout"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultFAIL {
		t.Fatalf("Result = %s, want FAIL", result.Result)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "business-read-probe" && len(check.Evidence) == 1 && strings.Contains(check.Evidence[0], "default.book") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Checks = %#v", result.Checks)
	}
}

func TestAnalyzer_WarnsWhenCollectionBinlogSizeIsPartial(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Inventory: model.ClusterInventory{
			Milvus: model.MilvusInventory{
				DatabaseCount:        1,
				CollectionCount:      2,
				TotalBinlogSizeBytes: int64Ptr(1000),
				Databases: []model.DatabaseInventory{
					{Name: "default", Collections: []string{"book", "movie"}},
				},
				Collections: []model.CollectionInventory{
					{Database: "default", Name: "book", RowCount: int64Ptr(10), BinlogSizeBytes: int64Ptr(1000)},
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
			{Name: "milvus-inventory", Status: model.CheckStatusPass, Message: "Milvus inventory collected successfully"},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultWARN {
		t.Fatalf("Result = %s, want WARN", result.Result)
	}
	if result.Summary.TotalBinlogSizeBytes == nil || *result.Summary.TotalBinlogSizeBytes != 1000 {
		t.Fatalf("Summary.TotalBinlogSizeBytes = %#v, want 1000", result.Summary.TotalBinlogSizeBytes)
	}
	foundWarning := false
	foundCheck := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "default.movie") {
			foundWarning = true
		}
	}
	for _, check := range result.Checks {
		if check.Name == "milvus-binlog-size" && check.Status == model.CheckStatusWarn {
			foundCheck = true
		}
	}
	if !foundWarning || !foundCheck {
		t.Fatalf("Warnings=%#v Checks=%#v", result.Warnings, result.Checks)
	}
}

func TestAnalyzer_WarnsWhenTotalBinlogSizeIsMissing(t *testing.T) {
	t.Parallel()

	result, err := (analyzers.InventoryAnalyzer{}).Analyze(context.Background(), model.AnalyzeInput{
		Config: analysisConfig(),
		Inventory: model.ClusterInventory{
			Milvus: model.MilvusInventory{
				DatabaseCount:        1,
				CollectionCount:      1,
				TotalBinlogSizeBytes: nil,
				Databases: []model.DatabaseInventory{
					{Name: "default", Collections: []string{"book"}},
				},
				Collections: []model.CollectionInventory{
					{Database: "default", Name: "book", RowCount: int64Ptr(10), BinlogSizeBytes: int64Ptr(1000)},
				},
			},
		},
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
			{Name: "milvus-inventory", Status: model.CheckStatusPass, Message: "Milvus inventory collected successfully"},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultWARN {
		t.Fatalf("Result = %s, want WARN", result.Result)
	}
	if result.Summary.TotalBinlogSizeBytes != nil {
		t.Fatalf("Summary.TotalBinlogSizeBytes = %#v, want nil", result.Summary.TotalBinlogSizeBytes)
	}
	foundWarning := false
	foundCheck := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "total binlog size unavailable") {
			foundWarning = true
		}
	}
	for _, check := range result.Checks {
		if check.Name == "milvus-binlog-size" && check.Status == model.CheckStatusWarn && strings.Contains(check.Message, "total binlog size unavailable") {
			foundCheck = true
		}
	}
	if !foundWarning || !foundCheck {
		t.Fatalf("Warnings=%#v Checks=%#v", result.Warnings, result.Checks)
	}
}

func TestAnalyzer_LowersConfidenceWhenSkipChecksPresent(t *testing.T) {
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
			{Name: "k8s-pod-health", Status: model.CheckStatusSkip, Message: "arch_profile unknown, pod health check skipped"},
		},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultPASS {
		t.Fatalf("Result = %s, want PASS", result.Result)
	}
	if result.Confidence != model.ConfidenceLow {
		t.Fatalf("Confidence = %s, want low", result.Confidence)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func float64Ptr(v float64) *float64 {
	return &v
}
