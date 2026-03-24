# Changelog

All notable changes to this project will be documented in this file.

## [v0.3.0] - Unreleased

This planned release marks the point where `main` should be described as an early deliverable with real Milvus and Kubernetes inspection, not a skeleton/stub repository.

### Highlights

- P0 completed on the real collection path
- P1 completed with Business Read Probe on the real Milvus path
- P2 completed with RW Probe minimal write/read closure on the real Milvus path
- Read probe disable toggle completed and merged

### Included capabilities

- Real Milvus SDK connectivity in `check`
- Real Milvus inventory collection for version, `arch_profile`, databases, collections, `row_count`, `total_row_count`, and `binlog_size_bytes`
- Real Kubernetes client-go collection for pods, services, endpoints, request/limit facts, and metrics-server-backed usage
- K8s pod / service / endpoint / resource enrichment
- Business Read Probe with target-level evidence and `min_success_targets` handling
- RW Probe with stale-test cleanup, create database, create collection, insert, flush, create-index, load-collection await, query, and optional cleanup
- `text` / `json` output, `--detail`, stable exit codes, and synced example config/output

### Compatibility

- Current validation focus is Milvus `2.4.7`
- Design compatibility target remains `v2.4` (`2.4.x` / `2.5.x`) and `v2.6` (`2.6.x+`) architecture profiles
- `arch_profile=unknown` is handled with degrade semantics instead of process failure

### Added

- Real Milvus SDK connectivity in `check`
- Real Milvus inventory collection for version, `arch_profile`, databases, collections, `row_count`, and `binlog_size_bytes`
- Real Kubernetes client-go collection for pods, services, endpoints, request/limit facts, and metrics-server-backed usage
- Real Business Read Probe with target-level evidence and `min_success_targets` handling
- Real RW Probe with stale-test cleanup, create database, create collection, insert, flush, create-index, load-collection await, query, and optional cleanup
- Detail output for Milvus inventory, K8s inventory, Business Read Probe targets, RW Probe steps, and analyzer checks

### Changed

- `main` is now documented as a real early deliverable with live inspection capability
- README quickstart now documents the current build / validate / detail-mode check commands that were re-run during doc sync
- Bundled output examples now track the current failure-path default output instead of stale pre-sync samples
- README and project status wording no longer describe the project as skeleton, stub, or “no real SDK/client/inventory/probes”

### Known limitations

- Deeper MQ / MinIO / etcd dedicated health checks are not yet included in the current shipped checks
- The analyzer is still conservative and not yet a full operator-grade rule engine
- `standby` is not fully closed beyond the current minimal severity/confidence path
- RW Probe currently ends at the query-based closure and does not yet implement broader search-verification coverage
- Business Read Probe only enters the search branch when `anns_field` is configured
- Kubernetes resource usage depends on metrics-server availability and permissions
- `binlog_size_bytes` parsing is validated for the currently supported payload shapes, not all historical variants
- The `version` subcommand still carries a placeholder version string and should be updated during release cut
