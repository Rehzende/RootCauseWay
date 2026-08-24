"""Tests for CorrelationEngine."""

import pytest
from unittest.mock import AsyncMock
from uuid import uuid4

from app.correlation.engine import CorrelationEngine


@pytest.fixture
def engine():
    return CorrelationEngine()


def _quiet_backend() -> AsyncMock:
    """A backend mock with all correlation-relevant methods stubbed to "no match"
    so tests only need to override what they care about."""
    backend = AsyncMock()
    backend.list_correlation_rules = AsyncMock(return_value=[])
    backend.check_correlation = AsyncMock(return_value={})
    backend.find_incident_by_fingerprint = AsyncMock(return_value=None)
    backend.get_software_dependency_graph = AsyncMock(return_value={})
    backend.list_open_incidents_by_software = AsyncMock(return_value=[])
    backend.add_incident_event = AsyncMock(return_value={})
    return backend


class TestCorrelationEngine:
    @pytest.mark.asyncio
    async def test_correlation_found(self, engine):
        incident_id = uuid4()
        backend = _quiet_backend()
        backend.check_correlation = AsyncMock(return_value={
            "incident_id": str(incident_id), "rule_id": "same_service",
        })

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
        )
        assert result == incident_id

    @pytest.mark.asyncio
    async def test_no_correlation(self, engine):
        backend = _quiet_backend()

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
        )
        assert result is None

    @pytest.mark.asyncio
    async def test_correlation_different_services(self, engine):
        backend = _quiet_backend()
        backend.check_correlation = AsyncMock(return_value=None)

        result = await engine.check_correlation(
            backend, uuid4(), "svc-2", {"alert_name": "HighCPU"},
        )
        assert result is None

    @pytest.mark.asyncio
    async def test_correlation_rule_time_window(self, engine):
        backend = _quiet_backend()
        backend.list_correlation_rules = AsyncMock(return_value=[
            {"time_window_seconds": 600, "type": "same_service"},
        ])

        await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
        )
        # Should use rule's time window
        call_kwargs = backend.check_correlation.call_args[1]
        assert call_kwargs["time_window_seconds"] == 600

    @pytest.mark.asyncio
    async def test_correlation_rules_fetch_failure(self, engine):
        backend = _quiet_backend()
        backend.list_correlation_rules = AsyncMock(side_effect=Exception("fail"))

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {},
        )
        assert result is None


class TestExcludeIncidentIdPropagation:
    """exclude_incident_id -- the incident IngestAlert (Go) already created for
    THIS exact alert instance, before alert.received was even published -- must
    reach both backend calls it can affect. Found live: without it, every
    brand-new incident is trivially "an open incident on this software_id
    within the window" (itself) and self-correlates; the pipeline never runs.
    """

    @pytest.mark.asyncio
    async def test_same_service_check_receives_exclude_incident_id(self, engine):
        backend = _quiet_backend()
        exclude_id = uuid4()

        await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
            exclude_incident_id=exclude_id,
        )

        call_kwargs = backend.check_correlation.call_args[1]
        assert call_kwargs["exclude_incident_id"] == exclude_id

    @pytest.mark.asyncio
    async def test_fingerprint_dedup_receives_exclude_incident_id(self, engine):
        backend = _quiet_backend()
        exclude_id = uuid4()

        await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU", "fingerprint": "fp-abc"},
            exclude_incident_id=exclude_id,
        )

        call_kwargs = backend.find_incident_by_fingerprint.call_args[1]
        assert call_kwargs["exclude_incident_id"] == exclude_id

    @pytest.mark.asyncio
    async def test_no_exclude_id_defaults_to_none(self, engine):
        backend = _quiet_backend()

        await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
        )

        call_kwargs = backend.check_correlation.call_args[1]
        assert call_kwargs["exclude_incident_id"] is None


