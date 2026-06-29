package grpc

import (
	"regexp"
	"strings"

	dotnetcommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	protoPackageRE = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z0-9_.]+)\s*;`)
	protoServiceRE = regexp.MustCompile(`(?s)\bservice\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{(.*?)\}`)
	protoRPCRE     = regexp.MustCompile(`\brpc\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	baseServiceRE  = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*([A-Za-z_][A-Za-z0-9_]*)Base\b`)
)

func Operations(ctx dotnetcommon.FileContext) []model.Operation {
	var operations []model.Operation
	for _, class := range dotnetcommon.FindClasses(ctx.Source) {
		service := serviceFromBases(class.Bases)
		if service == "" {
			continue
		}
		for _, method := range dotnetcommon.Methods(class.Body) {
			if strings.EqualFold(method.Name, class.Name) {
				continue
			}
			operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "SERVER", "grpc", "RPC", service+"/"+method.Name, class.Name+"."+method.Name, ctx.SourcePath, "grpc-dotnet", "medium"))
		}
	}
	return operations
}

func ProtoOperations(serviceName string, sourcePath string, source string) []model.Operation {
	protoPackage := ""
	if match := protoPackageRE.FindStringSubmatch(source); len(match) == 2 {
		protoPackage = match[1]
	}
	var operations []model.Operation
	for _, serviceMatch := range protoServiceRE.FindAllStringSubmatch(source, -1) {
		service := serviceMatch[1]
		if protoPackage != "" {
			service = protoPackage + "." + service
		}
		for _, rpcMatch := range protoRPCRE.FindAllStringSubmatch(serviceMatch[2], -1) {
			operations = append(operations, dotnetcommon.Operation(serviceName, "SERVER", "grpc", "RPC", service+"/"+rpcMatch[1], "protobuf service definition", sourcePath, "protobuf", "medium"))
		}
	}
	return operations
}

func ProtoServiceCount(source string) int {
	return len(protoServiceRE.FindAllStringSubmatch(source, -1))
}

func Edges(ctx dotnetcommon.FileContext) []model.Edge {
	var edges []model.Edge
	for _, call := range dotnetcommon.Calls(ctx.Source, "ForAddress") {
		target, ok := dotnetcommon.FirstString(call.Args)
		if !ok {
			continue
		}
		normalized := dotnetcommon.NormalizeTargetService(target)
		if dotnetcommon.IgnoreTarget(normalized, ctx.ServiceName) {
			continue
		}
		edges = append(edges, dotnetcommon.Edge(ctx.ServiceName, normalized, "grpc", ctx.SourcePath, "high"))
	}
	return edges
}

func serviceFromBases(bases string) string {
	if match := baseServiceRE.FindStringSubmatch(bases); len(match) == 3 {
		return match[1]
	}
	return ""
}
