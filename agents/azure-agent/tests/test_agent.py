from __future__ import annotations

import socket
from datetime import datetime, timezone
from typing import Any
from unittest.mock import AsyncMock

import pytest

from app.a2a.models import DataPart, Message, Role, TaskStatus
from app.agent import AzureAgent, _tcp_check


class FakeReader:
    def __init__(self):
        self.activity_log_calls: list[tuple] = []
        self.nsg_calls: list[tuple] = []
        self.keyvault_calls: list[tuple] = []

    async def activity_log_events(self, resource_id, start, end):
        self.activity_log_calls.append((resource_id, start, end))
        return [{"event_name": "Microsoft.ContainerService/managedClusters/write", "status": "Succeeded"}]

    async def nsg_rules(self, resource_group, nsg_name):
        self.nsg_calls.append((resource_group, nsg_name))
        return {"name": nsg_name, "custom_rules": [{"name": "allow-outbound", "priority": 2000}], "default_rules": []}

    async def keyvault_info(self, resource_group, vault_name):
        self.keyvault_calls.append((resource_group, vault_name))
        return {"name": vault_name, "rbac_authorization_enabled": True, "role_assignments": []}


def _software_context(**overrides) -> dict[str, Any]:
    base = {
        "cloud_resources": {
            "subscription_id": "sub-1",
            "resource_group": "rootcauseway-azure-lab",
            "aks_cluster": "rootcauseway-lab-aks",
            "key_vault": "rootcauseway-lab-kv",
            "nsg": "rootcauseway-lab-nsg",
        },
        "databases": {"host": "127.0.0.1", "port": 1},
    }
    base.update(overrides)
    return base


def _message(skill_id: str, software_context: dict[str, Any], credentials: dict[str, Any] | None = None) -> Message:
    data = {"skill_id": skill_id, "software_context": software_context}
    if credentials:
        data["credentials"] = credentials
    return Message(role=Role.USER, parts=[DataPart(data=data)])


@pytest.mark.asyncio
async def test_aks_activity_log_returns_events():
    fake = FakeReader()
    agent = AzureAgent(reader_factory=lambda creds, sub: fake)

    task = await agent.handle_task("t1", _message("azure-aks-activity-log", _software_context()))

    assert task.status == TaskStatus.COMPLETED
    result = task.artifacts[0].parts[0].data
    assert result["event_count"] == 1
    assert "managedClusters/rootcauseway-lab-aks" in result["resource"]
    assert fake.activity_log_calls[0][0].endswith("/managedClusters/rootcauseway-lab-aks")


@pytest.mark.asyncio
async def test_aks_activity_log_missing_cluster_name_returns_error_not_crash():
    fake = FakeReader()
    agent = AzureAgent(reader_factory=lambda creds, sub: fake)
    ctx = _software_context()
    del ctx["cloud_resources"]["aks_cluster"]

    task = await agent.handle_task("t1", _message("azure-aks-activity-log", ctx))

    assert task.status == TaskStatus.COMPLETED
    result = task.artifacts[0].parts[0].data
    assert "error" in result
    assert fake.activity_log_calls == []


@pytest.mark.asyncio
async def test_no_credential_returns_graceful_error():
    agent = AzureAgent(reader_factory=lambda creds, sub: None)

    task = await agent.handle_task("t1", _message("azure-aks-activity-log", _software_context()))

    result = task.artifacts[0].parts[0].data
    assert "error" in result
    assert "credential" in result["error"]


@pytest.mark.asyncio
async def test_keyvault_diagnostics_never_returns_secret_content():
    fake = FakeReader()
    agent = AzureAgent(reader_factory=lambda creds, sub: fake)

    task = await agent.handle_task("t1", _message("azure-keyvault-diagnostics", _software_context()))

    result = task.artifacts[0].parts[0].data
    assert result["name"] == "rootcauseway-lab-kv"
    assert "secret" not in str(result).lower() or "note" in result
    assert "audit trail" in result["note"]


@pytest.mark.asyncio
async def test_network_diagnostics_combines_nsg_and_tcp_check():
    fake = FakeReader()
    agent = AzureAgent(reader_factory=lambda creds, sub: fake)

    task = await agent.handle_task("t1", _message("azure-network-diagnostics", _software_context()))

    result = task.artifacts[0].parts[0].data
    assert result["nsg"]["name"] == "rootcauseway-lab-nsg"
    assert "postgres_reachability" in result
    # port 1 on localhost should refuse immediately
    assert result["postgres_reachability"]["reachable"] is False


@pytest.mark.asyncio
async def test_unknown_skill_id_fails_the_task():
    agent = AzureAgent(reader_factory=lambda creds, sub: FakeReader())

    task = await agent.handle_task("t1", _message("not-a-real-skill", _software_context()))

    assert task.status == TaskStatus.FAILED


@pytest.mark.asyncio
async def test_skill_exception_is_caught_and_recorded_not_raised():
    class BoomReader:
        async def activity_log_events(self, *a, **kw):
            raise RuntimeError("boom")

    agent = AzureAgent(reader_factory=lambda creds, sub: BoomReader())

    task = await agent.handle_task("t1", _message("azure-aks-activity-log", _software_context()))

    assert task.status == TaskStatus.COMPLETED
    assert "boom" in task.artifacts[0].parts[0].data["error"]


# --- _tcp_check classification ---


@pytest.mark.asyncio
async def test_tcp_check_connection_refused():
    # Nothing listens on port 1 on localhost.
    result = await _tcp_check("127.0.0.1", 1)
    assert result["reachable"] is False
    assert result["error"] == "connection_refused"


@pytest.mark.asyncio
async def test_tcp_check_dns_failure():
    result = await _tcp_check("this-host-does-not-exist.invalid", 5432)
    assert result["reachable"] is False
    assert result["error"] == "dns_resolution_failed"


@pytest.mark.asyncio
async def test_tcp_check_reachable():
    srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    srv.bind(("127.0.0.1", 0))
    srv.listen(1)
    port = srv.getsockname()[1]
    try:
        result = await _tcp_check("127.0.0.1", port)
        assert result["reachable"] is True
        assert "latency_ms" in result
    finally:
        srv.close()
