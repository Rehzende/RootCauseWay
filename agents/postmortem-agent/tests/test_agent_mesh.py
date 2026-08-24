"""Tests for PostmortemAgent's A2A mesh call to rca-agent (generating a
fresh RCA when the orchestrator triggered postmortem without one -- the
incident.resolved path, see agent-service/app/workers/alert_worker.py)."""

from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest

from app.a2a.models import Artifact, DataPart, Message, Role, Task, TaskStatus
from app.agent import PostmortemAgent


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
    return PostmortemAgent(
        api_base="http://lm-studio:1234/v1", api_key="x", model="m",
        rca_agent_url="http://rca-agent:8092",
    )


@pytest.mark.asyncio
async def test_fetches_rca_when_missing(agent):
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"title":"x"}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock(
             return_value=_task({
                 "rci": {"investigation_summary": "fetched"},
                 "rca": {"root_cause_summary": "pool exhaustion"},
             })
         )) as mock_send:
        await agent.handle_task("t1", _message({"alert": {"title": "x"}}))

    mock_send.assert_awaited_once()
    assert mock_send.await_args.args[0] == "http://rca-agent:8092"


@pytest.mark.asyncio
async def test_does_not_fetch_rca_when_already_present(agent):
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"title":"x"}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock()) as mock_send:
        await agent.handle_task("t1", _message({
            "alert": {"title": "x"},
            "rca": {"root_cause_summary": "already have this"},
        }))

    mock_send.assert_not_awaited()


@pytest.mark.asyncio
async def test_mesh_call_failure_does_not_crash_handle_task(agent):
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"title":"x"}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock(side_effect=RuntimeError("peer down"))):
        task = await agent.handle_task("t1", _message({"alert": {"title": "x"}}))

    assert task.status == TaskStatus.COMPLETED


@pytest.mark.asyncio
async def test_no_mesh_call_when_peer_url_not_configured():
    agent = PostmortemAgent(api_base="http://lm-studio:1234/v1", api_key="x", model="m")
    with patch.object(agent, "_call_llm", new=AsyncMock(return_value=('{"title":"x"}', {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}))), \
         patch.object(agent._peer, "send_task", new=AsyncMock()) as mock_send:
        await agent.handle_task("t1", _message({"alert": {"title": "x"}}))

    mock_send.assert_not_awaited()
