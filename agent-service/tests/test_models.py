"""Tests for Pydantic models matching the contract schemas."""

import uuid
from datetime import datetime, timezone

import pytest

from app.models.events import (
    AgentStatusPayload,
    AlertReceivedPayload,
    EventEnvelope,
    Evidence,
    EvidenceCollectedPayload,
    Hypothesis,
    HypothesisGeneratedPayload,
    NormalizedAlert,
    TriageCompletedPayload,
    TriageResult,
)
from app.models.api import Agent, AgentConfig, IncidentEvidenceCreate, IncidentEventCreate, IncidentUpdate


class TestEventEnvelope:
    def test_parse_valid_envelope(self):
        data = {
            "event_id": str(uuid.uuid4()),
            "event_type": "alert.received",
            "org_id": str(uuid.uuid4()),
            "timestamp": "2024-01-15T10:30:00Z",
            "payload": {"key": "value"},
        }
        envelope = EventEnvelope.model_validate(data)
        assert envelope.event_type == "alert.received"
        assert isinstance(envelope.event_id, uuid.UUID)
        assert isinstance(envelope.org_id, uuid.UUID)

    def test_envelope_requires_all_fields(self):
        with pytest.raises(Exception):
            EventEnvelope.model_validate({"event_type": "test"})


class TestNormalizedAlert:
    def test_parse_full_alert(self):
        data = {
            "title": "High CPU Usage",
            "description": "CPU usage above 90% for 5 minutes",
            "severity": "critical",
            "source": "datadog",
            "service": "api-gateway",
            "tags": {"env": "production"},
            "started_at": "2024-01-15T10:00:00Z",
        }
        alert = NormalizedAlert.model_validate(data)
        assert alert.title == "High CPU Usage"
        assert alert.severity == "critical"
        assert alert.tags == {"env": "production"}

    def test_parse_minimal_alert(self):
        alert = NormalizedAlert(title="Test", severity="low", source="custom")
        assert alert.description is None
        assert alert.service is None


class TestAlertReceivedPayload:
    def test_parse_full_payload(self):
        data = {
            "alert_snapshot_id": str(uuid.uuid4()),
            "incident_id": str(uuid.uuid4()),
            "software_id": str(uuid.uuid4()),
            "webhook_source": "datadog",
            "normalized_alert": {
                "title": "DB Connection Pool Exhausted",
                "description": "All connections in use",
                "severity": "high",
                "source": "datadog",
                "service": "user-service",
            },
        }
        payload = AlertReceivedPayload.model_validate(data)
        assert payload.normalized_alert.title == "DB Connection Pool Exhausted"
        assert payload.webhook_source == "datadog"


class TestTriageResult:
    def test_parse_triage_result(self):
        data = {
            "severity_assessment": "critical",
            "category": "infrastructure",
            "affected_components": ["api-gateway", "load-balancer"],
            "summary": "Critical infrastructure failure",
            "confidence": 0.85,
        }
        result = TriageResult.model_validate(data)
        assert result.severity_assessment == "critical"
        assert len(result.affected_components) == 2
        assert result.confidence == 0.85

    def test_confidence_bounds(self):
        with pytest.raises(Exception):
            TriageResult(
                severity_assessment="low",
                category="test",
                summary="test",
                confidence=1.5,
            )


class TestTriageCompletedPayload:
    def test_parse(self):
        data = {
            "incident_id": str(uuid.uuid4()),
            "triage_result": {
                "severity_assessment": "medium",
                "category": "application",
                "affected_components": [],
                "summary": "Minor issue",
                "confidence": 0.6,
            },
        }
        payload = TriageCompletedPayload.model_validate(data)
        assert payload.triage_result.category == "application"


class TestEvidence:
    def test_parse_evidence(self):
        data = {
            "type": "log",
            "title": "Error logs from api-gateway",
            "content": {"entries": ["error1", "error2"]},
            "source": "datadog",
        }
        evidence = Evidence.model_validate(data)
        assert evidence.type == "log"
        assert len(evidence.content["entries"]) == 2


class TestHypothesis:
    def test_parse_hypothesis(self):
        eid = str(uuid.uuid4())
        data = {
            "root_cause": "Memory leak in connection pool",
            "confidence": 0.78,
            "supporting_evidence": [eid],
            "recommended_actions": ["Restart service", "Increase pool size"],
            "mitigation_steps": ["Scale horizontally"],
        }
        h = Hypothesis.model_validate(data)
        assert h.root_cause == "Memory leak in connection pool"
        assert len(h.recommended_actions) == 2


class TestAgentStatusPayload:
    def test_parse_status(self):
        data = {
            "incident_id": str(uuid.uuid4()),
            "agent_id": str(uuid.uuid4()),
            "agent_name": "triage-agent",
            "status": "started",
            "message": None,
            "progress": 0,
        }
        status = AgentStatusPayload.model_validate(data)
        assert status.status == "started"


class TestApiModels:
    def test_agent_model(self):
        data = {
            "id": str(uuid.uuid4()),
            "org_id": str(uuid.uuid4()),
            "name": "Triage Bot",
            "type": "triage",
            "config": {"model": "gpt-4o", "temperature": 0.3, "system_prompt": "You are a triage bot"},
            "enabled": True,
        }
        agent = Agent.model_validate(data)
        assert agent.name == "Triage Bot"
        assert agent.config.model == "gpt-4o"

    def test_incident_update(self):
        update = IncidentUpdate(status="investigating", severity="high")
        dumped = update.model_dump(exclude_none=True)
        assert "assignee_id" not in dumped
        assert dumped["status"] == "investigating"

    def test_incident_event_create(self):
        event = IncidentEventCreate(type="agent_action", data={"action": "triage"})
        assert event.type == "agent_action"

    def test_incident_evidence_create(self):
        evidence = IncidentEvidenceCreate(
            type="agent_output",
            title="Triage output",
            content={"result": "ok"},
            source="crewai",
        )
        assert evidence.type == "agent_output"
