package golang

import (
	gocommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type operationDetector func(gocommon.FileContext) []model.Operation
type edgeDetector func(gocommon.FileContext) []model.Edge
type riskDetector func(gocommon.FileContext) []model.Risk

func runOperationDetectors(ctx gocommon.FileContext, detectors ...operationDetector) []model.Operation {
	var operations []model.Operation
	for _, detector := range detectors {
		operations = append(operations, detector(ctx)...)
	}
	return operations
}

func runEdgeDetectors(ctx gocommon.FileContext, detectors ...edgeDetector) []model.Edge {
	var edges []model.Edge
	for _, detector := range detectors {
		edges = append(edges, detector(ctx)...)
	}
	return edges
}

func runRiskDetectors(ctx gocommon.FileContext, detectors ...riskDetector) []model.Risk {
	var risks []model.Risk
	for _, detector := range detectors {
		risks = append(risks, detector(ctx)...)
	}
	return risks
}
