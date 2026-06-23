package com.example;

import io.opentelemetry.api.trace.Span;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestMethod;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/orders")
public class OrderController {
    @GetMapping
    public String list() {
        return "ok";
    }

    @GetMapping("/{orderId:[0-9]+}")
    public String find() {
        Span.current().setAttribute("user.id", "123");
        return "ok";
    }

    @PostMapping(path = "/")
    public String create() {
        return "ok";
    }

    @RequestMapping(path = {"/{orderId}/cancel", "/legacy/{orderId}/cancel"}, method = {RequestMethod.POST, RequestMethod.DELETE})
    public String cancel() {
        return "ok";
    }

    @DeleteMapping("/**")
    public String deleteMany() {
        return "ok";
    }
}

