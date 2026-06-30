package python

import (
	pycommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type operationDetector func(pycommon.FileContext) []model.Operation
type edgeDetector func(pycommon.FileContext) []model.Edge
type configFindingDetector func(pycommon.FileContext) []model.ConfigFinding
type riskDetector func(pycommon.FileContext) []model.Risk

func runOperationDetectors(ctx pycommon.FileContext, detectors ...operationDetector) []model.Operation {
	var operations []model.Operation
	for _, detector := range detectors {
		operations = append(operations, detector(ctx)...)
	}
	return operations
}

func runEdgeDetectors(ctx pycommon.FileContext, detectors ...edgeDetector) []model.Edge {
	var edges []model.Edge
	for _, detector := range detectors {
		edges = append(edges, detector(ctx)...)
	}
	return edges
}

func runConfigFindingDetectors(ctx pycommon.FileContext, detectors ...configFindingDetector) []model.ConfigFinding {
	var findings []model.ConfigFinding
	for _, detector := range detectors {
		findings = append(findings, detector(ctx)...)
	}
	return findings
}

func runRiskDetectors(ctx pycommon.FileContext, detectors ...riskDetector) []model.Risk {
	var risks []model.Risk
	for _, detector := range detectors {
		risks = append(risks, detector(ctx)...)
	}
	return risks
}
