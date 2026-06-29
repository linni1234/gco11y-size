package analyzer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type fixtureCase struct {
	Name                      string
	AnalyzerIDs               []string
	ServiceNames              []string
	OperationCount            int
	EdgeCount                 int
	RequiredOperations        []operationExpectation
	AbsentOperations          []operationExpectation
	RequiredEdges             []edgeExpectation
	DetectorExpectations      []detectorExpectation
	RequiredConfigFindings    []configFindingExpectation
	RequiredRiskAreas         []string
	RequiredWarningSubstrings []string
	DetectedLanguageContains  []string
}

type operationExpectation struct {
	Service  string
	Kind     string
	Protocol string
	Method   string
	Route    string
}

type edgeExpectation struct {
	SourceService string
	TargetService string
	Protocol      string
}

type detectorExpectation struct {
	Operation operationExpectation
	Required  []string
	Forbidden []string
}

type configFindingExpectation struct {
	Kind  string
	Name  string
	Value string
}

var analyzerFixtureCases = []fixtureCase{
	{
		Name:           "simple-service",
		AnalyzerIDs:    []string{"java"},
		ServiceNames:   []string{"order-api"},
		OperationCount: 10,
		EdgeCount:      1,
		RequiredOperations: []operationExpectation{
			httpOperation("order-api", "SERVER", "GET", "/api/orders"),
			httpOperation("order-api", "SERVER", "GET", "/api/orders/{orderId}"),
			httpOperation("order-api", "SERVER", "POST", "/api/orders/{orderId}/cancel"),
			httpOperation("order-api", "SERVER", "DELETE", "/api/orders/legacy/{orderId}/cancel"),
			{Service: "order-api", Kind: "CONSUMER", Protocol: "kafka", Method: "MESSAGE", Route: "kafka:orders.created"},
		},
		RequiredEdges: []edgeExpectation{
			{SourceService: "order-api", TargetService: "payment-service", Protocol: "http"},
		},
		RequiredRiskAreas: []string{"span attributes"},
	},
	{
		Name:           "two-services",
		AnalyzerIDs:    []string{"java"},
		ServiceNames:   []string{"order-service", "payment-service"},
		OperationCount: 3,
		EdgeCount:      1,
		RequiredOperations: []operationExpectation{
			httpOperation("order-service", "SERVER", "GET", "/checkout/{cartId}"),
			httpOperation("payment-service", "SERVER", "GET", "/payments/{paymentId}"),
		},
		RequiredEdges: []edgeExpectation{
			{SourceService: "order-service", TargetService: "payment-service", Protocol: "http"},
		},
	},
	{
		Name:           "configured-service",
		AnalyzerIDs:    []string{"java"},
		ServiceNames:   []string{"configured-service"},
		OperationCount: 1,
		EdgeCount:      0,
		RequiredOperations: []operationExpectation{
			httpOperation("configured-service", "SERVER", "GET", "/configured"),
		},
		RequiredConfigFindings: []configFindingExpectation{
			{Kind: "dimension-hint", Name: "tenant.id"},
		},
		RequiredRiskAreas: []string{"custom dimensions"},
	},
	{
		Name:           "gateway-service",
		AnalyzerIDs:    []string{"java"},
		ServiceNames:   []string{"api-gateway"},
		OperationCount: 4,
		EdgeCount:      2,
		RequiredOperations: []operationExpectation{
			httpOperation("api-gateway", "SERVER", "ANY", "/products/**"),
			httpOperation("api-gateway", "SERVER", "ANY", "/price-lists/**"),
			httpOperation("api-gateway", "SERVER", "ANY", "/prices/**"),
			httpOperation("api-gateway", "SERVER", "ANY", "/fallback/unavailable"),
		},
		RequiredEdges: []edgeExpectation{
			{SourceService: "api-gateway", TargetService: "product-service", Protocol: "http"},
			{SourceService: "api-gateway", TargetService: "pricing-service", Protocol: "http"},
		},
	},
	{
		Name:           "quarkus-service",
		AnalyzerIDs:    []string{"java"},
		ServiceNames:   []string{"quarkus-orders"},
		OperationCount: 7,
		EdgeCount:      0,
		RequiredOperations: []operationExpectation{
			httpOperation("quarkus-orders", "SERVER", "GET", "/root/rest/orders/{id}"),
			httpOperation("quarkus-orders", "SERVER", "POST", "/root/rest/orders"),
			httpOperation("quarkus-orders", "SERVER", "GET", "/root/reactive/ping"),
			httpOperation("quarkus-orders", "SERVER", "GET", "/root/reactive/hello/{name}"),
			httpOperation("quarkus-orders", "SERVER", "POST", "/root/reactive/hello/{name}"),
			httpOperation("quarkus-orders", "SERVER", "ANY", "/root/reactive/status"),
			httpOperation("quarkus-orders", "SERVER", "DELETE", "/root/reactive/cleanup"),
		},
		AbsentOperations: []operationExpectation{
			httpOperation("quarkus-orders", "SERVER", "GET", "/root/rest/remote/{id}"),
		},
		DetectorExpectations: []detectorExpectation{
			{
				Operation: httpOperation("quarkus-orders", "SERVER", "GET", "/root/rest/orders/{id}"),
				Required:  []string{"quarkus-rest"},
				Forbidden: []string{"jax-rs"},
			},
			{
				Operation: httpOperation("quarkus-orders", "SERVER", "GET", "/root/reactive/ping"),
				Required:  []string{"quarkus-reactive-routes"},
			},
		},
		RequiredConfigFindings: []configFindingExpectation{
			{Kind: "service-name", Name: "quarkus.application.name", Value: "quarkus-orders"},
			{Kind: "quarkus-http-root-path", Value: "/root"},
			{Kind: "quarkus-rest-path", Value: "/rest"},
		},
		RequiredWarningSubstrings: []string{"regex-only reactive route"},
	},
	{
		Name:           "plain-java",
		AnalyzerIDs:    []string{"java"},
		ServiceNames:   []string{"plain-java-service"},
		OperationCount: 17,
		EdgeCount:      4,
		RequiredOperations: []operationExpectation{
			httpOperation("plain-java-service", "SERVER", "GET", "/jax/orders/{id}"),
			httpOperation("plain-java-service", "SERVER", "ANY", "/legacy/*"),
			httpOperation("plain-java-service", "SERVER", "ANY", "/xml/*"),
			httpOperation("plain-java-service", "SERVER", "GET", "/items/{id}"),
			httpOperation("plain-java-service", "SERVER", "ANY", "/health"),
			{Service: "plain-java-service", Kind: "SERVER", Protocol: "grpc", Method: "RPC", Route: "Greeter/sayHello"},
			{Service: "plain-java-service", Kind: "CONSUMER", Protocol: "kafka", Method: "MESSAGE", Route: "kafka:orders.created"},
			{Service: "plain-java-service", Kind: "PRODUCER", Protocol: "rabbit", Method: "MESSAGE", Route: "rabbit:events/order.created"},
			{Service: "plain-java-service", Kind: "INTERNAL", Protocol: "custom", Method: "SPAN", Route: "custom-work"},
		},
		RequiredEdges: []edgeExpectation{
			{SourceService: "plain-java-service", TargetService: "inventory-service", Protocol: "grpc"},
			{SourceService: "plain-java-service", TargetService: "payments.internal", Protocol: "http"},
			{SourceService: "plain-java-service", TargetService: "kafka:audit.events", Protocol: "kafka"},
			{SourceService: "plain-java-service", TargetService: "rabbit:events/order.created", Protocol: "rabbit"},
		},
		DetectorExpectations: []detectorExpectation{
			{
				Operation: operationExpectation{Service: "plain-java-service", Kind: "SERVER", Protocol: "grpc", Method: "RPC", Route: "Greeter/sayHello"},
				Required:  []string{"grpc-java", "protobuf"},
			},
		},
	},
	{
		Name:                     "go-service",
		AnalyzerIDs:              []string{"go"},
		ServiceNames:             []string{"shop-go"},
		OperationCount:           18,
		EdgeCount:                6,
		DetectedLanguageContains: []string{"go"},
		RequiredOperations: []operationExpectation{
			httpOperation("shop-go", "SERVER", "GET", "/health"),
			httpOperation("shop-go", "SERVER", "ANY", "/legacy"),
			httpOperation("shop-go", "SERVER", "GET", "/api/orders/{id}"),
			httpOperation("shop-go", "SERVER", "POST", "/api/orders"),
			httpOperation("shop-go", "SERVER", "GET", "/api/registered/helper/{id}"),
			httpOperation("shop-go", "SERVER", "PUT", "/v1/customers/{id}"),
			httpOperation("shop-go", "SERVER", "GET", "/chi/{id}"),
			httpOperation("shop-go", "SERVER", "POST", "/nested/{id}"),
			httpOperation("shop-go", "SERVER", "GET", "/gorilla/{id}"),
			httpOperation("shop-go", "SERVER", "GET", "/fiber/{id}"),
			{Service: "shop-go", Kind: "SERVER", Protocol: "grpc", Method: "RPC", Route: "Greeter/SayHello"},
			{Service: "shop-go", Kind: "CONSUMER", Protocol: "kafka", Method: "MESSAGE", Route: "kafka:orders.created"},
			{Service: "shop-go", Kind: "PRODUCER", Protocol: "kafka", Method: "MESSAGE", Route: "kafka:audit.events"},
			{Service: "shop-go", Kind: "CONSUMER", Protocol: "rabbit", Method: "MESSAGE", Route: "rabbit:orders.queue"},
			{Service: "shop-go", Kind: "PRODUCER", Protocol: "rabbit", Method: "MESSAGE", Route: "rabbit:events/order.created"},
			{Service: "shop-go", Kind: "CONSUMER", Protocol: "nats", Method: "MESSAGE", Route: "nats:inventory.updated"},
			{Service: "shop-go", Kind: "PRODUCER", Protocol: "nats", Method: "MESSAGE", Route: "nats:billing.created"},
			{Service: "shop-go", Kind: "INTERNAL", Protocol: "custom", Method: "SPAN", Route: "custom-go-work"},
		},
		RequiredEdges: []edgeExpectation{
			{SourceService: "shop-go", TargetService: "payments.internal", Protocol: "http"},
			{SourceService: "shop-go", TargetService: "inventory", Protocol: "http"},
			{SourceService: "shop-go", TargetService: "inventory-service", Protocol: "grpc"},
			{SourceService: "shop-go", TargetService: "kafka:audit.events", Protocol: "kafka"},
			{SourceService: "shop-go", TargetService: "rabbit:events/order.created", Protocol: "rabbit"},
			{SourceService: "shop-go", TargetService: "nats:billing.created", Protocol: "nats"},
		},
		DetectorExpectations: []detectorExpectation{
			{
				Operation: operationExpectation{Service: "shop-go", Kind: "SERVER", Protocol: "grpc", Method: "RPC", Route: "Greeter/SayHello"},
				Required:  []string{"protobuf", "grpc-go", "connect-go"},
			},
		},
		RequiredRiskAreas: []string{"span attributes"},
	},
	{
		Name:                     "dotnet-service",
		AnalyzerIDs:              []string{"dotnet"},
		ServiceNames:             []string{"dotnet-checkout"},
		OperationCount:           28,
		EdgeCount:                16,
		DetectedLanguageContains: []string{"csharp", "csharp/aspnet-core"},
		RequiredOperations: []operationExpectation{
			httpOperation("dotnet-checkout", "SERVER", "GET", "/api/v1/orders/{id}"),
			httpOperation("dotnet-checkout", "SERVER", "POST", "/api/v1/orders"),
			httpOperation("dotnet-checkout", "SERVER", "GET", "/api/orders/{id}"),
			httpOperation("dotnet-checkout", "SERVER", "GET", "/orders/page"),
			httpOperation("dotnet-checkout", "SERVER", "GET", "/dashboard"),
			{Service: "dotnet-checkout", Kind: "SERVER", Protocol: "grpc", Method: "RPC", Route: "checkout.Greeter/SayHello"},
			{Service: "dotnet-checkout", Kind: "CONSUMER", Protocol: "kafka", Method: "MESSAGE", Route: "kafka:orders.created"},
			{Service: "dotnet-checkout", Kind: "PRODUCER", Protocol: "rabbit", Method: "MESSAGE", Route: "rabbit:events/order.created"},
			{Service: "dotnet-checkout", Kind: "CONSUMER", Protocol: "servicebus", Method: "MESSAGE", Route: "servicebus:billing.commands"},
			{Service: "dotnet-checkout", Kind: "CONSUMER", Protocol: "masstransit", Method: "MESSAGE", Route: "masstransit:orders-masstransit"},
			{Service: "dotnet-checkout", Kind: "INTERNAL", Protocol: "custom", Method: "SPAN", Route: "reconcile-orders"},
		},
		RequiredEdges: []edgeExpectation{
			{SourceService: "dotnet-checkout", TargetService: "sqlserver:orders", Protocol: "sqlserver"},
			{SourceService: "dotnet-checkout", TargetService: "postgresql:readmodel", Protocol: "postgresql"},
			{SourceService: "dotnet-checkout", TargetService: "redis:redis.internal", Protocol: "redis"},
			{SourceService: "dotnet-checkout", TargetService: "mongodb:orders", Protocol: "mongodb"},
			{SourceService: "dotnet-checkout", TargetService: "inventory-service.internal", Protocol: "http"},
			{SourceService: "dotnet-checkout", TargetService: "inventory-grpc.internal", Protocol: "grpc"},
			{SourceService: "dotnet-checkout", TargetService: "kafka:orders.audit", Protocol: "kafka"},
			{SourceService: "dotnet-checkout", TargetService: "rabbit:events/order.created", Protocol: "rabbit"},
			{SourceService: "dotnet-checkout", TargetService: "servicebus:shipping.events", Protocol: "servicebus"},
		},
		DetectorExpectations: []detectorExpectation{
			{
				Operation: httpOperation("dotnet-checkout", "SERVER", "GET", "/api/v1/orders/{id}"),
				Required:  []string{"aspnet-core-minimal-api"},
			},
			{
				Operation: operationExpectation{Service: "dotnet-checkout", Kind: "SERVER", Protocol: "grpc", Method: "RPC", Route: "checkout.Greeter/SayHello"},
				Required:  []string{"protobuf"},
			},
		},
		RequiredConfigFindings: []configFindingExpectation{
			{Kind: "service-name", Name: "OTEL_SERVICE_NAME", Value: "dotnet-checkout"},
			{Kind: "connection-string", Name: "OrdersDb", Value: "sqlserver"},
			{Kind: "yarp-route", Name: "products", Value: "products"},
			{Kind: "dotnet-data-framework", Name: "entity-framework", Value: "detected"},
			{Kind: "dotnet-background-framework", Name: "hangfire", Value: "detected"},
			{Kind: "dotnet-aspire-resource", Name: "AddPostgres", Value: "orders-db"},
		},
		RequiredRiskAreas: []string{"span attributes"},
	},
	{
		Name:                     "nodejs-service",
		AnalyzerIDs:              []string{"nodejs"},
		ServiceNames:             []string{"node-checkout"},
		OperationCount:           29,
		EdgeCount:                14,
		DetectedLanguageContains: []string{"javascript", "typescript", "nodejs"},
		RequiredOperations: []operationExpectation{
			httpOperation("node-checkout", "SERVER", "GET", "/api/orders/{id}"),
			httpOperation("node-checkout", "SERVER", "POST", "/api/orders"),
			httpOperation("node-checkout", "SERVER", "GET", "/fast/health"),
			httpOperation("node-checkout", "SERVER", "GET", "/nest/orders/{id}"),
			httpOperation("node-checkout", "SERVER", "DELETE", "/api/orders/{id}"),
			httpOperation("node-checkout", "SERVER", "POST", "/api/reports"),
			httpOperation("node-checkout", "SERVER", "POST", "/api/checkout"),
			httpOperation("node-checkout", "SERVER", "POST", "/serverless/orders"),
			{Service: "node-checkout", Kind: "SERVER", Protocol: "grpc", Method: "RPC", Route: "checkout.Greeter/SayHello"},
			{Service: "node-checkout", Kind: "CONSUMER", Protocol: "kafka", Method: "MESSAGE", Route: "kafka:orders.created"},
			{Service: "node-checkout", Kind: "PRODUCER", Protocol: "rabbit", Method: "MESSAGE", Route: "rabbit:events/order.created"},
			{Service: "node-checkout", Kind: "PRODUCER", Protocol: "sqs", Method: "MESSAGE", Route: "sqs:orders-queue"},
			{Service: "node-checkout", Kind: "CONSUMER", Protocol: "servicebus", Method: "MESSAGE", Route: "servicebus:billing.commands"},
			{Service: "node-checkout", Kind: "INTERNAL", Protocol: "custom", Method: "SPAN", Route: "reconcile-orders"},
		},
		RequiredEdges: []edgeExpectation{
			{SourceService: "node-checkout", TargetService: "inventory-service.internal", Protocol: "http"},
			{SourceService: "node-checkout", TargetService: "inventory-grpc.internal", Protocol: "grpc"},
			{SourceService: "node-checkout", TargetService: "postgresql:orders", Protocol: "postgresql"},
			{SourceService: "node-checkout", TargetService: "mysql:shop", Protocol: "mysql"},
			{SourceService: "node-checkout", TargetService: "mongodb:orders", Protocol: "mongodb"},
			{SourceService: "node-checkout", TargetService: "redis:redis.internal", Protocol: "redis"},
			{SourceService: "node-checkout", TargetService: "kafka:orders.audit", Protocol: "kafka"},
			{SourceService: "node-checkout", TargetService: "rabbit:events/order.created", Protocol: "rabbit"},
			{SourceService: "node-checkout", TargetService: "sqs:orders-queue", Protocol: "sqs"},
			{SourceService: "node-checkout", TargetService: "servicebus:shipping.events", Protocol: "servicebus"},
		},
		DetectorExpectations: []detectorExpectation{
			{
				Operation: httpOperation("node-checkout", "SERVER", "GET", "/api/orders/{id}"),
				Required:  []string{"express"},
			},
			{
				Operation: operationExpectation{Service: "node-checkout", Kind: "SERVER", Protocol: "grpc", Method: "RPC", Route: "checkout.Greeter/SayHello"},
				Required:  []string{"protobuf", "grpc-js"},
			},
		},
		RequiredConfigFindings: []configFindingExpectation{
			{Kind: "service-name", Name: "package.name", Value: "node-checkout"},
			{Kind: "nodejs-web-framework", Name: "express", Value: "detected"},
			{Kind: "nodejs-dependency-framework", Name: "prisma", Value: "detected"},
			{Kind: "nodejs-messaging-framework", Name: "kafkajs", Value: "detected"},
			{Kind: "instrumentation", Name: "opentelemetry-js", Value: "detected"},
		},
		RequiredRiskAreas: []string{"span attributes"},
	},
	{
		Name:                     "nodejs-frontend-only",
		AnalyzerIDs:              []string{"nodejs"},
		ServiceNames:             []string{"node-frontend-only"},
		OperationCount:           0,
		EdgeCount:                0,
		DetectedLanguageContains: []string{"typescript", "nodejs"},
		AbsentOperations: []operationExpectation{
			httpOperation("node-frontend-only", "SERVER", "GET", "/dashboard"),
		},
	},
}

