"""Collects evidence snapshots from observability sources when an alert arrives."""

from __future__ import annotations

import logging
import time
from typing import Any
from uuid import UUID

import httpx

from app.models.api import IncidentEvidenceCreate
from app.services.backend_client import BackendClient

logger = logging.getLogger(__name__)

# Default time range for queries (30 minutes)
_DEFAULT_RANGE_SECONDS = 1800


class SnapshotCollector:
    """Collects evidence snapshots from observability sources when an alert arrives."""

    def __init__(self, backend_client: BackendClient) -> None:
        self._backend = backend_client

    async def collect_snapshots(
        self,
        incident_id: UUID,
        software_id: str,
        alert: dict[str, Any],
        org_id: UUID,
    ) -> list[dict[str, Any]]:
        """Fetch observability sources for the software and query each one.

        Returns a list of evidence dicts that were stored on the incident.
        """
        try:
            sources = await self._backend.get_software_observability(software_id)
        except Exception:
            logger.warning(
                "Could not fetch observability sources for software %s, skipping snapshot collection",
                software_id,
            )
            return []

        collected: list[dict[str, Any]] = []

        for source in sources:
            source_type = source.get("source_type", "").lower()
            try:
                # Each snapshot is an (evidence_type, data) pair; datadog can
                # yield several (metrics + logs) from a single source.
                snapshots: list[tuple[str, dict[str, Any]]] = []
                if source_type == "prometheus":
                    data = await self._query_prometheus(source, alert)
                    if data:
                        snapshots.append((_evidence_type_for(source_type), data))
                elif source_type == "loki":
                    data = await self._query_loki(source, alert)
                    if data:
                        snapshots.append((_evidence_type_for(source_type), data))
                elif source_type == "tempo":
                    data = await self._query_tempo(source, alert)
                    if data:
                        snapshots.append((_evidence_type_for(source_type), data))
                elif source_type == "datadog":
                    snapshots = await self._query_datadog(source, alert)
                else:
                    logger.debug("Unknown source type %s, skipping", source_type)
                    continue

                for evidence_type, data in snapshots:
                    evidence = IncidentEvidenceCreate(
                        type=evidence_type,
                        title=f"Auto-collected {source_type} snapshot",
                        content=data,
                        source=source.get("name", source_type),
                    )
                    result = await self._backend.add_incident_evidence(incident_id, evidence, org_id)
                    collected.append(result)
                    logger.info(
                        "Stored %s evidence for incident %s from source %s",
                        source_type, incident_id, source.get("name"),
                    )

            except Exception:
                logger.warning(
                    "Failed to collect %s snapshot from source %s for incident %s",
                    source_type, source.get("name"), incident_id,
                    exc_info=True,
                )

        return collected

    # ------------------------------------------------------------------
    # Source-specific query methods
    # ------------------------------------------------------------------

    async def _query_prometheus(
        self,
        source: dict[str, Any],
        alert: dict[str, Any],
        time_range_seconds: int = _DEFAULT_RANGE_SECONDS,
    ) -> dict[str, Any] | None:
        """Query Prometheus for metrics related to the alert."""
        base_url = source.get("url", "").rstrip("/")
        if not base_url:
            return None

        end = time.time()
        start = end - time_range_seconds

        labels = alert.get("labels", {})
        pod = labels.get("pod", "")
        namespace = labels.get("namespace", "")
        service = labels.get("service", labels.get("job", ""))

        selector = _build_prom_selector(namespace=namespace, pod=pod, service=service)
        queries = _build_prom_queries(selector, alert)

        results: dict[str, Any] = {"queries": {}}
        async with _http_client(source) as client:
            for name, expr in queries.items():
                try:
                    resp = await client.get(
                        f"{base_url}/api/v1/query_range",
                        params={
                            "query": expr,
                            "start": str(start),
                            "end": str(end),
                            "step": "60",
                        },
                    )
                    resp.raise_for_status()
                    results["queries"][name] = resp.json().get("data", {})
                except Exception:
                    logger.debug("Prometheus query %s failed", name, exc_info=True)

        return results if results["queries"] else None

    async def _query_loki(
        self,
        source: dict[str, Any],
        alert: dict[str, Any],
        time_range_seconds: int = _DEFAULT_RANGE_SECONDS,
    ) -> dict[str, Any] | None:
        """Query Loki for logs related to the alert."""
        base_url = source.get("url", "").rstrip("/")
        if not base_url:
            return None

        end_ns = int(time.time() * 1e9)
        start_ns = end_ns - int(time_range_seconds * 1e9)

        labels = alert.get("labels", {})
        namespace = labels.get("namespace", "")
        pod = labels.get("pod", "")

        logql = _build_logql(namespace=namespace, pod=pod)

        async with _http_client(source) as client:
            try:
                resp = await client.get(
                    f"{base_url}/loki/api/v1/query_range",
                    params={
                        "query": logql,
                        "start": str(start_ns),
                        "end": str(end_ns),
                        "limit": "200",
                    },
                )
                resp.raise_for_status()
                return {"query": logql, "data": resp.json().get("data", {})}
            except Exception:
                logger.debug("Loki query failed", exc_info=True)
                return None

    async def _query_tempo(
        self,
        source: dict[str, Any],
        alert: dict[str, Any],
    ) -> dict[str, Any] | None:
        """Query Tempo for recent traces from the affected service."""
        base_url = source.get("url", "").rstrip("/")
        if not base_url:
            return None

        labels = alert.get("labels", {})
        service_name = labels.get("service", labels.get("job", ""))
        if not service_name:
            return None

        async with _http_client(source) as client:
            try:
                resp = await client.get(
                    f"{base_url}/api/traces",
                    params={
                        "service": service_name,
                        "limit": "20",
                        "minDuration": "100ms",
                    },
                )
                resp.raise_for_status()
                return {"service": service_name, "traces": resp.json().get("traces", [])}
            except Exception:
                logger.debug("Tempo query failed", exc_info=True)
                return None

    async def _query_datadog(
        self,
        source: dict[str, Any],
        alert: dict[str, Any],
        time_range_seconds: int = _DEFAULT_RANGE_SECONDS,
    ) -> list[tuple[str, dict[str, Any]]]:
        """Query Datadog for metrics (v1 query API) and logs (v2 logs search).

        Returns a list of (evidence_type, data) pairs: metrics are stored as
        ``metric`` evidence and logs as ``log`` evidence.
        """
        auth_config = source.get("auth_config") or {}
        api_key = auth_config.get("api_key", auth_config.get("key", ""))
        app_key = auth_config.get("app_key", auth_config.get("application_key", ""))
        if not api_key or not app_key:
            logger.warning(
                "Datadog source %s missing api_key/app_key, skipping", source.get("name")
            )
            return []

        site = source.get("site") or auth_config.get("site") or "datadoghq.com"
        base_url = (
            source.get("url") or source.get("base_url") or f"https://api.{site}"
        ).rstrip("/")

        end = int(time.time())
        start = end - time_range_seconds

        labels = alert.get("labels", {})
        service = labels.get("service", labels.get("job", ""))

        headers = {
            "DD-API-KEY": api_key,
            "DD-APPLICATION-KEY": app_key,
        }

        snapshots: list[tuple[str, dict[str, Any]]] = []
        async with httpx.AsyncClient(headers=headers, timeout=30.0) as client:
            # --- Metrics (v1 query API) ---
            metrics: dict[str, Any] = {"queries": {}}
            for name, query in _build_datadog_queries(source, alert).items():
                try:
                    resp = await client.get(
                        f"{base_url}/api/v1/query",
                        params={"from": str(start), "to": str(end), "query": query},
                    )
                    resp.raise_for_status()
                    series = resp.json().get("series", [])
                    if series:
                        metrics["queries"][name] = {"query": query, "series": series}
                except Exception:
                    logger.debug("Datadog metric query %s failed", name, exc_info=True)
            if metrics["queries"]:
                snapshots.append(("metric", metrics))

            # --- Logs (v2 logs search) ---
            log_query = _build_datadog_log_query(source, service=service)
            try:
                resp = await client.post(
                    f"{base_url}/api/v2/logs/events/search",
                    json={
                        "filter": {
                            "query": log_query,
                            "from": _iso_utc(start),
                            "to": _iso_utc(end),
                        },
                        "page": {"limit": 200},
                        "sort": "-timestamp",
                    },
                )
                resp.raise_for_status()
                snapshots.append(
                    ("log", {"query": log_query, "data": resp.json().get("data", [])})
                )
            except Exception:
                logger.debug("Datadog logs query failed", exc_info=True)

        return snapshots


