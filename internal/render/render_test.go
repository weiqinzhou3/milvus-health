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
			DatabaseCount:            1,
			CollectionCount:          1,
			TotalRowCount:            int64Ptr(123),
			TotalBinlogSizeBytes:     int64Ptr(4567),
			PodCount:                 2,
			ReadyPodCount:            1,
			NotReadyPodCount:         1,
			MetricsAvailablePodCount: 1,
			ServiceCount:             1,
			EndpointCount:            1,
		},
		Probes: model.ProbeOutputView{
			BusinessRead: model.BusinessReadProbeResult{
				Enabled:           true,
				Executed:          true,
				Status:            model.CheckStatusPass,
				ConfiguredTargets: 1,
				SuccessfulTargets: 1,
				MinSuccessTargets: 1,
				Message:           "1/1 read probe targets succeeded",
				Targets: []model.BusinessReadTargetResult{
					{
						Database:   "default",
						Collection: "book",
						Action:     model.ProbeActionQuery,
						Success:    true,
						DurationMS: 12,
						RowCount:   int64Ptr(123),
					},
				},
			},
			RW: model.RWProbeResult{
				Status:          model.CheckStatusPass,
				Enabled:         true,
				InsertRows:      3,
				VectorDim:       4,
				CleanupEnabled:  true,
				CleanupExecuted: true,
				TestDatabase:    "milvus_health_test_1700000000000000000",
				TestCollection:  "rw_probe",
				Message:         "rw probe completed successfully",
				StepResults: []model.ProbeStepResult{
					{Name: "check-pre-existing-test-databases", Success: true, DurationMS: 1},
					{Name: "create-database", Success: true, DurationMS: 2},
					{Name: "create-collection", Success: true, DurationMS: 3},
					{Name: "insert", Success: true, DurationMS: 4},
					{Name: "flush", Success: true, DurationMS: 5},
					{Name: "create-index", Success: true, DurationMS: 6},
					{Name: "load-collection", Success: true, DurationMS: 7},
					{Name: "query", Success: true, DurationMS: 8},
					{Name: "cleanup", Success: true, DurationMS: 9},
				},
			},
		},
		Inventory: &model.ClusterInventory{
			Milvus: model.MilvusInventory{
				Reachable:            true,
				ServerVersion:        "2.6.1",
				DatabaseCount:        1,
				CollectionCount:      1,
				TotalRowCount:        int64Ptr(123),
				TotalBinlogSizeBytes: int64Ptr(4567),
				Databases: []model.DatabaseInventory{
					{Name: "default", Collections: []string{"book"}},
				},
				Collections: []model.CollectionInventory{
					{Database: "default", Name: "book", CollectionID: 1001, RowCount: int64Ptr(123), BinlogSizeBytes: int64Ptr(4567)},
				},
			},
			K8s: model.K8sInventory{
				Namespace:                "milvus",
				TotalPodCount:            2,
				ReadyPodCount:            1,
				NotReadyPodCount:         1,
				ResourceUsageAvailable:   true,
				ResourceUsagePartial:     true,
				MetricsAvailablePodCount: 1,
				Pods: []model.PodStatusSummary{
					{
						Name:               "milvus-0",
						Phase:              "Running",
						Ready:              true,
						CPUUsage:           "125m",
						MemoryUsage:        "256Mi",
						CPURequest:         "500m",
						CPULimit:           "1",
						MemoryRequest:      "512Mi",
						MemoryLimit:        "1Gi",
						CPURequestRatio:    float64Ptr(0.25),
						CPULimitRatio:      float64Ptr(0.125),
						MemoryRequestRatio: float64Ptr(0.5),
						MemoryLimitRatio:   float64Ptr(0.25),
					},
					{
						Name:  "milvus-1",
						Phase: "Pending",
						Ready: false,
					},
				},
				Services:  []model.ServiceInventory{{Name: "attu", Type: "NodePort", Ports: []string{"3000:30031/tcp"}}},
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
	for _, token := range []string{"Cluster", "Milvus URI", "Milvus Version", "Arch Profile", "Overall Result", "Standby", "Confidence", "Exit Code", "Run Mode: dangerous rw_probe_enabled=true cleanup_enabled=true", "Summary: databases=1 collections=1 total_rows=123 total_binlog_size_bytes=4567 pods=2", "K8s Summary: ready=1 not_ready=1 services=1 endpoints=1 resource_usage=partial (1/2 pods have metrics)", "Business Read Probe: status=pass configured_targets=1 successful_targets=1 min_success_targets=1 message=1/1 read probe targets succeeded", "RW Probe: status=pass enabled=true insert_rows=3 vector_dim=4 cleanup_enabled=true cleanup_executed=true message=rw probe completed successfully", "Databases: default(book)"} {
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
	if !strings.Contains(string(out), "Collection Detail:\n- default.book: row_count=123 binlog_size_bytes=4567") {
		t.Fatalf("detail=true should include collection row count detail: %s", out)
	}
	if !strings.Contains(string(out), "Pod Detail:\n- milvus-0: phase=Running ready=true restart_count=0 cpu_usage=125m memory_usage=256Mi cpu_request=500m cpu_limit=1 memory_request=512Mi memory_limit=1Gi cpu_request_ratio=0.2500 cpu_limit_ratio=0.1250 memory_request_ratio=0.5000 memory_limit_ratio=0.2500") {
		t.Fatalf("detail=true should include pod detail: %s", out)
	}
	if !strings.Contains(string(out), "- milvus-1: phase=Pending ready=false restart_count=0 cpu_usage=unknown memory_usage=unknown") {
		t.Fatalf("detail=true should include unknown metric detail: %s", out)
	}
	if !strings.Contains(string(out), "Service Detail:\n- attu: type=NodePort ports=3000:30031/tcp") {
		t.Fatalf("detail=true should include service detail: %s", out)
	}
	if !strings.Contains(string(out), "Endpoint Detail:\n- milvus: addresses=10.0.0.1") {
		t.Fatalf("detail=true should include endpoint detail: %s", out)
	}
	if !strings.Contains(string(out), "Business Read Probe Targets:\n- default.book: action=query success=true duration_ms=12 row_count=123") {
		t.Fatalf("detail=true should include business read probe targets: %s", out)
	}
	if !strings.Contains(string(out), "RW Probe Detail: test_database=milvus_health_test_1700000000000000000 test_collection=rw_probe insert_rows=3 vector_dim=4 cleanup_enabled=true cleanup_executed=true") {
		t.Fatalf("detail=true should include rw probe detail: %s", out)
	}
	if !strings.Contains(string(out), "RW Probe Steps:\n- check-pre-existing-test-databases: success=true duration_ms=1") {
		t.Fatalf("detail=true should include rw probe steps: %s", out)
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
	for _, field := range []string{"cluster", "result", "standby", "confidence", "exit_code", "mode"} {
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

func TestJSONRenderer_DetailFalse_OmitsBusinessReadTargets(t *testing.T) {
	t.Parallel()

	out, err := (render.JSONRenderer{}).Render(sampleResult(), render.RenderOptions{Detail: false})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if strings.Contains(string(out), "\"targets\"") {
		t.Fatalf("detail=false should omit business read targets: %s", out)
	}
}

func TestJSONRenderer_DetailFalse_OmitsRWProbeSteps(t *testing.T) {
	t.Parallel()

	out, err := (render.JSONRenderer{}).Render(sampleResult(), render.RenderOptions{Detail: false})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if strings.Contains(string(out), "\"steps\"") {
		t.Fatalf("detail=false should omit rw probe steps: %s", out)
	}
}

func TestRenderers_RWProbeSkipSummaryIsConsistent(t *testing.T) {
	t.Parallel()

	result := sampleResult()
	result.Probes.RW = model.RWProbeResult{
		Status:          model.CheckStatusSkip,
		Enabled:         false,
		InsertRows:      3,
		VectorDim:       4,
		CleanupEnabled:  false,
		CleanupExecuted: false,
		Message:         "rw probe disabled",
	}

	textOut, err := (render.TextRenderer{}).Render(result, render.RenderOptions{Detail: false})
	if err != nil {
		t.Fatalf("text Render() error = %v", err)
	}
	if !strings.Contains(string(textOut), "RW Probe: status=skip enabled=false insert_rows=3 vector_dim=4 cleanup_enabled=false cleanup_executed=false message=rw probe disabled") {
		t.Fatalf("text output should include rw skip summary: %s", textOut)
	}
	if !strings.Contains(string(textOut), "Run Mode: safe rw_probe_enabled=false cleanup_enabled=false") {
		t.Fatalf("text output should include safe run mode summary: %s", textOut)
	}

	jsonOut, err := (render.JSONRenderer{}).Render(result, render.RenderOptions{Detail: false})
	if err != nil {
		t.Fatalf("json Render() error = %v", err)
	}

	var decoded struct {
		Mode struct {
			Name           string `json:"name"`
			RWProbeEnabled bool   `json:"rw_probe_enabled"`
			CleanupEnabled bool   `json:"cleanup_enabled"`
		} `json:"mode"`
		Probes struct {
			RW struct {
				Status          model.CheckStatus `json:"status"`
				Enabled         bool              `json:"enabled"`
				InsertRows      int               `json:"insert_rows"`
				VectorDim       int               `json:"vector_dim"`
				CleanupEnabled  bool              `json:"cleanup_enabled"`
				CleanupExecuted bool              `json:"cleanup_executed"`
				Message         string            `json:"message"`
			} `json:"rw"`
		} `json:"probes"`
	}
	if err := json.Unmarshal(jsonOut, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.Probes.RW.Status != model.CheckStatusSkip {
		t.Fatalf("RW status = %s, want skip", decoded.Probes.RW.Status)
	}
	if decoded.Mode.Name != "safe" || decoded.Mode.RWProbeEnabled || decoded.Mode.CleanupEnabled {
		t.Fatalf("mode = %#v, want safe mode with both toggles false", decoded.Mode)
	}
	if decoded.Probes.RW.Enabled {
		t.Fatalf("RW enabled = %t, want false", decoded.Probes.RW.Enabled)
	}
	if decoded.Probes.RW.InsertRows != 3 || decoded.Probes.RW.VectorDim != 4 {
		t.Fatalf("RW summary = %#v", decoded.Probes.RW)
	}
	if decoded.Probes.RW.CleanupEnabled || decoded.Probes.RW.CleanupExecuted {
		t.Fatalf("RW cleanup flags = %#v, want disabled and not executed", decoded.Probes.RW)
	}
	if decoded.Probes.RW.Message != "rw probe disabled" {
		t.Fatalf("RW message = %q, want %q", decoded.Probes.RW.Message, "rw probe disabled")
	}
	if strings.Contains(string(jsonOut), "\"steps\"") || strings.Contains(string(jsonOut), "\"test_database\"") || strings.Contains(string(jsonOut), "\"test_collection\"") {
		t.Fatalf("detail=false should keep only rw summary fields: %s", jsonOut)
	}
}

func TestRenderers_BusinessReadDisabledOutputIsConsistent(t *testing.T) {
	t.Parallel()

	result := sampleResult()
	result.Result = model.FinalResultPASS
	result.Confidence = model.ConfidenceLow
	result.Probes.BusinessRead = model.BusinessReadProbeResult{
		Enabled:           false,
		Executed:          false,
		Status:            model.CheckStatusSkip,
		MinSuccessTargets: 1,
		Message:           "disabled by config",
	}
	result.Checks = []model.CheckResult{
		{Name: "business-read-probe", Status: model.CheckStatusSkip, Message: "disabled by config"},
	}

	textOut, err := (render.TextRenderer{}).Render(result, render.RenderOptions{Detail: true})
	if err != nil {
		t.Fatalf("text Render() error = %v", err)
	}
	if !strings.Contains(string(textOut), "business-read-probe [skip]: disabled by config") {
		t.Fatalf("text output should include business-read-probe disabled check: %s", textOut)
	}

	jsonOut, err := (render.JSONRenderer{}).Render(result, render.RenderOptions{Detail: true})
	if err != nil {
		t.Fatalf("json Render() error = %v", err)
	}

	var decoded struct {
		Probes struct {
			BusinessRead struct {
				Enabled  bool              `json:"enabled"`
				Executed bool              `json:"executed"`
				Status   model.CheckStatus `json:"status"`
				Message  string            `json:"message"`
			} `json:"business_read"`
		} `json:"probes"`
		Checks []model.CheckResult `json:"checks"`
	}
	if err := json.Unmarshal(jsonOut, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.Probes.BusinessRead.Status != model.CheckStatusSkip {
		t.Fatalf("business read status = %s, want skip", decoded.Probes.BusinessRead.Status)
	}
	if decoded.Probes.BusinessRead.Enabled {
		t.Fatalf("business read enabled = %t, want false", decoded.Probes.BusinessRead.Enabled)
	}
	if decoded.Probes.BusinessRead.Executed {
		t.Fatalf("business read executed = %t, want false", decoded.Probes.BusinessRead.Executed)
	}
	if decoded.Probes.BusinessRead.Message != "disabled by config" {
		t.Fatalf("business read message = %q, want %q", decoded.Probes.BusinessRead.Message, "disabled by config")
	}
	if len(decoded.Checks) != 1 || decoded.Checks[0].Name != "business-read-probe" || decoded.Checks[0].Status != model.CheckStatusSkip {
		t.Fatalf("checks = %#v", decoded.Checks)
	}
}

func TestJSONRenderer_UsesNullForMissingRatios(t *testing.T) {
	t.Parallel()

	out, err := (render.JSONRenderer{}).Render(sampleResult(), render.RenderOptions{Detail: true})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	var decoded struct {
		Inventory struct {
			K8s struct {
				Pods []map[string]any `json:"pods"`
			} `json:"k8s"`
		} `json:"inventory"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.Inventory.K8s.Pods[1]["cpu_limit_ratio"] != nil {
		t.Fatalf("cpu_limit_ratio should be null, got %#v", decoded.Inventory.K8s.Pods[1]["cpu_limit_ratio"])
	}
}

func TestRenderers_UseUnknownAndNullForMissingBinlogSize(t *testing.T) {
	t.Parallel()

	result := sampleResult()
	result.Summary.TotalBinlogSizeBytes = nil
	result.Inventory.Milvus.TotalBinlogSizeBytes = nil
	result.Inventory.Milvus.Collections[0].BinlogSizeBytes = nil

	textOut, err := (render.TextRenderer{}).Render(result, render.RenderOptions{Detail: true})
	if err != nil {
		t.Fatalf("text Render() error = %v", err)
	}
	if !strings.Contains(string(textOut), "total_binlog_size_bytes=unknown") {
		t.Fatalf("text output should render unknown binlog size: %s", textOut)
	}

	jsonOut, err := (render.JSONRenderer{}).Render(result, render.RenderOptions{Detail: true})
	if err != nil {
		t.Fatalf("json Render() error = %v", err)
	}
	if !strings.Contains(string(jsonOut), `"total_binlog_size_bytes": null`) {
		t.Fatalf("json output should render null total binlog size: %s", jsonOut)
	}
	if !strings.Contains(string(jsonOut), `"binlog_size_bytes": null`) {
		t.Fatalf("json output should render null collection binlog size: %s", jsonOut)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func float64Ptr(v float64) *float64 {
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
