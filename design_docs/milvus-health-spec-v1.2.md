# milvus-health Specification v1.2

> **变更说明（v1.1 → v1.2）**
>
> 本版本基于红队评审结果修订：
>
> 1. §12.5 FAIL 规则补充 `pod_restart_fail` 触发条件——原配置字段有定义但规则表中缺少对应 FAIL 项
> 2. §22 目录结构文件名修正为 v1.2
> 3. §24 当前版本结论更新

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

```bash
milvus-health check --config ./config.yaml
milvus-health check --config ./config.yaml --format json
milvus-health validate --config ./config.yaml
milvus-health version
```

### 2.3 首版 CLI 使用原则
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
工具在 `CollectClusterInfo` 阶段获取 Milvus 版本号后，必须立即完成架构档位（`ArchProfile`）推断。

档位推断规则（以 semver 主次版本判断）：

| 版本范围 | 识别档位 |
|---|---|
| `2.4.x` / `2.5.x` | `v2.4` |
| `2.6.x` 及以上 | `v2.6` |
| 无法识别 | `unknown` |

**`arch_profile=unknown` 降级语义（精确定义）：**

- Milvus 连接性检查与元数据盘点照常执行
- K8s collector 仍可执行基础 Pod/Service 枚举，并将原始事实填入 `K8sStatus`（Phase、Ready、RestartCount 等），供 detail 输出和人工排查使用
- **K8s collector 不得**因架构未知而自行产出任何 FAIL/WARN 类 CheckResult——collector 在 `arch_profile=unknown` 时产出**空 CheckResult 列表**
- **K8sAnalyzer** 统一检测到 `arch_profile=unknown` 后，产出一条 `category=k8s, status=skip, message="arch_profile unknown, pod health check skipped"` 的 CheckResult，代表本轮 K8s 健康判定整体跳过
- 此 skip 不触发 FAIL，但会将 confidence 降级为 low（见 §16）

**ArchProfile 枚举值定义（全局统一）：**

| 档位 | JSON/text 输出值 |
|---|---|
| v2.4/v2.5 | `"v2.4"` |
| v2.6+ | `"v2.6"` |
| 未知 | `"unknown"` |

### 3.3 输出中的版本信息
`ClusterInfo` 及所有输出格式（text / json）必须包含：
- `milvus_version`：从集群获取的实际版本号字符串
- `arch_profile`：工具识别的架构档位（`"v2.4"` / `"v2.6"` / `"unknown"`）

---

## 4. 语言选型

**主实现语言：Go**

选型原则：低环境依赖、单二进制交付、适配客户现场、便于后续扩展。

约束：shell 仅作少量辅助命令调用，不承载核心逻辑；Python 不作为首版主实现语言。

---

## 5. 首版范围

### 5.1 首版目标（P0）
首版必须实现对目标 Milvus 集群的基础健康判断，重点是"判断能力"，而不是"报告文件生成能力"。

首版必须回答的问题：
1. 能否连接目标 Milvus
2. 当前有哪些 database 和 collection
3. collection 的基本状态如何
4. Milvus 相关 Pod 是否健康（版本感知）
5. restore 后的业务数据是否可读
6. 当前环境是否可完成一轮测试读写
7. 最终是否可判定为 PASS / WARN / FAIL
8. 最终是否可判定为 standby = true / false

### 5.2 P0 必做能力

#### A. 集群基础信息
- Milvus version、架构档位（`arch_profile`）
- 目标连接地址、namespace
- MQ 类型（若可识别）
- 组件列表、Pod 列表、Service 列表

#### B. 元数据盘点
- database 数量、名称列表
- collection 数量、名称列表
- 集群 total row count（各 collection 行数求和）
- 集群 total binlog size bytes（见 §5.5）
- 每个 collection：
  - database、row count、binlog size bytes（见 §5.5）
  - partition count、shard num（可获取时）、replica num（可获取时）
  - load state
  - index count、index type
  - vector field list（含字段名与维度）
  - indexed / unindexed vector field count

#### C. K8s 基础健康
- Pod phase、Ready 状态、Restart count
- 容器 waiting reason（最小要求：能识别 `CrashLoopBackOff`）
- CPU 使用率、Memory 使用率（可获取时）
- CPU request / limit、Memory request / limit（可获取时）
- **CPU 使用率 / limit 占比**（`cpu_limit_ratio`，用于 WARN 判断，见 §12.6）
- **Memory 使用率 / limit 占比**（`mem_limit_ratio`，用于 WARN 判断）
- CPU 使用率 / request 占比、Memory 使用率 / request 占比（仅展示，不触发 WARN）
- metrics 不可用原因（结构化，见 §14）

