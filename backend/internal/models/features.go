package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Loop Engineering: Feedback ---

type IncidentFeedback struct {
	ID            uuid.UUID       `json:"id"`
	IncidentID    uuid.UUID       `json:"incident_id"`
	UserID        *uuid.UUID      `json:"user_id,omitempty"`
	TargetType    string          `json:"target_type"`
	Rating        string          `json:"rating"`
	Correction    string          `json:"correction"`
	OriginalData  json.RawMessage `json:"original_data"`
	CorrectedData json.RawMessage `json:"corrected_data"`
	CreatedAt     time.Time       `json:"created_at"`
}

type CreateFeedbackRequest struct {
	TargetType    string          `json:"target_type" binding:"required"`
	Rating        string          `json:"rating" binding:"required"`
	Correction    string          `json:"correction"`
	OriginalData  json.RawMessage `json:"original_data"`
	CorrectedData json.RawMessage `json:"corrected_data"`
}

// --- Loop Engineering: Knowledge Base ---

type KnowledgeBaseEntry struct {
	ID                uuid.UUID       `json:"id"`
	OrgID             uuid.UUID       `json:"org_id"`
	IncidentID        *uuid.UUID      `json:"incident_id,omitempty"`
	SoftwareID        *uuid.UUID      `json:"software_id,omitempty"`
	Category          string          `json:"category"`
	ErrorPattern      string          `json:"error_pattern"`
	RootCauseSummary  string          `json:"root_cause_summary"`
	ResolutionSummary string          `json:"resolution_summary"`
	LessonsLearned    json.RawMessage `json:"lessons_learned"`
	ActionItems       json.RawMessage `json:"action_items"`
	Tags              json.RawMessage `json:"tags"`
	HumanValidated    bool            `json:"human_validated"`
	Confidence        float64         `json:"confidence"`
	TimesReferenced   int             `json:"times_referenced"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	// Similarity is the cosine similarity (0-1) to the search query.
	// Only set on results from semantic (embedding-based) search.
	Similarity *float64 `json:"similarity,omitempty"`
}

type CreateKnowledgeBaseRequest struct {
	IncidentID        *uuid.UUID      `json:"incident_id,omitempty"`
	SoftwareID        *uuid.UUID      `json:"software_id,omitempty"`
	Category          string          `json:"category"`
	ErrorPattern      string          `json:"error_pattern"`
	RootCauseSummary  string          `json:"root_cause_summary" binding:"required"`
	ResolutionSummary string          `json:"resolution_summary"`
	LessonsLearned    json.RawMessage `json:"lessons_learned"`
	ActionItems       json.RawMessage `json:"action_items"`
	Tags              json.RawMessage `json:"tags"`
}

type KnowledgeBaseSearchRequest struct {
	// SearchKnowledgeBase binds this from a JSON body on POST, or manually
	// from query params on GET (gin's form binder can't decode *uuid.UUID
	// from a query string, so ShouldBind(form) isn't used here) -- see
	// that handler for why both request shapes exist.
	SoftwareID   *uuid.UUID `json:"software_id,omitempty"`
	ErrorPattern string     `json:"error_pattern"`
	// Query is free-text used for semantic search; falls back to ErrorPattern when empty.
	Query string `json:"query,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// --- Loop Engineering: Similar Incidents ---

type SimilarIncident struct {
	ID                uuid.UUID       `json:"id"`
	IncidentID        uuid.UUID       `json:"incident_id"`
	SimilarIncidentID uuid.UUID       `json:"similar_incident_id"`
	SimilarityScore   float64         `json:"similarity_score"`
	MatchedOn         json.RawMessage `json:"matched_on"`
	CreatedAt         time.Time       `json:"created_at"`
}

type CreateSimilarIncidentRequest struct {
	SimilarIncidentID uuid.UUID       `json:"similar_incident_id" binding:"required"`
	SimilarityScore   float64         `json:"similarity_score" binding:"required"`
	MatchedOn         json.RawMessage `json:"matched_on"`
}

// SimilarIncidentMatch is an embedding-based nearest-neighbor incident match.
type SimilarIncidentMatch struct {
	IncidentID uuid.UUID `json:"incident_id"`
	Title      string    `json:"title"`
	Severity   string    `json:"severity"`
	Status     string    `json:"status"`
	Similarity float64   `json:"similarity"`
	CreatedAt  time.Time `json:"created_at"`
}

// --- Correlation ---

type CorrelationRule struct {
	ID                uuid.UUID       `json:"id"`
	OrgID             uuid.UUID       `json:"org_id"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	RuleType          string          `json:"rule_type"`
	Config            json.RawMessage `json:"config"`
	TimeWindowSeconds int             `json:"time_window_seconds"`
	Enabled           bool            `json:"enabled"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type CreateCorrelationRuleRequest struct {
	Name              string          `json:"name" binding:"required"`
	Description       string          `json:"description"`
	RuleType          string          `json:"rule_type" binding:"required"`
	Config            json.RawMessage `json:"config"`
	TimeWindowSeconds int             `json:"time_window_seconds"`
}

type AlertGroup struct {
	ID                uuid.UUID  `json:"id"`
	IncidentID        uuid.UUID  `json:"incident_id"`
	AlertSnapshotID   uuid.UUID  `json:"alert_snapshot_id"`
	CorrelationRuleID *uuid.UUID `json:"correlation_rule_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

type CreateAlertGroupRequest struct {
	AlertSnapshotID   uuid.UUID  `json:"alert_snapshot_id" binding:"required"`
	CorrelationRuleID *uuid.UUID `json:"correlation_rule_id,omitempty"`
}

// --- Notifications ---

type NotificationChannel struct {
	ID          uuid.UUID       `json:"id"`
	OrgID       uuid.UUID       `json:"org_id"`
	Name        string          `json:"name"`
	ChannelType string          `json:"channel_type"`
	Config      json.RawMessage `json:"config"`
	Enabled     bool            `json:"enabled"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type CreateNotificationChannelRequest struct {
	Name        string          `json:"name" binding:"required"`
	ChannelType string          `json:"channel_type" binding:"required"`
	Config      json.RawMessage `json:"config"`
}

type EscalationPolicy struct {
	ID                 uuid.UUID       `json:"id"`
	OrgID              uuid.UUID       `json:"org_id"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	SoftwareID         *uuid.UUID      `json:"software_id,omitempty"`
	SeverityFilter     json.RawMessage `json:"severity_filter"`
	Steps              json.RawMessage `json:"steps"`
	RepeatAfterSeconds *int            `json:"repeat_after_seconds,omitempty"`
	Enabled            bool            `json:"enabled"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type CreateEscalationPolicyRequest struct {
	Name               string          `json:"name" binding:"required"`
	Description        string          `json:"description"`
	SoftwareID         *uuid.UUID      `json:"software_id,omitempty"`
	SeverityFilter     json.RawMessage `json:"severity_filter"`
	Steps              json.RawMessage `json:"steps"`
	RepeatAfterSeconds *int            `json:"repeat_after_seconds,omitempty"`
}

type NotificationLogEntry struct {
	ID           uuid.UUID       `json:"id"`
	OrgID        uuid.UUID       `json:"org_id"`
	IncidentID   *uuid.UUID      `json:"incident_id,omitempty"`
	ChannelID    *uuid.UUID      `json:"channel_id,omitempty"`
	PolicyID     *uuid.UUID      `json:"policy_id,omitempty"`
	EventType    string          `json:"event_type"`
	Recipient    string          `json:"recipient"`
	Payload      json.RawMessage `json:"payload"`
	Status       string          `json:"status"`
	ErrorMessage string          `json:"error_message"`
	SentAt       *time.Time      `json:"sent_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

type CreateNotificationLogRequest struct {
	IncidentID *uuid.UUID      `json:"incident_id,omitempty"`
	ChannelID  *uuid.UUID      `json:"channel_id,omitempty"`
	PolicyID   *uuid.UUID      `json:"policy_id,omitempty"`
	EventType  string          `json:"event_type" binding:"required"`
	Recipient  string          `json:"recipient"`
	Payload    json.RawMessage `json:"payload"`
}

// --- Notification Interactivity (bidirectional Slack/Teams) ---
//
// NotificationChannel.Config is a free-form JSONB blob (see above), so no
// column changes are needed to support these fields -- they're simply
// additional keys an operator can set on a slack/teams channel's config.
// SlackChannelConfig and TeamsChannelConfig document/parse those additive
// keys; see notification_interactive_handlers.go for how they're used to
// verify inbound requests to /webhooks/slack/interactive and
// /webhooks/teams/interactive.

// SlackChannelConfig is the shape of NotificationChannel.Config for
// channel_type == "slack". SigningSecret is the Slack app's signing secret
// (from api.slack.com/apps -> Basic Information -> Signing Secret), used to
// verify the X-Slack-Signature header on inbound interactive payloads.
type SlackChannelConfig struct {
	WebhookURL    string `json:"webhook_url,omitempty"`
	SigningSecret string `json:"signing_secret,omitempty"`
}

// TeamsChannelConfig is the shape of NotificationChannel.Config for
// channel_type == "teams". VerificationToken is a shared secret RootCauseway expects
// to receive back (via Authorization: Bearer <token> or the
// X-Teams-Verification-Token header) with Action.Submit callbacks. This is a
// deliberately minimal verification scheme -- full Bot Framework JWT
// validation against Microsoft's OpenID configuration is out of scope here.
type TeamsChannelConfig struct {
	WebhookURL        string `json:"webhook_url,omitempty"`
	VerificationToken string `json:"verification_token,omitempty"`
}

// NotificationInteraction records a single inbound action (acknowledge,
// resolve, view_rca) triggered by a user clicking a button in a Slack
// message or Teams Adaptive Card.
type NotificationInteraction struct {
	ID           uuid.UUID  `json:"id"`
	OrgID        uuid.UUID  `json:"org_id"`
	IncidentID   uuid.UUID  `json:"incident_id"`
	ChannelID    *uuid.UUID `json:"channel_id,omitempty"`
	ChannelType  string     `json:"channel_type"`
	Action       string     `json:"action"`
	Actor        string     `json:"actor"`
	RequestToken string     `json:"request_token,omitempty"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// --- Runbooks ---

type Runbook struct {
	ID                uuid.UUID       `json:"id"`
	OrgID             uuid.UUID       `json:"org_id"`
	SoftwareID        *uuid.UUID      `json:"software_id,omitempty"`
	Name              string          `json:"name"`
	Slug              string          `json:"slug"`
	Description       string          `json:"description"`
	TriggerConditions json.RawMessage `json:"trigger_conditions"`
	AutoTrigger       bool            `json:"auto_trigger"`
	Enabled           bool            `json:"enabled"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
}

type CreateRunbookRequest struct {
	SoftwareID        *uuid.UUID      `json:"software_id,omitempty"`
	Name              string          `json:"name" binding:"required"`
	Slug              string          `json:"slug" binding:"required"`
	Description       string          `json:"description"`
	TriggerConditions json.RawMessage `json:"trigger_conditions"`
	AutoTrigger       bool            `json:"auto_trigger"`
}

type RunbookStep struct {
	ID             uuid.UUID       `json:"id"`
	RunbookID      uuid.UUID       `json:"runbook_id"`
	StepOrder      int             `json:"step_order"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	StepType       string          `json:"step_type"`
	Config         json.RawMessage `json:"config"`
	SkillID        *uuid.UUID      `json:"skill_id,omitempty"`
	TimeoutSeconds int             `json:"timeout_seconds"`
	OnFailure      string          `json:"on_failure"`
	MaxRetries     int             `json:"max_retries"`
	CreatedAt      time.Time       `json:"created_at"`
}

type CreateRunbookStepRequest struct {
	StepOrder      int             `json:"step_order" binding:"required"`
	Name           string          `json:"name" binding:"required"`
	Description    string          `json:"description"`
	StepType       string          `json:"step_type" binding:"required"`
	Config         json.RawMessage `json:"config"`
	SkillID        *uuid.UUID      `json:"skill_id,omitempty"`
	TimeoutSeconds int             `json:"timeout_seconds"`
	OnFailure      string          `json:"on_failure"`
	MaxRetries     int             `json:"max_retries"`
}

// CompleteExecutionStepRequest is the optional body for
// POST /runbook-executions/:execId/steps/:stepId/complete. A human "Mark
// Complete" click sends no body (Status defaults to "completed" in the
// handler); RunbookExecutor's automation loop sends an explicit status
// ("completed" or "failed") plus the automated step's result payload.
type CompleteExecutionStepRequest struct {
	Status string                 `json:"status,omitempty"`
	Output map[string]interface{} `json:"output,omitempty"`
}

// ReorderRunbookStepsRequest matches frontend/src/services/api.ts's
// reorderRunbookSteps -- step_ids in the desired display order.
type ReorderRunbookStepsRequest struct {
	StepIDs []string `json:"step_ids" binding:"required"`
}

type RunbookExecution struct {
	ID          uuid.UUID       `json:"id"`
	RunbookID   uuid.UUID       `json:"runbook_id"`
	IncidentID  *uuid.UUID      `json:"incident_id,omitempty"`
	TriggeredBy string          `json:"triggered_by"`
	Status      string          `json:"status"`
	CurrentStep int             `json:"current_step"`
	StepResults json.RawMessage `json:"step_results"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

type CreateRunbookExecutionRequest struct {
	RunbookID   uuid.UUID  `json:"runbook_id" binding:"required"`
	IncidentID  *uuid.UUID `json:"incident_id,omitempty"`
	TriggeredBy string     `json:"triggered_by"`
}

type UpdateRunbookExecutionRequest struct {
	Status      *string         `json:"status,omitempty"`
	CurrentStep *int            `json:"current_step,omitempty"`
	StepResults json.RawMessage `json:"step_results,omitempty"`
}

// ExecutionStepResult represents the status of an individual step in a runbook execution.
type ExecutionStepResult struct {
	StepID      string                 `json:"step_id"`
	StepName    string                 `json:"step_name"`
	StepType    string                 `json:"step_type"`
	Status      string                 `json:"status"`
	Output      map[string]interface{} `json:"output,omitempty"`
	StartedAt   *string                `json:"started_at,omitempty"`
	CompletedAt *string                `json:"completed_at,omitempty"`
}

// --- Change Events ---

type ChangeEvent struct {
	ID          uuid.UUID       `json:"id"`
	OrgID       uuid.UUID       `json:"org_id"`
	SoftwareID  *uuid.UUID      `json:"software_id,omitempty"`
	ChangeType  string          `json:"change_type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Source      string          `json:"source"`
	SourceURL   string          `json:"source_url"`
	CommitSHA   string          `json:"commit_sha"`
	Author      string          `json:"author"`
	Environment string          `json:"environment"`
	Metadata    json.RawMessage `json:"metadata"`
	OccurredAt  time.Time       `json:"occurred_at"`
	CreatedAt   time.Time       `json:"created_at"`
}

type CreateChangeEventRequest struct {
	SoftwareID  *uuid.UUID      `json:"software_id,omitempty"`
	ChangeType  string          `json:"change_type" binding:"required"`
	Title       string          `json:"title" binding:"required"`
	Description string          `json:"description"`
	Source      string          `json:"source"`
	SourceURL   string          `json:"source_url"`
	CommitSHA   string          `json:"commit_sha"`
	Author      string          `json:"author"`
	Environment string          `json:"environment"`
	Metadata    json.RawMessage `json:"metadata"`
	OccurredAt  *time.Time      `json:"occurred_at,omitempty"`
}

// --- Analytics ---

type AnalyticsMTTR struct {
	SoftwareID     uuid.UUID `json:"software_id"`
	SoftwareName   string    `json:"software_name"`
	AvgMTTRSeconds float64   `json:"avg_mttr_seconds"`
	IncidentCount  int       `json:"incident_count"`
	Period         string    `json:"period"`
}

type AnalyticsIncidentTrend struct {
	Date     string `json:"date"`
	Count    int    `json:"count"`
	Severity string `json:"severity"`
}

type AnalyticsAgentEffectiveness struct {
	AgentName     string  `json:"agent_name"`
	TotalTasks    int     `json:"total_tasks"`
	SuccessRate   float64 `json:"success_rate"`
	AvgDurationMs float64 `json:"avg_duration_ms"`
}

type AnalyticsCostByModel struct {
	Model       string  `json:"model"`
	TotalRuns   int     `json:"total_runs"`
	TotalTokens int     `json:"total_tokens"`
	EstCostUSD  float64 `json:"est_cost_usd"`
}

type AnalyticsCostByIncident struct {
	IncidentID      uuid.UUID `json:"incident_id"`
	IncidentTitle   string    `json:"incident_title"`
	TotalRuns       int       `json:"total_runs"`
	TotalTokens     int       `json:"total_tokens"`
	EstCostUSD      float64   `json:"est_cost_usd"`
	TotalDurationMs int       `json:"total_duration_ms"`
	CreatedAt       time.Time `json:"created_at"`
}

// --- Correlation Check ---

// CorrelationCheckRequest matches the payload agent-service's
// CorrelationEngine._check_same_service actually sends (see
// BackendClient.check_correlation) -- an in-flight alert being evaluated
// for correlation, not yet persisted as an AlertSnapshot, so there's no
// alert_snapshot_id to reference at this point.
type CorrelationCheckRequest struct {
	SoftwareID        uuid.UUID      `json:"software_id" binding:"required"`
	Alert             map[string]any `json:"alert"`
	TimeWindowSeconds int            `json:"time_window_seconds"`
	// ExcludeIncidentID is the incident IngestAlert already created for THIS
	// exact alert instance, before alert.received was even published --
	// without excluding it, every brand-new incident self-correlates against
	// itself (it's trivially "an open incident on this software_id within
	// the window") and the pipeline never runs for it. See
	// CorrelationCheck's doc comment for the live-found bug this closes.
	ExcludeIncidentID *uuid.UUID `json:"exclude_incident_id,omitempty"`
}

type CorrelationCheckResponse struct {
	Correlated bool       `json:"correlated"`
	IncidentID *uuid.UUID `json:"incident_id,omitempty"`
	RuleID     *uuid.UUID `json:"rule_id,omitempty"`
	RuleName   string     `json:"rule_name,omitempty"`
}
