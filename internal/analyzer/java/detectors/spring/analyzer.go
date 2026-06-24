package spring

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type Analyzer struct{}

func New() Analyzer {
	return Analyzer{}
}

func (Analyzer) ID() string {
	return "springboot"
}

func (Analyzer) Analyze(ctx framework.Context) (framework.Result, error) {
	serviceNames := copyServiceNames(ctx.ServiceNames)
	result := framework.Result{ServiceNames: serviceNames}
	scanSpringConfig(ctx.Repo, ctx.OtelConfig, &result, serviceNames)

	javaFiles := 0
	err := filepath.WalkDir(ctx.Repo, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", path, walkErr))
			return nil
		}
		if entry.IsDir() {
			if common.ShouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".java" {
			return nil
		}
		if common.IsTestPath(ctx.Repo, path) {
			return nil
		}

		javaFiles++
		root := common.InferServiceRoot(ctx.Repo, path)
		serviceName, source := serviceNameForRoot(ctx.Repo, root, serviceNames)
		ensureService(&result, model.Service{
			Name:   serviceName,
			Root:   common.RelPath(ctx.Repo, root),
			Source: source,
		})

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", common.RelPath(ctx.Repo, path), readErr))
			return nil
		}
		parsed := parseJavaSource(serviceName, common.RelPath(ctx.Repo, path), string(content))
		result.Operations = append(result.Operations, parsed.operations...)
		result.Edges = append(result.Edges, parsed.edges...)
		result.Risks = append(result.Risks, parsed.risks...)
		return nil
	})
	if err != nil {
		return framework.Result{}, err
	}

	if javaFiles > 0 || hasSpringFindings(result.ConfigFindings) || len(result.Operations) > 0 {
		result.DetectedLanguages = append(result.DetectedLanguages, "java/spring-boot")
	}
	if javaFiles > 0 && len(result.Operations) == 0 {
		result.Warnings = append(result.Warnings, "Java files were found, but no Spring controller or listener operations were detected")
	}
	return result, nil
}

func scanSpringConfig(repo string, otelConfig string, result *framework.Result, serviceNames map[string]string) {
	scanSpringConfigPath(repo, repo, result, serviceNames)
	if strings.TrimSpace(otelConfig) == "" {
		return
	}
	path := otelConfig
	if !filepath.IsAbs(path) {
		path = filepath.Join(repo, path)
	}
	scanSpringConfigPath(repo, path, result, serviceNames)
}

func scanSpringConfigPath(repo string, path string, result *framework.Result, serviceNames map[string]string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}
	if !info.IsDir() {
		parseSpringConfigFile(repo, path, result, serviceNames)
		return
	}
	_ = filepath.WalkDir(path, func(current string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not read config path %s: %v", current, walkErr))
			return nil
		}
		if entry.IsDir() {
			if common.ShouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if common.IsConfigFile(current) {
			parseSpringConfigFile(repo, current, result, serviceNames)
		}
		return nil
	})
}

var springApplicationNameRE = regexp.MustCompile(`(?im)^\s*spring\.application\.name\s*[:=]\s*["']?([A-Za-z0-9_.-]+)["']?`)

func parseSpringConfigFile(repo string, path string, result *framework.Result, serviceNames map[string]string) {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("could not read config file %s: %v", path, err))
		return
	}
	content := string(contentBytes)
	source := common.RelPath(repo, path)
	root := common.InferServiceRoot(repo, path)

	for _, match := range springApplicationNameRE.FindAllStringSubmatch(content, -1) {
		recordSpringServiceName(repo, root, source, common.SanitizeServiceName(match[1]), result, serviceNames)
	}
	if serviceName := parseSpringYAMLServiceName(content); serviceName != "" {
		recordSpringServiceName(repo, root, source, common.SanitizeServiceName(serviceName), result, serviceNames)
	}

	serviceName, _ := serviceNameForRoot(repo, root, serviceNames)
	for _, route := range parseSpringCloudGatewayRoutes(content) {
		ensureService(result, model.Service{
			Name:   serviceName,
			Root:   common.RelPath(repo, root),
			Source: "config",
		})
		result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{
			Kind:    "gateway-route",
			Name:    route.id,
			Value:   strings.Join(route.paths, ","),
			Source:  source,
			Service: serviceName,
		})
		for _, pathPattern := range route.paths {
			result.Operations = append(result.Operations, model.Operation{
				Service: serviceName,
				Kind:    "SERVER",
				Method:  "ANY",
				Route:   common.NormalizeRoute(pathPattern),
				Handler: "gateway route " + route.id,
				Source:  source,
				Origin:  "gateway",
			})
		}
		target := gatewayTargetService(route)
		if target != "" && target != serviceName {
			result.Edges = append(result.Edges, model.Edge{
				SourceService: serviceName,
				TargetService: target,
				Protocol:      "http",
				Source:        source,
				Confidence:    "high",
			})
		}
	}
}

