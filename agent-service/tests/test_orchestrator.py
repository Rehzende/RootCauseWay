"""Tests for the orchestrator and context builder."""

from __future__ import annotations

import asyncio
import json
import time
import uuid
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.a2a.client import A2AClient
from app.a2a.models import (
    AgentCard,
    AgentSkill,
    Artifact,
    DataPart,
    Message,
    Role,
    Task,
    TaskStatus,
    TextPart,
)
from app.orchestrator.context_builder import ContextBuilder
from app.orchestrator.orchestrator import Orchestrator, OrchestratorDecision
from app.orchestrator.resume_listener import ResumeListener
from app.services.backend_client import BackendClient


@pytest.fixture
def mock_backend():
    client = AsyncMock(spec=BackendClient)
    client.get_software = AsyncMock(return_value={
        # Real SoftwareEntry JSON field names (backend/internal/models/
        # models.go) -- a bug found live had build_context reading
        # entirely different (nonexistent) keys, so this fixture used to
        # mirror the *wrong* names rather than catching it.
        "id": "sw-1",
        "name": "payment-service",
        "description": "Payment processing",
        "status": "active",
        "repository_url": "https://github.com/org/payment-service",
        "cloud_resources": [{"type": "kubernetes", "name": "payment-pods"}],
        "database_info": [{"type": "postgresql", "name": "payments_db"}],
        "stakeholders": ["alice"],
        "sre_team": ["bob"],
    })
    client.list_a2a_agents = AsyncMock(return_value=[])
    client.list_skills = AsyncMock(return_value=[])
    client.get_software_credentials = AsyncMock(return_value=[])
    client.evaluate_access_policy = AsyncMock(return_value={"allowed": False})
    client.request_credential_lease = AsyncMock(return_value={})
    client.revoke_credential_lease = AsyncMock(return_value={})
    client.create_orchestrator_decision = AsyncMock(return_value={})
    client.create_a2a_task = AsyncMock(return_value={})
    client.update_a2a_task = AsyncMock(return_value={})
    client.create_rci = AsyncMock(return_value={})
    client.create_rca = AsyncMock(return_value={})
    client.create_postmortem = AsyncMock(return_value={})
    return client


@pytest.fixture
def mock_a2a_client():
    client = AsyncMock(spec=A2AClient)
    client.discover = AsyncMock(return_value=AgentCard(
        name="Test Agent",
        url="http://localhost:8090",
        version="0.1.0",
        skills=[AgentSkill(id="test", name="Test")],
    ))
    client.send_task = AsyncMock(return_value=Task(
        id="task-1",
        status=TaskStatus.COMPLETED,
        artifacts=[
            Artifact(
                name="result",
                parts=[DataPart(data={"severity_assessment": "high", "summary": "test"})],
            )
        ],
    ))
    return client


_MOCK_USAGE = {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}


@pytest.fixture
def mock_llm_call():
    async def llm_call(prompt: str) -> tuple[str, dict]:
        import json

        return json.dumps({
            "severity_assessment": "high",
            "reasoning": "Critical alert requires full pipeline",
            "agent_calls": [
                {
                    "agent_id": "triage",
                    "agent_url": "http://triage-agent:8090",
                    "skill_id": "triage",
                    "priority": 1,
                    "input_summary": "Triage this alert",
                },
                {
                    "agent_id": "evidence",
                    "agent_url": "http://evidence-agent:8091",
                    "skill_id": "evidence-collection",
                    "priority": 2,
                    "input_summary": "Collect evidence",
                },
            ],
        }), _MOCK_USAGE

    return llm_call


class TestContextBuilder:
    async def test_build_context_success(self, mock_backend):
        builder = ContextBuilder()
        ctx = await builder.build_context("sw-1", mock_backend, uuid.uuid4())
        assert ctx["software"]["name"] == "payment-service"
        assert ctx["repository_url"] == "https://github.com/org/payment-service"
        assert len(ctx["cloud_resources"]) == 1
        # "databases" in the returned context maps to the backend's
        # database_info field -- not a "databases" field, which doesn't
        # exist on SoftwareEntry at all.
        assert len(ctx["databases"]) == 1
        assert ctx["databases"][0]["name"] == "payments_db"
        assert ctx["team"]["stakeholders"] == ["alice"]
        assert ctx["team"]["sre_team"] == ["bob"]

    async def test_build_context_fallback(self):
        backend = AsyncMock(spec=BackendClient)
        backend.get_software = AsyncMock(side_effect=Exception("not found"))
        builder = ContextBuilder()
        ctx = await builder.build_context("sw-missing", backend, uuid.uuid4())
        assert ctx["software"]["id"] == "sw-missing"


class TestOrchestratorDecision:
    def test_to_dict(self):
        decision = OrchestratorDecision(
            severity_assessment="high",
            reasoning="test",
            agent_calls=[{"agent_id": "triage", "agent_url": "http://x:8090"}],
        )
        d = decision.to_dict()
        assert d["severity_assessment"] == "high"
        assert len(d["agent_calls"]) == 1


