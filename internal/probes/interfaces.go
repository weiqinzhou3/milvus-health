package probes

import (
	"context"

	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type ProbeScope struct {
	Database   string
	Collection string
}

type BusinessReadProbe interface {
	Run(ctx context.Context, cfg *model.Config, scope ProbeScope) (model.BusinessReadProbeResult, error)
}

type RWProbe interface {
	Run(ctx context.Context, cfg *model.Config) (model.RWProbeResult, error)
}
