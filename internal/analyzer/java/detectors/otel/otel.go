package otel

import (
	"fmt"
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	javacommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	httpRouteAttributeRE  = regexp.MustCompile(`setAttribute\s*\(\s*"http\.route"\s*,\s*"([^"]+)"`)
	spanBuilderRE         = regexp.MustCompile(`spanBuilder\s*\(\s*"([^"]+)"`)
	withSpanRE            = regexp.MustCompile(`@WithSpan(?:\s*\(\s*"([^"]+)"\s*\))?`)
	highCardinalityAttrRE = regexp.MustCompile(`(?:setAttribute|tag)\s*\(\s*"([^"]+)"`)
)

func Operations(serviceName string, sourcePath string, source string) []model.Operation {
	var operations []model.Operation
	for _, match := range httpRouteAttributeRE.FindAllStringSubmatch(source, -1) {
		operations = append(operations, javacommon.Operation(serviceName, "SERVER", "http", "ANY", basecommon.NormalizeRoute(match[1]), "OpenTelemetry http.route attribute", sourcePath, "otel-http-route", "low"))
	}
	for _, match := range spanBuilderRE.FindAllStringSubmatch(source, -1) {
		operations = append(operations, javacommon.Operation(serviceName, "INTERNAL", "custom", "SPAN", match[1], "OpenTelemetry spanBuilder", sourcePath, "otel-span", "low"))
	}
	for _, match := range withSpanRE.FindAllStringSubmatch(source, -1) {
		name := match[1]
		if name == "" {
			name = "WithSpan"
		}
		operations = append(operations, javacommon.Operation(serviceName, "INTERNAL", "custom", "SPAN", name, "@WithSpan", sourcePath, "otel-span", "low"))
	}
	return operations
}

func Risks(sourcePath string, source string) []model.Risk {
	var risks []model.Risk
	for _, match := range highCardinalityAttrRE.FindAllStringSubmatch(source, -1) {
		attr := strings.ToLower(match[1])
		if basecommon.LooksHighCardinalityAttribute(attr) {
			risks = append(risks, model.Risk{
				Severity: "medium",
				Area:     "span attributes",
				Message:  fmt.Sprintf("span attribute %q looks high-cardinality if enabled as an App O11y dimension", match[1]),
				Source:   sourcePath,
			})
		}
	}
	return risks
}
