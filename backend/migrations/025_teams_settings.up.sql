-- Org-level Microsoft Teams (Graph API) integration settings, managed via
-- the Integrations settings UI -- replaces the TEAMS_TENANT_ID/
-- TEAMS_CLIENT_ID/TEAMS_CLIENT_SECRET/TEAMS_ORGANIZER_USER_ID env vars
-- that teams.NewClientFromEnv() previously required (a single client for
-- the whole deployment, one Azure tenant only, a full backend redeploy to
-- change). Same "plain columns on organizations" pattern as
-- pipeline_hitl_gate_enabled (018) and the LLM settings (023): one row of
-- config per org, no separate table needed.
--
-- teams_client_secret_encrypted is stored encrypted (crypto.Cipher,
-- AES-256-GCM -- the same cipher already used for skill credential JSON
-- blobs in skills_repositories.go), since a Graph app-only client secret
-- grants full tenant Teams access, unlike default_llm_api_key_ref (023),
-- which this migration deliberately does not touch or retrofit.
ALTER TABLE organizations
    ADD COLUMN teams_tenant_id VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN teams_client_id VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN teams_client_secret_encrypted TEXT NOT NULL DEFAULT '',
    ADD COLUMN teams_organizer_user_id VARCHAR(200) NOT NULL DEFAULT '';
