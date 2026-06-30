package common

import (
	"net/url"
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type FileContext struct {
	ServiceName string
	SourcePath  string
	Source      string
}

type Call struct {
	Name     string
	Receiver string
	Args     string
	Start    int
	End      int
}

type DecoratedBlock struct {
	Name       string
	Kind       string
	Decorators []string
}

var (
	callHeadRE       = regexp.MustCompile(`(?s)(?:\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*)?\b([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	importRE         = regexp.MustCompile(`(?m)^\s*import\s+([A-Za-z0-9_.,\s]+)`)
	fromImportRE     = regexp.MustCompile(`(?m)^\s*from\s+([A-Za-z0-9_.]+)\s+import\s+`)
	httpLiteralRE    = regexp.MustCompile(`https?://[A-Za-z0-9_.:${}/?=&%+\-]+`)
	identifierRE     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	identifierCharRE = regexp.MustCompile(`[A-Za-z0-9_]`)
	flaskParamRE     = regexp.MustCompile(`<(?:(?:string|int|float|path|uuid|any)\s*:\s*)?([A-Za-z_][A-Za-z0-9_]*)>`)
	djangoNamedRE    = regexp.MustCompile(`\(\?P<([A-Za-z_][A-Za-z0-9_]*)>[^)]+\)`)
	djangoTypedRE    = regexp.MustCompile(`<[A-Za-z_][A-Za-z0-9_]*:([A-Za-z_][A-Za-z0-9_]*)>`)
)

func Operation(serviceName string, kind string, protocol string, method string, route string, handler string, sourcePath string, detector string, confidence string) model.Operation {
	if strings.EqualFold(protocol, "http") || strings.EqualFold(protocol, "websocket") {
		route = NormalizeRoute(route)
	}
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

func Edge(serviceName string, target string, protocol string, sourcePath string, confidence string) model.Edge {
	return model.Edge{
		SourceService: serviceName,
		TargetService: target,
		Protocol:      strings.ToLower(protocol),
		Source:        sourcePath,
		Confidence:    confidence,
	}
}

func StripPythonComments(source string) string {
	var out strings.Builder
	inString := byte(0)
	triple := false
	escaped := false
	for i := 0; i < len(source); i++ {
		ch := source[i]
		if inString != 0 {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if triple {
				if ch == inString && i+2 < len(source) && source[i+1] == inString && source[i+2] == inString {
					out.WriteByte(source[i+1])
					out.WriteByte(source[i+2])
					i += 2
					inString = 0
					triple = false
				}
				continue
			}
			if ch == inString {
				inString = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			inString = ch
			triple = i+2 < len(source) && source[i+1] == ch && source[i+2] == ch
			out.WriteByte(ch)
			if triple {
				out.WriteByte(source[i+1])
				out.WriteByte(source[i+2])
				i += 2
			}
			continue
		}
		if ch == '#' {
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

func Calls(source string, names ...string) []Call {
	allowed := map[string]bool{}
	for _, name := range names {
		allowed[name] = true
	}
	var calls []Call
	for _, match := range callHeadRE.FindAllStringSubmatchIndex(source, -1) {
		name := source[match[4]:match[5]]
		if len(allowed) > 0 && !allowed[name] {
			continue
		}
		receiver := ""
		if match[2] >= 0 {
			receiver = source[match[2]:match[3]]
		}
		open := match[1] - 1
		args, end, ok := ReadBalanced(source, open, '(', ')')
		if !ok {
			continue
		}
		calls = append(calls, Call{Name: name, Receiver: receiver, Args: args, Start: match[0], End: end})
	}
	return calls
}

func ReadBalanced(source string, open int, openChar byte, closeChar byte) (string, int, bool) {
	if open < 0 || open >= len(source) || source[open] != openChar {
		return "", open, false
	}
	depth := 0
	inString := byte(0)
	triple := false
	escaped := false
	for i := open; i < len(source); i++ {
		ch := source[i]
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if triple {
				if ch == inString && i+2 < len(source) && source[i+1] == inString && source[i+2] == inString {
					i += 2
					inString = 0
					triple = false
				}
				continue
			}
			if ch == inString {
				inString = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			inString = ch
			triple = i+2 < len(source) && source[i+1] == ch && source[i+2] == ch
			if triple {
				i += 2
			}
			continue
		}
		if ch == openChar {
			depth++
			continue
		}
		if ch == closeChar {
			depth--
			if depth == 0 {
				return source[open+1 : i], i + 1, true
			}
		}
	}
	return "", open, false
}

func ExtractStringLiterals(value string) []string {
	var values []string
	for i := 0; i < len(value); i++ {
		for i < len(value) && isStringPrefix(value[i]) {
			if i+1 < len(value) && (value[i+1] == '\'' || value[i+1] == '"') {
				i++
				break
			}
			i++
		}
		if i >= len(value) || (value[i] != '\'' && value[i] != '"') {
			continue
		}
		quote := value[i]
		triple := i+2 < len(value) && value[i+1] == quote && value[i+2] == quote
		start := i + 1
		if triple {
			start = i + 3
		}
		var out strings.Builder
		escaped := false
		for j := start; j < len(value); j++ {
			ch := value[j]
			if escaped {
				out.WriteByte(ch)
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if triple {
				if ch == quote && j+2 < len(value) && value[j+1] == quote && value[j+2] == quote {
					values = append(values, out.String())
					i = j + 2
					break
				}
			} else if ch == quote {
				values = append(values, out.String())
				i = j
				break
			}
			out.WriteByte(ch)
		}
	}
	return values
}

func FirstString(value string) (string, bool) {
	values := ExtractStringLiterals(value)
	if len(values) == 0 {
		return "", false
	}
	return values[0], true
}

func KeywordString(args string, keyword string) (string, bool) {
	re := regexp.MustCompile(`(?s)\b` + regexp.QuoteMeta(keyword) + `\s*=\s*`)
	match := re.FindStringIndex(args)
	if len(match) != 2 {
		return "", false
	}
	return FirstString(args[match[1]:])
}

func KeywordStrings(args string, keyword string) []string {
	re := regexp.MustCompile(`(?s)\b` + regexp.QuoteMeta(keyword) + `\s*=\s*`)
	match := re.FindStringIndex(args)
	if len(match) != 2 {
		return nil
	}
	return ExtractStringLiterals(args[match[1]:])
}

func FirstIdentifier(args string) (string, bool) {
	beforeComma := args
	if comma := strings.Index(beforeComma, ","); comma >= 0 {
		beforeComma = beforeComma[:comma]
	}
	beforeComma = strings.TrimSpace(beforeComma)
	if identifierRE.MatchString(beforeComma) {
		return beforeComma, true
	}
	return "", false
}

func Imports(source string) []string {
	seen := map[string]bool{}
	var imports []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		value = strings.Trim(value, ",")
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		imports = append(imports, value)
	}
	for _, match := range fromImportRE.FindAllStringSubmatch(source, -1) {
		add(match[1])
	}
	for _, match := range importRE.FindAllStringSubmatch(source, -1) {
		for _, part := range strings.Split(match[1], ",") {
			part = strings.TrimSpace(part)
			if fields := strings.Fields(part); len(fields) > 0 {
				add(fields[0])
			}
		}
	}
	return imports
}

func HasImport(source string, candidates ...string) bool {
	for _, imported := range Imports(source) {
		for _, candidate := range candidates {
			if imported == candidate || strings.HasPrefix(imported, candidate+".") || strings.Contains(imported, candidate) {
				return true
			}
		}
	}
	return false
}

func HasIdentifier(source string, ident string) bool {
	idx := strings.Index(source, ident)
	for idx >= 0 {
		beforeOK := idx == 0 || !identifierCharRE.MatchString(source[idx-1:idx])
		after := idx + len(ident)
		afterOK := after >= len(source) || !identifierCharRE.MatchString(source[after:after+1])
		if beforeOK && afterOK {
			return true
		}
		next := strings.Index(source[idx+len(ident):], ident)
		if next < 0 {
			return false
		}
		idx += len(ident) + next
	}
	return false
}

func DecoratedBlocks(source string) []DecoratedBlock {
	lines := strings.Split(source, "\n")
	var pending []string
	var blocks []DecoratedBlock
	defRE := regexp.MustCompile(`^\s*(?:async\s+)?def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	classRE := regexp.MustCompile(`^\s*class\s+([A-Za-z_][A-Za-z0-9_]*)`)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "@") {
			pending = append(pending, trimmed)
			continue
		}
		if len(pending) > 0 {
			if match := defRE.FindStringSubmatch(line); len(match) == 2 {
				blocks = append(blocks, DecoratedBlock{Name: match[1], Kind: "function", Decorators: append([]string(nil), pending...)})
				pending = nil
				continue
			}
			if match := classRE.FindStringSubmatch(line); len(match) == 2 {
				blocks = append(blocks, DecoratedBlock{Name: match[1], Kind: "class", Decorators: append([]string(nil), pending...)})
				pending = nil
				continue
			}
			if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				pending = nil
			}
		}
	}
	return blocks
}

func NormalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	route = strings.Trim(route, `"'`)
	route = strings.TrimPrefix(route, "^")
	route = strings.TrimSuffix(route, "$")
	route = strings.TrimSuffix(route, "/?")
	route = djangoNamedRE.ReplaceAllString(route, `{$1}`)
	route = djangoTypedRE.ReplaceAllString(route, `{$1}`)
	route = flaskParamRE.ReplaceAllString(route, `{$1}`)
	route = regexp.MustCompile(`\\/?`).ReplaceAllString(route, "/")
	route = strings.ReplaceAll(route, `\/`, "/")
	return basecommon.NormalizeRoute(route)
}

func JoinRoutes(parts ...string) string {
	joined := "/"
	for _, part := range parts {
		part = NormalizeRoute(part)
		if part == "/" {
			continue
		}
		if joined == "/" {
			joined = part
			continue
		}
		joined = basecommon.NormalizeRoute(strings.TrimRight(joined, "/") + "/" + strings.TrimLeft(part, "/"))
	}
	return NormalizeRoute(joined)
}

func IsHTTPMethod(value string) bool {
	switch strings.ToUpper(value) {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "TRACE", "CONNECT":
		return true
	default:
		return false
	}
}

func NormalizeTargetService(raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, `"'`))
	if raw == "" || strings.Contains(raw, "${") || strings.Contains(raw, "%") || strings.Contains(raw, "{") {
		return ""
	}
	raw = strings.TrimPrefix(raw, "dns:///")
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		parsed, err := url.Parse(raw)
		if err == nil && parsed.Hostname() != "" {
			raw = parsed.Hostname()
		}
	}
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

func HTTPURLs(source string) []string {
	return httpLiteralRE.FindAllString(source, -1)
}

func isStringPrefix(ch byte) bool {
	switch ch {
	case 'r', 'R', 'u', 'U', 'b', 'B', 'f', 'F':
		return true
	default:
		return false
	}
}
