package chi

import (
	"go/ast"
	"go/token"
	"strings"

	gocommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var methods = map[string]string{
	"Get":     "GET",
	"Post":    "POST",
	"Put":     "PUT",
	"Delete":  "DELETE",
	"Patch":   "PATCH",
	"Head":    "HEAD",
	"Options": "OPTIONS",
	"Trace":   "TRACE",
	"Connect": "CONNECT",
}

func Operations(ctx gocommon.FileContext) []model.Operation {
	if !gocommon.HasImport(ctx.File, "github.com/go-chi/chi/v5") && !gocommon.HasImport(ctx.File, "github.com/go-chi/chi") {
		return nil
	}
	receivers := gocommon.ImportReceivers(ctx.File, "github.com/go-chi/chi/v5", "github.com/go-chi/chi")
	roots := gocommon.CollectRootVarsFromReceivers(ctx.File, receivers, "NewRouter")
	groups := gocommon.CollectGroupPrefixesForRoots(ctx.File, roots)
	skip := map[token.Pos]bool{}
	var operations []model.Operation

	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		if gocommon.SelectorName(call) != "Route" {
			return
		}
		if !gocommon.ReceiverAllowed(gocommon.ReceiverName(call), roots, groups) {
			return
		}
		prefix, ok := gocommon.StringArg(call, 0)
		if !ok || len(call.Args) < 2 {
			return
		}
		funcLit, ok := call.Args[1].(*ast.FuncLit)
		if !ok || funcLit.Type == nil || len(funcLit.Type.Params.List) == 0 || len(funcLit.Type.Params.List[0].Names) == 0 {
			return
		}
		routeReceiver := funcLit.Type.Params.List[0].Names[0].Name
		ast.Inspect(funcLit.Body, func(node ast.Node) bool {
			nested, ok := node.(*ast.CallExpr)
			if !ok || gocommon.ReceiverName(nested) != routeReceiver {
				return true
			}
			op, ok := operationFromCall(ctx, nested, map[string]bool{routeReceiver: true}, map[string]string{routeReceiver: gocommon.NormalizeRoute(prefix)})
			if ok {
				operations = append(operations, op)
				skip[nested.Pos()] = true
			}
			return true
		})
	})

	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		if skip[call.Pos()] {
			return
		}
		op, ok := operationFromCall(ctx, call, roots, groups)
		if ok {
			operations = append(operations, op)
		}
	})
	return operations
}

func operationFromCall(ctx gocommon.FileContext, call *ast.CallExpr, roots map[string]bool, groups map[string]string) (model.Operation, bool) {
	receiver := gocommon.ReceiverName(call)
	if !gocommon.ReceiverAllowed(receiver, roots, groups) {
		return model.Operation{}, false
	}
	selector := gocommon.SelectorName(call)
	method, ok := methods[selector]
	if !ok {
		if selector != "Method" && selector != "MethodFunc" && selector != "Mount" {
			return model.Operation{}, false
		}
	}

	pathIndex := 0
	if selector == "Method" || selector == "MethodFunc" {
		rawMethod, ok := gocommon.StringArg(call, 0)
		if !ok {
			return model.Operation{}, false
		}
		method = strings.ToUpper(rawMethod)
		pathIndex = 1
	}
	if selector == "Mount" {
		method = "ANY"
	}
	path, ok := gocommon.StringArg(call, pathIndex)
	if !ok {
		return model.Operation{}, false
	}
	handler := ""
	if len(call.Args) > pathIndex+1 {
		handler = gocommon.HandlerName(call.Args[pathIndex+1])
	}
	route := gocommon.RouteForReceiver(groups, receiver, path)
	return gocommon.Operation(ctx.ServiceName, "SERVER", "http", method, route, handler, ctx.SourcePath, "chi", "high"), true
}
