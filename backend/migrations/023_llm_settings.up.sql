-- Org-level default LLM provider/model settings, managed via the LLM &
-- Tokens settings UI. Mirrors the pipeline_hitl_gate_enabled pattern
-- (migration 018): a handful of plain columns on organizations, not a
-- separate table, since it's one row of config per org.
--
-- Per-agent override deliberately reuses the existing a2a_agents.
-- managed_config JSONB column ("for managed: image, resources, replicas")
-- rather than adding new dedicated columns -- model/temperature are exactly
-- that kind of per-agent managed config, and managed_config already flows
-- untouched through Create/Update/Get, so no repository SQL changes needed
-- for the override half of this feature.
ALTER TABLE organizations
    ADD COLUMN default_llm_provider_type VARCHAR(20) NOT NULL DEFAULT 'lm_studio',
    ADD COLUMN default_llm_base_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN default_llm_model VARCHAR(200) NOT NULL DEFAULT '',
    ADD COLUMN default_llm_api_key_ref VARCHAR(200) NOT NULL DEFAULT '';
