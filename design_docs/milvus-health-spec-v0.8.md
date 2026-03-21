# milvus-health Specification v0.8

> **变更说明（v0.7 → v0.8）**
>
> 本版本基于红队评审结果修订，核心变更如下：
> 1. 新增 §3 版本兼容声明，明确支持 Milvus 2.4.x 与 2.6.x 两个架构档位
> 2. §11 关键 Pod 定义重写，改为版本感知的动态组件列表
> 3. §9 Business Read Probe 补充 search query vector 构造规则
> 4. §10 RW Probe 补充预存数据清理逻辑
> 5. §16 退出码语义澄清（exit 2 vs exit 4 的边界）
> 6. §14 新增 confidence 计算规则
> 7. §15 新增 standby 与最终结论的合法组合矩阵
> 8. §5 validate 命令说明补充"仅做静态校验"声明
> 9. §5 `--modules` 参数冲突行为明确
> 10. 配置样例 URI 格式修正（去掉错误的 `tcp://` 前缀）

---

## 1. 文档目标

本文档定义 `milvus-health` 的首版产品规格，作为后续开发、评审、测试与交付的统一依据。
`milvus-health` 的定位不是一次性巡检脚本，而是一个面向 Milvus 集群的可执行 CLI 工具，首版重点是给出一个明确判断：

- 这个 Milvus 集群是否能连通
- 当前集群的 database / collection 状态如何
- Milvus 相关 Pod 是否健康
- 真实业务数据是否可读
- 当前环境是否可完成一轮最小测试读写
- 当前集群是否可判定为健康、可服务、standby

本文档同时明确首版 CLI 的交互原则：

- **默认输出到 terminal / stdout**
- **日志、诊断信息、错误详情输出到 stderr**
- **若用户需要保存结果到文件，应由 shell 重定向或管道完成，而不是由工具默认落盘**

---

## 2. 产品名称与形态

### 2.1 产品名称
`milvus-health`

### 2.2 产品形态
单二进制命令行工具，采用 `command + subcommand` 风格。

示例：

```bash
milvus-health check --config ./config.yaml
milvus-health check --config ./config.yaml --format json
milvus-health validate --config ./config.yaml
milvus-health version
```

### 2.3 首版 CLI 使用原则
首版 `milvus-health` 必须遵循标准 CLI / Unix 风格：

- 结果正文输出到 `stdout`
- 调试日志、警告、错误详情输出到 `stderr`
- 最终状态通过固定退出码表达
- 工具本身不要求用户指定输出目录
- 工具本身不强制生成 `txt` / `json` / `md` 文件

典型使用方式：

```bash
milvus-health check --config ./config.yaml
milvus-health check --config ./config.yaml > report.txt
milvus-health check --config ./config.yaml --format json > report.json
milvus-health check --config ./config.yaml 2> debug.log
milvus-health check --config ./config.yaml | tee report.txt
```

---

## 3. 版本兼容声明

### 3.1 目标版本范围
首版必须同时兼容以下两个 Milvus 大版本：

| 版本档位 | 代表版本 | 架构特征 |
|---|---|---|
| `v2.4` | 2.4.x | 独立 coordinator（datacoord / querycoord / indexcoord） |
| `v2.6` | 2.6.x | 合并 coordinator（mixCoord） + Streaming Node，indexNode 移除 |

### 3.2 架构档位识别
工具在 `CollectClusterInfo` 阶段获取 Milvus 版本号后，必须立即完成架构档位（`ArchProfile`）推断，后续所有版本相关判定均基于此档位。

档位推断规则（以 semver 主次版本判断）：

| 版本范围 | 识别档位 |
|---|---|
| `2.4.x` | `v2.4` |
| `2.5.x` | `v2.4`（coordinator 结构与 2.4 一致，可能包含 mixCoord 过渡态，工具以实际存在的 Pod 为准） |
| `2.6.x` 及以上 | `v2.6` |
| 无法识别 | `unknown`（工具降级为仅连接性与元数据检查，Pod 检查标记为 skip） |

### 3.3 输出中的版本信息
`ClusterInfo` 及所有输出格式（text / json）必须包含：
- `milvus_version`：从集群获取的实际版本号字符串
- `arch_profile`：工具识别的架构档位（`v2.4` / `v2.6` / `unknown`）

---

## 4. 语言选型

### 4.1 首版主实现语言
Go

### 4.2 选型原则
首版以"低环境依赖、单二进制交付、适配客户现场、便于后续扩展"为优先目标，因此主实现语言选择 Go。

### 4.3 约束
- shell 仅允许作为少量辅助命令调用，不承载核心逻辑
- Python 不作为首版主实现语言

---

## 5. 首版范围

