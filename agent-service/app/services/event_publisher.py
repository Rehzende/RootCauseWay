"""Publishes events to Redis channels."""

from __future__ import annotations

import json
import uuid
from datetime import datetime, timezone
from typing import Any

import redis.asyncio as redis

from app.models.events import (
    AgentStatusPayload,
    EventEnvelope,
    EvidenceCollectedPayload,
    HypothesisGeneratedPayload,
    TriageCompletedPayload,
)


class EventPublisher:
    def __init__(self, redis_client: redis.Redis):
        self._redis = redis_client

    def _build_envelope(self, event_type: str, org_id: uuid.UUID, payload: dict[str, Any]) -> str:
        envelope = EventEnvelope(
            event_id=uuid.uuid4(),
            event_type=event_type,
            org_id=org_id,
            timestamp=datetime.now(timezone.utc),
            payload=payload,
        )
        return envelope.model_dump_json()

    async def publish_triage_completed(self, org_id: uuid.UUID, payload: TriageCompletedPayload) -> None:
        channel = f"rootcauseway:{org_id}:triage.completed"
        msg = self._build_envelope("triage.completed", org_id, payload.model_dump(mode="json"))
        await self._redis.publish(channel, msg)

    async def publish_evidence_collected(self, org_id: uuid.UUID, payload: EvidenceCollectedPayload) -> None:
        channel = f"rootcauseway:{org_id}:evidence.collected"
        msg = self._build_envelope("evidence.collected", org_id, payload.model_dump(mode="json"))
        await self._redis.publish(channel, msg)

    async def publish_hypothesis_generated(self, org_id: uuid.UUID, payload: HypothesisGeneratedPayload) -> None:
        channel = f"rootcauseway:{org_id}:hypothesis.generated"
        msg = self._build_envelope("hypothesis.generated", org_id, payload.model_dump(mode="json"))
        await self._redis.publish(channel, msg)

    async def publish_agent_status(self, org_id: uuid.UUID, payload: AgentStatusPayload) -> None:
        channel = f"rootcauseway:{org_id}:agent.status"
        msg = self._build_envelope("agent.status", org_id, payload.model_dump(mode="json"))
        await self._redis.publish(channel, msg)
