package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAKSTokenMinter lets tests exercise ResolveCredentialData's
// azure_aks_jit path without a real Azure tenant, service principal, or
// network access -- none of which this test environment has.
type fakeAKSTokenMinter struct {
	accessToken string
	expiresOn   time.Time
	err         error
}

func (m *fakeAKSTokenMinter) MintToken(ctx context.Context) (aksToken, error) {
	if m.err != nil {
		return aksToken{}, m.err
	}
	return aksToken{AccessToken: m.accessToken, ExpiresOn: m.expiresOn}, nil
}

// withFakeAKSTokenMinter swaps the package-level token minter factory for
// the test's duration and restores it afterward, so this test's fake
// never leaks into other tests.
func withFakeAKSTokenMinter(t *testing.T, fake *fakeAKSTokenMinter) {
	t.Helper()
	orig := newAKSTokenMinter
	newAKSTokenMinter = func(cfg azureAKSJITConfig) (aksTokenMinter, error) {
		return fake, nil
	}
	t.Cleanup(func() { newAKSTokenMinter = orig })
}

const fakeCAData = "ZmFrZS1jYS1jZXJ0" // base64("fake-ca-cert")

func aksJITConfigJSON() string {
	return `{"server":"https://rootcauseway-lab-aks.hcp.brazilsouth.azmk8s.io:443","certificate_authority_data":"` + fakeCAData + `","tenant_id":"t","client_id":"c","client_secret":"s"}`
}

func TestRequestLease_AzureAKSJITProvider_ResolvesRealCredentialData(t *testing.T) {
	expiry := time.Now().Add(1 * time.Hour)
	fake := &fakeAKSTokenMinter{accessToken: "fresh-aks-token", expiresOn: expiry}
	withFakeAKSTokenMinter(t, fake)

	orgID := uuid.New()
	rcID := uuid.New()
	providerID := uuid.New()

	rc := &models.ResourceCredential{ID: rcID, ProviderID: providerID, ResourceType: "kubernetes_cluster", CredentialPath: "rootcauseway-lab-aks", DefaultTTL: 300}
	provider := &models.CredentialProvider{
		ID: providerID, ProviderType: "azure_aks_jit",
		Config: json.RawMessage(aksJITConfigJSON()),
	}
	leaseRepo := &fakeLeaseRepo{}
	svc := NewCredentialLeaseService(leaseRepo, &fakePolicyRepo{}, &fakeResourceCredentialRepo{rc: rc}, &fakeProviderRepo{provider: provider})

	lease, err := svc.RequestLease(context.Background(), orgID, models.RequestLeaseRequest{
		IncidentID: uuid.New(), AgentID: uuid.New(), ResourceCredentialID: rcID,
	})

	require.NoError(t, err)
	require.NotNil(t, lease.CredentialData)
	assert.Equal(t, "azure_aks_jit", lease.CredentialData["provider"])
	assert.Equal(t, "rootcauseway-lab-aks", lease.CredentialData["credential_path"])

	kubeconfig, ok := lease.CredentialData["kubeconfig"].(string)
	require.True(t, ok, "kubeconfig must be present as a string")
	assert.Contains(t, kubeconfig, "fresh-aks-token", "the freshly minted token must be embedded")
	assert.Contains(t, kubeconfig, "https://rootcauseway-lab-aks.hcp.brazilsouth.azmk8s.io:443")
	assert.Contains(t, kubeconfig, fakeCAData)
	assert.NotContains(t, kubeconfig, "exec", "must embed the raw token, not an exec-plugin (kubelogin) config -- k8s-agent has no kubelogin binary")
	assert.NotNil(t, leaseRepo.created, "the lease audit row must still get persisted")
}

func TestResolveAzureAKSJITCredential_MissingConfigFields_ReturnsClearError(t *testing.T) {
	_, err := resolveAzureAKSJITCredential(context.Background(),
		json.RawMessage(`{"server":"https://example.com"}`), "some-cluster")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate_authority_data")
	assert.Contains(t, err.Error(), "tenant_id")
	assert.Contains(t, err.Error(), "client_id")
	assert.Contains(t, err.Error(), "client_secret")
}

func TestResolveAzureAKSJITCredential_InvalidBase64CA_ReturnsClearError(t *testing.T) {
	cfg := `{"server":"https://example.com","certificate_authority_data":"not-valid-base64!!!","tenant_id":"t","client_id":"c","client_secret":"s"}`
	_, err := resolveAzureAKSJITCredential(context.Background(), json.RawMessage(cfg), "some-cluster")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "certificate_authority_data")
	assert.Contains(t, err.Error(), "base64")
}

