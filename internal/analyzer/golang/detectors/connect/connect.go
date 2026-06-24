package connect

import (
	"go/ast"
	"regexp"
	"strings"

	gocommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var newHandlerRE = regexp.MustCompile(`^New([A-Za-z0-9_]+)Handler$`)

func Operations(ctx gocommon.FileContext) []model.Operation {
	if !hasConnectHint(ctx.File) {
		return nil
	}
	methodsByReceiver := receiverMethods(ctx.File)
	var operations []model.Operation
	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		match := newHandlerRE.FindStringSubmatch(gocommon.SelectorName(call))
		if len(match) != 2 || len(call.Args) == 0 {
			return
		}
		service := strings.TrimSuffix(match[1], "Service")
		receiver := receiverType(call.Args[0])
		for _, method := range methodsByReceiver[receiver] {
			operations = append(operations, gocommon.Operation(ctx.ServiceName, "SERVER", "grpc", "RPC", service+"/"+method, "Connect handler registration", ctx.SourcePath, "connect-go", "medium"))
		}
	})
	return operations
}

func hasConnectHint(file *ast.File) bool {
	for _, path := range gocommon.ImportPaths(file) {
		if strings.Contains(path, "connectrpc.com/connect") || strings.Contains(path, "/connect") {
			return true
		}
	}
	return false
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