#### D. 业务探测
- Business Read Probe
- RW Probe

#### E. 分析结论
- PASS / WARN / FAIL
- standby true / false
- confidence（high / medium / low）
- 关键风险摘要、失败项与告警项

---

### 5.3 P1（后续增强）
- 更多 collection 级分析
- 性能项（p99 / search latency / mutation latency）
- PVC / Node 层补充
- 文件化报告导出（markdown / html）
- 通过 `milvus_datacoord_stored_index_files_size` Prometheus 指标补充 index 文件大小

### 5.4 P2（后续增强）
- etcd 深度健康
- MinIO / S3 对象级检查
- Pulsar backlog / Kafka lag / Woodpecker 深度检查
- Segment / Flowgraph / Index task 深度状态
- 离线 metadata 分析

---

### 5.5 数据量口径约束（重要）

#### 5.5.1 字段命名与语义

首版将数据量字段命名为 `binlog_size_bytes`，而**不是** `data_size_bytes`。

| 字段名 | 含义 | 首版支持 |
|---|---|---|
| `binlog_size_bytes` | Milvus binlog 占用字节数（flushed segments） | ✅ P0 |
| `index_size_bytes` | index 文件占用字节数 | P1（需 Prometheus） |
| `total_storage_bytes` | binlog + index 合计 | P1 |

#### 5.5.2 数据来源

首版通过 Milvus gRPC `GetMetrics("system_info")` 调用获取 `DataCoordQuotaMetrics`，从中提取：
- `TotalBinlogSize int64`：集群全量 binlog 总大小
- `CollectionBinlogSize map[int64]int64`：per-collection binlog 大小，key 为 collection ID

collection ID → collection name 的映射使用 inventory 阶段已采集的信息完成，无需额外调用。

#### 5.5.3 已知限制与 unknown 语义（重要）

- 仅统计已 flush 的 segment，growing（未 flush）segment 的数据不计入
- 不含 index 文件大小，实际存储占用通常显著高于此值
- `GetMetrics` 调用失败时，该字段语义为 `unknown`

**`unknown` 的统一输出规则（全局适用）：**
- **text 输出**：显示字符串 `unknown`
- **JSON 输出**：字段出现但值为 `null`（即 `"total_binlog_size_bytes": null`）
- **禁止**：JSON 中省略该字段（即不允许使用 `omitempty` 标签于语义为 unknown-when-nil 的字段），否则机器消费方无法区分"数据为 0"与"数据不可用"
- 类似规则适用于所有 `*int64` / `*float64` 语义为"可能不可用"的字段（包括 `BinlogSizeBytes`、各 ratio 字段等）

#### 5.5.4 估算禁止条款
首版**禁止**使用 `row_count × vector_dim × element_size` 之类的推导方式估算数据量。

---

## 6. CLI 设计

### 6.1 必须命令

#### `check`
执行完整巡检流程。

```bash
milvus-health check --config ./config.yaml
milvus-health check --config ./config.yaml --format json
milvus-health check --config ./config.yaml --verbose
```

| 参数 | 必填 | 说明 |
|---|:---:|---|
| `--config` | 是 | 配置文件路径 |
| `--format` | 否 | `text`（默认）或 `json` |
| `--profile` | 否 | 巡检级别，首版只支持 `p0` |
| `--verbose` | 否 | 输出调试日志到 stderr |
| `--timeout` | 否 | 整体超时（秒），同时限制单次 API 调用，防止 TCP 层卡死 |
| `--cleanup` | 否 | 覆盖配置中的 `probe.rw.cleanup` |
| `--modules` | 否 | 可选值：`k8s`、`probe`；`milvus` 为必选，不出现在此列表也默认启用；若传入未知模块名，返回 CONFIG_ERROR（exit 3） |
| `--database` | 否 | 限定某 database 做盘点/探测（单值） |
| `--collection` | 否 | 限定某 collection 做盘点/探测（单值） |
| `--detail` | 否 | 输出 collection / Pod 明细，默认关闭 |

