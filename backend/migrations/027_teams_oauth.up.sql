-- Switches the Teams (Graph API) integration from app-only client_credentials
-- auth (which requires a tenant admin to grant an Application Access Policy
-- via PowerShell -- no REST API or admin-center GUI exists for that specific
-- policy object, and that PowerShell step is a real adoption barrier for a
-- multi-tenant product) to a delegated OAuth authorization-code flow: a
-- dedicated service/bot Microsoft account connects once via a normal browser
-- consent screen, and RootCauseway creates meetings as that user (Graph /me/...)
-- using a stored, rotating refresh token instead of impersonating an
-- arbitrary organizer.
--
-- teams_organizer_user_id was previously a manually-typed field (the user
-- being impersonated by app-only auth); it becomes teams_connected_account_email,
-- read-only and auto-populated from Graph /me right after the OAuth connect
-- flow completes -- never user-typed again.
--
-- No production org has a working Teams integration yet (the Application
-- Access Policy was never actually granted in practice), so this is a clean
-- redesign, not a migration with a live fallback path.
ALTER TABLE organizations
    ADD COLUMN teams_refresh_token_encrypted TEXT NOT NULL DEFAULT '';

ALTER TABLE organizations
    RENAME COLUMN teams_organizer_user_id TO teams_connected_account_email;
