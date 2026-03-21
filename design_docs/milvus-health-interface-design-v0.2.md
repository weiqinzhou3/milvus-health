# milvus-health 接口设计文档 v0.2

基于规格文档 `milvus-health Specification v0.8` 进行模块拆分与接口设计，目标是为首版 Go 实现提供可直接落地的工程接口约束。

> **变更说明（v0.1 → v0.2）**：
> 1. 新增 `MilvusArchProfile` 枚举及版本识别逻辑
> 2. `ClusterInfo` 补充 `ArchProfile` 字段
> 3. `MilvusClient` 接口移除有状态的 `UseDatabase`，改为各方法直接传 db 参数
> 4. `AnalyzeInput` 移除 `Detail bool`，该字段下移至渲染层（`Renderer.Render`）
> 5. `StandbyAnalyzer` / `SummaryBuilder` 补充实际方法签名
> 6. `RWProbeResult` 区分 `CleanupEnabled` 与 `CleanupExecuted`
> 7. `K8sClient` 补充 metrics 不可用的结构化原因
> 8. `ServiceInfo.Ports` 明确格式定义
> 9. `Run` 伪代码修正：非关键错误不提前 return，尽量采集更多事实
> 10. `K8sCollector` 新增版本感知的组件角色解析逻辑

---

## 1. 文档目标

本文档不重复 spec 中的产品需求，而是将其转换为开发视角的：

- 模块边界
- 包职责
- 核心数据流
- 接口定义
- 错误模型
- 命令编排关系
- 未来扩展点

本文档默认实现语言为 Go，默认目录结构遵循 spec 建议：

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
├── docs/
└── main.go
```

---

## 2. 设计原则

根据 spec，首版接口设计必须满足以下原则：

1. **命令层与业务层分离**  
   CLI 只负责参数解析、执行编排、输出与退出码映射；不承载具体采集和分析逻辑。

2. **采集、探测、分析、渲染分层**  
   `collectors` 负责事实采集，`probes` 负责业务探测，`analyzers` 负责规则判定，`render` 负责输出格式。

3. **统一事实模型与统一检查结果**  
   无论 Milvus/K8s/Probe，最终都要归并为统一的 `MetadataSnapshot`、`CheckResult`、`AnalysisResult`。

4. **stdout / stderr 严格分离**  
   渲染器只负责正式结果正文，日志统一走 stderr。

5. **可测试、可替换、可扩展**  
   所有外部依赖（Milvus client、K8s client、时钟、ID 生成器）都应通过接口注入，避免硬编码。

6. **分析与渲染职责不混淆**  
   `Detail bool` 属于渲染决策，不属于分析决策。分析器永远产出完整的 CheckResult 列表；渲染器根据 `Detail` 决定输出哪些。

---

## 3. 首版模块拆分

建议拆为 9 个一级功能模块。

### 3.1 `cmd/`
职责：
- 程序入口
- Cobra 命令注册（建议）
- 子命令路由：`check` / `validate` / `version`

### 3.2 `internal/cli/`
职责：
- CLI 参数模型
- 命令执行 orchestrator
- 退出码映射
- stderr 日志控制

### 3.3 `internal/config/`
职责：
- YAML 配置文件加载
- 默认值填充
- 配置校验
- CLI 覆盖配置

### 3.4 `internal/model/`
职责：
- 全部领域模型定义
- Snapshot / Probe / Check / Result / ExitCode / ErrorCode 模型

### 3.5 `internal/collectors/`
职责：
- Milvus 基础信息采集
- Milvus inventory 采集
- K8s Pod/Service/Resource 采集

建议子模块：
- `collectors/milvus`
- `collectors/k8s`

### 3.6 `internal/probes/`
职责：
- Business Read Probe
- RW Probe

### 3.7 `internal/analyzers/`
职责：
- 规则判定
- 检查项归一化
- PASS/WARN/FAIL 与 standby 计算
- confidence 计算

### 3.8 `internal/render/`
职责：
- `text` 输出
- `json` 输出
- 输出契约保证
- `Detail bool` 控制（渲染层决策，不属于分析层）

### 3.9 `internal/platform/`
职责：
- 外部客户端构造与依赖封装
- Milvus client 封装
- K8s client 封装
- 运行时辅助能力（时钟、UUID、随机数据生成）

说明：spec 里没有强制这一层，但为了降低 `collectors` / `probes` 与 SDK 的强耦合，建议首版就加。

---

## 4. 总体调用链设计

`check` 命令建议采用如下调用链：

```text
CLI -> ConfigLoader -> ConfigValidator -> Runner
    -> Collectors
        -> MilvusCollector      (ArchProfile 在此确定)
        -> K8sCollector         (ArchProfile 作为输入)
    -> Probes
        -> BusinessReadProbe
        -> RWProbe
    -> Analyzer
    -> Renderer                 (Detail bool 在此消费)
    -> ExitCodeMapper
```

### 4.1 `validate` 调用链

```text
CLI -> ConfigLoader -> ConfigValidator -> ResultRenderer(optional text only) -> exit 0/3
```

### 4.2 `version` 调用链

```text
CLI -> VersionProvider -> stdout -> exit 0
```

---

## 5. 核心数据流

### 5.1 输入
输入来源只有两类：

1. 配置文件 YAML
2. CLI 参数覆盖

### 5.2 中间事实层
首版建议统一汇总为：

- `ClusterInfo`（含 `ArchProfile`）
- `MilvusInventory`
- `K8sStatus`
- `BusinessReadProbeResult`
- `RWProbeResult`
- `MetadataSnapshot`

### 5.3 分析输出层
分析后统一得到：

- `[]CheckResult`（完整列表，不受 Detail 影响）
- `AnalysisSummary`
- `AnalysisResult`

### 5.4 渲染输出层
最终由 render 层将 `AnalysisResult` 渲染为：

- `text` 字节流（Detail 决定输出哪些明细）
- `json` 字节流（Detail 决定 `checks` 字段是否展开）

---

## 6. `internal/model` 模型设计

### 6.1 基础枚举

```go
type OutputFormat string

