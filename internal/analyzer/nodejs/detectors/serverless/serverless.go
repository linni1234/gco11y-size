package serverless

import (
	"regexp"
	"strings"

	nodecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	serverlessHTTPBlockRE = regexp.MustCompile(`(?ms)-\s+http(?:Api)?\s*:\s*(?:\n\s+)?(?:path\s*:\s*([^\n]+).*?method\s*:\s*([A-Za-z]+)|method\s*:\s*([A-Za-z]+).*?path\s*:\s*([^\n]+))`)
	functionURLRE         = regexp.MustCompile(`(?m)\bexports\.handler\b|\bexport\s+(?:async\s+)?function\s+(?:handler|fetch)\b|onRequest\s*\(`)
)

func Operations(ctx nodecommon.FileContext) []model.Operation {
	lower := strings.ToLower(ctx.SourcePath)
	if strings.HasSuffix(lower, "serverless.yml") || strings.HasSuffix(lower, "serverless.yaml") {
		return serverlessYAMLOperations(ctx)
	}
	if functionURLRE.MatchString(ctx.Source) && hasServerlessHint(ctx.Source) {
		return []model.Operation{nodecommon.Operation(ctx.ServiceName, "SERVER", "http", "ANY", "/{function}", "serverless handler", ctx.SourcePath, "serverless", "low")}
	}
	return nil
}

func ConfigFindings(ctx nodecommon.FileContext) []model.ConfigFinding {
	if strings.Contains(strings.ToLower(ctx.SourcePath), "serverless") || hasServerlessHint(ctx.Source) {
		return []model.ConfigFinding{{Kind: "nodejs-serverless", Name: "serverless", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName}}
	}
	return nil
}

func serverlessYAMLOperations(ctx nodecommon.FileContext) []model.Operation {
	var operations []model.Operation
	for _, match := range serverlessHTTPBlockRE.FindAllStringSubmatch(ctx.Source, -1) {
		path := firstNonEmpty(match[1], match[4])
		method := strings.ToUpper(firstNonEmpty(match[2], match[3]))
		if method == "" {
			method = "ANY"
		}
		if strings.EqualFold(method, "ANY") || nodecommon.IsHTTPMethod(method) {
			operations = append(operations, nodecommon.Operation(ctx.ServiceName, "SERVER", "http", method, strings.Trim(path, ` "'`), "serverless HTTP event", ctx.SourcePath, "serverless", "medium"))
		}
	}
	return operations
}

func hasServerlessHint(source string) bool {
	return strings.Contains(source, "aws-lambda") ||
		strings.Contains(source, "firebase-functions") ||
		strings.Contains(source, "onRequest") ||
		strings.Contains(source, "APIGatewayProxy") ||
		strings.Contains(source, "RequestHandler")
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
