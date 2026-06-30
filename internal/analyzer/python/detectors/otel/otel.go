package otel

import (
	"fmt"
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	pycommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var resourceServiceNameRE = regexp.MustCompile(`(?s)['"]service\.name['"]\s*:\s*['"]([^'"]+)['"]`)

func Operations(ctx pycommon.FileContext) []model.Operation {
	if !strings.Contains(ctx.Source, "start_span") && !strings.Contains(ctx.Source, "start_as_current_span") {
		return nil
	}
	var operations []model.Operation
	for _, call := range pycommon.Calls(ctx.Source, "start_span", "start_as_current_span") {
		name, ok := pycommon.FirstString(call.Args)
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		operations = append(operations, pycommon.Operation(ctx.ServiceName, "INTERNAL", "custom", "SPAN", name, "manual span", ctx.SourcePath, "opentelemetry", "medium"))
	}
	return operations
}

func ConfigFindings(ctx pycommon.FileContext) []model.ConfigFinding {
	var findings []model.ConfigFinding
	if pycommon.HasImport(ctx.Source, "opentelemetry") || strings.Contains(ctx.Source, "opentelemetry") {
		findings = append(findings, model.ConfigFinding{Kind: "instrumentation", Name: "opentelemetry-python", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	if pycommon.HasImport(ctx.Source, "ddtrace") {
		findings = append(findings, model.ConfigFinding{Kind: "instrumentation", Name: "ddtrace", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	if pycommon.HasImport(ctx.Source, "sentry_sdk") {
		findings = append(findings, model.ConfigFinding{Kind: "instrumentation", Name: "sentry", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	for _, match := range resourceServiceNameRE.FindAllStringSubmatch(ctx.Source, -1) {
		findings = append(findings, model.ConfigFinding{Kind: "service-name", Name: "otel.resource.service.name", Value: basecommon.SanitizeServiceName(match[1]), Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	for _, imported := range pycommon.Imports(ctx.Source) {
		if strings.HasPrefix(imported, "opentelemetry.instrumentation") {
			findings = append(findings, model.ConfigFinding{Kind: "instrumentation", Name: imported, Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
		}
	}
	return findings
}

func Risks(ctx pycommon.FileContext) []model.Risk {
	var risks []model.Risk
	for _, call := range pycommon.Calls(ctx.Source, "set_attribute", "set_attributes", "set_tag", "add_event") {
		for _, attribute := range pycommon.ExtractStringLiterals(call.Args) {
			if basecommon.LooksHighCardinalityAttribute(strings.ToLower(attribute)) {
				risks = append(risks, model.Risk{
					Severity: "high",
					Area:     "span attributes",
					Message:  fmt.Sprintf("span attribute %q is likely high-cardinality", attribute),
					Source:   ctx.SourcePath,
				})
			}
		}
	}
	return risks
}