class TestOrchestrator:
    async def test_handle_incident_dispatches_tasks(
        self, mock_backend, mock_a2a_client, mock_llm_call
    ):
        orchestrator = Orchestrator(
            backend_client=mock_backend,
            a2a_client=mock_a2a_client,
            llm_call=mock_llm_call,
        )

        incident_id = uuid.uuid4()
        alert = {
            "title": "High CPU on payment-service",
            "severity": "high",
            "description": "CPU at 95%",
            "source": "prometheus",
        }

        results = await orchestrator.handle_incident(
            incident_id=incident_id,
            alert=alert,
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        # Should have dispatched 3 tasks: triage + evidence-collection (the
        # LLM's picks) + rca. The fixture's mock_llm_call deliberately omits
        # "rca" despite claiming "Critical alert requires full pipeline" --
        # pinning the live-found gap where the skill-selection LLM
        # repeatedly skipped "rca" in favor of a differently-named skill
        # that merely *sounds* like root cause analysis, leaving real
        # incidents with no RCI/RCA at all. _analyze_and_select_skills now
        # forces "rca" in for any non-low severity when it's missing and
        # available. Results are keyed by skill_id (see mock_llm_call's
        # agent_calls above) -- the fixture's "evidence-collection"
        # skill_id, not the unrelated "evidence" agent_id, is the real key.
        assert mock_a2a_client.send_task.call_count == 3
        assert "triage" in results
        assert "evidence-collection" in results
        assert "rca" in results

    async def test_handle_incident_agent_failure(
        self, mock_backend, mock_a2a_client, mock_llm_call
    ):
        mock_a2a_client.send_task = AsyncMock(side_effect=Exception("agent down"))

        orchestrator = Orchestrator(
            backend_client=mock_backend,
            a2a_client=mock_a2a_client,
            llm_call=mock_llm_call,
        )

        results = await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "low"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        # Should gracefully handle failures
        for key in results:
            assert results[key] == {"error": "agent_unavailable"}

    async def test_results_keyed_by_skill_id_not_agent_id(
        self, mock_backend, mock_a2a_client
    ):
        """A real run found RCI/RCA/postmortem silently never persisted once
        Orchestrator._discover_agents started returning real registry data:
        results[agent_id] = ... used the agent's DB UUID as the dict key,
        but _persist_results (and the outer-loop knowledge extraction) look
        results up by skill name via results.get("rca")/.get("postmortem").
        Previously this went unnoticed because the _default_agents()
        fallback path (always taken, since discovery was broken) happens to
        use the skill name as its "id" field too -- agent_id and skill_id
        were coincidentally equal. This test forces them to differ, the way
        a real a2a_agents row's UUID does.
        """
        real_agent_uuid = str(uuid.uuid4())

        async def llm_call(prompt: str) -> tuple[str, dict]:
            return json.dumps({
                "severity_assessment": "high",
                "reasoning": "test",
                "agent_calls": [
                    {
                        "agent_id": real_agent_uuid,
                        "agent_url": "http://rca-agent:8092",
                        "skill_id": "rca",
                        "priority": 1,
                        "input_summary": "Investigate root cause",
                    },
                ],
            }), _MOCK_USAGE

        # Matches the real rca-agent's A2A response shape: separate
        # artifacts named "rci"/"rca", each holding its own flat data --
        # _extract_result keys the result dict by artifact.name.
        mock_a2a_client.send_task = AsyncMock(return_value=Task(
            id="task-1",
            status=TaskStatus.COMPLETED,
            artifacts=[
                Artifact(name="rci", parts=[DataPart(data={"investigation_summary": "..."})]),
                Artifact(name="rca", parts=[DataPart(data={"root_cause_summary": "pool exhaustion"})]),
            ],
        ))

        orchestrator = Orchestrator(
            backend_client=mock_backend,
            a2a_client=mock_a2a_client,
            llm_call=llm_call,
        )

        results = await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "high"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        assert "rca" in results
        assert real_agent_uuid not in results
        mock_backend.create_rca.assert_awaited_once()
        mock_backend.create_rci.assert_awaited_once()

    async def test_timeline_events_emitted_for_successful_dispatch(
        self, mock_backend, mock_a2a_client, mock_llm_call
    ):
        """Regression test for the "Recent Timeline: No events yet" gap:
        before this, only correlated_alert and war_room_created ever wrote
        an incident_events row -- triage/evidence/rca/postmortem progress
        was invisible on the incident's timeline no matter how much of the
        pipeline actually ran."""
        orchestrator = Orchestrator(
            backend_client=mock_backend,
            a2a_client=mock_a2a_client,
            llm_call=mock_llm_call,
        )

        await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "high"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        event_types = [c.args[1].type for c in mock_backend.add_incident_event.await_args_list]
        # mock_llm_call's fixture dispatches triage + evidence-collection
        # (rca gets force-added for non-low severity, see
        # test_handle_incident_dispatches_tasks above).
        assert "agent_run_started" in event_types
        assert "agent_run_completed" in event_types
        assert "triage_started" in event_types
        assert "triage_completed" in event_types
        assert "evidence_collected" in event_types
        assert "rca_started" in event_types

    async def test_timeline_event_agent_run_failed_on_dispatch_exception(
        self, mock_backend, mock_a2a_client, mock_llm_call
    ):
        mock_a2a_client.send_task = AsyncMock(side_effect=Exception("agent down"))

        orchestrator = Orchestrator(
            backend_client=mock_backend,
            a2a_client=mock_a2a_client,
            llm_call=mock_llm_call,
        )

        await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "low"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        event_types = [c.args[1].type for c in mock_backend.add_incident_event.await_args_list]
        assert "agent_run_started" in event_types
        assert "agent_run_failed" in event_types
        assert "agent_run_completed" not in event_types

    async def test_timeline_events_for_rci_rca_and_hypothesis_on_persist(
        self, mock_backend, mock_a2a_client
    ):
        """rci_completed/rca_completed/hypothesis_generated must fire from
        _persist_results (i.e. only once the artifact is actually confirmed
        written), not just because the rca-agent's A2A call returned."""
        async def llm_call(prompt: str) -> tuple[str, dict]:
            return json.dumps({
                "severity_assessment": "high",
                "reasoning": "test",
                "agent_calls": [
                    {
                        "agent_id": "rca-1",
                        "agent_url": "http://rca-agent:8092",
                        "skill_id": "rca",
                        "priority": 1,
                        "input_summary": "Investigate root cause",
                    },
                ],
            }), _MOCK_USAGE

        mock_a2a_client.send_task = AsyncMock(return_value=Task(
            id="task-1",
            status=TaskStatus.COMPLETED,
            artifacts=[
                Artifact(name="hypothesis", parts=[DataPart(data={"summary": "..."})]),
                Artifact(name="rci", parts=[DataPart(data={"investigation_summary": "..."})]),
                Artifact(name="rca", parts=[DataPart(data={"root_cause_summary": "pool exhaustion"})]),
            ],
        ))

        orchestrator = Orchestrator(
            backend_client=mock_backend,
            a2a_client=mock_a2a_client,
            llm_call=llm_call,
        )

        await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "high"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        event_types = [c.args[1].type for c in mock_backend.add_incident_event.await_args_list]
        assert "rca_started" in event_types
        assert "hypothesis_generated" in event_types
        assert "rci_completed" in event_types
        assert "rca_completed" in event_types

    async def test_timeline_events_skip_persist_completion_when_backend_write_fails(
        self, mock_backend, mock_a2a_client
    ):
        """If create_rca itself fails, rca_completed must NOT be emitted --
        the A2A call succeeding is not the same as the artifact actually
        being persisted."""
        mock_backend.create_rca = AsyncMock(side_effect=Exception("db down"))

        async def llm_call(prompt: str) -> tuple[str, dict]:
            return json.dumps({
                "severity_assessment": "high",
                "reasoning": "test",
                "agent_calls": [
                    {
                        "agent_id": "rca-1",
                        "agent_url": "http://rca-agent:8092",
                        "skill_id": "rca",
                        "priority": 1,
                        "input_summary": "Investigate root cause",
                    },
                ],
            }), _MOCK_USAGE

        mock_a2a_client.send_task = AsyncMock(return_value=Task(
            id="task-1",
            status=TaskStatus.COMPLETED,
            artifacts=[
                Artifact(name="rca", parts=[DataPart(data={"root_cause_summary": "pool exhaustion"})]),
            ],
        ))

        orchestrator = Orchestrator(
            backend_client=mock_backend,
            a2a_client=mock_a2a_client,
            llm_call=llm_call,
        )

        await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "high"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        event_types = [c.args[1].type for c in mock_backend.add_incident_event.await_args_list]
        assert "rca_completed" not in event_types

    async def test_default_decision_on_llm_failure(self, mock_backend, mock_a2a_client):
        async def failing_llm(prompt: str) -> str:
            raise RuntimeError("LLM down")

        orchestrator = Orchestrator(
            backend_client=mock_backend,
            a2a_client=mock_a2a_client,
            llm_call=failing_llm,
        )

        results = await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "critical"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        # Should still dispatch using default pipeline
        assert mock_a2a_client.send_task.call_count > 0

    async def test_extract_result(self, mock_backend, mock_a2a_client, mock_llm_call):
        orchestrator = Orchestrator(
            backend_client=mock_backend,
            a2a_client=mock_a2a_client,
            llm_call=mock_llm_call,
        )

        task = Task(
            id="t1",
            status=TaskStatus.COMPLETED,
            artifacts=[
                Artifact(name="triage_result", parts=[DataPart(data={"severity": "high"})]),
            ],
        )
        result = orchestrator._extract_result(task)
        assert result["triage_result"] == {"severity": "high"}


class TestRcaSafetyNet:
    """_analyze_and_select_skills' forced-"rca" safety net. Found live: the
    skill-selection LLM (a small/fast local model) repeatedly picked a
    Kubernetes-focused skill whose own description also mentions "structured
    root-cause analysis" and skipped "rca" entirely -- but "rca" is the only
    skill whose output actually becomes the incident's structured RCI/RCA
    fields, so real incidents kept ending up with neither."""

    RCA_SKILL = {
        "id": "rca", "name": "Root Cause Analysis",
        "description": "Generate RCI, RCA with 5 Whys, and root cause hypothesis",
        "required_resource_types": [],
        "agents": [{"id": "rca-agent-1", "url": "http://rca-agent:8092", "name": "RCA Agent"}],
    }
    TRIAGE_SKILL = {
        "id": "triage", "name": "Triage", "description": "Alert triage",
        "required_resource_types": [], "agents": [{"id": "t1", "url": "http://triage-agent:8090", "name": "Triage"}],
    }

    @staticmethod
    def _llm_call_returning(severity: str, skill_ids: list[str]):
        async def llm_call(prompt: str) -> tuple[str, dict]:
            return json.dumps({
                "severity_assessment": severity,
                "reasoning": "test",
                "agent_calls": [
                    {"agent_id": s, "agent_url": f"http://{s}:1", "skill_id": s, "priority": i + 1}
                    for i, s in enumerate(skill_ids)
                ],
            }), _MOCK_USAGE
        return llm_call

    async def test_forces_rca_in_when_llm_omits_it_for_high_severity(self, mock_backend, mock_a2a_client):
        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client,
            llm_call=self._llm_call_returning("high", ["triage"]),
        )
        decision = await orchestrator._analyze_and_select_skills(
            {"title": "x"}, {}, [self.TRIAGE_SKILL, self.RCA_SKILL],
        )
        skill_ids = [c["skill_id"] for c in decision.agent_calls]
        assert skill_ids == ["triage", "rca"]

    async def test_does_not_force_rca_for_low_severity(self, mock_backend, mock_a2a_client):
        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client,
            llm_call=self._llm_call_returning("low", ["triage"]),
        )
        decision = await orchestrator._analyze_and_select_skills(
            {"title": "x"}, {}, [self.TRIAGE_SKILL, self.RCA_SKILL],
        )
        skill_ids = [c["skill_id"] for c in decision.agent_calls]
        assert "rca" not in skill_ids

    async def test_does_not_duplicate_rca_when_llm_already_included_it(self, mock_backend, mock_a2a_client):
        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client,
            llm_call=self._llm_call_returning("high", ["triage", "rca"]),
        )
        decision = await orchestrator._analyze_and_select_skills(
            {"title": "x"}, {}, [self.TRIAGE_SKILL, self.RCA_SKILL],
        )
        skill_ids = [c["skill_id"] for c in decision.agent_calls]
        assert skill_ids.count("rca") == 1

    async def test_does_not_force_rca_when_no_rca_skill_available(self, mock_backend, mock_a2a_client):
        """A software/org with no rca-agent linked shouldn't get a bogus call."""
        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client,
            llm_call=self._llm_call_returning("high", ["triage"]),
        )
        decision = await orchestrator._analyze_and_select_skills(
            {"title": "x"}, {}, [self.TRIAGE_SKILL],
        )
        skill_ids = [c["skill_id"] for c in decision.agent_calls]
        assert "rca" not in skill_ids

    async def test_forced_rca_call_gets_agent_url_resolved(self, mock_backend, mock_a2a_client):
        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client,
            llm_call=self._llm_call_returning("medium", ["triage"]),
        )
        decision = await orchestrator._analyze_and_select_skills(
            {"title": "x"}, {}, [self.TRIAGE_SKILL, self.RCA_SKILL],
        )
        rca_call = next(c for c in decision.agent_calls if c["skill_id"] == "rca")
        assert rca_call["agent_url"] == "http://rca-agent:8092"


