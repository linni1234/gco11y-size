package quarkus

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	javacommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type Context struct {
	IsQuarkus       bool
	ApplicationName string
	HTTPRootPath    string
	RESTPath        string
	ApplicationPath string
}

type RouteResult struct {
	Operations []model.Operation
	Warnings   []string
}

type routeMapping struct {
	methods []string
	paths   []string
}

var (
	methodDeclRE           = regexp.MustCompile(`\b(?:public|protected|private)?\s*(?:static\s+)?(?:[\w<>\[\],.?]+\s+)+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	routeParamRE           = regexp.MustCompile(`(^|/):([A-Za-z_][A-Za-z0-9_]*)`)
	routeHTTPMethodRE      = regexp.MustCompile(`(?i)(?:Route\.)?HttpMethod\.([A-Z]+)|\b(GET|POST|PUT|DELETE|PATCH|OPTIONS|HEAD)\b`)
	configKeyLineRE        = regexp.MustCompile(`(?im)^\s*quarkus\.`)
	quarkusYAMLRootRE      = regexp.MustCompile(`(?m)^\s*quarkus\s*:`)
	quarkusImportRE        = regexp.MustCompile(`(?m)^\s*import\s+io\.quarkus\.`)
	applicationPathRE      = regexp.MustCompile(`@(?:[A-Za-z0-9_$.]+\.)?ApplicationPath\s*\(([^)]*)\)`)
	assignedValuePrefixTpl = `\b%s\s*=`
)

func Discover(repo string) (map[string]Context, []model.ConfigFinding, []string) {
	contexts := map[string]Context{}
	var findings []model.ConfigFinding
	var warnings []string

	err := filepath.WalkDir(repo, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			warnings = append(warnings, fmt.Sprintf("could not read %s: %v", path, walkErr))
			return nil
		}
		if entry.IsDir() {
			if basecommon.ShouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if basecommon.IsTestPath(repo, path) {
			return nil
		}

		name := strings.ToLower(filepath.Base(path))
		ext := strings.ToLower(filepath.Ext(path))
		if !isQuarkusDiscoveryFile(path, name, ext) {
			return nil
		}

		contentBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			warnings = append(warnings, fmt.Sprintf("could not read %s: %v", basecommon.RelPath(repo, path), readErr))
			return nil
		}
		content := string(contentBytes)
		root := basecommon.InferServiceRoot(repo, path)
		source := basecommon.RelPath(repo, path)
		ctx := contexts[root]

		if isQuarkusBuildFile(name, ext) && strings.Contains(content, "io.quarkus") {
			ctx.IsQuarkus = true
		}
		if ext == ".java" {
			if quarkusImportRE.MatchString(content) || strings.Contains(content, "io.quarkus.") {
				ctx.IsQuarkus = true
			}
			if appPath := applicationPath(content); appPath != "" {
				ctx.ApplicationPath = appPath
			}
		}
		if basecommon.IsConfigFile(path) {
			if configKeyLineRE.MatchString(content) || quarkusYAMLRootRE.MatchString(content) {
				ctx.IsQuarkus = true
			}
			ctx, findings = applyConfig(ctx, content, source, findings)
		}

		contexts[root] = ctx
		return nil
	})
	if err != nil {
		warnings = append(warnings, err.Error())
	}
	return contexts, findings, warnings
}

func (ctx Context) RESTBasePath() string {
	restPath := ctx.RESTPath
	if ctx.ApplicationPath != "" {
		restPath = ctx.ApplicationPath
	}
	return joinBase(ctx.HTTPRootPath, restPath)
}

func (ctx Context) HTTPBasePath() string {
	return normalizeBasePath(ctx.HTTPRootPath)
}

func ReactiveRouteOperations(ctx Context, serviceName string, sourcePath string, source string) RouteResult {
	clean := javacommon.StripJavaComments(source)
	lines := strings.Split(clean, "\n")
	var pending []string
	classPaths := []string{""}
	className := ""
	result := RouteResult{}

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "@") {
			block := trimmed
			balance := javacommon.ParenBalance(block)
			for balance > 0 && i+1 < len(lines) {
				i++
				next := strings.TrimSpace(lines[i])
				block += " " + next
				balance += javacommon.ParenBalance(next)
			}
			pending = append(pending, block)
			continue
		}
		if matches := javacommon.JavaClassDeclRE.FindStringSubmatch(trimmed); len(matches) == 2 {
			className = matches[1]
			if paths := routeBasePaths(pending); len(paths) > 0 {
				classPaths = paths
			} else {
				classPaths = []string{""}
			}
			pending = nil
			continue
		}
		if matches := methodDeclRE.FindStringSubmatch(trimmed); len(matches) == 2 {
			methodName := matches[1]
			if !isPrivateOrStaticMethod(trimmed) {
				mappings, warnings := routeMappings(pending, methodName, sourcePath)
				result.Warnings = append(result.Warnings, warnings...)
				for _, mapping := range mappings {
					for _, classPath := range classPaths {
						for _, path := range mapping.paths {
							for _, method := range mapping.methods {
								route := joinBase(ctx.HTTPBasePath(), javacommon.JoinRoutes(classPath, normalizeVertxPath(path)))
								result.Operations = append(result.Operations, javacommon.Operation(
									serviceName,
									"SERVER",
									"http",
									method,
									route,
									strings.Trim(className+"."+methodName, "."),
									sourcePath,
									"quarkus-reactive-routes",
									"high",
								))
							}
						}
					}
				}
			}
			pending = nil
			continue
		}
		if strings.HasPrefix(trimmed, "package ") || strings.HasPrefix(trimmed, "import ") || strings.HasSuffix(trimmed, ";") {
			pending = nil
		}
	}
	return result
}

func isQuarkusDiscoveryFile(path string, name string, ext string) bool {
	if ext == ".java" || basecommon.IsConfigFile(path) {
		return true
	}
	return isQuarkusBuildFile(name, ext)
}

func isQuarkusBuildFile(name string, ext string) bool {
	return name == "pom.xml" || name == "build.gradle" || name == "build.gradle.kts" || ext == ".gradle"
}

func applyConfig(ctx Context, content string, source string, findings []model.ConfigFinding) (Context, []model.ConfigFinding) {
	if value := configValue(content, "quarkus.application.name"); value != "" {
		ctx.ApplicationName = basecommon.SanitizeServiceName(value)
	}
	if value := configValue(content, "quarkus.http.root-path"); value != "" {
		ctx.HTTPRootPath = normalizeBasePath(value)
		findings = append(findings, model.ConfigFinding{
			Kind:   "quarkus-http-root-path",
			Name:   "quarkus.http.root-path",
			Value:  ctx.HTTPRootPath,
			Source: source,
		})
	}
	if value := configValue(content, "quarkus.rest.path"); value != "" {
		ctx.RESTPath = normalizeBasePath(value)
		findings = append(findings, model.ConfigFinding{
			Kind:   "quarkus-rest-path",
			Name:   "quarkus.rest.path",
			Value:  ctx.RESTPath,
			Source: source,
		})
	}
	return ctx, findings
}

func configValue(content string, key string) string {
	re := regexp.MustCompile(`(?im)^\s*` + regexp.QuoteMeta(key) + `\s*[:=]\s*["']?([^"'\s#]+)["']?`)
	if matches := re.FindStringSubmatch(content); len(matches) == 2 {
		return strings.Trim(matches[1], `"'`)
	}
	return simpleYAMLValue(content, key)
}

