# gco11y-size

`gco11y-size` is a local CLI for estimating how many active metric series Grafana Cloud Application Observability may generate from application traces.

It scans source code, detects application operations and service-to-service dependency hints, applies a Grafana Cloud App O11y sizing model, and writes a standalone HTML workspace report plus machine-readable JSON.

The tool is designed for pre-deployment and customer sizing conversations. It runs offline by default and does not need access to Grafana Cloud unless you explicitly enable optional calibration.

## Why This Exists

Grafana Cloud Application Observability generates metrics from traces through metrics-generator style processors:

- Span metrics for RED-style request count and latency.
- Service graph metrics for client/server call edges.
- Host or target info for service, environment, and instance metadata.

Active series are driven by label cardinality, not request volume alone. Source code cannot produce an exact billable series count, but it can estimate the important static drivers: services, operations, route names, span kinds, environments, instances, histogram type, custom dimensions, and service graph edges.

## Features

- CLI-first and local by default.
- Scans local folders, GitHub/GitLab HTTPS URLs, SSH URLs, and shorthand Git inputs.
- Supports single-repo and multi-repo workspace reports with the same output format.
- Produces standalone `report.html` and `report.json` files.
- Supports Java, Go, .NET/C#, and JavaScript/TypeScript analyzers through a shared analyzer interface.
- Estimates span metrics, service graph metrics, and host or target info separately.
- Supports native, classic, and dual histogram sizing.
- Supports environment, instance, processor, custom dimension, and gateway method-expansion controls.
- Supports per-service sizing overrides for instances and environments.
- Uses local `git` for private repository authentication. The tool does not store OAuth tokens.
- Can optionally query Grafana Cloud Prometheus with read-only credentials for calibration.

## Install

Clone and run from source:

```sh
git clone https://github.com/linni1234/gco11y-size.git
cd gco11y-size
go run ./cmd/gco11y-size scan --repo ./path/to/repo
```

Build a local binary:

```sh
go build -o gco11y-size ./cmd/gco11y-size
./gco11y-size scan --repo ./path/to/repo
```

Install the local checkout into your `GOBIN`:

```sh
go install ./cmd/gco11y-size
gco11y-size scan --repo ./path/to/repo
```

## Quick Start

Scan a local repository:

```sh
go run ./cmd/gco11y-size scan --repo ./path/to/repo
```

Scan a remote repository:

```sh
go run ./cmd/gco11y-size scan --repo https://github.com/acme/checkout-service.git --ref main
go run ./cmd/gco11y-size scan --repo git@gitlab.com:acme/checkout-service.git --ref release-2026
go run ./cmd/gco11y-size scan --repo github.com/acme/checkout-service
```

By default, every scan writes a workspace report:

- `report.html`: standalone HTML report.
- `report.json`: machine-readable workspace report.

A single `--repo` is represented as a workspace with one repository entry. Repeat `--repo` or use `--input` to scan multiple repositories in one report.

## Example

Run the included example workspace:

```sh
go run ./cmd/gco11y-size scan \
  --input example/example.json \
  --out example/workspace.html \
  --json example/workspace.json
```

Use explicit sizing controls:

```sh
go run ./cmd/gco11y-size scan \
  --repo ./monorepo \
  --otel-config ./alloy \
  --environment prod \
  --environment staging \
  --histogram-type classic \
  --histogram-buckets 14 \
  --status-values 3 \
  --instance-label enabled \
  --instances-per-service 3 \
  --service-instances api-gateway=2 \
  --service-environments inventory-service=prod \
  --dimension tenant.id=25 \
  --gateway-methods GET,POST,PUT,DELETE,PATCH \
  --out app-o11y-sizing.html \
  --json app-o11y-sizing.json
```

## Workspace Input

For repeatable customer sizing, use a JSON input file:

```sh
go run ./cmd/gco11y-size scan --input sizing.json --out workspace.html --json workspace.json
```

Example:

```json
{
  "defaults": {
    "histogram_type": "native",
    "histogram_buckets": 14,
    "status_values": 3,
    "environments": ["prod", "staging"],
    "instances_per_service": 3,
    "instance_label_enabled": true,
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

## Supported Inputs

| Input type | Example | Notes |
| --- | --- | --- |
| Local folder | `--repo ./services/checkout` | Scanned directly. |
| GitHub/GitLab HTTPS | `--repo https://github.com/acme/app.git` | Uses local `git clone`. |
| SSH Git URL | `--repo git@gitlab.com:acme/app.git` | Uses local SSH keys and config. |
| Shorthand Git URL | `--repo github.com/acme/app` | Resolved to a cloneable Git URL. |
| Workspace JSON | `--input sizing.json` | Best for repeatable multi-repo sizing. |

