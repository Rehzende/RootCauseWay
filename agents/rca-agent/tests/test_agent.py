"""Core handle_task coverage for RCAAgent -- happy path and the LLM failure
fallback. Mesh-specific behavior (fetching evidence/k8s diagnostics from a
peer) is covered separately in test_agent_mesh.py."""

from __future__ import annotations

import json
from unittest.mock import AsyncMock, patch

import pytest

from app.a2a.models import DataPart, Message, Role, TaskStatus
from app.agent import RCAAgent
from app.observability.metrics import ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL


@pytest.fixture
def agent():
    # No mesh peer URLs configured -- these tests exercise handle_task with
    # evidence already present in the input, so the mesh fetch never fires.
    return RCAAgent(api_base="http://lm-studio:1234/v1", api_key="x", model="m")


def _message(data: dict) -> Message:
    return Message(role=Role.USER, parts=[DataPart(data=data)])


_USAGE = {"model": "m", "prompt_tokens": 100, "completion_tokens": 50, "total_tokens": 150}


@pytest.mark.asyncio
async def test_handle_task_happy_path_returns_rci_rca_hypothesis(agent):
    llm_output = json.dumps({
        "rci": {"investigation_summary": "...", "impact_assessment": "high", "affected_services": ["checkout"]},
        "rca": {"root_cause_summary": "pool exhaustion", "root_cause_category": "config", "contributing_factors": [], "five_whys": [], "confidence": 0.9},
        "hypothesis": {"root_cause": "pool exhaustion", "confidence": 0.9, "recommended_actions": [], "mitigation_steps": []},
    })
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=(llm_output, _USAGE))):
        task = await agent.handle_task("t1", _message({
            "alert": {"title": "x"}, "evidence": {"summary": "already collected"},
        }))

    assert task.status == TaskStatus.COMPLETED
    names = {a.name for a in task.artifacts}
    assert names == {"rci", "rca", "hypothesis", "llm_usage"}
    rca_artifact = next(a for a in task.artifacts if a.name == "rca")
    assert rca_artifact.parts[0].data["root_cause_summary"] == "pool exhaustion"
    usage_artifact = next(a for a in task.artifacts if a.name == "llm_usage")
    assert usage_artifact.parts[0].data == _USAGE


@pytest.mark.asyncio
async def test_handle_task_injects_skill_prompt_template_into_prompt(agent):
    """A platform audit found Skill.prompt_template was captured on
    create/edit but never read anywhere -- a user could fill it in and it
    had zero effect on the actual LLM call. The orchestrator now threads
    it through as input_data["skill_prompt_template"]; this pins that the
    agent actually uses it, as an addition to the prompt (not a
    replacement -- the JSON schema instructions must still be present, or
    downstream parsing/persistence breaks)."""
    llm_output = json.dumps({"rci": {}, "rca": {"confidence": 0.5}, "hypothesis": {}})
    mock_call_llm = AsyncMock(return_value=(llm_output, _USAGE))
    with patch.object(agent, "_call_llm", new=mock_call_llm):
        await agent.handle_task("t1", _message({
            "alert": {"title": "x"}, "evidence": {"summary": "present"},
            "skill_prompt_template": "Focus specifically on memory growth patterns and heap usage over time.",
        }))

    prompt = mock_call_llm.await_args.args[0]
    assert "Focus specifically on memory growth patterns and heap usage over time." in prompt
    assert "## Additional Skill-Specific Instructions" in prompt
    # The output-format contract must still be intact -- a custom skill
    # instruction must not crowd out the schema the rest of the pipeline
    # depends on to parse this agent's response.
    assert 'Respond with ONLY valid JSON containing three sections:' in prompt


@pytest.mark.asyncio
async def test_handle_task_without_skill_prompt_template_omits_section(agent):
    llm_output = json.dumps({"rci": {}, "rca": {"confidence": 0.5}, "hypothesis": {}})
    mock_call_llm = AsyncMock(return_value=(llm_output, _USAGE))
    with patch.object(agent, "_call_llm", new=mock_call_llm):
        await agent.handle_task("t1", _message({
            "alert": {"title": "x"}, "evidence": {"summary": "present"},
        }))

    prompt = mock_call_llm.await_args.args[0]
    assert "Additional Skill-Specific Instructions" not in prompt


@pytest.mark.asyncio
async def test_handle_task_llm_failure_falls_back_and_records_metric(agent):
    before = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="rca_agent", error_type="llm_call_failed"
    )._value.get()

    with patch.object(agent, "_call_llm", new=AsyncMock(side_effect=RuntimeError("down"))):
        task = await agent.handle_task("t1", _message({
            "alert": {"title": "x"}, "evidence": {"summary": "present"},
        }))

    assert task.status == TaskStatus.COMPLETED
    rca_data = next(a for a in task.artifacts if a.name == "rca").parts[0].data
    assert rca_data["confidence"] == 0.0

    after = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="rca_agent", error_type="llm_call_failed"
    )._value.get()
    assert after == before + 1


@pytest.mark.asyncio
async def test_call_llm_uses_orchestrator_provided_llm_config_override(agent, monkeypatch):
    """The orchestrator resolves org-default / per-agent-override LLM
    settings (see Orchestrator.handle_incident) and sends them as
    input_data["llm_config"] on every task -- this agent must actually use
    them for the call, not just the constructor's baked-in defaults, or the
    LLM & Tokens settings UI has no live effect."""
    captured: dict[str, Any] = {}

    class _FakeResponse:
        def raise_for_status(self):
            pass

        def json(self):
            return {"choices": [{"message": {"content": "ok"}}]}

    class _FakeClient:
        async def __aenter__(self):
            return self

        async def __aexit__(self, *a):
            return False

        async def post(self, url, headers=None, json=None):
            captured["url"] = url
            captured["headers"] = headers
            captured["json"] = json
            return _FakeResponse()

    import app.agent as agent_module
    monkeypatch.setattr(agent_module.httpx, "AsyncClient", lambda **kw: _FakeClient())

    await agent._call_llm("prompt", {
        "api_base": "http://openrouter:9999/v1",
        "api_key": "override-key",
        "model": "anthropic/claude-sonnet-4-6",
        "temperature": 0.5,
    })

    assert captured["url"] == "http://openrouter:9999/v1/chat/completions"
    assert captured["headers"]["Authorization"] == "Bearer override-key"
    assert captured["json"]["model"] == "anthropic/claude-sonnet-4-6"
    assert captured["json"]["temperature"] == 0.5


@pytest.mark.asyncio
async def test_call_llm_falls_back_to_instance_defaults_without_llm_config(agent, monkeypatch):
    captured: dict[str, Any] = {}

    class _FakeResponse:
        def raise_for_status(self):
            pass

        def json(self):
            return {"choices": [{"message": {"content": "ok"}}]}

    class _FakeClient:
        async def __aenter__(self):
            return self

        async def __aexit__(self, *a):
            return False

        async def post(self, url, headers=None, json=None):
            captured["url"] = url
            captured["json"] = json
            return _FakeResponse()

    import app.agent as agent_module
    monkeypatch.setattr(agent_module.httpx, "AsyncClient", lambda **kw: _FakeClient())

    await agent._call_llm("prompt")  # no llm_config at all

    assert captured["url"] == "http://lm-studio:1234/v1/chat/completions"
    assert captured["json"]["model"] == "m"  # the fixture's constructor default
