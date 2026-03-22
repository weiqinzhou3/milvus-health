package probes

import (
	"context"

	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type NoopRWProbe struct{}

func (NoopRWProbe) Run(ctx context.Context, cfg *model.Config) (model.RWProbeResult, error) {
	_ = ctx
	return model.RWProbeResult{
		Status:  model.CheckStatusSkip,
		Enabled: cfg != nil && cfg.Probe.RW.Enabled,
		Message: "rw probe not implemented in this iteration",
	}, nil
}
