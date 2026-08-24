"""Regression test for the incident.resolved -> postmortem trigger.

A real incident close found that closing/resolving an incident never
produced a postmortem, for two independent reasons -- both fixed in
alert_worker.py's _handle_incident_resolved:

1. CreateAgentRunRequest was built without the required `incident_id`
   field, so pydantic rejected it before the postmortem agent ever ran.
2. Once (1) was fixed, agent_type="postmortem" 500'd against the backend's
   agent_runs_agent_type_check constraint, which only allows
   "postmortem_generator" (see migration 002_incident_cockpit.up.sql) --
   the same class of bug fixed earlier in orchestrator.py's
   _AGENT_TYPE_BY_SKILL mapping.
"""

from __future__ import annotations

import uuid
from unittest.mock import AsyncMock

import pytest

from app.observability.metrics import ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL
from app.workers.alert_worker import AlertWorker


@pytest.fixture
def org_id():
    return uuid.uuid4()


@pytest.fixture
def incident_id():
    return uuid.uuid4()


@pytest.fixture
def worker(monkeypatch):
    monkeypatch.setenv("EVENT_RETRY_BACKOFF_BASE", "0")
    redis_client = AsyncMock()
    backend_client = AsyncMock()
    event_publisher = AsyncMock()
    w = AlertWorker(redis_client, backend_client, event_publisher)
    return w


@pytest.mark.asyncio
async def test_handle_incident_resolved_creates_agent_run_with_incident_id(worker, org_id, incident_id):
    worker._backend.get_incident.return_value = {"software_id": str(uuid.uuid4()), "severity": "high"}
    worker._backend.get_rca.return_value = {"rca": {}}
    worker._backend.get_rci.return_value = {}
    worker._backend.create_agent_run.return_value = {"id": str(uuid.uuid4())}
    worker._backend.update_agent_run.return_value = {}

    # Skip the real orchestrator dispatch/HITL-gate logic -- this test only
    # guards the CreateAgentRunRequest construction that crashed in
    # production, not the postmortem generation itself.
    worker._orchestrator.run_postmortem_stage = AsyncMock(
        return_value={"status": "completed", "postmortem": {"title": "stub"}}
    )
    worker._notifier.notify = AsyncMock()

    data = {
        "org_id": str(org_id),
        "payload": {"incident_id": str(incident_id)},
    }

    await worker._handle_incident_resolved(data)

    worker._backend.create_agent_run.assert_awaited_once()
    call_args = worker._backend.create_agent_run.await_args
    called_incident_id, request = call_args.args
    assert called_incident_id == incident_id
    assert request.incident_id == incident_id
    assert request.agent_name == "postmortem"
    # Must match the DB CHECK constraint on agent_runs.agent_type, not the
    # bare agent_name -- "postmortem" alone 500s.
    assert request.agent_type == "postmortem_generator"


@pytest.mark.asyncio
async def test_handle_incident_resolved_records_swallowed_error_on_failure(worker, org_id, incident_id):
    """The bugs this file guards were invisible in production until someone
    went looking at pod logs -- this asserts the failure path now also
    increments rootcauseway_swallowed_errors_total{component="alert_worker",
    error_type="postmortem_generation_failed"}, which the
    RootCausewayAgentServiceSwallowedError Prometheus rule alerts on."""
    worker._backend.get_incident.side_effect = RuntimeError("backend unreachable")

    before = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="alert_worker", error_type="postmortem_generation_failed"
    )._value.get()

    data = {"org_id": str(org_id), "payload": {"incident_id": str(incident_id)}}
    await worker._handle_incident_resolved(data)

    after = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="alert_worker", error_type="postmortem_generation_failed"
    )._value.get()
    assert after == before + 1