### 5.1 首版目标（P0）
首版必须实现对目标 Milvus 集群的基础健康判断，重点是"判断能力"，而不是"报告文件生成能力"。

### 首版必须回答的问题
1. 能否连接目标 Milvus
2. 当前有哪些 database 和 collection
3. collection 的基本状态如何
4. Milvus 相关 Pod 是否健康（版本感知）
5. restore 后的业务数据是否可读
6. 当前环境是否可完成一轮测试读写
7. 最终是否可判定为 PASS / WARN / FAIL
8. 最终是否可判定为 standby = true / false

---

### 5.2 P0 必做能力

#### A. 集群基础信息
- Milvus version
- 架构档位（`arch_profile`）
- 目标连接地址
- namespace
- MQ 类型（若可识别）
- 组件列表
- Pod 列表
- Service 列表

#### B. 元数据盘点
- database 数量
- database 名称列表
- collection 数量
- collection 名称列表
- 每个 collection 的：
  - database
  - row count（通过 `GetCollectionStatistics` 获取，首版固定此方式，不使用 `count(*)` query）
  - partition count
  - shard num（可获取时）
  - replica num（可获取时）
  - load state
  - index count
  - index type
  - vector field list（含字段名与维度）
  - indexed vector field count
  - unindexed vector field count

#### C. K8s 基础健康
- Pod phase
- Ready 状态
- Restart count
- CPU 使用率（可获取时）
- Memory 使用率（可获取时）
- request / limit（可获取时）

#### D. 业务探测
- Business Read Probe
- RW Probe

#### E. 分析结论
- PASS / WARN / FAIL
- standby true / false
- confidence（high / medium / low）
- 关键风险摘要
- 失败项与告警项

---

### 5.3 P1（后续增强）
- 更多 collection 级分析
- 更多性能项（如 p99 / search latency / mutation latency）
- PVC / Node 层补充
- 更丰富的风险建议
- 文件化报告导出（如 markdown / html）

### 5.4 P2（后续增强）
- etcd 深度健康
- MinIO / S3 对象级检查
- Pulsar backlog / Kafka lag / Woodpecker 深度检查
- Segment / Flowgraph / Index task 深度状态
- 离线 metadata 分析

---

## 6. CLI 设计

### 6.1 首版必须命令

#### 6.1.1 `check`
执行完整巡检流程，是首版主命令。

##### 作用
- 读取配置
- 执行元数据采集
- 执行业务读探测
- 执行测试读写探测
- 执行规则分析
- 将结果输出到 `stdout`
- 返回固定退出码

##### 示例
```bash
milvus-health check --config ./config.yaml
milvus-health check --config ./config.yaml --format text
milvus-health check --config ./config.yaml --format json
milvus-health check --config ./config.yaml --verbose
```

##### 参数表

| 参数 | 必填 | 说明 |
|---|:---:|---|
| `--config` | 是 | 配置文件路径 |
| `--format` | 否 | 输出格式，首版支持 `text`、`json`，默认 `text` |
| `--profile` | 否 | 巡检级别，首版默认只支持 `p0` |
| `--verbose` | 否 | 输出调试日志到 stderr |
| `--timeout` | 否 | 整体执行超时，单位秒；同时作为单次 Milvus/K8s API 调用的上限，防止 TCP 层卡死 |
| `--cleanup` | 否 | 覆盖配置中的 `probe.rw.cleanup` |
| `--modules` | 否 | 选择执行模块，默认 `milvus,k8s,probe`；`milvus` 为必选，不可关闭；若用户传入的列表中不含 `milvus`，工具自动补入并在 stderr 输出提示 |
| `--database` | 否 | 仅限定某个 database 做盘点/探测 |
| `--collection` | 否 | 仅限定某个 collection 做盘点/探测（单值） |
| `--detail` | 否 | 输出更详细的 collection / pod 明细，默认关闭 |

**`--modules` 行为约束：**
- `milvus` 模块不可关闭；若用户传入 `--modules k8s,probe`，工具自动补全为 `milvus,k8s,probe` 并在 stderr 输出警告
- 关闭 `k8s` 模块时，K8s 相关检查项全部输出 `skip`
- 关闭 `probe` 模块时，Business Read Probe 与 RW Probe 均输出 `skip`

---

#### 6.1.2 `validate`
校验配置文件合法性与执行前置条件。

##### 作用
- 检查 YAML 结构
- 检查必填项
- 检查互斥配置
- 检查连接参数是否完整

> **重要说明：** `validate` 仅做**静态校验**，不发起任何网络请求，不验证 Milvus 或 K8s 是否实际可连通。通过 `validate` 仅代表配置文件格式合法、必填项完整，**不代表运行时一定成功**。

