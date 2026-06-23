package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func TestRunScanLocalRepoWritesSourceMetadata(t *testing.T) {
	outDir := t.TempDir()
	htmlPath := filepath.Join(outDir, "report.html")
	jsonPath := filepath.Join(outDir, "report.json")
	err := run([]string{
		"scan",
		"--repo", "../../testdata/fixtures/two-services",
		"--out", htmlPath,
		"--json", jsonPath,
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var report model.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if report.Source.Type != "local" {
		t.Fatalf("source type = %q, want local", report.Source.Type)
	}
	if report.Source.ResolvedPath == "" {
		t.Fatalf("expected resolved path in source metadata")
	}
	if report.Options.HistogramType != "native" {
		t.Fatalf("histogram type = %q, want native", report.Options.HistogramType)
	}
	if !report.Options.InstanceLabelEnabled {
		t.Fatalf("instance label enabled = false, want true")
	}
	if _, err := os.Stat(htmlPath); err != nil {
		t.Fatalf("HTML report was not written: %v", err)
	}
}

func TestRunScanInstanceLabelDisabled(t *testing.T) {
	outDir := t.TempDir()
	jsonPath := filepath.Join(outDir, "report.json")
	err := run([]string{
		"scan",
		"--repo", "../../testdata/fixtures/simple-service",
		"--instance-label", "disabled",
		"--out", "",
		"--json", jsonPath,
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var report model.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if report.Options.InstanceLabelEnabled {
		t.Fatalf("instance label enabled = true, want false")
	}
}

func TestRunScanDeprecatedNativeHistogramsAliasSetsBoth(t *testing.T) {
	outDir := t.TempDir()
	jsonPath := filepath.Join(outDir, "report.json")
	err := run([]string{
		"scan",
		"--repo", "../../testdata/fixtures/simple-service",
		"--native-histograms",
		"--out", "",
		"--json", jsonPath,
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var report model.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if report.Options.HistogramType != "both" {
		t.Fatalf("histogram type = %q, want both", report.Options.HistogramType)
	}
}

func TestRunScanRejectsConflictingHistogramFlags(t *testing.T) {
	err := run([]string{
		"scan",
		"--repo", "../../testdata/fixtures/simple-service",
		"--histogram-type", "native",
		"--native-histograms",
		"--out", "",
		"--json", "",
	})
	if err == nil {
		t.Fatal("expected conflicting histogram flags to fail")
	}
}

func TestRunScanRejectsInvalidInstanceLabel(t *testing.T) {
	err := run([]string{
		"scan",
		"--repo", "../../testdata/fixtures/simple-service",
		"--instance-label", "maybe",
		"--out", "",
		"--json", "",
	})
	if err == nil {
		t.Fatal("expected invalid instance label to fail")
	}
}

func TestRunScanMultipleReposWritesWorkspaceReport(t *testing.T) {
	outDir := t.TempDir()
	htmlPath := filepath.Join(outDir, "workspace.html")
	jsonPath := filepath.Join(outDir, "workspace.json")
	err := run([]string{
		"scan",
		"--repo", "../../testdata/fixtures/simple-service",
		"--repo", "../../testdata/fixtures/gateway-service",
		"--environment", "prod",
		"--environment", "staging",
		"--service-instances", "order-api=3",
		"--out", htmlPath,
		"--json", jsonPath,
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var workspace model.WorkspaceReport
	if err := json.Unmarshal(data, &workspace); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if got, want := len(workspace.Repositories), 2; got != want {
		t.Fatalf("repository count = %d, want %d", got, want)
	}
	if got, want := len(workspace.Options.EnvironmentNames), 2; got != want {
		t.Fatalf("environment name count = %d, want %d", got, want)
	}
	if got := workspace.Aggregate.Estimate.TotalExpected; got <= 0 {
		t.Fatalf("aggregate expected = %d, want > 0", got)
	}
	if _, err := os.Stat(htmlPath); err != nil {
		t.Fatalf("workspace HTML report was not written: %v", err)
	}
	html, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("ReadFile HTML returned error: %v", err)
	}
	for _, expected := range []string{"Repository Overview", "simple-service", "gateway-service", "side-rail", "rail-nav", "workspace-subnav", "data-workspace-tab=\"repo-simple-service\"", "data-workspace-section=\"repo-simple-service-processors\"", "data-workspace-view=\"overview\""} {
		if !strings.Contains(string(html), expected) {
			t.Fatalf("workspace HTML did not contain %q", expected)
		}
	}
}

func TestRunScanWorkspaceInputJSON(t *testing.T) {
	outDir := t.TempDir()
	inputPath := filepath.Join(outDir, "sizing.json")
	jsonPath := filepath.Join(outDir, "workspace.json")
	input := `{
  "defaults": {
    "histogram_type": "classic",
    "histogram_buckets": 2,
    "status_values": 2,
    "environments": ["prod", "staging"],
    "instances_per_service": 2
  },
  "repos": [
    {"name": "orders", "repo": "../../testdata/fixtures/simple-service"},
    {"name": "gateway", "repo": "../../testdata/fixtures/gateway-service", "environments": ["prod"], "instances_per_service": 1}
  ],
  "service_overrides": [
    {"service": "api-gateway", "instances_per_service": 3, "environments": ["prod"]}
  ]
}`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	err := run([]string{
		"scan",
		"--input", inputPath,
		"--out", "",
		"--json", jsonPath,
	})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	var workspace model.WorkspaceReport
	if err := json.Unmarshal(data, &workspace); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if workspace.Options.HistogramType != "classic" {
		t.Fatalf("histogram type = %q, want classic", workspace.Options.HistogramType)
	}
	if got, want := len(workspace.Repositories), 2; got != want {
		t.Fatalf("repository count = %d, want %d", got, want)
	}
	if workspace.Repositories[0].Name != "orders" || workspace.Repositories[1].Name != "gateway" {
		t.Fatalf("repository names = %#v", workspace.Repositories)
	}
	if got := workspace.Aggregate.Estimate.TotalExpected; got <= 0 {
		t.Fatalf("aggregate expected = %d, want > 0", got)
	}
}
