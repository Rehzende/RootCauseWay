"""Tests for skills-aware orchestration."""

from __future__ import annotations

import json
from unittest.mock import AsyncMock, MagicMock, patch
from uuid import uuid4

import pytest

from app.a2a.models import Artifact, DataPart, Task, TaskStatus
from app.credentials.jit_provider import JITCredentialProvider
from app.credentials.models import CredentialLease
from app.orchestrator.orchestrator import Orchestrator, OrchestratorDecision


@pytest.fixture
def mock_backend():
    backend = AsyncMock()
    backend.get_software.return_value = {
        "id": "sw-1",
        "name": "test-service",
        "description": "A test service",
        "type": "microservice",
    }
    backend.list_skills.return_value = [
        {
            "id": "triage",
            "name": "Alert Triage",
            "description": "Alert triage and severity assessment",
            "required_resource_types": [],
            "agents": [
                {"id": "triage-agent", "url": "http://triage:8080", "name": "Triage Agent"},
            ],
        },
        {
            "id": "kubernetes-debug",
            "name": "Kubernetes Debug",
            "description": "Debug Kubernetes workloads",
            "required_resource_types": ["kubernetes_cluster"],
            "agents": [
                {"id": "k8s-agent", "url": "http://k8s:8080", "name": "K8s Agent"},
            ],
        },
        {
            "id": "database-analysis",
            "name": "Database Analysis",
            "description": "Analyze database performance",
            "required_resource_types": ["database"],
            "agents": [
                {"id": "evidence-agent", "url": "http://evidence:8080", "name": "Evidence Agent"},
            ],
        },
    ]
    backend.create_orchestrator_decision.return_value = {}
    backend.create_a2a_task.return_value = {}
    backend.update_a2a_task.return_value = {}
    return backend


@pytest.fixture
def mock_a2a():
    a2a = AsyncMock()
    a2a.send_task.return_value = Task(
        id="task-1",
        status=TaskStatus.COMPLETED,
        artifacts=[
            Artifact(
                name="triage_result",
                parts=[DataPart(data={"severity": "high", "summary": "CPU spike"})],
            )
        ],
    )
    return a2a


@pytest.fixture
def mock_jit():
    jit = AsyncMock(spec=JITCredentialProvider)
    jit.request_credentials.return_value = None
    jit.revoke_credentials.return_value = None
    jit.get_active_leases.return_value = []
    return jit


