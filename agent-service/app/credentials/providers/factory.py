"""Factory for credential providers."""

from __future__ import annotations

from app.credentials.providers.aws_provider import AWSSTSProvider
from app.credentials.providers.azure_provider import AzureMIProvider
from app.credentials.providers.base import CredentialProviderInterface
from app.credentials.providers.static_provider import StaticProvider
from app.credentials.providers.vault_provider import VaultProvider

_PROVIDERS: dict[str, type[CredentialProviderInterface]] = {
    "hashicorp_vault": VaultProvider,
    "aws_sts": AWSSTSProvider,
    "azure_managed_identity": AzureMIProvider,
    "static": StaticProvider,
}


def get_provider(provider_type: str) -> CredentialProviderInterface:
    """Return an instance of the requested credential provider.

    Args:
        provider_type: One of 'hashicorp_vault', 'aws_sts',
                       'azure_managed_identity', 'static'.

    Raises:
        KeyError: If the provider type is not registered.
    """
    cls = _PROVIDERS.get(provider_type)
    if cls is None:
        raise KeyError(
            f"Unknown credential provider '{provider_type}'. "
            f"Available: {', '.join(_PROVIDERS)}"
        )
    return cls()
