"""OpenTelemetry tracing + Prometheus metrics for the azure-agent microservice."""

from __future__ import annotations

import logging
import os

from opentelemetry import trace
from opentelemetry.sdk.resources import SERVICE_NAME, Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.instrumentation.httpx import HTTPXClientInstrumentor
from prometheus_fastapi_instrumentator import Instrumentator

logger = logging.getLogger(__name__)

OTEL_SERVICE_NAME = os.getenv("OTEL_SERVICE_NAME", "rootcauseway-azure-agent")
OTEL_EXPORTER_OTLP_ENDPOINT = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
ENVIRONMENT = os.getenv("ENVIRONMENT", "production")


def setup_telemetry(app) -> None:
    """Wire up tracing (→ Tempo via OTLP) and metrics (→ Prometheus /metrics)."""

    # 1. Prometheus metrics — always on, scraped by the ServiceMonitor.
    Instrumentator().instrument(app).expose(app, endpoint="/metrics", include_in_schema=False)

    # 2. Tracing — only if an OTLP endpoint is configured.
    resource = Resource.create(attributes={
        SERVICE_NAME: OTEL_SERVICE_NAME,
        "deployment.environment": ENVIRONMENT,
    })
    tracer_provider = TracerProvider(resource=resource)

    if OTEL_EXPORTER_OTLP_ENDPOINT:
        base_url = OTEL_EXPORTER_OTLP_ENDPOINT.rstrip("/")
        if base_url.endswith("/v1/traces"):
            base_url = base_url[: -len("/v1/traces")]
        exporter = OTLPSpanExporter(endpoint=f"{base_url}/v1/traces")
        tracer_provider.add_span_processor(BatchSpanProcessor(exporter))
        logger.info(f"[OTEL] Tracing -> {base_url}/v1/traces (service: {OTEL_SERVICE_NAME})")
    else:
        logger.warning("[OTEL] No OTLP endpoint configured - spans won't be exported.")

    trace.set_tracer_provider(tracer_provider)

    FastAPIInstrumentor.instrument_app(app)
    HTTPXClientInstrumentor().instrument()