func TestAnalyzerFixtures(t *testing.T) {
	for _, tc := range analyzerFixtureCases {
		t.Run(tc.Name, func(t *testing.T) {
			analysis, err := Analyze(fixture(t, tc.Name), "")
			if err != nil {
				t.Fatalf("Analyze returned error: %v", err)
			}
			assertServiceNames(t, analysis, tc.ServiceNames)
			if got, want := len(analysis.Operations), tc.OperationCount; got != want {
				t.Fatalf("operations = %d, want %d: %#v", got, want, analysis.Operations)
			}
			if got, want := len(analysis.Edges), tc.EdgeCount; got != want {
				t.Fatalf("edges = %d, want %d: %#v", got, want, analysis.Edges)
			}
			for _, expected := range tc.RequiredOperations {
				_ = findOperation(t, analysis, expected)
			}
			for _, absent := range tc.AbsentOperations {
				assertNoOperation(t, analysis, absent)
			}
			for _, expected := range tc.RequiredEdges {
				assertEdge(t, analysis, expected)
			}
			for _, expected := range tc.DetectorExpectations {
				assertOperationDetectors(t, analysis, expected)
			}
			for _, expected := range tc.RequiredConfigFindings {
				assertConfigFinding(t, analysis, expected)
			}
			for _, area := range tc.RequiredRiskAreas {
				assertRiskArea(t, analysis, area)
			}
			for _, substring := range tc.RequiredWarningSubstrings {
				assertWarningContains(t, analysis, substring)
			}
			for _, language := range tc.DetectedLanguageContains {
				if !strings.Contains(analysis.DetectedLanguage, language) {
					t.Fatalf("detected language = %q, want substring %q", analysis.DetectedLanguage, language)
				}
			}
		})
	}
}

