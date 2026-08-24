"""Core handle_task coverage for EvidenceAgent -- happy path and the LLM
failure fallback. Mesh-specific behavior (fetching k8s diagnostics) is
covered separately in test_agent_mesh.py."""

from __future__ import annotations

import json
from unittest.mock import AsyncMock, patch

import pytest

from app.a2a.models import DataPart, Message, Role, TaskStatus
from app.agent import EvidenceAgent
from app.observability.metrics import ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL


@pytest.fixture
def agent():
    return EvidenceAgent(api_base="http://lm-studio:1234/v1", api_key="x", model="m")


def _message(data: dict) -> Message:
    return Message(role=Role.USER, parts=[DataPart(data=data)])


_USAGE = {"model": "m", "prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150}


@pytest.mark.asyncio
async def test_handle_task_injects_skill_prompt_template_into_prompt(agent):
    """See rca-agent/tests/test_agent.py's identical test for the full
    rationale -- Skill.prompt_template used to be captured and never
    read anywhere."""
    llm_output = json.dumps({
        "evidence_findings": [], "recommended_data_sources": [], "log_queries": [],
        "summary": "x",
    })
    mock_call_llm = AsyncMock(return_value=(llm_output, _USAGE))
    with patch.object(agent, "_call_llm", new=mock_call_llm):
        await agent.handle_task("t1", _message({
            "alert": {"title": "x"},
            "skill_prompt_template": "Always check for connection pool exhaustion in the evidence.",
        }))

    prompt = mock_call_llm.await_args.args[0]
    assert "Always check for connection pool exhaustion in the evidence." in prompt
    assert "## Additional Skill-Specific Instructions" in prompt
    assert "Respond with ONLY valid JSON:" in prompt


@pytest.mark.asyncio
async def test_handle_task_without_skill_prompt_template_omits_section(agent):
    llm_output = json.dumps({
        "evidence_findings": [], "recommended_data_sources": [], "log_queries": [],
        "summary": "x",
    })
    mock_call_llm = AsyncMock(return_value=(llm_output, _USAGE))
    with patch.object(agent, "_call_llm", new=mock_call_llm):
        await agent.handle_task("t1", _message({"alert": {"title": "x"}}))

    prompt = mock_call_llm.await_args.args[0]
    assert "Additional Skill-Specific Instructions" not in prompt


@pytest.mark.asyncio
async def test_handle_task_happy_path_returns_evidence_result(agent):
    llm_output = json.dumps({
        "evidence_findings": [{"type": "log", "title": "connection errors", "source": "app logs", "relevance": "root cause", "priority": "high"}],
        "recommended_data_sources": ["Loki"],
        "log_queries": [],
        "summary": "Check application logs for connection errors",
    })
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=(llm_output, _USAGE))):
        task = await agent.handle_task("t1", _message({"alert": {"title": "x"}}))

    assert task.status == TaskStatus.COMPLETED
    data = task.artifacts[0].parts[0].data
    assert data["recommended_data_sources"] == ["Loki"]

    usage_artifact = next(a for a in task.artifacts if a.name == "llm_usage")
    assert usage_artifact.parts[0].data == _USAGE


@pytest.mark.asyncio
async def test_handle_task_llm_failure_falls_back_and_records_metric(agent):
    before = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="evidence_agent", error_type="llm_call_failed"
    )._value.get()

    with patch.object(agent, "_call_llm", new=AsyncMock(side_effect=RuntimeError("down"))):
        task = await agent.handle_task("t1", _message({"alert": {"title": "x"}}))

    assert task.status == TaskStatus.COMPLETED
    assert task.artifacts[0].parts[0].data["summary"] == "Evidence collection failed"

    after = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="evidence_agent", error_type="llm_call_failed"
    )._value.get()
    assert after == before + 1
