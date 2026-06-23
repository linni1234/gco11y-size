package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func Analyze(repo string, otelConfig string) (model.Analysis, error) {
	absRepo, err := filepath.Abs(repo)
	if err != nil {
		return model.Analysis{}, err
	}
	info, err := os.Stat(absRepo)
	if err != nil {
		return model.Analysis{}, err
	}
	if !info.IsDir() {
		return model.Analysis{}, fmt.Errorf("--repo must point to a directory")
	}

	base := scanConfig(absRepo, otelConfig)
	serviceNames := copyStringMap(base.ServiceNames)
	results := []framework.Result{base}

	for _, analyzer := range registeredAnalyzers() {
		result, err := analyzer.Analyze(framework.Context{
			Repo:         absRepo,
			OtelConfig:   otelConfig,
			ServiceNames: copyStringMap(serviceNames),
		})
		if err != nil {
			return model.Analysis{}, fmt.Errorf("%s analyzer failed: %w", analyzer.ID(), err)
		}
		for root, serviceName := range result.ServiceNames {
			serviceNames[root] = serviceName
		}
		results = append(results, result)
	}

	return finalizeAnalysis(absRepo, results), nil
}

func registeredAnalyzers() []framework.Analyzer {
	return []framework.Analyzer{
		java.New(),
	}
}

func finalizeAnalysis(repo string, results []framework.Result) model.Analysis {
	services := map[string]*model.Service{}
	var operations []model.Operation
	var edges []model.Edge
	var findings []model.ConfigFinding
	var risks []model.Risk
	var warnings []string
	var languages []string

	for _, result := range results {
		for _, service := range result.Services {
			if service.Name == "" {
				continue
			}
			service.OperationCount = 0
			service.EdgeCount = 0
			if existing := services[service.Name]; existing != nil {
				mergeService(existing, service)
				continue
			}
			copyService := service
			services[service.Name] = &copyService
		}
		operations = append(operations, result.Operations...)
		edges = append(edges, result.Edges...)
		findings = append(findings, result.ConfigFindings...)
		risks = append(risks, result.Risks...)
		warnings = append(warnings, result.Warnings...)
		languages = appendUnique(languages, result.DetectedLanguages...)
	}

	operations = dedupeOperations(operations)
	edges = dedupeEdges(edges)
	for _, op := range operations {
		service := services[op.Service]
		if service == nil {
			services[op.Service] = &model.Service{Name: op.Service, Root: ".", Source: "operation"}
			service = services[op.Service]
		}
		service.OperationCount++
	}
	for _, edge := range edges {
		if service := services[edge.SourceService]; service != nil {
			service.EdgeCount++
		}
	}

	serviceList := make([]model.Service, 0, len(services))
	for _, service := range services {
		serviceList = append(serviceList, *service)
	}
	sort.Slice(serviceList, func(i, j int) bool {
		return serviceList[i].Name < serviceList[j].Name
	})
	sort.Slice(operations, func(i, j int) bool {
		if operations[i].Service != operations[j].Service {
			return operations[i].Service < operations[j].Service
		}
		if operations[i].Route != operations[j].Route {
			return operations[i].Route < operations[j].Route
		}
		return operations[i].Method < operations[j].Method
	})
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].SourceService != edges[j].SourceService {
			return edges[i].SourceService < edges[j].SourceService
		}
		return edges[i].TargetService < edges[j].TargetService
	})
	sort.Slice(risks, func(i, j int) bool {
		if risks[i].Severity != risks[j].Severity {
			return riskRank(risks[i].Severity) > riskRank(risks[j].Severity)
		}
		return risks[i].Message < risks[j].Message
	})

	language := "unknown"
	if len(languages) > 0 {
		language = strings.Join(languages, ",")
	} else {
		warnings = append(warnings, "no supported application source files were found under the repository path")
	}

	return model.Analysis{
		Repository:       repo,
		GeneratedAt:      time.Now().UTC(),
		Services:         serviceList,
		Operations:       operations,
		Edges:            edges,
		ConfigFindings:   findings,
		Risks:            risks,
		Warnings:         warnings,
		DetectedLanguage: language,
	}
}

func mergeService(existing *model.Service, next model.Service) {
	if existing.Root == "" || existing.Root == "." {
		existing.Root = next.Root
	}
	if existing.Source == "" || existing.Source == "operation" || (existing.Source == "path" && next.Source == "config") {
		existing.Source = next.Source
	}
}

func dedupeOperations(operations []model.Operation) []model.Operation {
	seen := map[string]model.Operation{}
	for _, op := range operations {
		op = normalizeOperation(op)
		key := operationKey(op)
		if existing, ok := seen[key]; ok {
			seen[key] = mergeOperation(existing, op)
			continue
		}
		seen[key] = op
	}
	out := make([]model.Operation, 0, len(seen))
	for _, op := range seen {
		out = append(out, op)
	}
	return out
}

