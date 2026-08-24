"""Azure SDK client construction + credential resolution for azure-agent.

Credential precedence (highest to lowest):
1. `task_credentials["azure"]` = {tenant_id, client_id, client_secret} --
   passed per-task by the orchestrator, mirroring the exact pattern
   k8s-agent already uses for `credentials.kubeconfig`
   (agents/k8s-agent/app/agent.py). Not yet wired on the orchestrator side
   (see project backlog: skills' required_resource_types is [] everywhere,
   so the JIT lease-request loop never fires) -- reading it here first
   means this agent needs ZERO changes the day that gets wired up.
2. AZURE_TENANT_ID / AZURE_CLIENT_ID / AZURE_CLIENT_SECRET env vars --
   today's actual path: a static credential set at deploy time (same
   static-config pattern the other 5 RootCauseway services already use for their
   LLM API keys).

The Azure mgmt SDKs (azure-mgmt-monitor, azure-mgmt-network,
azure-mgmt-keyvault, azure-mgmt-authorization) are synchronous -- calls
run via asyncio.to_thread so they don't block the FastAPI event loop.
"""

from __future__ import annotations

import os
from datetime import datetime, timezone
from typing import Any, Protocol

from azure.identity import ClientSecretCredential


def resolve_credential(task_credentials: dict[str, Any] | None) -> ClientSecretCredential | None:
    azure_creds = (task_credentials or {}).get("azure") or {}
    tenant_id = azure_creds.get("tenant_id") or os.getenv("AZURE_TENANT_ID", "")
    client_id = azure_creds.get("client_id") or os.getenv("AZURE_CLIENT_ID", "")
    client_secret = azure_creds.get("client_secret") or os.getenv("AZURE_CLIENT_SECRET", "")
    if not (tenant_id and client_id and client_secret):
        return None
    return ClientSecretCredential(tenant_id=tenant_id, client_id=client_id, client_secret=client_secret)


def resolve_subscription_id(task_credentials: dict[str, Any] | None) -> str:
    azure_creds = (task_credentials or {}).get("azure") or {}
    return azure_creds.get("subscription_id") or os.getenv("AZURE_SUBSCRIPTION_ID", "")


class AzureReader(Protocol):
    """What agent.py actually needs from the Azure SDK -- narrow on
    purpose so tests can inject a fake instead of touching real Azure."""

    async def activity_log_events(
        self, resource_id: str, start: datetime, end: datetime
    ) -> list[dict[str, Any]]: ...

    async def nsg_rules(self, resource_group: str, nsg_name: str) -> dict[str, Any]: ...

    async def keyvault_info(self, resource_group: str, vault_name: str) -> dict[str, Any]: ...


class RealAzureReader:
    """Thin async wrapper over the (synchronous) Azure mgmt SDKs."""

    def __init__(self, credential: ClientSecretCredential, subscription_id: str):
        self._credential = credential
        self._subscription_id = subscription_id

    async def activity_log_events(
        self, resource_id: str, start: datetime, end: datetime
    ) -> list[dict[str, Any]]:
        import asyncio

        def _list() -> list[dict[str, Any]]:
            from azure.mgmt.monitor import MonitorManagementClient

            client = MonitorManagementClient(self._credential, self._subscription_id)
            odata_filter = (
                f"eventTimestamp ge '{start.isoformat()}' and "
                f"eventTimestamp le '{end.isoformat()}' and "
                f"resourceUri eq '{resource_id}'"
            )
            events = client.activity_logs.list(
                filter=odata_filter,
                select="eventName,operationName,status,eventTimestamp,resourceId,caller,level,properties",
            )
            out = []
            for e in events:
                out.append({
                    "event_name": getattr(e.event_name, "value", None) or getattr(e.event_name, "localized_value", None),
                    "operation_name": getattr(e.operation_name, "value", None) or getattr(e.operation_name, "localized_value", None),
                    "status": getattr(e.status, "value", None) or getattr(e.status, "localized_value", None),
                    "level": e.level,
                    "timestamp": e.event_timestamp.isoformat() if e.event_timestamp else None,
                    "caller": e.caller,
                })
            return out

        return await asyncio.to_thread(_list)

    async def nsg_rules(self, resource_group: str, nsg_name: str) -> dict[str, Any]:
        import asyncio

        def _get() -> dict[str, Any]:
            from azure.mgmt.network import NetworkManagementClient

            client = NetworkManagementClient(self._credential, self._subscription_id)
            nsg = client.network_security_groups.get(resource_group, nsg_name)
            rules = [
                {
                    "name": r.name,
                    "priority": r.priority,
                    "direction": r.direction,
                    "access": r.access,
                    "protocol": r.protocol,
                    "source": r.source_address_prefix or r.source_address_prefixes,
                    "destination_port_range": r.destination_port_range or r.destination_port_ranges,
                }
                for r in (nsg.security_rules or [])
            ]
            default_rules = [
                {
                    "name": r.name,
                    "priority": r.priority,
                    "direction": r.direction,
                    "access": r.access,
                }
                for r in (nsg.default_security_rules or [])
            ]
            return {"name": nsg.name, "custom_rules": rules, "default_rules": default_rules}

        return await asyncio.to_thread(_get)

    async def keyvault_info(self, resource_group: str, vault_name: str) -> dict[str, Any]:
        import asyncio

        def _get() -> dict[str, Any]:
            from azure.mgmt.authorization import AuthorizationManagementClient
            from azure.mgmt.keyvault import KeyVaultManagementClient

            kv_client = KeyVaultManagementClient(self._credential, self._subscription_id)
            vault = kv_client.vaults.get(resource_group, vault_name)
            props = vault.properties
            result: dict[str, Any] = {
                "name": vault.name,
                "location": vault.location,
                "rbac_authorization_enabled": props.enable_rbac_authorization,
                "public_network_access": props.public_network_access,
                "soft_delete_enabled": props.enable_soft_delete,
            }
            if props.enable_rbac_authorization:
                authz_client = AuthorizationManagementClient(self._credential, self._subscription_id)
                assignments = authz_client.role_assignments.list_for_scope(vault.id)
                result["role_assignments"] = [
                    {"principal_id": a.principal_id, "role_definition_id": a.role_definition_id}
                    for a in assignments
                ]
            return result

        return await asyncio.to_thread(_get)


def utcnow() -> datetime:
    return datetime.now(timezone.utc)
