# milvus-health Project Status

Last updated: 2026-03-21

## 1. Current conclusion

The current `main` branch is still in the **Skeleton** stage.

This means the repository already has a runnable Go CLI project structure, basic config handling, basic output rendering, exit-code mapping, examples, and smoke tests. However, the repository still should **not** yet be treated as having production-ready Milvus/Kubernetes collection, real probe execution, or a complete health-analysis pipeline on `main`.

### What can be truthfully claimed today

- The project is a Go CLI tool with `check`, `validate`, and `version` commands.
- The project can be built and tested with the current Makefile.
- The project supports static YAML loading and validation.
- The project supports minimal `text` and `json` rendering.
- The project already has example config and example outputs.
- The project already has smoke tests for the current CLI skeleton behavior.

### What cannot be truthfully claimed today

- Real Milvus connection and metadata collection are available on `main`.
- Real Kubernetes connection and pod/resource collection are available on `main`.
- Real Business Read Probe / RW Probe are available on `main`.
- Real health analysis based on collected Milvus/K8s facts is available on `main`.
- The tool can already output per-collection row counts, total row counts, total data size, or pod resource usage from a real environment.

## 2. Stage assessment

Current stage: **Stage 1 / Skeleton**

Suggested next stage target: **Vertical Slice 1 - Real Milvus Inventory**

Suggested stage sequence:

1. Skeleton
2. Real Milvus inventory vertical slice
3. Real Kubernetes basic health vertical slice
4. Analyzer rule expansion
5. Business Read Probe
6. RW Probe
7. Detail-mode enrichment and operator usability polish

## 3. Module status overview

| Module | Status | Current assessment |
|---|---|---|
| CLI (`cmd`) | Implemented | `check` / `validate` / `version` command skeleton exists |
| App entry (`main.go`) | Implemented | Standard CLI entry already exists |
| Config loading | Implemented | YAML loading is present |
| Config validation | Partially implemented | Static validation exists, but only for skeleton-stage fields and rules |
| Default values / CLI overrides | Implemented | Basic defaulting and override path exists |
| Output rendering | Partially implemented | `text` / `json` renderers exist, but only for minimal skeleton output |
| Exit-code mapping | Implemented | Pass/Warn/Fail/error mapping path exists |
| Analyzer | Partially implemented | `InventoryAnalyzer` is wired on `main`, but overall health judgement is still incomplete and skeleton-stage |
| Milvus platform client | Partially implemented | SDK wiring is visible on `main`, but it should not yet be treated as production-validated capability |
| Kubernetes platform client | Partially implemented | `client-go` wiring is visible on `main`, but it should not yet be treated as production-validated capability |
| Collectors | Partially implemented | Basic inventory collection paths exist, but the project still lacks full real-environment delivery confidence |
| Probes | Placeholder only | Business Read / RW probe real logic is not yet visible on `main` |
| Tests | Partially implemented | Smoke tests exist; repository also claims first batch of unit tests, but current `main` is still contract/skeleton-oriented |
| Examples | Implemented | Example config and example outputs exist |

## 4. Current repository facts

### Structure visible on `main`

Top-level structure currently visible:

- `cmd`
- `design_docs`
- `docs`
- `examples`
- `internal`
- `test`
- `.gitignore`
- `Makefile`
- `README.md`
- `go.mod`
- `go.sum`
- `main.go`

### Internal module structure currently visible

Under `internal`, the following module folders are present:

- `analyzers`
- `cli`
- `collectors`
- `config`
- `model`
- `platform`
- `probes`
- `render`

### Test structure currently visible

Under `test`, the repository currently exposes:

- `smoke_test.go`

### Repository history visibility

At the time of this status review, the local `main` branch already contains multiple commits.  
Even so, the current branch state should still be treated as **Skeleton** from a delivery and capability standpoint.

## 5. What is already implemented

The following items are clearly implemented on `main`:

### 5.1 Build / run skeleton

- Go module exists
- `main.go` calls the CLI entry
- `Makefile` provides:
  - `fmt`
  - `test`
  - `build`
  - `run-help`
  - `clean`

### 5.2 CLI command skeleton

The CLI already exposes:

- `milvus-health version`
- `milvus-health check --help`
- `milvus-health validate --help`

### 5.3 Config skeleton

Current config flow already includes:

- YAML file loading
- static validation
- default value application
- CLI override application

### 5.4 Render skeleton

Current render path already includes:

- text output renderer
- json output renderer
- minimal detail flag handling

### 5.5 Exit-code skeleton

Current exit-code path already includes:

