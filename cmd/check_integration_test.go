package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/analyzers"
	"github.com/weiqinzhou3/milvus-health/internal/cli"
	collectork8s "github.com/weiqinzhou3/milvus-health/internal/collectors/k8s"
	collectormilvus "github.com/weiqinzhou3/milvus-health/internal/collectors/milvus"
	"github.com/weiqinzhou3/milvus-health/internal/config"
	platformk8s "github.com/weiqinzhou3/milvus-health/internal/platform/k8s"
	platformmilvus "github.com/weiqinzhou3/milvus-health/internal/platform/milvus"
	"github.com/weiqinzhou3/milvus-health/internal/probes"
	"github.com/weiqinzhou3/milvus-health/internal/render"
)

func writeIntegrationConfig(t *testing.T, format string, detail bool) string {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := "cluster:\n" +
		"  name: milvus-prod-sh\n" +
		"  milvus:\n" +
		"    uri: 127.0.0.1:19530\n" +
		"k8s:\n" +
		"  namespace: milvus\n" +
		"  kubeconfig: /tmp/fake-kubeconfig\n" +
		"dependencies:\n" +
		"  mq:\n" +
		"    type: pulsar\n" +
		"probe:\n" +
		"  read:\n" +
		"    enabled: true\n" +
		"    min_success_targets: 1\n" +
		"    targets:\n" +
		"      - database: default\n" +
		"        collection: book\n" +
		"        query_expr: \"id >= 0\"\n" +
		"  rw:\n" +
		"    enabled: false\n" +
		"rules:\n" +
		"  resource_warn_ratio: 0.85\n" +
		"  require_probe_for_standby: true\n" +
		"output:\n" +
		"  format: " + format + "\n" +
		"  detail: " + map[bool]string{true: "true", false: "false"}[detail] + "\n" +
		"timeout_sec: 60\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return configPath
}

func fakeMilvusFailureDependencies() dependencies {
	return dependencies{
		checkRunner: cli.DefaultCheckRunner{
			Loader:          config.YAMLLoader{},
			Validator:       config.ConfigValidator{},
			DefaultApplier:  config.DefaultValueApplier{},
			OverrideApplier: config.CLIOverrideApplier{},
			MilvusCollector: collectormilvus.DefaultCollector{
				Factory: platformmilvus.FakeClientFactory{
					Err: errors.New("dial failed"),
				},
			},
			K8sCollector: collectork8s.DefaultCollector{
				Factory: platformk8s.FakeClientFactory{
					Client: &platformk8s.FakeClient{},
				},
			},
			ReadProbe: probes.DefaultBusinessReadProbe{
				Factory: platformmilvus.FakeClientFactory{
					Err: errors.New("dial failed"),
				},
			},
			Analyzer: analyzers.InventoryAnalyzer{},
		},
		validateRunner:  cli.DefaultValidateRunner{Loader: config.YAMLLoader{}, Validator: config.ConfigValidator{}, DefaultApplier: config.DefaultValueApplier{}},
		rendererFactory: render.DefaultRendererFactory{},
		exitMapper:      cli.DefaultExitCodeMapper{},
	}
}

