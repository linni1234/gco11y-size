package otel

import (
	"fmt"
	"go/ast"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	gocommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func Operations(ctx gocommon.FileContext) []model.Operation {
	if !hasOTelHint(ctx.File) {
		return nil
	}
	var operations []model.Operation
	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		if gocommon.SelectorName(call) != "Start" {
			return
		}
		spanName, ok := gocommon.StringArg(call, 1)
		if !ok || strings.TrimSpace(spanName) == "" {
			return
		}
		operations = append(operations, gocommon.Operation(ctx.ServiceName, "INTERNAL", "custom", "SPAN", spanName, "OpenTelemetry span", ctx.SourcePath, "otel-go", "medium"))
	})
	return operations
}

func Risks(ctx gocommon.FileContext) []model.Risk {
	if !hasOTelHint(ctx.File) {
		return nil
	}
	seen := map[string]bool{}
	var risks []model.Risk
	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		name := gocommon.SelectorName(call)
		if name == "" || len(call.Args) == 0 {
			return
		}
		if !isAttributeConstructor(name) {
			return
		}
		key, ok := gocommon.StringArg(call, 0)
		if !ok || !basecommon.LooksHighCardinalityAttribute(strings.ToLower(key)) || seen[key] {
			return
		}
		seen[key] = true
		risks = append(risks, model.Risk{
			Severity: "high",
			Area:     "span attributes",
			Message:  fmt.Sprintf("Go OTel attribute %q is likely high-cardinality if enabled as a span-metrics dimension", key),
			Source:   ctx.SourcePath,
		})
	})
	return risks
}

func hasOTelHint(file *ast.File) bool {
	for _, path := range gocommon.ImportPaths(file) {
		if strings.Contains(path, "go.opentelemetry.io/otel") {
			return true
		}
	}
	return false
}

func isAttributeConstructor(name string) bool {
	switch name {
	case "String", "Int", "Int64", "Float64", "Bool", "StringSlice", "IntSlice", "Int64Slice", "Float64Slice", "BoolSlice":
		return true
	default:
		return false
	}
}
