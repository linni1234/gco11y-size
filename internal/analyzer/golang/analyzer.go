package golang

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/chi"
	gocommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/connect"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/echo"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/fiber"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/gin"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/gorilla"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/grpc"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/messaging"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/nethttp"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/otel"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/outbound"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type Analyzer struct{}

func New() Analyzer {
	return Analyzer{}
}

func (Analyzer) ID() string {
	return "go"
}

func (Analyzer) Analyze(ctx framework.Context) (framework.Result, error) {
	result := framework.Result{ServiceNames: map[string]string{}}
	serviceNames := copyServiceNames(ctx.ServiceNames)
	goModules := discoverGoModules(ctx.Repo, &result, serviceNames)
	goFiles := 0
	protoFiles := 0
	var fileContexts []gocommon.FileContext

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
		switch strings.ToLower(filepath.Ext(path)) {
		case ".go":
			if basecommon.IsTestPath(ctx.Repo, path) || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			goFiles++
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", sourcePath, readErr))
				return nil
			}
			file, parseErr := parser.ParseFile(token.NewFileSet(), path, content, parser.ParseComments)
			if parseErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not parse Go file %s: %v", sourcePath, parseErr))
				return nil
			}
			ensureService(&result, model.Service{Name: serviceName, Root: basecommon.RelPath(ctx.Repo, root), Source: serviceSource})
			fileCtx := gocommon.FileContext{ServiceName: serviceName, SourcePath: sourcePath, File: file, Source: string(content)}
			fileContexts = append(fileContexts, fileCtx)
			result.Operations = append(result.Operations, runOperationDetectors(
				fileCtx,
				nethttp.Operations,
				gin.Operations,
				echo.Operations,
				chi.Operations,
				gorilla.Operations,
				fiber.Operations,
				grpc.Operations,
				connect.Operations,
				messaging.Operations,
				otel.Operations,
			)...)
			result.Edges = append(result.Edges, runEdgeDetectors(fileCtx, grpc.Edges, messaging.Edges, outbound.Edges)...)
			result.Risks = append(result.Risks, runRiskDetectors(fileCtx, otel.Risks)...)
		case ".proto":
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
			}
		}
		return nil
	})
	if err != nil {
		return framework.Result{}, err
	}

	result.Operations = append(result.Operations, gin.ProjectOperations(fileContexts)...)

	if goFiles > 0 || goModules > 0 {
		result.DetectedLanguages = append(result.DetectedLanguages, "go")
	}
	if goFiles == 0 && protoFiles > 0 {
		result.Warnings = append(result.Warnings, "protobuf service definitions were detected without Go source files")
	}
	return result, nil
}