**`--modules` 行为约束：**
- `milvus` 始终启用，用户无法关闭
- 若用户传 `--modules k8s,probe`，实际执行等同于 `milvus,k8s,probe`，stderr 输出提示
- 若用户传 `--modules milvus,unknown_mod`，返回 exit 3，message 说明未知模块名
- 关闭 `k8s` 时，K8s 相关检查项全部输出 `skip`
- 关闭 `probe` 时，Business Read Probe 与 RW Probe 均输出 `skip`
- **skip 产出职责（固定）：** `--modules` 导致模块关闭时，由 `CheckRunner` 在编排层统一补齐对应 `CheckResult{status=skip}`；collector / probe / analyzer 不得以"未执行"为由静默省略检查项。这一保证在 Milvus 连接失败（`goto analyze`）的异常路径上同样必须兑现。

**校验职责分工：**
- `--modules` 等 CLI 运行参数的合法性校验由 `OptionsValidator` 负责，在配置文件静态校验（`Validator`）**之前**执行，与配置文件内容无关
- 配置文件的字段合法性校验由 `Validator` 负责（见 §8 接口约束）

#### `validate`
仅做**静态校验**，不发起任何网络请求。通过 validate 仅代表配置文件格式合法、必填项完整，不代表运行时一定成功。

```bash
milvus-health validate --config ./config.yaml
```

| 参数 | 必填 | 说明 |
|---|:---:|---|
| `--config` | 是 | 配置文件路径 |
| `--verbose` | 否 | 输出调试日志到 stderr |

#### `version`
```bash
milvus-health version
```

### 6.2 首版不对外暴露的能力
`collect`、`probe`、`analyze` 首版内部模块化实现，不作为独立命令对外暴露。

---

## 7. 配置文件设计

格式：YAML

### 7.1 配置字段表

| 路径 | 必填 | 类型 | 说明 |
|---|:---:|---|---|
| `cluster.name` | 是 | string | 目标集群名称 |
| `cluster.milvus.uri` | 是 | string | 格式为 `host:port`，例如 `milvus.milvus.svc.cluster.local:19530` |
| `cluster.milvus.username` | 否 | string | 用户名 |
| `cluster.milvus.password` | 否 | string | 密码 |
| `cluster.milvus.token` | 否 | string | token（与 password 同时存在时 token 优先） |
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
| `rules.pod_restart_warn` | 否 | int | Pod restart count 达到此值时触发 WARN |
| `rules.pod_restart_fail` | 否 | int | Pod restart count 达到此值时触发 FAIL（须大于 pod_restart_warn） |
| `rules.resource_warn_ratio` | 否 | float | Pod **usage/limit** 占比 WARN 阈值（如 0.85 表示超过 85% limit 时告警） |
| `rules.require_probe_for_standby` | 否 | bool | true 时未执行 probe 则 standby 强制为 false |
| `output.format` | 否 | string | 默认输出格式（`text` / `json`），CLI 参数可覆盖 |
| `output.detail` | 否 | bool | 是否默认输出明细，CLI 参数可覆盖 |

### 7.2 配置约束
- `cluster.milvus.uri` 格式必须为 `host:port`，不得携带 `tcp://`、`grpc://` 等 scheme 前缀；`validate` 检测到 scheme 前缀时返回 CONFIG_ERROR 并提示正确格式
- `password` 与 `token` 同时存在时，token 优先，实现中必须固定此优先级并写入 README
- `rules.resource_warn_ratio` 对应 **usage/limit** 占比，不是 usage/request 占比
- `rules.pod_restart_fail` 须严格大于 `rules.pod_restart_warn`；两者都配置时 `Validator` 校验此约束

### 7.3 `config.example.yaml`

```yaml
cluster:
  name: milvus-prod-sh
  milvus:
    # 格式为 host:port，不带 scheme 前缀
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
  pod_restart_warn: 3
  pod_restart_fail: 10
  # resource_warn_ratio 对应 usage/limit 占比，0.85 = 超过 85% limit 时 WARN
  resource_warn_ratio: 0.85
  require_probe_for_standby: true

output:
  format: text
  detail: false
```

---

## 8. 数据模型

### 8.1 MetadataSnapshot

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

`MetadataSnapshot` 是内部事实模型，首版实现可在内存中构造，不要求默认落地成文件。

### 8.2 CheckResult

