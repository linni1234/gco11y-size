from opentelemetry import trace
from opentelemetry.sdk.resources import Resource

resource = Resource.create({"service.name": "python-checkout"})
tracer = trace.get_tracer("checkout")

with tracer.start_as_current_span("reconcile-orders") as span:
    span.set_attribute("user.id", "42")
