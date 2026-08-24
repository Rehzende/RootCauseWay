"""Regression test for Orchestrator._discover_agents' field-name mapping.

A real incident found every A2A dispatch quietly falling back to the
.env-baked default agent URLs because _discover_agents read
agent_data["url"], but the backend's /internal/a2a/agents response (and the
a2a_agents table it's backed by, see backend/internal/models/models.go)
names that field "endpoint_url". url="" made every self._a2a.discover(url)
call fail (relative "/.well-known/agent.json" with no scheme/host), so
_discover_agents silently returned [] on every run -- dynamic agent
discovery from the registry never worked.
"""

from __future__ import annotations

import uuid
from unittest.mock import AsyncMock

import pytest

from app.a2a.models import AgentCard
from app.orchestrator.orchestrator import Orchestrator


@pytest.fixture
def orchestrator():
    backend = AsyncMock()
    a2a = AsyncMock()
    llm_call = AsyncMock()
    return Orchestrator(backend, a2a, llm_call), backend, a2a


@pytest.mark.asyncio
async def test_discover_agents_uses_endpoint_url_field(orchestrator):
    orch, backend, a2a = orchestrator
    org_id = uuid.uuid4()
    agent_id = str(uuid.uuid4())

    backend.list_a2a_agents.return_value = [
        {
            "id": agent_id,
            "name": "Postmortem Agent",
            "endpoint_url": "http://postmortem-agent:8093",
            "hosting_type": "managed",
        }
    ]
    a2a.discover.return_value = AgentCard(
        name="Postmortem Agent",
        url="http://postmortem-agent:8093",
        version="0.1.0",
    )

    agents = await orch._discover_agents(org_id)

    # The bug made this call with url="" and swallowed the resulting
    # exception, so agents ended up empty -- assert both that discover() saw
    # the real registry URL and that it made it into the returned entry.
    a2a.discover.assert_awaited_once_with("http://postmortem-agent:8093")
    assert len(agents) == 1
    assert agents[0]["url"] == "http://postmortem-agent:8093"
