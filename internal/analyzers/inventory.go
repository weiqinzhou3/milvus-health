package analyzers

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/weiqinzhou3/milvus-health/internal/collectors"
	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type InventoryAnalyzer struct {
	MilvusCollector collectors.MilvusInventoryCollector
	K8sCollector    collectors.K8sInventoryCollector
}

func (a InventoryAnalyzer) Analyze(ctx context.Context, input model.AnalyzeInput) (*model.AnalysisResult, error) {
	cfg := input.Config
	if cfg == nil {
		return nil, &model.AppError{Code: model.ErrCodeRuntime, Message: "config is nil"}
	}

	result := &model.AnalysisResult{
		Result:     model.FinalResultPASS,
		Standby:    false,
		Confidence: model.ConfidenceHigh,
		Inventory:  &model.ClusterInventory{},
		Cluster: model.ClusterOutputView{
			Name:      cfg.Cluster.Name,
			MilvusURI: cfg.Cluster.Milvus.URI,
			Namespace: cfg.K8s.Namespace,
		},
		Probes: model.ProbeOutputView{
			BusinessRead: model.BusinessReadProbeResult{
				Status:  model.CheckStatusSkip,
				Message: "not enabled in this iteration",
			},
			RW: model.RWProbeResult{
				Status:  model.CheckStatusSkip,
				Enabled: cfg.Probe.RW.Enabled,
				Message: "rw probe not enabled in this iteration",
			},
		},
	}

	var milvusErr *model.AppError
	if a.MilvusCollector == nil {
		milvusErr = &model.AppError{Code: model.ErrCodeMilvusCollect, Message: "milvus collector is nil"}
	} else {
		milvusInventory, err := a.MilvusCollector.Collect(ctx, cfg)
		if err != nil {
			errors.As(err, &milvusErr)
			if milvusErr == nil {
				milvusErr = &model.AppError{Code: model.ErrCodeMilvusCollect, Cause: err, Message: err.Error()}
			}
		} else {
			result.Inventory.Milvus = milvusInventory
			result.Cluster.MilvusVersion = milvusInventory.ServerVersion
			result.Cluster.ArchProfile = model.DetectArchProfile(milvusInventory.ServerVersion)
			result.Summary.DatabaseCount = len(milvusInventory.Databases)
			result.Summary.CollectionCount = len(milvusInventory.Collections)

			result.Checks = append(result.Checks,
				model.CheckResult{
					Category: "milvus",
					Name:     "milvus-connectivity",
					Status:   model.CheckStatusPass,
					Message:  "Milvus is reachable",
					Target:   cfg.Cluster.Milvus.URI,
				},
				model.CheckResult{
					Category: "milvus",
					Name:     "milvus-version",
					Status:   model.CheckStatusPass,
					Message:  fmt.Sprintf("Milvus version %s", milvusInventory.ServerVersion),
					Target:   cfg.Cluster.Milvus.URI,
				},
			)

			if milvusInventory.CapabilityDegraded {
				result.Warnings = append(result.Warnings, fmt.Sprintf("milvus capability degraded: %s", strings.Join(milvusInventory.DegradedCapabilities, ", ")))
				result.Checks = append(result.Checks, model.CheckResult{
					Category:       "milvus",
					Name:           "milvus-capabilities",
					Status:         model.CheckStatusWarn,
					Message:        fmt.Sprintf("Milvus capability degraded: %s", strings.Join(milvusInventory.DegradedCapabilities, ", ")),
					Recommendation: "verify Milvus server version and SDK compatibility",
				})
				result.Result = model.FinalResultWARN
				result.Confidence = model.ConfidenceMedium
			} else {
				result.Checks = append(result.Checks, model.CheckResult{
					Category: "milvus",
					Name:     "milvus-capabilities",
					Status:   model.CheckStatusPass,
					Message:  fmt.Sprintf("Milvus inventory collected: %d databases, %d collections", len(milvusInventory.Databases), len(milvusInventory.Collections)),
				})
			}
		}
	}

	if milvusErr != nil {
		result.Failures = append(result.Failures, milvusErr.Message)
		result.Checks = append(result.Checks,
			model.CheckResult{
				Category:       "milvus",
				Name:           "milvus-connectivity",
				Status:         model.CheckStatusFail,
				Message:        "Milvus is unavailable",
				Target:         cfg.Cluster.Milvus.URI,
				Recommendation: "verify Milvus address, credentials, and network connectivity",
				Evidence:       []string{milvusErr.Message},
			},
			model.CheckResult{
				Category:       "milvus",
				Name:           "milvus-version",
				Status:         model.CheckStatusSkip,
				Message:        "Milvus version unavailable because connection failed",
				Recommendation: "restore Milvus connectivity before collecting inventory",
			},
		)
		result.Result = model.FinalResultFAIL
		result.Confidence = model.ConfidenceLow
	}

	var k8sErr *model.AppError
	if a.K8sCollector == nil {
		k8sErr = &model.AppError{Code: model.ErrCodeK8sCollect, Message: "k8s collector is nil"}
	} else {
		k8sInventory, err := a.K8sCollector.Collect(ctx, cfg)
		if err != nil {
			errors.As(err, &k8sErr)
			if k8sErr == nil {
				k8sErr = &model.AppError{Code: model.ErrCodeK8sCollect, Cause: err, Message: err.Error()}
			}
		} else {
			result.Inventory.K8s = k8sInventory
			result.Summary.PodCount = len(k8sInventory.Pods)

			var failedPods []string
			var notReadyPods []string
			for _, pod := range k8sInventory.Pods {
				switch {
				case strings.EqualFold(pod.Phase, "Failed"), strings.EqualFold(pod.Phase, "Unknown"):
					failedPods = append(failedPods, pod.Name)
				case !pod.Ready:
					notReadyPods = append(notReadyPods, pod.Name)
				}
			}

			status := model.CheckStatusPass
			message := fmt.Sprintf("K8s inventory collected: %d pods", len(k8sInventory.Pods))
			recommendation := ""
			if len(failedPods) > 0 {
				status = model.CheckStatusFail
				message = fmt.Sprintf("Failed pods detected: %s", strings.Join(failedPods, ", "))
				recommendation = "inspect the failed pods before running deeper health checks"
				result.Result = model.FinalResultFAIL
				result.Failures = append(result.Failures, message)
				result.Confidence = model.ConfidenceMedium
			} else if len(notReadyPods) > 0 {
				status = model.CheckStatusWarn
				message = fmt.Sprintf("NotReady pods detected: %s", strings.Join(notReadyPods, ", "))
				recommendation = "check pod readiness and dependent services"
				if result.Result == model.FinalResultPASS {
					result.Result = model.FinalResultWARN
				}
				result.Warnings = append(result.Warnings, message)
				if result.Confidence == model.ConfidenceHigh {
					result.Confidence = model.ConfidenceMedium
				}
			}

			result.Checks = append(result.Checks, model.CheckResult{
				Category:       "k8s",
				Name:           "k8s-pod-readiness",
				Status:         status,
				Message:        message,
				Target:         cfg.K8s.Namespace,
				Recommendation: recommendation,
			})
		}
	}

	if k8sErr != nil {
		message := fmt.Sprintf("K8s inventory unavailable: %s", k8sErr.Message)
		result.Warnings = append(result.Warnings, message)
		result.Checks = append(result.Checks, model.CheckResult{
			Category:       "k8s",
			Name:           "k8s-pod-readiness",
			Status:         model.CheckStatusWarn,
			Message:        message,
			Target:         cfg.K8s.Namespace,
			Recommendation: "verify kubeconfig or in-cluster credentials",
		})
		if result.Result == model.FinalResultPASS {
			result.Result = model.FinalResultWARN
		}
		if result.Confidence == model.ConfidenceHigh {
			result.Confidence = model.ConfidenceMedium
		}
	}

	return result, nil
}
