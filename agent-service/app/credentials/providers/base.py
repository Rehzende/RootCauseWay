"""Abstract base for credential providers."""

from __future__ import annotations

from abc import ABC, abstractmethod
from typing import Any


class CredentialProviderInterface(ABC):
    """Interface that all credential providers must implement."""

    @abstractmethod
    async def generate_credential(
        self,
        config: dict[str, Any],
        credential_path: str,
        scope: dict[str, Any],
        ttl: int,
    ) -> dict[str, Any]:
        """Generate a temporary credential.

        Args:
            config: Provider-specific configuration.
            credential_path: Path/identifier for the credential resource.
            scope: Access scope restrictions.
            ttl: Time-to-live in seconds.

        Returns:
            Dictionary with credential data (keys vary by provider).
        """

    @abstractmethod
    async def revoke_credential(self, credential_data: dict[str, Any]) -> None:
        """Revoke a previously issued credential.

        Args:
            credential_data: The credential data returned by generate_credential.
        """
