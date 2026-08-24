"""Pydantic models for calling the Go backend API (contracts/openapi/rootcauseway-api.yaml)."""

from __future__ import annotations

from datetime import datetime
from typing import Any, Optional
from uuid import UUID

from pydantic import BaseModel


class AgentConfig(BaseModel):
    model: str | None = None
    temperature: float | None = None
    tools: list[str] | None = None
    system_prompt: str | None = None


class Agent(BaseModel):
    id: UUID
    org_id: UUID
    name: str
    type: str  # triage | evidence_analysis | hypothesis | debug | custom
    description: str | None = None
    config: AgentConfig | None = None
    enabled: bool = True
    created_at: datetime | None = None
    updated_at: datetime | None = None


class IncidentUpdate(BaseModel):
    status: str | None = None
    severity: str | None = None
    assignee_id: UUID | None = None
    root_cause: str | None = None
    mitigation: str | None = None


class IncidentEventCreate(BaseModel):
    type: str  # comment | status_changed | agent_action
    data: dict[str, Any] | None = None


class IncidentEvidenceCreate(BaseModel):
    type: str  # log | metric | trace | snapshot | agent_output | manual
    title: str
    content: dict[str, Any]
    source: str | None = None


class PaginatedResponse(BaseModel):
    data: list[dict[str, Any]]
    total: int
    page: int
    per_page: int


# --- Agent Run tracking ---


class AgentRun(BaseModel):
    id: UUID
    incident_id: UUID
    agent_id: UUID | None = None
    agent_name: str
    agent_type: str
    status: str  # running | completed | failed
    parent_run_id: UUID | None = None
    input_data: dict[str, Any] | None = None
    output_data: dict[str, Any] | None = None
    error_message: str | None = None
    model_used: str | None = None
    tokens_used: int | None = None
    duration_ms: int | None = None
    started_at: datetime | None = None
    completed_at: datetime | None = None


class CreateAgentRunRequest(BaseModel):
    incident_id: UUID
    agent_name: str
    agent_type: str
    agent_id: UUID | None = None
    parent_run_id: UUID | None = None
    input_data: dict[str, Any] | None = None


class UpdateAgentRunRequest(BaseModel):
    status: str | None = None
    output_data: dict[str, Any] | None = None
    error_message: str | None = None
    model_used: str | None = None
    tokens_used: int | None = None
    duration_ms: int | None = None
    completed_at: datetime | None = None


# --- RCI / RCA / Postmortem data ---


class FiveWhyEntry(BaseModel):
    why: str
    answer: str


class RCIData(BaseModel):
    investigation_summary: str
    impact_assessment: str
    affected_services: list[str] = []
    affected_users_estimate: str | None = None
    detection_method: str | None = None
    evidence_ids: list[UUID] = []


class RCAData(BaseModel):
    root_cause_summary: str
    root_cause_category: str
    contributing_factors: list[str] = []
    five_whys: list[FiveWhyEntry] = []
    confidence: float = 0.0
    evidence_ids: list[UUID] = []


class ActionItem(BaseModel):
    title: str
    description: str | None = None
    priority: str | None = None
    assignee: str | None = None


class PostmortemData(BaseModel):
    title: str
    executive_summary: str
    incident_timeline_narrative: str | None = None
    root_cause_detail: str | None = None
    impact_detail: str | None = None
    lessons_learned: list[str] = []
    action_items: list[ActionItem] = []
    what_went_well: list[str] = []
    what_went_wrong: list[str] = []
    prevention_measures: list[str] = []
