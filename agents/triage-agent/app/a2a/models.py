"""Pydantic models for Google A2A Protocol."""

from __future__ import annotations

from enum import Enum
from typing import Any, Literal
from uuid import UUID

from pydantic import BaseModel, Field


# --- Agent Card ---


class AgentSkill(BaseModel):
    id: str
    name: str
    description: str | None = None
    input_modes: list[str] = Field(default_factory=lambda: ["application/json"], alias="inputModes")
    output_modes: list[str] = Field(default_factory=lambda: ["application/json"], alias="outputModes")
    examples: list[str] = Field(default_factory=list)

    model_config = {"populate_by_name": True}


class AgentCapabilities(BaseModel):
    streaming: bool = False
    push_notifications: bool = Field(default=False, alias="pushNotifications")
    state_transition_history: bool = Field(default=True, alias="stateTransitionHistory")

    model_config = {"populate_by_name": True}


class AgentAuthentication(BaseModel):
    schemes: list[str] = Field(default_factory=lambda: ["none"])


class AgentCard(BaseModel):
    name: str
    description: str | None = None
    url: str
    version: str
    capabilities: AgentCapabilities = Field(default_factory=AgentCapabilities)
    authentication: AgentAuthentication = Field(default_factory=AgentAuthentication)
    default_input_modes: list[str] = Field(
        default_factory=lambda: ["application/json"], alias="defaultInputModes"
    )
    default_output_modes: list[str] = Field(
        default_factory=lambda: ["application/json"], alias="defaultOutputModes"
    )
    skills: list[AgentSkill] = Field(default_factory=list)

    model_config = {"populate_by_name": True}


# --- Task ---


class TaskStatus(str, Enum):
    SUBMITTED = "submitted"
    WORKING = "working"
    INPUT_REQUIRED = "input-required"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELED = "canceled"


class Role(str, Enum):
    USER = "user"
    AGENT = "agent"


# --- Parts ---


class TextPart(BaseModel):
    type: Literal["text"] = "text"
    text: str


class DataPart(BaseModel):
    type: Literal["data"] = "data"
    data: dict[str, Any]


class FilePart(BaseModel):
    type: Literal["file"] = "file"
    file: dict[str, Any]  # name, mimeType, bytes


Part = TextPart | DataPart | FilePart


class Message(BaseModel):
    role: Role
    parts: list[Part]


class Artifact(BaseModel):
    name: str
    description: str | None = None
    parts: list[Part]


class Task(BaseModel):
    id: str
    status: TaskStatus = TaskStatus.SUBMITTED
    message: Message | None = None
    artifacts: list[Artifact] = Field(default_factory=list)


# --- JSON-RPC ---


class JSONRPCRequest(BaseModel):
    jsonrpc: str = "2.0"
    method: str
    params: dict[str, Any] | None = None
    id: str | int | None = None


class JSONRPCError(BaseModel):
    code: int
    message: str
    data: Any | None = None


class JSONRPCResponse(BaseModel):
    jsonrpc: str = "2.0"
    result: Any | None = None
    error: JSONRPCError | None = None
    id: str | int | None = None
