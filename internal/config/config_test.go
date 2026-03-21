package config_test

import (
	"path/filepath"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/config"
	"github.com/weiqinzhou3/milvus-health/internal/model"
)

func validConfig() *model.Config {
	return &model.Config{
		Cluster: model.ClusterConfig{
			Name: "test",
			Milvus: model.MilvusConfig{
				URI:      "localhost:19530",
				Password: "pw",
			},
		},
		Probe: model.ProbeConfig{
			Read: model.ReadProbeConfig{
				MinSuccessTargets: 1,
				Targets: []model.ReadProbeTarget{
					{
						Database:   "default",
						Collection: "book",
						QueryExpr:  "id >= 0",
					},
				},
			},
			RW: model.RWProbeConfig{
				Enabled:            true,
				TestDatabasePrefix: "milvus_health_test",
				Cleanup:            true,
				InsertRows:         100,
				VectorDim:          128,
			},
		},
		Output: model.OutputConfig{Format: model.OutputFormatText},
	}
}

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

	cfg := validConfig()

	if err := (config.ConfigValidator{}).Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidator_Validate_Fail_WhenMilvusURIEmpty(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Cluster.Milvus.URI = ""

	if err := (config.ConfigValidator{}).Validate(cfg); err == nil {
		t.Fatal("Validate() expected error")
	}
}

func TestConfigValidator_Validate_Fail_WhenURIHasScheme(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Cluster.Milvus.URI = "tcp://host:19530"

	if err := (config.ConfigValidator{}).Validate(cfg); err == nil {
		t.Fatal("Validate() expected error")
	}
}

func TestConfigValidator_Validate_Fail_WhenFormatInvalid(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Output.Format = "yaml"

	err := (config.ConfigValidator{}).Validate(cfg)
	if err == nil {
		t.Fatal("Validate() expected error")
	}
	assertHasFieldError(t, err, "output.format")
}

func TestConfigValidator_Validate_Fail_WhenReadProbeTargetMissingRequiredField(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Probe.Read.Targets[0].Collection = ""

	if err := (config.ConfigValidator{}).Validate(cfg); err == nil {
		t.Fatal("Validate() expected error")
	}
}

func TestConfigValidator_Validate_Success_WhenMinSuccessTargetsIsZero(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Probe.Read.MinSuccessTargets = 0

	if err := (config.ConfigValidator{}).Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidator_Validate_Success_WhenQueryExprEmpty(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Probe.Read.Targets[0].QueryExpr = ""

	if err := (config.ConfigValidator{}).Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidator_Validate_Fail_WhenMinSuccessTargetsNegative(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Probe.Read.MinSuccessTargets = -1

	err := (config.ConfigValidator{}).Validate(cfg)
	if err == nil {
		t.Fatal("Validate() expected error")
	}
	assertHasFieldError(t, err, "probe.read.min_success_targets")
}

func TestConfigValidator_Validate_Fail_WhenURIHasScheme_FieldReported(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Cluster.Milvus.URI = "tcp://host:19530"

	err := (config.ConfigValidator{}).Validate(cfg)
	if err == nil {
		t.Fatal("Validate() expected error")
	}
	assertHasFieldError(t, err, "cluster.milvus.uri")
}

func TestConfigValidator_Validate_Fail_WhenRWProbeVectorDimInvalid(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Probe.RW.VectorDim = 0

	if err := (config.ConfigValidator{}).Validate(cfg); err == nil {
		t.Fatal("Validate() expected error")
	}
}

func TestConfigValidator_Validate_Fail_WhenRWProbeInsertRowsInvalid(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Probe.RW.InsertRows = 0

	if err := (config.ConfigValidator{}).Validate(cfg); err == nil {
		t.Fatal("Validate() expected error")
	}
}

func TestConfigValidator_Validate_Success_WhenTokenAndPasswordBothSet(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Cluster.Milvus.Token = "token-value"

	if err := (config.ConfigValidator{}).Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
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

func TestDefaultValueApplier_AppliesExpectedDefaults(t *testing.T) {
	t.Parallel()

	cfg := &model.Config{}
	(config.DefaultValueApplier{}).Apply(cfg)

	if cfg.Probe.RW.TestDatabasePrefix == "" {
		t.Fatal("Probe.RW.TestDatabasePrefix should have default")
	}
	if cfg.Probe.RW.InsertRows <= 0 {
		t.Fatal("Probe.RW.InsertRows should have positive default")
	}
	if cfg.Probe.RW.VectorDim <= 0 {
		t.Fatal("Probe.RW.VectorDim should have positive default")
	}
	if cfg.TimeoutSec <= 0 {
		t.Fatal("TimeoutSec should have positive default")
	}
}

func TestCLIOverrideApplier_ApplyCheckOverrides(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Output.Detail = false
	cfg.Probe.RW.Cleanup = false

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

func TestCLIOverrideApplier_TimeoutOverride(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.TimeoutSec = 30

	if err := (config.CLIOverrideApplier{}).ApplyCheckOverrides(cfg, model.CheckOptions{TimeoutSec: 45}); err != nil {
		t.Fatalf("ApplyCheckOverrides() error = %v", err)
	}
	if cfg.TimeoutSec != 45 {
		t.Fatalf("TimeoutSec = %d, want 45", cfg.TimeoutSec)
	}
}

func TestCLIOverrideApplier_CleanupOverride(t *testing.T) {
	t.Parallel()

	cfg := validConfig()
	cfg.Probe.RW.Cleanup = true
	cleanup := false

	if err := (config.CLIOverrideApplier{}).ApplyCheckOverrides(cfg, model.CheckOptions{Cleanup: &cleanup}); err != nil {
		t.Fatalf("ApplyCheckOverrides() error = %v", err)
	}
	if cfg.Probe.RW.Cleanup {
		t.Fatal("Probe.RW.Cleanup should be false")
	}
}

func assertHasFieldError(t *testing.T, err error, field string) {
	t.Helper()

	cfgErr, ok := err.(*config.ConfigError)
	if !ok {
		t.Fatalf("error type = %T, want *config.ConfigError", err)
	}
	for _, entry := range cfgErr.Fields {
		if entry.Field == field {
			return
		}
	}
	t.Fatalf("field error %q not found in %+v", field, cfgErr.Fields)
}
