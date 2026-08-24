-- Adds 'azure_key_vault' as a recognized credential_providers.provider_type
-- -- the first credential provider type with a real, live implementation
-- in ResolveCredentialData (services/azure_keyvault_provider.go) beyond
-- 'static'. Every other type in the original CHECK list (004) still
-- fails loudly with ErrCredentialProviderNotImplemented; this migration
-- only widens what's *storable*, it doesn't change resolution behavior
-- for those.
ALTER TABLE credential_providers
    DROP CONSTRAINT credential_providers_provider_type_check;

ALTER TABLE credential_providers
    ADD CONSTRAINT credential_providers_provider_type_check
    CHECK (provider_type IN ('hashicorp_vault', 'aws_sts', 'azure_managed_identity', 'gcp_workload_identity', 'azure_key_vault', 'static', 'custom'));
