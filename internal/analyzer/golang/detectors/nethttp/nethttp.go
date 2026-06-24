package nethttp

import (
	"go/ast"
	"strings"

	gocommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func Operations(ctx gocommon.FileContext) []model.Operation {
	if !gocommon.HasImport(ctx.File, "net/http") {
		return nil
	}
	var operations []model.Operation
	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		name := gocommon.CallName(call.Fun)
		if name != "http.HandleFunc" && name != "http.Handle" {
			return
		}
		pattern, ok := gocommon.StringArg(call, 0)
		if !ok {
			return
		}
		method, route := gocommon.ParseRoutePattern(pattern)
		handler := ""
		if len(call.Args) > 1 {
			handler = gocommon.HandlerName(call.Args[1])
		}
		operations = append(operations, gocommon.Operation(ctx.ServiceName, "SERVER", "http", method, route, handler, ctx.SourcePath, "go-net-http", confidenceForPattern(pattern)))
	})
	return operations
}

func confidenceForPattern(pattern string) string {
	fields := strings.Fields(pattern)
	if len(fields) >= 2 && gocommon.IsHTTPMethod(fields[0]) {
		return "high"
	}
	return "medium"
}
