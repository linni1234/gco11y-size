# Java Analyzer

The Java analyzer scans Java source, Spring configuration, servlet metadata, protobuf files, and common JVM client patterns. It emits neutral `model.Service`, `model.Operation`, `model.Edge`, `model.ConfigFinding`, and `model.Risk` records for the shared estimator.

## What Is Analyzed

- Spring MVC routes from `@RestController`, `@Controller`, `@RequestMapping`, `@GetMapping`, `@PostMapping`, `@PutMapping`, `@DeleteMapping`, and `@PatchMapping`.
- Spring Cloud Gateway routes from configuration, including route predicates such as path matches and downstream service URIs.
- Spring messaging consumers from `@KafkaListener` and `@RabbitListener`.
- JAX-RS and Jakarta REST routes from `@Path`, `@GET`, `@POST`, `@PUT`, `@DELETE`, `@PATCH`, `@HEAD`, and `@OPTIONS`.
- Quarkus HTTP routes from Quarkus-owned JAX-RS resources and `@Route`/`@RouteBase`, including `quarkus.http.root-path`, `quarkus.rest.path`, and `quarkus.application.name` config hints.
- Servlet routes from `@WebServlet`, `web.xml`, and dynamic servlet mapping patterns.
- JDK HTTP server contexts from `HttpServer.createContext(...)`.
- Generic Java routing hints for common route-registration styles such as Javalin/Spark-like APIs.
- gRPC operations from Java `*Grpc.*ImplBase` implementations and `.proto` service definitions when a service identity is known.
- Plain Java messaging patterns for Kafka, RabbitMQ, and JMS producers and consumers.
- Outbound service-graph edge hints from Feign-style clients, static HTTP URLs, Java HTTP clients, gRPC target builders, Kafka producers, and RabbitMQ publishers.
- OpenTelemetry hints such as `http.route`, `spanBuilder(...)`, `@WithSpan`, and likely high-cardinality span attributes.
- Service names from Spring and OpenTelemetry configuration such as `spring.application.name`, `otel.service.name`, `OTEL_SERVICE_NAME`, and `service.name`.
- Config hints for environments, metrics-generator processors, and custom dimensions.

## What Is Missing Or Limited

- Dynamic routes built from variables, reflection, runtime registration, service discovery, or external route databases are only partially detected.
- Framework-specific Java stacks beyond the current detectors may be missed, for example Micronaut, Vert.x Web, Helidon, Play Framework, Dropwizard/Jersey edge cases, and custom internal frameworks.
- Quarkus Reactive Messaging channels and Quarkus REST Client outbound edges are not resolved from configuration yet.
- Annotation aliases, meta-annotations, Kotlin source, Scala source, and generated JVM bytecode are not analyzed deeply.
- Request methods or paths hidden behind constants may be missed unless the detector can resolve them from simple string literals.
- Feign and HTTP client edge detection is static and best-effort. It cannot prove that a call executes at runtime.
- `.proto` files without a clear service identity may be reported as config findings instead of counted as operations.
- Runtime-only span names from manual instrumentation, interceptors, AOP, filters, servlet containers, or OpenTelemetry auto-instrumentation may not appear in source.
- Cardinality estimates cannot know runtime label values such as actual status-code spread, tenant IDs, user IDs, pod instances, or environment fan-out.
- The analyzer does not run code, build the project, resolve dependency graphs, or evaluate Spring profiles.

## Extension Points

- Add Java framework detectors under `internal/analyzer/java/detectors/<framework>`.
- Reuse helpers in `internal/analyzer/java/detectors/common` for annotation parsing, Java string literals, route joining, target normalization, and operation construction.
- Emit only neutral model records. The estimator should not need to know which Java framework produced an operation.
