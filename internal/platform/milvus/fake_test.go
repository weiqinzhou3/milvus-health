package milvus_test

import (
	"context"
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
