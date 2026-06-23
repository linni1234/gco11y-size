package java

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	javacommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/grpc"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/jaxrs"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/messaging"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/otel"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/outbound"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/routing"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/servlet"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/spring"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type Analyzer struct{}

func New() Analyzer {
	return Analyzer{}
}

func (Analyzer) ID() string {
	return "java"
}

func (Analyzer) Analyze(ctx framework.Context) (framework.Result, error) {
	springResult, err := spring.New().Analyze(ctx)
	if err != nil {
		return framework.Result{}, err
	}
	result := normalizeSpringResult(springResult)
	serviceNames := copyServiceNames(ctx.ServiceNames)
	for root, serviceName := range result.ServiceNames {
		serviceNames[root] = serviceName
	}

	javaFiles := 0
	protoFiles := 0
	err = filepath.WalkDir(ctx.Repo, func(path string, entry os.DirEntry, walkErr error) error {
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
		switch strings.ToLower(filepath.Ext(path)) {
		case ".java":
			if basecommon.IsTestPath(ctx.Repo, path) {
				return nil
			}
			javaFiles++
			ensureService(&result, model.Service{Name: serviceName, Root: basecommon.RelPath(ctx.Repo, root), Source: serviceSource})
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", basecommon.RelPath(ctx.Repo, path), readErr))
				return nil
			}
			sourcePath := basecommon.RelPath(ctx.Repo, path)
			source := string(content)
			result.Operations = append(result.Operations, runOperationDetectors(
				serviceName,
				sourcePath,
				source,
				jaxrs.Operations,
				servlet.Operations,
				routing.Operations,
				grpc.JavaOperations,
				messaging.Operations,
				otel.Operations,
			)...)
			result.Edges = append(result.Edges, runEdgeDetectors(serviceName, sourcePath, source, outbound.Edges)...)
			result.Risks = append(result.Risks, runRiskDetectors(sourcePath, source, otel.Risks)...)
		case ".proto":
			if basecommon.IsTestPath(ctx.Repo, path) {
				return nil
			}
			protoFiles++
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", basecommon.RelPath(ctx.Repo, path), readErr))
				return nil
			}
			sourcePath := basecommon.RelPath(ctx.Repo, path)
			if serviceSource == "config" {
				ensureService(&result, model.Service{Name: serviceName, Root: basecommon.RelPath(ctx.Repo, root), Source: serviceSource})
				result.Operations = append(result.Operations, grpc.ProtoOperations(serviceName, sourcePath, string(content))...)
			} else if grpc.ProtoServiceCount(string(content)) > 0 {
				result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{
					Kind:   "grpc-proto",
					Name:   filepath.Base(path),
					Value:  "service definitions detected but not counted without a service identity",
					Source: sourcePath,
				})
			}
		case ".xml":
			if strings.EqualFold(filepath.Base(path), "web.xml") {
				if basecommon.IsTestPath(ctx.Repo, path) {
					return nil
				}
				content, readErr := os.ReadFile(path)
				if readErr != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("could not read %s: %v", basecommon.RelPath(ctx.Repo, path), readErr))
					return nil
				}
				ensureService(&result, model.Service{Name: serviceName, Root: basecommon.RelPath(ctx.Repo, root), Source: serviceSource})
				result.Operations = append(result.Operations, servlet.WebXMLOperations(serviceName, basecommon.RelPath(ctx.Repo, path), string(content))...)
			}
		}
		return nil
	})
	if err != nil {
		return framework.Result{}, err
	}

	if javaFiles > 0 || protoFiles > 0 || len(result.Operations) > 0 {
		result.DetectedLanguages = javacommon.AppendUnique(result.DetectedLanguages, "java")
		result.Warnings = filterNoSpringOperationsWarning(result.Warnings)
	}
	return result, nil
}
