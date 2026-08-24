"""Tests for AlertWorker._handle_runbook_execution_started.

FeaturesHandler.ExecuteRunbook (Go) already published runbook.execution.
started on every run; nothing consumed it, so an "automated" step type was
purely cosmetic. This pins the consumer side: the event is routed to
RunbookExecutor, and a failure here degrades gracefully (recorded via the
swallowed-error counter) rather than crashing the whole worker.
"""

from __future__ import annotations

import uuid
from datetime import datetime, timezone
from unittest.mock import AsyncMock, patch

import pytest

from app.observability.metrics import ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL
from app.workers.alert_worker import AlertWorker


@pytest.fixture
def worker():
    return AlertWorker(
        redis_client=AsyncMock(),
        backend_client=AsyncMock(),
        event_publisher=AsyncMock(),
    )


def _runbook_started_data(org_id, execution_id):
    return {
        "event_id": str(uuid.uuid4()),
        "event_type": "runbook.execution.started",
        "org_id": str(org_id),
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "payload": {"execution_id": str(execution_id), "runbook_id": str(uuid.uuid4())},
    }


def test_runbook_execution_started_is_in_handled_event_types():
    assert "runbook.execution.started" in AlertWorker.HANDLED_EVENT_TYPES


@pytest.mark.asyncio
async def test_dispatches_to_runbook_executor_with_execution_context(worker):
    org_id = uuid.uuid4()
    execution_id = uuid.uuid4()
    incident_id = uuid.uuid4()
    worker._backend.get_runbook_execution.return_value = {
        "id": str(execution_id), "incident_id": str(incident_id), "status": "running",
    }

    with patch.object(worker._runbook_executor, "run_automated_steps", new=AsyncMock(return_value={"status": "completed"})) as mock_run:
        await worker._handle_runbook_execution_started(_runbook_started_data(org_id, execution_id))

    mock_run.assert_awaited_once()
    kwargs = mock_run.await_args.kwargs
    assert kwargs["execution_id"] == execution_id
    assert kwargs["incident_id"] == incident_id
    assert kwargs["org_id"] == org_id
    assert kwargs["orchestrator"] is worker._orchestrator
    assert kwargs["jit_provider"] is worker._orchestrator._jit


@pytest.mark.asyncio
async def test_failure_is_swallowed_and_recorded_not_raised(worker):
    org_id = uuid.uuid4()
    execution_id = uuid.uuid4()
    before = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="alert_worker", error_type="runbook_automation_failed"
    )._value.get()

    worker._backend.get_runbook_execution.side_effect = RuntimeError("backend unreachable")

    await worker._handle_runbook_execution_started(_runbook_started_data(org_id, execution_id))

    after = ROOTCAUSEWAY_SWALLOWED_ERRORS_TOTAL.labels(
        component="alert_worker", error_type="runbook_automation_failed"
    )._value.get()
    assert after == before + 1