func TestAnalyzerFixtureTableCoversFixtureDirectories(t *testing.T) {
	expected := map[string]bool{}
	for _, tc := range analyzerFixtureCases {
		expected[tc.Name] = true
	}

	entries, err := os.ReadDir(fixturesRoot(t))
	if err != nil {
		t.Fatalf("could not read fixtures: %v", err)
	}
	actual := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() {
			actual[entry.Name()] = true
		}
	}

	if missing := missingKeys(actual, expected); len(missing) > 0 {
		t.Fatalf("fixtures missing test cases: %s", strings.Join(missing, ", "))
	}
	if extra := missingKeys(expected, actual); len(extra) > 0 {
		t.Fatalf("fixture test cases without directories: %s", strings.Join(extra, ", "))
	}
}

func TestAnalyzerFixtureTableCoversRegisteredAnalyzers(t *testing.T) {
	covered := map[string]bool{}
	for _, tc := range analyzerFixtureCases {
		for _, analyzerID := range tc.AnalyzerIDs {
			covered[analyzerID] = true
		}
	}

	registered := map[string]bool{}
	for _, analyzer := range registeredAnalyzers() {
		registered[analyzer.ID()] = true
		if !covered[analyzer.ID()] {
			t.Fatalf("registered analyzer %q has no fixture coverage", analyzer.ID())
		}
	}
	if extra := missingKeys(registered, covered); len(extra) > 0 {
		t.Fatalf("fixture cases reference unknown analyzer IDs: %s", strings.Join(extra, ", "))
	}
}

