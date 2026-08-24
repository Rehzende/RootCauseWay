"""Tests for Orchestrator._build_postmortem_context / _dispatch_postmortem.

Found live: this pair used to build {"incident": ..., "rci": ..., "rca":
...} and send it as a TextPart. PostmortemAgent.handle_task's own
_extract_data only reads DataPart.data (`hasattr(part, "data")`), so every
postmortem generated via the incident.resolved trigger (or a HITL-gate
resume) ran with a completely empty input_data -- alert/triage/evidence/
rci/rca/software_context were all `{}` in the prompt, on every model, the
whole time this code path existed. Confirmed by resolving a real incident
and inspecting the resulting "Postmortem: Generation Failed" placeholder's
traceback, which pointed at JSON-parsing a response generated from an
empty-context prompt.
"""

from __future__ import annotations

import uuid
from unittest.mock import AsyncMock

import pytest

from app.a2a.client import A2AClient
from app.a2a.models import Artifact, DataPart, Task, TaskStatus
from app.orchestrator.orchestrator import Orchestrator
from app.services.backend_client import BackendClient


@pytest.fixture
def backend():
    client = AsyncMock(spec=BackendClient)
    client.get_organization_settings = AsyncMock(return_value={"pipeline_hitl_gate_enabled": False})
    return client


@pytest.fixture
def a2a():
    return AsyncMock(spec=A2AClient)


@pytest.fixture
def orchestrator(backend, a2a):
    return Orchestrator(backend_client=backend, a2a_client=a2a, llm_call=AsyncMock())


@pytest.mark.asyncio
async def test_build_postmortem_context_shape_matches_what_the_agent_reads(orchestrator, backend):
    """PostmortemAgent.handle_task reads alert/triage/evidence/rci/rca/
    software_context -- the old shape ({"incident": ...}) had none of
    these keys except rci/rca, which coincidentally matched."""
    incident_id = uuid.uuid4()
    org_id = uuid.uuid4()
    software_id = "sw-1"
    backend.get_incident.return_value = {
        "id": str(incident_id),
        "title": "PulseBackendHighErrorRate",
        "description": "5xx rate above 5%",
        "severity": "high",
        "software_id": software_id,
        "evidence": [{"id": "e1", "type": "log", "title": "manual note"}],
    }
    backend.get_software.return_value = {"id": software_id, "name": "Pulso Backend"}
    backend.get_rca.return_value = {"root_cause_summary": "x", "confidence": 0.8}
    backend.get_rci.return_value = {"investigation_summary": "y"}

    context = await orchestrator._build_postmortem_context(incident_id, org_id)

    assert context["alert"] == {
        "title": "PulseBackendHighErrorRate",
        "description": "5xx rate above 5%",
        "severity": "high",
    }
    assert context["software_context"]["name"] == "Pulso Backend"
    assert context["rca"]["root_cause_summary"] == "x"
    assert context["rci"]["investigation_summary"] == "y"
    assert context["evidence"] == [{"id": "e1", "type": "log", "title": "manual note"}]
    assert "incident" not in context
    backend.get_software.assert_awaited_once_with(software_id, org_id)


@pytest.mark.asyncio
async def test_build_postmortem_context_degrades_gracefully_without_software_or_rca(orchestrator, backend):
    incident_id = uuid.uuid4()
    org_id = uuid.uuid4()
    backend.get_incident.return_value = {"id": str(incident_id), "title": "x", "severity": "high"}
    backend.get_software.side_effect = Exception("404")
    backend.get_rca.side_effect = Exception("no rca")
    backend.get_rci.side_effect = Exception("no rci")

    context = await orchestrator._build_postmortem_context(incident_id, org_id)

    assert context["alert"]["title"] == "x"
    assert context["software_context"] == {}
    assert context["rca"] is None
    assert context["rci"] is None
    assert context["evidence"] == []


@pytest.mark.asyncio
async def test_dispatch_postmortem_sends_a_datapart_not_textpart(orchestrator, backend, a2a):
    """The core bug: DataPart is what PostmortemAgent's _extract_data
    (`hasattr(part, "data")`) can actually read. A TextPart silently
    produces an empty input_data with no error anywhere."""
    incident_id = uuid.uuid4()
    a2a.send_task.return_value = Task(
        id="t1", status=TaskStatus.COMPLETED,
        artifacts=[Artifact(name="postmortem", parts=[DataPart(data={"title": "Postmortem: x"})])],
    )
    context = {"alert": {"title": "x"}, "rci": None, "rca": None, "software_context": {}, "evidence": []}

    await orchestrator._dispatch_postmortem(incident_id, context, uuid.uuid4())

    sent_message = a2a.send_task.await_args.args[2]
    assert len(sent_message.parts) == 1
    part = sent_message.parts[0]
    assert part.type == "data"
    assert part.data == context


@pytest.mark.asyncio
async def test_dispatch_postmortem_emits_started_and_completed_timeline_events(orchestrator, backend, a2a):
    incident_id = uuid.uuid4()
    org_id = uuid.uuid4()
    a2a.send_task.return_value = Task(
        id="t1", status=TaskStatus.COMPLETED,
        artifacts=[Artifact(name="postmortem", parts=[DataPart(data={"title": "Postmortem: x"})])],
    )
    context = {"alert": {"title": "x"}, "rci": None, "rca": None, "software_context": {}, "evidence": []}

    await orchestrator._dispatch_postmortem(incident_id, context, org_id)

    event_types = [c.args[1].type for c in backend.add_incident_event.await_args_list]
    assert event_types == ["postmortem_started", "postmortem_completed"]
    for call in backend.add_incident_event.await_args_list:
        assert call.args[0] == incident_id
        assert call.args[2] == org_id


@pytest.mark.asyncio
async def test_run_postmortem_stage_with_no_explicit_context_builds_and_sends_real_alert_data(
    orchestrator, backend, a2a,
):
    """End-to-end: the incident.resolved trigger path (alert_worker.py
    now calls run_postmortem_stage(incident_id, org_id) with no context
    arg) must result in the agent actually receiving the real alert, not
    an empty dict."""
    incident_id = uuid.uuid4()
    org_id = uuid.uuid4()
    backend.get_incident.return_value = {
        "id": str(incident_id), "title": "PulseBackendHighErrorRate",
        "description": "d", "severity": "high", "software_id": "sw-1", "evidence": [],
    }
    backend.get_software.return_value = {"id": "sw-1", "name": "Pulso Backend"}
    backend.get_rca.side_effect = Exception("none")
    backend.get_rci.side_effect = Exception("none")
    a2a.send_task.return_value = Task(
        id="t1", status=TaskStatus.COMPLETED,
        artifacts=[Artifact(name="postmortem", parts=[DataPart(data={"title": "Postmortem: real"})])],
    )

    result = await orchestrator.run_postmortem_stage(incident_id, org_id)

    assert result["status"] == "completed"
    sent_message = a2a.send_task.await_args.args[2]
    sent_data = sent_message.parts[0].data
    assert sent_data["alert"]["title"] == "PulseBackendHighErrorRate"
