package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"milvus-health/internal/model"
)

type Loader interface {
	Load(path string) (*model.Config, error)
}

type Validator interface {
	Validate(cfg *model.Config) error
}

type DefaultApplier interface {
	Apply(cfg *model.Config)
}

type OverrideApplier interface {
	ApplyCheckOverrides(cfg *model.Config, opts model.CheckOptions) error
}

type YAMLLoader struct{}

func (YAMLLoader) Load(path string) (*model.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg model.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

type ConfigValidator struct{}

func (ConfigValidator) Validate(cfg *model.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if strings.TrimSpace(cfg.Cluster.Name) == "" {
		return fmt.Errorf("cluster.name is required")
	}
	if strings.TrimSpace(cfg.Cluster.Milvus.URI) == "" {
		return fmt.Errorf("cluster.milvus.uri is required")
	}
	if strings.Contains(cfg.Cluster.Milvus.URI, "://") {
		return fmt.Errorf("cluster.milvus.uri must be host:port without scheme")
	}
	switch cfg.Output.Format {
	case model.OutputFormatText, model.OutputFormatJSON:
	default:
		return fmt.Errorf("output.format must be text or json")
	}
	if cfg.Probe.Read.MinSuccessTargets < 0 {
		return fmt.Errorf("probe.read.min_success_targets must be >= 0")
	}
	return nil
}

type DefaultValueApplier struct{}

func (DefaultValueApplier) Apply(cfg *model.Config) {
	if cfg.Output.Format == "" {
		cfg.Output.Format = model.OutputFormatText
	}
	if cfg.Probe.Read.MinSuccessTargets == 0 {
		cfg.Probe.Read.MinSuccessTargets = 1
	}
}

type CLIOverrideApplier struct{}

func (CLIOverrideApplier) ApplyCheckOverrides(cfg *model.Config, opts model.CheckOptions) error {
	if opts.Format != "" {
		cfg.Output.Format = opts.Format
	}
	if opts.Detail {
		cfg.Output.Detail = true
	}
	if opts.Cleanup != nil {
		cfg.Probe.RW.Cleanup = *opts.Cleanup
	}
	return nil
}
