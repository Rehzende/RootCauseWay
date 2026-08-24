// --- Auth ---
export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

// --- Organization ---
export interface Organization {
  id: string;
  name: string;
  slug: string;
  pipeline_hitl_gate_enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface OrgSettingsResponse {
  org_id: string;
  pipeline_hitl_gate_enabled: boolean;
  default_llm_provider_type?: string;
  default_llm_base_url?: string;
  default_llm_model?: string;
  default_llm_api_key_ref?: string;
  // Microsoft Teams (Graph API) integration, used by War Room. The client
  // secret and refresh token are never returned -- only whether each is
  // set. teams_connected_account is read-only/informational, populated by
  // the OAuth connect flow (see initiateTeamsOAuth) -- never user-typed.
  teams_tenant_id?: string;
  teams_client_id?: string;
  teams_client_secret_set?: boolean;
  teams_refresh_token_set?: boolean;
  teams_connected_account?: string;
  teams_configured?: boolean;
}

// --- User ---
export type UserRole = 'admin' | 'operator' | 'viewer';

export interface User {
  id: string;
  org_id: string;
  name: string;
  email: string;
  role: UserRole;
  created_at: string;
}

// --- Software Catalog ---
export type SoftwareStatus = 'active' | 'deprecated' | 'archived';
export type CloudProvider = 'aws' | 'azure' | 'gcp' | 'on_prem' | 'hybrid';

export interface CloudResource {
  type: string;
  name: string;
  region?: string;
  id?: string;
}

export interface DatabaseInfo {
  type: string;
  name: string;
  host?: string;
  engine?: string;
}

export interface Person {
  name: string;
  email: string;
  role?: string;
  slack?: string;
}

export interface SoftwareEntry {
  id: string;
  org_id: string;
  name: string;
  slug: string;
  description: string;
  owner_id: string;
  repository_url: string;
  pipeline_url?: string;
  cloud_provider?: CloudProvider;
  cloud_resources?: CloudResource[];
  database_info?: DatabaseInfo[];
  infra_details?: Record<string, unknown>;
  stakeholders?: Person[];
  sre_team?: Person[];
  architects?: Person[];
  runbook_url?: string;
  dashboard_url?: string;
  dependencies?: string[];
  tags: string[];
  status: SoftwareStatus;
  created_at: string;
  updated_at: string;
}

export interface CreateSoftwareRequest {
  name: string;
  slug: string;
  description?: string;
  owner_id?: string;
  repository_url?: string;
  pipeline_url?: string;
  cloud_provider?: CloudProvider;
  cloud_resources?: CloudResource[];
  database_info?: DatabaseInfo[];
  infra_details?: Record<string, unknown>;
  stakeholders?: Person[];
  sre_team?: Person[];
  architects?: Person[];
  runbook_url?: string;
  dashboard_url?: string;
  dependencies?: string[];
  tags?: string[];
}

// --- Agent Registry ---
export type AgentType = 'triage' | 'evidence_analysis' | 'hypothesis' | 'debug' | 'custom';

export interface AgentConfig {
  model?: string;
  temperature?: number;
  tools?: string[];
  system_prompt?: string;
}

export interface Agent {
  id: string;
  org_id: string;
  name: string;
  type: AgentType;
  description: string;
  config: AgentConfig;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateAgentRequest {
  name: string;
  type: AgentType;
  description?: string;
  config?: AgentConfig;
}

// --- Webhook ---
export type WebhookSource = 'datadog' | 'prometheus_alertmanager' | 'grafana' | 'otel' | 'custom';

export interface Webhook {
  id: string;
  org_id: string;
  name: string;
  source: WebhookSource;
  software_id: string;
  endpoint_token: string;
  secret: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateWebhookRequest {
  name: string;
  source: WebhookSource;
  software_id: string;
}

// --- Incident ---
export type Severity = 'critical' | 'high' | 'medium' | 'low';
export type IncidentStatus = 'open' | 'investigating' | 'mitigated' | 'resolved' | 'closed';

export type IncidentEventType =
  | 'alert_received'
  | 'triage_started'
  | 'triage_completed'
  | 'evidence_collected'
  | 'hypothesis_generated'
  | 'status_changed'
  | 'comment'
  | 'agent_action'
  | 'rci_started'
  | 'rci_completed'
  | 'rca_started'
  | 'rca_completed'
  | 'postmortem_started'
  | 'postmortem_completed'
  | 'agent_run_started'
  | 'agent_run_completed'
  | 'agent_run_failed'
  | 'correlated_alert'
  | 'war_room_created';

export type EvidenceType = 'log' | 'metric' | 'trace' | 'snapshot' | 'agent_output' | 'manual';

export interface IncidentEvent {
  id: string;
  incident_id: string;
  type: IncidentEventType;
  actor: string;
  data: Record<string, unknown>;
  created_at: string;
}

export interface IncidentEvidence {
  id: string;
  incident_id: string;
  type: EvidenceType;
  title: string;
  content: Record<string, unknown>;
  source: string;
  collected_at: string;
}

export interface Incident {
  id: string;
  // Sequential per-org display number (e.g. 42 -> "INC-0042", see
  // formatIncidentCode). Assigned once at creation, immutable; the UUID
  // `id` above remains the real identifier for API paths and FKs.
  incident_number: number;
  org_id: string;
  software_id: string;
  title: string;
  description: string;
  severity: Severity;
  status: IncidentStatus;
  assignee_id: string;
  source_alert_id: string;
  root_cause: string;
  mitigation: string;
  timeline: IncidentEvent[];
  evidence: IncidentEvidence[];
  created_at: string;
  updated_at: string;
  resolved_at: string;
  // HITL pipeline approval gate (see /incidents/{id}/approve-stage)
  awaiting_approval_stage?: string | null;
  approved_by?: string | null;
  approved_at?: string | null;
}

export interface ApproveStageResponse {
  incident_id: string;
  stage: string;
  approved_by: string;
  status: string;
}

export interface UpdateIncidentRequest {
  status?: IncidentStatus;
  severity?: Severity;
  assignee_id?: string;
  root_cause?: string;
  mitigation?: string;
}

export interface CreateIncidentEventRequest {
  type: 'comment' | 'status_changed' | 'agent_action';
  data?: Record<string, unknown>;
}

export interface CreateIncidentEvidenceRequest {
  type: EvidenceType;
  title: string;
  content: Record<string, unknown>;
  source?: string;
}

// --- Alert ---
export interface NormalizedAlert {
  title: string;
  description: string;
  severity: Severity;
  source: WebhookSource;
  service: string;
  tags: Record<string, string>;
  started_at: string;
  labels: Record<string, string>;
}

export interface AlertSnapshot {
  id: string;
  incident_id: string;
  software_id: string;
  raw_payload: Record<string, unknown>;
  normalized: NormalizedAlert;
  snapshots: Record<string, unknown>;
  created_at: string;
}

// --- Pagination ---
export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}

// --- Agent Runs ---
export type AgentRunStatus = 'pending' | 'running' | 'completed' | 'failed';

export interface AgentRun {
  id: string;
  incident_id: string;
  agent_id: string;
  agent_name: string;
  agent_type: AgentType;
  status: AgentRunStatus;
  parent_run_id: string | null;
  input_data: Record<string, unknown>;
  output_data: Record<string, unknown>;
  error_message: string | null;
  model_used: string;
  tokens_used: number;
  duration_ms: number;
  started_at: string;
  completed_at: string | null;
}

// --- RCI ---
export type AnalysisStatus = 'draft' | 'in_progress' | 'completed' | 'reviewed';

export interface IncidentRCI {
  id: string;
  incident_id: string;
  status: AnalysisStatus;
  investigation_summary: string;
  impact_assessment: string;
  affected_services: string[];
  affected_users_estimate: number;
  detection_method: string;
  detection_time: string;
  evidence_ids: string[];
}

// --- RCA ---
export interface IncidentRCA {
  id: string;
  incident_id: string;
  status: AnalysisStatus;
  root_cause_summary: string;
  root_cause_category: string;
  contributing_factors: string[];
  five_whys: string[];
  confidence: number;
  evidence_ids: string[];
}

// --- Postmortem ---
export type PostmortemStatus = 'draft' | 'in_review' | 'published';

export interface PostmortemActionItem {
  description: string;
  owner: string;
  due_date: string;
  completed: boolean;
}

export interface IncidentPostmortem {
  id: string;
  incident_id: string;
  status: PostmortemStatus;
  title: string;
  executive_summary: string;
  incident_timeline_narrative: string;
  root_cause_detail: string;
  impact_detail: string;
  lessons_learned: string[];
  action_items: PostmortemActionItem[];
  what_went_well: string[];
  what_went_wrong: string[];
  prevention_measures: string[];
}

// --- Incident Full ---
export interface SoftwareContext {
  id: string;
  name: string;
  slug: string;
  description?: string;
  status: string;
  cloud_provider?: string;
  repository_url?: string;
  dashboard_url?: string;
  runbook_url?: string;
  pipeline_url?: string;
  tags?: string[];
  sre_team?: any[];
  stakeholders?: any[];
  architects?: any[];
  owner_id?: string;
  dependencies?: any[];
}

export interface IncidentFull extends Incident {
  software?: SoftwareContext | null;
  agent_runs: AgentRun[];
  rci: IncidentRCI | null;
  rca: IncidentRCA | null;
  postmortem: IncidentPostmortem | null;
}

// --- A2A Agents ---
export type AgentHostingType = 'managed' | 'byoa';
export type LLMProvider = 'platform' | 'custom';
export type A2AAgentType = 'triage' | 'evidence_analysis' | 'rca' | 'postmortem' | 'custom';
export type A2AAuthType = 'none' | 'bearer' | 'api_key';
export type A2AHealthStatus = 'healthy' | 'unhealthy' | 'unknown';
export type A2ATaskStatus = 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';

export interface AgentSkill {
  id: string;
  name: string;
  description?: string;
}

export interface A2AAgent {
  id: string;
  org_id: string;
  name: string;
  description: string;
  agent_type: A2AAgentType;
  endpoint_url: string;
  agent_card: Record<string, unknown>;
  skills: AgentSkill[];
  allowed_software_ids: string[];
  auth_type: A2AAuthType;
  enabled: boolean;
  is_system: boolean;
  hosting_type: AgentHostingType;
  managed_config?: Record<string, unknown>;
  llm_provider: LLMProvider;
  llm_api_key_ref?: string;
  auto_scale: boolean;
  min_replicas: number;
  max_replicas: number;
  health_status: A2AHealthStatus;
  last_health_check: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateA2AAgentRequest {
  name: string;
  description?: string;
  agent_type: A2AAgentType;
  endpoint_url: string;
  auth_type?: A2AAuthType;
  auth_credentials?: string;
  skills?: AgentSkill[];
  allowed_software_ids?: string[];
  hosting_type?: AgentHostingType;
  managed_config?: Record<string, unknown>;
  llm_provider?: LLMProvider;
  llm_api_key_ref?: string;
  auto_scale?: boolean;
  min_replicas?: number;
  max_replicas?: number;
}

export interface A2ATask {
  id: string;
  incident_id: string;
  agent_id: string;
  agent_run_id: string | null;
  task_type: string;
  status: A2ATaskStatus;
  input_message: Record<string, unknown>;
  output_artifacts: Record<string, unknown>[];
  error_message: string | null;
  orchestrator_reasoning: string | null;
  priority: number;
  depends_on: string[];
  submitted_at: string;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
}

export interface OrchestratorDecision {
  id: string;
  incident_id: string;
  decision_type: string;
  reasoning: string;
  selected_agents: string[];
  context_used: Record<string, unknown>;
  confidence: number;
  created_at: string;
}

// --- Skills Registry ---
export type SkillCategory = 'infrastructure' | 'application' | 'database' | 'network' | 'security' | 'cloud' | 'observability' | 'custom';

export interface Skill {
  id: string;
  org_id: string;
  name: string;
  slug: string;
  description: string;
  category: SkillCategory;
  prompt_template?: string;
  input_schema?: Record<string, unknown>;
  output_schema?: Record<string, unknown>;
  required_tools?: string[];
  required_resource_types?: string[];
  required_permissions?: string[];
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateSkillRequest {
  name: string;
  slug: string;
  description?: string;
  category: string;
  prompt_template?: string;
  required_resource_types?: string[];
  required_permissions?: string[];
}

export interface AgentSkillLink {
  id: string;
  agent_id: string;
  skill_id: string;
  priority: number;
}

// --- Credential Management ---
export type CredentialProviderType = 'hashicorp_vault' | 'aws_sts' | 'azure_managed_identity' | 'azure_key_vault' | 'gcp_workload_identity' | 'static' | 'custom';

export interface CredentialProvider {
  id: string;
  org_id: string;
  name: string;
  provider_type: CredentialProviderType;
  config: Record<string, unknown>;
  enabled: boolean;
  created_at: string;
}

export interface CreateCredentialProviderRequest {
  name: string;
  provider_type: CredentialProviderType;
  config: Record<string, unknown>;
}

export interface ResourceCredential {
  id: string;
  software_id: string;
  resource_name: string;
  resource_type: string;
  provider_id: string;
  credential_path: string;
  default_ttl_seconds: number;
  max_ttl_seconds: number;
}

export interface CreateResourceCredentialRequest {
  software_id: string;
  resource_name: string;
  resource_type: string;
  provider_id: string;
  credential_path: string;
  default_ttl_seconds: number;
  max_ttl_seconds: number;
}

export type AccessPolicyTargetType = 'agent' | 'skill' | 'agent_type';

export interface AccessPolicy {
  id: string;
  name: string;
  target_type: AccessPolicyTargetType;
  target_id: string;
  resource_type: string;
  allowed_actions: string[];
  max_ttl_seconds: number;
  require_approval: boolean;
  enabled: boolean;
}

export interface CreateAccessPolicyRequest {
  name: string;
  target_type: AccessPolicyTargetType;
  target_id: string;
  resource_type: string;
  allowed_actions: string[];
  max_ttl_seconds: number;
  require_approval: boolean;
}

export type CredentialLeaseStatus = 'active' | 'expired' | 'revoked';

export interface CredentialLease {
  id: string;
  incident_id?: string;
  agent_id?: string;
  skill_id?: string;
  status: CredentialLeaseStatus;
  scope: Record<string, unknown>;
  issued_at: string;
  expires_at: string;
  revoked_at?: string;
  request_reason: string;
}

// --- Loop Engineering ---
export interface IncidentFeedback {
  id: string;
  incident_id: string;
  user_id?: string;
  target_type: string;
  rating: 'positive' | 'negative' | 'neutral';
  correction?: string;
  created_at: string;
}

export interface KnowledgeBaseEntry {
  id: string;
  org_id: string;
  incident_id?: string;
  software_id?: string;
  category?: string;
  error_pattern?: string;
  root_cause_summary: string;
  resolution_summary?: string;
  lessons_learned?: string[];
  tags?: string[];
  human_validated: boolean;
  confidence: number;
  times_referenced: number;
  created_at: string;
}

export interface SimilarIncident {
  id: string;
  incident_id: string;
  similar_incident_id: string;
  similarity_score: number;
  matched_on: Record<string, unknown>;
}

// --- Correlation ---
export interface CorrelationRule {
  id: string;
  name: string;
  rule_type: string;
  config: Record<string, unknown>;
  time_window_seconds: number;
  enabled: boolean;
}

export interface AlertGroup {
  id: string;
  incident_id: string;
  alert_snapshot_id: string;
  correlation_rule_id?: string;
}

// --- Notifications ---
export type NotificationChannelType = 'slack' | 'teams' | 'pagerduty' | 'email' | 'webhook';

export interface NotificationChannel {
  id: string;
  name: string;
  channel_type: NotificationChannelType;
  config: Record<string, unknown>;
  enabled: boolean;
}

export interface EscalationStep {
  delay_seconds: number;
  channel_id: string;
  recipients: string[];
}

export interface EscalationPolicy {
  id: string;
  name: string;
  description?: string;
  software_id?: string;
  severity_filter: string[];
  steps: EscalationStep[];
  enabled: boolean;
}

export interface NotificationLogEntry {
  id: string;
  incident_id?: string;
  channel_id?: string;
  event_type: string;
  recipient?: string;
  status: string;
  sent_at?: string;
  created_at: string;
}

// --- Runbooks ---
export type RunbookStepType = 'manual' | 'automated' | 'approval' | 'notification' | 'condition';

export interface RunbookStep {
  id: string;
  runbook_id: string;
  step_order: number;
  name: string;
  description?: string;
  step_type: RunbookStepType;
  config: Record<string, unknown>;
  skill_id?: string;
  timeout_seconds: number;
  on_failure: string;
}

export interface Runbook {
  id: string;
  software_id?: string;
  name: string;
  slug: string;
  description?: string;
  trigger_conditions: Record<string, unknown>;
  auto_trigger: boolean;
  enabled: boolean;
  steps?: RunbookStep[];
}

export interface StepResult {
  step_id: string;
  step_name?: string;
  step_type?: string;
  status: string;
  output?: Record<string, unknown>;
  started_at?: string;
  completed_at?: string;
}

export interface RunbookExecution {
  id: string;
  runbook_id: string;
  incident_id?: string;
  triggered_by?: string;
  status: string;
  current_step: number;
  step_results: StepResult[];
  started_at?: string;
  completed_at?: string;
}

// --- Change Events ---
export interface ChangeEvent {
  id: string;
  software_id?: string;
  change_type: string;
  title: string;
  description?: string;
  source?: string;
  source_url?: string;
  commit_sha?: string;
  author?: string;
  environment?: string;
  occurred_at: string;
}

// --- Analytics ---
export interface AnalyticsMTTR {
  software_id: string;
  software_name: string;
  avg_mttr_seconds: number;
  incident_count: number;
}

export interface AnalyticsIncidentTrend {
  date: string;
  count: number;
  severity: string;
}

export interface AnalyticsAgentEffectiveness {
  agent_name: string;
  total_tasks: number;
  success_rate: number;
  avg_duration_ms: number;
}

// --- Roles & Permissions ---
export interface Role {
  id: string;
  org_id: string;
  name: string;
  slug: string;
  description?: string;
  is_system: boolean;
  created_at: string;
}

export interface Permission {
  id: string;
  resource: string;
  action: string;
  description?: string;
}

export interface RoleWithPermissions extends Role {
  permissions: Permission[];
}

export interface UserWithRoles extends User {
  roles: Role[];
  permissions: Record<string, string[]>;
  sso_provider?: string;
  avatar_url?: string;
  last_login_at?: string;
  is_active: boolean;
}

// --- SSO ---
export interface SSOProvider {
  id: string;
  org_id: string;
  name: string;
  provider_type: string;
  client_id: string;
  issuer_url?: string;
  auto_provision_users: boolean;
  default_role_id?: string;
  enabled: boolean;
}

export interface CreateSSOProviderRequest {
  name: string;
  provider_type: string;
  client_id: string;
  client_secret: string;
  issuer_url?: string;
  authorization_url?: string;
  token_url?: string;
  scopes?: string;
  auto_provision_users?: boolean;
  default_role_id?: string;
}

// --- API Keys ---
export interface APIKey {
  id: string;
  name: string;
  key_prefix: string;
  scopes: string[];
  expires_at?: string;
  last_used_at?: string;
  is_active: boolean;
  created_at: string;
}

export interface APIKeyWithSecret extends APIKey {
  key: string;
}

// --- Audit Log ---
export interface AuditLogEntry {
  id: string;
  user_id?: string;
  action: string;
  resource_type: string;
  resource_id?: string;
  details: Record<string, unknown>;
  ip_address?: string;
  request_id?: string;
  created_at: string;
}

// --- Marketplace ---
export type MarketplaceCategory = 'triage' | 'evidence' | 'rca' | 'postmortem' | 'security' | 'infrastructure' | 'database' | 'cloud' | 'custom';

export interface MarketplaceAgent {
  id: string;
  name: string;
  slug: string;
  description?: string;
  long_description?: string;
  author: string;
  author_url?: string;
  version: string;
  category: MarketplaceCategory;
  icon_url?: string;
  docker_image?: string;
  agent_card: Record<string, unknown>;
  skills: AgentSkill[];
  required_credentials: string[];
  config_schema: Record<string, unknown>;
  readme?: string;
  downloads: number;
  rating: number;
  verified: boolean;
  published: boolean;
}

export type InstalledAgentStatus = 'installing' | 'installed' | 'updating' | 'failed' | 'uninstalled';

export interface InstalledAgent {
  id: string;
  org_id: string;
  marketplace_agent_id: string;
  a2a_agent_id?: string;
  config: Record<string, unknown>;
  version?: string;
  status: InstalledAgentStatus;
  installed_at: string;
}

// --- Observability Data Sources ---
export type ObservabilitySourceType = 'datadog' | 'prometheus' | 'loki' | 'tempo' | 'grafana' | 'elasticsearch' | 'splunk' | 'cloudwatch' | 'azure_monitor' | 'gcp_monitoring' | 'newrelic' | 'dynatrace' | 'jaeger' | 'zipkin' | 'custom';
export type ObservabilityAuthType = 'api_key' | 'bearer' | 'basic' | 'oauth2' | 'none';

export interface ObservabilitySource {
  id: string;
  org_id: string;
  name: string;
  source_type: ObservabilitySourceType;
  base_url: string;
  auth_type: ObservabilityAuthType;
  auth_config: Record<string, unknown>;
  capabilities: string[];
  monitored_software_ids: string[];
  timeout_seconds: number;
  verify_ssl: boolean;
  custom_headers: Record<string, string>;
  enabled: boolean;
  health_status: 'healthy' | 'unhealthy' | 'unknown';
  last_health_check?: string;
  description?: string;
  environment?: string;
  region?: string;
  created_at: string;
  updated_at: string;
}

export interface SnapshotConfig {
  id: string;
  source_id: string;
  software_id?: string;
  name: string;
  snapshot_type: 'metrics' | 'logs' | 'traces' | 'dashboard' | 'alerts' | 'custom';
  query_template: string;
  time_range_seconds: number;
  parameters: Record<string, unknown>;
  enabled: boolean;
}

// --- Cost Analytics ---
export interface AnalyticsCostByModel {
  model: string;
  total_runs: number;
  total_tokens: number;
  est_cost_usd: number;
}

export interface AnalyticsCostByIncident {
  incident_id: string;
  incident_title: string;
  total_runs: number;
  total_tokens: number;
  est_cost_usd: number;
  total_duration_ms: number;
  created_at: string;
}

// --- War Room ---
export type WarRoomStatus = 'scheduled' | 'active' | 'ended' | 'summarized';

export interface WarRoomActionItem {
  description: string;
  owner_hint?: string;
}

export interface WarRoomSummary {
  executive_summary: string;
  key_points: string[];
  action_items: WarRoomActionItem[];
}

export interface WarRoomAttendee {
  name?: string;
  email?: string;
  join_time?: string;
  leave_time?: string;
}

export interface WarRoomMeeting {
  id: string;
  org_id: string;
  incident_id: string;
  provider: string;
  external_meeting_id: string;
  join_url: string;
  status: WarRoomStatus;
  started_at?: string | null;
  ended_at?: string | null;
  raw_transcript?: string | null;
  attendance?: WarRoomAttendee[] | null;
  summary?: WarRoomSummary | null;
  created_at: string;
  updated_at: string;
}

// --- SLO / Error Budget Tracking ---
export type SLOType = 'availability' | 'latency' | 'error_rate';
export type SLOHealthStatus = 'healthy' | 'at_risk' | 'exhausted';

export interface SLODefinition {
  id: string;
  org_id: string;
  software_id: string;
  name: string;
  slo_type: SLOType;
  target_percentage: number;
  measurement_window_days: number;
  created_at: string;
  updated_at: string;
}

export interface CreateSLODefinitionRequest {
  software_id: string;
  name: string;
  slo_type: SLOType;
  target_percentage: number;
  measurement_window_days?: number;
}

export interface UpdateSLODefinitionRequest {
  name?: string;
  slo_type?: SLOType;
  target_percentage?: number;
  measurement_window_days?: number;
}

export interface SLOStatus {
  slo_definition_id: string;
  software_id: string;
  slo_type: SLOType;
  target_percentage: number;
  window_start: string;
  window_end: string;
  current_percentage: number;
  error_budget_total_minutes: number;
  error_budget_consumed_minutes: number;
  error_budget_remaining_percentage: number;
  status: SLOHealthStatus;
  incident_count: number;
  is_approximated: boolean;
}

export interface SoftwareSLOStatus {
  software_id: string;
  slos: SLOStatus[];
}

// --- Data Retention & Archival ---
export type RetentionResourceType = 'evidence' | 'incidents' | 'agent_runs';
export type RetentionActionType = 'archive' | 'delete';

export interface RetentionPolicy {
  id: string;
  org_id: string;
  resource_type: RetentionResourceType;
  retention_days: number;
  action: RetentionActionType;
  enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateRetentionPolicyRequest {
  resource_type: RetentionResourceType;
  retention_days: number;
  action: RetentionActionType;
  enabled?: boolean;
}

export interface UpdateRetentionPolicyRequest {
  retention_days?: number;
  action?: RetentionActionType;
  enabled?: boolean;
}

export interface RetentionSweepResult {
  policy_id: string;
  resource_type: string;
  action: string;
  matched_count: number;
  archived_count: number;
  deleted_count: number;
  errors?: string[];
}

export interface RetentionSweepSummary {
  org_id: string;
  started_at: string;
  results: RetentionSweepResult[];
}

// --- Error ---
export interface ErrorResponse {
  error: string;
  code: string;
  details: Record<string, unknown>;
}
