package k8s

import (
	"context"
	"time"
)

type FakeClient struct {
	Pods         []Pod
	PodsErr      error
	Services     []Service
	ServicesErr  error
	Endpoints    []Endpoint
	EndpointsErr error
}

func (f *FakeClient) ListPods(ctx context.Context, namespace string) ([]Pod, error) {
	_ = ctx
	_ = namespace
	return append([]Pod(nil), f.Pods...), f.PodsErr
}

func (f *FakeClient) ListServices(ctx context.Context, namespace string) ([]Service, error) {
	_ = ctx
	_ = namespace
	return append([]Service(nil), f.Services...), f.ServicesErr
}

func (f *FakeClient) ListEndpoints(ctx context.Context, namespace string) ([]Endpoint, error) {
	_ = ctx
	_ = namespace
	return append([]Endpoint(nil), f.Endpoints...), f.EndpointsErr
}

type FakeClientFactory struct {
	Client *FakeClient
	Err    error
}

func (f FakeClientFactory) New(ctx context.Context, cfg Config, timeout time.Duration) (Client, error) {
	_ = ctx
	_ = cfg
	_ = timeout
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Client, nil
}
