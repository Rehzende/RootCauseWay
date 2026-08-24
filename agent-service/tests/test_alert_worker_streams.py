"""Tests for AlertWorker Redis Streams transport (consumer group, ack, retry, DLQ, autoclaim)."""

from __future__ import annotations

import json
import uuid
from datetime import datetime, timezone
from unittest.mock import AsyncMock

import pytest
from redis.exceptions import ResponseError

from app.workers.alert_worker import AlertWorker


@pytest.fixture(autouse=True)
def fast_backoff(monkeypatch):
    """Disable retry sleep so tests run instantly."""
    monkeypatch.setenv("EVENT_RETRY_BACKOFF_BASE", "0")


@pytest.fixture
def org_id():
    return uuid.uuid4()


@pytest.fixture
def envelope_json(org_id):
    envelope = {
        "event_id": str(uuid.uuid4()),
        "event_type": "alert.received",
        "org_id": str(org_id),
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "payload": {
            "alert_snapshot_id": str(uuid.uuid4()),
            "incident_id": str(uuid.uuid4()),
            "software_id": str(uuid.uuid4()),
            "webhook_source": "datadog",
            "normalized_alert": {
                "title": "High CPU Usage",
                "severity": "critical",
                "source": "datadog",
                "service": "api-gateway",
            },
        },
    }
    return json.dumps(envelope)


@pytest.fixture
def stream_fields(org_id, envelope_json):
    """Fields as XADDed by the Go backend (bytes, as returned by redis-py)."""
    return {
        b"org_id": str(org_id).encode(),
        b"event_type": b"alert.received",
        b"payload": envelope_json.encode(),
        b"published_at": datetime.now(timezone.utc).isoformat().encode(),
    }


@pytest.fixture
def mock_redis():
    return AsyncMock()


@pytest.fixture
def worker(mock_redis):
    w = AlertWorker(mock_redis, AsyncMock(), AsyncMock(), consumer_name="test-consumer")
    w._handle_message = AsyncMock()
    return w


# ---- Consumer group setup ----


class TestEnsureGroup:
    async def test_creates_group_with_mkstream(self, worker, mock_redis):
        await worker._ensure_group()
        mock_redis.xgroup_create.assert_awaited_once_with(
            "rootcauseway:events", "agent-service", id="0", mkstream=True
        )

    async def test_ignores_busygroup_error(self, worker, mock_redis):
        mock_redis.xgroup_create.side_effect = ResponseError(
            "BUSYGROUP Consumer Group name already exists"
        )
        await worker._ensure_group()  # must not raise

    async def test_propagates_other_errors(self, worker, mock_redis):
        mock_redis.xgroup_create.side_effect = ResponseError("NOAUTH")
        with pytest.raises(ResponseError):
            await worker._ensure_group()


# ---- Ack on success ----


class TestProcessEntry:
    async def test_success_acks_and_dispatches(self, worker, mock_redis, stream_fields, envelope_json):
        ok = await worker._process_entry(b"1-0", stream_fields)

        assert ok is True
        worker._handle_message.assert_awaited_once()
        message = worker._handle_message.await_args.args[0]
        # The stream payload carries the same envelope JSON as the old pub/sub message.
        assert json.loads(message["data"]) == json.loads(envelope_json)
        mock_redis.xack.assert_awaited_once_with("rootcauseway:events", "agent-service", b"1-0")
        mock_redis.xadd.assert_not_awaited()  # nothing dead-lettered

    async def test_unhandled_event_type_is_acked_and_skipped(self, worker, mock_redis, stream_fields):
        stream_fields[b"event_type"] = b"agent.status"

        ok = await worker._process_entry(b"1-0", stream_fields)

        assert ok is True
        worker._handle_message.assert_not_awaited()
        mock_redis.xack.assert_awaited_once()

    async def test_retries_then_succeeds_without_dlq(self, worker, mock_redis, stream_fields):
        worker._handle_message.side_effect = [RuntimeError("boom"), RuntimeError("boom"), None]

        ok = await worker._process_entry(b"1-0", stream_fields)

        assert ok is True
        assert worker._handle_message.await_count == 3
        mock_redis.xack.assert_awaited_once()
        mock_redis.xadd.assert_not_awaited()

    async def test_exhausted_retries_dead_letters_and_acks(self, worker, mock_redis, stream_fields):
        worker._handle_message.side_effect = RuntimeError("permanent failure")

        ok = await worker._process_entry(b"1-0", stream_fields)

        assert ok is False
        # initial attempt + 3 retries (event_max_retries default)
        assert worker._handle_message.await_count == 4

        mock_redis.xadd.assert_awaited_once()
        dlq_call = mock_redis.xadd.await_args
        assert dlq_call.args[0] == "rootcauseway:events:dlq"
        dlq_fields = dlq_call.args[1]
        assert dlq_fields["error"] == "permanent failure"
        assert dlq_fields["error_type"] == "RuntimeError"
        assert dlq_fields["original_entry_id"] == "1-0"
        assert dlq_fields["consumer"] == "test-consumer"
        assert dlq_fields["retries"] == "3"
        assert dlq_fields["event_type"] == "alert.received"
        assert "failed_at" in dlq_fields
        assert dlq_call.kwargs.get("maxlen") == 10000
        assert dlq_call.kwargs.get("approximate") is True

        # Original entry acked so it doesn't stay pending forever.
        mock_redis.xack.assert_awaited_once_with("rootcauseway:events", "agent-service", b"1-0")

    async def test_dlq_write_failure_still_acks(self, worker, mock_redis, stream_fields):
        worker._handle_message.side_effect = RuntimeError("boom")
        mock_redis.xadd.side_effect = ConnectionError("redis down")

        ok = await worker._process_entry(b"1-0", stream_fields)

        assert ok is False
        mock_redis.xack.assert_awaited_once()


