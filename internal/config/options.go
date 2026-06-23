package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

const (
	ProcessorSpanMetricsCount   = "span-metrics-count"
	ProcessorSpanMetricsLatency = "span-metrics-latency"
	ProcessorSpanMetricsSize    = "span-metrics-size"
	ProcessorServiceGraph       = "service-graph"
	ProcessorHostInfo           = "host-info"

	HistogramTypeNative  = "native"
	HistogramTypeClassic = "classic"
	HistogramTypeBoth    = "both"
)

var DefaultProcessors = []string{
	ProcessorSpanMetricsCount,
	ProcessorSpanMetricsLatency,
	ProcessorServiceGraph,
	ProcessorHostInfo,
}

type DimensionFlags []model.Dimension

func (d *DimensionFlags) String() string {
	if d == nil {
		return ""
	}
	parts := make([]string, 0, len(*d))
	for _, dim := range *d {
		parts = append(parts, fmt.Sprintf("%s=%d", dim.Name, dim.Cardinality))
	}
	return strings.Join(parts, ",")
}

func (d *DimensionFlags) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("dimension must be name=cardinality")
	}
	name, rawCardinality, ok := strings.Cut(value, "=")
	if !ok {
		return fmt.Errorf("dimension %q must be name=cardinality", value)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("dimension name cannot be empty")
	}
	cardinality, err := strconv.Atoi(strings.TrimSpace(rawCardinality))
	if err != nil || cardinality < 1 {
		return fmt.Errorf("dimension %q cardinality must be a positive integer", name)
	}
	*d = append(*d, model.Dimension{Name: name, Cardinality: cardinality})
	return nil
}

func SplitProcessors(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return append([]string(nil), DefaultProcessors...)
	}
	seen := map[string]bool{}
	var processors []string
	for _, part := range strings.Split(raw, ",") {
		p := strings.ToLower(strings.TrimSpace(part))
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		processors = append(processors, p)
	}
	return processors
}

func ProcessorEnabled(processors []string, name string) bool {
	for _, processor := range processors {
		if processor == name {
			return true
		}
	}
	return false
}

func NormalizeHistogramType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return HistogramTypeNative
	}
	return value
}

func ValidateOptions(opts model.Options) error {
	if strings.TrimSpace(opts.Repo) == "" {
		return fmt.Errorf("--repo is required")
	}
	if opts.HistogramBuckets < 1 {
		return fmt.Errorf("--histogram-buckets must be at least 1")
	}
	switch NormalizeHistogramType(opts.HistogramType) {
	case HistogramTypeNative, HistogramTypeClassic, HistogramTypeBoth:
	default:
		return fmt.Errorf("--histogram-type must be one of: native, classic, both")
	}
	if opts.StatusValues < 1 {
		return fmt.Errorf("--status-values must be at least 1")
	}
	if opts.Environments < 1 {
		return fmt.Errorf("--environments must be at least 1")
	}
	if len(opts.EnvironmentNames) > 0 {
		seen := map[string]bool{}
		for _, environment := range opts.EnvironmentNames {
			environment = strings.TrimSpace(environment)
			if environment == "" {
				return fmt.Errorf("--environment values cannot be empty")
			}
			if seen[environment] {
				return fmt.Errorf("--environment %q is duplicated", environment)
			}
			seen[environment] = true
		}
	}
	if opts.InstancesPerService < 1 {
		return fmt.Errorf("--instances-per-service must be at least 1")
	}
	for _, override := range opts.ServiceOverrides {
		if strings.TrimSpace(override.Service) == "" {
			return fmt.Errorf("service override service cannot be empty")
		}
		if override.Environments < 0 {
			return fmt.Errorf("service override for %q has negative environment count", override.Service)
		}
		if override.InstancesPerService < 0 {
			return fmt.Errorf("service override for %q has negative instances_per_service", override.Service)
		}
		for _, environment := range override.EnvironmentNames {
			if strings.TrimSpace(environment) == "" {
				return fmt.Errorf("service override for %q has an empty environment", override.Service)
			}
		}
	}
	if opts.GatewayMethodCount < 0 {
		return fmt.Errorf("--gateway-method-count cannot be negative")
	}
	if opts.GatewayMethodCount > 0 && len(opts.GatewayMethods) > 0 && opts.GatewayMethodCount != len(opts.GatewayMethods) {
		return fmt.Errorf("--gateway-method-count must match --gateway-methods length when both are set")
	}
	known := map[string]bool{
		ProcessorSpanMetricsCount:   true,
		ProcessorSpanMetricsLatency: true,
		ProcessorSpanMetricsSize:    true,
		ProcessorServiceGraph:       true,
		ProcessorHostInfo:           true,
	}
	for _, processor := range opts.Processors {
		if !known[processor] {
			return fmt.Errorf("unknown processor %q", processor)
		}
	}
	return nil
}
