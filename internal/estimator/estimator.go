package estimator

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/nilslindholm/metricgenerationsizer/internal/config"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func Estimate(analysis model.Analysis, opts model.Options, calibration *model.CloudCalibration) model.Estimate {
	dimensionMultiplier := customDimensionMultiplier(opts.CustomDimensions)
	includedOps := includedOperations(analysis.Operations, opts.IncludeClientProducer)
	statusValues := max(1, opts.StatusValues)
	buckets := max(1, opts.HistogramBuckets)
	histogramType := config.NormalizeHistogramType(opts.HistogramType)
	latencySeriesPerLabelSet := histogramSeriesPerLabelSet(buckets, histogramType)

	var breakdown []model.ProcessorEstimate
	var componentBreakdown []model.ComponentEstimate
	operationContrib := map[string]model.OperationEstimate{}
	serviceContrib := map[string]int{}
	totalExpected := 0
	totalLow := 0
	totalHigh := 0

	if config.ProcessorEnabled(opts.Processors, config.ProcessorSpanMetricsCount) {
		expected := spanMetricTotal(includedOps, opts, statusValues, dimensionMultiplier, 1, 1)
		low := spanMetricTotal(includedOps, opts, max(1, statusValues-1), lowDimensionMultiplier(opts.CustomDimensions), 1, 1)
		high := spanMetricTotal(includedOps, opts, statusValues+1, highDimensionMultiplier(opts.CustomDimensions), 1, 1.05)
		breakdown = append(breakdown, model.ProcessorEstimate{
			Processor: config.ProcessorSpanMetricsCount,
			Low:       low,
			Expected:  expected,
			High:      high,
			Formula:   "unique span-name label sets x environments x status values x custom dimension cardinality x instance label values x 1 count counter",
		})
		totalLow += low
		totalExpected += expected
		totalHigh += high
		addOperationContrib(operationContrib, serviceContrib, includedOps, opts, statusValues, dimensionMultiplier, 1)
	}

	if config.ProcessorEnabled(opts.Processors, config.ProcessorSpanMetricsLatency) {
		expected := spanMetricTotal(includedOps, opts, statusValues, dimensionMultiplier, latencySeriesPerLabelSet, 1)
		low := spanMetricTotal(includedOps, opts, max(1, statusValues-1), lowDimensionMultiplier(opts.CustomDimensions), lowHistogramSeriesPerLabelSet(buckets, histogramType), 1)
		high := spanMetricTotal(includedOps, opts, statusValues+1, highDimensionMultiplier(opts.CustomDimensions), highHistogramSeriesPerLabelSet(buckets, histogramType), 1.05)
		breakdown = append(breakdown, model.ProcessorEstimate{
			Processor: config.ProcessorSpanMetricsLatency,
			Low:       low,
			Expected:  expected,
			High:      high,
			Formula:   fmt.Sprintf("unique span-name label sets x environments x status values x custom dimension cardinality x instance label values x (%s)", latencyHistogramFormula(histogramType)),
		})
		totalLow += low
		totalExpected += expected
		totalHigh += high
		addOperationContrib(operationContrib, serviceContrib, includedOps, opts, statusValues, dimensionMultiplier, latencySeriesPerLabelSet)
	}

	if config.ProcessorEnabled(opts.Processors, config.ProcessorSpanMetricsSize) {
		expected := spanMetricTotal(includedOps, opts, statusValues, dimensionMultiplier, 1, 1)
		low := spanMetricTotal(includedOps, opts, max(1, statusValues-1), lowDimensionMultiplier(opts.CustomDimensions), 1, 1)
		high := spanMetricTotal(includedOps, opts, statusValues+1, highDimensionMultiplier(opts.CustomDimensions), 1, 1.05)
		breakdown = append(breakdown, model.ProcessorEstimate{
			Processor: config.ProcessorSpanMetricsSize,
			Low:       low,
			Expected:  expected,
			High:      high,
			Formula:   "unique span-name label sets x environments x status values x custom dimension cardinality x instance label values x 1 size series",
		})
		totalLow += low
		totalExpected += expected
		totalHigh += high
		addOperationContrib(operationContrib, serviceContrib, includedOps, opts, statusValues, dimensionMultiplier, 1)
	}
	if spanExpected := spanMetricsExpected(breakdown); spanExpected > 0 {
		componentBreakdown = append(componentBreakdown, model.ComponentEstimate{
			Component: "span-metrics",
			Expected:  spanExpected,
			Formula:   fmt.Sprintf("unique span-name label sets (service + operation/span name + span kind) x environments x status values x instance label values x enabled counter/histogram series; latency uses %s", latencyHistogramFormula(histogramType)),
		})
	}

	if config.ProcessorEnabled(opts.Processors, config.ProcessorServiceGraph) {
		seriesPerEdge := 2 + 2*latencySeriesPerLabelSet
		expected := serviceGraphTotal(analysis.Edges, opts, seriesPerEdge, 1)
		low := serviceGraphTotal(analysis.Edges, opts, 2+2*lowHistogramSeriesPerLabelSet(buckets, histogramType), 1)
		high := serviceGraphTotal(analysis.Edges, opts, 2+2*highHistogramSeriesPerLabelSet(buckets, histogramType), 1.2)
		breakdown = append(breakdown, model.ProcessorEstimate{
			Processor: config.ProcessorServiceGraph,
			Low:       low,
			Expected:  expected,
			High:      high,
			Formula:   fmt.Sprintf("directed service edges x environments x instance label values x (request_total + failed_total + 2 x (%s))", latencyHistogramFormula(histogramType)),
		})
		totalLow += low
		totalExpected += expected
		totalHigh += high
		componentBreakdown = append(componentBreakdown, model.ComponentEstimate{
			Component: "service-graph",
			Expected:  expected,
			Formula:   fmt.Sprintf("directed service edges x environments x instance label values x (request_total + failed_total + client histogram + server histogram); each histogram uses %s", latencyHistogramFormula(histogramType)),
		})
		for _, edge := range analysis.Edges {
			sizing := effectiveServiceSizing(edge.SourceService, edge.Repository, opts)
			serviceContrib[serviceContributionKey(edge.SourceService, edge.Repository)] += sizing.environments * generatedMetricInstanceMultiplierForInstances(opts, sizing.instances) * seriesPerEdge
		}
	}

	if config.ProcessorEnabled(opts.Processors, config.ProcessorHostInfo) {
		expected := 0
		low := 0
		high := 0
		for _, service := range analysis.Services {
			sizing := effectiveServiceSizing(service.Name, service.Repository, opts)
			expected += sizing.environments * sizing.instances
			low += sizing.environments
			high += sizing.environments * max(1, sizing.instances+1)
		}
		breakdown = append(breakdown, model.ProcessorEstimate{
			Processor: config.ProcessorHostInfo,
			Low:       low,
			Expected:  expected,
			High:      high,
			Formula:   "unique service/environment/instance combinations, target_info style",
		})
		totalLow += low
		totalExpected += expected
		totalHigh += high
		componentBreakdown = append(componentBreakdown, model.ComponentEstimate{
			Component: "host-info",
			Expected:  expected,
			Formula:   "unique service/environment/instance combinations, target_info style",
		})
		for _, service := range analysis.Services {
			sizing := effectiveServiceSizing(service.Name, service.Repository, opts)
			serviceContrib[serviceContributionKey(service.Name, service.Repository)] += sizing.environments * sizing.instances
		}
	}

	operationContributors := make([]model.OperationEstimate, 0, len(operationContrib))
	for _, contributor := range operationContrib {
		operationContributors = append(operationContributors, contributor)
	}
	sort.Slice(operationContributors, func(i, j int) bool {
		if operationContributors[i].Expected != operationContributors[j].Expected {
			return operationContributors[i].Expected > operationContributors[j].Expected
		}
		if operationContributors[i].Service != operationContributors[j].Service {
			return operationContributors[i].Service < operationContributors[j].Service
		}
		return operationContributors[i].Route < operationContributors[j].Route
	})
	if len(operationContributors) > 20 {
		operationContributors = operationContributors[:20]
	}

	serviceBreakdown := make([]model.ServiceEstimate, 0, len(serviceContrib))
	for key, expected := range serviceContrib {
		service, repository := splitServiceContributionKey(key)
		serviceBreakdown = append(serviceBreakdown, model.ServiceEstimate{Service: service, Repository: repository, Expected: expected})
	}
	sort.Slice(serviceBreakdown, func(i, j int) bool {
		if serviceBreakdown[i].Expected != serviceBreakdown[j].Expected {
			return serviceBreakdown[i].Expected > serviceBreakdown[j].Expected
		}
		return serviceBreakdown[i].Service < serviceBreakdown[j].Service
	})

	return model.Estimate{
		TotalLow:              totalLow,
		TotalExpected:         totalExpected,
		TotalHigh:             max(totalHigh, totalExpected),
		ProcessorBreakdown:    breakdown,
		ComponentBreakdown:    componentBreakdown,
		OperationContributors: operationContributors,
		ServiceBreakdown:      serviceBreakdown,
		UncertaintyModel:      uncertaintyModel(opts, dimensionMultiplier, baseGeneratedMetricInstanceMultiplier(opts), histogramType, buckets),
		Assumptions:           assumptions(opts, dimensionMultiplier),
		CloudCalibration:      calibration,
	}
}

