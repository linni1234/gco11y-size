package spring

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type javaParseResult struct {
	operations []model.Operation
	edges      []model.Edge
	risks      []model.Risk
}

type mapping struct {
	methods []string
	paths   []string
	risks   []string
}

var (
	classDeclRE      = regexp.MustCompile(`\b(?:class|interface|record)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	methodDeclRE     = regexp.MustCompile(`\b(?:public|protected|private)\s+(?:static\s+)?(?:[\w<>\[\],.?]+\s+)+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	annotationNameRE = regexp.MustCompile(`^@([A-Za-z_][A-Za-z0-9_.$]*)`)
	requestMethodRE  = regexp.MustCompile(`RequestMethod\.([A-Z]+)`)
	stringLiteralRE  = regexp.MustCompile(`"([^"\\]*(?:\\.[^"\\]*)*)"`)
	feignClientRE    = regexp.MustCompile(`@FeignClient\s*\(([^)]*)\)`)
	httpURLRE        = regexp.MustCompile(`https?://[A-Za-z0-9_.:${}/-]+`)
	attributeRE      = regexp.MustCompile(`(?:setAttribute|tag)\s*\(\s*"([^"]+)"`)
)

func parseJavaSource(serviceName string, sourcePath string, source string) javaParseResult {
	clean := stripJavaComments(source)
	lines := strings.Split(clean, "\n")
	var pending []string
	controllerActive := false
	classBases := []string{""}
	className := ""
	result := javaParseResult{}

	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "@") {
			block := trimmed
			balance := parenBalance(block)
			for balance > 0 && i+1 < len(lines) {
				i++
				next := strings.TrimSpace(lines[i])
				block += " " + next
				balance += parenBalance(next)
			}
			pending = append(pending, block)
			continue
		}

		if matches := classDeclRE.FindStringSubmatch(trimmed); len(matches) == 2 {
			className = matches[1]
			if hasControllerAnnotation(pending) {
				controllerActive = true
				classBases = classLevelPaths(pending)
			}
			pending = nil
			continue
		}

		if matches := methodDeclRE.FindStringSubmatch(trimmed); len(matches) == 2 {
			methodName := matches[1]
			if controllerActive {
				for _, m := range methodLevelMappings(pending) {
					for _, base := range classBases {
						for _, path := range m.paths {
							for _, method := range m.methods {
								route := common.NormalizeRoute(joinRoutes(base, path))
								result.operations = append(result.operations, model.Operation{
									Service: serviceName,
									Kind:    "SERVER",
									Method:  method,
									Route:   route,
									Handler: strings.Trim(className+"."+methodName, "."),
									Source:  sourcePath,
									Risks:   m.risks,
								})
							}
						}
					}
				}
			}
			for _, op := range listenerOperations(serviceName, sourcePath, className, methodName, pending) {
				result.operations = append(result.operations, op)
			}
			pending = nil
			continue
		}

		if strings.HasPrefix(trimmed, "package ") || strings.HasPrefix(trimmed, "import ") || strings.HasSuffix(trimmed, ";") {
			pending = nil
		}
	}

	result.edges = append(result.edges, parseOutboundEdges(serviceName, sourcePath, clean)...)
	result.risks = append(result.risks, parseJavaRisks(sourcePath, clean)...)
	return result
}

func stripJavaComments(source string) string {
	var out strings.Builder
	inBlock := false
	inString := false
	escaped := false
	for i := 0; i < len(source); i++ {
		ch := source[i]
		next := byte(0)
		if i+1 < len(source) {
			next = source[i+1]
		}
		if inBlock {
			if ch == '*' && next == '/' {
				inBlock = false
				i++
			}
			if ch == '\n' {
				out.WriteByte('\n')
			}
			continue
		}
		if inString {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}
		if ch == '/' && next == '*' {
			inBlock = true
			i++
			continue
		}
		if ch == '/' && next == '/' {
			for i < len(source) && source[i] != '\n' {
				i++
			}
			if i < len(source) {
				out.WriteByte('\n')
			}
			continue
		}
		out.WriteByte(ch)
	}
	return out.String()
}

func parenBalance(value string) int {
	balance := 0
	inString := false
	escaped := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '(' {
			balance++
		}
		if ch == ')' {
			balance--
		}
	}
	return balance
}

func hasControllerAnnotation(annotations []string) bool {
	for _, annotation := range annotations {
		name := simpleAnnotationName(annotation)
		if name == "RestController" || name == "Controller" {
			return true
		}
	}
	return false
}

func classLevelPaths(annotations []string) []string {
	for _, annotation := range annotations {
		if simpleAnnotationName(annotation) == "RequestMapping" {
			paths := extractPathStrings(annotationArgs(annotation))
			if len(paths) > 0 {
				return paths
			}
		}
	}
	return []string{""}
}

