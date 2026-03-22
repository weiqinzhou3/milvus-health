# milvus-health 接口设计文档 v0.7

基于规格文档 `milvus-health Specification v1.2` 进行模块拆分与接口设计。

> **变更说明（v0.6 → v0.7）**
>
> 本版本基于生产交付场景修订 K8s 资源采集设计：
>
> 1. `K8sConfig` 新增 `ResourceUsageSource`，明确动态 usage 采集是可配置能力，不再把 `metrics.k8s.io` 视为硬依赖
> 2. `K8sClient.ListPodMetrics` 语义增加“仅在 source 允许时调用”的约束；collector 必须区分 `disabled`、`metrics-server not found`、`insufficient permissions`
> 3. `K8sStatus` / `PodStatus` 保持静态规格始终可采集，动态 usage 与 ratio 可降级为 unknown/null
> 4. 明确“直接抓取 Milvus 组件 metrics 端口”不是 v0.7 首版通用 K8s usage 数据源，仅作为后续增强候选

---

## 1. 文档目标

将 spec 转换为开发视角的模块边界、接口定义、数据流、错误模型与扩展点约束。默认语言 Go。

---

## 2. 设计原则

1. **命令层与业务层分离**：CLI 只负责参数解析、执行编排、输出与退出码映射
2. **采集、探测、分析、渲染分层**
3. **统一事实模型**：所有采集结果归并为 `MetadataSnapshot`、`CheckResult`、`AnalysisResult`
4. **stdout / stderr 严格分离**
5. **可测试、可替换、可扩展**：所有外部依赖通过接口注入
6. **渲染决策不进入分析层**：`Detail bool` 属于渲染决策；分析层 `Checks` 列表始终完整，渲染层通过 shadow struct 省略，不修改原始数据
7. **platform 包无内部依赖**：不导入 `model` 包，类型映射由 collectors 层完成
8. **collector 不做健康判定**：`arch_profile=unknown` 时，K8s collector 产出空 CheckResult，K8sAnalyzer 统一产出 skip
9. **动态 usage 源可配置**：K8s 静态事实（Pod/Service/requests/limits）始终来自 apiserver；动态 CPU/Memory usage 是独立能力，可按 source 启用或关闭

---

## 3. 模块拆分

| 模块 | 职责 |
|---|---|
| `cmd/` | 程序入口，Cobra 命令注册 |
| `internal/cli/` | CLI 参数模型、orchestrator、退出码映射 |
| `internal/config/` | YAML 加载、默认值、静态校验（Validator）、CLI 参数校验（OptionsValidator）、CLI 覆盖 |
| `internal/model/` | 全部领域模型（含 ArchProfile、CriticalType、IgnoredReason、PodRole、ProbeAction） |
| `internal/collectors/milvus` | Milvus 采集、ArchProfile 识别、binlog size 获取 |
| `internal/collectors/k8s` | K8s 采集，版本感知组件列表；unknown 时产出空 CheckResult |
| `internal/probes/` | Business Read Probe、RW Probe（含预检清理） |
| `internal/analyzers/` | 规则判定、standby、confidence、exit code；unknown 时产出 skip CheckResult |
| `internal/render/` | text / json 输出，通过 shadow struct 实现 checks 省略 |
| `internal/platform/` | 外部 client 抽象，不导入 model 包 |

---

## 4. 总体调用链

```text
CLI -> OptionsValidator           (CLI 参数校验，最先执行)
    -> ConfigLoader
    -> DefaultApplier
    -> OverrideApplier
    -> ConfigValidator            (配置文件静态校验)
    -> CheckRunner
        -> [补齐 disabled 模块的 skip CheckResult（在任何 goto 之前）]
        -> MilvusCollector        (ArchProfile 在此确定)
        -> K8sCollector           (接受 ArchProfile；unknown 时产出空 CheckResult)
        -> BusinessReadProbe
        -> RWProbe
        -> Analyzer               (K8sAnalyzer 在 unknown 时产出 skip CheckResult)
        -> Renderer               (Detail bool 在此消费；shadow struct 省略 checks)
        -> ExitCodeMapper
```

---

## 5. `internal/model` 模型设计

### 5.1 基础枚举

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

// MilvusArchProfile 枚举值与 spec §3.2 全局统一
type MilvusArchProfile string
const (
    ArchProfileV24     MilvusArchProfile = "v2.4"
    ArchProfileV26     MilvusArchProfile = "v2.6"
    ArchProfileUnknown MilvusArchProfile = "unknown"
)

// DetectArchProfile 使用 semver 比较，不硬编码具体版本
func DetectArchProfile(version string) MilvusArchProfile {
    major, minor, err := parseMajorMinor(version)
    if err != nil {
        return ArchProfileUnknown
    }
    switch {
    case major == 2 && minor <= 5:
        return ArchProfileV24
    case major == 2 && minor >= 6:
        return ArchProfileV26
    case major > 2:
        return ArchProfileV26
    default:
        return ArchProfileUnknown
    }
}

// parseMajorMinor 解析 "2.4.7" -> (2, 4, nil)
func parseMajorMinor(version string) (major, minor int, err error)

// MetricsUnavailableReason model 层枚举
type MetricsUnavailableReason string
const (
    MetricsUnavailableReasonNone             MetricsUnavailableReason = ""
    MetricsUnavailableReasonNotFound         MetricsUnavailableReason = "metrics-server not found"
    MetricsUnavailableReasonPermissionDenied MetricsUnavailableReason = "insufficient permissions"
    MetricsUnavailableReasonUnknown          MetricsUnavailableReason = "unknown"
)

// CriticalType 描述 Pod 在 Milvus 集群中的关键等级
type CriticalType string
const (
    CriticalTypeCoordinator CriticalType = "coordinator"
    CriticalTypeDataPlane   CriticalType = "data_plane"
    CriticalTypeDependency  CriticalType = "dependency"
    CriticalTypeNonCritical CriticalType = "non_critical"
)

