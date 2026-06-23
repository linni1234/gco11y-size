package outbound

import (
	"regexp"
	"strings"

	javacommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/grpc"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	httpLiteralRE       = regexp.MustCompile(`https?://[A-Za-z0-9_.:${}/-]+`)
	producerRecordRE    = regexp.MustCompile(`new\s+ProducerRecord(?:<[^>]+>)?\s*\(\s*"([^"]+)"`)
	kafkaTemplateSendRE = regexp.MustCompile(`KafkaTemplate(?:<[^>]+>)?[^;]*\.send\s*\(\s*"([^"]+)"`)
	rabbitPublishRE     = regexp.MustCompile(`basicPublish\s*\(\s*"([^"]*)"\s*,\s*"([^"]*)"`)
)

func Edges(serviceName string, sourcePath string, source string) []model.Edge {
	var edges []model.Edge
	if hasHTTPClientHint(source) {
		for _, rawURL := range httpLiteralRE.FindAllString(source, -1) {
			target := javacommon.NormalizeTargetService(rawURL)
			if javacommon.IgnoreTarget(target, serviceName) {
				continue
			}
			edges = append(edges, model.Edge{SourceService: serviceName, TargetService: target, Protocol: "http", Source: sourcePath, Confidence: "medium"})
		}
	}
	for _, target := range grpc.Targets(source) {
		if javacommon.IgnoreTarget(target, serviceName) {
			continue
		}
		edges = append(edges, model.Edge{SourceService: serviceName, TargetService: target, Protocol: "grpc", Source: sourcePath, Confidence: "medium"})
	}
	for _, match := range producerRecordRE.FindAllStringSubmatch(source, -1) {
		edges = append(edges, model.Edge{SourceService: serviceName, TargetService: "kafka:" + match[1], Protocol: "kafka", Source: sourcePath, Confidence: "medium"})
	}
	for _, match := range kafkaTemplateSendRE.FindAllStringSubmatch(source, -1) {
		edges = append(edges, model.Edge{SourceService: serviceName, TargetService: "kafka:" + match[1], Protocol: "kafka", Source: sourcePath, Confidence: "medium"})
	}
	for _, match := range rabbitPublishRE.FindAllStringSubmatch(source, -1) {
		target := "rabbit:" + strings.Trim(match[1]+"/"+match[2], "/")
		edges = append(edges, model.Edge{SourceService: serviceName, TargetService: target, Protocol: "rabbit", Source: sourcePath, Confidence: "medium"})
	}
	return edges
}

func hasHTTPClientHint(source string) bool {
	for _, hint := range []string{"HttpClient", "HttpRequest", "OkHttpClient", "Request.Builder", "CloseableHttpClient", "HttpGet", "HttpPost", "WebTarget", "Retrofit"} {
		if strings.Contains(source, hint) {
			return true
		}
	}
	return false
}
