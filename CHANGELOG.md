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
- RW Probe with explicit dangerous-mode gating, pre-existing test-database conflict detection, create database, create collection, insert, flush, create-index, load-collection await, query, and optional current-run cleanup
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
- Real RW Probe with explicit dangerous-mode gating, pre-existing test-database conflict detection, create database, create collection, insert, flush, create-index, load-collection await, query, and optional current-run cleanup
- Detail output for Milvus inventory, K8s inventory, Business Read Probe targets, RW Probe steps, and analyzer checks

### Changed

- `main` is now documented as a real early deliverable with live inspection capability
- `check` is now documented and rendered as safe-by-default, with explicit `safe` / `dangerous` mode output
- Phase 02 now enforces strict YAML known-field validation; unknown config keys fail fast instead of being silently ignored
- `output.format` / `output.detail` now follow a single merge rule: `CLI explicit > YAML > default`
- `check` rendering now consumes the merged effective config, so YAML output settings work without requiring matching CLI flags
- `probe.read.min_success_targets` validation is now tightened to `>= 1` to match the currently supported runtime semantics
- Business Read Probe now renders explicit unavailable/not-run messages instead of leaving failure-path output blank
- Historical prefixed RW test databases are no longer deleted implicitly; conflicts now fail fast and require manual handling
- README quickstart now documents the current build / validate / detail-mode check commands that were re-run during doc sync
- Bundled output examples now track the current failure-path default output instead of stale pre-sync samples
- README and project status wording no longer describe the project as skeleton, stub, or “no real SDK/client/inventory/probes”
- Release preparation now includes GoReleaser packaging for Linux and macOS on `amd64` / `arm64`
- GitHub Actions now provides a minimal verify path on PR / push and a tag-driven release path that emits archives plus `checksums.txt`

### Release artifacts

- Binary archives:
  - `milvus-health_<version>_linux_amd64.tar.gz`
  - `milvus-health_<version>_linux_arm64.tar.gz`
  - `milvus-health_<version>_darwin_amd64.tar.gz`
  - `milvus-health_<version>_darwin_arm64.tar.gz`
- Checksum manifest:
  - `checksums.txt`
- Development builds may report a development value such as `dev`
- Release builds inject the real version through `ldflags`
- `milvus-health version` in release artifacts reports the injected release version

### Known limitations

- Deeper MQ / MinIO / etcd dedicated health checks are not yet included in the current shipped checks
- The analyzer is still conservative and not yet a full operator-grade rule engine
- `standby` is not fully closed beyond the current minimal severity/confidence path
- RW Probe currently ends at the query-based closure and does not yet implement broader search-verification coverage
- Business Read Probe only enters the search branch when `anns_field` is configured
- Kubernetes resource usage depends on metrics-server availability and permissions
- `binlog_size_bytes` parsing is validated for the currently supported payload shapes, not all historical variants