func TestRequestLease_AzureAKSJITProvider_MintTokenError_DoesNotPersistLease(t *testing.T) {
	fake := &fakeAKSTokenMinter{err: errors.New("AADSTS_FAKE: token request failed")}
	withFakeAKSTokenMinter(t, fake)

	orgID := uuid.New()
	rcID := uuid.New()
	providerID := uuid.New()

	rc := &models.ResourceCredential{ID: rcID, ProviderID: providerID, ResourceType: "kubernetes_cluster", CredentialPath: "rootcauseway-lab-aks", DefaultTTL: 300}
	provider := &models.CredentialProvider{
		ID: providerID, ProviderType: "azure_aks_jit",
		Config: json.RawMessage(aksJITConfigJSON()),
	}
	leaseRepo := &fakeLeaseRepo{}
	svc := NewCredentialLeaseService(leaseRepo, &fakePolicyRepo{}, &fakeResourceCredentialRepo{rc: rc}, &fakeProviderRepo{provider: provider})

	_, err := svc.RequestLease(context.Background(), orgID, models.RequestLeaseRequest{
		IncidentID: uuid.New(), AgentID: uuid.New(), ResourceCredentialID: rcID,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "token request failed")
	assert.Nil(t, leaseRepo.created, "must not persist a lease row when minting the AKS token failed")
}

// TestRequestLease_AzureAKSJITProvider_CapsLeaseExpiryToRealTokenLifetime
// guards the core "this is real JIT, not a decorative TTL" design
// requirement from the project backlog: a lease must never claim validity
// longer than the actual Azure AD token it wraps. Requests a 1-hour lease
// but the token itself only lives 5 more minutes -- the lease's
// expires_at must reflect the shorter, real expiry.
func TestRequestLease_AzureAKSJITProvider_CapsLeaseExpiryToRealTokenLifetime(t *testing.T) {
	shortExpiry := time.Now().Add(5 * time.Minute)
	fake := &fakeAKSTokenMinter{accessToken: "soon-to-expire-token", expiresOn: shortExpiry}
	withFakeAKSTokenMinter(t, fake)

	orgID := uuid.New()
	rcID := uuid.New()
	providerID := uuid.New()

	rc := &models.ResourceCredential{ID: rcID, ProviderID: providerID, ResourceType: "kubernetes_cluster", CredentialPath: "rootcauseway-lab-aks", DefaultTTL: 3600}
	provider := &models.CredentialProvider{
		ID: providerID, ProviderType: "azure_aks_jit",
		Config: json.RawMessage(aksJITConfigJSON()),
	}
	leaseRepo := &fakeLeaseRepo{}
	svc := NewCredentialLeaseService(leaseRepo, &fakePolicyRepo{}, &fakeResourceCredentialRepo{rc: rc}, &fakeProviderRepo{provider: provider})

	lease, err := svc.RequestLease(context.Background(), orgID, models.RequestLeaseRequest{
		IncidentID: uuid.New(), AgentID: uuid.New(), ResourceCredentialID: rcID, TTLSeconds: 3600,
	})

	require.NoError(t, err)
	expiresAt, ok := lease.CredentialData["expires_at"].(int64)
	require.True(t, ok)
	assert.LessOrEqual(t, expiresAt, shortExpiry.Unix(), "lease must not outlive the real Azure AD token")
	assert.Greater(t, expiresAt, time.Now().Unix(), "sanity: still a real, non-expired timestamp")
}

func TestBuildEphemeralKubeconfig_EmbedsRawTokenNoExecPlugin(t *testing.T) {
	kubeconfig := buildEphemeralKubeconfig("https://example.com:443", fakeCAData, "raw-token-value")

	assert.True(t, strings.Contains(kubeconfig, "token: raw-token-value"))
	assert.Contains(t, kubeconfig, "server: https://example.com:443")
	assert.Contains(t, kubeconfig, "certificate-authority-data: "+fakeCAData)
	assert.NotContains(t, kubeconfig, "exec:")
	assert.NotContains(t, kubeconfig, "kubelogin")

	// Confirm the CA data round-trips as valid base64 (sanity on the
	// fixture itself, not just the template).
	_, err := base64.StdEncoding.DecodeString(fakeCAData)
	require.NoError(t, err)
}