func httpOperation(service string, kind string, method string, route string) operationExpectation {
	return operationExpectation{
		Service:  service,
		Kind:     kind,
		Protocol: "http",
		Method:   method,
		Route:    route,
	}
}

func assertServiceNames(t *testing.T, analysis model.Analysis, expected []string) {
	t.Helper()
	var actual []string
	for _, service := range analysis.Services {
		actual = append(actual, service.Name)
	}
	sort.Strings(actual)
	expected = append([]string(nil), expected...)
	sort.Strings(expected)
	if strings.Join(actual, "\x00") != strings.Join(expected, "\x00") {
		t.Fatalf("services = %#v, want %#v", actual, expected)
	}
}

func findOperation(t *testing.T, analysis model.Analysis, expected operationExpectation) model.Operation {
	t.Helper()
	for _, op := range analysis.Operations {
		if op.Service == expected.Service &&
			op.Kind == expected.Kind &&
			op.Protocol == expected.Protocol &&
			op.Method == expected.Method &&
			op.Route == expected.Route {
			return op
		}
	}
	t.Fatalf("operation %s %s %s %s %s was not found in %#v", expected.Service, expected.Kind, expected.Protocol, expected.Method, expected.Route, analysis.Operations)
	return model.Operation{}
}

func assertNoOperation(t *testing.T, analysis model.Analysis, expected operationExpectation) {
	t.Helper()
	for _, op := range analysis.Operations {
		if op.Service == expected.Service &&
			op.Kind == expected.Kind &&
			op.Protocol == expected.Protocol &&
			op.Method == expected.Method &&
			op.Route == expected.Route {
			t.Fatalf("operation %s %s %s %s %s was found unexpectedly in %#v", expected.Service, expected.Kind, expected.Protocol, expected.Method, expected.Route, analysis.Operations)
		}
	}
}

