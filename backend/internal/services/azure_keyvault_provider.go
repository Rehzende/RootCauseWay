package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

// azureKeyVaultConfig is the shape of CredentialProvider.Config for
// provider_type "azure_key_vault". Same tenant/client/secret shape as the
// Teams integration settings (see TeamsSettings) -- service-principal
// (client credentials) auth, since this backend runs outside Azure (a
// homelab k3s cluster), so Managed Identity/IMDS auth isn't available.
type azureKeyVaultConfig struct {
	VaultURL     string `json:"vault_url"`
	TenantID     string `json:"tenant_id"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func (c azureKeyVaultConfig) validate() error {
	missing := []string{}
	if c.VaultURL == "" {
		missing = append(missing, "vault_url")
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
		return fmt.Errorf("azure_key_vault provider config missing required field(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

// azureSecret is what azureSecretGetter.GetSecret returns -- just the
// fields ResolveCredentialData actually surfaces to the caller.
type azureSecret struct {
	Value   string
	Version string
}

// azureSecretGetter is the minimal Key Vault surface ResolveCredentialData
// needs. Narrow on purpose: lets tests fake a vault response without a
// real Azure tenant, real service principal, or network access -- none of
// which this test environment has.
type azureSecretGetter interface {
	GetSecret(ctx context.Context, name, version string) (azureSecret, error)
}

// realAzureSecretGetter wraps the actual azsecrets SDK client.
type realAzureSecretGetter struct {
	client *azsecrets.Client
}

func (g *realAzureSecretGetter) GetSecret(ctx context.Context, name, version string) (azureSecret, error) {
	resp, err := g.client.GetSecret(ctx, name, version, nil)
	if err != nil {
		return azureSecret{}, fmt.Errorf("get secret %q from key vault: %w", name, err)
	}
	if resp.Value == nil {
		return azureSecret{}, fmt.Errorf("key vault returned no value for secret %q", name)
	}
	v := ""
	if resp.ID != nil {
		v = resp.ID.Version()
	}
	return azureSecret{Value: *resp.Value, Version: v}, nil
}

// newAzureSecretGetter builds the real Key Vault client for the given
// config. A package-level var (not a plain function) so tests can swap
// it for a fake without touching ResolveCredentialData's own signature.
var newAzureSecretGetter = func(cfg azureKeyVaultConfig) (azureSecretGetter, error) {
	cred, err := azidentity.NewClientSecretCredential(cfg.TenantID, cfg.ClientID, cfg.ClientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("build azure client secret credential: %w", err)
	}
	client, err := azsecrets.NewClient(cfg.VaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("build key vault client: %w", err)
	}
	return &realAzureSecretGetter{client: client}, nil
}

// resolveAzureKeyVaultCredential fetches a secret from Azure Key Vault.
// credentialPath is the secret name, optionally suffixed "/<version>" to
// pin a specific version (Key Vault's own convention); an empty version
// fetches the current/latest one.
func resolveAzureKeyVaultCredential(ctx context.Context, rawConfig json.RawMessage, credentialPath string) (map[string]interface{}, error) {
	var cfg azureKeyVaultConfig
	if err := json.Unmarshal(rawConfig, &cfg); err != nil {
		return nil, fmt.Errorf("invalid azure_key_vault provider config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	name, version, _ := strings.Cut(credentialPath, "/")

	getter, err := newAzureSecretGetter(cfg)
	if err != nil {
		return nil, err
	}
	secret, err := getter.GetSecret(ctx, name, version)
	if err != nil {
		return nil, err
	}

	// "provider" is not set here -- credentialEnvelope (skills_service.go)
	// already sets it for every provider type, this only adds the
	// secret-specific fields on top.
	return map[string]interface{}{
		"value":          secret.Value,
		"secret_name":    name,
		"secret_version": secret.Version,
	}, nil
}
