import axios from 'axios';
import type {
  ObservabilitySource,
  SnapshotConfig,
  LoginRequest,
  LoginResponse,
  SoftwareEntry,
  CreateSoftwareRequest,
  Agent,
  CreateAgentRequest,
  Webhook,
  CreateWebhookRequest,
  Incident,
  UpdateIncidentRequest,
  IncidentEvent,
  CreateIncidentEventRequest,
  IncidentEvidence,
  CreateIncidentEvidenceRequest,
  PaginatedResponse,
  IncidentFull,
  AgentRun,
  IncidentRCI,
  IncidentRCA,
  IncidentPostmortem,
  A2AAgent,
  CreateA2AAgentRequest,
  A2ATask,
  OrchestratorDecision,
  Skill,
  CreateSkillRequest,
  AgentSkillLink,
  CredentialProvider,
  CreateCredentialProviderRequest,
  ResourceCredential,
  CreateResourceCredentialRequest,
  AccessPolicy,
  CreateAccessPolicyRequest,
  CredentialLease,
  IncidentFeedback,
  KnowledgeBaseEntry,
  SimilarIncident,
  CorrelationRule,
  AlertGroup,
  NotificationChannel,
  EscalationPolicy,
  NotificationLogEntry,
  Runbook,
  RunbookStep,
  RunbookExecution,
  ChangeEvent,
  AnalyticsMTTR,
  AnalyticsIncidentTrend,
  AnalyticsAgentEffectiveness,
  AnalyticsCostByModel,
  AnalyticsCostByIncident,
  UserWithRoles,
  RoleWithPermissions,
  Permission,
  SSOProvider,
  CreateSSOProviderRequest,
  APIKey,
  APIKeyWithSecret,
  AuditLogEntry,
  MarketplaceAgent,
  InstalledAgent,
  WarRoomMeeting,
  ApproveStageResponse,
  OrgSettingsResponse,
  SLODefinition,
  CreateSLODefinitionRequest,
  UpdateSLODefinitionRequest,
  SLOStatus,
  SoftwareSLOStatus,
  RetentionPolicy,
  CreateRetentionPolicyRequest,
  UpdateRetentionPolicyRequest,
  RetentionSweepSummary,
} from '@/types/api';

const api = axios.create({
  baseURL: '/api/v1',
  headers: { 'Content-Type': 'application/json' },
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('rootcauseway_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (res) => res,
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('rootcauseway_token');
      window.location.href = '/login';
    }
    return Promise.reject(error);
  }
);

export default api;

// --- Auth ---
export const login = (data: LoginRequest) =>
  api.post<LoginResponse>('/auth/login', data).then((r) => r.data);

// --- Software ---
export const listSoftware = (page = 1, perPage = 20) =>
  api.get<PaginatedResponse<SoftwareEntry>>('/software', { params: { page, per_page: perPage } }).then((r) => r.data);

export const createSoftware = (data: CreateSoftwareRequest) =>
  api.post<SoftwareEntry>('/software', data).then((r) => r.data);

export const getSoftware = (id: string) =>
  api.get<SoftwareEntry>(`/software/${id}`).then((r) => r.data);

export const updateSoftware = (id: string, data: CreateSoftwareRequest) =>
  api.put<SoftwareEntry>(`/software/${id}`, data).then((r) => r.data);

export const deleteSoftware = (id: string) =>
  api.delete(`/software/${id}`);