func fakeRealDependencies() dependencies {
	testDB := "milvus_health_test_1700000000000000000"

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
						CollectionIDs: map[string]map[string]int64{
							"default": {"book": 1001},
						},
						Descriptions: map[string]map[string]platformmilvus.CollectionDescription{
							"default": {
								"book": {
									ID:   1001,
									Name: "book",
									Fields: []platformmilvus.CollectionField{
										{Name: "id", DataType: "Int64", IsPrimaryKey: true},
									},
								},
							},
						},
						RowCounts: map[string]map[string]int64{
							"default": {"book": 123},
						},
						LoadStates: map[string]map[string]platformmilvus.LoadState{
							"default": {"book": platformmilvus.LoadStateLoaded},
						},
						QueryResults: map[string]map[string]platformmilvus.QueryResult{
							"default": {"book": {ResultCount: 1}},
						},
						MetricsByType: map[string]string{
							"system_info": `{"quota_metrics":{"total_binlog_size":4567,"collection_binlog_size":{"1001":4567}}}`,
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
			ReadProbe: probes.DefaultBusinessReadProbe{
				Factory: platformmilvus.FakeClientFactory{
					Client: &platformmilvus.FakeClient{
						Descriptions: map[string]map[string]platformmilvus.CollectionDescription{
							"default": {
								"book": {
									ID:   1001,
									Name: "book",
									Fields: []platformmilvus.CollectionField{
										{Name: "id", DataType: "Int64", IsPrimaryKey: true},
									},
								},
							},
						},
						RowCounts: map[string]map[string]int64{
							"default": {"book": 123},
						},
						LoadStates: map[string]map[string]platformmilvus.LoadState{
							"default": {"book": platformmilvus.LoadStateLoaded},
						},
						QueryResults: map[string]map[string]platformmilvus.QueryResult{
							"default": {"book": {ResultCount: 1}},
						},
					},
				},
			},
			RWProbe: probes.DefaultRWProbe{
				Factory: platformmilvus.FakeClientFactory{
					Client: &platformmilvus.FakeClient{
						Databases: []string{"default"},
						InsertResults: map[string]map[string]platformmilvus.InsertResult{
							testDB: {"rw_probe": {InsertCount: 100}},
						},
						QueryResults: map[string]map[string]platformmilvus.QueryResult{
							testDB: {"rw_probe": {ResultCount: 100}},
						},
					},
				},
				Now: func() time.Time {
					return time.Unix(1700000000, 0)
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
	if exitCode != 1 {
		t.Fatalf("Execute() = %d, want 1; stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	for _, token := range []string{"Cluster:", "Milvus Version: 2.6.1", "Arch Profile: v2.6", "Run Mode: safe rw_probe_enabled=false cleanup_enabled=false", "Summary: databases=1 collections=1 total_rows=123 total_binlog_size_bytes=4567 pods=1", "K8s Summary: ready=1 not_ready=0 services=1 endpoints=1 resource_usage=available (1/1 pods have metrics)", "Business Read Probe: status=pass configured_targets=1 successful_targets=1 min_success_targets=1 message=1/1 read probe targets succeeded", "RW Probe: status=skip enabled=false insert_rows=100 vector_dim=128 cleanup_enabled=false cleanup_executed=false message=rw probe disabled", "Warnings: standby confidence downgraded because probes were skipped", "Databases: default(book)"} {
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
	if exitCode != 1 {
		t.Fatalf("Execute() = %d, want 1; stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	out := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(out, "{") || !strings.HasSuffix(out, "}") {
		t.Fatalf("stdout is not pure JSON: %q", stdout.String())
	}
}

func TestCheck_UsesYAMLFormatWithoutCLIOverride(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, fakeRealDependencies())
	configPath := writeIntegrationConfig(t, "json", false)

	exitCode := app.Execute([]string{"check", "--config", configPath})
	if exitCode != 1 {
		t.Fatalf("Execute() = %d, want 1; stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	out := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(out, "{") || !strings.HasSuffix(out, "}") {
		t.Fatalf("stdout should follow YAML output.format=json: %q", stdout.String())
	}
}

func TestCheck_UsesYAMLDetailWithoutCLIOverride(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, fakeRealDependencies())
	configPath := writeIntegrationConfig(t, "text", true)

	exitCode := app.Execute([]string{"check", "--config", configPath})
	if exitCode != 1 {
		t.Fatalf("Execute() = %d, want 1; stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Checks:\n") || !strings.Contains(stdout.String(), "Collection Detail:\n") {
		t.Fatalf("stdout should follow YAML output.detail=true: %s", stdout.String())
	}
}

func TestCheck_CLIFormatOverridesYAML(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, fakeRealDependencies())
	configPath := writeIntegrationConfig(t, "json", false)

	exitCode := app.Execute([]string{"check", "--config", configPath, "--format", "text"})
	if exitCode != 1 {
		t.Fatalf("Execute() = %d, want 1; stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "Cluster: ") {
		t.Fatalf("stdout should follow CLI --format text override: %q", stdout.String())
	}
}

func TestCheck_CLIDetailOverridesYAML(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, fakeRealDependencies())
	configPath := writeIntegrationConfig(t, "text", true)

	exitCode := app.Execute([]string{"check", "--config", configPath, "--detail=false"})
	if exitCode != 1 {
		t.Fatalf("Execute() = %d, want 1; stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "Checks:\n") || strings.Contains(stdout.String(), "Collection Detail:\n") {
		t.Fatalf("stdout should follow CLI --detail=false override: %s", stdout.String())
	}
}

func TestCheck_FailureOutputShowsBusinessReadUnavailableState(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, fakeMilvusFailureDependencies())
	configPath := writeIntegrationConfig(t, "text", false)

	exitCode := app.Execute([]string{"check", "--config", configPath})
	if exitCode != 2 {
		t.Fatalf("Execute() = %d, want 2; stdout=%s stderr=%s", exitCode, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Business Read Probe: status=skip configured_targets=1 successful_targets=0 min_success_targets=1 message=not run because Milvus connectivity failed") {
		t.Fatalf("stdout should expose business read unavailable semantics: %s", stdout.String())
	}
}

func TestExitCodeStillMatchesAnalysisResult(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := newApp(&stdout, &stderr, fakeRealDependencies())

	exitCode := app.Execute([]string{"check", "--config", "../examples/config.example.yaml", "--format", "text"})
	if exitCode != 1 {
		t.Fatalf("Execute() = %d, want 1", exitCode)
	}
}
