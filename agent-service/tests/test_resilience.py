"""Tests for app.services.resilience (retry + circuit breaker)."""

from __future__ import annotations

import httpx
import pytest

from app.services.resilience import (
    CircuitBreaker,
    CircuitBreakerOpenError,
    ResilientCaller,
    is_retryable_error,
    retry_async,
)


class FakeClock:
    def __init__(self) -> None:
        self.now = 0.0

    def __call__(self) -> float:
        return self.now

    def advance(self, seconds: float) -> None:
        self.now += seconds


async def _no_sleep(_delay: float) -> None:
    return None


def _http_status_error(status_code: int) -> httpx.HTTPStatusError:
    request = httpx.Request("GET", "http://agent/")
    response = httpx.Response(status_code, request=request)
    return httpx.HTTPStatusError("boom", request=request, response=response)


# -- is_retryable_error --


class TestRetryableClassification:
    def test_5xx_is_retryable(self):
        assert is_retryable_error(_http_status_error(500)) is True
        assert is_retryable_error(_http_status_error(503)) is True

    def test_4xx_is_not_retryable(self):
        assert is_retryable_error(_http_status_error(400)) is False
        assert is_retryable_error(_http_status_error(404)) is False

    def test_transport_errors_are_retryable(self):
        assert is_retryable_error(httpx.ConnectError("refused")) is True
        assert is_retryable_error(httpx.ReadTimeout("slow")) is True

    def test_other_exceptions_not_retryable(self):
        assert is_retryable_error(ValueError("nope")) is False


# -- retry_async --


class TestRetryAsync:
    @pytest.mark.asyncio
    async def test_succeeds_after_transient_failures(self):
        calls = {"n": 0}

        async def flaky():
            calls["n"] += 1
            if calls["n"] < 3:
                raise _http_status_error(502)
            return "ok"

        result = await retry_async(flaky, attempts=3, sleep=_no_sleep)
        assert result == "ok"
        assert calls["n"] == 3

    @pytest.mark.asyncio
    async def test_no_retry_on_4xx(self):
        calls = {"n": 0}

        async def bad_request():
            calls["n"] += 1
            raise _http_status_error(400)

        with pytest.raises(httpx.HTTPStatusError):
            await retry_async(bad_request, attempts=3, sleep=_no_sleep)
        assert calls["n"] == 1

    @pytest.mark.asyncio
    async def test_raises_after_exhausting_attempts(self):
        calls = {"n": 0}

        async def always_down():
            calls["n"] += 1
            raise httpx.ConnectError("refused")

        with pytest.raises(httpx.ConnectError):
            await retry_async(always_down, attempts=3, sleep=_no_sleep)
        assert calls["n"] == 3

    @pytest.mark.asyncio
    async def test_backoff_is_exponential_with_jitter(self):
        delays: list[float] = []

        async def record_sleep(delay: float) -> None:
            delays.append(delay)

        async def always_down():
            raise httpx.ConnectError("refused")

        with pytest.raises(httpx.ConnectError):
            await retry_async(always_down, attempts=3, base_delay=1.0, sleep=record_sleep)

        assert len(delays) == 2
        assert 1.0 <= delays[0] <= 1.25  # 1s + up to 25% jitter
        assert 2.0 <= delays[1] <= 2.5  # 2s + up to 25% jitter


# -- CircuitBreaker --


