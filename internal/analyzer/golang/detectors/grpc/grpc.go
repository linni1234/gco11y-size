package grpc

import (
	"go/ast"
	"regexp"
	"strings"

	gocommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	protoPackageRE   = regexp.MustCompile(`(?m)^\s*package\s+([A-Za-z0-9_.]+)\s*;`)
	protoServiceRE   = regexp.MustCompile(`(?s)\bservice\s+([A-Za-z_][A-Za-z0-9_]*)\s*\{(.*?)\}`)
	protoRPCRE       = regexp.MustCompile(`\brpc\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	registerServerRE = regexp.MustCompile(`^Register([A-Za-z0-9_]+)Server$`)
)

func Operations(ctx gocommon.FileContext) []model.Operation {
	methodsByReceiver := receiverMethods(ctx.File)
	var operations []model.Operation
	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		match := registerServerRE.FindStringSubmatch(gocommon.SelectorName(call))
		if len(match) != 2 || len(call.Args) < 2 {
			return
		}
		service := strings.TrimSuffix(match[1], "Service")
		receiver := receiverType(call.Args[1])
		for _, method := range methodsByReceiver[receiver] {
			operations = append(operations, gocommon.Operation(ctx.ServiceName, "SERVER", "grpc", "RPC", service+"/"+method, "gRPC service registration", ctx.SourcePath, "grpc-go", "medium"))
		}
	})
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
			operations = append(operations, gocommon.Operation(serviceName, "SERVER", "grpc", "RPC", service+"/"+rpcMatch[1], "protobuf service definition", sourcePath, "protobuf", "medium"))
		}
	}
	return operations
}

func ProtoServiceCount(source string) int {
	return len(protoServiceRE.FindAllStringSubmatch(source, -1))
}

func Edges(ctx gocommon.FileContext) []model.Edge {
	var edges []model.Edge
	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		switch gocommon.CallName(call.Fun) {
		case "grpc.Dial", "grpc.DialContext", "grpc.NewClient":
			target, ok := gocommon.StringArg(call, 0)
			if !ok {
				return
			}
			normalized := gocommon.NormalizeTargetService(target)
			if gocommon.IgnoreTarget(normalized, ctx.ServiceName) {
				return
			}
			edges = append(edges, gocommon.Edge(ctx.ServiceName, normalized, "grpc", ctx.SourcePath, "medium"))
		}
	})
	return edges
}

func receiverMethods(file *ast.File) map[string][]string {
	methods := map[string][]string{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		receiver := receiverType(fn.Recv.List[0].Type)
		if receiver == "" {
			continue
		}
		methods[receiver] = append(methods[receiver], fn.Name.Name)
	}
	return methods
}

func receiverType(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.UnaryExpr:
		return receiverType(value.X)
	case *ast.StarExpr:
		return receiverType(value.X)
	case *ast.CompositeLit:
		return receiverType(value.Type)
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	case *ast.CallExpr:
		return receiverType(value.Fun)
	}
	return ""
}
