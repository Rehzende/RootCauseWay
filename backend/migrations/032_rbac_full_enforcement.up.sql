-- Real backend RBAC enforcement (RBACEnforcer.RequirePermission has existed,
-- fully implemented, since migration 010 -- never wired onto a single
-- route). Wiring it in now surfaces a real gap: the permission catalog
-- (also from 010) was never extended as the product grew past its original
-- 12 resources, so several real feature areas (role/permission management
-- itself, knowledge base, SLOs, observability sources, the agent
-- marketplace) have no permission to gate against at all. Add them here so
-- turning enforcement on doesn't lock even the Admin role out of its own
-- screens.
--
-- Every other route in the app folds into one of the original 12 resources
-- (see the mapping comment in cmd/api/main.go's route registration) --
-- these 5 are the only genuinely new resource areas.
INSERT INTO permissions (id, resource, action, description) VALUES
    (uuid_generate_v4(), 'roles', 'read', 'View roles and permissions'),
    (uuid_generate_v4(), 'roles', 'write', 'Manage roles, role-permission grants, and user-role assignments'),
    (uuid_generate_v4(), 'knowledge_base', 'read', 'View knowledge base entries'),
    (uuid_generate_v4(), 'knowledge_base', 'write', 'Manage knowledge base entries'),
    (uuid_generate_v4(), 'slo', 'read', 'View SLO definitions and status'),
    (uuid_generate_v4(), 'slo', 'write', 'Manage SLO definitions'),
    (uuid_generate_v4(), 'observability', 'read', 'View observability sources and snapshot configs'),
    (uuid_generate_v4(), 'observability', 'write', 'Manage observability sources and snapshot configs'),
    (uuid_generate_v4(), 'marketplace', 'read', 'Browse the agent marketplace'),
    (uuid_generate_v4(), 'marketplace', 'write', 'Install/uninstall marketplace agents')
ON CONFLICT (resource, action) DO NOTHING;

-- Grant the new permissions to every org's existing system roles, following
-- the exact same tier pattern the original 010 seed already established
-- (confirmed against live data, not guessed): Admin gets everything;
-- Operator gets read+write on operational/day-to-day resources but not
-- security-sensitive ones (roles is Admin-only, same tier as the existing
-- users/settings/audit -- none of which Operator holds either); Viewer
-- gets read-only on the same resources it already has, i.e. everything
-- except the Admin-only ones.
--
-- Scoped to orgs that already have these is_system roles (every org that's
-- actually using RBAC today) -- an org with no roles seeded yet gets
-- nothing here, same as it already has nothing for the original 12
-- resources.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.is_system AND r.slug = 'admin'
  AND p.resource IN ('roles', 'knowledge_base', 'slo', 'observability', 'marketplace')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.is_system AND r.slug = 'operator'
  AND p.resource IN ('knowledge_base', 'slo', 'observability', 'marketplace')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.is_system AND r.slug = 'viewer'
  AND p.resource IN ('knowledge_base', 'slo', 'observability', 'marketplace')
  AND p.action = 'read'
ON CONFLICT DO NOTHING;
