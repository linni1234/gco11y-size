package com.example;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.client.RestTemplate;

// License: https://www.apache.org/licenses/LICENSE-2.0
@RestController
public class NoisyIntegrationTest {
    private final RestTemplate restTemplate = new RestTemplate();

    @GetMapping("/test-only")
    public String testOnly() {
        return restTemplate.getForObject("https://github.com/acme/test-only", String.class);
    }
}
