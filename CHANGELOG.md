# Changelog

All notable changes to this project will be documented in this file.

## [v0.3.1] - 2026-03-27

This patch release publishes the completed Phase 1 / Phase 2 line on top of the already-public `v0.3.0` baseline. It keeps the current beta positioning, but tightens the shipped safety boundary and config/output contract.

### Highlights

- `check` is now explicitly documented and rendered as safe-by-default, with visible `safe` / `dangerous` run mode output
- RW Probe no longer performs implicit historical cleanup of prefixed test databases; pre-existing conflicts now fail fast and require manual handling
- Phase 02 config/output fixes are now part of the shipped release: strict YAML known-field validation, `CLI explicit > YAML > default`, and YAML-driven `output.format` / `output.detail`
- Business Read Probe skip / unavailable paths now render explicit messages instead of blank failure-path output
- This remains an engineer-assisted beta release rather than a self-serve zero-risk operator platform

### Changed

- `check --help`, text output, and JSON output now expose the current run mode and cleanup intent
- `probe.read.min_success_targets` validation is tightened to `>= 1`
- `validate` and `check` now reject unknown YAML fields instead of silently ignoring them
- Release packaging continues to use the existing GoReleaser-based Linux/macOS archive flow with `checksums.txt`

### Known limitations

- Dedicated MQ / MinIO / etcd health checks are not included yet
- The analyzer remains conservative and is not yet a full operator-grade rule engine
- `standby` handling is not fully closed beyond the current minimal severity / confidence path
- RW Probe still ends at query-based closure and does not yet provide broader search-verification coverage
- Business Read Probe only enters the search branch when `anns_field` is configured
- Kubernetes resource usage still depends on metrics-server availability and permissions

## [v0.3.0] - 2026-03-24

This release marks the point where `main` should be described as an early deliverable with real Milvus and Kubernetes inspection, not a skeleton/stub repository.

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
