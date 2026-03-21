# milvus-health

`milvus-health` 是一个面向 Milvus 集群的 Go CLI 巡检工具。项目目标是用标准 Unix CLI 风格输出明确的健康判断结果：

- 正式结果输出到 `stdout`
- 日志、warning、错误详情输出到 `stderr`
- 最终状态通过固定退出码表达

当前仓库仍处于第一阶段：`工程初始化 + 项目骨架 + TDD 基础设施 + 最小可运行闭环`。这意味着它已经可以编译、测试、展示 CLI 结构，但主分支仍应被视为 skeleton，而不是已经具备真实巡检能力的交付版本。

## 文档入口

- [docs/project-status.md](docs/project-status.md): 当前主分支状态、阶段判断、已实现/未实现边界。
- [docs/dev-workflow.md](docs/dev-workflow.md): 多角色协作职责、Definition of Done、开发规则与推荐工作流。
- [docs/README.md](docs/README.md): `docs/` 目录入口说明。

## 给外部模型的快速上下文

如果你是外部分析模型，请先基于以下事实理解本仓库：

- 这是一个 Go CLI 项目，不是脚本集合。
- 当前实现重点是分层、契约、可测试性，不是业务完整度。
- 当前 `check` 命令返回的是 stub analysis，不会连接真实 Milvus 或 K8s。
- 当前 `validate` 命令只做最小 YAML 加载和静态校验，不发起任何网络请求。
- `collectors`、`probes`、`platform` 目前只有接口或占位实现。
- `render` 已有最小 `text/json` 输出，`json` 输出保证为纯 JSON。
- 项目要求严格按 spec 和 interface design doc 演进，避免在骨架阶段提前塞入伪业务逻辑。

## 当前明确支持的版本目标

本仓库当前版本识别逻辑按本轮要求收敛为：

- Milvus `2.4.7` -> `v24`
- Milvus `2.6.x` -> `v26`
- 其他版本 -> `unknown`

这部分目前只体现在模型层版本识别与测试契约中，还没有下沉到真实 collector 或 probe 逻辑。

## 当前已完成内容

- Go module 初始化
- `Makefile` 门禁
- CLI 骨架：`check` / `validate` / `version`
- `main.go` 入口
- `internal/model` 最小领域模型
- `internal/config` 最小 loader / validator / default applier / override applier
- `internal/render` 最小 text/json renderer
- `internal/cli` 最小 runner 与 exit code mapper
- `examples/config.example.yaml`
- `examples/output.text.example.txt`
- `examples/output.json.example.json`
- 基于 TDD 的首批单元测试与 smoke 测试

## 当前未实现内容

以下能力当前明确未做，分析时不要误判为“应该已经存在但漏了”：

- 真实 Milvus SDK 接入
- 真实 K8s client 接入
- 真实 inventory 采集
- 真实 Business Read Probe
- 真实 RW Probe
- 真实 analyzer 规则体系
- 真实 collection / pod detail 明细
- 复杂并发执行

## 目录说明

```text
milvus-health/
├── cmd/                CLI 命令注册
├── internal/
│   ├── cli/            runner、退出码映射
│   ├── config/         配置加载、默认值、校验、CLI 覆盖
│   ├── collectors/     采集接口占位
│   ├── probes/         probe 接口占位
│   ├── analyzers/      analyzer fake/stub
│   ├── render/         text/json 渲染
│   ├── model/          领域模型、枚举、错误模型
│   └── platform/       外部客户端接口占位
├── examples/           样例配置与样例输出
├── docs/               项目内说明入口
├── test/               CLI smoke 测试
├── design_docs/        原始 spec 与 interface design doc
├── main.go
├── Makefile
└── go.mod
```

## 当前可用命令

- `milvus-health version`
- `milvus-health check --help`
- `milvus-health validate --help`

当前 `check` 命令需要配置文件路径，但执行结果仍是 stub 输出；其意义在于验证 CLI 编排、配置注入、渲染和退出码链路。

## 本地使用

```bash
make fmt
make test
make build
make run-help
```

构建产物输出到 `./bin/milvus-health`。

## 配置说明

配置样例见 [examples/config.example.yaml](examples/config.example.yaml)。

当前已实现的最小配置约束：

- `cluster.name` 必填
- `cluster.milvus.uri` 必填，格式必须是 `host:port`
- `cluster.milvus.uri` 不能带 `tcp://` 等 scheme
- `output.format` 只接受 `text` 或 `json`
- `probe.read.min_success_targets >= 0`

认证字段已保留：

- `cluster.milvus.username`
- `cluster.milvus.password`
- `cluster.milvus.token`

当前尚未接入真实客户端，但后续实现会遵循 `token` 优先于 `password` 的约束。

## 测试覆盖重点

当前测试重点是“契约”，不是业务正确性：

- 模型枚举与版本识别
- YAML 加载与配置静态校验
- 默认值与 CLI override
- text/json 渲染契约
- 退出码映射
- runner 最小闭环
- CLI smoke：`version` / `check --help` / `validate --help`

## 建议的外部分析方向

如果你要基于这个仓库做下一步分析，优先看这些问题：

1. 当前模型层是否已经覆盖后续 collector/analyzer 所需的最小字段集。
2. runner 编排接口是否足以支撑后续 fake -> real implementation 的平滑替换。
3. config / render / exit code 的契约是否已经稳定，适合继续扩展。
4. 哪些 stub 应该下一轮先替换成 fake，再替换成真实实现。
5. 版本差异 `2.4.7` 与 `2.6.x` 应如何下沉到 collector 和 analyzer 的职责边界。
