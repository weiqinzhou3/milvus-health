package cli_test

import (
	"context"
	"errors"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/cli"
	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type fakeLoader struct {
	cfg *model.Config
	err error
}

func (f fakeLoader) Load(path string) (*model.Config, error) {
	return f.cfg, f.err
}

type fakeValidator struct {
	err error
}

func (f fakeValidator) Validate(cfg *model.Config) error {
	return f.err
}

type fakeDefaultApplier struct{}

func (fakeDefaultApplier) Apply(cfg *model.Config) {
	cfg.TimeoutSec = 30
}

type fakeOverrideApplier struct {
	err error
}

func (f fakeOverrideApplier) ApplyCheckOverrides(cfg *model.Config, opts model.CheckOptions) error {
	if opts.TimeoutSec > 0 {
		cfg.TimeoutSec = opts.TimeoutSec
	}
	return f.err
}

type fakeAnalyzer struct {
	result *model.AnalysisResult
	err    error
	input  model.AnalyzeInput
}

func (f *fakeAnalyzer) Analyze(ctx context.Context, input model.AnalyzeInput) (*model.AnalysisResult, error) {
	f.input = input
	return f.result, f.err
}

type fakeMilvusCollector struct {
	clusterInfo  model.ClusterInfo
	clusterErr   error
	inventory    model.MilvusInventory
	inventoryErr error
}

func (f fakeMilvusCollector) CollectClusterInfo(ctx context.Context, cfg *model.Config) (model.ClusterInfo, error) {
	_ = ctx
	_ = cfg
	return f.clusterInfo, f.clusterErr
}

func (f fakeMilvusCollector) CollectInventory(ctx context.Context, cfg *model.Config) (model.MilvusInventory, error) {
	_ = ctx
	_ = cfg
	return f.inventory, f.inventoryErr
}

func TestValidateRunner_Run_ReturnsNil_ForValidConfig(t *testing.T) {
	t.Parallel()

	runner := cli.DefaultValidateRunner{
		Loader:         fakeLoader{cfg: &model.Config{}},
		DefaultApplier: fakeDefaultApplier{},
		Validator:      fakeValidator{},
	}

	if err := runner.Run(context.Background(), model.ValidateOptions{ConfigPath: "test.yaml"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestCheckRunner_Run_ReturnsStubAnalysisResult(t *testing.T) {
	t.Parallel()

	expected := &model.AnalysisResult{
		Result:     model.FinalResultWARN,
		Standby:    false,
		Confidence: model.ConfidenceLow,
		ExitCode:   1,
	}

	runner := cli.DefaultCheckRunner{
		Loader:          fakeLoader{cfg: &model.Config{}},
		DefaultApplier:  fakeDefaultApplier{},
		OverrideApplier: fakeOverrideApplier{},
		Validator:       fakeValidator{},
		MilvusCollector: fakeMilvusCollector{
			clusterInfo: model.ClusterInfo{
				Name:          "demo",
				MilvusURI:     "127.0.0.1:19530",
				Namespace:     "milvus",
				MilvusVersion: "2.6.1",
				ArchProfile:   model.ArchProfileV26,
				MQType:        "unknown",
			},
			inventory: model.MilvusInventory{
				ServerVersion:   "2.6.1",
				DatabaseCount:   1,
				CollectionCount: 1,
				TotalRowCount:   int64Ptr(10),
				Databases: []model.DatabaseInventory{
					{Name: "default", Collections: []string{"book"}},
				},
				Collections: []model.CollectionInventory{
					{Database: "default", Name: "book", RowCount: int64Ptr(10)},
				},
			},
		},
		Analyzer: &fakeAnalyzer{result: expected},
	}

	got, err := runner.Run(context.Background(), model.CheckOptions{ConfigPath: "test.yaml"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got != expected {
		t.Fatalf("Run() got %#v, want %#v", got, expected)
	}
}

func TestCheckRunner_FullStubPipeline_Works(t *testing.T) {
	t.Parallel()

	expected := &model.AnalysisResult{Result: model.FinalResultPASS, ExitCode: 0}
	runner := cli.DefaultCheckRunner{
		Loader:          fakeLoader{cfg: &model.Config{}},
		DefaultApplier:  fakeDefaultApplier{},
		OverrideApplier: fakeOverrideApplier{},
		Validator:       fakeValidator{},
		MilvusCollector: fakeMilvusCollector{
			clusterInfo: model.ClusterInfo{
				Name:          "demo",
				MilvusURI:     "127.0.0.1:19530",
				Namespace:     "milvus",
				MilvusVersion: "2.6.1",
				ArchProfile:   model.ArchProfileV26,
				MQType:        "unknown",
			},
			inventory: model.MilvusInventory{
				ServerVersion:   "2.6.1",
				DatabaseCount:   1,
				CollectionCount: 1,
				TotalRowCount:   int64Ptr(10),
				Databases: []model.DatabaseInventory{
					{Name: "default", Collections: []string{"book"}},
				},
				Collections: []model.CollectionInventory{
					{Database: "default", Name: "book", RowCount: int64Ptr(10)},
				},
			},
		},
		Analyzer: &fakeAnalyzer{
			result: expected,
		},
	}

	got, err := runner.Run(context.Background(), model.CheckOptions{ConfigPath: "test.yaml", TimeoutSec: 60})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got != expected {
		t.Fatalf("Run() got %#v, want %#v", got, expected)
	}
}

func TestCheckRunner_CollectsMilvusFactsBeforeAnalyze(t *testing.T) {
	t.Parallel()

	collector := fakeMilvusCollector{
		clusterInfo: model.ClusterInfo{
			Name:          "demo",
			MilvusURI:     "127.0.0.1:19530",
			Namespace:     "milvus",
			MilvusVersion: "2.5.4",
			ArchProfile:   model.ArchProfileV24,
			MQType:        "unknown",
		},
		inventory: model.MilvusInventory{
			ServerVersion:   "2.5.4",
			DatabaseCount:   2,
			CollectionCount: 3,
			TotalRowCount:   int64Ptr(60),
			Databases: []model.DatabaseInventory{
				{Name: "analytics", Collections: []string{"events"}},
				{Name: "default", Collections: []string{"book", "movie"}},
			},
			Collections: []model.CollectionInventory{
				{Database: "analytics", Name: "events", RowCount: int64Ptr(10)},
				{Database: "default", Name: "book", RowCount: int64Ptr(20)},
				{Database: "default", Name: "movie", RowCount: int64Ptr(30)},
			},
		},
	}
	analyzer := &fakeAnalyzer{result: &model.AnalysisResult{}}
	runner := cli.DefaultCheckRunner{
		Loader:          fakeLoader{cfg: &model.Config{}},
		DefaultApplier:  fakeDefaultApplier{},
		OverrideApplier: fakeOverrideApplier{},
		Validator:       fakeValidator{},
		MilvusCollector: collector,
		Analyzer:        analyzer,
	}

	_, err := runner.Run(context.Background(), model.CheckOptions{ConfigPath: "test.yaml"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := analyzer.input
	if got.Snapshot.Cluster.MilvusVersion != "2.5.4" {
		t.Fatalf("Snapshot.Cluster = %#v", got.Snapshot.Cluster)
	}
	if got.Inventory.Milvus.CollectionCount != 3 {
		t.Fatalf("Inventory = %#v", got.Inventory.Milvus)
	}
	if got.Inventory.Milvus.TotalRowCount == nil || *got.Inventory.Milvus.TotalRowCount != 60 {
		t.Fatalf("Inventory.TotalRowCount = %#v", got.Inventory.Milvus.TotalRowCount)
	}
	if len(got.Checks) != 3 {
		t.Fatalf("Checks = %#v, want 3 checks", got.Checks)
	}
}

func TestCheckRunner_TransformsMilvusFailureIntoAnalyzeInput(t *testing.T) {
	t.Parallel()

	analyzer := &fakeAnalyzer{result: &model.AnalysisResult{}}
	runner := cli.DefaultCheckRunner{
		Loader:          fakeLoader{cfg: &model.Config{}},
		DefaultApplier:  fakeDefaultApplier{},
		OverrideApplier: fakeOverrideApplier{},
		Validator:       fakeValidator{},
		MilvusCollector: fakeMilvusCollector{clusterErr: errors.New("dial failed")},
		Analyzer:        analyzer,
	}

	_, err := runner.Run(context.Background(), model.CheckOptions{ConfigPath: "test.yaml"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := analyzer.input
	if len(got.Failures) != 1 {
		t.Fatalf("Failures = %#v", got.Failures)
	}
	if got.Checks[0].Status != model.CheckStatusFail {
		t.Fatalf("first check = %#v", got.Checks[0])
	}
}

func TestValidateRunner_ReturnsAppError_ForInvalidConfig(t *testing.T) {
	t.Parallel()

	runner := cli.DefaultValidateRunner{
		Loader:         fakeLoader{cfg: &model.Config{}},
		DefaultApplier: fakeDefaultApplier{},
		Validator:      fakeValidator{err: errors.New("invalid config")},
	}

	err := runner.Run(context.Background(), model.ValidateOptions{ConfigPath: "bad.yaml"})
	var appErr *model.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Run() error = %T, want *model.AppError", err)
	}
	if appErr.Code != model.ErrCodeConfigInvalid {
		t.Fatalf("AppError.Code = %s, want %s", appErr.Code, model.ErrCodeConfigInvalid)
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
