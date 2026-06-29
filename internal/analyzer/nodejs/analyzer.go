package nodejs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	nodecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/dependencies"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/grpc"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/otel"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/serverless"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/web"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type Analyzer struct{}

func New() Analyzer {
	return Analyzer{}
}

func (Analyzer) ID() string {
	return "nodejs"
}

func (Analyzer) Analyze(ctx framework.Context) (framework.Result, error) {
	result := framework.Result{ServiceNames: map[string]string{}}
	serviceNames := copyServiceNames(ctx.ServiceNames)
	packageCount := discoverPackages(ctx.Repo, &result, serviceNames)
	jsFiles := 0
	tsFiles := 0
	protoFiles := 0
	backendEvidence := false

	err := filepath.WalkDir(ctx.Repo, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", path, walkErr))
			return nil
		}
		if entry.IsDir() {
			if basecommon.ShouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		root := basecommon.InferServiceRoot(ctx.Repo, path)
		serviceName, serviceSource := serviceNameForRoot(ctx.Repo, root, serviceNames)
		sourcePath := basecommon.RelPath(ctx.Repo, path)
		ext := strings.ToLower(filepath.Ext(path))

		switch {
		case isJavaScriptSource(path):
			if basecommon.IsTestPath(ctx.Repo, path) || isGeneratedJS(path) {
				return nil
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", sourcePath, readErr))
				return nil
			}
			source := nodecommon.StripJSComments(string(content))
			if ext == ".ts" || ext == ".tsx" || ext == ".mts" || ext == ".cts" {
				tsFiles++
			} else {
				jsFiles++
			}
			if shouldSkipFrontendSource(sourcePath, source) {
				return nil
			}
			ensureService(&result, model.Service{Name: serviceName, Root: basecommon.RelPath(ctx.Repo, root), Source: serviceSource})
			fileCtx := nodecommon.FileContext{ServiceName: serviceName, SourcePath: sourcePath, Source: source}
			operations := runOperationDetectors(fileCtx, web.Operations, serverless.Operations, grpc.Operations, dependencies.Operations, otel.Operations)
			edges := runEdgeDetectors(fileCtx, grpc.Edges, dependencies.Edges)
			findings := runConfigFindingDetectors(fileCtx, web.ConfigFindings, serverless.ConfigFindings, dependencies.ConfigFindings, otel.ConfigFindings)
			if len(operations) > 0 || len(edges) > 0 || hasBackendHint(sourcePath, source) {
				backendEvidence = true
			}
			result.Operations = append(result.Operations, operations...)
			result.Edges = append(result.Edges, edges...)
			result.ConfigFindings = append(result.ConfigFindings, findings...)
			result.Risks = append(result.Risks, runRiskDetectors(fileCtx, otel.Risks)...)
		case strings.EqualFold(ext, ".proto"):
			if packageCount == 0 {
				return nil
			}
			if basecommon.IsTestPath(ctx.Repo, path) {
				return nil
			}
			protoFiles++
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", sourcePath, readErr))
				return nil
			}
			if grpc.ProtoServiceCount(string(content)) > 0 {
				ensureService(&result, model.Service{Name: serviceName, Root: basecommon.RelPath(ctx.Repo, root), Source: serviceSource})
				result.Operations = append(result.Operations, grpc.ProtoOperations(serviceName, sourcePath, string(content))...)
				backendEvidence = true
			}
		case isConfigRootFile(filepath.Base(path)):
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", sourcePath, readErr))
				return nil
			}
			fileCtx := nodecommon.FileContext{ServiceName: serviceName, SourcePath: sourcePath, Source: string(content)}
			operations := serverless.Operations(fileCtx)
			if len(operations) > 0 {
				ensureService(&result, model.Service{Name: serviceName, Root: basecommon.RelPath(ctx.Repo, root), Source: serviceSource})
				backendEvidence = true
			}
			result.Operations = append(result.Operations, operations...)
			result.ConfigFindings = append(result.ConfigFindings, serverless.ConfigFindings(fileCtx)...)
		}
		return nil
	})
	if err != nil {
		return framework.Result{}, err
	}

	if jsFiles > 0 {
		result.DetectedLanguages = appendUnique(result.DetectedLanguages, "javascript")
	}
	if tsFiles > 0 {
		result.DetectedLanguages = appendUnique(result.DetectedLanguages, "typescript")
	}
	if packageCount > 0 || backendEvidence {
		result.DetectedLanguages = appendUnique(result.DetectedLanguages, "nodejs")
	}
	if jsFiles == 0 && tsFiles == 0 && protoFiles > 0 {
		result.Warnings = append(result.Warnings, "protobuf service definitions were detected without JavaScript or TypeScript source files")
	}
	return result, nil
}

func shouldSkipFrontendSource(sourcePath string, source string) bool {
	path := filepath.ToSlash(sourcePath)
	if strings.Contains(path, "/pages/api/") || strings.HasSuffix(path, "/route.ts") || strings.HasSuffix(path, "/route.js") {
		return false
	}
	if strings.HasSuffix(path, ".jsx") || strings.HasSuffix(path, ".tsx") {
		return !hasBackendHint(path, source)
	}
	if strings.Contains(path, "/app/") || strings.Contains(path, "/pages/") || strings.Contains(path, "/components/") {
		return !hasBackendHint(path, source)
	}
	return false
}

func hasBackendHint(sourcePath string, source string) bool {
	return nodecommon.HasImport(source, "express", "fastify", "@nestjs/", "koa", "@koa/router", "@hapi/hapi", "hapi", "hono", "next/server", "@grpc/grpc-js", "kafkajs", "amqplib", "@opentelemetry/", "aws-lambda", "firebase-functions") ||
		strings.Contains(source, "@Controller") ||
		strings.Contains(source, "WebApplication") ||
		strings.Contains(source, "server.route") ||
		strings.Contains(source, "ApolloServer") ||
		strings.Contains(source, "PrismaClient") ||
		strings.Contains(sourcePath, "/pages/api/") ||
		strings.HasSuffix(sourcePath, "/route.ts") ||
		strings.HasSuffix(sourcePath, "/route.js")
}