class TestOrchestratorParallelization:
    """Task A: evidence/snapshot-independent stages should run concurrently
    (asyncio.gather), not strictly sequentially. See the parallelization
    note in Orchestrator.handle_incident for why hypothesis-vs-evidence is
    NOT parallelized (hypothesis genuinely depends on evidence output)."""

    async def test_context_and_similar_incidents_run_concurrently(
        self, mock_backend, mock_a2a_client, mock_llm_call,
    ):
        events: list[str] = []

        async def slow_get_software(software_id, org_id):
            events.append("context_start")
            await asyncio.sleep(0.05)
            events.append("context_end")
            return {"id": software_id, "name": "svc"}

        async def slow_search_kb(**kwargs):
            events.append("similar_start")
            await asyncio.sleep(0.05)
            events.append("similar_end")
            return []

        mock_backend.get_software = AsyncMock(side_effect=slow_get_software)
        mock_backend.search_knowledge_base = AsyncMock(side_effect=slow_search_kb)

        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client, llm_call=mock_llm_call,
        )

        start = time.monotonic()
        await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "high"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )
        elapsed = time.monotonic() - start

        # Both stages must have started before either finished -- proof of
        # real overlap, not just tolerant timing.
        assert set(events[:2]) == {"context_start", "similar_start"}
        assert elapsed < 0.09  # sequential would be >= 0.10s

    async def test_extra_concurrent_task_runs_alongside_context_building(
        self, mock_backend, mock_a2a_client, mock_llm_call,
    ):
        """The extra_concurrent_task hook (e.g. for a future snapshot
        collector integration) is folded into the same asyncio.gather."""
        called = {"snapshot": False}

        async def fake_snapshot_collection():
            called["snapshot"] = True
            return ["snapshot-evidence"]

        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client, llm_call=mock_llm_call,
        )

        await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "high"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
            extra_concurrent_task=fake_snapshot_collection(),
        )

        assert called["snapshot"] is True

    async def test_records_pipeline_timings(self, mock_backend, mock_a2a_client, mock_llm_call):
        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client, llm_call=mock_llm_call,
        )

        await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "high"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        assert "context_and_similar_incidents_ms" in orchestrator.last_pipeline_timings
        assert "total_pipeline_ms" in orchestrator.last_pipeline_timings


