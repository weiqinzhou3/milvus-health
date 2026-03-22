package k8s

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestClientGoClient_ListPodsServicesAndEndpointSlices(t *testing.T) {
	t.Parallel()

	clientset := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "milvus-0", Namespace: "milvus"},
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
	client := &clientGoClient{clientset: clientset}

	pods, err := client.ListPods(context.Background(), "milvus")
	if err != nil {
		t.Fatalf("ListPods() error = %v", err)
	}
	if len(pods) != 1 || !pods[0].Ready || pods[0].RestartCount != 2 {
		t.Fatalf("ListPods() = %#v", pods)
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

func TestClientGoClient_NodePortWithZeroNodePort_FallsBackToPlainFormat(t *testing.T) {
	t.Parallel()

	// A NodePort-type service whose NodePort field is 0 must render as plain "port/protocol",
	// not "port:0/protocol". The guard `port.NodePort > 0` prevents the colon form.
	clientset := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "milvus"},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{
					{Port: 8080, NodePort: 0, Protocol: corev1.ProtocolTCP},
				},
			},
		},
	)
	client := &clientGoClient{clientset: clientset}

	services, err := client.ListServices(context.Background(), "milvus")
	if err != nil {
		t.Fatalf("ListServices() error = %v", err)
	}
	if len(services) != 1 || services[0].Ports[0] != "8080/tcp" {
		t.Fatalf("ListServices() = %#v, want port string \"8080/tcp\"", services)
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
