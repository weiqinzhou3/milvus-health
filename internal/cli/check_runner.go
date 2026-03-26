package cli

import (
	"context"
	"strings"
	"time"

	collectork8s "github.com/weiqinzhou3/milvus-health/internal/collectors/k8s"
	collectormilvus "github.com/weiqinzhou3/milvus-health/internal/collectors/milvus"
	"github.com/weiqinzhou3/milvus-health/internal/config"
	"github.com/weiqinzhou3/milvus-health/internal/model"
	"github.com/weiqinzhou3/milvus-health/internal/probes"
)

type DefaultCheckRunner struct {
	Loader          config.Loader
	Validator       config.Validator
	DefaultApplier  config.DefaultApplier
	OverrideApplier config.OverrideApplier
	MilvusCollector collectormilvus.Collector
	K8sCollector    collectork8s.Collector
	ReadProbe       probes.BusinessReadProbe
	RWProbe         probes.RWProbe
	Analyzer        Analyzer
}

func (r DefaultCheckRunner) Run(ctx context.Context, opts model.CheckOptions) (*model.AnalysisResult, error) {
	cfg, err := config.ResolveCheckConfig(r.Loader, r.DefaultApplier, r.OverrideApplier, r.Validator, opts)
	if err != nil {
		return nil, &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
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
			BusinessReadProbe: model.BusinessReadProbeResult{
				Enabled:           cfg.Probe.Read.IsEnabled(),
				Executed:          false,
				Status:            model.CheckStatusSkip,
				ConfiguredTargets: len(cfg.Probe.Read.Targets),
				MinSuccessTargets: cfg.Probe.Read.MinSuccessTargets,
				Message:           defaultBusinessReadProbeMessage(cfg),
			},
			RWProbe: defaultRWProbeResult(cfg),
		},
		StartedAt: startedAt,
	}

	if r.ReadProbe != nil && !cfg.Probe.Read.IsEnabled() {
		probeResult, probeErr := r.ReadProbe.Run(ctx, cfg, probes.ProbeScope{
			Database:   opts.Database,
			Collection: opts.Collection,
		})
		input.Snapshot.BusinessReadProbe = probeResult
		if probeErr != nil {
			input.Failures = append(input.Failures, probeErr.Error())
		}
	}

	if r.MilvusCollector == nil {
		input.Snapshot.BusinessReadProbe = businessReadProbeUnavailable(input.Snapshot.BusinessReadProbe, "not run because Milvus collector is unavailable")
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
			input.Snapshot.BusinessReadProbe = businessReadProbeUnavailable(input.Snapshot.BusinessReadProbe, "not run because Milvus connectivity failed")
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

			if r.ReadProbe != nil && cfg.Probe.Read.IsEnabled() {
				probeResult, probeErr := r.ReadProbe.Run(ctx, cfg, probes.ProbeScope{
					Database:   opts.Database,
					Collection: opts.Collection,
				})
				input.Snapshot.BusinessReadProbe = probeResult
				if probeErr != nil {
					input.Failures = append(input.Failures, probeErr.Error())
				}
			} else if r.ReadProbe == nil && cfg.Probe.Read.IsEnabled() {
				input.Snapshot.BusinessReadProbe = businessReadProbeUnavailable(input.Snapshot.BusinessReadProbe, "not run because business read probe is unavailable")
			}
			if r.RWProbe != nil && cfg.Probe.RW.Enabled {
				rwResult, rwErr := r.RWProbe.Run(ctx, cfg)
				input.Snapshot.RWProbe = rwResult
				if rwErr != nil {
					input.Failures = append(input.Failures, rwErr.Error())
				}
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

	input.Snapshot.Cluster.MQType = resolveMQType(input.Snapshot.Cluster.MQType, cfg, input.Inventory.K8s)
	input.EndedAt = time.Now()
	result, err := r.Analyzer.Analyze(ctx, input)
	if result != nil {
		result.AppliedConfig = cfg
	}
	return result, err
}

func defaultRWProbeResult(cfg *model.Config) model.RWProbeResult {
	result := model.RWProbeResult{
		Status:         model.CheckStatusSkip,
		CleanupEnabled: cfg.Probe.RW.Cleanup,
		InsertRows:     cfg.Probe.RW.InsertRows,
		VectorDim:      cfg.Probe.RW.VectorDim,
		Enabled:        cfg.Probe.RW.Enabled,
	}
	if cfg.Probe.RW.Enabled {
		result.Message = "rw probe not executed"
		return result
	}
	result.Message = "rw probe disabled"
	return result
}

func resolveMQType(current string, cfg *model.Config, k8sInventory model.K8sInventory) string {
	if mqType := model.NormalizeMQType(current); mqType != "unknown" {
		return mqType
	}
	if cfg != nil {
		if mqType := model.NormalizeMQType(cfg.Dependencies.MQ.Type); mqType != "unknown" {
			return mqType
		}
	}

	hasPulsar := false
	hasKafka := false
	for _, service := range k8sInventory.Services {
		name := strings.ToLower(service.Name)
		if strings.Contains(name, "pulsar") {
			hasPulsar = true
		}
		if strings.Contains(name, "kafka") {
			hasKafka = true
		}
	}

	switch {
	case hasPulsar && !hasKafka:
		return "pulsar"
	case hasKafka && !hasPulsar:
		return "kafka"
	default:
		return "unknown"
	}
}

func defaultBusinessReadProbeMessage(cfg *model.Config) string {
	switch {
	case cfg == nil:
		return "not run"
	case !cfg.Probe.Read.IsEnabled():
		return "disabled by config"
	case len(cfg.Probe.Read.Targets) == 0:
		return "not configured"
	default:
		return "not run"
	}
}

func businessReadProbeUnavailable(result model.BusinessReadProbeResult, message string) model.BusinessReadProbeResult {
	if result.Check != nil || result.Executed || !result.Enabled || result.ConfiguredTargets == 0 {
		return result
	}
	result.Status = model.CheckStatusSkip
	result.Message = message
	result.Check = &model.CheckResult{
		Category: "probe",
		Name:     "business-read-probe",
		Status:   model.CheckStatusSkip,
		Message:  message,
		Actual: map[string]any{
			"enabled":            result.Enabled,
			"executed":           result.Executed,
			"configured_targets": result.ConfiguredTargets,
			"successful_targets": result.SuccessfulTargets,
		},
		Expected: result.MinSuccessTargets,
	}
	return result
}
