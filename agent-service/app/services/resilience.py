"""Reusable resilience helpers: retry with backoff + per-key circuit breakers.

Designed for outbound HTTP calls (A2A agents, LLM providers, observability
APIs). No external dependencies beyond httpx, which is already used
throughout the service.
"""

from __future__ import annotations

import asyncio
import logging
import random
import time
from typing import Any, Awaitable, Callable

import httpx

logger = logging.getLogger(__name__)


class CircuitBreakerOpenError(RuntimeError):
    """Raised when a call is rejected because the circuit breaker is open."""

    def __init__(self, key: str, retry_after_seconds: float) -> None:
        super().__init__(
            f"Circuit breaker open for '{key}'; failing fast "
            f"(next probe in {max(retry_after_seconds, 0.0):.1f}s)"
        )
        self.key = key
        self.retry_after_seconds = max(retry_after_seconds, 0.0)


class CircuitBreaker:
    """Minimal circuit breaker: closed -> open after N consecutive failures,
    half-open probe after a cooldown, closed again on success."""

    CLOSED = "closed"
    OPEN = "open"
    HALF_OPEN = "half_open"

    def __init__(
        self,
        key: str = "default",
        failure_threshold: int = 5,
        cooldown_seconds: float = 30.0,
        clock: Callable[[], float] = time.monotonic,
    ) -> None:
        self.key = key
        self.failure_threshold = max(1, failure_threshold)
        self.cooldown_seconds = cooldown_seconds
        self._clock = clock
        self._state = self.CLOSED
        self._consecutive_failures = 0
        self._opened_at = 0.0

    @property
    def state(self) -> str:
        """Current effective state (open transitions to half_open after cooldown)."""
        if self._state == self.OPEN and self._cooldown_elapsed():
            return self.HALF_OPEN
        return self._state

    @property
    def consecutive_failures(self) -> int:
        return self._consecutive_failures

    def _cooldown_elapsed(self) -> bool:
        return (self._clock() - self._opened_at) >= self.cooldown_seconds

    def check(self) -> None:
        """Raise CircuitBreakerOpenError if calls must fail fast.

        When the cooldown has elapsed the breaker moves to half-open and
        lets the call through as a probe.
        """
        if self._state != self.OPEN:
            return
        if self._cooldown_elapsed():
            self._state = self.HALF_OPEN
            logger.info("Circuit breaker '%s' half-open: allowing probe", self.key)
            return
        remaining = self.cooldown_seconds - (self._clock() - self._opened_at)
        raise CircuitBreakerOpenError(self.key, remaining)

    def record_success(self) -> None:
        if self._state != self.CLOSED:
            logger.info("Circuit breaker '%s' closed after successful call", self.key)
        self._consecutive_failures = 0
        self._state = self.CLOSED

    def record_failure(self) -> None:
        self._consecutive_failures += 1
        if self._state == self.HALF_OPEN or self._consecutive_failures >= self.failure_threshold:
            if self._state != self.OPEN:
                logger.warning(
                    "Circuit breaker '%s' opened after %d consecutive failures",
                    self.key,
                    self._consecutive_failures,
                )
            self._state = self.OPEN
            self._opened_at = self._clock()


def is_retryable_error(exc: BaseException) -> bool:
    """Retry on connect errors, timeouts and 5xx responses; never on 4xx."""
    if isinstance(exc, httpx.HTTPStatusError):
        return exc.response.status_code >= 500
    return isinstance(exc, (httpx.TimeoutException, httpx.TransportError))


async def retry_async(
    func: Callable[[], Awaitable[Any]],
    *,
    attempts: int = 3,
    base_delay: float = 1.0,
    max_delay: float = 30.0,
    retryable: Callable[[BaseException], bool] = is_retryable_error,
    sleep: Callable[[float], Awaitable[None]] = asyncio.sleep,
) -> Any:
    """Run ``func`` with exponential backoff + jitter (1s, 2s, 4s, ...)."""
    attempts = max(1, attempts)
    for attempt in range(attempts):
        try:
            return await func()
        except Exception as exc:
            if attempt == attempts - 1 or not retryable(exc):
                raise
            delay = min(base_delay * (2**attempt), max_delay)
            delay += random.uniform(0, delay * 0.25)  # jitter
            logger.warning(
                "Retryable error on attempt %d/%d (%s: %s); retrying in %.2fs",
                attempt + 1,
                attempts,
                type(exc).__name__,
                exc,
                delay,
            )
            await sleep(delay)
    raise RuntimeError("unreachable")  # pragma: no cover


class ResilientCaller:
    """Retry + per-key circuit breaker for async callables.

    Each ``key`` (e.g. an agent endpoint URL) gets its own breaker so one
    degraded endpoint does not block calls to healthy ones.
    """

    def __init__(
        self,
        *,
        attempts: int = 3,
        base_delay: float = 1.0,
        max_delay: float = 30.0,
        breaker_threshold: int = 5,
        breaker_cooldown_seconds: float = 30.0,
        retryable: Callable[[BaseException], bool] = is_retryable_error,
        sleep: Callable[[float], Awaitable[None]] = asyncio.sleep,
        clock: Callable[[], float] = time.monotonic,
    ) -> None:
        self._attempts = attempts
        self._base_delay = base_delay
        self._max_delay = max_delay
        self._breaker_threshold = breaker_threshold
        self._breaker_cooldown_seconds = breaker_cooldown_seconds
        self._retryable = retryable
        self._sleep = sleep
        self._clock = clock
        self._breakers: dict[str, CircuitBreaker] = {}

    def breaker(self, key: str) -> CircuitBreaker:
        if key not in self._breakers:
            self._breakers[key] = CircuitBreaker(
                key=key,
                failure_threshold=self._breaker_threshold,
                cooldown_seconds=self._breaker_cooldown_seconds,
                clock=self._clock,
            )
        return self._breakers[key]

    def breaker_state(self, key: str) -> str:
        return self.breaker(key).state

    @property
    def breaker_states(self) -> dict[str, str]:
        return {key: breaker.state for key, breaker in self._breakers.items()}

    async def call(self, key: str, func: Callable[[], Awaitable[Any]]) -> Any:
        """Execute ``func`` guarded by the key's breaker, retrying transient errors."""
        breaker = self.breaker(key)
        breaker.check()
        try:
            result = await retry_async(
                func,
                attempts=self._attempts,
                base_delay=self._base_delay,
                max_delay=self._max_delay,
                retryable=self._retryable,
                sleep=self._sleep,
            )
        except Exception as exc:
            # Only failures that look like endpoint degradation trip the breaker;
            # client-side errors (4xx) do not.
            if self._retryable(exc):
                breaker.record_failure()
            raise
        breaker.record_success()
        return result
