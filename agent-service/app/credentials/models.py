"""Pydantic models for JIT credential management."""

from __future__ import annotations

from datetime import datetime
from enum import Enum
from typing import Any
from uuid import UUID

from pydantic import BaseModel, Field


class LeaseStatus(str, Enum):
    ACTIVE = "active"
    REVOKED = "revoked"
    EXPIRED = "expired"


class CredentialLease(BaseModel):
    id: UUID
    incident_id: UUID
    agent_id: UUID
    skill_id: str
    resource_credential_id: UUID
    status: LeaseStatus = LeaseStatus.ACTIVE
    scope: dict[str, Any] = Field(default_factory=dict)
    credential_data: dict[str, Any] = Field(default_factory=dict)
    issued_at: datetime | None = None
    expires_at: datetime | None = None


class LeaseRequest(BaseModel):
    incident_id: UUID
    agent_id: UUID
    skill_id: str
    resource_credential_id: UUID
    ttl_seconds: int = 900
    scope: dict[str, Any] = Field(default_factory=dict)
    reason: str = ""


class CredentialProviderConfig(BaseModel):
    provider_type: str  # hashicorp_vault | aws_sts | azure_managed_identity | static
    config: dict[str, Any] = Field(default_factory=dict)