# ------------------------------------------------------------------
# Helpers
# ------------------------------------------------------------------


def _evidence_type_for(source_type: str) -> str:
    return {
        "prometheus": "metric",
        "loki": "log",
        "tempo": "trace",
        "datadog": "metric",
    }.get(source_type, "snapshot")


def _build_prom_selector(*, namespace: str, pod: str, service: str) -> str:
    parts: list[str] = []
    if namespace:
        parts.append(f'namespace="{namespace}"')
    if pod:
        parts.append(f'pod=~"{pod}.*"')
    if service:
        parts.append(f'service="{service}"')
    return "{" + ", ".join(parts) + "}" if parts else "{}"


def _build_prom_queries(selector: str, alert: dict[str, Any]) -> dict[str, str]:
    """Build a set of PromQL queries relevant to the alert."""
    alert_name = alert.get("labels", {}).get("alertname", "").lower()

    queries: dict[str, str] = {}

    # Always include error rate
    queries["error_rate"] = f'rate(http_requests_total{{status=~"5..",{selector[1:-1]}}}[5m])'

    if "cpu" in alert_name:
        queries["cpu_usage"] = f"rate(container_cpu_usage_seconds_total{selector}[5m])"
    elif "memory" in alert_name or "oom" in alert_name:
        queries["memory_usage"] = f"container_memory_working_set_bytes{selector}"
    elif "latency" in alert_name or "duration" in alert_name:
        queries["request_duration"] = (
            f"histogram_quantile(0.99, rate(http_request_duration_seconds_bucket{selector}[5m]))"
        )
    else:
        # Generic: include CPU and memory for general alerts
        queries["cpu_usage"] = f"rate(container_cpu_usage_seconds_total{selector}[5m])"
        queries["memory_usage"] = f"container_memory_working_set_bytes{selector}"

    return queries


