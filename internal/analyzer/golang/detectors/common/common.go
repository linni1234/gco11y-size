package common

import (
	"go/ast"
	"go/token"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type FileContext struct {
	ServiceName string
	SourcePath  string
	File        *ast.File
	Source      string
}

var (
	colonParamRE    = regexp.MustCompile(`/:([A-Za-z_][A-Za-z0-9_]*)`)
	starParamRE     = regexp.MustCompile(`/\*([A-Za-z_][A-Za-z0-9_]*)`)
	versionImportRE = regexp.MustCompile(`^v[0-9]+$`)
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

func ForEachCall(file *ast.File, visit func(*ast.CallExpr)) {
	if file == nil {
		return
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok {
			visit(call)
		}
		return true
	})
}

func CallName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		prefix := CallName(value.X)
		if prefix == "" {
			return value.Sel.Name
		}
		return prefix + "." + value.Sel.Name
	case *ast.StarExpr:
		return CallName(value.X)
	case *ast.CallExpr:
		return CallName(value.Fun)
	}
	return ""
}

func SelectorName(call *ast.CallExpr) string {
	if call == nil {
		return ""
	}
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
		return selector.Sel.Name
	}
	if ident, ok := call.Fun.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func ReceiverName(call *ast.CallExpr) string {
	if call == nil {
		return ""
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if ident, ok := selector.X.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

func StringArg(call *ast.CallExpr, index int) (string, bool) {
	if call == nil || index < 0 || len(call.Args) <= index {
		return "", false
	}
	return StringLiteral(call.Args[index])
}

func StringLiteral(expr ast.Expr) (string, bool) {
	switch value := expr.(type) {
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		unquoted, err := strconv.Unquote(value.Value)
		if err != nil {
			return "", false
		}
		return unquoted, true
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", false
		}
		left, leftOK := StringLiteral(value.X)
		right, rightOK := StringLiteral(value.Y)
		if !leftOK || !rightOK {
			return "", false
		}
		return left + right, true
	case *ast.ParenExpr:
		return StringLiteral(value.X)
	}
	return "", false
}

func StringArgs(args []ast.Expr) []string {
	var values []string
	for _, arg := range args {
		if value, ok := StringLiteral(arg); ok {
			values = append(values, value)
		}
	}
	return values
}

func StringValuesInExpr(expr ast.Expr) []string {
	var values []string
	ast.Inspect(expr, func(node ast.Node) bool {
		expr, ok := node.(ast.Expr)
		if !ok {
			return true
		}
		if value, ok := StringLiteral(expr); ok {
			values = append(values, value)
			return false
		}
		return true
	})
	return values
}

func CompositeStringValue(expr ast.Expr, keys ...string) (string, bool) {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return "", false
	}
	for _, element := range lit.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if !ok || !matchesAny(key.Name, keys) {
			continue
		}
		if value, ok := StringLiteral(keyValue.Value); ok {
			return value, true
		}
	}
	return "", false
}

func CollectGroupPrefixes(file *ast.File) map[string]string {
	return CollectGroupPrefixesForRoots(file, nil)
}

func CollectRootVars(file *ast.File, constructors ...string) map[string]bool {
	roots := map[string]bool{}
	ForEachCall(file, func(call *ast.CallExpr) {
		if !matchesAny(CallName(call.Fun), constructors) {
			return
		}
		assignment := enclosingAssign(file, call)
		if assignment == nil || len(assignment.Lhs) == 0 {
			return
		}
		ident, ok := assignment.Lhs[0].(*ast.Ident)
		if ok {
			roots[ident.Name] = true
		}
	})
	return roots
}

func CollectRootVarsFromReceivers(file *ast.File, receivers map[string]bool, constructors ...string) map[string]bool {
	roots := map[string]bool{}
	ForEachCall(file, func(call *ast.CallExpr) {
		if !receivers[ReceiverName(call)] || !matchesAny(SelectorName(call), constructors) {
			return
		}
		assignment := enclosingAssign(file, call)
		if assignment == nil || len(assignment.Lhs) == 0 {
			return
		}
		ident, ok := assignment.Lhs[0].(*ast.Ident)
		if ok {
			roots[ident.Name] = true
		}
	})
	return roots
}

