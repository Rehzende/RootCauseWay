"""A2A Client for the orchestrator to call specialized agents."""

from __future__ import annotations

import logging
import time
from typing import Any

import httpx
from mlflow.tracing import get_tracing_context_headers_for_http_request

from app.a2a.models import (
    AgentCard,
    JSONRPCRequest,
    JSONRPCResponse,
    Message,
    Task,
)
from app.config.settings import get_settings
from app.observability.metrics import (
    A2A_OUTCOME_CIRCUIT_OPEN,
    A2A_OUTCOME_FAILURE,
    A2A_OUTCOME_SUCCESS,
    record_a2a_call,
)
from app.services.resilience import CircuitBreakerOpenError, ResilientCaller

logger = logging.getLogger(__name__)


class A2AClient:
    """Client that speaks the Google A2A protocol to reach specialized agents.

    All HTTP calls are retried with exponential backoff + jitter on connect
    errors, timeouts and 5xx responses (never on 4xx), and guarded by a
    per-endpoint circuit breaker so a degraded agent fails fast instead of
    stalling the pipeline.
    """

    def __init__(
        self,
        http_client: httpx.AsyncClient | None = None,
        auth_token: str | None = None,
        api_key: str | None = None,
        *,
        retry_attempts: int | None = None,
        retry_base_delay_seconds: float | None = None,
        breaker_threshold: int | None = None,
        breaker_cooldown_seconds: float | None = None,
        request_timeout_seconds: float | None = None,
    ):
        settings = get_settings()
        timeout = (
            request_timeout_seconds
            if request_timeout_seconds is not None
            else settings.a2a_request_timeout_seconds
        )
        self._client = http_client or httpx.AsyncClient(timeout=timeout)
        self._auth_token = auth_token
        self._api_key = api_key
        self._resilience = ResilientCaller(
            attempts=(
                retry_attempts if retry_attempts is not None else settings.a2a_retry_attempts
            ),
            base_delay=(
                retry_base_delay_seconds
                if retry_base_delay_seconds is not None
                else settings.a2a_retry_base_delay_seconds
            ),
            breaker_threshold=(
                breaker_threshold
                if breaker_threshold is not None
                else settings.a2a_breaker_threshold
            ),
            breaker_cooldown_seconds=(
                breaker_cooldown_seconds
                if breaker_cooldown_seconds is not None
                else settings.a2a_breaker_cooldown_seconds
            ),
        )

    def _headers(self) -> dict[str, str]:
        headers: dict[str, str] = {"Content-Type": "application/json"}
        if self._auth_token:
            headers["Authorization"] = f"Bearer {self._auth_token}"
        if self._api_key:
            headers["X-API-Key"] = self._api_key
        # Propagate the current MLflow span (orchestrator.handle_incident or
        # a nested span) as a W3C traceparent header, so the receiving
        # agent's own @mlflow.trace span attaches as a CHILD of this one
        # instead of starting a disconnected root trace. Without this, every
        # incident produces N+1 unrelated single-agent traces instead of one
        # trace showing the whole orchestrator -> agent -> LLM call tree.
        # No-ops (empty dict) if there's no active span, e.g. in tests that
        # call send_task() directly outside of handle_incident.
        headers.update(get_tracing_context_headers_for_http_request())
        return headers

    @staticmethod
    def _endpoint_key(agent_url: str) -> str:
        return agent_url.rstrip("/")

    def breaker_state(self, agent_url: str) -> str:
        """Circuit breaker state for an agent endpoint: closed | open | half_open."""
        return self._resilience.breaker_state(self._endpoint_key(agent_url))

    @property
    def breaker_states(self) -> dict[str, str]:
        """Breaker state for every endpoint contacted so far."""
        return self._resilience.breaker_states

    async def _request(self, agent_url: str, func) -> httpx.Response:
        """Run an HTTP call with retry + circuit breaker for the endpoint."""
        start = time.monotonic()
        try:
            response = await self._resilience.call(self._endpoint_key(agent_url), func)
        except CircuitBreakerOpenError:
            record_a2a_call(agent_url, time.monotonic() - start, A2A_OUTCOME_CIRCUIT_OPEN)
            raise
        except Exception:
            record_a2a_call(agent_url, time.monotonic() - start, A2A_OUTCOME_FAILURE)
            raise
        record_a2a_call(agent_url, time.monotonic() - start, A2A_OUTCOME_SUCCESS)
        return response

    async def discover(self, agent_url: str) -> AgentCard:
        """Fetch the agent's Agent Card via GET /.well-known/agent.json."""
        url = f"{agent_url.rstrip('/')}/.well-known/agent.json"

        async def _do() -> httpx.Response:
            resp = await self._client.get(url, headers=self._headers())
            resp.raise_for_status()
            return resp

        resp = await self._request(agent_url, _do)
        return AgentCard.model_validate(resp.json())

    async def _rpc(self, agent_url: str, method: str, params: dict[str, Any] | None = None) -> Any:
        request = JSONRPCRequest(method=method, params=params, id=1)

        async def _do() -> httpx.Response:
            resp = await self._client.post(
                agent_url.rstrip("/") + "/",
                json=request.model_dump(),
                headers=self._headers(),
            )
            resp.raise_for_status()
            return resp

        resp = await self._request(agent_url, _do)
        rpc_resp = JSONRPCResponse.model_validate(resp.json())
        if rpc_resp.error:
            raise RuntimeError(f"A2A RPC error: {rpc_resp.error.code} {rpc_resp.error.message}")
        return rpc_resp.result

    async def send_task(self, agent_url: str, task_id: str, message: Message) -> Task:
        """Send a task to an agent via tasks/send."""
        result = await self._rpc(agent_url, "tasks/send", {
            "id": task_id,
            "message": message.model_dump(),
        })
        return Task.model_validate(result)

    async def get_task(self, agent_url: str, task_id: str) -> Task:
        """Get task status via tasks/get."""
        result = await self._rpc(agent_url, "tasks/get", {"id": task_id})
        return Task.model_validate(result)

    async def cancel_task(self, agent_url: str, task_id: str) -> Task:
        """Cancel a task via tasks/cancel."""
        result = await self._rpc(agent_url, "tasks/cancel", {"id": task_id})
        return Task.model_validate(result)

    async def close(self) -> None:
        await self._client.aclose()
