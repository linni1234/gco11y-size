package dependencies

import (
	"net/url"
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	nodecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/nodejs/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	queueURLRE     = regexp.MustCompile(`https?://[^'"` + "`" + `\s]+/([A-Za-z0-9_.-]+)`)
	databaseNameRE = regexp.MustCompile(`\b(?:database|dbName)\s*:\s*['"` + "`" + `]([^'"` + "`" + `]+)['"` + "`" + `]`)
)

func Operations(ctx nodecommon.FileContext) []model.Operation {
	var operations []model.Operation
	operations = append(operations, kafkaOperations(ctx)...)
	operations = append(operations, rabbitOperations(ctx)...)
	operations = append(operations, cloudQueueOperations(ctx)...)
	operations = append(operations, bullOperations(ctx)...)
	return operations
}

func Edges(ctx nodecommon.FileContext) []model.Edge {
	seen := map[string]bool{}
	var edges []model.Edge
	add := func(edge model.Edge) {
		key := edge.Protocol + "\x00" + edge.TargetService
		if edge.TargetService == "" || nodecommon.IgnoreTarget(edge.TargetService, ctx.ServiceName) || seen[key] {
			return
		}
		seen[key] = true
		edges = append(edges, edge)
	}
	for _, edge := range dataEdges(ctx) {
		add(edge)
	}
	for _, edge := range httpEdges(ctx) {
		add(edge)
	}
	for _, op := range Operations(ctx) {
		if op.Kind != "PRODUCER" {
			continue
		}
		add(nodecommon.Edge(ctx.ServiceName, op.Route, op.Protocol, ctx.SourcePath, op.Confidence))
	}
	return edges
}

func ConfigFindings(ctx nodecommon.FileContext) []model.ConfigFinding {
	var findings []model.ConfigFinding
	for _, item := range []struct {
		kind string
		name string
		hint bool
	}{
		{kind: "nodejs-dependency-framework", name: "prisma", hint: strings.Contains(ctx.Source, "PrismaClient")},
		{kind: "nodejs-dependency-framework", name: "typeorm", hint: nodecommon.HasImport(ctx.Source, "typeorm")},
		{kind: "nodejs-dependency-framework", name: "sequelize", hint: nodecommon.HasImport(ctx.Source, "sequelize")},
		{kind: "nodejs-dependency-framework", name: "knex", hint: nodecommon.HasImport(ctx.Source, "knex")},
		{kind: "nodejs-dependency-framework", name: "mongoose", hint: nodecommon.HasImport(ctx.Source, "mongoose")},
		{kind: "nodejs-dependency-framework", name: "redis", hint: nodecommon.HasImport(ctx.Source, "redis", "ioredis")},
		{kind: "nodejs-messaging-framework", name: "kafkajs", hint: nodecommon.HasImport(ctx.Source, "kafkajs")},
		{kind: "nodejs-messaging-framework", name: "amqplib", hint: nodecommon.HasImport(ctx.Source, "amqplib")},
		{kind: "nodejs-messaging-framework", name: "bullmq", hint: nodecommon.HasImport(ctx.Source, "bullmq", "bull")},
		{kind: "nodejs-cloud-sdk", name: "aws-sdk", hint: nodecommon.HasImport(ctx.Source, "@aws-sdk/")},
		{kind: "nodejs-cloud-sdk", name: "google-pubsub", hint: nodecommon.HasImport(ctx.Source, "@google-cloud/pubsub")},
		{kind: "nodejs-cloud-sdk", name: "azure-service-bus", hint: nodecommon.HasImport(ctx.Source, "@azure/service-bus")},
	} {
		if item.hint {
			findings = append(findings, model.ConfigFinding{Kind: item.kind, Name: item.name, Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
		}
	}
	return findings
}

func dataEdges(ctx nodecommon.FileContext) []model.Edge {
	var edges []model.Edge
	source := ctx.Source
	add := func(provider string, target string, confidence string) {
		if target == "" {
			target = provider + ":configured"
			confidence = "low"
		}
		edges = append(edges, nodecommon.Edge(ctx.ServiceName, target, providerProtocol(provider), ctx.SourcePath, confidence))
	}
	if strings.Contains(source, "PrismaClient") {
		add("db", "db:prisma", "low")
	}
	if nodecommon.HasImport(source, "typeorm") {
		provider := firstProvider(source, "postgresql")
		add(provider, dbTarget(provider, source), "medium")
	}
	if nodecommon.HasImport(source, "sequelize") {
		provider := "mysql"
		raw := firstURLForProvider(source, "mysql")
		if raw == "" {
			provider = "postgresql"
			raw = firstURLForProvider(source, provider)
		}
		if raw != "" {
			add(provider, targetFromURL(provider, raw), "medium")
		} else {
			add(provider, dbTarget(provider, source), "medium")
		}
	}
	if nodecommon.HasImport(source, "knex") {
		provider := firstProvider(source, "db")
		add(provider, dbTarget(provider, source), "medium")
	}
	if nodecommon.HasImport(source, "mongoose", "mongodb") {
		for _, raw := range nodecommon.ExtractStringLiterals(source) {
			if strings.HasPrefix(raw, "mongodb://") || strings.HasPrefix(raw, "mongodb+srv://") {
				add("mongodb", targetFromURL("mongodb", raw), "medium")
				break
			}
		}
	}
	if nodecommon.HasImport(source, "pg") || strings.Contains(source, "Pool(") {
		if raw := firstURLForProvider(source, "postgresql"); raw != "" {
			add("postgresql", targetFromURL("postgresql", raw), "medium")
		}
	}
	if nodecommon.HasImport(source, "mysql", "mysql2") {
		if raw := firstURLForProvider(source, "mysql"); raw != "" {
			add("mysql", targetFromURL("mysql", raw), "medium")
		}
	}
	if nodecommon.HasImport(source, "redis", "ioredis") {
		for _, raw := range nodecommon.ExtractStringLiterals(source) {
			if strings.HasPrefix(raw, "redis://") || strings.Contains(raw, ".redis") || strings.Contains(raw, "redis.") {
				add("redis", targetFromURL("redis", raw), "medium")
				break
			}
		}
	}
	if nodecommon.HasImport(source, "@elastic/elasticsearch", "@opensearch-project/opensearch") {
		for _, raw := range nodecommon.HTTPURLs(source) {
			target := nodecommon.NormalizeTargetService(raw)
			if target != "" {
				add("search", "search:"+target, "medium")
				break
			}
		}
	}
	return edges
}

func httpEdges(ctx nodecommon.FileContext) []model.Edge {
	if !hasHTTPClientHint(ctx.Source) {
		return nil
	}
	var edges []model.Edge
	for _, raw := range nodecommon.HTTPURLs(ctx.Source) {
		if skipHTTPURL(raw) {
			continue
		}
		target := nodecommon.NormalizeTargetService(raw)
		if nodecommon.IgnoreTarget(target, ctx.ServiceName) {
			continue
		}
		edges = append(edges, nodecommon.Edge(ctx.ServiceName, target, "http", ctx.SourcePath, "medium"))
	}
	return edges
}

func kafkaOperations(ctx nodecommon.FileContext) []model.Operation {
	if !nodecommon.HasImport(ctx.Source, "kafkajs", "node-rdkafka") {
		return nil
	}
	var operations []model.Operation
	for _, call := range nodecommon.Calls(ctx.Source, "subscribe", "send", "produce") {
		switch call.Name {
		case "subscribe":
			if topic, ok := nodecommon.ObjectFieldString(call.Args, "topic"); ok {
				operations = append(operations, messageOperation(ctx, "CONSUMER", "kafka", "kafka:"+topic, "kafkajs", "medium"))
			}
		case "send":
			if topic, ok := nodecommon.ObjectFieldString(call.Args, "topic"); ok {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "kafka", "kafka:"+topic, "kafkajs", "medium"))
			}
		case "produce":
			if topic, ok := nodecommon.FirstString(call.Args); ok {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "kafka", "kafka:"+topic, "node-rdkafka", "medium"))
			}
		}
	}
	return operations
}

