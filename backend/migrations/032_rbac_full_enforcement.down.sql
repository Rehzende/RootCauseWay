DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE resource IN ('roles', 'knowledge_base', 'slo', 'observability', 'marketplace')
);
DELETE FROM permissions WHERE resource IN ('roles', 'knowledge_base', 'slo', 'observability', 'marketplace');