// IgnoredReason 描述 Pod 被分析层忽略健康判定的原因（与 spec §12.4 对齐）
// Ignored=true 的 Pod 不参与重启计数、ratio WARN 计算、关键组件 FAIL 判定
// 注意：unmatched-role 仅在 arch_profile != unknown 时触发，见 shouldIgnore 实现
type IgnoredReason string
const (
    IgnoredReasonNone           IgnoredReason = ""
    IgnoredReasonPhaseSucceeded IgnoredReason = "phase-succeeded"  // Pod.Phase == "Succeeded"
    IgnoredReasonDebugPod       IgnoredReason = "debug-pod"        // 名称含 -debug 或 label purpose=debug
    IgnoredReasonUnmatchedRole  IgnoredReason = "unmatched-role"   // 角色无法映射到已知组件（arch!=unknown 时）
)

// PodRole 描述 Pod 被识别的 Milvus 组件角色
type PodRole string
const (
    PodRoleProxy         PodRole = "proxy"
    PodRoleRootCoord     PodRole = "rootcoord"
    PodRoleDataCoord     PodRole = "datacoord"
    PodRoleQueryCoord    PodRole = "querycoord"
    PodRoleIndexCoord    PodRole = "indexcoord"
    PodRoleMixCoord      PodRole = "mixcoord"
    PodRoleDataNode      PodRole = "datanode"
    PodRoleQueryNode     PodRole = "querynode"
    PodRoleIndexNode     PodRole = "indexnode"
    PodRoleStreamingNode PodRole = "streaming"
    PodRoleUnknown       PodRole = "unknown"
)

// ProbeAction 描述 Business Read Probe 对单个 target 实际执行的操作
// 与 spec §10.3 状态机对齐
type ProbeAction string
const (
    ProbeActionDescribeFailed ProbeAction = "describe-failed"
    ProbeActionQuery          ProbeAction = "query"
    ProbeActionQueryAndSearch ProbeAction = "query+search"
)
```

### 5.2 配置模型

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
    URI      string `yaml:"uri"`
    Username string `yaml:"username"`
    Password string `yaml:"password"`
    Token    string `yaml:"token"`
}

type K8sConfig struct {
    Namespace           string `yaml:"namespace"`
    Kubeconfig          string `yaml:"kubeconfig"`
    ResourceUsageSource string `yaml:"resource_usage.source"` // auto | metrics-api | disabled
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
    AnnsField    string   `yaml:"anns_field"`
    TopK         int      `yaml:"topk"`
    OutputFields []string `yaml:"output_fields"`
}

type RWProbeConfig struct {
    Enabled            bool   `yaml:"enabled"`
    TestDatabasePrefix string `yaml:"test_database_prefix"`
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

// K8sResourceUsageSource 为动态 CPU/Memory usage 的来源策略
type K8sResourceUsageSource string
const (
    K8sResourceUsageSourceAuto       K8sResourceUsageSource = "auto"
    K8sResourceUsageSourceMetricsAPI K8sResourceUsageSource = "metrics-api"
    K8sResourceUsageSourceDisabled   K8sResourceUsageSource = "disabled"
)

type OutputConfig struct {
    Format OutputFormat `yaml:"format"`
    Detail bool         `yaml:"detail"`
}
```

### 5.3 CLI 运行参数模型

```go
type CheckOptions struct {
    ConfigPath string
    Format     OutputFormat
    Profile    string
    Verbose    bool
    TimeoutSec int
    Cleanup    *bool
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

### 5.4 事实模型

```go
type MetadataSnapshot struct {
    ClusterInfo       ClusterInfo
    MilvusInventory   MilvusInventory
    K8sStatus         K8sStatus
    BusinessReadProbe BusinessReadProbeResult
    RWProbe           RWProbeResult
}

type ClusterInfo struct {
    Name          string            `json:"name"`
    MilvusURI     string            `json:"milvus_uri"`
    Namespace     string            `json:"namespace"`
    MilvusVersion string            `json:"milvus_version"`
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
    TotalRowCount             int64               `json:"total_row_count"`
    // nil 时 JSON 输出 null——不得加 omitempty
    TotalBinlogSizeBytes      *int64              `json:"total_binlog_size_bytes"`
    UnindexedVectorFieldCount int                 `json:"unindexed_vector_field_count"`
}

// DatabaseInventory 描述单个 database 及其下的 collection 列表
type DatabaseInventory struct {
    Name        string                `json:"name"`
    Collections []CollectionInventory `json:"collections"`
}

type CollectionInventory struct {
    Database                  string        `json:"database"`
    Name                      string        `json:"name"`
    CollectionID              int64         `json:"collection_id"`
    RowCount                  int64         `json:"row_count"`
    // nil 时 JSON 输出 null——不得加 omitempty
    BinlogSizeBytes           *int64        `json:"binlog_size_bytes"`
    BinlogSizeAvailable       bool          `json:"binlog_size_available"`
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
    Namespace                 string                   `json:"namespace"`
    ArchProfile               MilvusArchProfile        `json:"arch_profile"`
    ResourceUsageSource       K8sResourceUsageSource   `json:"resource_usage_source"`
    TotalPodCount             int                      `json:"total_pod_count"`
    ReadyPodCount             int                      `json:"ready_pod_count"`
    NotReadyPodCount          int                      `json:"not_ready_pod_count"`
    ResourceUsageAvailable    bool                     `json:"resource_usage_available"`
    ResourceUsagePartial      bool                     `json:"resource_usage_partial,omitempty"`
    MetricsAvailablePodCount  int                      `json:"metrics_available_pod_count,omitempty"`
    ResourceUnavailableReason MetricsUnavailableReason `json:"resource_unavailable_reason,omitempty"`
    Pods                      []PodStatus              `json:"pods"`
    Services                  []ServiceInfo            `json:"services"`
}