Remote repositories are shallow-cloned into a temporary worktree and removed after the report is written. Use `--keep-worktree` to retain the clone for debugging, and `--workdir <path>` to choose the parent directory used for temporary worktrees.

## Supported Analyzers

| Language | Coverage summary | Details |
| --- | --- | --- |
| Java | Spring MVC, Spring Cloud Gateway, Quarkus HTTP/config, JAX-RS, servlets, gRPC/protobuf, messaging, outbound dependencies, and OpenTelemetry hints. | [Java analyzer README](internal/analyzer/java/README.md) |
| Go | `net/http`, Gin, Echo, Chi, Gorilla mux, Fiber, gRPC/protobuf, Connect, messaging, outbound dependencies, and OpenTelemetry hints. | [Go analyzer README](internal/analyzer/golang/README.md) |
| .NET/C# | ASP.NET Core Minimal APIs/controllers/Razor/Blazor route hints, SignalR, YARP, gRPC/protobuf, EF/data clients, messaging/background frameworks, Aspire/Orleans hints, and OpenTelemetry hints. | [.NET analyzer README](internal/analyzer/dotnet/README.md) |
| JavaScript/TypeScript | Node.js backends using Express, Fastify, NestJS, Koa/Hapi/Hono best-effort, Next.js API/route handlers, serverless HTTP handlers, gRPC/protobuf, data clients, messaging, and OpenTelemetry hints. | [Node.js analyzer README](internal/analyzer/nodejs/README.md) |

Operations include protocol, confidence, detector metadata, source file, and handler hints where available. The shared merge layer deduplicates operations by:

```text
service + span_kind + protocol + method/action + normalized operation
```

That lets overlapping detectors count once while HTTP, gRPC, Kafka, RabbitMQ, JMS, NATS, and custom spans stay separate.

## Sizing Model

Defaults model Grafana Cloud Application Observability-style generated metrics:

- Span metrics include `SERVER` and `CONSUMER` operations by default.
- `CLIENT` and `PRODUCER` span kinds are included only with `--include-client-producer`.
- Span metric label sets are based on service, operation or span name, span kind, status code, environment, instance label, and configured custom dimensions.
- Service graph series are estimated per directed service edge.
- Host info is estimated per unique service, environment, and instance combination.
- Native histograms are the default because they use one series per latency label set.
- Classic histograms use emitted `_bucket` series plus `_sum` and `_count`.
- `both` models a migration period where classic and native histograms are emitted together.

Main formula inputs:

- operations and endpoints
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

### Histogram Types

| Type | Series factor for latency histograms | Use case |
| --- | ---: | --- |
| `native` | `1` | Default, lower active series footprint. |
| `classic` | `histogram_buckets + 2` | Legacy dashboards and classic `histogram_quantile()` queries. |
| `both` | `histogram_buckets + 3` | Temporary migration mode only. |

`--native-histograms` is still accepted as a deprecated alias for `--histogram-type=both`.

### Uncertainty Bounds

The report exposes low, expected, and high bounds for span metrics:

| Bound | Status values | Dimension multiplier | Buffer |
| --- | --- | --- | --- |
| Low | `status_values - 1` | reduced | none |
| Expected | `status_values` | configured | none |
| High | `status_values + 1` | inflated | 5% |

Bounds assume standard OpenTelemetry status-code style cardinality. Custom status dimensions or high-cardinality custom labels can exceed the high estimate.

## CLI Reference

Common scan flags:

| Flag | Description |
| --- | --- |
| `--repo <path-or-url>` | Repository path or Git URL. May be repeated. |
| `--input <file>` | JSON workspace input file. |
| `--ref <branch\|tag\|sha>` | Git ref for remote repositories. |
| `--workdir <path>` | Parent directory for temporary remote clones. |
| `--keep-worktree` | Keep remote clone after scanning. |
| `--otel-config <file-or-dir>` | Additional Alloy, OTel Collector, Spring, or Kubernetes config path. |
| `--out <file>` | HTML report path. Use an empty value to skip. |
| `--json <file>` | JSON report path. Use an empty value to skip. |
| `--processors <list>` | Comma-separated processors to include. |
| `--histogram-type native\|classic\|both` | Histogram implementation for span metrics and service graph metrics. |
| `--histogram-buckets <n>` | Classic histogram bucket series per label set. Include `+Inf`. |
| `--status-values <n>` | Expected status-code label values per operation. |
| `--environments <n>` | Number of environments when names are not provided. |
| `--environment <name>` | Named environment. May be repeated. |
| `--instances-per-service <n>` | Default service instances per environment. |
| `--instance-label enabled\|disabled` | Whether generated trace metrics include instance label cardinality. |
| `--service-instances <service=count>` | Per-service instance override. |
| `--service-environments <service=envs>` | Per-service environment override. |
| `--dimension <name=cardinality>` | Custom span dimension cardinality. May be repeated. |
| `--gateway-methods <methods>` | Expand Spring Cloud Gateway `ANY` routes when `http.method` is a dimension. |
| `--include-client-producer` | Include `CLIENT` and `PRODUCER` span kinds in span metrics. |
| `--grafana-query` | Query Grafana Cloud Prometheus when credentials are configured. |

