package routing

import (
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	javacommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	createContextRE    = regexp.MustCompile(`\.createContext\s*\(\s*"([^"]+)"`)
	routeCallRE        = regexp.MustCompile(`(?m)(?:^|[.\s])(?i:get|post|put|delete|patch|options|head)\s*\(\s*"(/[^"]*)"`)
	routeMethodFirstRE = regexp.MustCompile(`(?i)\b(?:route|addRoute|register|handle)\s*\(\s*"(GET|POST|PUT|DELETE|PATCH|OPTIONS|HEAD|ANY)"\s*,\s*"(/[^"]*)"`)
	routePathFirstRE   = regexp.MustCompile(`(?i)\b(?:route|addRoute|register|handle)\s*\(\s*"(/[^"]*)"\s*,\s*"(GET|POST|PUT|DELETE|PATCH|OPTIONS|HEAD|ANY)"`)
)

func Operations(serviceName string, sourcePath string, source string) []model.Operation {
	var operations []model.Operation
	operations = append(operations, httpServerOperations(serviceName, sourcePath, source)...)
	operations = append(operations, routeRegistrationOperations(serviceName, sourcePath, source)...)
	return operations
}

func httpServerOperations(serviceName string, sourcePath string, source string) []model.Operation {
	var operations []model.Operation
	for _, match := range createContextRE.FindAllStringSubmatch(source, -1) {
		path := match[1]
		if strings.HasPrefix(path, "/") {
			operations = append(operations, javacommon.Operation(serviceName, "SERVER", "http", "ANY", basecommon.NormalizeRoute(path), "HttpServer.createContext", sourcePath, "jdk-httpserver", "medium"))
		}
	}
	return operations
}

func routeRegistrationOperations(serviceName string, sourcePath string, source string) []model.Operation {
	if !hasRouteServerHint(source) {
		return nil
	}
	var operations []model.Operation
	for _, match := range routeCallRE.FindAllStringSubmatch(source, -1) {
		method := strings.ToUpper(routeMethodFromCall(match[0]))
		operations = append(operations, javacommon.Operation(serviceName, "SERVER", "http", method, basecommon.NormalizeRoute(match[1]), "route registration", sourcePath, "java-route-registration", "medium"))
	}
	for _, match := range routeMethodFirstRE.FindAllStringSubmatch(source, -1) {
		operations = append(operations, javacommon.Operation(serviceName, "SERVER", "http", strings.ToUpper(match[1]), basecommon.NormalizeRoute(match[2]), "route registration", sourcePath, "java-route-registration", "medium"))
	}
	for _, match := range routePathFirstRE.FindAllStringSubmatch(source, -1) {
		operations = append(operations, javacommon.Operation(serviceName, "SERVER", "http", strings.ToUpper(match[2]), basecommon.NormalizeRoute(match[1]), "route registration", sourcePath, "java-route-registration", "medium"))
	}
	return operations
}

func hasRouteServerHint(source string) bool {
	for _, hint := range []string{"Javalin", "Spark", "Router", "Route", "Routing", "HttpServer", "Undertow", "Vertx", "Ratpack", "Jooby", "ContextHandler"} {
		if strings.Contains(source, hint) {
			return true
		}
	}
	return false
}

func routeMethodFromCall(call string) string {
	lower := strings.ToLower(call)
	for _, method := range []string{"get", "post", "put", "delete", "patch", "options", "head"} {
		if strings.Contains(lower, method+"(") {
			return strings.ToUpper(method)
		}
	}
	return "ANY"
}
