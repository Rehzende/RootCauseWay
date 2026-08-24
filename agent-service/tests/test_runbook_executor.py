"""Tests for RunbookExecutor -- the runbook automation loop.

A platform audit found this class existed but was never imported anywhere,
and its original design called BackendClient.update_runbook_execution with
an ad-hoc payload shape that never matched the real Go model -- even wired
up, every call would have 400'd. Rewritten to drive state through
POST .../steps/:stepId/complete instead, reusing the Go handler's already-
correct advancement logic. These tests pin the new contract.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock
from uuid import uuid4

import pytest

from app.runbooks.executor import RunbookExecutor


@pytest.fixture
def backend():
    return AsyncMock()


@pytest.fixture
def jit():
    return AsyncMock()


@pytest.fixture
def orchestrator():
    orch = AsyncMock()
    orch.dispatch_single_skill.return_value = {"summary": "did the thing"}
    return orch


@pytest.fixture
def executor():
    return RunbookExecutor()


def _step_result(step_id: str, step_type: str, status: str) -> dict:
    return {"step_id": step_id, "step_type": step_type, "status": status}


@pytest.mark.asyncio
async def test_runs_a_single_automated_step_to_completion(executor, backend, jit, orchestrator):
    """The step CompleteExecutionStep marked "running" gets dispatched via
    the orchestrator, then completed with the real result as output."""
    org_id = uuid4()
    execution_id = uuid4()
    incident_id = uuid4()
    runbook_id = "rb-1"
    skill_id = str(uuid4())

    backend.get_runbook_execution.return_value = {
        "id": str(execution_id), "runbook_id": runbook_id, "incident_id": str(incident_id),
        "status": "running",
        "step_results": [_step_result("s1", "automated", "running")],
    }
    backend.get_runbook.return_value = {"id": runbook_id, "software_id": "sw-1"}
    backend.list_runbook_steps.return_value = [
        {"id": "s1", "step_type": "automated", "skill_id": skill_id, "name": "Restart pod"},
    ]
    backend.list_skills.return_value = [
        {"id": skill_id, "agents": [{"id": "a1", "url": "http://rca-agent:8092"}]},
    ]
    # Second call (after completing s1) reports the execution as done.
    backend.complete_runbook_execution_step.return_value = {
        "id": str(execution_id), "status": "completed",
        "step_results": [_step_result("s1", "automated", "completed")],
    }

    result = await executor.run_automated_steps(
        backend_client=backend, jit_provider=jit, orchestrator=orchestrator,
        org_id=org_id, execution_id=execution_id, incident_id=incident_id,
    )

    orchestrator.dispatch_single_skill.assert_awaited_once()
    call_kwargs = orchestrator.dispatch_single_skill.await_args.kwargs
    assert call_kwargs["skill_id"] == skill_id
    assert call_kwargs["agent_url"] == "http://rca-agent:8092"

    backend.complete_runbook_execution_step.assert_awaited_once()
    complete_args = backend.complete_runbook_execution_step.await_args
    assert complete_args.args[:2] == (execution_id, "s1")
    assert complete_args.kwargs["status"] == "completed"

    assert result["status"] == "completed"


@pytest.mark.asyncio
async def test_stops_at_a_manual_step_without_calling_the_agent(executor, backend, jit, orchestrator):
    """A manual step needs a human -- the loop must not try to dispatch it,
    and must leave the execution exactly where the human's "Mark Complete"
    click will find it."""
    org_id, execution_id, incident_id = uuid4(), uuid4(), uuid4()
    backend.get_runbook_execution.return_value = {
        "id": str(execution_id), "runbook_id": "rb-1", "status": "running",
        "step_results": [_step_result("s1", "manual", "pending_action")],
    }
    backend.get_runbook.return_value = {"id": "rb-1"}
    backend.list_runbook_steps.return_value = [{"id": "s1", "step_type": "manual", "name": "Check dashboard"}]
    backend.list_skills.return_value = []

    result = await executor.run_automated_steps(
        backend_client=backend, jit_provider=jit, orchestrator=orchestrator,
        org_id=org_id, execution_id=execution_id, incident_id=incident_id,
    )

    orchestrator.dispatch_single_skill.assert_not_called()
    backend.complete_runbook_execution_step.assert_not_called()
    assert result["step_results"][0]["status"] == "pending_action"


@pytest.mark.asyncio
async def test_stops_at_an_approval_step(executor, backend, jit, orchestrator):
    org_id, execution_id, incident_id = uuid4(), uuid4(), uuid4()
    backend.get_runbook_execution.return_value = {
        "id": str(execution_id), "runbook_id": "rb-1", "status": "running",
        "step_results": [_step_result("s1", "approval", "pending_approval")],
    }
    backend.get_runbook.return_value = {"id": "rb-1"}
    backend.list_runbook_steps.return_value = [{"id": "s1", "step_type": "approval"}]
    backend.list_skills.return_value = []

    await executor.run_automated_steps(
        backend_client=backend, jit_provider=jit, orchestrator=orchestrator,
        org_id=org_id, execution_id=execution_id, incident_id=incident_id,
    )

    orchestrator.dispatch_single_skill.assert_not_called()


@pytest.mark.asyncio
async def test_runs_automated_then_condition_then_stops_at_manual(executor, backend, jit, orchestrator):
    """A realistic mixed runbook: automated step runs, its result feeds a
    condition step, which continues into a manual step where automation
    correctly stops."""
    org_id, execution_id, incident_id = uuid4(), uuid4(), uuid4()
    skill_id = str(uuid4())

    backend.get_runbook_execution.side_effect = [
        {  # initial fetch
            "id": str(execution_id), "runbook_id": "rb-1", "status": "running",
            "step_results": [
                _step_result("s1", "automated", "running"),
                _step_result("s2", "condition", "pending"),
                _step_result("s3", "manual", "pending"),
            ],
        },
    ]
    backend.get_runbook.return_value = {"id": "rb-1", "software_id": "sw-1"}
    backend.list_runbook_steps.return_value = [
        {"id": "s1", "step_type": "automated", "skill_id": skill_id},
        {"id": "s2", "step_type": "condition", "condition": {"check_step_id": "s1", "expected_status": "completed"}},
        {"id": "s3", "step_type": "manual"},
    ]
    backend.list_skills.return_value = [{"id": skill_id, "agents": [{"id": "a1", "url": "http://x:8092"}]}]

    # complete_runbook_execution_step is called twice (s1, then s2) --
    # each time returning the execution advanced one step further.
    backend.complete_runbook_execution_step.side_effect = [
        {
            "id": str(execution_id), "status": "running",
            "step_results": [
                _step_result("s1", "automated", "completed"),
                _step_result("s2", "condition", "running"),
                _step_result("s3", "manual", "pending"),
            ],
        },
        {
            "id": str(execution_id), "status": "running",
            "step_results": [
                _step_result("s1", "automated", "completed"),
                _step_result("s2", "condition", "completed"),
                _step_result("s3", "manual", "pending_action"),
            ],
        },
    ]

    result = await executor.run_automated_steps(
        backend_client=backend, jit_provider=jit, orchestrator=orchestrator,
        org_id=org_id, execution_id=execution_id, incident_id=incident_id,
    )

    assert orchestrator.dispatch_single_skill.await_count == 1
    assert backend.complete_runbook_execution_step.await_count == 2
    assert result["step_results"][2]["status"] == "pending_action"


@pytest.mark.asyncio
async def test_already_terminal_execution_does_nothing(executor, backend, jit, orchestrator):
    org_id, execution_id, incident_id = uuid4(), uuid4(), uuid4()
    backend.get_runbook_execution.return_value = {
        "id": str(execution_id), "runbook_id": "rb-1", "status": "completed",
        "step_results": [_step_result("s1", "automated", "completed")],
    }
    backend.get_runbook.return_value = {"id": "rb-1"}
    backend.list_runbook_steps.return_value = [{"id": "s1", "step_type": "automated"}]
    backend.list_skills.return_value = []

    await executor.run_automated_steps(
        backend_client=backend, jit_provider=jit, orchestrator=orchestrator,
        org_id=org_id, execution_id=execution_id, incident_id=incident_id,
    )

    orchestrator.dispatch_single_skill.assert_not_called()
    backend.complete_runbook_execution_step.assert_not_called()


@pytest.mark.asyncio
async def test_dispatch_failure_completes_step_as_failed_not_raised(executor, backend, jit, orchestrator):
    """A single automated step failing (agent unreachable, bad skill_id,
    etc) must be recorded via the normal complete-step call, not blow up
    the whole automation pass."""
    org_id, execution_id, incident_id = uuid4(), uuid4(), uuid4()
    skill_id = str(uuid4())
    orchestrator.dispatch_single_skill.side_effect = RuntimeError("agent unreachable")

    backend.get_runbook_execution.return_value = {
        "id": str(execution_id), "runbook_id": "rb-1", "status": "running",
        "step_results": [_step_result("s1", "automated", "running")],
    }
    backend.get_runbook.return_value = {"id": "rb-1", "software_id": "sw-1"}
    backend.list_runbook_steps.return_value = [{"id": "s1", "step_type": "automated", "skill_id": skill_id}]
    backend.list_skills.return_value = [{"id": skill_id, "agents": [{"id": "a1", "url": "http://x:8092"}]}]
    backend.complete_runbook_execution_step.return_value = {
        "id": str(execution_id), "status": "completed_with_errors",
        "step_results": [_step_result("s1", "automated", "failed")],
    }

    result = await executor.run_automated_steps(
        backend_client=backend, jit_provider=jit, orchestrator=orchestrator,
        org_id=org_id, execution_id=execution_id, incident_id=incident_id,
    )

    complete_kwargs = backend.complete_runbook_execution_step.await_args.kwargs
    assert complete_kwargs["status"] == "failed"
    assert result["status"] == "completed_with_errors"
