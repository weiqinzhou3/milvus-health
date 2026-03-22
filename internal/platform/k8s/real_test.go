package k8s

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

type metricsDiscoveryStub struct {
	err error
}

func (s metricsDiscoveryStub) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	_ = groupVersion
	if s.err != nil {
		return nil, s.err
	}
	return &metav1.APIResourceList{GroupVersion: "metrics.k8s.io/v1beta1"}, nil
}

type podMetricsListerStub struct {
	list *metricsv1beta1.PodMetricsList
	err  error
}

func (s podMetricsListerStub) List(ctx context.Context, namespace string) (*metricsv1beta1.PodMetricsList, error) {
	_ = ctx
	_ = namespace
	return s.list, s.err
}

func TestClientGoClient_ListPodsServicesAndEndpointSlices(t *testing.T) {
	t.Parallel()

	clientset := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "milvus-0", Namespace: "milvus"},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "main",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1000m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
				ContainerStatuses: []corev1.ContainerStatus{
					{RestartCount: 2},
				},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "milvus", Namespace: "milvus"},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{Port: 19530, Protocol: corev1.ProtocolTCP},
				},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "attu", Namespace: "milvus"},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{
					{Port: 3000, NodePort: 30031, Protocol: corev1.ProtocolTCP},
				},
			},
		},
		&discoveryv1.EndpointSlice{
			ObjectMeta:  metav1.ObjectMeta{Name: "milvus-abc", Namespace: "milvus"},
			AddressType: discoveryv1.AddressTypeIPv4,
			Endpoints: []discoveryv1.Endpoint{
				{Addresses: []string{"10.0.0.1", "10.0.0.2"}},
			},
		},
	)
	client := &clientGoClient{
		clientset:       clientset,
		metricsClient:   metricsfake.NewSimpleClientset(),
		discoveryClient: metricsDiscoveryStub{},
		podMetricsLister: podMetricsListerStub{
			list: &metricsv1beta1.PodMetricsList{
				Items: []metricsv1beta1.PodMetrics{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "milvus-0", Namespace: "milvus"},
						Containers: []metricsv1beta1.ContainerMetrics{
							{
								Name: "main",
								Usage: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("125m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
		},
	}

	pods, err := client.ListPods(context.Background(), "milvus")
	if err != nil {
		t.Fatalf("ListPods() error = %v", err)
	}
	if len(pods) != 1 || !pods[0].Ready || pods[0].RestartCount != 2 {
		t.Fatalf("ListPods() = %#v", pods)
	}
	if pods[0].CPURequest != "500m" || pods[0].CPULimit != "1" || pods[0].MemoryRequest != "512Mi" || pods[0].MemoryLimit != "1Gi" {
		t.Fatalf("ListPods() resource aggregation = %#v", pods[0])
	}

	services, err := client.ListServices(context.Background(), "milvus")
	if err != nil {
		t.Fatalf("ListServices() error = %v", err)
	}
	if len(services) != 2 || services[0].Ports[0] != "3000:30031/tcp" || services[1].Ports[0] != "19530/tcp" {
		t.Fatalf("ListServices() = %#v", services)
	}

	endpoints, err := client.ListEndpoints(context.Background(), "milvus")
	if err != nil {
		t.Fatalf("ListEndpoints() error = %v", err)
	}
	if len(endpoints) != 1 || len(endpoints[0].Addresses) != 2 {
		t.Fatalf("ListEndpoints() = %#v", endpoints)
	}

	metrics, err := client.ListPodMetrics(context.Background(), "milvus")
	if err != nil {
		t.Fatalf("ListPodMetrics() error = %v", err)
	}
	if !metrics.Available || len(metrics.Metrics) != 1 || metrics.Metrics[0].CPUUsage != "125m" || metrics.Metrics[0].MemoryUsage != "256Mi" {
		t.Fatalf("ListPodMetrics() = %#v", metrics)
	}
}

func TestClientGoClient_ListEndpoints_FallsBackToCoreEndpoints(t *testing.T) {
	t.Parallel()

	clientset := fake.NewSimpleClientset(
		&corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{Name: "milvus", Namespace: "milvus"},
			Subsets: []corev1.EndpointSubset{
				{Addresses: []corev1.EndpointAddress{{IP: "10.0.0.3"}}},
			},
		},
	)
	clientset.PrependReactor("list", "endpointslices", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "discovery.k8s.io", Resource: "endpointslices"}, "milvus")
	})
	client := &clientGoClient{clientset: clientset}

	endpoints, err := client.ListEndpoints(context.Background(), "milvus")
	if err != nil {
		t.Fatalf("ListEndpoints() error = %v", err)
	}
	if len(endpoints) != 1 || endpoints[0].Name != "milvus" || endpoints[0].Addresses[0] != "10.0.0.3" {
		t.Fatalf("ListEndpoints() = %#v", endpoints)
	}
}

func TestShouldFallbackToEndpoints(t *testing.T) {
	t.Parallel()

	if !shouldFallbackToEndpoints(apierrors.NewForbidden(schema.GroupResource{Resource: "endpointslices"}, "test", errors.New("forbidden"))) {
		t.Fatal("shouldFallbackToEndpoints() should accept forbidden")
	}
	if shouldFallbackToEndpoints(errors.New("boom")) {
		t.Fatal("shouldFallbackToEndpoints() should reject generic errors")
	}
}

func TestClientGoClient_ListPodMetrics_ReturnsUnavailableWhenMetricsServerMissing(t *testing.T) {
	t.Parallel()

	clientset := fake.NewSimpleClientset()
	client := &clientGoClient{
		clientset:       clientset,
		metricsClient:   metricsfake.NewSimpleClientset(),
		discoveryClient: metricsDiscoveryStub{err: apierrors.NewNotFound(schema.GroupResource{Group: "metrics.k8s.io", Resource: "pods"}, "pods.metrics.k8s.io")},
	}

	result, err := client.ListPodMetrics(context.Background(), "milvus")
	if err != nil {
		t.Fatalf("ListPodMetrics() error = %v", err)
	}
	if result.Available || result.UnavailableReason != "metrics-server not found" {
		t.Fatalf("ListPodMetrics() = %#v", result)
	}
}

func TestClientGoClient_ListPodMetrics_ReturnsUnavailableWhenForbidden(t *testing.T) {
	t.Parallel()

	clientset := fake.NewSimpleClientset()
	client := &clientGoClient{
		clientset:       clientset,
		metricsClient:   metricsfake.NewSimpleClientset(),
		discoveryClient: metricsDiscoveryStub{err: apierrors.NewForbidden(schema.GroupResource{Group: "metrics.k8s.io", Resource: "pods"}, "pods.metrics.k8s.io", errors.New("forbidden"))},
	}

	result, err := client.ListPodMetrics(context.Background(), "milvus")
	if err != nil {
		t.Fatalf("ListPodMetrics() error = %v", err)
	}
	if result.Available || result.UnavailableReason != "insufficient permissions" {
		t.Fatalf("ListPodMetrics() = %#v", result)
	}
}
