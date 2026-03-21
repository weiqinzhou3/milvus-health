package analyzers

import (
	"context"

	"milvus-health/internal/model"
)

type Analyzer interface {
	Analyze(ctx context.Context, input model.AnalyzeInput) (*model.AnalysisResult, error)
}

type FakeAnalyzer struct{}

func (FakeAnalyzer) Analyze(ctx context.Context, input model.AnalyzeInput) (*model.AnalysisResult, error) {
	_ = ctx
	cfg := input.Config
	result := &model.AnalysisResult{
		Result:     model.FinalResultWARN,
		Standby:    false,
		Confidence: model.ConfidenceLow,
		Summary:    model.AnalysisSummary{},
		Probes: model.ProbeOutputView{
			BusinessRead: model.BusinessReadProbeResult{
				Status:  model.CheckStatusSkip,
				Message: "stub analyzer",
			},
			RW: model.RWProbeResult{
				Status:  model.CheckStatusSkip,
				Enabled: cfg != nil && cfg.Probe.RW.Enabled,
				Message: "stub analyzer",
			},
		},
		Warnings: []string{"stub analysis only"},
		Checks: []model.CheckResult{
			{Name: "stub-check", Status: model.CheckStatusWarn, Message: "skeleton runner"},
		},
	}
	if cfg != nil {
		result.Cluster = model.ClusterOutputView{
			Name:      cfg.Cluster.Name,
			MilvusURI: cfg.Cluster.Milvus.URI,
			Namespace: cfg.K8s.Namespace,
		}
	}
	return result, nil
}
