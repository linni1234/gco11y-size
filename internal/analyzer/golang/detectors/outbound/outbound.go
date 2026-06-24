package outbound

import (
	"go/ast"
	"regexp"
	"strings"

	gocommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var httpLiteralRE = regexp.MustCompile(`https?://[A-Za-z0-9_.:${}/-]+`)

func Edges(ctx gocommon.FileContext) []model.Edge {
	seen := map[string]bool{}
	var edges []model.Edge
	add := func(raw string, confidence string) {
		target := gocommon.NormalizeTargetService(raw)
		if gocommon.IgnoreTarget(target, ctx.ServiceName) || seen[target] {
			return
		}
		seen[target] = true
		edges = append(edges, gocommon.Edge(ctx.ServiceName, target, "http", ctx.SourcePath, confidence))
	}

	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		switch gocommon.CallName(call.Fun) {
		case "http.Get", "http.Head", "http.Post", "http.PostForm":
			if raw, ok := gocommon.StringArg(call, 0); ok {
				add(raw, "high")
			}
		case "http.NewRequest", "http.NewRequestWithContext":
			index := 1
			if gocommon.CallName(call.Fun) == "http.NewRequestWithContext" {
				index = 2
			}
			if raw, ok := gocommon.StringArg(call, index); ok {
				add(raw, "high")
			}
		default:
			if hasRestyHint(ctx.File) && isRestyMethod(gocommon.SelectorName(call)) {
				if raw, ok := gocommon.StringArg(call, 0); ok {
					add(raw, "medium")
				}
			}
		}
	})

	if hasHTTPClientHint(ctx.File) {
		ast.Inspect(ctx.File, func(node ast.Node) bool {
			expr, ok := node.(ast.Expr)
			if !ok {
				return true
			}
			raw, ok := gocommon.StringLiteral(expr)
			if ok && httpLiteralRE.MatchString(raw) {
				add(raw, "low")
				return false
			}
			return true
		})
	}
	return edges
}

func hasHTTPClientHint(file *ast.File) bool {
	for _, path := range gocommon.ImportPaths(file) {
		if path == "net/http" || strings.Contains(path, "resty") || strings.Contains(path, "fasthttp") {
			return true
		}
	}
	return false
}

func hasRestyHint(file *ast.File) bool {
	for _, path := range gocommon.ImportPaths(file) {
		if strings.Contains(path, "go-resty/resty") {
			return true
		}
	}
	return false
}

func isRestyMethod(name string) bool {
	switch name {
	case "Get", "Post", "Put", "Delete", "Patch", "Head", "Options":
		return true
	default:
		return false
	}
}
