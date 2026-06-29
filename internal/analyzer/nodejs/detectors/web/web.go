package web

import (
	"path/filepath"
	"regexp"
	"strings"

	nodecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	routerAssignRE     = regexp.MustCompile(`(?m)\b(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(?:express\.)?Router\s*\(`)
	routeChainRE       = regexp.MustCompile(`(?s)\.route\s*\(\s*['"` + "`" + `]([^'"` + "`" + `]+)['"` + "`" + `]\s*\)\s*\.\s*(get|post|put|delete|patch|head|options|all)\s*\(`)
	nestControllerRE   = regexp.MustCompile(`(?s)@Controller\s*(?:\(\s*['"` + "`" + `]([^'"` + "`" + `]*)['"` + "`" + `]\s*\))?.*?class\s+([A-Za-z_$][A-Za-z0-9_$]*)[^{]*\{(.*?)\n\}`)
	nestMethodRE       = regexp.MustCompile(`(?s)@(Get|Post|Put|Delete|Patch|Head|Options|All)\s*(?:\(\s*['"` + "`" + `]([^'"` + "`" + `]*)['"` + "`" + `]\s*\))?\s*(?:public|private|protected|async|\s)*([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	nestGlobalPrefixRE = regexp.MustCompile(`\.setGlobalPrefix\s*\(\s*['"` + "`" + `]([^'"` + "`" + `]+)['"` + "`" + `]\s*\)`)
	exportMethodRE     = regexp.MustCompile(`(?m)\bexport\s+(?:async\s+)?function\s+(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s*\(`)
	exportConstRE      = regexp.MustCompile(`(?m)\bexport\s+const\s+(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\b`)
)

func Operations(ctx nodecommon.FileContext) []model.Operation {
	var operations []model.Operation
	operations = append(operations, expressLikeOperations(ctx)...)
	operations = append(operations, fastifyOperations(ctx)...)
	operations = append(operations, hapiOperations(ctx)...)
	operations = append(operations, nestOperations(ctx)...)
	operations = append(operations, nextOperations(ctx)...)
	operations = append(operations, graphQLOperations(ctx)...)
	return operations
}

func ConfigFindings(ctx nodecommon.FileContext) []model.ConfigFinding {
	var findings []model.ConfigFinding
	for _, framework := range []struct {
		name string
		hint bool
	}{
		{name: "express", hint: nodecommon.HasImport(ctx.Source, "express")},
		{name: "fastify", hint: nodecommon.HasImport(ctx.Source, "fastify")},
		{name: "nestjs", hint: nodecommon.HasImport(ctx.Source, "@nestjs/")},
		{name: "nextjs", hint: strings.Contains(ctx.SourcePath, "next.config.") || strings.Contains(ctx.SourcePath, "/app/") || strings.Contains(ctx.SourcePath, "/pages/api/")},
		{name: "koa", hint: nodecommon.HasImport(ctx.Source, "koa", "@koa/router")},
		{name: "hapi", hint: nodecommon.HasImport(ctx.Source, "@hapi/hapi", "hapi")},
		{name: "hono", hint: nodecommon.HasImport(ctx.Source, "hono")},
		{name: "graphql", hint: strings.Contains(ctx.Source, "ApolloServer") || strings.Contains(ctx.Source, "graphqlHTTP")},
	} {
		if framework.hint {
			findings = append(findings, model.ConfigFinding{Kind: "nodejs-web-framework", Name: framework.name, Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
		}
	}
	return findings
}

func expressLikeOperations(ctx nodecommon.FileContext) []model.Operation {
	if !nodecommon.HasImport(ctx.Source, "express", "koa", "@koa/router", "hono") && !strings.Contains(ctx.Source, "Router(") {
		return nil
	}
	prefixes := collectRouterPrefixes(ctx.Source)
	var operations []model.Operation
	for _, call := range nodecommon.Calls(ctx.Source, "get", "post", "put", "delete", "patch", "head", "options", "all") {
		if !receiverLooksLikeRouter(call.Receiver, prefixes) {
			continue
		}
		route, ok := nodecommon.FirstString(call.Args)
		if !ok {
			continue
		}
		method := strings.ToUpper(call.Name)
		if method == "ALL" {
			method = "ANY"
		}
		detector := "express"
		if nodecommon.HasImport(ctx.Source, "koa", "@koa/router") {
			detector = "koa"
		}
		if nodecommon.HasImport(ctx.Source, "hono") {
			detector = "hono"
		}
		operations = append(operations, nodecommon.Operation(ctx.ServiceName, "SERVER", "http", method, prefixedRoute(prefixes, call.Receiver, route), "route handler", ctx.SourcePath, detector, "high"))
	}
	for _, match := range routeChainRE.FindAllStringSubmatch(ctx.Source, -1) {
		method := strings.ToUpper(match[2])
		if method == "ALL" {
			method = "ANY"
		}
		operations = append(operations, nodecommon.Operation(ctx.ServiceName, "SERVER", "http", method, match[1], "route chain", ctx.SourcePath, "express", "high"))
	}
	return operations
}

func fastifyOperations(ctx nodecommon.FileContext) []model.Operation {
	if !nodecommon.HasImport(ctx.Source, "fastify") && !strings.Contains(ctx.Source, "fastify(") {
		return nil
	}
	var operations []model.Operation
	for _, call := range nodecommon.Calls(ctx.Source, "get", "post", "put", "delete", "patch", "head", "options", "all") {
		if call.Receiver == "" {
			continue
		}
		route, ok := nodecommon.FirstString(call.Args)
		if !ok {
			continue
		}
		method := strings.ToUpper(call.Name)
		if method == "ALL" {
			method = "ANY"
		}
		operations = append(operations, nodecommon.Operation(ctx.ServiceName, "SERVER", "http", method, route, "Fastify route", ctx.SourcePath, "fastify", "high"))
	}
	for _, call := range nodecommon.Calls(ctx.Source, "route") {
		methods := nodecommon.ObjectFieldStrings(call.Args, "method")
		route, ok := nodecommon.ObjectFieldString(call.Args, "url")
		if !ok {
			route, ok = nodecommon.ObjectFieldString(call.Args, "path")
		}
		if !ok {
			continue
		}
		if len(methods) == 0 {
			methods = []string{"ANY"}
		}
		for _, method := range methods {
			if !nodecommon.IsHTTPMethod(method) {
				continue
			}
			operations = append(operations, nodecommon.Operation(ctx.ServiceName, "SERVER", "http", strings.ToUpper(method), route, "Fastify route", ctx.SourcePath, "fastify", "high"))
		}
	}
	return operations
}

func hapiOperations(ctx nodecommon.FileContext) []model.Operation {
	if !nodecommon.HasImport(ctx.Source, "@hapi/hapi", "hapi") {
		return nil
	}
	var operations []model.Operation
	for _, call := range nodecommon.Calls(ctx.Source, "route") {
		methods := nodecommon.ObjectFieldStrings(call.Args, "method")
		route, ok := nodecommon.ObjectFieldString(call.Args, "path")
		if !ok {
			continue
		}
		if len(methods) == 0 {
			methods = []string{"ANY"}
		}
		for _, method := range methods {
			if !nodecommon.IsHTTPMethod(method) {
				continue
			}
			operations = append(operations, nodecommon.Operation(ctx.ServiceName, "SERVER", "http", strings.ToUpper(method), route, "Hapi route", ctx.SourcePath, "hapi", "medium"))
		}
	}
	return operations
}

func nestOperations(ctx nodecommon.FileContext) []model.Operation {
	if !nodecommon.HasImport(ctx.Source, "@nestjs/") && !strings.Contains(ctx.Source, "@Controller") {
		return nil
	}
	globalPrefix := ""
	if match := nestGlobalPrefixRE.FindStringSubmatch(ctx.Source); len(match) == 2 {
		globalPrefix = match[1]
	}
	var operations []model.Operation
	for _, classMatch := range nestControllerRE.FindAllStringSubmatch(ctx.Source, -1) {
		base := classMatch[1]
		body := classMatch[3]
		for _, methodMatch := range nestMethodRE.FindAllStringSubmatch(body, -1) {
			method := strings.ToUpper(methodMatch[1])
			if method == "ALL" {
				method = "ANY"
			}
			route := methodMatch[2]
			fullRoute := nodecommon.JoinRoutes(base, route)
			if globalPrefix != "" {
				fullRoute = nodecommon.JoinRoutes(globalPrefix, fullRoute)
			}
			operations = append(operations, nodecommon.Operation(ctx.ServiceName, "SERVER", "http", method, fullRoute, classMatch[2]+"."+methodMatch[3], ctx.SourcePath, "nestjs", "high"))
		}
	}
	if strings.Contains(ctx.Source, "@WebSocketGateway") {
		operations = append(operations, nodecommon.Operation(ctx.ServiceName, "INTERNAL", "custom", "SPAN", "websocket-gateway", "NestJS WebSocket gateway", ctx.SourcePath, "nestjs", "low"))
	}
	return operations
}

func nextOperations(ctx nodecommon.FileContext) []model.Operation {
	path := filepath.ToSlash(ctx.SourcePath)
	var operations []model.Operation
	if (strings.Contains(path, "/app/") || strings.HasPrefix(path, "app/")) && (strings.HasSuffix(path, "/route.ts") || strings.HasSuffix(path, "/route.js")) {
		route := nextRouteFromAppPath(path)
		methods := exportedHTTPMethods(ctx.Source)
		if len(methods) == 0 {
			methods = []string{"ANY"}
		}
		for _, method := range methods {
			operations = append(operations, nodecommon.Operation(ctx.ServiceName, "SERVER", "http", method, route, "Next.js route handler", ctx.SourcePath, "nextjs-app-router", "high"))
		}
	}
	if strings.Contains(path, "/pages/api/") || strings.HasPrefix(path, "pages/api/") {
		route := nextRouteFromPagesAPIPath(path)
		methods := methodsFromPagesAPI(ctx.Source)
		if len(methods) == 0 {
			methods = []string{"ANY"}
		}
		for _, method := range methods {
			operations = append(operations, nodecommon.Operation(ctx.ServiceName, "SERVER", "http", method, route, "Next.js API route", ctx.SourcePath, "nextjs-pages-api", "medium"))
		}
	}
	return operations
}

func graphQLOperations(ctx nodecommon.FileContext) []model.Operation {
	if !strings.Contains(ctx.Source, "ApolloServer") && !strings.Contains(ctx.Source, "graphqlHTTP") {
		return nil
	}
	route := "/graphql"
	for _, value := range nodecommon.ExtractStringLiterals(ctx.Source) {
		if strings.Contains(strings.ToLower(value), "graphql") && strings.HasPrefix(value, "/") {
			route = value
			break
		}
	}
	return []model.Operation{nodecommon.Operation(ctx.ServiceName, "SERVER", "http", "POST", route, "GraphQL endpoint", ctx.SourcePath, "graphql", "medium")}
}

func collectRouterPrefixes(source string) map[string]string {
	prefixes := map[string]string{"app": "", "router": ""}
	for _, match := range routerAssignRE.FindAllStringSubmatch(source, -1) {
		prefixes[match[1]] = ""
	}
	for _, call := range nodecommon.Calls(source, "use") {
		prefix, ok := nodecommon.FirstString(call.Args)
		if !ok || !strings.HasPrefix(prefix, "/") {
			continue
		}
		for receiver := range prefixes {
			if regexp.MustCompile(`\b` + regexp.QuoteMeta(receiver) + `\b`).MatchString(call.Args[strings.Index(call.Args, prefix)+len(prefix):]) {
				prefixes[receiver] = nodecommon.JoinRoutes(prefixes[receiver], prefix)
			}
		}
	}
	return prefixes
}

func receiverLooksLikeRouter(receiver string, prefixes map[string]string) bool {
	if receiver == "" {
		return false
	}
	if _, ok := prefixes[receiver]; ok {
		return true
	}
	lower := strings.ToLower(receiver)
	return strings.Contains(lower, "router") || strings.Contains(lower, "app")
}

func prefixedRoute(prefixes map[string]string, receiver string, route string) string {
	if prefix := prefixes[receiver]; prefix != "" {
		return nodecommon.JoinRoutes(prefix, route)
	}
	return nodecommon.NormalizeRoute(route)
}

func exportedHTTPMethods(source string) []string {
	seen := map[string]bool{}
	var methods []string
	add := func(method string) {
		method = strings.ToUpper(method)
		if nodecommon.IsHTTPMethod(method) && !seen[method] {
			seen[method] = true
			methods = append(methods, method)
		}
	}
	for _, match := range exportMethodRE.FindAllStringSubmatch(source, -1) {
		add(match[1])
	}
	for _, match := range exportConstRE.FindAllStringSubmatch(source, -1) {
		add(match[1])
	}
	return methods
}

func methodsFromPagesAPI(source string) []string {
	var methods []string
	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		if strings.Contains(source, `method === "`+method+`"`) || strings.Contains(source, `method === '`+method+`'`) || strings.Contains(source, `method == "`+method+`"`) || strings.Contains(source, `method == '`+method+`'`) {
			methods = append(methods, method)
		}
	}
	return methods
}

func nextRouteFromAppPath(path string) string {
	idx := strings.Index(path, "/app/")
	if idx >= 0 {
		path = path[idx+len("/app/"):]
	} else {
		path = strings.TrimPrefix(path, "app/")
	}
	path = strings.TrimSuffix(path, "/route.ts")
	path = strings.TrimSuffix(path, "/route.js")
	path = strings.TrimSuffix(path, "/route.mjs")
	path = strings.TrimSuffix(path, "/route.tsx")
	return "/" + strings.Trim(nodecommon.NormalizeRoute(path), "/")
}

func nextRouteFromPagesAPIPath(path string) string {
	idx := strings.Index(path, "/pages/api/")
	if idx >= 0 {
		path = path[idx+len("/pages/api/"):]
	} else {
		path = strings.TrimPrefix(path, "pages/api/")
	}
	path = strings.TrimSuffix(path, filepath.Ext(path))
	if strings.HasSuffix(path, "/index") {
		path = strings.TrimSuffix(path, "/index")
	}
	return "/api/" + strings.Trim(nodecommon.NormalizeRoute(path), "/")
}
