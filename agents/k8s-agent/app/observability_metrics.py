"""Swallowed-error counter for k8s-agent.

Named observability_metrics.py (not observability/metrics.py, to avoid
clashing with the existing app/observability.py module) -- registers into
the SAME default prometheus_client registry that
telemetry.py's Instrumentator().expose(app, endpoint="/metrics") already
serves, so no separate mount is needed here (unlike the other 4 agents,
which had no prometheus_client wiring at all before this).
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