const (
    OutputFormatText OutputFormat = "text"
    OutputFormatJSON OutputFormat = "json"
)

type CheckStatus string

const (
    CheckStatusPass CheckStatus = "pass"
    CheckStatusWarn CheckStatus = "warn"
    CheckStatusFail CheckStatus = "fail"
    CheckStatusSkip CheckStatus = "skip"
)

type FinalResult string

const (
    FinalResultPASS FinalResult = "PASS"
    FinalResultWARN FinalResult = "WARN"
    FinalResultFAIL FinalResult = "FAIL"
)

type ConfidenceLevel string

const (
    ConfidenceHigh   ConfidenceLevel = "high"
    ConfidenceMedium ConfidenceLevel = "medium"
    ConfidenceLow    ConfidenceLevel = "low"
)

// MilvusArchProfile 表示集群的架构档位，由版本号推断得出
// v24: Milvus 2.4.x，独立 coordinator 架构
// v26: Milvus 2.6.x，mixCoord + StreamingNode 架构
// unknown: 无法识别的版本，K8s 组件健康检查降为 WARN
type MilvusArchProfile string

const (
    ArchProfileV24     MilvusArchProfile = "v24"
    ArchProfileV26     MilvusArchProfile = "v26"
    ArchProfileUnknown MilvusArchProfile = "unknown"
)

// DetectArchProfile 根据 Milvus 版本字符串推断 ArchProfile
func DetectArchProfile(version string) MilvusArchProfile {
    switch {
    case strings.HasPrefix(version, "2.4."):
        return ArchProfileV24
    case strings.HasPrefix(version, "2.6."):
        return ArchProfileV26
    default:
        return ArchProfileUnknown
    }
}

// MetricsUnavailableReason 描述 Pod 资源使用率无法获取的原因
type MetricsUnavailableReason string

const (
    MetricsUnavailableReasonNotFound        MetricsUnavailableReason = "metrics-server-not-found"
    MetricsUnavailableReasonPermissionDenied MetricsUnavailableReason = "permission-denied"
    MetricsUnavailableReasonUnknown         MetricsUnavailableReason = "unknown"
)
```

### 6.2 配置模型

```go
type Config struct {
    Cluster      ClusterConfig      `yaml:"cluster"`
    K8s          K8sConfig          `yaml:"k8s"`
    Dependencies DependenciesConfig `yaml:"dependencies"`
    Probe        ProbeConfig        `yaml:"probe"`
    Rules        RulesConfig        `yaml:"rules"`
    Output       OutputConfig       `yaml:"output"`
}

type ClusterConfig struct {
    Name   string       `yaml:"name"`
    Milvus MilvusConfig `yaml:"milvus"`
}

type MilvusConfig struct {
    // URI 格式：host:port，不带 tcp:// 前缀
    // 例如：milvus.milvus.svc.cluster.local:19530
    URI      string `yaml:"uri"`
    Username string `yaml:"username"`
    Password string `yaml:"password"`
    Token    string `yaml:"token"`
}

type K8sConfig struct {
    Namespace  string `yaml:"namespace"`
    Kubeconfig string `yaml:"kubeconfig"`
}

type DependenciesConfig struct {
    MQ MQConfig `yaml:"mq"`
}

type MQConfig struct {
    Type string `yaml:"type"`
}

type ProbeConfig struct {
    Read ReadProbeConfig `yaml:"read"`
    RW   RWProbeConfig   `yaml:"rw"`
}

type ReadProbeConfig struct {
    MinSuccessTargets int               `yaml:"min_success_targets"`
    Targets           []ReadProbeTarget `yaml:"targets"`
}

type ReadProbeTarget struct {
    Database     string   `yaml:"database"`
    Collection   string   `yaml:"collection"`
    QueryExpr    string   `yaml:"query_expr"`
    // AnnsField 若配置，工具将 describe collection 获取该字段实际维度，
    // 再用随机向量（固定种子 42）执行 ANN search。
    // 若 describe 失败或字段不存在，自动降级为 query-only 并记录 WARN。
    AnnsField    string   `yaml:"anns_field"`
    TopK         int      `yaml:"topk"`
    OutputFields []string `yaml:"output_fields"`
}

type RWProbeConfig struct {
    Enabled            bool   `yaml:"enabled"`
    TestDatabasePrefix string `yaml:"test_database_prefix"`
    // Cleanup 控制 probe 结束后是否删除测试 database/collection
    // 注意：无论此值如何，工具每次启动时都会清理前次遗留的同前缀 database
    Cleanup            bool   `yaml:"cleanup"`
    InsertRows         int    `yaml:"insert_rows"`
    VectorDim          int    `yaml:"vector_dim"`
}

type RulesConfig struct {
    PodRestartWarn         int     `yaml:"pod_restart_warn"`
    PodRestartFail         int     `yaml:"pod_restart_fail"`
    ResourceWarnRatio      float64 `yaml:"resource_warn_ratio"`
    RequireProbeForStandby bool    `yaml:"require_probe_for_standby"`
}

type OutputConfig struct {
    Format OutputFormat `yaml:"format"`
    Detail bool         `yaml:"detail"`
}
```

### 6.3 CLI 运行参数模型

```go
type CheckOptions struct {
    ConfigPath string
    Format     OutputFormat
    Profile    string
    Verbose    bool
    TimeoutSec int
    // Cleanup 使用指针，方便区分"用户未传"（nil）和"用户显式覆盖"（&true / &false）
    Cleanup    *bool
    // Modules 可选值为 "k8s" / "probe"；"milvus" 为必选，不出现在此列表中也默认启用
    Modules    []string
    Database   string
    Collection string
    Detail     bool
}

type ValidateOptions struct {
    ConfigPath string
    Verbose    bool
}
```

### 6.4 事实模型

```go
type MetadataSnapshot struct {
    ClusterInfo       ClusterInfo             `json:"cluster_info"`
    MilvusInventory   MilvusInventory         `json:"milvus_inventory"`
    K8sStatus         K8sStatus               `json:"k8s_status"`
    BusinessReadProbe BusinessReadProbeResult `json:"business_read_probe"`
    RWProbe           RWProbeResult           `json:"rw_probe"`
}

