package dotnet

import (
	dotnetcommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type operationDetector func(dotnetcommon.FileContext) []model.Operation
type edgeDetector func(dotnetcommon.FileContext) []model.Edge
type configFindingDetector func(dotnetcommon.FileContext) []model.ConfigFinding
type riskDetector func(dotnetcommon.FileContext) []model.Risk

func runOperationDetectors(ctx dotnetcommon.FileContext, detectors ...operationDetector) []model.Operation {
	var operations []model.Operation
	for _, detector := range detectors {
		operations = append(operations, detector(ctx)...)
	}
	return operations
}

func runEdgeDetectors(ctx dotnetcommon.FileContext, detectors ...edgeDetector) []model.Edge {
	var edges []model.Edge
	for _, detector := range detectors {
		edges = append(edges, detector(ctx)...)
	}
	return edges
}

func runConfigFindingDetectors(ctx dotnetcommon.FileContext, detectors ...configFindingDetector) []model.ConfigFinding {
	var findings []model.ConfigFinding
	for _, detector := range detectors {
		findings = append(findings, detector(ctx)...)
	}
	return findings
}

func runRiskDetectors(ctx dotnetcommon.FileContext, detectors ...riskDetector) []model.Risk {
	var risks []model.Risk
	for _, detector := range detectors {
		risks = append(risks, detector(ctx)...)
	}
	return risks
}
