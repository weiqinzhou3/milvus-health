package probes_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/model"
	platformmilvus "github.com/weiqinzhou3/milvus-health/internal/platform/milvus"
	"github.com/weiqinzhou3/milvus-health/internal/probes"
)

func TestRWProbe_Run_SkipsWhenDisabled(t *testing.T) {
	t.Parallel()

	result, err := (probes.DefaultRWProbe{}).Run(context.Background(), rwProbeConfig(false, true, 3, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusSkip {
		t.Fatalf("Status = %s, want skip", result.Status)
	}
	if result.Message != "rw probe disabled" {
		t.Fatalf("Message = %q, want %q", result.Message, "rw probe disabled")
	}
}

func TestRWProbe_Run_SucceedsWithCleanup(t *testing.T) {
	t.Parallel()

	testDB := "milvus_health_test_1700000000000000000"
	client := &platformmilvus.FakeClient{
		Databases: []string{"default", "milvus_health_test_1699999999999999999"},
		Collections: map[string][]string{
			"milvus_health_test_1699999999999999999": {"stale_rw"},
		},
		InsertResults: map[string]map[string]platformmilvus.InsertResult{
			testDB: {"rw_probe": {InsertCount: 3}},
		},
		QueryResults: map[string]map[string]platformmilvus.QueryResult{
			testDB: {"rw_probe": {ResultCount: 3}},
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, true, 3, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusPass {
		t.Fatalf("Status = %s, want pass", result.Status)
	}
	if !result.CleanupExecuted {
		t.Fatal("CleanupExecuted should be true")
	}
	if result.TestDatabase != testDB || result.TestCollection != "rw_probe" {
		t.Fatalf("result = %#v", result)
	}
	wantSteps := []string{"cleanup-stale-databases", "create-database", "create-collection", "insert", "flush", "create-index", "load-collection", "query", "cleanup"}
	if got := stepNames(result.StepResults); strings.Join(got, ",") != strings.Join(wantSteps, ",") {
		t.Fatalf("step names = %#v, want %#v", got, wantSteps)
	}
	if client.LastCreateCollectionRequest.VectorDim != 4 {
		t.Fatalf("LastCreateCollectionRequest = %#v", client.LastCreateCollectionRequest)
	}
	if len(client.LastInsertRequest.IDs) != 3 || len(client.LastInsertRequest.Vectors) != 3 {
		t.Fatalf("LastInsertRequest = %#v", client.LastInsertRequest)
	}
	if client.LastQueryRequest.Expr != "id in [0,1,2]" || client.LastQueryRequest.Limit != 3 {
		t.Fatalf("LastQueryRequest = %#v", client.LastQueryRequest)
	}
	if !containsOperation(client.Operations, "create-index:"+testDB+".rw_probe") {
		t.Fatalf("Operations = %#v", client.Operations)
	}
	if !containsOperation(client.Operations, "load-collection:"+testDB+".rw_probe") {
		t.Fatalf("Operations = %#v", client.Operations)
	}
	if !containsOperation(client.Operations, "drop-collection:milvus_health_test_1699999999999999999.stale_rw") {
		t.Fatalf("Operations = %#v", client.Operations)
	}
	if !containsOperation(client.Operations, "drop-database:"+testDB) {
		t.Fatalf("Operations = %#v", client.Operations)
	}
}

func TestRWProbe_Run_FailsWhenStaleCleanupFails(t *testing.T) {
	t.Parallel()

	client := &platformmilvus.FakeClient{
		Databases: []string{"milvus_health_test_1699999999999999999"},
		Collections: map[string][]string{
			"milvus_health_test_1699999999999999999": {"stale_rw"},
		},
		DropCollectionErrs: map[string]map[string]error{
			"milvus_health_test_1699999999999999999": {"stale_rw": errors.New("permission denied")},
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, true, 3, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if !strings.Contains(result.Message, "pre-existing test data cleanup failed") {
		t.Fatalf("Message = %q", result.Message)
	}
	if len(result.StepResults) != 1 || result.StepResults[0].Name != "cleanup-stale-databases" || result.StepResults[0].Success {
		t.Fatalf("StepResults = %#v", result.StepResults)
	}
	if containsOperation(client.Operations, "create-database:"+result.TestDatabase) {
		t.Fatalf("Operations = %#v", client.Operations)
	}
}

func TestRWProbe_Run_SucceedsWithoutCleanupWhenCleanupDisabled(t *testing.T) {
	t.Parallel()

	testDB := "milvus_health_test_1700000000000000000"
	client := &platformmilvus.FakeClient{
		InsertResults: map[string]map[string]platformmilvus.InsertResult{
			testDB: {"rw_probe": {InsertCount: 2}},
		},
		QueryResults: map[string]map[string]platformmilvus.QueryResult{
			testDB: {"rw_probe": {ResultCount: 2}},
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, false, 2, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusPass {
		t.Fatalf("Status = %s, want pass", result.Status)
	}
	if result.CleanupExecuted {
		t.Fatal("CleanupExecuted should be false when cleanup is disabled")
	}
	wantSteps := []string{"cleanup-stale-databases", "create-database", "create-collection", "insert", "flush", "create-index", "load-collection", "query"}
	if got := stepNames(result.StepResults); strings.Join(got, ",") != strings.Join(wantSteps, ",") {
		t.Fatalf("step names = %#v, want %#v", got, wantSteps)
	}
	if containsOperation(client.Operations, "drop-collection:"+testDB+".rw_probe") {
		t.Fatalf("Operations = %#v", client.Operations)
	}
	if containsOperation(client.Operations, "drop-database:"+testDB) {
		t.Fatalf("Operations = %#v", client.Operations)
	}
}

func TestRWProbe_Run_DoesNotCleanupAfterFailureWhenCleanupDisabled(t *testing.T) {
	t.Parallel()

	testDB := "milvus_health_test_1700000000000000000"
	client := &platformmilvus.FakeClient{
		InsertResults: map[string]map[string]platformmilvus.InsertResult{
			testDB: {"rw_probe": {InsertCount: 2}},
		},
		QueryErrs: map[string]map[string]error{
			testDB: {"rw_probe": errors.New("query timeout")},
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, false, 2, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if !strings.Contains(result.Message, "query failed") {
		t.Fatalf("Message = %q", result.Message)
	}
	if result.CleanupExecuted {
		t.Fatal("CleanupExecuted should be false when cleanup is disabled")
	}
	wantSteps := []string{"cleanup-stale-databases", "create-database", "create-collection", "insert", "flush", "create-index", "load-collection", "query"}
	if got := stepNames(result.StepResults); strings.Join(got, ",") != strings.Join(wantSteps, ",") {
		t.Fatalf("step names = %#v, want %#v", got, wantSteps)
	}
	if containsOperation(client.Operations, "drop-collection:"+testDB+".rw_probe") {
		t.Fatalf("Operations = %#v", client.Operations)
	}
	if containsOperation(client.Operations, "drop-database:"+testDB) {
		t.Fatalf("Operations = %#v", client.Operations)
	}
}

func TestRWProbe_Run_FailsWhenCreateDatabaseFails(t *testing.T) {
	t.Parallel()

	testDB := "milvus_health_test_1700000000000000000"
	client := &platformmilvus.FakeClient{
		CreateDatabaseErr: map[string]error{
			testDB: errors.New("database quota exceeded"),
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, true, 2, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if !strings.Contains(result.Message, "create database failed") {
		t.Fatalf("Message = %q", result.Message)
	}
	wantSteps := []string{"cleanup-stale-databases", "create-database"}
	if got := stepNames(result.StepResults); strings.Join(got, ",") != strings.Join(wantSteps, ",") {
		t.Fatalf("step names = %#v, want %#v", got, wantSteps)
	}
	if result.CleanupExecuted {
		t.Fatal("CleanupExecuted should be false when database creation fails")
	}
	if containsOperation(client.Operations, "create-collection:"+testDB+".rw_probe") {
		t.Fatalf("Operations = %#v", client.Operations)
	}
}

func TestRWProbe_Run_FailsWhenCreateCollectionFailsAndCleansDatabase(t *testing.T) {
	t.Parallel()

	testDB := "milvus_health_test_1700000000000000000"
	client := &platformmilvus.FakeClient{
		CreateCollectionErrs: map[string]map[string]error{
			testDB: {"rw_probe": errors.New("create collection failed")},
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, true, 2, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if !strings.Contains(result.Message, "create collection failed") {
		t.Fatalf("Message = %q", result.Message)
	}
	if !result.CleanupExecuted {
		t.Fatal("CleanupExecuted should be true after collection creation failure")
	}
	wantSteps := []string{"cleanup-stale-databases", "create-database", "create-collection", "cleanup"}
	if got := stepNames(result.StepResults); strings.Join(got, ",") != strings.Join(wantSteps, ",") {
		t.Fatalf("step names = %#v, want %#v", got, wantSteps)
	}
	if !containsOperation(client.Operations, "drop-database:"+testDB) {
		t.Fatalf("Operations = %#v", client.Operations)
	}
	if containsOperation(client.Operations, "drop-collection:"+testDB+".rw_probe") {
		t.Fatalf("Operations = %#v", client.Operations)
	}
}

func TestRWProbe_Run_FailsWhenInsertCountMismatches(t *testing.T) {
	t.Parallel()

	testDB := "milvus_health_test_1700000000000000000"
	client := &platformmilvus.FakeClient{
		InsertResults: map[string]map[string]platformmilvus.InsertResult{
			testDB: {"rw_probe": {InsertCount: 1}},
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, true, 2, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if !strings.Contains(result.Message, "insert failed") {
		t.Fatalf("Message = %q", result.Message)
	}
	if got := result.StepResults[len(result.StepResults)-2]; got.Name != "insert" || got.Success {
		t.Fatalf("insert step = %#v", got)
	}
	if !result.CleanupExecuted {
		t.Fatal("CleanupExecuted should be true after insert failure when cleanup is enabled")
	}
}

func TestRWProbe_Run_FailsWhenFlushFails(t *testing.T) {
	t.Parallel()

	testDB := "milvus_health_test_1700000000000000000"
	client := &platformmilvus.FakeClient{
		InsertResults: map[string]map[string]platformmilvus.InsertResult{
			testDB: {"rw_probe": {InsertCount: 2}},
		},
		FlushErrs: map[string]map[string]error{
			testDB: {"rw_probe": errors.New("flush timeout")},
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, true, 2, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if !strings.Contains(result.Message, "flush failed") {
		t.Fatalf("Message = %q", result.Message)
	}
	if got := result.StepResults[len(result.StepResults)-2]; got.Name != "flush" || got.Success {
		t.Fatalf("flush step = %#v", got)
	}
	if !result.CleanupExecuted {
		t.Fatal("CleanupExecuted should be true after flush failure when cleanup is enabled")
	}
}

func TestRWProbe_Run_FailsWhenReadbackQueryFails(t *testing.T) {
	t.Parallel()

	testDB := "milvus_health_test_1700000000000000000"
	client := &platformmilvus.FakeClient{
		InsertResults: map[string]map[string]platformmilvus.InsertResult{
			testDB: {"rw_probe": {InsertCount: 2}},
		},
		QueryErrs: map[string]map[string]error{
			testDB: {"rw_probe": errors.New("query timeout")},
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, true, 2, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if !strings.Contains(result.Message, "query failed") {
		t.Fatalf("Message = %q", result.Message)
	}
	if !result.CleanupExecuted {
		t.Fatal("CleanupExecuted should be true after failure when cleanup is enabled")
	}
	if got := result.StepResults[len(result.StepResults)-2]; got.Name != "query" || got.Success {
		t.Fatalf("query step = %#v", got)
	}
}

func TestRWProbe_Run_FailsWhenLoadCollectionFails(t *testing.T) {
	t.Parallel()

	testDB := "milvus_health_test_1700000000000000000"
	client := &platformmilvus.FakeClient{
		InsertResults: map[string]map[string]platformmilvus.InsertResult{
			testDB: {"rw_probe": {InsertCount: 2}},
		},
		LoadErrs: map[string]map[string]error{
			testDB: {"rw_probe": errors.New("load timeout")},
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, true, 2, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if !strings.Contains(result.Message, "load collection failed") {
		t.Fatalf("Message = %q", result.Message)
	}
	if !result.CleanupExecuted {
		t.Fatal("CleanupExecuted should be true after load failure when cleanup is enabled")
	}
	if got := result.StepResults[len(result.StepResults)-2]; got.Name != "load-collection" || got.Success {
		t.Fatalf("load step = %#v", got)
	}
}

func TestRWProbe_Run_FailsWhenCreateIndexFails(t *testing.T) {
	t.Parallel()

	testDB := "milvus_health_test_1700000000000000000"
	client := &platformmilvus.FakeClient{
		InsertResults: map[string]map[string]platformmilvus.InsertResult{
			testDB: {"rw_probe": {InsertCount: 2}},
		},
		CreateIndexErrs: map[string]map[string]error{
			testDB: {"rw_probe": errors.New("index build timeout")},
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, true, 2, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if !strings.Contains(result.Message, "create index failed") {
		t.Fatalf("Message = %q", result.Message)
	}
	if !result.CleanupExecuted {
		t.Fatal("CleanupExecuted should be true after create index failure when cleanup is enabled")
	}
	if got := result.StepResults[len(result.StepResults)-2]; got.Name != "create-index" || got.Success {
		t.Fatalf("create-index step = %#v", got)
	}
}

func TestRWProbe_Run_FailsWhenCleanupFails(t *testing.T) {
	t.Parallel()

	testDB := "milvus_health_test_1700000000000000000"
	client := &platformmilvus.FakeClient{
		InsertResults: map[string]map[string]platformmilvus.InsertResult{
			testDB: {"rw_probe": {InsertCount: 1}},
		},
		QueryResults: map[string]map[string]platformmilvus.QueryResult{
			testDB: {"rw_probe": {ResultCount: 1}},
		},
		DropCollectionErrs: map[string]map[string]error{
			testDB: {"rw_probe": errors.New("drop collection failed")},
		},
	}

	result, err := (probes.DefaultRWProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
		Now:     fixedProbeTime(),
	}).Run(context.Background(), rwProbeConfig(true, true, 1, 4))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if !result.CleanupExecuted {
		t.Fatal("CleanupExecuted should be true when cleanup was attempted")
	}
	last := result.StepResults[len(result.StepResults)-1]
	if last.Name != "cleanup" || last.Success {
		t.Fatalf("cleanup step = %#v", last)
	}
	if !strings.Contains(result.Message, "cleanup failed") {
		t.Fatalf("Message = %q", result.Message)
	}
}

func rwProbeConfig(enabled, cleanup bool, insertRows, vectorDim int) *model.Config {
	return &model.Config{
		Cluster: model.ClusterConfig{
			Milvus: model.MilvusConfig{URI: "127.0.0.1:19530"},
		},
		TimeoutSec: 30,
		Probe: model.ProbeConfig{
			RW: model.RWProbeConfig{
				Enabled:            enabled,
				TestDatabasePrefix: "milvus_health_test",
				Cleanup:            cleanup,
				InsertRows:         insertRows,
				VectorDim:          vectorDim,
			},
		},
	}
}

func fixedProbeTime() func() time.Time {
	return func() time.Time {
		return time.Unix(1700000000, 0)
	}
}

func stepNames(steps []model.ProbeStepResult) []string {
	names := make([]string, 0, len(steps))
	for _, step := range steps {
		names = append(names, step.Name)
	}
	return names
}

func containsOperation(operations []string, want string) bool {
	for _, operation := range operations {
		if operation == want {
			return true
		}
	}
	return false
}