type ClusterInfo struct {
    Name          string            `json:"name"`
    MilvusURI     string            `json:"milvus_uri"`
    Namespace     string            `json:"namespace"`
    MilvusVersion string            `json:"milvus_version"`
    // ArchProfile 由版本字符串推断，驱动后续 K8s 组件健康检查逻辑
    ArchProfile   MilvusArchProfile `json:"arch_profile"`
    MQType        string            `json:"mq_type"`
    Components    []string          `json:"components"`
    Services      []string          `json:"services"`
}

type MilvusInventory struct {
    DatabaseCount             int                 `json:"database_count"`
    CollectionCount           int                 `json:"collection_count"`
    Databases                 []DatabaseInventory `json:"databases"`
    LoadedCollectionCount     int                 `json:"loaded_collection_count"`
    UnloadedCollectionCount   int                 `json:"unloaded_collection_count"`
    UnindexedVectorFieldCount int                 `json:"unindexed_vector_field_count"`
}

type DatabaseInventory struct {
    Name        string                `json:"name"`
    Collections []CollectionInventory `json:"collections"`
}

type CollectionInventory struct {
    Database                  string        `json:"database"`
    Name                      string        `json:"name"`
    RowCount                  int64         `json:"row_count"`
    PartitionCount            int           `json:"partition_count"`
    ShardNum                  int           `json:"shard_num,omitempty"`
    ReplicaNum                int           `json:"replica_num,omitempty"`
    LoadState                 string        `json:"load_state"`
    IndexCount                int           `json:"index_count"`
    IndexTypes                []string      `json:"index_types"`
    VectorFields              []VectorField `json:"vector_fields"`
    IndexedVectorFieldCount   int           `json:"indexed_vector_field_count"`
    UnindexedVectorFieldCount int           `json:"unindexed_vector_field_count"`
}

type VectorField struct {
    Name      string `json:"name"`
    Dim       int    `json:"dim,omitempty"`
    Indexed   bool   `json:"indexed"`
    IndexType string `json:"index_type,omitempty"`
}

type K8sStatus struct {
    Namespace              string                   `json:"namespace"`
    TotalPodCount          int                      `json:"total_pod_count"`
    ReadyPodCount          int                      `json:"ready_pod_count"`
    NotReadyPodCount       int                      `json:"not_ready_pod_count"`
    ResourceUsageAvailable bool                     `json:"resource_usage_available"`
    // ResourceUnavailableReason 仅在 ResourceUsageAvailable=false 时有意义
    ResourceUnavailableReason MetricsUnavailableReason `json:"resource_unavailable_reason,omitempty"`
    Pods                   []PodStatus              `json:"pods"`
    Services               []ServiceInfo            `json:"services"`
}

type PodStatus struct {
    Name          string `json:"name"`
    Role          string `json:"role"`
    Phase         string `json:"phase"`
    Ready         bool   `json:"ready"`
    RestartCount  int32  `json:"restart_count"`
    CPUUsage      string `json:"cpu_usage,omitempty"`      // 格式: "125m"（millicores）
    MemoryUsage   string `json:"memory_usage,omitempty"`   // 格式: "512Mi"
    CPURequest    string `json:"cpu_request,omitempty"`
    CPULimit      string `json:"cpu_limit,omitempty"`
    MemoryRequest string `json:"memory_request,omitempty"`
    MemoryLimit   string `json:"memory_limit,omitempty"`
}

type ServiceInfo struct {
    Name  string   `json:"name"`
    Type  string   `json:"type"`
    // Ports 格式：["19530/TCP", "9091/TCP"]
    // 每个元素为 "port/protocol" 形式
    Ports []string `json:"ports"`
}
```

### 6.5 Probe 结果模型

```go
type BusinessReadProbeResult struct {
    Status            CheckStatus               `json:"status"`
    ConfiguredTargets int                       `json:"configured_targets"`
    SuccessfulTargets int                       `json:"successful_targets"`
    MinSuccessTargets int                       `json:"min_success_targets"`
    Message           string                    `json:"message"`
    Targets           []BusinessReadTargetResult `json:"targets,omitempty"`
}

type BusinessReadTargetResult struct {
    Database   string `json:"database"`
    Collection string `json:"collection"`
    // Action 记录实际执行的操作："query"、"search"、"search-degraded-to-query"
    Action     string `json:"action"`
    Success    bool   `json:"success"`
    DurationMS int64  `json:"duration_ms"`
    Error      string `json:"error,omitempty"`
    RowCount   int64  `json:"row_count,omitempty"`
}

type RWProbeResult struct {
    Status  CheckStatus `json:"status"`
    Enabled bool        `json:"enabled"`
    // TestDatabase 本次 probe 使用的测试 database 名（含 runID 后缀）
    TestDatabase   string `json:"test_database,omitempty"`
    TestCollection string `json:"test_collection,omitempty"`
    InsertRows     int    `json:"insert_rows,omitempty"`
    VectorDim      int    `json:"vector_dim,omitempty"`
    // CleanupEnabled: 配置中 cleanup 是否为 true
    CleanupEnabled bool `json:"cleanup_enabled,omitempty"`
    // CleanupExecuted: 本次运行实际是否执行了 cleanup 操作
    // 若 CleanupEnabled=true 但 CleanupExecuted=false，说明 cleanup 步骤失败
    CleanupExecuted bool              `json:"cleanup_executed,omitempty"`
    StepResults     []ProbeStepResult `json:"steps,omitempty"`
    Message         string            `json:"message,omitempty"`
}

type ProbeStepResult struct {
    Name       string `json:"name"`
    Success    bool   `json:"success"`
    DurationMS int64  `json:"duration_ms"`
    Error      string `json:"error,omitempty"`
}
```

### 6.6 分析结果模型

```go
type CheckResult struct {
    Category   string      `json:"category"`
    Name       string      `json:"name"`
    Target     string      `json:"target"`
    Status     CheckStatus `json:"status"`
    Severity   string      `json:"severity"`
    Message    string      `json:"message"`
    Actual     any         `json:"actual,omitempty"`
    Expected   any         `json:"expected,omitempty"`
    DurationMS int64       `json:"duration_ms,omitempty"`
}

