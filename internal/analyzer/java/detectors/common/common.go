package common

import (
	"net/url"
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	JavaClassDeclRE       = regexp.MustCompile(`\b(?:class|interface|record)\s+([A-Za-z_][A-Za-z0-9_]*)`)
	JavaMethodDeclRE      = regexp.MustCompile(`\b(?:public|protected|private)\s+(?:static\s+)?(?:[\w<>\[\],.?]+\s+)+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	JavaStringLiteralRE   = regexp.MustCompile(`"([^"\\]*(?:\\.[^"\\]*)*)"`)
	HighCardinalityAttrRE = regexp.MustCompile(`(?:setAttribute|tag)\s*\(\s*"([^"]+)"`)

	javaAnnotationNameRE = regexp.MustCompile(`^@([A-Za-z_][A-Za-z0-9_.$]*)`)
)

func Operation(serviceName string, kind string, protocol string, method string, route string, handler string, sourcePath string, detector string, confidence string) model.Operation {
	return model.Operation{
		Service:    serviceName,
		Kind:       strings.ToUpper(kind),
		Protocol:   strings.ToLower(protocol),
		Method:     strings.ToUpper(method),
		Route:      route,
		Handler:    handler,
		Source:     sourcePath,
		Origin:     detector,
		Confidence: confidence,
		Detectors:  []string{detector},
	}
}

func InferProtocol(route string) string {
	route = strings.ToLower(route)
	switch {
	case strings.HasPrefix(route, "kafka:"):
		return "kafka"
	case strings.HasPrefix(route, "rabbit:"):
		return "rabbit"
	case strings.HasPrefix(route, "jms:"):
		return "jms"
	case strings.HasPrefix(route, "grpc:"):
		return "grpc"
	default:
		return "http"
	}
}

func StripJavaComments(source string) string {
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

func ParenBalance(value string) int {
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

func SimpleAnnotationName(annotation string) string {
	matches := javaAnnotationNameRE.FindStringSubmatch(strings.TrimSpace(annotation))
	if len(matches) != 2 {
		return ""
	}
	parts := strings.Split(matches[1], ".")
	return parts[len(parts)-1]
}

func AnnotationArgs(annotation string) string {
	open := strings.Index(annotation, "(")
	close := strings.LastIndex(annotation, ")")
	if open < 0 || close <= open {
		return ""
	}
	return annotation[open+1 : close]
}

func ExtractAnnotationStrings(args string) []string {
	if strings.TrimSpace(args) == "" {
		return nil
	}
	values := ExtractAssignedStrings(args, "path", "value", "urlPatterns")
	if len(values) == 0 && !strings.Contains(args, "=") {
		values = ExtractStringLiterals(args)
	}
	return values
}

func ExtractAssignedStrings(args string, names ...string) []string {
	var values []string
	for _, name := range names {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*=`)
		matches := re.FindAllStringIndex(args, -1)
		for _, match := range matches {
			values = append(values, ExtractStringLiterals(ReadAnnotationValue(args[match[1]:]))...)
		}
	}
	return values
}

func ReadAnnotationValue(value string) string {
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

func ExtractStringLiterals(value string) []string {
	matches := JavaStringLiteralRE.FindAllStringSubmatch(value, -1)
	values := make([]string, 0, len(matches))
	for _, match := range matches {
		values = append(values, strings.ReplaceAll(match[1], `\"`, `"`))
	}
	return values
}

func JoinRoutes(base string, path string) string {
	if base == "" {
		return path
	}
	if path == "" {
		return base
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func NormalizeTargetService(raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, `"'`))
	raw = strings.TrimPrefix(raw, "dns:///")
	raw = strings.TrimPrefix(raw, "static://")
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
	return basecommon.SanitizeServiceName(raw)
}

func IgnoreTarget(target string, serviceName string) bool {
	return target == "" || target == serviceName || target == "localhost" || target == "127.0.0.1" || target == "0.0.0.0"
}

func AppendUnique(values []string, extra ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range extra {
		if value != "" && !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}
