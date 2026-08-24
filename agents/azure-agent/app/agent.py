"""Azure Agent — collects evidence directly from Azure's own management
APIs (Activity Log, NSG rules, Key Vault properties/RBAC) instead of
kubectl, so it doesn't depend on cross-cluster Kubernetes credentials the
way an AKS-hosted incident's kubectl-based diagnostics would (see
project backlog: AKS dynamic JIT token-minting is designed but not
implemented -- this agent covers the same "what happened to this Azure
resource" question through a different, already-available door).

Three skills, dispatched by `skill_id` in the task payload:
  - azure-aks-activity-log:   AKS control-plane events (Activity Log)
  - azure-keyvault-diagnostics: Key Vault properties + current RBAC grants
    (NOT a secret-access audit trail -- that needs a Log Analytics
    workspace + diagnostic settings routing to it, neither of which is
    provisioned in this lab; see the class docstring below)
  - azure-network-diagnostics: NSG rule dump + raw TCP reachability check
    against Storage/Postgres host:port (network-only, no SDK, no
    credential -- same "diagnose without storing access data" approach
    already decided for those two resources in the project backlog)

Resource names/IDs come from `software_context.cloud_resources` and
`software_context.databases` (see agent-service/app/orchestrator/
context_builder.py) -- populated once, in the software catalog, not
re-discovered per call.
"""

from __future__ import annotations

import asyncio
import logging
import socket
import time
from datetime import timedelta
from typing import Any

import mlflow
from mlflow.entities import SpanType

from app.a2a.models import Artifact, DataPart, Message, Task, TaskStatus
from app.azure_clients import AzureReader, RealAzureReader, resolve_credential, resolve_subscription_id, utcnow
from app.observability_metrics import record_swallowed_error

logger = logging.getLogger(__name__)

TCP_CHECK_TIMEOUT = 5.0
ACTIVITY_LOG_LOOKBACK = timedelta(hours=2)


