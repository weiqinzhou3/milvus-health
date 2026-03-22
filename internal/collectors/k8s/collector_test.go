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

	client := &platformk8s.FakeClient{
		Pods: []platformk8s.Pod{{
			Name:          "milvus-0",
			Phase:         "Running",
			Ready:         true,
			RestartCount:  1,
			CPURequest:    "500m",
			CPULimit:      "1000m",
			MemoryRequest: "512Mi",
			MemoryLimit:   "1Gi",
		}},
		Services:  []platformk8s.Service{{Name: "attu", Type: "NodePort", Ports: []string{"3000:30031/tcp"}}},
		Endpoints: []platformk8s.Endpoint{{Name: "milvus-abc", Addresses: []string{"10.0.0.1"}}},
		Metrics: platformk8s.PlatformMetricsResult{
			Available: true,
			Metrics: []platformk8s.PodMetric{{
				PodName:     "milvus-0",
				CPUUsage:    "125m",
				MemoryUsage: "256Mi",
			}},
		},
	}
	collector := k8s.DefaultCollector{
		Factory: platformk8s.FakeClientFactory{
			Client: client,
		},
	}

	inventory, err := collector.Collect(context.Background(), testConfig(model.K8sResourceUsageSourceMetricsAPI))
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if len(inventory.Pods) != 1 || inventory.Pods[0].Name != "milvus-0" {
		t.Fatalf("Pods = %#v", inventory.Pods)
	}
	if !inventory.ResourceUsageAvailable || inventory.MetricsAvailablePodCount != 1 || inventory.ResourceUsagePartial {
		t.Fatalf("Resource usage summary = %#v", inventory)
	}
	if inventory.Pods[0].CPUUsage != "125m" || inventory.Pods[0].MemoryUsage != "256Mi" {
		t.Fatalf("Pod metrics = %#v", inventory.Pods[0])
	}
	if inventory.Pods[0].CPULimitRatio == nil || *inventory.Pods[0].CPULimitRatio != 0.125 {
		t.Fatalf("Pod cpu limit ratio = %#v", inventory.Pods[0].CPULimitRatio)
	}
	if inventory.Pods[0].MemoryRequestRatio == nil || *inventory.Pods[0].MemoryRequestRatio != 0.5 {
		t.Fatalf("Pod memory request ratio = %#v", inventory.Pods[0].MemoryRequestRatio)
	}
	if len(inventory.Services) != 1 || inventory.Services[0].Name != "attu" || inventory.Services[0].Ports[0] != "3000:30031/tcp" {
		t.Fatalf("Services = %#v", inventory.Services)
	}
	if len(inventory.Endpoints) != 1 || inventory.Endpoints[0].Name != "milvus-abc" {
		t.Fatalf("Endpoints = %#v", inventory.Endpoints)
	}
	if inventory.ResourceUsageSource != model.K8sResourceUsageSourceMetricsAPI {
		t.Fatalf("ResourceUsageSource = %q", inventory.ResourceUsageSource)
	}
	if client.MetricsCalls != 1 {
		t.Fatalf("MetricsCalls = %d, want 1", client.MetricsCalls)
	}
}

func TestCollector_DisabledSource_SkipsPodMetricsAndPreservesStaticResources(t *testing.T) {
	t.Parallel()

	client := &platformk8s.FakeClient{
		Pods: []platformk8s.Pod{{
			Name:          "milvus-0",
			Phase:         "Running",
			Ready:         true,
			CPURequest:    "500m",
			CPULimit:      "1000m",
			MemoryRequest: "512Mi",
			MemoryLimit:   "1Gi",
		}},
		Metrics: platformk8s.PlatformMetricsResult{
			Available: true,
			Metrics: []platformk8s.PodMetric{{
				PodName:     "milvus-0",
				CPUUsage:    "125m",
				MemoryUsage: "256Mi",
			}},
		},
	}
	collector := k8s.DefaultCollector{
		Factory: platformk8s.FakeClientFactory{
			Client: client,
		},
	}

	inventory, err := collector.Collect(context.Background(), testConfig(model.K8sResourceUsageSourceDisabled))
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if inventory.ResourceUsageAvailable {
		t.Fatalf("ResourceUsageAvailable = true, want false")
	}
	if inventory.ResourceUnavailableReason != model.MetricsUnavailableReasonDisabled {
		t.Fatalf("ResourceUnavailableReason = %q", inventory.ResourceUnavailableReason)
	}
	if inventory.ResourceUsageSource != model.K8sResourceUsageSourceDisabled {
		t.Fatalf("ResourceUsageSource = %q", inventory.ResourceUsageSource)
	}
	if client.MetricsCalls != 0 {
		t.Fatalf("MetricsCalls = %d, want 0", client.MetricsCalls)
	}
	if inventory.Pods[0].CPURequest != "500m" || inventory.Pods[0].CPULimit != "1000m" {
		t.Fatalf("static resources = %#v", inventory.Pods[0])
	}
	if inventory.Pods[0].CPULimitRatio != nil || inventory.Pods[0].MemoryLimitRatio != nil {
		t.Fatalf("Ratios should be nil when metrics are unavailable: %#v", inventory.Pods[0])
	}
	if inventory.Pods[0].CPUUsage != "" || inventory.Pods[0].MemoryUsage != "" {
		t.Fatalf("usage should stay empty when source is disabled: %#v", inventory.Pods[0])
	}
}

