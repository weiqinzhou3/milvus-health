package probes_test

import (
	"context"
	"errors"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/model"
	platformmilvus "github.com/weiqinzhou3/milvus-health/internal/platform/milvus"
	"github.com/weiqinzhou3/milvus-health/internal/probes"
)

func TestBusinessReadProbe_Run_SkipsWhenNoTargets(t *testing.T) {
	t.Parallel()

	probe := probes.DefaultBusinessReadProbe{}
	cfg := &model.Config{
		Probe: model.ProbeConfig{
			Read: model.ReadProbeConfig{MinSuccessTargets: 1},
		},
	}

	result, err := probe.Run(context.Background(), cfg, probes.ProbeScope{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusSkip || result.Message != "not configured" {
		t.Fatalf("result = %#v", result)
	}
	if !result.Enabled || result.Executed {
		t.Fatalf("enabled/executed = %#v, want enabled=true executed=false", result)
	}
	if result.Check == nil || result.Check.Name != "business-read-probe" || result.Check.Status != model.CheckStatusSkip {
		t.Fatalf("Check = %#v", result.Check)
	}
}

func TestBusinessReadProbe_Run_SkipsWhenDisabledByConfig(t *testing.T) {
	t.Parallel()

	probe := probes.DefaultBusinessReadProbe{}
	cfg := &model.Config{
		Probe: model.ProbeConfig{
			Read: model.ReadProbeConfig{
				Enabled: boolPtr(false),
				Targets: []model.ReadProbeTarget{{
					Database:   "default",
					Collection: "book",
					QueryExpr:  "id >= 0",
				}},
			},
		},
	}

	result, err := probe.Run(context.Background(), cfg, probes.ProbeScope{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusSkip {
		t.Fatalf("Status = %s, want skip", result.Status)
	}
	if result.Message != "disabled by config" {
		t.Fatalf("Message = %q, want %q", result.Message, "disabled by config")
	}
	if result.Enabled {
		t.Fatalf("Enabled = %t, want false", result.Enabled)
	}
	if result.Executed {
		t.Fatalf("Executed = %t, want false", result.Executed)
	}
	if result.Check == nil {
		t.Fatal("Check = nil, want disabled business-read-probe check")
	}
	if result.Check.Name != "business-read-probe" || result.Check.Status != model.CheckStatusSkip || result.Check.Message != "disabled by config" {
		t.Fatalf("Check = %#v", result.Check)
	}
}

func TestBusinessReadProbe_Run_QuerySuccess(t *testing.T) {
	t.Parallel()

	client := &platformmilvus.FakeClient{
		Descriptions: map[string]map[string]platformmilvus.CollectionDescription{
			"default": {
				"book": {ID: 1001, Name: "book", Fields: []platformmilvus.CollectionField{{Name: "id", DataType: "Int64", IsPrimaryKey: true}}},
			},
		},
		LoadStates: map[string]map[string]platformmilvus.LoadState{
			"default": {"book": platformmilvus.LoadStateLoaded},
		},
		RowCounts: map[string]map[string]int64{
			"default": {"book": 123},
		},
		QueryResults: map[string]map[string]platformmilvus.QueryResult{
			"default": {"book": {ResultCount: 1}},
		},
	}
	cfg := &model.Config{
		Cluster: model.ClusterConfig{
			Milvus: model.MilvusConfig{URI: "127.0.0.1:19530"},
		},
		TimeoutSec: 30,
		Probe: model.ProbeConfig{
			Read: model.ReadProbeConfig{
				MinSuccessTargets: 1,
				Targets: []model.ReadProbeTarget{{
					Database:   "default",
					Collection: "book",
					QueryExpr:  "id >= 0",
				}},
			},
		},
	}

	result, err := (probes.DefaultBusinessReadProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
	}).Run(context.Background(), cfg, probes.ProbeScope{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusPass {
		t.Fatalf("Status = %s, want pass", result.Status)
	}
	if !result.Enabled || !result.Executed {
		t.Fatalf("enabled/executed = %#v, want enabled=true executed=true", result)
	}
	if result.SuccessfulTargets != 1 || len(result.Targets) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if result.Targets[0].Action != model.ProbeActionQuery || !result.Targets[0].Success {
		t.Fatalf("target = %#v", result.Targets[0])
	}
	if result.Check == nil || result.Check.Name != "business-read-probe" || result.Check.Status != model.CheckStatusPass {
		t.Fatalf("Check = %#v", result.Check)
	}
	if client.LastQueryRequest.Expr != "id >= 0" || client.LastQueryRequest.Limit != 1 {
		t.Fatalf("LastQueryRequest = %#v", client.LastQueryRequest)
	}
}

func TestBusinessReadProbe_Run_QueryAndSearchFailWhenAnnsFieldMissing(t *testing.T) {
	t.Parallel()

	client := &platformmilvus.FakeClient{
		Descriptions: map[string]map[string]platformmilvus.CollectionDescription{
			"default": {
				"book": {ID: 1001, Name: "book", Fields: []platformmilvus.CollectionField{{Name: "id", DataType: "Int64", IsPrimaryKey: true}}},
			},
		},
		LoadStates: map[string]map[string]platformmilvus.LoadState{
			"default": {"book": platformmilvus.LoadStateLoaded},
		},
		RowCounts: map[string]map[string]int64{
			"default": {"book": 123},
		},
		QueryResults: map[string]map[string]platformmilvus.QueryResult{
			"default": {"book": {ResultCount: 1}},
		},
	}
	cfg := &model.Config{
		Cluster: model.ClusterConfig{
			Milvus: model.MilvusConfig{URI: "127.0.0.1:19530"},
		},
		TimeoutSec: 30,
		Probe: model.ProbeConfig{
			Read: model.ReadProbeConfig{
				MinSuccessTargets: 1,
				Targets: []model.ReadProbeTarget{{
					Database:   "default",
					Collection: "book",
					QueryExpr:  "id >= 0",
					AnnsField:  "vector",
					TopK:       3,
				}},
			},
		},
	}

	result, err := (probes.DefaultBusinessReadProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
	}).Run(context.Background(), cfg, probes.ProbeScope{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if result.Targets[0].Action != model.ProbeActionQueryAndSearch {
		t.Fatalf("target = %#v", result.Targets[0])
	}
	if result.Targets[0].Success {
		t.Fatalf("target should fail: %#v", result.Targets[0])
	}
}

func TestBusinessReadProbe_Run_WarnsWhenSuccessBelowMinimum(t *testing.T) {
	t.Parallel()

	client := &platformmilvus.FakeClient{
		Descriptions: map[string]map[string]platformmilvus.CollectionDescription{
			"default": {
				"book":  {ID: 1001, Name: "book", Fields: []platformmilvus.CollectionField{{Name: "id", DataType: "Int64", IsPrimaryKey: true}}},
				"movie": {ID: 1002, Name: "movie", Fields: []platformmilvus.CollectionField{{Name: "id", DataType: "Int64", IsPrimaryKey: true}}},
			},
		},
		LoadStates: map[string]map[string]platformmilvus.LoadState{
			"default": {"book": platformmilvus.LoadStateLoaded, "movie": platformmilvus.LoadStateLoaded},
		},
		RowCounts: map[string]map[string]int64{
			"default": {"book": 1, "movie": 2},
		},
		QueryResults: map[string]map[string]platformmilvus.QueryResult{
			"default": {"book": {ResultCount: 1}},
		},
		QueryErrs: map[string]map[string]error{
			"default": {"movie": errors.New("query failed")},
		},
	}
	cfg := &model.Config{
		Cluster: model.ClusterConfig{
			Milvus: model.MilvusConfig{URI: "127.0.0.1:19530"},
		},
		TimeoutSec: 30,
		Probe: model.ProbeConfig{
			Read: model.ReadProbeConfig{
				MinSuccessTargets: 2,
				Targets: []model.ReadProbeTarget{
					{Database: "default", Collection: "book", QueryExpr: "id >= 0"},
					{Database: "default", Collection: "movie", QueryExpr: "id >= 0"},
				},
			},
		},
	}

	result, err := (probes.DefaultBusinessReadProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
	}).Run(context.Background(), cfg, probes.ProbeScope{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusWarn {
		t.Fatalf("Status = %s, want warn", result.Status)
	}
	if result.SuccessfulTargets != 1 || result.ConfiguredTargets != 2 {
		t.Fatalf("result = %#v", result)
	}
}

func TestBusinessReadProbe_Run_FailsWhenNoTargetsSucceedEvenIfMinSuccessTargetsZero(t *testing.T) {
	t.Parallel()

	client := &platformmilvus.FakeClient{
		Descriptions: map[string]map[string]platformmilvus.CollectionDescription{
			"default": {
				"book": {ID: 1001, Name: "book", Fields: []platformmilvus.CollectionField{{Name: "id", DataType: "Int64", IsPrimaryKey: true}}},
			},
		},
		QueryErrs: map[string]map[string]error{
			"default": {"book": errors.New("query failed")},
		},
	}
	cfg := &model.Config{
		Cluster: model.ClusterConfig{
			Milvus: model.MilvusConfig{URI: "127.0.0.1:19530"},
		},
		TimeoutSec: 30,
		Probe: model.ProbeConfig{
			Read: model.ReadProbeConfig{
				MinSuccessTargets: 0,
				Targets: []model.ReadProbeTarget{{
					Database:   "default",
					Collection: "book",
					QueryExpr:  "id >= 0",
				}},
			},
		},
	}

	result, err := (probes.DefaultBusinessReadProbe{
		Factory: platformmilvus.FakeClientFactory{Client: client},
	}).Run(context.Background(), cfg, probes.ProbeScope{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != model.CheckStatusFail {
		t.Fatalf("Status = %s, want fail", result.Status)
	}
	if result.Message != "no read probe targets succeeded" {
		t.Fatalf("Message = %q, want %q", result.Message, "no read probe targets succeeded")
	}
}

func boolPtr(v bool) *bool {
	return &v
}