func includedOperations(operations []model.Operation, includeClientProducer bool) []model.Operation {
	var out []model.Operation
	for _, op := range operations {
		switch strings.ToUpper(op.Kind) {
		case "SERVER", "CONSUMER":
			out = append(out, op)
		case "CLIENT", "PRODUCER":
			if includeClientProducer {
				out = append(out, op)
			}
		}
	}
	return out
}

func weightedOperationCount(operations []model.Operation, opts model.Options) int {
	total := 0
	for _, op := range operations {
		total += operationWeight(op, opts)
	}
	return total
}

func spanMetricTotal(operations []model.Operation, opts model.Options, statusValues int, dimensionMultiplier int, histogramFactor int, buffer float64) int {
	total := 0
	for _, op := range operations {
		sizing := effectiveServiceSizing(op.Service, op.Repository, opts)
		instanceMultiplier := generatedMetricInstanceMultiplierForInstances(opts, sizing.instances)
		total += operationWeight(op, opts) * sizing.environments * max(1, statusValues) * max(1, dimensionMultiplier) * instanceMultiplier * max(1, histogramFactor)
	}
	if buffer != 1 {
		return int(math.Ceil(float64(total) * buffer))
	}
	return total
}

func serviceGraphTotal(edges []model.Edge, opts model.Options, seriesPerEdge int, buffer float64) int {
	total := 0
	for _, edge := range edges {
		sizing := effectiveServiceSizing(edge.SourceService, edge.Repository, opts)
		instanceMultiplier := generatedMetricInstanceMultiplierForInstances(opts, sizing.instances)
		total += sizing.environments * instanceMultiplier * max(1, seriesPerEdge)
	}
	if buffer != 1 {
		return int(math.Ceil(float64(total) * buffer))
	}
	return total
}

