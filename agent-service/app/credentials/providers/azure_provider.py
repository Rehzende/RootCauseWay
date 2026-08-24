"""Azure Managed Identity credential provider."""

from __future__ import annotations

import logging
from typing import Any

import httpx

from app.credentials.providers.base import CredentialProviderInterface

logger = logging.getLogger(__name__)

AZURE_IMDS_URL = "http://169.254.169.254/metadata/identity/oauth2/token"


class AzureMIProvider(CredentialProviderInterface):
    """Gets tokens from the Azure Instance Metadata Service (IMDS).

    Supports custom scopes for accessing different Azure resources.
    """

    async def generate_credential(
        self,
        config: dict[str, Any],
        credential_path: str,
        scope: dict[str, Any],
        ttl: int,
    ) -> dict[str, Any]:
        resource = scope.get("resource", credential_path)
        client_id = config.get("client_id")

        params: dict[str, str] = {
            "api-version": "2019-08-01",
            "resource": resource,
        }
        if client_id:
            params["client_id"] = client_id

        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.get(
                AZURE_IMDS_URL,
                params=params,
                headers={"Metadata": "true"},
            )
            resp.raise_for_status()
            data = resp.json()

        return {
            "provider": "azure_managed_identity",
            "access_token": data["access_token"],
            "token_type": data.get("token_type", "Bearer"),
            "expires_on": data.get("expires_on", ""),
            "resource": resource,
        }

    async def revoke_credential(self, credential_data: dict[str, Any]) -> None:
        # Azure MI tokens cannot be revoked; they expire naturally.
        logger.debug("Azure MI tokens expire naturally, no explicit revocation")
