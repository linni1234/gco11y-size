package aspnet

import (
	"path/filepath"
	"regexp"
	"strings"

	dotnetcommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	mapGroupAssignRE = regexp.MustCompile(`(?s)\b(?:var\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*MapGroup\s*(?:<[^;{}()]+>)?\s*\(`)
	pageRouteRE      = regexp.MustCompile(`(?m)^\s*@page(?:\s+"([^"]*)")?`)
)

func Operations(ctx dotnetcommon.FileContext) []model.Operation {
	var operations []model.Operation
	operations = append(operations, minimalAPIOperations(ctx)...)
	operations = append(operations, controllerOperations(ctx)...)
	return operations
}

func RazorOperations(ctx dotnetcommon.FileContext) []model.Operation {
	match := pageRouteRE.FindStringSubmatch(ctx.Source)
	if len(match) == 0 {
		return nil
	}
	route := ""
	if len(match) > 1 {
		route = match[1]
	}
	if strings.TrimSpace(route) == "" {
		route = routeFromRazorPath(ctx.SourcePath)
	}
	detector := "razor-pages"
	if strings.EqualFold(filepath.Ext(ctx.SourcePath), ".razor") {
		detector = "blazor"
	}
	return []model.Operation{dotnetcommon.Operation(ctx.ServiceName, "SERVER", "http", "GET", dotnetcommon.NormalizeRoute(route), "page route", ctx.SourcePath, detector, "medium")}
}

func minimalAPIOperations(ctx dotnetcommon.FileContext) []model.Operation {
	groups := collectMapGroups(ctx.Source)
	var operations []model.Operation
	for _, call := range dotnetcommon.Calls(ctx.Source,
		"MapGet", "MapPost", "MapPut", "MapDelete", "MapPatch", "MapMethods", "Map", "MapHealthChecks", "MapHub", "MapControllerRoute", "MapReverseProxy",
	) {
		switch call.Name {
		case "MapReverseProxy":
			route := "/{**catch-all}"
			if value, ok := dotnetcommon.FirstString(call.Args); ok {
				route = value
			}
			op := dotnetcommon.Operation(ctx.ServiceName, "SERVER", "http", "ANY", prefixedRoute(groups, call.Receiver, route), "reverse proxy route", ctx.SourcePath, "yarp", "low")
			op.Origin = "gateway"
			operations = append(operations, op)
		case "MapControllerRoute":
			route := conventionalRoute(call.Args)
			if route == "" {
				continue
			}
			operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "SERVER", "http", "ANY", route, "conventional route", ctx.SourcePath, "aspnet-core-mvc", "low"))
		case "MapHub":
			route, ok := dotnetcommon.FirstString(call.Args)
			if !ok {
				continue
			}
			handler := strings.TrimSpace(call.Generic)
			if handler == "" {
				handler = "SignalR hub"
			}
			operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "SERVER", "http", "ANY", prefixedRoute(groups, call.Receiver, route), handler, ctx.SourcePath, "signalr", "high"))
		case "MapHealthChecks":
			route, ok := dotnetcommon.FirstString(call.Args)
			if !ok {
				route = "/health"
			}
			operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "SERVER", "http", "GET", prefixedRoute(groups, call.Receiver, route), "health check", ctx.SourcePath, "aspnet-core-healthchecks", "high"))
		case "MapMethods":
			stringsInCall := dotnetcommon.ExtractStringLiterals(call.Args)
			if len(stringsInCall) == 0 {
				continue
			}
			route := stringsInCall[0]
			methods := httpMethods(stringsInCall[1:])
			if len(methods) == 0 {
				methods = []string{"ANY"}
			}
			for _, method := range methods {
				operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "SERVER", "http", method, prefixedRoute(groups, call.Receiver, route), "minimal API endpoint", ctx.SourcePath, "aspnet-core-minimal-api", "high"))
			}
		default:
			route, ok := dotnetcommon.FirstString(call.Args)
			if !ok {
				continue
			}
			method := methodForMapCall(call.Name)
			operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "SERVER", "http", method, prefixedRoute(groups, call.Receiver, route), "minimal API endpoint", ctx.SourcePath, "aspnet-core-minimal-api", "high"))
		}
	}
	return operations
}

func controllerOperations(ctx dotnetcommon.FileContext) []model.Operation {
	var operations []model.Operation
	for _, class := range dotnetcommon.FindClasses(ctx.Source) {
		if !looksLikeController(class) {
			continue
		}
		classRoutes := routeAttributes(class.Attributes)
		if len(classRoutes) == 0 {
			classRoutes = []string{""}
		}
		for _, method := range dotnetcommon.Methods(class.Body) {
			httpRoutes := httpAttributeRoutes(method.Attributes)
			if len(httpRoutes) == 0 {
				continue
			}
			for _, classRoute := range classRoutes {
				for _, httpRoute := range httpRoutes {
					route := dotnetcommon.JoinRoutes(classRoute, httpRoute.Route)
					route = dotnetcommon.ReplaceRouteTokens(route, class.Name, method.Name)
					operations = append(operations, dotnetcommon.Operation(ctx.ServiceName, "SERVER", "http", httpRoute.Method, route, class.Name+"."+method.Name, ctx.SourcePath, "aspnet-core-mvc", "high"))
				}
			}
		}
	}
	return operations
}

