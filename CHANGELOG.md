# Changelog

All notable changes to this project will be documented in this file.

## [v0.3.0] - Unreleased

This planned release marks the point where `main` should be described as an early deliverable instead of a skeleton/stub repository.

### Highlights

- P0 closed on the real collection path
- P1 closed with Business Read Probe on the real Milvus path
- P2 closed with RW Probe minimal write/read closure on the real Milvus path

### Added

- Real Milvus SDK connectivity in `check`
- Real Milvus inventory collection for version, `arch_profile`, databases, collections, `row_count`, and `binlog_size_bytes`
- Real Kubernetes client-go collection for pods, services, endpoints, request/limit facts, and metrics-server-backed usage
- Real Business Read Probe with target-level evidence and `min_success_targets` handling
- Real RW Probe with stale-test cleanup, create database, create collection, insert, flush, create-index, load-collection await, query, and optional cleanup
- Detail output for Milvus inventory, K8s inventory, Business Read Probe targets, RW Probe steps, and analyzer checks

### Changed

- `main` is now documented as a real early deliverable with live inspection capability
- README quickstart now documents the current detail-mode commands, while the bundled output examples continue to track the default text/json failure-path outputs
- README and project status wording no longer describe the project as skeleton, stub, or “no real SDK/client/inventory/probes”

### Known limitations

- The analyzer is still conservative and not yet a full operator-grade rule engine
- `standby` is not fully closed beyond the current minimal severity/confidence path
- RW Probe currently ends at the query-based closure and does not yet implement broader search-verification coverage
- Kubernetes resource usage depends on metrics-server availability and permissions
- `binlog_size_bytes` parsing is validated for the currently supported payload shapes, not all historical variants
- The `version` subcommand still carries a placeholder version string and should be updated during release cut