func normalizeOperation(op model.Operation) model.Operation {
	op.Kind = strings.ToUpper(strings.TrimSpace(op.Kind))
	op.Method = strings.ToUpper(strings.TrimSpace(op.Method))
	op.Protocol = strings.ToLower(strings.TrimSpace(op.Protocol))
	op.Confidence = strings.ToLower(strings.TrimSpace(op.Confidence))
	if op.Kind == "" {
		op.Kind = "SERVER"
	}
	if op.Method == "" {
		op.Method = "ANY"
	}
	if op.Protocol == "" {
		op.Protocol = inferOperationProtocol(op)
	}
	if op.Confidence == "" {
		op.Confidence = "medium"
	}
	if len(op.Detectors) == 0 && op.Origin != "" {
		op.Detectors = []string{op.Origin}
	}
	return op
}

func inferOperationProtocol(op model.Operation) string {
	route := strings.ToLower(op.Route)
	switch {
	case strings.HasPrefix(route, "kafka:"):
		return "kafka"
	case strings.HasPrefix(route, "rabbit:"):
		return "rabbit"
	case strings.HasPrefix(route, "jms:"):
		return "jms"
	case strings.HasPrefix(route, "grpc:"):
		return "grpc"
	default:
		return "http"
	}
}

func operationKey(op model.Operation) string {
	return strings.Join([]string{
		op.Service,
		op.Kind,
		op.Protocol,
		op.Method,
		canonicalOperation(op.Protocol, op.Route),
	}, "\x00")
}

func canonicalOperation(protocol string, route string) string {
	route = strings.TrimSpace(route)
	switch strings.ToLower(protocol) {
	case "http":
		route = regexpMustCompile(`\{[^}]+\}`).ReplaceAllString(route, "{}")
		route = regexpMustCompile(`/+`).ReplaceAllString(route, "/")
		return strings.ToLower(strings.TrimRight(route, "/"))
	case "grpc":
		route = strings.TrimPrefix(route, "grpc:")
		parts := strings.Split(route, "/")
		if len(parts) == 2 {
			serviceParts := strings.Split(parts[0], ".")
			parts[0] = serviceParts[len(serviceParts)-1]
			parts[1] = strings.ToLower(parts[1])
			return strings.ToLower(strings.Join(parts, "/"))
		}
		return strings.ToLower(route)
	default:
		return strings.ToLower(route)
	}
}

func mergeOperation(existing model.Operation, next model.Operation) model.Operation {
	if confidenceRank(next.Confidence) > confidenceRank(existing.Confidence) {
		next.Risks = appendUnique(next.Risks, existing.Risks...)
		next.Detectors = appendUnique(next.Detectors, existing.Detectors...)
		next.Source = mergeDelimited(next.Source, existing.Source)
		if next.Handler == "" {
			next.Handler = existing.Handler
		}
		if next.Origin == "" {
			next.Origin = existing.Origin
		}
		return next
	}
	existing.Risks = appendUnique(existing.Risks, next.Risks...)
	existing.Detectors = appendUnique(existing.Detectors, next.Detectors...)
	existing.Source = mergeDelimited(existing.Source, next.Source)
	if existing.Handler == "" {
		existing.Handler = next.Handler
	}
	if existing.Origin == "" {
		existing.Origin = next.Origin
	}
	return existing
}

func mergeDelimited(first string, second string) string {
	if strings.TrimSpace(first) == "" {
		return second
	}
	if strings.TrimSpace(second) == "" || strings.Contains(first, second) {
		return first
	}
	return first + ", " + second
}

func dedupeEdges(edges []model.Edge) []model.Edge {
	seen := map[string]model.Edge{}
	for _, edge := range edges {
		if edge.TargetService == "" || edge.TargetService == edge.SourceService {
			continue
		}
		key := strings.Join([]string{edge.SourceService, edge.TargetService, edge.Protocol}, "\x00")
		if existing, ok := seen[key]; ok {
			if confidenceRank(edge.Confidence) > confidenceRank(existing.Confidence) {
				seen[key] = edge
			}
			continue
		}
		seen[key] = edge
	}
	out := make([]model.Edge, 0, len(seen))
	for _, edge := range seen {
		out = append(out, edge)
	}
	return out
}

func riskRank(severity string) int {
	switch strings.ToLower(severity) {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func confidenceRank(confidence string) int {
	switch strings.ToLower(confidence) {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func appendUnique(values []string, extra ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range extra {
		if value != "" && !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}

func copyStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func regexpMustCompile(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}
