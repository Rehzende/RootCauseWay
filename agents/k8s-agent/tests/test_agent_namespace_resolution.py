"""Regression test for a real bug found live validating the Azure lab chaos
pipeline: Go's NormalizedAlert (backend/internal/models/models.go) promotes
`service` to a top-level field on the alert dict sent to agents (the
normalizer copies it up from the label), but never does the same for
`namespace` -- that one only ever exists nested inside the label map. And
on the Python side (agent-service/app/models/events.py), NormalizedAlert
doesn't even declare a `labels` field -- only `tags` -- so Go's
`Labels: alert.Labels` is silently dropped by Pydantic and never reaches
this agent; only `Tags: alert.Labels` (same data, published under the
other field name) survives. handle_task's old fallback chain checked
`alert.get("namespace")` directly (and, in a first-pass fix, the wrong
`alert.labels.namespace` nesting) and never found it either way, silently
defaulting to "default" -- so kubectl queries hit the wrong namespace,
came back empty, and the RCA agent fabricated a root cause from zero real
evidence (confirmed live, twice: a Chaos Mesh pod-failure fault in
namespace "demo" was diagnosed first as "unhandled exception in
application code" and then as "misconfigured pod resource limits", both
at confidence 0.80, purely because no k8s evidence was ever actually
collected).
"""

from __future__ import annotations

from unittest.mock import AsyncMock, patch

import pytest

from app.a2a.models import DataPart, Message, Role
from app.agent import K8sDebugAgent


@pytest.mark.asyncio
async def test_handle_task_resolves_namespace_from_alert_tags():
    """This is the shape the orchestrator ACTUALLY sends in production:
    alert.namespace doesn't exist, and neither does alert.labels (Pydantic
    drops it) -- only alert.tags.namespace survives the round trip."""
    agent = K8sDebugAgent()
    seen_namespaces: list[str] = []

    async def _fake_collect(namespace, pod, labels, kubeconfig):
        seen_namespaces.append(namespace)
        return {"pods": []}

    with patch.object(agent, "_collect", new=_fake_collect):
        await agent.handle_task(
            "t1",
            Message(
                role=Role.USER,
                parts=[DataPart(data={
                    "alert": {
                        "service": "azure-lab-demo-app",
                        "tags": {"namespace": "demo", "service": "azure-lab-demo-app"},
                    },
                })],
            ),
        )

    assert seen_namespaces == ["demo"]


@pytest.mark.asyncio
async def test_handle_task_still_accepts_nested_alert_labels_if_ever_present():
    """Defensive fallback: if a future producer actually populates
    alert.labels (unlike today's Pydantic model), still use it."""
    agent = K8sDebugAgent()
    seen_namespaces: list[str] = []

    async def _fake_collect(namespace, pod, labels, kubeconfig):
        seen_namespaces.append(namespace)
        return {"pods": []}

    with patch.object(agent, "_collect", new=_fake_collect):
        await agent.handle_task(
            "t1",
            Message(
                role=Role.USER,
                parts=[DataPart(data={
                    "alert": {"labels": {"namespace": "demo"}},
                })],
            ),
        )

    assert seen_namespaces == ["demo"]


@pytest.mark.asyncio
async def test_handle_task_prefers_top_level_namespace_over_alert():
    agent = K8sDebugAgent()
    seen_namespaces: list[str] = []

    async def _fake_collect(namespace, pod, labels, kubeconfig):
        seen_namespaces.append(namespace)
        return {"pods": []}

    with patch.object(agent, "_collect", new=_fake_collect):
        await agent.handle_task(
            "t1",
            Message(
                role=Role.USER,
                parts=[DataPart(data={
                    "namespace": "explicit-ns",
                    "alert": {"tags": {"namespace": "demo"}},
                })],
            ),
        )

    assert seen_namespaces == ["explicit-ns"]


@pytest.mark.asyncio
async def test_handle_task_defaults_to_default_when_nothing_present():
    agent = K8sDebugAgent()
    seen_namespaces: list[str] = []

    async def _fake_collect(namespace, pod, labels, kubeconfig):
        seen_namespaces.append(namespace)
        return {"pods": []}

    with patch.object(agent, "_collect", new=_fake_collect):
        await agent.handle_task(
            "t1",
            Message(role=Role.USER, parts=[DataPart(data={})]),
        )

    assert seen_namespaces == ["default"]
