# milvus-health Project Status

Last updated: 2026-03-23

## 1. Current conclusion

`main` 当前已经合并 P0、P1、P2 迭代成果，不再应被描述为 skeleton、stub，或“只有静态校验、没有真实巡检能力”的仓库。

当前 `main` 可以被准确描述为：

- 一个具备真实 Milvus / Kubernetes 巡检能力的早期可交付版本
- `check` 已接入真实 Milvus SDK、真实 Kubernetes client-go、真实 inventory 采集，以及真实 Probe
- `validate` 是独立的静态配置校验入口；真实环境连通性、inventory 与 probe 都在 `check` 中执行
- `text` / `json` 输出、`--detail`、退出码、样例配置与样例输出都已经形成可用闭环

## 2. Milestone status

| Milestone | Status | Notes |
|---|---|---|
| P0 | Completed and merged | 真实 Milvus/K8s 基础采集、配置加载校验、渲染、退出码链路已闭环 |
| P1 | Completed and merged | Business Read Probe 已接入真实 Milvus 路径 |
| P2 | Completed and merged | RW Probe 最小写读闭环已接入真实 Milvus 路径 |

这里的 “P0 / P1 / P2 completed” 指当前仓库定义的这三轮交付范围已经完成并合并到 `main`，不等于长期 roadmap 上所有后续增强项都已完成。

## 3. Capability scope on `main`

### 3.1 CLI and config

- `check` / `validate` / `version` 命令路径均已存在
- YAML 加载、默认值注入、CLI override、静态校验已接通
- `cluster.milvus.uri`、`output.format`、`probe.read.*`、`probe.rw.*`、`rules.resource_warn_ratio` 等关键字段已有约束

### 3.2 Real Milvus collection

- 真实 Milvus SDK 连接已接入 `check`
- 已采集：
  - `milvus_version`
  - `arch_profile`
  - 数据库列表
  - 每库集合列表
  - 每集合 `row_count`
  - 集群总 `row_count`
  - 每集合与总 `binlog_size_bytes`
- `arch_profile` 识别已覆盖 `v2.4` / `v2.6` 路径，并兼容 `v` 前缀版本号

### 3.3 Real Kubernetes collection

- 真实 Kubernetes client-go 已接入 `check`
- 已采集：
  - Pods
  - Services
  - Endpoints
  - Pod CPU/Memory request 与 limit
  - metrics-server 提供的 CPU/Memory usage
- 已实现 degrade 语义：
  - `metrics-server not found`
  - `insufficient permissions`
  - partial metrics coverage

### 3.4 Real probes

- Business Read Probe 已接入真实路径：
  - `DescribeCollection`
  - `row_count` best effort
  - `load state` best effort
  - `query`
  - `optional search`
- RW Probe 已接入真实路径：
  - cleanup stale prefixed databases
  - create database
  - create collection
  - insert
  - flush
  - create-index
  - load-collection(await)
  - query
  - optional cleanup

### 3.5 Output and analysis

- `text` / `json` 输出已反映真实采集结果
- `--detail` 已展开：
  - Milvus inventory
  - collection detail
  - K8s pod/service/endpoint detail
  - Business Read Probe target detail
  - RW Probe step detail
  - `checks`
- Analyzer 已能基于真实结果给出最小可用 PASS / WARN / FAIL 判断，并处理连接失败、inventory 缺失、probe skip/fail、K8s metrics 缺失、Pod readiness/restart、usage/limit ratio 等场景

## 4. What this repository should no longer claim

以下旧口径已经不再符合当前 `main`：

- “still skeleton”
- “check is stub”
- “validate only static, no real network” 作为整个项目状态的总结
- “no real Milvus SDK / K8s client / inventory / probes”

准确说法应是：

- `validate` 仍然是静态配置校验命令
- `check` 已经是连接真实 Milvus / K8s 并执行真实 probe 的巡检命令

## 5. Known limitations

- 当前 analyzer 仍然偏保守，尚未成为完整的 operator-grade 规则引擎
- `standby` 路径仍未完全闭环；当前结果更偏向保守表达
- RW Probe 当前是最小可用 query-based closure，还不是更完整的 search-verification 版本
- Business Read Probe 只有在配置 `anns_field` 时才会进入 search 分支
- K8s 资源使用率依赖 `metrics-server` 与权限；缺失时按 degrade 语义处理
- `binlog_size_bytes` 解析已覆盖当前验证过的 payload 形态，但尚未声称覆盖所有历史变体
- [examples/config.example.yaml](../examples/config.example.yaml) 故意使用失败路径示例，便于快速演示 detail 输出；它不是生产环境配置
- `version` 子命令的版本字符串仍是占位值，正式 release cut 时需要单独更新

## 6. Validation status

当前仓库应至少通过：

- `go test ./...`
- `go build ./...`
- `./bin/milvus-health validate --config examples/config.example.yaml`
- `./bin/milvus-health check --config examples/config.example.yaml --format text --detail`
- `./bin/milvus-health check --config examples/config.example.yaml --format json --detail`

## 7. Documentation sync note

本文件、[README.md](../README.md)、[CHANGELOG.md](../CHANGELOG.md)、[examples/output.text.example.txt](../examples/output.text.example.txt)、[examples/output.json.example.json](../examples/output.json.example.json) 应保持同步，统一反映：

- `main` 已完成 P0 / P1 / P2
- 仓库已具备真实巡检能力
- 当前阶段是“早期可交付版本”，而不是 skeleton/stub
