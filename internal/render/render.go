package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"milvus-health/internal/model"
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
	if opts.Detail && len(result.Checks) > 0 {
		b.WriteString("Checks:\n")
		for _, check := range result.Checks {
			fmt.Fprintf(&b, "- %s: %s\n", check.Name, check.Status)
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
		return nil, fmt.Errorf("unsupported format %q", format)
	}
}