func rabbitOperations(ctx nodecommon.FileContext) []model.Operation {
	if !nodecommon.HasImport(ctx.Source, "amqplib") {
		return nil
	}
	var operations []model.Operation
	for _, call := range nodecommon.Calls(ctx.Source, "consume", "publish", "sendToQueue") {
		switch call.Name {
		case "consume":
			if queue, ok := nodecommon.FirstString(call.Args); ok {
				operations = append(operations, messageOperation(ctx, "CONSUMER", "rabbit", "rabbit:"+queue, "amqplib", "medium"))
			}
		case "publish":
			values := nodecommon.ExtractStringLiterals(call.Args)
			if len(values) >= 2 {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "rabbit", "rabbit:"+strings.Trim(values[0]+"/"+values[1], "/"), "amqplib", "medium"))
			}
		case "sendToQueue":
			if queue, ok := nodecommon.FirstString(call.Args); ok {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "rabbit", "rabbit:"+queue, "amqplib", "medium"))
			}
		}
	}
	return operations
}

func cloudQueueOperations(ctx nodecommon.FileContext) []model.Operation {
	var operations []model.Operation
	if nodecommon.HasImport(ctx.Source, "@aws-sdk/client-sqs") {
		for _, call := range nodecommon.Calls(ctx.Source, "SendMessageCommand", "ReceiveMessageCommand") {
			queue := queueName(call.Args)
			if queue == "" {
				queue = "configured"
			}
			kind := "PRODUCER"
			if call.Name == "ReceiveMessageCommand" {
				kind = "CONSUMER"
			}
			operations = append(operations, messageOperation(ctx, kind, "sqs", "sqs:"+queue, "aws-sqs", "medium"))
		}
	}
	if nodecommon.HasImport(ctx.Source, "@aws-sdk/client-sns") {
		for _, call := range nodecommon.Calls(ctx.Source, "PublishCommand") {
			topic := firstARNName(call.Args)
			if topic == "" {
				topic = "configured"
			}
			operations = append(operations, messageOperation(ctx, "PRODUCER", "sns", "sns:"+topic, "aws-sns", "medium"))
		}
	}
	if nodecommon.HasImport(ctx.Source, "@google-cloud/pubsub") {
		for _, call := range nodecommon.Calls(ctx.Source, "topic", "subscription") {
			name, ok := nodecommon.FirstString(call.Args)
			if !ok {
				continue
			}
			kind := "PRODUCER"
			protocol := "pubsub"
			if call.Name == "subscription" {
				kind = "CONSUMER"
			}
			operations = append(operations, messageOperation(ctx, kind, protocol, protocol+":"+name, "google-pubsub", "medium"))
		}
	}
	if nodecommon.HasImport(ctx.Source, "@azure/service-bus") {
		for _, call := range nodecommon.Calls(ctx.Source, "createSender", "createReceiver") {
			name, ok := nodecommon.FirstString(call.Args)
			if !ok {
				continue
			}
			kind := "PRODUCER"
			if call.Name == "createReceiver" {
				kind = "CONSUMER"
			}
			operations = append(operations, messageOperation(ctx, kind, "servicebus", "servicebus:"+name, "azure-service-bus", "medium"))
		}
	}
	return operations
}

