package nodejs

import (
	nodecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type operationDetector func(nodecommon.FileContext) []model.Operation
type edgeDetector func(nodecommon.FileContext) []model.Edge
type configFindingDetector func(nodecommon.FileContext) []model.ConfigFinding
type riskDetector func(nodecommon.FileContext) []model.Risk

func runOperationDetectors(ctx nodecommon.FileContext, detectors ...operationDetector) []model.Operation {
	var operations []model.Operation
	for _, detector := range detectors {
		operations = append(operations, detector(ctx)...)
	}
	return operations
}

func runEdgeDetectors(ctx nodecommon.FileContext, detectors ...edgeDetector) []model.Edge {
	var edges []model.Edge
	for _, detector := range detectors {
		edges = append(edges, detector(ctx)...)
	}
	return edges
}

func runConfigFindingDetectors(ctx nodecommon.FileContext, detectors ...configFindingDetector) []model.ConfigFinding {
	var findings []model.ConfigFinding
	for _, detector := range detectors {
		findings = append(findings, detector(ctx)...)
	}
	return findings
}

func runRiskDetectors(ctx nodecommon.FileContext, detectors ...riskDetector) []model.Risk {
	var risks []model.Risk
	for _, detector := range detectors {
		risks = append(risks, detector(ctx)...)
	}
	return risks
}
