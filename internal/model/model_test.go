package model_test

import (
	"testing"

	"milvus-health/internal/model"
)

func TestDetectArchProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    model.MilvusArchProfile
	}{
		{name: "2.4.7 -> v24", version: "2.4.7", want: model.ArchProfileV24},
		{name: "2.6.x -> v26", version: "2.6.1", want: model.ArchProfileV26},
		{name: "2.4 other patch -> unknown", version: "2.4.3", want: model.ArchProfileUnknown},
		{name: "unknown -> unknown", version: "3.0.0", want: model.ArchProfileUnknown},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := model.DetectArchProfile(tt.version); got != tt.want {
				t.Fatalf("DetectArchProfile(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestConstantsContract(t *testing.T) {
	t.Parallel()

	if model.OutputFormatText != "text" {
		t.Fatalf("OutputFormatText = %q", model.OutputFormatText)
	}
	if model.OutputFormatJSON != "json" {
		t.Fatalf("OutputFormatJSON = %q", model.OutputFormatJSON)
	}
	if model.FinalResultPASS != "PASS" {
		t.Fatalf("FinalResultPASS = %q", model.FinalResultPASS)
	}
	if model.FinalResultWARN != "WARN" {
		t.Fatalf("FinalResultWARN = %q", model.FinalResultWARN)
	}
	if model.FinalResultFAIL != "FAIL" {
		t.Fatalf("FinalResultFAIL = %q", model.FinalResultFAIL)
	}
}
