package probes

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/weiqinzhou3/milvus-health/internal/model"
	platformmilvus "github.com/weiqinzhou3/milvus-health/internal/platform/milvus"
)

const rwProbeCollectionName = "rw_probe"

type DefaultRWProbe struct {
	Factory platformmilvus.ClientFactory
	Now     func() time.Time
}

func (p DefaultRWProbe) Run(ctx context.Context, cfg *model.Config) (result model.RWProbeResult, err error) {
	result = model.RWProbeResult{
		Status:         model.CheckStatusSkip,
		Enabled:        cfg != nil && cfg.Probe.RW.Enabled,
		CleanupEnabled: cfg != nil && cfg.Probe.RW.Cleanup,
		Message:        "rw probe disabled",
	}
	if cfg == nil {
		return result, &model.AppError{Code: model.ErrCodeProbeRW, Message: "config is nil"}
	}
	result.InsertRows = cfg.Probe.RW.InsertRows
	result.VectorDim = cfg.Probe.RW.VectorDim
	if !cfg.Probe.RW.Enabled {
		return result, nil
	}

	client, err := p.newClient(ctx, cfg)
	if err != nil {
		return result, &model.AppError{Code: model.ErrCodeProbeRW, Message: fmt.Sprintf("create milvus client: %v", err), Cause: err}
	}
	defer client.Close(ctx)

	runID := p.now().UTC().UnixNano()
	result.TestDatabase = fmt.Sprintf("%s_%d", cfg.Probe.RW.TestDatabasePrefix, runID)
	result.TestCollection = rwProbeCollectionName

	step := runProbeStep("cleanup-stale-databases", func() error {
		return p.cleanupStaleTestDatabases(ctx, client, cfg.Probe.RW.TestDatabasePrefix)
	})
	result.StepResults = append(result.StepResults, step)
	if !step.Success {
		result.Status = model.CheckStatusFail
		result.Message = fmt.Sprintf("pre-existing test data cleanup failed: %s", step.Error)
		return result, nil
	}

	var createdDB bool
	var createdCollection bool
	defer func() {
		if !result.Enabled || !result.CleanupEnabled {
			return
		}
		if !createdDB {
			return
		}
		result.CleanupExecuted = true
		step := runProbeStep("cleanup", func() error {
			return p.cleanupTestResources(ctx, client, result.TestDatabase, result.TestCollection, createdCollection)
		})
		result.StepResults = append(result.StepResults, step)
		if !step.Success {
			result = finalizeRWResult(result, step.Error)
		}
	}()

	step = runProbeStep("create-database", func() error {
		return client.CreateDatabase(ctx, result.TestDatabase)
	})
	result.StepResults = append(result.StepResults, step)
	if !step.Success {
		result.Status = model.CheckStatusFail
		result.Message = fmt.Sprintf("create database failed: %s", step.Error)
		return result, nil
	}
	createdDB = true

	step = runProbeStep("create-collection", func() error {
		return client.CreateCollection(ctx, platformmilvus.CreateCollectionRequest{
			Database:   result.TestDatabase,
			Collection: result.TestCollection,
			VectorDim:  cfg.Probe.RW.VectorDim,
		})
	})
	result.StepResults = append(result.StepResults, step)
	if !step.Success {
		result.Status = model.CheckStatusFail
		result.Message = fmt.Sprintf("create collection failed: %s", step.Error)
		return result, nil
	}
	createdCollection = true

	ids, payloads, vectors := buildRWProbeRows(cfg.Probe.RW.InsertRows, cfg.Probe.RW.VectorDim)
	step = runProbeStep("insert", func() error {
		insertResult, err := client.Insert(ctx, platformmilvus.InsertRequest{
			Database:   result.TestDatabase,
			Collection: result.TestCollection,
			IDs:        ids,
			Payloads:   payloads,
			Vectors:    vectors,
		})
		if err != nil {
			return err
		}
		if insertResult.InsertCount != int64(cfg.Probe.RW.InsertRows) {
			return fmt.Errorf("insert count = %d, want %d", insertResult.InsertCount, cfg.Probe.RW.InsertRows)
		}
		return nil
	})
	result.StepResults = append(result.StepResults, step)
	if !step.Success {
		result.Status = model.CheckStatusFail
		result.Message = fmt.Sprintf("insert failed: %s", step.Error)
		return result, nil
	}

	step = runProbeStep("flush", func() error {
		return client.Flush(ctx, result.TestDatabase, result.TestCollection)
	})
	result.StepResults = append(result.StepResults, step)
	if !step.Success {
		result.Status = model.CheckStatusFail
		result.Message = fmt.Sprintf("flush failed: %s", step.Error)
		return result, nil
	}

	step = runProbeStep("query", func() error {
		queryResult, err := client.Query(ctx, platformmilvus.QueryRequest{
			Database:   result.TestDatabase,
			Collection: result.TestCollection,
			Expr:       idsExpr(ids),
			Limit:      cfg.Probe.RW.InsertRows,
		})
		if err != nil {
			return err
		}
		if queryResult.ResultCount != cfg.Probe.RW.InsertRows {
			return fmt.Errorf("query result count = %d, want %d", queryResult.ResultCount, cfg.Probe.RW.InsertRows)
		}
		return nil
	})
	result.StepResults = append(result.StepResults, step)
	if !step.Success {
		result.Status = model.CheckStatusFail
		result.Message = fmt.Sprintf("query failed: %s", step.Error)
		return result, nil
	}

	result.Status = model.CheckStatusPass
	result.Message = "rw probe completed successfully"
	return result, nil
}