# ---- Batch consumption ----


class TestConsumeOnce:
    async def test_reads_with_consumer_group(self, worker, mock_redis):
        mock_redis.xreadgroup.return_value = []

        processed = await worker._consume_once()

        assert processed == 0
        mock_redis.xreadgroup.assert_awaited_once_with(
            groupname="agent-service",
            consumername="test-consumer",
            streams={"rootcauseway:events": ">"},
            count=10,
            block=1000,
        )

    async def test_processes_all_entries_in_batch(self, worker, mock_redis, stream_fields):
        mock_redis.xreadgroup.return_value = [
            (b"rootcauseway:events", [(b"1-0", stream_fields), (b"1-1", stream_fields)]),
        ]

        processed = await worker._consume_once()

        assert processed == 2
        assert worker._handle_message.await_count == 2
        assert mock_redis.xack.await_count == 2


# ---- Startup recovery (XAUTOCLAIM) ----


class TestClaimStaleMessages:
    async def test_claims_and_processes_stale_entries(self, worker, mock_redis, stream_fields):
        mock_redis.xautoclaim.return_value = (b"0-0", [(b"1-0", stream_fields)], [])

        claimed = await worker._claim_stale_messages()

        assert claimed == 1
        mock_redis.xautoclaim.assert_awaited_once_with(
            "rootcauseway:events",
            "agent-service",
            "test-consumer",
            min_idle_time=60000,
            start_id="0-0",
            count=100,
        )
        worker._handle_message.assert_awaited_once()
        mock_redis.xack.assert_awaited_once()

    async def test_no_pending_entries(self, worker, mock_redis):
        mock_redis.xautoclaim.return_value = (b"0-0", [], [])

        claimed = await worker._claim_stale_messages()

        assert claimed == 0
        worker._handle_message.assert_not_awaited()

    async def test_acks_trimmed_entries_with_no_fields(self, worker, mock_redis):
        mock_redis.xautoclaim.return_value = (b"0-0", [(b"1-0", None)], [])

        claimed = await worker._claim_stale_messages()

        assert claimed == 0
        worker._handle_message.assert_not_awaited()
        mock_redis.xack.assert_awaited_once()

    async def test_paginates_until_cursor_wraps(self, worker, mock_redis, stream_fields):
        mock_redis.xautoclaim.side_effect = [
            (b"5-0", [(b"1-0", stream_fields)], []),
            (b"0-0", [(b"5-0", stream_fields)], []),
        ]

        claimed = await worker._claim_stale_messages()

        assert claimed == 2
        assert mock_redis.xautoclaim.await_count == 2
        # Second call resumes from the returned cursor.
        assert mock_redis.xautoclaim.await_args_list[1].kwargs["start_id"] == b"5-0"


# ---- Configuration ----


class TestConfiguration:
    async def test_settings_override_via_env(self, monkeypatch, mock_redis):
        monkeypatch.setenv("EVENT_STREAM_NAME", "custom:stream")
        monkeypatch.setenv("EVENT_CONSUMER_GROUP", "custom-group")
        monkeypatch.setenv("EVENT_DLQ_STREAM", "custom:dlq")
        monkeypatch.setenv("EVENT_MAX_RETRIES", "1")
        monkeypatch.setenv("EVENT_AUTOCLAIM_IDLE_MS", "5000")

        w = AlertWorker(mock_redis, AsyncMock(), AsyncMock(), consumer_name="c1")

        assert w._stream == "custom:stream"
        assert w._group == "custom-group"
        assert w._dlq_stream == "custom:dlq"
        assert w._max_retries == 1
        assert w._autoclaim_idle_ms == 5000

    async def test_consumer_name_defaults_to_host_and_pid(self, mock_redis):
        import os
        import socket

        w = AlertWorker(mock_redis, AsyncMock(), AsyncMock())

        assert socket.gethostname() in w._consumer
        assert str(os.getpid()) in w._consumer
