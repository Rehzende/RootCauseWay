"""Core handle_task coverage for PostmortemAgent -- happy path and the LLM
failure fallback. Mesh-specific behavior (fetching a missing RCA from a
peer) is covered separately in test_agent_mesh.py."""

from __future__ import annotations

import json
from unittest.mock import AsyncMock, patch

import pytest

from app.a2a.models import DataPart, Message, Role, TaskStatus
from app.agent import PostmortemAgent
from app.observability.metrics import ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL


@pytest.fixture
def agent():
    return PostmortemAgent(api_base="http://lm-studio:1234/v1", api_key="x", model="m")


def _message(data: dict) -> Message:
    return Message(role=Role.USER, parts=[DataPart(data=data)])


_USAGE = {"model": "m", "prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150}


@pytest.mark.asyncio
async def test_handle_task_injects_skill_prompt_template_into_prompt(agent):
    """See rca-agent/tests/test_agent.py's identical test for the full
    rationale -- Skill.prompt_template used to be captured and never
    read anywhere."""
    llm_output = json.dumps({
        "title": "x", "executive_summary": "x", "incident_timeline": [],
        "incident_timeline_narrative": "x", "root_cause_detail": "x", "impact_detail": "x",
        "lessons_learned": [], "action_items": [], "what_went_well": [],
        "what_went_wrong": [], "prevention_measures": [],
    })
    mock_call_llm = AsyncMock(return_value=(llm_output, _USAGE))
    with patch.object(agent, "_call_llm", new=mock_call_llm):
        await agent.handle_task("t1", _message({
            "alert": {"title": "x"}, "rca": {"root_cause_summary": "pool exhaustion"},
            "skill_prompt_template": "Always call out on-call fatigue as a contributing factor when relevant.",
        }))

    prompt = mock_call_llm.await_args.args[0]
    assert "Always call out on-call fatigue as a contributing factor when relevant." in prompt
    assert "## Additional Skill-Specific Instructions" in prompt
    assert "Respond with ONLY valid JSON:" in prompt


@pytest.mark.asyncio
async def test_handle_task_without_skill_prompt_template_omits_section(agent):
    llm_output = json.dumps({
        "title": "x", "executive_summary": "x", "incident_timeline": [],
        "incident_timeline_narrative": "x", "root_cause_detail": "x", "impact_detail": "x",
        "lessons_learned": [], "action_items": [], "what_went_well": [],
        "what_went_wrong": [], "prevention_measures": [],
    })
    mock_call_llm = AsyncMock(return_value=(llm_output, _USAGE))
    with patch.object(agent, "_call_llm", new=mock_call_llm):
        await agent.handle_task("t1", _message({
            "alert": {"title": "x"}, "rca": {"root_cause_summary": "pool exhaustion"},
        }))

    prompt = mock_call_llm.await_args.args[0]
    assert "Additional Skill-Specific Instructions" not in prompt


@pytest.mark.asyncio
async def test_handle_task_happy_path_returns_postmortem(agent):
    llm_output = json.dumps({
        "title": "Postmortem: Checkout Outage",
        "executive_summary": "...",
        "incident_timeline": [],
        "incident_timeline_narrative": "...",
        "root_cause_detail": "pool exhaustion",
        "impact_detail": "...",
        "lessons_learned": [],
        "action_items": [],
        "what_went_well": [],
        "what_went_wrong": [],
        "prevention_measures": [],
    })
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=(llm_output, _USAGE))):
        task = await agent.handle_task("t1", _message({
            "alert": {"title": "x"}, "rca": {"root_cause_summary": "pool exhaustion"},
        }))

    assert task.status == TaskStatus.COMPLETED
    assert task.artifacts[0].name == "postmortem"
    assert task.artifacts[0].parts[0].data["title"] == "Postmortem: Checkout Outage"
    usage_artifact = next(a for a in task.artifacts if a.name == "llm_usage")
    assert usage_artifact.parts[0].data == _USAGE


@pytest.mark.asyncio
async def test_handle_task_llm_failure_falls_back_and_records_metric(agent):
    before = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="postmortem_agent", error_type="llm_call_failed"
    )._value.get()

    with patch.object(agent, "_call_llm", new=AsyncMock(side_effect=RuntimeError("down"))):
        task = await agent.handle_task("t1", _message({
            "alert": {"title": "x"}, "rca": {"root_cause_summary": "present"},
        }))

    assert task.status == TaskStatus.COMPLETED
    assert task.artifacts[0].parts[0].data["title"] == "Postmortem: Generation Failed"

    after = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="postmortem_agent", error_type="llm_call_failed"
    )._value.get()
    assert after == before + 1
