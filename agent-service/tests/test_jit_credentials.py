"""Tests for JIT credential management."""

from __future__ import annotations

import time
from unittest.mock import AsyncMock, patch
from uuid import UUID, uuid4

import pytest

from app.credentials.jit_provider import JITCredentialProvider
from app.credentials.models import CredentialLease, LeaseStatus
from app.credentials.providers.factory import get_provider
from app.credentials.providers.static_provider import StaticProvider


# --- StaticProvider tests ---


class TestStaticProvider:
    @pytest.fixture
    def provider(self):
        return StaticProvider()

    async def test_generate_credential(self, provider):
        config = {"credentials": {"username": "test_user", "password": "test_pass"}}
        result = await provider.generate_credential(
            config=config,
            credential_path="db/test",
            scope={"database": "mydb"},
            ttl=300,
        )

        assert result["provider"] == "static"
        assert result["username"] == "test_user"
        assert result["password"] == "test_pass"
        assert result["credential_path"] == "db/test"
        assert result["scope"] == {"database": "mydb"}
        assert result["expires_at"] > result["issued_at"]
        assert result["expires_at"] - result["issued_at"] == pytest.approx(300, abs=1)

    async def test_generate_credential_tracks_active(self, provider):
        config = {"credentials": {"token": "abc"}}
        result = await provider.generate_credential(config, "path", {}, 60)
        cred_id = result["credential_id"]
        assert cred_id in provider._active_credentials

    async def test_revoke_credential(self, provider):
        config = {"credentials": {"token": "abc"}}
        result = await provider.generate_credential(config, "path", {}, 60)
        cred_id = result["credential_id"]
        assert cred_id in provider._active_credentials

        await provider.revoke_credential(result)
        assert cred_id not in provider._active_credentials

    async def test_revoke_nonexistent_credential(self, provider):
        # Should not raise
        await provider.revoke_credential({"credential_id": "nonexistent"})
        await provider.revoke_credential({})


# --- Factory tests ---


class TestProviderFactory:
    def test_get_static_provider(self):
        p = get_provider("static")
        assert isinstance(p, StaticProvider)

    def test_get_vault_provider(self):
        from app.credentials.providers.vault_provider import VaultProvider
        p = get_provider("hashicorp_vault")
        assert isinstance(p, VaultProvider)

    def test_get_aws_provider(self):
        from app.credentials.providers.aws_provider import AWSSTSProvider
        p = get_provider("aws_sts")
        assert isinstance(p, AWSSTSProvider)

    def test_get_azure_provider(self):
        from app.credentials.providers.azure_provider import AzureMIProvider
        p = get_provider("azure_managed_identity")
        assert isinstance(p, AzureMIProvider)

    def test_unknown_provider_raises(self):
        with pytest.raises(KeyError, match="Unknown credential provider"):
            get_provider("nonexistent")


# --- JITCredentialProvider tests ---


class TestJITCredentialProvider:
    @pytest.fixture
    def mock_backend(self):
        backend = AsyncMock()
        return backend

    @pytest.fixture
    def jit(self, mock_backend):
        return JITCredentialProvider(mock_backend)

    async def test_request_credentials_success(self, jit, mock_backend):
        incident_id = uuid4()
        agent_id = uuid4()
        software_id = str(uuid4())
        lease_id = uuid4()

        mock_backend.get_software_credentials.return_value = [
            {"id": str(uuid4()), "resource_type": "database", "provider": "static"},
        ]
        mock_backend.evaluate_access_policy.return_value = {
            "allowed": True,
            "scope": {"read_only": True},
        }
        mock_backend.request_credential_lease.return_value = {
            "id": str(lease_id),
            "incident_id": str(incident_id),
            "agent_id": str(agent_id),
            "skill_id": "db-analysis",
            "resource_credential_id": str(uuid4()),
            "status": "active",
            "scope": {"read_only": True},
            "credential_data": {"username": "temp_user", "password": "temp_pass"},
            "issued_at": "2025-01-01T00:00:00",
            "expires_at": "2025-01-01T00:15:00",
        }

        lease = await jit.request_credentials(
            incident_id=incident_id,
            agent_id=agent_id,
            skill_id="db-analysis",
            software_id=software_id,
            resource_type="database",
            org_id=uuid4(),
        )

        assert lease is not None
        assert lease.id == lease_id
        assert lease.credential_data["username"] == "temp_user"
        mock_backend.evaluate_access_policy.assert_called_once()
        mock_backend.request_credential_lease.assert_called_once()

    async def test_request_credentials_denied_by_policy(self, jit, mock_backend):
        mock_backend.get_software_credentials.return_value = [
            {"id": str(uuid4()), "resource_type": "database"},
        ]
        mock_backend.evaluate_access_policy.return_value = {"allowed": False}

        lease = await jit.request_credentials(
            incident_id=uuid4(),
            agent_id=uuid4(),
            skill_id="db-analysis",
            software_id=str(uuid4()),
            resource_type="database",
            org_id=uuid4(),
        )

        assert lease is None
        mock_backend.request_credential_lease.assert_not_called()

    async def test_request_credentials_no_matching_type(self, jit, mock_backend):
        mock_backend.get_software_credentials.return_value = [
            {"id": str(uuid4()), "resource_type": "kubernetes_cluster"},
        ]

        lease = await jit.request_credentials(
            incident_id=uuid4(),
            agent_id=uuid4(),
            skill_id="db-analysis",
            software_id=str(uuid4()),
            resource_type="database",
            org_id=uuid4(),
        )

        assert lease is None

    async def test_request_credentials_backend_failure(self, jit, mock_backend):
        mock_backend.get_software_credentials.side_effect = Exception("connection error")

        lease = await jit.request_credentials(
            incident_id=uuid4(),
            agent_id=uuid4(),
            skill_id="db-analysis",
            software_id=str(uuid4()),
            resource_type="database",
            org_id=uuid4(),
        )

        assert lease is None

    async def test_revoke_credentials(self, jit, mock_backend):
        lease_id = uuid4()
        mock_backend.revoke_credential_lease.return_value = {"status": "revoked"}

        await jit.revoke_credentials(lease_id)
        mock_backend.revoke_credential_lease.assert_called_once_with(lease_id)

    async def test_get_active_leases_empty(self, jit):
        leases = await jit.get_active_leases(uuid4())
        assert leases == []

    async def test_credential_ttl_enforcement(self):
        """Verify static provider sets correct TTL boundaries."""
        provider = StaticProvider()
        ttl = 120
        before = time.time()
        result = await provider.generate_credential(
            config={"credentials": {}},
            credential_path="test",
            scope={},
            ttl=ttl,
        )
        after = time.time()

        assert result["issued_at"] >= before
        assert result["issued_at"] <= after
        assert result["expires_at"] >= before + ttl
        assert result["expires_at"] <= after + ttl