func CollectGroupPrefixesForRoots(file *ast.File, roots map[string]bool) map[string]string {
	groups := map[string]string{}
	ForEachCall(file, func(call *ast.CallExpr) {
		if SelectorName(call) != "Group" {
			return
		}
		path, ok := StringArg(call, 0)
		if !ok {
			return
		}
		assignment := enclosingAssign(file, call)
		if assignment == nil || len(assignment.Lhs) == 0 {
			return
		}
		ident, ok := assignment.Lhs[0].(*ast.Ident)
		if !ok {
			return
		}
		prefix := NormalizeRoute(path)
		receiver := ReceiverName(call)
		if len(roots) > 0 && !roots[receiver] && groups[receiver] == "" {
			return
		}
		if receiver != "" {
			if base := groups[receiver]; base != "" {
				prefix = JoinRoutes(base, prefix)
			}
		}
		groups[ident.Name] = prefix
	})
	return groups
}

func ReceiverAllowed(receiver string, roots map[string]bool, groups map[string]string) bool {
	if receiver == "" {
		return false
	}
	return roots[receiver] || groups[receiver] != ""
}

func RouteForReceiver(groups map[string]string, receiver string, path string) string {
	path = NormalizeRoute(path)
	if receiver == "" {
		return path
	}
	if prefix := groups[receiver]; prefix != "" {
		return JoinRoutes(prefix, path)
	}
	return path
}

func ParseRoutePattern(pattern string) (method string, route string) {
	method = "ANY"
	route = strings.TrimSpace(pattern)
	parts := strings.Fields(route)
	if len(parts) >= 2 && IsHTTPMethod(parts[0]) {
		method = strings.ToUpper(parts[0])
		route = parts[1]
	}
	return method, NormalizeRoute(route)
}

func NormalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	route = colonParamRE.ReplaceAllString(route, `/{$1}`)
	route = starParamRE.ReplaceAllString(route, `/{$1}`)
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

func IsHTTPMethod(name string) bool {
	switch strings.ToUpper(name) {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "TRACE", "CONNECT":
		return true
	default:
		return false
	}
}

func NormalizeTargetService(raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, `"'`))
	raw = strings.TrimPrefix(raw, "dns:///")
	raw = strings.TrimPrefix(raw, "static://")
	if raw == "" || strings.Contains(raw, "${") || strings.Contains(raw, "%s") {
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

func HandlerName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return CallName(value)
	case *ast.FuncLit:
		return "inline handler"
	case *ast.UnaryExpr:
		return HandlerName(value.X)
	case *ast.CompositeLit:
		return CallName(value.Type)
	}
	return ""
}

func ImportPaths(file *ast.File) []string {
	var paths []string
	if file == nil {
		return paths
	}
	for _, spec := range file.Imports {
		if spec.Path == nil {
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}

func ImportReceivers(file *ast.File, suffixes ...string) map[string]bool {
	receivers := map[string]bool{}
	if file == nil {
		return receivers
	}
	for _, spec := range file.Imports {
		if spec.Path == nil {
			continue
		}
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || !matchesImport(path, suffixes) {
			continue
		}
		name := defaultImportName(path)
		if spec.Name != nil && spec.Name.Name != "." && spec.Name.Name != "_" {
			name = spec.Name.Name
		}
		if name != "" {
			receivers[name] = true
		}
	}
	return receivers
}

func HasImport(file *ast.File, suffix string) bool {
	for _, path := range ImportPaths(file) {
		if matchesImport(path, []string{suffix}) {
			return true
		}
	}
	return false
}

func matchesImport(path string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if path == suffix || strings.HasSuffix(path, "/"+suffix) {
			return true
		}
	}
	return false
}

func defaultImportName(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	last := parts[len(parts)-1]
	if versionImportRE.MatchString(last) && len(parts) > 1 {
		return parts[len(parts)-2]
	}
	return last
}

func matchesAny(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func enclosingAssign(file *ast.File, target *ast.CallExpr) *ast.AssignStmt {
	var found *ast.AssignStmt
	ast.Inspect(file, func(node ast.Node) bool {
		if found != nil {
			return false
		}
		assign, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, rhs := range assign.Rhs {
			if rhs == target {
				found = assign
				return false
			}
		}
		return true
	})
	return found
}