class TestFingerprintDedup:
    @pytest.mark.asyncio
    async def test_dedup_short_circuits_full_correlation(self, engine):
        incident_id = uuid4()
        backend = _quiet_backend()
        backend.find_incident_by_fingerprint = AsyncMock(
            return_value={"id": str(incident_id), "software_id": "svc-1"},
        )

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU", "fingerprint": "fp-abc"},
        )

        assert result == incident_id
        # Dedup match short-circuits -- same-service / cascade logic never runs.
        backend.list_correlation_rules.assert_not_called()
        backend.check_correlation.assert_not_called()
        backend.get_software_dependency_graph.assert_not_called()

    @pytest.mark.asyncio
    async def test_dedup_records_match_distinctly(self, engine):
        incident_id = uuid4()
        backend = _quiet_backend()
        backend.find_incident_by_fingerprint = AsyncMock(
            return_value={"id": str(incident_id), "software_id": "svc-1"},
        )

        await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU", "fingerprint": "fp-abc"},
        )

        backend.add_incident_event.assert_awaited_once()
        call_args = backend.add_incident_event.call_args
        assert call_args[0][0] == incident_id
        event = call_args[0][1]
        assert event.type == "correlated_alert"
        assert event.data["match_type"] == "fingerprint_dedup"
        assert event.data["matched_on"] == {"fingerprint": "fp-abc"}

    @pytest.mark.asyncio
    async def test_no_fingerprint_skips_dedup_lookup(self, engine):
        backend = _quiet_backend()

        await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
        )

        backend.find_incident_by_fingerprint.assert_not_called()

    @pytest.mark.asyncio
    async def test_dedup_no_match_falls_through_to_same_service(self, engine):
        incident_id = uuid4()
        backend = _quiet_backend()
        backend.find_incident_by_fingerprint = AsyncMock(return_value=None)
        backend.check_correlation = AsyncMock(return_value={
            "incident_id": str(incident_id), "rule_id": "same_service",
        })

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU", "fingerprint": "fp-new"},
        )

        assert result == incident_id
        backend.find_incident_by_fingerprint.assert_awaited_once()

    @pytest.mark.asyncio
    async def test_dedup_lookup_failure_falls_through(self, engine):
        backend = _quiet_backend()
        backend.find_incident_by_fingerprint = AsyncMock(side_effect=Exception("backend down"))

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU", "fingerprint": "fp-x"},
        )

        assert result is None

    @pytest.mark.asyncio
    async def test_dedup_uses_configured_window(self, engine):
        backend = _quiet_backend()

        await engine.check_correlation(
            backend, uuid4(), "svc-1", {"fingerprint": "fp-abc"},
            dedup_window_seconds=120,
        )

        call_kwargs = backend.find_incident_by_fingerprint.call_args[1]
        assert call_kwargs["window_seconds"] == 120


