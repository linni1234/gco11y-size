package servlet

import (
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	javacommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	webServletRE        = regexp.MustCompile(`@WebServlet\s*\(([^)]*)\)`)
	webXMLURLPatternRE  = regexp.MustCompile(`(?is)<url-pattern>\s*([^<]+)\s*</url-pattern>`)
	servletAddMappingRE = regexp.MustCompile(`\.addMapping\s*\(([^)]*)\)`)
)

func Operations(serviceName string, sourcePath string, source string) []model.Operation {
	var operations []model.Operation
	for _, match := range webServletRE.FindAllStringSubmatch(source, -1) {
		for _, path := range javacommon.ExtractAnnotationStrings(match[1]) {
			if strings.HasPrefix(path, "/") {
				operations = append(operations, javacommon.Operation(serviceName, "SERVER", "http", "ANY", basecommon.NormalizeRoute(path), "servlet", sourcePath, "servlet", "high"))
			}
		}
	}
	for _, match := range servletAddMappingRE.FindAllStringSubmatch(source, -1) {
		for _, path := range javacommon.ExtractStringLiterals(match[1]) {
			if strings.HasPrefix(path, "/") {
				operations = append(operations, javacommon.Operation(serviceName, "SERVER", "http", "ANY", basecommon.NormalizeRoute(path), "servlet dynamic mapping", sourcePath, "servlet", "medium"))
			}
		}
	}
	return operations
}

func WebXMLOperations(serviceName string, sourcePath string, source string) []model.Operation {
	var operations []model.Operation
	for _, match := range webXMLURLPatternRE.FindAllStringSubmatch(source, -1) {
		path := strings.TrimSpace(match[1])
		if strings.HasPrefix(path, "/") {
			operations = append(operations, javacommon.Operation(serviceName, "SERVER", "http", "ANY", basecommon.NormalizeRoute(path), "web.xml servlet mapping", sourcePath, "servlet-webxml", "high"))
		}
	}
	return operations
}
