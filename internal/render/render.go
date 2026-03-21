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
	fmt.Fprintf(&b, "Overall Result: %s\n", result.Result)
	fmt.Fprintf(&b, "Standby: %t\n", result.Standby)
	fmt.Fprintf(&b, "Confidence: %s\n", result.Confidence)
	fmt.Fprintf(&b, "Exit Code: %d\n", result.ExitCode)
	fmt.Fprintf(&b, "Summary: databases=%d collections=%d pods=%d\n", result.Summary.DatabaseCount, result.Summary.CollectionCount, result.Summary.PodCount)
	if len(result.Warnings) > 0 {
		fmt.Fprintf(&b, "Warnings: %s\n", strings.Join(result.Warnings, "; "))
	}
	if len(result.Failures) > 0 {
		fmt.Fprintf(&b, "Failures: %s\n", strings.Join(result.Failures, "; "))
	}
	if opts.Detail && result.Inventory != nil {
		if result.Inventory.Milvus.ServerVersion != "" || len(result.Inventory.Milvus.Databases) > 0 || len(result.Inventory.Milvus.Collections) > 0 {
			fmt.Fprintf(&b, "Inventory: milvus_version=%s databases=%d collections=%d\n", result.Inventory.Milvus.ServerVersion, len(result.Inventory.Milvus.Databases), len(result.Inventory.Milvus.Collections))
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
		payload.Inventory = nil
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
