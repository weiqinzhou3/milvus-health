package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/analyzers"
	"github.com/weiqinzhou3/milvus-health/internal/cli"
	"github.com/weiqinzhou3/milvus-health/internal/collectors"
	"github.com/weiqinzhou3/milvus-health/internal/config"
	"github.com/weiqinzhou3/milvus-health/internal/platform"
	"github.com/weiqinzhou3/milvus-health/internal/render"
)

func fakeRealDependencies() dependencies {
	return dependencies{
		checkRunner: cli.DefaultCheckRunner{
			Loader:          config.YAMLLoader{},
			Validator:       config.ConfigValidator{},
			DefaultApplier:  config.DefaultValueApplier{},
			OverrideApplier: config.CLIOverrideApplier{},
			Analyzer: analyzers.InventoryAnalyzer{
				MilvusCollector: collectors.DefaultMilvusInventoryCollector{
					Factory: platform.FakeMilvusClientFactory{
						Client: &platform.FakeMilvusClient{
							Version:   "2.6.1",
							Databases: []string{"default"},
							Collections: map[string][]platform.MilvusCollection{
								"default": {{Database: "default", Name: "book", ShardNum: 2, FieldCount: 3}},
							},
						},
					},
				},
				K8sCollector: collectors.DefaultK8sInventoryCollector{
					Factory: platform.FakeK8sClientFactory{
						Client: &platform.FakeK8sClient{
							Pods:      []platform.PodInfo{{Name: "milvus-0", Phase: "Running", Ready: true}},
							Services:  []platform.ServiceInfo{{Name: "milvus", Type: "ClusterIP", Ports: []string{"19530/tcp"}}},
							Endpoints: []platform.EndpointInfo{{Name: "milvus", Addresses: []string{"10.0.0.1"}}},
						},
					},
				},
			},
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
	for _, token := range []string{"Cluster:", "Overall Result: PASS", "Summary: databases=1 collections=1 pods=1"} {
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
