package milvus_test

import (
	"context"
	"strings"
	"testing"

	platformmilvus "github.com/weiqinzhou3/milvus-health/internal/platform/milvus"
)

func TestFakeClient_ListDatabasesReturnsCopy(t *testing.T) {
	t.Parallel()

	client := &platformmilvus.FakeClient{
		Databases: []string{"default", "analytics"},
	}

	databases, err := client.ListDatabases(context.Background())
	if err != nil {
		t.Fatalf("ListDatabases() error = %v", err)
	}
	databases[0] = "mutated"
	if client.Databases[0] != "default" {
		t.Fatalf("FakeClient should return a copy, got %#v", client.Databases)
	}
}

func TestFakeClient_CloseMarksClosed(t *testing.T) {
	t.Parallel()

	client := &platformmilvus.FakeClient{}
	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !client.Closed {
		t.Fatal("Close() should mark client closed")
	}
}

func TestFakeClient_GetCollectionRowCount(t *testing.T) {
	t.Parallel()

	client := &platformmilvus.FakeClient{
		RowCounts: map[string]map[string]int64{
			"default": {"book": 42},
		},
	}

	got, err := client.GetCollectionRowCount(context.Background(), "default", "book")
	if err != nil {
		t.Fatalf("GetCollectionRowCount() error = %v", err)
	}
	if got != 42 {
		t.Fatalf("GetCollectionRowCount() = %d, want 42", got)
	}
}

func TestFakeClient_QueryRequiresLoadedCollection(t *testing.T) {
	t.Parallel()

	client := &platformmilvus.FakeClient{
		LoadStates: map[string]map[string]platformmilvus.LoadState{
			"default": {"book": platformmilvus.LoadStateNotLoad},
		},
		QueryResults: map[string]map[string]platformmilvus.QueryResult{
			"default": {"book": {ResultCount: 1}},
		},
	}

	req := platformmilvus.QueryRequest{
		Database:   "default",
		Collection: "book",
		Expr:       "id >= 0",
		Limit:      1,
	}
	if _, err := client.Query(context.Background(), req); err == nil || !strings.Contains(err.Error(), "requires loaded collection") {
		t.Fatalf("Query() error = %v, want loaded contract failure", err)
	}
	if err := client.LoadCollection(context.Background(), "default", "book"); err != nil {
		t.Fatalf("LoadCollection() error = %v", err)
	}

	got, err := client.Query(context.Background(), req)
	if err != nil {
		t.Fatalf("Query() after load error = %v", err)
	}
	if got.ResultCount != 1 {
		t.Fatalf("Query() after load = %#v, want ResultCount=1", got)
	}
}
