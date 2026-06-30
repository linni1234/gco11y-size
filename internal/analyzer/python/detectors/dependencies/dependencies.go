package dependencies

import (
	"net/url"
	"regexp"
	"strings"

	basecommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/common"
	pycommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/python/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	queueURLRE         = regexp.MustCompile(`https?://[^'"\s]+/([A-Za-z0-9_.-]+)$`)
	databaseSettingRE  = regexp.MustCompile(`(?is)\bDATABASES\s*=\s*\{.*?['"]ENGINE['"]\s*:\s*['"]([^'"]+)['"].*?['"]NAME['"]\s*:\s*['"]([^'"]+)['"]`)
	celeryDecoratorRE  = regexp.MustCompile(`^@(?:[A-Za-z_][A-Za-z0-9_]*\.)?(?:task|shared_task)\s*(?:\((.*)\))?$`)
	schedulerDecorRE   = regexp.MustCompile(`^@(?:[A-Za-z_][A-Za-z0-9_]*\.)?(?:scheduled_job|periodic_task|actor)\s*(?:\((.*)\))?$`)
	resourceTableRE    = regexp.MustCompile(`\.Table\s*\(\s*['"]([^'"]+)['"]`)
	collectionStringRE = regexp.MustCompile(`(?i)\b(?:collection|index|class_name)\s*=\s*['"]([^'"]+)['"]`)
)

func Operations(ctx pycommon.FileContext) []model.Operation {
	var operations []model.Operation
	operations = append(operations, celeryOperations(ctx)...)
	operations = append(operations, rqOperations(ctx)...)
	operations = append(operations, kafkaOperations(ctx)...)
	operations = append(operations, rabbitOperations(ctx)...)
	operations = append(operations, cloudQueueOperations(ctx)...)
	operations = append(operations, schedulerOperations(ctx)...)
	return operations
}