##### 示例
```bash
milvus-health validate --config ./config.yaml
milvus-health validate --config ./config.yaml --verbose
```

##### 参数表

| 参数 | 必填 | 说明 |
|---|:---:|---|
| `--config` | 是 | 配置文件路径 |
| `--verbose` | 否 | 输出调试日志到 stderr |

---

#### 6.1.3 `version`
输出工具版本。

##### 示例
```bash
milvus-health version
```

---

### 6.2 首版不对外暴露的能力
以下能力首版内部需要按模块化方式实现，但不要求首版以独立命令对外暴露：

- `collect`
- `probe`
- `analyze`

说明：
- 首版重点是一个命令直接回答"这套 Milvus 健不健康"
- 若后续需要离线分析或导出文件，再独立暴露相应命令

---

## 7. 配置文件设计

配置文件格式：YAML

### 7.1 配置字段表

| 路径 | 必填 | 类型 | 说明 |
|---|:---:|---|---|
| `cluster.name` | 是 | string | 目标集群名称 |
| `cluster.milvus.uri` | 是 | string | Milvus 连接地址，格式为 `host:port`，例如 `milvus.milvus.svc.cluster.local:19530` |
| `cluster.milvus.username` | 否 | string | 用户名 |
| `cluster.milvus.password` | 否 | string | 密码 |
| `cluster.milvus.token` | 否 | string | token，若使用 token 认证 |
| `k8s.namespace` | 否 | string | K8s namespace |
| `k8s.kubeconfig` | 否 | string | kubeconfig 路径 |
| `dependencies.mq.type` | 否 | string | `pulsar` / `kafka` / `woodpecker` / `unknown` |
| `probe.read.targets` | 否 | list | 业务读探测目标列表 |
| `probe.read.min_success_targets` | 否 | int | 最少成功目标数，默认 1 |
| `probe.rw.enabled` | 否 | bool | 是否执行 RW Probe |
| `probe.rw.test_database_prefix` | 否 | string | 测试 database 前缀 |
| `probe.rw.cleanup` | 否 | bool | 是否清理测试数据 |
| `probe.rw.insert_rows` | 否 | int | 测试插入行数 |
| `probe.rw.vector_dim` | 否 | int | 测试向量维度 |
| `rules.pod_restart_warn` | 否 | int | Pod restart WARN 阈值 |
| `rules.pod_restart_fail` | 否 | int | Pod restart FAIL 阈值 |
| `rules.resource_warn_ratio` | 否 | float | Pod CPU/Memory 使用率 WARN 阈值 |
| `rules.require_probe_for_standby` | 否 | bool | 若为 true，则未执行 probe 时 standby 必须为 false |
| `output.format` | 否 | string | 默认输出格式，支持 `text`、`json`，CLI 参数可覆盖 |
| `output.detail` | 否 | bool | 是否默认输出详细明细，CLI 参数可覆盖 |

### 7.2 配置约束
- `cluster.milvus.uri` 格式必须为 `host:port`，**不得**携带 `tcp://`、`grpc://` 等 scheme 前缀；若检测到 scheme 前缀，`validate` 应报错并提示正确格式
- 首版配置文件中**不再要求**提供 `output.dir`
- 首版配置文件中**不再要求**声明输出文件名
- 配置只负责描述"检查目标与规则"，不负责规定"结果保存到哪个文件"
- `cluster.milvus.password` 与 `cluster.milvus.token` 不建议同时配置；若同时存在，**token 优先**，实现中必须固定此优先级并写入 README
- 若设置了 `--database` / `--collection` CLI 参数，CLI 参数优先级高于配置文件中的默认范围

### 7.3 `config.example.yaml` 样例

```yaml
cluster:
  name: milvus-prod-sh
  milvus:
    # 格式为 host:port，不带任何 scheme 前缀
    uri: milvus.milvus.svc.cluster.local:19530
    username: root
    password: ""
    token: ""

k8s:
  namespace: milvus
  kubeconfig: /home/admin/.kube/config

dependencies:
  mq:
    type: pulsar

probe:
  read:
    min_success_targets: 1
    targets:
      - database: default
        collection: book
        query_expr: "id >= 0"
        output_fields:
          - id
          - title
      - database: default
        collection: articles
        anns_field: vector
        topk: 3
        output_fields:
          - id
          - category

  rw:
    enabled: true
    test_database_prefix: milvus_health_test
    cleanup: true
    insert_rows: 100
    vector_dim: 128

rules:
  pod_restart_warn: 1
  pod_restart_fail: 5
  resource_warn_ratio: 0.85
  require_probe_for_standby: true

output:
  format: text
  detail: false
```