class TestPipelineHITLGate:
    """Task B: the pipeline should pause before postmortem when the org has
    pipeline_hitl_gate_enabled, and resume via run_postmortem_only()."""

    async def test_handle_incident_pauses_before_postmortem_when_gate_enabled(
        self, mock_backend, mock_a2a_client,
    ):
        async def llm_call(prompt: str) -> tuple[str, dict]:
            return json.dumps({
                "severity_assessment": "high",
                "reasoning": "postmortem stage selected for this test",
                "agent_calls": [
                    {
                        "agent_id": "postmortem",
                        "agent_url": "http://postmortem-agent:8093",
                        "skill_id": "postmortem",
                        "priority": 1,
                        "input_summary": "Generate postmortem",
                    },
                ],
            }), _MOCK_USAGE

        mock_backend.get_organization_settings = AsyncMock(
            return_value={"pipeline_hitl_gate_enabled": True},
        )
        mock_backend.mark_awaiting_approval = AsyncMock(return_value={})

        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client, llm_call=llm_call,
        )

        incident_id = uuid.uuid4()
        results = await orchestrator.handle_incident(
            incident_id=incident_id,
            alert={"title": "test", "severity": "high"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        assert results["postmortem"]["status"] == "paused"
        assert results["postmortem"]["awaiting_approval_stage"] == "postmortem"
        mock_backend.mark_awaiting_approval.assert_awaited_once_with(incident_id, "postmortem")
        # The postmortem agent must never actually be dispatched while
        # gated -- but the dispatch loop `continue`s past a gated
        # postmortem call rather than stopping the whole pipeline, so
        # _analyze_and_select_skills' rca safety net (severity=high, no
        # "rca" in this test's single-skill LLM decision) still runs and
        # dispatches to rca-agent. That's correct: the postmortem gate is
        # about withholding the final write-up for human review, not about
        # blocking the root cause analysis a reviewer would want to see.
        mock_a2a_client.send_task.assert_awaited_once()
        sent_message = mock_a2a_client.send_task.call_args.args[2]
        assert sent_message.parts[0].data["skill_id"] == "rca"

    async def test_handle_incident_runs_postmortem_when_gate_disabled(
        self, mock_backend, mock_a2a_client,
    ):
        async def llm_call(prompt: str) -> tuple[str, dict]:
            return json.dumps({
                "severity_assessment": "high",
                "reasoning": "postmortem stage selected for this test",
                "agent_calls": [
                    {
                        "agent_id": "postmortem",
                        "agent_url": "http://postmortem-agent:8093",
                        "skill_id": "postmortem",
                        "priority": 1,
                        "input_summary": "Generate postmortem",
                    },
                ],
            }), _MOCK_USAGE

        mock_backend.get_organization_settings = AsyncMock(
            return_value={"pipeline_hitl_gate_enabled": False},
        )

        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client, llm_call=llm_call,
        )

        results = await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "high"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        # Not gated: the postmortem call proceeds through the normal dispatch path.
        assert "postmortem" not in [
            v.get("awaiting_approval_stage") for v in results.values() if isinstance(v, dict)
        ]
        mock_a2a_client.send_task.assert_called()

    async def test_run_postmortem_only_resumes_and_completes(
        self, mock_backend, mock_a2a_client, mock_llm_call,
    ):
        mock_backend.get_incident = AsyncMock(return_value={"id": "inc-1", "title": "t"})
        mock_backend.get_rca = AsyncMock(return_value={"root_cause_summary": "disk full"})
        mock_backend.get_rci = AsyncMock(return_value={"investigation_summary": "..."})

        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client, llm_call=mock_llm_call,
        )

        incident_id = uuid.uuid4()
        org_id = uuid.uuid4()
        result = await orchestrator.run_postmortem_only(incident_id, org_id)

        assert result["status"] == "completed"
        mock_a2a_client.send_task.assert_awaited()
        mock_backend.get_incident.assert_awaited_once_with(incident_id, org_id)

    async def test_maybe_gate_postmortem_fails_open_on_settings_error(
        self, mock_backend, mock_a2a_client, mock_llm_call,
    ):
        """If the org-settings lookup itself fails, don't strand the
        incident -- proceed without gating rather than blocking forever."""
        mock_backend.get_organization_settings = AsyncMock(side_effect=Exception("backend down"))

        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client, llm_call=mock_llm_call,
        )

        gate_result = await orchestrator._maybe_gate_postmortem(uuid.uuid4(), uuid.uuid4())
        assert gate_result is None


