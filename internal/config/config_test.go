package config_test

import (
	"path/filepath"
	"testing"

	"milvus-health/internal/config"
	"milvus-health/internal/model"
)

func TestYAMLLoader_Load_Success(t *testing.T) {
	t.Parallel()

	loader := config.YAMLLoader{}
	cfg, err := loader.Load(filepath.Join("..", "..", "examples", "config.example.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
	if cfg.Cluster.Name == "" {
		t.Fatal("cluster.name should not be empty")
	}
}

func TestConfigValidator_Validate_Success_MinimalConfig(t *testing.T) {
	t.Parallel()

	cfg := &model.Config{
		Cluster: model.ClusterConfig{
			Name: "test",
			Milvus: model.MilvusConfig{
				URI: "localhost:19530",
			},
		},
		Output: model.OutputConfig{Format: model.OutputFormatText},
	}

	if err := (config.ConfigValidator{}).Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidator_Validate_Fail_WhenURIHasScheme(t *testing.T) {
	t.Parallel()

	cfg := &model.Config{
		Cluster: model.ClusterConfig{
			Name: "test",
			Milvus: model.MilvusConfig{
				URI: "tcp://host:19530",
			},
		},
		Output: model.OutputConfig{Format: model.OutputFormatText},
	}

	if err := (config.ConfigValidator{}).Validate(cfg); err == nil {
		t.Fatal("Validate() expected error")
	}
}

func TestConfigValidator_Validate_Fail_WhenFormatInvalid(t *testing.T) {
	t.Parallel()

	cfg := &model.Config{
		Cluster: model.ClusterConfig{
			Name: "test",
			Milvus: model.MilvusConfig{
				URI: "localhost:19530",
			},
		},
		Output: model.OutputConfig{Format: "yaml"},
	}

	if err := (config.ConfigValidator{}).Validate(cfg); err == nil {
		t.Fatal("Validate() expected error")
	}
}

func TestDefaultValueApplier_Apply(t *testing.T) {
	t.Parallel()

	cfg := &model.Config{}
	(config.DefaultValueApplier{}).Apply(cfg)

	if cfg.Output.Format != model.OutputFormatText {
		t.Fatalf("Output.Format = %q, want %q", cfg.Output.Format, model.OutputFormatText)
	}
	if cfg.Probe.Read.MinSuccessTargets != 1 {
		t.Fatalf("Probe.Read.MinSuccessTargets = %d, want 1", cfg.Probe.Read.MinSuccessTargets)
	}
}

func TestCLIOverrideApplier_ApplyCheckOverrides(t *testing.T) {
	t.Parallel()

	cfg := &model.Config{
		Output: model.OutputConfig{
			Format: model.OutputFormatText,
			Detail: false,
		},
		Probe: model.ProbeConfig{
			RW: model.RWProbeConfig{
				Cleanup: false,
			},
		},
	}

	cleanup := true
	opts := model.CheckOptions{
		Format:  model.OutputFormatJSON,
		Detail:  true,
		Cleanup: &cleanup,
	}

	if err := (config.CLIOverrideApplier{}).ApplyCheckOverrides(cfg, opts); err != nil {
		t.Fatalf("ApplyCheckOverrides() error = %v", err)
	}
	if cfg.Output.Format != model.OutputFormatJSON {
		t.Fatalf("Output.Format = %q, want %q", cfg.Output.Format, model.OutputFormatJSON)
	}
	if !cfg.Output.Detail {
		t.Fatal("Output.Detail should be true")
	}
	if !cfg.Probe.RW.Cleanup {
		t.Fatal("Probe.RW.Cleanup should be true")
	}
}
