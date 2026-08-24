"""A2AClient propagates the active MLflow span as a W3C traceparent header
on every outbound call -- otherwise every incident produces N+1 disconnected
single-agent traces instead of one trace showing orchestrator -> agent ->
LLM call (and agent -> peer agent, for the mesh calls) as a single tree.
See agent-service/app/a2a/client.py's _headers() for the full rationale.

Mocks mlflow's own get_tracing_context_headers_for_http_request() at the
call boundary rather than spinning up a real tracking store/span: mlflow's
own W3C-propagation correctness is its responsibility (already proven live
against the real cluster -- see the MLflow upgrade validation), ours is
just to call it and merge the result into every outbound request.
"""

from __future__ import annotations

from unittest.mock import patch

import httpx
import pytest
import respx

from app.a2a.client import A2AClient
from app.a2a.models import Message, Role, TextPart

AGENT_URL = "http://triage-agent:8090"


def _message() -> Message:
    return Message(role=Role.USER, parts=[TextPart(text="triage this")])


def _rpc_task_response(task_id: str = "task-1") -> httpx.Response:
    return httpx.Response(
        200,
        json={"jsonrpc": "2.0", "result": {"id": task_id, "status": "completed"}, "id": 1},
    )


class TestTracePropagation:
    @pytest.mark.asyncio
    @respx.mock
    async def test_no_active_span_sends_no_traceparent(self):
        route = respx.post(f"{AGENT_URL}/").mock(return_value=_rpc_task_response())
        client = A2AClient()

        with patch("app.a2a.client.get_tracing_context_headers_for_http_request", return_value={}):
            await client.send_task(AGENT_URL, "task-1", _message())

        assert "traceparent" not in route.calls.last.request.headers

    @pytest.mark.asyncio
    @respx.mock
    async def test_active_span_traceparent_is_merged_into_request_headers(self):
        route = respx.post(f"{AGENT_URL}/").mock(return_value=_rpc_task_response())
        client = A2AClient(auth_token="tok123")
        fake_traceparent = "00-" + "a" * 32 + "-" + "b" * 16 + "-01"

        with patch(
            "app.a2a.client.get_tracing_context_headers_for_http_request",
            return_value={"traceparent": fake_traceparent},
        ):
            await client.send_task(AGENT_URL, "task-1", _message())

        sent_headers = route.calls.last.request.headers
        assert sent_headers["traceparent"] == fake_traceparent
        # traceparent injection must not clobber the other headers _headers() sets
        assert sent_headers["Authorization"] == "Bearer tok123"
        assert sent_headers["Content-Type"] == "application/json"
