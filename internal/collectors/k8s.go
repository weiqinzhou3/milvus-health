package collectors

import (
	"context"
	"fmt"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/model"
	"github.com/weiqinzhou3/milvus-health/internal/platform"
)

type DefaultK8sInventoryCollector struct {
	Factory platform.K8sClientFactory
}

func (c DefaultK8sInventoryCollector) Collect(ctx context.Context, cfg *model.Config) (model.K8sInventory, error) {
	if cfg == nil {
		return model.K8sInventory{}, &model.AppError{Code: model.ErrCodeK8sCollect, Message: "config is nil"}
	}
	if c.Factory == nil {
		return model.K8sInventory{}, &model.AppError{Code: model.ErrCodeK8sCollect, Message: "k8s client factory is nil"}
	}

	client, err := c.Factory.New(ctx, cfg.K8s, time.Duration(cfg.TimeoutSec)*time.Second)
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

	inventory := model.K8sInventory{Namespace: namespace}
	for _, pod := range pods {
		inventory.Pods = append(inventory.Pods, model.PodStatusSummary{
			Name:         pod.Name,
			Phase:        pod.Phase,
			Ready:        pod.Ready,
			RestartCount: pod.RestartCount,
		})
	}
	for _, svc := range services {
		inventory.Services = append(inventory.Services, model.ServiceInventory{
			Name:  svc.Name,
			Type:  svc.Type,
			Ports: append([]string(nil), svc.Ports...),
		})
	}
	for _, ep := range endpoints {
		inventory.Endpoints = append(inventory.Endpoints, model.EndpointInventory{
			Name:      ep.Name,
			Addresses: append([]string(nil), ep.Addresses...),
		})
	}

	return inventory, nil
}
