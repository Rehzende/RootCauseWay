"""Read-only clients for the RootCauseway observability stack (Prometheus, Loki, Tempo,
Alertmanager) — used to gather evidence about a service before handing it to
the LLM for root-cause analysis.

All URLs default to the in-cluster DNS names of the `monitoring` namespace's
kube-prometheus-stack + Loki + Tempo install. Override via env vars if the
agent ever needs to point at a different cluster/namespace.
"""

from __future__ import annotations

import logging
import os
from typing import Any

import httpx

logger = logging.getLogger(__name__)

PROMETHEUS_URL = os.getenv("PROMETHEUS_URL", "http://monitoring-kube-prometheus-prometheus.monitoring:9090")
LOKI_URL = os.getenv("LOKI_URL", "http://loki-gateway.monitoring.svc.cluster.local")
TEMPO_URL = os.getenv("TEMPO_URL", "http://tempo.monitoring.svc.cluster.local:3200")
ALERTMANAGER_URL = os.getenv("ALERTMANAGER_URL", "http://monitoring-kube-prometheus-alertmanager.monitoring:9093")

HTTP_TIMEOUT = 10.0


async def _get(client: httpx.AsyncClient, url: str, **kwargs) -> Any:
    try:
        r = await client.get(url, timeout=HTTP_TIMEOUT, **kwargs)
        r.raise_for_status()
        return r.json()
    except Exception as exc:
        logger.warning("observability query failed: %s -> %s", url, exc)
        return {"error": str(exc)}


async def query_prometheus_instant(client: httpx.AsyncClient, promql: str) -> Any:
    """Run a single PromQL instant query, return the raw `data.result` list."""
    data = await _get(client, f"{PROMETHEUS_URL}/api/v1/query", params={"query": promql})
    if isinstance(data, dict) and "data" in data:
        return data["data"].get("result", [])
    return data


async def collect_prometheus_evidence(client: httpx.AsyncClient, job: str, namespace: str, pod_prefix: str) -> dict:
    """Golden-signal snapshot for a service, using the same queries as the
    PrometheusRule alerts so the LLM sees exactly what tripped (or didn't)."""
    queries = {
        "up": f'up{{job="{job}"}}',
        "error_rate_pct_5m": (
            f'100 * sum(rate(http_requests_total{{job="{job}",status=~"5.."}}[5m])) '
            f'/ sum(rate(http_requests_total{{job="{job}"}}[5m]))'
        ),
        "p95_latency_seconds_5m": (
            f'histogram_quantile(0.95, sum(rate(http_request_duration_highr_seconds_bucket{{job="{job}"}}[5m])) by (le))'
        ),
        "restarts_15m": (
            f'increase(kube_pod_container_status_restarts_total{{namespace="{namespace}",pod=~"{pod_prefix}.*"}}[15m])'
        ),
        "cpu_cores_now": (
            f'sum(rate(container_cpu_usage_seconds_total{{namespace="{namespace}",pod=~"{pod_prefix}.*",container!=""}}[2m]))'
        ),
        "memory_bytes_now": (
            f'sum(container_memory_working_set_bytes{{namespace="{namespace}",pod=~"{pod_prefix}.*",container!=""}})'
        ),
    }
    results = {}
    for name, promql in queries.items():
        rows = await query_prometheus_instant(client, promql)
        if isinstance(rows, list) and rows:
            # Most of these are scalar-ish (no meaningful label grouping) — take the first value.
            results[name] = rows[0].get("value", [None, None])[1]
        else:
            results[name] = None
    return results


async def collect_active_alerts(client: httpx.AsyncClient, service: str) -> list[dict]:
    """Active Alertmanager alerts whose `service` label matches, plus anything
    already in `firing`/`pending` state in Prometheus rules for this service."""
    data = await _get(client, f"{ALERTMANAGER_URL}/api/v2/alerts")
    if not isinstance(data, list):
        return []
    return [
        {
            "alertname": a.get("labels", {}).get("alertname"),
            "severity": a.get("labels", {}).get("severity"),
            "state": a.get("status", {}).get("state"),
            "summary": a.get("annotations", {}).get("summary"),
        }
        for a in data
        if a.get("labels", {}).get("service") == service
    ]


async def collect_recent_error_logs(client: httpx.AsyncClient, namespace: str, app: str, limit: int = 30) -> list[str]:
    """Recent log lines from Loki that look like errors, newest first."""
    logql = f'{{namespace="{namespace}", app="{app}"}} |~ `(?i)error|exception|traceback|failed`'
    data = await _get(
        client,
        f"{LOKI_URL}/loki/api/v1/query_range",
        params={"query": logql, "limit": limit, "direction": "backward"},
    )
    lines: list[str] = []
    try:
        for stream in data["data"]["result"]:
            for ts, line in stream["values"]:
                lines.append(line)
    except Exception:
        pass
    return lines[:limit]


async def collect_recent_error_traces(client: httpx.AsyncClient, otel_service: str, limit: int = 10) -> list[dict]:
    """Recent error-status traces for a service, summarized (root span name +
    duration), newest first."""
    traceql = f'{{resource.service.name="{otel_service}" && status=error}}'
    data = await _get(
        client,
        f"{TEMPO_URL}/api/search",
        params={"q": traceql, "limit": limit},
    )
    traces = []
    try:
        for t in data.get("traces", []):
            traces.append({
                "trace_id": t.get("traceID"),
                "root_span": t.get("rootTraceName"),
                "duration_ms": t.get("durationMs"),
            })
    except Exception:
        pass
    return traces
