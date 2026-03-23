# milvus-health

`milvus-health` 是一个面向 Milvus 集群与其 Kubernetes 部署的 Go CLI 巡检工具。`main` 当前已经合并 P0、P1、P2 迭代，不再是 skeleton/stub 仓库，而是一个具备真实巡检能力的早期可交付版本。

## 项目简介

这个项目的目标是用标准 Unix CLI 风格给出稳定的巡检结果：

- 正式结果输出到 `stdout`
- 错误、warning 与诊断信息保留在结构化结果里
- 最终状态通过固定退出码表达

当前 `check` 已经接入真实 Milvus SDK、真实 Kubernetes client-go、真实 inventory 采集、Business Read Probe，以及最小可用的 RW Probe 闭环。

## 当前能力范围

- 真实 Milvus 连接与基础事实采集：`milvus_version`、`arch_profile`、数据库/集合清单、`row_count`、`binlog_size_bytes`
- 真实 Kubernetes 采集：Pods、Services、Endpoints、request/limit、metrics-server 提供的 CPU/Memory usage，以及缺失时的 degrade 语义
- 真实 Probe：
  - Business Read Probe：`DescribeCollection -> row_count(best effort) -> load state(best effort) -> query -> optional search`
  - RW Probe：`cleanup stale db -> create database -> create collection -> insert -> flush -> create-index -> load-collection(await) -> query -> optional cleanup`
- 输出能力：`text` / `json`、`--detail` 明细、稳定退出码、`summary` / `checks` / `failures` / `warnings`
- 配置能力：YAML 加载、默认值注入、静态校验、CLI 覆盖
- 依赖识别：显式 `dependencies.mq.type`，或基于 Kubernetes service 名称的保守推断

## Quickstart

```bash
make build
./bin/milvus-health validate --config examples/config.example.yaml
./bin/milvus-health check --config examples/config.example.yaml --format text --detail
./bin/milvus-health check --config examples/config.example.yaml --format json --detail
```

说明：

- 构建产物位于 `./bin/milvus-health`
- `examples/config.example.yaml` 默认使用 `127.0.0.1:19530`、`timeout_sec: 1`，并故意指向一个不存在的 kubeconfig，用于演示失败路径与 detail 输出
- 因此 `validate` 应该成功；如果你没有准备真实 Milvus/K8s 环境，上述两个 `check` 通常会返回 `exit code 2`
- 仓库内置的默认非 `--detail` 输出样例见 [examples/output.text.example.txt](examples/output.text.example.txt) 和 [examples/output.json.example.json](examples/output.json.example.json)；开启 `--detail` 时会在此基础上追加 inventory / checks / probe step 明细

## 最小配置说明

最小必填字段：

- `cluster.name`
- `cluster.milvus.uri`

最小示例：

```yaml
cluster:
  name: demo-cluster
  milvus:
    uri: 127.0.0.1:19530

output:
  format: text
```

字段说明：

- `cluster.milvus.uri` 必须是 `host:port`，不能带 `tcp://`、`http://` 等 scheme
- `cluster.milvus.username`、`cluster.milvus.password`、`cluster.milvus.token` 为可选认证字段
- `k8s.kubeconfig` 可选；未提供时会尝试 in-cluster config
- `dependencies.mq.type` 可选，支持 `pulsar`、`kafka`、`rocksmq`、`unknown`、`woodpecker`
- `output.format` 支持 `text` 或 `json`

当前默认值：

- `output.format = text`
- `probe.read.min_success_targets = 1`
- `probe.read.targets[*].query_expr = "id >= 0"`
- `probe.read.targets[*].topk = 3`
- `probe.rw.test_database_prefix = milvus_health_test`
- `probe.rw.insert_rows = 100`
- `probe.rw.vector_dim = 128`
- `timeout_sec = 60`
- `rules.resource_warn_ratio = 0.85`

`validate` 负责配置加载、默认值与静态校验；真实连接、inventory 与 probe 执行发生在 `check`。

完整样例配置见 [examples/config.example.yaml](examples/config.example.yaml)。

## 输出说明

`check` 的核心输出字段包括：

- `summary`
  - 聚合数据库数、集合数、总行数、总 binlog 大小、Pod/Service/Endpoint 数量，以及 K8s metrics 覆盖情况
- `checks`
  - 每条检查项都带有 `status`、`message`，可能还包含 `recommendation`、`evidence`、`actual`、`expected`
  - `json` 模式下，只有开启 `--detail` 时才会输出完整 `checks`
- `detail`
  - `--detail` 会展开 inventory、collection 详情、pod/service/endpoint 详情、Business Read Probe 目标明细、RW Probe 步骤明细
- `exit_code`
  - `0` = PASS
  - `1` = WARN
  - `2` = FAIL
  - `3` = config invalid
  - `4` = runtime / render error

## Real-env 注意事项

- `probe.rw.enabled=true` 时，`check` 不再是纯只读命令；它会创建临时 database / collection，并在 `cleanup=true` 时尝试清理
- 建议为 `probe.rw.test_database_prefix` 使用专门前缀，避免与真实业务库冲突
- `cleanup=false` 只建议用于调试；否则会保留测试资源
- 在集群外执行时，通常需要显式提供 `k8s.kubeconfig`
- Kubernetes 资源使用率依赖 `metrics-server` 与 RBAC；缺失时会降级为 `warn` / `unknown`，而不是让整个命令崩溃
- 样例配置故意展示失败路径；接入真实环境时，请替换 Milvus 地址、认证信息、namespace、kubeconfig，以及 probe 目标

## 开发与测试命令

```bash
make fmt
make test
make build
make run-help
go test ./...
go build ./...
```

## 文档入口

- [CHANGELOG.md](CHANGELOG.md): 即将发布版本与已知限制摘要
- [docs/project-status.md](docs/project-status.md): 当前 `main` 的能力边界与状态判断
- [docs/dev-workflow.md](docs/dev-workflow.md): 分支、协作与交付规则
- [docs/README.md](docs/README.md): `docs/` 目录入口
- [design_docs/milvus-health-spec-v1.3.md](design_docs/milvus-health-spec-v1.3.md): 规格说明
- [design_docs/milvus-health-interface-design-v0.7.md](design_docs/milvus-health-interface-design-v0.7.md): 接口设计说明
