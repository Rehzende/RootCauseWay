"""HTTP client for calling the Go backend API."""

from __future__ import annotations

from typing import Any
from uuid import UUID

import httpx

from app.models.api import (
    Agent,
    CreateAgentRunRequest,
    IncidentEvidenceCreate,
    IncidentEventCreate,
    IncidentUpdate,
    UpdateAgentRunRequest,
)


class BackendClient:
    def __init__(self, base_url: str, http_client: httpx.AsyncClient | None = None):
        self.base_url = base_url.rstrip("/")
        self._client = http_client or httpx.AsyncClient(base_url=self.base_url, timeout=30.0)

    async def close(self) -> None:
        await self._client.aclose()

    async def get_agents(self, org_id: UUID, agent_type: str | None = None) -> list[Agent]:
        params: dict[str, str] = {}
        if agent_type:
            params["type"] = agent_type
        resp = await self._client.get(
            "/internal/agents",
            params=params,
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        items = data.get("data", data) if isinstance(data, dict) else data
        return [Agent.model_validate(item) for item in items]

    async def get_incident(self, incident_id: UUID, org_id: UUID) -> dict:
        # X-Org-ID is required here, not optional: GetIncident went through
        # verifyIncidentOwnership as part of the multi-tenant isolation fix
        # (incident.OrgID != getOrgID(c) -> 404), and the internal route's
        # own middleware only ever sets org_id in the gin context when this
        # header is present. Every one of the four internal incident calls
        # below was missing it until this fix -- found live, the hard way:
        # every agent-service write to an incident (evidence, events,
        # get-before-re-analyze) was silently 404ing since that security
        # fix deployed, with no metric or alert catching it because
        # httpx.HTTPStatusError from raise_for_status() was caught by each
        # caller's own generic `except Exception: logger.warning(...)`.
        resp = await self._client.get(
            f"/internal/incidents/{incident_id}",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def update_incident(self, incident_id: UUID, update: IncidentUpdate, org_id: UUID) -> dict:
        resp = await self._client.patch(
            f"/internal/incidents/{incident_id}",
            json=update.model_dump(exclude_none=True),
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        return resp.json()

    async def add_incident_event(self, incident_id: UUID, event: IncidentEventCreate, org_id: UUID) -> dict:
        resp = await self._client.post(
            f"/internal/incidents/{incident_id}/events",
            json=event.model_dump(exclude_none=True),
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        return resp.json()

    async def add_incident_evidence(self, incident_id: UUID, evidence: IncidentEvidenceCreate, org_id: UUID) -> dict:
        resp = await self._client.post(
            f"/internal/incidents/{incident_id}/evidence",
            json=evidence.model_dump(exclude_none=True),
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        return resp.json()

    async def create_agent_run(self, incident_id: UUID, data: CreateAgentRunRequest) -> dict:
        resp = await self._client.post(
            f"/internal/incidents/{incident_id}/runs",
            json=data.model_dump(exclude_none=True, mode="json"),
        )
        resp.raise_for_status()
        return resp.json()

    async def update_agent_run(self, incident_id: UUID, run_id: UUID, data: UpdateAgentRunRequest) -> dict:
        resp = await self._client.patch(
            f"/internal/incidents/{incident_id}/runs/{run_id}",
            json=data.model_dump(exclude_none=True, mode="json"),
        )
        resp.raise_for_status()
        return resp.json()

    async def get_rci(self, incident_id: UUID) -> dict | None:
        resp = await self._client.get(f"/internal/incidents/{incident_id}/rci")
        if resp.status_code == 404:
            return None
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def get_rca(self, incident_id: UUID) -> dict | None:
        resp = await self._client.get(f"/internal/incidents/{incident_id}/rca")
        if resp.status_code == 404:
            return None
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def create_rci(self, incident_id: UUID, data: dict) -> dict:
        resp = await self._client.post(
            f"/internal/incidents/{incident_id}/rci",
            json=data,
        )
        resp.raise_for_status()
        return resp.json()

    async def create_rca(self, incident_id: UUID, data: dict) -> dict:
        resp = await self._client.post(
            f"/internal/incidents/{incident_id}/rca",
            json=data,
        )
        resp.raise_for_status()
        return resp.json()

    async def create_postmortem(self, incident_id: UUID, data: dict) -> dict:
        resp = await self._client.post(
            f"/internal/incidents/{incident_id}/postmortem",
            json=data,
        )
        resp.raise_for_status()
        return resp.json()

    # --- Software catalog ---

    async def get_software(self, software_id: str | UUID, org_id: UUID) -> dict:
        resp = await self._client.get(
            f"/internal/software/{software_id}",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def get_software_observability(self, software_id: str | UUID) -> list[dict]:
        """Get observability sources configured for a software entry."""
        resp = await self._client.get(f"/internal/software/{software_id}/observability")
        resp.raise_for_status()
        data = resp.json()
        items = data.get("data", data) if isinstance(data, dict) else data
        return items if isinstance(items, list) else []

    # --- A2A agents ---

    async def list_a2a_agents(self, org_id: UUID) -> list[dict]:
        resp = await self._client.get(
            "/internal/a2a/agents",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        items = data.get("data", data) if isinstance(data, dict) else data
        return items

    async def create_a2a_task(self, incident_id: UUID, data: dict) -> dict:
        resp = await self._client.post(
            f"/internal/incidents/{incident_id}/a2a-tasks",
            json=data,
        )
        resp.raise_for_status()
        return resp.json()

    async def update_a2a_task(self, incident_id: UUID, task_id: str, data: dict) -> dict:
        resp = await self._client.patch(
            f"/internal/incidents/{incident_id}/a2a-tasks/{task_id}",
            json=data,
        )
        resp.raise_for_status()
        return resp.json()

    async def create_orchestrator_decision(self, incident_id: UUID, data: dict) -> dict:
        resp = await self._client.post(
            f"/internal/incidents/{incident_id}/orchestrator/decisions",
            json=data,
        )
        resp.raise_for_status()
        return resp.json()

    # --- Skills ---

    async def list_skills(self, org_id: UUID) -> list[dict]:
        """List available skills with their linked agents."""
        resp = await self._client.get(
            "/internal/skills",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        items = data.get("data", data) if isinstance(data, dict) else data
        return items

    # --- Credentials ---

    async def get_software_credentials(self, software_id: str | UUID, org_id: UUID) -> list[dict]:
        """Get resource credentials configured for a software entry."""
        resp = await self._client.get(
            f"/internal/software/{software_id}/credentials",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        items = data.get("data", data) if isinstance(data, dict) else data
        return items

    async def request_credential_lease(self, data: dict) -> dict:
        """Request a JIT credential lease from the backend."""
        resp = await self._client.post(
            "/internal/credentials/lease",
            json=data,
        )
        resp.raise_for_status()
        return resp.json()

    async def revoke_credential_lease(self, lease_id: UUID) -> dict:
        """Revoke an active credential lease."""
        resp = await self._client.post(
            f"/internal/credentials/lease/{lease_id}/revoke",
        )
        resp.raise_for_status()
        return resp.json()

    async def evaluate_access_policy(
        self, agent_id: UUID, skill_id: str, resource_type: str
    ) -> dict:
        """Evaluate whether an agent+skill combination can access a resource type."""
        resp = await self._client.post(
            "/internal/access-policies/evaluate",
            json={
                "agent_id": str(agent_id),
                "skill_id": skill_id,
                "resource_type": resource_type,
            },
        )
        resp.raise_for_status()
        return resp.json()

    # --- Feedback ---

    async def create_feedback(self, incident_id: UUID, data: dict) -> dict:
        resp = await self._client.post(
            f"/internal/incidents/{incident_id}/feedback", json=data,
        )
        resp.raise_for_status()
        return resp.json()

    async def list_feedback(self, incident_id: UUID) -> list[dict]:
        resp = await self._client.get(f"/internal/incidents/{incident_id}/feedback")
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    # --- Knowledge Base ---

    async def search_knowledge_base(
        self, org_id: UUID, software_id: str, query: str, limit: int = 5,
    ) -> list[dict]:
        resp = await self._client.get(
            "/internal/knowledge-base/search",
            params={"software_id": software_id, "query": query, "limit": limit},
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def create_knowledge_entry(self, org_id: UUID, data: dict) -> dict:
        resp = await self._client.post(
            "/internal/knowledge-base",
            json=data,
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        return resp.json()

    async def increment_kb_references(self, entry_id: UUID) -> dict:
        resp = await self._client.post(
            f"/internal/knowledge-base/{entry_id}/increment-references",
        )
        resp.raise_for_status()
        return resp.json()

    # --- Correlation ---

    async def list_correlation_rules(self, org_id: UUID) -> list[dict]:
        resp = await self._client.get(
            "/internal/correlation-rules",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def check_correlation(
        self, org_id: UUID, software_id: str, alert: dict, time_window_seconds: int,
        exclude_incident_id: UUID | None = None,
    ) -> dict | None:
        body: dict[str, Any] = {
            "software_id": software_id,
            "alert": alert,
            "time_window_seconds": time_window_seconds,
        }
        if exclude_incident_id is not None:
            body["exclude_incident_id"] = str(exclude_incident_id)
        resp = await self._client.post(
            "/internal/correlation/check",
            json=body,
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        return data if data.get("incident_id") else None

    # --- Correlation enrichment (dependency graph + fingerprint dedup) ---

    async def get_software_dependency_graph(self, software_id: str | UUID) -> dict | None:
        """Get the upstream (declared dependencies) and downstream (dependents)
        services for a software catalog entry, for dependency-graph cascade
        correlation. Returns None if the software entry can't be found."""
        resp = await self._client.get(f"/internal/software/{software_id}/dependency-graph")
        if resp.status_code == 404:
            return None
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def list_open_incidents_by_software(
        self, org_id: UUID, software_ids: list[str], time_window_seconds: int,
    ) -> list[dict]:
        """List open incidents on any of the given software ids, created within
        the recency window. Used for dependency-graph cascade correlation."""
        if not software_ids:
            return []
        params = [("software_id", sid) for sid in software_ids]
        params.append(("window_seconds", str(time_window_seconds)))
        resp = await self._client.get(
            "/internal/incidents/open-by-software",
            params=params,
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        items = data.get("data", data) if isinstance(data, dict) else data
        return items if isinstance(items, list) else []

    async def find_incident_by_fingerprint(
        self, org_id: UUID, fingerprint: str, window_seconds: int,
        exclude_incident_id: UUID | None = None,
    ) -> dict | None:
        """Find the most recent incident whose alert fingerprint matches, within
        a recency window. Used to dedup literally-repeated alerts."""
        params: dict[str, Any] = {"fingerprint": fingerprint, "window_seconds": window_seconds}
        if exclude_incident_id is not None:
            params["exclude_incident_id"] = str(exclude_incident_id)
        resp = await self._client.get(
            "/internal/incidents/by-fingerprint",
            params=params,
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        item = data.get("data", data) if isinstance(data, dict) else data
        return item if item else None

    # --- Notifications ---

    async def list_notification_channels(self, org_id: UUID) -> list[dict]:
        resp = await self._client.get(
            "/internal/notification-channels",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def list_escalation_policies(self, org_id: UUID) -> list[dict]:
        resp = await self._client.get(
            "/internal/escalation-policies",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def create_notification_log(self, org_id: UUID, data: dict) -> dict:
        resp = await self._client.post(
            "/internal/notification-logs",
            json=data,
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        return resp.json()

    # --- Runbooks ---

    async def list_runbooks(self, org_id: UUID) -> list[dict]:
        resp = await self._client.get(
            "/internal/runbooks",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def get_runbook(self, runbook_id: str | UUID, org_id: UUID) -> dict:
        resp = await self._client.get(
            f"/internal/runbooks/{runbook_id}",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        return resp.json()

    async def list_runbook_steps(self, runbook_id: str | UUID, org_id: UUID) -> list[dict]:
        resp = await self._client.get(
            f"/internal/runbooks/{runbook_id}/steps",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def get_runbook_execution(self, execution_id: UUID, org_id: UUID) -> dict:
        resp = await self._client.get(
            f"/internal/runbook-executions/{execution_id}",
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        return resp.json()

    async def update_runbook_execution(self, execution_id: UUID, data: dict, org_id: UUID) -> dict:
        resp = await self._client.patch(
            f"/internal/runbook-executions/{execution_id}", json=data,
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        return resp.json()

    async def complete_runbook_execution_step(
        self,
        execution_id: UUID,
        step_id: str,
        org_id: UUID,
        status: str | None = None,
        output: dict | None = None,
    ) -> dict:
        """Advance a runbook execution past one step. A human "Mark
        Complete" click in the UI hits this same endpoint with no body
        (status defaults to "completed" server-side); the automation loop
        (RunbookExecutor.run_automated_steps) passes an explicit status
        ("completed" or "failed") plus the step's result payload."""
        body: dict[str, Any] = {}
        if status:
            body["status"] = status
        if output is not None:
            body["output"] = output
        resp = await self._client.post(
            f"/internal/runbook-executions/{execution_id}/steps/{step_id}/complete",
            json=body,
            headers={"X-Org-ID": str(org_id)},
        )
        resp.raise_for_status()
        return resp.json()

    # --- Change Events ---

    async def create_change_event(self, data: dict) -> dict:
        resp = await self._client.post("/internal/change-events", json=data)
        resp.raise_for_status()
        return resp.json()

    async def list_recent_changes(
        self, software_id: str, hours: int = 24,
    ) -> list[dict]:
        resp = await self._client.get(
            "/internal/change-events",
            params={"software_id": software_id, "hours": hours},
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    # --- Similar Incidents ---

    async def create_similar_incident(self, incident_id: UUID, data: dict) -> dict:
        resp = await self._client.post(
            f"/internal/incidents/{incident_id}/similar", json=data,
        )
        resp.raise_for_status()
        return resp.json()

    async def list_similar_incidents(self, incident_id: UUID) -> list[dict]:
        resp = await self._client.get(
            f"/internal/incidents/{incident_id}/similar",
        )
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    # --- War Room ---

    async def get_warroom(self, meeting_id: str | UUID) -> dict:
        """Fetch a war room meeting (including raw_transcript) by meeting ID."""
        resp = await self._client.get(f"/internal/warroom/{meeting_id}")
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def update_warroom_summary(
        self, meeting_id: str | UUID, summary: dict, participants: list[dict],
    ) -> dict:
        """Write back the LLM-generated summary + participant list for a
        war room meeting once the transcript has been summarized."""
        resp = await self._client.patch(
            f"/internal/warroom/{meeting_id}/summary",
            json={"summary": summary, "participants": participants},
        )
        resp.raise_for_status()
        return resp.json()

    # --- Pipeline HITL gate ---

    async def get_organization_settings(self, org_id: UUID) -> dict:
        """Fetch org-level settings: pipeline_hitl_gate_enabled (whether to
        pause before postmortem for human approval) and the org's default
        LLM provider/model (default_llm_provider_type/base_url/model/
        api_key_ref, set via the LLM & Tokens settings UI, migration
        023_llm_settings). See
        backend/internal/handlers/pipeline_gate_handlers.go:GetOrgSettingsInternal.
        """
        resp = await self._client.get(f"/internal/organizations/{org_id}/settings")
        resp.raise_for_status()
        data = resp.json()
        return data.get("data", data) if isinstance(data, dict) else data

    async def mark_awaiting_approval(self, incident_id: UUID, stage: str) -> dict:
        """Mark an incident as paused, awaiting human approval of `stage`.

        See backend/internal/handlers/pipeline_gate_handlers.go:MarkAwaitingApprovalInternal.
        The human resumes the run via POST /incidents/{id}/approve-stage,
        which publishes a pipeline.stage_approved event consumed by
        app/orchestrator/resume_listener.py.
        """
        resp = await self._client.post(
            f"/internal/incidents/{incident_id}/awaiting-approval",
            json={"stage": stage},
        )
        resp.raise_for_status()
        return resp.json()
