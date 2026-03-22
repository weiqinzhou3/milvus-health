package probes

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/model"
	platformmilvus "github.com/weiqinzhou3/milvus-health/internal/platform/milvus"
)

type DefaultBusinessReadProbe struct {
	Factory platformmilvus.ClientFactory
}

func (p DefaultBusinessReadProbe) Run(ctx context.Context, cfg *model.Config, scope ProbeScope) (model.BusinessReadProbeResult, error) {
	result := model.BusinessReadProbeResult{
		Status:            model.CheckStatusSkip,
		MinSuccessTargets: 1,
		Message:           "not configured",
	}
	if cfg == nil {
		return result, &model.AppError{Code: model.ErrCodeProbeRead, Message: "config is nil"}
	}

	result.MinSuccessTargets = cfg.Probe.Read.MinSuccessTargets
	if len(cfg.Probe.Read.Targets) == 0 {
		return result, nil
	}

	client, err := p.newClient(ctx, cfg)
	if err != nil {
		return result, &model.AppError{Code: model.ErrCodeProbeRead, Message: fmt.Sprintf("create milvus client: %v", err), Cause: err}
	}
	defer client.Close(ctx)

	filteredTargets := make([]model.ReadProbeTarget, 0, len(cfg.Probe.Read.Targets))
	for _, target := range cfg.Probe.Read.Targets {
		if !matchesScope(target, scope) {
			continue
		}
		filteredTargets = append(filteredTargets, target)
	}
	if len(filteredTargets) == 0 {
		result.Message = "all targets filtered out"
		return result, nil
	}

	result.ConfiguredTargets = len(filteredTargets)
	result.Targets = make([]model.BusinessReadTargetResult, 0, len(filteredTargets))
	for _, target := range filteredTargets {
		targetResult := p.runTarget(ctx, client, target)
		result.Targets = append(result.Targets, targetResult)
		if targetResult.Success {
			result.SuccessfulTargets++
		}
	}

	switch {
	case result.SuccessfulTargets == 0:
		result.Status = model.CheckStatusFail
		result.Message = "no read probe targets succeeded"
	case result.SuccessfulTargets >= result.MinSuccessTargets:
		result.Status = model.CheckStatusPass
		result.Message = fmt.Sprintf("%d/%d read probe targets succeeded", result.SuccessfulTargets, result.ConfiguredTargets)
	case result.SuccessfulTargets > 0:
		result.Status = model.CheckStatusWarn
		result.Message = fmt.Sprintf("%d/%d read probe targets succeeded, below min_success_targets=%d", result.SuccessfulTargets, result.ConfiguredTargets, result.MinSuccessTargets)
	default:
		result.Status = model.CheckStatusFail
		result.Message = "no read probe targets succeeded"
	}
	return result, nil
}

func (p DefaultBusinessReadProbe) runTarget(ctx context.Context, client platformmilvus.Client, target model.ReadProbeTarget) model.BusinessReadTargetResult {
	start := time.Now()
	result := model.BusinessReadTargetResult{
		Database:   target.Database,
		Collection: target.Collection,
		Action:     model.ProbeActionDescribeFailed,
	}
	defer func() {
		result.DurationMS = time.Since(start).Milliseconds()
	}()

	description, err := client.DescribeCollection(ctx, target.Database, target.Collection)
	if err != nil {
		result.Error = fmt.Sprintf("describe collection: %v", err)
		return result
	}

	if rowCount, err := client.GetCollectionRowCount(ctx, target.Database, target.Collection); err == nil {
		result.RowCount = int64Ptr(rowCount)
	} else {
		result.Error = joinProbeError(result.Error, fmt.Sprintf("row count unavailable: %v", err))
	}

	if _, err := client.GetCollectionLoadState(ctx, target.Database, target.Collection); err != nil {
		result.Error = joinProbeError(result.Error, fmt.Sprintf("load state unavailable: %v", err))
	}

	result.Action = model.ProbeActionQuery
	if strings.TrimSpace(target.AnnsField) != "" {
		result.Action = model.ProbeActionQueryAndSearch
	}

	if _, err := client.Query(ctx, platformmilvus.QueryRequest{
		Database:     target.Database,
		Collection:   target.Collection,
		Expr:         target.QueryExpr,
		Limit:        1,
		OutputFields: append([]string(nil), target.OutputFields...),
	}); err != nil {
		result.Error = joinProbeError(result.Error, fmt.Sprintf("query failed: %v", err))
		return result
	}

	if result.Action != model.ProbeActionQueryAndSearch {
		result.Success = true
		return result
	}

	dim, err := vectorFieldDim(description, target.AnnsField)
	if err != nil {
		result.Error = joinProbeError(result.Error, err.Error())
		return result
	}

	if _, err := client.Search(ctx, platformmilvus.SearchRequest{
		Database:     target.Database,
		Collection:   target.Collection,
		Expr:         target.QueryExpr,
		AnnsField:    target.AnnsField,
		TopK:         target.TopK,
		Vector:       buildQueryVector(dim),
		OutputFields: append([]string(nil), target.OutputFields...),
	}); err != nil {
		result.Error = joinProbeError(result.Error, fmt.Sprintf("search failed: %v", err))
		return result
	}

	result.Success = true
	return result
}

func (p DefaultBusinessReadProbe) newClient(ctx context.Context, cfg *model.Config) (platformmilvus.Client, error) {
	if p.Factory == nil {
		return nil, fmt.Errorf("milvus client factory is nil")
	}
	return p.Factory.New(ctx, platformmilvus.Config{
		Address:  cfg.Cluster.Milvus.URI,
		Username: cfg.Cluster.Milvus.Username,
		Password: cfg.Cluster.Milvus.Password,
		Token:    cfg.Cluster.Milvus.Token,
	}, time.Duration(cfg.TimeoutSec)*time.Second)
}

func matchesScope(target model.ReadProbeTarget, scope ProbeScope) bool {
	if scope.Database != "" && target.Database != scope.Database {
		return false
	}
	if scope.Collection != "" && target.Collection != scope.Collection {
		return false
	}
	return true
}

func vectorFieldDim(description platformmilvus.CollectionDescription, annsField string) (int, error) {
	for _, field := range description.Fields {
		if field.Name != annsField {
			continue
		}
		if !field.IsVector || field.Dimension <= 0 {
			return 0, fmt.Errorf("anns_field %q does not have a valid vector dim", annsField)
		}
		return int(field.Dimension), nil
	}
	return 0, fmt.Errorf("anns_field %q not found in schema", annsField)
}

func buildQueryVector(dim int) []float32 {
	rng := rand.New(rand.NewSource(42))
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = rng.Float32()*2 - 1
	}
	return vec
}

func joinProbeError(current, next string) string {
	if strings.TrimSpace(next) == "" {
		return current
	}
	if strings.TrimSpace(current) == "" {
		return next
	}
	return current + "; " + next
}

func int64Ptr(v int64) *int64 {
	return &v
}