| 字段 | 类型 | 说明 |
|---|---|---|
| `category` | string | 检查类别（`milvus` / `k8s` / `probe`） |
| `name` | string | 检查项名称 |
| `target` | string | 检查对象（可选） |
| `status` | string | `pass` / `warn` / `fail` / `skip` |
| `severity` | string | `info` / `low` / `medium` / `high` |
| `message` | string | 结果描述 |
| `actual` | any | 实际值（可选） |
| `expected` | any | 期望值（可选） |
| `duration_ms` | int | 执行耗时（可选） |

### 8.3 AnalysisResult

| 字段 | 类型 | 说明 |
|---|---|---|
| `result` | string | `PASS` / `WARN` / `FAIL` |
| `standby` | bool | 是否可判定为 standby |
| `confidence` | string | `high` / `medium` / `low` |
| `fail_count` | int | FAIL 项数量 |
| `warn_count` | int | WARN 项数量 |
| `checks` | list | 完整 CheckResult 列表，**分析层始终保留**；渲染层根据 `Detail` 决定展示粒度，不得修改此列表 |
| `summary` | object | 摘要信息 |

---

## 9. 输出与展示规则

### 9.1 输出格式
首版支持 `text`（面向人阅读，默认）和 `json`（面向机器处理）。

### 9.2 stdout / stderr 契约
- stdout：仅输出正式结果正文（text 或 json）
- stderr：调试日志、warning 提示、错误详情
- `--verbose` 只能增加 stderr 内容，不得污染 stdout
- `--format json` 时，stdout 必须是可被 `jq` 直接解析的纯 JSON

### 9.3 text 格式最低要求
默认 text 输出必须包含：
1. 集群基本信息摘要（含 `arch_profile`）
2. 巡检执行信息摘要
3. 最终结论（PASS / WARN / FAIL）
4. standby 判定与 confidence
5. database / collection 摘要（含 total rows 与 total binlog size）
6. Pod 健康摘要（含 metrics 可用状态；`arch_profile=unknown` 时显示"pod health check skipped: arch_profile unknown"）
7. Business Read Probe 结果
8. RW Probe 结果
9. 风险项清单、失败项清单

### 9.4 detail 模式
开启 `--detail` 后额外输出：
- collection 逐项明细（含行数、binlog size、load state、索引状态）
- Pod 逐项明细（含 usage、request/limit、各比例；Ignored Pod 标注原因）
- 完整 CheckResult 列表

**`checks` 字段的渲染策略：**
- `Detail=false`：JSON renderer 通过 shadow struct 序列化，输出空数组 `[]`；text renderer 不输出 CheckResult 明细
- `Detail=true`：完整输出
- 渲染层不得修改 `AnalysisResult.Checks` 原始内容；必须通过 shadow struct 实现省略，而非将原始列表置 nil

### 9.5 文件保存方式
首版不强制生成任何输出文件。若调用方需要保存结果，应使用 shell 能力完成。

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
- Databases:          2
- Collections:        5
- Loaded:             4
- Unloaded:           1
- Total Rows:         12,560,331
- Total Binlog Size:  4.2 GiB
- Unindexed vector fields: 1

Collection Detail (detail=true)
- default.articles: rows=8,200,120, binlog_size=2.8GiB, load_state=Loaded, indexes=1
- default.images:   rows=4,360,211, binlog_size=1.4GiB, load_state=Loaded, indexes=1

K8s Summary
- Total Pods:      12
- Ready Pods:      11
- NotReady Pods:   1
- Restart WARN:    1 pod exceeded threshold
- Resource Usage:  partial (9/12 pods have metrics)

