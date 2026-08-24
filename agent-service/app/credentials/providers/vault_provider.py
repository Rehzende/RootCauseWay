"""HashiCorp Vault credential provider."""

from __future__ import annotations

import logging
from typing import Any

import httpx

from app.credentials.providers.base import CredentialProviderInterface

logger = logging.getLogger(__name__)


class VaultProvider(CredentialProviderInterface):
    """Generates dynamic secrets via HashiCorp Vault API.

    Supports:
    - Kubernetes auth method
    - Database dynamic credentials
    - AWS STS credential generation via Vault
    """

    def __init__(self) -> None:
        self._client: httpx.AsyncClient | None = None

    def _get_client(self, config: dict[str, Any]) -> httpx.AsyncClient:
        if self._client is None:
            vault_addr = config.get("vault_addr", "http://127.0.0.1:8200")
            vault_token = config.get("vault_token", "")
            self._client = httpx.AsyncClient(
                base_url=vault_addr,
                headers={"X-Vault-Token": vault_token},
                timeout=30.0,
            )
        return self._client

    async def generate_credential(
        self,
        config: dict[str, Any],
        credential_path: str,
        scope: dict[str, Any],
        ttl: int,
    ) -> dict[str, Any]:
        client = self._get_client(config)
        resp = await client.post(
            f"/v1/{credential_path}",
            json={"ttl": f"{ttl}s", **scope},
        )
        resp.raise_for_status()
        data = resp.json()
        return {
            "provider": "hashicorp_vault",
            "lease_id": data.get("lease_id", ""),
            "credential_path": credential_path,
            **data.get("data", {}),
        }

    async def revoke_credential(self, credential_data: dict[str, Any]) -> None:
        lease_id = credential_data.get("lease_id")
        if not lease_id:
            return
        config = credential_data.get("_config", {})
        client = self._get_client(config)
        try:
            await client.post("/v1/sys/leases/revoke", json={"lease_id": lease_id})
        except Exception:
            logger.warning("Failed to revoke Vault lease %s", lease_id)
