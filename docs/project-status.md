# milvus-health Project Status

Last updated: 2026-03-22

## 1. Current conclusion

The current working branch now provides **Iteration A2 / Milvus Inventory Enrichment**, **Iteration B / Kubernetes Basic Status Collection**, **Iteration B2 / Kubernetes Metrics Enrichment**, **Iteration D1 / Milvus Binlog Size**, **Iteration D1.1 / system_info parser compatibility**, and **Iteration P1 / Business Read Probe** on the real collection path.

This branch can now truthfully claim:

- real Milvus SDK connection is wired into `check`
- real `milvus_version` is collected
- `arch_profile` is derived from the real version using the spec v1.2 mapping, including `v`-prefixed versions such as `v2.4.7` and `v2.6.1`
- real database names and per-database collection name lists are collected
- real per-collection row count collection is wired through `GetCollectionStatistics`
- real cluster total row count is reported when all collection row counts are available
- Milvus binlog size collection remains sourced from `GetMetrics("system_info")` / DataCoord quota metrics
- D1 exposed a real Milvus 2.4.7 parser-compatibility gap: some `system_info` payloads report binlog metrics under `nodes_info[].infos.quota_metrics.TotalBinlogSize` / `CollectionBinlogSize`
- D1.1 updates the parser to accept both snake_case fake-test payloads and the observed 2.4.7 nested/CamelCase payload shape
- binlog size still degrades to `null` / `unknown` instead of failing the whole inventory when metrics collection fails or the payload cannot be parsed
- real Kubernetes pod basic status collection is wired into `check`
- real Kubernetes pod CPU/memory usage collection is wired into `check` when metrics-server is available
- real Kubernetes pod CPU/memory request and limit collection is wired into `check`
- real Kubernetes pod usage/limit and usage/request ratio rendering is wired into text/json output, with `null` / `unknown` degrade semantics when metrics are missing
- real Kubernetes metrics degrade semantics are now surfaced, including `metrics-server not found`, `insufficient permissions`, and partial metrics coverage
- real Kubernetes service inventory is collected
- NodePort service ports now render as `port:nodePort/protocol` for detail output, for example `3000:30031/tcp`
- real Kubernetes endpoint inventory is collected with `EndpointSlice` first and `Endpoints` fallback
- `mq_type` now has minimal reliable detection: explicit config is honored, `pulsar`/`kafka` can be inferred conservatively from K8s service names, and `rocksmq` is supported via explicit config or alias normalization
- real Business Read Probe is wired into `check`: configured `probe.read.targets` execute minimal `DescribeCollection -> row_count(best effort) -> load state(best effort) -> query -> optional search` probe flow, target-level evidence is preserved, and `min_success_targets` drives pass/warn/fail
- `check` text/json output is now driven by real Milvus facts when Milvus is reachable

This branch still should **not** be treated as having full P0 coverage. RW probe, richer Milvus inventory metrics beyond row count and binlog size, Prometheus/component-metrics resource usage sources, the full analyzer rule matrix, and a fully closed standby judgment path are still out of scope or skeleton-only. Binlog size should also not yet be described as universally stable beyond the currently validated payload shapes.

## 2. Stage assessment

Current stage: **Stage 5 / Business Read Probe added on top of real Milvus inventory, binlog size, and Kubernetes basic status plus metrics/resource usage**

