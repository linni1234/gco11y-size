package messaging

import (
	"go/ast"
	"strings"

	gocommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/golang/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

func Operations(ctx gocommon.FileContext) []model.Operation {
	receiverTypes := collectReceiverTypes(ctx.File)
	var operations []model.Operation
	gocommon.ForEachCall(ctx.File, func(call *ast.CallExpr) {
		switch gocommon.SelectorName(call) {
		case "NewReader":
			if topic, ok := topicFromConfig(call); ok {
				operations = append(operations, messageOperation(ctx, "CONSUMER", "kafka", "kafka:"+topic, "kafka-go", "high"))
			}
		case "NewWriter":
			if topic, ok := topicFromConfig(call); ok {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "kafka", "kafka:"+topic, "kafka-go", "medium"))
			}
		case "ConsumePartition":
			if topic, ok := gocommon.StringArg(call, 0); ok {
				operations = append(operations, messageOperation(ctx, "CONSUMER", "kafka", "kafka:"+topic, "sarama", "medium"))
			}
		case "Consume":
			if queue, ok := gocommon.StringArg(call, 0); ok {
				operations = append(operations, messageOperation(ctx, "CONSUMER", "rabbit", "rabbit:"+queue, "rabbitmq", "medium"))
			}
		case "Subscribe", "QueueSubscribe", "ChanSubscribe":
			if subject, ok := gocommon.StringArg(call, 0); ok && hasNATSHint(ctx.File) {
				operations = append(operations, messageOperation(ctx, "CONSUMER", "nats", "nats:"+subject, "nats", "medium"))
			}
		case "WriteMessages":
			for _, topic := range topicsFromArgs(call.Args) {
				operations = append(operations, messageOperation(ctx, "PRODUCER", "kafka", "kafka:"+topic, "kafka-go", "medium"))
			}
		case "Publish":
			operations = append(operations, publishOperations(ctx, call, receiverTypes)...)
		}
	})
	return operations
}

func Edges(ctx gocommon.FileContext) []model.Edge {
	var edges []model.Edge
	for _, op := range Operations(ctx) {
		if op.Kind != "PRODUCER" {
			continue
		}
		edges = append(edges, gocommon.Edge(ctx.ServiceName, op.Route, op.Protocol, ctx.SourcePath, op.Confidence))
	}
	return edges
}

func messageOperation(ctx gocommon.FileContext, kind string, protocol string, route string, detector string, confidence string) model.Operation {
	return gocommon.Operation(ctx.ServiceName, kind, protocol, "MESSAGE", route, "message handler", ctx.SourcePath, detector, confidence)
}

func publishOperations(ctx gocommon.FileContext, call *ast.CallExpr, receiverTypes map[string]string) []model.Operation {
	receiverType := receiverTypes[gocommon.ReceiverName(call)]
	if isRabbitReceiver(receiverType) || (receiverType == "" && hasRabbitHint(ctx.File) && !hasNATSHint(ctx.File)) {
		exchange, exchangeOK := gocommon.StringArg(call, 0)
		routingKey, routingOK := gocommon.StringArg(call, 1)
		if exchangeOK || routingOK {
			target := strings.Trim(exchange+"/"+routingKey, "/")
			if target != "" {
				return []model.Operation{messageOperation(ctx, "PRODUCER", "rabbit", "rabbit:"+target, "rabbitmq", "medium")}
			}
		}
	}
	if isNATSReceiver(receiverType) || (receiverType == "" && hasNATSHint(ctx.File) && !hasRabbitHint(ctx.File)) {
		if subject, ok := gocommon.StringArg(call, 0); ok {
			return []model.Operation{messageOperation(ctx, "PRODUCER", "nats", "nats:"+subject, "nats", "medium")}
		}
	}
	return nil
}

func topicFromConfig(call *ast.CallExpr) (string, bool) {
	for _, arg := range call.Args {
		if topic, ok := gocommon.CompositeStringValue(arg, "Topic"); ok {
			return topic, true
		}
	}
	return "", false
}

func topicsFromArgs(args []ast.Expr) []string {
	var topics []string
	for _, arg := range args {
		if topic, ok := gocommon.CompositeStringValue(arg, "Topic"); ok {
			topics = append(topics, topic)
		}
	}
	return topics
}

func collectReceiverTypes(file *ast.File) map[string]string {
	types := map[string]string{}
	if file == nil {
		return types
	}
	ast.Inspect(file, func(node ast.Node) bool {
		valueSpec, ok := node.(*ast.ValueSpec)
		if !ok || valueSpec.Type == nil {
			return true
		}
		typeName := gocommon.CallName(valueSpec.Type)
		for _, name := range valueSpec.Names {
			types[name.Name] = typeName
		}
		return true
	})
	return types
}

func isRabbitReceiver(typeName string) bool {
	return strings.Contains(typeName, "amqp.Channel") || strings.Contains(typeName, "rabbit")
}

func isNATSReceiver(typeName string) bool {
	return strings.Contains(typeName, "nats.Conn")
}

func hasRabbitHint(file *ast.File) bool {
	for _, path := range gocommon.ImportPaths(file) {
		if strings.Contains(path, "amqp") || strings.Contains(path, "rabbitmq") {
			return true
		}
	}
	return false
}

func hasNATSHint(file *ast.File) bool {
	for _, path := range gocommon.ImportPaths(file) {
		if strings.Contains(path, "nats-io/nats.go") {
			return true
		}
	}
	return false
}