### 7.4 配置样例说明
- `cluster.milvus.uri` 为首版必填，格式为 `host:port`
- `probe.read.targets` 至少建议配置 1 个真实业务 collection，避免只依赖 RW Probe 做健康判定
- `probe.rw.enabled` 在生产环境建议默认开启，但若客户环境禁止测试写入，可显式关闭
- `output.format` 和 `output.detail` 只是默认值，CLI 参数可以覆盖

---

## 8. 数据模型

### 8.1 MetadataSnapshot
一次采集/探测的事实集合。

建议顶层结构：

```json
{
  "cluster_info": {},
  "milvus_inventory": {},
  "k8s_status": {},
  "business_read_probe": {},
  "rw_probe": {},
  "summary": {}
}
```

说明：
- `MetadataSnapshot` 是内部事实模型
- 首版实现可以在内存中构造，不要求默认落地成文件

---

### 8.2 CheckResult
统一的单检查项结果结构。

字段建议：

| 字段 | 类型 | 说明 |
|---|---|---|
| `category` | string | 检查类别 |
| `name` | string | 检查项名称 |
| `target` | string | 检查对象 |
| `status` | string | `pass` / `warn` / `fail` / `skip` |
| `severity` | string | `info` / `low` / `medium` / `high` |
| `message` | string | 结果描述 |
| `actual` | any | 实际值 |
| `expected` | any | 期望值 |
| `duration_ms` | int | 执行耗时 |

---

### 8.3 AnalysisResult
分析结论统一结构。

字段建议：

| 字段 | 类型 | 说明 |
|---|---|---|
| `result` | string | `PASS` / `WARN` / `FAIL` |
| `standby` | bool | 是否可判定为 standby |
| `confidence` | string | `high` / `medium` / `low`，计算规则见 §14 |
| `fail_count` | int | FAIL 项数量 |
| `warn_count` | int | WARN 项数量 |
| `checks` | list | CheckResult 列表 |
| `summary` | object | 摘要信息 |

---

## 9. 输出与展示规则

### 9.1 输出格式
首版固定支持：

- `text`：面向人阅读，默认格式
- `json`：面向机器处理

首版不要求支持：

- `md`
- `html`
- `csv`

以上可作为后续增强。

### 9.2 stdout / stderr 契约
`milvus-health` 首版必须严格区分输出流：

#### stdout
仅输出正式结果正文：
- `text` 格式结果
- 或 `json` 格式结果

#### stderr
输出：
- 调试日志
- warning 提示
- 错误详情
- 诊断性补充信息

约束：
- `--verbose` 只能增加 `stderr` 内容，不得污染 `stdout`
- 当 `--format json` 时，`stdout` 必须保证是可直接被 `jq` 解析的纯 JSON
- 不允许把日志与 JSON 正文混写到 `stdout`

### 9.3 text 格式最低要求
默认 text 输出必须至少包含：

1. 集群基本信息摘要（含 `arch_profile`）
2. 巡检执行信息摘要
3. 最终结论（PASS / WARN / FAIL）
4. standby 判定
5. confidence 级别
6. database / collection 摘要
7. Pod 健康摘要
8. Business Read Probe 结果
9. RW Probe 结果
10. 风险项清单
11. 失败项清单

### 9.4 detail 模式
默认 terminal 只输出摘要级信息。
若开启 `--detail`，可额外输出：

- collection 逐项明细
- Pod 逐项明细
- 更完整的 CheckResult 列表

### 9.5 文件保存方式
首版不强制生成任何输出文件。若调用方需要保存结果，应使用 shell 能力完成：

```bash
milvus-health check --config ./config.yaml > report.txt
milvus-health check --config ./config.yaml --format json > report.json
milvus-health check --config ./config.yaml --verbose 2> debug.log
milvus-health check --config ./config.yaml | tee report.txt
```

### 9.6 text 输出样例

```text
milvus-health check result
==========================
Cluster:        milvus-prod-sh
Milvus URI:     milvus.milvus.svc.cluster.local:19530
Namespace:      milvus
Milvus Version: 2.4.7
Arch Profile:   v2.4
MQ Type:        pulsar
Elapsed:        4.8s

Overall Result: WARN
Standby:        true
Confidence:     medium
Exit Code:      1

Inventory Summary
- Databases:   2
- Collections: 5
- Loaded:      4
- Unloaded:    1
- Unindexed vector fields: 1

K8s Summary
- Total Pods:      12
- Ready Pods:      11
- NotReady Pods:   1
- Restart WARN:    1 pod exceeded threshold
- Resource Usage:  unavailable

Business Read Probe
- Configured Targets: 2
- Successful Targets: 1
- Result: WARN
- Message: one target query failed, but minimum success target requirement was met

RW Probe
- Enabled: true
- Result: PASS
- Insert Rows: 100
- Vector Dim: 128

Warnings
1. collection default.articles has 1 unindexed vector field
2. pod milvus-querynode-0 restart count is 2
3. resource usage unavailable because metrics-server is missing

Failures
- none
```

