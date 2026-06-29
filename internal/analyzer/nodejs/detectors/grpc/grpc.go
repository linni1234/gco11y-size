package grpc

import (
	"regexp"
	"strings"

	nodecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	protoPackageRE = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z0-9_.]+)\s*;`)
	protoServiceRE = regexp.MustCompile(`(?s)\bservice\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{(.*?)\}`)
	protoRPCRE     = regexp.MustCompile(`\brpc\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	addServiceRE   = regexp.MustCompile(`(?s)\.addService\s*\(\s*([A-Za-z0-9_.$]+)\.service\s*,\s*\{(.*?)\}\s*\)`)
	handlerKeyRE   = regexp.MustCompile(`(?m)\b([A-Za-z_][A-Za-z0-9_]*)\s*:`)
	newClientRE    = regexp.MustCompile(`\bnew\s+[A-Za-z0-9_.$]+\s*\(\s*['"` + "`" + `]([^'"` + "`" + `]+)['"` + "`" + `]`)
)

func Operations(ctx nodecommon.FileContext) []model.Operation {
	if !nodecommon.HasImport(ctx.Source, "@grpc/grpc-js", "grpc") && !strings.Contains(ctx.Source, "addService(") {
		return nil
	}
	var operations []model.Operation
	for _, match := range addServiceRE.FindAllStringSubmatch(ctx.Source, -1) {
		service := serviceName(match[1])
		for _, method := range handlerKeyRE.FindAllStringSubmatch(match[2], -1) {
			operations = append(operations, nodecommon.Operation(ctx.ServiceName, "SERVER", "grpc", "RPC", service+"/"+method[1], "gRPC service registration", ctx.SourcePath, "grpc-js", "medium"))
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
			operations = append(operations, nodecommon.Operation(serviceName, "SERVER", "grpc", "RPC", service+"/"+rpcMatch[1], "protobuf service definition", sourcePath, "protobuf", "medium"))
		}
	}
	return operations
}

func ProtoServiceCount(source string) int {
	return len(protoServiceRE.FindAllStringSubmatch(source, -1))
}

func Edges(ctx nodecommon.FileContext) []model.Edge {
	if !nodecommon.HasImport(ctx.Source, "@grpc/grpc-js", "grpc") {
		return nil
	}
	var edges []model.Edge
	for _, call := range nodecommon.Calls(ctx.Source, "Client", "loadPackageDefinition") {
		if call.Name != "Client" {
			continue
		}
		target, ok := nodecommon.FirstString(call.Args)
		if !ok {
			continue
		}
		normalized := nodecommon.NormalizeTargetService(target)
		if nodecommon.IgnoreTarget(normalized, ctx.ServiceName) {
			continue
		}
		edges = append(edges, nodecommon.Edge(ctx.ServiceName, normalized, "grpc", ctx.SourcePath, "medium"))
	}
	for _, match := range newClientRE.FindAllStringSubmatch(ctx.Source, -1) {
		normalized := nodecommon.NormalizeTargetService(match[1])
		if nodecommon.IgnoreTarget(normalized, ctx.ServiceName) {
			continue
		}
		edges = append(edges, nodecommon.Edge(ctx.ServiceName, normalized, "grpc", ctx.SourcePath, "medium"))
	}
	return edges
}

func serviceName(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "Service")
	if dot := strings.LastIndex(raw, "."); dot >= 0 {
		raw = raw[dot+1:]
	}
	return raw
}
