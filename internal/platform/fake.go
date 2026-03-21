package platform

import (
	"context"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type FakeMilvusClient struct {
	PingErr        error
	Version        string
	VersionErr     error
	Databases      []string
	DatabasesErr   error
	Collections    map[string][]MilvusCollection
	CollectionErrs map[string]error
	Closed         bool
}

func (f *FakeMilvusClient) Ping(ctx context.Context) error {
	_ = ctx
	return f.PingErr
}

func (f *FakeMilvusClient) GetServerVersion(ctx context.Context) (string, error) {
	_ = ctx
	return f.Version, f.VersionErr
}

func (f *FakeMilvusClient) ListDatabases(ctx context.Context) ([]string, error) {
	_ = ctx
	return append([]string(nil), f.Databases...), f.DatabasesErr
}

func (f *FakeMilvusClient) ListCollections(ctx context.Context, database string) ([]MilvusCollection, error) {
	_ = ctx
	if err := f.CollectionErrs[database]; err != nil {
		return nil, err
	}
	return append([]MilvusCollection(nil), f.Collections[database]...), nil
}

func (f *FakeMilvusClient) Close(ctx context.Context) error {
	_ = ctx
	f.Closed = true
	return nil
}

type FakeMilvusClientFactory struct {
	Client *FakeMilvusClient
	Err    error
}

func (f FakeMilvusClientFactory) New(ctx context.Context, cfg model.MilvusConfig, timeout time.Duration) (MilvusClient, error) {
	_ = ctx
	_ = cfg
	_ = timeout
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Client, nil
}

type FakeK8sClient struct {
	Pods         []PodInfo
	PodsErr      error
	Services     []ServiceInfo
	ServicesErr  error
	Endpoints    []EndpointInfo
	EndpointsErr error
}

func (f *FakeK8sClient) ListPods(ctx context.Context, namespace string) ([]PodInfo, error) {
	_ = ctx
	_ = namespace
	return append([]PodInfo(nil), f.Pods...), f.PodsErr
}

func (f *FakeK8sClient) ListServices(ctx context.Context, namespace string) ([]ServiceInfo, error) {
	_ = ctx
	_ = namespace
	return append([]ServiceInfo(nil), f.Services...), f.ServicesErr
}

func (f *FakeK8sClient) ListEndpoints(ctx context.Context, namespace string) ([]EndpointInfo, error) {
	_ = ctx
	_ = namespace
	return append([]EndpointInfo(nil), f.Endpoints...), f.EndpointsErr
}

type FakeK8sClientFactory struct {
	Client *FakeK8sClient
	Err    error
}

func (f FakeK8sClientFactory) New(ctx context.Context, cfg model.K8sConfig, timeout time.Duration) (K8sClient, error) {
	_ = ctx
	_ = cfg
	_ = timeout
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Client, nil
}
