package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAzureSecretGetter lets tests exercise ResolveCredentialData's
// azure_key_vault path without a real Azure tenant, service principal, or
// network access -- none of which this test environment has.
type fakeAzureSecretGetter struct {
	gotName, gotVersion string
	value               string
	version             string
	err                 error
}

func (g *fakeAzureSecretGetter) GetSecret(ctx context.Context, name, version string) (azureSecret, error) {
	g.gotName, g.gotVersion = name, version
	if g.err != nil {
		return azureSecret{}, g.err
	}
	return azureSecret{Value: g.value, Version: g.version}, nil
}

// withFakeAzureSecretGetter swaps the package-level Key Vault client
// factory for the test's duration and restores it afterward, so this
// test's fake never leaks into other tests.
func withFakeAzureSecretGetter(t *testing.T, fake *fakeAzureSecretGetter) {
	t.Helper()
	orig := newAzureSecretGetter
	newAzureSecretGetter = func(cfg azureKeyVaultConfig) (azureSecretGetter, error) {
		return fake, nil
	}
	t.Cleanup(func() { newAzureSecretGetter = orig })
}

func TestRequestLease_AzureKeyVaultProvider_ResolvesRealCredentialData(t *testing.T) {
	fake := &fakeAzureSecretGetter{value: "s3cr3t-db-password", version: "abc123"}
	withFakeAzureSecretGetter(t, fake)

	orgID := uuid.New()
	rcID := uuid.New()
	providerID := uuid.New()

	rc := &models.ResourceCredential{ID: rcID, ProviderID: providerID, ResourceType: "database", CredentialPath: "prod-db-password", DefaultTTL: 300}
	provider := &models.CredentialProvider{
		ID: providerID, ProviderType: "azure_key_vault",
		Config: json.RawMessage(`{"vault_url":"https://myvault.vault.azure.net/","tenant_id":"t","client_id":"c","client_secret":"s"}`),
	}
	leaseRepo := &fakeLeaseRepo{}
	svc := NewCredentialLeaseService(leaseRepo, &fakePolicyRepo{}, &fakeResourceCredentialRepo{rc: rc}, &fakeProviderRepo{provider: provider})

	lease, err := svc.RequestLease(context.Background(), orgID, models.RequestLeaseRequest{
		IncidentID: uuid.New(), AgentID: uuid.New(), ResourceCredentialID: rcID,
	})

	require.NoError(t, err)
	require.NotNil(t, lease.CredentialData)
	assert.Equal(t, "s3cr3t-db-password", lease.CredentialData["value"])
	assert.Equal(t, "abc123", lease.CredentialData["secret_version"])
	assert.Equal(t, "prod-db-password", lease.CredentialData["secret_name"])
	assert.Equal(t, "azure_key_vault", lease.CredentialData["provider"])
	assert.Equal(t, "prod-db-password", lease.CredentialData["credential_path"])
	assert.Equal(t, "prod-db-password", fake.gotName)
	assert.Equal(t, "", fake.gotVersion)
	assert.NotNil(t, leaseRepo.created, "the lease audit row must still get persisted")
}

func TestResolveAzureKeyVaultCredential_CredentialPathWithVersion_SplitsNameAndVersion(t *testing.T) {
	fake := &fakeAzureSecretGetter{value: "v"}
	withFakeAzureSecretGetter(t, fake)

	_, err := resolveAzureKeyVaultCredential(context.Background(),
		json.RawMessage(`{"vault_url":"https://v.vault.azure.net/","tenant_id":"t","client_id":"c","client_secret":"s"}`),
		"prod-db-password/9c5f2e3b1a4d4e0f8b6c7d8e9f0a1b2c")

	require.NoError(t, err)
	assert.Equal(t, "prod-db-password", fake.gotName)
	assert.Equal(t, "9c5f2e3b1a4d4e0f8b6c7d8e9f0a1b2c", fake.gotVersion)
}

func TestResolveAzureKeyVaultCredential_MissingConfigFields_ReturnsClearError(t *testing.T) {
	_, err := resolveAzureKeyVaultCredential(context.Background(),
		json.RawMessage(`{"vault_url":"https://v.vault.azure.net/"}`), "some-secret")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant_id")
	assert.Contains(t, err.Error(), "client_id")
	assert.Contains(t, err.Error(), "client_secret")
}

func TestRequestLease_AzureKeyVaultProvider_GetSecretError_DoesNotPersistLease(t *testing.T) {
	fake := &fakeAzureSecretGetter{err: errors.New("key vault: secret not found")}
	withFakeAzureSecretGetter(t, fake)

	orgID := uuid.New()
	rcID := uuid.New()
	providerID := uuid.New()

	rc := &models.ResourceCredential{ID: rcID, ProviderID: providerID, ResourceType: "database", CredentialPath: "missing-secret", DefaultTTL: 300}
	provider := &models.CredentialProvider{
		ID: providerID, ProviderType: "azure_key_vault",
		Config: json.RawMessage(`{"vault_url":"https://myvault.vault.azure.net/","tenant_id":"t","client_id":"c","client_secret":"s"}`),
	}
	leaseRepo := &fakeLeaseRepo{}
	svc := NewCredentialLeaseService(leaseRepo, &fakePolicyRepo{}, &fakeResourceCredentialRepo{rc: rc}, &fakeProviderRepo{provider: provider})

	_, err := svc.RequestLease(context.Background(), orgID, models.RequestLeaseRequest{
		IncidentID: uuid.New(), AgentID: uuid.New(), ResourceCredentialID: rcID,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret not found")
	assert.Nil(t, leaseRepo.created, "must not persist a lease row when Key Vault fetch failed")
}
