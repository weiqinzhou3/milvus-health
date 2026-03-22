# milvus-health Project Status

Last updated: 2026-03-22

## 1. Current conclusion

The current working branch now provides **Iteration A2 / Milvus Inventory Enrichment**, **Iteration B / Kubernetes Basic Status Collection**, **Iteration B2 / Kubernetes Metrics Enrichment**, and **Iteration D1 / Milvus Binlog Size** on the real collection path.

This branch can now truthfully claim:

- real Milvus SDK connection is wired into `check`
- real `milvus_version` is collected
- `arch_profile` is derived from the real version using the spec v1.2 mapping, including `v`-prefixed versions such as `v2.4.7` and `v2.6.1`
- real database names and per-database collection name lists are collected
- real per-collection row count collection is wired through `GetCollectionStatistics`
- real cluster total row count is reported when all collection row counts are available
- real per-collection `binlog_size_bytes` is collected from Milvus `GetMetrics("system_info")` / DataCoord quota metrics
- real cluster total `binlog_size_bytes` is collected from Milvus `GetMetrics("system_info")`
- Milvus binlog size now degrades to `null` / `unknown` instead of failing the whole inventory when metrics collection is unavailable or partial
- real Kubernetes pod basic status collection is wired into `check`
- real Kubernetes pod CPU/memory usage collection is wired into `check` when metrics-server is available
- real Kubernetes pod CPU/memory request and limit collection is wired into `check`
- real Kubernetes pod usage/limit and usage/request ratio rendering is wired into text/json output, with `null` / `unknown` degrade semantics when metrics are missing
- real Kubernetes metrics degrade semantics are now surfaced, including `metrics-server not found`, `insufficient permissions`, and partial metrics coverage
- real Kubernetes service inventory is collected
- NodePort service ports now render as `port:nodePort/protocol` for detail output, for example `3000:30031/tcp`
- real Kubernetes endpoint inventory is collected with `EndpointSlice` first and `Endpoints` fallback
- `mq_type` now has minimal reliable detection: explicit config is honored, `pulsar`/`kafka` can be inferred conservatively from K8s service names, and `rocksmq` is supported via explicit config or alias normalization
- `check` text/json output is now driven by real Milvus facts when Milvus is reachable

This branch still should **not** be treated as having full P0 coverage. Business read / RW probes, richer Milvus inventory metrics beyond row count and binlog size, Prometheus/component-metrics resource usage sources, and the full analyzer rule matrix are still out of scope or skeleton-only.

## 2. Stage assessment

Current stage: **Stage 4 / Real Milvus inventory with row count and binlog size enrichment plus Kubernetes basic status and metrics/resource usage**

Suggested next stage target: **Stage 5 - Analyzer rule expansion**

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
| Output rendering | Partially implemented | `text` / `json` renderers now expose real Milvus version/database/collection facts, total row count, total `binlog_size_bytes`, per-collection `binlog_size_bytes`, plus K8s pod/service/endpoint counts, pod CPU/memory usage, request/limit facts, ratio fields, and metrics degrade summaries; detail mode includes minimal Milvus and K8s detail, including NodePort `port:nodePort/protocol` rendering |
| Exit-code mapping | Implemented | Pass/Warn/Fail/error mapping path exists |
| Analyzer | Minimal runtime path | Analyzer consumes collected Milvus and K8s facts, warns on partial row count, partial/unknown binlog size, pod not ready, restart_count > 0, metrics unavailable/partial, and usage/limit ratio threshold breaches; it is not yet a full P0 rules engine |
| Milvus platform client | Minimally implemented | Real client methods for `GetVersion`, `ListDatabases`, `ListCollections`, collection ID lookup, per-collection row count, and `GetMetrics("system_info")` now exist |
| Kubernetes platform client | Minimally implemented | Real client methods for `ListPods`, `ListServices`, `ListEndpoints`, and `ListPodMetrics` now exist, with spec-aligned metrics degrade semantics |
| Milvus collector | Minimally implemented | `CollectClusterInfo` and `CollectInventory` are real for version/database/collection inventory, row count enrichment, and `binlog_size_bytes` enrichment; `arch_profile` detection now accepts `v`-prefixed versions |
| Kubernetes collector | Minimally implemented | Real pod/service/endpoint inventory collection is wired through the check runner; pod metrics, request/limit enrichment, ratio calculation, and partial/unavailable metrics semantics are now included; NodePort service details are preserved in rendered port strings |
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
- real per-collection collection ID lookup via `DescribeCollection`
- real per-collection and total `binlog_size_bytes` via `GetMetrics("system_info")`
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
- `inventory.milvus.total_binlog_size_bytes`
- `inventory.milvus.databases[].name`
- `inventory.milvus.databases[].collections[]`
- `inventory.milvus.collections[].row_count`
- `inventory.milvus.collections[].binlog_size_bytes`
- `summary.total_row_count`
- `summary.total_binlog_size_bytes`
- `summary.pod_count`
- `summary.service_count`
- `summary.endpoint_count`
- `inventory.k8s.pods[]`
- `inventory.k8s.services[]`
- `inventory.k8s.endpoints[]`

## 5. What is intentionally not implemented in this branch

- collection size
- total data size
- index count / index type
- vector field list
- load state
- shard / replica / partition detail beyond current minimal legacy compatibility structs
- Business Read Probe
- RW Probe
- full analyzer rule matrix
- probes
- Prometheus / component metrics resource usage sources
- standby / confidence advanced logic beyond minimal severity mapping

## 6. Known gaps and technical debt

1. `mq_type` detection is intentionally conservative: only explicit config and clear `pulsar` / `kafka` service-name signals are recognized automatically; otherwise it remains `unknown`.
2. Row count collection currently degrades to `unknown` on a per-collection basis if `GetCollectionStatistics` fails for that collection; the inventory path stays successful but total row count also becomes `unknown`.
3. Collection size / index detail / load-state detail and total data size remain out of scope beyond the current row count and binlog size slice.
4. The analyzer is intentionally minimal and should not yet be described as a full operator-grade health analyzer.
5. Example outputs still demonstrate the failure path because the bundled example config points at an unavailable local Milvus endpoint.
6. Flat legacy packages under `internal/platform` and `internal/collectors` still exist for compatibility; the new real paths are under `internal/platform/milvus`, `internal/platform/k8s`, `internal/collectors/milvus`, and `internal/collectors/k8s`.
7. Business read probe and RW probe are still not implemented.
8. Prometheus-backed or component-metrics-backed resource usage sources are still not implemented.

## 7. Validation status

The branch currently passes:

- `go test ./...`
- `go build ./...`

## 8. Collaboration rule

For this repository, work is not done unless:

1. code is committed on a dedicated branch
2. the branch is pushed to GitHub
3. branch / commit / changed files / commands run are reported back
