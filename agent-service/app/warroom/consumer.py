"""Redis Streams consumer for warroom.meeting.created/warroom.meeting.ended.

Mirrors the transport conventions of app/workers/alert_worker.py (durable
``rootcauseway:events`` stream, XREADGROUP with a consumer group, XACK on success,
XAUTOCLAIM recovery on startup) but under its own dedicated consumer group
(``warroom_consumer_group``, default "warroom-service") so it doesn't
compete with AlertWorker's consumption of the same stream. Event types
other than the two handled here are XACKed and skipped, exactly like
AlertWorker skips event types it doesn't handle.

On a warroom.meeting.created message: notify the org's configured channels
(Slack/Teams/webhook/PagerDuty) with the join link, reusing the same
NotificationDispatcher/notification_channels/escalation_policies path
incident_created and rca_completed already go through -- before this, the
join_url only ever became an incident timeline event, never an active
notification.

On a warroom.meeting.ended message: fetch the meeting (with its raw
transcript, already captured by the Go backend's EndWarRoom) from the
backend, summarize the transcript with an LLM, and write the summary +
participant list back via PATCH /internal/warroom/:meetingId/summary.
"""

from __future__ import annotations

import asyncio
import json
import logging
import os
import socket
from typing import Any
from uuid import UUID

import redis.asyncio as redis
from redis.exceptions import ResponseError

from app.config.settings import get_settings
from app.notifications.dispatcher import NotificationDispatcher
from app.services.backend_client import BackendClient
from app.warroom.summarizer import WarRoomSummarizer

logger = logging.getLogger(__name__)


def _as_str(value: Any) -> str:
    """Decode redis bytes responses to str (client may not use decode_responses)."""
    if isinstance(value, bytes):
        return value.decode("utf-8", errors="replace")
    return str(value)


class WarRoomConsumer:
    HANDLED_EVENT_TYPES = frozenset({"warroom.meeting.created", "warroom.meeting.ended"})

    def __init__(
        self,
        redis_client: redis.Redis,
        backend_client: BackendClient,
        summarizer: WarRoomSummarizer | None = None,
        notifier: NotificationDispatcher | None = None,
        consumer_name: str | None = None,
    ):
        self._redis = redis_client
        self._backend = backend_client
        self._summarizer = summarizer or WarRoomSummarizer()
        self._notifier = notifier or NotificationDispatcher()
        self._running = False

        settings = get_settings()
        self._stream = settings.event_stream_name
        self._group = settings.warroom_consumer_group
        self._autoclaim_idle_ms = settings.warroom_autoclaim_idle_ms
        self._poll_interval_ms = settings.warroom_poll_interval_ms
        # Unique per instance so multiple replicas can share the consumer group.
        self._consumer = consumer_name or f"{socket.gethostname()}-{os.getpid()}-warroom"

    async def start(self) -> None:
        """Consume warroom.meeting.ended events from the durable Redis Stream."""
        self._running = True
        await self._ensure_group()

        try:
            claimed = await self._claim_stale_messages()
            if claimed:
                logger.info("WarRoomConsumer recovered %d stale pending stream entries", claimed)
        except Exception:
            logger.exception("Failed to autoclaim stale pending entries on startup")

        logger.info(
            "WarRoomConsumer started: stream=%s group=%s consumer=%s",
            self._stream, self._group, self._consumer,
        )

        while self._running:
            try:
                await self._consume_once()
            except asyncio.CancelledError:
                raise
            except Exception:
                logger.exception("Error reading from event stream in WarRoomConsumer")
                await asyncio.sleep(1.0)

    async def stop(self) -> None:
        self._running = False
        await self._summarizer.close()
        await self._notifier.close()

    # ---- Redis Streams transport ----

    async def _ensure_group(self) -> None:
        try:
            await self._redis.xgroup_create(self._stream, self._group, id="0", mkstream=True)
            logger.info("Created consumer group %s on stream %s", self._group, self._stream)
        except ResponseError as exc:
            if "BUSYGROUP" not in str(exc):
                raise

    async def _consume_once(self) -> int:
        response = await self._redis.xreadgroup(
            groupname=self._group,
            consumername=self._consumer,
            streams={self._stream: ">"},
            count=10,
            block=self._poll_interval_ms,
        )
        if not response:
            return 0

        processed = 0
        for _stream_name, entries in response:
            for entry_id, fields in entries:
                await self._process_entry(entry_id, fields)
                processed += 1
        return processed

    async def _claim_stale_messages(self) -> int:
        start_id = "0-0"
        claimed = 0
        while True:
            result = await self._redis.xautoclaim(
                self._stream,
                self._group,
                self._consumer,
                min_idle_time=self._autoclaim_idle_ms,
                start_id=start_id,
                count=100,
            )
            next_id, entries = result[0], result[1]
            for entry_id, fields in entries:
                if not fields:
                    await self._redis.xack(self._stream, self._group, entry_id)
                    continue
                await self._process_entry(entry_id, fields)
                claimed += 1
            if not entries or _as_str(next_id) == "0-0":
                break
            start_id = next_id
        return claimed

    async def _process_entry(self, entry_id: Any, fields: dict[Any, Any]) -> bool:
        decoded = {_as_str(k): _as_str(v) for k, v in fields.items()}
        payload_json = decoded.get("payload", "")
        event_type = decoded.get("event_type", "")
        org_id = decoded.get("org_id", "")

        if event_type not in self.HANDLED_EVENT_TYPES:
            await self._redis.xack(self._stream, self._group, entry_id)
            return True

        try:
            if event_type == "warroom.meeting.created":
                await self._handle_created_message(payload_json, org_id)
            else:
                await self._handle_ended_message(payload_json)
        except Exception:
            logger.exception("Error processing %s entry %s", event_type, _as_str(entry_id))
            # Ack anyway: a failed summarization/notification shouldn't be
            # retried forever. The meeting/incident state itself is
            # unaffected either way -- this consumer only reacts to it.
        await self._redis.xack(self._stream, self._group, entry_id)
        return True

    # ---- Message handling ----

    async def _handle_created_message(self, payload_json: str, org_id: str) -> None:
        envelope = json.loads(payload_json)
        payload = envelope.get("payload", {})
        incident_id = payload.get("incident_id")
        if not incident_id or not org_id:
            logger.warning("warroom.meeting.created event missing incident_id/org_id: %s", envelope)
            return

        logger.info("Notifying channels for war room created on incident %s", incident_id)
        await self._notifier.notify(
            self._backend,
            UUID(org_id),
            UUID(incident_id),
            "war_room_created",
            {
                "incident_id": incident_id,
                "severity": payload.get("severity", "medium"),
                "join_url": payload.get("join_url", ""),
            },
        )

    async def _handle_ended_message(self, payload_json: str) -> None:
        envelope = json.loads(payload_json)
        payload = envelope.get("payload", {})
        meeting_id = payload.get("meeting_id")
        if not meeting_id:
            logger.warning("warroom.meeting.ended event missing meeting_id: %s", envelope)
            return

        logger.info("Summarizing war room meeting %s", meeting_id)

        meeting = await self._backend.get_warroom(meeting_id)
        transcript = meeting.get("raw_transcript") or ""

        summary = await self._summarizer.summarize(transcript)

        attendance = meeting.get("attendance") or []
        participants = attendance if isinstance(attendance, list) else []

        await self._backend.update_warroom_summary(
            meeting_id,
            summary.model_dump(mode="json"),
            participants,
        )

        logger.info("War room meeting %s summarized and written back", meeting_id)
