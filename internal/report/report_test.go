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

func TestRenderWorkspaceHTML(t *testing.T) {
	workspace := model.WorkspaceReport{
		Version:     model.Version,
		GeneratedAt: time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC),
		Aggregate: model.Report{
			Estimate: model.Estimate{TotalLow: 10, TotalExpected: 20, TotalHigh: 30},
			Analysis: model.Analysis{
				Services:   []model.Service{{Name: "checkout"}},
				Operations: []model.Operation{{Service: "checkout", Kind: "SERVER", Protocol: "http", Method: "GET", Route: "/checkout"}},
				Edges:      []model.Edge{{SourceService: "checkout", TargetService: "payment", Protocol: "http"}},
			},
		},
		Repositories: []model.RepositoryReport{
			{
				Name: "checkout",
				Report: model.Report{
					Options: model.Options{HistogramType: "native", Environments: 1, InstancesPerService: 2},
					Source:  model.SourceMetadata{Original: "./checkout", Type: "local", ResolvedPath: "/repo/checkout"},
					Analysis: model.Analysis{
						Services:   []model.Service{{Name: "checkout", Root: ".", OperationCount: 1}},
						Operations: []model.Operation{{Service: "checkout", Kind: "SERVER", Protocol: "http", Method: "GET", Route: "/checkout"}},
					},
					Estimate: model.Estimate{
						TotalLow:      10,
						TotalExpected: 20,
						TotalHigh:     30,
						ProcessorBreakdown: []model.ProcessorEstimate{
							{Processor: "span-metrics-count", Low: 1, Expected: 2, High: 3, Formula: "test"},
						},
						OperationContributors: []model.OperationEstimate{
							{Service: "checkout", Protocol: "http", Method: "GET", Route: "/checkout", Expected: 2},
						},
					},
				},
			},
		},
	}
	html, err := RenderWorkspaceHTML(workspace)
	if err != nil {
		t.Fatalf("RenderWorkspaceHTML returned error: %v", err)
	}
	for _, expected := range []string{"Repository Overview", "side-rail", "rail-nav", "workspace-subnav", "href=\"#repo-checkout\"", "data-workspace-tab=\"repo-checkout\"", "data-workspace-section=\"repo-checkout-processors\"", "data-workspace-view=\"overview\"", "workspace-view active", "checkout", "Processors", "Top Operations", "themeToggle"} {
		if !strings.Contains(html, expected) {
			t.Fatalf("workspace HTML did not contain %q", expected)
		}
	}
}