def _iso_utc(epoch_seconds: int) -> str:
    from datetime import datetime, timezone

    return datetime.fromtimestamp(epoch_seconds, tz=timezone.utc).isoformat()


def _configured_snapshot_queries(source: dict[str, Any], snapshot_type: str) -> dict[str, str]:
    """Extract enabled query templates of a given snapshot_type from the source's
    snapshot configs (if the backend attached any)."""
    queries: dict[str, str] = {}
    for cfg in source.get("snapshot_configs") or []:
        if (
            cfg.get("snapshot_type") == snapshot_type
            and cfg.get("query_template")
            and cfg.get("enabled", True)
        ):
            queries[cfg.get("name", f"query_{len(queries)}")] = cfg["query_template"]
    return queries


def _build_datadog_scope(*, namespace: str, pod: str, service: str) -> str:
    parts: list[str] = []
    if service:
        parts.append(f"service:{service}")
    if namespace:
        parts.append(f"kube_namespace:{namespace}")
    if pod:
        parts.append(f"pod_name:{pod}*")
    return "{" + ",".join(parts) + "}" if parts else "{*}"


def _build_datadog_queries(source: dict[str, Any], alert: dict[str, Any]) -> dict[str, str]:
    """Build Datadog metric queries: configured snapshot queries win, otherwise
    derive defaults from the alert labels (same convention as Prometheus)."""
    configured = _configured_snapshot_queries(source, "metrics")
    if configured:
        return configured

    labels = alert.get("labels", {})
    scope = _build_datadog_scope(
        namespace=labels.get("namespace", ""),
        pod=labels.get("pod", ""),
        service=labels.get("service", labels.get("job", "")),
    )
    alert_name = labels.get("alertname", "").lower()

    queries: dict[str, str] = {
        "error_rate": f"sum:trace.http.request.errors{scope}.as_rate()",
    }
    if "cpu" in alert_name:
        queries["cpu_usage"] = f"avg:kubernetes.cpu.usage.total{scope}"
    elif "memory" in alert_name or "oom" in alert_name:
        queries["memory_usage"] = f"avg:kubernetes.memory.usage{scope}"
    elif "latency" in alert_name or "duration" in alert_name:
        queries["request_duration"] = f"avg:trace.http.request.duration{scope}"
    else:
        queries["cpu_usage"] = f"avg:kubernetes.cpu.usage.total{scope}"
        queries["memory_usage"] = f"avg:kubernetes.memory.usage{scope}"

    return queries


def _build_datadog_log_query(source: dict[str, Any], *, service: str) -> str:
    """Build the Datadog logs search query: configured logs query wins,
    otherwise filter errors for the affected service."""
    configured = _configured_snapshot_queries(source, "logs")
    if configured:
        return next(iter(configured.values()))
    if service:
        return f"service:{service} status:error"
    return "status:error"


def _build_logql(*, namespace: str, pod: str) -> str:
    parts: list[str] = []
    if namespace:
        parts.append(f'namespace="{namespace}"')
    if pod:
        parts.append(f'pod=~"{pod}.*"')
    selector = "{" + ", ".join(parts) + "}" if parts else '{job=~".+"}'
    return f'{selector} |= "error" or "ERROR" or "panic" or "fatal"'


def _http_client(source: dict[str, Any]) -> httpx.AsyncClient:
    """Create an httpx client with auth configured from the source."""
    headers: dict[str, str] = {}
    auth_type = source.get("auth_type", "none").lower()
    auth_config = source.get("auth_config", {})

    if auth_type == "bearer":
        token = auth_config.get("token", "")
        if token:
            headers["Authorization"] = f"Bearer {token}"
    elif auth_type == "api_key":
        key = auth_config.get("key", "")
        header_name = auth_config.get("header", "X-Api-Key")
        if key:
            headers[header_name] = key
    elif auth_type == "basic":
        # httpx handles basic auth separately but we set it via header for simplicity
        import base64
        user = auth_config.get("username", "")
        pwd = auth_config.get("password", "")
        encoded = base64.b64encode(f"{user}:{pwd}".encode()).decode()
        headers["Authorization"] = f"Basic {encoded}"

    return httpx.AsyncClient(headers=headers, timeout=30.0)
