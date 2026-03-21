package render_test

import (
	"encoding/json"
	"strings"
	"testing"

	"milvus-health/internal/model"
	"milvus-health/internal/render"
)

func sampleResult() *model.AnalysisResult {
	return &model.AnalysisResult{
		Cluster: model.ClusterOutputView{
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
			PodCount:        2,
		},
		Probes: model.ProbeOutputView{
			BusinessRead: model.BusinessReadProbeResult{Status: model.CheckStatusPass, Message: "ok"},
			RW:           model.RWProbeResult{Status: model.CheckStatusSkip, Enabled: false, Message: "stub"},
		},
		Checks: []model.CheckResult{
			{Name: "stub-check", Status: model.CheckStatusWarn, Message: "stub"},
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
	for _, token := range []string{"Cluster", "Overall Result", "Standby", "Confidence", "Exit Code"} {
		if !strings.Contains(text, token) {
			t.Fatalf("text output missing %q: %s", token, text)
		}
	}
}