Suggested next stage target: **Stage 6 - RW Probe**

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
| Output rendering | Partially implemented | `text` / `json` renderers now expose real Milvus version/database/collection facts, total row count, total `binlog_size_bytes`, per-collection `binlog_size_bytes`, Business Read Probe summary, plus K8s pod/service/endpoint counts, pod CPU/memory usage, request/limit facts, ratio fields, and metrics degrade summaries; detail mode includes minimal Milvus, Business Read Probe target detail, and K8s detail, including NodePort `port:nodePort/protocol` rendering |
| Exit-code mapping | Implemented | Pass/Warn/Fail/error mapping path exists |
| Analyzer | Minimal runtime path | Analyzer consumes collected Milvus and K8s facts plus Business Read Probe results, warns on partial row count, partial/unknown binlog size, pod not ready, restart_count > 0, metrics unavailable/partial, usage/limit ratio threshold breaches, and read-probe warn/fail/skip states; it is not yet a full P0 rules engine or full standby analyzer |
| Milvus platform client | Minimally implemented | Real client methods for `GetVersion`, `ListDatabases`, `ListCollections`, collection ID lookup, per-collection row count, minimal `DescribeCollection`, load-state lookup, query, search, and `GetMetrics("system_info")` now exist |
| Kubernetes platform client | Minimally implemented | Real client methods for `ListPods`, `ListServices`, `ListEndpoints`, and `ListPodMetrics` now exist, with spec-aligned metrics degrade semantics |
| Milvus collector | Minimally implemented | `CollectClusterInfo` and `CollectInventory` are real for version/database/collection inventory, row count enrichment, and `binlog_size_bytes` enrichment; D1.1 extends `system_info` parsing to the observed nested/CamelCase 2.4.7 payload shape; `arch_profile` detection now accepts `v`-prefixed versions |
| Kubernetes collector | Minimally implemented | Real pod/service/endpoint inventory collection is wired through the check runner; pod metrics, request/limit enrichment, ratio calculation, and partial/unavailable metrics semantics are now included; NodePort service details are preserved in rendered port strings |
| Probes | Partially implemented | Business Read Probe real logic is implemented; RW probe remains placeholder/no-op only |
| Tests | Implemented for this slice | Platform tests, K8s collector tests, Business Read Probe unit tests, runner tests, renderer golden tests, analyzer tests, command/integration tests, and smoke tests cover the current slice |
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
- parser compatibility for both snake_case test payloads and the observed Milvus 2.4.7 nested/CamelCase `quota_metrics` payload shape
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
8. execute Business Read Probe
9. assemble snapshot and checks
10. run minimal analyzer
11. render text/json output

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
- `probes.business_read.status`
- `probes.business_read.configured_targets`
- `probes.business_read.successful_targets`
- `probes.business_read.min_success_targets`
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
- RW Probe
- full analyzer rule matrix
- Prometheus / component metrics resource usage sources
- standby / confidence advanced logic beyond minimal severity mapping

## 6. Known gaps and technical debt

1. `mq_type` detection is intentionally conservative: only explicit config and clear `pulsar` / `kafka` service-name signals are recognized automatically; otherwise it remains `unknown`.
2. Row count collection currently degrades to `unknown` on a per-collection basis if `GetCollectionStatistics` fails for that collection; the inventory path stays successful but total row count also becomes `unknown`.
3. Collection size / index detail / load-state detail and total data size remain out of scope beyond the current row count and binlog size slice.
4. The analyzer is intentionally minimal and should not yet be described as a full operator-grade health analyzer.
5. Example outputs still demonstrate the failure path because the bundled example config points at an unavailable local Milvus endpoint.
6. Flat legacy packages under `internal/platform` and `internal/collectors` still exist for compatibility; the new real paths are under `internal/platform/milvus`, `internal/platform/k8s`, `internal/collectors/milvus`, and `internal/collectors/k8s`.
7. RW probe is still not implemented.
8. Standby is still not fully closed: `require_probe_for_standby` and broader standby rule coverage are not yet fully implemented beyond the current minimal severity/confidence path.
9. Prometheus-backed or component-metrics-backed resource usage sources are still not implemented.
10. Binlog size parsing is now compatible with the validated snake_case payload and the observed Milvus 2.4.7 nested/CamelCase payload, but it should not yet be described as broadly validated across all historical payload variants.

## 7. Validation status

The branch currently passes:

- `go test ./...`
- `go build ./...`

## 8. Collaboration rule

For this repository, work is not done unless:

1. code is committed on a dedicated branch
2. the branch is pushed to GitHub
3. branch / commit / changed files / commands run are reported back
