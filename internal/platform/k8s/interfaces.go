package k8s

import (
	"context"
	"time"
)

type Config struct {
	Namespace  string
	Kubeconfig string
}

type Pod struct {
	Name         string
	Phase        string
	Ready        bool
	RestartCount int32
}

type Service struct {
	Name  string
	Type  string
	Ports []string
}

type Endpoint struct {
	Name      string
	Addresses []string
}

type Client interface {
	ListPods(ctx context.Context, namespace string) ([]Pod, error)
	ListServices(ctx context.Context, namespace string) ([]Service, error)
	ListEndpoints(ctx context.Context, namespace string) ([]Endpoint, error)
}

type ClientFactory interface {
	New(ctx context.Context, cfg Config, timeout time.Duration) (Client, error)
}
