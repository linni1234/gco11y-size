package model

import "time"

const Version = "0.2.0"

type Dimension struct {
	Name        string `json:"name"`
	Cardinality int    `json:"cardinality"`
}

type Options struct {
	Repo                  string      `json:"repo"`
	OtelConfig            string      `json:"otel_config,omitempty"`
	Ref                   string      `json:"ref,omitempty"`
	Workdir               string      `json:"workdir,omitempty"`
	KeepWorktree          bool        `json:"keep_worktree"`
	GatewayMethodCount    int         `json:"gateway_method_count,omitempty"`
	GatewayMethods        []string    `json:"gateway_methods,omitempty"`
	Profile               string      `json:"profile"`
	Processors            []string    `json:"processors"`
	HistogramBuckets      int         `json:"histogram_buckets"`
	HistogramType         string      `json:"histogram_type"`
	StatusValues          int         `json:"status_values"`
	Environments          int         `json:"environments"`
	InstancesPerService   int         `json:"instances_per_service"`
	InstanceLabelEnabled  bool        `json:"instance_label_enabled"`
	CustomDimensions      []Dimension `json:"custom_dimensions,omitempty"`
	IncludeClientProducer bool        `json:"include_client_producer"`
	GrafanaQuery          bool        `json:"grafana_query"`
}

type Analysis struct {
	Repository       string          `json:"repository"`
	GeneratedAt      time.Time       `json:"generated_at"`
	Services         []Service       `json:"services"`
	Operations       []Operation     `json:"operations"`
	Edges            []Edge          `json:"edges"`
	ConfigFindings   []ConfigFinding `json:"config_findings,omitempty"`
	Risks            []Risk          `json:"risks,omitempty"`
	Warnings         []string        `json:"warnings,omitempty"`
	DetectedLanguage string          `json:"detected_language"`
}

type Service struct {
	Name           string `json:"name"`
	Root           string `json:"root"`
	Source         string `json:"source"`
	OperationCount int    `json:"operation_count"`
	EdgeCount      int    `json:"edge_count"`
}

type Operation struct {
	Service    string   `json:"service"`
	Kind       string   `json:"kind"`
	Protocol   string   `json:"protocol,omitempty"`
	Method     string   `json:"method"`
	Route      string   `json:"route"`
	Handler    string   `json:"handler,omitempty"`
	Source     string   `json:"source"`
	Origin     string   `json:"origin,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
	Detectors  []string `json:"detectors,omitempty"`
	Risks      []string `json:"risks,omitempty"`
}

type Edge struct {
	SourceService string `json:"source_service"`
	TargetService string `json:"target_service"`
	Protocol      string `json:"protocol"`
	Source        string `json:"source"`
	Confidence    string `json:"confidence"`
}

type ConfigFinding struct {
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	Source  string `json:"source"`
	Service string `json:"service,omitempty"`
}

type Risk struct {
	Severity string `json:"severity"`
	Area     string `json:"area"`
	Message  string `json:"message"`
	Source   string `json:"source,omitempty"`
}

type Estimate struct {
	TotalLow              int                 `json:"total_low"`
	TotalExpected         int                 `json:"total_expected"`
	TotalHigh             int                 `json:"total_high"`
	ProcessorBreakdown    []ProcessorEstimate `json:"processor_breakdown"`
	ComponentBreakdown    []ComponentEstimate `json:"component_breakdown"`
	OperationContributors []OperationEstimate `json:"operation_contributors"`
	ServiceBreakdown      []ServiceEstimate   `json:"service_breakdown"`
	UncertaintyModel      []UncertaintyModel  `json:"uncertainty_model,omitempty"`
	Assumptions           []string            `json:"assumptions"`
	CloudCalibration      *CloudCalibration   `json:"cloud_calibration,omitempty"`
}

type ProcessorEstimate struct {
	Processor string `json:"processor"`
	Low       int    `json:"low"`
	Expected  int    `json:"expected"`
	High      int    `json:"high"`
	Formula   string `json:"formula"`
}

type ComponentEstimate struct {
	Component string `json:"component"`
	Expected  int    `json:"expected"`
	Formula   string `json:"formula"`
}

type OperationEstimate struct {
	Service  string `json:"service"`
	Protocol string `json:"protocol,omitempty"`
	Method   string `json:"method"`
	Route    string `json:"route"`
	Kind     string `json:"kind"`
	Origin   string `json:"origin,omitempty"`
	Expected int    `json:"expected"`
}

type ServiceEstimate struct {
	Service  string `json:"service"`
	Expected int    `json:"expected"`
}

type UncertaintyModel struct {
	Scope            string             `json:"scope"`
	Formula          string             `json:"formula"`
	HistogramFactors []HistogramFactor  `json:"histogram_factors,omitempty"`
	Bounds           []UncertaintyBound `json:"bounds"`
	Notes            []string           `json:"notes,omitempty"`
}

type HistogramFactor struct {
	Metric string `json:"metric"`
	Factor string `json:"factor"`
	Value  int    `json:"value"`
}

type UncertaintyBound struct {
	Bound               string  `json:"bound"`
	StatusRule          string  `json:"status_rule"`
	StatusValues        int     `json:"status_values"`
	DimensionRule       string  `json:"dimension_rule"`
	DimensionMultiplier int     `json:"dimension_multiplier"`
	Buffer              string  `json:"buffer"`
	BufferMultiplier    float64 `json:"buffer_multiplier"`
}

type CloudCalibration struct {
	Configured     bool      `json:"configured"`
	Queried        bool      `json:"queried"`
	ObservedSeries int       `json:"observed_series,omitempty"`
	Query          string    `json:"query,omitempty"`
	Message        string    `json:"message,omitempty"`
	Source         string    `json:"source,omitempty"`
	ObservedAt     time.Time `json:"observed_at,omitempty"`
}

type SourceMetadata struct {
	Original          string `json:"original"`
	Type              string `json:"type"`
	Provider          string `json:"provider,omitempty"`
	CloneURL          string `json:"clone_url,omitempty"`
	ResolvedPath      string `json:"resolved_path"`
	RequestedRef      string `json:"requested_ref,omitempty"`
	ResolvedRef       string `json:"resolved_ref,omitempty"`
	WorktreeTemporary bool   `json:"worktree_temporary"`
	WorktreeRetained  bool   `json:"worktree_retained"`
}

type Report struct {
	Version     string         `json:"version"`
	GeneratedAt time.Time      `json:"generated_at"`
	Options     Options        `json:"options"`
	Source      SourceMetadata `json:"source"`
	Analysis    Analysis       `json:"analysis"`
	Estimate    Estimate       `json:"estimate"`
}
