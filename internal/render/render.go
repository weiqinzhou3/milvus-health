package render

import (
	"encoding/json"
	"fmt"
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
	fmt.Fprintf(&b, "Summary: databases=%d collections=%d pods=%d\n", result.Summary.DatabaseCount, result.Summary.CollectionCount, result.Summary.PodCount)
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
			fmt.Fprintf(&b, "Inventory: milvus_version=%s databases=%d collections=%d\n", displayString(result.Inventory.Milvus.ServerVersion, "unknown"), result.Inventory.Milvus.DatabaseCount, result.Inventory.Milvus.CollectionCount)
		}
		if result.Inventory.K8s.Namespace != "" || len(result.Inventory.K8s.Pods) > 0 {
			fmt.Fprintf(&b, "Inventory: namespace=%s pods=%d services=%d endpoints=%d\n", result.Inventory.K8s.Namespace, len(result.Inventory.K8s.Pods), len(result.Inventory.K8s.Services), len(result.Inventory.K8s.Endpoints))
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
	payload := *result
	if !opts.Detail {
		payload.Checks = nil
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
