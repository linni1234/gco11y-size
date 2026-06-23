package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