func operationWeight(op model.Operation, opts model.Options) int {
	if op.Origin == "gateway" && strings.EqualFold(op.Method, "ANY") {
		if len(opts.GatewayMethods) > 0 {
			return len(opts.GatewayMethods)
		}
		if opts.GatewayMethodCount > 0 {
			return opts.GatewayMethodCount
		}
	}
	return 1
}

type serviceSizing struct {
	environments int
	instances    int
}

func effectiveServiceSizing(service string, repository string, opts model.Options) serviceSizing {
	sizing := serviceSizing{
		environments: baseEnvironmentCount(opts),
		instances:    max(1, opts.InstancesPerService),
	}
	if override, ok := matchingServiceOverride(service, repository, opts.ServiceOverrides); ok {
		if len(override.EnvironmentNames) > 0 {
			sizing.environments = len(override.EnvironmentNames)
		} else if override.Environments > 0 {
			sizing.environments = override.Environments
		}
		if override.InstancesPerService > 0 {
			sizing.instances = override.InstancesPerService
		}
	}
	sizing.environments = max(1, sizing.environments)
	sizing.instances = max(1, sizing.instances)
	return sizing
}

func matchingServiceOverride(service string, repository string, overrides []model.ServiceSizingOverride) (model.ServiceSizingOverride, bool) {
	var generic *model.ServiceSizingOverride
	for i := range overrides {
		override := overrides[i]
		if override.Service != service {
			continue
		}
		if override.Repository != "" && override.Repository == repository {
			return override, true
		}
		if override.Repository == "" {
			generic = &overrides[i]
		}
	}
	if generic != nil {
		return *generic, true
	}
	return model.ServiceSizingOverride{}, false
}

func baseEnvironmentCount(opts model.Options) int {
	if len(opts.EnvironmentNames) > 0 {
		return len(opts.EnvironmentNames)
	}
	return max(1, opts.Environments)
}

func baseGeneratedMetricInstanceMultiplier(opts model.Options) int {
	return generatedMetricInstanceMultiplierForInstances(opts, max(1, opts.InstancesPerService))
}

func generatedMetricInstanceMultiplierForInstances(opts model.Options, instances int) int {
	if opts.InstanceLabelEnabled {
		return max(1, instances)
	}
	return 1
}

func spanMetricsExpected(breakdown []model.ProcessorEstimate) int {
	total := 0
	for _, item := range breakdown {
		switch item.Processor {
		case config.ProcessorSpanMetricsCount, config.ProcessorSpanMetricsLatency, config.ProcessorSpanMetricsSize:
			total += item.Expected
		}
	}
	return total
}

