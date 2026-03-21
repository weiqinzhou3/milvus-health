package collectors_test

import (
	"context"
	"errors"
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/collectors"
	"github.com/weiqinzhou3/milvus-health/internal/model"
	"github.com/weiqinzhou3/milvus-health/internal/platform"
)

func TestK8sInventoryCollector_ReturnsInventory_FromFakeClient(t *testing.T) {
	t.Parallel()

	collector := collectors.DefaultK8sInventoryCollector{
		Factory: platform.FakeK8sClientFactory{
			Client: &platform.FakeK8sClient{
				Pods:      []platform.PodInfo{{Name: "milvus-0", Phase: "Running", Ready: true, RestartCount: 1}},
				Services:  []platform.ServiceInfo{{Name: "milvus", Type: "ClusterIP", Ports: []string{"19530/tcp"}}},
				Endpoints: []platform.EndpointInfo{{Name: "milvus", Addresses: []string{"10.0.0.1"}}},
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
}

func TestK8sInventoryCollector_ReturnsAppError_WhenClientFails(t *testing.T) {
	t.Parallel()

	collector := collectors.DefaultK8sInventoryCollector{
		Factory: platform.FakeK8sClientFactory{
			Client: &platform.FakeK8sClient{
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
