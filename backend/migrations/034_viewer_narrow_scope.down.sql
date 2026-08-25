INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.is_system AND r.slug = 'viewer'
  AND p.resource IN ('agents', 'software', 'runbooks', 'slo', 'webhooks', 'observability', 'marketplace')
  AND p.action = 'read'
ON CONFLICT DO NOTHING;
