package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Organization represents a tenant organization.
type Organization struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	Slug string    `json:"slug"`
	// PipelineHITLGateEnabled gates the postmortem stage behind human approval
	// when true: the orchestrator pauses after RCA instead of auto-generating
	// the postmortem, and waits for a pipeline.stage_approved event.
	PipelineHITLGateEnabled bool      `json:"pipeline_hitl_gate_enabled"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// User represents an authenticated user.
type User struct {
	ID           uuid.UUID `json:"id"`
	OrgID        uuid.UUID `json:"org_id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // admin, operator, viewer
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SoftwareEntry represents a registered software/service.
type SoftwareEntry struct {
	ID             uuid.UUID       `json:"id"`
	OrgID          uuid.UUID       `json:"org_id"`
	Name           string          `json:"name"`
	Slug           string          `json:"slug"`
	Description    string          `json:"description"`
	OwnerID        *uuid.UUID      `json:"owner_id,omitempty"`
	RepositoryURL  string          `json:"repository_url"`
	Tags           []string        `json:"tags"`
	Status         string          `json:"status"` // active, deprecated, archived
	PipelineURL    string          `json:"pipeline_url"`
	CloudProvider  string          `json:"cloud_provider"`
	CloudResources json.RawMessage `json:"cloud_resources"`
	DatabaseInfo   json.RawMessage `json:"database_info"`
	InfraDetails   json.RawMessage `json:"infra_details"`
	Stakeholders   json.RawMessage `json:"stakeholders"`
	SreTeam        json.RawMessage `json:"sre_team"`
	Architects     json.RawMessage `json:"architects"`
	RunbookURL     string          `json:"runbook_url"`
	DashboardURL   string          `json:"dashboard_url"`
	Dependencies   json.RawMessage `json:"dependencies"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// CreateSoftwareRequest is the request body for creating a software entry.
type CreateSoftwareRequest struct {
	Name           string          `json:"name" binding:"required"`
	Slug           string          `json:"slug" binding:"required"`
	Description    string          `json:"description"`
	OwnerID        *uuid.UUID      `json:"owner_id,omitempty"`
	RepositoryURL  string          `json:"repository_url"`
	Tags           []string        `json:"tags"`
	PipelineURL    string          `json:"pipeline_url"`
	CloudProvider  string          `json:"cloud_provider"`
	CloudResources json.RawMessage `json:"cloud_resources"`
	DatabaseInfo   json.RawMessage `json:"database_info"`
	InfraDetails   json.RawMessage `json:"infra_details"`
	Stakeholders   json.RawMessage `json:"stakeholders"`
	SreTeam        json.RawMessage `json:"sre_team"`
	Architects     json.RawMessage `json:"architects"`
	RunbookURL     string          `json:"runbook_url"`
	DashboardURL   string          `json:"dashboard_url"`
	Dependencies   json.RawMessage `json:"dependencies"`
}

// AgentConfig holds configuration for an AI agent.
type AgentConfig struct {
	Model        string   `json:"model,omitempty"`
	Temperature  float64  `json:"temperature,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	SystemPrompt string   `json:"system_prompt,omitempty"`
}

// Agent represents an AI agent configuration.
type Agent struct {
	ID          uuid.UUID   `json:"id"`
	OrgID       uuid.UUID   `json:"org_id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"` // triage, evidence_analysis, hypothesis, debug, custom
	Description string      `json:"description"`
	Config      AgentConfig `json:"config"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// CreateAgentRequest is the request body for creating an agent.
type CreateAgentRequest struct {
	Name        string      `json:"name" binding:"required"`
	Type        string      `json:"type" binding:"required"`
	Description string      `json:"description"`
	Config      AgentConfig `json:"config"`
}

// Webhook represents a configured webhook endpoint.
type Webhook struct {
	ID            uuid.UUID `json:"id"`
	OrgID         uuid.UUID `json:"org_id"`
	Name          string    `json:"name"`
	Source        string    `json:"source"` // datadog, prometheus_alertmanager, grafana, otel, custom
	SoftwareID    uuid.UUID `json:"software_id"`
	EndpointToken string    `json:"endpoint_token"`
	Secret        string    `json:"secret,omitempty"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateWebhookRequest is the request body for creating a webhook.
type CreateWebhookRequest struct {
	Name       string    `json:"name" binding:"required"`
	Source     string    `json:"source" binding:"required"`
	SoftwareID uuid.UUID `json:"software_id" binding:"required"`
}

// NormalizedAlert is a source-agnostic alert representation.
type NormalizedAlert struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Severity    string            `json:"severity"`
	Source      string            `json:"source"`
	Service     string            `json:"service"`
	Tags        map[string]string `json:"tags"`
	StartedAt   time.Time         `json:"started_at"`
	Labels      map[string]string `json:"labels"`
	// Fingerprint identifies literally-repeated firings of the same alert (e.g. an
	// Alertmanager fingerprint) so the correlation engine can dedup them instead of
	// re-running full correlation logic. Optional; empty when the source doesn't
	// provide one.
	Fingerprint string `json:"fingerprint,omitempty"`
}

