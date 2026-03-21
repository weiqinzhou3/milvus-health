# milvus-health

`milvus-health` 是一个面向 Milvus 集群的 Go CLI 巡检工具。当前阶段仅为 skeleton：已完成门禁、目录骨架、TDD 基础设施、配置静态校验最小实现，以及 `check`/`validate`/`version` 的可运行骨架。

当前可用命令：

- `milvus-health version`
- `milvus-health check --help`
- `milvus-health validate --help`

本轮不包含真实 Milvus/K8s 巡检逻辑，只保留接口、stub runner 与最小 renderer。

本地使用：

- `make fmt`
- `make test`
- `make build`
- `make run-help`

配置样例见 `examples/config.example.yaml`。
认证优先级按 spec 处理为 `token` 优先于 `password`；本轮仅保留配置字段与校验骨架，未接真实客户端。
