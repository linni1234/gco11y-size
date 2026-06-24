package gin

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
	if !gocommon.HasImport(ctx.File, "github.com/gin-gonic/gin") {
		return nil
	}
	receivers := gocommon.ImportReceivers(ctx.File, "github.com/gin-gonic/gin")
	roots := gocommon.CollectRootVarsFromReceivers(ctx.File, receivers, "Default", "New")
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
		operations = append(operations, gocommon.Operation(ctx.ServiceName, "SERVER", "http", method, route, handler, ctx.SourcePath, "gin", "high"))
	})
	return operations
}

type routeTemplate struct {
	Method  string
	Route   string
	Handler string
	Source  string
}

func ProjectOperations(files []gocommon.FileContext) []model.Operation {
	templates := map[string][]routeTemplate{}
	for _, ctx := range files {
		for key, routes := range registrationTemplates(ctx) {
			templates[key] = append(templates[key], routes...)
		}
	}
	if len(templates) == 0 {
		return nil
	}

	var operations []model.Operation
	for _, ctx := range files {
		receivers := gocommon.ImportReceivers(ctx.File, "github.com/gin-gonic/gin")
		roots := gocommon.CollectRootVarsFromReceivers(ctx.File, receivers, "Default", "New")
		groups := gocommon.CollectGroupPrefixesForRoots(ctx.File, roots)
		gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
			if len(call.Args) == 0 {
				return
			}
			routes := templates[gocommon.SelectorName(call)]
			if len(routes) == 0 {
				routes = templates[gocommon.CallName(call.Fun)]
			}
			if len(routes) == 0 {
				return
			}
			prefix, ok := routePrefixFromExpr(call.Args[0], roots, groups)
			if !ok {
				return
			}
			for _, route := range routes {
				operations = append(operations, gocommon.Operation(ctx.ServiceName, "SERVER", "http", route.Method, gocommon.JoinRoutes(prefix, route.Route), route.Handler, route.Source, "gin-registration", "medium"))
			}
		})
	}
	return operations
}

func registrationTemplates(ctx gocommon.FileContext) map[string][]routeTemplate {
	out := map[string][]routeTemplate{}
	if !gocommon.HasImport(ctx.File, "github.com/gin-gonic/gin") {
		return out
	}
	receivers := gocommon.ImportReceivers(ctx.File, "github.com/gin-gonic/gin")
	for _, decl := range ctx.File.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Type == nil || fn.Type.Params == nil {
			continue
		}
		routerParams := ginRouterParams(fn, receivers)
		if len(routerParams) == 0 {
			continue
		}
		routes := routesInFunction(ctx, fn, routerParams)
		if len(routes) == 0 {
			continue
		}
		out[fn.Name.Name] = append(out[fn.Name.Name], routes...)
		if ctx.File.Name != nil {
			out[ctx.File.Name.Name+"."+fn.Name.Name] = append(out[ctx.File.Name.Name+"."+fn.Name.Name], routes...)
		}
	}
	return out
}

func ginRouterParams(fn *ast.FuncDecl, receivers map[string]bool) map[string]bool {
	params := map[string]bool{}
	for _, field := range fn.Type.Params.List {
		if !isGinRouterType(field.Type, receivers) {
			continue
		}
		for _, name := range field.Names {
			params[name.Name] = true
		}
	}
	return params
}

func isGinRouterType(expr ast.Expr, receivers map[string]bool) bool {
	switch value := expr.(type) {
	case *ast.StarExpr:
		return isGinRouterType(value.X, receivers)
	case *ast.SelectorExpr:
		ident, ok := value.X.(*ast.Ident)
		return ok && receivers[ident.Name] && (value.Sel.Name == "RouterGroup" || value.Sel.Name == "Engine")
	}
	return false
}

func routesInFunction(ctx gocommon.FileContext, fn *ast.FuncDecl, roots map[string]bool) []routeTemplate {
	groups := collectGroupPrefixesInNode(fn.Body, roots)
	var routes []routeTemplate
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		method, ok := methods[gocommon.SelectorName(call)]
		if !ok {
			return true
		}
		receiver := gocommon.ReceiverName(call)
		if !gocommon.ReceiverAllowed(receiver, roots, groups) {
			return true
		}
		path, ok := gocommon.StringArg(call, 0)
		if !ok {
			return true
		}
		handler := ""
		if len(call.Args) > 1 {
			handler = gocommon.HandlerName(call.Args[1])
		}
		routes = append(routes, routeTemplate{
			Method:  method,
			Route:   gocommon.RouteForReceiver(groups, receiver, path),
			Handler: handler,
			Source:  ctx.SourcePath,
		})
		return true
	})
	return routes
}

func collectGroupPrefixesInNode(root ast.Node, roots map[string]bool) map[string]string {
	groups := map[string]string{}
	ast.Inspect(root, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || gocommon.SelectorName(call) != "Group" {
			return true
		}
		path, ok := gocommon.StringArg(call, 0)
		if !ok {
			return true
		}
		assign := enclosingAssign(root, call)
		if assign == nil || len(assign.Lhs) == 0 {
			return true
		}
		ident, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		receiver := gocommon.ReceiverName(call)
		if !gocommon.ReceiverAllowed(receiver, roots, groups) {
			return true
		}
		groups[ident.Name] = gocommon.RouteForReceiver(groups, receiver, path)
		return true
	})
	return groups
}

func routePrefixFromExpr(expr ast.Expr, roots map[string]bool, groups map[string]string) (string, bool) {
	switch value := expr.(type) {
	case *ast.Ident:
		if roots[value.Name] {
			return "/", true
		}
		if prefix := groups[value.Name]; prefix != "" {
			return prefix, true
		}
	case *ast.CallExpr:
		if gocommon.SelectorName(value) == "Group" {
			path, ok := gocommon.StringArg(value, 0)
			if !ok {
				return "", false
			}
			receiver := gocommon.ReceiverName(value)
			if gocommon.ReceiverAllowed(receiver, roots, groups) {
				return gocommon.RouteForReceiver(groups, receiver, path), true
			}
		}
	}
	return "", false
}

func enclosingAssign(root ast.Node, target *ast.CallExpr) *ast.AssignStmt {
	var found *ast.AssignStmt
	ast.Inspect(root, func(node ast.Node) bool {
		if found != nil {
			return false
		}
		assign, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, rhs := range assign.Rhs {
			if rhs == target {
				found = assign
				return false
			}
		}
		return true
	})
	return found
}
