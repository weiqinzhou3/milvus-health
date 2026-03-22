package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/analyzers"
	"github.com/weiqinzhou3/milvus-health/internal/cli"
	collectormilvus "github.com/weiqinzhou3/milvus-health/internal/collectors/milvus"
	"github.com/weiqinzhou3/milvus-health/internal/config"
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
	for _, token := range []string{"Cluster:", "Milvus Version: 2.6.1", "Arch Profile: v2.6", "Summary: databases=1 collections=1 total_rows=123 pods=0", "Databases: default(book)"} {
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