class TestResumeListener:
    """Task B: the standalone resume listener consumes pipeline.stage_approved
    and resumes postmortem via Orchestrator.run_postmortem_only(). It must
    not import from app.workers.alert_worker."""

    def test_does_not_import_alert_worker(self):
        """resume_listener.py may *discuss* alert_worker.py in comments
        (explaining why it's deliberately independent) but must not actually
        import from it."""
        import ast

        import app.orchestrator.resume_listener as module

        with open(module.__file__) as f:
            source = f.read()
        tree = ast.parse(source)
        imported_modules = [
            alias.name
            for node in ast.walk(tree)
            if isinstance(node, ast.Import)
            for alias in node.names
        ] + [
            node.module
            for node in ast.walk(tree)
            if isinstance(node, ast.ImportFrom) and node.module
        ]
        assert not any("alert_worker" in m for m in imported_modules), (
            "resume_listener.py must not import from app.workers.alert_worker"
        )

    async def test_handle_message_resumes_postmortem(self):
        orchestrator = AsyncMock()
        orchestrator.run_postmortem_only = AsyncMock(return_value={"status": "completed"})
        redis_client = AsyncMock()

        listener = ResumeListener(redis_client, orchestrator)

        incident_id = uuid.uuid4()
        approved_by = uuid.uuid4()
        org_id = uuid.uuid4()
        envelope = json.dumps({
            "event_type": "pipeline.stage_approved",
            "org_id": str(org_id),
            "payload": {
                "incident_id": str(incident_id),
                "stage": "postmortem",
                "approved_by": str(approved_by),
            },
        })
        message = {"type": "pmessage", "data": envelope.encode()}

        await listener._handle_message(message)

        orchestrator.run_postmortem_only.assert_awaited_once_with(incident_id, org_id)

    async def test_handle_message_ignores_non_approval_events(self):
        orchestrator = AsyncMock()
        listener = ResumeListener(AsyncMock(), orchestrator)

        message = {
            "data": json.dumps({"event_type": "alert.received", "payload": {}}).encode(),
        }
        await listener._handle_message(message)

        orchestrator.run_postmortem_only.assert_not_called()

    async def test_handle_message_ignores_non_postmortem_stage(self):
        orchestrator = AsyncMock()
        listener = ResumeListener(AsyncMock(), orchestrator)

        message = {
            "data": json.dumps({
                "event_type": "pipeline.stage_approved",
                "payload": {"incident_id": str(uuid.uuid4()), "stage": "rca"},
            }).encode(),
        }
        await listener._handle_message(message)

        orchestrator.run_postmortem_only.assert_not_called()

    async def test_handle_message_ignores_malformed_payload(self):
        orchestrator = AsyncMock()
        listener = ResumeListener(AsyncMock(), orchestrator)

        # Not valid JSON at all.
        await listener._handle_message({"data": b"not json"})
        # Valid JSON but missing incident_id.
        await listener._handle_message({
            "data": json.dumps({
                "event_type": "pipeline.stage_approved", "payload": {"stage": "postmortem"},
            }).encode(),
        })

        orchestrator.run_postmortem_only.assert_not_called()

    async def test_handle_message_ignores_event_missing_org_id(self):
        """org_id is required to resolve the incident/persist evidence
        downstream (see BackendClient.get_incident's X-Org-ID header) --
        an event without it must not silently proceed with a garbage org."""
        orchestrator = AsyncMock()
        listener = ResumeListener(AsyncMock(), orchestrator)

        await listener._handle_message({
            "data": json.dumps({
                "event_type": "pipeline.stage_approved",
                "payload": {"incident_id": str(uuid.uuid4()), "stage": "postmortem"},
            }).encode(),
        })

        orchestrator.run_postmortem_only.assert_not_called()


