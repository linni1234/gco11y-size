package dependencies

import (
	"encoding/json"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	dotnetcommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/dotnet/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

type ConfigResult struct {
	Operations     []model.Operation
	Edges          []model.Edge
	ConfigFindings []model.ConfigFinding
}

var (
	connectionCallProviders = map[string]string{
		"UseSqlServer":          "sqlserver",
		"UseNpgsql":             "postgresql",
		"UseMySql":              "mysql",
		"UseSqlite":             "sqlite",
		"UseCosmos":             "cosmos",
		"SqlConnection":         "sqlserver",
		"NpgsqlConnection":      "postgresql",
		"MySqlConnection":       "mysql",
		"SqliteConnection":      "sqlite",
		"MongoClient":           "mongodb",
		"CosmosClient":          "cosmos",
		"CreateClient":          "cosmos",
		"UseInMemoryDatabase":   "inmemory",
		"ConnectionMultiplexer": "redis",
		"Connect":               "redis",
	}
	connectionStringRefRE = regexp.MustCompile(`GetConnectionString\s*\(\s*"([^"]+)"\s*\)`)
)

func Operations(ctx dotnetcommon.FileContext) []model.Operation {
	return messagingOperations(ctx)
}

func Edges(ctx dotnetcommon.FileContext) []model.Edge {
	seen := map[string]bool{}
	var edges []model.Edge
	add := func(edge model.Edge) {
		key := edge.Protocol + "\x00" + edge.TargetService
		if edge.TargetService == "" || dotnetcommon.IgnoreTarget(edge.TargetService, ctx.ServiceName) || seen[key] {
			return
		}
		seen[key] = true
		edges = append(edges, edge)
	}
	for _, edge := range dataEdges(ctx) {
		add(edge)
	}
	for _, edge := range outboundHTTPEdges(ctx) {
		add(edge)
	}
	for _, op := range messagingOperations(ctx) {
		if op.Kind != "PRODUCER" {
			continue
		}
		add(dotnetcommon.Edge(ctx.ServiceName, op.Route, op.Protocol, ctx.SourcePath, op.Confidence))
	}
	for _, edge := range aspireResourceEdges(ctx) {
		add(edge)
	}
	return edges
}

func ConfigFindings(ctx dotnetcommon.FileContext) []model.ConfigFinding {
	var findings []model.ConfigFinding
	if strings.Contains(ctx.Source, "DbContext") || strings.Contains(ctx.Source, "DbSet<") || strings.Contains(ctx.Source, "Microsoft.EntityFrameworkCore") {
		findings = append(findings, model.ConfigFinding{Kind: "dotnet-data-framework", Name: "entity-framework", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	if strings.Contains(ctx.Source, "Dapper") {
		findings = append(findings, model.ConfigFinding{Kind: "dotnet-data-framework", Name: "dapper", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	if strings.Contains(ctx.Source, "MassTransit") {
		findings = append(findings, model.ConfigFinding{Kind: "dotnet-messaging-framework", Name: "masstransit", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	if strings.Contains(ctx.Source, "NServiceBus") {
		findings = append(findings, model.ConfigFinding{Kind: "dotnet-messaging-framework", Name: "nservicebus", Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	for _, call := range dotnetcommon.Calls(ctx.Source, "AddPostgres", "AddRedis", "AddRabbitMQ", "AddKafka", "AddProject") {
		name, _ := dotnetcommon.FirstString(call.Args)
		if name == "" {
			name = strings.ToLower(call.Name)
		}
		findings = append(findings, model.ConfigFinding{Kind: "dotnet-aspire-resource", Name: call.Name, Value: name, Source: ctx.SourcePath, Service: ctx.ServiceName})
	}
	return findings
}

func ConfigFromJSON(serviceName string, sourcePath string, content string) ConfigResult {
	var root any
	if err := json.Unmarshal([]byte(content), &root); err != nil {
		return ConfigResult{}
	}
	object, ok := root.(map[string]any)
	if !ok {
		return ConfigResult{}
	}
	var result ConfigResult
	for name, value := range stringMap(object["ConnectionStrings"]) {
		provider := providerFromConnectionNameOrValue(name, value)
		if provider == "" {
			provider = "db"
		}
		target := targetFromConnectionString(provider, value)
		if target == "" {
			target = provider + ":" + basecommon.SanitizeServiceName(name)
		}
		result.ConfigFindings = append(result.ConfigFindings, model.ConfigFinding{Kind: "connection-string", Name: name, Value: provider, Source: sourcePath, Service: serviceName})
		result.Edges = append(result.Edges, dotnetcommon.Edge(serviceName, target, providerProtocol(provider), sourcePath, "medium"))
	}
	operations, edges, findings := yarpFromConfig(serviceName, sourcePath, object)
	result.Operations = append(result.Operations, operations...)
	result.Edges = append(result.Edges, edges...)
	result.ConfigFindings = append(result.ConfigFindings, findings...)
	return result
}

func dataEdges(ctx dotnetcommon.FileContext) []model.Edge {
	var edges []model.Edge
	for _, call := range dotnetcommon.Calls(ctx.Source,
		"UseSqlServer", "UseNpgsql", "UseMySql", "UseSqlite", "UseCosmos", "UseInMemoryDatabase",
		"SqlConnection", "NpgsqlConnection", "MySqlConnection", "SqliteConnection", "MongoClient", "CosmosClient", "CreateClient", "Connect",
	) {
		provider := providerForCall(call)
		if provider == "" || provider == "inmemory" {
			continue
		}
		confidence := "medium"
		raw, ok := dotnetcommon.FirstString(call.Args)
		if !ok {
			if ref := connectionStringRefRE.FindStringSubmatch(call.Args); len(ref) == 2 {
				raw = ref[1]
				confidence = "low"
			}
		}
		target := targetFromConnectionString(provider, raw)
		if target == "" && raw != "" {
			target = provider + ":" + basecommon.SanitizeServiceName(raw)
		}
		if target == "" {
			target = provider + ":configured"
			confidence = "low"
		}
		edges = append(edges, dotnetcommon.Edge(ctx.ServiceName, target, providerProtocol(provider), ctx.SourcePath, confidence))
	}
	return edges
}

func outboundHTTPEdges(ctx dotnetcommon.FileContext) []model.Edge {
	if !hasHTTPClientHint(ctx.Source) {
		return nil
	}
	var edges []model.Edge
	for _, raw := range dotnetcommon.HTTPURLs(ctx.Source) {
		if skipHTTPURL(ctx.Source, raw) {
			continue
		}
		target := dotnetcommon.NormalizeTargetService(raw)
		if dotnetcommon.IgnoreTarget(target, ctx.ServiceName) {
			continue
		}
		edges = append(edges, dotnetcommon.Edge(ctx.ServiceName, target, "http", ctx.SourcePath, "medium"))
	}
	return edges
}

func skipHTTPURL(source string, raw string) bool {
	idx := strings.Index(source, raw)
	if idx < 0 {
		return false
	}
	start := idx - 120
	if start < 0 {
		start = 0
	}
	end := idx + len(raw) + 40
	if end > len(source) {
		end = len(source)
	}
	context := source[start:end]
	return strings.Contains(context, "AccountEndpoint") ||
		strings.Contains(context, "CosmosClient") ||
		strings.Contains(context, "GrpcChannel") ||
		strings.Contains(context, "ForAddress")
}

func messagingOperations(ctx dotnetcommon.FileContext) []model.Operation {
	var operations []model.Operation
	hasKafka := strings.Contains(ctx.Source, "Confluent.Kafka") || strings.Contains(ctx.Source, "IKafka") || strings.Contains(ctx.Source, "Kafka")
	hasRabbit := strings.Contains(ctx.Source, "RabbitMQ") || strings.Contains(ctx.Source, "IModel") || strings.Contains(ctx.Source, "BasicPublish")
	hasServiceBus := strings.Contains(ctx.Source, "Azure.Messaging.ServiceBus") || strings.Contains(ctx.Source, "ServiceBusClient")
	hasMassTransit := strings.Contains(ctx.Source, "MassTransit")
	hasNServiceBus := strings.Contains(ctx.Source, "NServiceBus")
	for _, call := range dotnetcommon.Calls(ctx.Source, "Subscribe", "Produce", "ProduceAsync", "BasicConsume", "BasicPublish", "CreateProcessor", "CreateSender", "ReceiveEndpoint", "EndpointName") {
		switch call.Name {
		case "Subscribe":
			if hasKafka {
				for _, topic := range dotnetcommon.ExtractStringLiterals(call.Args) {
					operations = append(operations, messageOperation(ctx, "CONSUMER", "kafka", "kafka:"+topic, "kafka-dotnet", "medium"))
				}
			} else if hasNServiceBus {
				operations = append(operations, messageOperation(ctx, "CONSUMER", "nservicebus", "nservicebus:subscription", "nservicebus", "low"))
			}
		case "Produce", "ProduceAsync":
			if hasKafka {
				if topic, ok := dotnetcommon.FirstString(call.Args); ok {
					operations = append(operations, messageOperation(ctx, "PRODUCER", "kafka", "kafka:"+topic, "kafka-dotnet", "medium"))
				}
			}
		case "BasicConsume":
			if hasRabbit {
				queue := firstNamedOrPositional(call.Args, "queue", 0)
				if queue != "" {
					operations = append(operations, messageOperation(ctx, "CONSUMER", "rabbit", "rabbit:"+queue, "rabbitmq-dotnet", "medium"))
				}
			}
		case "BasicPublish":
			if hasRabbit {
				exchange := firstNamedOrPositional(call.Args, "exchange", 0)
				routingKey := firstNamedOrPositional(call.Args, "routingKey", 1)
				target := strings.Trim(exchange+"/"+routingKey, "/")
				if target != "" {
					operations = append(operations, messageOperation(ctx, "PRODUCER", "rabbit", "rabbit:"+target, "rabbitmq-dotnet", "medium"))
				}
			}
		case "CreateProcessor":
			if hasServiceBus {
				if entity, ok := dotnetcommon.FirstString(call.Args); ok {
					operations = append(operations, messageOperation(ctx, "CONSUMER", "servicebus", "servicebus:"+entity, "azure-service-bus", "medium"))
				}
			}
		case "CreateSender":
			if hasServiceBus {
				if entity, ok := dotnetcommon.FirstString(call.Args); ok {
					operations = append(operations, messageOperation(ctx, "PRODUCER", "servicebus", "servicebus:"+entity, "azure-service-bus", "medium"))
				}
			}
		case "ReceiveEndpoint":
			if hasMassTransit {
				if queue, ok := dotnetcommon.FirstString(call.Args); ok {
					operations = append(operations, messageOperation(ctx, "CONSUMER", "masstransit", "masstransit:"+queue, "masstransit", "medium"))
				}
			}
		case "EndpointName":
			if hasNServiceBus {
				if endpoint, ok := dotnetcommon.FirstString(call.Args); ok {
					operations = append(operations, messageOperation(ctx, "CONSUMER", "nservicebus", "nservicebus:"+endpoint, "nservicebus", "low"))
				}
			}
		}
	}
	return operations
}

func aspireResourceEdges(ctx dotnetcommon.FileContext) []model.Edge {
	var edges []model.Edge
	for _, call := range dotnetcommon.Calls(ctx.Source, "AddPostgres", "AddRedis", "AddRabbitMQ", "AddKafka") {
		name, ok := dotnetcommon.FirstString(call.Args)
		if !ok {
			continue
		}
		protocol := "custom"
		target := ""
		switch call.Name {
		case "AddPostgres":
			protocol = "postgresql"
			target = "postgresql:" + basecommon.SanitizeServiceName(name)
		case "AddRedis":
			protocol = "redis"
			target = "redis:" + basecommon.SanitizeServiceName(name)
		case "AddRabbitMQ":
			protocol = "rabbit"
			target = "rabbit:" + basecommon.SanitizeServiceName(name)
		case "AddKafka":
			protocol = "kafka"
			target = "kafka:" + basecommon.SanitizeServiceName(name)
		}
		edges = append(edges, dotnetcommon.Edge(ctx.ServiceName, target, protocol, ctx.SourcePath, "low"))
	}
	return edges
}

func yarpFromConfig(serviceName string, sourcePath string, object map[string]any) ([]model.Operation, []model.Edge, []model.ConfigFinding) {
	reverseProxy, ok := object["ReverseProxy"].(map[string]any)
	if !ok {
		return nil, nil, nil
	}
	clusters := map[string]string{}
	for clusterID, clusterValue := range nestedMap(reverseProxy, "Clusters") {
		cluster, ok := clusterValue.(map[string]any)
		if !ok {
			continue
		}
		for _, destinationValue := range nestedMap(cluster, "Destinations") {
			destination, ok := destinationValue.(map[string]any)
			if !ok {
				continue
			}
			address, _ := destination["Address"].(string)
			target := dotnetcommon.NormalizeTargetService(address)
			if target != "" {
				clusters[clusterID] = target
				break
			}
		}
	}
	var operations []model.Operation
	var edges []model.Edge
	var findings []model.ConfigFinding
	for routeID, routeValue := range nestedMap(reverseProxy, "Routes") {
		route, ok := routeValue.(map[string]any)
		if !ok {
			continue
		}
		path := ""
		if match, ok := route["Match"].(map[string]any); ok {
			path, _ = match["Path"].(string)
		}
		if path == "" {
			path = "/{**catch-all}"
		}
		clusterID, _ := route["ClusterId"].(string)
		op := dotnetcommon.Operation(serviceName, "SERVER", "http", "ANY", dotnetcommon.NormalizeRoute(path), "reverse proxy route", sourcePath, "yarp", "medium")
		op.Origin = "gateway"
		operations = append(operations, op)
		findings = append(findings, model.ConfigFinding{Kind: "yarp-route", Name: routeID, Value: clusterID, Source: sourcePath, Service: serviceName})
		if target := clusters[clusterID]; target != "" {
			edges = append(edges, dotnetcommon.Edge(serviceName, target, "http", sourcePath, "medium"))
		}
	}
	return operations, edges, findings
}

func messageOperation(ctx dotnetcommon.FileContext, kind string, protocol string, route string, detector string, confidence string) model.Operation {
	return dotnetcommon.Operation(ctx.ServiceName, kind, protocol, "MESSAGE", route, "message handler", ctx.SourcePath, detector, confidence)
}

func providerForCall(call dotnetcommon.Call) string {
	if call.Name == "Connect" && !strings.Contains(call.Receiver, "ConnectionMultiplexer") {
		return ""
	}
	if call.Name == "CreateClient" && !strings.Contains(call.Args, "Cosmos") && call.Receiver != "cosmosClient" {
		return ""
	}
	return connectionCallProviders[call.Name]
}

func targetFromConnectionString(provider string, raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, `"'`))
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "Name=") {
		return provider + ":" + basecommon.SanitizeServiceName(strings.TrimPrefix(raw, "Name="))
	}
	if provider == "mongodb" && strings.HasPrefix(raw, "mongodb") {
		parsed, err := url.Parse(raw)
		if err == nil && parsed.Hostname() != "" {
			db := strings.Trim(parsed.Path, "/")
			if db != "" {
				return "mongodb:" + basecommon.SanitizeServiceName(db)
			}
			return "mongodb:" + basecommon.SanitizeServiceName(parsed.Hostname())
		}
	}
	if provider == "redis" {
		host := strings.Split(raw, ",")[0]
		host = strings.Split(host, ":")[0]
		return "redis:" + basecommon.SanitizeServiceName(host)
	}
	fields := parseConnectionString(raw)
	db := firstNonEmpty(fields["database"], fields["initial catalog"], fields["db"])
	host := firstNonEmpty(fields["server"], fields["host"], fields["data source"], fields["address"], fields["accountendpoint"])
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		if parsed, err := url.Parse(host); err == nil && parsed.Hostname() != "" {
			host = parsed.Hostname()
		}
	}
	if db != "" {
		return provider + ":" + basecommon.SanitizeServiceName(db)
	}
	if host != "" {
		return provider + ":" + basecommon.SanitizeServiceName(host)
	}
	if !strings.Contains(raw, ";") && !strings.Contains(raw, "=") {
		return provider + ":" + basecommon.SanitizeServiceName(raw)
	}
	return ""
}

func parseConnectionString(raw string) map[string]string {
	fields := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		fields[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return fields
}

func providerFromConnectionNameOrValue(name string, value string) string {
	lower := strings.ToLower(name + " " + value)
	switch {
	case strings.Contains(lower, "npgsql"), strings.Contains(lower, "postgres"):
		return "postgresql"
	case strings.Contains(lower, "mysql"), strings.Contains(lower, "mariadb"):
		return "mysql"
	case strings.Contains(lower, "sqlite"):
		return "sqlite"
	case strings.Contains(lower, "cosmos"):
		return "cosmos"
	case strings.Contains(lower, "mongo"):
		return "mongodb"
	case strings.Contains(lower, "redis"):
		return "redis"
	case strings.Contains(lower, "server="), strings.Contains(lower, "initial catalog"):
		return "sqlserver"
	default:
		return ""
	}
}

func providerProtocol(provider string) string {
	switch provider {
	case "sqlserver", "postgresql", "mysql", "sqlite", "cosmos", "mongodb", "redis":
		return provider
	default:
		return "db"
	}
}

func hasHTTPClientHint(source string) bool {
	return strings.Contains(source, "HttpClient") ||
		strings.Contains(source, "IHttpClientFactory") ||
		strings.Contains(source, "RestClient") ||
		strings.Contains(source, "RestSharp") ||
		strings.Contains(source, "Refit") ||
		strings.Contains(source, "BaseAddress")
}

func firstNamedOrPositional(args string, name string, index int) string {
	if value, ok := dotnetcommon.NamedString(args, name); ok {
		return value
	}
	values := dotnetcommon.ExtractStringLiterals(args)
	if index >= 0 && len(values) > index {
		return values[index]
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringMap(value any) map[string]string {
	out := map[string]string{}
	object, ok := value.(map[string]any)
	if !ok {
		return out
	}
	for key, raw := range object {
		if str, ok := raw.(string); ok {
			out[key] = str
		}
	}
	return out
}

func nestedMap(object map[string]any, name string) map[string]any {
	value, ok := object[name].(map[string]any)
	if ok {
		return value
	}
	for key, value := range object {
		if strings.EqualFold(key, name) {
			if typed, ok := value.(map[string]any); ok {
				return typed
			}
		}
	}
	return map[string]any{}
}

func IsDotNetConfig(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.HasPrefix(name, "appsettings") || name == "launchsettings.json"
}