func histogramSeriesPerLabelSet(bucketSeries int, histogramType string) int {
	switch histogramType {
	case config.HistogramTypeClassic:
		return max(1, bucketSeries) + 2
	case config.HistogramTypeBoth:
		return max(1, bucketSeries) + 3
	default:
		return 1
	}
}

func lowHistogramSeriesPerLabelSet(bucketSeries int, histogramType string) int {
	switch histogramType {
	case config.HistogramTypeClassic:
		return max(1, bucketSeries-1) + 2
	case config.HistogramTypeBoth:
		return max(1, bucketSeries-1) + 3
	default:
		return 1
	}
}

func highHistogramSeriesPerLabelSet(bucketSeries int, histogramType string) int {
	switch histogramType {
	case config.HistogramTypeClassic:
		return max(1, bucketSeries+1) + 2
	case config.HistogramTypeBoth:
		return max(1, bucketSeries+1) + 3
	default:
		return 1
	}
}

func latencyHistogramFormula(histogramType string) string {
	switch histogramType {
	case config.HistogramTypeClassic:
		return "classic histogram bucket series + _sum + _count"
	case config.HistogramTypeBoth:
		return "classic histogram bucket series + _sum + _count + native histogram series"
	default:
		return "native histogram series"
	}
}

func uncertaintyModel(opts model.Options, dimensionMultiplier int, instanceMultiplier int, histogramType string, buckets int) []model.UncertaintyModel {
	statusValues := max(1, opts.StatusValues)
	return []model.UncertaintyModel{
		{
			Scope:   "span-metrics",
			Formula: "series = operations x environments x status_values(bound) x dim_cardinality(bound) x instance_label_values x histogram_factor",
			HistogramFactors: []model.HistogramFactor{
				{Metric: config.ProcessorSpanMetricsCount, Factor: "1 count counter", Value: 1},
				{Metric: config.ProcessorSpanMetricsSize, Factor: "1 size series", Value: 1},
				{Metric: config.ProcessorSpanMetricsLatency, Factor: latencyHistogramFormula(histogramType), Value: histogramSeriesPerLabelSet(buckets, histogramType)},
			},
			Bounds: []model.UncertaintyBound{
				{
					Bound:               "low",
					StatusRule:          "status_values - 1",
					StatusValues:        max(1, statusValues-1),
					DimensionRule:       "reduced",
					DimensionMultiplier: lowDimensionMultiplier(opts.CustomDimensions),
					Buffer:              "none",
					BufferMultiplier:    1,
				},
				{
					Bound:               "expected",
					StatusRule:          "status_values",
					StatusValues:        statusValues,
					DimensionRule:       "configured",
					DimensionMultiplier: dimensionMultiplier,
					Buffer:              "none",
					BufferMultiplier:    1,
				},
				{
					Bound:               "high",
					StatusRule:          "status_values + 1",
					StatusValues:        statusValues + 1,
					DimensionRule:       "inflated",
					DimensionMultiplier: highDimensionMultiplier(opts.CustomDimensions),
					Buffer:              "+5%",
					BufferMultiplier:    1.05,
				},
			},
			Notes: []string{
				"Bounds assume standard OTel status codes (ok, error, unset). Custom status dimensions are not accounted for and may exceed the high estimate.",
				"Count and size span metrics use histogram_factor 1; latency span metrics use the selected histogram type.",
				fmt.Sprintf("Instance label values are modeled as %d for generated trace metrics.", instanceMultiplier),
			},
		},
	}
}

func addOperationContrib(operationContrib map[string]model.OperationEstimate, serviceContrib map[string]int, operations []model.Operation, opts model.Options, statusValues int, dimensionMultiplier int, histogramFactor int) {
	for _, op := range operations {
		weight := operationWeight(op, opts)
		sizing := effectiveServiceSizing(op.Service, op.Repository, opts)
		perOperation := sizing.environments * max(1, statusValues) * max(1, dimensionMultiplier) * generatedMetricInstanceMultiplierForInstances(opts, sizing.instances) * max(1, histogramFactor)
		key := strings.Join([]string{op.Repository, op.Service, op.Kind, op.Protocol, op.Method, op.Route, op.Origin}, "\x00")
		existing := operationContrib[key]
		if existing.Service == "" {
			existing = model.OperationEstimate{
				Service:    op.Service,
				Repository: op.Repository,
				Protocol:   op.Protocol,
				Method:     op.Method,
				Route:      op.Route,
				Kind:       op.Kind,
				Origin:     op.Origin,
			}
		}
		existing.Expected += perOperation * weight
		operationContrib[key] = existing
		serviceContrib[serviceContributionKey(op.Service, op.Repository)] += perOperation * weight
	}
}

