package com.example;

import org.springframework.kafka.annotation.KafkaListener;
import org.springframework.stereotype.Component;

@Component
public class OrderEvents {
    @KafkaListener(topics = {"orders.created", "orders.cancelled"})
    public void handle(String payload) {
    }
}