func Edges(ctx pycommon.FileContext) []model.Edge {
	seen := map[string]bool{}
	var edges []model.Edge
	add := func(edge model.Edge) {
		key := edge.Protocol + "\x00" + edge.TargetService
		if edge.TargetService == "" || pycommon.IgnoreTarget(edge.TargetService, ctx.ServiceName) || seen[key] {
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
		if op.Kind == "PRODUCER" {
			add(pycommon.Edge(ctx.ServiceName, op.Route, op.Protocol, ctx.SourcePath, op.Confidence))
		}
	}
	return edges
}

func ConfigFindings(ctx pycommon.FileContext) []model.ConfigFinding {
	var findings []model.ConfigFinding
	for _, item := range []struct {
		kind string
		name string
		hint bool
	}{
		{kind: "python-dependency-framework", name: "sqlalchemy", hint: pycommon.HasImport(ctx.Source, "sqlalchemy")},
		{kind: "python-dependency-framework", name: "django-orm", hint: pycommon.HasImport(ctx.Source, "django") && strings.Contains(ctx.Source, "DATABASES")},
		{kind: "python-dependency-framework", name: "flask-sqlalchemy", hint: pycommon.HasImport(ctx.Source, "flask_sqlalchemy")},
		{kind: "python-dependency-framework", name: "psycopg", hint: pycommon.HasImport(ctx.Source, "psycopg", "psycopg2", "asyncpg")},
		{kind: "python-dependency-framework", name: "mysql", hint: pycommon.HasImport(ctx.Source, "pymysql", "MySQLdb", "mysql")},
		{kind: "python-dependency-framework", name: "mongodb", hint: pycommon.HasImport(ctx.Source, "pymongo", "motor")},
		{kind: "python-dependency-framework", name: "redis", hint: pycommon.HasImport(ctx.Source, "redis")},
		{kind: "python-dependency-framework", name: "search", hint: pycommon.HasImport(ctx.Source, "elasticsearch", "opensearchpy")},
		{kind: "python-dependency-framework", name: "vector-db", hint: hasVectorHint(ctx.Source)},
		{kind: "python-messaging-framework", name: "celery", hint: pycommon.HasImport(ctx.Source, "celery")},
		{kind: "python-messaging-framework", name: "rq", hint: pycommon.HasImport(ctx.Source, "rq")},
		{kind: "python-messaging-framework", name: "dramatiq", hint: pycommon.HasImport(ctx.Source, "dramatiq")},
		{kind: "python-messaging-framework", name: "kafka", hint: pycommon.HasImport(ctx.Source, "kafka", "confluent_kafka", "aiokafka")},
		{kind: "python-messaging-framework", name: "rabbitmq", hint: pycommon.HasImport(ctx.Source, "pika", "aio_pika", "kombu")},
		{kind: "python-messaging-framework", name: "scheduler", hint: pycommon.HasImport(ctx.Source, "apscheduler")},
		{kind: "python-cloud-sdk", name: "boto3", hint: pycommon.HasImport(ctx.Source, "boto3", "botocore")},
		{kind: "python-cloud-sdk", name: "google-cloud", hint: pycommon.HasImport(ctx.Source, "google.cloud")},
		{kind: "python-cloud-sdk", name: "azure-sdk", hint: pycommon.HasImport(ctx.Source, "azure")},
		{kind: "python-ai-sdk", name: "openai", hint: pycommon.HasImport(ctx.Source, "openai")},
		{kind: "python-ai-sdk", name: "anthropic", hint: pycommon.HasImport(ctx.Source, "anthropic")},
		{kind: "python-ai-sdk", name: "langchain", hint: pycommon.HasImport(ctx.Source, "langchain", "llama_index")},
	} {
		if item.hint {
			findings = append(findings, model.ConfigFinding{Kind: item.kind, Name: item.name, Value: "detected", Source: ctx.SourcePath, Service: ctx.ServiceName})
		}
	}
	return findings
}

func dataEdges(ctx pycommon.FileContext) []model.Edge {
	var edges []model.Edge
	add := func(provider string, target string, confidence string) {
		if target == "" {
			target = provider + ":configured"
			confidence = "low"
		}
		edges = append(edges, pycommon.Edge(ctx.ServiceName, target, providerProtocol(provider), ctx.SourcePath, confidence))
	}
	for _, raw := range pycommon.ExtractStringLiterals(ctx.Source) {
		lower := strings.ToLower(raw)
		switch {
		case strings.HasPrefix(lower, "postgres://"), strings.HasPrefix(lower, "postgresql://"):
			add("postgresql", targetFromURL("postgresql", raw), "medium")
		case strings.HasPrefix(lower, "mysql://"):
			add("mysql", targetFromURL("mysql", raw), "medium")
		case strings.HasPrefix(lower, "sqlite:///"), strings.HasPrefix(lower, "sqlite://"):
			add("sqlite", targetFromURL("sqlite", raw), "medium")
		case strings.HasPrefix(lower, "mongodb://"), strings.HasPrefix(lower, "mongodb+srv://"):
			add("mongodb", targetFromURL("mongodb", raw), "medium")
		case strings.HasPrefix(lower, "redis://"), strings.HasPrefix(lower, "rediss://"):
			add("redis", targetFromURL("redis", raw), "medium")
		case strings.HasPrefix(lower, "memcached://"):
			add("memcached", targetFromURL("memcached", raw), "medium")
		}
	}
	if match := databaseSettingRE.FindStringSubmatch(ctx.Source); len(match) == 3 {
		provider := providerFromDjangoEngine(match[1])
		add(provider, provider+":"+basecommon.SanitizeServiceName(match[2]), "medium")
	}
	if pycommon.HasImport(ctx.Source, "boto3") {
		for _, match := range resourceTableRE.FindAllStringSubmatch(ctx.Source, -1) {
			add("dynamodb", "dynamodb:"+basecommon.SanitizeServiceName(match[1]), "medium")
		}
		if strings.Contains(ctx.Source, "client(\"dynamodb\")") || strings.Contains(ctx.Source, "client('dynamodb')") {
			add("dynamodb", "dynamodb:configured", "low")
		}
	}
	if pycommon.HasImport(ctx.Source, "elasticsearch", "opensearchpy") {
		for _, raw := range pycommon.HTTPURLs(ctx.Source) {
			if target := pycommon.NormalizeTargetService(raw); target != "" {
				add("search", "search:"+target, "medium")
				break
			}
		}
	}
	for provider, imports := range map[string][]string{
		"qdrant":   {"qdrant_client"},
		"weaviate": {"weaviate"},
		"pinecone": {"pinecone"},
		"chromadb": {"chromadb"},
		"milvus":   {"pymilvus"},
		"pgvector": {"pgvector"},
	} {
		if !pycommon.HasImport(ctx.Source, imports...) {
			continue
		}
		target := provider + ":configured"
		confidence := "low"
		for _, raw := range pycommon.HTTPURLs(ctx.Source) {
			if normalized := pycommon.NormalizeTargetService(raw); normalized != "" {
				target = provider + ":" + normalized
				confidence = "medium"
				break
			}
		}
		if match := collectionStringRE.FindStringSubmatch(ctx.Source); len(match) == 2 {
			target = provider + ":" + basecommon.SanitizeServiceName(match[1])
			confidence = "medium"
		}
		add(provider, target, confidence)
	}
	return edges
}

func httpEdges(ctx pycommon.FileContext) []model.Edge {
	if !hasHTTPClientHint(ctx.Source) {
		return nil
	}
	var edges []model.Edge
	for _, raw := range pycommon.HTTPURLs(ctx.Source) {
		if skipHTTPURL(raw) {
			continue
		}
		target := pycommon.NormalizeTargetService(raw)
		if pycommon.IgnoreTarget(target, ctx.ServiceName) {
			continue
		}
		edges = append(edges, pycommon.Edge(ctx.ServiceName, target, "http", ctx.SourcePath, "medium"))
	}
	return edges
}

func celeryOperations(ctx pycommon.FileContext) []model.Operation {
	if !pycommon.HasImport(ctx.Source, "celery") {
		return nil
	}
	var operations []model.Operation
	for _, block := range pycommon.DecoratedBlocks(ctx.Source) {
		for _, decorator := range block.Decorators {
			match := celeryDecoratorRE.FindStringSubmatch(decorator)
			if len(match) == 0 {
				continue
			}
			args := ""
			if len(match) > 1 {
				args = match[1]
			}
			queue := firstKeywordOrString(args, "queue")
			if queue == "" {
				queue = firstKeywordOrString(args, "name")
			}
			if queue == "" {
				queue = block.Name
			}
			operations = append(operations, messageOperation(ctx, "CONSUMER", "celery", "celery:"+basecommon.SanitizeServiceName(queue), block.Name, "celery", "medium"))
		}
	}
	for _, call := range pycommon.Calls(ctx.Source, "send_task", "apply_async", "delay") {
		queue := firstKeywordOrString(call.Args, "queue")
		if queue == "" && call.Name == "send_task" {
			queue, _ = pycommon.FirstString(call.Args)
		}
		if queue == "" {
			continue
		}
		operations = append(operations, messageOperation(ctx, "PRODUCER", "celery", "celery:"+basecommon.SanitizeServiceName(queue), call.Name, "celery", "low"))
	}
	return operations
}

func rqOperations(ctx pycommon.FileContext) []model.Operation {
	if !pycommon.HasImport(ctx.Source, "rq", "arq", "huey", "dramatiq", "taskiq") {
		return nil
	}
	var operations []model.Operation
	for _, call := range pycommon.Calls(ctx.Source, "Queue", "Worker", "enqueue", "actor", "task") {
		switch call.Name {
		case "Queue":
			if queue, ok := pycommon.FirstString(call.Args); ok {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "redis-queue", "redis-queue:"+basecommon.SanitizeServiceName(queue), "queue", "rq", "low"))
			}
		case "Worker":
			for _, queue := range pycommon.ExtractStringLiterals(call.Args) {
				operations = append(operations, messageOperation(ctx, "CONSUMER", "redis-queue", "redis-queue:"+basecommon.SanitizeServiceName(queue), "worker", "rq", "medium"))
			}
		case "enqueue":
			if queue, ok := pycommon.KeywordString(call.Args, "queue"); ok {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "redis-queue", "redis-queue:"+basecommon.SanitizeServiceName(queue), "enqueue", "rq", "low"))
			}
		}
	}
	return operations
}

