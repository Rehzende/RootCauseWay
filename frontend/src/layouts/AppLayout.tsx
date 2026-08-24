import { NavLink, Outlet } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import {
  LayoutDashboard, Box, Bot, Puzzle, Webhook, AlertTriangle,
  KeyRound, Settings, LogOut, BarChart3, BookOpen, Bell,
  Users, ShieldCheck, ScrollText, Store, Database, ShieldAlert,
  Brain, GitCommit, BellRing, Gauge, Archive,
} from 'lucide-react';
import { useAuth } from '@/hooks/useAuth';
import { useWebSocket } from '@/hooks/useWebSocket';
import api from '@/services/api';

interface NavItem {
  to: string;
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  badge?: 'quarantine';
}

interface NavCategory {
  title: string;
  items: NavItem[];
}

const navCategories: NavCategory[] = [
  {
    title: 'Operations',
    items: [
      { to: '/', icon: LayoutDashboard, label: 'Dashboard' },
      { to: '/incidents', icon: AlertTriangle, label: 'Incidents' },
      { to: '/quarantine', icon: ShieldAlert, label: 'Quarantine', badge: 'quarantine' },
      { to: '/analytics', icon: BarChart3, label: 'Analytics' },
    ],
  },
  {
    title: 'Catalog',
    items: [
      { to: '/software', icon: Box, label: 'Software' },
      { to: '/runbooks', icon: BookOpen, label: 'Runbooks' },
      { to: '/slo-dashboard', icon: Gauge, label: 'SLOs' },
    ],
  },
  {
    title: 'Intelligence',
    items: [
      { to: '/agents', icon: Bot, label: 'Agents' },
      { to: '/skills', icon: Puzzle, label: 'Skills' },
      { to: '/marketplace', icon: Store, label: 'Marketplace' },
      { to: '/knowledge-base', icon: Brain, label: 'Knowledge Base' },
    ],
  },
  {
    title: 'Integrations',
    items: [
      { to: '/webhooks', icon: Webhook, label: 'Webhooks' },
      { to: '/data-sources', icon: Database, label: 'Data Sources' },
      { to: '/notifications', icon: Bell, label: 'Notifications' },
      { to: '/notification-channels', icon: BellRing, label: 'Notif. Channels' },
      { to: '/change-events', icon: GitCommit, label: 'Change Events' },
    ],
  },
  {
    title: 'Administration',
    items: [
      { to: '/users', icon: Users, label: 'Users' },
      { to: '/roles', icon: ShieldCheck, label: 'Roles' },
      { to: '/credentials', icon: KeyRound, label: 'Credentials' },
      { to: '/audit-log', icon: ScrollText, label: 'Audit Log' },
      { to: '/retention', icon: Archive, label: 'Retention' },
      { to: '/settings', icon: Settings, label: 'Settings' },
    ],
  },
];

export function AppLayout() {
  const { user, logout } = useAuth();
  const { isConnected } = useWebSocket();

  const { data: quarantineCount } = useQuery({
    queryKey: ['quarantine-count'],
    queryFn: () =>
      api.get<{ total: number }>('/quarantine?resolved=false&per_page=1')
        .then((r: any) => r.data?.total ?? 0),
    refetchInterval: 30000,
  });

  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside className="flex w-64 flex-col bg-sidebar text-white">
        <div className="flex h-16 items-center gap-2 px-6">
          <AlertTriangle className="h-6 w-6 text-blue-400" />
          <span className="text-lg font-bold tracking-tight">RootCauseway</span>
        </div>

        <nav className="flex-1 overflow-y-auto px-3 py-2">
          {navCategories.map((category, idx) => (
            <div key={category.title} className={idx > 0 ? 'mt-4' : ''}>
              <p className="mb-1 px-3 text-[10px] font-semibold uppercase tracking-widest text-gray-500">
                {category.title}
              </p>
              <div className="space-y-0.5">
                {category.items.map(({ to, icon: Icon, label, badge }) => (
                  <NavLink
                    key={to}
                    to={to}
                    end={to === '/'}
                    className={({ isActive }) =>
                      `flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
                        isActive
                          ? 'bg-white/10 text-white'
                          : 'text-gray-400 hover:bg-white/5 hover:text-white'
                      }`
                    }
                  >
                    <Icon className="h-4.5 w-4.5" />
                    <span className="flex-1">{label}</span>
                    {badge === 'quarantine' && typeof quarantineCount === 'number' && quarantineCount > 0 && (
                      <span className="flex h-5 min-w-[20px] items-center justify-center rounded-full bg-amber-500 px-1.5 text-[10px] font-bold text-white">
                        {quarantineCount}
                      </span>
                    )}
                  </NavLink>
                ))}
              </div>
            </div>
          ))}
        </nav>

        <div className="border-t border-white/10 p-4">
          <div className="mb-2 flex items-center gap-2 px-1">
            <span className={`h-2 w-2 rounded-full ${isConnected ? 'bg-green-400' : 'bg-red-400 animate-pulse'}`} />
            <span className="text-[11px] text-gray-400">
              {isConnected ? 'Live' : 'Reconnecting...'}
            </span>
          </div>
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-blue-500 text-sm font-bold">
              {user?.name?.charAt(0)?.toUpperCase() ?? 'U'}
            </div>
            <div className="flex-1 truncate">
              <p className="truncate text-sm font-medium">{user?.name ?? 'User'}</p>
              <p className="truncate text-xs text-gray-400">{user?.email}</p>
            </div>
            <button onClick={logout} className="text-gray-400 hover:text-white" title="Logout">
              <LogOut className="h-4 w-4" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto bg-gray-50">
        <Outlet />
      </main>
    </div>
  );
}
