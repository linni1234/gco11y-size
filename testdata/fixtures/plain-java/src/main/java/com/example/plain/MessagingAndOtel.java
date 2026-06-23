package com.example.plain;

import io.opentelemetry.api.trace.Span;
import java.util.Arrays;
import org.apache.kafka.clients.consumer.KafkaConsumer;
import org.apache.kafka.clients.producer.ProducerRecord;

public class MessagingAndOtel {
    public void messaging(KafkaConsumer<String, String> consumer) {
        consumer.subscribe(Arrays.asList("orders.created", "orders.cancelled"));
        new ProducerRecord<String, String>("audit.events", "payload");
        channel.basicConsume("work.queue", true, null, consumerTag -> {});
        channel.basicPublish("events", "order.created", null, new byte[0]);
        session.createConsumer(session.createQueue("billing.queue"));
        Span.current().setAttribute("http.route", "/manual/{id}");
        tracer.spanBuilder("custom-work").startSpan();
    }
}
