"""Regression test for Orchestrator._discover_skills' agent-less-skill guard.

A platform audit found that PgSkillRepository.List returned skills with no
agent info at all (bare `SELECT * FROM skills`, no join to agent_skills/
a2a_agents), so every skill_id the LLM tried to dispatch had no ground-truth
agent_url to resolve to. Worse: _discover_skills returning *any* non-empty
list -- even a single unlinked skill, the exact state right after a user
creates a custom skill via the UI before linking an agent -- permanently
bypassed the working agent-card-discovery fallback in handle_incident() for
the rest of that org's incidents (that fallback only runs when
_discover_skills returns empty). Fixed on the Go side (backend now joins
agent_skills/a2a_agents and populates each skill's "agents" field) and
here, defensively: a skill with no "agents" is dropped rather than trusted.
"""

from __future__ import annotations

import uuid
from unittest.mock import AsyncMock

import pytest

from app.orchestrator.orchestrator import Orchestrator


@pytest.fixture
def orchestrator():
    backend = AsyncMock()
    a2a = AsyncMock()
    llm_call = AsyncMock()
    return Orchestrator(backend, a2a, llm_call), backend


@pytest.mark.asyncio
async def test_discover_skills_drops_skills_with_no_linked_agent(orchestrator):
    orch, backend = orchestrator
    org_id = uuid.uuid4()
    backend.list_skills.return_value = [
        {"id": "triage", "agents": [{"id": "a1", "url": "http://triage-agent:8090"}]},
        {"id": "orphan-skill", "agents": []},
        {"id": "another-orphan"},  # "agents" key missing entirely
    ]

    result = await orch._discover_skills(org_id)

    assert [s["id"] for s in result] == ["triage"]


@pytest.mark.asyncio
async def test_discover_skills_returns_empty_when_all_skills_are_agentless(orchestrator):
    """This is the exact state right after a user creates their first
    custom skill via the UI, before linking any agent -- must return []
    so handle_incident()'s fallback to agent-card discovery still runs,
    not a list of one unusable skill that silently blocks it."""
    orch, backend = orchestrator
    org_id = uuid.uuid4()
    backend.list_skills.return_value = [{"id": "brand-new-skill", "agents": []}]

    result = await orch._discover_skills(org_id)

    assert result == []


@pytest.mark.asyncio
async def test_discover_skills_keeps_all_when_all_linked(orchestrator):
    orch, backend = orchestrator
    org_id = uuid.uuid4()
    backend.list_skills.return_value = [
        {"id": "triage", "agents": [{"id": "a1", "url": "http://triage-agent:8090"}]},
        {"id": "rca", "agents": [{"id": "a2", "url": "http://rca-agent:8092"}]},
    ]

    result = await orch._discover_skills(org_id)

    assert [s["id"] for s in result] == ["triage", "rca"]


@pytest.mark.asyncio
async def test_discover_skills_drops_disabled_skills(orchestrator):
    """A platform audit found the enable/disable toggle persisted correctly
    but had zero effect on dispatch: PgSkillRepository.List doesn't filter
    by `enabled` (the admin UI needs to keep listing/re-enabling disabled
    skills), so a disabled skill kept being offered to the LLM and
    dispatched exactly like an enabled one."""
    orch, backend = orchestrator
    org_id = uuid.uuid4()
    backend.list_skills.return_value = [
        {"id": "triage", "enabled": True, "agents": [{"id": "a1", "url": "http://triage-agent:8090"}]},
        {"id": "disabled-skill", "enabled": False, "agents": [{"id": "a2", "url": "http://x:8092"}]},
    ]

    result = await orch._discover_skills(org_id)

    assert [s["id"] for s in result] == ["triage"]


@pytest.mark.asyncio
async def test_discover_skills_treats_missing_enabled_field_as_enabled(orchestrator):
    """Callers/fixtures that omit "enabled" entirely (e.g. older test
    doubles, or a backend response shape that predates the field) must
    still be dispatchable -- only an explicit False excludes a skill."""
    orch, backend = orchestrator
    org_id = uuid.uuid4()
    backend.list_skills.return_value = [
        {"id": "triage", "agents": [{"id": "a1", "url": "http://triage-agent:8090"}]},
    ]

    result = await orch._discover_skills(org_id)

    assert [s["id"] for s in result] == ["triage"]


@pytest.mark.asyncio
async def test_discover_skills_returns_empty_on_backend_error(orchestrator):
    orch, backend = orchestrator
    org_id = uuid.uuid4()
    backend.list_skills.side_effect = RuntimeError("backend unavailable")

    result = await orch._discover_skills(org_id)

    assert result == []