func assertEdge(t *testing.T, analysis model.Analysis, expected edgeExpectation) {
	t.Helper()
	for _, edge := range analysis.Edges {
		if edge.SourceService == expected.SourceService &&
			edge.TargetService == expected.TargetService &&
			edge.Protocol == expected.Protocol {
			return
		}
	}
	t.Fatalf("edge %s -> %s (%s) was not found in %#v", expected.SourceService, expected.TargetService, expected.Protocol, analysis.Edges)
}

func assertOperationDetectors(t *testing.T, analysis model.Analysis, expected detectorExpectation) {
	t.Helper()
	op := findOperation(t, analysis, expected.Operation)
	for _, detector := range expected.Required {
		if !contains(op.Detectors, detector) {
			t.Fatalf("operation %s detectors = %#v, want %s", expected.Operation.Route, op.Detectors, detector)
		}
	}
	for _, detector := range expected.Forbidden {
		if contains(op.Detectors, detector) {
			t.Fatalf("operation %s detectors = %#v, did not want %s", expected.Operation.Route, op.Detectors, detector)
		}
	}
}

func assertConfigFinding(t *testing.T, analysis model.Analysis, expected configFindingExpectation) {
	t.Helper()
	for _, finding := range analysis.ConfigFindings {
		if expected.Kind != "" && finding.Kind != expected.Kind {
			continue
		}
		if expected.Name != "" && finding.Name != expected.Name {
			continue
		}
		if expected.Value != "" && finding.Value != expected.Value {
			continue
		}
		return
	}
	t.Fatalf("config finding %#v was not found in %#v", expected, analysis.ConfigFindings)
}

func assertRiskArea(t *testing.T, analysis model.Analysis, area string) {
	t.Helper()
	for _, risk := range analysis.Risks {
		if risk.Area == area {
			return
		}
	}
	t.Fatalf("risk area %q was not found in %#v", area, analysis.Risks)
}

func assertWarningContains(t *testing.T, analysis model.Analysis, substring string) {
	t.Helper()
	for _, warning := range analysis.Warnings {
		if strings.Contains(warning, substring) {
			return
		}
	}
	t.Fatalf("warning containing %q was not found in %#v", substring, analysis.Warnings)
}

func missingKeys(required map[string]bool, available map[string]bool) []string {
	var missing []string
	for key := range required {
		if !available[key] {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	return missing
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
	return filepath.Join(fixturesRoot(t), name)
}

func fixturesRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "fixtures")
}
