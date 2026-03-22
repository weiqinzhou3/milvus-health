package cli

import (
	"context"
	"time"

	collectork8s "github.com/weiqinzhou3/milvus-health/internal/collectors/k8s"
	collectormilvus "github.com/weiqinzhou3/milvus-health/internal/collectors/milvus"
	"github.com/weiqinzhou3/milvus-health/internal/config"
	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type DefaultCheckRunner struct {
	Loader          config.Loader
	Validator       config.Validator
	DefaultApplier  config.DefaultApplier
	OverrideApplier config.OverrideApplier
	MilvusCollector collectormilvus.Collector
	K8sCollector    collectork8s.Collector
	Analyzer        Analyzer
}

func (r DefaultCheckRunner) Run(ctx context.Context, opts model.CheckOptions) (*model.AnalysisResult, error) {
	cfg, err := r.Loader.Load(opts.ConfigPath)
	if err != nil {
		return nil, &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
	}
	if r.DefaultApplier != nil {
		r.DefaultApplier.Apply(cfg)
	}
	if r.OverrideApplier != nil {
		if err := r.OverrideApplier.ApplyCheckOverrides(cfg, opts); err != nil {
			return nil, &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
		}
	}
	if r.Validator != nil {
		if err := r.Validator.Validate(cfg); err != nil {
			return nil, &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
		}
	}
	if r.Analyzer == nil {
		return nil, &model.AppError{Code: model.ErrCodeRuntime, Message: "analyzer is nil"}
	}

	startedAt := time.Now()
	input := model.AnalyzeInput{
		Config:    cfg,
		Inventory: model.ClusterInventory{},
		Snapshot: model.MetadataSnapshot{
			Cluster: model.ClusterInfo{
				Name:        cfg.Cluster.Name,
				MilvusURI:   cfg.Cluster.Milvus.URI,
				Namespace:   cfg.K8s.Namespace,
				ArchProfile: model.ArchProfileUnknown,
				MQType:      "unknown",
			},
		},
		StartedAt: startedAt,
	}

	if r.MilvusCollector == nil {
		input.Failures = append(input.Failures, "milvus collector is nil")
		input.Checks = append(input.Checks,
			model.CheckResult{
				Category:       "milvus",
				Name:           "milvus-connectivity",
				Status:         model.CheckStatusFail,
				Target:         cfg.Cluster.Milvus.URI,
				Message:        "Milvus collector is unavailable",
				Recommendation: "wire a Milvus collector into the check runner",
			},
			model.CheckResult{
				Category: "milvus",
				Name:     "milvus-version",
				Status:   model.CheckStatusSkip,
				Message:  "Milvus version unavailable because collection did not start",
			},
			model.CheckResult{
				Category: "milvus",
				Name:     "milvus-inventory",
				Status:   model.CheckStatusSkip,
				Message:  "Milvus inventory unavailable because collection did not start",
			},
		)
	} else {
		clusterInfo, err := r.MilvusCollector.CollectClusterInfo(ctx, cfg)
		if err != nil {
			input.Failures = append(input.Failures, err.Error())
			input.Checks = append(input.Checks,
				model.CheckResult{
					Category:       "milvus",
					Name:           "milvus-connectivity",
					Status:         model.CheckStatusFail,
					Target:         cfg.Cluster.Milvus.URI,
					Message:        "Milvus is unavailable",
					Recommendation: "verify Milvus address, credentials, and network connectivity",
					Evidence:       []string{err.Error()},
				},
				model.CheckResult{
					Category:       "milvus",
					Name:           "milvus-version",
					Status:         model.CheckStatusSkip,
					Message:        "Milvus version unavailable because connection failed",
					Recommendation: "restore Milvus connectivity before collecting inventory",
				},
				model.CheckResult{
					Category:       "milvus",
					Name:           "milvus-inventory",
					Status:         model.CheckStatusSkip,
					Message:        "Milvus inventory unavailable because connection failed",
					Recommendation: "restore Milvus connectivity before collecting inventory",
				},
			)
		} else {
			input.Snapshot.Cluster = clusterInfo
			input.Checks = append(input.Checks,
				model.CheckResult{
					Category: "milvus",
					Name:     "milvus-connectivity",
					Status:   model.CheckStatusPass,
					Target:   cfg.Cluster.Milvus.URI,
					Message:  "Milvus is reachable",
				},
				model.CheckResult{
					Category: "milvus",
					Name:     "milvus-version",
					Status:   model.CheckStatusPass,
					Target:   cfg.Cluster.Milvus.URI,
					Message:  "Milvus version collected successfully",
					Actual:   clusterInfo.MilvusVersion,
				},
			)

			inventory, err := r.MilvusCollector.CollectInventory(ctx, cfg)
			if err != nil {
				input.Failures = append(input.Failures, err.Error())
				input.Checks = append(input.Checks, model.CheckResult{
					Category:       "milvus",
					Name:           "milvus-inventory",
					Status:         model.CheckStatusFail,
					Message:        "Milvus inventory collection failed",
					Recommendation: "verify database and collection metadata APIs are reachable",
					Evidence:       []string{err.Error()},
				})
			} else {
				inventory.ServerVersion = clusterInfo.MilvusVersion
				input.Inventory.Milvus = inventory
				input.Snapshot.Milvus = inventory
				input.Checks = append(input.Checks, model.CheckResult{
					Category: "milvus",
					Name:     "milvus-inventory",
					Status:   model.CheckStatusPass,
					Message:  "Milvus inventory collected successfully",
					Actual: map[string]int{
						"database_count":   inventory.DatabaseCount,
						"collection_count": inventory.CollectionCount,
					},
				})
			}
		}
	}

	if r.K8sCollector == nil {
		input.Warnings = append(input.Warnings, "k8s collector is nil")
		input.Checks = append(input.Checks, model.CheckResult{
			Category:       "k8s",
			Name:           "k8s-collection",
			Status:         model.CheckStatusSkip,
			Target:         cfg.K8s.Namespace,
			Message:        "Kubernetes collection is unavailable because the collector is not wired",
			Recommendation: "wire a Kubernetes collector into the check runner",
		})
	} else {
		k8sInventory, err := r.K8sCollector.Collect(ctx, cfg)
		if err != nil {
			input.Failures = append(input.Failures, err.Error())
			input.Checks = append(input.Checks, model.CheckResult{
				Category:       "k8s",
				Name:           "k8s-collection",
				Status:         model.CheckStatusFail,
				Target:         cfg.K8s.Namespace,
				Message:        "Kubernetes inventory collection failed",
				Recommendation: "verify kubeconfig, namespace access, and Kubernetes API reachability",
				Evidence:       []string{err.Error()},
			})
		} else {
			input.Inventory.K8s = k8sInventory
			input.Snapshot.K8s = k8sInventory
			input.Checks = append(input.Checks, model.CheckResult{
				Category: "k8s",
				Name:     "k8s-collection",
				Status:   model.CheckStatusPass,
				Target:   cfg.K8s.Namespace,
				Message:  "Kubernetes inventory collected successfully",
				Actual: map[string]int{
					"pod_count":      len(k8sInventory.Pods),
					"service_count":  len(k8sInventory.Services),
					"endpoint_count": len(k8sInventory.Endpoints),
				},
			})
		}
	}

	input.EndedAt = time.Now()
	return r.Analyzer.Analyze(ctx, input)
}
