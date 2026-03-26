# milvus-health v0.3.0 Release Notes

`v0.3.0` is the first release candidate intended as an early usable release for DBA / ops validation. It is not positioned as a fully production-grade or fully covered operator platform.

## Highlights

- First release-prep cut with real Milvus and Kubernetes inspection on `check`
- P0 / P1 / P2 scope closed on `main`, including Business Read Probe and RW Probe minimal write/read closure
- Read probe disable toggle is merged and reflected in shipped behavior
- Release packaging is prepared for Linux and macOS on `amd64` / `arm64`, with archives and `checksums.txt`

## Included capabilities

- Real Milvus baseline inventory:
  - server version
  - `arch_profile`
  - databases
  - collections
  - collection-level and total `row_count`
  - collection-level and total `binlog_size_bytes`
- Kubernetes baseline health inventory:
  - pods
  - services
  - endpoints
  - CPU / memory request and limit facts
  - metrics-server-backed usage when available
- Business Read Probe:
  - `DescribeCollection`
  - best-effort row-count check
  - best-effort load-state check
  - query path
  - optional search path when `anns_field` is configured
- RW Probe:
  - explicit dangerous-mode gating
  - pre-existing test-database conflict detection
  - create database
  - create collection
  - insert
  - flush
  - create index
  - load collection await
  - query closure
  - optional cleanup for current-run resources only
- Probe toggle behavior:
  - `probe.read.enabled=false` keeps the check item but returns `business-read-probe=skip`
  - message is `disabled by config`
  - confidence is reduced to `low`

## Compatibility

- Current validation focus is Milvus `2.4.7`
- The implementation is designed around Milvus `v2.4` and `v2.6` architecture profiles
- This release does not claim a complete production support matrix across all Milvus, Kubernetes, MQ, object storage, or managed-service variants
- Release artifacts are prepared for:
  - Linux `amd64`
  - Linux `arm64`
  - macOS `amd64`
  - macOS `arm64`

## Known limitations

- This is still an early usable release for validation, not a claim of comprehensive production readiness
- Dedicated MQ / MinIO / etcd health checks are not part of `v0.3.0`
- The analyzer remains conservative and is not yet a full operator-grade rule engine
- `standby` handling is not fully closed beyond the current minimal severity / confidence path
- RW Probe stops at query-based closure and does not yet provide broader search-verification coverage
- Business Read Probe enters the search branch only when `anns_field` is configured
- Kubernetes usage collection depends on metrics-server availability and permissions
- `binlog_size_bytes` parsing is validated against currently covered payload shapes, not all historical variants

## Upgrade / usage notes

- Build and validation baseline for this release-prep branch is:
  - `go test ./... -count=1`
  - `go build ./...`
- CI verifies PR / push with `go test ./...` and `go build ./...`
- Tag-based release packaging is prepared with `goreleaser release --clean`
- Generated artifacts are compressed archives plus `checksums.txt`
- `milvus-health version` is expected to report the release version from build-time metadata in tagged artifacts
- Users should continue to validate against their own Milvus / Kubernetes topology before treating results as an operational gate
- `check` is safe by default; real Milvus write traffic only happens when `probe.rw.enabled=true`
