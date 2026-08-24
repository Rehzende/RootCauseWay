"""Tests for WarRoomConsumer: Redis Streams transport + summarize-and-writeback flow."""

from __future__ import annotations

import json
import uuid
from unittest.mock import AsyncMock

import pytest
from redis.exceptions import ResponseError

from app.warroom.consumer import WarRoomConsumer
from app.warroom.summarizer import WarRoomSummary


@pytest.fixture
def meeting_id():
    return str(uuid.uuid4())


@pytest.fixture
def incident_id():
    return str(uuid.uuid4())


@pytest.fixture
def org_id():
    return str(uuid.uuid4())


@pytest.fixture
def envelope_json(meeting_id, incident_id, org_id):
    envelope = {
        "event_id": str(uuid.uuid4()),
        "event_type": "warroom.meeting.ended",
        "org_id": org_id,
        "timestamp": "2026-07-02T10:00:00Z",
        "payload": {
            "meeting_id": meeting_id,
            "incident_id": incident_id,
            "external_meeting_id": "mock-meeting-1",
        },
    }
    return json.dumps(envelope)


@pytest.fixture
def stream_fields(org_id, envelope_json):
    return {
        b"org_id": str(org_id).encode(),
        b"event_type": b"warroom.meeting.ended",
        b"payload": envelope_json.encode(),
        b"published_at": b"2026-07-02T10:00:00Z",
    }


@pytest.fixture
def mock_redis():
    return AsyncMock()


@pytest.fixture
def mock_backend():
    return AsyncMock()


@pytest.fixture
def mock_summarizer():
    m = AsyncMock()
    m.summarize.return_value = WarRoomSummary(
        executive_summary="Outage resolved via rollback.",
        key_points=["502s spiked"],
        action_items=[],
    )
    return m


@pytest.fixture
def consumer(mock_redis, mock_backend, mock_summarizer):
    return WarRoomConsumer(
        mock_redis, mock_backend, summarizer=mock_summarizer, consumer_name="test-warroom-consumer",
    )


class TestEnsureGroup:
    async def test_creates_group_with_mkstream(self, consumer, mock_redis):
        await consumer._ensure_group()
        mock_redis.xgroup_create.assert_awaited_once_with(
            "rootcauseway:events", "warroom-service", id="0", mkstream=True
        )

    async def test_ignores_busygroup_error(self, consumer, mock_redis):
        mock_redis.xgroup_create.side_effect = ResponseError(
            "BUSYGROUP Consumer Group name already exists"
        )
        await consumer._ensure_group()  # must not raise

    async def test_propagates_other_errors(self, consumer, mock_redis):
        mock_redis.xgroup_create.side_effect = ResponseError("NOAUTH")
        with pytest.raises(ResponseError):
            await consumer._ensure_group()


class TestProcessEntry:
    async def test_warroom_event_calls_backend_and_summarizer(
        self, consumer, mock_redis, mock_backend, mock_summarizer, stream_fields, meeting_id,
    ):
        mock_backend.get_warroom.return_value = {
            "id": meeting_id,
            "raw_transcript": "Alice: kicking off the war room.",
            "attendance": [{"name": "Alice", "email": "alice@example.com"}],
        }

        ok = await consumer._process_entry(b"1-0", stream_fields)

        assert ok is True
        mock_backend.get_warroom.assert_awaited_once_with(meeting_id)
        mock_summarizer.summarize.assert_awaited_once_with("Alice: kicking off the war room.")
        mock_backend.update_warroom_summary.assert_awaited_once()
        call_args = mock_backend.update_warroom_summary.await_args
        assert call_args.args[0] == meeting_id
        assert call_args.args[1]["executive_summary"] == "Outage resolved via rollback."
        assert call_args.args[2] == [{"name": "Alice", "email": "alice@example.com"}]

        mock_redis.xack.assert_awaited_once_with("rootcauseway:events", "warroom-service", b"1-0")

    async def test_unhandled_event_type_is_acked_and_skipped(
        self, consumer, mock_redis, mock_backend, stream_fields,
    ):
        stream_fields[b"event_type"] = b"alert.received"

        ok = await consumer._process_entry(b"1-0", stream_fields)

        assert ok is True
        mock_backend.get_warroom.assert_not_awaited()
        mock_redis.xack.assert_awaited_once_with("rootcauseway:events", "warroom-service", b"1-0")

    async def test_backend_error_is_acked_not_retried_forever(
        self, consumer, mock_redis, mock_backend, stream_fields,
    ):
        mock_backend.get_warroom.side_effect = RuntimeError("backend unreachable")

        ok = await consumer._process_entry(b"1-0", stream_fields)

        assert ok is True  # acked regardless, to avoid infinite pending growth
        mock_redis.xack.assert_awaited_once_with("rootcauseway:events", "warroom-service", b"1-0")

    async def test_missing_meeting_id_skips_backend_call(
        self, consumer, mock_redis, mock_backend, org_id,
    ):
        envelope = {
            "event_id": str(uuid.uuid4()),
            "event_type": "warroom.meeting.ended",
            "org_id": org_id,
            "timestamp": "2026-07-02T10:00:00Z",
            "payload": {},
        }
        fields = {
            b"org_id": org_id.encode(),
            b"event_type": b"warroom.meeting.ended",
            b"payload": json.dumps(envelope).encode(),
            b"published_at": b"2026-07-02T10:00:00Z",
        }

        ok = await consumer._process_entry(b"1-0", fields)

        assert ok is True
        mock_backend.get_warroom.assert_not_awaited()
        mock_redis.xack.assert_awaited_once()


class TestConsumeOnce:
    async def test_reads_with_dedicated_consumer_group(self, consumer, mock_redis):
        mock_redis.xreadgroup.return_value = []

        processed = await consumer._consume_once()

        assert processed == 0
        mock_redis.xreadgroup.assert_awaited_once_with(
            groupname="warroom-service",
            consumername="test-warroom-consumer",
            streams={"rootcauseway:events": ">"},
            count=10,
            block=1000,
        )


class TestClaimStaleMessages:
    async def test_claims_and_processes_stale_entries(
        self, consumer, mock_redis, mock_backend, stream_fields, meeting_id,
    ):
        mock_backend.get_warroom.return_value = {"raw_transcript": "hi", "attendance": []}
        mock_redis.xautoclaim.return_value = (b"0-0", [(b"1-0", stream_fields)], [])

        claimed = await consumer._claim_stale_messages()

        assert claimed == 1
        mock_redis.xautoclaim.assert_awaited_once_with(
            "rootcauseway:events",
            "warroom-service",
            "test-warroom-consumer",
            min_idle_time=60000,
            start_id="0-0",
            count=100,
        )
        mock_backend.get_warroom.assert_awaited_once_with(meeting_id)
