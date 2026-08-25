"""Pins a live-found bug in AlertWorker's alert.received handling: IngestAlert
(Go) creates and commits the incident row *before* this handler ever runs
(publishing alert.received happens after), so CorrelationEngine.check_correlation
must be told to exclude payload.incident_id -- otherwise the very first alert
of a brand-new incident finds itself as "an open incident on this
software_id within the window" and self-correlates, and the pipeline below
(SnapshotCollector + orchestrator.handle_incident) never runs. Confirmed
live: a real fired Pulso alert self-correlated and no triage/evidence/rca
agent was ever dispatched for it.
"""

from __future__ import annotations

import json
import uuid
from datetime import datetime, timezone
from unittest.mock import AsyncMock, patch

import pytest

from app.workers.alert_worker import AlertWorker


@pytest.fixture
def worker():
    return AlertWorker(
        redis_client=AsyncMock(),
        backend_client=AsyncMock(),
        event_publisher=AsyncMock(),
    )


def _alert_received_message(org_id, incident_id, software_id, snapshot_id):
    return {"data": json.dumps(_alert_received_data(org_id, incident_id, software_id, snapshot_id))}


def _alert_received_data(org_id, incident_id, software_id, snapshot_id):
    return {
        "event_id": str(uuid.uuid4()),
        "event_type": "alert.received",
        "org_id": str(org_id),
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "payload": {
            "alert_snapshot_id": str(snapshot_id),
            "incident_id": str(incident_id),
            "software_id": str(software_id),
            "webhook_source": "prometheus_alertmanager",
            "normalized_alert": {
                "title": "PulseBackendHighErrorRate",
                "description": "5xx rate above 5%",
                "severity": "high",
                "source": "prometheus_alertmanager",
            },
        },
    }


@pytest.mark.asyncio
async def test_correlation_check_excludes_the_incident_this_alert_created(worker):
    org_id = uuid.uuid4()
    incident_id = uuid.uuid4()
    software_id = uuid.uuid4()
    snapshot_id = uuid.uuid4()

    # No real correlation match either way -- only the exclusion argument matters here.
    worker._backend.list_correlation_rules = AsyncMock(return_value=[])
    worker._backend.get_software_dependency_graph = AsyncMock(return_value=None)
    worker._backend.check_correlation = AsyncMock(return_value=None)

    with patch("app.workers.alert_worker.SnapshotCollector") as MockCollector:
        MockCollector.return_value.collect_snapshots = AsyncMock(return_value=[])
        with patch.object(worker._orchestrator, "handle_incident", new=AsyncMock(return_value={})):
            await worker._handle_message(
                _alert_received_message(org_id, incident_id, software_id, snapshot_id)
            )

    worker._backend.check_correlation.assert_awaited_once()
    call_kwargs = worker._backend.check_correlation.call_args[1]
    assert call_kwargs["exclude_incident_id"] == incident_id


@pytest.mark.asyncio
async def test_correlation_check_uses_configured_dedup_window(worker):
    """dedup_window_seconds used to never be threaded through from settings
    at all -- CorrelationEngine's own class default (900s) was silently used
    regardless of correlation_dedup_window_seconds. Pins the wiring fix."""
    org_id = uuid.uuid4()
    incident_id = uuid.uuid4()
    software_id = uuid.uuid4()
    snapshot_id = uuid.uuid4()

    with patch.object(
        worker._correlation, "check_correlation", new=AsyncMock(return_value=None),
    ) as mock_check:
        with patch("app.workers.alert_worker.SnapshotCollector") as MockCollector:
            MockCollector.return_value.collect_snapshots = AsyncMock(return_value=[])
            with patch.object(worker._orchestrator, "handle_incident", new=AsyncMock(return_value={})):
                await worker._handle_message(
                    _alert_received_message(org_id, incident_id, software_id, snapshot_id)
                )

    mock_check.assert_awaited_once()
    call_kwargs = mock_check.call_args[1]
    from app.config.settings import get_settings
    assert call_kwargs["dedup_window_seconds"] == get_settings().correlation_dedup_window_seconds


@pytest.mark.asyncio
async def test_self_only_match_no_longer_short_circuits_the_pipeline(worker):
    """Direct regression pin: before the fix, this exact scenario (the only
    "open incident" found is the one this alert created) made
    CorrelationEngine.check_correlation return non-None, so the code below
    never called orchestrator.handle_incident at all."""
    org_id = uuid.uuid4()
    incident_id = uuid.uuid4()
    software_id = uuid.uuid4()
    snapshot_id = uuid.uuid4()

    worker._backend.list_correlation_rules = AsyncMock(return_value=[])
    worker._backend.get_software_dependency_graph = AsyncMock(return_value=None)

    async def fake_check_correlation(*, org_id, software_id, alert, time_window_seconds, exclude_incident_id=None):
        # Simulates the real (fixed) backend: the only open incident is the
        # caller-excluded one, so there's no correlation.
        return None

    worker._backend.check_correlation = AsyncMock(side_effect=fake_check_correlation)

    with patch("app.workers.alert_worker.SnapshotCollector") as MockCollector:
        MockCollector.return_value.collect_snapshots = AsyncMock(return_value=[])
        with patch.object(worker._orchestrator, "handle_incident", new=AsyncMock(return_value={})) as mock_handle:
            await worker._handle_message(
                _alert_received_message(org_id, incident_id, software_id, snapshot_id)
            )

    mock_handle.assert_awaited_once()