func methodLevelMappings(annotations []string) []mapping {
	var out []mapping
	for _, annotation := range annotations {
		name := simpleAnnotationName(annotation)
		methods, ok := methodsForAnnotation(name, annotation)
		if !ok {
			continue
		}
		args := annotationArgs(annotation)
		paths := extractPathStrings(args)
		if len(paths) == 0 {
			paths = []string{""}
		}
		var risks []string
		if hasDynamicAnnotationExpression(args) {
			risks = append(risks, "mapping path contains dynamic expression or placeholder")
		}
		for _, path := range paths {
			if strings.Contains(path, "**") || strings.Contains(path, "*") {
				risks = append(risks, "wildcard route may hide multiple operations behind one source mapping")
			}
		}
		out = append(out, mapping{methods: methods, paths: paths, risks: risks})
	}
	return out
}

func simpleAnnotationName(annotation string) string {
	matches := annotationNameRE.FindStringSubmatch(strings.TrimSpace(annotation))
	if len(matches) != 2 {
		return ""
	}
	parts := strings.Split(matches[1], ".")
	return parts[len(parts)-1]
}

func annotationArgs(annotation string) string {
	open := strings.Index(annotation, "(")
	close := strings.LastIndex(annotation, ")")
	if open < 0 || close <= open {
		return ""
	}
	return annotation[open+1 : close]
}

func methodsForAnnotation(name string, annotation string) ([]string, bool) {
	switch name {
	case "GetMapping":
		return []string{"GET"}, true
	case "PostMapping":
		return []string{"POST"}, true
	case "PutMapping":
		return []string{"PUT"}, true
	case "DeleteMapping":
		return []string{"DELETE"}, true
	case "PatchMapping":
		return []string{"PATCH"}, true
	case "RequestMapping":
		matches := requestMethodRE.FindAllStringSubmatch(annotation, -1)
		if len(matches) == 0 {
			return []string{"ANY"}, true
		}
		methods := make([]string, 0, len(matches))
		for _, match := range matches {
			methods = common.AppendUnique(methods, match[1])
		}
		return methods, true
	default:
		return nil, false
	}
}

func extractPathStrings(args string) []string {
	if strings.TrimSpace(args) == "" {
		return nil
	}
	var values []string
	for _, assignment := range []string{"path", "value"} {
		values = append(values, extractAssignedStrings(args, assignment)...)
	}
	if len(values) == 0 && !strings.Contains(args, "=") {
		values = append(values, extractStringLiterals(args)...)
	}

	var paths []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "{") {
			paths = common.AppendUnique(paths, value)
		}
	}
	return paths
}