func kafkaOperations(ctx pycommon.FileContext) []model.Operation {
	if !pycommon.HasImport(ctx.Source, "kafka", "confluent_kafka", "aiokafka") {
		return nil
	}
	var operations []model.Operation
	for _, call := range pycommon.Calls(ctx.Source, "KafkaConsumer", "subscribe", "send", "produce", "Producer", "Consumer") {
		switch call.Name {
		case "KafkaConsumer":
			for _, topic := range pycommon.ExtractStringLiterals(call.Args) {
				if strings.Contains(topic, ":") {
					continue
				}
				operations = append(operations, messageOperation(ctx, "CONSUMER", "kafka", "kafka:"+topic, "message handler", "kafka-python", "medium"))
				break
			}
		case "subscribe":
			for _, topic := range pycommon.ExtractStringLiterals(call.Args) {
				operations = append(operations, messageOperation(ctx, "CONSUMER", "kafka", "kafka:"+topic, "message handler", "kafka", "medium"))
				break
			}
		case "send", "produce":
			if topic, ok := pycommon.FirstString(call.Args); ok {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "kafka", "kafka:"+topic, "message send", "kafka", "medium"))
			}
		}
	}
	return operations
}

func rabbitOperations(ctx pycommon.FileContext) []model.Operation {
	if !pycommon.HasImport(ctx.Source, "pika", "aio_pika", "kombu") {
		return nil
	}
	var operations []model.Operation
	for _, call := range pycommon.Calls(ctx.Source, "basic_consume", "basic_publish", "publish", "send") {
		switch call.Name {
		case "basic_consume":
			queue := firstKeywordOrString(call.Args, "queue")
			if queue != "" {
				operations = append(operations, messageOperation(ctx, "CONSUMER", "rabbit", "rabbit:"+queue, "message handler", "rabbitmq", "medium"))
			}
		case "basic_publish", "publish", "send":
			exchange := firstKeywordOrString(call.Args, "exchange")
			routingKey := firstKeywordOrString(call.Args, "routing_key")
			target := strings.Trim(exchange+"/"+routingKey, "/")
			if target == "" {
				target, _ = pycommon.FirstString(call.Args)
			}
			if target != "" {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "rabbit", "rabbit:"+target, "message send", "rabbitmq", "medium"))
			}
		}
	}
	return operations
}