### 9.7 json 输出样例

```json
{
  "cluster": {
    "name": "milvus-prod-sh",
    "milvus_uri": "milvus.milvus.svc.cluster.local:19530",
    "namespace": "milvus",
    "milvus_version": "2.4.7",
    "arch_profile": "v2.4",
    "mq_type": "pulsar"
  },
  "result": "WARN",
  "standby": true,
  "confidence": "medium",
  "exit_code": 1,
  "elapsed_ms": 4800,
  "summary": {
    "database_count": 2,
    "collection_count": 5,
    "loaded_collection_count": 4,
    "unloaded_collection_count": 1,
    "unindexed_vector_field_count": 1,
    "total_pod_count": 12,
    "ready_pod_count": 11,
    "not_ready_pod_count": 1,
    "fail_count": 0,
    "warn_count": 3
  },
  "probes": {
    "business_read": {
      "status": "warn",
      "configured_targets": 2,
      "successful_targets": 1,
      "min_success_targets": 1,
      "message": "one target query failed, but minimum success target requirement was met"
    },
    "rw": {
      "status": "pass",
      "enabled": true,
      "insert_rows": 100,
      "vector_dim": 128
    }
  },
  "warnings": [
    "collection default.articles has 1 unindexed vector field",
    "pod milvus-querynode-0 restart count is 2",
    "resource usage unavailable because metrics-server is missing"
  ],
  "failures": [],
  "checks": [
    {
      "category": "milvus",
      "name": "connectivity",
      "target": "milvus.milvus.svc.cluster.local:19530",
      "status": "pass",
      "severity": "info",
      "message": "milvus connection established",
      "actual": true,
      "expected": true,
      "duration_ms": 120
    },
    {
      "category": "probe",
      "name": "business_read_probe",
      "target": "default.articles",
      "status": "warn",
      "severity": "medium",
      "message": "query failed on one configured target",
      "actual": 1,
      "expected": 2,
      "duration_ms": 340
    }
  ]
}
```

### 9.8 输出样例约束
- text 样例和 json 样例必须在仓库中同步存在于 `examples/` 或 `docs/` 中
- 程序真实输出允许比样例更丰富，但不能比样例缺少核心摘要字段
- README 中的命令示例、spec 中的样例、实际程序输出三者必须保持一致性

---

## 10. Business Read Probe 规格

### 10.1 目标
验证 restore 后业务数据真实可读，而不仅是元数据存在。

### 10.2 输入
由配置文件 `probe.read.targets` 提供。

每个 target 至少包含：
- `database`
- `collection`

可选包含：
- `query_expr`：若提供则执行 query；未提供时使用默认表达式 `"id >= 0"`
- `anns_field`：若提供则额外执行 search；**不提供时跳过 search，仅执行 query**
- `topk`：search 时使用，默认 3
- `output_fields`

### 10.3 执行动作
对每个 target，执行顺序如下：

1. `DescribeCollection`：获取 schema（含各向量字段的维度信息）
2. 获取 row count（`GetCollectionStatistics`）
3. 检查 collection load state
4. 执行 query（使用 `query_expr`，若未配置则使用 `"id >= 0"`，`limit` 固定为 1）
5. 若配置了 `anns_field`，则额外执行 search：
   - **query vector 构造规则**：从步骤 1 的 schema 中读取 `anns_field` 对应字段的维度 `dim`，生成 `dim` 维的随机 float32 向量（每个元素取值 [-1.0, 1.0]）作为 query vector
   - 若 schema 中找不到 `anns_field` 对应字段或无法获取维度，则该 target 的 search 步骤标记为 fail，并在 message 中说明原因
6. 记录成功 / 失败 / 耗时 / 报错信息

### 10.4 成功标准
首版规则：

- 若未配置任何 `probe.read.targets`，则 Business Read Probe 状态为 `skip`
- 若配置了 targets：
  - 成功数 **>= `min_success_targets`**（注意含等于），则判定为 `pass`
  - 成功数 > 0 但 < `min_success_targets`，判定为 `warn`
  - 成功数 = 0，判定为 `fail`
- 默认 `min_success_targets = 1`

---

## 11. RW Probe 规格

### 11.1 目标
验证当前集群具备最小闭环写入、索引、加载、查询能力。

### 11.2 首版固定 schema

#### 固定字段
- `id`：Int64，主键，非 auto id
- `vector`：FloatVector
- `payload`：VarChar，最大长度 256

#### 默认向量维度
- `vector_dim = 128`

#### 默认插入行数
- `insert_rows = 100`

---