// --- Agents ---
export const listAgents = (params?: { type?: string; page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<Agent>>('/agents', { params }).then((r) => r.data);

export const createAgent = (data: CreateAgentRequest) =>
  api.post<Agent>('/agents', data).then((r) => r.data);

export const getAgent = (id: string) =>
  api.get<Agent>(`/agents/${id}`).then((r) => r.data);

export const updateAgent = (id: string, data: CreateAgentRequest) =>
  api.put<Agent>(`/agents/${id}`, data).then((r) => r.data);

export const deleteAgent = (id: string) =>
  api.delete(`/agents/${id}`);

// --- Webhooks ---
export const listWebhooks = () =>
  api.get<PaginatedResponse<Webhook>>('/webhooks').then((r) => r.data);

export const createWebhook = (data: CreateWebhookRequest) =>
  api.post<Webhook>('/webhooks', data).then((r) => r.data);

export const getWebhook = (id: string) =>
  api.get<Webhook>(`/webhooks/${id}`).then((r) => r.data);

export const deleteWebhook = (id: string) =>
  api.delete(`/webhooks/${id}`);

// --- Incidents ---
export const listIncidents = (params?: { status?: string; severity?: string; software_id?: string; from?: string; page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<Incident>>('/incidents', { params }).then((r) => r.data);

export const getIncident = (id: string) =>
  api.get<Incident>(`/incidents/${id}`).then((r) => r.data);

export const updateIncident = (id: string, data: UpdateIncidentRequest) =>
  api.patch<Incident>(`/incidents/${id}`, data).then((r) => r.data);

export const addIncidentEvent = (id: string, data: CreateIncidentEventRequest) =>
  api.post<IncidentEvent>(`/incidents/${id}/events`, data).then((r) => r.data);

export const addIncidentEvidence = (id: string, data: CreateIncidentEvidenceRequest) =>
  api.post<IncidentEvidence>(`/incidents/${id}/evidence`, data).then((r) => r.data);

// --- Incident Full ---
export const getIncidentFull = (id: string) =>
  api.get<IncidentFull>(`/incidents/${id}/full`).then((r) => r.data);

export const getIncidentDAG = (id: string) =>
  api.get<AgentRun[]>(`/incidents/${id}/dag`).then((r) => r.data);

// --- RCI ---
export const getRCI = (incidentId: string) =>
  api.get<IncidentRCI>(`/incidents/${incidentId}/rci`).then((r) => r.data);

export const updateRCI = (incidentId: string, data: Partial<IncidentRCI>) =>
  api.patch<IncidentRCI>(`/incidents/${incidentId}/rci`, data).then((r) => r.data);

// --- RCA ---
export const getRCA = (incidentId: string) =>
  api.get<IncidentRCA>(`/incidents/${incidentId}/rca`).then((r) => r.data);

export const updateRCA = (incidentId: string, data: Partial<IncidentRCA>) =>
  api.patch<IncidentRCA>(`/incidents/${incidentId}/rca`, data).then((r) => r.data);

// --- Postmortem ---
export const getPostmortem = (incidentId: string) =>
  api.get<IncidentPostmortem>(`/incidents/${incidentId}/postmortem`).then((r) => r.data);

export const updatePostmortem = (incidentId: string, data: Partial<IncidentPostmortem>) =>
  api.patch<IncidentPostmortem>(`/incidents/${incidentId}/postmortem`, data).then((r) => r.data);

// --- A2A Agents ---
export const listA2AAgents = (params?: { type?: string; page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<A2AAgent>>('/a2a/agents', { params }).then((r) => r.data);

export const createA2AAgent = (data: CreateA2AAgentRequest) =>
  api.post<A2AAgent>('/a2a/agents', data).then((r) => r.data);

export const getA2AAgent = (id: string) =>
  api.get<A2AAgent>(`/a2a/agents/${id}`).then((r) => r.data);

export const updateA2AAgent = (id: string, data: Partial<CreateA2AAgentRequest> & { enabled?: boolean }) =>
  api.put<A2AAgent>(`/a2a/agents/${id}`, data).then((r) => r.data);

export const deleteA2AAgent = (id: string) =>
  api.delete(`/a2a/agents/${id}`);

// --- A2A Tasks ---
export const listA2ATasks = (incidentId: string) =>
  api.get<A2ATask[]>(`/incidents/${incidentId}/a2a/tasks`).then((r) => r.data);

export const getA2ATask = (taskId: string) =>
  api.get<A2ATask>(`/a2a/tasks/${taskId}`).then((r) => r.data);

// --- Orchestrator Decisions ---
export const listOrchestratorDecisions = (incidentId: string) =>
  api.get<OrchestratorDecision[]>(`/incidents/${incidentId}/orchestrator/decisions`).then((r) => r.data);

// --- Skills Registry ---
export const listSkills = (params?: { category?: string; page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<Skill>>('/skills', { params }).then((r) => r.data);

export const createSkill = (data: CreateSkillRequest) =>
  api.post<Skill>('/skills', data).then((r) => r.data);

export const getSkill = (id: string) =>
  api.get<Skill>(`/skills/${id}`).then((r) => r.data);

export const updateSkill = (id: string, data: Partial<CreateSkillRequest> & { enabled?: boolean }) =>
  api.put<Skill>(`/skills/${id}`, data).then((r) => r.data);

export const deleteSkill = (id: string) =>
  api.delete(`/skills/${id}`);

export const linkSkillToAgent = (agentId: string, skillId: string) =>
  api.post<AgentSkillLink>(`/a2a/agents/${agentId}/skills`, { skill_id: skillId }).then((r) => r.data);

export const unlinkSkillFromAgent = (agentId: string, skillId: string) =>
  api.delete(`/a2a/agents/${agentId}/skills/${skillId}`);

export const listAgentSkills = (agentId: string) =>
  api.get<AgentSkillLink[]>(`/a2a/agents/${agentId}/skills`).then((r) => r.data);

// --- Credential Providers ---
export const listCredentialProviders = () =>
  api.get<PaginatedResponse<CredentialProvider>>('/credentials/providers').then((r) => r.data);

export const createCredentialProvider = (data: CreateCredentialProviderRequest) =>
  api.post<CredentialProvider>('/credentials/providers', data).then((r) => r.data);

export const deleteCredentialProvider = (id: string) =>
  api.delete(`/credentials/providers/${id}`);

// --- Resource Credentials ---
export const listResourceCredentials = (softwareId: string) =>
  api.get<ResourceCredential[]>(`/software/${softwareId}/credentials`).then((r) => r.data);

export const createResourceCredential = (data: CreateResourceCredentialRequest) =>
  api.post<ResourceCredential>('/credentials/resources', data).then((r) => r.data);

export const deleteResourceCredential = (id: string) =>
  api.delete(`/credentials/resources/${id}`);

// --- Access Policies ---
export const listAccessPolicies = () =>
  api.get<PaginatedResponse<AccessPolicy>>('/access-policies').then((r) => r.data);

export const createAccessPolicy = (data: CreateAccessPolicyRequest) =>
  api.post<AccessPolicy>('/access-policies', data).then((r) => r.data);

export const deleteAccessPolicy = (id: string) =>
  api.delete(`/access-policies/${id}`);

// --- Credential Leases ---
export const listCredentialLeases = (params?: { status?: string; page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<CredentialLease>>('/credentials/leases', { params }).then((r) => r.data);

export const listActiveLeases = () =>
  api.get<PaginatedResponse<CredentialLease>>('/credentials/leases', { params: { status: 'active' } }).then((r) => r.data);

export const revokeCredentialLease = (leaseId: string) =>
  api.post(`/credentials/leases/${leaseId}/revoke`).then((r) => r.data);

// --- Feedback ---
export const submitFeedback = (incidentId: string, data: { target_type: string; rating: 'positive' | 'negative' | 'neutral'; correction?: string }) =>
  api.post<IncidentFeedback>(`/incidents/${incidentId}/feedback`, data).then((r) => r.data);

export const listFeedback = (incidentId: string) =>
  api.get<IncidentFeedback[]>(`/incidents/${incidentId}/feedback`).then((r) => r.data);

// --- Knowledge Base ---
// GET /knowledge-base only ever filtered by `category` -- the free-text
// `query` the KnowledgeBasePage search box sends was silently ignored.
// /knowledge-base/search is the handler that actually reads `query`
// (SearchKnowledgeBase, already dual GET/POST for agent-service's internal
// use). Also a bare array, not PaginatedResponse -- the backend never
// wrapped it in {data,total,...}, so `.data` on the result used to always
// be undefined regardless of what the backend actually returned.
export const searchKnowledgeBase = (params?: { query?: string; category?: string; page?: number; per_page?: number }) =>
  api.get<KnowledgeBaseEntry[]>('/knowledge-base/search', { params }).then((r) => r.data);

export const getKnowledgeBaseEntry = (id: string) =>
  api.get<KnowledgeBaseEntry>(`/knowledge-base/${id}`).then((r) => r.data);

// --- Similar Incidents ---
export const listSimilarIncidents = (incidentId: string) =>
  api.get<SimilarIncident[]>(`/incidents/${incidentId}/similar`).then((r) => r.data);

// --- Correlation Rules ---
export const listCorrelationRules = () =>
  api.get<PaginatedResponse<CorrelationRule>>('/correlation-rules').then((r) => r.data);

export const createCorrelationRule = (data: Partial<CorrelationRule>) =>
  api.post<CorrelationRule>('/correlation-rules', data).then((r) => r.data);

export const updateCorrelationRule = (id: string, data: Partial<CorrelationRule>) =>
  api.put<CorrelationRule>(`/correlation-rules/${id}`, data).then((r) => r.data);

export const deleteCorrelationRule = (id: string) =>
  api.delete(`/correlation-rules/${id}`);

// --- Alert Groups ---
export const listAlertGroups = (incidentId: string) =>
  api.get<AlertGroup[]>(`/incidents/${incidentId}/alert-groups`).then((r) => r.data);

// --- Notification Channels ---
export const listNotificationChannels = () =>
  api.get<PaginatedResponse<NotificationChannel>>('/notification-channels').then((r) => r.data);

export const createNotificationChannel = (data: Partial<NotificationChannel>) =>
  api.post<NotificationChannel>('/notification-channels', data).then((r) => r.data);

export const updateNotificationChannel = (id: string, data: Partial<NotificationChannel>) =>
  api.put<NotificationChannel>(`/notification-channels/${id}`, data).then((r) => r.data);

export const deleteNotificationChannel = (id: string) =>
  api.delete(`/notification-channels/${id}`);

// --- Escalation Policies ---
export const listEscalationPolicies = () =>
  api.get<PaginatedResponse<EscalationPolicy>>('/escalation-policies').then((r) => r.data);

export const createEscalationPolicy = (data: Partial<EscalationPolicy>) =>
  api.post<EscalationPolicy>('/escalation-policies', data).then((r) => r.data);

export const updateEscalationPolicy = (id: string, data: Partial<EscalationPolicy>) =>
  api.put<EscalationPolicy>(`/escalation-policies/${id}`, data).then((r) => r.data);

export const deleteEscalationPolicy = (id: string) =>
  api.delete(`/escalation-policies/${id}`);

// --- Notification Log ---
export const listNotificationLogs = (params?: { incident_id?: string; page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<NotificationLogEntry>>('/notifications/logs', { params }).then((r) => r.data);

// --- Runbooks ---
// ListRunbooks (Go) returns a bare array (c.JSON(200, items)), never
// {data,total,...} -- the PaginatedResponse<Runbook> type here was a lie
// the type system couldn't catch, so RunbooksPage's `runbooksData?.data`
// was always undefined regardless of what the backend returned. Found
// live validating the Skills registry fix (same bug class, same fix, as
// searchKnowledgeBase's /knowledge-base -> /knowledge-base/search above).
export const listRunbooks = (params?: { software_id?: string; page?: number; per_page?: number }) =>
  api.get<Runbook[]>('/runbooks', { params }).then((r) => r.data);

export const getRunbook = (id: string) =>
  api.get<Runbook>(`/runbooks/${id}`).then((r) => r.data);

export const createRunbook = (data: Partial<Runbook>) =>
  api.post<Runbook>('/runbooks', data).then((r) => r.data);

export const updateRunbook = (id: string, data: Partial<Runbook>) =>
  api.put<Runbook>(`/runbooks/${id}`, data).then((r) => r.data);

export const deleteRunbook = (id: string) =>
  api.delete(`/runbooks/${id}`);

// --- Runbook Steps ---
export const listRunbookSteps = (runbookId: string) =>
  api.get<RunbookStep[]>(`/runbooks/${runbookId}/steps`).then((r) => r.data);

export const createRunbookStep = (runbookId: string, data: Partial<RunbookStep>) =>
  api.post<RunbookStep>(`/runbooks/${runbookId}/steps`, data).then((r) => r.data);

export const updateRunbookStep = (runbookId: string, stepId: string, data: Partial<RunbookStep>) =>
  api.put<RunbookStep>(`/runbooks/${runbookId}/steps/${stepId}`, data).then((r) => r.data);

export const deleteRunbookStep = (runbookId: string, stepId: string) =>
  api.delete(`/runbooks/${runbookId}/steps/${stepId}`);

export const reorderRunbookSteps = (runbookId: string, stepIds: string[]) =>
  api.post(`/runbooks/${runbookId}/steps/reorder`, { step_ids: stepIds }).then((r) => r.data);

// --- Runbook Executions ---
export const executeRunbook = (runbookId: string, incidentId?: string) =>
  api.post<RunbookExecution>(`/runbooks/${runbookId}/execute`, { incident_id: incidentId }).then((r) => r.data);

export const getRunbookExecution = (executionId: string) =>
  api.get<RunbookExecution>(`/runbook-executions/${executionId}`).then((r) => r.data);

export const listRunbookExecutions = (runbookId: string) =>
  api.get<RunbookExecution[]>(`/runbooks/${runbookId}/executions`).then((r) => r.data);

export const completeExecutionStep = (execId: string, stepId: string) =>
  api.post<RunbookExecution>(`/runbook-executions/${execId}/steps/${stepId}/complete`).then((r) => r.data);

// --- A2A Agent Health ---
export const healthCheckA2AAgent = (agentId: string) =>
  api.post<A2AAgent>(`/a2a/agents/${agentId}/health-check`).then((r) => r.data);

export const healthCheckAllA2AAgents = () =>
  api.post<A2AAgent[]>('/a2a/agents/health-check-all').then((r) => r.data);

// --- Change Events ---
export const listChangeEvents = (params?: { software_id?: string; since?: string; until?: string; page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<ChangeEvent>>('/change-events', { params }).then((r) => r.data);

export const getChangeEvent = (id: string) =>
  api.get<ChangeEvent>(`/change-events/${id}`).then((r) => r.data);

// --- Analytics ---
export const getAnalyticsMTTR = (params?: { since?: string; until?: string }) =>
  api.get<AnalyticsMTTR[]>('/analytics/mttr', { params }).then((r) => r.data);

export const getAnalyticsIncidentTrends = (params?: { since?: string; until?: string }) =>
  api.get<AnalyticsIncidentTrend[]>('/analytics/trends', { params }).then((r) => r.data);

export const getAnalyticsAgentEffectiveness = () =>
  api.get<AnalyticsAgentEffectiveness[]>('/analytics/agent-effectiveness').then((r) => r.data);

export const getCostByModel = () =>
  api.get<AnalyticsCostByModel[]>('/analytics/cost-by-model').then((r) => r.data);

export const getCostByIncident = () =>
  api.get<AnalyticsCostByIncident[]>('/analytics/cost-by-incident').then((r) => r.data);

// --- Auth (extended) ---
export const getCurrentUser = () =>
  api.get<{ user: UserWithRoles; permissions: Record<string, string[]> }>('/auth/me').then((r) => ({
    ...r.data.user,
    permissions: r.data.permissions ?? {},
    roles: r.data.user.roles ?? [],
  } as UserWithRoles));

export const ssoLogin = (provider: string) => {
  window.location.href = `/api/v1/auth/sso/${provider}/login`;
};

// --- Users ---
export const listUsers = (params?: { page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<UserWithRoles>>('/users', { params }).then((r) => r.data);

export const createUser = (data: { name: string; email: string; password: string; role_ids?: string[] }) =>
  api.post<UserWithRoles>('/users', data).then((r) => r.data);

export const getUser = (id: string) =>
  api.get<UserWithRoles>(`/users/${id}`).then((r) => r.data);

export const updateUser = (id: string, data: Partial<{ name: string; email: string; is_active: boolean }>) =>
  api.patch<UserWithRoles>(`/users/${id}`, data).then((r) => r.data);

export const deleteUser = (id: string) =>
  api.delete(`/users/${id}`);

// --- Roles ---
export const listRoles = (params?: { page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<RoleWithPermissions>>('/roles', { params }).then((r) => r.data);

export const createRole = (data: { name: string; slug: string; description?: string }) =>
  api.post<RoleWithPermissions>('/roles', data).then((r) => r.data);

export const getRole = (id: string) =>
  api.get<RoleWithPermissions>(`/roles/${id}`).then((r) => r.data);

export const updateRole = (id: string, data: Partial<{ name: string; slug: string; description: string }>) =>
  api.patch<RoleWithPermissions>(`/roles/${id}`, data).then((r) => r.data);

export const deleteRole = (id: string) =>
  api.delete(`/roles/${id}`);

// --- Permissions ---
export const listPermissions = () =>
  api.get<Permission[]>('/permissions').then((r) => r.data);

export const grantPermission = (roleId: string, permissionId: string) =>
  api.post(`/roles/${roleId}/permissions`, { permission_id: permissionId }).then((r) => r.data);

export const revokePermission = (roleId: string, permissionId: string) =>
  api.delete(`/roles/${roleId}/permissions/${permissionId}`);

// --- Role Assignment ---
export const assignRole = (userId: string, roleId: string) =>
  api.post(`/users/${userId}/roles`, { role_id: roleId }).then((r) => r.data);

export const unassignRole = (userId: string, roleId: string) =>
  api.delete(`/users/${userId}/roles/${roleId}`);

// --- API Keys ---
export const listAPIKeys = () =>
  api.get<PaginatedResponse<APIKey>>('/auth/api-keys').then((r) => r.data.data ?? []);

export const createAPIKey = (data: { name: string; role_id?: string; scopes?: string[]; expires_at?: string }) =>
  api.post<APIKeyWithSecret>('/auth/api-keys', data).then((r) => r.data);

export const deleteAPIKey = (id: string) =>
  api.delete(`/auth/api-keys/${id}`);

// --- SSO Providers ---
export const listSSOProviders = () =>
  api.get<SSOProvider[]>('/sso-providers').then((r) => r.data);

export const createSSOProvider = (data: CreateSSOProviderRequest) =>
  api.post<SSOProvider>('/sso-providers', data).then((r) => r.data);

export const updateSSOProvider = (id: string, data: Partial<CreateSSOProviderRequest> & { enabled?: boolean }) =>
  api.patch<SSOProvider>(`/sso-providers/${id}`, data).then((r) => r.data);

export const deleteSSOProvider = (id: string) =>
  api.delete(`/sso-providers/${id}`);

// --- Audit Log ---
export const listAuditLog = (params?: { user_id?: string; action?: string; resource_type?: string; from?: string; to?: string; page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<AuditLogEntry>>('/audit-log', { params }).then((r) => r.data);

// --- Marketplace ---
export const listMarketplaceAgents = (params?: { category?: string; search?: string }) =>
  api.get<MarketplaceAgent[]>('/marketplace', { params }).then((r) => r.data);

export const getMarketplaceAgent = (slug: string) =>
  api.get<MarketplaceAgent>(`/marketplace/${slug}`).then((r) => r.data);

export const installAgent = (slug: string, config?: Record<string, unknown>) =>
  api.post<InstalledAgent>(`/marketplace/${slug}/install`, { config }).then((r) => r.data);

export const uninstallAgent = (installedId: string) =>
  api.delete(`/marketplace/installed/${installedId}`);

export const listInstalledAgents = () =>
  api.get<InstalledAgent[]>('/marketplace/installed').then((r) => r.data);

// --- Observability Data Sources ---
export const listObservabilitySources = (sourceType?: string) =>
  api.get<ObservabilitySource[]>('/observability/sources', { params: sourceType ? { source_type: sourceType } : undefined }).then((r) => r.data);

export const createObservabilitySource = (data: Partial<ObservabilitySource>) =>
  api.post<ObservabilitySource>('/observability/sources', data).then((r) => r.data);

export const getObservabilitySource = (id: string) =>
  api.get<ObservabilitySource>(`/observability/sources/${id}`).then((r) => r.data);

export const updateObservabilitySource = (id: string, data: Partial<ObservabilitySource>) =>
  api.put<ObservabilitySource>(`/observability/sources/${id}`, data).then((r) => r.data);

export const deleteObservabilitySource = (id: string) =>
  api.delete(`/observability/sources/${id}`);

export const checkSourceHealth = (id: string) =>
  api.post<{ status: string; message?: string }>(`/observability/sources/${id}/health`).then((r) => r.data);

// --- Snapshot Configs ---
export const listSnapshotConfigs = (sourceId: string) =>
  api.get<SnapshotConfig[]>(`/observability/sources/${sourceId}/snapshots`).then((r) => r.data);

export const createSnapshotConfig = (sourceId: string, data: Partial<SnapshotConfig>) =>
  api.post<SnapshotConfig>(`/observability/sources/${sourceId}/snapshots`, data).then((r) => r.data);

// sourceId is accepted (not used in the path) to keep this function's
// signature stable for callers -- the corrected backend route addresses a
// snapshot config directly by its own id, not nested under its source.
export const updateSnapshotConfig = (_sourceId: string, configId: string, data: Partial<SnapshotConfig>) =>
  api.put<SnapshotConfig>(`/observability/snapshots/${configId}`, data).then((r) => r.data);

export const deleteSnapshotConfig = (_sourceId: string, configId: string) =>
  api.delete(`/observability/snapshots/${configId}`);

// --- Agent Run Rerun ---
export const rerunAgentRun = (incidentId: string, runId: string) =>
  api.post<AgentRun>(`/incidents/${incidentId}/runs/${runId}/rerun`).then(r => r.data);

// --- Evidence Upload ---
export const uploadEvidence = (incidentId: string, file: File) => {
  const formData = new FormData();
  formData.append('file', file);
  return api.post<IncidentEvidence>(`/incidents/${incidentId}/evidence/upload`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' },
  }).then((r) => r.data);
};

// --- Onboarding ---
export interface OnboardingStatus {
  completed: boolean;
  has_software: boolean;
  has_webhooks: boolean;
  has_incidents: boolean;
}

export const getOnboardingStatus = () =>
  api.get<OnboardingStatus>('/onboarding/status').then((r) => r.data);

// --- Quarantine ---
export interface QuarantinedAlert {
  id: string;
  org_id: string;
  source: string;
  reason: string;
  raw_payload: Record<string, unknown>;
  resolved: boolean;
  software_id: string | null;
  created_at: string;
  resolved_at: string | null;
}

export const listQuarantine = (params?: { resolved?: boolean; page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<QuarantinedAlert>>('/quarantine', {
    params: {
      ...params,
      resolved: params?.resolved ? 'true' : 'false',
    },
  }).then((r) => r.data);

// --- Agent Runs (all) ---
export const listAllAgentRuns = (params?: { page?: number; per_page?: number }) =>
  api.get<PaginatedResponse<AgentRun>>('/agent-runs', { params }).then((r) => r.data);

// --- War Room ---
export const getWarRoom = (incidentId: string) =>
  api.get<WarRoomMeeting>(`/incidents/${incidentId}/warroom`)
    .then((r) => r.data)
    .catch((err) => {
      if (err?.response?.status === 404) return null;
      throw err;
    });

export const createWarRoom = (incidentId: string) =>
  api.post<WarRoomMeeting>(`/incidents/${incidentId}/warroom`).then((r) => r.data);

export const endWarRoom = (meetingId: string) =>
  api.post<WarRoomMeeting>(`/warroom/${meetingId}/end`).then((r) => r.data);

// --- HITL Approval Gate ---
export const approveIncidentStage = (incidentId: string) =>
  api.post<ApproveStageResponse>(`/incidents/${incidentId}/approve-stage`).then((r) => r.data);

// --- Organization Settings ---
export const getOrganizationSettings = (orgId: string) =>
  api.get<OrgSettingsResponse>(`/organizations/${orgId}/settings`).then((r) => r.data);

export const updateOrganizationSettings = (
  orgId: string,
  data: {
    pipeline_hitl_gate_enabled?: boolean;
    default_llm_provider_type?: string;
    default_llm_base_url?: string;
    default_llm_model?: string;
    default_llm_api_key_ref?: string;
    teams_tenant_id?: string;
    teams_client_id?: string;
    teams_client_secret?: string;
  },
) => api.patch<OrgSettingsResponse>(`/organizations/${orgId}/settings`, data).then((r) => r.data);

// Starts the delegated Teams OAuth connect flow: returns the Microsoft
// authorize URL for the caller to navigate the browser to (window.location.href
// = data.authorize_url) -- a plain fetch/XHR can't be redirected through
// Microsoft's own consent screen. See TeamsOAuthHandler.Authorize.
export const initiateTeamsOAuth = (orgId: string) =>
  api.post<{ authorize_url: string }>(`/organizations/${orgId}/integrations/teams/oauth/authorize`).then((r) => r.data);

// Clears the connected Teams account (refresh token + connected email) --
// tenant_id/client_id/client_secret stay saved, so reconnecting doesn't
// require re-entering the Azure AD app registration. See
// TeamsOAuthHandler.Disconnect.
export const disconnectTeamsOAuth = (orgId: string) =>
  api.post<{ disconnected: boolean }>(`/organizations/${orgId}/integrations/teams/oauth/disconnect`).then((r) => r.data);

// --- Postmortem Export ---
export const exportPostmortem = async (incidentId: string, format: 'markdown' | 'pdf') => {
  const res = await api.get(`/incidents/${incidentId}/postmortem/export`, {
    params: { format },
    responseType: 'blob',
  });
  const disposition: string | undefined = res.headers?.['content-disposition'];
  const match = disposition?.match(/filename="?([^"]+)"?/);
  const filename = match?.[1] ?? `postmortem-${incidentId}.${format === 'pdf' ? 'pdf' : 'md'}`;
  return { blob: res.data as Blob, filename };
};

// --- SLO Definitions ---
export const listSLODefinitions = () =>
  api.get<SLODefinition[]>('/slo-definitions').then((r) => r.data);

export const createSLODefinition = (data: CreateSLODefinitionRequest) =>
  api.post<SLODefinition>('/slo-definitions', data).then((r) => r.data);

export const updateSLODefinition = (id: string, data: UpdateSLODefinitionRequest) =>
  api.put<SLODefinition>(`/slo-definitions/${id}`, data).then((r) => r.data);

export const deleteSLODefinition = (id: string) =>
  api.delete(`/slo-definitions/${id}`);

export const getSLOStatus = (id: string) =>
  api.get<SLOStatus>(`/slo-definitions/${id}/status`).then((r) => r.data);

export const getSoftwareSLOStatus = (softwareId: string) =>
  api.get<SoftwareSLOStatus>(`/software/${softwareId}/slo-status`).then((r) => r.data);

// --- Retention Policies ---
export const listRetentionPolicies = () =>
  api.get<{ policies: RetentionPolicy[] }>('/retention-policies').then((r) => r.data.policies ?? []);

export const createRetentionPolicy = (data: CreateRetentionPolicyRequest) =>
  api.post<RetentionPolicy>('/retention-policies', data).then((r) => r.data);

export const updateRetentionPolicy = (id: string, data: UpdateRetentionPolicyRequest) =>
  api.put<RetentionPolicy>(`/retention-policies/${id}`, data).then((r) => r.data);

export const deleteRetentionPolicy = (id: string) =>
  api.delete(`/retention-policies/${id}`);

export const runRetentionSweep = () =>
  api.post<RetentionSweepSummary>('/retention-policies/sweep').then((r) => r.data);
