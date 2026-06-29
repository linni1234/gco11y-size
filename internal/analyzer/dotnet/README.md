# .NET/C# Analyzer

The .NET/C# analyzer scans C# source, Razor/Blazor route hints, ASP.NET Core configuration, protobuf files, and common .NET dependency patterns. It emits neutral `model.Service`, `model.Operation`, `model.Edge`, `model.ConfigFinding`, and `model.Risk` records for the shared estimator.

## What Is Analyzed

- Service names from `.csproj`, `appsettings*.json`, `launchSettings.json`, `OTEL_SERVICE_NAME`, `OTEL_RESOURCE_ATTRIBUTES`, and OpenTelemetry `AddService(...)` hints.
- ASP.NET Core Minimal API routes from `MapGet`, `MapPost`, `MapPut`, `MapDelete`, `MapPatch`, `MapMethods`, route groups, and health checks.
- ASP.NET Core MVC/Web API controller routes from `[Route]`, `[HttpGet]`, `[HttpPost]`, `[HttpPut]`, `[HttpDelete]`, `[HttpPatch]`, `[HttpHead]`, `[HttpOptions]`, and `[AcceptVerbs]`.
- Razor Pages and Blazor Server/Web App route hints from `@page`.
- SignalR hubs from `MapHub<T>(...)`.
- YARP reverse-proxy routes and downstream targets from `ReverseProxy` config, plus low-confidence `MapReverseProxy()` source hints.
- gRPC operations from `.proto` service definitions and C# service classes inheriting generated `*Base` classes.
- Data dependency edges from EF Core provider calls, EF/DbContext hints, Dapper/ADO.NET connection patterns, SQL Server, PostgreSQL, MySQL, SQLite, Cosmos DB, MongoDB, and Redis.
- Messaging operations and producer edges from Kafka, RabbitMQ, Azure Service Bus, MassTransit, and NServiceBus patterns.
- Background and distributed work hints from `BackgroundService`, `IHostedService`, Hangfire, Quartz.NET, .NET Aspire AppHost resources, and Orleans grains/streams.
- OpenTelemetry custom spans from `ActivitySource.StartActivity(...)` and high-cardinality span tag risks from `SetTag`, `AddTag`, and `SetAttribute`.

## What Is Missing Or Limited

- The analyzer does not run `dotnet`, build projects, resolve NuGet packages, execute source generators, or use Roslyn type information.
- Dynamic route registration, reflection, generated endpoints, conventions hidden behind custom abstractions, and runtime-only configuration are best-effort.
- Full Blazor WebAssembly client analysis is out of scope; only server-relevant `@page` hints are counted.
- Desktop, mobile, and game frameworks such as WPF, WinForms, MAUI, Avalonia, Xamarin, and Unity are out of scope unless backend service code appears in the scanned repository.
- EF queries, Dapper calls, and every background job are not counted as inbound HTTP operations. They are represented as dependency edges, producer/consumer operations, or internal span hints when useful for sizing.
- Connection-string references such as `GetConnectionString("OrdersDb")` can produce low-confidence provider edges when the actual configured target is not statically resolved.

## Extension Points

- Add .NET detectors under `internal/analyzer/dotnet/detectors/<area>`.
- Reuse helpers in `internal/analyzer/dotnet/detectors/common` for C# comment stripping, balanced call parsing, string literal extraction, route normalization, target normalization, and neutral model construction.
- Prefer structured parsing for JSON/XML config and focused C# source scanning for source patterns.
- Emit only neutral model records. The estimator should not need to know which .NET framework produced an operation or edge.
