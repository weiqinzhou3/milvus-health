package platform

import "context"

type MilvusClient interface {
	GetVersion(ctx context.Context) (string, error)
}

type K8sClient interface {
	ListNamespaces(ctx context.Context) ([]string, error)
}
