# Grafana Cloud App O11y Series Estimator

`gco11y-size` is a local CLI that scans application source code and estimates the active metric series Grafana Cloud Application Observability can create from metrics-generator style span metrics, service graph metrics, and host/target info. It currently ships with a Java analyzer that includes Spring Boot plus common plain-Java detectors.

The tool is intentionally offline by default. Source code can forecast operations, routes, service names, outbound dependency hints, and high-cardinality risks, but exact active series still depend on runtime traces and actual label values.

## Run

```sh
go run ./cmd/gco11y-size scan --repo ./path/to/java-repo
```

`--repo` accepts a local folder or a Git remote:

```sh
go run ./cmd/gco11y-size scan --repo https://github.com/acme/checkout-service.git --ref main
go run ./cmd/gco11y-size scan --repo git@gitlab.com:acme/checkout-service.git --ref release-2026
go run ./cmd/gco11y-size scan --repo github.com/acme/checkout-service
```

This writes:

- `report.html`: standalone workspace sizing report
- `report.json`: machine-readable workspace report

Example with sizing controls:

```sh
go run ./cmd/gco11y-size scan \
  --repo ./monorepo \
  --otel-config ./alloy \
  --environments 3 \
  --histogram-type classic \
  --histogram-buckets 14 \
  --status-values 3 \
  --instance-label enabled \
  --instances-per-service 3 \
  --dimension tenant.id=25 \
  --gateway-methods GET,POST,PUT,DELETE,PATCH \
  --out app-o11y-sizing.html \
  --json app-o11y-sizing.json
```

Remote repositories are shallow-cloned into a temporary worktree and deleted after the report is written. Use `--keep-worktree` to retain the clone for debugging, and `--workdir <path>` to choose the parent directory used for temporary worktrees.

## Workspace Scans

Every scan produces a workspace report. A single repository is represented as a workspace with one repository entry; repeat `--repo` to scan multiple local folders or Git remotes in the same workspace:

```sh
go run ./cmd/gco11y-size scan \
  --repo ./services/api-gateway \
  --repo ./services/checkout \
  --repo github.com/acme/inventory-service \
  --environment prod \
  --environment staging \
  --instances-per-service 3 \
  --service-instances api-gateway=2 \
  --service-environments inventory-service=prod \
  --out workspace.html \
  --json workspace.json
```

The HTML report opens on a workspace overview and includes a left-menu entry for each repository. Expanding a repository reveals submenu links for source metadata, processors, services, top operations, and service graph details.

Workspace totals are estimated from a merged analysis rather than a naive sum of per-repo totals. The aggregate layer deduplicates operations by service, span kind, protocol, method/action, and normalized operation name, and deduplicates service graph edges by source service, target service, and protocol.

Named environments can be passed with repeated `--environment`; otherwise `--environments <count>` is used. Per-service overrides use:

```sh
--service-instances <service>=<count>
--service-instances <repo-name>:<service>=<count>
--service-environments <service>=prod,staging
--service-environments <repo-name>:<service>=prod
```

Use `repo-name:` when the same service name appears in multiple repos and you only want the override to apply to one repo tab.

You can also use a JSON input file:

```sh
go run ./cmd/gco11y-size scan --input sizing.json --out workspace.html --json workspace.json
```

Example `sizing.json`:

```json
{
  "defaults": {
    "histogram_type": "classic",
    "histogram_buckets": 14,
    "status_values": 3,
    "environments": ["prod", "staging"],
    "instances_per_service": 3,
    "processors": [
      "span-metrics-count",
      "span-metrics-latency",
      "service-graph",
      "host-info"
    ]
  },
  "repos": [
    {
      "name": "api-gateway",
      "repo": "./services/api-gateway"
    },
    {
      "name": "checkout",
      "repo": "github.com/acme/checkout-service",
      "ref": "main"
    }
  ],
  "service_overrides": [
    {
      "service": "api-gateway",
      "instances_per_service": 2,
      "environments": ["prod", "staging"]
    },
    {
      "repository": "checkout",
      "service": "checkout-service",
      "instances_per_service": 5,
      "environments": ["prod"]
    }
  ]
}
```

