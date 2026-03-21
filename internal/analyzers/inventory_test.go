package analyzers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/analyzers"
	"github.com/weiqinzhou3/milvus-health/internal/collectors"
	"github.com/weiqinzhou3/milvus-health/internal/model"
	"github.com/weiqinzhou3/milvus-health/internal/platform"
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
			Read: model.ReadProbeConfig{MinSuccessTargets: 1},
			RW:   model.RWProbeConfig{Enabled: false},
		},
		TimeoutSec: 1,
	}
}

func newAnalyzer(milvusClient *platform.FakeMilvusClient, k8sClient *platform.FakeK8sClient) analyzers.InventoryAnalyzer {
	return analyzers.InventoryAnalyzer{
		MilvusCollector: collectors.DefaultMilvusInventoryCollector{
			Factory: platform.FakeMilvusClientFactory{Client: milvusClient},
		},
		K8sCollector: collectors.DefaultK8sInventoryCollector{
			Factory: platform.FakeK8sClientFactory{Client: k8sClient},
		},
	}
}

func TestAnalyzer_ReturnsFAIL_WhenMilvusUnavailable(t *testing.T) {
	t.Parallel()

	analyzer := newAnalyzer(
		&platform.FakeMilvusClient{PingErr: context.DeadlineExceeded},
		&platform.FakeK8sClient{},
	)

	result, err := analyzer.Analyze(context.Background(), model.AnalyzeInput{Config: analysisConfig()})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultFAIL {
		t.Fatalf("Result = %s, want FAIL", result.Result)
	}
}

func TestAnalyzer_ReturnsWARN_WhenInventoryPartial(t *testing.T) {
	t.Parallel()

	analyzer := newAnalyzer(
		&platform.FakeMilvusClient{
			Version:      "2.4.7",
			DatabasesErr: platform.ErrCapabilityUnavailable,
			Collections: map[string][]platform.MilvusCollection{
				"default": {{Database: "default", Name: "book"}},
			},
		},
		&platform.FakeK8sClient{},
	)

	result, err := analyzer.Analyze(context.Background(), model.AnalyzeInput{Config: analysisConfig()})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultWARN {
		t.Fatalf("Result = %s, want WARN", result.Result)
	}
}

func TestAnalyzer_ReturnsPASS_WhenMinimalChecksPass(t *testing.T) {
	t.Parallel()

	analyzer := newAnalyzer(
		&platform.FakeMilvusClient{
			Version:   "2.6.1",
			Databases: []string{"default"},
			Collections: map[string][]platform.MilvusCollection{
				"default": {{Database: "default", Name: "book", ShardNum: 2, FieldCount: 3}},
			},
		},
		&platform.FakeK8sClient{
			Pods: []platform.PodInfo{{Name: "milvus-0", Phase: "Running", Ready: true}},
		},
	)

	result, err := analyzer.Analyze(context.Background(), model.AnalyzeInput{Config: analysisConfig()})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultPASS {
		t.Fatalf("Result = %s, want PASS", result.Result)
	}
	if result.Summary.CollectionCount != 1 || result.Summary.PodCount != 1 {
		t.Fatalf("Summary = %#v", result.Summary)
	}
}

func TestAnalyzer_MapsPodReadinessToChecks(t *testing.T) {
	t.Parallel()

	analyzer := newAnalyzer(
		&platform.FakeMilvusClient{
			Version:   "2.6.1",
			Databases: []string{"default"},
			Collections: map[string][]platform.MilvusCollection{
				"default": {{Database: "default", Name: "book"}},
			},
		},
		&platform.FakeK8sClient{
			Pods: []platform.PodInfo{{Name: "proxy-0", Phase: "Running", Ready: false}},
		},
	)

	result, err := analyzer.Analyze(context.Background(), model.AnalyzeInput{Config: analysisConfig()})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Result != model.FinalResultWARN {
		t.Fatalf("Result = %s, want WARN", result.Result)
	}
	found := false
	for _, check := range result.Checks {
		if check.Name == "k8s-pod-readiness" && check.Status == model.CheckStatusWarn && strings.Contains(check.Message, "proxy-0") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Checks = %#v", result.Checks)
	}
}