func (p DefaultRWProbe) cleanupStaleTestDatabases(ctx context.Context, client platformmilvus.Client, prefix string) error {
	databases, err := client.ListDatabases(ctx)
	if err != nil {
		return err
	}
	for _, database := range databases {
		if !strings.HasPrefix(database, prefix+"_") {
			continue
		}
		collections, err := client.ListCollections(ctx, database)
		if err != nil {
			return fmt.Errorf("list collections for stale database %q: %w", database, err)
		}
		for _, collection := range collections {
			if err := client.DropCollection(ctx, database, collection); err != nil {
				return fmt.Errorf("drop stale collection %q in database %q: %w", collection, database, err)
			}
		}
		if err := client.DropDatabase(ctx, database); err != nil {
			return fmt.Errorf("drop stale database %q: %w", database, err)
		}
	}
	return nil
}

func (p DefaultRWProbe) cleanupTestResources(ctx context.Context, client platformmilvus.Client, database, collection string, createdCollection bool) error {
	if createdCollection {
		if err := client.DropCollection(ctx, database, collection); err != nil {
			return fmt.Errorf("drop test collection: %w", err)
		}
	}
	if err := client.DropDatabase(ctx, database); err != nil {
		return fmt.Errorf("drop test database: %w", err)
	}
	return nil
}

func (p DefaultRWProbe) newClient(ctx context.Context, cfg *model.Config) (platformmilvus.Client, error) {
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

func (p DefaultRWProbe) now() time.Time {
	if p.Now != nil {
		return p.Now()
	}
	return time.Now()
}

func runProbeStep(name string, fn func() error) model.ProbeStepResult {
	start := time.Now()
	step := model.ProbeStepResult{Name: name, Success: true}
	if err := fn(); err != nil {
		step.Success = false
		step.Error = err.Error()
	}
	step.DurationMS = time.Since(start).Milliseconds()
	return step
}

func buildRWProbeRows(insertRows, vectorDim int) ([]int64, []string, [][]float32) {
	ids := make([]int64, insertRows)
	payloads := make([]string, insertRows)
	vectors := make([][]float32, insertRows)
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < insertRows; i++ {
		ids[i] = int64(i)
		payloads[i] = "payload-" + strconv.Itoa(i)
		vector := make([]float32, vectorDim)
		for j := range vector {
			vector[j] = rng.Float32()*2 - 1
		}
		vectors[i] = vector
	}
	return ids, payloads, vectors
}

func idsExpr(ids []int64) string {
	if len(ids) == 0 {
		return "id in []"
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatInt(id, 10))
	}
	return "id in [" + strings.Join(parts, ",") + "]"
}

func finalizeRWResult(result model.RWProbeResult, cleanupErr string) model.RWProbeResult {
	if cleanupErr == "" {
		return result
	}
	if result.Message == "" {
		result.Message = "cleanup failed: " + cleanupErr
	} else {
		result.Message = result.Message + "; cleanup failed: " + cleanupErr
	}
	result.Status = model.CheckStatusFail
	return result
}
