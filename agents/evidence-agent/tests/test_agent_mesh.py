"""Tests for EvidenceAgent's A2A mesh call to k8s-agent (enriching evidence
with live cluster diagnostics when the alert is k8s-related)."""

from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest

from app.a2a.models import Artifact, DataPart, Message, Role, Task, TaskStatus
from app.agent import EvidenceAgent


def _task(artifacts: dict) -> Task:
    return Task(
        id="t1",
        status=TaskStatus.COMPLETED,
        artifacts=[Artifact(name=k, parts=[DataPart(data=v)]) for k, v in artifacts.items()],
    )


def _message(data: dict) -> Message:
    return Message(role=Role.USER, parts=[DataPart(data=data)])


@pytest.fixture
def agent():
    return EvidenceAgent(
        api_base="http://lm-studio:1234/v1", api_key="x", model="m",
        k8s_agent_url="http://rootcauseway-k8s-agent:8094",
    )


@pytest.mark.asyncio
async def test_fetches_k8s_diagnostics_when_namespace_present(agent):
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"summary":"x"}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock(
             return_value=_task({"k8s_cluster_data": {"pods": ["a"]}})
         )) as mock_send:
        task = await agent.handle_task("t1", _message({
            "alert": {"title": "x", "service": "svc", "labels": {"namespace": "prod"}},
        }))

    mock_send.assert_awaited_once()
    assert mock_send.await_args.args[0] == "http://rootcauseway-k8s-agent:8094"
    result_data = task.artifacts[0].parts[0].data
    assert result_data["k8s_diagnostics"] == {"pods": ["a"]}


@pytest.mark.asyncio
async def test_no_mesh_call_when_namespace_absent(agent):
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"summary":"x"}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock()) as mock_send:
        await agent.handle_task("t1", _message({"alert": {"title": "x"}}))

    mock_send.assert_not_awaited()


@pytest.mark.asyncio
async def test_mesh_call_failure_does_not_crash_handle_task(agent):
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"summary":"x"}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock(side_effect=RuntimeError("peer down"))):
        task = await agent.handle_task("t1", _message({
            "alert": {"title": "x", "labels": {"namespace": "prod"}},
        }))

    assert task.status == TaskStatus.COMPLETED
    assert "k8s_diagnostics" not in task.artifacts[0].parts[0].data


@pytest.mark.asyncio
async def test_no_mesh_call_when_peer_url_not_configured():
    agent = EvidenceAgent(api_base="http://lm-studio:1234/v1", api_key="x", model="m")
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"summary":"x"}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock()) as mock_send:
        await agent.handle_task("t1", _message({
            "alert": {"title": "x", "labels": {"namespace": "prod"}},
        }))

    mock_send.assert_not_awaited()