JSON defaults apply to every repo. Repo entries can override `ref`, `otel_config`, `workdir`, `keep_worktree`, `environments`, `environment_count`, and `instances_per_service`. Service overrides have the highest sizing specificity for matching services.

## What It Detects

The current built-in Java analyzer detects:

- Spring MVC routes from `@RestController`, `@Controller`, `@RequestMapping`, `@GetMapping`, `@PostMapping`, `@PutMapping`, `@DeleteMapping`, and `@PatchMapping`
- JAX-RS/Jakarta REST routes from `@Path`, `@GET`, `@POST`, `@PUT`, `@DELETE`, `@PATCH`, `@HEAD`, and `@OPTIONS`
- Servlet mappings from `@WebServlet`, `web.xml`, and dynamic `addMapping(...)`
- JDK HTTP server contexts from `HttpServer.createContext(...)`
- Javalin/Spark/custom-router style registrations when server/router hints are present
- gRPC server operations from Java `*Grpc.*ImplBase` implementations and `.proto` service definitions tied to a service identity
- Message consumers from `@KafkaListener` and `@RabbitListener`
- Plain Java Kafka, RabbitMQ, and JMS producer/consumer patterns
- Outbound edges from Feign clients and static `http://` / `https://` client URLs
- Outbound edges from Java HTTP clients, gRPC channels, Kafka producers, and RabbitMQ publishers
- OpenTelemetry hints such as `http.route`, `spanBuilder(...)`, and `@WithSpan`
- Service names from `spring.application.name`, `otel.service.name`, `OTEL_SERVICE_NAME`, and `service.name` resource attributes
- Config hints for environments, processors, and custom dimensions
- Cardinality risks such as likely user/session/request ID attributes or risky configured dimensions

Operations include protocol, confidence, and detector metadata in the JSON/HTML report. The merge layer deduplicates by service, span kind, protocol, method/action, and normalized operation name, so overlapping detectors such as Java gRPC implementation plus `.proto` definitions count once while HTTP, gRPC, Kafka, Rabbit, JMS, and custom spans stay separate.

## Adding Frameworks

Analyzer modules live behind the shared `internal/analyzer/framework.Analyzer` interface. The core `internal/analyzer` package only validates the repo, runs registered analyzers, merges their neutral facts, and returns the shared `model.Analysis` used by the estimator and report renderer.

The Java analyzer is intentionally split into a coordinator plus focused detector packages:

```text
internal/analyzer/java/
  analyzer.go
  detector.go
  common.go
  detectors/
    common/
    spring/
    grpc/
    jaxrs/
    servlet/
    routing/
    messaging/
    outbound/
    otel/
```

`internal/analyzer/java/analyzer.go` owns repo walking and service-root resolution. Each detector owns one source of Java evidence: Spring MVC/Gateway/listeners, gRPC Java/protobuf, JAX-RS, servlet mappings, generic Java routing, messaging, outbound dependencies, or OpenTelemetry hints. Detector output is still neutral `model.Operation`, `model.Edge`, `model.Service`, `model.ConfigFinding`, and `model.Risk` data.

To add a new framework or language:

1. For another Java framework, add a focused package under `internal/analyzer/java/detectors/<framework>` and call it from the Java coordinator.
2. For another language, create a package such as `internal/analyzer/nodeexpress` or `internal/analyzer/fastapi` and implement `framework.Analyzer`.
3. Emit neutral `model.Service`, `model.Operation`, `model.Edge`, `model.ConfigFinding`, and `model.Risk` values.
4. Reuse helpers from `internal/analyzer/common` for repo walking, service-root inference, route normalization, and cardinality-risk checks. Java detectors can also reuse `internal/analyzer/java/detectors/common` for annotation parsing, string literals, target normalization, and operation construction.
5. Register new top-level language analyzers in `registeredAnalyzers()` in `internal/analyzer/analyzer.go`. Java detector packages do not need registration there; they are wired through `internal/analyzer/java/analyzer.go`.

