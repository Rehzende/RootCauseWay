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
                  <Route path="/quarantine" element={<QuarantinePage />} />
                  <Route path="/software" element={<SoftwarePage />} />
                  <Route path="/agents" element={<AgentsPage />} />
                  <Route path="/skills" element={<SkillsPage />} />
                  <Route path="/skills/:id" element={<SkillDetailPage />} />
                  <Route path="/marketplace" element={<MarketplacePage />} />
                  <Route path="/marketplace/:slug" element={<MarketplaceDetailPage />} />
                  <Route path="/data-sources" element={<DataSourcesPage />} />
                  <Route path="/webhooks" element={<WebhooksPage />} />
                  <Route path="/incidents" element={<IncidentsPage />} />
                  <Route path="/incidents/:id" element={<IncidentDetailPage />} />
                  <Route path="/credentials" element={<CredentialsPage />} />
                  <Route path="/analytics" element={<AnalyticsPage />} />
                  <Route path="/runbooks" element={<RunbooksPage />} />
                  <Route path="/runbooks/:id" element={<RunbookDetailPage />} />
                  <Route path="/notifications" element={<NotificationsPage />} />
                  <Route path="/knowledge-base" element={<KnowledgeBasePage />} />
                  <Route path="/notification-channels" element={<NotificationChannelsPage />} />
                  <Route path="/change-events" element={<ChangeEventsPage />} />
                  <Route path="/slo-dashboard" element={<SLODashboardPage />} />
                  <Route path="/retention" element={<RetentionPage />} />
                  <Route path="/users" element={<UsersPage />} />
                  <Route path="/roles" element={<RolesPage />} />
                  <Route path="/audit-log" element={<AuditLogPage />} />
                  <Route path="/settings" element={<SettingsPage />} />
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
