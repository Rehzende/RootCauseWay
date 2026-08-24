"""Reusable A2A server router for FastAPI agents."""

from __future__ import annotations

import logging
from typing import Any, Awaitable, Callable

import mlflow
from fastapi import APIRouter, Request
from fastapi.responses import JSONResponse
from mlflow.tracing import set_tracing_context_from_http_request_headers

from app.a2a.models import (
    AgentCard,
    JSONRPCError,
    JSONRPCRequest,
    JSONRPCResponse,
    Message,
    Task,
    TaskStatus,
)

logger = logging.getLogger(__name__)

# Handler type: receives a task_id and Message, returns a Task with artifacts
TaskHandler = Callable[[str, Message], Awaitable[Task]]


def create_a2a_router(card: AgentCard, handler: TaskHandler) -> APIRouter:
    """Create a FastAPI router that exposes A2A protocol endpoints.

    Args:
        card: The AgentCard describing this agent.
        handler: Async function that processes a task. Receives (task_id, message)
                 and returns a completed Task with artifacts.
    """
    router = APIRouter()
    tasks: dict[str, Task] = {}

    @router.get("/.well-known/agent.json")
    async def agent_card():
        return card.model_dump(by_alias=True)

    @router.post("/")
    async def jsonrpc_dispatch(request: Request):
        body = await request.json()
        rpc_req = JSONRPCRequest.model_validate(body)

        if rpc_req.method == "agent/card":
            return _success(rpc_req.id, card.model_dump(by_alias=True))

        if rpc_req.method == "tasks/send":
            return await _handle_send(rpc_req, dict(request.headers))

        if rpc_req.method == "tasks/get":
            return _handle_get(rpc_req)

        if rpc_req.method == "tasks/cancel":
            return _handle_cancel(rpc_req)

        return _error(rpc_req.id, -32601, f"Method not found: {rpc_req.method}")

    async def _handle_send(rpc_req: JSONRPCRequest, request_headers: dict[str, str]) -> JSONResponse:
        params = rpc_req.params or {}
        task_id = params.get("id")
        message_data = params.get("message")

        if not task_id or not message_data:
            return _error(rpc_req.id, -32602, "Missing id or message in params")

        message = Message.model_validate(message_data)

        # Mark as working
        task = Task(id=task_id, status=TaskStatus.WORKING, message=message)
        tasks[task_id] = task

        # If the caller propagated an MLflow trace context (see
        # app/a2a/client.py's get_tracing_context_headers_for_http_request),
        # run the handler under it so this agent's own @mlflow.trace span
        # attaches as a child instead of starting a disconnected root trace.
        # set_tracing_context_from_http_request_headers *raises* when
        # "traceparent" is absent, so this must stay opt-in: callers that
        # haven't been updated yet (or k8s-agent's standalone Alertmanager
        # webhook path, which has no orchestrator span to propagate at all)
        # fall through to today's behavior of a fresh root trace.
        has_traceparent = "traceparent" in request_headers or "Traceparent" in request_headers

        async def _run() -> Task:
            return await handler(task_id, message)

        try:
            if has_traceparent:
                with set_tracing_context_from_http_request_headers(request_headers):
                    result_task = await _run()
                    # set_tracing_context_from_http_request_headers only keeps
                    # its (in-memory, client-side) "remote trace" placeholder
                    # registered for the lifetime of this `with` block -- it
                    # pops it the instant the block exits. MLflow's own trace
                    # export is ASYNC (a background queue), so without an
                    # explicit flush *before* that pop, handle_task's span is
                    # built correctly (right trace_id, right parent_id --
                    # confirmed live) but silently never reaches the backend:
                    # by the time the queue gets to it, the placeholder that
                    # let it resolve which trace it belongs to is already
                    # gone, and the export is dropped without raising (mlflow
                    # deliberately never lets a tracing failure break the
                    # traced call). Only needed on this branch -- a locally
                    # rooted trace (no propagated context) has no such
                    # placeholder to race against and already exports fine on
                    # this process's own schedule, same as before this fix.
                    mlflow.flush_trace_async_logging()
            else:
                result_task = await _run()
            tasks[task_id] = result_task
            return _success(rpc_req.id, result_task.model_dump())
        except Exception as exc:
            logger.exception("Handler failed for task %s", task_id)
            failed_task = Task(id=task_id, status=TaskStatus.FAILED)
            tasks[task_id] = failed_task
            return _error(rpc_req.id, -32000, str(exc))

    def _handle_get(rpc_req: JSONRPCRequest) -> JSONResponse:
        params = rpc_req.params or {}
        task_id = params.get("id")
        if not task_id or task_id not in tasks:
            return _error(rpc_req.id, -32602, "Task not found")
        return _success(rpc_req.id, tasks[task_id].model_dump())

    def _handle_cancel(rpc_req: JSONRPCRequest) -> JSONResponse:
        params = rpc_req.params or {}
        task_id = params.get("id")
        if not task_id or task_id not in tasks:
            return _error(rpc_req.id, -32602, "Task not found")
        tasks[task_id].status = TaskStatus.CANCELED
        return _success(rpc_req.id, tasks[task_id].model_dump())

    return router


def _success(rpc_id: str | int | None, result: Any) -> JSONResponse:
    resp = JSONRPCResponse(result=result, id=rpc_id)
    return JSONResponse(content=resp.model_dump())


def _error(rpc_id: str | int | None, code: int, message: str) -> JSONResponse:
    resp = JSONRPCResponse(error=JSONRPCError(code=code, message=message), id=rpc_id)
    return JSONResponse(content=resp.model_dump(), status_code=200)