The estimator intentionally does not know which framework produced an operation. It only consumes neutral operations, edges, and services. The final merge layer deduplicates operations across detectors by service, span kind, protocol, method/action, and normalized operation name. For example, a gRPC operation found in both Java generated-code implementations and `.proto` definitions counts once, while HTTP, gRPC, Kafka, Rabbit, JMS, and custom spans remain distinct.

## Sizing Model

Defaults model Grafana Cloud App Observability-style behavior:

- Span metrics include `SERVER` and `CONSUMER` operations by default.
- `CLIENT` and `PRODUCER` span kinds are included only with `--include-client-producer`.
- One included span-metric operation means one unique span-name label set: `service + operation/span_name + span_kind`, before environment, status, and custom dimensions are applied.
- `--histogram-type` controls latency histogram sizing for span metrics and service graph metrics. The default is `native`.
- `native` uses one series per latency label set.
- `classic` uses emitted `_bucket` series + `_sum` + `_count`; include the `+Inf` bucket in `--histogram-buckets`.
- `both` models a migration period: classic series plus one native histogram series. Avoid using this permanently unless you intentionally need both query styles.
- `--native-histograms` is still accepted as a deprecated alias for `--histogram-type both`.
- Service graph series are estimated per directed service edge: request/failure counters plus client and server latency histograms.
- Host info is estimated per unique service, environment, and instance combination.
- `--instance-label enabled` models Grafana Cloud's generated trace metrics instance label. When enabled, span metrics and service graph metrics are multiplied by `--instances-per-service`; when disabled, generated trace metrics stay service-level. Host info still uses `--instances-per-service`.
- Repeated `--environment <name>` or JSON `"environments": ["prod", "staging"]` can be used instead of a raw environment count. The number of names drives series sizing, and the names are shown in reports.
- `--service-instances`, `--service-environments`, and JSON `service_overrides` let a specific service use a different instance or environment count than the workspace default.
- Spring Cloud Gateway `ANY` routes are counted once by default. Use `--gateway-methods` or `--gateway-method-count` when `http.method` is configured as a span-metrics dimension.
- Span-metric low/expected/high bounds are explicit in the report: low uses `status_values - 1` with reduced custom dimensions, expected uses configured values, and high uses `status_values + 1` with inflated custom dimensions and a 5% buffer.
- Bounds assume standard OTel status codes (`ok`, `error`, `unset`). Custom status dimensions are not accounted for and may exceed the high estimate.

Main formula inputs:

- operations/endpoints
- environments
- status values
- histogram type
- histogram buckets
- enabled processors
- custom dimension cardinalities
- service graph edges
- service instances
- generated metrics instance label setting
- optional Spring Cloud Gateway method expansion

## Optional Grafana Cloud Calibration

Set read-only Prometheus credentials and pass `--grafana-query` to compare against currently observed generated series:

```sh
export GRAFANA_CLOUD_PROM_URL="https://prometheus-prod-xx.grafana.net"
export GRAFANA_CLOUD_PROM_USER="<instance-id>"
export GRAFANA_CLOUD_PROM_TOKEN="<token>"

go run ./cmd/gco11y-size scan --repo ./repo --grafana-query
```

Without `--grafana-query`, detected credentials are reported but no network query is made.

## GitHub And GitLab Authentication

Remote repository access uses the local `git` executable. That means authentication works the same way it does for `git clone` on the customer machine:

- HTTPS private repos can use Git Credential Manager, including browser OAuth and MFA flows.
- SSH repos use local SSH keys and SSH config.
- Self-hosted GitLab or GitHub Enterprise work when the provided Git URL works with local `git clone`.

The estimator does not implement OAuth itself in v1, and it does not store GitHub or GitLab tokens. Report metadata redacts URL user info if a credentialed HTTPS URL is provided.

## Test

```sh
GOCACHE="$PWD/.gocache" go test ./...
```

## License

MIT. See [LICENSE](LICENSE).
