# JavaScript/TypeScript / Node.js Analyzer

The Node.js analyzer scans JavaScript, TypeScript, serverless config, Next.js route files, protobuf files, and common Node dependency patterns. It emits neutral `model.Service`, `model.Operation`, `model.Edge`, `model.ConfigFinding`, and `model.Risk` records for the shared estimator.

## What Is Analyzed

- Service names from `package.json`.
- Express routes from `app.METHOD(...)`, `router.METHOD(...)`, `app.route(...).get/post/...`, and mounted routers through `app.use('/prefix', router)`.
- Fastify routes from shorthand methods and `fastify.route({ method, url })`.
- NestJS HTTP routes from `@Controller(...)` and method decorators such as `@Get`, `@Post`, `@Put`, `@Delete`, and `@Patch`; WebSocket gateways are represented as low-confidence internal span hints.
- Koa, Hapi, and Hono best-effort route registrations.
- Next.js App Router handlers from `app/**/route.{js,ts}` and Pages API handlers from `pages/api/**`.
- Serverless Framework HTTP events and low-confidence Node serverless handler hints.
- gRPC operations from `.proto` service definitions and JS/TS `addService(...)` server registrations.
- Dependency edges from Prisma, TypeORM, Sequelize, Knex, Mongoose/MongoDB, PostgreSQL, MySQL, Redis, Elasticsearch/OpenSearch, HTTP clients, and gRPC clients.
- Messaging operations and producer edges from KafkaJS, node-rdkafka, amqplib/RabbitMQ, Bull/BullMQ/Bee-Queue, AWS SQS/SNS, Google Pub/Sub, and Azure Service Bus.
- OpenTelemetry custom spans from `startSpan`/`startActiveSpan`, service-name resource hints, and high-cardinality span attribute risks.

## What Is Missing Or Limited

- The analyzer does not run Node.js, install packages, compile TypeScript, evaluate decorators, resolve aliases, or execute framework plugins.
- Browser-only JavaScript is intentionally not counted as backend operations. React/Vue/Svelte components, normal Next.js page components, generated bundles, tests, and static assets are skipped or ignored.
- Dynamic routes, computed strings, generated route manifests, dependency-injection-only registrations, and environment-only URLs are best-effort.
- Next.js frontend page routes are not counted. Only backend-relevant App Router `route.*` files and `pages/api/**` files produce HTTP operations.
- GraphQL is represented as a single HTTP endpoint hint when static GraphQL server evidence is found; schema field cardinality is not expanded.
- Dependency detections are static hints. They do not prove calls execute at runtime.

## Extension Points

- Add Node.js detectors under `internal/analyzer/nodejs/detectors/<area>`.
- Reuse helpers in `internal/analyzer/nodejs/detectors/common` for JS/TS comment stripping, balanced call parsing, import detection, string extraction, route normalization, target normalization, and neutral model construction.
- Prefer low confidence for computed or convention-only evidence, and rely on the shared merge layer for overlapping detectors.
- Emit only neutral model records. The estimator should not need to know which Node.js framework produced an operation or edge.
