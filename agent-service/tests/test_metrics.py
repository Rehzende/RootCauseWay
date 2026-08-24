"""Tests for app.observability.metrics: A2A call metrics, stream queue depth
gauge, and the /metrics ASGI route."""

from __future__ import annotations

import asyncio

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.observability.metrics import (
    A2A_CALLS_TOTAL,
    A2A_CALL_DURATION_SECONDS,
    A2A_OUTCOME_FAILURE,
    A2A_OUTCOME_SUCCESS,
    STREAM_PENDING_MESSAGES,
    StreamQueueDepthUpdater,
    metrics_app,
    metrics_text,
    record_a2a_call,
)


def _counter_value(counter, **labels) -> float:
    return counter.labels(**labels)._value.get()


def _gauge_value(gauge, **labels) -> float:
    return gauge.labels(**labels)._value.get()


class TestRecordA2ACall:
    def test_success_increments_counter_and_observes_duration(self):
        agent_url = "http://triage-agent:8090"
        before = _counter_value(A2A_CALLS_TOTAL, agent_url=agent_url, outcome=A2A_OUTCOME_SUCCESS)

        record_a2a_call(agent_url, 0.25, A2A_OUTCOME_SUCCESS)

        after = _counter_value(A2A_CALLS_TOTAL, agent_url=agent_url, outcome=A2A_OUTCOME_SUCCESS)
        assert after == before + 1

        sample_sum = A2A_CALL_DURATION_SECONDS.labels(
            agent_url=agent_url, outcome=A2A_OUTCOME_SUCCESS
        )._sum.get()
        assert sample_sum >= 0.25

    def test_failure_and_circuit_open_are_separately_labeled(self):
        agent_url = "http://rca-agent:8092"
        before_fail = _counter_value(A2A_CALLS_TOTAL, agent_url=agent_url, outcome=A2A_OUTCOME_FAILURE)

        record_a2a_call(agent_url, 0.1, A2A_OUTCOME_FAILURE)
        record_a2a_call(agent_url, 0.05, "circuit_open")

        after_fail = _counter_value(A2A_CALLS_TOTAL, agent_url=agent_url, outcome=A2A_OUTCOME_FAILURE)
        circuit_open_value = _counter_value(A2A_CALLS_TOTAL, agent_url=agent_url, outcome="circuit_open")

        assert after_fail == before_fail + 1
        assert circuit_open_value >= 1


class TestStreamQueueDepthUpdater:
    @pytest.mark.asyncio
    async def test_poll_once_updates_gauge_from_mocked_xlen(self):
        class FakeRedis:
            def __init__(self, length: int):
                self._length = length
                self.closed = False

            async def xlen(self, stream_name: str) -> int:
                assert stream_name == "rootcauseway:events:test"
                return self._length

            async def aclose(self):
                self.closed = True

        fake = FakeRedis(42)
        updater = StreamQueueDepthUpdater(
            "redis://unused:6379/0",
            "rootcauseway:events:test",
            redis_factory=lambda _url: fake,
        )
        await updater.start()
        try:
            # start() already triggers the loop task; give it a tick to run
            # the first poll, then assert directly via _poll_once for
            # determinism instead of relying on the sleep interval.
            await updater._poll_once()
        finally:
            await updater.stop()

        assert fake.closed is True
        assert _gauge_value(STREAM_PENDING_MESSAGES, stream="rootcauseway:events:test") == 42

    @pytest.mark.asyncio
    async def test_poll_once_swallows_errors(self):
        class FailingRedis:
            async def xlen(self, stream_name: str) -> int:
                raise ConnectionError("boom")

            async def aclose(self):
                pass

        updater = StreamQueueDepthUpdater(
            "redis://unused:6379/0",
            "rootcauseway:events:failing",
            redis_factory=lambda _url: FailingRedis(),
        )
        await updater.start()
        try:
            # Should not raise even though xlen() fails.
            await updater._poll_once()
        finally:
            await updater.stop()

    @pytest.mark.asyncio
    async def test_start_is_idempotent_and_stop_cancels_task(self):
        class FakeRedis:
            async def xlen(self, stream_name: str) -> int:
                return 1

            async def aclose(self):
                pass

        updater = StreamQueueDepthUpdater(
            "redis://unused:6379/0",
            "rootcauseway:events:idem",
            interval_seconds=0.01,
            redis_factory=lambda _url: FakeRedis(),
        )
        await updater.start()
        first_task = updater._task
        await updater.start()  # no-op, already running
        assert updater._task is first_task

        await asyncio.sleep(0.03)
        await updater.stop()
        assert updater._task is None


class TestMetricsRoute:
    def test_metrics_asgi_route_returns_200_with_expected_metric_names(self):
        # Mount the same metrics_app used by app.main onto a minimal FastAPI
        # app, rather than importing app.main directly -- app.main's
        # lifespan starts the alert worker / war room consumer against a
        # real Redis connection, which this unit test should not depend on.
        test_app = FastAPI()
        test_app.mount("/metrics", metrics_app)

        record_a2a_call("http://triage-agent:8090", 0.1, A2A_OUTCOME_SUCCESS)

        client = TestClient(test_app)
        resp = client.get("/metrics")

        assert resp.status_code == 200
        body = resp.text
        assert "rootcauseway_a2a_call_duration_seconds" in body
        assert "rootcauseway_a2a_calls_total" in body
        assert "rootcauseway_stream_pending_messages" in body

    def test_metrics_text_helper_returns_expected_content_type(self):
        payload, content_type = metrics_text()
        assert b"rootcauseway_a2a_calls_total" in payload
        assert "text/plain" in content_type
