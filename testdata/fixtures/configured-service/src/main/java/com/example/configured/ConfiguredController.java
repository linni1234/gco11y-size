package com.example.configured;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class ConfiguredController {
    @GetMapping("/configured")
    public String configured() {
        return "ok";
    }
}

