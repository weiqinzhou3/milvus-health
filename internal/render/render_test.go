package render_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/model"
	"github.com/weiqinzhou3/milvus-health/internal/render"
)

func sampleResult() *model.AnalysisResult {
	return &model.AnalysisResult{
		Cluster: model.ClusterInfo{
			Name:          "demo-cluster",
			MilvusURI:     "localhost:19530",
			Namespace:     "milvus",
			MilvusVersion: "2.6.1",
			ArchProfile:   model.ArchProfileV26,
			MQType:        "pulsar",
		},
		Result:     model.FinalResultWARN,
		Standby:    false,
		Confidence: model.ConfidenceMedium,
		ExitCode:   1,
		Summary: model.AnalysisSummary{
			DatabaseCount:   1,
			CollectionCount: 1,
			TotalRowCount:   int64Ptr(123),
			PodCount:        2,
			ServiceCount:    1,
			EndpointCount:   1,
		},
		Probes: model.ProbeOutputView{
			BusinessRead: model.BusinessReadProbeResult{Status: model.CheckStatusPass, Message: "ok"},
			RW:           model.RWProbeResult{Status: model.CheckStatusSkip, Enabled: false, Message: "stub"},
		},
		Inventory: &model.ClusterInventory{
			Milvus: model.MilvusInventory{
				Reachable:       true,
				ServerVersion:   "2.6.1",
				DatabaseCount:   1,
				CollectionCount: 1,
				TotalRowCount:   int64Ptr(123),
				Databases: []model.DatabaseInventory{
					{Name: "default", Collections: []string{"book"}},
				},
				Collections: []model.CollectionInventory{
					{Database: "default", Name: "book", RowCount: int64Ptr(123)},
				},
			},
			K8s: model.K8sInventory{
				Namespace: "milvus",
				Pods:      []model.PodStatusSummary{{Name: "milvus-0", Phase: "Running", Ready: true}},
				Services:  []model.ServiceInventory{{Name: "milvus", Type: "ClusterIP", Ports: []string{"19530/tcp"}}},
				Endpoints: []model.EndpointInventory{{Name: "milvus", Addresses: []string{"10.0.0.1"}}},
			},
		},
		Checks: []model.CheckResult{
			{Name: "stub-check", Status: model.CheckStatusWarn, Message: "stub", Recommendation: "inspect fake pipeline", Evidence: []string{"warn evidence"}},
		},
	}
}

func TestJSONRenderer_Render_ReturnsValidJSON(t *testing.T) {
	t.Parallel()

	out, err := (render.JSONRenderer{}).Render(sampleResult(), render.RenderOptions{Detail: false})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not valid json: %v", err)
	}
}

func TestJSONRenderer_Render_OmitsOrKeepsChecksBasedOnDetail(t *testing.T) {
	t.Parallel()

	r := render.JSONRenderer{}

	outNoDetail, err := r.Render(sampleResult(), render.RenderOptions{Detail: false})
	if err != nil {
		t.Fatalf("Render(false) error = %v", err)
	}
	var noDetail map[string]any
	if err := json.Unmarshal(outNoDetail, &noDetail); err != nil {
		t.Fatalf("Unmarshal(false) error = %v", err)
	}
	if checks, ok := noDetail["checks"]; ok {
		items, _ := checks.([]any)
		if len(items) != 0 {
			t.Fatalf("checks should be omitted or empty when detail=false, got %v", checks)
		}
	}

	outDetail, err := r.Render(sampleResult(), render.RenderOptions{Detail: true})
	if err != nil {
		t.Fatalf("Render(true) error = %v", err)
	}
	var detail map[string]any
	if err := json.Unmarshal(outDetail, &detail); err != nil {
		t.Fatalf("Unmarshal(true) error = %v", err)
	}
	checks, ok := detail["checks"]
	if !ok {
		t.Fatal("checks should exist when detail=true")
	}
	items, _ := checks.([]any)
	if len(items) == 0 {
		t.Fatal("checks should not be empty when detail=true")
	}
}

