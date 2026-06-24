package echo

import (
	"go/ast"

	gocommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var methods = map[string]string{
	"GET":     "GET",
	"POST":    "POST",
	"PUT":     "PUT",
	"DELETE":  "DELETE",
	"PATCH":   "PATCH",
	"HEAD":    "HEAD",
	"OPTIONS": "OPTIONS",
	"Any":     "ANY",
}

func Operations(ctx gocommon.FileContext) []model.Operation {
	if !gocommon.HasImport(ctx.File, "github.com/labstack/echo/v4") && !gocommon.HasImport(ctx.File, "github.com/labstack/echo") {
		return nil
	}
	receivers := gocommon.ImportReceivers(ctx.File, "github.com/labstack/echo/v4", "github.com/labstack/echo")
	roots := gocommon.CollectRootVarsFromReceivers(ctx.File, receivers, "New")
	groups := gocommon.CollectGroupPrefixesForRoots(ctx.File, roots)
	var operations []model.Operation
	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		method, ok := methods[gocommon.SelectorName(call)]
		if !ok {
			return
		}
		receiver := gocommon.ReceiverName(call)
		if !gocommon.ReceiverAllowed(receiver, roots, groups) {
			return
		}
		path, ok := gocommon.StringArg(call, 0)
		if !ok {
			return
		}
		handler := ""
		if len(call.Args) > 1 {
			handler = gocommon.HandlerName(call.Args[1])
		}
		route := gocommon.RouteForReceiver(groups, receiver, path)
		operations = append(operations, gocommon.Operation(ctx.ServiceName, "SERVER", "http", method, route, handler, ctx.SourcePath, "echo", "high"))
	})
	return operations
}
