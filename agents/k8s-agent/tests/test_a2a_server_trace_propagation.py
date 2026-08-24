"""create_a2a_router (app/a2a/server.py, byte-identical across all 5 A2A
agent microservices) runs the task handler under the caller's propagated
MLflow trace context, when one is present -- so the receiving agent's own
@mlflow.trace span (e.g. triage_agent.handle_task) attaches as a CHILD of
the caller's span instead of starting a disconnected root trace. See
app/a2a/client.py's identical client-side half of this.

set_tracing_context_from_http_request_headers *raises* when "traceparent"
is absent, so this must stay opt-in -- the no-traceparent test below pins
that regression: a caller that hasn't been updated yet (or k8s-agent's
standalone Alertmanager webhook path, which has no orchestrator span to
propagate at all) must still get a normal response, not a 500.

Mocks mlflow's own set_tracing_context_from_http_request_headers at the
call boundary: mlflow's own propagation correctness is proven live
elsewhere (the MLflow upgrade validation), ours is just to call it (or
not) correctly around the handler.

test_flushes_before_the_context_manager_exits pins a real bug found live:
set_tracing_context_from_http_request_headers only keeps its in-memory
"remote trace" placeholder registered for the lifetime of its `with`
block, popping it the instant the block exits -- but mlflow's trace
export is async (a background queue). Calling flush_trace_async_logging()
*after* the `with` block (the natural-looking place to put it) is exactly
as broken as not calling it at all: the placeholder is already gone by
then, so the span is built correctly (confirmed live -- right trace_id,
right parent_id) but silently never reaches the backend. The flush must
happen *inside* the block, while the placeholder still resolves the span
to its trace.
"""

from __future__ import annotations

from contextlib import contextmanager
from unittest.mock import patch

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.a2a.models import AgentCard, Message, Role, Task, TaskStatus, TextPart
from app.a2a.server import create_a2a_router


@pytest.fixture
def client():
    card = AgentCard(name="Test Agent", url="http://test-agent:9999", version="0.1.0")

    async def handler(task_id: str, message: Message) -> Task:
        return Task(id=task_id, status=TaskStatus.COMPLETED, message=message)

    app = FastAPI()
    app.include_router(create_a2a_router(card, handler))
    return TestClient(app)


def _rpc_body(task_id: str = "task-1") -> dict:
    return {
        "jsonrpc": "2.0",
        "method": "tasks/send",
        "id": 1,
        "params": {
            "id": task_id,
            "message": Message(role=Role.USER, parts=[TextPart(text="hi")]).model_dump(),
        },
    }


class TestServerTracePropagation:
    def test_traceparent_header_runs_handler_under_propagated_context(self, client):
        fake_traceparent = "00-" + "a" * 32 + "-" + "b" * 16 + "-01"

        @contextmanager
        def fake_context_manager(headers):
            fake_context_manager.received_headers = headers
            yield

        with patch(
            "app.a2a.server.set_tracing_context_from_http_request_headers",
            side_effect=fake_context_manager,
        ) as mock_set_context:
            resp = client.post("/", json=_rpc_body(), headers={"traceparent": fake_traceparent})

        assert resp.status_code == 200
        assert resp.json()["result"]["status"] == "completed"
        mock_set_context.assert_called_once()
        assert mock_set_context.call_args.args[0]["traceparent"] == fake_traceparent

    def test_no_traceparent_header_skips_context_manager_and_still_succeeds(self, client):
        with patch("app.a2a.server.set_tracing_context_from_http_request_headers") as mock_set_context:
            resp = client.post("/", json=_rpc_body())

        assert resp.status_code == 200
        assert resp.json()["result"]["status"] == "completed"
        mock_set_context.assert_not_called()

    def test_handler_exception_under_traceparent_still_returns_rpc_error(self):
        card = AgentCard(name="Test Agent", url="http://test-agent:9999", version="0.1.0")

        async def failing_handler(task_id: str, message: Message) -> Task:
            raise RuntimeError("boom")

        app = FastAPI()
        app.include_router(create_a2a_router(card, failing_handler))
        client = TestClient(app, raise_server_exceptions=False)
        fake_traceparent = "00-" + "a" * 32 + "-" + "b" * 16 + "-01"

        resp = client.post("/", json=_rpc_body(), headers={"traceparent": fake_traceparent})

        assert resp.status_code == 200
        assert resp.json()["error"]["message"] == "boom"

    def test_flushes_before_the_context_manager_exits(self, client):
        fake_traceparent = "00-" + "a" * 32 + "-" + "b" * 16 + "-01"
        events: list[str] = []

        @contextmanager
        def fake_context_manager(headers):
            events.append("context_enter")
            yield
            events.append("context_exit")

        with patch(
            "app.a2a.server.set_tracing_context_from_http_request_headers",
            side_effect=fake_context_manager,
        ), patch(
            "app.a2a.server.mlflow.flush_trace_async_logging",
            side_effect=lambda: events.append("flush"),
        ) as mock_flush:
            resp = client.post("/", json=_rpc_body(), headers={"traceparent": fake_traceparent})

        assert resp.status_code == 200
        mock_flush.assert_called_once()
        assert events == ["context_enter", "flush", "context_exit"]
