package report

import (
	"strings"
	"testing"
	"time"

	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func TestRenderHTML(t *testing.T) {
	r := model.Report{
		Version:     model.Version,
		GeneratedAt: time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC),
		Options: model.Options{
			Repo:                 ".",
			Profile:              "grafana-cloud-app-o11y",
			Environments:         1,
			HistogramBuckets:     14,
			StatusValues:         3,
			InstancesPerService:  2,
			InstanceLabelEnabled: true,
		},
		Analysis: model.Analysis{
			Repository: "/repo",
			Services:   []model.Service{{Name: "checkout", Root: ".", OperationCount: 1}},
			Operations: []model.Operation{{Service: "checkout", Kind: "SERVER", Protocol: "http", Method: "GET", Route: "/checkout", Source: "CheckoutController.java", Confidence: "high", Detectors: []string{"spring-mvc"}}},
		},
		Source: model.SourceMetadata{
			Original:     "https://github.com/acme/checkout.git",
			Type:         "git",
			Provider:     "github",
			CloneURL:     "https://github.com/acme/checkout.git",
			ResolvedPath: "/tmp/checkout",
			RequestedRef: "main",
			ResolvedRef:  "abcdef123456",
		},
		Estimate: model.Estimate{
			TotalLow:      10,
			TotalExpected: 20,
			TotalHigh:     30,
			ProcessorBreakdown: []model.ProcessorEstimate{
				{Processor: "span-metrics-latency", Low: 10, Expected: 20, High: 30, Formula: "test"},
			},
			UncertaintyModel: []model.UncertaintyModel{
				{
					Scope:   "span-metrics",
					Formula: "series = operations x environments x status_values(bound) x dim_cardinality(bound) x histogram_factor",
					HistogramFactors: []model.HistogramFactor{
						{Metric: "span-metrics-latency", Factor: "native histogram series", Value: 1},
					},
					Bounds: []model.UncertaintyBound{
						{Bound: "low", StatusRule: "status_values - 1", StatusValues: 2, DimensionRule: "reduced", DimensionMultiplier: 1, Buffer: "none", BufferMultiplier: 1},
						{Bound: "expected", StatusRule: "status_values", StatusValues: 3, DimensionRule: "configured", DimensionMultiplier: 1, Buffer: "none", BufferMultiplier: 1},
						{Bound: "high", StatusRule: "status_values + 1", StatusValues: 4, DimensionRule: "inflated", DimensionMultiplier: 1, Buffer: "+5%", BufferMultiplier: 1.05},
					},
					Notes: []string{"Bounds assume standard OTel status codes."},
				},
			},
		},
	}
	html, err := RenderHTML(r)
	if err != nil {
		t.Fatalf("RenderHTML returned error: %v", err)
	}
	for _, expected := range []string{"Grafana Cloud App O11y Series Estimate", "Expected active series", "Component Breakdown", "Uncertainty Model", "inflated", "Bounds assume standard OTel status codes", "Top Span Metric Operation Contributors", "/checkout", "span-metrics-latency", "https://github.com/acme/checkout.git", "abcdef123456", "data-theme=\"dark\"", "themeToggle", "Light mode", "Dark mode", "prepareResponsiveTables", "responsive-table", "Protocol", "Detectors", "spring-mvc", "Instance label", "Instances / service"} {
		if !strings.Contains(html, expected) {
			t.Fatalf("rendered HTML did not contain %q", expected)
		}
	}
}