class AzureAgent:
    """Dispatches to one of three Azure-native diagnostics, based on the
    task's `skill_id`. `reader_factory` is injectable for tests -- default
    builds a RealAzureReader from the resolved credential."""

    def __init__(self, reader_factory=None, **_kwargs: Any):
        self._reader_factory = reader_factory or self._default_reader_factory

    @staticmethod
    def _default_reader_factory(task_credentials: dict[str, Any] | None, subscription_id: str) -> AzureReader | None:
        credential = resolve_credential(task_credentials)
        if not credential:
            return None
        return RealAzureReader(credential, subscription_id)

    @mlflow.trace(span_type=SpanType.AGENT, name="azure_agent.handle_task")
    async def handle_task(self, task_id: str, message: Message) -> Task:
        input_data = self._extract_data(message)
        skill_id = input_data.get("skill_id", "")
        software_context = input_data.get("software_context") or {}
        cloud_resources = software_context.get("cloud_resources") or {}
        credentials = input_data.get("credentials") or {}

        subscription_id = cloud_resources.get("subscription_id") or resolve_subscription_id(credentials)
        resource_group = cloud_resources.get("resource_group", "")

        try:
            if skill_id == "azure-aks-activity-log":
                result = await self._aks_activity_log(credentials, subscription_id, resource_group, cloud_resources)
            elif skill_id == "azure-keyvault-diagnostics":
                result = await self._keyvault_diagnostics(credentials, subscription_id, resource_group, cloud_resources)
            elif skill_id == "azure-network-diagnostics":
                result = await self._network_diagnostics(credentials, subscription_id, resource_group, cloud_resources, software_context)
            else:
                return Task(
                    id=task_id,
                    status=TaskStatus.FAILED,
                    artifacts=[Artifact(
                        name="error",
                        description="unknown skill_id",
                        parts=[DataPart(data={"error": f"azure-agent has no skill {skill_id!r}"})],
                    )],
                )
        except Exception as exc:
            record_swallowed_error("azure_agent", f"{skill_id or 'unknown'}_failed")
            logger.exception("azure-agent skill %s failed", skill_id)
            return Task(
                id=task_id,
                status=TaskStatus.COMPLETED,
                artifacts=[Artifact(
                    name="azure_diagnostics",
                    description=f"azure-agent {skill_id} failed",
                    parts=[DataPart(data={"error": str(exc)})],
                )],
            )

        return Task(
            id=task_id,
            status=TaskStatus.COMPLETED,
            artifacts=[Artifact(
                name="azure_diagnostics",
                description=f"Azure-native diagnostics for skill={skill_id}",
                parts=[DataPart(data=result)],
            )],
        )

    async def _aks_activity_log(
        self, credentials: dict[str, Any], subscription_id: str, resource_group: str, cloud_resources: dict[str, Any],
    ) -> dict[str, Any]:
        aks_cluster = cloud_resources.get("aks_cluster", "")
        if not (subscription_id and resource_group and aks_cluster):
            return {"error": "missing subscription_id/resource_group/aks_cluster in software_context.cloud_resources"}

        reader = self._reader_factory(credentials, subscription_id)
        if reader is None:
            return {"error": "no Azure credential available (task credentials.azure or AZURE_TENANT_ID/CLIENT_ID/CLIENT_SECRET env)"}

        resource_id = (
            f"/subscriptions/{subscription_id}/resourceGroups/{resource_group}"
            f"/providers/Microsoft.ContainerService/managedClusters/{aks_cluster}"
        )
        end = utcnow()
        start = end - ACTIVITY_LOG_LOOKBACK
        events = await reader.activity_log_events(resource_id, start, end)
        return {
            "resource": resource_id,
            "window": {"start": start.isoformat(), "end": end.isoformat()},
            "event_count": len(events),
            "events": events,
        }

    async def _keyvault_diagnostics(
        self, credentials: dict[str, Any], subscription_id: str, resource_group: str, cloud_resources: dict[str, Any],
    ) -> dict[str, Any]:
        vault_name = cloud_resources.get("key_vault", "")
        if not (subscription_id and resource_group and vault_name):
            return {"error": "missing subscription_id/resource_group/key_vault in software_context.cloud_resources"}

        reader = self._reader_factory(credentials, subscription_id)
        if reader is None:
            return {"error": "no Azure credential available (task credentials.azure or AZURE_TENANT_ID/CLIENT_ID/CLIENT_SECRET env)"}

        info = await reader.keyvault_info(resource_group, vault_name)
        info["note"] = (
            "Vault properties and current RBAC role assignments only -- NOT "
            "a secret-access audit trail. Reading who accessed which secret "
            "requires diagnostic settings routed to a Log Analytics "
            "workspace, not provisioned in this environment."
        )
        return info

    async def _network_diagnostics(
        self,
        credentials: dict[str, Any],
        subscription_id: str,
        resource_group: str,
        cloud_resources: dict[str, Any],
        software_context: dict[str, Any],
    ) -> dict[str, Any]:
        result: dict[str, Any] = {}

        nsg_name = cloud_resources.get("nsg", "")
        if subscription_id and resource_group and nsg_name:
            reader = self._reader_factory(credentials, subscription_id)
            if reader is not None:
                result["nsg"] = await reader.nsg_rules(resource_group, nsg_name)
            else:
                result["nsg"] = {"error": "no Azure credential available"}
        else:
            result["nsg"] = {"error": "missing subscription_id/resource_group/nsg in software_context.cloud_resources"}

        # Network-only reachability -- no SDK, no credential, matches the
        # "Storage/Postgres never get a stored credential, only host:port
        # network diagnostics" decision in the project backlog.
        databases = software_context.get("databases") or {}
        host = databases.get("host", "")
        port = databases.get("port")
        if host and port:
            result["postgres_reachability"] = await _tcp_check(host, int(port))

        return result

    def _extract_data(self, message: Message) -> dict[str, Any]:
        for part in message.parts:
            if isinstance(part, DataPart):
                return part.data
        return {}


async def _tcp_check(host: str, port: int) -> dict[str, Any]:
    """Raw TCP connect -- classifies timeout vs refused vs reset vs DNS
    failure, the same reusable network-only diagnostic already decided in
    the project backlog for Storage/Postgres (no cloud SDK involved)."""
    start = time.monotonic()
    try:
        conn = asyncio.open_connection(host, port)
        _reader, writer = await asyncio.wait_for(conn, timeout=TCP_CHECK_TIMEOUT)
        writer.close()
        try:
            await writer.wait_closed()
        except Exception:
            pass
        return {"host": host, "port": port, "reachable": True, "latency_ms": round((time.monotonic() - start) * 1000, 1)}
    except asyncio.TimeoutError:
        return {"host": host, "port": port, "reachable": False, "error": "timeout", "timeout_s": TCP_CHECK_TIMEOUT}
    except ConnectionRefusedError:
        return {"host": host, "port": port, "reachable": False, "error": "connection_refused"}
    except ConnectionResetError:
        return {"host": host, "port": port, "reachable": False, "error": "connection_reset"}
    except socket.gaierror as exc:
        return {"host": host, "port": port, "reachable": False, "error": "dns_resolution_failed", "detail": str(exc)}
    except OSError as exc:
        return {"host": host, "port": port, "reachable": False, "error": "os_error", "detail": str(exc)}
