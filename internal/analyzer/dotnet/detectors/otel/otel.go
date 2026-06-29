package otel

import (
	"fmt"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	dotnetcommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func Operations(ctx dotnetcommon.FileContext) []model.Operation {
	var operations []model.Operation
	for _, call := range dotnetcommon.Calls(ctx.Source, "StartActivity", "StartSpan") {
		name, ok := dotnetcommon.FirstString(call.Args)
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "INTERNAL", "custom", "SPAN", name, "manual span", ctx.SourcePath, "opentelemetry", "medium"))
	}
	return operations
}

func ConfigFindings(ctx dotnetcommon.FileContext) []model.ConfigFinding {
	var findings []model.ConfigFinding
	for _, call := range dotnetcommon.Calls(ctx.Source, "AddService") {
		if value, ok := dotnetcommon.FirstString(call.Args); ok {
			findings = append(findings, model.ConfigFinding{Kind: "service-name", Name: "otel.resource.add-service", Value: basecommon.SanitizeServiceName(value), Source: ctx.SourcePath, Service: ctx.ServiceName})
		}
	}
	if strings.Contains(ctx.Source, "OpenTelemetry") || strings.Contains(ctx.Source, "ActivitySource") {
		findings = append(findings, model.ConfigFinding{Kind: "instrumentation", Name: "opentelemetry-dotnet", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	return findings
}

func Risks(ctx dotnetcommon.FileContext) []model.Risk {
	var risks []model.Risk
	for _, call := range dotnetcommon.Calls(ctx.Source, "SetTag", "AddTag", "SetAttribute") {
		attribute, ok := dotnetcommon.FirstString(call.Args)
		if !ok {
			continue
		}
		if basecommon.LooksHighCardinalityAttribute(strings.ToLower(attribute)) {
			risks = append(risks, model.Risk{
				Severity: "high",
				Area:     "span attributes",
				Message:  fmt.Sprintf("span attribute %q is likely high-cardinality", attribute),
				Source:   ctx.SourcePath,
			})
		}
	}
	return risks
}
