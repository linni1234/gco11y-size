package otel

import (
	"fmt"
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	nodecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var resourceServiceNameRE = regexp.MustCompile(`(?s)['"]service\.name['"]\s*:\s*['"]([^'"]+)['"]`)

func Operations(ctx nodecommon.FileContext) []model.Operation {
	if !strings.Contains(ctx.Source, "startSpan") && !strings.Contains(ctx.Source, "startActiveSpan") {
		return nil
	}
	var operations []model.Operation
	for _, call := range nodecommon.Calls(ctx.Source, "startSpan", "startActiveSpan") {
		name, ok := nodecommon.FirstString(call.Args)
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		operations = append(operations, nodecommon.Operation(ctx.ServiceName, "INTERNAL", "custom", "SPAN", name, "manual span", ctx.SourcePath, "opentelemetry", "medium"))
	}
	return operations
}

func ConfigFindings(ctx nodecommon.FileContext) []model.ConfigFinding {
	var findings []model.ConfigFinding
	if nodecommon.HasImport(ctx.Source, "@opentelemetry/") || strings.Contains(ctx.Source, "opentelemetry") {
		findings = append(findings, model.ConfigFinding{Kind: "instrumentation", Name: "opentelemetry-js", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	for _, match := range resourceServiceNameRE.FindAllStringSubmatch(ctx.Source, -1) {
		findings = append(findings, model.ConfigFinding{Kind: "service-name", Name: "otel.resource.service.name", Value: basecommon.SanitizeServiceName(match[1]), Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	return findings
}

func Risks(ctx nodecommon.FileContext) []model.Risk {
	var risks []model.Risk
	for _, call := range nodecommon.Calls(ctx.Source, "setAttribute", "setAttributes", "addEvent") {
		for _, attribute := range nodecommon.ExtractStringLiterals(call.Args) {
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
