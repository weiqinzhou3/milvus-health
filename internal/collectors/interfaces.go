package collectors

import "context"

type MilvusCollector interface {
	Ping(ctx context.Context) error
}

type K8sCollector interface {
	Ping(ctx context.Context) error
}