func simpleYAMLValue(content string, wanted string) string {
	lines := strings.Split(content, "\n")
	stack := map[int]string{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		trimmed := strings.TrimSpace(line)
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(strings.SplitN(value, "#", 2)[0])
		stack[indent] = key
		for existingIndent := range stack {
			if existingIndent > indent {
				delete(stack, existingIndent)
			}
		}
		if stackPath(stack) == wanted && value != "" {
			return strings.Trim(value, `"'`)
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

func applicationPath(source string) string {
	clean := javacommon.StripJavaComments(source)
	for _, match := range applicationPathRE.FindAllStringSubmatch(clean, -1) {
		if values := javacommon.ExtractAnnotationStrings(match[1]); len(values) > 0 {
			return normalizeBasePath(values[0])
		}
	}
	return ""
}

func routeBasePaths(annotations []string) []string {
	for _, annotation := range annotations {
		if javacommon.SimpleAnnotationName(annotation) != "RouteBase" {
			continue
		}
		args := javacommon.AnnotationArgs(annotation)
		paths := javacommon.ExtractAssignedStrings(args, "path", "value")
		if len(paths) == 0 && !strings.Contains(args, "=") {
			paths = javacommon.ExtractStringLiterals(args)
		}
		return normalizePaths(paths)
	}
	return nil
}

func routeMappings(annotations []string, methodName string, sourcePath string) ([]routeMapping, []string) {
	var mappings []routeMapping
	var warnings []string
	for _, annotation := range annotations {
		if javacommon.SimpleAnnotationName(annotation) != "Route" {
			continue
		}
		args := javacommon.AnnotationArgs(annotation)
		if strings.Contains(args, "HandlerType.FAILURE") || strings.Contains(args, "type = FAILURE") {
			continue
		}
		paths := javacommon.ExtractAssignedStrings(args, "path", "value")
		if len(paths) == 0 && !strings.Contains(args, "=") {
			paths = javacommon.ExtractStringLiterals(args)
		}
		if len(paths) == 0 {
			if assignedValue(args, "regex") != "" {
				warnings = append(warnings, fmt.Sprintf("%s: Quarkus regex-only reactive route on %s was skipped", sourcePath, methodName))
				continue
			}
			paths = []string{methodName}
		}
		mappings = append(mappings, routeMapping{
			methods: routeMethods(args),
			paths:   normalizePaths(paths),
		})
	}
	return mappings, warnings
}

func routeMethods(args string) []string {
	raw := assignedValue(args, "methods")
	if raw == "" {
		return []string{"ANY"}
	}
	seen := map[string]bool{}
	var methods []string
	for _, match := range routeHTTPMethodRE.FindAllStringSubmatch(raw, -1) {
		method := strings.ToUpper(firstNonEmpty(match[1], match[2]))
		if method != "" && !seen[method] {
			methods = append(methods, method)
			seen[method] = true
		}
	}
	if len(methods) == 0 {
		return []string{"ANY"}
	}
	return methods
}

func assignedValue(args string, name string) string {
	re := regexp.MustCompile(fmt.Sprintf(assignedValuePrefixTpl, regexp.QuoteMeta(name)))
	match := re.FindStringIndex(args)
	if match == nil {
		return ""
	}
	return strings.TrimSpace(javacommon.ReadAnnotationValue(args[match[1]:]))
}

func normalizePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, normalizeBasePath(path))
	}
	return out
}

func normalizeBasePath(path string) string {
	path = strings.TrimSpace(strings.Trim(path, `"'`))
	if path == "" || path == "/" {
		return ""
	}
	return basecommon.NormalizeRoute(path)
}

func normalizeVertxPath(path string) string {
	return routeParamRE.ReplaceAllString(path, `${1}{$2}`)
}

func joinBase(base string, path string) string {
	return basecommon.NormalizeRoute(javacommon.JoinRoutes(normalizeBasePath(base), normalizeBasePath(path)))
}

func isPrivateOrStaticMethod(line string) bool {
	fields := strings.Fields(line)
	for _, field := range fields {
		if field == "private" || field == "static" {
			return true
		}
	}
	return false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
