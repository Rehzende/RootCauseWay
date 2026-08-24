"""Core handle_task coverage for TriageAgent -- happy path and the LLM
failure fallback. No test existed for this agent at all before the platform
audit; it's the simplest of the 5 A2A microservices (no mesh calls) so it's
the cleanest place to establish the pattern the other 4 already extend
with mesh-specific tests (see test_agent_mesh.py in evidence/rca/postmortem)."""

from __future__ import annotations

import json
from unittest.mock import AsyncMock, patch

import pytest

from app.a2a.models import DataPart, Message, Role, TaskStatus
from app.agent import TriageAgent
from app.observability.metrics import ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL


@pytest.fixture
def agent():
    return TriageAgent(api_base="http://lm-studio:1234/v1", api_key="x", model="m")


def _message(data: dict) -> Message:
    return Message(role=Role.USER, parts=[DataPart(data=data)])


_USAGE = {"model": "m", "prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150}


@pytest.mark.asyncio
async def test_handle_task_injects_skill_prompt_template_into_prompt(agent):
    """See rca-agent/tests/test_agent.py's identical test for the full
    rationale -- Skill.prompt_template used to be captured and never
    read anywhere."""
    llm_output = json.dumps({"severity_assessment": "medium", "category": "x", "affected_components": [], "summary": "x", "confidence": 0.5})
    mock_call_llm = AsyncMock(return_value=(llm_output, _USAGE))
    with patch.object(agent, "_call_llm", new=mock_call_llm):
        await agent.handle_task("t1", _message({
            "alert": {"title": "x"},
            "skill_prompt_template": "Always flag anything mentioning connection pools as high severity.",
        }))

    prompt = mock_call_llm.await_args.args[0]
    assert "Always flag anything mentioning connection pools as high severity." in prompt
    assert "## Additional Skill-Specific Instructions" in prompt
    assert "Respond with ONLY valid JSON:" in prompt


@pytest.mark.asyncio
async def test_handle_task_happy_path_returns_triage_result(agent):
    llm_output = json.dumps({
        "severity_assessment": "high",
        "category": "database",
        "affected_components": ["postgres"],
        "summary": "Connection pool exhausted",
        "confidence": 0.85,
    })
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=(llm_output, _USAGE))):
        task = await agent.handle_task("t1", _message({
            "alert": {"title": "DB errors", "severity": "high"},
        }))

    assert task.status == TaskStatus.COMPLETED
    assert len(task.artifacts) == 2
    assert task.artifacts[0].name == "triage_result"
    data = task.artifacts[0].parts[0].data
    assert data["severity_assessment"] == "high"
    assert data["affected_components"] == ["postgres"]

    usage_artifact = next(a for a in task.artifacts if a.name == "llm_usage")
    assert usage_artifact.parts[0].data == _USAGE


@pytest.mark.asyncio
async def test_handle_task_llm_failure_falls_back_and_records_metric(agent):
    before = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="triage_agent", error_type="llm_call_failed"
    )._value.get()

    with patch.object(agent, "_call_llm", new=AsyncMock(side_effect=RuntimeError("LM Studio unreachable"))):
        task = await agent.handle_task("t1", _message({
            "alert": {"title": "DB errors", "severity": "critical"},
        }))

    # Must still return a COMPLETED task with a usable fallback -- a
    # crashed triage stage shouldn't take the whole pipeline down.
    assert task.status == TaskStatus.COMPLETED
    data = task.artifacts[0].parts[0].data
    assert data["severity_assessment"] == "critical"  # falls back to the alert's own severity
    assert data["confidence"] == 0.0

    after = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="triage_agent", error_type="llm_call_failed"
    )._value.get()
    assert after == before + 1


@pytest.mark.asyncio
async def test_handle_task_llm_malformed_json_falls_back(agent):
    """_parse_json returns {} for unparseable output -- handle_task must
    still complete rather than propagate a KeyError/JSONDecodeError."""
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=("not json at all", _USAGE))):
        task = await agent.handle_task("t1", _message({"alert": {"title": "x"}}))

    assert task.status == TaskStatus.COMPLETED
    assert task.artifacts[0].parts[0].data == {}


def test_parse_json_extracts_from_markdown_fence(agent):
    text = 'Here is the result:\n```json\n{"severity_assessment": "low"}\n```\n'
    assert agent._parse_json(text) == {"severity_assessment": "low"}


@pytest.mark.asyncio
async def test_call_llm_extracts_real_usage_from_api_response(agent, monkeypatch):
    """_call_llm must report the API's own `usage` field and echoed `model`
    -- not a fabricated estimate. This replaces the orchestrator's old
    chars/4 heuristic + hardcoded model label with real data."""
    class _FakeResponse:
        def raise_for_status(self):
            pass

        def json(self):
            return {
                "model": "qwen2.5-coder-14b-instruct",
                "choices": [{"message": {"content": "hello"}}],
                "usage": {"prompt_tokens": 321, "completion_tokens": 45, "total_tokens": 366},
            }

    class _FakeClient:
        async def __aenter__(self):
            return self

        async def __aexit__(self, *a):
            return False

        async def post(self, *a, **kw):
            return _FakeResponse()

    monkeypatch.setattr("app.agent.httpx.AsyncClient", lambda **kw: _FakeClient())

    content, usage = await agent._call_llm("prompt")

    assert content == "hello"
    assert usage == {
        "model": "qwen2.5-coder-14b-instruct",
        "prompt_tokens": 321,
        "completion_tokens": 45,
        "total_tokens": 366,
    }


@pytest.mark.asyncio
async def test_call_llm_falls_back_to_configured_model_when_response_omits_it(agent, monkeypatch):
    """Some OpenAI-compatible servers don't echo `model` back -- fall back
    to the model this agent was configured with rather than reporting None."""
    class _FakeResponse:
        def raise_for_status(self):
            pass

        def json(self):
            return {"choices": [{"message": {"content": "hello"}}]}

    class _FakeClient:
        async def __aenter__(self):
            return self

        async def __aexit__(self, *a):
            return False

        async def post(self, *a, **kw):
            return _FakeResponse()

    monkeypatch.setattr("app.agent.httpx.AsyncClient", lambda **kw: _FakeClient())

    _, usage = await agent._call_llm("prompt")

    assert usage == {"model": "m", "prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0}
