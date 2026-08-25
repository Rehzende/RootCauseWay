-- Follow-up to 033: the user wants the reader (Viewer) role narrowed
-- further than "no admin resources" -- no catalog (software, runbooks,
-- slo) and no integrations (webhooks, observability/data-sources,
-- marketplace) either, just the incident-facing/reporting surface:
-- incidents, knowledge_base, analytics (agents/agents' health status is
-- reachable through incident detail already; Viewer doesn't need the
-- standalone Agents/Catalog/Integrations screens).
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE slug = 'viewer' AND is_system)
  AND permission_id IN (
    SELECT id FROM permissions
    WHERE resource IN ('agents', 'software', 'runbooks', 'slo', 'webhooks', 'observability', 'marketplace')
      AND action = 'read'
  );