class TestSemanticEventsForSkill:
    def test_triage(self):
        assert Orchestrator._semantic_events_for_skill("triage") == ("triage_started", ["triage_completed"])

    def test_evidence_collection_has_no_started_type(self):
        assert Orchestrator._semantic_events_for_skill("evidence-collection") == (None, ["evidence_collected"])

    def test_rca_completion_handled_by_persist_results_not_here(self):
        # rca_completed/hypothesis_generated/rci_completed are emitted from
        # _persist_results once the artifacts are confirmed persisted, so
        # this mapping only carries the "started" half for rca.
        assert Orchestrator._semantic_events_for_skill("rca") == ("rca_started", [])

    def test_postmortem_completion_handled_by_persist_results_not_here(self):
        assert Orchestrator._semantic_events_for_skill("postmortem") == ("postmortem_started", [])

    def test_unknown_skill_has_no_semantic_type(self):
        assert Orchestrator._semantic_events_for_skill("k8s-debug") == (None, [])
        assert Orchestrator._semantic_events_for_skill("azure-aks-activity-log") == (None, [])

    def test_case_insensitive(self):
        assert Orchestrator._semantic_events_for_skill("RCA") == ("rca_started", [])


class TestFieldSanitizers:
    """_sanitize_rci/_sanitize_rca/_sanitize_postmortem are the allow-lists
    that decide which LLM-produced JSON keys actually reach the backend --
    a platform audit found several prompt fields (rci.timeline,
    postmortem.incident_timeline) that were silently dropped here because
    nothing kept the prompt schema and this allow-list in sync. These tests
    pin the currently-intended allow-list, including detection_time (a real
    DB column/frontend field that the RCA prompt now populates but that used
    to be dropped here unconditionally)."""

    def test_sanitize_rci_passes_through_valid_detection_time(self):
        out = Orchestrator._sanitize_rci({
            "investigation_summary": "x",
            "detection_time": "2026-08-20T03:07:28.533415Z",
        })
        assert out["detection_time"] == "2026-08-20T03:07:28.533415Z"

    def test_sanitize_rci_drops_unparseable_detection_time(self):
        """The LLM sometimes writes a vague/non-ISO string, or the literal
        word "unknown" -- Go's *time.Time JSON binding would 400 the whole
        RCI create on that, so it must be dropped rather than forwarded."""
        out = Orchestrator._sanitize_rci({
            "investigation_summary": "x",
            "detection_time": "sometime last Tuesday",
        })
        assert "detection_time" not in out

    def test_sanitize_rci_drops_null_detection_time(self):
        out = Orchestrator._sanitize_rci({
            "investigation_summary": "x",
            "detection_time": None,
        })
        assert "detection_time" not in out

    def test_sanitize_rci_still_drops_timeline(self):
        """rci.timeline (the structured array) has no typed column on the
        Go side -- it's preserved only in the raw evidence blob the
        orchestrator stores separately, not in the typed RCI create
        request. Confirms that pre-existing, documented trade-off didn't
        regress while adding detection_time."""
        out = Orchestrator._sanitize_rci({
            "investigation_summary": "x",
            "timeline": [{"time": "t0", "event": "e0"}],
        })
        assert "timeline" not in out

    def test_sanitize_postmortem_passes_through_narrative_not_raw_timeline(self):
        out = Orchestrator._sanitize_postmortem({
            "title": "x",
            "executive_summary": "y",
            "incident_timeline": [{"time": "t0", "event": "e0", "actor": "system"}],
            "incident_timeline_narrative": "At t0, e0 happened.",
        })
        assert out["incident_timeline_narrative"] == "At t0, e0 happened."
        assert "incident_timeline" not in out


