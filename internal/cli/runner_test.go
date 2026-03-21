package cli_test

import (
	"context"
	"errors"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/cli"
	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type fakeLoader struct {
	cfg *model.Config
	err error
}

func (f fakeLoader) Load(path string) (*model.Config, error) {
	return f.cfg, f.err
}

type fakeValidator struct {
	err error
}

func (f fakeValidator) Validate(cfg *model.Config) error {
	return f.err
}

type fakeDefaultApplier struct{}

func (fakeDefaultApplier) Apply(cfg *model.Config) {
	cfg.TimeoutSec = 30
}

type fakeOverrideApplier struct {
	err error
}

func (f fakeOverrideApplier) ApplyCheckOverrides(cfg *model.Config, opts model.CheckOptions) error {
	if opts.TimeoutSec > 0 {
		cfg.TimeoutSec = opts.TimeoutSec
	}
	return f.err
}

type fakeAnalyzer struct {
	result *model.AnalysisResult
	err    error
}

func (f fakeAnalyzer) Analyze(ctx context.Context, input model.AnalyzeInput) (*model.AnalysisResult, error) {
	return f.result, f.err
}

func TestValidateRunner_Run_ReturnsNil_ForValidConfig(t *testing.T) {
	t.Parallel()

	runner := cli.DefaultValidateRunner{
		Loader:         fakeLoader{cfg: &model.Config{}},
		DefaultApplier: fakeDefaultApplier{},
		Validator:      fakeValidator{},
	}

	if err := runner.Run(context.Background(), model.ValidateOptions{ConfigPath: "test.yaml"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestCheckRunner_Run_ReturnsStubAnalysisResult(t *testing.T) {
	t.Parallel()

	expected := &model.AnalysisResult{
		Result:     model.FinalResultWARN,
		Standby:    false,
		Confidence: model.ConfidenceLow,
		ExitCode:   1,
	}

	runner := cli.DefaultCheckRunner{
		Loader:          fakeLoader{cfg: &model.Config{}},
		DefaultApplier:  fakeDefaultApplier{},
		OverrideApplier: fakeOverrideApplier{},
		Validator:       fakeValidator{},
		Analyzer:        fakeAnalyzer{result: expected},
	}

	got, err := runner.Run(context.Background(), model.CheckOptions{ConfigPath: "test.yaml"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got != expected {
		t.Fatalf("Run() got %#v, want %#v", got, expected)
	}
}

func TestCheckRunner_FullStubPipeline_Works(t *testing.T) {
	t.Parallel()

	expected := &model.AnalysisResult{Result: model.FinalResultPASS, ExitCode: 0}
	runner := cli.DefaultCheckRunner{
		Loader:          fakeLoader{cfg: &model.Config{}},
		DefaultApplier:  fakeDefaultApplier{},
		OverrideApplier: fakeOverrideApplier{},
		Validator:       fakeValidator{},
		Analyzer: fakeAnalyzer{
			result: expected,
		},
	}

	got, err := runner.Run(context.Background(), model.CheckOptions{ConfigPath: "test.yaml", TimeoutSec: 60})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got != expected {
		t.Fatalf("Run() got %#v, want %#v", got, expected)
	}
}

func TestValidateRunner_ReturnsAppError_ForInvalidConfig(t *testing.T) {
	t.Parallel()

	runner := cli.DefaultValidateRunner{
		Loader:         fakeLoader{cfg: &model.Config{}},
		DefaultApplier: fakeDefaultApplier{},
		Validator:      fakeValidator{err: errors.New("invalid config")},
	}

	err := runner.Run(context.Background(), model.ValidateOptions{ConfigPath: "bad.yaml"})
	var appErr *model.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Run() error = %T, want *model.AppError", err)
	}
	if appErr.Code != model.ErrCodeConfigInvalid {
		t.Fatalf("AppError.Code = %s, want %s", appErr.Code, model.ErrCodeConfigInvalid)
	}
}
