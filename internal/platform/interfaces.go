package platform

import (
	"context"
	"errors"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/model"
)

var ErrCapabilityUnavailable = errors.New("capability unavailable")

type MilvusCollection struct {
	Database   string
	Name       string
	ShardNum   int32
	FieldCount int
}

type PodInfo struct {
	Name         string
	Phase        string
	Ready        bool
	RestartCount int32
}

type ServiceInfo struct {
	Name  string
	Type  string
	Ports []string
}

type EndpointInfo struct {
	Name      string
	Addresses []string
}

type MilvusClient interface {
	Ping(ctx context.Context) error
	GetServerVersion(ctx context.Context) (string, error)
	ListDatabases(ctx context.Context) ([]string, error)
	ListCollections(ctx context.Context, database string) ([]MilvusCollection, error)
	Close(ctx context.Context) error
}

type MilvusClientFactory interface {
	New(ctx context.Context, cfg model.MilvusConfig, timeout time.Duration) (MilvusClient, error)
}

type K8sClient interface {
	ListPods(ctx context.Context, namespace string) ([]PodInfo, error)
	ListServices(ctx context.Context, namespace string) ([]ServiceInfo, error)
	ListEndpoints(ctx context.Context, namespace string) ([]EndpointInfo, error)
}

type K8sClientFactory interface {
	New(ctx context.Context, cfg model.K8sConfig, timeout time.Duration) (K8sClient, error)
}
