package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/model"
	platformk8s "github.com/weiqinzhou3/milvus-health/internal/platform/k8s"
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

	inventory := model.K8sInventory{Namespace: namespace}
	for _, pod := range pods {
		inventory.Pods = append(inventory.Pods, model.PodStatusSummary{
			Name:         pod.Name,
			Phase:        pod.Phase,
			Ready:        pod.Ready,
			RestartCount: pod.RestartCount,
		})
	}
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

	return inventory, nil
}
