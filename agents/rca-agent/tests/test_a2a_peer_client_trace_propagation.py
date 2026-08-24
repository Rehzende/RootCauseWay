"""A2APeerClient (the mesh client this agent uses to call a peer agent
directly, e.g. k8s-agent) propagates the active MLflow span as a W3C
traceparent header -- otherwise a mesh call (agent X -> agent A) shows up
as its own disconnected trace instead of nesting under the incident's one
trace. Mocks mlflow's own get_tracing_context_headers_for_http_request()
at the call boundary: mlflow's own propagation correctness is proven live
elsewhere (the MLflow upgrade validation), ours is just to call it and
merge the result into every outbound mesh request.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, patch

import httpx
import pytest

from app.a2a.client import A2APeerClient
from app.a2a.models import Message, Role, TextPart

PEER_URL = "http://rootcauseway-k8s-agent:8094"


def _message() -> Message:
    return Message(role=Role.USER, parts=[TextPart(text="peer request")])


def _rpc_response() -> httpx.Response:
    return httpx.Response(
        200,
        json={"jsonrpc": "2.0", "result": {"id": "task-1", "status": "completed"}, "id": 1},
        request=httpx.Request("POST", f"{PEER_URL}/"),
    )


class TestPeerClientTracePropagation:
    @pytest.mark.asyncio
    async def test_no_active_span_sends_no_traceparent(self):
        client = A2APeerClient()
        with patch("app.a2a.client.get_tracing_context_headers_for_http_request", return_value={}), \
             patch.object(httpx.AsyncClient, "post", new=AsyncMock(return_value=_rpc_response())) as mock_post:
            await client.send_task(PEER_URL, "task-1", _message())

        assert "traceparent" not in mock_post.call_args.kwargs["headers"]

    @pytest.mark.asyncio
    async def test_active_span_traceparent_is_merged_into_request_headers(self):
        client = A2APeerClient()
        fake_traceparent = "00-" + "a" * 32 + "-" + "b" * 16 + "-01"

        with patch(
            "app.a2a.client.get_tracing_context_headers_for_http_request",
            return_value={"traceparent": fake_traceparent},
        ), patch.object(httpx.AsyncClient, "post", new=AsyncMock(return_value=_rpc_response())) as mock_post:
            await client.send_task(PEER_URL, "task-1", _message())

        sent_headers = mock_post.call_args.kwargs["headers"]
        assert sent_headers["traceparent"] == fake_traceparent
        assert sent_headers["Content-Type"] == "application/json"