type PodStatus struct {
    Name          string        `json:"name"`
    Role          PodRole       `json:"role"`
    Phase         string        `json:"phase"`
    Ready         bool          `json:"ready"`
    RestartCount  int32         `json:"restart_count"`
    // 容器级 waiting reason；用于识别 CrashLoopBackOff（空数组表示无 waiting reason）
    ContainerWaitingReasons []string `json:"container_waiting_reasons"`
    IsCritical    bool          `json:"is_critical"`
    CriticalType  CriticalType  `json:"critical_type,omitempty"`
    // Ignored=true 时，分析层跳过该 Pod 的重启计数、ratio WARN、关键组件 FAIL 判定
    Ignored       bool          `json:"ignored,omitempty"`
    IgnoredReason IgnoredReason `json:"ignored_reason,omitempty"`
    CPUUsage      string        `json:"cpu_usage,omitempty"`
    MemoryUsage   string        `json:"memory_usage,omitempty"`
    CPURequest    string        `json:"cpu_request,omitempty"`
    CPULimit      string        `json:"cpu_limit,omitempty"`
    MemoryRequest string        `json:"memory_request,omitempty"`
    MemoryLimit   string        `json:"memory_limit,omitempty"`
    // usage/limit 占比（触发 WARN 判断）；nil 时 JSON 输出 null——不得加 omitempty
    CPULimitRatio    *float64 `json:"cpu_limit_ratio"`
    MemoryLimitRatio *float64 `json:"memory_limit_ratio"`
    // usage/request 占比（仅展示）；nil 时 JSON 输出 null——不得加 omitempty
    CPURequestRatio    *float64 `json:"cpu_request_ratio"`
    MemoryRequestRatio *float64 `json:"memory_request_ratio"`
}

type ServiceInfo struct {
    Name  string   `json:"name"`
    Type  string   `json:"type"`
    Ports []string `json:"ports"`
}
```

### 5.5 CheckResult 定义

```go
// CheckResult 是单个检查项的结构化结果，与 spec §8.2 对应
// category 取值：milvus / k8s / probe
type CheckResult struct {
    Category   string      `json:"category"`
    Name       string      `json:"name"`
    Target     string      `json:"target,omitempty"`
    Status     CheckStatus `json:"status"`
    Severity   string      `json:"severity"`
    Message    string      `json:"message"`
    Actual     any         `json:"actual,omitempty"`
    Expected   any         `json:"expected,omitempty"`
    DurationMS int64       `json:"duration_ms,omitempty"`
}
```

### 5.6 Probe 结果模型

```go
type BusinessReadProbeResult struct {
    Status            CheckStatus                `json:"status"`
    ConfiguredTargets int                        `json:"configured_targets"`
    SuccessfulTargets int                        `json:"successful_targets"`
    MinSuccessTargets int                        `json:"min_success_targets"`
    Message           string                     `json:"message"`
    Targets           []BusinessReadTargetResult `json:"targets,omitempty"`
}

type BusinessReadTargetResult struct {
    Database   string      `json:"database"`
    Collection string      `json:"collection"`
    // Action 与 spec §10.3 状态机对齐
    // DescribeCollection 失败 → ProbeActionDescribeFailed
    // 未配置 anns_field → ProbeActionQuery
    // 配置了 anns_field → ProbeActionQueryAndSearch（无论 search 成功与否）
    Action     ProbeAction `json:"action"`
    Success    bool        `json:"success"`
    DurationMS int64       `json:"duration_ms"`
    Error      string      `json:"error,omitempty"`
    RowCount   int64       `json:"row_count,omitempty"`
}

