ALTER TABLE credential_providers
    DROP CONSTRAINT credential_providers_provider_type_check;

ALTER TABLE credential_providers
    ADD CONSTRAINT credential_providers_provider_type_check
    CHECK (provider_type IN ('hashicorp_vault', 'aws_sts', 'azure_managed_identity', 'gcp_workload_identity', 'static', 'custom'));
