package analyzer

import (
	"path/filepath"
	"testing"

	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func TestAnalyzeSimpleService(t *testing.T) {
	analysis, err := Analyze(fixture(t, "simple-service"), "")
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if got, want := len(analysis.Services), 1; got != want {
		t.Fatalf("services = %d, want %d", got, want)
	}
	if analysis.Services[0].Name != "order-api" {
		t.Fatalf("service name = %q, want order-api", analysis.Services[0].Name)
	}
	if got, want := len(analysis.Operations), 10; got != want {
		t.Fatalf("operations = %d, want %d: %#v", got, want, analysis.Operations)
	}
	assertOperation(t, analysis, "order-api", "SERVER", "GET", "/api/orders")
	assertOperation(t, analysis, "order-api", "SERVER", "GET", "/api/orders/{orderId}")
	assertOperation(t, analysis, "order-api", "SERVER", "POST", "/api/orders/{orderId}/cancel")
	assertOperation(t, analysis, "order-api", "SERVER", "DELETE", "/api/orders/legacy/{orderId}/cancel")
	assertOperation(t, analysis, "order-api", "CONSUMER", "MESSAGE", "kafka:orders.created")

	if got, want := len(analysis.Edges), 1; got != want {
		t.Fatalf("edges = %d, want %d: %#v", got, want, analysis.Edges)
	}
	if analysis.Edges[0].TargetService != "payment-service" {
		t.Fatalf("edge target = %q, want payment-service", analysis.Edges[0].TargetService)
	}
	if len(analysis.Risks) == 0 {
		t.Fatalf("expected high-cardinality risk for user.id span attribute")
	}
}

func TestAnalyzeTwoServices(t *testing.T) {
	analysis, err := Analyze(fixture(t, "two-services"), "")
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if got, want := len(analysis.Services), 2; got != want {
		t.Fatalf("services = %d, want %d", got, want)
	}
	if got, want := len(analysis.Operations), 3; got != want {
		t.Fatalf("operations = %d, want %d: %#v", got, want, analysis.Operations)
	}
	assertOperation(t, analysis, "order-service", "SERVER", "GET", "/checkout/{cartId}")
	assertOperation(t, analysis, "payment-service", "SERVER", "GET", "/payments/{paymentId}")
	if got, want := len(analysis.Edges), 1; got != want {
		t.Fatalf("edges = %d, want %d: %#v", got, want, analysis.Edges)
	}
	edge := analysis.Edges[0]
	if edge.SourceService != "order-service" || edge.TargetService != "payment-service" {
		t.Fatalf("edge = %#v, want order-service -> payment-service", edge)
	}
}

func TestConfigFindingsAndDimensionRisk(t *testing.T) {
	analysis, err := Analyze(fixture(t, "configured-service"), "")
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if got, want := analysis.Services[0].Name, "configured-service"; got != want {
		t.Fatalf("service name = %q, want %q", got, want)
	}
	var foundDimension bool
	var foundRisk bool
	for _, finding := range analysis.ConfigFindings {
		if finding.Kind == "dimension-hint" && finding.Name == "tenant.id" {
			foundDimension = true
		}
	}
	for _, risk := range analysis.Risks {
		if risk.Area == "custom dimensions" {
			foundRisk = true
		}
	}
	if !foundDimension {
		t.Fatalf("expected tenant.id dimension finding")
	}
	if !foundRisk {
		t.Fatalf("expected custom dimension risk")
	}
}

func TestAnalyzeSpringCloudGatewayRoutes(t *testing.T) {
	analysis, err := Analyze(fixture(t, "gateway-service"), "")
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if got, want := len(analysis.Operations), 4; got != want {
		t.Fatalf("operations = %d, want %d: %#v", got, want, analysis.Operations)
	}
	assertOperation(t, analysis, "api-gateway", "SERVER", "ANY", "/products/**")
	assertOperation(t, analysis, "api-gateway", "SERVER", "ANY", "/price-lists/**")
	assertOperation(t, analysis, "api-gateway", "SERVER", "ANY", "/prices/**")
	assertOperation(t, analysis, "api-gateway", "SERVER", "ANY", "/fallback/unavailable")
	if got, want := len(analysis.Edges), 2; got != want {
		t.Fatalf("edges = %d, want %d: %#v", got, want, analysis.Edges)
	}
	assertEdge(t, analysis, "api-gateway", "product-service")
	assertEdge(t, analysis, "api-gateway", "pricing-service")
}

func TestAnalyzePlainJavaMixedDetectors(t *testing.T) {
	analysis, err := Analyze(fixture(t, "plain-java"), "")
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if got, want := len(analysis.Services), 1; got != want {
		t.Fatalf("services = %d, want %d", got, want)
	}
	if got, want := analysis.Services[0].Name, "plain-java-service"; got != want {
		t.Fatalf("service name = %q, want %q", got, want)
	}
	if got, want := len(analysis.Operations), 17; got != want {
		t.Fatalf("operations = %d, want %d: %#v", got, want, analysis.Operations)
	}
	assertOperationProtocol(t, analysis, "plain-java-service", "SERVER", "http", "GET", "/jax/orders/{id}")
	assertOperationProtocol(t, analysis, "plain-java-service", "SERVER", "http", "ANY", "/legacy/*")
	assertOperationProtocol(t, analysis, "plain-java-service", "SERVER", "http", "ANY", "/xml/*")
	assertOperationProtocol(t, analysis, "plain-java-service", "SERVER", "http", "GET", "/items/{id}")
	assertOperationProtocol(t, analysis, "plain-java-service", "SERVER", "http", "ANY", "/health")
	assertOperationProtocol(t, analysis, "plain-java-service", "SERVER", "grpc", "RPC", "Greeter/sayHello")
	assertOperationProtocol(t, analysis, "plain-java-service", "CONSUMER", "kafka", "MESSAGE", "kafka:orders.created")
	assertOperationProtocol(t, analysis, "plain-java-service", "PRODUCER", "rabbit", "MESSAGE", "rabbit:events/order.created")
	assertOperationProtocol(t, analysis, "plain-java-service", "INTERNAL", "custom", "SPAN", "custom-work")

	grpcOp := findOperation(t, analysis, "plain-java-service", "SERVER", "grpc", "RPC", "Greeter/sayHello")
	if !contains(grpcOp.Detectors, "grpc-java") || !contains(grpcOp.Detectors, "protobuf") {
		t.Fatalf("grpc operation detectors = %#v, want grpc-java and protobuf", grpcOp.Detectors)
	}
	if got, want := len(analysis.Edges), 4; got != want {
		t.Fatalf("edges = %d, want %d: %#v", got, want, analysis.Edges)
	}
	assertEdgeProtocol(t, analysis, "plain-java-service", "inventory-service", "grpc")
	assertEdgeProtocol(t, analysis, "plain-java-service", "payments.internal", "http")
	assertEdgeProtocol(t, analysis, "plain-java-service", "kafka:audit.events", "kafka")
	assertEdgeProtocol(t, analysis, "plain-java-service", "rabbit:events/order.created", "rabbit")
}

func assertOperation(t *testing.T, analysis model.Analysis, service string, kind string, method string, route string) {
	t.Helper()
	for _, op := range analysis.Operations {
		if op.Service == service && op.Kind == kind && op.Method == method && op.Route == route {
			return
		}
	}
	t.Fatalf("operation %s %s %s %s was not found in %#v", service, kind, method, route, analysis.Operations)
}

func assertOperationProtocol(t *testing.T, analysis model.Analysis, service string, kind string, protocol string, method string, route string) {
	t.Helper()
	_ = findOperation(t, analysis, service, kind, protocol, method, route)
}

func findOperation(t *testing.T, analysis model.Analysis, service string, kind string, protocol string, method string, route string) model.Operation {
	t.Helper()
	for _, op := range analysis.Operations {
		if op.Service == service && op.Kind == kind && op.Protocol == protocol && op.Method == method && op.Route == route {
			return op
		}
	}
	t.Fatalf("operation %s %s %s %s %s was not found in %#v", service, kind, protocol, method, route, analysis.Operations)
	return model.Operation{}
}

func assertEdge(t *testing.T, analysis model.Analysis, source string, target string) {
	t.Helper()
	for _, edge := range analysis.Edges {
		if edge.SourceService == source && edge.TargetService == target {
			return
		}
	}
	t.Fatalf("edge %s -> %s was not found in %#v", source, target, analysis.Edges)
}

func assertEdgeProtocol(t *testing.T, analysis model.Analysis, source string, target string, protocol string) {
	t.Helper()
	for _, edge := range analysis.Edges {
		if edge.SourceService == source && edge.TargetService == target && edge.Protocol == protocol {
			return
		}
	}
	t.Fatalf("edge %s -> %s (%s) was not found in %#v", source, target, protocol, analysis.Edges)
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "fixtures", name)
}
