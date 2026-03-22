package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

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

type MilvusArchProfile string

const (
	ArchProfileV24     MilvusArchProfile = "v2.4"
	ArchProfileV26     MilvusArchProfile = "v2.6"
	ArchProfileUnknown MilvusArchProfile = "unknown"
)

func DetectArchProfile(version string) MilvusArchProfile {
	major, minor, ok := parseMajorMinor(version)
	if !ok {
		return ArchProfileUnknown
	}

	switch {
	case major == 2 && (minor == 4 || minor == 5):
		return ArchProfileV24
	case major == 2 && minor >= 6:
		return ArchProfileV26
	case major > 2:
		return ArchProfileV26
	default:
		return ArchProfileUnknown
	}
}

func parseMajorMinor(version string) (int, int, bool) {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return 0, 0, false
	}

	parts := strings.Split(trimmed, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}
	return major, minor, true
}

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

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Cause.Error())
	}
	return string(e.Code)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type Config struct {
	Cluster    ClusterConfig `yaml:"cluster"`
	K8s        K8sConfig     `yaml:"k8s"`
	Probe      ProbeConfig   `yaml:"probe"`
	Output     OutputConfig  `yaml:"output"`
	TimeoutSec int           `yaml:"timeout_sec"`
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
	Namespace  string `yaml:"namespace"`
	Kubeconfig string `yaml:"kubeconfig"`
}

type ProbeConfig struct {
	Read ReadProbeConfig `yaml:"read"`
	RW   RWProbeConfig   `yaml:"rw"`
}

type ReadProbeConfig struct {
	MinSuccessTargets    int               `yaml:"min_success_targets"`
	Targets              []ReadProbeTarget `yaml:"targets"`
	minSuccessTargetsSet bool              `yaml:"-"`
}

func (c *ReadProbeConfig) UnmarshalYAML(unmarshal func(any) error) error {
	type rawReadProbeConfig struct {
		MinSuccessTargets *int              `yaml:"min_success_targets"`
		Targets           []ReadProbeTarget `yaml:"targets"`
	}

	var raw rawReadProbeConfig
	if err := unmarshal(&raw); err != nil {
		return err
	}

	c.Targets = raw.Targets
	c.minSuccessTargetsSet = raw.MinSuccessTargets != nil
	if raw.MinSuccessTargets != nil {
		c.MinSuccessTargets = *raw.MinSuccessTargets
	} else {
		c.MinSuccessTargets = 0
	}
	return nil
}

func (c ReadProbeConfig) HasExplicitMinSuccessTargets() bool {
	return c.minSuccessTargetsSet
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

type OutputConfig struct {
	Format OutputFormat `yaml:"format"`
	Detail bool         `yaml:"detail"`
}

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

type CheckResult struct {
	Category       string      `json:"category,omitempty"`
	Name           string      `json:"name"`
	Target         string      `json:"target,omitempty"`
	Status         CheckStatus `json:"status"`
	Severity       string      `json:"severity,omitempty"`
	Message        string      `json:"message"`
	Recommendation string      `json:"recommendation,omitempty"`
	Evidence       []string    `json:"evidence,omitempty"`
	Actual         any         `json:"actual,omitempty"`
	Expected       any         `json:"expected,omitempty"`
	DurationMS     int64       `json:"duration_ms,omitempty"`
}

type AnalysisSummary struct {
	DatabaseCount   int    `json:"database_count"`
	CollectionCount int    `json:"collection_count"`
	TotalRowCount   *int64 `json:"total_row_count"`
	PodCount        int    `json:"pod_count"`
}

type ClusterInventory struct {
	Milvus MilvusInventory `json:"milvus,omitempty"`
	K8s    K8sInventory    `json:"k8s,omitempty"`
}

type MilvusInventory struct {
	Reachable            bool                  `json:"reachable"`
	ServerVersion        string                `json:"server_version,omitempty"`
	DatabaseCount        int                   `json:"database_count"`
	CollectionCount      int                   `json:"collection_count"`
	TotalRowCount        *int64                `json:"total_row_count"`
	DatabaseNames        []string              `json:"database_names,omitempty"`
	Databases            []DatabaseInventory   `json:"databases,omitempty"`
	Collections          []CollectionInventory `json:"collections,omitempty"`
	CapabilityDegraded   bool                  `json:"capability_degraded,omitempty"`
	DegradedCapabilities []string              `json:"degraded_capabilities,omitempty"`
}

type DatabaseInventory struct {
	Name        string   `json:"name"`
	Collections []string `json:"collections,omitempty"`
}

type CollectionInventory struct {
	Database   string `json:"database"`
	Name       string `json:"name"`
	RowCount   *int64 `json:"row_count"`
	ShardNum   int32  `json:"shard_num,omitempty"`
	FieldCount int    `json:"field_count,omitempty"`
}

type K8sInventory struct {
	Namespace string              `json:"namespace,omitempty"`
	Pods      []PodStatusSummary  `json:"pods,omitempty"`
	Services  []ServiceInventory  `json:"services,omitempty"`
	Endpoints []EndpointInventory `json:"endpoints,omitempty"`
}

type PodStatusSummary struct {
	Name         string `json:"name"`
	Phase        string `json:"phase"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restart_count"`
}

type ServiceInventory struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Ports []string `json:"ports,omitempty"`
}