type AnalysisSummary struct {
    DatabaseCount             int `json:"database_count"`
    CollectionCount           int `json:"collection_count"`
    LoadedCollectionCount     int `json:"loaded_collection_count"`
    UnloadedCollectionCount   int `json:"unloaded_collection_count"`
    UnindexedVectorFieldCount int `json:"unindexed_vector_field_count"`
    TotalPodCount             int `json:"total_pod_count"`
    ReadyPodCount             int `json:"ready_pod_count"`
    NotReadyPodCount          int `json:"not_ready_pod_count"`
    FailCount                 int `json:"fail_count"`
    WarnCount                 int `json:"warn_count"`
}

type AnalysisResult struct {
    Cluster    ClusterOutputView `json:"cluster"`
    Result     FinalResult       `json:"result"`
    Standby    bool              `json:"standby"`
    Confidence ConfidenceLevel   `json:"confidence"`
    // ExitCode 为预期退出码，与 shell 实际退出码在正常情况下一致
    // 使用方应以进程真实退出码为准，不应依赖此字段做自动化判断
    ExitCode   int               `json:"exit_code"`
    ElapsedMS  int64             `json:"elapsed_ms"`
    Summary    AnalysisSummary   `json:"summary"`
    Probes     ProbeOutputView   `json:"probes"`
    Warnings   []string          `json:"warnings"`
    Failures   []string          `json:"failures"`
    // Checks 完整 CheckResult 列表，渲染层根据 Detail 决定是否在输出中展开
    Checks     []CheckResult     `json:"checks,omitempty"`
}

type ClusterOutputView struct {
    Name          string            `json:"name"`
    MilvusURI     string            `json:"milvus_uri"`
    Namespace     string            `json:"namespace"`
    MilvusVersion string            `json:"milvus_version"`
    ArchProfile   MilvusArchProfile `json:"arch_profile"`
    MQType        string            `json:"mq_type"`
}

type ProbeOutputView struct {
    BusinessRead BusinessReadProbeResult `json:"business_read"`
    RW           RWProbeResult           `json:"rw"`
}
```

---

## 7. `internal/config` 接口设计

### 7.1 职责
- 读取配置文件
- 反序列化
- 填充默认值
- 校验配置
- 应用 CLI 覆盖项

### 7.2 接口定义

```go
type Loader interface {
    Load(path string) (*model.Config, error)
}

type Validator interface {
    Validate(cfg *model.Config) error
}

type DefaultApplier interface {
    Apply(cfg *model.Config)
}

type OverrideApplier interface {
    ApplyCheckOverrides(cfg *model.Config, opts model.CheckOptions) error
}
```

### 7.3 建议实现

```go
type YAMLLoader struct{}
type ConfigValidator struct{}
type DefaultValueApplier struct{}
type CLIOverrideApplier struct{}
```

### 7.4 校验点

`Validate(cfg)` 至少检查：

1. `cluster.name` 非空
2. `cluster.milvus.uri` 非空，且**不以 `tcp://` 开头**（URI 格式应为 `host:port`，若包含 `://` 前缀则返回 CONFIG_ERROR 并给出修正提示）
3. `output.format` 只能是 `text/json`
4. `probe.read.min_success_targets >= 0`
5. `probe.rw.insert_rows > 0`（启用 RW Probe 时）
6. `probe.rw.vector_dim > 0`（启用 RW Probe 时）
7. `password` 与 `token` 同时存在时，token 优先，password 忽略，在 stderr 输出 WARN（不阻断执行）
8. `rules.resource_warn_ratio` 在 `(0, 1]`
9. `--modules` 中若出现未知模块名（非 `k8s` / `probe`），返回 CONFIG_ERROR

### 7.5 配置错误返回模型

```go
type FieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
}

type ConfigError struct {
    Code    string       `json:"code"`
    Message string       `json:"message"`
    Fields  []FieldError `json:"fields,omitempty"`
}

func (e *ConfigError) Error() string
```

---

## 8. `internal/platform` 接口设计

### 8.1 Milvus 客户端抽象

**重要设计决策：不暴露 `UseDatabase`**

`UseDatabase` 是有状态方法，在并发场景（即使首版是顺序的，后续也可能被改为并发）下存在竞态风险。所有需要区分 database 的方法均通过参数直接传入 `db string`，在实现层内部处理 database 切换或使用 SDK 支持的 db 参数。

```go
type MilvusClient interface {
    GetVersion(ctx context.Context) (string, error)
    ListDatabases(ctx context.Context) ([]string, error)
    ListCollections(ctx context.Context, db string) ([]string, error)
    DescribeCollection(ctx context.Context, db, collection string) (*CollectionSchema, error)
    GetCollectionStats(ctx context.Context, db, collection string) (*CollectionStats, error)
    GetLoadState(ctx context.Context, db, collection string) (string, error)
    ListIndexes(ctx context.Context, db, collection string) ([]IndexInfo, error)
    Query(ctx context.Context, req QueryRequest) (*QueryResult, error)
    Search(ctx context.Context, req SearchRequest) (*SearchResult, error)

    CreateDatabase(ctx context.Context, db string) error
    DropDatabase(ctx context.Context, db string) error
    CreateCollection(ctx context.Context, req CreateCollectionRequest) error
    DropCollection(ctx context.Context, db, collection string) error
    Insert(ctx context.Context, req InsertRequest) error
    Flush(ctx context.Context, db, collection string) error
    CreateIndex(ctx context.Context, req CreateIndexRequest) error
    LoadCollection(ctx context.Context, db, collection string) error
}
```

### 8.2 K8s 客户端抽象

```go
type K8sClient interface {
    ListPods(ctx context.Context, namespace string) ([]PodInfo, error)
    ListServices(ctx context.Context, namespace string) ([]ServiceInfo, error)
    // ListPodMetrics 可能因 metrics-server 不存在或权限不足而失败
    // 调用方应检查返回的 MetricsUnavailableReason 并做相应降级处理
    ListPodMetrics(ctx context.Context, namespace string) ([]PodMetric, *model.MetricsUnavailableReason, error)
}
```

