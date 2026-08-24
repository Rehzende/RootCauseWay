package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// aksAADServerScope is the AKS AAD Server application's well-known app ID,
// used as the token audience (with the v2 "/.default" suffix) when minting
// a token that AKS's API server will accept as a Bearer credential. This
// is the SAME value regardless of tenant/cluster -- it identifies "the AKS
// managed-cluster API surface" as a first-party Azure resource, not any
// customer-specific object.
//
// NOTE (from the original design writeup, project backlog): the Microsoft
// docs mix the legacy AAD-integration model with the newer Azure-RBAC-for-
// Kubernetes model this cluster actually uses (--enable-aad
// --enable-azure-rbac). This constant was the design's best-available
// answer at the time and MUST be confirmed against a real, running AKS
// cluster (a token minted with it either works or AKS returns 401/403) --
// that live check has not happened yet as of writing this file (the AKS
// cluster is stopped via stop.sh to save cost). Do that check before
// trusting this in a real incident investigation.
const aksAADServerScope = "6dae42f8-4368-4678-94ff-3960e28e3630/.default"

// azureAKSJITConfig is the shape of CredentialProvider.Config for
// provider_type "azure_aks_jit". Server/CertificateAuthorityData are
// static cluster connection metadata (not secrets, rarely change) --
// deliberately NOT fetched dynamically via the AKS management API on
// every lease, to avoid needing extra ARM permissions
// (Microsoft.ContainerService/managedClusters/listClusterUserCredential/
// action) beyond what minting the data-plane token itself requires. Only
// the Bearer token embedded in the resulting kubeconfig is minted fresh
// per lease -- that's the part that actually needs to be "just in time".
type azureAKSJITConfig struct {
	Server                   string `json:"server"`
	CertificateAuthorityData string `json:"certificate_authority_data"`
	TenantID                 string `json:"tenant_id"`
	ClientID                 string `json:"client_id"`
	ClientSecret             string `json:"client_secret"`
}

func (c azureAKSJITConfig) validate() error {
	missing := []string{}
	if c.Server == "" {
		missing = append(missing, "server")
	}
	if c.CertificateAuthorityData == "" {
		missing = append(missing, "certificate_authority_data")
	}
	if c.TenantID == "" {
		missing = append(missing, "tenant_id")
	}
	if c.ClientID == "" {
		missing = append(missing, "client_id")
	}
	if c.ClientSecret == "" {
		missing = append(missing, "client_secret")
	}
	if len(missing) > 0 {
		return fmt.Errorf("azure_aks_jit provider config missing required field(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

// aksToken is what aksTokenMinter.MintToken returns.
type aksToken struct {
	AccessToken string
	ExpiresOn   time.Time
}

// aksTokenMinter is the minimal Azure AD surface ResolveCredentialData
// needs. Narrow on purpose: lets tests fake a minted token without a real
// Azure tenant, real service principal, or network access.
type aksTokenMinter interface {
	MintToken(ctx context.Context) (aksToken, error)
}

// realAKSTokenMinter wraps the actual azidentity credential.
type realAKSTokenMinter struct {
	cred *azidentity.ClientSecretCredential
}

func (m *realAKSTokenMinter) MintToken(ctx context.Context) (aksToken, error) {
	tok, err := m.cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{aksAADServerScope}})
	if err != nil {
		return aksToken{}, fmt.Errorf("mint AKS-scoped token: %w", err)
	}
	return aksToken{AccessToken: tok.Token, ExpiresOn: tok.ExpiresOn}, nil
}

// newAKSTokenMinter builds the real Azure AD client for the given config.
// A package-level var (not a plain function) so tests can swap it for a
// fake without touching ResolveCredentialData's own signature -- same
// pattern as azure_keyvault_provider.go's newAzureSecretGetter.
var newAKSTokenMinter = func(cfg azureAKSJITConfig) (aksTokenMinter, error) {
	cred, err := azidentity.NewClientSecretCredential(cfg.TenantID, cfg.ClientID, cfg.ClientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("build azure client secret credential: %w", err)
	}
	return &realAKSTokenMinter{cred: cred}, nil
}

// buildEphemeralKubeconfig embeds a raw bearer token directly (no exec
// plugin, no kubelogin binary needed inside the agent container) -- the
// whole point of minting the token here instead of handing k8s-agent the
// service principal's own client_secret and making IT do the
// client_credentials grant on every kubectl invocation.
func buildEphemeralKubeconfig(server, caData, token string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: aks
  cluster:
    server: %s
    certificate-authority-data: %s
contexts:
- name: aks
  context:
    cluster: aks
    user: aks-jit-user
current-context: aks
users:
- name: aks-jit-user
  user:
    token: %s
`, server, caData, token)
}

// resolveAzureAKSJITCredential mints a fresh, short-lived Azure AD token
// scoped to the AKS API server and wraps it as a minimal kubeconfig.
// credentialPath is unused (there's no equivalent of Key Vault's "secret
// name" for this provider type -- one provider config names exactly one
// cluster) but kept in the signature for consistency with the other
// resolve* functions and ResolveCredentialData's dispatch.
func resolveAzureAKSJITCredential(ctx context.Context, rawConfig json.RawMessage, credentialPath string) (map[string]interface{}, error) {
	var cfg azureAKSJITConfig
	if err := json.Unmarshal(rawConfig, &cfg); err != nil {
		return nil, fmt.Errorf("invalid azure_aks_jit provider config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if _, err := base64.StdEncoding.DecodeString(cfg.CertificateAuthorityData); err != nil {
		return nil, fmt.Errorf("azure_aks_jit provider config: certificate_authority_data is not valid base64: %w", err)
	}

	minter, err := newAKSTokenMinter(cfg)
	if err != nil {
		return nil, err
	}
	tok, err := minter.MintToken(ctx)
	if err != nil {
		return nil, err
	}

	kubeconfig := buildEphemeralKubeconfig(cfg.Server, cfg.CertificateAuthorityData, tok.AccessToken)

	// "provider" is not set here -- credentialEnvelope (skills_service.go)
	// already sets it for every provider type, this only adds the
	// AKS-specific fields on top. token_expires_at is separate from the
	// envelope's own "expires_at" (which reflects the lease's requested
	// TTL, not necessarily the real token's lifetime) -- see
	// ResolveCredentialData's azure_aks_jit case, which caps the lease's
	// expiry to whichever of the two is sooner: a lease must never claim
	// validity the Azure token itself doesn't actually have.
	return map[string]interface{}{
		"kubeconfig":       kubeconfig,
		"token_expires_at": tok.ExpiresOn.Unix(),
	}, nil
}
