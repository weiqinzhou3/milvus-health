# milvus-health Project Status

Last updated: 2026-03-22

## 1. Current conclusion

The current working branch now provides **Iteration A2 / Milvus Inventory Enrichment**, **Iteration B / Kubernetes Basic Status Collection**, and the **Iteration C1 post-trial fixes** on the real collection path.

This branch can now truthfully claim:

- real Milvus SDK connection is wired into `check`
- real `milvus_version` is collected
- `arch_profile` is derived from the real version using the spec v1.2 mapping, including `v`-prefixed versions such as `v2.4.7` and `v2.6.1`
- real database names and per-database collection name lists are collected
- real per-collection row count collection is wired through `GetCollectionStatistics`
- real cluster total row count is reported when all collection row counts are available
- real Kubernetes pod basic status collection is wired into `check`
- real Kubernetes service inventory is collected
- NodePort service ports now render as `port:nodePort/protocol` for detail output, for example `3000:30031/tcp`
- real Kubernetes endpoint inventory is collected with `EndpointSlice` first and `Endpoints` fallback
- `mq_type` now has minimal reliable detection: explicit config is honored, `pulsar`/`kafka` can be inferred conservatively from K8s service names, and `rocksmq` is supported via explicit config or alias normalization
- `check` text/json output is now driven by real Milvus facts when Milvus is reachable

This branch still should **not** be treated as having full P0 coverage. Collection size, total data size / binlog size, Kubernetes metrics/resource usage, read/write probes, richer collection metrics, and full analyzer rules are still out of scope or skeleton-only.

## 2. Stage assessment

Current stage: **Stage 3 / Real Milvus inventory with row count enrichment plus real Kubernetes basic status, with C1 post-trial fixes**

Suggested next stage target: **Vertical Slice 4 - Kubernetes metrics/resource usage**

Suggested stage sequence:

1. Skeleton
2. Real Milvus inventory vertical slice
3. Real Milvus inventory enrichment
4. Kubernetes metrics/resource usage
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
| Output rendering | Partially implemented | `text` / `json` renderers now expose real Milvus version/database/collection facts plus K8s pod/service/endpoint counts; detail mode includes minimal Milvus and K8s detail, including NodePort `port:nodePort/protocol` rendering |
| Exit-code mapping | Implemented | Pass/Warn/Fail/error mapping path exists |
| Analyzer | Minimal runtime path | Analyzer consumes collected Milvus and K8s facts, warns on partial row count, pod not ready, and restart_count > 0; it is not yet a full P0 rules engine |
| Milvus platform client | Minimally implemented | Real client methods for `GetVersion`, `ListDatabases`, `ListCollections`, and per-collection row count now exist |
| Kubernetes platform client | Minimally implemented | Real client methods for `ListPods`, `ListServices`, and endpoint collection now exist |
| Milvus collector | Minimally implemented | `CollectClusterInfo` and `CollectInventory` are real for version/database/collection inventory and row count enrichment; `arch_profile` detection now accepts `v`-prefixed versions |
| Kubernetes collector | Minimally implemented | Real pod/service/endpoint inventory collection is wired through the check runner, and NodePort service details are preserved in rendered port strings |
| Probes | Placeholder only | Business Read / RW probe real logic is still not implemented |
| Tests | Implemented for this slice | Platform tests, K8s collector tests, runner tests, renderer golden tests, analyzer tests, command/integration tests, and smoke tests cover the current slice |
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
7. collect Kubernetes pod/service/endpoint inventory
8. assemble snapshot and checks
9. run minimal analyzer
10. render text/json output

### 4.3 Real facts now visible in output

- `cluster.milvus_version`
- `cluster.arch_profile`
- `cluster.mq_type`
- `inventory.milvus.database_count`
- `inventory.milvus.collection_count`
- `inventory.milvus.total_row_count`
- `inventory.milvus.databases[].name`
- `inventory.milvus.databases[].collections[]`
- `inventory.milvus.collections[].row_count`
- `summary.total_row_count`
- `summary.pod_count`
- `summary.service_count`
- `summary.endpoint_count`
- `inventory.k8s.pods[]`
- `inventory.k8s.services[]`
- `inventory.k8s.endpoints[]`

## 5. What is intentionally not implemented in this branch

- binlog size / data size
- collection size
- total data size
- index count / index type
- vector field list
- load state
- shard / replica / partition detail beyond current minimal legacy compatibility structs
- Kubernetes metrics / resource usage
- Business Read Probe
- RW Probe
- full analyzer rule matrix
- probes
- standby / confidence advanced logic beyond minimal severity mapping

## 6. Known gaps and technical debt

1. `mq_type` detection is intentionally conservative: only explicit config and clear `pulsar` / `kafka` service-name signals are recognized automatically; otherwise it remains `unknown`.
2. Row count collection currently degrades to `unknown` on a per-collection basis if `GetCollectionStatistics` fails for that collection; the inventory path stays successful but total row count also becomes `unknown`.
3. Collection size and total data size are still not implemented.
4. The analyzer is intentionally minimal and should not yet be described as a full operator-grade health analyzer.
5. Example outputs still demonstrate the failure path because the bundled example config points at an unavailable local Milvus endpoint.
6. Flat legacy packages under `internal/platform` and `internal/collectors` still exist for compatibility; the new real paths are under `internal/platform/milvus`, `internal/platform/k8s`, `internal/collectors/milvus`, and `internal/collectors/k8s`.

## 7. Validation status

The branch currently passes:

- `go test ./...`
- `go build ./...`

## 8. Collaboration rule

For this repository, work is not done unless:

1. code is committed on a dedicated branch
2. the branch is pushed to GitHub
3. branch / commit / changed files / commands run are reported back
