ALTER TABLE organizations
    RENAME COLUMN teams_connected_account_email TO teams_organizer_user_id;

ALTER TABLE organizations
    DROP COLUMN IF EXISTS teams_refresh_token_encrypted;
