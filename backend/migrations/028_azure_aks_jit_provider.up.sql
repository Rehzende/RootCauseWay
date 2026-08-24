-- Adds 'azure_aks_jit' as a recognized credential_providers.provider_type
-- -- the JIT AKS credential design documented in the project backlog:
-- a dynamic Azure AD token minted fresh at lease-request time
-- (audience = AKS AAD Server), wrapped as an ephemeral kubeconfig, NOT a
-- static kubeconfig stored and merely "loaned out" with a decorative TTL
-- (that would still be permanent access underneath). See
-- services/azure_aks_jit_provider.go for the implementation.
ALTER TABLE credential_providers
    DROP CONSTRAINT credential_providers_provider_type_check;

ALTER TABLE credential_providers
    ADD CONSTRAINT credential_providers_provider_type_check
    CHECK (provider_type IN ('hashicorp_vault', 'aws_sts', 'azure_managed_identity', 'gcp_workload_identity', 'azure_key_vault', 'azure_aks_jit', 'static', 'custom'));
