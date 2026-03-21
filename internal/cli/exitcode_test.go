package cli_test

import (
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/cli"
	"github.com/weiqinzhou3/milvus-health/internal/model"
)

func TestExitCodeMapper_FromAnalysis(t *testing.T) {
	t.Parallel()

	m := cli.DefaultExitCodeMapper{}

	tests := []struct {
		result model.FinalResult
		want   int
	}{
		{result: model.FinalResultPASS, want: 0},
		{result: model.FinalResultWARN, want: 1},
		{result: model.FinalResultFAIL, want: 2},
	}

	for _, tt := range tests {
		if got := m.FromAnalysis(&model.AnalysisResult{Result: tt.result}); got != tt.want {
			t.Fatalf("FromAnalysis(%q) = %d, want %d", tt.result, got, tt.want)
		}
	}
}

func TestExitCodeMapper_FromError(t *testing.T) {
	t.Parallel()

	m := cli.DefaultExitCodeMapper{}

	if got := m.FromError(&model.AppError{Code: model.ErrCodeConfigInvalid}); got != 3 {
		t.Fatalf("config invalid = %d, want 3", got)
	}
	if got := m.FromError(&model.AppError{Code: model.ErrCodeRuntime}); got != 4 {
		t.Fatalf("runtime = %d, want 4", got)
	}
}

func TestExitCodeMapper_FromAppError_ConfigInvalid(t *testing.T) {
	t.Parallel()

	m := cli.DefaultExitCodeMapper{}
	if got := m.FromError(&model.AppError{Code: model.ErrCodeConfigInvalid, Message: "bad config"}); got != 3 {
		t.Fatalf("config invalid = %d, want 3", got)
	}
}