**ListPodMetrics 返回语义：**
- 成功：返回 `[]PodMetric, nil, nil`
- metrics-server 不存在（404 / discovery 失败）：返回 `nil, &MetricsUnavailableReasonNotFound, nil`（不是 error，是预期降级）
- 权限不足（403）：返回 `nil, &MetricsUnavailableReasonPermissionDenied, nil`
- 其他意外错误：返回 `nil, &MetricsUnavailableReasonUnknown, err`

### 8.3 运行时辅助抽象

```go
type Clock interface {
    Now() time.Time
}

type IDGenerator interface {
    // NewRunID 生成 8 位十六进制随机字符串，用于 RW Probe 的测试 database 名后缀
    NewRunID() string
    // NewProbeSuffix 生成探测任务内部的唯一标识
    NewProbeSuffix() string
}
```

---

## 9. `internal/collectors` 接口设计

### 9.1 MilvusCollector

职责：
- 连接 Milvus
- 获取版本并推断 ArchProfile
- 获取 database / collection / schema / load state / index / row count
- 形成 `MilvusInventory`
- 同时产出基础 `CheckResult`

```go
type MilvusCollector interface {
    // CollectClusterInfo 获取集群基础信息，含版本和 ArchProfile 推断
    CollectClusterInfo(ctx context.Context, cfg *model.Config) (model.ClusterInfo, []model.CheckResult, error)
    // CollectInventory 盘点 database/collection，scope 限定范围（空表示全量）
    CollectInventory(ctx context.Context, cfg *model.Config, scope CollectScope) (model.MilvusInventory, []model.CheckResult, error)
}

type CollectScope struct {
    Database   string
    Collection string
}
```

### 9.2 K8sCollector

职责：
- 获取 Pod 列表
- 按 ArchProfile 解析组件角色（不同版本关键组件不同）
- Ready / Restart / Phase 归一化
- 获取 Service 列表
- 尝试获取 metrics，处理降级
- 产出 `K8sStatus`

```go
type K8sCollector interface {
    // CollectStatus 需要 archProfile 参数以确定关键组件列表
    CollectStatus(ctx context.Context, cfg *model.Config, archProfile model.MilvusArchProfile) (model.K8sStatus, []model.CheckResult, error)
}
```

### 9.3 建议实现类

```go
type DefaultMilvusCollector struct {
    Client platform.MilvusClient
}

type DefaultK8sCollector struct {
    Client platform.K8sClient
}
```

### 9.4 关键内部辅助方法

Milvus 侧建议拆分：

```go
func (c *DefaultMilvusCollector) collectDatabases(ctx context.Context) ([]string, error)
func (c *DefaultMilvusCollector) collectCollections(ctx context.Context, db string) ([]model.CollectionInventory, error)
func (c *DefaultMilvusCollector) buildCollectionInventory(ctx context.Context, db, collection string) (model.CollectionInventory, error)
func (c *DefaultMilvusCollector) detectVectorFields(schema *platform.CollectionSchema) []model.VectorField
func (c *DefaultMilvusCollector) matchIndexToVectorFields(fields []model.VectorField, indexes []platform.IndexInfo) []model.VectorField
```

K8s 侧建议拆分：

```go
// normalizePodRole 按 ArchProfile 映射 Pod 角色
// v24: 识别 rootcoord/datacoord/querycoord/indexcoord/datanode/querynode/indexnode
// v26: 识别 rootcoord/mixcoord/datanode/querynode/streaming-node
func (c *DefaultK8sCollector) normalizePodRole(name string, labels map[string]string, arch model.MilvusArchProfile) string

func (c *DefaultK8sCollector) calculateReadySummary(pods []model.PodStatus) (total, ready, notReady int)
func (c *DefaultK8sCollector) mergeMetrics(pods []model.PodStatus, metrics []platform.PodMetric) []model.PodStatus

// checkLegacyCoordinators 在 v26 集群上检测是否残留 v24 独立 coordinator Pod
func (c *DefaultK8sCollector) checkLegacyCoordinators(pods []model.PodStatus, arch model.MilvusArchProfile) []model.CheckResult
```

---

## 10. `internal/probes` 接口设计

### 10.1 BusinessReadProbe

```go
type BusinessReadProbe interface {
    Run(ctx context.Context, cfg *model.Config, scope ProbeScope) (model.BusinessReadProbeResult, []model.CheckResult, error)
}

type ProbeScope struct {
    Database   string
    Collection string
}
```

#### search 向量构造逻辑

```go
// buildQueryVector 根据 describe collection 结果构造 ANN search 向量
// 使用固定种子 42，保证可复现
func buildQueryVector(dim int) []float32 {
    rng := rand.New(rand.NewSource(42))
    vec := make([]float32, dim)
    for i := range vec {
        vec[i] = rng.Float32()
    }
    return vec
}
```

#### 过滤策略
如果用户传了 `--database` / `--collection`，probe 层按 scope 过滤 targets，跳过不匹配的 target（记为 `skip`，不计入 configured_targets）。

### 10.2 RWProbe

```go
type RWProbe interface {
    Run(ctx context.Context, cfg *model.Config) (model.RWProbeResult, []model.CheckResult, error)
}
```

### 10.3 RWProbe 拆分建议

