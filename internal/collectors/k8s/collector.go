package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/model"
	platformk8s "github.com/weiqinzhou3/milvus-health/internal/platform/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Collector interface {
	Collect(ctx context.Context, cfg *model.Config) (model.K8sInventory, error)
}

type DefaultCollector struct {
	Factory platformk8s.ClientFactory
}

func (c DefaultCollector) Collect(ctx context.Context, cfg *model.Config) (model.K8sInventory, error) {
	if cfg == nil {
		return model.K8sInventory{}, &model.AppError{Code: model.ErrCodeK8sCollect, Message: "config is nil"}
	}
	if c.Factory == nil {
		return model.K8sInventory{}, &model.AppError{Code: model.ErrCodeK8sCollect, Message: "k8s client factory is nil"}
	}

	client, err := c.Factory.New(ctx, platformk8s.Config{
		Namespace:  cfg.K8s.Namespace,
		Kubeconfig: cfg.K8s.Kubeconfig,
	}, time.Duration(cfg.TimeoutSec)*time.Second)
	if err != nil {
		return model.K8sInventory{}, &model.AppError{Code: model.ErrCodeK8sCollect, Message: fmt.Sprintf("create k8s client: %v", err), Cause: err}
	}

	namespace := cfg.K8s.Namespace
	pods, err := client.ListPods(ctx, namespace)
	if err != nil {
		return model.K8sInventory{}, &model.AppError{Code: model.ErrCodeK8sCollect, Message: fmt.Sprintf("list pods: %v", err), Cause: err}
	}
	services, err := client.ListServices(ctx, namespace)
	if err != nil {
		return model.K8sInventory{}, &model.AppError{Code: model.ErrCodeK8sCollect, Message: fmt.Sprintf("list services: %v", err), Cause: err}
	}
	endpoints, err := client.ListEndpoints(ctx, namespace)
	if err != nil {
		return model.K8sInventory{}, &model.AppError{Code: model.ErrCodeK8sCollect, Message: fmt.Sprintf("list endpoints: %v", err), Cause: err}
	}
	inventory := model.K8sInventory{
		Namespace:           namespace,
		ResourceUsageSource: resolveResourceUsageSource(cfg),
	}
	for _, pod := range pods {
		inventory.Pods = append(inventory.Pods, model.PodStatusSummary{
			Name:          pod.Name,
			Phase:         pod.Phase,
			Ready:         pod.Ready,
			RestartCount:  pod.RestartCount,
			CPURequest:    pod.CPURequest,
			CPULimit:      pod.CPULimit,
			MemoryRequest: pod.MemoryRequest,
			MemoryLimit:   pod.MemoryLimit,
		})
		if pod.Ready {
			inventory.ReadyPodCount++
		} else {
			inventory.NotReadyPodCount++
		}
	}
	inventory.TotalPodCount = len(inventory.Pods)
	for _, service := range services {
		inventory.Services = append(inventory.Services, model.ServiceInventory{
			Name:  service.Name,
			Type:  service.Type,
			Ports: append([]string(nil), service.Ports...),
		})
	}
	for _, endpoint := range endpoints {
		inventory.Endpoints = append(inventory.Endpoints, model.EndpointInventory{
			Name:      endpoint.Name,
			Addresses: append([]string(nil), endpoint.Addresses...),
		})
	}
	if inventory.ResourceUsageSource == model.K8sResourceUsageSourceDisabled {
		inventory.ResourceUsageAvailable = false
		inventory.ResourceUnavailableReason = model.MetricsUnavailableReasonDisabled
		return inventory, nil
	}

	metricsResult, err := client.ListPodMetrics(ctx, namespace)
	if err != nil {
		return model.K8sInventory{}, &model.AppError{Code: model.ErrCodeK8sCollect, Message: fmt.Sprintf("list pod metrics: %v", err), Cause: err}
	}
	inventory.ResourceUsageAvailable = metricsResult.Available
	inventory.Pods, inventory.ResourceUnavailableReason, inventory.MetricsAvailablePodCount = mergeMetrics(inventory.Pods, metricsResult)
	inventory.ResourceUsagePartial = inventory.MetricsAvailablePodCount > 0 && inventory.MetricsAvailablePodCount < len(inventory.Pods)

	return inventory, nil
}

func resolveResourceUsageSource(cfg *model.Config) model.K8sResourceUsageSource {
	if cfg == nil || cfg.K8s.ResourceUsage.Source == "" {
		return model.K8sResourceUsageSourceAuto
	}
	return cfg.K8s.ResourceUsage.Source
}

func mergeMetrics(
	pods []model.PodStatusSummary,
	result platformk8s.PlatformMetricsResult,
) ([]model.PodStatusSummary, model.MetricsUnavailableReason, int) {
	metricsByPod := make(map[string]platformk8s.PodMetric, len(result.Metrics))
	for _, metric := range result.Metrics {
		metricsByPod[metric.PodName] = metric
	}

	count := 0
	for i := range pods {
		metric, ok := metricsByPod[pods[i].Name]
		if !ok {
			continue
		}
		pods[i].CPUUsage = metric.CPUUsage
		pods[i].MemoryUsage = metric.MemoryUsage
		pods[i].CPULimitRatio = calculateRatio(metric.CPUUsage, pods[i].CPULimit, corev1.ResourceCPU)
		pods[i].MemoryLimitRatio = calculateRatio(metric.MemoryUsage, pods[i].MemoryLimit, corev1.ResourceMemory)
		pods[i].CPURequestRatio = calculateRatio(metric.CPUUsage, pods[i].CPURequest, corev1.ResourceCPU)
		pods[i].MemoryRequestRatio = calculateRatio(metric.MemoryUsage, pods[i].MemoryRequest, corev1.ResourceMemory)
		count++
	}

	if result.Available {
		return pods, model.MetricsUnavailableReasonNone, count
	}
	return pods, mapMetricsReason(result.UnavailableReason), count
}

func mapMetricsReason(raw string) model.MetricsUnavailableReason {
	switch raw {
	case string(model.MetricsUnavailableReasonNotFound):
		return model.MetricsUnavailableReasonNotFound
	case string(model.MetricsUnavailableReasonPermissionDenied):
		return model.MetricsUnavailableReasonPermissionDenied
	case "":
		return model.MetricsUnavailableReasonNone
	default:
		return model.MetricsUnavailableReasonUnknown
	}
}

func calculateRatio(usage, limit string, resourceName corev1.ResourceName) *float64 {
	if usage == "" || limit == "" {
		return nil
	}
	usageQty, err := resource.ParseQuantity(usage)
	if err != nil {
		return nil
	}
	limitQty, err := resource.ParseQuantity(limit)
	if err != nil {
		return nil
	}

	var numerator float64
	var denominator float64
	switch resourceName {
	case corev1.ResourceCPU:
		numerator = float64(usageQty.MilliValue())
		denominator = float64(limitQty.MilliValue())
	default:
		numerator = float64(usageQty.Value())
		denominator = float64(limitQty.Value())
	}
	if denominator <= 0 {
		return nil
	}
	ratio := numerator / denominator
	return &ratio
}
