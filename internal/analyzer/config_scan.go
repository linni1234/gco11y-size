package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	serviceNamePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?im)^\s*otel\.service\.name\s*[:=]\s*["']?([A-Za-z0-9_.-]+)["']?`),
		regexp.MustCompile(`(?im)\bOTEL_SERVICE_NAME\s*[:=]\s*["']?([A-Za-z0-9_.-]+)["']?`),
		regexp.MustCompile(`(?im)\bservice\.name\s*=\s*([A-Za-z0-9_.-]+)`),
	}
	environmentPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?im)^\s*spring\.profiles\.active\s*[:=]\s*["']?([A-Za-z0-9_.-]+)["']?`),
		regexp.MustCompile(`(?im)\bdeployment\.environment\s*[:=]\s*["']?([A-Za-z0-9_.-]+)["']?`),
		regexp.MustCompile(`(?im)\benvironment\s*[:=]\s*["']?([A-Za-z0-9_.-]+)["']?`),
	}
)

func scanConfig(repo string, otelConfig string) framework.Result {
	result := framework.Result{ServiceNames: map[string]string{}}
	scanConfigPath(repo, repo, &result)
	if strings.TrimSpace(otelConfig) != "" {
		path := otelConfig
		if !filepath.IsAbs(path) {
			path = filepath.Join(repo, path)
		}
		scanConfigPath(repo, path, &result)
	}
	return result
}

func scanConfigPath(repo string, path string, result *framework.Result) {
	info, err := os.Stat(path)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not read config path %s: %v", path, err))
		return
	}
	if !info.IsDir() {
		parseConfigFile(repo, path, result)
		return
	}
	_ = filepath.WalkDir(path, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not read config path %s: %v", current, walkErr))
			return nil
		}
		if entry.IsDir() {
			if common.ShouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if common.IsConfigFile(current) {
			parseConfigFile(repo, current, result)
		}
		return nil
	})
}

func parseConfigFile(repo string, path string, result *framework.Result) {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not read config file %s: %v", path, err))
		return
	}
	content := string(contentBytes)
	source := common.RelPath(repo, path)
	root := common.InferServiceRoot(repo, path)

	for _, pattern := range serviceNamePatterns {
		for _, match := range pattern.FindAllStringSubmatch(content, -1) {
			serviceName := common.SanitizeServiceName(match[1])
			result.ServiceNames[root] = serviceName
			result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{
				Kind:    "service-name",
				Name:    "service.name",
				Value:   serviceName,
				Source:  source,
				Service: serviceName,
			})
		}
	}

	for _, pattern := range environmentPatterns {
		for _, match := range pattern.FindAllStringSubmatch(content, -1) {
			result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{
				Kind:   "environment",
				Name:   "deployment.environment",
				Value:  strings.Trim(match[1], `"'`),
				Source: source,
			})
		}
	}
	for _, env := range environmentsFromFilename(path) {
		result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{
			Kind:   "environment",
			Name:   "profile",
			Value:  env,
			Source: source,
		})
	}

	lower := strings.ToLower(content)
	if strings.Contains(lower, "span_metrics") || strings.Contains(lower, "span-metrics") || strings.Contains(lower, "spanmetrics") {
		result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{Kind: "processor", Name: "span-metrics", Value: "detected", Source: source})
	}
	if strings.Contains(lower, "service_graph") || strings.Contains(lower, "service-graph") {
		result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{Kind: "processor", Name: "service-graph", Value: "detected", Source: source})
	}
	if strings.Contains(lower, "host_info") || strings.Contains(lower, "host-info") || strings.Contains(lower, "target_info") {
		result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{Kind: "processor", Name: "host-info", Value: "detected", Source: source})
	}

	for _, dimension := range parseDimensionHints(content) {
		result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{
			Kind:   "dimension-hint",
			Name:   dimension,
			Value:  "configured",
			Source: source,
		})
		if common.LooksHighCardinalityAttribute(strings.ToLower(dimension)) {
			result.Risks = append(result.Risks, model.Risk{
				Severity: "high",
				Area:     "custom dimensions",
				Message:  fmt.Sprintf("configured dimension %q is likely high-cardinality", dimension),
				Source:   source,
			})
		}
	}
}

func environmentsFromFilename(path string) []string {
	name := strings.ToLower(filepath.Base(path))
	prefixes := []string{"application-", "bootstrap-"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			withoutPrefix := strings.TrimPrefix(name, prefix)
			env := strings.TrimSuffix(withoutPrefix, filepath.Ext(withoutPrefix))
			if env != "" {
				return []string{env}
			}
		}
	}
	return nil
}

func parseDimensionHints(content string) []string {
	var dimensions []string
	re := regexp.MustCompile(`(?im)\bdimensions?\s*[:=]\s*\[?([A-Za-z0-9_.,/" -]+)\]?`)
	for _, match := range re.FindAllStringSubmatch(content, -1) {
		for _, part := range strings.Split(match[1], ",") {
			part = strings.Trim(strings.TrimSpace(part), `"'[]`)
			if part == "" || strings.Contains(part, " ") || strings.Contains(part, "/") {
				continue
			}
			dimensions = appendUnique(dimensions, part)
		}
	}
	return dimensions
}
