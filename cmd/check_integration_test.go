package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/analyzers"
	"github.com/weiqinzhou3/milvus-health/internal/cli"
	collectork8s "github.com/weiqinzhou3/milvus-health/internal/collectors/k8s"
	collectormilvus "github.com/weiqinzhou3/milvus-health/internal/collectors/milvus"
	"github.com/weiqinzhou3/milvus-health/internal/config"
	platformk8s "github.com/weiqinzhou3/milvus-health/internal/platform/k8s"
	platformmilvus "github.com/weiqinzhou3/milvus-health/internal/platform/milvus"
	"github.com/weiqinzhou3/milvus-health/internal/render"
)

func fakeRealDependencies() dependencies {
	return dependencies{
		checkRunner: cli.DefaultCheckRunner{
			Loader:          config.YAMLLoader{},
			Validator:       config.ConfigValidator{},
			DefaultApplier:  config.DefaultValueApplier{},
			OverrideApplier: config.CLIOverrideApplier{},
			MilvusCollector: collectormilvus.DefaultCollector{
				Factory: platformmilvus.FakeClientFactory{
					Client: &platformmilvus.FakeClient{
						Version:   "2.6.1",
						Databases: []string{"default"},
						Collections: map[string][]string{
							"default": {"book"},
						},
						RowCounts: map[string]map[string]int64{
							"default": {"book": 123},
						},
					},
				},
			},
			K8sCollector: collectork8s.DefaultCollector{
				Factory: platformk8s.FakeClientFactory{
					Client: &platformk8s.FakeClient{
						Pods: []platformk8s.Pod{{
							Name:          "proxy-0",
							Phase:         "Running",
							Ready:         true,
							RestartCount:  0,
							CPURequest:    "500m",
							CPULimit:      "1000m",
							MemoryRequest: "512Mi",
							MemoryLimit:   "1Gi",
						}},
						Services:  []platformk8s.Service{{Name: "milvus", Type: "ClusterIP", Ports: []string{"19530/tcp"}}},
						Endpoints: []platformk8s.Endpoint{{Name: "milvus-abc", Addresses: []string{"10.0.0.1"}}},
						Metrics: platformk8s.PlatformMetricsResult{
							Available: true,
							Metrics: []platformk8s.PodMetric{{
								PodName:     "proxy-0",
								CPUUsage:    "125m",
								MemoryUsage: "256Mi",
							}},
						},
					},
				},
			},
			Analyzer: analyzers.InventoryAnalyzer{},
		},
		validateRunner:  cli.DefaultValidateRunner{Loader: config.YAMLLoader{}, Validator: config.ConfigValidator{}, DefaultApplier: config.DefaultValueApplier{}},
		rendererFactory: render.DefaultRendererFactory{},
		exitMapper:      cli.DefaultExitCodeMapper{},
	}
}

func TestCheckWithFakeRealPipeline_StillReturnsStableText(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, fakeRealDependencies())

	exitCode := app.Execute([]string{"check", "--config", "../examples/config.example.yaml", "--format", "text"})
	if exitCode != 0 {
		t.Fatalf("Execute() = %d, want 0; stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	for _, token := range []string{"Cluster:", "Milvus Version: 2.6.1", "Arch Profile: v2.6", "Summary: databases=1 collections=1 total_rows=123 pods=1", "K8s Summary: ready=1 not_ready=0 services=1 endpoints=1 resource_usage=available (1/1 pods have metrics)", "Databases: default(book)"} {
		if !strings.Contains(stdout.String(), token) {
			t.Fatalf("stdout missing %q: %s", token, stdout.String())
		}
	}
}

func TestCheckFormatJSON_StillReturnsPureJSON(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, fakeRealDependencies())

	exitCode := app.Execute([]string{"check", "--config", "../examples/config.example.yaml", "--format", "json"})
	if exitCode != 0 {
		t.Fatalf("Execute() = %d, want 0; stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	out := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(out, "{") || !strings.HasSuffix(out, "}") {
		t.Fatalf("stdout is not pure JSON: %q", stdout.String())
	}
}

func TestExitCodeStillMatchesAnalysisResult(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, fakeRealDependencies())

	exitCode := app.Execute([]string{"check", "--config", "../examples/config.example.yaml", "--format", "text"})
	if exitCode != 0 {
		t.Fatalf("Execute() = %d, want 0", exitCode)
	}
}