func recordSpringServiceName(repo string, root string, source string, serviceName string, result *framework.Result, serviceNames map[string]string) {
	if serviceName == "" {
		return
	}
	serviceNames[root] = serviceName
	result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{
		Kind:    "service-name",
		Name:    "spring.application.name",
		Value:   serviceName,
		Source:  source,
		Service: serviceName,
	})
	ensureService(result, model.Service{
		Name:   serviceName,
		Root:   common.RelPath(repo, root),
		Source: "config",
	})
}

func serviceNameForRoot(repo string, root string, serviceNames map[string]string) (string, string) {
	if serviceName := serviceNames[root]; serviceName != "" {
		return serviceName, "config"
	}
	return common.FallbackServiceName(repo, root), "path"
}

func ensureService(result *framework.Result, service model.Service) {
	if service.Name == "" {
		return
	}
	for _, existing := range result.Services {
		if existing.Name == service.Name {
			return
		}
	}
	result.Services = append(result.Services, service)
}

func hasSpringFindings(findings []model.ConfigFinding) bool {
	for _, finding := range findings {
		if finding.Name == "spring.application.name" || finding.Kind == "gateway-route" {
			return true
		}
	}
	return false
}

func copyServiceNames(in map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

type gatewayRoute struct {
	id    string
	uri   string
	paths []string
}

func parseSpringCloudGatewayRoutes(content string) []gatewayRoute {
	if !strings.Contains(strings.ToLower(content), "gateway") || !strings.Contains(strings.ToLower(content), "routes") {
		return nil
	}
	lines := strings.Split(content, "\n")
	inRoutes := false
	routesIndent := -1
	var current *gatewayRoute
	var routes []gatewayRoute

	flush := func() {
		if current == nil {
			return
		}
		current.paths = cleanGatewayPaths(current.paths)
		if current.id != "" && len(current.paths) > 0 {
			routes = append(routes, *current)
		}
		current = nil
	}

	for _, rawLine := range lines {
		line := stripYAMLComment(rawLine)
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if !inRoutes {
			if strings.TrimSuffix(trimmed, ":") == "routes" {
				inRoutes = true
				routesIndent = indent
			}
			continue
		}
		if indent <= routesIndent && !strings.HasPrefix(trimmed, "-") {
			flush()
			inRoutes = false
			routesIndent = -1
			if strings.TrimSuffix(trimmed, ":") == "routes" {
				inRoutes = true
				routesIndent = indent
			}
			continue
		}
		if strings.HasPrefix(trimmed, "- id:") {
			flush()
			current = &gatewayRoute{id: cleanYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "- id:")))}
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(trimmed, "uri:") {
			current.uri = cleanYAMLScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "uri:")))
			continue
		}
		if strings.Contains(trimmed, "Path=") {
			_, rawPaths, _ := strings.Cut(trimmed, "Path=")
			current.paths = append(current.paths, splitGatewayPathList(rawPaths)...)
			continue
		}
		if strings.HasPrefix(trimmed, "- ") && looksLikeGatewayPath(strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))) {
			current.paths = append(current.paths, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
			continue
		}
	}
	flush()
	return routes
}

func stripYAMLComment(line string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:i]
			}
		}
	}
	return line
}

func splitGatewayPathList(raw string) []string {
	raw = cleanYAMLScalar(raw)
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")
	var paths []string
	for _, part := range strings.Split(raw, ",") {
		part = cleanYAMLScalar(part)
		if looksLikeGatewayPath(part) {
			paths = append(paths, part)
		}
	}
	return paths
}

func cleanGatewayPaths(paths []string) []string {
	var out []string
	for _, path := range paths {
		path = cleanYAMLScalar(path)
		if looksLikeGatewayPath(path) {
			out = common.AppendUnique(out, path)
		}
	}
	return out
}

func cleanYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, ",")
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}

func looksLikeGatewayPath(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.Contains(path, " ")
}

func gatewayTargetService(route gatewayRoute) string {
	if strings.HasPrefix(strings.ToLower(route.uri), "lb://") {
		return normalizeTargetService(strings.TrimPrefix(route.uri, "lb://"))
	}
	if route.id != "" {
		return common.SanitizeServiceName(route.id)
	}
	return normalizeTargetService(route.uri)
}

func parseSpringYAMLServiceName(content string) string {
	lines := strings.Split(content, "\n")
	stack := map[int]string{}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, ":") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		key, value, _ := strings.Cut(trimmed, ":")
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		for existingIndent := range stack {
			if existingIndent >= indent {
				delete(stack, existingIndent)
			}
		}
		stack[indent] = key
		path := stackPath(stack)
		if path == "spring.application.name" && value != "" {
			return value
		}
	}
	return ""
}

func stackPath(stack map[int]string) string {
	indents := make([]int, 0, len(stack))
	for indent := range stack {
		indents = append(indents, indent)
	}
	sort.Ints(indents)
	parts := make([]string, 0, len(indents))
	for _, indent := range indents {
		parts = append(parts, stack[indent])
	}
	return strings.Join(parts, ".")
}