type EndpointInventory struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses,omitempty"`
}

type BusinessReadProbeResult struct {
	Status            CheckStatus `json:"status"`
	ConfiguredTargets int         `json:"configured_targets"`
	SuccessfulTargets int         `json:"successful_targets"`
	MinSuccessTargets int         `json:"min_success_targets"`
	Message           string      `json:"message"`
}

type RWProbeResult struct {
	Status          CheckStatus `json:"status"`
	Enabled         bool        `json:"enabled"`
	CleanupEnabled  bool        `json:"cleanup_enabled,omitempty"`
	CleanupExecuted bool        `json:"cleanup_executed,omitempty"`
	Message         string      `json:"message,omitempty"`
}

type ProbeOutputView struct {
	BusinessRead BusinessReadProbeResult `json:"business_read"`
	RW           RWProbeResult           `json:"rw"`
}

type ClusterInfo struct {
	Name          string            `json:"name"`
	MilvusURI     string            `json:"milvus_uri"`
	Namespace     string            `json:"namespace"`
	MilvusVersion string            `json:"milvus_version"`
	ArchProfile   MilvusArchProfile `json:"arch_profile"`
	MQType        string            `json:"mq_type"`
}

type ClusterOutputView = ClusterInfo

type AnalysisResult struct {
	Cluster    ClusterInfo       `json:"cluster"`
	Result     FinalResult       `json:"result"`
	Standby    bool              `json:"standby"`
	Confidence ConfidenceLevel   `json:"confidence"`
	ExitCode   int               `json:"exit_code"`
	ElapsedMS  int64             `json:"elapsed_ms"`
	Summary    AnalysisSummary   `json:"summary"`
	Probes     ProbeOutputView   `json:"probes"`
	Inventory  *ClusterInventory `json:"inventory,omitempty"`
	Warnings   []string          `json:"warnings,omitempty"`
	Failures   []string          `json:"failures,omitempty"`
	Checks     []CheckResult     `json:"checks,omitempty"`
}

type MetadataSnapshot struct {
	Cluster ClusterInfo     `json:"cluster"`
	Milvus  MilvusInventory `json:"milvus"`
	K8s     K8sInventory    `json:"k8s"`
}

type AnalyzeInput struct {
	Config    *Config
	Inventory ClusterInventory
	Snapshot  MetadataSnapshot
	Checks    []CheckResult
	Warnings  []string
	Failures  []string
	StartedAt time.Time
	EndedAt   time.Time
}
