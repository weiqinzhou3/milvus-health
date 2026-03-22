package k8s_test

import (
	"context"
	"errors"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/collectors/k8s"
	"github.com/weiqinzhou3/milvus-health/internal/model"
	platformk8s "github.com/weiqinzhou3/milvus-health/internal/platform/k8s"
)

func TestCollector_ReturnsInventory_FromFakeClient(t *testing.T) {
	t.Parallel()

	collector := k8s.DefaultCollector{
		Factory: platformk8s.FakeClientFactory{
			Client: &platformk8s.FakeClient{
				Pods:      []platformk8s.Pod{{Name: "milvus-0", Phase: "Running", Ready: true, RestartCount: 1}},
				Services:  []platformk8s.Service{{Name: "milvus", Type: "ClusterIP", Ports: []string{"19530/tcp"}}},
				Endpoints: []platformk8s.Endpoint{{Name: "milvus-abc", Addresses: []string{"10.0.0.1"}}},
			},
		},
	}

	inventory, err := collector.Collect(context.Background(), testConfig())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(inventory.Pods) != 1 || inventory.Pods[0].Name != "milvus-0" {
		t.Fatalf("Pods = %#v", inventory.Pods)
	}
	if len(inventory.Services) != 1 || inventory.Services[0].Name != "milvus" {
		t.Fatalf("Services = %#v", inventory.Services)
	}
	if len(inventory.Endpoints) != 1 || inventory.Endpoints[0].Name != "milvus-abc" {
		t.Fatalf("Endpoints = %#v", inventory.Endpoints)
	}
}

func TestCollector_ReturnsAppError_WhenClientFails(t *testing.T) {
	t.Parallel()

	collector := k8s.DefaultCollector{
		Factory: platformk8s.FakeClientFactory{
			Client: &platformk8s.FakeClient{
				PodsErr: errors.New("forbidden"),
			},
		},
	}

	_, err := collector.Collect(context.Background(), testConfig())
	var appErr *model.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Collect() error = %T, want *model.AppError", err)
	}
	if appErr.Code != model.ErrCodeK8sCollect {
		t.Fatalf("AppError.Code = %s, want %s", appErr.Code, model.ErrCodeK8sCollect)
	}
}

func testConfig() *model.Config {
	return &model.Config{
		K8s: model.K8sConfig{
			Namespace:  "milvus",
			Kubeconfig: "/tmp/fake",
		},
		TimeoutSec: 5,
	}
}