class TestRealUsageTracking:
    """The orchestrator used to fabricate model_used/tokens_used (a hardcoded
    "anthropic/claude-sonnet-4-6" label for rca/postmortem regardless of
    what actually ran, plus a chars/4 heuristic). It now prefers the real
    `llm_usage` artifact each A2A agent reports from the API's own `usage`
    field, only falling back to the heuristic when an agent couldn't report
    real usage at all (e.g. it errored before calling the LLM)."""

    async def test_uses_real_llm_usage_when_agent_reports_it(
        self, mock_backend, mock_a2a_client, mock_llm_call,
    ):
        mock_a2a_client.send_task = AsyncMock(return_value=Task(
            id="task-1",
            status=TaskStatus.COMPLETED,
            artifacts=[
                Artifact(name="triage_result", parts=[DataPart(data={"summary": "x"})]),
                Artifact(name="llm_usage", parts=[DataPart(data={
                    "model": "qwen2.5-coder-14b-instruct",
                    "prompt_tokens": 900, "completion_tokens": 120, "total_tokens": 1020,
                })]),
            ],
        ))
        # The shared mock_backend fixture doesn't configure create_agent_run
        # with a real ID -- its default MagicMock return breaks
        # uuid.UUID(run_id) downstream, which every other test in this file
        # tolerates by simply never asserting anything about
        # update_agent_run. This is the first test that actually depends on
        # the create->update round trip succeeding, so give it one here.
        mock_backend.create_agent_run = AsyncMock(return_value={"id": str(uuid.uuid4())})
        mock_backend.update_agent_run = AsyncMock(return_value={})

        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client, llm_call=mock_llm_call,
        )
        await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "high"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        update_calls = mock_backend.update_agent_run.call_args_list
        assert update_calls, "expected at least one update_agent_run call"
        # update_agent_run(incident_id, run_id, request) -- request is the 3rd positional arg.
        req = update_calls[0][0][2]
        assert req.model_used == "qwen2.5-coder-14b-instruct"
        assert req.tokens_used == 1020

    async def test_falls_back_to_heuristic_when_agent_reports_no_usage(
        self, mock_backend, mock_a2a_client, mock_llm_call,
    ):
        """An agent that errored before ever calling the LLM (or an older
        deployment that hasn't been redeployed with the llm_usage artifact
        yet) has no real usage to report -- must still record *something*
        rather than crash agent_run bookkeeping."""
        mock_a2a_client.send_task = AsyncMock(return_value=Task(
            id="task-1",
            status=TaskStatus.COMPLETED,
            artifacts=[
                Artifact(name="triage_result", parts=[DataPart(data={"summary": "x"})]),
            ],
        ))
        mock_backend.create_agent_run = AsyncMock(return_value={"id": str(uuid.uuid4())})
        mock_backend.update_agent_run = AsyncMock(return_value={})

        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client, llm_call=mock_llm_call,
        )
        await orchestrator.handle_incident(
            incident_id=uuid.uuid4(),
            alert={"title": "test", "severity": "high"},
            software_id="sw-1",
            org_id=uuid.uuid4(),
        )

        update_calls = mock_backend.update_agent_run.call_args_list
        assert update_calls, "expected at least one update_agent_run call"
        req = update_calls[0][0][2]
        assert req.model_used  # falls back to the configured cheap model, not None
        assert req.tokens_used > 0


