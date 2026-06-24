package gorilla

import (
	"go/ast"
	"strings"

	gocommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func Operations(ctx gocommon.FileContext) []model.Operation {
	if !gocommon.HasImport(ctx.File, "github.com/gorilla/mux") {
		return nil
	}
	var operations []model.Operation
	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		if gocommon.SelectorName(call) != "Methods" {
			return
		}
		inner, ok := receiverCall(call)
		if !ok {
			return
		}
		path, ok := routePath(inner)
		if !ok {
			return
		}
		methods := gocommon.StringArgs(call.Args)
		if len(methods) == 0 {
			methods = []string{"ANY"}
		}
		handler := ""
		if len(inner.Args) > 1 {
			handler = gocommon.HandlerName(inner.Args[1])
		}
		for _, method := range methods {
			operations = append(operations, gocommon.Operation(ctx.ServiceName, "SERVER", "http", strings.ToUpper(method), gocommon.NormalizeRoute(path), handler, ctx.SourcePath, "gorilla-mux", "high"))
		}
	})
	return operations
}

func receiverCall(call *ast.CallExpr) (*ast.CallExpr, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, false
	}
	inner, ok := selector.X.(*ast.CallExpr)
	return inner, ok
}

func routePath(call *ast.CallExpr) (string, bool) {
	switch gocommon.SelectorName(call) {
	case "Handle", "HandleFunc", "Path", "PathPrefix":
		return gocommon.StringArg(call, 0)
	default:
		return "", false
	}
}
