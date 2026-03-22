package model_test

import (
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/model"
)

func TestDetectArchProfile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    model.MilvusArchProfile
	}{
		{name: "v2.4.7 -> v2.4", version: "v2.4.7", want: model.ArchProfileV24},
		{name: "v2.6.1 -> v2.6", version: "v2.6.1", want: model.ArchProfileV26},
		{name: "2.4.7 -> v2.4", version: "2.4.7", want: model.ArchProfileV24},
		{name: "2.5.3 -> v2.4", version: "2.5.3", want: model.ArchProfileV24},
		{name: "2.6.0 -> v2.6", version: "2.6.0", want: model.ArchProfileV26},
		{name: "2.6.12 -> v2.6", version: "2.6.12", want: model.ArchProfileV26},
		{name: "2.7.1 -> v2.6", version: "2.7.1", want: model.ArchProfileV26},
		{name: "3.0.0 -> v2.6", version: "3.0.0", want: model.ArchProfileV26},
		{name: "2.3.9 -> unknown", version: "2.3.9", want: model.ArchProfileUnknown},
		{name: "empty -> unknown", version: "", want: model.ArchProfileUnknown},
		{name: "invalid -> unknown", version: "bad.version", want: model.ArchProfileUnknown},
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

func TestNormalizeMQType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "pulsar", input: "pulsar", want: "pulsar"},
		{name: "kafka", input: "Kafka", want: "kafka"},
		{name: "rocksmq", input: "rocksmq", want: "rocksmq"},
		{name: "woodpecker alias", input: "woodpecker", want: "rocksmq"},
		{name: "blank", input: "", want: "unknown"},
		{name: "invalid", input: "rabbitmq", want: "unknown"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := model.NormalizeMQType(tt.input); got != tt.want {
				t.Fatalf("NormalizeMQType(%q) = %q, want %q", tt.input, got, tt.want)
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
