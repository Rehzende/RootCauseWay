"""Prometheus metrics for the agent-service.

Exposes:
  - A2A call duration/outcome metrics, updated via ``record_a2a_call`` (called
    from ``app.a2a.client``).
  - A Redis stream "pending messages" gauge, kept fresh by a small
    self-contained background updater (``StreamQueueDepthUpdater``) that owns
    its own Redis connection. This module intentionally does NOT import
    anything from ``app.workers.alert_worker`` to avoid colliding with that
    file's owner -- it is fully independent.
  - A ``/metrics`` ASGI app suitable for mounting on the FastAPI app.
"""

from __future__ import annotations

import asyncio
import logging

from prometheus_client import CONTENT_TYPE_LATEST, Counter, Gauge, Histogram, generate_latest
from prometheus_client import make_asgi_app

logger = logging.getLogger(__name__)

# --- A2A call metrics ------------------------------------------------------

A2A_CALL_DURATION_SECONDS = Histogram(
    "rootcauseway_a2a_call_duration_seconds",
    "Duration of outbound A2A calls in seconds, labeled by agent URL and outcome.",
    ["agent_url", "outcome"],
)

A2A_CALLS_TOTAL = Counter(
    "rootcauseway_a2a_calls_total",
    "Total outbound A2A calls, labeled by agent URL and outcome.",
    ["agent_url", "outcome"],
)

#: Recognized outcomes for record_a2a_call. Any other value is still
#: recorded (labels are free-form), but these are the expected set.
A2A_OUTCOME_SUCCESS = "success"
A2A_OUTCOME_FAILURE = "failure"
A2A_OUTCOME_CIRCUIT_OPEN = "circuit_open"


def record_a2a_call(agent_url: str, duration_seconds: float, outcome: str) -> None:
    """Record the outcome and duration of an outbound A2A call.

    ``outcome`` is expected to be one of "success", "failure" or
    "circuit_open", but any string is accepted as a label value.
    """
    A2A_CALL_DURATION_SECONDS.labels(agent_url=agent_url, outcome=outcome).observe(
        duration_seconds
    )
    A2A_CALLS_TOTAL.labels(agent_url=agent_url, outcome=outcome).inc()


# --- Swallowed-error visibility ---------------------------------------------
#
# The codebase has many `except Exception: logger.exception(...)` sites where
# a pipeline stage degrades gracefully (the incident keeps processing) but
# something real broke -- an agent_run never got its DAG-tracking row, a
# postmortem never got generated, an A2A dispatch failed outright. Before
# this metric, the only trace of that was a log line nobody was watching.
# record_swallowed_error() turns each of those into a Prometheus counter, and
# rootcauseway-apps-alerts' RootCausewayAgentServiceSwallowedError rule (see the k8s
# PrometheusRule) fires an alert on ANY occurrence -- which, via the
# existing dual-webhook Alertmanager config, becomes an incident the same
# way a real production alert would.

ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL = Counter(
    "rootcauseway_swallowed_errors_total",
    "Errors caught, logged, and NOT re-raised, labeled by component and "
    "error_type. Each occurrence means a pipeline stage silently degraded "
    "instead of failing loud -- see the record_swallowed_error() call site "
    "for what actually broke.",
    ["component", "error_type"],
)


def record_swallowed_error(component: str, error_type: str) -> None:
    """Call from inside an `except:` block that logs-and-continues, right
    before (or instead of) the logger call, so the failure shows up in
    Prometheus even though the caller never sees an exception."""
    ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(component=component, error_type=error_type).inc()


# --- Stale event backpressure -----------------------------------------------
#
# The single shared local LLM is the real capacity bottleneck (see
# alert_worker.py's staleness check) -- this counter is how an operator
# notices the backlog is bad enough that events are aging out, as opposed
# to just watching rootcauseway_stream_pending_messages climb with no signal of
# what's actually happening to that backlog.

ROOTCAUSEWAY_STALE_EVENTS_SKIPPED_TOTAL = Counter(
    "rootcauseway_stale_events_skipped_total",
    "Stream events skipped because they'd already aged past "
    "stale_event_threshold_seconds by the time they were dequeued, "
    "labeled by event_type.",
    ["event_type"],
)


def record_stale_event_skipped(event_type: str) -> None:
    ROOTCAUSEWAY_STALE_EVENTS_SKIPPED_TOTAL.labels(event_type=event_type).inc()


# --- Redis stream queue depth gauge ----------------------------------------

STREAM_PENDING_MESSAGES = Gauge(
    "rootcauseway_stream_pending_messages",
    "Number of messages currently in the configured Redis event stream (XLEN).",
    ["stream"],
)


class StreamQueueDepthUpdater:
    """Periodically polls XLEN on the configured Redis stream and updates a
    gauge with the result.

    Owns its own Redis connection (built from settings), independent of any
    connection used elsewhere in the service (e.g. the alert worker), so it
    can be started/stopped without touching other subsystems.
    """

    def __init__(
        self,
        redis_url: str,
        stream_name: str,
        *,
        interval_seconds: float = 15.0,
        redis_factory=None,
    ) -> None:
        self._redis_url = redis_url
        self._stream_name = stream_name
        self._interval_seconds = interval_seconds
        self._redis_factory = redis_factory
        self._redis = None
        self._task: asyncio.Task | None = None
        self._running = False

    def _make_redis(self):
        if self._redis_factory is not None:
            return self._redis_factory(self._redis_url)
        import redis.asyncio as redis

        return redis.from_url(self._redis_url)

    async def _poll_once(self) -> None:
        try:
            length = await self._redis.xlen(self._stream_name)
            STREAM_PENDING_MESSAGES.labels(stream=self._stream_name).set(length)
        except Exception:  # pragma: no cover - defensive, logged for visibility
            logger.exception(
                "Failed to poll queue depth for stream %s", self._stream_name
            )

    async def _run(self) -> None:
        while self._running:
            await self._poll_once()
            await asyncio.sleep(self._interval_seconds)

    async def start(self) -> None:
        if self._running:
            return
        self._redis = self._make_redis()
        self._running = True
        self._task = asyncio.create_task(self._run())
        logger.info(
            "Stream queue depth updater started for stream=%s interval=%ss",
            self._stream_name,
            self._interval_seconds,
        )

    async def stop(self) -> None:
        self._running = False
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
            self._task = None
        if self._redis is not None:
            await self._redis.aclose()
            self._redis = None


# --- /metrics ASGI app -------------------------------------------------------

metrics_app = make_asgi_app()


def metrics_text() -> tuple[bytes, str]:
    """Return the raw metrics payload and content type (helper for tests)."""
    return generate_latest(), CONTENT_TYPE_LATEST
