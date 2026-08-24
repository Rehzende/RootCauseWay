"""Pydantic models matching contracts/events/redis-events.yaml."""

from __future__ import annotations

from datetime import datetime
from typing import Any, Optional
from uuid import UUID

from pydantic import BaseModel, Field


# --- Shared / Envelope ---


class EventEnvelope(BaseModel):
    event_id: UUID
    event_type: str
    org_id: UUID
    timestamp: datetime
    payload: dict[str, Any]


# --- alert.received ---


class NormalizedAlert(BaseModel):
    title: str
    description: str | None = None
    severity: str  # critical | high | medium | low
    source: str
    service: str | None = None
    tags: dict[str, Any] | None = None
    started_at: datetime | None = None


class AlertReceivedPayload(BaseModel):
    alert_snapshot_id: UUID
    incident_id: UUID
    software_id: UUID
    webhook_source: str | None = None
    normalized_alert: NormalizedAlert


# --- triage.completed ---


class TriageResult(BaseModel):
    severity_assessment: str  # critical | high | medium | low
    category: str
    affected_components: list[str] = Field(default_factory=list)
    suggested_assignee_id: UUID | None = None
    summary: str
    confidence: float = Field(ge=0, le=1)


class TriageCompletedPayload(BaseModel):
    incident_id: UUID
    triage_result: TriageResult


# --- evidence.collected ---


class Evidence(BaseModel):
    type: str  # log | metric | trace | snapshot | agent_output
    title: str
    content: dict[str, Any]
    source: str


class EvidenceCollectedPayload(BaseModel):
    incident_id: UUID
    evidence: Evidence


# --- hypothesis.generated ---


class Hypothesis(BaseModel):
    root_cause: str
    confidence: float = Field(ge=0, le=1)
    supporting_evidence: list[UUID] = Field(default_factory=list)
    recommended_actions: list[str] = Field(default_factory=list)
    mitigation_steps: list[str] = Field(default_factory=list)


class HypothesisGeneratedPayload(BaseModel):
    incident_id: UUID
    hypothesis: Hypothesis


# --- agent.status ---


class AgentStatusPayload(BaseModel):
    incident_id: UUID
    agent_id: UUID
    agent_name: str | None = None
    status: str  # started | running | completed | failed
    message: str | None = None
    progress: float | None = Field(default=None, ge=0, le=100)
