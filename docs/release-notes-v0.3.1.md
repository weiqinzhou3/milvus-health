# milvus-health v0.3.1 Release Notes

`v0.3.1` is a patch release on top of the already-published `v0.3.0`. It republishes the completed Phase 1 / Phase 2 line with the current safety-boundary and config/output-contract fixes, while keeping the project positioned as a safe-by-default, engineer-assisted beta tool.

## Highlights

- Safe by default:
  - `check` is explicitly documented as `safe` by default
  - RW probe stays off unless `probe.rw.enabled=true`
  - text / JSON output now render the effective run mode (`safe` or `dangerous`)
- No implicit historical cleanup:
  - RW Probe no longer deletes historical prefixed test databases implicitly
  - pre-existing conflicts now fail fast and require manual cleanup or a different `probe.rw.test_database_prefix`
  - `--cleanup` only applies to resources created by the current RW probe run
- Config / output contract fixes:
  - YAML unknown fields now fail fast during `validate` and `check`
  - effective config precedence is `CLI explicit > YAML > default`
  - YAML `output.format` / `output.detail` now drive rendering correctly without needing matching CLI flags
  - `probe.read.min_success_targets` now validates as `>= 1`
- Better operational clarity:
  - Business Read Probe skip / unavailable paths now emit explicit messages instead of blank failure-path output
  - the release continues to position `milvus-health` as an engineer-assisted beta rather than a self-serve zero-risk platform

## Release artifacts

This release follows the existing GoReleaser configuration already in the repository. Published artifacts are:

- `milvus-health_0.3.1_darwin_amd64.tar.gz`
- `milvus-health_0.3.1_darwin_arm64.tar.gz`
- `milvus-health_0.3.1_linux_amd64.tar.gz`
- `milvus-health_0.3.1_linux_arm64.tar.gz`
- `checksums.txt`

Windows artifacts are not included in this release because the current repository release configuration only targets Linux and macOS.

## Known limitations

- This is still a beta release intended for engineer-assisted usage
- Dedicated MQ / MinIO / etcd health checks are not part of `v0.3.1`
- The analyzer remains conservative and is not yet a full operator-grade rule engine
- `standby` handling is not fully closed beyond the current minimal severity / confidence path
- RW Probe stops at query-based closure and does not yet provide broader search-verification coverage
- Business Read Probe enters the search branch only when `anns_field` is configured
- Kubernetes usage collection depends on metrics-server availability and permissions
- `binlog_size_bytes` parsing is validated against currently covered payload shapes, not all historical variants

## Validation baseline

This release cut is expected to pass:

- `go test ./...`
- `go build ./...`
- `goreleaser release --clean --snapshot --release-notes docs/release-notes-v0.3.1.md`