type httpRoute struct {
	Method string
	Route  string
}

func httpAttributeRoutes(attributes string) []httpRoute {
	var routes []httpRoute
	for _, attr := range dotnetcommon.Attributes(attributes) {
		name := strings.TrimSuffix(attr.Name, "Attribute")
		switch name {
		case "HttpGet", "HttpPost", "HttpPut", "HttpDelete", "HttpPatch", "HttpHead", "HttpOptions":
			method := strings.TrimPrefix(name, "Http")
			route := ""
			if value, ok := dotnetcommon.FirstString(attr.Args); ok {
				route = value
			}
			routes = append(routes, httpRoute{Method: strings.ToUpper(method), Route: route})
		case "AcceptVerbs":
			values := dotnetcommon.ExtractStringLiterals(attr.Args)
			methods := httpMethods(values)
			route := ""
			if named, ok := dotnetcommon.NamedString(attr.Args, "Route"); ok {
				route = named
			}
			if len(methods) == 0 {
				methods = []string{"ANY"}
			}
			for _, method := range methods {
				routes = append(routes, httpRoute{Method: method, Route: route})
			}
		case "Route":
			if value, ok := dotnetcommon.FirstString(attr.Args); ok {
				routes = append(routes, httpRoute{Method: "ANY", Route: value})
			}
		}
	}
	return routes
}

func routeAttributes(attributes string) []string {
	var routes []string
	for _, attr := range dotnetcommon.Attributes(attributes) {
		name := strings.TrimSuffix(attr.Name, "Attribute")
		if name != "Route" {
			continue
		}
		if value, ok := dotnetcommon.FirstString(attr.Args); ok {
			routes = append(routes, value)
		}
	}
	return routes
}

func looksLikeController(class dotnetcommon.ClassBlock) bool {
	if strings.HasSuffix(class.Name, "Controller") {
		return true
	}
	if strings.Contains(class.Bases, "ControllerBase") || strings.Contains(class.Bases, "Controller") {
		return true
	}
	for _, attr := range dotnetcommon.Attributes(class.Attributes) {
		if strings.TrimSuffix(attr.Name, "Attribute") == "ApiController" {
			return true
		}
	}
	return false
}

func collectMapGroups(source string) map[string]string {
	groups := map[string]string{}
	matches := mapGroupAssignRE.FindAllStringSubmatchIndex(source, -1)
	for pass := 0; pass < 4; pass++ {
		changed := false
		for _, match := range matches {
			name := source[match[2]:match[3]]
			receiver := source[match[4]:match[5]]
			open := match[1] - 1
			args, _, ok := dotnetcommon.ReadBalanced(source, open, '(', ')')
			if !ok {
				continue
			}
			route, ok := dotnetcommon.FirstString(args)
			if !ok {
				continue
			}
			prefix := dotnetcommon.NormalizeRoute(route)
			if parent := groups[receiver]; parent != "" {
				prefix = dotnetcommon.JoinRoutes(parent, prefix)
			} else if receiver != "app" && receiver != "endpoints" && receiver != "builder" {
				continue
			}
			if groups[name] != prefix {
				groups[name] = prefix
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return groups
}

func prefixedRoute(groups map[string]string, receiver string, route string) string {
	route = dotnetcommon.NormalizeRoute(route)
	if prefix := groups[receiver]; prefix != "" {
		return dotnetcommon.JoinRoutes(prefix, route)
	}
	return route
}

func methodForMapCall(name string) string {
	switch name {
	case "MapGet":
		return "GET"
	case "MapPost":
		return "POST"
	case "MapPut":
		return "PUT"
	case "MapDelete":
		return "DELETE"
	case "MapPatch":
		return "PATCH"
	default:
		return "ANY"
	}
}

func httpMethods(values []string) []string {
	var methods []string
	for _, value := range values {
		if dotnetcommon.IsHTTPMethod(value) {
			methods = append(methods, strings.ToUpper(value))
		}
	}
	return methods
}

func conventionalRoute(args string) string {
	if value, ok := dotnetcommon.NamedString(args, "pattern"); ok {
		return dotnetcommon.NormalizeRoute(strings.ReplaceAll(strings.ReplaceAll(value, "{controller=Home}", "{controller}"), "{action=Index}", "{action}"))
	}
	for _, value := range dotnetcommon.ExtractStringLiterals(args) {
		if strings.Contains(value, "{controller") || strings.Contains(value, "{action") {
			return dotnetcommon.NormalizeRoute(strings.ReplaceAll(strings.ReplaceAll(value, "{controller=Home}", "{controller}"), "{action=Index}", "{action}"))
		}
	}
	return ""
}

func routeFromRazorPath(sourcePath string) string {
	path := filepath.ToSlash(sourcePath)
	for _, marker := range []string{"/Pages/", "Pages/", "/Components/", "Components/"} {
		if idx := strings.Index(path, marker); idx >= 0 {
			path = path[idx+len(marker):]
			break
		}
	}
	path = strings.TrimSuffix(path, filepath.Ext(path))
	if strings.EqualFold(filepath.Base(path), "Index") {
		path = strings.TrimSuffix(filepath.Dir(path), ".")
	}
	return "/" + strings.Trim(path, "/")
}
