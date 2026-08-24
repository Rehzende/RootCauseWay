"""Tests for RCAAgent's A2A mesh calls (fetching supplementary data from
evidence-agent / k8s-agent when the orchestrator didn't include it)."""

from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest

from app.a2a.models import Artifact, DataPart, Message, Role, Task, TaskStatus
from app.agent import RCAAgent


def _task(artifacts: dict) -> Task:
    return Task(
        id="t1",
        status=TaskStatus.COMPLETED,
        artifacts=[Artifact(name=k, parts=[DataPart(data=v)]) for k, v in artifacts.items()],
    )


@pytest.fixture
def agent():
    return RCAAgent(
        api_base="http://lm-studio:1234/v1", api_key="x", model="m",
        evidence_agent_url="http://evidence-agent:8091",
        k8s_agent_url="http://rootcauseway-k8s-agent:8094",
    )


def _message(data: dict) -> Message:
    return Message(role=Role.USER, parts=[DataPart(data=data)])


@pytest.mark.asyncio
async def test_fetches_evidence_when_missing(agent):
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"rci":{},"rca":{},"hypothesis":{}}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock(
             return_value=_task({"evidence_result": {"summary": "fetched from peer"}})
         )) as mock_send:
        await agent.handle_task("t1", _message({"alert": {"title": "x"}}))

    mock_send.assert_awaited_once()
    called_url = mock_send.await_args.args[0]
    assert called_url == "http://evidence-agent:8091"


@pytest.mark.asyncio
async def test_does_not_fetch_evidence_when_already_present(agent):
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"rci":{},"rca":{},"hypothesis":{}}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock()) as mock_send:
        await agent.handle_task("t1", _message({
            "alert": {"title": "x"},
            "evidence": {"summary": "already have this"},
        }))

    mock_send.assert_not_awaited()


@pytest.mark.asyncio
async def test_fetches_k8s_diagnostics_when_namespace_present(agent):
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"rci":{},"rca":{},"hypothesis":{}}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock(
             return_value=_task({"k8s_cluster_data": {"pods": ["a"]}})
         )) as mock_send:
        await agent.handle_task("t1", _message({
            "alert": {"title": "x", "service": "svc", "labels": {"namespace": "prod"}},
            "evidence": {"summary": "have evidence but not k8s diagnostics"},
        }))

    mock_send.assert_awaited_once()
    called_url = mock_send.await_args.args[0]
    assert called_url == "http://rootcauseway-k8s-agent:8094"


@pytest.mark.asyncio
async def test_no_mesh_calls_when_namespace_absent(agent):
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"rci":{},"rca":{},"hypothesis":{}}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock()) as mock_send:
        await agent.handle_task("t1", _message({
            "alert": {"title": "x"},
            "evidence": {"summary": "already have this"},
        }))

    mock_send.assert_not_awaited()


@pytest.mark.asyncio
async def test_mesh_call_failure_does_not_crash_handle_task(agent):
    """A down peer must degrade gracefully, not fail the whole RCA."""
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"rci":{},"rca":{},"hypothesis":{}}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock(side_effect=RuntimeError("peer down"))):
        task = await agent.handle_task("t1", _message({"alert": {"title": "x"}}))

    assert task.status == TaskStatus.COMPLETED


@pytest.mark.asyncio
async def test_no_mesh_calls_when_peer_urls_not_configured():
    agent = RCAAgent(api_base="http://lm-studio:1234/v1", api_key="x", model="m")
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"rci":{},"rca":{},"hypothesis":{}}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock()) as mock_send:
        await agent.handle_task("t1", _message({
            "alert": {"title": "x", "labels": {"namespace": "prod"}},
        }))

    mock_send.assert_not_awaited()
