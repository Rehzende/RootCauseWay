"""Coverage for AlertWorker's stale-event backpressure guard.

A single shared local LLM is the real capacity bottleneck under sustained
load (observed live during the platform audit: a 122-message stream
backlog accumulated from a single day's worth of testing, each event
taking 1-3 minutes of LLM time to process). An event that's been sitting in
the queue for a long time by the time it's dequeued is skipped rather than
burning that capacity on something likely already resolved/superseded.
"""

from __future__ import annotations

import json
import uuid
from datetime import datetime, timedelta, timezone
from unittest.mock import AsyncMock

import pytest

from app.observability.metrics import ROOTCAUSEWAY_STALE_EVENTS_SKIPPED_TOTAL
from app.workers.alert_worker import AlertWorker


@pytest.fixture
def worker(monkeypatch):
    monkeypatch.setenv("EVENT_RETRY_BACKOFF_BASE", "0")
    return AlertWorker(AsyncMock(), AsyncMock(), AsyncMock())


def _envelope(event_type: str, age_seconds: float) -> dict:
    ts = datetime.now(timezone.utc) - timedelta(seconds=age_seconds)
    return {
        "event_id": str(uuid.uuid4()),
        "event_type": event_type,
        "org_id": str(uuid.uuid4()),
        "timestamp": ts.isoformat(),
        "payload": {"incident_id": str(uuid.uuid4())},
    }


def test_is_stale_true_past_threshold(worker):
    worker._backend  # sanity: fixture constructed
    data = _envelope("incident.resolved", age_seconds=1000)  # > default 900s
    assert worker._is_stale(data) is True


def test_is_stale_false_within_threshold(worker):
    data = _envelope("incident.resolved", age_seconds=10)
    assert worker._is_stale(data) is False


def test_is_stale_false_when_threshold_disabled(worker, monkeypatch):
    # get_settings() constructs a fresh Settings() per call (no caching),
    # so the env var takes effect immediately.
    monkeypatch.setenv("STALE_EVENT_THRESHOLD_SECONDS", "0")
    data = _envelope("incident.resolved", age_seconds=100000)
    assert worker._is_stale(data) is False


def test_is_stale_false_on_missing_timestamp(worker):
    """Fail open: a malformed/missing timestamp must not silently drop a
    real event."""
    data = {"event_type": "incident.resolved", "payload": {}}
    assert worker._is_stale(data) is False


@pytest.mark.asyncio
async def test_handle_message_skips_stale_event_and_records_metric(worker):
    before = ROOTCAUSEWAY_STALE_EVENTS_SKIPPED_TOTAL.labels(event_type="incident.resolved")._value.get()

    envelope = _envelope("incident.resolved", age_seconds=1000)
    await worker._handle_message({"data": json.dumps(envelope)})

    # Must not have touched the backend at all -- skipped before any real work.
    worker._backend.get_incident.assert_not_awaited()

    after = ROOTCAUSEWAY_STALE_EVENTS_SKIPPED_TOTAL.labels(event_type="incident.resolved")._value.get()
    assert after == before + 1
