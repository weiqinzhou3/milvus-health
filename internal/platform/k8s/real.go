package k8s

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type ClientGoClientFactory struct{}

func (ClientGoClientFactory) New(ctx context.Context, cfg Config, timeout time.Duration) (Client, error) {
	_ = ctx
	restCfg, err := buildRestConfig(cfg, timeout)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	return &clientGoClient{clientset: clientset}, nil
}

type clientGoClient struct {
	clientset kubernetes.Interface
}

func (c *clientGoClient) ListPods(ctx context.Context, namespace string) ([]Pod, error) {
	list, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods := make([]Pod, 0, len(list.Items))
	for _, pod := range list.Items {
		pods = append(pods, Pod{
			Name:         pod.Name,
			Phase:        string(pod.Status.Phase),
			Ready:        podReady(pod),
			RestartCount: podRestartCount(pod),
		})
	}
	sort.Slice(pods, func(i, j int) bool { return pods[i].Name < pods[j].Name })
	return pods, nil
}

func (c *clientGoClient) ListServices(ctx context.Context, namespace string) ([]Service, error) {
	list, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	services := make([]Service, 0, len(list.Items))
	for _, service := range list.Items {
		services = append(services, Service{
			Name:  service.Name,
			Type:  string(service.Spec.Type),
			Ports: servicePorts(service),
		})
	}
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	return services, nil
}

func (c *clientGoClient) ListEndpoints(ctx context.Context, namespace string) ([]Endpoint, error) {
	endpoints, err := c.listEndpointSlices(ctx, namespace)
	if err == nil {
		return endpoints, nil
	}
	if !shouldFallbackToEndpoints(err) {
		return nil, err
	}
	return c.listEndpoints(ctx, namespace)
}

func (c *clientGoClient) listEndpointSlices(ctx context.Context, namespace string) ([]Endpoint, error) {
	list, err := c.clientset.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	endpoints := make([]Endpoint, 0, len(list.Items))
	for _, slice := range list.Items {
		endpoints = append(endpoints, Endpoint{
			Name:      slice.Name,
			Addresses: endpointSliceAddresses(slice),
		})
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].Name < endpoints[j].Name })
	return endpoints, nil
}

func (c *clientGoClient) listEndpoints(ctx context.Context, namespace string) ([]Endpoint, error) {
	list, err := c.clientset.CoreV1().Endpoints(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	endpoints := make([]Endpoint, 0, len(list.Items))
	for _, endpoint := range list.Items {
		endpoints = append(endpoints, Endpoint{
			Name:      endpoint.Name,
			Addresses: endpointAddresses(endpoint),
		})
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].Name < endpoints[j].Name })
	return endpoints, nil
}

func buildRestConfig(cfg Config, timeout time.Duration) (*rest.Config, error) {
	if cfg.Kubeconfig != "" {
		restCfg, err := clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
		if err != nil {
			return nil, err
		}
		if timeout > 0 {
			restCfg.Timeout = timeout
		}
		return restCfg, nil
	}

	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	if timeout > 0 {
		restCfg.Timeout = timeout
	}
	return restCfg, nil
}

func podReady(pod corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func podRestartCount(pod corev1.Pod) int32 {
	var restarts int32
	for _, status := range pod.Status.ContainerStatuses {
		restarts += status.RestartCount
	}
	return restarts
}

func servicePorts(service corev1.Service) []string {
	ports := make([]string, 0, len(service.Spec.Ports))
	for _, port := range service.Spec.Ports {
		ports = append(ports, fmt.Sprintf("%d/%s", port.Port, strings.ToLower(string(port.Protocol))))
	}
	return ports
}

func endpointSliceAddresses(slice discoveryv1.EndpointSlice) []string {
	addresses := make([]string, 0)
	for _, endpoint := range slice.Endpoints {
		addresses = append(addresses, endpoint.Addresses...)
	}
	sort.Strings(addresses)
	return addresses
}

func endpointAddresses(endpoint corev1.Endpoints) []string {
	addresses := make([]string, 0)
	for _, subset := range endpoint.Subsets {
		for _, address := range subset.Addresses {
			addresses = append(addresses, address.IP)
		}
	}
	sort.Strings(addresses)
	return addresses
}

func shouldFallbackToEndpoints(err error) bool {
	return apierrors.IsNotFound(err) ||
		apierrors.IsMethodNotSupported(err) ||
		apierrors.IsForbidden(err)
}