```go
// prepareTestNames 生成本次 probe 的 database / collection 名（前缀 + runID）
func (p *DefaultRWProbe) prepareTestNames(prefix string) (dbName, colName string)

// cleanupStaleTestDatabases 在 probe 开始前清理前次遗留的同前缀 database
// 清理失败记录 WARN，不阻断主流程
func (p *DefaultRWProbe) cleanupStaleTestDatabases(ctx context.Context, prefix string) []model.CheckResult

func (p *DefaultRWProbe) createDatabase(ctx context.Context, db string) model.ProbeStepResult
func (p *DefaultRWProbe) createCollection(ctx context.Context, db, col string, dim int) model.ProbeStepResult
func (p *DefaultRWProbe) insertRows(ctx context.Context, db, col string, rows int, dim int) model.ProbeStepResult
func (p *DefaultRWProbe) flush(ctx context.Context, db, col string) model.ProbeStepResult
func (p *DefaultRWProbe) createIndex(ctx context.Context, db, col string) model.ProbeStepResult
func (p *DefaultRWProbe) loadCollection(ctx context.Context, db, col string) model.ProbeStepResult
func (p *DefaultRWProbe) search(ctx context.Context, db, col string, dim int) model.ProbeStepResult
func (p *DefaultRWProbe) query(ctx context.Context, db, col string) model.ProbeStepResult
func (p *DefaultRWProbe) cleanup(ctx context.Context, db string) model.ProbeStepResult
```

### 10.4 RWProbe 数据生成器接口

```go
type RWDataGenerator interface {
    // BuildRows 生成测试数据行，使用固定种子保证稳定可复现
    BuildRows(count int, dim int) ([]map[string]any, error)
}
```

---

## 11. `internal/analyzers` 接口设计

### 11.1 Analyzer 职责

输入：
- `MetadataSnapshot`
- collectors/probes 过程中产生的基础 `CheckResult`

输出：
- 聚合后的 `AnalysisResult`（含完整 `[]CheckResult`，不受 Detail 影响）

### 11.2 接口定义

```go
type Analyzer interface {
    Analyze(ctx context.Context, input AnalyzeInput) (*model.AnalysisResult, error)
}

// AnalyzeInput 不含 Detail，Detail 属于渲染决策，不属于分析决策
type AnalyzeInput struct {
    Config    *model.Config
    Snapshot  model.MetadataSnapshot
    Checks    []model.CheckResult
    StartedAt time.Time
    EndedAt   time.Time
}
```

### 11.3 子分析器建议

首版可以一个 Analyzer 实现完，但内部建议按规则来源拆开。各子分析器接口定义如下：

```go
type ConnectivityAnalyzer interface {
    // Analyze 检查 Milvus 连接状态，返回 CheckResult
    Analyze(info model.ClusterInfo) []model.CheckResult
}

type InventoryAnalyzer interface {
    // Analyze 检查 collection/database 元数据状态（索引、load state 等）
    Analyze(inv model.MilvusInventory) []model.CheckResult
}

type K8sAnalyzer interface {
    // Analyze 按 ArchProfile 检查 Pod 健康状态
    // 需要传入 archProfile 以使用对应版本的关键组件列表
    Analyze(status model.K8sStatus, archProfile model.MilvusArchProfile, rules model.RulesConfig) []model.CheckResult
}

type ProbeAnalyzer interface {
    // Analyze 分析 Business Read Probe 和 RW Probe 的结果
    Analyze(br model.BusinessReadProbeResult, rw model.RWProbeResult, cfg *model.Config) []model.CheckResult
}

type StandbyAnalyzer interface {
    // ComputeStandby 根据 spec §17 的条件计算 standby 值
    // 返回 standby bool 及附加说明消息（如"standby confidence downgraded because probes were skipped"）
    ComputeStandby(checks []model.CheckResult, br model.BusinessReadProbeResult, rw model.RWProbeResult, cfg *model.Config) (standby bool, notes []string)
}

type SummaryBuilder interface {
    // Build 汇总所有 CheckResult，计算 PASS/WARN/FAIL、confidence、fail/warn 数量、warnings/failures 文本
    Build(checks []model.CheckResult, snapshot model.MetadataSnapshot, cfg *model.Config) model.AnalysisSummary
    // ComputeFinalResult 根据所有 CheckResult 得出最终 FinalResult
    ComputeFinalResult(checks []model.CheckResult) model.FinalResult
    // ComputeConfidence 按 spec §18 规则计算 confidence
    ComputeConfidence(checks []model.CheckResult, snapshot model.MetadataSnapshot, opts model.CheckOptions) model.ConfidenceLevel
}
```

### 11.4 分析器必须做的事

1. 汇总所有基础 `CheckResult`
2. 根据 spec 补齐最终判定逻辑
3. 统计 fail/warn 数量
4. 生成 warnings / failures 文本摘要
5. 计算 `standby`（禁止 FAIL + standby=true 组合）
6. 按 spec §18 计算 `confidence`
7. 映射 `exit_code`

### 11.5 关键规则落点

| 规则 | 建议所在分析器 |
|---|---|
| Milvus 连接失败即 FAIL | ConnectivityAnalyzer |
| collection 未建向量索引 => WARN | InventoryAnalyzer |
| unloaded collection => WARN（非 FAIL） | InventoryAnalyzer |
| Pod restart 超阈值 => WARN/FAIL | K8sAnalyzer |
| v26 集群发现 v24 独立 coordinator => WARN | K8sAnalyzer |
| metrics 缺失 => WARN（含 reason） | K8sAnalyzer |
| Business Read Probe 判定 | ProbeAnalyzer |
| RW Probe 判定 | ProbeAnalyzer |
| standby 计算（含合法组合校验） | StandbyAnalyzer |
| PASS/WARN/FAIL 汇总 | SummaryBuilder |
| confidence 计算 | SummaryBuilder |

---

## 12. `internal/render` 接口设计

### 12.1 Renderer 顶层接口

```go
// RenderOptions 携带渲染层决策参数，不影响分析结果
type RenderOptions struct {
    Detail bool
}

type Renderer interface {
    // Render 将 AnalysisResult 渲染为字节流
    // Detail 在此消费：Detail=false 只输出摘要，Detail=true 展开 collection/pod 明细
    Render(result *model.AnalysisResult, opts RenderOptions) ([]byte, error)
}
```

### 12.2 工厂接口

```go
type RendererFactory interface {
    Get(format model.OutputFormat) (Renderer, error)
}
```

### 12.3 实现类

```go
type TextRenderer struct{}
type JSONRenderer struct{}
type DefaultRendererFactory struct{}
```

### 12.4 渲染约束