Pod Detail (detail=true)
- milvus-querynode-0: cpu=125m/1000m(12.5%req,12.5%lim), mem=512Mi/2Gi(25.0%req,25.0%lim)
- milvus-datanode-0:  cpu=98m/500m(19.6%req,19.6%lim),  mem=420Mi/1Gi(41.0%req,41.0%lim)
- milvus-debug:       [ignored: phase-succeeded]

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
3. resource usage unavailable for 3 pods: metrics-server not found

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
    "total_row_count": 12560331,
    "total_binlog_size_bytes": 4509715660,
    "unindexed_vector_field_count": 1,
    "total_pod_count": 12,
    "ready_pod_count": 11,
    "not_ready_pod_count": 1,
    "metrics_available_pod_count": 9,
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
    "resource usage unavailable for 3 pods: metrics-server not found"
  ],
  "failures": [],
  "checks": []
}
```

**样例说明：**
- `total_binlog_size_bytes` 在 GetMetrics 失败时输出 `null`（不省略字段）
- `checks` 在 `Detail=false` 时输出空数组 `[]`；`Detail=true` 时输出完整列表

### 9.8 输出样例约束
- text / json 样例必须在仓库 `examples/` 中同步存在
- 程序真实输出允许比样例更丰富，但不能缺少核心摘要字段
- README、spec 样例、实际程序输出三者必须保持一致

---

## 10. Business Read Probe 规格

### 10.1 目标
验证 restore 后业务数据真实可读。

### 10.2 输入

每个 target 至少包含 `database`、`collection`，可选：
- `query_expr`：未提供时使用 `"id >= 0"`
- `anns_field`：提供时额外执行 search；**不提供时跳过 search，仅 query**
- `topk`：search 时使用，默认 3
- `output_fields`

### 10.3 执行动作与 Action 字段状态机

对每个 target，执行顺序如下：

| 步骤 | 操作 | 中断处理 | 此时 Action 值 |
|---|---|---|---|
| 1 | `DescribeCollection`（获取 schema，含向量字段维度） | 失败 → target fail，不执行后续步骤 | `"describe-failed"` |
| 2 | `GetCollectionStatistics`（获取 row count） | 失败不阻断，记录并继续 | — |
| 3 | 检查 collection load state | — | — |
| 4 | 执行 query（`limit=1`） | 失败 → target fail | `"query"` |
| 5 | 若配置了 `anns_field`，执行 search | 见下 | `"query+search"` |

**Step 5 search 规则（严格模式）：**
- 从 step 1 的 schema 中读取 `anns_field` 字段的 `dim`
- 若 schema 中找不到 `anns_field` 字段或 dim 为 0：target 记为 fail，Action = `"query+search"`，Error 中说明原因
- 生成 `dim` 维随机 float32 向量，每个元素值域 `[-1.0, 1.0]`，固定种子 42
- search 失败：target 记为 fail
- **首版禁止静默降级**：一旦配置了 `anns_field`，无论任何原因 search 无法执行，均判定该 target 失败，不允许改为"仅 query 成功"来规避

**未配置 `anns_field` 时：** Action = `"query"`，steps 1-4 全部成功则 target 成功。

### 10.4 成功标准
- 未配置任何 targets → `skip`
- 成功数 **>= `min_success_targets`**（含等号）→ `pass`
- 成功数 > 0 且 < `min_success_targets` → `warn`
- 成功数 = 0 → `fail`
- 默认 `min_success_targets = 1`

若所有 targets 被 `--collection` scope 过滤掉，probe 状态为 `skip`（不计为 fail 或 warn）。

---

## 11. RW Probe 规格

### 11.1 目标
验证集群具备最小闭环写入、索引、加载、查询能力。

### 11.2 首版固定 schema
- `id`：Int64，主键，非 auto id
- `vector`：FloatVector，维度 = `vector_dim`（默认 128）
- `payload`：VarChar，最大长度 256

### 11.3 执行步骤

**预检阶段（正式步骤前必须执行）：**
- 构造本次测试 database 名：`{test_database_prefix}_{runID}`
- 扫描已存在的同前缀 database（`ListDatabases` + 前缀过滤，使用 `strings.HasPrefix(dbName, prefix+"_")` 匹配，避免误清理业务 database）
- 若发现同前缀遗留 database：
  - 尝试 drop collection → drop database 清理
  - **清理失败：RW Probe 记为 `fail`，message = `"pre-existing test data cleanup failed: <e>"`，不执行后续正式步骤**
  - 清理成功：继续执行正式步骤

**正式步骤（按顺序）：**
1. 创建测试 database
2. 创建测试 collection
3. 插入测试数据
4. flush
5. 为 `vector` 字段创建索引
6. load collection
7. 执行 search
8. 执行 query
9. 若 `cleanup = true`，删除测试 collection / database

### 11.4 成功标准
- 所有步骤成功 → `pass`
- 预检清理失败 → `fail`（不执行后续步骤）
- 任一正式步骤失败 → `fail`
- 配置 `enabled = false` → `skip`

---

## 12. 关键 Pod 定义（版本感知）

工具识别 `arch_profile` 后，使用对应组件列表进行健康判断。

### 12.1 v2.4 档位关键组件

| 组件角色 | 关键等级 | 识别方式 |
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

| 组件角色 | 关键等级 | 识别方式 |
|---|---|---|
| proxy | 关键 coordinator | label 或名称前缀 `proxy` |
| rootcoord | 关键 coordinator | label 或名称前缀 `rootcoord` |
| mixcoord | 关键 coordinator | label 或名称前缀 `mixcoord` |
| querynode | 关键 data plane | label 或名称前缀 `querynode` |
| datanode | 关键 data plane | label 或名称前缀 `datanode` |
| streaming node | 关键 data plane | label 或名称前缀 `streaming` |

**额外规则：** v2.6 档位集群中，若发现独立的 `indexcoord` / `datacoord` / `querycoord` Pod 存在，输出 WARN，message = `"legacy coordinator pods detected in v2.6 cluster, upgrade may be incomplete"`。

### 12.3 unknown 档位处理

当 `arch_profile=unknown` 时，处理职责如下：

| 层 | 职责 |
|---|---|
| **K8s collector** | 照常执行 Pod/Service 枚举，将原始事实（Phase、Ready、RestartCount 等）填入 `K8sStatus.Pods`；产出**空 CheckResult 列表**（不自行判定 FAIL/WARN） |
| **K8s Analyzer** | 检测到 `arch_profile=unknown`，统一产出一条 `category="k8s", status="skip", message="arch_profile unknown, pod health check skipped"` 的 CheckResult；不对任何 Pod 产出 FAIL/WARN |
| **渲染层** | K8s Summary 显示"pod health check skipped: arch_profile unknown" |

### 12.4 Pod Ignored 规则

以下 Pod 不参与健康判定（分析层跳过），在 detail 输出中标注忽略原因：

| 忽略原因（IgnoredReason） | 触发条件 |
|---|---|
| `phase-succeeded` | Pod.Phase == "Succeeded"（即 Completed 状态的 Job Pod） |
| `debug-pod` | Pod 名称包含 `-debug`，或 label 含 `purpose=debug` |
| `unmatched-role` | Pod 角色无法映射到任何已知 Milvus 组件（Role = "unknown"），且 `arch_profile != unknown` |

**注意：** `arch_profile=unknown` 时所有 Pod 的 Role 均为 `unknown`，但此时 `unmatched-role` 规则**不触发**——Pod 不因 arch_profile=unknown 而被标记为 Ignored，原始事实仍保留在 `K8sStatus.Pods` 中供 detail 查看。

### 12.5 FAIL 规则
- 任一关键 coordinator 类组件不存在或未 Ready
- 关键 data plane 组件**全部**不可用
- 关键 Pod phase 为 `Failed`
- 关键 Pod 任一容器 `waiting.reason == CrashLoopBackOff`
- 关键 Pod restart count 达到或超过 `rules.pod_restart_fail`（配置了该阈值时）

### 12.6 WARN 规则
- 存在关键 Pod restart count 达到或超过 `rules.pod_restart_warn`（但未达到 `pod_restart_fail`）
- 存在关键 Pod **usage/limit 占比**超过 `resource_warn_ratio`（CPU 或 Memory 任一触发即告警）
- 某类横向扩展组件副本数低于预期但未全部失败

---

## 13. 未建索引判定规则

- 仅针对向量字段做索引检查
- 某向量字段不存在任何向量索引 → 该字段视为未建索引
- `unindexed_vector_field_count > 0` → collection 级检查记为 WARN，不直接 FAIL
- 首版不对标量字段索引做强制健康判定

---

## 14. Pod 资源采集失败的降级规则

客户现场可能缺失 metrics-server 或权限不足，规则如下：

- Pod 列表与基础状态可获取，但 CPU / Memory 使用率**全部**无法获取：资源检查项状态为 `warn`
- Pod 列表与基础状态可获取，部分 Pod 无法获取资源数据（**partial**）：
  - 有 metrics 的 Pod 正常计算比例并触发 WARN 判断
  - 无 metrics 的 Pod ratio 字段为 `null`（JSON），text 中显示 `unknown`
  - K8s Summary 显示 `partial (N/M pods have metrics)`
  - 整体不升级为 FAIL，但在 warnings 中说明缺少 metrics 的 Pod 数量与原因
- 工具须区分两类失败原因：
  - `"metrics-server not found"`
  - `"insufficient permissions"`
- 若连 Pod 基础状态都无法获取 → K8s 检查为 `fail`

---

## 15. 判定模型

### 15.1 PASS 条件
同时满足：
- Milvus 可连接
- 元数据采集成功
- 关键 Pod 健康，**或** K8s 模块被关闭，**或** `arch_profile=unknown`（此时 Pod 健康判定为 skip，视为已满足，但 confidence 降级为 low）
- Business Read Probe 为 `pass` 或 `skip`
- RW Probe 为 `pass` 或 `skip`
- 无 FAIL 级检查项

### 15.2 WARN 条件
不属于 FAIL，但存在 WARN 级风险项。典型情形：
- 存在未建索引向量字段
- 存在未 loaded collection
- 资源使用率不可获取（全部或部分）
- Pod restart 超过 warn 阈值（但未达到 fail 阈值）
- 部分 read probe target 失败，但未低于 min_success_targets

### 15.3 FAIL 条件
满足任一：
- 无法连接 Milvus
- 元数据采集失败
- 关键 Pod 检查失败
- Business Read Probe 全失败（成功数为 0）
- RW Probe 失败
- 核心流程运行时中断

---

## 16. confidence 计算规则

| 条件 | confidence |
|---|---|
| 所有模块均执行（未 skip），且无 FAIL、无 WARN | `high` |
| 所有模块均执行，存在 WARN 但无 FAIL | `medium` |
| 存在任一模块被 skip，或 `arch_profile=unknown`，或存在 FAIL | `low` |

优先级：从上到下匹配，取第一个满足的。

---

## 17. standby 判定

### 17.1 合法组合矩阵

| result | standby | 是否合法 | 说明 |
|---|:---:|:---:|---|
| PASS | true | ✅ | 正常健康 |
| PASS | false | ✅ | require_probe_for_standby=true 且 probe 全 skip |
| WARN | true | ✅ | 有风险但仍可服务 |
| WARN | false | ✅ | 有风险且不满足 standby 条件 |
| FAIL | true | ❌ | 非法，FAIL 时 standby 必须为 false |
| FAIL | false | ✅ | 正常失败状态 |

### 17.2 standby = true 条件
- 最终结果不为 FAIL
- 关键 Pod 检查通过（或 K8s 模块 skip，或 arch_profile=unknown 导致的 skip）
- 已启用的 probe 结果不为 FAIL
- 若 `require_probe_for_standby = true`，则必须至少成功执行一种 probe

### 17.3 跳过 probe 时的语义
- `require_probe_for_standby = true` 且 probe 均为 skip → standby 必须为 false
- 结果中标注：`"standby confidence downgraded because probes were skipped"`

---

## 18. 固定退出码

| 退出码 | 含义 | 触发场景 |
|:---:|---|---|
| `0` | PASS | 巡检通过，无 FAIL 无 WARN |
| `1` | WARN | 巡检完成，存在 WARN 级风险，无 FAIL |
| `2` | FAIL | 巡检完成，集群被判定为不健康 |
| `3` | CONFIG_ERROR | 配置文件无法加载或校验失败；传入未知 `--modules` 模块名 |
| `4` | RUNTIME_ERROR | 工具自身意外中断（panic、依赖缺失、timeout） |

**exit 2 与 exit 4 的精确边界：**

| 情形 | 退出码 |
|---|---|
| Milvus 连接建立后 ping/version 调用失败 | 2（业务层 FAIL） |
| Milvus gRPC 客户端对象创建本身失败（地址格式问题等） | 4（工具无法运行） |
| K8s API 不可访问（K8s 检查 FAIL） | 2 |
| 工具 panic / 未捕获异常 | 4 |
| `--timeout` 超时导致整体终止 | 4 |
| Business Read Probe 全失败 | 2 |
| RW Probe 失败 | 2 |

`validate` 成功返回 `0`，配置非法返回 `3`。

---

## 19. 首版工程约束

- 必须模块化实现，不允许所有逻辑堆到单文件 main 中
- 采集与分析必须分层；probe 必须独立模块化
- 配置解析必须与运行逻辑分离
- 所有错误必须结构化返回
- 所有检查项必须统一产出 CheckResult
- `json` 输出必须满足可被标准 JSON 工具直接消费的要求
- `examples/config.example.yaml`、spec 样例、README 示例必须同步维护
- 语义为"unknown-when-nil"的字段（`*int64`、`*float64`）不得使用 `omitempty`，JSON 输出 `null` 而非省略字段

---

## 20. K8s 权限要求

最小 K8s RBAC 权限（建议在 README 中提供此样例）：

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

说明：`metrics.k8s.io` 权限仅在集群安装了 metrics-server 时生效；若无此权限，工具按 §14 降级规则处理。

---

## 21. 首版验收标准

### 21.1 基本能力
- 能正常执行 `version`、`validate`、`check`

### 21.2 正常场景
- text 输出可在 terminal 中直接阅读
- `--format json` 输出可被 `jq` 正常解析
- 返回码为 `0` 或 `1`
- 能正确显示 database / collection 数量、total rows、total binlog size
- 能正确显示 Pod 状态（含 usage/request/limit ratio）
- 能正确识别 `arch_profile`

### 21.3 异常场景
- 配置缺失时返回 `3`
- URI 格式错误（含 scheme 前缀）时 `validate` 返回 `3` 并提示
- 传入未知 `--modules` 模块名时返回 `3`
- Milvus 不可连接时返回 `2`
- Business Read Probe 全失败时返回 `2`
- RW Probe 失败时返回 `2`
- 工具自身 panic 时返回 `4`
- stderr 中可看到足够定位问题的错误信息

### 21.4 输出质量
- text 输出清晰列出 FAIL / WARN 原因
- json 输出不得混入日志
- binlog_size 不可用时 JSON 输出 `null`，不省略字段
- Pod resource ratio 不可用时 JSON 输出 `null`，text 显示 `unknown`
- `arch_profile=unknown` 时 K8s Summary 正确显示 skip 说明
- detail 模式下能看到 collection / Pod 明细

### 21.5 版本兼容性
- 在 2.4.x 集群上能正确识别独立 coordinator 组件
- 在 2.6.x 集群上能正确识别 mixCoord / Streaming Node，不对旧组件缺失触发 FAIL
- `arch_profile=unknown` 时工具不崩溃，输出合理的 skip 说明

### 21.6 管道兼容性
- 支持 `> report.txt`、`| jq .`、`2> debug.log`、`| tee report.txt`

---

## 22. 建议目录结构

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
│   └── milvus-health-spec-v1.2.md
└── main.go
```