class TestLLMConfigResolution:
    """The org's default LLM settings (LLM & Tokens settings UI, migration
    023_llm_settings) and a per-agent managed_config override must actually
    reach each dispatched agent as input_data["llm_config"] -- this is the
    plumbing that used to be a documented no-op (input_data["llm_config"]
    was built but no agent ever read it)."""

    async def test_org_default_llm_settings_reach_agent_input(
        self, mock_backend, mock_a2a_client, mock_llm_call,
    ):
        mock_backend.get_organization_settings = AsyncMock(return_value={
            "default_llm_provider_type": "openrouter",
            "default_llm_base_url": "https://openrouter.ai/api/v1",
            "default_llm_model": "anthropic/claude-sonnet-4-6",
            "default_llm_api_key_ref": "sk-or-test",
        })
        mock_a2a_client.send_task = AsyncMock(return_value=Task(
            id="task-1", status=TaskStatus.COMPLETED,
            artifacts=[Artifact(name="triage_result", parts=[DataPart(data={"summary": "x"})])],
        ))

        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client, llm_call=mock_llm_call,
        )
        await orchestrator.handle_incident(
            incident_id=uuid.uuid4(), alert={"title": "test", "severity": "high"},
            software_id="sw-1", org_id=uuid.uuid4(),
        )

        send_calls = mock_a2a_client.send_task.call_args_list
        assert send_calls, "expected at least one send_task call"
        message = send_calls[0][0][2]
        llm_config = message.parts[0].data["llm_config"]
        assert llm_config["api_base"] == "https://openrouter.ai/api/v1"
        assert llm_config["model"] == "anthropic/claude-sonnet-4-6"
        assert llm_config["api_key"] == "sk-or-test"

    async def test_per_agent_override_wins_over_org_default(
        self, mock_backend, mock_a2a_client, mock_llm_call,
    ):
        mock_backend.get_organization_settings = AsyncMock(return_value={
            "default_llm_model": "org-default-model",
        })
        mock_backend.list_a2a_agents = AsyncMock(return_value=[
            {
                "id": "triage", "endpoint_url": "http://triage-agent:8090",
                "hosting_type": "managed", "llm_provider": "platform",
                "managed_config": {"model": "per-agent-override-model", "temperature": 0.9},
            },
        ])
        mock_a2a_client.discover = AsyncMock(return_value=AgentCard(
            name="triage", url="http://triage-agent:8090", version="0.1.0",
            skills=[AgentSkill(id="triage", name="Triage", description="Triage")],
        ))
        mock_a2a_client.send_task = AsyncMock(return_value=Task(
            id="task-1", status=TaskStatus.COMPLETED,
            artifacts=[Artifact(name="triage_result", parts=[DataPart(data={"summary": "x"})])],
        ))

        orchestrator = Orchestrator(
            backend_client=mock_backend, a2a_client=mock_a2a_client, llm_call=mock_llm_call,
        )
        await orchestrator.handle_incident(
            incident_id=uuid.uuid4(), alert={"title": "test", "severity": "high"},
            software_id="sw-1", org_id=uuid.uuid4(),
        )

        send_calls = mock_a2a_client.send_task.call_args_list
        assert send_calls, "expected at least one send_task call"
        message = send_calls[0][0][2]
        llm_config = message.parts[0].data["llm_config"]
        assert llm_config["model"] == "per-agent-override-model"
        assert llm_config["temperature"] == 0.9