func bullOperations(ctx nodecommon.FileContext) []model.Operation {
	if !nodecommon.HasImport(ctx.Source, "bullmq", "bull", "bee-queue") {
		return nil
	}
	var operations []model.Operation
	for _, call := range nodecommon.Calls(ctx.Source, "Queue", "Worker", "add") {
		switch call.Name {
		case "Queue":
			if name, ok := nodecommon.FirstString(call.Args); ok {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "redis-queue", "redis-queue:"+name, "bullmq", "low"))
			}
		case "Worker":
			if name, ok := nodecommon.FirstString(call.Args); ok {
				operations = append(operations, messageOperation(ctx, "CONSUMER", "redis-queue", "redis-queue:"+name, "bullmq", "medium"))
			}
		case "add":
			if name, ok := nodecommon.FirstString(call.Args); ok {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "redis-queue", "redis-queue:"+name, "bullmq", "low"))
			}
		}
	}
	return operations
}

func messageOperation(ctx nodecommon.FileContext, kind string, protocol string, route string, detector string, confidence string) model.Operation {
	return nodecommon.Operation(ctx.ServiceName, kind, protocol, "MESSAGE", route, "message handler", ctx.SourcePath, detector, confidence)
}

func firstProvider(source string, fallback string) string {
	lower := strings.ToLower(source)
	switch {
	case strings.Contains(lower, "postgres"), strings.Contains(lower, "pg"):
		return "postgresql"
	case strings.Contains(lower, "mysql"):
		return "mysql"
	case strings.Contains(lower, "mssql"), strings.Contains(lower, "sqlserver"):
		return "mssql"
	case strings.Contains(lower, "sqlite"):
		return "sqlite"
	default:
		return fallback
	}
}

func dbTarget(provider string, source string) string {
	if match := databaseNameRE.FindStringSubmatch(source); len(match) == 2 {
		return provider + ":" + basecommon.SanitizeServiceName(match[1])
	}
	return ""
}

func firstURLForProvider(source string, provider string) string {
	for _, raw := range nodecommon.ExtractStringLiterals(source) {
		lower := strings.ToLower(raw)
		switch provider {
		case "postgresql":
			if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
				return raw
			}
		case "mysql":
			if strings.HasPrefix(lower, "mysql://") {
				return raw
			}
		case "redis":
			if strings.HasPrefix(lower, "redis://") {
				return raw
			}
		}
	}
	return ""
}

func targetFromURL(provider string, raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return provider + ":" + basecommon.SanitizeServiceName(raw)
	}
	name := strings.Trim(parsed.Path, "/")
	if name == "" {
		name = parsed.Hostname()
	}
	return provider + ":" + basecommon.SanitizeServiceName(name)
}

func providerProtocol(provider string) string {
	switch provider {
	case "postgresql", "mysql", "mssql", "sqlite", "mongodb", "redis", "search":
		return provider
	default:
		return "db"
	}
}

func hasHTTPClientHint(source string) bool {
	return strings.Contains(source, "fetch(") ||
		nodecommon.HasImport(source, "axios", "got", "undici", "node:http", "node:https", "http", "https")
}

func skipHTTPURL(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, "amazonaws.com") && strings.Contains(lower, "sqs") ||
		strings.Contains(lower, "localhost") ||
		strings.Contains(lower, "127.0.0.1")
}

func queueName(args string) string {
	for _, value := range nodecommon.ExtractStringLiterals(args) {
		if match := queueURLRE.FindStringSubmatch(value); len(match) == 2 {
			return basecommon.SanitizeServiceName(match[1])
		}
	}
	if value, ok := nodecommon.ObjectFieldString(args, "QueueUrl"); ok {
		return basecommon.SanitizeServiceName(value)
	}
	return ""
}

func firstARNName(args string) string {
	for _, value := range nodecommon.ExtractStringLiterals(args) {
		if colon := strings.LastIndex(value, ":"); colon >= 0 {
			return basecommon.SanitizeServiceName(value[colon+1:])
		}
	}
	return ""
}