### 11.3 执行步骤
若 `probe.rw.enabled = true`，则必须按以下顺序执行：

**预检阶段（正式步骤前）：**
- 检查测试 database（由 `test_database_prefix` + 随机 suffix 拼成）是否已存在
- 若已存在（说明上次运行被强制中断、cleanup 未完成），**先执行清理**（drop collection → drop database），再继续后续步骤
- 若 cleanup 失败，RW Probe 记为 fail，并在 message 中说明"pre-existing test data cleanup failed"

**正式步骤：**
1. 创建测试 database
2. 创建测试 collection
3. 插入测试数据
4. flush
5. 为 `vector` 字段创建索引
6. load collection
7. 执行 search
8. 执行 query
9. 若 `cleanup = true`，则删除测试 collection / database

---

### 11.4 成功标准
所有步骤成功，则 RW Probe 为 `pass`。
任一关键步骤失败，则 RW Probe 为 `fail`。
若配置显式关闭，则为 `skip`。

---

## 12. 关键 Pod 定义（版本感知）

首版按架构档位分别定义关键组件。工具在识别 `arch_profile` 后，使用对应列表进行健康判断。

### 12.1 v2.4 档位关键组件

| 组件角色 | 关键等级 | Pod 识别方式 |
|---|---|---|
| proxy | 关键 coordinator | label `app.kubernetes.io/component=proxy` 或名称前缀 |
| rootcoord | 关键 coordinator | label 或名称前缀 `rootcoord` |
| datacoord | 关键 coordinator | label 或名称前缀 `datacoord` |
| querycoord | 关键 coordinator | label 或名称前缀 `querycoord` |
| indexcoord | 关键 coordinator | label 或名称前缀 `indexcoord` |
| querynode | 关键 data plane | label 或名称前缀 `querynode` |
| datanode | 关键 data plane | label 或名称前缀 `datanode` |
| indexnode | 关键 data plane | label 或名称前缀 `indexnode` |

### 12.2 v2.6 档位关键组件

| 组件角色 | 关键等级 | Pod 识别方式 |
|---|---|---|
| proxy | 关键 coordinator | label 或名称前缀 `proxy` |
| rootcoord | 关键 coordinator | label 或名称前缀 `rootcoord` |
| mixcoord | 关键 coordinator | label 或名称前缀 `mixcoord` |
| querynode | 关键 data plane | label 或名称前缀 `querynode` |
| datanode | 关键 data plane | label 或名称前缀 `datanode` |
| streaming node | 关键 data plane | label 或名称前缀 `streaming` |

**额外规则：** 在 v2.6 档位集群中，若发现独立的 `indexcoord` / `datacoord` / `querycoord` Pod 仍在运行，工具应输出 WARN，message 为 `legacy coordinator pods detected in v2.6 cluster, upgrade may be incomplete`。

### 12.3 unknown 档位处理
若 `arch_profile = unknown`，Pod 检查项整体标记为 `skip`，在输出中说明无法识别架构档位。

### 12.4 FAIL 规则（适用于所有档位）
满足任一情况则关键 Pod 检查为 FAIL：
- 任一关键 coordinator 类组件不存在
- 任一关键 coordinator 类组件 Pod 未 Ready
- 关键 data plane 组件**全部**不可用
- 关键 Pod phase 为 `CrashLoopBackOff` / `Failed`

### 12.5 WARN 规则
满足任一情况则关键 Pod 检查为 WARN：
- 存在关键 Pod restart 超过 `pod_restart_warn`
- 存在关键 Pod 资源使用率超过 `resource_warn_ratio`
- 某类横向扩展组件副本数低于预期但未全部失败

---

## 13. 未建索引判定规则

首版固定规则：

- 仅针对向量字段做索引检查
- 一个 collection 中，若某向量字段不存在任何向量索引，则该字段视为未建索引
- `unindexed_vector_field_count` = 未建索引的向量字段数
- 若 `unindexed_vector_field_count > 0`，则 collection 级检查记为 WARN，不直接 FAIL

说明：
- 首版不对标量字段索引做强制健康判定
- 首版不尝试判断"哪个字段是业务主检索字段"，避免推断歧义

---

## 14. Pod 资源采集失败的降级规则

由于客户现场可能缺失 metrics-server 或权限不足，首版规则固定如下：

- 若 Pod 列表与基础状态可获取，但 CPU / Memory 使用率无法获取，则资源检查项状态为 `warn`
- 工具须区分两类失败原因，并在 message 中明确标注：
  - `resource usage unavailable: metrics-server not found`
  - `resource usage unavailable: insufficient permissions`
- 不得因为资源使用率缺失直接导致整体 FAIL
- 若连 Pod 基础状态都无法获取，则 K8s 检查为 `fail`

