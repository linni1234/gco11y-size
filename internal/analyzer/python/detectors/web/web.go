package web

import (
	"regexp"
	"strings"

	pycommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	fastAPIAppRE      = regexp.MustCompile(`(?m)\b([A-Za-z_][A-Za-z0-9_]*)\s*=\s*FastAPI\s*\(`)
	routerAssignRE    = regexp.MustCompile(`(?m)\b([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(?:APIRouter|Blueprint)\s*\((.*?)\)`)
	blueprintAssignRE = regexp.MustCompile(`(?m)\b([A-Za-z_][A-Za-z0-9_]*)\s*=\s*Blueprint\s*\((.*?)\)`)
	decoratorCallRE   = regexp.MustCompile(`^@([A-Za-z_][A-Za-z0-9_]*)(?:\.([A-Za-z_][A-Za-z0-9_]*))?\s*(?:\((.*)\))?$`)
	djangoURLConfRE   = regexp.MustCompile(`(?m)\burlpatterns\s*=`)
	tornadoTupleRE    = regexp.MustCompile(`\(\s*r?["']([^"']+)["']\s*,\s*[A-Za-z_][A-Za-z0-9_]*`)
	healthStringRE    = regexp.MustCompile(`(?i)["'](/(?:health|healthz|ready|readyz|live|livez|metrics))["']`)
)

var httpMethods = map[string]string{
	"get":     "GET",
	"post":    "POST",
	"put":     "PUT",
	"delete":  "DELETE",
	"patch":   "PATCH",
	"head":    "HEAD",
	"options": "OPTIONS",
}

func Operations(ctx pycommon.FileContext) []model.Operation {
	var operations []model.Operation
	operations = append(operations, decoratorOperations(ctx)...)
	operations = append(operations, registrationOperations(ctx)...)
	operations = append(operations, djangoOperations(ctx)...)
	operations = append(operations, bestEffortOperations(ctx)...)
	operations = append(operations, graphQLOperations(ctx)...)
	return operations
}

