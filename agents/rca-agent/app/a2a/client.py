"""Minimal A2A client for agent-to-agent (mesh) calls.

Trimmed down from agent-service/app/a2a/client.py: no circuit breaker or
Prometheus metrics (those stay in the orchestrator, which fans out to many
agents and needs to fail fast on a degraded one). This client only backs
the specific supplementary-data calls this agent makes to a peer when its
own input is missing something it needs (see agent.py) -- a handful of
bounded, non-recursive calls, not a general-purpose dispatcher -- so plain
retry-with-backoff is enough.
"""

from __future__ import annotations

import asyncio
import logging
from typing import Any

import httpx
from mlflow.tracing import get_tracing_context_headers_for_http_request

from app.a2a.models import JSONRPCRequest, JSONRPCResponse, Message, Task

logger = logging.getLogger(__name__)


class A2APeerClient:
    def __init__(self, timeout: float = 150.0, retry_attempts: int = 2):
        self._client = httpx.AsyncClient(timeout=timeout)
        self._retry_attempts = retry_attempts

    async def send_task(self, agent_url: str, task_id: str, message: Message) -> Task:
        request = JSONRPCRequest(
            method="tasks/send",
            params={"id": task_id, "message": message.model_dump()},
            id=1,
        )

        last_exc: Exception = RuntimeError(f"A2A peer call to {agent_url} never attempted")
        for attempt in range(self._retry_attempts + 1):
            try:
                headers = {"Content-Type": "application/json"}
                # Propagate the caller's active MLflow span (this agent's own
                # handle_task, or an inbound traceparent it's still under) so
                # the peer's span attaches as a child -- see
                # agent-service/app/a2a/client.py's identical rationale.
                headers.update(get_tracing_context_headers_for_http_request())
                resp = await self._client.post(
                    agent_url.rstrip("/") + "/",
                    json=request.model_dump(),
                    headers=headers,
                )
                resp.raise_for_status()
                rpc_resp = JSONRPCResponse.model_validate(resp.json())
                if rpc_resp.error:
                    raise RuntimeError(
                        f"A2A RPC error calling {agent_url}: "
                        f"{rpc_resp.error.code} {rpc_resp.error.message}"
                    )
                return Task.model_validate(rpc_resp.result)
            except Exception as exc:
                last_exc = exc
                if attempt < self._retry_attempts:
                    logger.warning(
                        "A2A peer call to %s failed (attempt %d/%d): %s",
                        agent_url, attempt + 1, self._retry_attempts + 1, exc,
                    )
                    await asyncio.sleep(1.0 * (attempt + 1))
        raise last_exc

    @staticmethod
    def extract_result(task: Task) -> dict[str, Any]:
        """Same flattening _extract_result uses elsewhere: {artifact.name: data}."""
        result: dict[str, Any] = {}
        for artifact in task.artifacts:
            for part in artifact.parts:
                if hasattr(part, "data"):
                    result[artifact.name] = part.data
                elif hasattr(part, "text"):
                    result[artifact.name] = part.text
        return result

    async def close(self) -> None:
        await self._client.aclose()
