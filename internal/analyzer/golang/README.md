# Go Analyzer

The Go analyzer scans Go modules, Go source files, protobuf files, common HTTP routers, RPC registrations, messaging clients, outbound dependencies, and OpenTelemetry hints. It uses Go AST parsing where possible and emits neutral `model.Service`, `model.Operation`, `model.Edge`, `model.ConfigFinding`, and `model.Risk` records for the shared estimator.

## What Is Analyzed

- Service names from `go.mod` module names, plus shared config hints such as `OTEL_SERVICE_NAME`, `otel.service.name`, and `service.name`.
- Standard library `net/http` handlers from `http.HandleFunc(...)`, `http.Handle(...)`, and Go 1.22 method-aware route patterns such as `GET /health`.
- Gin routes from `gin.Default`, `gin.New`, `Group(...)`, common HTTP methods, and helper registration functions that accept `*gin.RouterGroup`.
- Echo routes from `echo.New`, `Group(...)`, and common HTTP method registrations.
- Chi routes from `chi.NewRouter`, direct method registrations, `Method`, `MethodFunc`, `Mount`, and nested `Route(...)` handlers.
- Gorilla mux routes from `Handle`, `HandleFunc`, `Path`, and `PathPrefix` chained with `Methods(...)`.
- Fiber routes from `fiber.New`, `Group(...)`, `Get`, `Post`, `Put`, `Delete`, `Patch`, `Head`, `Options`, and `All`.
- gRPC operations from `.proto` service definitions and Go `Register<Service>Server(...)` registrations.
- Connect RPC handlers from `New<Service>Handler(...)`.
- Messaging consumers and producers from common Kafka, RabbitMQ, and NATS client patterns.
- Outbound service-graph edge hints from `net/http`, Resty-style calls, gRPC dial calls, and messaging producers.
- OpenTelemetry custom spans from `tracer.Start(...)`.
- OpenTelemetry attribute risks such as `user.id`, request IDs, session IDs, tokens, and other likely high-cardinality attributes.

## What Is Missing Or Limited

- Dynamic routes built from variables, loops, maps, code generation, reflection, or runtime plugin registration are only partially detected.
- Router middleware, auth groups, and route wrappers are not modeled as extra operations unless they register routes directly.
- Gin helper-function expansion currently targets common `*gin.RouterGroup` registration patterns. Equivalent helper expansion for Echo, Chi, Fiber, and Gorilla is not yet as deep.
- Frameworks beyond the current detectors may be missed, for example Goa, Kratos, Hertz, Twirp, Huma, Buffalo, Beego, httprouter, fasthttp routers, and custom internal routers.
- Routes registered through interfaces or dependency-injected router abstractions may not be resolved.
- Outbound edge detection only captures static target strings or simple literals. Targets built from config, environment variables, service discovery, or request data are not resolved.
- gRPC operation detection is strongest when `.proto` files are present. Generated code or hand-written servers without protobuf context may produce lower-confidence names.
- The analyzer does not run code, build packages, resolve full type information, or load module dependencies.
- Runtime-only span names from OpenTelemetry auto-instrumentation, interceptors, middleware, or dynamic instrumentation may not appear in source.
- Cardinality estimates cannot know runtime label values such as actual status-code spread, tenant IDs, user IDs, pod instances, or environment fan-out.

## Extension Points

- Add Go framework detectors under `internal/analyzer/golang/detectors/<framework>`.
- Reuse helpers in `internal/analyzer/golang/detectors/common` for AST call inspection, import alias handling, string literal extraction, route normalization, target normalization, and operation construction.
- Prefer AST parsing over raw regex for Go source. Regex is acceptable for non-Go formats such as `.proto` or simple config hints.
- Emit only neutral model records. The estimator should not need to know which Go framework produced an operation.
