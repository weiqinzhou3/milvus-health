package render

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type RenderOptions struct {
	Detail bool
}

type Renderer interface {
	Render(result *model.AnalysisResult, opts RenderOptions) ([]byte, error)
}

type RendererFactory interface {
	Get(format model.OutputFormat) (Renderer, error)
}

type TextRenderer struct{}

func (TextRenderer) Render(result *model.AnalysisResult, opts RenderOptions) ([]byte, error) {
	if result == nil {
		return nil, fmt.Errorf("analysis result is nil")
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Cluster: %s\n", result.Cluster.Name)
	fmt.Fprintf(&b, "Milvus URI: %s\n", result.Cluster.MilvusURI)
	fmt.Fprintf(&b, "Namespace: %s\n", result.Cluster.Namespace)
	fmt.Fprintf(&b, "Milvus Version: %s\n", displayString(result.Cluster.MilvusVersion, "unknown"))
	fmt.Fprintf(&b, "Arch Profile: %s\n", displayString(string(result.Cluster.ArchProfile), string(model.ArchProfileUnknown)))
	fmt.Fprintf(&b, "MQ Type: %s\n", displayString(result.Cluster.MQType, "unknown"))
	fmt.Fprintf(&b, "Overall Result: %s\n", result.Result)
	fmt.Fprintf(&b, "Standby: %t\n", result.Standby)
	fmt.Fprintf(&b, "Confidence: %s\n", result.Confidence)
	fmt.Fprintf(&b, "Exit Code: %d\n", result.ExitCode)
	fmt.Fprintf(&b, "Summary: databases=%d collections=%d total_rows=%s pods=%d\n",
		result.Summary.DatabaseCount,
		result.Summary.CollectionCount,
		displayInt64(result.Summary.TotalRowCount),
		result.Summary.PodCount,
	)
	fmt.Fprintf(&b, "K8s Summary: ready=%d not_ready=%d services=%d endpoints=%d resource_usage=%s\n",
		result.Summary.ReadyPodCount,
		result.Summary.NotReadyPodCount,
		result.Summary.ServiceCount,
		result.Summary.EndpointCount,
		formatResourceUsageSummary(result),
	)
	if result.Inventory != nil {
		fmt.Fprintf(&b, "Databases: %s\n", formatDatabases(result.Inventory.Milvus.Databases))
	}
	if len(result.Warnings) > 0 {
		fmt.Fprintf(&b, "Warnings: %s\n", strings.Join(result.Warnings, "; "))
	}
	if len(result.Failures) > 0 {
		fmt.Fprintf(&b, "Failures: %s\n", strings.Join(result.Failures, "; "))
	}
	if opts.Detail && result.Inventory != nil {
		if result.Inventory.Milvus.ServerVersion != "" || result.Inventory.Milvus.DatabaseCount > 0 || result.Inventory.Milvus.CollectionCount > 0 {
			fmt.Fprintf(&b, "Inventory: milvus_version=%s databases=%d collections=%d total_rows=%s\n",
				displayString(result.Inventory.Milvus.ServerVersion, "unknown"),
				result.Inventory.Milvus.DatabaseCount,
				result.Inventory.Milvus.CollectionCount,
				displayInt64(result.Inventory.Milvus.TotalRowCount),
			)
		}
		if len(result.Inventory.Milvus.Collections) > 0 {
			b.WriteString("Collection Detail:\n")
			for _, collection := range result.Inventory.Milvus.Collections {
				fmt.Fprintf(&b, "- %s.%s: row_count=%s\n", collection.Database, collection.Name, displayInt64(collection.RowCount))
			}
		}
		if result.Inventory.K8s.Namespace != "" || len(result.Inventory.K8s.Pods) > 0 {
			fmt.Fprintf(&b, "Inventory: namespace=%s pods=%d ready=%d not_ready=%d services=%d endpoints=%d resource_usage=%s\n",
				result.Inventory.K8s.Namespace,
				len(result.Inventory.K8s.Pods),
				result.Inventory.K8s.ReadyPodCount,
				result.Inventory.K8s.NotReadyPodCount,
				len(result.Inventory.K8s.Services),
				len(result.Inventory.K8s.Endpoints),
				formatInventoryResourceUsageSummary(result.Inventory.K8s),
			)
			if len(result.Inventory.K8s.Pods) > 0 {
				b.WriteString("Pod Detail:\n")
				for _, pod := range result.Inventory.K8s.Pods {
					fmt.Fprintf(&b, "- %s: phase=%s ready=%t restart_count=%d cpu_usage=%s memory_usage=%s cpu_request=%s cpu_limit=%s memory_request=%s memory_limit=%s cpu_request_ratio=%s cpu_limit_ratio=%s memory_request_ratio=%s memory_limit_ratio=%s\n",
						pod.Name,
						pod.Phase,
						pod.Ready,
						pod.RestartCount,
						displayString(pod.CPUUsage, "unknown"),
						displayString(pod.MemoryUsage, "unknown"),
						displayString(pod.CPURequest, "unknown"),
						displayString(pod.CPULimit, "unknown"),
						displayString(pod.MemoryRequest, "unknown"),
						displayString(pod.MemoryLimit, "unknown"),
						displayRatio(pod.CPURequestRatio),
						displayRatio(pod.CPULimitRatio),
						displayRatio(pod.MemoryRequestRatio),
						displayRatio(pod.MemoryLimitRatio),
					)
				}
			}
			if len(result.Inventory.K8s.Services) > 0 {
				b.WriteString("Service Detail:\n")
				for _, service := range result.Inventory.K8s.Services {
					fmt.Fprintf(&b, "- %s: type=%s ports=%s\n", service.Name, service.Type, strings.Join(service.Ports, ","))
				}
			}
			if len(result.Inventory.K8s.Endpoints) > 0 {
				b.WriteString("Endpoint Detail:\n")
				for _, endpoint := range result.Inventory.K8s.Endpoints {
					fmt.Fprintf(&b, "- %s: addresses=%s\n", endpoint.Name, strings.Join(endpoint.Addresses, ","))
				}
			}
		}
	}
	if opts.Detail && len(result.Checks) > 0 {
		b.WriteString("Checks:\n")
		for _, check := range result.Checks {
			fmt.Fprintf(&b, "- %s [%s]: %s\n", check.Name, check.Status, check.Message)
			if check.Recommendation != "" {
				fmt.Fprintf(&b, "  Recommendation: %s\n", check.Recommendation)
			}
		}
	}
	return []byte(b.String()), nil
}

type JSONRenderer struct{}

func (JSONRenderer) Render(result *model.AnalysisResult, opts RenderOptions) ([]byte, error) {
	if result == nil {
		return nil, fmt.Errorf("analysis result is nil")
	}

	type output struct {
		Cluster    model.ClusterInfo       `json:"cluster"`
		Result     model.FinalResult       `json:"result"`
		Standby    bool                    `json:"standby"`
		Confidence model.ConfidenceLevel   `json:"confidence"`
		ExitCode   int                     `json:"exit_code"`
		ElapsedMS  int64                   `json:"elapsed_ms"`
		Summary    model.AnalysisSummary   `json:"summary"`
		Probes     model.ProbeOutputView   `json:"probes"`
		Inventory  *model.ClusterInventory `json:"inventory,omitempty"`
		Warnings   []string                `json:"warnings,omitempty"`
		Failures   []string                `json:"failures,omitempty"`
		Checks     []model.CheckResult     `json:"checks"`
	}

	payload := output{
		Cluster:    result.Cluster,
		Result:     result.Result,
		Standby:    result.Standby,
		Confidence: result.Confidence,
		ExitCode:   result.ExitCode,
		ElapsedMS:  result.ElapsedMS,
		Summary:    result.Summary,
		Probes:     result.Probes,
		Inventory:  result.Inventory,
		Warnings:   result.Warnings,
		Failures:   result.Failures,
	}
	if opts.Detail {
		payload.Checks = append([]model.CheckResult(nil), result.Checks...)
	} else {
		payload.Checks = []model.CheckResult{}
	}
	return json.MarshalIndent(payload, "", "  ")
}

type DefaultRendererFactory struct{}

func (DefaultRendererFactory) Get(format model.OutputFormat) (Renderer, error) {
	switch format {
	case model.OutputFormatText:
		return TextRenderer{}, nil
	case model.OutputFormatJSON:
		return JSONRenderer{}, nil
	default:
		return nil, &model.AppError{Code: model.ErrCodeRender, Message: fmt.Sprintf("unsupported output format %q", format)}
	}
}

func displayString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func displayInt64(value *int64) string {
	if value == nil {
		return "unknown"
	}
	return strconv.FormatInt(*value, 10)
}

func displayRatio(value *float64) string {
	if value == nil {
		return "unknown"
	}
	return strconv.FormatFloat(*value, 'f', 4, 64)
}

func formatResourceUsageSummary(result *model.AnalysisResult) string {
	if result.Inventory == nil {
		if result.Summary.MetricsAvailablePodCount > 0 && result.Summary.MetricsAvailablePodCount < result.Summary.PodCount {
			return fmt.Sprintf("partial (%d/%d pods have metrics)", result.Summary.MetricsAvailablePodCount, result.Summary.PodCount)
		}
		if result.Summary.MetricsAvailablePodCount == result.Summary.PodCount && result.Summary.PodCount > 0 {
			return "available"
		}
		return "unknown"
	}
	return formatInventoryResourceUsageSummary(result.Inventory.K8s)
}

func formatInventoryResourceUsageSummary(k8s model.K8sInventory) string {
	switch {
	case !k8s.ResourceUsageAvailable && k8s.ResourceUnavailableReason != "":
		return string(k8s.ResourceUnavailableReason)
	case k8s.ResourceUsagePartial:
		return fmt.Sprintf("partial (%d/%d pods have metrics)", k8s.MetricsAvailablePodCount, len(k8s.Pods))
	case k8s.ResourceUsageAvailable:
		return fmt.Sprintf("available (%d/%d pods have metrics)", k8s.MetricsAvailablePodCount, len(k8s.Pods))
	default:
		return "unknown"
	}
}

func formatDatabases(databases []model.DatabaseInventory) string {
	if len(databases) == 0 {
		return "none"
	}

	parts := make([]string, 0, len(databases))
	for _, database := range databases {
		if len(database.Collections) == 0 {
			parts = append(parts, database.Name+"()")
			continue
		}
		parts = append(parts, fmt.Sprintf("%s(%s)", database.Name, strings.Join(database.Collections, ", ")))
	}
	return strings.Join(parts, "; ")
}
