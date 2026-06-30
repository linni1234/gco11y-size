package serverless

import (
	"regexp"
	"strings"

	pycommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	serverlessHTTPBlockRE = regexp.MustCompile(`(?ms)-\s+http(?:Api)?\s*:\s*(?:\n\s+)?(?:path\s*:\s*([^\n]+).*?method\s*:\s*([A-Za-z]+)|method\s*:\s*([A-Za-z]+).*?path\s*:\s*([^\n]+))`)
	samEventRE            = regexp.MustCompile(`(?ms)Type\s*:\s*Api\s*.*?Path\s*:\s*([^\n]+).*?Method\s*:\s*([A-Za-z]+)`)
	functionJSONRouteRE   = regexp.MustCompile(`(?s)"route"\s*:\s*"([^"]+)".*?"methods"\s*:\s*\[(.*?)\]`)
	cloudFunctionRE       = regexp.MustCompile(`(?m)@functions_framework\.http|@https_fn\.on_request|def\s+(?:handler|lambda_handler|main)\s*\(`)
)

func Operations(ctx pycommon.FileContext) []model.Operation {
	path := strings.ToLower(ctx.SourcePath)
	switch {
	case strings.HasSuffix(path, "serverless.yml") || strings.HasSuffix(path, "serverless.yaml") || strings.HasSuffix(path, "template.yml") || strings.HasSuffix(path, "template.yaml"):
		return yamlOperations(ctx)
	case strings.HasSuffix(path, "function.json"):
		return functionJSONOperations(ctx)
	case strings.Contains(ctx.Source, "Chalice("):
		return chaliceOperations(ctx)
	case cloudFunctionRE.MatchString(ctx.Source) && hasServerlessHint(ctx.Source):
		return []model.Operation{pycommon.Operation(ctx.ServiceName, "SERVER", "http", "ANY", "/{function}", "serverless handler", ctx.SourcePath, "python-serverless", "low")}
	default:
		return nil
	}
}

func ConfigFindings(ctx pycommon.FileContext) []model.ConfigFinding {
	path := strings.ToLower(ctx.SourcePath)
	if strings.Contains(path, "serverless") || strings.HasSuffix(path, "function.json") || hasServerlessHint(ctx.Source) {
		return []model.ConfigFinding{{Kind: "python-serverless", Name: "serverless", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName}}
	}
	return nil
}

func yamlOperations(ctx pycommon.FileContext) []model.Operation {
	var operations []model.Operation
	for _, match := range serverlessHTTPBlockRE.FindAllStringSubmatch(ctx.Source, -1) {
		path := firstNonEmpty(match[1], match[4])
		method := strings.ToUpper(firstNonEmpty(match[2], match[3]))
		if method == "" {
			method = "ANY"
		}
		if method == "ANY" || pycommon.IsHTTPMethod(method) {
			operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", method, strings.Trim(path, ` "'`), "serverless HTTP event", ctx.SourcePath, "serverless", "medium"))
		}
	}
	for _, match := range samEventRE.FindAllStringSubmatch(ctx.Source, -1) {
		method := strings.ToUpper(strings.TrimSpace(match[2]))
		if method == "" {
			method = "ANY"
		}
		if method == "ANY" || pycommon.IsHTTPMethod(method) {
			operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", method, strings.Trim(match[1], ` "'`), "SAM API event", ctx.SourcePath, "aws-sam", "medium"))
		}
	}
	return operations
}

func functionJSONOperations(ctx pycommon.FileContext) []model.Operation {
	var operations []model.Operation
	for _, match := range functionJSONRouteRE.FindAllStringSubmatch(ctx.Source, -1) {
		methods := pycommon.ExtractStringLiterals(match[2])
		if len(methods) == 0 {
			methods = []string{"ANY"}
		}
		for _, method := range methods {
			method = strings.ToUpper(method)
			if method != "ANY" && !pycommon.IsHTTPMethod(method) {
				continue
			}
			operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", method, match[1], "Azure Function HTTP trigger", ctx.SourcePath, "azure-functions", "medium"))
		}
	}
	return operations
}

func chaliceOperations(ctx pycommon.FileContext) []model.Operation {
	var operations []model.Operation
	for _, block := range pycommon.DecoratedBlocks(ctx.Source) {
		for _, decorator := range block.Decorators {
			if !strings.Contains(decorator, ".route") {
				continue
			}
			route, ok := pycommon.FirstString(decorator)
			if !ok {
				continue
			}
			methods := pycommon.KeywordStrings(decorator, "methods")
			if len(methods) == 0 {
				methods = []string{"GET"}
			}
			for _, method := range methods {
				method = strings.ToUpper(method)
				if pycommon.IsHTTPMethod(method) {
					operations = append(operations, pycommon.Operation(ctx.ServiceName, "SERVER", "http", method, route, block.Name, ctx.SourcePath, "chalice", "high"))
				}
			}
		}
	}
	return operations
}

func hasServerlessHint(source string) bool {
	return strings.Contains(source, "functions_framework") ||
		strings.Contains(source, "azure.functions") ||
		strings.Contains(source, "aws_lambda") ||
		strings.Contains(source, "APIGatewayProxy") ||
		strings.Contains(source, "lambda_handler") ||
		strings.Contains(source, "firebase_functions") ||
		strings.Contains(source, "Chalice(")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
