"""Just-in-time credential provider for agents."""

from __future__ import annotations

import logging
from typing import Any
from uuid import UUID

from app.credentials.models import CredentialLease, LeaseRequest
from app.services.backend_client import BackendClient

logger = logging.getLogger(__name__)


class JITCredentialProvider:
    """Manages just-in-time credential requests for agents.

    Coordinates between the backend API (access policies, leases) and
    the credential provider implementations (Vault, AWS STS, etc.).
    """

    def __init__(self, backend_client: BackendClient) -> None:
        self._backend = backend_client
        self._active_leases: dict[UUID, list[CredentialLease]] = {}  # incident_id -> leases

    async def request_credentials(
        self,
        incident_id: UUID,
        agent_id: UUID,
        skill_id: str,
        software_id: str | UUID,
        resource_type: str,
        org_id: UUID,
        ttl_seconds: int = 900,
        reason: str = "",
    ) -> CredentialLease | None:
        """Request JIT credentials for an agent performing a skill.

        1. Get resource credentials for the software
        2. Filter by resource_type
        3. Evaluate access policy for agent+skill+resource
        4. If policy allows, request a lease from backend
        5. Return lease with credential info
        """
        # 1. Get resource credentials for the software
        try:
            all_credentials = await self._backend.get_software_credentials(software_id, org_id)
        except Exception:
            logger.warning(
                "Failed to fetch credentials for software %s", software_id
            )
            return None

        # 2. Filter by resource_type
        matching = [
            c for c in all_credentials
            if c.get("resource_type") == resource_type
        ]
        if not matching:
            logger.info(
                "No credentials of type '%s' for software %s",
                resource_type, software_id,
            )
            return None

        resource_credential = matching[0]
        resource_credential_id = UUID(resource_credential["id"])

        # 3. Evaluate access policy
        try:
            policy_result = await self._backend.evaluate_access_policy(
                agent_id, skill_id, resource_type
            )
            if not policy_result.get("allowed", False):
                logger.warning(
                    "Access policy denied: agent=%s skill=%s resource_type=%s",
                    agent_id, skill_id, resource_type,
                )
                return None
        except Exception:
            logger.warning("Access policy evaluation failed, denying by default")
            return None

        # 4. Request lease from backend
        lease_request = LeaseRequest(
            incident_id=incident_id,
            agent_id=agent_id,
            skill_id=skill_id,
            resource_credential_id=resource_credential_id,
            ttl_seconds=ttl_seconds,
            scope=policy_result.get("scope", {}),
            reason=reason,
        )

        try:
            lease_data = await self._backend.request_credential_lease(
                lease_request.model_dump(mode="json")
            )
        except Exception:
            logger.warning("Failed to request credential lease")
            return None

        # 5. Return lease
        lease = CredentialLease.model_validate(lease_data)
        self._active_leases.setdefault(incident_id, []).append(lease)
        return lease

    async def revoke_credentials(self, lease_id: UUID) -> None:
        """Revoke an active lease after agent completes."""
        try:
            await self._backend.revoke_credential_lease(lease_id)
        except Exception:
            logger.warning("Failed to revoke credential lease %s", lease_id)

        # Remove from tracking
        for leases in self._active_leases.values():
            self._active_leases[next(
                (iid for iid, ls in self._active_leases.items()
                 if any(l.id == lease_id for l in ls)),
                None,  # type: ignore[arg-type]
            )] = [l for l in leases if l.id != lease_id]
            break

    async def get_active_leases(self, incident_id: UUID) -> list[CredentialLease]:
        """List active leases for an incident."""
        return self._active_leases.get(incident_id, [])

    async def revoke_all_for_incident(self, incident_id: UUID) -> None:
        """Revoke all active leases for an incident."""
        leases = self._active_leases.pop(incident_id, [])
        for lease in leases:
            try:
                await self._backend.revoke_credential_lease(lease.id)
            except Exception:
                logger.warning("Failed to revoke lease %s", lease.id)
