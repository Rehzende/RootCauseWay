"""Prometheus metrics for this agent -- trimmed version of
agent-service/app/observability/metrics.py's swallowed-error counter (no
A2A call metrics or stream queue gauge here, those are orchestrator-only
concerns). See that file's module docstring for the full rationale: turns
an `except: logger.exception(...)`-and-continue site into a real Prometheus
alert instead of a log line nobody's watching.
"""

from __future__ import annotations

from prometheus_client import CONTENT_TYPE_LATEST, Counter, generate_latest
from prometheus_client import make_asgi_app

ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL = Counter(
    "rootcauseway_swallowed_errors_total",
    "Errors caught, logged, and NOT re-raised, labeled by component and "
    "error_type. Each occurrence means a pipeline stage silently degraded "
    "instead of failing loud.",
    ["component", "error_type"],
)


def record_swallowed_error(component: str, error_type: str) -> None:
    ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(component=component, error_type=error_type).inc()


metrics_app = make_asgi_app()


def metrics_text() -> tuple[bytes, str]:
    return generate_latest(), CONTENT_TYPE_LATEST