func ConfigFindings(ctx pycommon.FileContext) []model.ConfigFinding {
	var findings []model.ConfigFinding
	for _, framework := range []struct {
		name string
		hint bool
	}{
		{name: "fastapi", hint: pycommon.HasImport(ctx.Source, "fastapi") || strings.Contains(ctx.Source, "FastAPI(")},
		{name: "starlette", hint: pycommon.HasImport(ctx.Source, "starlette")},
		{name: "flask", hint: pycommon.HasImport(ctx.Source, "flask") || strings.Contains(ctx.Source, "Flask(")},
		{name: "quart", hint: pycommon.HasImport(ctx.Source, "quart")},
		{name: "django", hint: pycommon.HasImport(ctx.Source, "django") || djangoURLConfRE.MatchString(ctx.Source) || strings.Contains(ctx.SourcePath, "manage.py")},
		{name: "django-rest-framework", hint: pycommon.HasImport(ctx.Source, "rest_framework") || strings.Contains(ctx.Source, "DefaultRouter(") || strings.Contains(ctx.Source, "SimpleRouter(")},
		{name: "sanic", hint: pycommon.HasImport(ctx.Source, "sanic")},
		{name: "aiohttp", hint: pycommon.HasImport(ctx.Source, "aiohttp")},
		{name: "tornado", hint: pycommon.HasImport(ctx.Source, "tornado")},
		{name: "falcon", hint: pycommon.HasImport(ctx.Source, "falcon")},
		{name: "bottle", hint: pycommon.HasImport(ctx.Source, "bottle")},
		{name: "litestar", hint: pycommon.HasImport(ctx.Source, "litestar", "starlite")},
		{name: "graphql", hint: hasGraphQLHint(ctx.Source)},
	} {
		if framework.hint {
			findings = append(findings, model.ConfigFinding{Kind: "python-web-framework", Name: framework.name, Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
		}
	}
	for _, value := range pycommon.ExtractStringLiterals(ctx.Source) {
		if strings.Contains(strings.ToLower(value), "module:app") || strings.Contains(value, "gunicorn") || strings.Contains(value, "uvicorn") {
			findings = append(findings, model.ConfigFinding{Kind: "python-runtime", Name: "asgi-wsgi-target", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
			break
		}
	}
	return findings
}

func decoratorOperations(ctx pycommon.FileContext) []model.Operation {
	if !hasWebHint(ctx.Source) {
		return nil
	}
	prefixes := collectRoutePrefixes(ctx.Source)
	var operations []model.Operation
	for _, block := range pycommon.DecoratedBlocks(ctx.Source) {
		for _, decorator := range block.Decorators {
			receiver, methodName, args, ok := parseDecorator(decorator)
			if !ok {
				continue
			}
			lowerMethod := strings.ToLower(methodName)
			switch {
			case httpMethods[lowerMethod] != "":
				route, ok := pycommon.FirstString(args)
				if !ok {
					route = "/"
				}
				detector := detectorForReceiver(ctx.Source, receiver)
				operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", httpMethods[lowerMethod], prefixedRoute(prefixes, receiver, route), block.Name, ctx.SourcePath, detector, "high"))
			case lowerMethod == "route":
				route, ok := pycommon.FirstString(args)
				if !ok {
					continue
				}
				methods := methodsFromArgs(args)
				if len(methods) == 0 {
					methods = []string{"GET"}
				}
				for _, method := range methods {
					if !pycommon.IsHTTPMethod(method) {
						continue
					}
					detector := detectorForReceiver(ctx.Source, receiver)
					operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", strings.ToUpper(method), prefixedRoute(prefixes, receiver, route), block.Name, ctx.SourcePath, detector, "high"))
				}
			case lowerMethod == "websocket" || lowerMethod == "websocket_route":
				route, ok := pycommon.FirstString(args)
				if !ok {
					continue
				}
				operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "websocket", "CONNECT", prefixedRoute(prefixes, receiver, route), block.Name, ctx.SourcePath, detectorForReceiver(ctx.Source, receiver), "medium"))
			case receiver == "" && (strings.EqualFold(methodName, "get") || strings.EqualFold(methodName, "post") || strings.EqualFold(methodName, "put") || strings.EqualFold(methodName, "patch") || strings.EqualFold(methodName, "delete")):
				route, ok := pycommon.FirstString(args)
				if ok {
					operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", strings.ToUpper(methodName), route, block.Name, ctx.SourcePath, "litestar", "medium"))
				}
			}
		}
	}
	return operations
}

func registrationOperations(ctx pycommon.FileContext) []model.Operation {
	prefixes := collectRoutePrefixes(ctx.Source)
	var operations []model.Operation
	for _, call := range pycommon.Calls(ctx.Source, "add_api_route", "add_url_rule", "add_route") {
		route, ok := pycommon.FirstString(call.Args)
		if !ok {
			continue
		}
		methods := methodsFromArgs(call.Args)
		if len(methods) == 0 {
			methods = []string{"ANY"}
		}
		for _, method := range methods {
			if method != "ANY" && !pycommon.IsHTTPMethod(method) {
				continue
			}
			operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", method, prefixedRoute(prefixes, call.Receiver, route), call.Name, ctx.SourcePath, detectorForReceiver(ctx.Source, call.Receiver), "medium"))
		}
	}
	for _, call := range pycommon.Calls(ctx.Source, "Route", "WebSocketRoute", "Mount") {
		route, ok := pycommon.FirstString(call.Args)
		if !ok {
			continue
		}
		protocol := "http"
		method := "ANY"
		detector := "starlette"
		if call.Name == "WebSocketRoute" {
			protocol = "websocket"
			method = "CONNECT"
		}
		operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", protocol, method, route, call.Name, ctx.SourcePath, detector, "medium"))
	}
	return operations
}

func djangoOperations(ctx pycommon.FileContext) []model.Operation {
	if !pycommon.HasImport(ctx.Source, "django", "rest_framework") && !djangoURLConfRE.MatchString(ctx.Source) && !strings.Contains(ctx.Source, "DefaultRouter(") {
		return nil
	}
	var operations []model.Operation
	for _, call := range pycommon.Calls(ctx.Source, "path", "re_path", "url") {
		route, ok := pycommon.FirstString(call.Args)
		if !ok || strings.Contains(call.Args, "include(") || strings.HasPrefix(strings.ToLower(route), "admin") {
			continue
		}
		detector := "django"
		confidence := "medium"
		if call.Name == "re_path" || call.Name == "url" {
			confidence = "low"
		}
		operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", "ANY", route, "Django URL pattern", ctx.SourcePath, detector, confidence))
	}
	for _, call := range pycommon.Calls(ctx.Source, "register") {
		if !strings.Contains(strings.ToLower(call.Receiver), "router") && !strings.Contains(ctx.Source, "DefaultRouter(") && !strings.Contains(ctx.Source, "SimpleRouter(") {
			continue
		}
		route, ok := pycommon.FirstString(call.Args)
		if !ok {
			continue
		}
		operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", "ANY", route, "DRF router", ctx.SourcePath, "django-rest-framework", "medium"))
	}
	for _, block := range pycommon.DecoratedBlocks(ctx.Source) {
		for _, decorator := range block.Decorators {
			_, name, args, ok := parseDecorator(decorator)
			if !ok {
				continue
			}
			if name == "action" {
				route, ok := pycommon.KeywordString(args, "url_path")
				if !ok {
					route = block.Name
				}
				methods := methodsFromArgs(args)
				if len(methods) == 0 {
					methods = []string{"ANY"}
				}
				for _, method := range methods {
					operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", method, route, block.Name, ctx.SourcePath, "django-rest-framework", "low"))
				}
			}
		}
	}
	return operations
}

func bestEffortOperations(ctx pycommon.FileContext) []model.Operation {
	var operations []model.Operation
	if pycommon.HasImport(ctx.Source, "aiohttp") {
		for _, call := range pycommon.Calls(ctx.Source, "get", "post", "put", "delete", "patch", "route") {
			route, ok := pycommon.FirstString(call.Args)
			if !ok || (!strings.EqualFold(call.Receiver, "web") && call.Receiver != "") {
				continue
			}
			method := strings.ToUpper(call.Name)
			if method == "ROUTE" {
				method = "ANY"
			}
			operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", method, route, "aiohttp route", ctx.SourcePath, "aiohttp", "medium"))
		}
	}
	if pycommon.HasImport(ctx.Source, "tornado") {
		for _, match := range tornadoTupleRE.FindAllStringSubmatch(ctx.Source, -1) {
			operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", "ANY", match[1], "Tornado URLSpec", ctx.SourcePath, "tornado", "low"))
		}
	}
	if pycommon.HasImport(ctx.Source, "falcon") {
		for _, call := range pycommon.Calls(ctx.Source, "add_route") {
			route, ok := pycommon.FirstString(call.Args)
			if ok {
				operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", "ANY", route, "Falcon route", ctx.SourcePath, "falcon", "medium"))
			}
		}
	}
	if pycommon.HasImport(ctx.Source, "bottle") {
		for _, match := range healthStringRE.FindAllStringSubmatch(ctx.Source, -1) {
			operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", "ANY", match[1], "health route", ctx.SourcePath, "bottle", "low"))
		}
	}
	return operations
}

func graphQLOperations(ctx pycommon.FileContext) []model.Operation {
	if !hasGraphQLHint(ctx.Source) {
		return nil
	}
	route := "/graphql"
	for _, value := range pycommon.ExtractStringLiterals(ctx.Source) {
		if strings.HasPrefix(value, "/") && strings.Contains(strings.ToLower(value), "graphql") {
			route = value
			break
		}
	}
	return []model.Operation{pycommon.Operation(ctx.ServiceName, "SERVER", "http", "POST", route, "GraphQL endpoint", ctx.SourcePath, "graphql", "medium")}
}

func collectRoutePrefixes(source string) map[string]string {
	prefixes := map[string]string{"app": "", "application": ""}
	for _, match := range fastAPIAppRE.FindAllStringSubmatch(source, -1) {
		prefix := ""
		if args, _, ok := argsAfterMatch(source, match[0]); ok {
			if value, ok := pycommon.KeywordString(args, "root_path"); ok {
				prefix = value
			}
		}
		prefixes[match[1]] = prefix
	}
	for _, match := range routerAssignRE.FindAllStringSubmatch(source, -1) {
		prefix := ""
		if value, ok := pycommon.KeywordString(match[2], "prefix"); ok {
			prefix = value
		}
		if value, ok := pycommon.KeywordString(match[2], "url_prefix"); ok {
			prefix = value
		}
		prefixes[match[1]] = prefix
	}
	for _, match := range blueprintAssignRE.FindAllStringSubmatch(source, -1) {
		if value, ok := pycommon.KeywordString(match[2], "url_prefix"); ok {
			prefixes[match[1]] = value
		}
	}
	for _, call := range pycommon.Calls(source, "include_router", "register_blueprint") {
		child, ok := pycommon.FirstIdentifier(call.Args)
		if !ok {
			continue
		}
		prefix := ""
		if value, ok := pycommon.KeywordString(call.Args, "prefix"); ok {
			prefix = value
		}
		if value, ok := pycommon.KeywordString(call.Args, "url_prefix"); ok {
			prefix = value
		}
		parentPrefix := prefixes[call.Receiver]
		childPrefix := prefixes[child]
		prefixes[child] = pycommon.JoinRoutes(parentPrefix, prefix, childPrefix)
	}
	return prefixes
}

func parseDecorator(decorator string) (string, string, string, bool) {
	match := decoratorCallRE.FindStringSubmatch(strings.TrimSpace(decorator))
	if len(match) != 4 {
		return "", "", "", false
	}
	receiver := match[1]
	method := match[2]
	args := match[3]
	if method == "" {
		method = receiver
		receiver = ""
	}
	return receiver, method, args, true
}

func methodsFromArgs(args string) []string {
	values := pycommon.KeywordStrings(args, "methods")
	seen := map[string]bool{}
	var methods []string
	for _, value := range values {
		method := strings.ToUpper(value)
		if pycommon.IsHTTPMethod(method) && !seen[method] {
			seen[method] = true
			methods = append(methods, method)
		}
	}
	return methods
}

func prefixedRoute(prefixes map[string]string, receiver string, route string) string {
	if prefix := prefixes[receiver]; prefix != "" {
		return pycommon.JoinRoutes(prefix, route)
	}
	return pycommon.NormalizeRoute(route)
}

func detectorForReceiver(source string, receiver string) string {
	switch {
	case pycommon.HasImport(source, "flask"):
		return "flask"
	case pycommon.HasImport(source, "quart"):
		return "quart"
	case pycommon.HasImport(source, "sanic"):
		return "sanic"
	case pycommon.HasImport(source, "fastapi"):
		return "fastapi"
	case pycommon.HasImport(source, "starlette"):
		return "starlette"
	case receiver != "":
		return "python-web"
	default:
		return "python-web"
	}
}

func hasWebHint(source string) bool {
	return pycommon.HasImport(source, "fastapi", "starlette", "flask", "quart", "sanic", "litestar", "starlite", "bottle", "falcon") ||
		strings.Contains(source, "FastAPI(") ||
		strings.Contains(source, "Flask(") ||
		strings.Contains(source, "Blueprint(") ||
		strings.Contains(source, "APIRouter(")
}

func hasGraphQLHint(source string) bool {
	lower := strings.ToLower(source)
	return pycommon.HasImport(source, "graphene", "ariadne", "strawberry") ||
		strings.Contains(lower, "graphql") ||
		strings.Contains(source, "GraphQL")
}

func argsAfterMatch(source string, matched string) (string, int, bool) {
	idx := strings.Index(source, matched)
	if idx < 0 {
		return "", 0, false
	}
	open := strings.Index(source[idx:], "(")
	if open < 0 {
		return "", 0, false
	}
	return pycommon.ReadBalanced(source, idx+open, '(', ')')
}
