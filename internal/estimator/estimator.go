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
	environments := max(1, opts.Environments)
	buckets := max(1, opts.HistogramBuckets)
	histogramType := config.NormalizeHistogramType(opts.HistogramType)
	latencySeriesPerLabelSet := histogramSeriesPerLabelSet(buckets, histogramType)
	instanceMultiplier := generatedMetricInstanceMultiplier(opts)

	var breakdown []model.ProcessorEstimate
	var componentBreakdown []model.ComponentEstimate
	operationContrib := map[string]model.OperationEstimate{}
	serviceContrib := map[string]int{}
	totalExpected := 0
	totalLow := 0
	totalHigh := 0

	if config.ProcessorEnabled(opts.Processors, config.ProcessorSpanMetricsCount) {
		perOp := environments * statusValues * dimensionMultiplier * instanceMultiplier
		expected := weightedOperationCount(includedOps, opts) * perOp
		low := weightedOperationCount(includedOps, opts) * environments * max(1, statusValues-1) * lowDimensionMultiplier(opts.CustomDimensions) * instanceMultiplier
		high := int(math.Ceil(float64(weightedOperationCount(includedOps, opts)*environments*(statusValues+1)*highDimensionMultiplier(opts.CustomDimensions)*instanceMultiplier) * 1.05))
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
		addOperationContrib(operationContrib, serviceContrib, includedOps, perOp, opts)
	}

	if config.ProcessorEnabled(opts.Processors, config.ProcessorSpanMetricsLatency) {
		perOp := environments * statusValues * dimensionMultiplier * instanceMultiplier * latencySeriesPerLabelSet
		expected := weightedOperationCount(includedOps, opts) * perOp
		low := weightedOperationCount(includedOps, opts) * environments * max(1, statusValues-1) * lowDimensionMultiplier(opts.CustomDimensions) * instanceMultiplier * lowHistogramSeriesPerLabelSet(buckets, histogramType)
		high := int(math.Ceil(float64(weightedOperationCount(includedOps, opts)*environments*(statusValues+1)*highDimensionMultiplier(opts.CustomDimensions)*instanceMultiplier*highHistogramSeriesPerLabelSet(buckets, histogramType)) * 1.05))
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
		addOperationContrib(operationContrib, serviceContrib, includedOps, perOp, opts)
	}

	if config.ProcessorEnabled(opts.Processors, config.ProcessorSpanMetricsSize) {
		perOp := environments * statusValues * dimensionMultiplier * instanceMultiplier
		expected := weightedOperationCount(includedOps, opts) * perOp
		low := weightedOperationCount(includedOps, opts) * environments * max(1, statusValues-1) * lowDimensionMultiplier(opts.CustomDimensions) * instanceMultiplier
		high := int(math.Ceil(float64(weightedOperationCount(includedOps, opts)*environments*(statusValues+1)*highDimensionMultiplier(opts.CustomDimensions)*instanceMultiplier) * 1.05))
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
		addOperationContrib(operationContrib, serviceContrib, includedOps, perOp, opts)
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
		expected := len(analysis.Edges) * environments * instanceMultiplier * seriesPerEdge
		low := len(analysis.Edges) * environments * instanceMultiplier * (2 + 2*lowHistogramSeriesPerLabelSet(buckets, histogramType))
		high := int(math.Ceil(float64(len(analysis.Edges)*environments*instanceMultiplier*(2+2*highHistogramSeriesPerLabelSet(buckets, histogramType))) * 1.2))
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
			serviceContrib[edge.SourceService] += environments * instanceMultiplier * seriesPerEdge
		}
	}

	if config.ProcessorEnabled(opts.Processors, config.ProcessorHostInfo) {
		serviceCount := len(analysis.Services)
		expected := serviceCount * environments * max(1, opts.InstancesPerService)
		low := serviceCount * environments
		high := serviceCount * environments * max(1, opts.InstancesPerService+1)
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
			serviceContrib[service.Name] += environments * max(1, opts.InstancesPerService)
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
	for service, expected := range serviceContrib {
		serviceBreakdown = append(serviceBreakdown, model.ServiceEstimate{Service: service, Expected: expected})
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
		UncertaintyModel:      uncertaintyModel(opts, dimensionMultiplier, instanceMultiplier, histogramType, buckets),
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

func generatedMetricInstanceMultiplier(opts model.Options) int {
	if opts.InstanceLabelEnabled {
		return max(1, opts.InstancesPerService)
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

func addOperationContrib(operationContrib map[string]model.OperationEstimate, serviceContrib map[string]int, operations []model.Operation, perOperation int, opts model.Options) {
	for _, op := range operations {
		weight := operationWeight(op, opts)
		key := strings.Join([]string{op.Service, op.Kind, op.Protocol, op.Method, op.Route, op.Origin}, "\x00")
		existing := operationContrib[key]
		if existing.Service == "" {
			existing = model.OperationEstimate{
				Service:  op.Service,
				Protocol: op.Protocol,
				Method:   op.Method,
				Route:    op.Route,
				Kind:     op.Kind,
				Origin:   op.Origin,
			}
		}
		existing.Expected += perOperation * weight
		operationContrib[key] = existing
		serviceContrib[op.Service] += perOperation * weight
	}
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
		fmt.Sprintf("Span metric labels are multiplied by %d environment(s), %d status value(s), and custom dimension cardinality %d.", max(1, opts.Environments), max(1, opts.StatusValues), dimensionMultiplier),
		"Service graph edges are directed static-code estimates; each edge contributes request and failure counters plus client and server latency histograms.",
		"Host info sizing assumes one target-info style series per service instance and environment.",
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
