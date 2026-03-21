package cli

import (
	"context"

	"github.com/weiqinzhou3/milvus-health/internal/config"
	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type CheckRunner interface {
	Run(ctx context.Context, opts model.CheckOptions) (*model.AnalysisResult, error)
}

type ValidateRunner interface {
	Run(ctx context.Context, opts model.ValidateOptions) error
}

type Analyzer interface {
	Analyze(ctx context.Context, input model.AnalyzeInput) (*model.AnalysisResult, error)
}

type DefaultValidateRunner struct {
	Loader         config.Loader
	Validator      config.Validator
	DefaultApplier config.DefaultApplier
}

func (r DefaultValidateRunner) Run(ctx context.Context, opts model.ValidateOptions) error {
	_ = ctx
	cfg, err := r.Loader.Load(opts.ConfigPath)
	if err != nil {
		return &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
	}
	if r.DefaultApplier != nil {
		r.DefaultApplier.Apply(cfg)
	}
	if r.Validator != nil {
		if err := r.Validator.Validate(cfg); err != nil {
			return &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
		}
	}
	return nil
}
