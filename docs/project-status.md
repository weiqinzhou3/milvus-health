# milvus-health Project Status

Last updated: 2026-03-21

## 1. Current conclusion

The current working branch has moved beyond the first Milvus slice and now provides **Iteration A2 / Milvus Inventory Enrichment** for the real Milvus path.

This branch can now truthfully claim:

- real Milvus SDK connection is wired into `check`
- real `milvus_version` is collected
- `arch_profile` is derived from the real version using the spec v1.2 mapping
- real database names and per-database collection name lists are collected
- real per-collection row count collection is wired through `GetCollectionStatistics`
- real cluster total row count is reported when all collection row counts are available
- `check` text/json output is now driven by real Milvus facts when Milvus is reachable

This branch still should **not** be treated as having full P0 coverage. Data size / binlog size, Kubernetes health, read/write probes, richer collection metrics, and full analyzer rules are still out of scope or skeleton-only.

## 2. Stage assessment

Current stage: **Stage 2.5 / Real Milvus inventory with row count enrichment**

Suggested next stage target: **Vertical Slice 3 - Real Kubernetes Basic Health**

Suggested stage sequence:

1. Skeleton
2. Real Milvus inventory vertical slice
3. Real Milvus inventory enrichment
4. Real Kubernetes basic health vertical slice
5. Analyzer rule expansion
6. Business Read Probe
7. RW Probe
8. Detail-mode enrichment and operator usability polish

## 3. Module status overview

| Module | Status | Current assessment |
|---|---|---|
| CLI (`cmd`) | Implemented | `check` / `validate` / `version` command path exists and `check` now reaches real Milvus collection |
| App entry (`main.go`) | Implemented | Standard CLI entry already exists |
| Config loading | Implemented | YAML loading is present |
| Config validation | Implemented for current contract | Static validation, defaulting, and CLI override path are wired before collection |
| Output rendering | Partially implemented | `text` / `json` renderers now expose real Milvus version/database/collection facts plus row count summary; detail mode still only covers the current minimal Milvus collection detail |
| Exit-code mapping | Implemented | Pass/Warn/Fail/error mapping path exists |
| Analyzer | Minimal runtime path | Analyzer consumes collected Milvus facts and runner-produced checks, and now warns when row count collection is partial; it is not yet a full P0 rules engine |
| Milvus platform client | Minimally implemented | Real client methods for `GetVersion`, `ListDatabases`, `ListCollections`, and per-collection row count now exist |
| Kubernetes platform client | Placeholder only | Not part of this iteration |
| Milvus collector | Minimally implemented | `CollectClusterInfo` and `CollectInventory` are real for version/database/collection inventory and row count enrichment |
| Kubernetes collector | Placeholder only | Not part of this iteration |
| Probes | Placeholder only | Business Read / RW probe real logic is still not implemented |
| Tests | Implemented for this slice | Platform fake tests, Milvus collector tests, runner tests, renderer tests, analyzer tests, command/integration tests, smoke tests all cover the current slice |
| Examples | Implemented | Example outputs updated to current runtime behavior |

## 4. What is implemented in this branch

### 4.1 Real Milvus collection path

- Milvus SDK client factory under `internal/platform/milvus`
- real version collection
- real database listing
- real collection listing per database
- real per-collection row count via `GetCollectionStatistics`
- real total row count when all collection row counts are available
- arch profile detection based on real version

### 4.2 Check runner orchestration

`check` now follows this minimal path:

1. load config
2. apply defaults
3. apply CLI overrides
4. validate config
5. collect Milvus cluster info
6. collect Milvus inventory
7. assemble snapshot and checks
8. run minimal analyzer
9. render text/json output

### 4.3 Real facts now visible in output

- `cluster.milvus_version`
- `cluster.arch_profile`
- `inventory.milvus.database_count`
- `inventory.milvus.collection_count`
- `inventory.milvus.total_row_count`
- `inventory.milvus.databases[].name`
- `inventory.milvus.databases[].collections[]`
- `inventory.milvus.collections[].row_count`
- `summary.total_row_count`

## 5. What is intentionally not implemented in this branch

- binlog size / data size
- index count / index type
- vector field list
- load state
- shard / replica / partition detail beyond current minimal legacy compatibility structs
- Kubernetes health collection
- Business Read Probe
- RW Probe
- full analyzer rule matrix
- standby / confidence advanced logic beyond minimal severity mapping

## 6. Known gaps and technical debt

1. `mq_type` is still reported as `unknown`; this branch does not attempt automatic MQ detection.
2. Row count collection currently degrades to `unknown` on a per-collection basis if `GetCollectionStatistics` fails for that collection; the inventory path stays successful but total row count also becomes `unknown`.
3. The analyzer is intentionally minimal and should not yet be described as a full operator-grade health analyzer.
4. Example outputs still demonstrate the failure path because the bundled example config points at an unavailable local Milvus endpoint.
5. Flat legacy packages under `internal/platform` and `internal/collectors` still exist for compatibility; the new real Milvus path is in `internal/platform/milvus` and `internal/collectors/milvus`.

## 7. Validation status

The branch currently passes:

- `go test ./...`
- `go build ./...`

## 8. Collaboration rule

For this repository, work is not done unless:

1. code is committed on a dedicated branch
2. the branch is pushed to GitHub
3. branch / commit / changed files / commands run are reported back
