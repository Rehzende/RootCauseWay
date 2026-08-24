-- Lets the org's default LLM API key be resolved through the credential
-- vault (RequestLease/resolveCredentialData, see CredentialLeaseService)
-- instead of being stored and used as a literal secret value.
--
-- When NULL (default, and the state of every existing org): unchanged
-- behavior -- default_llm_api_key_ref is used as the literal key, exactly
-- as before this migration. When set: default_llm_api_key_ref is
-- reinterpreted as a credential_path passed to this provider, and the
-- resolved secret is used instead. ON DELETE SET NULL rather than
-- RESTRICT/CASCADE -- deleting the referenced credential_providers row
-- should fall back to the literal-ref behavior, not block the delete or
-- silently orphan the org's LLM config.
ALTER TABLE organizations
    ADD COLUMN default_llm_credential_provider_id UUID REFERENCES credential_providers(id) ON DELETE SET NULL;
