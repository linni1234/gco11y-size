package com.example.order;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.reactive.function.client.WebClient;

@RestController
@RequestMapping("/checkout")
public class CheckoutController {
    private final WebClient client = WebClient.builder()
            .baseUrl("http://payment-service")
            .build();

    @GetMapping("/{cartId}")
    public String get() {
        return client.get().uri("/payments/{cartId}").retrieve().toString();
    }

    @PostMapping
    public String start() {
        return "ok";
    }
}

