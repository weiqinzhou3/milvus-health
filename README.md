# milvus-health

`milvus-health` 是面向 DBA、SRE 与运维人员的 Milvus 集群健康检查 CLI。当前版本定位为工程师陪同使用的 beta 工具：`check` 默认以安全模式运行，只执行 inventory 与只读路径；只有显式开启 `probe.rw.enabled=true` 时才会进入会真实写 Milvus 的危险模式。它会连接真实 Milvus 与可选的 Kubernetes 集群，输出稳定的 `summary`、`checks`、`detail` 与 `exit code`，支持 `text` / `json` 两种结果格式，适合终端查看、重定向和自动化集成。

## 项目简介

`milvus-health check` 的职责是执行真实巡检并渲染结果：

- `summary` 聚合数据库、集合、总行数、binlog 大小、Pod / Service / Endpoint 等基础事实
- `mode` 明确展示当前是 `safe` 还是 `dangerous`，以及 RW / cleanup 是否开启
- `checks` 给出 PASS / WARN / FAIL / SKIP 检查项与建议
- `detail` 在开启 `--detail` 后展开 inventory、probe target、probe step 等明细
- `exit code` 用固定退出码表达最终状态，便于脚本化调用

`milvus-health validate` 仍然是独立的静态配置校验入口；真实 Milvus / K8s 连接、inventory 采集和 probe 执行都发生在 `check`。

Phase 02 当前固定了配置契约：

- YAML 未知字段会 fail fast，直接报错退出
- 最终配置优先级固定为 `CLI 显式参数 > YAML 配置 > 默认值`
- `output.format` / `output.detail` 会直接驱动 `check` 输出；CLI 显式传入时覆盖 YAML

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
- 可显式开启的 RW Probe
- `probe.read.enabled` / `probe.rw.enabled` 可通过配置控制是否执行
- 当前重点验证 Milvus `2.4.7`；版本兼容范围按 [design_docs/milvus-health-spec-v1.3.md](design_docs/milvus-health-spec-v1.3.md) 中的 `v2.4` / `v2.6` 设计声明执行

Business Read Probe 当前走真实 Milvus 读路径：

- `DescribeCollection`
- `row_count` best effort
- `load state` best effort
- `query`
- `optional search`

RW Probe 仅在显式开启 `probe.rw.enabled=true` 后进入危险模式，当前走最小可用的真实写读闭环：

- check pre-existing test databases
- create database
- create collection
- insert
- flush
- create-index
- load-collection await
- query
- optional cleanup for current-run resources only

## Quickstart

- 离线安装
  
从 GitHub Releases 下载与你的操作系统和 CPU 架构匹配的压缩包。

```bash
mkdir ./milvus-health
cd ./milvus-health
tar -xzf milvus-health_<version>_<os>_<arch>.tar.gz
chmod +x milvus-health
./milvus-health version
```

- 源码安装

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
- 当前示例配置默认是 `safe` 模式：`probe.rw.enabled=false`、`probe.rw.cleanup=false`
- 当前示例配置故意使用 `127.0.0.1:19530` 和一个不存在的 kubeconfig，用于演示真实失败路径
- 因此在未准备 Milvus / K8s 环境时，`validate` 会成功，而两个 `check` 通常返回 `exit code 2`
- 仓库内置样例输出见 [examples/output.text.example.txt](examples/output.text.example.txt) 与 [examples/output.json.example.json](examples/output.json.example.json)；它们跟踪当前默认非 `--detail` 输出，并显式展示 `Run Mode` / `mode`

## 最小配置说明

[examples/config.example.yaml](examples/config.example.yaml) 是推荐入口，至少需要关注以下字段：

- `cluster`
  - `name`：集群标识
  - `milvus.uri`：Milvus 地址，必须是 `host:port`
  - `milvus.username` / `password` / `token`：可选认证字段
- `output`
  - `format`：`text` 或 `json`
  - `detail`：默认输出是否展开明细；CLI 可用 `--detail` 或 `--detail=false` 显式覆盖
- `probe.read`
  - `enabled`：是否执行 Business Read Probe，默认 `true`
  - `min_success_targets`：最少成功 target 数
  - `targets[*]`：read probe 的数据库、集合、查询表达式和输出字段
- `probe.rw`
  - `enabled`：是否执行 RW Probe；`false` 为默认安全模式，`true` 为危险模式
  - `test_database_prefix`：测试库前缀
  - `cleanup`：是否清理当前 RW Probe 本次运行创建的测试资源
  - `insert_rows` / `vector_dim`：最小写入闭环参数

关键语义：

- YAML 中出现未知字段时，`validate` / `check` 都会直接报 `CONFIG_INVALID`
- `probe.read.enabled=false`：不执行 Business Read Probe，输出中仍保留 `business-read-probe`，状态为 `skip`，message 为 `disabled by config`
- `probe.rw.enabled=false`：不执行 RW Probe，不会真实写入 Milvus，输出中仍保留 `rw-probe`，状态为 `skip`
- `probe.rw.enabled=true`：进入危险模式，会创建临时 database / collection 并执行真实写入
- `probe.rw.cleanup=true`：只会尝试清理“本次运行创建”的测试资源，不会按前缀隐式删除历史测试库
- 若发现历史同前缀测试库已存在，RW Probe 会直接 fail fast，并提示人工清理或更换 `probe.rw.test_database_prefix`
- `probe.read.min_success_targets`：当前必须 `>= 1`；不再放行 `0`

默认值要点：

- `output.format = text`
- `probe.read.enabled = true`
- `probe.read.min_success_targets = 1`
- `probe.read.targets[*].query_expr = "id >= 0"`
- `probe.read.targets[*].topk = 3`
- `probe.rw.enabled = false`
- `probe.rw.cleanup = false`
- `probe.rw.test_database_prefix = milvus_health_test`
- `probe.rw.insert_rows = 100`
- `probe.rw.vector_dim = 128`
- `timeout_sec = 60`
- `rules.resource_warn_ratio = 0.85`

## 输出说明

`check` 输出围绕五部分展开：

- `summary`
  - 数据库数、集合数、总行数、总 binlog 大小、Pod 数、Service 数、Endpoint 数、metrics 覆盖情况
- `mode`
  - 当前是否为 `safe` / `dangerous`，以及 RW / cleanup 是否开启
- `checks`
  - 每条检查项包含 `status`、`message`，必要时包含 `recommendation`、`evidence`、`actual`
- `detail`
  - `output.detail=true` 或 `--detail` 会展开 Milvus inventory、collection 明细、K8s pod / service / endpoint 明细、Business Read Probe targets、RW Probe steps
  - `--detail=false` 会显式关闭 detail，即使 YAML 里是 `true`
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

当能力被配置关闭时，结果会明确表现为 `disabled by config`；例如 `probe.read.enabled=false` 会保留 `business-read-probe [skip]`，同时按当前设计把 `confidence` 降为 `low`。当 probe 因上游失败而未执行时，也会明确输出 `not run because ...`，不再出现空白状态。

## 使用注意事项

- 某些能力依赖真实 Milvus / Kubernetes API 可达；示例配置默认演示的是失败路径
- 当前版本更适合作为工程师陪同使用的 beta 工具，而不是客户自助式“零风险”巡检工具
- `probe.rw.enabled=true` 时，`check` 会进入危险模式：创建临时 database / collection，并执行真实读写
- `cleanup=true` 只会在 RW Probe 结束后尝试删除本次运行创建的测试资源；`cleanup=false` 绝不会执行删除
- 若历史同前缀测试库已存在，当前版本不会再隐式删库，而是直接失败并提示人工处理
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