#### TextRenderer
必须输出：
- Cluster 摘要（含 Arch Profile）
- Elapsed
- Overall Result
- Standby
- Confidence
- Exit Code
- Inventory Summary
- K8s Summary（含 metrics unavailable reason）
- Business Read Probe
- RW Probe（含 cleanup_enabled / cleanup_executed）
- Warnings
- Failures

Detail=true 时额外输出：
- collection 逐项明细
- Pod 逐项明细
- 完整 CheckResult 列表

#### JSONRenderer
必须保证：
- `json.MarshalIndent` 后的纯 JSON
- 不混入任何日志
- 输出字段至少覆盖 spec 样例要求
- Detail=false 时 `checks` 字段为空数组或省略
- Detail=true 时 `checks` 字段输出完整 CheckResult 列表

---

## 13. `internal/cli` 接口设计

### 13.1 Runner 设计

```go
type CheckRunner interface {
    Run(ctx context.Context, opts model.CheckOptions) (*model.AnalysisResult, error)
}

type ValidateRunner interface {
    Run(ctx context.Context, opts model.ValidateOptions) error
}
```

### 13.2 默认实现

```go
type DefaultCheckRunner struct {
    Loader          config.Loader
    Validator       config.Validator
    DefaultApplier  config.DefaultApplier
    OverrideApplier config.OverrideApplier

    MilvusCollector collectors.MilvusCollector
    K8sCollector    collectors.K8sCollector
    ReadProbe       probes.BusinessReadProbe
    RWProbe         probes.RWProbe
    Analyzer        analyzers.Analyzer
}
```

### 13.3 `Run` 编排建议伪代码

**核心原则：非关键错误不提前 return，尽量采集更多事实，统一交给 Analyzer 处理。只有配置错误、Milvus 连接建立失败（对象都无法创建）等真正无法继续的情况才提前终止。**

```go
func (r *DefaultCheckRunner) Run(ctx context.Context, opts model.CheckOptions) (*model.AnalysisResult, error) {
    cfg, err := r.Loader.Load(opts.ConfigPath)
    if err != nil {
        return nil, &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
    }

    r.DefaultApplier.Apply(cfg)
    if err := r.OverrideApplier.ApplyCheckOverrides(cfg, opts); err != nil {
        return nil, &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
    }
    if err := r.Validator.Validate(cfg); err != nil {
        return nil, &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
    }

    startedAt := time.Now()
    var allChecks []model.CheckResult
    snapshot := model.MetadataSnapshot{}

    // Milvus 采集：ConnectClusterInfo 失败则提前终止（无法继续）
    clusterInfo, checks, err := r.MilvusCollector.CollectClusterInfo(ctx, cfg)
    allChecks = append(allChecks, checks...)
    if err != nil {
        // 连接失败：仍然产出 AnalysisResult（FAIL），不直接 return error
        // 将错误转为 CheckResult，继续走 Analyzer
        allChecks = append(allChecks, connectivityFailCheck(err))
    } else {
        snapshot.ClusterInfo = clusterInfo

        // Inventory 采集：失败记录为 WARN/FAIL check，不提前 return
        inv, invChecks, invErr := r.MilvusCollector.CollectInventory(ctx, cfg, scopeFromOpts(opts))
        allChecks = append(allChecks, invChecks...)
        if invErr == nil {
            snapshot.MilvusInventory = inv
        }
    }

    // K8s 采集：模块可关闭，失败记录 check，不提前 return
    if moduleEnabled(opts.Modules, "k8s") && cfg.K8s.Namespace != "" {
        ks, ksChecks, _ := r.K8sCollector.CollectStatus(ctx, cfg, snapshot.ClusterInfo.ArchProfile)
        allChecks = append(allChecks, ksChecks...)
        snapshot.K8sStatus = ks
    }

    // Probe 采集：模块可关闭，失败记录 check，不提前 return
    if moduleEnabled(opts.Modules, "probe") {
        br, brChecks, _ := r.ReadProbe.Run(ctx, cfg, probeScopeFromOpts(opts))
        allChecks = append(allChecks, brChecks...)
        snapshot.BusinessReadProbe = br

        rw, rwChecks, _ := r.RWProbe.Run(ctx, cfg)
        allChecks = append(allChecks, rwChecks...)
        snapshot.RWProbe = rw
    }

    endedAt := time.Now()
    return r.Analyzer.Analyze(ctx, analyzers.AnalyzeInput{
        Config:    cfg,
        Snapshot:  snapshot,
        Checks:    allChecks,
        StartedAt: startedAt,
        EndedAt:   endedAt,
        // Detail 不在此传入，由调用方在 Renderer 层传入
    })
}
```

**`--modules` 处理规则：**
- `milvus` 模块始终启用，不受 `opts.Modules` 控制
- `moduleEnabled(opts.Modules, "k8s")` 实现：若 `opts.Modules` 为空（用户未传 `--modules`），返回 true；否则检查 "k8s" 是否在列表中
- 若 `probe` 模块被关闭，`BusinessReadProbe` 和 `RWProbe` 均返回 `skip` 状态

---

## 14. 错误模型设计

### 14.1 错误分类

```go
type ErrorCode string

const (
    ErrCodeConfigInvalid    ErrorCode = "CONFIG_INVALID"
    ErrCodeMilvusConnect    ErrorCode = "MILVUS_CONNECT_ERROR"
    ErrCodeMilvusCollect    ErrorCode = "MILVUS_COLLECT_ERROR"
    ErrCodeK8sCollect       ErrorCode = "K8S_COLLECT_ERROR"
    ErrCodeProbeRead        ErrorCode = "PROBE_READ_ERROR"
    ErrCodeProbeRW          ErrorCode = "PROBE_RW_ERROR"
    ErrCodeRender           ErrorCode = "RENDER_ERROR"
    ErrCodeRuntime          ErrorCode = "RUNTIME_ERROR"
)
```

### 14.2 结构化错误

```go
type AppError struct {
    Code      ErrorCode `json:"code"`
    Message   string    `json:"message"`
    Cause     error     `json:"-"`
    Retriable bool      `json:"retriable"`
}

func (e *AppError) Error() string
func (e *AppError) Unwrap() error
```

