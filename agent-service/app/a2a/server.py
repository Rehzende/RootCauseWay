"""Reusable A2A server router for FastAPI agents."""

from __future__ import annotations

import logging
from typing import Any, Awaitable, Callable

from fastapi import APIRouter, Request
from fastapi.responses import JSONResponse

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
            return await _handle_send(rpc_req)

        if rpc_req.method == "tasks/get":
            return _handle_get(rpc_req)

        if rpc_req.method == "tasks/cancel":
            return _handle_cancel(rpc_req)

        return _error(rpc_req.id, -32601, f"Method not found: {rpc_req.method}")

    async def _handle_send(rpc_req: JSONRPCRequest) -> JSONResponse:
        params = rpc_req.params or {}
        task_id = params.get("id")
        message_data = params.get("message")

        if not task_id or not message_data:
            return _error(rpc_req.id, -32602, "Missing id or message in params")

        message = Message.model_validate(message_data)

        # Mark as working
        task = Task(id=task_id, status=TaskStatus.WORKING, message=message)
        tasks[task_id] = task

        try:
            result_task = await handler(task_id, message)
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