func cloudQueueOperations(ctx pycommon.FileContext) []model.Operation {
	var operations []model.Operation
	if pycommon.HasImport(ctx.Source, "boto3", "botocore") {
		for _, call := range pycommon.Calls(ctx.Source, "send_message", "receive_message", "publish", "put_events", "put_record") {
			switch call.Name {
			case "send_message", "receive_message":
				queue := queueName(call.Args)
				if queue == "" {
					queue = "configured"
				}
				kind := "PRODUCER"
				if call.Name == "receive_message" {
					kind = "CONSUMER"
				}
				operations = append(operations, messageOperation(ctx, kind, "sqs", "sqs:"+queue, "message handler", "aws-sqs", "medium"))
			case "publish":
				topic := arnName(call.Args)
				if topic == "" {
					topic = "configured"
				}
				operations = append(operations, messageOperation(ctx, "PRODUCER", "sns", "sns:"+topic, "message send", "aws-sns", "medium"))
			case "put_events":
				operations = append(operations, messageOperation(ctx, "PRODUCER", "eventbridge", "eventbridge:configured", "message send", "aws-eventbridge", "low"))
			case "put_record":
				stream := firstKeywordOrString(call.Args, "StreamName")
				if stream == "" {
					stream = "configured"
				}
				operations = append(operations, messageOperation(ctx, "PRODUCER", "kinesis", "kinesis:"+stream, "message send", "aws-kinesis", "medium"))
			}
		}
	}
	if pycommon.HasImport(ctx.Source, "google.cloud") {
		for _, call := range pycommon.Calls(ctx.Source, "topic_path", "subscription_path", "publish", "subscribe") {
			values := pycommon.ExtractStringLiterals(call.Args)
			if len(values) == 0 {
				continue
			}
			name := basecommon.SanitizeServiceName(values[len(values)-1])
			switch call.Name {
			case "topic_path", "publish":
				operations = append(operations, messageOperation(ctx, "PRODUCER", "pubsub", "pubsub:"+name, "message send", "google-pubsub", "medium"))
			case "subscription_path", "subscribe":
				operations = append(operations, messageOperation(ctx, "CONSUMER", "pubsub", "pubsub:"+name, "message handler", "google-pubsub", "medium"))
			}
		}
	}
	if pycommon.HasImport(ctx.Source, "azure.servicebus") {
		for _, call := range pycommon.Calls(ctx.Source, "get_queue_sender", "get_queue_receiver", "get_topic_sender", "get_subscription_receiver") {
			name := firstKeywordOrString(call.Args, "queue_name")
			if name == "" {
				name = firstKeywordOrString(call.Args, "topic_name")
			}
			if name == "" {
				name, _ = pycommon.FirstString(call.Args)
			}
			if name == "" {
				continue
			}
			kind := "PRODUCER"
			if strings.Contains(call.Name, "receiver") {
				kind = "CONSUMER"
			}
			operations = append(operations, messageOperation(ctx, kind, "servicebus", "servicebus:"+name, "message handler", "azure-service-bus", "medium"))
		}
	}
	return operations
}

