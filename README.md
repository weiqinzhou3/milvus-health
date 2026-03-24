# milvus-health

`milvus-health` 是面向 DBA、SRE 与运维人员的 Milvus 集群健康检查 CLI。它会连接真实 Milvus 与可选的 Kubernetes 集群，输出稳定的 `summary`、`checks`、`detail` 与 `exit code`，支持 `text` / `json` 两种结果格式，适合终端查看、重定向和自动化集成。

当前 `main` 已完成 P0、P1、P2 主线能力，不应再被描述为 skeleton、stub 或“只有静态校验”的仓库。

## 项目简介

`milvus-health check` 的职责是执行真实巡检并渲染结果：

- `summary` 聚合数据库、集合、总行数、binlog 大小、Pod / Service / Endpoint 等基础事实
- `checks` 给出 PASS / WARN / FAIL / SKIP 检查项与建议
- `detail` 在开启 `--detail` 后展开 inventory、probe target、probe step 等明细
- `exit code` 用固定退出码表达最终状态，便于脚本化调用

`milvus-health validate` 仍然是独立的静态配置校验入口；真实 Milvus / K8s 连接、inventory 采集和 probe 执行都发生在 `check`。

## 当前已支持的核心能力

- 真实 Milvus 连接，基于真实 SDK 采集 `milvus_version`
- `arch_profile` 识别，当前覆盖 `v2.4` / `v2.6` 档位
- MQ 类型识别，支持显式配置和基于 K8s service 的保守推断
- database / collection inventory
- collection `row_count` 与 cluster `total_row_count`
- collection / cluster `binlog_size_bytes` 基础能力
- K8s pod / service / endpoint / resource enrichment
- K8s request / limit 与 metrics-server 提供的 CPU / Memory usage
- Business Read Probe
- RW Probe
- `probe.read.enabled` / `probe.rw.enabled` 可通过配置控制是否执行
- 当前重点验证 Milvus `2.4.7`；版本兼容范围按 [design_docs/milvus-health-spec-v1.3.md](design_docs/milvus-health-spec-v1.3.md) 中的 `v2.4` / `v2.6` 设计声明执行

Business Read Probe 当前走真实 Milvus 读路径：

- `DescribeCollection`
- `row_count` best effort
- `load state` best effort
- `query`
- `optional search`

RW Probe 当前走最小可用的真实写读闭环：

- cleanup stale prefixed databases
- create database
- create collection
- insert
- flush
- create-index
- load-collection await
- query
- optional cleanup

## Quickstart

以下命令已在当前仓库实跑验证：

```bash
git clone https://github.com/weiqinzhou3/milvus-health.git
cd milvus-health
go build -o ./bin/milvus-health .
./bin/milvus-health validate --config ./examples/config.example.yaml
./bin/milvus-health check --config ./examples/config.example.yaml --format text --detail
./bin/milvus-health check --config ./examples/config.example.yaml --format json --detail
```

说明：

- 入口配置为 [examples/config.example.yaml](examples/config.example.yaml)
- 当前示例配置故意使用 `127.0.0.1:19530`、`timeout_sec: 1` 和一个不存在的 kubeconfig，用于演示真实失败路径
- 因此在未准备 Milvus / K8s 环境时，`validate` 会成功，而两个 `check` 通常返回 `exit code 2`
- 仓库内置样例输出见 [examples/output.text.example.txt](examples/output.text.example.txt) 与 [examples/output.json.example.json](examples/output.json.example.json)；它们跟踪当前默认非 `--detail` 输出，`--detail` 会在此基础上追加 inventory、checks、probe target / step 明细

## 最小配置说明

[examples/config.example.yaml](examples/config.example.yaml) 是推荐入口，至少需要关注以下字段：

- `cluster`
  - `name`：集群标识
  - `milvus.uri`：Milvus 地址，必须是 `host:port`
  - `milvus.username` / `password` / `token`：可选认证字段
- `output`
  - `format`：`text` 或 `json`
  - `detail`：默认输出是否展开明细
- `probe.read`
  - `enabled`：是否执行 Business Read Probe，默认 `true`
  - `min_success_targets`：最少成功 target 数
  - `targets[*]`：read probe 的数据库、集合、查询表达式和输出字段
- `probe.rw`
  - `enabled`：是否执行 RW Probe
  - `test_database_prefix`：测试库前缀
  - `cleanup`：是否自动清理 RW Probe 产生的测试资源
  - `insert_rows` / `vector_dim`：最小写入闭环参数

关键语义：

- `probe.read.enabled=false`：不执行 Business Read Probe，输出中仍保留 `business-read-probe`，状态为 `skip`，message 为 `disabled by config`
- `probe.rw.enabled=false`：不执行 RW Probe，输出中仍保留 `rw-probe`，状态为 `skip`

默认值要点：

- `output.format = text`
- `probe.read.enabled = true`
- `probe.read.min_success_targets = 1`
- `probe.read.targets[*].query_expr = "id >= 0"`
- `probe.read.targets[*].topk = 3`
- `probe.rw.test_database_prefix = milvus_health_test`
- `probe.rw.insert_rows = 100`
- `probe.rw.vector_dim = 128`
- `timeout_sec = 60`
- `rules.resource_warn_ratio = 0.85`

## 输出说明

`check` 输出围绕四部分展开：

- `summary`
  - 数据库数、集合数、总行数、总 binlog 大小、Pod 数、Service 数、Endpoint 数、metrics 覆盖情况
- `checks`
  - 每条检查项包含 `status`、`message`，必要时包含 `recommendation`、`evidence`、`actual`
- `detail`
  - `--detail` 会展开 Milvus inventory、collection 明细、K8s pod / service / endpoint 明细、Business Read Probe targets、RW Probe steps
- `exit_code`
  - `0` = PASS
  - `1` = WARN
  - `2` = FAIL
  - `3` = config invalid
  - `4` = runtime / render error

状态语义：

- `PASS`：检查通过
- `WARN`：存在降级、部分缺失或保守告警
- `FAIL`：关键连接、采集或 probe 失败
- `SKIP`：该能力未执行，常见于配置关闭、前置条件不满足或上游采集失败

当能力被配置关闭时，结果会明确表现为 `disabled by config`；例如 `probe.read.enabled=false` 会保留 `business-read-probe [skip]`，同时按当前设计把 `confidence` 降为 `low`。

## 使用注意事项

- 某些能力依赖真实 Milvus / Kubernetes API 可达；示例配置默认演示的是失败路径
- `probe.rw.enabled=true` 时，`check` 不是纯只读命令，会创建临时 database / collection 并执行真实读写
- `cleanup=true` 会在 RW Probe 结束后尝试删除测试资源；`cleanup=false` 适合调试，但会保留测试资源
- 建议为 `probe.rw.test_database_prefix` 使用专用前缀，避免与真实业务库冲突
- 关闭 read probe 不会导致整体直接失败，但会让 `business-read-probe` 变为 `skip`，并将 `confidence` 降为 `low`
- K8s 资源使用率依赖 `metrics-server` 与权限；缺失时会按 degrade 语义输出，而不是让命令崩溃

## 开发与测试

```bash
go test ./...
go build ./...
```

文档入口：

- [docs/project-status.md](docs/project-status.md)
- [design_docs/milvus-health-spec-v1.3.md](design_docs/milvus-health-spec-v1.3.md)
- [design_docs/milvus-health-interface-design-v0.7.md](design_docs/milvus-health-interface-design-v0.7.md)
- [CHANGELOG.md](CHANGELOG.md)