---

## 23. 后续版本方向

- `probe` / `analyze` 命令独立暴露
- 支持离线读取 metadata 再生成报告
- 支持 etcd / MinIO / MQ 深度检查
- 支持 markdown / HTML 报告导出
- 支持 Prometheus 指标接入（获取 index 文件大小，补全 total storage）
- Milvus 2.6.x Woodpecker 深度检查

---

## 24. 当前版本结论

截至 v1.2，本文档在 v1.1 基础上完成了第三轮红队修订，核心补齐：

1. **`pod_restart_fail` 规则闭合**：§12.5 FAIL 规则补充"restart count 达到 `pod_restart_fail` → FAIL"，消除配置字段有定义但规则表中缺失对应条款的矛盾
2. **§7.1 字段说明更新**：`pod_restart_warn` / `pod_restart_fail` 的说明文字精确描述触发语义，并在约束中注明两者的大小关系
3. **§12.4 `unmatched-role` 规则精确化**：明确该规则在 `arch_profile=unknown` 时不触发，避免所有 Pod 被误标为 Ignored

---

## 25. 协作与交付约束

### 25.1 GitHub 提交要求
- 所有代码、测试、文档改动完成后，必须 push 到 GitHub
- 建议以独立分支提交，分支名应体现本轮目标
- 未 push 到 GitHub 的改动，不视为本轮完成

### 25.2 最低交付清单
每轮开发或测试完成后，执行人必须同步以下信息：
- branch 名、最后一个 commit SHA
- 改动文件列表
- 运行过的验收命令（格式：`命令 → 输出摘要`，例如 `make test → PASS 42/42`）
- 一条 reviewer 可直接在本地执行的验收命令（需说明前提条件）
- 一段简明变更说明：完成了什么、哪些仍是 stub、已知风险

### 25.3 评审可读性要求
- 改动说明必须能让 reviewer 定位：本轮改了哪些模块、输出契约是否变化、是否引入新配置字段
- 若包含输出样例变化，必须同步更新 `examples/`、README 与 spec 样例
- 若包含接口层变更，必须同步更新接口设计文档

### 25.4 验收原则
- reviewer 以 GitHub 可见的 branch / PR / commit 为唯一验收基准
- 仅口头说明"已完成"而无仓库证据，不进入验收
- `make fmt`、`make test`、`make build` 必须全部通过后才视为一轮交付就绪（项目须先建立 Makefile）
