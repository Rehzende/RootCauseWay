import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { AuthProvider, useAuth } from '@/hooks/useAuth';
import { AppLayout } from '@/layouts/AppLayout';
import { AuthLayout } from '@/layouts/AuthLayout';
import { ErrorBoundary } from '@/components/ErrorBoundary';
import { ToastProvider } from '@/components/Toast';
import { LoginPage } from '@/pages/LoginPage';
import { DashboardPage } from '@/pages/DashboardPage';
import { SoftwarePage } from '@/pages/SoftwarePage';
import { AgentsPage } from '@/pages/AgentsPage';
import { WebhooksPage } from '@/pages/WebhooksPage';
import { IncidentsPage } from '@/pages/IncidentsPage';
import { IncidentDetailPage } from '@/pages/IncidentDetailPage';
import { SettingsPage } from '@/pages/SettingsPage';
import { UsersPage } from '@/pages/UsersPage';
import { RolesPage } from '@/pages/RolesPage';
import { AuditLogPage } from '@/pages/AuditLogPage';
import { SkillsPage, SkillDetailPage } from '@/pages/SkillsPage';
import { CredentialsPage } from '@/pages/CredentialsPage';
import { AnalyticsPage } from '@/pages/AnalyticsPage';
import { RunbooksPage } from '@/pages/RunbooksPage';
import { RunbookDetailPage } from '@/pages/RunbookDetailPage';
import { NotificationsPage } from '@/pages/NotificationsPage';
import { OnboardingPage } from '@/pages/OnboardingPage';
import { QuarantinePage } from '@/pages/QuarantinePage';
import { MarketplacePage } from '@/pages/MarketplacePage';
import { MarketplaceDetailPage } from '@/pages/MarketplaceDetailPage';
import { DataSourcesPage } from '@/pages/DataSourcesPage';
import { KnowledgeBasePage } from '@/pages/KnowledgeBasePage';
import { NotificationChannelsPage } from '@/pages/NotificationChannelsPage';
import { ChangeEventsPage } from '@/pages/ChangeEventsPage';
import { SLODashboardPage } from '@/pages/SLODashboardPage';
import { RetentionPage } from '@/pages/RetentionPage';
import { AccessDeniedPage } from '@/components/PermissionGate';
import type { ReactNode } from 'react';

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
});

function AuthGuard({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function GuestGuard({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  if (isAuthenticated) return <Navigate to="/" replace />;
  return <>{children}</>;
}

// Blocks direct navigation to a route the user can't read at all --
// hiding the sidebar link (AppLayout) stops the click, this stops typing
// the URL. Every route below that maps to a resource in ROUTE_RESOURCE
// gets wrapped in this; Dashboard/onboarding are intentionally unwrapped.
function RequireResource({ resource, children }: { resource: string; children: ReactNode }) {
  const { hasPermission } = useAuth();
  if (!hasPermission(resource, 'read')) return <AccessDeniedPage />;
  return <>{children}</>;
}

export default function App() {
  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <BrowserRouter>
            <ToastProvider>
              <Routes>
                <Route element={<GuestGuard><AuthLayout /></GuestGuard>}>
                  <Route path="/login" element={<LoginPage />} />
                </Route>
                <Route element={<AuthGuard><AppLayout /></AuthGuard>}>
                  <Route path="/" element={<DashboardPage />} />
                  <Route path="/onboarding" element={<OnboardingPage />} />
                  <Route path="/quarantine" element={<RequireResource resource="incidents"><QuarantinePage /></RequireResource>} />
                  <Route path="/software" element={<RequireResource resource="software"><SoftwarePage /></RequireResource>} />
                  <Route path="/agents" element={<RequireResource resource="agents"><AgentsPage /></RequireResource>} />
                  <Route path="/skills" element={<RequireResource resource="skills"><SkillsPage /></RequireResource>} />
                  <Route path="/skills/:id" element={<RequireResource resource="skills"><SkillDetailPage /></RequireResource>} />
                  <Route path="/marketplace" element={<RequireResource resource="marketplace"><MarketplacePage /></RequireResource>} />
                  <Route path="/marketplace/:slug" element={<RequireResource resource="marketplace"><MarketplaceDetailPage /></RequireResource>} />
                  <Route path="/data-sources" element={<RequireResource resource="observability"><DataSourcesPage /></RequireResource>} />
                  <Route path="/webhooks" element={<RequireResource resource="webhooks"><WebhooksPage /></RequireResource>} />
                  <Route path="/incidents" element={<RequireResource resource="incidents"><IncidentsPage /></RequireResource>} />
                  <Route path="/incidents/:id" element={<RequireResource resource="incidents"><IncidentDetailPage /></RequireResource>} />
                  <Route path="/credentials" element={<RequireResource resource="credentials"><CredentialsPage /></RequireResource>} />
                  <Route path="/analytics" element={<RequireResource resource="analytics"><AnalyticsPage /></RequireResource>} />
                  <Route path="/runbooks" element={<RequireResource resource="runbooks"><RunbooksPage /></RequireResource>} />
                  <Route path="/runbooks/:id" element={<RequireResource resource="runbooks"><RunbookDetailPage /></RequireResource>} />
                  <Route path="/notifications" element={<RequireResource resource="notifications"><NotificationsPage /></RequireResource>} />
                  <Route path="/knowledge-base" element={<RequireResource resource="knowledge_base"><KnowledgeBasePage /></RequireResource>} />
                  <Route path="/notification-channels" element={<RequireResource resource="notifications"><NotificationChannelsPage /></RequireResource>} />
                  <Route path="/change-events" element={<RequireResource resource="software"><ChangeEventsPage /></RequireResource>} />
                  <Route path="/slo-dashboard" element={<RequireResource resource="slo"><SLODashboardPage /></RequireResource>} />
                  <Route path="/retention" element={<RequireResource resource="settings"><RetentionPage /></RequireResource>} />
                  <Route path="/users" element={<RequireResource resource="users"><UsersPage /></RequireResource>} />
                  <Route path="/roles" element={<RequireResource resource="roles"><RolesPage /></RequireResource>} />
                  <Route path="/audit-log" element={<RequireResource resource="audit"><AuditLogPage /></RequireResource>} />
                  <Route path="/settings" element={<RequireResource resource="settings"><SettingsPage /></RequireResource>} />
                </Route>
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </ToastProvider>
          </BrowserRouter>
        </AuthProvider>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}
