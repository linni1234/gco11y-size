package messaging

import (
	"regexp"
	"strings"

	javacommon "github.com/nilslindholm/metricgenerationsizer/internal/analyzer/java/detectors/common"
	"github.com/nilslindholm/metricgenerationsizer/internal/model"
)

var (
	producerRecordRE    = regexp.MustCompile(`new\s+ProducerRecord(?:<[^>]+>)?\s*\(\s*"([^"]+)"`)
	kafkaTemplateSendRE = regexp.MustCompile(`KafkaTemplate(?:<[^>]+>)?[^;]*\.send\s*\(\s*"([^"]+)"`)
	rabbitConsumeRE     = regexp.MustCompile(`basicConsume\s*\(\s*"([^"]+)"`)
	rabbitPublishRE     = regexp.MustCompile(`basicPublish\s*\(\s*"([^"]*)"\s*,\s*"([^"]*)"`)
	jmsConsumerRE       = regexp.MustCompile(`createConsumer\s*\([^;]*create(?:Queue|Topic)\s*\(\s*"([^"]+)"`)
	jmsProducerRE       = regexp.MustCompile(`createProducer\s*\([^;]*create(?:Queue|Topic)\s*\(\s*"([^"]+)"`)
	subscribeRE         = regexp.MustCompile(`subscribe\s*\(\s*(?:Collections\.singletonList|List\.of|Arrays\.asList)\s*\(([^)]*)\)`)
)

func Operations(serviceName string, sourcePath string, source string) []model.Operation {
	var operations []model.Operation
	for _, topic := range kafkaSubscribeTopics(source) {
		operations = append(operations, javacommon.Operation(serviceName, "CONSUMER", "kafka", "MESSAGE", "kafka:"+topic, "KafkaConsumer.subscribe", sourcePath, "kafka-client", "medium"))
	}
	for _, match := range producerRecordRE.FindAllStringSubmatch(source, -1) {
		operations = append(operations, javacommon.Operation(serviceName, "PRODUCER", "kafka", "MESSAGE", "kafka:"+match[1], "KafkaProducer.send", sourcePath, "kafka-client", "medium"))
	}
	for _, match := range kafkaTemplateSendRE.FindAllStringSubmatch(source, -1) {
		operations = append(operations, javacommon.Operation(serviceName, "PRODUCER", "kafka", "MESSAGE", "kafka:"+match[1], "KafkaTemplate.send", sourcePath, "spring-kafka", "medium"))
	}
	for _, match := range rabbitConsumeRE.FindAllStringSubmatch(source, -1) {
		operations = append(operations, javacommon.Operation(serviceName, "CONSUMER", "rabbit", "MESSAGE", "rabbit:"+match[1], "RabbitMQ basicConsume", sourcePath, "rabbit-client", "medium"))
	}
	for _, match := range rabbitPublishRE.FindAllStringSubmatch(source, -1) {
		route := "rabbit:" + strings.Trim(match[1]+"/"+match[2], "/")
		operations = append(operations, javacommon.Operation(serviceName, "PRODUCER", "rabbit", "MESSAGE", route, "RabbitMQ basicPublish", sourcePath, "rabbit-client", "medium"))
	}
	for _, match := range jmsConsumerRE.FindAllStringSubmatch(source, -1) {
		operations = append(operations, javacommon.Operation(serviceName, "CONSUMER", "jms", "MESSAGE", "jms:"+match[1], "JMS consumer", sourcePath, "jms", "medium"))
	}
	for _, match := range jmsProducerRE.FindAllStringSubmatch(source, -1) {
		operations = append(operations, javacommon.Operation(serviceName, "PRODUCER", "jms", "MESSAGE", "jms:"+match[1], "JMS producer", sourcePath, "jms", "medium"))
	}
	return operations
}

func kafkaSubscribeTopics(source string) []string {
	var topics []string
	for _, match := range subscribeRE.FindAllStringSubmatch(source, -1) {
		topics = append(topics, javacommon.ExtractStringLiterals(match[1])...)
	}
	return topics
}
