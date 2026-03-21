package analyzers

import (
	"context"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type InventoryAnalyzer struct{}

func (a InventoryAnalyzer) Analyze(ctx context.Context, input model.AnalyzeInput) (*model.AnalysisResult, error) {
	_ = ctx
	if input.Config == nil {
		return nil, &model.AppError{Code: model.ErrCodeRuntime, Message: "config is nil"}
	}

	cluster := input.Snapshot.Cluster
	if cluster.Name == "" {
		cluster = model.ClusterInfo{
			Name:        input.Config.Cluster.Name,
			MilvusURI:   input.Config.Cluster.Milvus.URI,
			Namespace:   input.Config.K8s.Namespace,
			ArchProfile: model.ArchProfileUnknown,
			MQType:      "unknown",
		}
	}
	if cluster.ArchProfile == "" {
		cluster.ArchProfile = model.ArchProfileUnknown
	}
	if cluster.MQType == "" {
		cluster.MQType = "unknown"
	}

	result := &model.AnalysisResult{
		Cluster:    cluster,
		Result:     model.FinalResultPASS,
		Standby:    false,
		Confidence: model.ConfidenceHigh,
		Summary: model.AnalysisSummary{
			DatabaseCount:   input.Inventory.Milvus.DatabaseCount,
			CollectionCount: input.Inventory.Milvus.CollectionCount,
			PodCount:        len(input.Inventory.K8s.Pods),
		},
		Probes: model.ProbeOutputView{
			BusinessRead: model.BusinessReadProbeResult{
				Status:  model.CheckStatusSkip,
				Message: "not enabled in this iteration",
			},
			RW: model.RWProbeResult{
				Status:  model.CheckStatusSkip,
				Enabled: input.Config.Probe.RW.Enabled,
				Message: "rw probe not enabled in this iteration",
			},
		},
		Warnings:  append([]string(nil), input.Warnings...),
		Failures:  append([]string(nil), input.Failures...),
		Checks:    append([]model.CheckResult(nil), input.Checks...),
		ElapsedMS: normalizeElapsedMS(input.StartedAt, input.EndedAt),
	}

	if hasMilvusFacts(input.Inventory.Milvus) || len(input.Inventory.K8s.Pods) > 0 {
		inventory := input.Inventory
		result.Inventory = &inventory
	}

	for _, check := range result.Checks {
		switch check.Status {
		case model.CheckStatusFail:
			result.Result = model.FinalResultFAIL
			result.Confidence = model.ConfidenceLow
		case model.CheckStatusWarn:
			if result.Result == model.FinalResultPASS {
				result.Result = model.FinalResultWARN
			}
			if result.Confidence == model.ConfidenceHigh {
				result.Confidence = model.ConfidenceMedium
			}
		}
	}

	if len(result.Failures) > 0 {
		result.Result = model.FinalResultFAIL
		result.Confidence = model.ConfidenceLow
	} else if len(result.Warnings) > 0 {
		if result.Result == model.FinalResultPASS {
			result.Result = model.FinalResultWARN
		}
		if result.Confidence == model.ConfidenceHigh {
			result.Confidence = model.ConfidenceMedium
		}
	}

	return result, nil
}

func hasMilvusFacts(inventory model.MilvusInventory) bool {
	return inventory.ServerVersion != "" ||
		inventory.DatabaseCount > 0 ||
		inventory.CollectionCount > 0 ||
		len(inventory.Databases) > 0 ||
		len(inventory.DatabaseNames) > 0
}

func normalizeElapsedMS(startedAt, endedAt time.Time) int64 {
	if startedAt.IsZero() || endedAt.IsZero() || endedAt.Before(startedAt) {
		return 0
	}

	elapsedMS := endedAt.Sub(startedAt).Milliseconds()
	if elapsedMS < 100 {
		return elapsedMS
	}
	return (elapsedMS / 100) * 100
}