### 14.3 退出码映射

```go
type ExitCodeMapper interface {
    FromAnalysis(result *model.AnalysisResult) int
    FromError(err error) int
}
```

映射原则（与 spec §19 严格对应）：
- PASS → 0
- WARN → 1
- FAIL（巡检结论）→ 2
- ConfigError（ErrCodeConfigInvalid）→ 3
- RuntimeError（ErrCodeRuntime、panic recover）→ 4

**注意：** `ErrCodeMilvusConnect` 在连接对象创建阶段失败映射到 4（工具无法运行）；在连接建立后 ping/version 调用失败，则转化为 CheckResult FAIL，最终映射到 2。

---

## 15. 模块间依赖关系约束

建议强制遵守以下依赖方向：

```text
cmd -> cli -> (config, collectors, probes, analyzers, render, model)
collectors -> platform, model
probes -> platform, model
analyzers -> model
render -> model          (Detail bool 在 render 层消费，不向上透传)
config -> model
model -> 无依赖
platform -> 无内部依赖
```

禁止：
- `render` 反向依赖 `collectors`
- `model` 依赖具体 SDK
- `analyzers` 直接调用 K8s 或 Milvus client
- `analyzers` 依赖 `Detail bool`（分析结果必须完整，不受 Detail 影响）

---

## 16. 首版接口与 spec 的映射关系

| spec 能力 | 模块 | 核心接口 |
|---|---|---|
| `check` | `cli` | `CheckRunner.Run` |
| `validate` | `cli/config` | `ValidateRunner.Run` / `Validator.Validate` |
| `version` | `cmd/cli` | `VersionProvider` |
| 版本识别 & ArchProfile | `model` | `DetectArchProfile` |
| Milvus 基础信息 | `collectors/milvus` | `CollectClusterInfo` |
| inventory 盘点 | `collectors/milvus` | `CollectInventory` |
| Pod/Service 状态（版本感知） | `collectors/k8s` | `CollectStatus(archProfile)` |
| Business Read Probe | `probes` | `BusinessReadProbe.Run` |
| RW Probe（含残留清理） | `probes` | `RWProbe.Run` |
| PASS/WARN/FAIL 分析 | `analyzers` | `Analyzer.Analyze` |
| confidence 计算 | `analyzers` | `SummaryBuilder.ComputeConfidence` |
| standby 计算（含合法组合校验） | `analyzers` | `StandbyAnalyzer.ComputeStandby` |
| text/json 输出（含 Detail） | `render` | `Renderer.Render(result, RenderOptions)` |

---

## 17. 首版建议文件级拆分

```text
internal/
├── cli/
│   ├── check_runner.go
│   ├── validate_runner.go
│   └── exit_code.go
├── config/
│   ├── loader.go
│   ├── validator.go
│   ├── defaults.go
│   └── override.go
├── model/
│   ├── config.go
│   ├── inventory.go
│   ├── probe.go
│   ├── result.go
│   ├── enums.go          # 含 MilvusArchProfile、MetricsUnavailableReason
│   └── error.go
├── platform/
│   ├── milvus_client.go
│   ├── k8s_client.go
│   ├── clock.go
│   └── id.go
├── collectors/
│   ├── milvus/
│   │   ├── collector.go
│   │   ├── cluster_info.go
│   │   └── inventory.go
│   └── k8s/
│       ├── collector.go
│       ├── pod_status.go
│       └── arch_resolver.go  # 版本感知的组件角色解析
├── probes/
│   ├── business_read.go
│   ├── rw.go
│   └── generator.go
├── analyzers/
│   ├── analyzer.go
│   ├── connectivity.go
│   ├── inventory.go
│   ├── k8s.go
│   ├── probe.go
│   ├── standby.go
│   └── summary.go
└── render/
    ├── factory.go
    ├── options.go     # RenderOptions 定义
    ├── text.go
    └── json.go
```

---

## 18. 关键实现建议

### 18.1 `check` 中的 `--modules`
- `milvus` 为必选，不允许关闭
- `k8s`、`probe` 可按模块开关
- 若 `probe` 被关闭，两个 probe 均输出 `skip`，对应 standby 判定遵循 `require_probe_for_standby` 配置

### 18.2 `--database` / `--collection` scope 传递
建议统一封装为 scope，在以下三个地方共用：
- `MilvusCollector.CollectInventory`
- `BusinessReadProbe.Run`
- `K8sCollector`（不适用，K8s 采集不受 collection 范围影响）
- analyzer 摘要构建

避免每层各自写一遍过滤逻辑。

### 18.3 ArchProfile 传递链
`ArchProfile` 在 `CollectClusterInfo` 阶段确定，之后作为显式参数传递给：
- `K8sCollector.CollectStatus(archProfile)`
- `K8sAnalyzer.Analyze(status, archProfile, rules)`

不建议通过全局变量或 context 隐式传递，保持调用链清晰可测。

---

## 19. 接口设计结论

基于 spec v0.8，首版 `milvus-health` 最合适的工程拆分是：

1. **CLI 编排层**：`check/validate/version` 三个命令入口
2. **配置层**：加载、默认值、校验、覆盖
3. **平台适配层**：Milvus/K8s client 抽象（无状态 UseDatabase）
4. **采集层**：Milvus inventory 与 K8s 状态采集（K8s 采集感知 ArchProfile）
5. **探测层**：Business Read Probe 与 RW Probe（含残留清理、search 向量构造）
6. **分析层**：统一规则判定、standby/confidence 计算、退出码映射
7. **渲染层**：text/json 严格输出契约（Detail 在此消费）
8. **模型层**：统一数据结构与错误模型（含 MilvusArchProfile）

---

## 20. 下一步最值得继续补的两份文档

按开发顺序，建议紧接着补这两份：

1. **开发任务拆分文档**  
   把上述模块进一步拆成 Iteration 1 / 2 / 3 的任务卡。

2. **接口 Mock 与验收用例文档**  
   明确每个接口在单元测试里如何 mock，以及首版最小验收样例（覆盖 2.4.x 和 2.6.x 两个版本场景）。
