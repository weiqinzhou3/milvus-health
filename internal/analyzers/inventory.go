package analyzers

import (
	"context"
	"fmt"
	"strings"
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
			DatabaseCount:            input.Inventory.Milvus.DatabaseCount,
			CollectionCount:          input.Inventory.Milvus.CollectionCount,
			TotalRowCount:            input.Inventory.Milvus.TotalRowCount,
			TotalBinlogSizeBytes:     input.Inventory.Milvus.TotalBinlogSizeBytes,
			PodCount:                 input.Inventory.K8s.TotalPodCount,
			ReadyPodCount:            input.Inventory.K8s.ReadyPodCount,
			NotReadyPodCount:         input.Inventory.K8s.NotReadyPodCount,
			MetricsAvailablePodCount: input.Inventory.K8s.MetricsAvailablePodCount,
			ServiceCount:             len(input.Inventory.K8s.Services),
			EndpointCount:            len(input.Inventory.K8s.Endpoints),
		},
		Probes: model.ProbeOutputView{
			BusinessRead: input.Snapshot.BusinessReadProbe,
			RW:           input.Snapshot.RWProbe,
		},
		Warnings:  append([]string(nil), input.Warnings...),
		Failures:  append([]string(nil), input.Failures...),
		Checks:    append([]model.CheckResult(nil), input.Checks...),
		ElapsedMS: normalizeElapsedMS(input.StartedAt, input.EndedAt),
	}

	if missingCollections := collectionsMissingRowCount(input.Inventory.Milvus); len(missingCollections) > 0 {
		result.Warnings = append(result.Warnings, buildRowCountWarning(missingCollections))
		result.Checks = append(result.Checks, model.CheckResult{
			Category:       "milvus",
			Name:           "milvus-row-count",
			Status:         model.CheckStatusWarn,
			Message:        buildRowCountWarning(missingCollections),
			Recommendation: "verify GetCollectionStatistics availability for the affected collections",
			Actual:         missingCollections,
		})
	}

	missingBinlogCollections := collectionsMissingBinlogSize(input.Inventory.Milvus)
	totalBinlogMissing := totalBinlogSizeMissing(input.Inventory.Milvus)
	if totalBinlogMissing || len(missingBinlogCollections) > 0 {
		warning := buildBinlogSizeWarning(missingBinlogCollections, totalBinlogMissing)
		var actual any
		if len(missingBinlogCollections) > 0 {
			actual = missingBinlogCollections
		}
		result.Warnings = append(result.Warnings, warning)
		result.Checks = append(result.Checks, model.CheckResult{
			Category:       "milvus",
			Name:           "milvus-binlog-size",
			Status:         model.CheckStatusWarn,
			Message:        warning,
			Recommendation: "verify GetMetrics(\"system_info\") availability and DataCoord quota metrics coverage",
			Actual:         actual,
		})
	}

	appendK8sChecks(result, input)
	appendProbeChecks(result, input)

	if hasMilvusFacts(input.Inventory.Milvus) || hasK8sFacts(input.Inventory.K8s) {
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
		case model.CheckStatusSkip:
			result.Confidence = model.ConfidenceLow
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

func appendProbeChecks(result *model.AnalysisResult, input model.AnalyzeInput) {
	appendBusinessReadProbeChecks(result, input)
	appendRWProbeChecks(result, input)
}

func appendBusinessReadProbeChecks(result *model.AnalysisResult, input model.AnalyzeInput) {
	probe := result.Probes.BusinessRead
	if probe.Status == "" {
		return
	}

	if probe.Check != nil && !hasCheckNamed(result.Checks, "business-read-probe") {
		result.Checks = append(result.Checks, *probe.Check)
	}

	switch probe.Status {
	case model.CheckStatusWarn:
		result.Warnings = append(result.Warnings, probe.Message)
	case model.CheckStatusFail:
		result.Failures = append(result.Failures, probe.Message)
	case model.CheckStatusSkip:
		if input.Config.Rules.RequireProbeForStandby {
			result.Warnings = appendUniqueWarning(result.Warnings, "standby confidence downgraded because probes were skipped")
		}
	}
}

func hasCheckNamed(checks []model.CheckResult, name string) bool {
	for _, check := range checks {
		if check.Name == name {
			return true
		}
	}
	return false
}

func appendRWProbeChecks(result *model.AnalysisResult, input model.AnalyzeInput) {
	probe := result.Probes.RW
	if probe.Status == "" {
		probe = model.RWProbeResult{
			Status:  model.CheckStatusSkip,
			Enabled: input.Config.Probe.RW.Enabled,
			Message: "rw probe disabled",
		}
		result.Probes.RW = probe
	}

	check := model.CheckResult{
		Category: "probe",
		Name:     "rw-probe",
		Status:   probe.Status,
		Message:  probe.Message,
		Actual: map[string]any{
			"enabled":          probe.Enabled,
			"insert_rows":      probe.InsertRows,
			"vector_dim":       probe.VectorDim,
			"cleanup_enabled":  probe.CleanupEnabled,
			"cleanup_executed": probe.CleanupExecuted,
		},
	}
	for _, step := range probe.StepResults {
		if step.Success || strings.TrimSpace(step.Error) == "" {
			continue
		}
		check.Evidence = append(check.Evidence, fmt.Sprintf("%s: %s", step.Name, step.Error))
	}
	result.Checks = append(result.Checks, check)

	switch probe.Status {
	case model.CheckStatusWarn:
		result.Warnings = append(result.Warnings, probe.Message)
	case model.CheckStatusFail:
		result.Failures = append(result.Failures, probe.Message)
	case model.CheckStatusSkip:
		if input.Config.Rules.RequireProbeForStandby {
			result.Warnings = appendUniqueWarning(result.Warnings, "standby confidence downgraded because probes were skipped")
		}
	}
}

func appendUniqueWarning(warnings []string, warning string) []string {
	for _, item := range warnings {
		if item == warning {
			return warnings
		}
	}
	return append(warnings, warning)
}

func appendK8sChecks(result *model.AnalysisResult, input model.AnalyzeInput) {
	if !hasK8sFacts(input.Inventory.K8s) {
		return
	}

	if result.Cluster.ArchProfile == model.ArchProfileUnknown {
		result.Checks = append(result.Checks, model.CheckResult{
			Category: "k8s",
			Name:     "k8s-pod-health",
			Status:   model.CheckStatusSkip,
			Target:   input.Inventory.K8s.Namespace,
			Message:  "arch_profile unknown, pod health check skipped",
		})
		return
	}

	notReadyPods := make([]string, 0)
	restartedPods := make([]string, 0)
	resourceWarnPods := make([]string, 0)
	for _, pod := range input.Inventory.K8s.Pods {
		if !pod.Ready {
			notReadyPods = append(notReadyPods, pod.Name)
		}
		if pod.RestartCount > 0 {
			restartedPods = append(restartedPods, pod.Name)
		}
		if exceedsWarnRatio(pod.CPULimitRatio, input.Config.Rules.ResourceWarnRatio) || exceedsWarnRatio(pod.MemoryLimitRatio, input.Config.Rules.ResourceWarnRatio) {
			resourceWarnPods = append(resourceWarnPods, pod.Name)
		}
	}

	switch {
	case len(notReadyPods) > 0:
		result.Warnings = append(result.Warnings, "pods not ready: "+strings.Join(notReadyPods, ", "))
		result.Checks = append(result.Checks, model.CheckResult{
			Category:       "k8s",
			Name:           "k8s-pod-readiness",
			Status:         model.CheckStatusWarn,
			Target:         input.Inventory.K8s.Namespace,
			Message:        "one or more pods are not ready",
			Recommendation: "inspect pod readiness, events, and container logs",
			Actual:         notReadyPods,
		})
	case len(restartedPods) > 0:
		result.Checks = append(result.Checks, model.CheckResult{
			Category: "k8s",
			Name:     "k8s-pod-health",
			Status:   model.CheckStatusPass,
			Target:   input.Inventory.K8s.Namespace,
			Message:  "all collected pods are ready",
		})
	default:
		result.Checks = append(result.Checks, model.CheckResult{
			Category: "k8s",
			Name:     "k8s-pod-health",
			Status:   model.CheckStatusPass,
			Target:   input.Inventory.K8s.Namespace,
			Message:  "all collected pods are ready with zero restarts",
		})
	}

	if len(restartedPods) > 0 {
		result.Warnings = append(result.Warnings, "pods restarted: "+strings.Join(restartedPods, ", "))
		result.Checks = append(result.Checks, model.CheckResult{
			Category:       "k8s",
			Name:           "k8s-pod-restarts",
			Status:         model.CheckStatusWarn,
			Target:         input.Inventory.K8s.Namespace,
			Message:        "one or more pods have restart_count > 0",
			Recommendation: "inspect prior crashes and restart causes before declaring the cluster healthy",
			Actual:         restartedPods,
		})
	}

	if !input.Inventory.K8s.ResourceUsageAvailable {
		message := "pod resource usage unavailable"
		if reason := string(input.Inventory.K8s.ResourceUnavailableReason); reason != "" {
			message += ": " + reason
			result.Warnings = append(result.Warnings, fmt.Sprintf("resource usage unavailable for %d pods: %s", len(input.Inventory.K8s.Pods), reason))
		}
		result.Checks = append(result.Checks, model.CheckResult{
			Category:       "k8s",
			Name:           "k8s-resource-usage",
			Status:         model.CheckStatusWarn,
			Target:         input.Inventory.K8s.Namespace,
			Message:        message,
			Recommendation: "install metrics-server or grant metrics.k8s.io read permissions",
		})
		return
	}

	if input.Inventory.K8s.ResourceUsagePartial {
		missingCount := len(input.Inventory.K8s.Pods) - input.Inventory.K8s.MetricsAvailablePodCount
		reason := string(input.Inventory.K8s.ResourceUnavailableReason)
		if reason == "" {
			reason = string(model.MetricsUnavailableReasonUnknown)
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("resource usage unavailable for %d pods: %s", missingCount, reason))
		result.Checks = append(result.Checks, model.CheckResult{
			Category:       "k8s",
			Name:           "k8s-resource-usage",
			Status:         model.CheckStatusWarn,
			Target:         input.Inventory.K8s.Namespace,
			Message:        fmt.Sprintf("resource usage partial (%d/%d pods have metrics)", input.Inventory.K8s.MetricsAvailablePodCount, len(input.Inventory.K8s.Pods)),
			Recommendation: "verify metrics-server coverage for the pods with unknown usage",
		})
	}

	if len(resourceWarnPods) > 0 {
		result.Warnings = append(result.Warnings, "pods above usage/limit threshold: "+strings.Join(resourceWarnPods, ", "))
		result.Checks = append(result.Checks, model.CheckResult{
			Category:       "k8s",
			Name:           "k8s-resource-ratio",
			Status:         model.CheckStatusWarn,
			Target:         input.Inventory.K8s.Namespace,
			Message:        "one or more pods exceed the configured usage/limit warning threshold",
			Recommendation: "inspect CPU and memory pressure before declaring the cluster healthy",
			Actual:         resourceWarnPods,
			Expected:       input.Config.Rules.ResourceWarnRatio,
		})
	}
}

func hasMilvusFacts(inventory model.MilvusInventory) bool {
	return inventory.ServerVersion != "" ||
		inventory.DatabaseCount > 0 ||
		inventory.CollectionCount > 0 ||
		inventory.TotalRowCount != nil ||
		inventory.TotalBinlogSizeBytes != nil ||
		len(inventory.Collections) > 0 ||
		len(inventory.Databases) > 0 ||
		len(inventory.DatabaseNames) > 0
}

func hasK8sFacts(inventory model.K8sInventory) bool {
	return inventory.Namespace != "" ||
		len(inventory.Pods) > 0 ||
		len(inventory.Services) > 0 ||
		len(inventory.Endpoints) > 0
}

func collectionsMissingRowCount(inventory model.MilvusInventory) []string {
	if len(inventory.Collections) == 0 {
		return nil
	}

	missing := make([]string, 0)
	for _, collection := range inventory.Collections {
		if collection.RowCount == nil {
			missing = append(missing, collection.Database+"."+collection.Name)
		}
	}
	return missing
}

func buildRowCountWarning(collections []string) string {
	return "row count unavailable for: " + strings.Join(collections, ", ")
}

func collectionsMissingBinlogSize(inventory model.MilvusInventory) []string {
	if len(inventory.Collections) == 0 {
		return nil
	}

	missing := make([]string, 0)
	for _, collection := range inventory.Collections {
		if collection.BinlogSizeBytes == nil {
			missing = append(missing, collection.Database+"."+collection.Name)
		}
	}
	return missing
}

func totalBinlogSizeMissing(inventory model.MilvusInventory) bool {
	return inventory.CollectionCount > 0 && inventory.TotalBinlogSizeBytes == nil
}

func buildBinlogSizeWarning(collections []string, totalMissing bool) string {
	switch {
	case totalMissing && len(collections) > 0:
		return "total binlog size unavailable; per-collection binlog size unavailable for: " + strings.Join(collections, ", ")
	case totalMissing:
		return "total binlog size unavailable"
	default:
		return "binlog size unavailable for: " + strings.Join(collections, ", ")
	}
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

func exceedsWarnRatio(value *float64, threshold float64) bool {
	return value != nil && *value > threshold
}
