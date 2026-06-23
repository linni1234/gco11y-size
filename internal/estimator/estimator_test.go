package estimator

import (
	"strings"
	"testing"

	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer"
	"github.com/nilslindholm/metricgenerationsizer/internal/config"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func TestEstimateDefaultProcessors(t *testing.T) {
	analysis, err := analyzer.Analyze("../../testdata/fixtures/simple-service", "")
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	opts := model.Options{
		Repo:                "../../testdata/fixtures/simple-service",
		Profile:             "grafana-cloud-app-o11y",
		Processors:          append([]string(nil), config.DefaultProcessors...),
		HistogramBuckets:    2,
		HistogramType:       config.HistogramTypeClassic,
		StatusValues:        2,
		Environments:        1,
		InstancesPerService: 1,
	}
	estimate := Estimate(analysis, opts, nil)
	if got, want := estimate.TotalExpected, 111; got != want {
		t.Fatalf("total expected = %d, want %d", got, want)
	}
	assertProcessor(t, estimate, config.ProcessorSpanMetricsCount, 20)
	assertProcessor(t, estimate, config.ProcessorSpanMetricsLatency, 80)
	assertProcessor(t, estimate, config.ProcessorServiceGraph, 10)
	assertProcessor(t, estimate, config.ProcessorHostInfo, 1)
}

func TestEstimateCustomDimensionsMultiplySpanMetrics(t *testing.T) {
	analysis := model.Analysis{
		Services: []model.Service{{Name: "checkout"}},
		Operations: []model.Operation{
			{Service: "checkout", Kind: "SERVER", Method: "GET", Route: "/checkout"},
		},
	}
	opts := model.Options{
		Processors: []string{
			config.ProcessorSpanMetricsCount,
			config.ProcessorSpanMetricsLatency,
		},
		HistogramBuckets: 4,
		HistogramType:    config.HistogramTypeClassic,
		StatusValues:     2,
		Environments:     3,
		CustomDimensions: []model.Dimension{{Name: "tenant.id", Cardinality: 5}},
	}
	estimate := Estimate(analysis, opts, nil)
	assertProcessor(t, estimate, config.ProcessorSpanMetricsCount, 30)
	assertProcessor(t, estimate, config.ProcessorSpanMetricsLatency, 180)
	if got, want := estimate.TotalExpected, 210; got != want {
		t.Fatalf("total expected = %d, want %d", got, want)
	}
}

func TestHistogramSeriesMultipliersAreExplicit(t *testing.T) {
	analysis := model.Analysis{
		Services: []model.Service{{Name: "checkout"}, {Name: "payment"}},
		Operations: []model.Operation{
			{Service: "checkout", Kind: "SERVER", Method: "GET", Route: "/checkout"},
		},
		Edges: []model.Edge{
			{SourceService: "checkout", TargetService: "payment", Protocol: "http"},
		},
	}
	opts := model.Options{
		Processors: []string{
			config.ProcessorSpanMetricsCount,
			config.ProcessorSpanMetricsLatency,
			config.ProcessorServiceGraph,
		},
		HistogramBuckets: 8,
		HistogramType:    config.HistogramTypeClassic,
		StatusValues:     3,
		Environments:     1,
	}
	estimate := Estimate(analysis, opts, nil)

	assertProcessor(t, estimate, config.ProcessorSpanMetricsCount, 3)
	assertProcessor(t, estimate, config.ProcessorSpanMetricsLatency, 30)
	assertProcessor(t, estimate, config.ProcessorServiceGraph, 22)
	if got, want := estimate.TotalExpected, 55; got != want {
		t.Fatalf("total expected = %d, want %d", got, want)
	}
	if formula := processorFormula(t, estimate, config.ProcessorSpanMetricsLatency); !strings.Contains(formula, "classic histogram bucket series + _sum + _count") {
		t.Fatalf("span latency formula does not mention explicit histogram series: %q", formula)
	}
	if formula := processorFormula(t, estimate, config.ProcessorServiceGraph); !strings.Contains(formula, "2 x (classic histogram bucket series + _sum + _count)") {
		t.Fatalf("service graph formula does not mention client/server histogram expansion: %q", formula)
	}
	if got, want := len(estimate.UncertaintyModel), 1; got != want {
		t.Fatalf("uncertainty model count = %d, want %d", got, want)
	}
	uncertainty := estimate.UncertaintyModel[0]
	if uncertainty.Scope != "span-metrics" {
		t.Fatalf("uncertainty scope = %q, want span-metrics", uncertainty.Scope)
	}
	assertUncertaintyBound(t, uncertainty, "low", 2, 1, 1)
	assertUncertaintyBound(t, uncertainty, "expected", 3, 1, 1)
	assertUncertaintyBound(t, uncertainty, "high", 4, 1, 1.05)
	if got, want := histogramFactorValue(t, uncertainty, config.ProcessorSpanMetricsLatency), 10; got != want {
		t.Fatalf("latency histogram factor = %d, want %d", got, want)
	}
	if !strings.Contains(strings.Join(uncertainty.Notes, " "), "standard OTel status codes") {
		t.Fatalf("uncertainty notes do not mention OTel status-code assumption: %#v", uncertainty.Notes)
	}
}

func TestHistogramTypeNativeIsDefault(t *testing.T) {
	analysis := model.Analysis{
		Services: []model.Service{{Name: "checkout"}, {Name: "payment"}},
		Operations: []model.Operation{
			{Service: "checkout", Kind: "SERVER", Method: "GET", Route: "/checkout"},
		},
		Edges: []model.Edge{
			{SourceService: "checkout", TargetService: "payment", Protocol: "http"},
		},
	}
	opts := model.Options{
		Processors: []string{
			config.ProcessorSpanMetricsLatency,
			config.ProcessorServiceGraph,
		},
		HistogramBuckets: 8,
		StatusValues:     2,
		Environments:     1,
	}
	estimate := Estimate(analysis, opts, nil)

	assertProcessor(t, estimate, config.ProcessorSpanMetricsLatency, 2)
	assertProcessor(t, estimate, config.ProcessorServiceGraph, 4)
	if got, want := estimate.TotalExpected, 6; got != want {
		t.Fatalf("total expected = %d, want %d", got, want)
	}
	if formula := processorFormula(t, estimate, config.ProcessorSpanMetricsLatency); !strings.Contains(formula, "native histogram series") {
		t.Fatalf("native span latency formula does not mention native histogram sizing: %q", formula)
	}
}

func TestHistogramTypeBothAddsNativeToClassicSeries(t *testing.T) {
	analysis := model.Analysis{
		Services: []model.Service{{Name: "checkout"}, {Name: "payment"}},
		Operations: []model.Operation{
			{Service: "checkout", Kind: "SERVER", Method: "GET", Route: "/checkout"},
		},
		Edges: []model.Edge{
			{SourceService: "checkout", TargetService: "payment", Protocol: "http"},
		},
	}
	opts := model.Options{
		Processors: []string{
			config.ProcessorSpanMetricsLatency,
			config.ProcessorServiceGraph,
		},
		HistogramBuckets: 8,
		HistogramType:    config.HistogramTypeBoth,
		StatusValues:     2,
		Environments:     1,
	}
	estimate := Estimate(analysis, opts, nil)

	assertProcessor(t, estimate, config.ProcessorSpanMetricsLatency, 22)
	assertProcessor(t, estimate, config.ProcessorServiceGraph, 24)
	if got, want := estimate.TotalExpected, 46; got != want {
		t.Fatalf("total expected = %d, want %d", got, want)
	}
	if formula := processorFormula(t, estimate, config.ProcessorSpanMetricsLatency); !strings.Contains(formula, "+ native histogram series") {
		t.Fatalf("both-mode span latency formula does not mention native histogram expansion: %q", formula)
	}
}

func TestInstanceLabelMultiplierAffectsGeneratedTraceMetrics(t *testing.T) {
	analysis := model.Analysis{
		Services: []model.Service{{Name: "checkout"}, {Name: "payment"}},
		Operations: []model.Operation{
			{Service: "checkout", Kind: "SERVER", Method: "GET", Route: "/checkout"},
		},
		Edges: []model.Edge{
			{SourceService: "checkout", TargetService: "payment", Protocol: "http"},
		},
	}
	opts := model.Options{
		Processors: []string{
			config.ProcessorSpanMetricsCount,
			config.ProcessorSpanMetricsLatency,
			config.ProcessorServiceGraph,
			config.ProcessorHostInfo,
		},
		HistogramBuckets:     2,
		HistogramType:        config.HistogramTypeClassic,
		StatusValues:         2,
		Environments:         1,
		InstancesPerService:  3,
		InstanceLabelEnabled: true,
	}
	estimate := Estimate(analysis, opts, nil)
	assertProcessor(t, estimate, config.ProcessorSpanMetricsCount, 6)
	assertProcessor(t, estimate, config.ProcessorSpanMetricsLatency, 24)
	assertProcessor(t, estimate, config.ProcessorServiceGraph, 30)
	assertProcessor(t, estimate, config.ProcessorHostInfo, 6)
	if got, want := estimate.TotalExpected, 66; got != want {
		t.Fatalf("total expected with instance label = %d, want %d", got, want)
	}
	if formula := processorFormula(t, estimate, config.ProcessorSpanMetricsLatency); !strings.Contains(formula, "instance label values") {
		t.Fatalf("span latency formula does not mention instance label values: %q", formula)
	}

	opts.InstanceLabelEnabled = false
	withoutInstanceLabel := Estimate(analysis, opts, nil)
	assertProcessor(t, withoutInstanceLabel, config.ProcessorSpanMetricsCount, 2)
	assertProcessor(t, withoutInstanceLabel, config.ProcessorSpanMetricsLatency, 8)
	assertProcessor(t, withoutInstanceLabel, config.ProcessorServiceGraph, 10)
	assertProcessor(t, withoutInstanceLabel, config.ProcessorHostInfo, 6)
	if got, want := withoutInstanceLabel.TotalExpected, 26; got != want {
		t.Fatalf("total expected without instance label = %d, want %d", got, want)
	}
}

func TestServiceSizingOverridesAffectSpanAndHostInfo(t *testing.T) {
	analysis := model.Analysis{
		Services: []model.Service{{Name: "checkout"}, {Name: "payment"}},
		Operations: []model.Operation{
			{Service: "checkout", Kind: "SERVER", Method: "GET", Route: "/checkout"},
		},
	}
	opts := model.Options{
		Processors: []string{
			config.ProcessorSpanMetricsCount,
			config.ProcessorHostInfo,
		},
		StatusValues:         1,
		Environments:         1,
		InstancesPerService:  1,
		InstanceLabelEnabled: true,
		ServiceOverrides: []model.ServiceSizingOverride{
			{Service: "checkout", EnvironmentNames: []string{"prod", "staging"}, InstancesPerService: 3},
		},
	}
	estimate := Estimate(analysis, opts, nil)
	assertProcessor(t, estimate, config.ProcessorSpanMetricsCount, 6)
	assertProcessor(t, estimate, config.ProcessorHostInfo, 7)
	if got, want := estimate.TotalExpected, 13; got != want {
		t.Fatalf("total expected = %d, want %d", got, want)
	}
	if got, want := serviceExpected(t, estimate, "checkout"), 12; got != want {
		t.Fatalf("checkout service expected = %d, want %d", got, want)
	}
}

func TestGatewayMethodExpansionOnlyAffectsGatewayOperations(t *testing.T) {
	analysis := model.Analysis{
		Services: []model.Service{{Name: "api-gateway"}, {Name: "orders"}},
		Operations: []model.Operation{
			{Service: "api-gateway", Kind: "SERVER", Method: "ANY", Route: "/products/**", Origin: "gateway"},
			{Service: "orders", Kind: "SERVER", Method: "GET", Route: "/orders"},
		},
	}
	opts := model.Options{
		Processors: []string{
			config.ProcessorSpanMetricsCount,
			config.ProcessorSpanMetricsLatency,
		},
		HistogramBuckets: 14,
		HistogramType:    config.HistogramTypeClassic,
		StatusValues:     3,
		Environments:     1,
	}
	baseline := Estimate(analysis, opts, nil)
	if got, want := operationExpected(t, baseline, "api-gateway", "/products/**"), 51; got != want {
		t.Fatalf("baseline gateway operation expected = %d, want %d", got, want)
	}
	if got, want := baseline.TotalExpected, 102; got != want {
		t.Fatalf("baseline total expected = %d, want %d", got, want)
	}

	opts.GatewayMethods = []string{"GET", "POST", "PUT", "DELETE"}
	expanded := Estimate(analysis, opts, nil)
	if got, want := operationExpected(t, expanded, "api-gateway", "/products/**"), 204; got != want {
		t.Fatalf("expanded gateway operation expected = %d, want %d", got, want)
	}
	if got, want := operationExpected(t, expanded, "orders", "/orders"), 51; got != want {
		t.Fatalf("controller operation expected = %d, want %d", got, want)
	}
	if got, want := expanded.TotalExpected, 255; got != want {
		t.Fatalf("expanded total expected = %d, want %d", got, want)
	}
}

func assertProcessor(t *testing.T, estimate model.Estimate, processor string, expected int) {
	t.Helper()
	for _, item := range estimate.ProcessorBreakdown {
		if item.Processor == processor {
			if item.Expected != expected {
				t.Fatalf("%s expected = %d, want %d", processor, item.Expected, expected)
			}
			return
		}
	}
	t.Fatalf("processor %s not found in %#v", processor, estimate.ProcessorBreakdown)
}

func assertUncertaintyBound(t *testing.T, uncertainty model.UncertaintyModel, bound string, statusValues int, dimensionMultiplier int, bufferMultiplier float64) {
	t.Helper()
	for _, item := range uncertainty.Bounds {
		if item.Bound == bound {
			if item.StatusValues != statusValues || item.DimensionMultiplier != dimensionMultiplier || item.BufferMultiplier != bufferMultiplier {
				t.Fatalf("bound %s = %#v, want status=%d dimension=%d buffer=%g", bound, item, statusValues, dimensionMultiplier, bufferMultiplier)
			}
			return
		}
	}
	t.Fatalf("bound %s not found in %#v", bound, uncertainty.Bounds)
}

func histogramFactorValue(t *testing.T, uncertainty model.UncertaintyModel, metric string) int {
	t.Helper()
	for _, item := range uncertainty.HistogramFactors {
		if item.Metric == metric {
			return item.Value
		}
	}
	t.Fatalf("histogram factor %s not found in %#v", metric, uncertainty.HistogramFactors)
	return 0
}

func processorFormula(t *testing.T, estimate model.Estimate, processor string) string {
	t.Helper()
	for _, item := range estimate.ProcessorBreakdown {
		if item.Processor == processor {
			return item.Formula
		}
	}
	t.Fatalf("processor %s not found in %#v", processor, estimate.ProcessorBreakdown)
	return ""
}

func operationExpected(t *testing.T, estimate model.Estimate, service string, route string) int {
	t.Helper()
	for _, item := range estimate.OperationContributors {
		if item.Service == service && item.Route == route {
			return item.Expected
		}
	}
	t.Fatalf("operation contribution %s %s not found in %#v", service, route, estimate.OperationContributors)
	return 0
}

func serviceExpected(t *testing.T, estimate model.Estimate, service string) int {
	t.Helper()
	for _, item := range estimate.ServiceBreakdown {
		if item.Service == service {
			return item.Expected
		}
	}
	t.Fatalf("service contribution %s not found in %#v", service, estimate.ServiceBreakdown)
	return 0
}
