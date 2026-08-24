"""Redis Streams consumer for the ``warroom.meeting.ended`` event.

Mirrors the transport conventions of app/workers/alert_worker.py (durable
``rootcauseway:events`` stream, XREADGROUP with a consumer group, XACK on success,
XAUTOCLAIM recovery on startup) but under its own dedicated consumer group
(``warroom_consumer_group``, default "warroom-service") so it doesn't
compete with AlertWorker's consumption of the same stream. Event types
other than warroom.meeting.ended are XACKed and skipped, exactly like
AlertWorker skips event types it doesn't handle.

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

import redis.asyncio as redis
from redis.exceptions import ResponseError

from app.config.settings import get_settings
from app.services.backend_client import BackendClient
from app.warroom.summarizer import WarRoomSummarizer

logger = logging.getLogger(__name__)


def _as_str(value: Any) -> str:
    """Decode redis bytes responses to str (client may not use decode_responses)."""
    if isinstance(value, bytes):
        return value.decode("utf-8", errors="replace")
    return str(value)


class WarRoomConsumer:
    HANDLED_EVENT_TYPES = frozenset({"warroom.meeting.ended"})

    def __init__(
        self,
        redis_client: redis.Redis,
        backend_client: BackendClient,
        summarizer: WarRoomSummarizer | None = None,
        consumer_name: str | None = None,
    ):
        self._redis = redis_client
        self._backend = backend_client
        self._summarizer = summarizer or WarRoomSummarizer()
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

        if event_type not in self.HANDLED_EVENT_TYPES:
            await self._redis.xack(self._stream, self._group, entry_id)
            return True

        try:
            await self._handle_message(payload_json)
        except Exception:
            logger.exception("Error processing warroom.meeting.ended entry %s", _as_str(entry_id))
            # Ack anyway: a failed summarization shouldn't be retried forever
            # against an unchanging transcript. The meeting stays in "ended"
            # status and can be retried manually via the internal endpoint.
        await self._redis.xack(self._stream, self._group, entry_id)
        return True

    # ---- Message handling ----

    async def _handle_message(self, payload_json: str) -> None:
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