// AlertQuarantine stores alerts that couldn't be matched to a software entry.
type AlertQuarantine struct {
	ID                 uuid.UUID       `json:"id"`
	OrgID              uuid.UUID       `json:"org_id"`
	WebhookID          uuid.UUID       `json:"webhook_id"`
	Source             string          `json:"source"`
	RawPayload         json.RawMessage `json:"raw_payload"`
	NormalizedTitle    string          `json:"normalized_title"`
	NormalizedSeverity string          `json:"normalized_severity"`
	Labels             json.RawMessage `json:"labels"`
	Reason             string          `json:"reason"`
	Resolved           bool            `json:"resolved"`
	ResolvedAt         *time.Time      `json:"resolved_at,omitempty"`
	ResolvedSoftwareID *uuid.UUID      `json:"resolved_software_id,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

type ResolveQuarantineRequest struct {
	SoftwareID uuid.UUID `json:"software_id" binding:"required"`
}

// AlertSnapshot stores the raw and normalized alert data.
type AlertSnapshot struct {
	ID         uuid.UUID       `json:"id"`
	IncidentID uuid.UUID       `json:"incident_id"`
	SoftwareID uuid.UUID       `json:"software_id"`
	RawPayload json.RawMessage `json:"raw_payload"`
	Normalized NormalizedAlert `json:"normalized"`
	Snapshots  json.RawMessage `json:"snapshots"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Incident represents an incident under investigation.
type Incident struct {
	ID uuid.UUID `json:"id"`
	// IncidentNumber is sequential per org (1, 2, 3, ...), assigned once at
	// creation (PgIncidentRepository.Create) and immutable after that --
	// see FormatIncidentCode for the "INC-0001" display form. The UUID
	// above stays the real identifier everywhere internally (foreign keys,
	// API paths); this is purely a human-friendly display/reference number.
	IncidentNumber int64               `json:"incident_number"`
	OrgID          uuid.UUID           `json:"org_id"`
	SoftwareID     uuid.UUID           `json:"software_id"`
	Title          string              `json:"title"`
	Description    string              `json:"description"`
	Severity       string              `json:"severity"`
	Status         string              `json:"status"`
	AssigneeID     *uuid.UUID          `json:"assignee_id,omitempty"`
	SourceAlertID  string              `json:"source_alert_id"`
	RootCause      string              `json:"root_cause"`
	Mitigation     string              `json:"mitigation"`
	Timeline       []IncidentEvent     `json:"timeline,omitempty"`
	Evidence       []IncidentEvidence  `json:"evidence,omitempty"`
	AgentRuns      []AgentRun          `json:"agent_runs,omitempty"`
	RCI            *IncidentRCI        `json:"rci,omitempty"`
	RCA            *IncidentRCA        `json:"rca,omitempty"`
	Postmortem     *IncidentPostmortem `json:"postmortem,omitempty"`
	// AwaitingApprovalStage is set (e.g. "postmortem") while the pipeline is
	// paused waiting for a human to approve the next stage via the HITL gate.
	// Nil/empty when the pipeline isn't paused.
	AwaitingApprovalStage *string    `json:"awaiting_approval_stage,omitempty"`
	ApprovedBy            *uuid.UUID `json:"approved_by,omitempty"`
	ApprovedAt            *time.Time `json:"approved_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	ResolvedAt            *time.Time `json:"resolved_at,omitempty"`
}

// FormatIncidentCode renders an incident's sequential per-org number as the
// human-friendly "INC-0001" display code. Zero-padded to 4 digits but not
// truncated -- number 12345 renders as "INC-12345", not clipped.
func FormatIncidentCode(incidentNumber int64) string {
	return fmt.Sprintf("INC-%04d", incidentNumber)
}

// UpdateIncidentRequest is the request body for updating an incident.
type UpdateIncidentRequest struct {
	Status     *string    `json:"status,omitempty"`
	Severity   *string    `json:"severity,omitempty"`
	AssigneeID *uuid.UUID `json:"assignee_id,omitempty"`
	RootCause  *string    `json:"root_cause,omitempty"`
	Mitigation *string    `json:"mitigation,omitempty"`
}

// IncidentEvent represents a timeline event within an incident.
type IncidentEvent struct {
	ID         uuid.UUID       `json:"id"`
	IncidentID uuid.UUID       `json:"incident_id"`
	Type       string          `json:"type"`
	Actor      string          `json:"actor"`
	Data       json.RawMessage `json:"data"`
	CreatedAt  time.Time       `json:"created_at"`
}

// CreateEventRequest is the request body for creating a timeline event.
type CreateEventRequest struct {
	Type string          `json:"type" binding:"required"`
	Data json.RawMessage `json:"data"`
}

// IncidentEvidence represents a piece of evidence attached to an incident.
type IncidentEvidence struct {
	ID            uuid.UUID       `json:"id"`
	IncidentID    uuid.UUID       `json:"incident_id"`
	Type          string          `json:"type"`
	Title         string          `json:"title"`
	Content       json.RawMessage `json:"content"`
	Source        string          `json:"source"`
	CollectedAt   time.Time       `json:"collected_at"`
	BlobPath      string          `json:"blob_path,omitempty"`
	BlobSizeBytes int64           `json:"blob_size_bytes,omitempty"`
	MimeType      string          `json:"mime_type,omitempty"`
}

// CreateEvidenceRequest is the request body for creating evidence.
type CreateEvidenceRequest struct {
	Type    string          `json:"type" binding:"required"`
	Title   string          `json:"title" binding:"required"`
	Content json.RawMessage `json:"content" binding:"required"`
	Source  string          `json:"source"`
}

// PaginatedResponse wraps a paginated list response.
type PaginatedResponse struct {
	Data    interface{} `json:"data"`
	Total   int         `json:"total"`
	Page    int         `json:"page"`
	PerPage int         `json:"per_page"`
}

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Error   string      `json:"error"`
	Code    string      `json:"code,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

// LoginRequest is the request body for authentication.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// LoginResponse is the response after successful authentication.
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

// AgentRun represents a single agent execution node in the DAG.
type AgentRun struct {
	ID           uuid.UUID       `json:"id"`
	IncidentID   uuid.UUID       `json:"incident_id"`
	AgentID      *uuid.UUID      `json:"agent_id,omitempty"`
	AgentName    string          `json:"agent_name"`
	AgentType    string          `json:"agent_type"`
	Status       string          `json:"status"`
	ParentRunID  *uuid.UUID      `json:"parent_run_id,omitempty"`
	InputData    json.RawMessage `json:"input_data"`
	OutputData   json.RawMessage `json:"output_data"`
	ErrorMessage string          `json:"error_message,omitempty"`
	ModelUsed    string          `json:"model_used,omitempty"`
	TokensUsed   int             `json:"tokens_used"`
	DurationMs   int             `json:"duration_ms"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// CreateAgentRunRequest is the request body for creating an agent run.
type CreateAgentRunRequest struct {
	AgentID     *uuid.UUID      `json:"agent_id,omitempty"`
	AgentName   string          `json:"agent_name" binding:"required"`
	AgentType   string          `json:"agent_type" binding:"required"`
	ParentRunID *uuid.UUID      `json:"parent_run_id,omitempty"`
	InputData   json.RawMessage `json:"input_data"`
	ModelUsed   string          `json:"model_used"`
}

// UpdateAgentRunRequest is the request body for updating an agent run.
type UpdateAgentRunRequest struct {
	Status       *string         `json:"status,omitempty"`
	OutputData   json.RawMessage `json:"output_data,omitempty"`
	ErrorMessage *string         `json:"error_message,omitempty"`
	ModelUsed    *string         `json:"model_used,omitempty"`
	TokensUsed   *int            `json:"tokens_used,omitempty"`
	DurationMs   *int            `json:"duration_ms,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
}

// IncidentRCI represents a Root Cause Investigation.
type IncidentRCI struct {
	ID                    uuid.UUID       `json:"id"`
	IncidentID            uuid.UUID       `json:"incident_id"`
	AgentRunID            *uuid.UUID      `json:"agent_run_id,omitempty"`
	Status                string          `json:"status"`
	InvestigationSummary  string          `json:"investigation_summary"`
	ImpactAssessment      json.RawMessage `json:"impact_assessment"`
	AffectedServices      json.RawMessage `json:"affected_services"`
	AffectedUsersEstimate *int            `json:"affected_users_estimate,omitempty"`
	DetectionMethod       string          `json:"detection_method"`
	DetectionTime         *time.Time      `json:"detection_time,omitempty"`
	AcknowledgmentTime    *time.Time      `json:"acknowledgment_time,omitempty"`
	TimeToDetectSeconds   *int            `json:"time_to_detect_seconds,omitempty"`
	EvidenceIDs           json.RawMessage `json:"evidence_ids"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

// CreateRCIRequest is the request body for creating an RCI.
type CreateRCIRequest struct {
	AgentRunID            *uuid.UUID      `json:"agent_run_id,omitempty"`
	Status                string          `json:"status"`
	InvestigationSummary  string          `json:"investigation_summary"`
	ImpactAssessment      json.RawMessage `json:"impact_assessment"`
	AffectedServices      json.RawMessage `json:"affected_services"`
	AffectedUsersEstimate *int            `json:"affected_users_estimate,omitempty"`
	DetectionMethod       string          `json:"detection_method"`
	DetectionTime         *time.Time      `json:"detection_time,omitempty"`
	AcknowledgmentTime    *time.Time      `json:"acknowledgment_time,omitempty"`
	TimeToDetectSeconds   *int            `json:"time_to_detect_seconds,omitempty"`
	EvidenceIDs           json.RawMessage `json:"evidence_ids"`
}

// IncidentRCA represents a Root Cause Analysis.
type IncidentRCA struct {
	ID                  uuid.UUID       `json:"id"`
	IncidentID          uuid.UUID       `json:"incident_id"`
	RCIID               *uuid.UUID      `json:"rci_id,omitempty"`
	AgentRunID          *uuid.UUID      `json:"agent_run_id,omitempty"`
	Status              string          `json:"status"`
	RootCauseSummary    string          `json:"root_cause_summary"`
	RootCauseCategory   string          `json:"root_cause_category"`
	ContributingFactors json.RawMessage `json:"contributing_factors"`
	FiveWhys            json.RawMessage `json:"five_whys"`
	Confidence          float64         `json:"confidence"`
	EvidenceIDs         json.RawMessage `json:"evidence_ids"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// CreateRCARequest is the request body for creating an RCA.
type CreateRCARequest struct {
	RCIID               *uuid.UUID      `json:"rci_id,omitempty"`
	AgentRunID          *uuid.UUID      `json:"agent_run_id,omitempty"`
	Status              string          `json:"status"`
	RootCauseSummary    string          `json:"root_cause_summary" binding:"required"`
	RootCauseCategory   string          `json:"root_cause_category"`
	ContributingFactors json.RawMessage `json:"contributing_factors"`
	FiveWhys            json.RawMessage `json:"five_whys"`
	Confidence          float64         `json:"confidence"`
	EvidenceIDs         json.RawMessage `json:"evidence_ids"`
}

// IncidentPostmortem represents a postmortem report.
type IncidentPostmortem struct {
	ID                        uuid.UUID       `json:"id"`
	IncidentID                uuid.UUID       `json:"incident_id"`
	RootCausewayD             *uuid.UUID      `json:"rca_id,omitempty"`
	AgentRunID                *uuid.UUID      `json:"agent_run_id,omitempty"`
	Status                    string          `json:"status"`
	Title                     string          `json:"title"`
	ExecutiveSummary          string          `json:"executive_summary"`
	IncidentTimelineNarrative string          `json:"incident_timeline_narrative"`
	RootCauseDetail           string          `json:"root_cause_detail"`
	ImpactDetail              string          `json:"impact_detail"`
	LessonsLearned            json.RawMessage `json:"lessons_learned"`
	ActionItems               json.RawMessage `json:"action_items"`
	WhatWentWell              json.RawMessage `json:"what_went_well"`
	WhatWentWrong             json.RawMessage `json:"what_went_wrong"`
	PreventionMeasures        json.RawMessage `json:"prevention_measures"`
	CreatedAt                 time.Time       `json:"created_at"`
	UpdatedAt                 time.Time       `json:"updated_at"`
	PublishedAt               *time.Time      `json:"published_at,omitempty"`
}

// CreatePostmortemRequest is the request body for creating a postmortem.
type CreatePostmortemRequest struct {
	RootCausewayD             *uuid.UUID      `json:"rca_id,omitempty"`
	AgentRunID                *uuid.UUID      `json:"agent_run_id,omitempty"`
	Status                    string          `json:"status"`
	Title                     string          `json:"title"`
	ExecutiveSummary          string          `json:"executive_summary"`
	IncidentTimelineNarrative string          `json:"incident_timeline_narrative"`
	RootCauseDetail           string          `json:"root_cause_detail"`
	ImpactDetail              string          `json:"impact_detail"`
	LessonsLearned            json.RawMessage `json:"lessons_learned"`
	ActionItems               json.RawMessage `json:"action_items"`
	WhatWentWell              json.RawMessage `json:"what_went_well"`
	WhatWentWrong             json.RawMessage `json:"what_went_wrong"`
	PreventionMeasures        json.RawMessage `json:"prevention_measures"`
}

// IncidentFull represents an incident with all cockpit data.
type IncidentFull struct {
	Incident `json:",inline"`
	Software *SoftwareEntry      `json:"software,omitempty"`
	DAG      []AgentRun          `json:"agent_runs"`
	Evidence []IncidentEvidence  `json:"evidence"`
	RCIData  *IncidentRCI        `json:"rci"`
	RCAData  *IncidentRCA        `json:"rca"`
	PMData   *IncidentPostmortem `json:"postmortem"`
}

// --- A2A Models ---

// A2AAgent represents an A2A-compatible agent registration.
type A2AAgent struct {
	ID                 uuid.UUID       `json:"id"`
	OrgID              uuid.UUID       `json:"org_id"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	AgentType          string          `json:"agent_type"`
	EndpointURL        string          `json:"endpoint_url"`
	AgentCard          json.RawMessage `json:"agent_card"`
	Skills             json.RawMessage `json:"skills"`
	AllowedSoftwareIDs json.RawMessage `json:"allowed_software_ids"`
	AuthType           string          `json:"auth_type"`
	AuthCredentials    string          `json:"auth_credentials,omitempty"`
	Enabled            bool            `json:"enabled"`
	IsSystem           bool            `json:"is_system"`
	HostingType        string          `json:"hosting_type"`              // "managed" | "byoa"
	ManagedConfig      json.RawMessage `json:"managed_config,omitempty"`  // for managed: image, resources, replicas
	LLMProvider        string          `json:"llm_provider,omitempty"`    // "platform" | "custom"
	LLMAPIKeyRef       string          `json:"llm_api_key_ref,omitempty"` // reference to credential store (for BYOA)
	AutoScale          bool            `json:"auto_scale"`
	MinReplicas        int             `json:"min_replicas"`
	MaxReplicas        int             `json:"max_replicas"`
	HealthStatus       string          `json:"health_status"`
	LastHealthCheck    *time.Time      `json:"last_health_check,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// CreateA2AAgentRequest is the request body for creating an A2A agent.
type CreateA2AAgentRequest struct {
	Name               string          `json:"name" binding:"required"`
	Description        string          `json:"description"`
	AgentType          string          `json:"agent_type" binding:"required"`
	EndpointURL        string          `json:"endpoint_url"`
	AgentCard          json.RawMessage `json:"agent_card"`
	Skills             json.RawMessage `json:"skills"`
	AllowedSoftwareIDs json.RawMessage `json:"allowed_software_ids"`
	AuthType           string          `json:"auth_type"`
	AuthCredentials    string          `json:"auth_credentials"`
	HostingType        string          `json:"hosting_type"`
	// ManagedConfig holds per-agent overrides for a managed agent, e.g.
	// {"model": "...", "temperature": 0.5} to override the org's default
	// LLM provider/model (see migration 023_llm_settings.up.sql) for just
	// this agent. Set via the Agents page's edit form.
	ManagedConfig json.RawMessage `json:"managed_config"`
	LLMProvider   string          `json:"llm_provider"`
	LLMAPIKeyRef  string          `json:"llm_api_key_ref"`
	AutoScale     bool            `json:"auto_scale"`
	MinReplicas   int             `json:"min_replicas"`
	MaxReplicas   int             `json:"max_replicas"`
}

// A2ATask represents a task dispatched to an A2A agent.
type A2ATask struct {
	ID                    uuid.UUID       `json:"id"`
	IncidentID            uuid.UUID       `json:"incident_id"`
	AgentID               uuid.UUID       `json:"agent_id"`
	AgentRunID            *uuid.UUID      `json:"agent_run_id,omitempty"`
	TaskType              string          `json:"task_type"`
	Status                string          `json:"status"`
	InputMessage          json.RawMessage `json:"input_message"`
	OutputArtifacts       json.RawMessage `json:"output_artifacts"`
	ErrorMessage          string          `json:"error_message"`
	OrchestratorReasoning string          `json:"orchestrator_reasoning"`
	Priority              int             `json:"priority"`
	DependsOn             *uuid.UUID      `json:"depends_on,omitempty"`
	SubmittedAt           *time.Time      `json:"submitted_at,omitempty"`
	StartedAt             *time.Time      `json:"started_at,omitempty"`
	CompletedAt           *time.Time      `json:"completed_at,omitempty"`
	CreatedAt             time.Time       `json:"created_at"`
}

// CreateA2ATaskRequest is the request body for creating an A2A task.
type CreateA2ATaskRequest struct {
	AgentID      uuid.UUID       `json:"agent_id" binding:"required"`
	TaskType     string          `json:"task_type" binding:"required"`
	InputMessage json.RawMessage `json:"input_message"`
	Priority     int             `json:"priority"`
	DependsOn    *uuid.UUID      `json:"depends_on,omitempty"`
}

// UpdateA2ATaskRequest is the request body for updating an A2A task.
type UpdateA2ATaskRequest struct {
	Status          *string         `json:"status,omitempty"`
	OutputArtifacts json.RawMessage `json:"output_artifacts,omitempty"`
	ErrorMessage    *string         `json:"error_message,omitempty"`
}

// --- Skills & Credentials Models ---

// Skill represents a reusable skill definition.
type Skill struct {
	ID                    uuid.UUID       `json:"id"`
	OrgID                 uuid.UUID       `json:"org_id"`
	Name                  string          `json:"name"`
	Slug                  string          `json:"slug"`
	Description           string          `json:"description"`
	Category              string          `json:"category"`
	PromptTemplate        string          `json:"prompt_template"`
	InputSchema           json.RawMessage `json:"input_schema"`
	OutputSchema          json.RawMessage `json:"output_schema"`
	RequiredTools         json.RawMessage `json:"required_tools"`
	RequiredResourceTypes json.RawMessage `json:"required_resource_types"`
	RequiredPermissions   json.RawMessage `json:"required_permissions"`
	Enabled               bool            `json:"enabled"`
	// Agents lists the A2A agents this skill is linked to (via the
	// agent_skills join table), populated by PgSkillRepository.List. This
	// is what agent-service's Orchestrator._discover_skills needs to
	// dispatch a skill at all -- until this field existed, ListSkills
	// returned skills with no way to resolve an agent_url, and a
	// non-empty skill list (even one skill, unlinked) silently bypassed
	// the working agent-card-based discovery fallback for the whole org.
	Agents    []SkillAgentRef `json:"agents,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// SkillAgentRef is the minimal agent info a skill needs to be dispatchable
// by the orchestrator -- same shape as the agent-card-discovery fallback
// path produces (Orchestrator._skills_from_agents), so agent-service can
// treat both sources uniformly.
type SkillAgentRef struct {
	ID            uuid.UUID       `json:"id"`
	URL           string          `json:"url"`
	Name          string          `json:"name"`
	HostingType   string          `json:"hosting_type"`
	LLMProvider   string          `json:"llm_provider"`
	ManagedConfig json.RawMessage `json:"managed_config"`
}

// CreateSkillRequest is the request body for creating a skill, and (reused,
// see UpdateSkill) for updating one.
type CreateSkillRequest struct {
	Name                  string          `json:"name" binding:"required"`
	Slug                  string          `json:"slug" binding:"required"`
	Description           string          `json:"description"`
	Category              string          `json:"category"`
	PromptTemplate        string          `json:"prompt_template"`
	InputSchema           json.RawMessage `json:"input_schema"`
	OutputSchema          json.RawMessage `json:"output_schema"`
	RequiredTools         json.RawMessage `json:"required_tools"`
	RequiredResourceTypes json.RawMessage `json:"required_resource_types"`
	RequiredPermissions   json.RawMessage `json:"required_permissions"`
	// Enabled is a pointer so "omitted" (leave as-is) is distinguishable
	// from "explicitly false" -- SkillsPage's enable/disable toggle is the
	// only caller that sends this without also sending every other field.
	Enabled *bool `json:"enabled,omitempty"`
}

// AgentSkillLink represents a link between an agent and a skill.
type AgentSkillLink struct {
	ID              uuid.UUID       `json:"id"`
	AgentID         uuid.UUID       `json:"agent_id"`
	SkillID         uuid.UUID       `json:"skill_id"`
	Priority        int             `json:"priority"`
	ConfigOverrides json.RawMessage `json:"config_overrides"`
	CreatedAt       time.Time       `json:"created_at"`
}

// CreateAgentSkillLinkRequest is the request body for linking a skill to an agent.
type CreateAgentSkillLinkRequest struct {
	SkillID         uuid.UUID       `json:"skill_id" binding:"required"`
	Priority        int             `json:"priority"`
	ConfigOverrides json.RawMessage `json:"config_overrides"`
}

// CredentialProvider represents a credential provider (e.g. Vault, AWS STS).
type CredentialProvider struct {
	ID           uuid.UUID       `json:"id"`
	OrgID        uuid.UUID       `json:"org_id"`
	Name         string          `json:"name"`
	ProviderType string          `json:"provider_type"`
	Config       json.RawMessage `json:"config"`
	Enabled      bool            `json:"enabled"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// CreateCredentialProviderRequest is the request body for creating a credential provider.
type CreateCredentialProviderRequest struct {
	Name         string          `json:"name" binding:"required"`
	ProviderType string          `json:"provider_type" binding:"required"`
	Config       json.RawMessage `json:"config"`
}

// ResourceCredential represents a credential binding for a software resource.
type ResourceCredential struct {
	ID             uuid.UUID       `json:"id"`
	OrgID          uuid.UUID       `json:"org_id"`
	SoftwareID     uuid.UUID       `json:"software_id"`
	ResourceName   string          `json:"resource_name"`
	ResourceType   string          `json:"resource_type"`
	ProviderID     uuid.UUID       `json:"provider_id"`
	CredentialPath string          `json:"credential_path"`
	DefaultTTL     int             `json:"default_ttl"`
	MaxTTL         int             `json:"max_ttl"`
	DefaultScope   json.RawMessage `json:"default_scope"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// CreateResourceCredentialRequest is the request body for creating a resource credential.
type CreateResourceCredentialRequest struct {
	SoftwareID     uuid.UUID       `json:"software_id" binding:"required"`
	ResourceName   string          `json:"resource_name" binding:"required"`
	ResourceType   string          `json:"resource_type" binding:"required"`
	ProviderID     uuid.UUID       `json:"provider_id" binding:"required"`
	CredentialPath string          `json:"credential_path"`
	DefaultTTL     int             `json:"default_ttl"`
	MaxTTL         int             `json:"max_ttl"`
	DefaultScope   json.RawMessage `json:"default_scope"`
	Metadata       json.RawMessage `json:"metadata"`
}

// AccessPolicy represents an access control policy.
type AccessPolicy struct {
	ID                uuid.UUID       `json:"id"`
	OrgID             uuid.UUID       `json:"org_id"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	TargetType        string          `json:"target_type"`
	TargetID          uuid.UUID       `json:"target_id"`
	ResourceType      string          `json:"resource_type"`
	AllowedActions    json.RawMessage `json:"allowed_actions"`
	ScopeRestrictions json.RawMessage `json:"scope_restrictions"`
	MaxTTL            int             `json:"max_ttl"`
	RequireApproval   bool            `json:"require_approval"`
	Enabled           bool            `json:"enabled"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

// CreateAccessPolicyRequest is the request body for creating an access policy.
type CreateAccessPolicyRequest struct {
	Name              string          `json:"name" binding:"required"`
	Description       string          `json:"description"`
	TargetType        string          `json:"target_type" binding:"required"`
	TargetID          uuid.UUID       `json:"target_id" binding:"required"`
	ResourceType      string          `json:"resource_type" binding:"required"`
	AllowedActions    json.RawMessage `json:"allowed_actions"`
	ScopeRestrictions json.RawMessage `json:"scope_restrictions"`
	MaxTTL            int             `json:"max_ttl"`
	RequireApproval   bool            `json:"require_approval"`
}

// CredentialLease represents a JIT credential lease.
type CredentialLease struct {
	ID                   uuid.UUID       `json:"id"`
	OrgID                uuid.UUID       `json:"org_id"`
	IncidentID           uuid.UUID       `json:"incident_id"`
	AgentID              uuid.UUID       `json:"agent_id"`
	SkillID              *uuid.UUID      `json:"skill_id,omitempty"`
	ResourceCredentialID uuid.UUID       `json:"resource_credential_id"`
	PolicyID             *uuid.UUID      `json:"policy_id,omitempty"`
	Status               string          `json:"status"`
	Scope                json.RawMessage `json:"scope"`
	IssuedAt             *time.Time      `json:"issued_at,omitempty"`
	ExpiresAt            *time.Time      `json:"expires_at,omitempty"`
	RevokedAt            *time.Time      `json:"revoked_at,omitempty"`
	RevokedBy            string          `json:"revoked_by"`
	RequestReason        string          `json:"request_reason"`
	ActionsPerformed     json.RawMessage `json:"actions_performed"`
	CreatedAt            time.Time       `json:"created_at"`
	// CredentialData is the resolved secret material -- generated fresh on
	// each RequestLease call, returned once in that response, and
	// deliberately NEVER persisted (no matching column on credential_leases;
	// only this struct field). Storing live secrets in the audit trail
	// would defeat the point of leasing them short-lived in the first
	// place. Omitted from JSON entirely (not even as null) on any other
	// response that reuses this struct (ListLeases, GetByID, etc), where
	// it's never populated.
	CredentialData map[string]interface{} `json:"credential_data,omitempty"`
}

// RequestLeaseRequest is the request body for requesting a credential lease.
type RequestLeaseRequest struct {
	IncidentID           uuid.UUID       `json:"incident_id" binding:"required"`
	AgentID              uuid.UUID       `json:"agent_id" binding:"required"`
	SkillID              *uuid.UUID      `json:"skill_id,omitempty"`
	ResourceCredentialID uuid.UUID       `json:"resource_credential_id" binding:"required"`
	TTLSeconds           int             `json:"ttl_seconds"`
	Scope                json.RawMessage `json:"scope"`
	Reason               string          `json:"reason"`
}

// EvaluatePolicyRequest is the request body for evaluating access policies.
type EvaluatePolicyRequest struct {
	AgentID      uuid.UUID `json:"agent_id" binding:"required"`
	SkillID      uuid.UUID `json:"skill_id"`
	ResourceType string    `json:"resource_type" binding:"required"`
}

// OrchestratorDecision represents a decision made by the orchestrator.
type OrchestratorDecision struct {
	ID             uuid.UUID       `json:"id"`
	IncidentID     uuid.UUID       `json:"incident_id"`
	DecisionType   string          `json:"decision_type"`
	Reasoning      string          `json:"reasoning"`
	SelectedAgents json.RawMessage `json:"selected_agents"`
	ContextUsed    json.RawMessage `json:"context_used"`
	Confidence     float64         `json:"confidence"`
	CreatedAt      time.Time       `json:"created_at"`
}

// CreateOrchestratorDecisionRequest is the request body for creating an orchestrator decision.
type CreateOrchestratorDecisionRequest struct {
	DecisionType   string          `json:"decision_type" binding:"required"`
	Reasoning      string          `json:"reasoning"`
	SelectedAgents json.RawMessage `json:"selected_agents"`
	ContextUsed    json.RawMessage `json:"context_used"`
	Confidence     float64         `json:"confidence"`
}
