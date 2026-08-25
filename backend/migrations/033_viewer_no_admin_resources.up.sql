-- The Viewer role has always granted users:read (see the roleDef in
-- handlers.SeedDefaultRoles), which is an administration resource --
-- unlike everything else Viewer holds (incidents, knowledge_base,
-- software, runbooks, agents, skills, webhooks, marketplace,
-- observability, slo, analytics), which are all operational/reporting
-- resources a read-only user legitimately needs. A reader should see the
-- product's data, not the org's user directory.
--
-- Revoke it from every org's existing Viewer role.
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE slug = 'viewer' AND is_system)
  AND permission_id IN (SELECT id FROM permissions WHERE resource = 'users' AND action = 'read');