class TestDependencyCascadeCorrelation:
    @pytest.mark.asyncio
    async def test_upstream_dependency_cascade(self, engine):
        db_incident_id = uuid4()
        db_software_id = str(uuid4())
        backend = _quiet_backend()
        backend.get_software_dependency_graph = AsyncMock(return_value={
            "software_id": "svc-1",
            "slug": "api-service",
            "upstream": [{"software_id": db_software_id, "slug": "postgres-primary"}],
            "downstream": [],
        })
        backend.list_open_incidents_by_software = AsyncMock(return_value=[
            {"id": str(db_incident_id), "software_id": db_software_id},
        ])

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "ConnectionRefused"},
        )

        assert result == db_incident_id
        call_kwargs = backend.list_open_incidents_by_software.call_args[1]
        assert call_kwargs["software_ids"] == [db_software_id]

    @pytest.mark.asyncio
    async def test_downstream_dependent_cascade(self, engine):
        consumer_incident_id = uuid4()
        consumer_software_id = str(uuid4())
        backend = _quiet_backend()
        backend.get_software_dependency_graph = AsyncMock(return_value={
            "software_id": "svc-1",
            "slug": "postgres-primary",
            "upstream": [],
            "downstream": [{"software_id": consumer_software_id, "slug": "checkout-service"}],
        })
        backend.list_open_incidents_by_software = AsyncMock(return_value=[
            {"id": str(consumer_incident_id), "software_id": consumer_software_id},
        ])

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "DiskFull"},
        )

        assert result == consumer_incident_id

    @pytest.mark.asyncio
    async def test_cascade_records_relationship_in_matched_on(self, engine):
        incident_id = uuid4()
        db_software_id = str(uuid4())
        backend = _quiet_backend()
        backend.get_software_dependency_graph = AsyncMock(return_value={
            "upstream": [{"software_id": db_software_id, "slug": "postgres-primary"}],
            "downstream": [],
        })
        backend.list_open_incidents_by_software = AsyncMock(return_value=[
            {"id": str(incident_id), "software_id": db_software_id},
        ])

        await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "ConnectionRefused"},
        )

        backend.add_incident_event.assert_awaited_once()
        event = backend.add_incident_event.call_args[0][1]
        assert event.data["match_type"] == "dependency_cascade"
        assert event.data["matched_on"]["relationship"] == "upstream_dependency"

    @pytest.mark.asyncio
    async def test_cascade_downstream_relationship_label(self, engine):
        incident_id = uuid4()
        consumer_software_id = str(uuid4())
        backend = _quiet_backend()
        backend.get_software_dependency_graph = AsyncMock(return_value={
            "upstream": [],
            "downstream": [{"software_id": consumer_software_id, "slug": "checkout-service"}],
        })
        backend.list_open_incidents_by_software = AsyncMock(return_value=[
            {"id": str(incident_id), "software_id": consumer_software_id},
        ])

        await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "DiskFull"},
        )

        event = backend.add_incident_event.call_args[0][1]
        assert event.data["matched_on"]["relationship"] == "downstream_dependent"

    @pytest.mark.asyncio
    async def test_same_service_match_skips_cascade_check(self, engine):
        incident_id = uuid4()
        backend = _quiet_backend()
        backend.check_correlation = AsyncMock(return_value={"incident_id": str(incident_id)})

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
        )

        assert result == incident_id
        backend.get_software_dependency_graph.assert_not_called()

    @pytest.mark.asyncio
    async def test_no_dependency_graph_falls_through_to_new_incident(self, engine):
        backend = _quiet_backend()
        backend.get_software_dependency_graph = AsyncMock(return_value=None)

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
        )
        assert result is None

    @pytest.mark.asyncio
    async def test_no_related_services_falls_through(self, engine):
        backend = _quiet_backend()
        backend.get_software_dependency_graph = AsyncMock(return_value={
            "upstream": [], "downstream": [],
        })

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
        )
        assert result is None
        backend.list_open_incidents_by_software.assert_not_called()

    @pytest.mark.asyncio
    async def test_related_services_but_no_open_incidents_falls_through(self, engine):
        backend = _quiet_backend()
        backend.get_software_dependency_graph = AsyncMock(return_value={
            "upstream": [{"software_id": str(uuid4()), "slug": "postgres-primary"}],
            "downstream": [],
        })
        backend.list_open_incidents_by_software = AsyncMock(return_value=[])

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
        )
        assert result is None

    @pytest.mark.asyncio
    async def test_dependency_graph_fetch_failure_falls_through(self, engine):
        backend = _quiet_backend()
        backend.get_software_dependency_graph = AsyncMock(side_effect=Exception("backend down"))

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
        )
        assert result is None

    @pytest.mark.asyncio
    async def test_cascade_lookup_failure_falls_through(self, engine):
        backend = _quiet_backend()
        backend.get_software_dependency_graph = AsyncMock(return_value={
            "upstream": [{"software_id": str(uuid4()), "slug": "postgres-primary"}],
            "downstream": [],
        })
        backend.list_open_incidents_by_software = AsyncMock(side_effect=Exception("backend down"))

        result = await engine.check_correlation(
            backend, uuid4(), "svc-1", {"alert_name": "HighCPU"},
        )
        assert result is None
