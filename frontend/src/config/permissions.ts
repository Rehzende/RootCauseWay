// Single source of truth for "which permission does this route need to be
// visible/reachable at all" -- mirrors the resource each path's GET routes
// are gated on in backend/cmd/api/main.go. Used by AppLayout to hide nav
// links a user can't act on and by App.tsx to block direct navigation to
// the same routes (a hidden nav link alone doesn't stop someone from
// typing the URL). Dashboard/onboarding/login are intentionally absent --
// every authenticated user lands there regardless of role.
export const ROUTE_RESOURCE: Record<string, string> = {
  '/incidents': 'incidents',
  '/quarantine': 'incidents',
  '/analytics': 'analytics',
  '/software': 'software',
  '/change-events': 'software',
  '/runbooks': 'runbooks',
  '/slo-dashboard': 'slo',
  '/agents': 'agents',
  '/skills': 'skills',
  '/marketplace': 'marketplace',
  '/knowledge-base': 'knowledge_base',
  '/webhooks': 'webhooks',
  '/data-sources': 'observability',
  '/notifications': 'notifications',
  '/notification-channels': 'notifications',
  '/users': 'users',
  '/roles': 'roles',
  '/credentials': 'credentials',
  '/audit-log': 'audit',
  '/retention': 'settings',
  '/settings': 'settings',
};
