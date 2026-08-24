ALTER TABLE organizations
    DROP COLUMN IF EXISTS teams_tenant_id,
    DROP COLUMN IF EXISTS teams_client_id,
    DROP COLUMN IF EXISTS teams_client_secret_encrypted,
    DROP COLUMN IF EXISTS teams_organizer_user_id;
