"""AWS STS credential provider."""

from __future__ import annotations

import logging
from typing import Any

from app.credentials.providers.base import CredentialProviderInterface

logger = logging.getLogger(__name__)


class AWSSTSProvider(CredentialProviderInterface):
    """Generates temporary AWS credentials via STS AssumeRole.

    Uses session tags for fine-grained scoping of assumed roles.
    """

    async def generate_credential(
        self,
        config: dict[str, Any],
        credential_path: str,
        scope: dict[str, Any],
        ttl: int,
    ) -> dict[str, Any]:
        try:
            import boto3  # type: ignore[import-untyped]
        except ImportError:
            raise RuntimeError("boto3 is required for AWS STS provider")

        role_arn = config.get("role_arn", credential_path)
        session_name = scope.get("session_name", "rootcauseway-agent-session")
        tags = [
            {"Key": k, "Value": str(v)}
            for k, v in scope.get("session_tags", {}).items()
        ]

        sts = boto3.client("sts")
        params: dict[str, Any] = {
            "RoleArn": role_arn,
            "RoleSessionName": session_name,
            "DurationSeconds": min(ttl, 3600),
        }
        if tags:
            params["Tags"] = tags

        response = sts.assume_role(**params)
        creds = response["Credentials"]
        return {
            "provider": "aws_sts",
            "access_key_id": creds["AccessKeyId"],
            "secret_access_key": creds["SecretAccessKey"],
            "session_token": creds["SessionToken"],
            "expiration": creds["Expiration"].isoformat(),
        }

    async def revoke_credential(self, credential_data: dict[str, Any]) -> None:
        # AWS STS temporary credentials cannot be explicitly revoked;
        # they expire naturally based on the TTL.
        logger.debug("AWS STS credentials expire naturally, no explicit revocation")
