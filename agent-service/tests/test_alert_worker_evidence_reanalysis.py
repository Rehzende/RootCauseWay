"""Tests for AlertWorker._handle_evidence_uploaded's synthetic alert context.

A platform audit found this path set alert["service"] = software_id (a raw
UUID) instead of the software's real slug/name -- when the LLM picked a
k8s-agent skill during this re-analysis, its Prometheus/Loki/Tempo queries
(which match on the real `service` label, e.g. "pulse-backend") found
nothing, producing a near-empty, low-confidence RCA. Confirmed live during a
full-pipeline test against a real Pulso incident. The original alert.received
path is unaffected -- it carries the real alert labels -- only this
evidence-triggered re-analysis path loses them.
"""

from __future__ import annotations

import json
import uuid
from datetime import datetime, timezone
from unittest.mock import AsyncMock, patch

import pytest

from app.workers.alert_worker import AlertWorker


@pytest.fixture
def org_id():
    return uuid.uuid4()


@pytest.fixture
def worker():
    return AlertWorker(
        redis_client=AsyncMock(),
        backend_client=AsyncMock(),
        event_publisher=AsyncMock(),
    )


def _evidence_uploaded_data(org_id, incident_id):
    return {
        "event_id": str(uuid.uuid4()),
        "event_type": "evidence.uploaded",
        "org_id": str(org_id),
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "payload": {
            "incident_id": str(incident_id),
            "evidence_id": str(uuid.uuid4()),
            "type": "log",
            "title": "manual note",
        },
    }


@pytest.mark.asyncio
async def test_reanalysis_alert_uses_software_slug_as_service(worker, org_id):
    incident_id = uuid.uuid4()
    software_id = uuid.uuid4()
    worker._backend.get_incident.return_value = {
        "id": str(incident_id),
        "software_id": str(software_id),
        "title": "PulseBackendHighErrorRate",
        "description": "5xx rate above 5%",
        "severity": "high",
    }
    worker._backend.get_software.return_value = {
        "id": str(software_id), "slug": "pulse-backend", "name": "Pulso Backend",
    }

    with patch.object(worker._orchestrator, "handle_incident", new=AsyncMock(return_value={})) as mock_handle:
        await worker._handle_evidence_uploaded(_evidence_uploaded_data(org_id, incident_id))

    worker._backend.get_software.assert_awaited_once_with(str(software_id), org_id)
    sent_alert = mock_handle.await_args.kwargs["alert"]
    assert sent_alert["service"] == "pulse-backend"


@pytest.mark.asyncio
async def test_reanalysis_alert_falls_back_to_software_name_when_no_slug(worker, org_id):
    incident_id = uuid.uuid4()
    software_id = uuid.uuid4()
    worker._backend.get_incident.return_value = {"id": str(incident_id), "software_id": str(software_id)}
    worker._backend.get_software.return_value = {"id": str(software_id), "name": "Pulso Backend"}

    with patch.object(worker._orchestrator, "handle_incident", new=AsyncMock(return_value={})) as mock_handle:
        await worker._handle_evidence_uploaded(_evidence_uploaded_data(org_id, incident_id))

    sent_alert = mock_handle.await_args.kwargs["alert"]
    assert sent_alert["service"] == "Pulso Backend"


@pytest.mark.asyncio
async def test_reanalysis_alert_falls_back_to_software_id_when_get_software_fails(worker, org_id):
    """Degrades gracefully rather than aborting the whole re-analysis --
    matches this same handler's existing get_incident failure handling."""
    incident_id = uuid.uuid4()
    software_id = uuid.uuid4()
    worker._backend.get_incident.return_value = {"id": str(incident_id), "software_id": str(software_id)}
    worker._backend.get_software.side_effect = Exception("404")

    with patch.object(worker._orchestrator, "handle_incident", new=AsyncMock(return_value={})) as mock_handle:
        await worker._handle_evidence_uploaded(_evidence_uploaded_data(org_id, incident_id))

    sent_alert = mock_handle.await_args.kwargs["alert"]
    assert sent_alert["service"] == str(software_id)
