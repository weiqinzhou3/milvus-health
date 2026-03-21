package platform_test

import (
	"context"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/platform"
)

func TestFakeMilvusClient_PingSuccess(t *testing.T) {
	t.Parallel()

	client := &platform.FakeMilvusClient{}
	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}

func TestFakeK8sClient_ListPods(t *testing.T) {
	t.Parallel()

	client := &platform.FakeK8sClient{
		Pods: []platform.PodInfo{{Name: "milvus-0", Phase: "Running", Ready: true, RestartCount: 1}},
	}

	pods, err := client.ListPods(context.Background(), "milvus")
	if err != nil {
		t.Fatalf("ListPods() error = %v", err)
	}
	if len(pods) != 1 || pods[0].Name != "milvus-0" {
		t.Fatalf("ListPods() = %#v", pods)
	}
}