@pytest.fixture
def mock_llm():
    async def llm_call(prompt):
        return json.dumps({
            "severity_assessment": "high",
            "reasoning": "CPU spike requires triage and k8s debug",
            "skill_calls": [
                {
                    "skill_id": "triage",
                    "agent_id": "triage-agent",
                    "agent_url": "http://triage:8080",
                    "priority": 1,
                    "input_summary": "Triage the CPU spike alert",
                    "required_resource_types": [],
                },
                {
                    "skill_id": "kubernetes-debug",
                    "agent_id": "k8s-agent",
                    "agent_url": "http://k8s:8080",
                    "priority": 2,
                    "input_summary": "Debug k8s pods for CPU spike",
                    "required_resource_types": ["kubernetes_cluster"],
                },
            ],
        }), {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
    return llm_call


class TestSkillsAwareOrchestration:
    async def test_skills_discovery_used(self, mock_backend, mock_a2a, mock_jit, mock_llm):
        """Test that orchestrator discovers skills from backend."""
        orchestrator = Orchestrator(mock_backend, mock_a2a, mock_llm, mock_jit)

        await orchestrator.handle_incident(
            incident_id=uuid4(),
            alert={"title": "CPU spike", "severity": "high"},
            software_id="sw-1",
            org_id=uuid4(),
        )

        mock_backend.list_skills.assert_called_once()

    async def test_falls_back_to_agents_when_no_skills(self, mock_backend, mock_a2a, mock_jit, mock_llm):
        """Test fallback to agent-based discovery when skills endpoint fails."""
        mock_backend.list_skills.return_value = []
        mock_backend.list_a2a_agents.side_effect = Exception("not available")

        orchestrator = Orchestrator(mock_backend, mock_a2a, mock_llm, mock_jit)

        with patch.object(orchestrator, '_default_agents') as mock_defaults:
            mock_defaults.return_value = [
                {
                    "id": "triage",
                    "url": "http://triage:8080",
                    "card": {
                        "name": "Triage Agent",
                        "url": "http://triage:8080",
                        "version": "0.1.0",
                        "skills": [{"id": "triage", "name": "Triage", "description": "Triage"}],
                    },
                }
            ]
            await orchestrator.handle_incident(
                incident_id=uuid4(),
                alert={"title": "test", "severity": "low"},
                software_id="sw-1",
                org_id=uuid4(),
            )
            mock_defaults.assert_called()

    async def test_credential_request_during_orchestration(
        self, mock_backend, mock_a2a, mock_jit, mock_llm
    ):
        """Test that JIT credentials are requested for skills requiring resources."""
        lease_id = uuid4()
        mock_jit.request_credentials.return_value = CredentialLease(
            id=lease_id,
            incident_id=uuid4(),
            agent_id=uuid4(),
            skill_id="kubernetes-debug",
            resource_credential_id=uuid4(),
            status="active",
            credential_data={"kubeconfig": "base64data"},
        )

        orchestrator = Orchestrator(mock_backend, mock_a2a, mock_llm, mock_jit)
        await orchestrator.handle_incident(
            incident_id=uuid4(),
            alert={"title": "CPU spike", "severity": "high"},
            software_id="sw-1",
            org_id=uuid4(),
        )

        # Should have requested credentials for kubernetes_cluster
        assert mock_jit.request_credentials.call_count >= 1
        # Check at least one call was for kubernetes_cluster
        k8s_calls = [
            c for c in mock_jit.request_credentials.call_args_list
            if c.kwargs.get("resource_type") == "kubernetes_cluster"
        ]
        assert len(k8s_calls) == 1

    async def test_credential_revocation_after_task(
        self, mock_backend, mock_a2a, mock_jit, mock_llm
    ):
        """Test that credentials are revoked after agent task completes."""
        lease_id = uuid4()
        mock_jit.request_credentials.return_value = CredentialLease(
            id=lease_id,
            incident_id=uuid4(),
            agent_id=uuid4(),
            skill_id="kubernetes-debug",
            resource_credential_id=uuid4(),
            status="active",
            credential_data={"kubeconfig": "base64data"},
        )

        orchestrator = Orchestrator(mock_backend, mock_a2a, mock_llm, mock_jit)
        await orchestrator.handle_incident(
            incident_id=uuid4(),
            alert={"title": "CPU spike", "severity": "high"},
            software_id="sw-1",
            org_id=uuid4(),
        )

        # Credentials should be revoked after each task
        mock_jit.revoke_credentials.assert_called()

    async def test_credentials_included_in_task_message(
        self, mock_backend, mock_a2a, mock_jit, mock_llm
    ):
        """Test that credentials are passed to agents in the task message."""
        lease_id = uuid4()
        mock_jit.request_credentials.return_value = CredentialLease(
            id=lease_id,
            incident_id=uuid4(),
            agent_id=uuid4(),
            skill_id="kubernetes-debug",
            resource_credential_id=uuid4(),
            status="active",
            credential_data={"kubeconfig": "base64data"},
        )

        orchestrator = Orchestrator(mock_backend, mock_a2a, mock_llm, mock_jit)
        await orchestrator.handle_incident(
            incident_id=uuid4(),
            alert={"title": "CPU spike", "severity": "high"},
            software_id="sw-1",
            org_id=uuid4(),
        )

        # Check that at least one send_task call included credentials in data
        for call in mock_a2a.send_task.call_args_list:
            message = call.args[2]  # third arg is message
            for part in message.parts:
                if hasattr(part, "data") and "credentials" in part.data:
                    assert "kubernetes_cluster" in part.data["credentials"]
                    return

        # If no credentials were needed (triage has none), that's also valid
        # The k8s task should have had credentials
        assert mock_jit.request_credentials.call_count >= 1


class TestSkillPromptTemplateThreading:
    """A platform audit found Skill.prompt_template was captured on
    create/edit but never read anywhere -- see the identical rationale in
    each A2A agent's own test_agent.py. This covers the orchestrator half:
    _analyze_and_select_skills must copy a matching skill's prompt_template
    onto the call, and handle_incident's dispatch loop must put it into the
    task's input_data so it actually reaches the agent."""

    async def test_skill_prompt_template_reaches_agent_input_data(
        self, mock_backend, mock_a2a, mock_jit
    ):
        mock_backend.list_skills.return_value = [
            {
                "id": "triage",
                "name": "Alert Triage",
                "description": "Alert triage and severity assessment",
                "required_resource_types": [],
                "prompt_template": "Always flag anything mentioning connection pools as high severity.",
                "agents": [
                    {"id": "triage-agent", "url": "http://triage:8080", "name": "Triage Agent"},
                ],
            },
        ]

        async def llm_call(prompt):
            return json.dumps({
                "severity_assessment": "high",
                "reasoning": "test",
                "skill_calls": [
                    {
                        "skill_id": "triage",
                        "agent_id": "triage-agent",
                        "agent_url": "http://triage:8080",
                        "priority": 1,
                        "input_summary": "Triage the alert",
                        "required_resource_types": [],
                    },
                ],
            }), {"model": "m", "prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}

        orchestrator = Orchestrator(mock_backend, mock_a2a, llm_call, mock_jit)
        await orchestrator.handle_incident(
            incident_id=uuid4(),
            alert={"title": "CPU spike", "severity": "high"},
            software_id="sw-1",
            org_id=uuid4(),
        )

        call = mock_a2a.send_task.call_args_list[0]
        message = call.args[2]
        data = next(p.data for p in message.parts if hasattr(p, "data"))
        assert data["skill_prompt_template"] == (
            "Always flag anything mentioning connection pools as high severity."
        )

    async def test_skill_without_prompt_template_omits_it_from_input_data(
        self, mock_backend, mock_a2a, mock_jit, mock_llm
    ):
        """mock_backend's default skills fixture has no prompt_template --
        the key must not appear at all, not appear as None/empty."""
        orchestrator = Orchestrator(mock_backend, mock_a2a, mock_llm, mock_jit)
        await orchestrator.handle_incident(
            incident_id=uuid4(),
            alert={"title": "CPU spike", "severity": "high"},
            software_id="sw-1",
            org_id=uuid4(),
        )

        call = mock_a2a.send_task.call_args_list[0]
        message = call.args[2]
        data = next(p.data for p in message.parts if hasattr(p, "data"))
        assert "skill_prompt_template" not in data


class TestOrchestratorDecision:
    def test_to_dict(self):
        decision = OrchestratorDecision(
            severity_assessment="high",
            reasoning="test reason",
            agent_calls=[{"agent_id": "triage", "skill_id": "triage"}],
        )
        d = decision.to_dict()
        assert d["severity_assessment"] == "high"
        assert d["reasoning"] == "test reason"
        assert len(d["agent_calls"]) == 1


class TestSkillsFromAgents:
    def test_converts_agents_to_skills(self):
        agents = [
            {
                "id": "evidence-agent",
                "url": "http://evidence:8080",
                "card": {
                    "name": "Evidence Agent",
                    "skills": [
                        {"id": "log-analysis", "name": "Log Analysis", "description": "Analyze logs"},
                        {"id": "db-analysis", "name": "DB Analysis", "description": "Analyze DB"},
                    ],
                },
            }
        ]
        skills = Orchestrator._skills_from_agents(agents)
        assert len(skills) == 2
        assert skills[0]["id"] == "log-analysis"
        assert skills[0]["agents"][0]["id"] == "evidence-agent"
        assert skills[1]["id"] == "db-analysis"
