package collectors

import (
	"context"

	"github.com/weiqinzhou3/milvus-health/internal/model"
)

type MilvusInventoryCollector interface {
	Collect(ctx context.Context, cfg *model.Config) (model.MilvusInventory, error)
}

type K8sInventoryCollector interface {
	Collect(ctx context.Context, cfg *model.Config) (model.K8sInventory, error)
}