func TestTextRenderer_Render_BasicSummary(t *testing.T) {
	t.Parallel()

	out, err := (render.TextRenderer{}).Render(sampleResult(), render.RenderOptions{Detail: false})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	text := string(out)
	for _, token := range []string{"Cluster", "Milvus URI", "Milvus Version", "Arch Profile", "Overall Result", "Standby", "Confidence", "Exit Code", "Summary: databases=1 collections=1 total_rows=123 pods=2", "K8s Summary: services=1 endpoints=1", "Databases: default(book)"} {
		if !strings.Contains(text, "Summary:") {
			t.Fatalf("text output missing summary: %s", text)
		}
		if !strings.Contains(text, token) {
			t.Fatalf("text output missing %q: %s", token, text)
		}
	}
}

func TestTextRenderer_DetailFalse_NoVerboseChecks(t *testing.T) {
	t.Parallel()

	out, err := (render.TextRenderer{}).Render(sampleResult(), render.RenderOptions{Detail: false})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if strings.Contains(string(out), "stub-check") {
		t.Fatalf("detail=false should not include check details: %s", out)
	}
}

func TestTextRenderer_DetailTrue_IncludesChecks(t *testing.T) {
	t.Parallel()

	out, err := (render.TextRenderer{}).Render(sampleResult(), render.RenderOptions{Detail: true})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !strings.Contains(string(out), "stub-check") {
		t.Fatalf("detail=true should include check details: %s", out)
	}
	if !strings.Contains(string(out), "Inventory:") {
		t.Fatalf("detail=true should include inventory summary: %s", out)
	}
	if !strings.Contains(string(out), "Collection Detail:\n- default.book: row_count=123") {
		t.Fatalf("detail=true should include collection row count detail: %s", out)
	}
	if !strings.Contains(string(out), "Pod Detail:\n- milvus-0: phase=Running ready=true restart_count=0") {
		t.Fatalf("detail=true should include pod detail: %s", out)
	}
	if !strings.Contains(string(out), "Service Detail:\n- milvus: type=ClusterIP ports=19530/tcp") {
		t.Fatalf("detail=true should include service detail: %s", out)
	}
	if !strings.Contains(string(out), "Endpoint Detail:\n- milvus: addresses=10.0.0.1") {
		t.Fatalf("detail=true should include endpoint detail: %s", out)
	}
}

func TestJSONRenderer_DetailFalse_StableShape(t *testing.T) {
	t.Parallel()

	out, err := (render.JSONRenderer{}).Render(sampleResult(), render.RenderOptions{Detail: false})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	for _, field := range []string{"cluster", "result", "standby", "confidence", "exit_code"} {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("missing field %q in output %s", field, out)
		}
	}
	if _, ok := decoded["inventory"]; !ok {
		t.Fatalf("detail=false should keep inventory, got %s", out)
	}
	checks, ok := decoded["checks"]
	if !ok {
		t.Fatalf("detail=false should keep checks as empty array, got %s", out)
	}
	items, _ := checks.([]any)
	if len(items) != 0 {
		t.Fatalf("detail=false should keep empty checks, got %s", out)
	}
}

func TestJSONRenderer_DetailTrue_IncludesChecks(t *testing.T) {
	t.Parallel()

	out, err := (render.JSONRenderer{}).Render(sampleResult(), render.RenderOptions{Detail: true})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, ok := decoded["checks"]; !ok {
		t.Fatalf("detail=true should include checks, got %s", out)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func TestRendererFactory_InvalidFormat_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := (render.DefaultRendererFactory{}).Get("yaml")
	if err == nil {
		t.Fatal("Get() expected error")
	}
}

func TestTextRenderer_Golden(t *testing.T) {
	t.Parallel()

	out, err := (render.TextRenderer{}).Render(sampleResult(), render.RenderOptions{Detail: true})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want, err := os.ReadFile(filepath.Join("testdata", "sample_detail.golden.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(bytes.TrimSpace(out), bytes.TrimSpace(want)) {
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", want, out)
	}
}

func TestJSONRenderer_Golden(t *testing.T) {
	t.Parallel()

	out, err := (render.JSONRenderer{}).Render(sampleResult(), render.RenderOptions{Detail: true})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	want, err := os.ReadFile(filepath.Join("testdata", "sample_detail.golden.json"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !bytes.Equal(bytes.TrimSpace(out), bytes.TrimSpace(want)) {
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", want, out)
	}
}