func extractAssignedStrings(args string, names ...string) []string {
	var values []string
	for _, name := range names {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*=`)
		matches := re.FindAllStringIndex(args, -1)
		for _, match := range matches {
			expression := readAnnotationValue(args[match[1]:])
			values = append(values, extractStringLiterals(expression)...)
		}
	}
	return values
}

func readAnnotationValue(value string) string {
	var out strings.Builder
	braceDepth := 0
	parenDepth := 0
	inString := false
	escaped := false
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if inString {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '(':
			parenDepth++
		case ')':
			if parenDepth == 0 && braceDepth == 0 {
				return out.String()
			}
			parenDepth--
		case ',':
			if braceDepth == 0 && parenDepth == 0 {
				return out.String()
			}
		}
		out.WriteByte(ch)
	}
	return out.String()
}

func hasDynamicAnnotationExpression(args string) bool {
	inString := false
	escaped := false
	for i := 0; i < len(args); i++ {
		ch := args[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			if ch == '$' && i+1 < len(args) && args[i+1] == '{' {
				return true
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == '+' {
			return true
		}
	}
	return false
}

func extractStringLiterals(value string) []string {
	matches := stringLiteralRE.FindAllStringSubmatch(value, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, strings.ReplaceAll(match[1], `\"`, `"`))
	}
	return values
}

func joinRoutes(base string, path string) string {
	if base == "" {
		return path
	}
	if path == "" {
		return base
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func listenerOperations(serviceName string, sourcePath string, className string, methodName string, annotations []string) []model.Operation {
	var operations []model.Operation
	for _, annotation := range annotations {
		name := simpleAnnotationName(annotation)
		args := annotationArgs(annotation)
		switch name {
		case "KafkaListener":
			topics := extractAssignedStrings(args, "topics", "topicPattern")
			if len(topics) == 0 {
				topics = []string{"unknown-topic"}
			}
			for _, topic := range topics {
				operations = append(operations, model.Operation{
					Service: serviceName,
					Kind:    "CONSUMER",
					Method:  "MESSAGE",
					Route:   "kafka:" + topic,
					Handler: strings.Trim(className+"."+methodName, "."),
					Source:  sourcePath,
				})
			}
		case "RabbitListener":
			queues := extractAssignedStrings(args, "queues")
			if len(queues) == 0 {
				queues = []string{"unknown-queue"}
			}
			for _, queue := range queues {
				operations = append(operations, model.Operation{
					Service: serviceName,
					Kind:    "CONSUMER",
					Method:  "MESSAGE",
					Route:   "rabbit:" + queue,
					Handler: strings.Trim(className+"."+methodName, "."),
					Source:  sourcePath,
				})
			}
		}
	}
	return operations
}

func parseOutboundEdges(serviceName string, sourcePath string, source string) []model.Edge {
	var edges []model.Edge
	for _, match := range feignClientRE.FindAllStringSubmatch(source, -1) {
		args := match[1]
		targets := append(extractAssignedStrings(args, "name", "value", "contextId"), extractAssignedStrings(args, "url")...)
		for _, target := range targets {
			target = normalizeTargetService(target)
			if target == "" || target == serviceName {
				continue
			}
			edges = append(edges, model.Edge{
				SourceService: serviceName,
				TargetService: target,
				Protocol:      "http",
				Source:        sourcePath,
				Confidence:    "high",
			})
		}
	}

	if hasOutboundHTTPClient(source) {
		for _, rawURL := range httpURLRE.FindAllString(source, -1) {
			target := normalizeTargetService(rawURL)
			if target == "" || target == serviceName || target == "localhost" || target == "127.0.0.1" {
				continue
			}
			edges = append(edges, model.Edge{
				SourceService: serviceName,
				TargetService: target,
				Protocol:      "http",
				Source:        sourcePath,
				Confidence:    "medium",
			})
		}
	}

	sendTopicRE := regexp.MustCompile(`KafkaTemplate(?:<[^>]+>)?[^;]*\.send\s*\(\s*"([^"]+)"`)
	for _, match := range sendTopicRE.FindAllStringSubmatch(source, -1) {
		edges = append(edges, model.Edge{
			SourceService: serviceName,
			TargetService: "kafka:" + match[1],
			Protocol:      "kafka",
			Source:        sourcePath,
			Confidence:    "medium",
		})
	}
	return edges
}

func hasOutboundHTTPClient(source string) bool {
	clientHints := []string{
		"RestTemplate",
		"WebClient",
		"RestClient",
		"HttpClient",
		"HttpRequest",
		"OkHttpClient",
		"WebTarget",
	}
	for _, hint := range clientHints {
		if strings.Contains(source, hint) {
			return true
		}
	}
	return false
}

func normalizeTargetService(raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, `"'`))
	if raw == "" || strings.Contains(raw, "${") {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		parsed, err := url.Parse(raw)
		if err == nil && parsed.Hostname() != "" {
			raw = parsed.Hostname()
		}
	}
	raw = strings.TrimPrefix(raw, "lb://")
	raw = strings.Trim(raw, "/")
	if slash := strings.Index(raw, "/"); slash >= 0 {
		raw = raw[:slash]
	}
	if colon := strings.Index(raw, ":"); colon >= 0 {
		raw = raw[:colon]
	}
	if strings.Contains(raw, ".svc") || strings.Contains(raw, ".cluster.local") {
		raw = strings.Split(raw, ".")[0]
	}
	return common.SanitizeServiceName(raw)
}

func parseJavaRisks(sourcePath string, source string) []model.Risk {
	var risks []model.Risk
	for _, match := range attributeRE.FindAllStringSubmatch(source, -1) {
		attr := strings.ToLower(match[1])
		if common.LooksHighCardinalityAttribute(attr) {
			risks = append(risks, model.Risk{
				Severity: "medium",
				Area:     "span attributes",
				Message:  fmt.Sprintf("span attribute %q looks high-cardinality if enabled as an App O11y dimension", match[1]),
				Source:   sourcePath,
			})
		}
	}
	if strings.Contains(source, "RestTemplate") || strings.Contains(source, "WebClient") || strings.Contains(source, "RestClient") {
		if !httpURLRE.MatchString(source) && !strings.Contains(source, "@FeignClient") {
			risks = append(risks, model.Risk{
				Severity: "low",
				Area:     "service graph",
				Message:  "outbound HTTP client detected, but target service could not be inferred from static source",
				Source:   sourcePath,
			})
		}
	}
	if strings.Contains(source, "${") && (strings.Contains(source, "Mapping(") || strings.Contains(source, "baseUrl(") || strings.Contains(source, ".uri(")) {
		risks = append(risks, model.Risk{
			Severity: "low",
			Area:     "static analysis",
			Message:  "configuration placeholders may hide routes or outbound service targets",
			Source:   sourcePath,
		})
	}
	return risks
}
