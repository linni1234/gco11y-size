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

var (
	callHeadRE         = regexp.MustCompile(`(?s)(?:\b([A-Za-z_$][A-Za-z0-9_$]*)\s*\.\s*)?\b([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	importFromRE       = regexp.MustCompile(`(?m)\bimport\b[^;\n]*?\bfrom\s*['"]([^'"]+)['"]`)
	importSideEffectRE = regexp.MustCompile(`(?m)\bimport\s*['"]([^'"]+)['"]`)
	requireRE          = regexp.MustCompile(`\brequire\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	httpLiteralRE      = regexp.MustCompile(`https?://[A-Za-z0-9_.:${}/?=&%-]+`)
	identifierBoundary = regexp.MustCompile(`[A-Za-z0-9_$]`)
	colonParamRE       = regexp.MustCompile(`/:(\w+)`)
	nextDynamicRE      = regexp.MustCompile(`\[\[?\.{3}([A-Za-z_][A-Za-z0-9_]*)\]?\]|\[([A-Za-z_][A-Za-z0-9_]*)\]`)
)

func Operation(serviceName string, kind string, protocol string, method string, route string, handler string, sourcePath string, detector string, confidence string) model.Operation {
	if strings.EqualFold(protocol, "http") {
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

func StripJSComments(source string) string {
	var out strings.Builder
	inBlock := false
	inString := byte(0)
	inTemplate := false
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
				continue
			}
			if ch == '\n' {
				out.WriteByte('\n')
			}
			continue
		}
		if inTemplate {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '`' {
				inTemplate = false
			}
			continue
		}
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
			if ch == inString {
				inString = 0
			}
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
		if ch == '\'' || ch == '"' {
			inString = ch
			out.WriteByte(ch)
			continue
		}
		if ch == '`' {
			inTemplate = true
			out.WriteByte(ch)
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
		if !allowed[name] {
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
	inTemplate := false
	escaped := false
	for i := open; i < len(source); i++ {
		ch := source[i]
		if inTemplate {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '`' {
				inTemplate = false
			}
			continue
		}
		if inString != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == inString {
				inString = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			inString = ch
			continue
		}
		if ch == '`' {
			inTemplate = true
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
		quote := value[i]
		if quote != '\'' && quote != '"' && quote != '`' {
			continue
		}
		var out strings.Builder
		escaped := false
		for j := i + 1; j < len(value); j++ {
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
			if ch == quote {
				str := out.String()
				if !strings.Contains(str, "${") {
					values = append(values, str)
				}
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

func Imports(source string) []string {
	seen := map[string]bool{}
	var imports []string
	add := func(value string) {
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		imports = append(imports, value)
	}
	for _, match := range importFromRE.FindAllStringSubmatch(source, -1) {
		add(match[1])
	}
	for _, match := range importSideEffectRE.FindAllStringSubmatch(source, -1) {
		add(match[1])
	}
	for _, match := range requireRE.FindAllStringSubmatch(source, -1) {
		add(match[1])
	}
	return imports
}

func HasImport(source string, candidates ...string) bool {
	for _, imported := range Imports(source) {
		for _, candidate := range candidates {
			if imported == candidate || strings.Contains(imported, candidate) {
				return true
			}
		}
	}
	return false
}

func HasIdentifier(source string, ident string) bool {
	idx := strings.Index(source, ident)
	for idx >= 0 {
		beforeOK := idx == 0 || !identifierBoundary.MatchString(source[idx-1:idx])
		after := idx + len(ident)
		afterOK := after >= len(source) || !identifierBoundary.MatchString(source[after:after+1])
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

func ObjectFieldString(object string, field string) (string, bool) {
	re := regexp.MustCompile(`(?s)\b` + regexp.QuoteMeta(field) + `\s*:\s*`)
	match := re.FindStringIndex(object)
	if len(match) != 2 {
		return "", false
	}
	return FirstString(object[match[1]:])
}

func ObjectFieldStrings(object string, field string) []string {
	re := regexp.MustCompile(`(?s)\b` + regexp.QuoteMeta(field) + `\s*:\s*`)
	match := re.FindStringIndex(object)
	if len(match) != 2 {
		return nil
	}
	return ExtractStringLiterals(object[match[1]:])
}

func NormalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	route = strings.Trim(route, `"'`)
	route = colonParamRE.ReplaceAllString(route, `/{$1}`)
	route = nextDynamicRE.ReplaceAllStringFunc(route, func(match string) string {
		match = strings.Trim(match, "[]")
		match = strings.TrimPrefix(match, "...")
		return "{" + match + "}"
	})
	return basecommon.NormalizeRoute(route)
}

func JoinRoutes(base string, path string) string {
	base = NormalizeRoute(base)
	path = NormalizeRoute(path)
	if base == "/" {
		return path
	}
	if path == "/" {
		return base
	}
	return basecommon.NormalizeRoute(strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/"))
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
	if raw == "" || strings.Contains(raw, "${") || strings.Contains(raw, "%") {
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
