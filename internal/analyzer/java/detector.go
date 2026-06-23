package java

import "github.com/nilslindholm/metricgenerationsizer/internal/model"

type operationDetector func(serviceName string, sourcePath string, source string) []model.Operation
type edgeDetector func(serviceName string, sourcePath string, source string) []model.Edge
type riskDetector func(sourcePath string, source string) []model.Risk

func runOperationDetectors(serviceName string, sourcePath string, source string, detectors ...operationDetector) []model.Operation {
	var operations []model.Operation
	for _, detector := range detectors {
		operations = append(operations, detector(serviceName, sourcePath, source)...)
	}
	return operations
}

func runEdgeDetectors(serviceName string, sourcePath string, source string, detectors ...edgeDetector) []model.Edge {
	var edges []model.Edge
	for _, detector := range detectors {
		edges = append(edges, detector(serviceName, sourcePath, source)...)
	}
	return edges
}

func runRiskDetectors(sourcePath string, source string, detectors ...riskDetector) []model.Risk {
	var risks []model.Risk
	for _, detector := range detectors {
		risks = append(risks, detector(sourcePath, source)...)
	}
	return risks
}
