"""Tests for the SnapshotCollector."""

from __future__ import annotations

import uuid
from unittest.mock import AsyncMock, patch

import httpx
import pytest
import respx

from app.evidence.snapshot_collector import (
    SnapshotCollector,
    _build_datadog_log_query,
    _build_datadog_queries,
    _build_prom_queries,
    _build_prom_selector,
)


@pytest.fixture
def backend():
    client = AsyncMock()
    client.get_software_observability = AsyncMock(return_value=[])
    client.add_incident_evidence = AsyncMock(return_value={"id": str(uuid.uuid4())})
    return client


@pytest.fixture
def collector(backend):
    return SnapshotCollector(backend)


@pytest.fixture
def sample_alert():
    return {
        "labels": {
            "alertname": "HighCPU",
            "namespace": "demo-store",
            "pod": "store-api-abc123",
            "service": "store-api",
        },
        "severity": "critical",
    }


# -- Prometheus query building --


class TestPromQueryBuilding:
    def test_selector_with_all_labels(self):
        sel = _build_prom_selector(namespace="demo", pod="api-xyz", service="api")
        assert 'namespace="demo"' in sel
        assert 'pod=~"api-xyz.*"' in sel
        assert 'service="api"' in sel

    def test_selector_empty(self):
        sel = _build_prom_selector(namespace="", pod="", service="")
        assert sel == "{}"

    def test_cpu_alert_includes_cpu_query(self):
        queries = _build_prom_queries("{}", {"labels": {"alertname": "HighCPU"}})
        assert "cpu_usage" in queries
        assert "error_rate" in queries

    def test_memory_alert_includes_memory_query(self):
        queries = _build_prom_queries("{}", {"labels": {"alertname": "OOMKilled"}})
        assert "memory_usage" in queries

    def test_generic_alert_includes_cpu_and_memory(self):
        queries = _build_prom_queries("{}", {"labels": {"alertname": "SomethingElse"}})
        assert "cpu_usage" in queries
        assert "memory_usage" in queries
        assert "error_rate" in queries


# -- Graceful failure --


class TestGracefulFailure:
    @pytest.mark.asyncio
    async def test_source_fetch_failure_returns_empty(self, collector, backend):
        backend.get_software_observability.side_effect = Exception("connection refused")
        result = await collector.collect_snapshots(
            incident_id=uuid.uuid4(),
            software_id="sw-1",
            alert={},
            org_id=uuid.uuid4(),
        )
        assert result == []

    @pytest.mark.asyncio
    async def test_individual_source_failure_continues(self, collector, backend):
        backend.get_software_observability.return_value = [
            {"source_type": "prometheus", "url": "http://prom:9090", "name": "prom1"},
            {"source_type": "loki", "url": "http://loki:3100", "name": "loki1"},
        ]

        with patch(
            "app.evidence.snapshot_collector._http_client"
        ) as mock_client_fn:
            mock_client = AsyncMock()
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=False)
            mock_client.get = AsyncMock(side_effect=Exception("timeout"))
            mock_client_fn.return_value = mock_client

            result = await collector.collect_snapshots(
                incident_id=uuid.uuid4(),
                software_id="sw-1",
                alert={"labels": {"namespace": "demo"}},
                org_id=uuid.uuid4(),
            )
            assert result == []


# -- Evidence storage --


class TestEvidenceStorage:
    @pytest.mark.asyncio
    async def test_prometheus_evidence_stored(self, collector, backend, sample_alert):
        backend.get_software_observability.return_value = [
            {"source_type": "prometheus", "url": "http://prom:9090", "name": "prom1"},
        ]

        mock_response = AsyncMock()
        mock_response.status_code = 200
        mock_response.raise_for_status = lambda: None
        mock_response.json.return_value = {"data": {"resultType": "matrix", "result": []}}

        with patch(
            "app.evidence.snapshot_collector._http_client"
        ) as mock_client_fn:
            mock_client = AsyncMock()
            mock_client.__aenter__ = AsyncMock(return_value=mock_client)
            mock_client.__aexit__ = AsyncMock(return_value=False)
            mock_client.get = AsyncMock(return_value=mock_response)
            mock_client_fn.return_value = mock_client

            result = await collector.collect_snapshots(
                incident_id=uuid.uuid4(),
                software_id="sw-1",
                alert=sample_alert,
                org_id=uuid.uuid4(),
            )

            assert len(result) == 1
            backend.add_incident_evidence.assert_called_once()
            call_args = backend.add_incident_evidence.call_args
            evidence = call_args[0][1]  # second positional arg
            assert evidence.type == "metric"
            assert evidence.source == "prom1"


# -- Datadog --


@pytest.fixture
def datadog_source():
    return {
        "source_type": "datadog",
        "name": "dd-prod",
        "auth_config": {"api_key": "dd-api-key-123", "app_key": "dd-app-key-456"},
    }


