"""ContextBuilder.build_context -- pins the fix for a live-found bug: it
called backend_client.get_software(software_id) with no org_id, and
get_software's internal route requires the X-Org-ID header (same
multi-tenant ownership check as GetIncident/GetSoftware's siblings) --
every call 404'd, silently caught, and every incident's RCA/postmortem ran
on "minimal context" (no repository, cloud resources, databases, SLA, etc)
instead of the software catalog's real enriched data. Confirmed live: a
real incident against Pulso's software row 404'd on this call throughout a
full pipeline run, well after the incident/evidence org_id fixes earlier
this session -- this call site was missed."""

from __future__ import annotations

from unittest.mock import AsyncMock
from uuid import uuid4

import pytest

from app.orchestrator.context_builder import ContextBuilder


@pytest.mark.asyncio
async def test_build_context_passes_org_id_to_get_software():
    org_id = uuid4()
    backend = AsyncMock()
    backend.get_software.return_value = {"id": "sw-1", "name": "Pulso Backend"}

    result = await ContextBuilder().build_context("sw-1", backend, org_id)

    backend.get_software.assert_awaited_once_with("sw-1", org_id)
    assert result["software"]["name"] == "Pulso Backend"


@pytest.mark.asyncio
async def test_build_context_falls_back_to_minimal_context_on_failure():
    org_id = uuid4()
    backend = AsyncMock()
    backend.get_software.side_effect = Exception("404")

    result = await ContextBuilder().build_context("sw-1", backend, org_id)

    assert result["software"] == {"id": "sw-1", "name": "unknown", "description": "", "status": "", "tags": []}
    assert result["cloud_resources"] == []
    assert result["databases"] == []
    assert result["infra_details"] == {}


@pytest.mark.asyncio
async def test_build_context_reads_the_real_software_entry_field_names():
    """A platform audit found build_context reading almost entirely
    nonexistent keys ("repository", "ci_cd_pipeline", "databases", "team",
    "observability", "sla") -- every one of them silently fell through to
    its empty default, for every incident, since this method was written.
    Only "dependencies" happened to match. "infra_details" (a real field)
    was never read at all. This test uses the exact JSON shape the real Go
    backend returns (backend/internal/models/models.go's SoftwareEntry) to
    pin every field actually flowing through now."""
    org_id = uuid4()
    backend = AsyncMock()
    backend.get_software.return_value = {
        "id": "sw-1", "name": "Pulso Backend", "description": "FastAPI backend",
        "status": "active", "tags": ["python", "fastapi"],
        "repository_url": "https://github.com/org/pulso-backend",
        "pipeline_url": "https://ci.example.com/pulso",
        "runbook_url": "https://runbooks.example.com/pulso",
        "dashboard_url": "https://grafana.example.com/pulso",
        "cloud_provider": "on_prem",
        "cloud_resources": [{"type": "kubernetes", "name": "pulse-backend-pods"}],
        "database_info": [{"type": "postgresql", "name": "pulso_db"}],
        "infra_details": {"cluster": "k3s-lx", "namespace": "pulse"},
        "dependencies": ["pulse-postgres", "pulse-redis"],
        "stakeholders": ["marcos"],
        "sre_team": ["marcos"],
        "architects": [],
    }

    ctx = await ContextBuilder().build_context("sw-1", backend, org_id)

    assert ctx["software"] == {
        "id": "sw-1", "name": "Pulso Backend", "description": "FastAPI backend",
        "status": "active", "tags": ["python", "fastapi"],
    }
    assert ctx["repository_url"] == "https://github.com/org/pulso-backend"
    assert ctx["pipeline_url"] == "https://ci.example.com/pulso"
    assert ctx["runbook_url"] == "https://runbooks.example.com/pulso"
    assert ctx["dashboard_url"] == "https://grafana.example.com/pulso"
    assert ctx["cloud_provider"] == "on_prem"
    assert ctx["cloud_resources"] == [{"type": "kubernetes", "name": "pulse-backend-pods"}]
    assert ctx["databases"] == [{"type": "postgresql", "name": "pulso_db"}]
    assert ctx["infra_details"] == {"cluster": "k3s-lx", "namespace": "pulse"}
    assert ctx["dependencies"] == ["pulse-postgres", "pulse-redis"]
    assert ctx["team"] == {"stakeholders": ["marcos"], "sre_team": ["marcos"], "architects": []}