type RWProbeResult struct {
    Status          CheckStatus       `json:"status"`
    Enabled         bool              `json:"enabled"`
    TestDatabase    string            `json:"test_database,omitempty"`
    TestCollection  string            `json:"test_collection,omitempty"`
    InsertRows      int               `json:"insert_rows,omitempty"`
    VectorDim       int               `json:"vector_dim,omitempty"`
    CleanupEnabled  bool              `json:"cleanup_enabled,omitempty"`
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

### 5.7 分析结果模型

```go
type AnalysisSummary struct {
    DatabaseCount             int    `json:"database_count"`
    CollectionCount           int    `json:"collection_count"`
    LoadedCollectionCount     int    `json:"loaded_collection_count"`
    UnloadedCollectionCount   int    `json:"unloaded_collection_count"`
    TotalRowCount             int64  `json:"total_row_count"`
    // nil 时 JSON 输出 null——不得加 omitempty
    TotalBinlogSizeBytes      *int64 `json:"total_binlog_size_bytes"`
    UnindexedVectorFieldCount int    `json:"unindexed_vector_field_count"`
    TotalPodCount             int    `json:"total_pod_count"`
    ReadyPodCount             int    `json:"ready_pod_count"`
    NotReadyPodCount          int    `json:"not_ready_pod_count"`
    MetricsAvailablePodCount  int    `json:"metrics_available_pod_count"`
    FailCount                 int    `json:"fail_count"`
    WarnCount                 int    `json:"warn_count"`
}

// ProbeOutputView 用于 AnalysisResult.Probes 字段的 JSON 序列化
type ProbeOutputView struct {
    BusinessRead BusinessReadProbeResult `json:"business_read"`
    RW           RWProbeResult           `json:"rw"`
}

type ClusterOutputView struct {
    Name          string            `json:"name"`
    MilvusURI     string            `json:"milvus_uri"`
    Namespace     string            `json:"namespace"`
    MilvusVersion string            `json:"milvus_version"`
    ArchProfile   MilvusArchProfile `json:"arch_profile"`
    MQType        string            `json:"mq_type"`
}

type AnalysisResult struct {
    Cluster    ClusterOutputView `json:"cluster"`
    Result     FinalResult       `json:"result"`
    Standby    bool              `json:"standby"`
    Confidence ConfidenceLevel   `json:"confidence"`
    ExitCode   int               `json:"exit_code"`
    ElapsedMS  int64             `json:"elapsed_ms"`
    Summary    AnalysisSummary   `json:"summary"`
    Probes     ProbeOutputView   `json:"probes"`
    Warnings   []string          `json:"warnings"`
    Failures   []string          `json:"failures"`
    // 分析层始终保留完整事实；渲染层通过 shadow struct 控制展示粒度——不得加 omitempty
    Checks     []CheckResult     `json:"checks"`
}
```

---

## 6. `internal/config` 接口设计

```go
type Loader interface { Load(path string) (*model.Config, error) }

// Validator 负责配置文件的静态校验，与 CLI 参数无关
type Validator interface {
    Validate(cfg *model.Config) (ValidationReport, error)
}

// OptionsValidator 负责 CLI 运行参数的合法性校验，不依赖配置文件内容
// 在 ConfigLoader 之前执行，validate 命令不需要此接口
type OptionsValidator interface {
    ValidateCheckOptions(opts model.CheckOptions) error
}

type DefaultApplier interface { Apply(cfg *model.Config) }
type OverrideApplier interface { ApplyCheckOverrides(cfg *model.Config, opts model.CheckOptions) error }

type ValidationReport struct {
    Warnings []string // 非阻断性警告，由 CLI 层决定是否打印到 stderr
}
```

**校验职责拆分：**

`Validator.Validate(cfg)` 只负责**配置文件静态校验**：
1. `cluster.name` 非空
2. `cluster.milvus.uri` 非空且不含 `://`（scheme 前缀 → CONFIG_ERROR）
3. `output.format` 只能是 `text` / `json`
4. `probe.read.min_success_targets >= 0`
5. `probe.rw.insert_rows > 0`（启用时）
6. `probe.rw.vector_dim > 0`（启用时）
7. password + token 同时存在 → 写入 `ValidationReport.Warnings`（不阻断）
8. `rules.resource_warn_ratio` 在 `(0, 1]`
9. `rules.pod_restart_warn < pod_restart_fail`（两者都配置时）

`OptionsValidator.ValidateCheckOptions(opts)` 负责 **CLI 运行参数校验**：
10. `--modules` 中出现未知名称（非 `k8s`/`probe`）→ CONFIG_ERROR，exit 3
11. `--format` 值只能是 `text` / `json`
12. 其他与 check 命令运行参数相关、不属于 YAML 配置的检查

---

## 7. `internal/platform` 接口设计

**platform 包不导入 model 包。**

### 7.1 Milvus 客户端抽象

```go
type CollectionSchema struct {
    CollectionID   int64
    CollectionName string
    Fields         []FieldSchema
}

type FieldSchema struct {
    Name      string
    FieldType string
    Dim       int
    IsPrimary bool
}

// CollectionStats 只含 RowCount
// binlog size 来自 GetMetrics，不在此结构体中——不得添加 BinlogSizeBytes 字段
type CollectionStats struct {
    RowCount int64
}

type IndexInfo struct {
    FieldName  string
    IndexName  string
    IndexType  string
    MetricType string
}

type MilvusClient interface {
    GetVersion(ctx context.Context) (string, error)
    ListDatabases(ctx context.Context) ([]string, error)
    DatabaseExists(ctx context.Context, db string) (bool, error)
    ListCollections(ctx context.Context, db string) ([]string, error)
    DescribeCollection(ctx context.Context, db, collection string) (*CollectionSchema, error)
    GetCollectionStats(ctx context.Context, db, collection string) (*CollectionStats, error)
    GetLoadState(ctx context.Context, db, collection string) (string, error)
    ListIndexes(ctx context.Context, db, collection string) ([]IndexInfo, error)
    // GetMetrics 返回 JSON 字符串，由 binlog_metrics.go 解析 DataCoordQuotaMetrics
    GetMetrics(ctx context.Context, metricType string) (string, error)
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

### 7.2 K8s 客户端抽象

```go
// PodInfo platform 内部类型，ListPods 一并返回 requests/limits（来自 PodSpec）
// ContainerWaitingReasons 为容器级 waiting reason 集合，至少需要覆盖 CrashLoopBackOff 判定
type PodInfo struct {
    Name                    string
    Labels                  map[string]string
    Phase                   string
    Ready                   bool
    RestartCount            int32
    ContainerWaitingReasons []string // e.g. ["CrashLoopBackOff"]
    CPURequest              string
    CPULimit                string
    MemoryRequest           string
    MemoryLimit             string
}

type PodMetric struct {
    PodName     string
    CPUUsage    string
    MemoryUsage string
}

// PlatformMetricsResult platform 内部类型，不依赖 model
type PlatformMetricsResult struct {
    Metrics           []PodMetric
    Available         bool
    UnavailableReason string // 由 collectors 层映射为 model.MetricsUnavailableReason
}

type K8sClient interface {
    ListPods(ctx context.Context, namespace string) ([]PodInfo, error)
    ListServices(ctx context.Context, namespace string) ([]ServiceInfo, error)
    // ListPodMetrics 仅在 ResourceUsageSource 为 auto / metrics-api 时调用。
    // 返回语义：
    //   metrics-server 不存在（404/discovery 失败）→ (PlatformMetricsResult{Available:false, UnavailableReason:"metrics-server not found"}, nil)
    //   权限不足（403）→ (PlatformMetricsResult{Available:false, UnavailableReason:"insufficient permissions"}, nil)
    //   K8s API server 不可达 → (PlatformMetricsResult{}, error)
    //   成功 → (PlatformMetricsResult{Available:true, Metrics:[...]}, nil)
    // 注意：v0.7 首版不通过 Milvus 组件 metrics 端口实现通用 Pod usage；该思路仅作为后续增强候选。
    ListPodMetrics(ctx context.Context, namespace string) (PlatformMetricsResult, error)
}

type ServiceInfo struct {
    Name  string
    Type  string
    Ports []string
}
```

---

## 8. `internal/collectors` 接口设计

### 8.1 MilvusCollector

```go
type MilvusCollector interface {
    CollectClusterInfo(ctx context.Context, cfg *model.Config) (model.ClusterInfo, []model.CheckResult, error)
    CollectInventory(ctx context.Context, cfg *model.Config, scope CollectScope) (model.MilvusInventory, []model.CheckResult, error)
}

type CollectScope struct {
    Database   string
    Collection string
}
```

**binlog size 获取流程：**

```go
// fetchBinlogSizes 调用 GetMetrics("system_info")，解析 DataCoordQuotaMetrics
// 返回: collectionID->bytes 映射 + TotalBinlogSize + error
func (c *DefaultMilvusCollector) fetchBinlogSizes(ctx context.Context) (map[int64]int64, int64, error)

// 失败处理：
// - fetchBinlogSizes 失败 -> 所有 collection BinlogSizeAvailable=false，BinlogSizeBytes=nil，TotalBinlogSizeBytes=nil
// - 添加 WARN CheckResult: "failed to fetch binlog size via GetMetrics: <e>"
// - 不阻断整体 inventory 采集

// buildCollectionInventory 用 collectionID 查 binlogSizes map
// 找到 -> BinlogSizeBytes=&size, BinlogSizeAvailable=true
// 未找到 -> BinlogSizeBytes=nil, BinlogSizeAvailable=false
```

### 8.2 K8sCollector

```go
type K8sCollector interface {
    // arch_profile=unknown 时：
    //   collector 照常枚举 Pod/Service，填充 K8sStatus.Pods 基础事实字段
    //   返回空 CheckResult 列表（不产出任何 FAIL/WARN）
    //   K8sAnalyzer 统一产出 skip CheckResult
    //
    // ResourceUsageSource 语义：
    //   disabled    -> 不调用 ListPodMetrics；仅采集静态 request/limit 与基础状态
    //   auto        -> 尝试 ListPodMetrics；not found / forbidden 时结构化降级
    //   metrics-api -> 明确要求 ListPodMetrics；not found / forbidden 时仍结构化降级，不直接 FAIL
    CollectStatus(ctx context.Context, cfg *model.Config, archProfile model.MilvusArchProfile) (model.K8sStatus, []model.CheckResult, error)
}
```

**关键内部方法：**

```go
// normalizePodRole 按 ArchProfile 映射 Pod 角色
// arch_profile=unknown 时返回 (PodRoleUnknown, CriticalTypeNonCritical, false)
func (c *DefaultK8sCollector) normalizePodRole(podInfo platform.PodInfo, arch model.MilvusArchProfile) (model.PodRole, model.CriticalType, bool)

// shouldIgnore 判断 Pod 是否应被忽略
// 需要 arch 参数以防止 arch_profile=unknown 时误触发 unmatched-role
//
// 判定规则：
//   Phase=="Succeeded"                                    -> (true, IgnoredReasonPhaseSucceeded)
//   名称含 -debug 或 label purpose=debug                  -> (true, IgnoredReasonDebugPod)
//   role==PodRoleUnknown && arch!=ArchProfileUnknown      -> (true, IgnoredReasonUnmatchedRole)
//   role==PodRoleUnknown && arch==ArchProfileUnknown      -> (false, IgnoredReasonNone)  ← 不触发
//   其他                                                  -> (false, IgnoredReasonNone)
func (c *DefaultK8sCollector) shouldIgnore(podInfo platform.PodInfo, role model.PodRole, arch model.MilvusArchProfile) (bool, model.IgnoredReason)

// mergeMetrics 合并 metrics，计算 ratio 字段
// ratio 字段为 *float64，nil 时 JSON 输出 null（无 omitempty）
// source=disabled 时不得调用本函数；collector 直接返回 ResourceUsageAvailable=false 且不产出 source-missing WARN
func (c *DefaultK8sCollector) mergeMetrics(
    pods []model.PodStatus,
    result platform.PlatformMetricsResult,
) ([]model.PodStatus, model.MetricsUnavailableReason, int)

func mapMetricsReason(raw string) model.MetricsUnavailableReason

func (c *DefaultK8sCollector) checkLegacyCoordinators(pods []model.PodStatus, arch model.MilvusArchProfile) []model.CheckResult

// calculateRatio 安全计算（分母为 0 或字段为空 -> nil）
func calculateRatio(usage, limit string) *float64
```

---

## 9. `internal/probes` 接口设计

### 9.1 BusinessReadProbe

```go
type BusinessReadProbe interface {
    Run(ctx context.Context, cfg *model.Config, scope ProbeScope) (model.BusinessReadProbeResult, []model.CheckResult, error)
}

type ProbeScope struct {
    Database   string
    Collection string
}
```

**search 向量构造（[-1.0, 1.0] 值域，固定种子 42）：**

```go
func buildQueryVector(dim int) []float32 {
    rng := rand.New(rand.NewSource(42))
    vec := make([]float32, dim)
    for i := range vec {
        vec[i] = rng.Float32()*2 - 1
    }
    return vec
}
```

**Action 字段状态机（与 spec §10.3 对齐）：**

```
DescribeCollection 失败 -> Action=ProbeActionDescribeFailed, Success=false
DescribeCollection 成功，未配置 anns_field:
  -> Action=ProbeActionQuery
  -> query 成功 -> Success=true
  -> query 失败 -> Success=false
DescribeCollection 成功，配置了 anns_field:
  -> Action=ProbeActionQueryAndSearch
  -> query 失败 -> Success=false（search 不执行）
  -> query 成功，anns_field 维度无法获取 -> Success=false，Error 说明原因
  -> query+search 均成功 -> Success=true
  -> search 失败 -> Success=false（禁止静默降级为 query 成功）
```

**scope 过滤：** 不匹配的 target 记为 skip，不计入 `configured_targets`。全部过滤 -> probe 状态 `skip`。

### 9.2 RWProbe

```go
type RWProbe interface {
    Run(ctx context.Context, cfg *model.Config) (model.RWProbeResult, []model.CheckResult, error)
}

// cleanupStaleTestDatabases 清理同前缀遗留 database
// strings.HasPrefix(dbName, prefix+"_") 匹配，避免误清理业务 database
// 失败 -> error -> Run 标记 RWProbe=FAIL，不执行后续步骤
func (p *DefaultRWProbe) cleanupStaleTestDatabases(ctx context.Context, prefix string) error
```

---

## 10. `internal/analyzers` 接口设计

```go
type Analyzer interface {
    Analyze(ctx context.Context, input AnalyzeInput) (*model.AnalysisResult, error)
}

// AnalyzeInput 不含 Detail
type AnalyzeInput struct {
    Config    *model.Config
    Snapshot  model.MetadataSnapshot
    Checks    []model.CheckResult
    StartedAt time.Time
    EndedAt   time.Time
}
```

### 10.1 子分析器接口

```go
type ConnectivityAnalyzer interface {
    Analyze(info model.ClusterInfo) []model.CheckResult
}

type InventoryAnalyzer interface {
    Analyze(inv model.MilvusInventory) []model.CheckResult
}

type K8sAnalyzer interface {
    // arch_profile=unknown 时必须产出一条 status=skip 的 CheckResult
    // 不依赖 collector 产出的 CheckResult（unknown 时 collector 产出空列表）
    Analyze(status model.K8sStatus, archProfile model.MilvusArchProfile, rules model.RulesConfig) []model.CheckResult
}

type ProbeAnalyzer interface {
    Analyze(br model.BusinessReadProbeResult, rw model.RWProbeResult, cfg *model.Config) []model.CheckResult
}

type StandbyAnalyzer interface {
    ComputeStandby(
        result model.FinalResult,
        br model.BusinessReadProbeResult,
        rw model.RWProbeResult,
        k8s model.K8sStatus,
        cfg *model.Config,
    ) (standby bool, notes []string)
}

// SummaryOutput 包含 SummaryBuilder.Build 的完整输出
type SummaryOutput struct {
    Result   model.FinalResult
    ExitCode int
    Summary  model.AnalysisSummary
    Warnings []string
    Failures []string
}

type SummaryBuilder interface {
    Build(checks []model.CheckResult, snapshot model.MetadataSnapshot, cfg *model.Config) SummaryOutput
    ComputeFinalResult(checks []model.CheckResult) model.FinalResult
    // arch_profile=unknown 时必须返回 ConfidenceLow
    ComputeConfidence(
        result model.FinalResult,
        br model.BusinessReadProbeResult,
        rw model.RWProbeResult,
        archProfile model.MilvusArchProfile,
    ) model.ConfidenceLevel
}
```

### 10.2 K8sAnalyzer `arch_profile=unknown` 处理伪代码

```go
func (a *DefaultK8sAnalyzer) Analyze(
    status model.K8sStatus,
    archProfile model.MilvusArchProfile,
    rules model.RulesConfig,
) []model.CheckResult {
    if archProfile == model.ArchProfileUnknown {
        return []model.CheckResult{{
            Category: "k8s",
            Name:     "pod_health",
            Status:   model.CheckStatusSkip,
            Severity: "info",
            Message:  "arch_profile unknown, pod health check skipped",
        }}
    }
    var results []model.CheckResult
    for _, pod := range status.Pods {
        if pod.Ignored {
            continue // 跳过 Completed Job、debug Pod、unmatched-role Pod
        }
        // 重启计数、ratio WARN、关键组件 FAIL 判定...
    }
    return results
}
```

### 10.3 关键规则落点

| 规则 | 分析器 |
|---|---|
| Milvus 连接失败即 FAIL | `ConnectivityAnalyzer` |
| collection 未建向量索引 / unloaded => WARN | `InventoryAnalyzer` |
| **`arch_profile=unknown` -> 整体 skip（一条 skip CheckResult）** | `K8sAnalyzer` |
| Pod restart 超 warn 阈值 => WARN（仅非 Ignored Pod） | `K8sAnalyzer` |
| Pod restart 超 fail 阈值 => FAIL（仅非 Ignored Pod） | `K8sAnalyzer` |
| Pod usage/limit ratio 超 resource_warn_ratio => WARN（仅非 Ignored Pod） | `K8sAnalyzer` |
| Pod CrashLoopBackOff => FAIL（仅非 Ignored Pod） | `K8sAnalyzer` |
| v2.6 集群发现 v2.4 旧 coordinator => WARN | `K8sAnalyzer` |
| metrics partial => WARN | `K8sAnalyzer` |
| Probe 判定（search 禁止静默降级） | `ProbeAnalyzer` |
| standby 计算 | `StandbyAnalyzer` |
| PASS/WARN/FAIL + exit code + Warnings/Failures | `SummaryBuilder` |
| confidence 计算（unknown=low） | `SummaryBuilder` |

---

## 11. `internal/render` 接口设计

```go
type RenderOptions struct {
    Detail bool
}

type Renderer interface {
    Render(result *model.AnalysisResult, opts RenderOptions) ([]byte, error)
}

type RendererFactory interface {
    Get(format model.OutputFormat) (Renderer, error)
}
```

**渲染约束：**

- TextRenderer：unknown 字段显示字符串 `unknown`；`arch_profile=unknown` 时 K8s Summary 显示 skip 说明
- JSONRenderer：unknown 字段输出 `null`（不省略）；**checks 通过 shadow struct 控制**：

```go
func (r *JSONRenderer) Render(result *model.AnalysisResult, opts RenderOptions) ([]byte, error) {
    // shadow struct——不修改原始 result
    type out struct {
        Cluster    model.ClusterOutputView `json:"cluster"`
        Result     model.FinalResult       `json:"result"`
        Standby    bool                    `json:"standby"`
        Confidence model.ConfidenceLevel   `json:"confidence"`
        ExitCode   int                     `json:"exit_code"`
        ElapsedMS  int64                   `json:"elapsed_ms"`
        Summary    model.AnalysisSummary   `json:"summary"`
        Probes     model.ProbeOutputView   `json:"probes"`
        Warnings   []string                `json:"warnings"`
        Failures   []string                `json:"failures"`
        Checks     []model.CheckResult     `json:"checks"`
    }
    o := out{
        Cluster:    result.Cluster,
        Result:     result.Result,
        Standby:    result.Standby,
        Confidence: result.Confidence,
        ExitCode:   result.ExitCode,
        ElapsedMS:  result.ElapsedMS,
        Summary:    result.Summary,
        Probes:     result.Probes,
        Warnings:   result.Warnings,
        Failures:   result.Failures,
    }
    if opts.Detail {
        o.Checks = result.Checks
    } else {
        o.Checks = []model.CheckResult{} // 空数组，不省略字段
    }
    return json.MarshalIndent(o, "", "  ")
}
```

---

## 12. `internal/cli` 接口设计

```go
type CheckRunner interface {
    Run(ctx context.Context, opts model.CheckOptions) (*model.AnalysisResult, error)
}

type ValidateRunner interface {
    Run(ctx context.Context, opts model.ValidateOptions) error
}
```

### 12.1 `DefaultCheckRunner` struct

```go
type DefaultCheckRunner struct {
    // OptionsValidator 最先调用，负责 CLI 参数校验，不依赖配置文件
    OptionsValidator config.OptionsValidator

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

### 12.2 `Run` 编排伪代码

**核心原则：disabled 模块的 skip CheckResult 在所有采集步骤之前统一补齐，保证 goto 路径同样可见这些 CheckResult。**

```go
func (r *DefaultCheckRunner) Run(ctx context.Context, opts model.CheckOptions) (*model.AnalysisResult, error) {
    // 1. CLI 参数校验（最先执行，不依赖 cfg，快速失败）
    if err := r.OptionsValidator.ValidateCheckOptions(opts); err != nil {
        return nil, &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
    }

    // 2. 加载配置
    cfg, err := r.Loader.Load(opts.ConfigPath)
    if err != nil {
        return nil, &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
    }

    // 3. 填充默认值
    r.DefaultApplier.Apply(cfg)

    // 4. 应用 CLI 覆盖参数
    if err := r.OverrideApplier.ApplyCheckOverrides(cfg, opts); err != nil {
        return nil, &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
    }

    // 5. 配置文件静态校验
    report, err := r.Validator.Validate(cfg)
    if err != nil {
        return nil, &model.AppError{Code: model.ErrCodeConfigInvalid, Cause: err}
    }
    // 非阻断警告写入 stderr（由 CLI 层通过 logger 实现，不丢弃）
    for _, w := range report.Warnings {
        log.Warn(w)
    }

    startedAt := time.Now()
    var allChecks []model.CheckResult
    snapshot := model.MetadataSnapshot{}

    // 6. 提前补齐被 --modules 禁用模块的 skip CheckResult
    // 必须在 goto analyze 之前完成，以保证即使 Milvus 连接失败，这些 skip 项也出现在结果中
    if !moduleEnabled(opts.Modules, "k8s") || cfg.K8s.Namespace == "" {
        allChecks = append(allChecks, model.CheckResult{
            Name:     "k8s-module",
            Category: "k8s",
            Status:   model.CheckStatusSkip,
            Severity: "info",
            Message:  "k8s module disabled by --modules",
        })
    }
    if !moduleEnabled(opts.Modules, "probe") {
        allChecks = append(allChecks,
            model.CheckResult{
                Name:     "business-read-probe",
                Category: "probe",
                Status:   model.CheckStatusSkip,
                Severity: "info",
                Message:  "probe module disabled by --modules",
            },
            model.CheckResult{
                Name:     "rw-probe",
                Category: "probe",
                Status:   model.CheckStatusSkip,
                Severity: "info",
                Message:  "probe module disabled by --modules",
            },
        )
    }

    // 7. Milvus 采集（始终执行）
    clusterInfo, checks, err := r.MilvusCollector.CollectClusterInfo(ctx, cfg)
    allChecks = append(allChecks, checks...)
    snapshot.ClusterInfo = clusterInfo
    if err != nil {
        goto analyze // checks 已含 connectivity=fail；步骤 6 已补齐 disabled 模块 skip
    }

    {
        inv, invChecks, invErr := r.MilvusCollector.CollectInventory(ctx, cfg, scopeFromOpts(opts))
        allChecks = append(allChecks, invChecks...)
        if invErr == nil {
            snapshot.MilvusInventory = inv
        }
    }

    // 8. K8s 采集（可关闭）
    // arch_profile=unknown 时 collector 产出空 CheckResult，K8sAnalyzer 统一产出 skip
    if moduleEnabled(opts.Modules, "k8s") && cfg.K8s.Namespace != "" {
        ks, ksChecks, _ := r.K8sCollector.CollectStatus(ctx, cfg, clusterInfo.ArchProfile)
        allChecks = append(allChecks, ksChecks...)
        snapshot.K8sStatus = ks
    }
    // else 分支已在步骤 6 补齐，无需重复

    // 9. Probe（可关闭）
    if moduleEnabled(opts.Modules, "probe") {
        br, brChecks, _ := r.ReadProbe.Run(ctx, cfg, probeScopeFromOpts(opts))
        allChecks = append(allChecks, brChecks...)
        snapshot.BusinessReadProbe = br

        rw, rwChecks, _ := r.RWProbe.Run(ctx, cfg)
        allChecks = append(allChecks, rwChecks...)
        snapshot.RWProbe = rw
    }
    // else 分支已在步骤 6 补齐，无需重复

analyze:
    endedAt := time.Now()
    return r.Analyzer.Analyze(ctx, analyzers.AnalyzeInput{
        Config:    cfg,
        Snapshot:  snapshot,
        Checks:    allChecks,
        StartedAt: startedAt,
        EndedAt:   endedAt,
    })
}
```

### 12.3 `--modules` 处理

```go
func normalizeModules(modules []string) ([]string, error) {
    known := map[string]bool{"k8s": true, "probe": true}
    for _, m := range modules {
        if m == "milvus" { continue }
        if !known[m] {
            return nil, fmt.Errorf("unknown module %q, valid values: k8s, probe", m)
        }
    }
    return modules, nil
}

func moduleEnabled(modules []string, name string) bool {
    if len(modules) == 0 { return true }
    for _, m := range modules { if m == name { return true } }
    return false
}
```

---

## 13. 错误模型

```go
type ErrorCode string
const (
    ErrCodeConfigInvalid ErrorCode = "CONFIG_INVALID"
    ErrCodeMilvusConnect ErrorCode = "MILVUS_CONNECT_ERROR"
    ErrCodeMilvusCollect ErrorCode = "MILVUS_COLLECT_ERROR"
    ErrCodeK8sCollect    ErrorCode = "K8S_COLLECT_ERROR"
    ErrCodeProbeRead     ErrorCode = "PROBE_READ_ERROR"
    ErrCodeProbeRW       ErrorCode = "PROBE_RW_ERROR"
    ErrCodeRender        ErrorCode = "RENDER_ERROR"
    ErrCodeRuntime       ErrorCode = "RUNTIME_ERROR"
)

type AppError struct {
    Code      ErrorCode `json:"code"`
    Message   string    `json:"message"`
    Cause     error     `json:"-"`
    Retriable bool      `json:"retriable"`
}

func (e *AppError) Error() string { return fmt.Sprintf("[%s] %s", e.Code, e.Message) }
func (e *AppError) Unwrap() error { return e.Cause }
```

**退出码映射：**

| 情形 | 退出码 |
|---|---|
| PASS | 0 |
| WARN | 1 |
| FAIL（巡检结论）| 2 |
| `ErrCodeConfigInvalid`（含 OptionsValidator 失败） | 3 |
| `ErrCodeRuntime`、panic、timeout | 4 |
| gRPC 客户端对象创建失败 | 4 |
| 连接建立后 version 调用失败 | 转为 CheckResult FAIL -> 2 |

---

## 14. 模块间依赖关系约束

```text
cmd -> cli -> (config, collectors, probes, analyzers, render, model)
collectors -> platform, model
probes -> platform, model
analyzers -> model
render -> model
config -> model
model -> 无依赖
platform -> 无内部依赖（不导入 model 包）
```

**禁止：**
- `render` 反向依赖 `collectors`
- `model` 依赖具体 SDK
- `analyzers` 直接调用外部 client
- `analyzers` 接收 `Detail bool`
- `platform` 导入 `model` 包
- `collectors` 在 `arch_profile=unknown` 时产出 FAIL/WARN 类 CheckResult

---

## 15. 首版接口与 spec 的映射关系

| spec 能力 | 模块 | 核心接口 |
|---|---|---|
| ArchProfile 识别 | `model` | `DetectArchProfile` |
| Milvus 基础信息 | `collectors/milvus` | `CollectClusterInfo` |
| inventory + binlog size | `collectors/milvus` | `CollectInventory` + `fetchBinlogSizes` |
| Pod/Service 状态（版本感知）| `collectors/k8s` | `CollectStatus(archProfile)` |
| `arch_profile=unknown` skip 产出 | `analyzers/k8s` | `K8sAnalyzer.Analyze` |
| Business Read Probe（严格 Action 状态机）| `probes` | `BusinessReadProbe.Run` |
| RW Probe（含预检清理）| `probes` | `RWProbe.Run` |
| PASS/WARN/FAIL + Warnings/Failures | `analyzers` | `SummaryBuilder.Build` |
| standby 计算 | `analyzers` | `StandbyAnalyzer.ComputeStandby` |
| confidence 计算（unknown=low）| `analyzers` | `SummaryBuilder.ComputeConfidence` |
| text/json 输出（shadow struct 省略 checks）| `render` | `Renderer.Render(result, RenderOptions)` |

---

## 16. 首版建议文件级拆分

```text
internal/
├── cli/
│   ├── check_runner.go          # DefaultCheckRunner；步骤 6 提前补齐 skip CheckResult
│   ├── validate_runner.go
│   └── exit_code.go
├── config/
│   ├── loader.go
│   ├── validator.go             # Validator，返回 ValidationReport
│   ├── options_validator.go     # OptionsValidator（--modules 等 CLI 参数）
│   ├── defaults.go
│   └── override.go
├── model/
│   ├── config.go                # Config / ClusterConfig / MilvusConfig / RulesConfig 等
│   ├── inventory.go             # DatabaseInventory / CollectionInventory / VectorField；BinlogSizeBytes 无 omitempty
│   ├── k8s.go                   # K8sStatus / PodStatus / ServiceInfo；ratio 字段无 omitempty
│   ├── probe.go                 # ProbeAction 枚举；BusinessReadProbeResult / RWProbeResult
│   ├── result.go                # CheckResult / AnalysisResult / AnalysisSummary / ProbeOutputView / ClusterOutputView；Checks 无 omitempty
│   ├── enums.go                 # ArchProfile/CriticalType/IgnoredReason/PodRole/ProbeAction/MetricsUnavailableReason
│   └── error.go                 # AppError / ErrorCode
├── platform/
│   ├── milvus_client.go         # CollectionStats 只含 RowCount
│   ├── k8s_client.go            # PodInfo 含 ContainerWaitingReasons；ListPodMetrics 错误语义精确定义
│   ├── clock.go
│   └── id.go
├── collectors/
│   ├── milvus/
│   │   ├── collector.go
│   │   ├── cluster_info.go
│   │   ├── inventory.go         # fetchBinlogSizes
│   │   └── binlog_metrics.go    # DataCoordQuotaMetrics 解析
│   └── k8s/
│       ├── collector.go         # unknown 时产出空 CheckResult
│       ├── pod_status.go        # mergeMetrics；ratio 字段无 omitempty
│       ├── pod_ignore.go        # shouldIgnore（含 arch 参数，arch_profile=unknown 时不触发 unmatched-role）
│       └── arch_resolver.go     # normalizePodRole（unknown->PodRoleUnknown）
├── probes/
│   ├── business_read.go         # ProbeAction 状态机；禁止 search 静默降级
│   ├── rw.go                    # cleanupStaleTestDatabases 失败->FAIL
│   └── generator.go
├── analyzers/
│   ├── analyzer.go
│   ├── connectivity.go
│   ├── inventory.go
│   ├── k8s.go                   # unknown->skip；Ignored Pod 跳过判定；pod_restart_fail->FAIL
│   ├── probe.go
│   ├── standby.go
│   └── summary.go               # SummaryOutput；confidence unknown=low
└── render/
    ├── factory.go
    ├── options.go
    ├── text.go
    └── json.go                  # shadow struct 省略 checks
```

---

## 17. 接口设计结论（v0.5->v0.6 改动汇总）

| 层 | 核心改动 |
|---|---|
| model | 补充 `CheckResult` struct 定义；补充 `DatabaseInventory` struct 定义；补充 `ProbeOutputView` struct 定义 |
| collectors/k8s | `shouldIgnore` 函数签名增加 `arch model.MilvusArchProfile` 参数；arch_profile=unknown 时 unmatched-role 规则不触发 |
| cli | `Run` 伪代码重构：步骤 6 在所有 goto 之前提前补齐 disabled 模块的 skip CheckResult；else 分支删除（避免重复补齐） |
| analyzers/k8s | 规则表补充"Pod restart 超 fail 阈值 => FAIL"和"Pod CrashLoopBackOff => FAIL"两行 |

---

## 18. 下一步最值得继续补的两份文档

1. **开发任务拆分文档**：模块分解为 Iteration 1/2/3 任务卡
2. **接口 Mock 与验收用例文档**：含 v2.4.x、v2.6.x、`arch_profile=unknown` 三套场景

---

## 19. GitHub 交付与验收约束

### 19.1 强制提交要求
- 所有代码、测试、文档完成后必须 push 到 GitHub
- 仅提供本地结果不成立 review

### 19.2 最低回传信息
- branch 名、commit SHA、改动文件列表
- 运行过的命令与结果摘要（格式：`命令 -> 输出摘要`）
- 一条 reviewer 可执行的验收命令（需说明前提条件）
- 简明变更说明：完成了什么、哪些仍是 stub、已知风险

### 19.3 与接口设计相关的约束
- 修改 `model`/`collectors`/`platform`/`render` 输出契约 -> 同步更新本文档
- 修改输出样例或 summary 字段 -> 同步更新 spec 样例与 `examples/`
- `make fmt`、`make test`、`make build` 全部通过后才视为交付就绪
