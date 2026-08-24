ALTER TABLE organizations
    DROP COLUMN IF EXISTS default_llm_provider_type,
    DROP COLUMN IF EXISTS default_llm_base_url,
    DROP COLUMN IF EXISTS default_llm_model,
    DROP COLUMN IF EXISTS default_llm_api_key_ref;
