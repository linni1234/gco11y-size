package grpc

import (
	"regexp"

	javacommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	grpcExtendsRE    = regexp.MustCompile(`extends\s+(?:[A-Za-z0-9_$.]+\.)?([A-Za-z0-9_]+)Grpc\.([A-Za-z0-9_]+)ImplBase`)
	grpcMethodRE     = regexp.MustCompile(`(?m)public\s+void\s+([A-Za-z_][A-Za-z0-9_]*)\s*\([^)]*StreamObserver\s*<`)
	grpcForAddressRE = regexp.MustCompile(`forAddress\s*\(\s*"([^"]+)"`)
	grpcForTargetRE  = regexp.MustCompile(`forTarget\s*\(\s*"([^"]+)"`)
	protoPackageRE   = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z0-9_.]+)\s*;`)
	protoServiceRE   = regexp.MustCompile(`(?s)\bservice\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{(.*?)\}`)
	protoRPCRE       = regexp.MustCompile(`\brpc\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
)

func JavaOperations(serviceName string, sourcePath string, source string) []model.Operation {
	matches := grpcExtendsRE.FindStringSubmatch(source)
	if len(matches) != 3 {
		return nil
	}
	service := matches[2]
	var operations []model.Operation
	for _, methodMatch := range grpcMethodRE.FindAllStringSubmatch(source, -1) {
		method := methodMatch[1]
		operations = append(operations, javacommon.Operation(serviceName, "SERVER", "grpc", "RPC", service+"/"+method, "gRPC service implementation", sourcePath, "grpc-java", "high"))
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
			operations = append(operations, javacommon.Operation(serviceName, "SERVER", "grpc", "RPC", service+"/"+rpcMatch[1], "protobuf service definition", sourcePath, "protobuf", "medium"))
		}
	}
	return operations
}

func ProtoServiceCount(source string) int {
	return len(protoServiceRE.FindAllStringSubmatch(source, -1))
}

func Targets(source string) []string {
	var targets []string
	for _, match := range grpcForAddressRE.FindAllStringSubmatch(source, -1) {
		targets = append(targets, javacommon.NormalizeTargetService(match[1]))
	}
	for _, match := range grpcForTargetRE.FindAllStringSubmatch(source, -1) {
		targets = append(targets, javacommon.NormalizeTargetService(match[1]))
	}
	return targets
}