class TestCircuitBreaker:
    def test_starts_closed(self):
        breaker = CircuitBreaker()
        assert breaker.state == CircuitBreaker.CLOSED
        breaker.check()  # no raise

    def test_opens_after_threshold_and_fails_fast(self):
        clock = FakeClock()
        breaker = CircuitBreaker(failure_threshold=3, cooldown_seconds=30.0, clock=clock)

        for _ in range(2):
            breaker.record_failure()
        assert breaker.state == CircuitBreaker.CLOSED

        breaker.record_failure()
        assert breaker.state == CircuitBreaker.OPEN
        with pytest.raises(CircuitBreakerOpenError):
            breaker.check()

    def test_success_resets_failure_count(self):
        breaker = CircuitBreaker(failure_threshold=3)
        breaker.record_failure()
        breaker.record_failure()
        breaker.record_success()
        breaker.record_failure()
        breaker.record_failure()
        assert breaker.state == CircuitBreaker.CLOSED

    def test_half_open_after_cooldown_then_closes_on_success(self):
        clock = FakeClock()
        breaker = CircuitBreaker(failure_threshold=1, cooldown_seconds=30.0, clock=clock)

        breaker.record_failure()
        assert breaker.state == CircuitBreaker.OPEN

        clock.advance(31.0)
        assert breaker.state == CircuitBreaker.HALF_OPEN
        breaker.check()  # probe allowed, no raise

        breaker.record_success()
        assert breaker.state == CircuitBreaker.CLOSED

    def test_half_open_reopens_on_probe_failure(self):
        clock = FakeClock()
        breaker = CircuitBreaker(failure_threshold=1, cooldown_seconds=30.0, clock=clock)

        breaker.record_failure()
        clock.advance(31.0)
        breaker.check()  # transitions to half-open probe

        breaker.record_failure()
        assert breaker.state == CircuitBreaker.OPEN
        with pytest.raises(CircuitBreakerOpenError):
            breaker.check()


# -- ResilientCaller --


class TestResilientCaller:
    @pytest.mark.asyncio
    async def test_breaker_opens_after_threshold_calls(self):
        clock = FakeClock()
        caller = ResilientCaller(
            attempts=1, breaker_threshold=2, breaker_cooldown_seconds=30.0,
            sleep=_no_sleep, clock=clock,
        )
        calls = {"n": 0}

        async def down():
            calls["n"] += 1
            raise httpx.ConnectError("refused")

        for _ in range(2):
            with pytest.raises(httpx.ConnectError):
                await caller.call("agent-a", down)

        assert caller.breaker_state("agent-a") == CircuitBreaker.OPEN
        with pytest.raises(CircuitBreakerOpenError):
            await caller.call("agent-a", down)
        assert calls["n"] == 2  # fail-fast: no call while open

    @pytest.mark.asyncio
    async def test_4xx_does_not_trip_breaker(self):
        caller = ResilientCaller(attempts=3, breaker_threshold=1, sleep=_no_sleep)

        async def bad_request():
            raise _http_status_error(404)

        with pytest.raises(httpx.HTTPStatusError):
            await caller.call("agent-a", bad_request)
        assert caller.breaker_state("agent-a") == CircuitBreaker.CLOSED

    @pytest.mark.asyncio
    async def test_half_open_recovery(self):
        clock = FakeClock()
        caller = ResilientCaller(
            attempts=1, breaker_threshold=1, breaker_cooldown_seconds=30.0,
            sleep=_no_sleep, clock=clock,
        )

        async def down():
            raise httpx.ConnectError("refused")

        async def up():
            return "ok"

        with pytest.raises(httpx.ConnectError):
            await caller.call("agent-a", down)
        assert caller.breaker_state("agent-a") == CircuitBreaker.OPEN

        clock.advance(31.0)
        assert caller.breaker_state("agent-a") == CircuitBreaker.HALF_OPEN
        assert await caller.call("agent-a", up) == "ok"
        assert caller.breaker_state("agent-a") == CircuitBreaker.CLOSED

    @pytest.mark.asyncio
    async def test_breakers_are_per_key(self):
        caller = ResilientCaller(attempts=1, breaker_threshold=1, sleep=_no_sleep)

        async def down():
            raise httpx.ConnectError("refused")

        async def up():
            return "ok"

        with pytest.raises(httpx.ConnectError):
            await caller.call("agent-a", down)
        assert caller.breaker_state("agent-a") == CircuitBreaker.OPEN

        # Other endpoints unaffected
        assert await caller.call("agent-b", up) == "ok"
        assert caller.breaker_states["agent-b"] == CircuitBreaker.CLOSED
