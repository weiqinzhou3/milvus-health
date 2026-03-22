package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ConfigError struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`
}

func (e *ConfigError) Error() string {
	if e == nil {
		return ""
	}
	if len(e.Fields) == 0 {
		return e.Message
	}
	var parts []string
	for _, field := range e.Fields {
		parts = append(parts, fmt.Sprintf("%s: %s", field.Field, field.Message))
	}
	return fmt.Sprintf("%s (%s)", e.Message, strings.Join(parts, ", "))
}

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
		return &ConfigError{Code: "CONFIG_INVALID", Message: "config is nil"}
	}
	var fields []FieldError
	if strings.TrimSpace(cfg.Cluster.Name) == "" {
		fields = append(fields, FieldError{Field: "cluster.name", Message: "is required"})
	}
	if strings.TrimSpace(cfg.Cluster.Milvus.URI) == "" {
		fields = append(fields, FieldError{Field: "cluster.milvus.uri", Message: "is required"})
	} else if strings.Contains(cfg.Cluster.Milvus.URI, "://") {
		fields = append(fields, FieldError{Field: "cluster.milvus.uri", Message: "must be host:port without scheme"})
	}
	if rawMQType := strings.TrimSpace(cfg.Dependencies.MQ.Type); rawMQType != "" && model.NormalizeMQType(rawMQType) == "unknown" {
		fields = append(fields, FieldError{Field: "dependencies.mq.type", Message: "must be pulsar, kafka, rocksmq, unknown, or woodpecker"})
	}
	switch cfg.Output.Format {
	case model.OutputFormatText, model.OutputFormatJSON:
	default:
		fields = append(fields, FieldError{Field: "output.format", Message: "must be text or json"})
	}
	if cfg.Probe.Read.MinSuccessTargets < 0 {
		fields = append(fields, FieldError{Field: "probe.read.min_success_targets", Message: "must be >= 0"})
	}
	for i, target := range cfg.Probe.Read.Targets {
		if strings.TrimSpace(target.Database) == "" {
			fields = append(fields, FieldError{Field: fmt.Sprintf("probe.read.targets[%d].database", i), Message: "is required"})
		}
		if strings.TrimSpace(target.Collection) == "" {
			fields = append(fields, FieldError{Field: fmt.Sprintf("probe.read.targets[%d].collection", i), Message: "is required"})
		}
	}
	if cfg.Probe.RW.Enabled {
		if strings.TrimSpace(cfg.Probe.RW.TestDatabasePrefix) == "" {
			fields = append(fields, FieldError{Field: "probe.rw.test_database_prefix", Message: "is required"})
		}
		if cfg.Probe.RW.InsertRows <= 0 {
			fields = append(fields, FieldError{Field: "probe.rw.insert_rows", Message: "must be > 0"})
		}
		if cfg.Probe.RW.VectorDim <= 0 {
			fields = append(fields, FieldError{Field: "probe.rw.vector_dim", Message: "must be > 0"})
		}
	}
	if len(fields) > 0 {
		return &ConfigError{
			Code:    "CONFIG_INVALID",
			Message: "config validation failed",
			Fields:  fields,
		}
	}
	return nil
}

type DefaultValueApplier struct{}

func (DefaultValueApplier) Apply(cfg *model.Config) {
	if cfg == nil {
		return
	}
	if cfg.Output.Format == "" {
		cfg.Output.Format = model.OutputFormatText
	}
	if cfg.Probe.Read.MinSuccessTargets == 0 && !cfg.Probe.Read.HasExplicitMinSuccessTargets() {
		cfg.Probe.Read.MinSuccessTargets = 1
	}
	if cfg.Probe.RW.TestDatabasePrefix == "" {
		cfg.Probe.RW.TestDatabasePrefix = "milvus_health_test"
	}
	if cfg.Probe.RW.InsertRows == 0 {
		cfg.Probe.RW.InsertRows = 100
	}
	if cfg.Probe.RW.VectorDim == 0 {
		cfg.Probe.RW.VectorDim = 128
	}
	if cfg.TimeoutSec == 0 {
		cfg.TimeoutSec = 60
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
	if opts.TimeoutSec > 0 {
		cfg.TimeoutSec = opts.TimeoutSec
	}
	return nil
}