---

## 15. 判定模型

### 15.1 最终结果
- `PASS`
- `WARN`
- `FAIL`

### 15.2 PASS 条件
同时满足：
- Milvus 可连接
- 元数据采集成功
- 关键 Pod 健康（或 K8s 模块被关闭）
- Business Read Probe 为 `pass` 或 `skip`
- RW Probe 为 `pass` 或 `skip`
- 无 FAIL 级检查项

### 15.3 WARN 条件
满足：
- 不属于 FAIL
- 但存在 WARN 级风险项

示例：
- 存在未建索引 collection
- 存在未 loaded collection（注：若运维已知该 collection 为归档状态，可忽略；但工具本身无法区分，统一输出 WARN）
- 资源使用率不可获取
- Pod restart 超过 warn 阈值
- 部分 read probe target 失败，但未低于最小成功数要求

### 15.4 FAIL 条件
满足任一：
- 无法连接 Milvus
- 元数据采集失败
- 关键 Pod 检查失败
- Business Read Probe 失败（成功数为 0）
- RW Probe 失败
- 核心流程运行时中断

---

## 16. confidence 计算规则

`confidence` 表达工具对最终判定结论的置信度，规则如下：

| 条件 | confidence |
|---|---|
| 所有模块均执行（未 skip），且无 FAIL、无 WARN | `high` |
| 所有模块均执行，存在 WARN 但无 FAIL | `medium` |
| 存在任一模块被 skip（如 probe skip、k8s skip、arch_profile=unknown） | `low` |
| 存在 FAIL | `low` |

优先级：条件从上到下匹配，取第一个满足的。

---

## 17. standby 判定

首版 standby 仅表示"当前集群具备基本服务能力"，不等价于"完全无风险"。

### 17.1 standby 与最终结论的合法组合矩阵

| result | standby | 是否合法 | 说明 |
|---|:---:|:---:|---|
| PASS | true | ✅ | 正常健康 |
| PASS | false | ✅ | 关键 Pod 健康但 require_probe_for_standby=true 且 probe 全 skip |
| WARN | true | ✅ | 有风险但仍可服务 |
| WARN | false | ✅ | 有风险且不满足 standby 条件 |
| FAIL | true | ❌ | 非法，FAIL 时 standby 必须为 false |
| FAIL | false | ✅ | 正常失败状态 |

### 17.2 standby = true 条件
同时满足：
- 最终结果不为 FAIL
- 关键 Pod 检查通过（或 K8s 模块 skip）
- 若启用了 Business Read Probe，则其结果不能为 FAIL
- 若启用了 RW Probe，则其结果不能为 FAIL
- 若 `rules.require_probe_for_standby = true`，则必须至少成功执行一种 probe

### 17.3 standby = false 条件
满足任一：
- 最终结果为 FAIL
- 关键 Pod 检查失败
- 已启用的 probe 中存在 FAIL
- 配置要求 probe 参与 standby 判定，但本次 probe 未执行成功

### 17.4 跳过 probe 时的语义
- 若 probe 被显式关闭，则允许最终结果为 PASS/WARN
- 但 standby 的计算必须遵循 `rules.require_probe_for_standby`
- 若 `require_probe_for_standby = true` 且 probe 均为 skip，则 standby 必须为 false
- 结果中必须明确标注：`standby confidence downgraded because probes were skipped`

---

## 18. 固定退出码

| 退出码 | 含义 | 触发场景 |
|:---:|---|---|
| `0` | PASS | 巡检通过，无 FAIL 无 WARN |
| `1` | WARN | 巡检完成，存在 WARN 级风险，无 FAIL |
| `2` | FAIL | 巡检完成，集群被判定为不健康（含 Milvus 不可连接、关键 Pod 异常、probe 失败等） |
| `3` | CONFIG_ERROR | 配置文件无法加载或校验失败，工具未能启动巡检 |
| `4` | RUNTIME_ERROR | 工具自身意外中断（panic、依赖缺失、不可预期的系统错误），与集群健康状态无关 |

**exit 2 与 exit 4 的边界：**
- Milvus 无法连接 → **exit 2**（这是一个有效的巡检结论：集群不健康）
- K8s API 无法访问 → **exit 2**（K8s 检查 FAIL）
- 工具自身 panic / 未捕获异常 → **exit 4**
- `--timeout` 超时导致整体终止 → **exit 4**

约束：
- 成功必须使用固定退出码
- 不允许随意扩展首版退出码含义
- `validate` 成功返回 `0`，配置非法返回 `3`

### 18.1 shell 语义说明
- 在 shell / CI 中，任何非 `0` 退出码通常都会被视为失败
- 首版设计中，`WARN = 1` 是有意行为，表示"巡检未完全通过，调用方必须显式处理"
- 调用方若需要区分 WARN 与 FAIL，必须自行判断退出码 `1` 与 `2`

