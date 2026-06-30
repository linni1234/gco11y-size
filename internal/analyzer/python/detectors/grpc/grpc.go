package grpc

import (
	"regexp"
	"strings"

	pycommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	protoPackageRE        = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z0-9_.]+)\s*;`)
	protoServiceRE        = regexp.MustCompile(`(?s)\bservice\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{(.*?)\}`)
	protoRPCRE            = regexp.MustCompile(`\brpc\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	addServicerRE         = regexp.MustCompile(`\badd_([A-Za-z_][A-Za-z0-9_]*)Servicer_to_server\s*\(`)
	servicerClassRE       = regexp.MustCompile(`(?s)class\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*Servicer[^)]*\)\s*:(.*?)(?:\nclass\s|\z)`)
	servicerMethodRE      = regexp.MustCompile(`(?m)^\s+def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	channelTargetMethodRE = regexp.MustCompile(`\b(?:insecure_channel|secure_channel|aio\.insecure_channel|aio\.secure_channel)\s*\(\s*['"]([^'"]+)['"]`)
)

func Operations(ctx pycommon.FileContext) []model.Operation {
	if !pycommon.HasImport(ctx.Source, "grpc") && !strings.Contains(ctx.Source, "Servicer_to_server") {
		return nil
	}
	var operations []model.Operation
	for _, classMatch := range servicerClassRE.FindAllStringSubmatch(ctx.Source, -1) {
		service := strings.TrimSuffix(classMatch[1], "Servicer")
		for _, methodMatch := range servicerMethodRE.FindAllStringSubmatch(classMatch[2], -1) {
			method := methodMatch[1]
			if method == "__init__" {
				continue
			}
			operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "grpc", "RPC", service+"/"+method, classMatch[1]+"."+method, ctx.SourcePath, "grpc-python", "medium"))
		}
	}
	if len(operations) == 0 {
		for _, match := range addServicerRE.FindAllStringSubmatch(ctx.Source, -1) {
			service := strings.TrimSuffix(match[1], "Servicer")
			operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "grpc", "RPC", service+"/configured", "gRPC servicer registration", ctx.SourcePath, "grpc-python", "low"))
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
			operations = append(operations, pycommon.Operation(serviceName, "SERVER", "grpc", "RPC", service+"/"+rpcMatch[1], "protobuf service definition", sourcePath, "protobuf", "medium"))
		}
	}
	return operations
}

func ProtoServiceCount(source string) int {
	return len(protoServiceRE.FindAllStringSubmatch(source, -1))
}

func Edges(ctx pycommon.FileContext) []model.Edge {
	if !pycommon.HasImport(ctx.Source, "grpc") {
		return nil
	}
	var edges []model.Edge
	for _, match := range channelTargetMethodRE.FindAllStringSubmatch(ctx.Source, -1) {
		target := pycommon.NormalizeTargetService(match[1])
		if pycommon.IgnoreTarget(target, ctx.ServiceName) {
			continue
		}
		edges = append(edges, pycommon.Edge(ctx.ServiceName, target, "grpc", ctx.SourcePath, "medium"))
	}
	return edges
}
