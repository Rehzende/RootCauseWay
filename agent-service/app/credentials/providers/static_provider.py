"""Static credential provider for development/MVP."""

from __future__ import annotations

import time
from typing import Any
from uuid import uuid4

from app.credentials.providers.base import CredentialProviderInterface


class StaticProvider(CredentialProviderInterface):
    """Returns pre-configured static credentials with TTL tracking.

    Useful for development, testing, and MVP deployments where dynamic
    secret generation is not yet configured.
    """

    def __init__(self) -> None:
        self._active_credentials: dict[str, dict[str, Any]] = {}

    async def generate_credential(
        self,
        config: dict[str, Any],
        credential_path: str,
        scope: dict[str, Any],
        ttl: int,
    ) -> dict[str, Any]:
        credential_id = str(uuid4())
        now = time.time()
        credential_data = {
            "credential_id": credential_id,
            "provider": "static",
            "credential_path": credential_path,
            "scope": scope,
            "issued_at": now,
            "expires_at": now + ttl,
            # Static credentials come from config
            **config.get("credentials", {}),
        }
        self._active_credentials[credential_id] = credential_data
        return credential_data

    async def revoke_credential(self, credential_data: dict[str, Any]) -> None:
        cred_id = credential_data.get("credential_id")
        if cred_id and cred_id in self._active_credentials:
            del self._active_credentials[cred_id]
