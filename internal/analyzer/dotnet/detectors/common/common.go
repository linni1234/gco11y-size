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
	Generic  string
	Args     string
	Start    int
}

type ClassBlock struct {
	Name       string
	Bases      string
	Attributes string
	Body       string
	Start      int
}

var (
	callHeadRE           = regexp.MustCompile(`(?s)(?:\b([A-Za-z_][A-Za-z0-9_]*)\s*\.\s*)?\b([A-Za-z_][A-Za-z0-9_]*)\s*(?:<([^>(){};]+)>)?\s*\(`)
	classHeadRE          = regexp.MustCompile(`(?s)((?:\s*\[[^\]]+\]\s*)*)\s*(?:public|internal|private|protected|sealed|abstract|partial|static|\s)*class\s+([A-Za-z_][A-Za-z0-9_]*)(?:\s*:\s*([^{]+))?[^{]*\{`)
	methodHeadRE         = regexp.MustCompile(`(?s)((?:\s*\[[^\]]+\]\s*)*)\s*(?:public|internal|private|protected|static|virtual|override|async|sealed|partial|new|\s)+(?:[A-Za-z_][A-Za-z0-9_.<>,\[\]?]+\s+)+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
	attributeRE          = regexp.MustCompile(`(?s)\[\s*([A-Za-z_][A-Za-z0-9_.]*)\s*(?:\((.*?)\))?\s*\]`)
	aspnetParamRE        = regexp.MustCompile(`\{(?:\*{1,2})?([A-Za-z_][A-Za-z0-9_]*)(?:[:?=][^}]*)?\}`)
	httpLiteralRE        = regexp.MustCompile(`https?://[A-Za-z0-9_.:${}/?=&%-]+`)
	identifierBoundaryRE = regexp.MustCompile(`[A-Za-z0-9_]`)
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

func Edge(serviceName string, target string, protocol string, sourcePath string, confidence string) model.Edge {
	return model.Edge{
		SourceService: serviceName,
		TargetService: target,
		Protocol:      strings.ToLower(protocol),
		Source:        sourcePath,
		Confidence:    confidence,
	}
}

func StripCSharpComments(source string) string {
	var out strings.Builder
	inBlock := false
	inString := false
	inChar := false
	verbatim := false
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
		if inString {
			out.WriteByte(ch)
			if verbatim {
				if ch == '"' {
					if next == '"' {
						out.WriteByte(next)
						i++
						continue
					}
					inString = false
					verbatim = false
				}
				continue
			}
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
		if inChar {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '\'' {
				inChar = false
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
		if ch == '\'' {
			inChar = true
			out.WriteByte(ch)
			continue
		}
		if ch == '"' || ((ch == '@' || ch == '$') && next == '"') || (ch == '$' && next == '@' && i+2 < len(source) && source[i+2] == '"') || (ch == '@' && next == '$' && i+2 < len(source) && source[i+2] == '"') {
			if ch == '@' || (ch == '$' && next == '@') {
				verbatim = true
			}
			inString = true
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
	matches := callHeadRE.FindAllStringSubmatchIndex(source, -1)
	for _, match := range matches {
		name := source[match[4]:match[5]]
		if !allowed[name] {
			continue
		}
		receiver := ""
		if match[2] >= 0 {
			receiver = source[match[2]:match[3]]
		}
		generic := ""
		if match[6] >= 0 {
			generic = strings.TrimSpace(source[match[6]:match[7]])
		}
		open := match[1] - 1
		args, _, ok := ReadBalanced(source, open, '(', ')')
		if !ok {
			continue
		}
		calls = append(calls, Call{Name: name, Receiver: receiver, Generic: generic, Args: args, Start: match[0]})
	}
	return calls
}

func ReadBalanced(source string, open int, openChar byte, closeChar byte) (string, int, bool) {
	if open < 0 || open >= len(source) || source[open] != openChar {
		return "", open, false
	}
	depth := 0
	inString := false
	inChar := false
	verbatim := false
	escaped := false
	for i := open; i < len(source); i++ {
		ch := source[i]
		next := byte(0)
		if i+1 < len(source) {
			next = source[i+1]
		}
		if inString {
			if verbatim {
				if ch == '"' {
					if next == '"' {
						i++
						continue
					}
					inString = false
					verbatim = false
				}
				continue
			}
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
		if inChar {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '\'' {
				inChar = false
			}
			continue
		}
		if ch == '\'' {
			inChar = true
			continue
		}
		if ch == '"' || ((ch == '@' || ch == '$') && next == '"') || (ch == '$' && next == '@' && i+2 < len(source) && source[i+2] == '"') || (ch == '@' && next == '$' && i+2 < len(source) && source[i+2] == '"') {
			if ch == '@' || (ch == '$' && next == '@') {
				verbatim = true
			}
			inString = true
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
		verbatim := false
		if value[i] == '$' && i+1 < len(value) && value[i+1] == '@' && i+2 < len(value) && value[i+2] == '"' {
			verbatim = true
			i += 2
		} else if value[i] == '@' && i+1 < len(value) && value[i+1] == '$' && i+2 < len(value) && value[i+2] == '"' {
			verbatim = true
			i += 2
		} else if value[i] == '@' && i+1 < len(value) && value[i+1] == '"' {
			verbatim = true
			i++
		} else if value[i] == '$' && i+1 < len(value) && value[i+1] == '"' {
			i++
		} else if value[i] != '"' {
			continue
		}

		var out strings.Builder
		escaped := false
		for j := i + 1; j < len(value); j++ {
			ch := value[j]
			if verbatim {
				if ch == '"' {
					if j+1 < len(value) && value[j+1] == '"' {
						out.WriteByte('"')
						j++
						continue
					}
					values = append(values, out.String())
					i = j
					break
				}
				out.WriteByte(ch)
				continue
			}
			if escaped {
				out.WriteByte(ch)
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
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

func NamedString(value string, name string) (string, bool) {
	re := regexp.MustCompile(`(?s)\b` + regexp.QuoteMeta(name) + `\s*:\s*`)
	match := re.FindStringIndex(value)
	if len(match) != 2 {
		re = regexp.MustCompile(`(?s)\b` + regexp.QuoteMeta(name) + `\s*=`)
		match = re.FindStringIndex(value)
	}
	if len(match) != 2 {
		return "", false
	}
	return FirstString(value[match[1]:])
}

func FindClasses(source string) []ClassBlock {
	var classes []ClassBlock
	for _, match := range classHeadRE.FindAllStringSubmatchIndex(source, -1) {
		open := match[1] - 1
		body, _, ok := ReadBalanced(source, open, '{', '}')
		if !ok {
			continue
		}
		bases := ""
		if match[6] >= 0 {
			bases = strings.TrimSpace(source[match[6]:match[7]])
		}
		attributes := source[match[2]:match[3]]
		if strings.TrimSpace(attributes) == "" {
			attributes = leadingAttributes(source, match[0])
		}
		classes = append(classes, ClassBlock{
			Name:       source[match[4]:match[5]],
			Bases:      bases,
			Attributes: attributes,
			Body:       body,
			Start:      match[0],
		})
	}
	return classes
}

func Methods(source string) []struct {
	Name       string
	Attributes string
} {
	var methods []struct {
		Name       string
		Attributes string
	}
	for _, match := range methodHeadRE.FindAllStringSubmatchIndex(source, -1) {
		attributes := source[match[2]:match[3]]
		if strings.TrimSpace(attributes) == "" {
			attributes = leadingAttributes(source, match[0])
		}
		methods = append(methods, struct {
			Name       string
			Attributes string
		}{
			Name:       source[match[4]:match[5]],
			Attributes: attributes,
		})
	}
	return methods
}

func leadingAttributes(source string, start int) string {
	i := start
	for i > 0 {
		j := i - 1
		for j >= 0 && (source[j] == ' ' || source[j] == '\t' || source[j] == '\r' || source[j] == '\n') {
			j--
		}
		if j < 0 || source[j] != ']' {
			break
		}
		depth := 0
		for j >= 0 {
			if source[j] == ']' {
				depth++
			}
			if source[j] == '[' {
				depth--
				if depth == 0 {
					i = j
					break
				}
			}
			j--
		}
		if j < 0 {
			break
		}
	}
	if i == start {
		return ""
	}
	return source[i:start]
}

func Attributes(value string) []struct {
	Name string
	Args string
} {
	var attrs []struct {
		Name string
		Args string
	}
	for _, match := range attributeRE.FindAllStringSubmatch(value, -1) {
		name := match[1]
		if dot := strings.LastIndex(name, "."); dot >= 0 {
			name = name[dot+1:]
		}
		args := ""
		if len(match) > 2 {
			args = match[2]
		}
		attrs = append(attrs, struct {
			Name string
			Args string
		}{Name: name, Args: args})
	}
	return attrs
}

func NormalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	route = strings.Trim(route, `"'`)
	route = strings.ReplaceAll(route, "[controller]", "{controller}")
	route = strings.ReplaceAll(route, "[Controller]", "{controller}")
	route = strings.ReplaceAll(route, "[action]", "{action}")
	route = strings.ReplaceAll(route, "[Action]", "{action}")
	route = aspnetParamRE.ReplaceAllString(route, `{$1}`)
	return basecommon.NormalizeRoute(route)
}

func ReplaceRouteTokens(route string, controller string, action string) string {
	controller = strings.TrimSuffix(controller, "Controller")
	controller = basecommon.SanitizeServiceName(controller)
	route = strings.ReplaceAll(route, "{controller}", controller)
	route = strings.ReplaceAll(route, "{action}", basecommon.SanitizeServiceName(action))
	return route
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
	if raw == "" || strings.Contains(raw, "{") || strings.Contains(raw, "%") {
		return ""
	}
	raw = strings.TrimPrefix(raw, "dns:///")
	raw = strings.TrimPrefix(raw, "static://")
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

func HTTPURLs(source string) []string {
	var urls []string
	for _, match := range httpLiteralRE.FindAllString(source, -1) {
		urls = append(urls, match)
	}
	return urls
}

func HasIdentifier(source string, ident string) bool {
	idx := strings.Index(source, ident)
	for idx >= 0 {
		beforeOK := idx == 0 || !identifierBoundaryRE.MatchString(source[idx-1:idx])
		after := idx + len(ident)
		afterOK := after >= len(source) || !identifierBoundaryRE.MatchString(source[after:after+1])
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
