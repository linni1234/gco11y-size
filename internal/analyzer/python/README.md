# Python Analyzer

The Python analyzer statically scans backend Python services without running Python, importing modules, installing packages, or starting framework apps.

## Coverage

- Web/API: FastAPI, Starlette, Flask, Quart, Django, Django REST Framework, Sanic, aiohttp, Tornado, Falcon, Bottle, Litestar/Starlite, GraphQL route hints, websockets, and health-style routes.
- Runtime/config: `pyproject.toml`, `setup.cfg`, `setup.py`, `requirements*.txt`, `Pipfile`, `poetry.lock`, `uv.lock`, `manage.py`, ASGI/WSGI entrypoint hints, `Procfile`, Docker command hints, Serverless Framework, SAM, Azure Functions, Chalice, and Google/Firebase function hints.
- Dependencies: SQLAlchemy, Django database settings, psycopg/psycopg2, asyncpg, MySQL clients, MongoDB/Motor, Redis, Memcached, Elasticsearch/OpenSearch, DynamoDB, and vector/search clients such as Qdrant, Weaviate, Pinecone, Chroma, Milvus, and pgvector.
- Messaging/background: Celery, RQ, Dramatiq, Huey, APScheduler, Kafka clients, RabbitMQ clients, AWS SQS/SNS/EventBridge/Kinesis, Google Pub/Sub, and Azure Service Bus.
- Observability: OpenTelemetry spans/resources/instrumentation imports, ddtrace and Sentry findings, and high-cardinality span attribute risks.
- gRPC/protobuf: `.proto` service definitions, Python servicer classes, `add_*Servicer_to_server`, and gRPC channel edges.

## Behavior

- Emits only neutral model records: `Service`, `Operation`, `Edge`, `ConfigFinding`, and `Risk`.
- Uses high confidence for literal routes and targets, medium for convention-based framework registrations, and low for dynamic/config-only evidence.
- Avoids counting tests, migrations, notebooks, generated protobuf files, ORM models, SQL queries, CLI scripts, and package metadata as inbound operations.
- Leaves dedupe to the shared analyzer merge layer, including `.proto` plus Python gRPC implementation overlaps.

## Extending

- Add Python detectors under `internal/analyzer/python/detectors/<area>`.
- Reuse helpers in `internal/analyzer/python/detectors/common` for Python imports, decorators, balanced calls, string extraction, route normalization, target normalization, and neutral model construction.
- Add fixture coverage under `testdata/fixtures` and assert representative operations, edges, config findings, risks, and false-positive guards in `internal/analyzer/analyzer_test.go`.
