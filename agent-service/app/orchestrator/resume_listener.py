"""Resume listener: consumes pipeline.stage_approved events and resumes the
HITL-gated postmortem stage.

Standalone Redis pub/sub subscriber -- deliberately independent from
app/workers/alert_worker.py's durable-stream consumer-group machinery
(XREADGROUP/XACK/retries/DLQ/etc.). It does not import from or modify that
module. It subscribes directly to the pub/sub channel pattern
"rootcauseway:*:pipeline.stage_approved" rather than reading the "rootcauseway:events"
stream, because:

  - Every backend-go event (including pipeline.stage_approved, published by
    backend/internal/handlers/pipeline_gate_handlers.go:ApproveStage) is
    dual-written to both the durable stream AND the pub/sub channel (see
    backend/internal/database/stream_publisher.go:publishDual), so pub/sub
    alone is sufficient here without losing the event.
  - Approval is a rare, explicitly human-triggered action. If this listener
    is briefly down when an operator clicks "approve," the incident simply
    stays in its "awaiting_approval_stage=postmortem" DB state (nothing is
    lost) until the listener is back up and the operator retries -- so
    pub/sub's fire-and-forget semantics are an acceptable tradeoff for the
    simplicity of not reimplementing alert_worker's consumer-group logic.

If stronger at-least-once delivery is needed later, this can be upgraded to
XREADGROUP the "rootcauseway:events" stream with its own consumer group (name:
"pipeline-gate-service", see contracts/events/redis-events.yaml) instead of
pub/sub; the event parsing / resume logic below would be unchanged.
"""

from __future__ import annotations

import asyncio
import json
import logging
import uuid
from typing import Any

import redis.asyncio as redis

from app.orchestrator.orchestrator import Orchestrator

logger = logging.getLogger(__name__)

EVENT_TYPE = "pipeline.stage_approved"
CHANNEL_PATTERN = "rootcauseway:*:pipeline.stage_approved"
# Only relevant if this listener is later upgraded to consume the durable
# "rootcauseway:events" stream instead of pub/sub -- kept here so the name is
# defined in one place ahead of that migration.
CONSUMER_GROUP_NAME = "pipeline-gate-service"


class ResumeListener:
    """Subscribes to pipeline.stage_approved and resumes the postmortem
    stage for the approved incident via Orchestrator.run_postmortem_only().
    """

    def __init__(self, redis_client: redis.Redis, orchestrator: Orchestrator):
        self._redis = redis_client
        self._orchestrator = orchestrator
        self._running = False
        self._pubsub: Any = None

    async def start(self) -> None:
        """Subscribe and process events until stop() is called.

        Intended to be run as a background asyncio task (see app/main.py),
        mirroring how AlertWorker.start() is scheduled.
        """
        self._running = True
        self._pubsub = self._redis.pubsub()
        await self._pubsub.psubscribe(CHANNEL_PATTERN)
        logger.info("ResumeListener subscribed to %s", CHANNEL_PATTERN)

        try:
            while self._running:
                try:
                    message = await self._pubsub.get_message(
                        ignore_subscribe_messages=True, timeout=1.0,
                    )
                except asyncio.CancelledError:
                    raise
                except Exception:
                    logger.exception("ResumeListener: error reading pub/sub message")
                    await asyncio.sleep(1.0)
                    continue

                if message is None:
                    continue

                try:
                    await self._handle_message(message)
                except Exception:
                    logger.exception("ResumeListener: error handling message: %s", message)
        finally:
            if self._pubsub is not None:
                try:
                    await self._pubsub.punsubscribe(CHANNEL_PATTERN)
                    await self._pubsub.aclose()
                except Exception:
                    logger.warning("ResumeListener: error closing pub/sub connection")

    async def stop(self) -> None:
        self._running = False

    async def _handle_message(self, message: dict[str, Any]) -> None:
        """Parse a raw pub/sub message and, if it's a pipeline.stage_approved
        event for the postmortem stage, resume that incident's pipeline."""
        raw = message.get("data")
        if raw is None:
            return
        if isinstance(raw, bytes):
            raw = raw.decode("utf-8", errors="replace")
        if not isinstance(raw, str):
            return

        try:
            envelope = json.loads(raw)
        except (json.JSONDecodeError, TypeError):
            logger.warning("ResumeListener: could not decode message payload: %r", raw)
            return

        if not isinstance(envelope, dict) or envelope.get("event_type") != EVENT_TYPE:
            return

        payload = envelope.get("payload") or {}
        incident_id_raw = payload.get("incident_id")
        org_id_raw = envelope.get("org_id")
        stage = payload.get("stage", "postmortem")

        if not incident_id_raw:
            logger.warning(
                "ResumeListener: %s event missing incident_id: %s", EVENT_TYPE, payload,
            )
            return

        try:
            incident_id = uuid.UUID(str(incident_id_raw))
        except (ValueError, AttributeError):
            logger.warning(
                "ResumeListener: invalid incident_id in %s event: %r",
                EVENT_TYPE, incident_id_raw,
            )
            return

        try:
            org_id = uuid.UUID(str(org_id_raw))
        except (ValueError, AttributeError, TypeError):
            logger.warning(
                "ResumeListener: missing/invalid org_id in %s event for incident %s: %r",
                EVENT_TYPE, incident_id, org_id_raw,
            )
            return

        if stage != "postmortem":
            logger.info(
                "ResumeListener: ignoring approved stage %r for incident %s "
                "(only postmortem is resumable today)", stage, incident_id,
            )
            return

        logger.info(
            "ResumeListener: resuming postmortem for incident %s after human approval",
            incident_id,
        )
        try:
            result = await self._orchestrator.run_postmortem_only(incident_id, org_id)
            logger.info(
                "ResumeListener: postmortem resumed for incident %s (status=%s)",
                incident_id, result.get("status"),
            )
        except Exception:
            logger.exception(
                "ResumeListener: failed to resume postmortem for incident %s", incident_id,
            )
