"""Tests for A2AClient retry + circuit breaker behavior."""

from __future__ import annotations

import asyncio

import httpx
import pytest
import respx

from app.a2a.client import A2AClient
from app.a2a.models import Message, Role, TaskStatus, TextPart
from app.services.resilience import CircuitBreakerOpenError

AGENT_URL = "http://triage-agent:8090"


def _message() -> Message:
    return Message(role=Role.USER, parts=[TextPart(text="triage this")])


def _rpc_task_response(task_id: str = "task-1") -> httpx.Response:
    return httpx.Response(
        200,
        json={
            "jsonrpc": "2.0",
            "result": {"id": task_id, "status": "completed"},
            "id": 1,
        },
    )


def _client(**kwargs) -> A2AClient:
    defaults = dict(
        retry_attempts=3,
        retry_base_delay_seconds=0.001,
        breaker_threshold=5,
        breaker_cooldown_seconds=30.0,
    )
    defaults.update(kwargs)
    return A2AClient(**defaults)


class TestRetry:
    @pytest.mark.asyncio
    @respx.mock
    async def test_send_task_retries_after_transient_5xx(self):
        route = respx.post(f"{AGENT_URL}/").mock(
            side_effect=[
                httpx.Response(503),
                httpx.Response(502),
                _rpc_task_response(),
            ]
        )
        client = _client()
        task = await client.send_task(AGENT_URL, "task-1", _message())
        assert task.status == TaskStatus.COMPLETED
        assert route.call_count == 3
        assert client.breaker_state(AGENT_URL) == "closed"

    @pytest.mark.asyncio
    @respx.mock
    async def test_send_task_retries_after_connect_error(self):
        route = respx.post(f"{AGENT_URL}/").mock(
            side_effect=[httpx.ConnectError("refused"), _rpc_task_response()]
        )
        client = _client()
        task = await client.send_task(AGENT_URL, "task-1", _message())
        assert task.id == "task-1"
        assert route.call_count == 2

    @pytest.mark.asyncio
    @respx.mock
    async def test_no_retry_on_400(self):
        route = respx.post(f"{AGENT_URL}/").mock(return_value=httpx.Response(400))
        client = _client()
        with pytest.raises(httpx.HTTPStatusError):
            await client.send_task(AGENT_URL, "task-1", _message())
        assert route.call_count == 1
        # 4xx is a client error, not endpoint degradation: breaker stays closed
        assert client.breaker_state(AGENT_URL) == "closed"

    @pytest.mark.asyncio
    @respx.mock
    async def test_raises_after_exhausting_retries(self):
        route = respx.post(f"{AGENT_URL}/").mock(return_value=httpx.Response(500))
        client = _client(retry_attempts=3)
        with pytest.raises(httpx.HTTPStatusError):
            await client.send_task(AGENT_URL, "task-1", _message())
        assert route.call_count == 3


class TestCircuitBreaker:
    @pytest.mark.asyncio
    @respx.mock
    async def test_breaker_opens_after_threshold_and_fails_fast(self):
        route = respx.post(f"{AGENT_URL}/").mock(return_value=httpx.Response(500))
        client = _client(retry_attempts=1, breaker_threshold=2)

        for _ in range(2):
            with pytest.raises(httpx.HTTPStatusError):
                await client.send_task(AGENT_URL, "task-1", _message())

        assert client.breaker_state(AGENT_URL) == "open"

        with pytest.raises(CircuitBreakerOpenError):
            await client.send_task(AGENT_URL, "task-1", _message())
        assert route.call_count == 2  # no HTTP call while open

    @pytest.mark.asyncio
    @respx.mock
    async def test_half_open_probe_recovers(self):
        route = respx.post(f"{AGENT_URL}/").mock(
            side_effect=[httpx.Response(500), _rpc_task_response()]
        )
        client = _client(
            retry_attempts=1, breaker_threshold=1, breaker_cooldown_seconds=0.05
        )

        with pytest.raises(httpx.HTTPStatusError):
            await client.send_task(AGENT_URL, "task-1", _message())
        assert client.breaker_state(AGENT_URL) == "open"

        await asyncio.sleep(0.06)
        assert client.breaker_state(AGENT_URL) == "half_open"

        task = await client.send_task(AGENT_URL, "task-1", _message())
        assert task.status == TaskStatus.COMPLETED
        assert client.breaker_state(AGENT_URL) == "closed"
        assert route.call_count == 2

    @pytest.mark.asyncio
    @respx.mock
    async def test_breaker_is_per_endpoint(self):
        other_url = "http://rca-agent:8092"
        respx.post(f"{AGENT_URL}/").mock(return_value=httpx.Response(500))
        respx.post(f"{other_url}/").mock(return_value=_rpc_task_response("task-2"))

        client = _client(retry_attempts=1, breaker_threshold=1)

        with pytest.raises(httpx.HTTPStatusError):
            await client.send_task(AGENT_URL, "task-1", _message())
        assert client.breaker_state(AGENT_URL) == "open"

        # Healthy endpoint unaffected
        task = await client.send_task(other_url, "task-2", _message())
        assert task.id == "task-2"

        states = client.breaker_states
        assert states[AGENT_URL] == "open"
        assert states[other_url.rstrip("/")] == "closed"

    @pytest.mark.asyncio
    @respx.mock
    async def test_discover_guarded_by_breaker(self):
        respx.get(f"{AGENT_URL}/.well-known/agent.json").mock(
            return_value=httpx.Response(500)
        )
        client = _client(retry_attempts=1, breaker_threshold=1)

        with pytest.raises(httpx.HTTPStatusError):
            await client.discover(AGENT_URL)
        assert client.breaker_state(AGENT_URL) == "open"

        with pytest.raises(CircuitBreakerOpenError):
            await client.discover(AGENT_URL)