func TestCollector_AutoSource_CurrentlyUsesMetricsAPI(t *testing.T) {
	t.Parallel()

	client := &platformk8s.FakeClient{
		Pods: []platformk8s.Pod{{
			Name:          "milvus-0",
			Phase:         "Running",
			Ready:         true,
			CPURequest:    "500m",
			CPULimit:      "1000m",
			MemoryRequest: "512Mi",
			MemoryLimit:   "1Gi",
		}},
		Metrics: platformk8s.PlatformMetricsResult{
			Available: true,
			Metrics: []platformk8s.PodMetric{{
				PodName:     "milvus-0",
				CPUUsage:    "125m",
				MemoryUsage: "256Mi",
			}},
		},
	}
	collector := k8s.DefaultCollector{
		Factory: platformk8s.FakeClientFactory{
			Client: client,
		},
	}

	inventory, err := collector.Collect(context.Background(), testConfig(model.K8sResourceUsageSourceAuto))
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if inventory.ResourceUsageSource != model.K8sResourceUsageSourceAuto {
		t.Fatalf("ResourceUsageSource = %q", inventory.ResourceUsageSource)
	}
	if !inventory.ResourceUsageAvailable || inventory.MetricsAvailablePodCount != 1 {
		t.Fatalf("inventory = %#v", inventory)
	}
	if client.MetricsCalls != 1 {
		t.Fatalf("MetricsCalls = %d, want 1", client.MetricsCalls)
	}
}

func TestCollector_MetricsAPISource_RecordsMetricsUnavailableReasonWithoutFailing(t *testing.T) {
	t.Parallel()

	client := &platformk8s.FakeClient{
		Pods: []platformk8s.Pod{{
			Name:          "milvus-0",
			Phase:         "Running",
			Ready:         true,
			CPURequest:    "500m",
			CPULimit:      "1000m",
			MemoryRequest: "512Mi",
			MemoryLimit:   "1Gi",
		}},
		Metrics: platformk8s.PlatformMetricsResult{
			Available:         false,
			UnavailableReason: "metrics-server not found",
		},
	}
	collector := k8s.DefaultCollector{
		Factory: platformk8s.FakeClientFactory{
			Client: client,
		},
	}

	inventory, err := collector.Collect(context.Background(), testConfig(model.K8sResourceUsageSourceMetricsAPI))
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if client.MetricsCalls != 1 {
		t.Fatalf("MetricsCalls = %d, want 1", client.MetricsCalls)
	}
	if inventory.ResourceUsageAvailable {
		t.Fatalf("ResourceUsageAvailable = true, want false")
	}
	if inventory.ResourceUnavailableReason != model.MetricsUnavailableReasonNotFound {
		t.Fatalf("ResourceUnavailableReason = %q", inventory.ResourceUnavailableReason)
	}
	if inventory.Pods[0].CPULimitRatio != nil || inventory.Pods[0].MemoryLimitRatio != nil {
		t.Fatalf("Ratios should be nil when metrics are unavailable: %#v", inventory.Pods[0])
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

	_, err := collector.Collect(context.Background(), testConfig(model.K8sResourceUsageSourceAuto))
	var appErr *model.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Collect() error = %T, want *model.AppError", err)
	}
	if appErr.Code != model.ErrCodeK8sCollect {
		t.Fatalf("AppError.Code = %s, want %s", appErr.Code, model.ErrCodeK8sCollect)
	}
}

func testConfig(source model.K8sResourceUsageSource) *model.Config {
	return &model.Config{
		K8s: model.K8sConfig{
			Namespace:  "milvus",
			Kubeconfig: "/tmp/fake",
			ResourceUsage: model.K8sResourceUsageConfig{
				Source: source,
			},
		},
		TimeoutSec: 5,
	}
}
