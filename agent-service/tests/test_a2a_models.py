"""Tests for A2A protocol models."""

from __future__ import annotations

import pytest

from app.a2a.models import (
    AgentCapabilities,
    AgentCard,
    AgentSkill,
    Artifact,
    DataPart,
    FilePart,
    JSONRPCRequest,
    JSONRPCResponse,
    Message,
    Role,
    Task,
    TaskStatus,
    TextPart,
)


class TestAgentCard:
    def test_minimal_card(self):
        card = AgentCard(name="test", url="http://localhost:8090", version="0.1.0")
        assert card.name == "test"
        assert card.skills == []
        assert card.capabilities.streaming is False

    def test_card_with_skills(self):
        card = AgentCard(
            name="test",
            url="http://localhost:8090",
            version="0.1.0",
            skills=[
                AgentSkill(id="triage", name="Triage", description="Alert triage"),
            ],
        )
        assert len(card.skills) == 1
        assert card.skills[0].id == "triage"

    def test_card_serialization_aliases(self):
        card = AgentCard(
            name="test",
            url="http://localhost:8090",
            version="0.1.0",
            capabilities=AgentCapabilities(streaming=True, push_notifications=True),
        )
        dumped = card.model_dump(by_alias=True)
        assert "pushNotifications" in dumped["capabilities"]
        assert dumped["capabilities"]["pushNotifications"] is True

    def test_card_validation_required_fields(self):
        with pytest.raises(Exception):
            AgentCard()  # missing required fields

    def test_skill_required_resource_types_defaults_empty(self):
        skill = AgentSkill(id="triage", name="Triage", description="Alert triage")
        assert skill.required_resource_types == []

    def test_skill_required_resource_types_survives_serialize_then_reparse(self):
        # Regression test: this is exactly the round trip the orchestrator does --
        # an agent (e.g. k8s-agent) declares required_resource_types on its own
        # AgentSkill, serves it via .well-known/agent.json (card.model_dump), and
        # the orchestrator's A2AClient.discover() re-parses that JSON with this
        # same AgentCard model (AgentCard.model_validate). Before this field
        # existed on AgentSkill, Pydantic silently dropped it on either hop --
        # the LLM prompt would show "requires: none" for a skill that actually
        # needs a JIT-leased kubernetes_cluster credential.
        card = AgentCard(
            name="k8s-agent",
            url="http://k8s-agent:8094",
            version="0.1.0",
            skills=[
                AgentSkill(
                    id="k8s-debug",
                    name="K8s Debug",
                    description="Debug Kubernetes issues",
                    required_resource_types=["kubernetes_cluster"],
                ),
            ],
        )
        dumped = card.model_dump(by_alias=True)
        assert dumped["skills"][0]["required_resource_types"] == ["kubernetes_cluster"]

        reparsed = AgentCard.model_validate(dumped)
        assert reparsed.skills[0].required_resource_types == ["kubernetes_cluster"]


class TestTask:
    def test_default_status(self):
        task = Task(id="task-1")
        assert task.status == TaskStatus.SUBMITTED
        assert task.artifacts == []

    def test_lifecycle_states(self):
        for status in TaskStatus:
            task = Task(id="task-1", status=status)
            assert task.status == status

    def test_task_with_artifacts(self):
        task = Task(
            id="task-1",
            status=TaskStatus.COMPLETED,
            artifacts=[
                Artifact(
                    name="result",
                    parts=[DataPart(data={"key": "value"})],
                )
            ],
        )
        assert len(task.artifacts) == 1
        assert task.artifacts[0].name == "result"

    def test_task_serialization_roundtrip(self):
        task = Task(
            id="task-1",
            status=TaskStatus.COMPLETED,
            message=Message(role=Role.USER, parts=[TextPart(text="hello")]),
            artifacts=[
                Artifact(name="r", parts=[DataPart(data={"x": 1})]),
            ],
        )
        dumped = task.model_dump()
        restored = Task.model_validate(dumped)
        assert restored.id == task.id
        assert restored.status == task.status


class TestMessageParts:
    def test_text_part(self):
        part = TextPart(text="hello")
        assert part.type == "text"
        assert part.text == "hello"

    def test_data_part(self):
        part = DataPart(data={"key": "val"})
        assert part.type == "data"
        assert part.data["key"] == "val"

    def test_file_part(self):
        part = FilePart(file={"name": "test.txt", "mimeType": "text/plain", "bytes": "aGVsbG8="})
        assert part.type == "file"

    def test_message_with_mixed_parts(self):
        msg = Message(
            role=Role.AGENT,
            parts=[
                TextPart(text="here is data"),
                DataPart(data={"result": 42}),
            ],
        )
        assert len(msg.parts) == 2
        assert msg.role == Role.AGENT

    def test_message_serialization(self):
        msg = Message(role=Role.USER, parts=[TextPart(text="test")])
        dumped = msg.model_dump()
        assert dumped["role"] == "user"
        assert dumped["parts"][0]["type"] == "text"


class TestJSONRPC:
    def test_request(self):
        req = JSONRPCRequest(method="tasks/send", params={"id": "1"}, id=1)
        assert req.jsonrpc == "2.0"
        assert req.method == "tasks/send"

    def test_response_success(self):
        resp = JSONRPCResponse(result={"status": "ok"}, id=1)
        assert resp.error is None
        assert resp.result == {"status": "ok"}

    def test_response_error(self):
        from app.a2a.models import JSONRPCError

        resp = JSONRPCResponse(error=JSONRPCError(code=-32601, message="not found"), id=1)
        assert resp.result is None
        assert resp.error.code == -32601
