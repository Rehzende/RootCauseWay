-- The a2a_agents repository queries `is_system` (see internal/database/a2a_repositories.go),
-- but no migration ever added that column to a2a_agents — only to `roles` (010_auth_rbac).
-- GET /api/v1/a2a/agents was 500ing with "column is_system does not exist".
ALTER TABLE a2a_agents ADD COLUMN IF NOT EXISTS is_system BOOLEAN DEFAULT false;
