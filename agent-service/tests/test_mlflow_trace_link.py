"""Coverage for Orchestrator._persist_mlflow_trace_link -- links an
incident to its MLflow trace as evidence, so the frontend can surface a
"view trace" link (previously the two systems had zero cross-reference,
per the platform audit backlog)."""

from __future__ import annotations

import uuid
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from app.orchestrator.orchestrator import Orchestrator


@pytest.fixture
def orchestrator():
    backend = AsyncMock()
    a2a = AsyncMock()
    llm_call = AsyncMock()
    return Orchestrator(backend, a2a, llm_call), backend


@pytest.mark.asyncio
async def test_persists_trace_link_when_span_active(orchestrator):
    orch, backend = orchestrator
    incident_id = uuid.uuid4()
    org_id = uuid.uuid4()

    fake_span = MagicMock()
    fake_span.trace_id = "tr-abc123"

    with patch("mlflow.get_current_active_span", return_value=fake_span), \
         patch("mlflow.get_experiment_by_name", return_value=MagicMock(experiment_id="1")):
        await orch._persist_mlflow_trace_link(incident_id, org_id)

    backend.add_incident_evidence.assert_awaited_once()
    # Regression: add_incident_evidence used to be called with no org_id at
    # all, which 404'd against the ownership check on the internal route
    # (see test_backend_client.py's TestAddIncidentEvidence for the same
    # fix at the HTTP layer) -- silently, since this whole call sits behind
    # a generic `except Exception` in orchestrator.py. Every MLflow trace
    # link was lost until this fix.
    call_incident_id, evidence, call_org_id = backend.add_incident_evidence.await_args.args
    assert call_incident_id == incident_id
    assert call_org_id == org_id
    assert evidence.type == "trace"
    # trace_id is stored as informational metadata but NOT used to build a
    # selectedTraceId=... deep link -- the client-side ID doesn't match
    # what the server stores it under (verified live), so the link points
    # at the experiment's traces tab instead of a specific (and likely
    # 404ing) trace.
    assert evidence.content["trace_id"] == "tr-abc123"
    assert evidence.content["url"] == "https://mlflow.rezende.lab/#/experiments/1/traces"


@pytest.mark.asyncio
async def test_no_op_when_no_active_span(orchestrator):
    """handle_incident might be called outside any @mlflow.trace context
    (e.g. certain test paths) -- must not error, just skip."""
    orch, backend = orchestrator
    with patch("mlflow.get_current_active_span", return_value=None):
        await orch._persist_mlflow_trace_link(uuid.uuid4(), uuid.uuid4())

    backend.add_incident_evidence.assert_not_awaited()


@pytest.mark.asyncio
async def test_backend_failure_does_not_raise(orchestrator):
    orch, backend = orchestrator
    backend.add_incident_evidence.side_effect = RuntimeError("backend down")
    fake_span = MagicMock()
    fake_span.trace_id = "tr-xyz"

    with patch("mlflow.get_current_active_span", return_value=fake_span), \
         patch("mlflow.get_experiment_by_name", return_value=None):
        await orch._persist_mlflow_trace_link(uuid.uuid4(), uuid.uuid4())  # must not raise
