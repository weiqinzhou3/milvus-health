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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
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
	metricsClient, err := metricsclientset.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}
	return &clientGoClient{clientset: clientset, metricsClient: metricsClient}, nil
}

type clientGoClient struct {
	clientset        kubernetes.Interface
	metricsClient    metricsclientset.Interface
	discoveryClient  metricsDiscovery
	podMetricsLister podMetricsLister
}

type metricsDiscovery interface {
	ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error)
}

type podMetricsLister interface {
	List(ctx context.Context, namespace string) (*metricsv1beta1.PodMetricsList, error)
}

type typedPodMetricsLister struct {
	client metricsclientset.Interface
}

func (l typedPodMetricsLister) List(ctx context.Context, namespace string) (*metricsv1beta1.PodMetricsList, error) {
	return l.client.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
}

func (c *clientGoClient) ListPods(ctx context.Context, namespace string) ([]Pod, error) {
	list, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	pods := make([]Pod, 0, len(list.Items))
	for _, pod := range list.Items {
		pods = append(pods, Pod{
			Name:          pod.Name,
			Phase:         string(pod.Status.Phase),
			Ready:         podReady(pod),
			RestartCount:  podRestartCount(pod),
			CPURequest:    podResourceString(pod.Spec.Containers, corev1.ResourceCPU, true),
			CPULimit:      podResourceString(pod.Spec.Containers, corev1.ResourceCPU, false),
			MemoryRequest: podResourceString(pod.Spec.Containers, corev1.ResourceMemory, true),
			MemoryLimit:   podResourceString(pod.Spec.Containers, corev1.ResourceMemory, false),
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

func (c *clientGoClient) ListPodMetrics(ctx context.Context, namespace string) (PlatformMetricsResult, error) {
	discoveryClient := c.discoveryClient
	if discoveryClient == nil {
		discoveryClient = c.clientset.Discovery()
	}
	if _, err := discoveryClient.ServerResourcesForGroupVersion("metrics.k8s.io/v1beta1"); err != nil {
		if result, ok := metricsUnavailableResult(err); ok {
			return result, nil
		}
		return PlatformMetricsResult{}, err
	}

	lister := c.podMetricsLister
	if lister == nil {
		lister = typedPodMetricsLister{client: c.metricsClient}
	}
	list, err := lister.List(ctx, namespace)
	if err != nil {
		if result, ok := metricsUnavailableResult(err); ok {
			return result, nil
		}
		return PlatformMetricsResult{}, err
	}

	metrics := make([]PodMetric, 0, len(list.Items))
	for _, item := range list.Items {
		cpu, memory := podMetricUsage(item)
		metrics = append(metrics, PodMetric{
			PodName:     item.Name,
			CPUUsage:    cpu,
			MemoryUsage: memory,
		})
	}
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].PodName < metrics[j].PodName })
	return PlatformMetricsResult{
		Available: true,
		Metrics:   metrics,
	}, nil
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
		value := fmt.Sprintf("%d", port.Port)
		if service.Spec.Type == corev1.ServiceTypeNodePort && port.NodePort > 0 {
			value = fmt.Sprintf("%s:%d", value, port.NodePort)
		}
		ports = append(ports, fmt.Sprintf("%s/%s", value, strings.ToLower(string(port.Protocol))))
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

func podResourceString(containers []corev1.Container, resourceName corev1.ResourceName, request bool) string {
	total, ok := sumPodResource(containers, resourceName, request)
	if !ok {
		return ""
	}
	return total.String()
}

func sumPodResource(containers []corev1.Container, resourceName corev1.ResourceName, request bool) (resource.Quantity, bool) {
	var total resource.Quantity
	found := false
	for _, container := range containers {
		var quantity resource.Quantity
		var ok bool
		if request {
			quantity, ok = container.Resources.Requests[resourceName]
		} else {
			quantity, ok = container.Resources.Limits[resourceName]
		}
		if !ok {
			continue
		}
		if !found {
			total = quantity.DeepCopy()
			found = true
			continue
		}
		total.Add(quantity)
	}
	return total, found
}

func metricsUnavailableResult(err error) (PlatformMetricsResult, bool) {
	switch {
	case apierrors.IsNotFound(err):
		return PlatformMetricsResult{
			Available:         false,
			UnavailableReason: "metrics-server not found",
		}, true
	case apierrors.IsForbidden(err):
		return PlatformMetricsResult{
			Available:         false,
			UnavailableReason: "insufficient permissions",
		}, true
	default:
		return PlatformMetricsResult{}, false
	}
}

func podMetricUsage(podMetrics metricsv1beta1.PodMetrics) (string, string) {
	cpu, ok := sumPodMetricResource(podMetrics.Containers, corev1.ResourceCPU)
	cpuUsage := ""
	if ok {
		cpuUsage = cpu.String()
	}

	memory, ok := sumPodMetricResource(podMetrics.Containers, corev1.ResourceMemory)
	memoryUsage := ""
	if ok {
		memoryUsage = memory.String()
	}

	return cpuUsage, memoryUsage
}

func sumPodMetricResource(containers []metricsv1beta1.ContainerMetrics, resourceName corev1.ResourceName) (resource.Quantity, bool) {
	var total resource.Quantity
	found := false
	for _, container := range containers {
		quantity, ok := container.Usage[resourceName]
		if !ok {
			continue
		}
		if !found {
			total = quantity.DeepCopy()
			found = true
			continue
		}
		total.Add(quantity)
	}
	return total, found
}
