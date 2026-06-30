package python

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	pycommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/dependencies"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/grpc"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/otel"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/serverless"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/web"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type Analyzer struct{}

func New() Analyzer {
	return Analyzer{}
}

func (Analyzer) ID() string {
	return "python"
}

func (Analyzer) Analyze(ctx framework.Context) (framework.Result, error) {
	result := framework.Result{ServiceNames: map[string]string{}}
	serviceNames := copyServiceNames(ctx.ServiceNames)
	packageCount := discoverPackages(ctx.Repo, &result, serviceNames)
	pythonFiles := 0
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

		root := inferPythonServiceRoot(ctx.Repo, path)
		serviceName, serviceSource := serviceNameForRoot(ctx.Repo, root, serviceNames)
		sourcePath := basecommon.RelPath(ctx.Repo, path)
		ext := strings.ToLower(filepath.Ext(path))

		switch {
		case isPythonSource(path):
			pythonFiles++
			if shouldSkipPythonSource(ctx.Repo, path) {
				return nil
			}
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", sourcePath, readErr))
				return nil
			}
			source := pycommon.StripPythonComments(string(content))
			ensureService(&result, model.Service{Name: serviceName, Root: basecommon.RelPath(ctx.Repo, root), Source: serviceSource})
			fileCtx := pycommon.FileContext{ServiceName: serviceName, SourcePath: sourcePath, Source: source}
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
			if packageCount == 0 || basecommon.IsTestPath(ctx.Repo, path) {
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
		case isPythonConfigFile(filepath.Base(path)):
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", sourcePath, readErr))
				return nil
			}
			fileCtx := pycommon.FileContext{ServiceName: serviceName, SourcePath: sourcePath, Source: string(content)}
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

	if pythonFiles > 0 || packageCount > 0 || backendEvidence {
		result.DetectedLanguages = appendUnique(result.DetectedLanguages, "python")
	}
	if pythonFiles == 0 && protoFiles > 0 {
		result.Warnings = append(result.Warnings, "protobuf service definitions were detected without Python source files")
	}
	return result, nil
}

func isPythonSource(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".py")
}

func shouldSkipPythonSource(repo string, path string) bool {
	if basecommon.IsTestPath(repo, path) {
		return true
	}
	slashPath := filepath.ToSlash(path)
	name := strings.ToLower(filepath.Base(path))
	if strings.HasSuffix(name, "_pb2.py") || strings.HasSuffix(name, "_pb2_grpc.py") || strings.HasSuffix(name, ".generated.py") {
		return true
	}
	if strings.Contains(slashPath, "/migrations/") ||
		strings.Contains(slashPath, "/notebooks/") ||
		strings.Contains(slashPath, "/docs/") ||
		strings.Contains(slashPath, "/examples/") {
		return true
	}
	return false
}

func isPythonConfigFile(name string) bool {
	lower := strings.ToLower(name)
	return lower == "serverless.yml" ||
		lower == "serverless.yaml" ||
		lower == "template.yml" ||
		lower == "template.yaml" ||
		lower == "function.json"
}

func hasBackendHint(sourcePath string, source string) bool {
	return pycommon.HasImport(source, "fastapi", "starlette", "flask", "quart", "django", "rest_framework", "sanic", "aiohttp", "tornado", "falcon", "bottle", "litestar", "starlite", "grpc", "celery", "rq", "dramatiq", "huey", "apscheduler", "opentelemetry", "boto3") ||
		strings.Contains(source, "FastAPI(") ||
		strings.Contains(source, "Flask(") ||
		strings.Contains(source, "APIRouter(") ||
		strings.Contains(source, "urlpatterns") ||
		strings.Contains(source, "DefaultRouter(") ||
		strings.Contains(source, "Servicer_to_server") ||
		strings.Contains(source, "Celery(") ||
		strings.Contains(sourcePath, "asgi.py") ||
		strings.Contains(sourcePath, "wsgi.py")
}
