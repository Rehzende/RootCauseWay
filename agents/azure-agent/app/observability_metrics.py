"""Swallowed-error counter for azure-agent.

Same pattern as the other 5 RootCauseway Python services: registers into the
default prometheus_client registry that telemetry.py's
Instrumentator().expose(app, endpoint="/metrics") already serves.
"""

from __future__ import annotations

from prometheus_client import Counter

ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL = Counter(
    "rootcauseway_swallowed_errors_total",
    "Errors caught, logged, and NOT re-raised, labeled by component and "
    "error_type. Each occurrence means a pipeline stage silently degraded "
    "instead of failing loud.",
    ["component", "error_type"],
)


def record_swallowed_error(component: str, error_type: str) -> None:
    ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(component=component, error_type=error_type).inc()