func serviceContributionKey(service string, repository string) string {
	return repository + "\x00" + service
}

func splitServiceContributionKey(key string) (string, string) {
	repository, service, ok := strings.Cut(key, "\x00")
	if !ok {
		return key, ""
	}
	return service, repository
}

func customDimensionMultiplier(dimensions []model.Dimension) int {
	multiplier := 1
	for _, dimension := range dimensions {
		multiplier *= max(1, dimension.Cardinality)
	}
	return max(1, multiplier)
}

func lowDimensionMultiplier(dimensions []model.Dimension) int {
	if len(dimensions) == 0 {
		return 1
	}
	multiplier := 1
	for _, dimension := range dimensions {
		multiplier *= max(1, int(math.Ceil(math.Sqrt(float64(max(1, dimension.Cardinality))))))
	}
	return multiplier
}

func highDimensionMultiplier(dimensions []model.Dimension) int {
	if len(dimensions) == 0 {
		return 1
	}
	multiplier := 1
	for _, dimension := range dimensions {
		multiplier *= max(1, int(math.Ceil(float64(max(1, dimension.Cardinality))*1.25)))
	}
	return multiplier
}

func assumptions(opts model.Options, dimensionMultiplier int) []string {
	histogramType := config.NormalizeHistogramType(opts.HistogramType)
	assumptions := []string{
		"Application calls are modeled as unique App O11y operations/endpoints, not request volume.",
		"One included operation is one unique span-name label set: service + operation/span name + span kind, before environment/status/custom dimensions are applied.",
		fmt.Sprintf("Span metric labels are multiplied by %d environment(s), %d status value(s), and custom dimension cardinality %d unless service-specific overrides apply.", baseEnvironmentCount(opts), max(1, opts.StatusValues), dimensionMultiplier),
		"Service graph edges are directed static-code estimates; each edge contributes request and failure counters plus client and server latency histograms.",
		"Host info sizing assumes one target-info style series per service instance and environment.",
	}
	if len(opts.EnvironmentNames) > 0 {
		assumptions = append(assumptions, fmt.Sprintf("Named environments are modeled as: %s.", strings.Join(opts.EnvironmentNames, ", ")))
	}
	if len(opts.ServiceOverrides) > 0 {
		assumptions = append(assumptions, fmt.Sprintf("%d service-specific environment/instance override(s) are applied before estimating processor series.", len(opts.ServiceOverrides)))
	}
	if opts.InstanceLabelEnabled {
		assumptions = append(assumptions, fmt.Sprintf("Generated trace metrics include an instance label, multiplying span metrics and service graph metrics by %d instance value(s) per service.", max(1, opts.InstancesPerService)))
	} else {
		assumptions = append(assumptions, "Generated trace metrics instance label is disabled, so span metrics and service graph metrics are not multiplied by service instances.")
	}
	switch histogramType {
	case config.HistogramTypeClassic:
		assumptions = append(assumptions, fmt.Sprintf("Histogram type classic uses %d emitted _bucket series plus _sum and _count per latency label set; include the +Inf bucket in --histogram-buckets.", max(1, opts.HistogramBuckets)))
	case config.HistogramTypeBoth:
		assumptions = append(assumptions, fmt.Sprintf("Histogram type both models migration mode: %d emitted classic _bucket series plus _sum, _count, and one native histogram series per latency label set.", max(1, opts.HistogramBuckets)))
	default:
		assumptions = append(assumptions, "Histogram type native is the default and uses one native histogram series per latency label set; --histogram-buckets is ignored unless --histogram-type=classic or both.")
	}
	if !opts.IncludeClientProducer {
		assumptions = append(assumptions, "CLIENT and PRODUCER span kinds are excluded from span-metric operation sizing by default.")
	}
	if len(opts.GatewayMethods) > 0 {
		assumptions = append(assumptions, fmt.Sprintf("Spring Cloud Gateway ANY routes are expanded by %d configured HTTP method(s): %s.", len(opts.GatewayMethods), strings.Join(opts.GatewayMethods, ", ")))
	} else if opts.GatewayMethodCount > 0 {
		assumptions = append(assumptions, fmt.Sprintf("Spring Cloud Gateway ANY routes are expanded by an assumed %d HTTP method(s).", opts.GatewayMethodCount))
	} else {
		assumptions = append(assumptions, "Spring Cloud Gateway ANY routes are counted once unless gateway method expansion is enabled.")
	}
	if len(opts.CustomDimensions) == 0 {
		assumptions = append(assumptions, "No custom span dimensions were configured for the estimate.")
	}
	return assumptions
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
