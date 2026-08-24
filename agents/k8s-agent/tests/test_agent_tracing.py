"""K8sDebugAgent.handle_task had no @mlflow.trace at all -- found live while
validating orchestrator -> agent -> LLM / agent -> peer-agent trace
propagation end to end (see the other 4 agents' identical handle_task
decorator): every other agent's handle_task shows up as its own span
(triage_agent.handle_task, evidence_agent.handle_task, ...), but k8s-agent
was invisible in every trace, whether called via the orchestrator, the
A2A mesh (evidence-agent/rca-agent/postmortem-agent's peer calls), or its
standalone Alertmanager webhook path -- its LLM usage (k8s_agent.analyze_service
/ k8s_agent.analyze_incident) was still traced, just never attached under a
handle_task root, and never inherited a propagated parent context either.
"""

from __future__ import annotations

from unittest.mock import AsyncMock, patch

import mlflow
import pytest

from app.a2a.models import DataPart, Message, Role
from app.agent import K8sDebugAgent


@pytest.mark.asyncio
async def test_handle_task_produces_a_named_span(tmp_path, monkeypatch):
    # get_current_active_span() only tracks real LiveSpans, not the
    # NonRecordingSpan placeholder mlflow uses with no backend configured
    # (confirmed live during the propagation debugging this pins) -- needs
    # a real tracking store for the decorator to produce an observable
    # span. mlflow-skinny (this service's dependency) doesn't support
    # sqlite for that (see the sibling agents' equivalent tests), and the
    # local file store needs this explicit opt-out since MLflow 3.x.
    monkeypatch.setenv("MLFLOW_ALLOW_FILE_STORE", "true")
    prior_uri = mlflow.get_tracking_uri()
    mlflow.set_tracking_uri(f"file://{tmp_path}")
    mlflow.set_experiment("test-k8s-agent-tracing")

    agent = K8sDebugAgent()
    seen_span_names: list[str] = []

    async def _fake_collect(*args, **kwargs):
        span = mlflow.get_current_active_span()
        seen_span_names.append(span.name if span else None)
        return {"pods": []}

    try:
        with patch.object(agent, "_collect", new=_fake_collect):
            result = await agent.handle_task(
                "t1",
                Message(role=Role.USER, parts=[DataPart(data={"namespace": "prod"})]),
            )
    finally:
        mlflow.set_tracking_uri(prior_uri)

    assert seen_span_names == ["k8s_agent.handle_task"]
    assert result.artifacts[0].name == "k8s_cluster_data"