Per-service overrides can be scoped to a repository tab:

```sh
--service-instances api-gateway=2
--service-instances "checkout:checkout-service=5"
--service-environments inventory-service=prod,staging
--service-environments "checkout:checkout-service=prod"
```

Use `repo-name:service-name` when the same service name appears in multiple repositories.

## Report Output

The HTML report includes:

- workspace overview
- per-repository drilldown
- source metadata
- processor breakdown
- detected services
- top operation contributors
- service graph edge estimates
- high-cardinality risks
- assumptions and uncertainty model

The JSON report is intended for CI, diffing, scripted review, and future UI reuse.

## Optional Grafana Cloud Calibration

Set read-only Prometheus credentials and pass `--grafana-query` to compare against currently observed generated series:

```sh
export GRAFANA_CLOUD_PROM_URL="https://prometheus-prod-xx.grafana.net"
export GRAFANA_CLOUD_PROM_USER="<instance-id>"
export GRAFANA_CLOUD_PROM_TOKEN="<token>"

go run ./cmd/gco11y-size scan --repo ./repo --grafana-query
```

Without `--grafana-query`, detected credentials are reported but no network query is made.

## Git Authentication

Remote repository access uses the local `git` executable:

- HTTPS private repositories can use Git Credential Manager, including browser OAuth and MFA flows.
- SSH repositories use local SSH keys and SSH config.
- Self-hosted GitLab and GitHub Enterprise work when the provided Git URL works with local `git clone`.

The estimator does not implement OAuth itself in v1 and does not store GitHub or GitLab tokens. Report metadata redacts URL user info if a credentialed HTTPS URL is provided.

## Project Layout

```text
cmd/gco11y-size/          CLI entrypoint
internal/analyzer/        Analyzer orchestration and merge layer
internal/analyzer/java/   Java analyzer and detectors
internal/analyzer/golang/ Go analyzer and detectors
internal/analyzer/dotnet/ .NET/C# analyzer and detectors
internal/analyzer/nodejs/ JavaScript/TypeScript analyzer and detectors
internal/config/          CLI and metrics-generator defaults
internal/estimator/       Active-series sizing formulas
internal/report/          HTML and JSON rendering
internal/source/          Local and Git source resolution
testdata/fixtures/        Analyzer and estimator fixtures
example/                  Customer-facing example workspace
```

## Development

Run tests:

```sh
GOCACHE="$PWD/.gocache" go test ./...
```

Run a fixture scan:

```sh
go run ./cmd/gco11y-size scan \
  --repo testdata/fixtures/go-service \
  --out /tmp/gco11y-go-fixture.html \
  --json /tmp/gco11y-go-fixture.json
```

Add another framework detector:

1. Add a focused detector package under the language analyzer.
2. Emit neutral `model.Service`, `model.Operation`, `model.Edge`, `model.ConfigFinding`, and `model.Risk` records.
3. Reuse shared helpers for route normalization, service-root inference, target normalization, and cardinality risk checks.
4. Add fixture coverage under `testdata/fixtures`.
5. Keep estimator logic language-agnostic.

Add another language:

1. Create a package under `internal/analyzer/<language>`.
2. Implement `framework.Analyzer`.
3. Register the analyzer in `registeredAnalyzers()` in `internal/analyzer/analyzer.go`.
4. Document coverage and gaps in `internal/analyzer/<language>/README.md`.

## Limitations

- This is a static estimator, not a billing oracle.
- Runtime-only spans, dynamic routes, generated routes, service discovery, feature flags, and runtime label values may not be visible in source.
- The estimator cannot know real status-code spread, tenant cardinality, pod churn, or which operations receive traffic.
- Service graph estimates depend on static dependency hints unless runtime telemetry is used for calibration.
- Generated active series can differ from the estimate when metrics-generator configuration, custom dimensions, histogram settings, or instrumentation changes.

## License

MIT. See [LICENSE](LICENSE).
