"""Tests for BackendClient using httpx mock (respx)."""

import uuid

import httpx
import pytest
import respx

from app.models.api import IncidentEvidenceCreate, IncidentEventCreate, IncidentUpdate
from app.services.backend_client import BackendClient


@pytest.fixture
def org_id():
    return uuid.uuid4()


@pytest.fixture
def base_url():
    return "http://testserver/api/v1"


@pytest.fixture
def client(base_url):
    return BackendClient(base_url, http_client=httpx.AsyncClient(base_url=base_url))


@pytest.mark.asyncio
class TestGetAgents:
    @respx.mock
    async def test_get_agents_returns_list(self, client, org_id):
        agent_data = {
            "id": str(uuid.uuid4()),
            "org_id": str(org_id),
            "name": "Triage Agent",
            "type": "triage",
            "enabled": True,
        }
        respx.get(f"{client.base_url}/internal/agents").mock(
            return_value=httpx.Response(200, json={"data": [agent_data], "total": 1, "page": 1, "per_page": 20})
        )
        agents = await client.get_agents(org_id)
        assert len(agents) == 1
        assert agents[0].name == "Triage Agent"

    @respx.mock
    async def test_get_agents_with_type_filter(self, client, org_id):
        respx.get(f"{client.base_url}/internal/agents", params={"type": "triage"}).mock(
            return_value=httpx.Response(200, json={"data": [], "total": 0, "page": 1, "per_page": 20})
        )
        agents = await client.get_agents(org_id, agent_type="triage")
        assert agents == []


@pytest.mark.asyncio
class TestGetIncident:
    @respx.mock
    async def test_get_incident_sends_org_id_header(self, client, org_id):
        incident_id = uuid.uuid4()
        route = respx.get(f"{client.base_url}/internal/incidents/{incident_id}").mock(
            return_value=httpx.Response(200, json={"id": str(incident_id), "title": "x"})
        )
        result = await client.get_incident(incident_id, org_id)
        assert result["title"] == "x"
        assert route.calls.last.request.headers["X-Org-ID"] == str(org_id)


@pytest.mark.asyncio
class TestUpdateIncident:
    @respx.mock
    async def test_update_incident(self, client, org_id):
        incident_id = uuid.uuid4()
        route = respx.patch(f"{client.base_url}/internal/incidents/{incident_id}").mock(
            return_value=httpx.Response(200, json={"id": str(incident_id), "status": "investigating"})
        )
        result = await client.update_incident(incident_id, IncidentUpdate(status="investigating"), org_id)
        assert result["status"] == "investigating"
        assert route.calls.last.request.headers["X-Org-ID"] == str(org_id)


@pytest.mark.asyncio
class TestAddIncidentEvent:
    @respx.mock
    async def test_add_event(self, client, org_id):
        incident_id = uuid.uuid4()
        route = respx.post(f"{client.base_url}/internal/incidents/{incident_id}/events").mock(
            return_value=httpx.Response(201, json={"id": str(uuid.uuid4()), "type": "agent_action"})
        )
        result = await client.add_incident_event(
            incident_id, IncidentEventCreate(type="agent_action", data={"step": "triage"}), org_id,
        )
        assert result["type"] == "agent_action"
        assert route.calls.last.request.headers["X-Org-ID"] == str(org_id)


@pytest.mark.asyncio
class TestAddIncidentEvidence:
    @respx.mock
    async def test_add_evidence(self, client, org_id):
        incident_id = uuid.uuid4()
        # Regression: this call used to send no X-Org-ID at all, which
        # 404'd against the internal route's own ownership check
        # (incident.OrgID != getOrgID(c), see verifyIncidentOwnership in
        # incident_handlers.go) -- silently, since add_incident_evidence's
        # only caller-side handling was a generic `except Exception:
        # logger.warning(...)`. Every incident's evidence trail (triage/
        # evidence/rca/postmortem output, MLflow trace link) was lost from
        # the moment that ownership check deployed until this fix.
        route = respx.post(f"{client.base_url}/internal/incidents/{incident_id}/evidence").mock(
            return_value=httpx.Response(201, json={"id": str(uuid.uuid4()), "type": "agent_output"})
        )
        result = await client.add_incident_evidence(
            incident_id,
            IncidentEvidenceCreate(type="agent_output", title="Test", content={"data": "ok"}, source="test"),
            org_id,
        )
        assert result["type"] == "agent_output"
        assert route.calls.last.request.headers["X-Org-ID"] == str(org_id)

    @respx.mock
    async def test_http_error_raises(self, client, org_id):
        incident_id = uuid.uuid4()
        respx.post(f"{client.base_url}/internal/incidents/{incident_id}/evidence").mock(
            return_value=httpx.Response(500, json={"error": "internal"})
        )
        with pytest.raises(httpx.HTTPStatusError):
            await client.add_incident_evidence(
                incident_id,
                IncidentEvidenceCreate(type="log", title="T", content={}, source="s"),
                org_id,
            )