- analysis result -> exit code mapping
- app error -> exit code mapping

### 5.6 Examples and smoke tests

Current repository already includes:

- `examples/config.example.yaml`
- `examples/output.text.example.txt`
- `examples/output.json.example.json`
- smoke tests that:
  - build the binary
  - test `version`
  - test `check --help`
  - test `validate --help`
  - validate example config
  - compare current text/json outputs against example golden files

## 6. What is not yet implemented on main

The following items are still not implemented on the current `main` branch:

### 6.1 Real external integrations

- production-ready Milvus SDK/client integration
- production-ready Kubernetes client integration
- real pod metrics integration
- real Milvus version/inventory collection
- real Kubernetes pod/service/status collection

### 6.2 Real health collection and analysis

- real cluster info collection
- real inventory collection
- real per-collection row count collection
- real total row count collection
- real total data size collection
- real pod CPU / memory usage collection
- real analyzer rules driven by collected runtime facts

### 6.3 Real probe execution

- Business Read Probe
- RW Probe
- timeout-aware runtime orchestration for real checks
- cleanup execution for RW test objects

### 6.4 Operator-grade detailed output

The repository does not yet visibly support these real outputs on `main`:

- per-collection row count detail
- per-collection data size detail
- cluster total rows
- cluster total data size
- per-pod CPU usage / memory usage
- request/limit usage ratios
- explicit degraded metrics-unavailable reasoning
- actionable DBA/operator recommendations from real observations

## 7. Known mismatches / review findings

The following review findings should be treated as current technical debt until fixed in code and reflected in `main`:

1. The repository `README.md` still states that the project is in skeleton stage. That conclusion should not yet be relaxed.
2. Although `main` now contains inventory collector and platform wiring, the branch still lacks enough real-environment validation to claim operator-grade health truth.
3. Spec/interface design have already moved forward conceptually, but `main` has not yet caught up with those targets.
4. Any work completed by Codex or Claude Code but not pushed to GitHub is effectively invisible to review and should be treated as not done.

## 8. Collaboration rule from now on

For all future development rounds, the following are mandatory:

1. All code changes must be pushed to GitHub.
2. Prefer a dedicated feature branch for each iteration.
3. The developer must report:
   - branch name
   - final commit SHA
   - changed file list
   - test/build commands executed
   - local validation command(s)
   - a concise change summary
4. The task is **not done** unless the code is visible in GitHub.
5. `docs/project-status.md` must be updated as part of the Definition of Done for each iteration.

## 9. Recommended next iteration

### Iteration target

**Iteration 2: Real Milvus Inventory Vertical Slice**

### Scope

Bring the project from skeleton into a first real runtime path by implementing:

- real Milvus connection
- real Milvus version collection
- architecture profile detection based on real version
- database list / collection list collection
- per-collection row count collection
- total row count summary
- analyzer rules for connectivity and inventory
- richer text/json rendering based on real Milvus facts

### Out of scope for that round

- real Kubernetes metrics collection
- pod CPU/memory usage
- Business Read Probe
- RW Probe
- full analyzer rule matrix
- advanced standby/confidence logic

## 10. Recommended follow-up iteration after that

### Iteration 3: Real Kubernetes Basic Health Vertical Slice

Recommended scope:

- namespace-level pod listing
- ready/restart phase collection
- pod CPU/memory usage collection
- request/limit ratio calculation
- degraded metrics-unavailable semantics
- analyzer rules for pod health and resource pressure

## 11. Reviewer checklist

When reviewing the next implementation round, check these first:

1. Is the code pushed to GitHub?
2. Is the branch/commit clearly provided?
3. Has `project-status.md` been updated?
4. Is `FakeAnalyzer` still wired into the runtime path, or has a real path replaced it?
5. Does `check` still only produce stub output?
6. Are new outputs based on real collected facts rather than invented placeholder data?
7. Are text/json outputs still contract-safe and machine-consumable?

## 12. Suggested update policy

Update this document whenever any of the following happens:

- a new vertical slice is merged
- a major module status changes
- a new real external integration is added
- a planned feature is explicitly deferred
- a spec/interface mismatch is discovered
- a reviewer accepts or rejects an iteration

## 13. Short status summary for external collaborators

Use this summary when briefing Codex / Claude Code / reviewers:

> The current `main` branch is still at skeleton stage. CLI, config, rendering, exit-code mapping, examples, and smoke tests already exist, but real Milvus collection, real Kubernetes collection, real probes, and real analyzer logic are not yet present on `main`. Any claimed progress must be pushed to GitHub and reflected in this status document before it is considered reviewable.
