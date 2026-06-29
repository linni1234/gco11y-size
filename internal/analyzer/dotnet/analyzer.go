package dotnet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/aspnet"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/background"
	dotnetcommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/dependencies"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/grpc"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/otel"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type Analyzer struct{}

func New() Analyzer {
	return Analyzer{}
}

func (Analyzer) ID() string {
	return "dotnet"
}

func (Analyzer) Analyze(ctx framework.Context) (framework.Result, error) {
	result := framework.Result{ServiceNames: map[string]string{}}
	serviceNames := copyServiceNames(ctx.ServiceNames)
	projectCount := discoverProjects(ctx.Repo, &result, serviceNames)
	configCount := scanDotNetConfigs(ctx.Repo, &result, serviceNames)
	csFiles := 0
	razorFiles := 0
	protoFiles := 0
	aspnetEvidence := false

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
		switch ext {
		case ".cs":
			if basecommon.IsTestPath(ctx.Repo, path) || isGeneratedCSharp(path) {
				return nil
			}
			csFiles++
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", sourcePath, readErr))
				return nil
			}
			ensureService(&result, model.Service{Name: serviceName, Root: basecommon.RelPath(ctx.Repo, root), Source: serviceSource})
			fileCtx := dotnetcommon.FileContext{ServiceName: serviceName, SourcePath: sourcePath, Source: dotnetcommon.StripCSharpComments(string(content))}
			operations := runOperationDetectors(fileCtx,
				aspnet.Operations,
				grpc.Operations,
				dependencies.Operations,
				background.Operations,
				otel.Operations,
			)
			if hasASPNETEvidence(fileCtx.Source, operations) {
				aspnetEvidence = true
			}
			result.Operations = append(result.Operations, operations...)
			result.Edges = append(result.Edges, runEdgeDetectors(fileCtx, grpc.Edges, dependencies.Edges)...)
			result.ConfigFindings = append(result.ConfigFindings, runConfigFindingDetectors(fileCtx, dependencies.ConfigFindings, background.ConfigFindings, otel.ConfigFindings)...)
			result.Risks = append(result.Risks, runRiskDetectors(fileCtx, otel.Risks)...)
		case ".cshtml", ".razor":
			if basecommon.IsTestPath(ctx.Repo, path) {
				return nil
			}
			razorFiles++
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", sourcePath, readErr))
				return nil
			}
			ensureService(&result, model.Service{Name: serviceName, Root: basecommon.RelPath(ctx.Repo, root), Source: serviceSource})
			fileCtx := dotnetcommon.FileContext{ServiceName: serviceName, SourcePath: sourcePath, Source: string(content)}
			operations := aspnet.RazorOperations(fileCtx)
			if len(operations) > 0 {
				aspnetEvidence = true
			}
			result.Operations = append(result.Operations, operations...)
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

	if csFiles > 0 || razorFiles > 0 || projectCount > 0 {
		result.DetectedLanguages = appendUnique(result.DetectedLanguages, "csharp")
		if aspnetEvidence {
			result.DetectedLanguages = appendUnique(result.DetectedLanguages, "csharp/aspnet-core")
		}
	}
	if csFiles == 0 && protoFiles > 0 {
		result.Warnings = append(result.Warnings, "protobuf service definitions were detected without C# source files")
	}
	if projectCount == 0 && configCount > 0 && (csFiles > 0 || razorFiles > 0) {
		result.Warnings = append(result.Warnings, "C# source files were detected without a .csproj service root")
	}
	return result, nil
}

func scanDotNetConfigs(repo string, result *framework.Result, serviceNames map[string]string) int {
	count := 0
	_ = filepath.WalkDir(repo, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			result.Warnings = append(result.Warnings, err.Error())
			return nil
		}
		if entry.IsDir() {
			if basecommon.ShouldSkipDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !dependencies.IsDotNetConfig(path) {
			return nil
		}
		count++
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			result.Warnings = append(result.Warnings, readErr.Error())
			return nil
		}
		root := basecommon.InferServiceRoot(repo, path)
		sourcePath := basecommon.RelPath(repo, path)
		for _, value := range configServiceNames(content) {
			if value.Value == "" {
				continue
			}
			serviceNames[root] = value.Value
			result.ServiceNames[root] = value.Value
			result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{Kind: "service-name", Name: value.Name, Value: value.Value, Source: sourcePath, Service: value.Value})
		}
		serviceName, serviceSource := serviceNameForRoot(repo, root, serviceNames)
		for _, value := range configEnvironments(content) {
			if value.Value == "" {
				continue
			}
			result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{Kind: "environment", Name: value.Name, Value: value.Value, Source: sourcePath, Service: serviceName})
		}
		configResult := dependencies.ConfigFromJSON(serviceName, sourcePath, string(content))
		if len(configResult.Operations) > 0 || len(configResult.Edges) > 0 {
			ensureService(result, model.Service{Name: serviceName, Root: basecommon.RelPath(repo, root), Source: serviceSource})
		}
		result.Operations = append(result.Operations, configResult.Operations...)
		result.Edges = append(result.Edges, configResult.Edges...)
		result.ConfigFindings = append(result.ConfigFindings, configResult.ConfigFindings...)
		return nil
	})
	return count
}

func configServiceNames(content []byte) []configValue {
	serviceNames, _ := dotnetConfigMetadata(content)
	return serviceNames
}

func configEnvironments(content []byte) []configValue {
	_, environments := dotnetConfigMetadata(content)
	return environments
}

func hasASPNETEvidence(source string, operations []model.Operation) bool {
	if strings.Contains(source, "Microsoft.AspNetCore") || strings.Contains(source, "WebApplication.CreateBuilder") || strings.Contains(source, "MapGet(") || strings.Contains(source, "MapControllerRoute(") {
		return true
	}
	for _, op := range operations {
		for _, detector := range op.Detectors {
			if strings.Contains(detector, "aspnet") || detector == "signalr" || detector == "razor-pages" || detector == "blazor" || detector == "yarp" {
				return true
			}
		}
	}
	return false
}

func isGeneratedCSharp(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.HasSuffix(name, ".g.cs") ||
		strings.HasSuffix(name, ".designer.cs") ||
		strings.HasSuffix(name, ".generated.cs")
}