@pytest.mark.asyncio
class TestRunbookAutomationClient:
    """The runbook automation call sites -- confirmed live these had the
    same missing-X-Org-ID-header bug as get_incident et al earlier this
    session, on top of /internal/runbooks* not being registered on the Go
    side at all."""

    @respx.mock
    async def test_get_runbook_sends_org_id_header(self, client, org_id):
        runbook_id = uuid.uuid4()
        route = respx.get(f"{client.base_url}/internal/runbooks/{runbook_id}").mock(
            return_value=httpx.Response(200, json={"id": str(runbook_id), "software_id": "sw-1"})
        )
        result = await client.get_runbook(runbook_id, org_id)
        assert result["software_id"] == "sw-1"
        assert route.calls.last.request.headers["X-Org-ID"] == str(org_id)

    @respx.mock
    async def test_list_runbook_steps_sends_org_id_header(self, client, org_id):
        runbook_id = uuid.uuid4()
        route = respx.get(f"{client.base_url}/internal/runbooks/{runbook_id}/steps").mock(
            return_value=httpx.Response(200, json=[{"id": "s1", "step_type": "automated"}])
        )
        steps = await client.list_runbook_steps(runbook_id, org_id)
        assert len(steps) == 1
        assert route.calls.last.request.headers["X-Org-ID"] == str(org_id)

    @respx.mock
    async def test_get_runbook_execution_sends_org_id_header(self, client, org_id):
        execution_id = uuid.uuid4()
        route = respx.get(f"{client.base_url}/internal/runbook-executions/{execution_id}").mock(
            return_value=httpx.Response(200, json={"id": str(execution_id), "status": "running"})
        )
        result = await client.get_runbook_execution(execution_id, org_id)
        assert result["status"] == "running"
        assert route.calls.last.request.headers["X-Org-ID"] == str(org_id)

    @respx.mock
    async def test_update_runbook_execution_sends_org_id_header(self, client, org_id):
        execution_id = uuid.uuid4()
        route = respx.patch(f"{client.base_url}/internal/runbook-executions/{execution_id}").mock(
            return_value=httpx.Response(200, json={"id": str(execution_id)})
        )
        await client.update_runbook_execution(execution_id, {"status": "completed"}, org_id)
        assert route.calls.last.request.headers["X-Org-ID"] == str(org_id)

    @respx.mock
    async def test_complete_runbook_execution_step_sends_status_and_output(self, client, org_id):
        execution_id = uuid.uuid4()
        route = respx.post(
            f"{client.base_url}/internal/runbook-executions/{execution_id}/steps/s1/complete"
        ).mock(return_value=httpx.Response(200, json={"id": str(execution_id), "status": "running"}))

        await client.complete_runbook_execution_step(
            execution_id, "s1", org_id, status="failed", output={"error": "timeout"},
        )

        assert route.calls.last.request.headers["X-Org-ID"] == str(org_id)
        import json as _json
        sent = _json.loads(route.calls.last.request.content)
        assert sent == {"status": "failed", "output": {"error": "timeout"}}

    @respx.mock
    async def test_complete_runbook_execution_step_no_args_sends_empty_body(self, client, org_id):
        """Matches the UI's plain "Mark Complete" click -- no status/output
        at all, letting the handler default to status=completed."""
        execution_id = uuid.uuid4()
        route = respx.post(
            f"{client.base_url}/internal/runbook-executions/{execution_id}/steps/s1/complete"
        ).mock(return_value=httpx.Response(200, json={"id": str(execution_id)}))

        await client.complete_runbook_execution_step(execution_id, "s1", org_id)

        import json as _json
        sent = _json.loads(route.calls.last.request.content)
        assert sent == {}


@pytest.mark.asyncio
class TestCorrelationExcludeIncidentId:
    """exclude_incident_id must reach the wire on both correlation-check
    entrypoints -- see CorrelationEngine.check_correlation's docstring for
    the live-found self-correlation bug this closes."""

    @respx.mock
    async def test_check_correlation_includes_exclude_incident_id_when_given(self, client, org_id):
        software_id = str(uuid.uuid4())
        exclude_id = uuid.uuid4()
        route = respx.post(f"{client.base_url}/internal/correlation/check").mock(
            return_value=httpx.Response(200, json={"correlated": False})
        )

        await client.check_correlation(
            org_id=org_id, software_id=software_id, alert={"title": "x"},
            time_window_seconds=300, exclude_incident_id=exclude_id,
        )

        import json as _json
        sent = _json.loads(route.calls.last.request.content)
        assert sent["exclude_incident_id"] == str(exclude_id)

    @respx.mock
    async def test_check_correlation_omits_exclude_incident_id_when_not_given(self, client, org_id):
        software_id = str(uuid.uuid4())
        route = respx.post(f"{client.base_url}/internal/correlation/check").mock(
            return_value=httpx.Response(200, json={"correlated": False})
        )

        await client.check_correlation(
            org_id=org_id, software_id=software_id, alert={"title": "x"}, time_window_seconds=300,
        )

        import json as _json
        sent = _json.loads(route.calls.last.request.content)
        assert "exclude_incident_id" not in sent

    @respx.mock
    async def test_find_incident_by_fingerprint_includes_exclude_incident_id_when_given(self, client, org_id):
        exclude_id = uuid.uuid4()
        route = respx.get(f"{client.base_url}/internal/incidents/by-fingerprint").mock(
            return_value=httpx.Response(200, json={"data": None})
        )

        await client.find_incident_by_fingerprint(
            org_id=org_id, fingerprint="fp-1", window_seconds=900, exclude_incident_id=exclude_id,
        )

        sent_params = route.calls.last.request.url.params
        assert sent_params["exclude_incident_id"] == str(exclude_id)