func schedulerOperations(ctx pycommon.FileContext) []model.Operation {
	if !pycommon.HasImport(ctx.Source, "apscheduler", "django_q", "huey", "dramatiq", "taskiq") {
		return nil
	}
	var operations []model.Operation
	for _, block := range pycommon.DecoratedBlocks(ctx.Source) {
		for _, decorator := range block.Decorators {
			if schedulerDecorRE.MatchString(decorator) {
				operations = append(operations, pycommon.Operation(ctx.ServiceName, "INTERNAL", "custom", "SPAN", "job:"+block.Name, block.Name, ctx.SourcePath, "python-background-job", "low"))
				break
			}
		}
	}
	return operations
}

func messageOperation(ctx pycommon.FileContext, kind string, protocol string, route string, handler string, detector string, confidence string) model.Operation {
	return pycommon.Operation(ctx.ServiceName, kind, protocol, "MESSAGE", route, handler, ctx.SourcePath, detector, confidence)
}

func firstKeywordOrString(args string, keyword string) string {
	if value, ok := pycommon.KeywordString(args, keyword); ok {
		return value
	}
	if value, ok := pycommon.FirstString(args); ok {
		return value
	}
	return ""
}

func queueName(args string) string {
	if value, ok := pycommon.KeywordString(args, "QueueUrl"); ok {
		if match := queueURLRE.FindStringSubmatch(value); len(match) == 2 {
			return basecommon.SanitizeServiceName(match[1])
		}
		return basecommon.SanitizeServiceName(value)
	}
	for _, value := range pycommon.ExtractStringLiterals(args) {
		if match := queueURLRE.FindStringSubmatch(value); len(match) == 2 {
			return basecommon.SanitizeServiceName(match[1])
		}
	}
	return ""
}

func arnName(args string) string {
	for _, value := range pycommon.ExtractStringLiterals(args) {
		if colon := strings.LastIndex(value, ":"); colon >= 0 {
			return basecommon.SanitizeServiceName(value[colon+1:])
		}
	}
	return ""
}

func targetFromURL(provider string, raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return provider + ":" + basecommon.SanitizeServiceName(raw)
	}
	name := strings.Trim(parsed.Path, "/")
	if provider == "redis" || provider == "memcached" || name == "" {
		name = parsed.Hostname()
	}
	if provider == "sqlite" {
		name = strings.Trim(strings.TrimPrefix(raw, "sqlite:///"), "/")
	}
	if name == "" {
		name = "configured"
	}
	return provider + ":" + basecommon.SanitizeServiceName(name)
}

func providerProtocol(provider string) string {
	switch provider {
	case "postgresql", "mysql", "sqlite", "mongodb", "redis", "memcached", "dynamodb", "search", "qdrant", "weaviate", "pinecone", "chromadb", "milvus", "pgvector":
		return provider
	default:
		return "db"
	}
}

func providerFromDjangoEngine(engine string) string {
	lower := strings.ToLower(engine)
	switch {
	case strings.Contains(lower, "postgres"):
		return "postgresql"
	case strings.Contains(lower, "mysql"):
		return "mysql"
	case strings.Contains(lower, "sqlite"):
		return "sqlite"
	case strings.Contains(lower, "oracle"):
		return "oracle"
	default:
		return "db"
	}
}

func hasHTTPClientHint(source string) bool {
	return pycommon.HasImport(source, "requests", "httpx", "aiohttp", "urllib3", "tornado.httpclient", "gql") ||
		strings.Contains(source, "requests.") ||
		strings.Contains(source, "httpx.") ||
		strings.Contains(source, "ClientSession(")
}

func skipHTTPURL(raw string) bool {
	lower := strings.ToLower(raw)
	return strings.Contains(lower, "localhost") ||
		strings.Contains(lower, "127.0.0.1") ||
		strings.Contains(lower, "amazonaws.com") && strings.Contains(lower, "sqs")
}

func hasVectorHint(source string) bool {
	return pycommon.HasImport(source, "qdrant_client", "weaviate", "pinecone", "chromadb", "pymilvus", "pgvector")
}