---

## 19. 首版工程约束

- 必须模块化实现，不允许所有逻辑堆到单文件 main 中
- 采集与分析必须分层
- probe 必须独立模块化
- 配置解析必须与运行逻辑分离
- 所有错误必须结构化返回
- 所有检查项必须统一产出 CheckResult
- 默认交互必须是 terminal / stdout 输出，而不是写入指定 output 目录
- `json` 输出必须满足可被标准 JSON 工具直接消费的要求
- `examples/config.example.yaml`、spec 中样例、README 示例必须同步维护

---

## 20. K8s 权限要求

工具运行时所需的最小 K8s RBAC 权限如下，建议在 README 和部署文档中提供此 ClusterRole 样例：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: milvus-health-reader
rules:
  - apiGroups: [""]
    resources: ["pods", "services"]
    verbs: ["list", "get"]
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods"]
    verbs: ["list", "get"]
```

说明：
- `metrics.k8s.io` 权限仅在集群安装了 metrics-server 时生效
- 若无此权限，工具按 §14 降级规则处理，输出 WARN 而非 FAIL

---

## 21. 首版验收标准

### 21.1 基本能力
- 能正常执行 `version`
- 能正常执行 `validate`
- 能正常执行 `check`

### 21.2 正常场景
在健康 Milvus 集群上：
- text 输出可在 terminal 中直接阅读
- `--format json` 输出可被 `jq` 正常解析
- 返回码为 `0` 或 `1`
- 能正确显示 database / collection 数量
- 能正确显示 Pod 状态
- 能正确识别 `arch_profile`（在 2.4.x 集群上显示 `v2.4`，在 2.6.x 集群上显示 `v2.6`）
- probe 结果与现场真实情况一致

### 21.3 异常场景
- 配置缺失时返回 `3`
- URI 格式错误（包含 scheme 前缀）时 `validate` 返回 `3` 并给出明确提示
- Milvus 不可连接时返回 `2`
- Business Read Probe 全失败时返回 `2`
- RW Probe 失败时返回 `2`
- 工具自身 panic 时返回 `4`
- `stderr` 中可看到足够定位问题的错误信息

### 21.4 输出质量
- 默认 text 输出必须可读
- text 输出必须清晰列出 FAIL / WARN 原因
- `json` 输出不得混入日志
- 默认摘要不能被 collection 明细淹没
- 开启 `--detail` 后能看到更完整明细
- 输出风格应与本 spec 中 text/json 样例保持一致

### 21.5 版本兼容性
- 在 2.4.x 集群上，能正确识别并检查独立 coordinator 组件
- 在 2.6.x 集群上，能正确识别 mixCoord 和 Streaming Node，不对旧组件缺失触发 FAIL

### 21.6 终端与管道兼容性
- 支持 `milvus-health check --config ./config.yaml > report.txt`
- 支持 `milvus-health check --config ./config.yaml --format json | jq .`
- 支持 `milvus-health check --config ./config.yaml --verbose 2> debug.log`
- 支持 `milvus-health check --config ./config.yaml | tee report.txt`

---

## 22. 建议目录结构（实现参考）

```text
milvus-health/
├── cmd/
├── internal/
│   ├── cli/
│   ├── config/
│   ├── collectors/
│   ├── probes/
│   ├── analyzers/
│   ├── render/
│   └── model/
├── examples/
│   ├── config.example.yaml
│   ├── output.text.example.txt
│   └── output.json.example.json
├── docs/
│   └── milvus-health-spec-v0.8.md
└── main.go
```

---

## 23. 后续版本方向

后续可以继续演进：
- `probe` 命令独立暴露
- `analyze` 命令独立暴露
- 支持离线读取 metadata 再生成报告
- 支持 etcd / MinIO / MQ 深度检查
- 支持 markdown / HTML 报告导出
- 支持 Prometheus 指标接入
- 支持更丰富的 profile（p1 / p2）
- Milvus 2.6.x Woodpecker 深度检查

---

## 24. 当前版本结论

截至 v0.8，本文档在 v0.7 基础上完成了红队修订，核心补齐内容：

1. **版本兼容**：明确 2.4.x 与 2.6.x 架构差异，关键组件判定改为版本感知
2. **配置安全**：URI 格式约束明确，validate 静态校验语义明确
3. **Probe 规格**：search query vector 构造规则明确，RW Probe 残留数据处理规则补齐
4. **判定规则**：confidence 计算规则、standby 合法组合矩阵、exit code 边界均已补齐
5. **工程约束**：K8s RBAC 要求、--modules 冲突行为、RowCount 获取方式已明确
