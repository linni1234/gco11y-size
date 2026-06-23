package java

import (
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/framework"
	javacommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func normalizeSpringResult(result framework.Result) framework.Result {
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Protocol == "" {
			op.Protocol = javacommon.InferProtocol(op.Route)
		}
		if op.Confidence == "" {
			op.Confidence = "high"
		}
		if len(op.Detectors) == 0 {
			switch {
			case op.Origin == "gateway":
				op.Detectors = []string{"spring-cloud-gateway"}
			case strings.HasPrefix(strings.ToLower(op.Route), "kafka:"):
				op.Detectors = []string{"spring-kafka"}
			case strings.HasPrefix(strings.ToLower(op.Route), "rabbit:"):
				op.Detectors = []string{"spring-rabbit"}
			default:
				op.Detectors = []string{"spring-mvc"}
			}
		}
	}
	return result
}

func serviceNameForRoot(repo string, root string, serviceNames map[string]string) (string, string) {
	if serviceName := serviceNames[root]; serviceName != "" {
		return serviceName, "config"
	}
	return basecommon.FallbackServiceName(repo, root), "path"
}

func ensureService(result *framework.Result, service model.Service) {
	if service.Name == "" {
		return
	}
	for _, existing := range result.Services {
		if existing.Name == service.Name {
			return
		}
	}
	result.Services = append(result.Services, service)
}

func copyServiceNames(in map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range in {
		out[key] = value
	}
	return out
}

func filterNoSpringOperationsWarning(warnings []string) []string {
	var out []string
	for _, warning := range warnings {
		if strings.Contains(warning, "no Spring controller or listener operations were detected") {
			continue
		}
		out = append(out, warning)
	}
	return out
}