class TestDatadog:
    @pytest.mark.asyncio
    @respx.mock
    async def test_metrics_and_logs_collected(
        self, collector, backend, sample_alert, datadog_source
    ):
        backend.get_software_observability.return_value = [datadog_source]

        metrics_route = respx.get("https://api.datadoghq.com/api/v1/query").mock(
            return_value=httpx.Response(
                200, json={"status": "ok", "series": [{"metric": "kubernetes.cpu.usage.total"}]}
            )
        )
        logs_route = respx.post("https://api.datadoghq.com/api/v2/logs/events/search").mock(
            return_value=httpx.Response(
                200, json={"data": [{"attributes": {"message": "boom"}}]}
            )
        )

        result = await collector.collect_snapshots(
            incident_id=uuid.uuid4(),
            software_id="sw-1",
            alert=sample_alert,
            org_id=uuid.uuid4(),
        )

        # One metric evidence + one log evidence
        assert len(result) == 2
        assert backend.add_incident_evidence.call_count == 2
        types = [c[0][1].type for c in backend.add_incident_evidence.call_args_list]
        assert types == ["metric", "log"]
        sources = [c[0][1].source for c in backend.add_incident_evidence.call_args_list]
        assert sources == ["dd-prod", "dd-prod"]

        assert metrics_route.called
        assert logs_route.called

    @pytest.mark.asyncio
    @respx.mock
    async def test_auth_headers_sent(self, collector, backend, sample_alert, datadog_source):
        backend.get_software_observability.return_value = [datadog_source]

        metrics_route = respx.get("https://api.datadoghq.com/api/v1/query").mock(
            return_value=httpx.Response(200, json={"series": []})
        )
        logs_route = respx.post("https://api.datadoghq.com/api/v2/logs/events/search").mock(
            return_value=httpx.Response(200, json={"data": []})
        )

        await collector.collect_snapshots(
            incident_id=uuid.uuid4(),
            software_id="sw-1",
            alert=sample_alert,
            org_id=uuid.uuid4(),
        )

        for route in (metrics_route, logs_route):
            request = route.calls[0].request
            assert request.headers["DD-API-KEY"] == "dd-api-key-123"
            assert request.headers["DD-APPLICATION-KEY"] == "dd-app-key-456"

        # Metrics query carries the alert time window
        params = metrics_route.calls[0].request.url.params
        assert "from" in params and "to" in params and "query" in params
        assert int(params["to"]) - int(params["from"]) == 1800

    @pytest.mark.asyncio
    @respx.mock
    async def test_custom_site_used(self, collector, backend, sample_alert):
        source = {
            "source_type": "datadog",
            "name": "dd-eu",
            "auth_config": {"api_key": "k", "app_key": "a", "site": "datadoghq.eu"},
        }
        backend.get_software_observability.return_value = [source]

        metrics_route = respx.get("https://api.datadoghq.eu/api/v1/query").mock(
            return_value=httpx.Response(200, json={"series": []})
        )
        respx.post("https://api.datadoghq.eu/api/v2/logs/events/search").mock(
            return_value=httpx.Response(200, json={"data": []})
        )

        result = await collector.collect_snapshots(
            incident_id=uuid.uuid4(),
            software_id="sw-1",
            alert=sample_alert,
            org_id=uuid.uuid4(),
        )
        assert metrics_route.called
        assert len(result) == 1  # empty metric series skipped, log evidence stored

    @pytest.mark.asyncio
    @respx.mock
    async def test_api_failure_skips_gracefully(
        self, collector, backend, sample_alert, datadog_source
    ):
        backend.get_software_observability.return_value = [datadog_source]

        respx.get("https://api.datadoghq.com/api/v1/query").mock(
            return_value=httpx.Response(500, json={"errors": ["boom"]})
        )
        respx.post("https://api.datadoghq.com/api/v2/logs/events/search").mock(
            side_effect=httpx.ConnectError("connection refused")
        )

        result = await collector.collect_snapshots(
            incident_id=uuid.uuid4(),
            software_id="sw-1",
            alert=sample_alert,
            org_id=uuid.uuid4(),
        )
        assert result == []
        backend.add_incident_evidence.assert_not_called()

    @pytest.mark.asyncio
    async def test_missing_credentials_skips(self, collector, backend, sample_alert):
        backend.get_software_observability.return_value = [
            {"source_type": "datadog", "name": "dd-nokeys", "auth_config": {}},
        ]
        result = await collector.collect_snapshots(
            incident_id=uuid.uuid4(),
            software_id="sw-1",
            alert=sample_alert,
            org_id=uuid.uuid4(),
        )
        assert result == []
        backend.add_incident_evidence.assert_not_called()


class TestDatadogQueryBuilding:
    def test_cpu_alert_builds_cpu_query(self):
        queries = _build_datadog_queries(
            {}, {"labels": {"alertname": "HighCPU", "service": "store-api"}}
        )
        assert "cpu_usage" in queries
        assert "error_rate" in queries
        assert "service:store-api" in queries["cpu_usage"]

    def test_memory_alert_builds_memory_query(self):
        queries = _build_datadog_queries({}, {"labels": {"alertname": "OOMKilled"}})
        assert "memory_usage" in queries

    def test_configured_snapshot_queries_win(self):
        source = {
            "snapshot_configs": [
                {
                    "name": "custom_latency",
                    "snapshot_type": "metrics",
                    "query_template": "avg:custom.latency{env:prod}",
                    "enabled": True,
                },
                {
                    "name": "disabled_one",
                    "snapshot_type": "metrics",
                    "query_template": "avg:ignored{*}",
                    "enabled": False,
                },
            ]
        }
        queries = _build_datadog_queries(source, {"labels": {"alertname": "HighCPU"}})
        assert queries == {"custom_latency": "avg:custom.latency{env:prod}"}

    def test_log_query_defaults_to_service_errors(self):
        assert _build_datadog_log_query({}, service="store-api") == "service:store-api status:error"
        assert _build_datadog_log_query({}, service="") == "status:error"

    def test_log_query_uses_configured_template(self):
        source = {
            "snapshot_configs": [
                {
                    "name": "app_logs",
                    "snapshot_type": "logs",
                    "query_template": "service:store-api @http.status_code:>=500",
                }
            ]
        }
        assert (
            _build_datadog_log_query(source, service="store-api")
            == "service:store-api @http.status_code:>=500"
        )
