package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EventEnvelope wraps all Redis pub/sub events.
type EventEnvelope struct {
	EventID   uuid.UUID   `json:"event_id"`
	EventType string      `json:"event_type"`
	OrgID     uuid.UUID   `json:"org_id"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}

// AlertReceivedPayload is published when a new alert is ingested.
type AlertReceivedPayload struct {
	AlertSnapshotID uuid.UUID       `json:"alert_snapshot_id"`
	IncidentID      uuid.UUID       `json:"incident_id"`
	SoftwareID      uuid.UUID       `json:"software_id"`
	WebhookSource   string          `json:"webhook_source"`
	NormalizedAlert NormalizedAlert  `json:"normalized_alert"`
}

// IncidentCreatedPayload is published (channel "incident.created") right
// after IngestAlert commits a new incident row. Drives the WebSocket bridge
// (internal/ws/redis_bridge.go) -> frontend's "new incident" toast on
// IncidentsPage/DashboardPage -- found live never firing because nothing
// published this event type at all (the Go ws.EventEmitter that was meant
// to was built but never wired into any handler).
type IncidentCreatedPayload struct {
	IncidentID uuid.UUID `json:"incident_id"`
	Title      string    `json:"title"`
	Severity   string    `json:"severity"`
	Status     string    `json:"status"`
	SoftwareID uuid.UUID `json:"software_id"`
}

// IncidentUpdatedPayload is published (channel "incident.updated") on every
// successful UpdateIncident call -- distinct from the existing
// "incident.resolved" publish in the same handler, which only fires on the
// terminal transition and exists to trigger postmortem generation, not to
// drive the UI's live incident list/dashboard refresh.
type IncidentUpdatedPayload struct {
	IncidentID uuid.UUID `json:"incident_id"`
	Title      string    `json:"title"`
	Severity   string    `json:"severity"`
	Status     string    `json:"status"`
}

// TriageResult holds the output of the triage agent.
type TriageResult struct {
	SeverityAssessment string     `json:"severity_assessment"`
	Category           string     `json:"category"`
	AffectedComponents []string   `json:"affected_components"`
	SuggestedAssigneeID *uuid.UUID `json:"suggested_assignee_id,omitempty"`
	Summary            string     `json:"summary"`
	Confidence         float64    `json:"confidence"`
}

// TriageCompletedPayload is published when triage finishes.
type TriageCompletedPayload struct {
	IncidentID   uuid.UUID    `json:"incident_id"`
	TriageResult TriageResult `json:"triage_result"`
}

// EvidencePayload describes a piece of collected evidence.
type EvidencePayload struct {
	Type    string          `json:"type"`
	Title   string          `json:"title"`
	Content json.RawMessage `json:"content"`
	Source  string          `json:"source"`
}

// EvidenceCollectedPayload is published when evidence is collected.
type EvidenceCollectedPayload struct {
	IncidentID uuid.UUID       `json:"incident_id"`
	Evidence   EvidencePayload `json:"evidence"`
}

// Hypothesis represents a root cause hypothesis.
type Hypothesis struct {
	RootCause          string      `json:"root_cause"`
	Confidence         float64     `json:"confidence"`
	SupportingEvidence []uuid.UUID `json:"supporting_evidence"`
	RecommendedActions []string    `json:"recommended_actions"`
	MitigationSteps    []string    `json:"mitigation_steps"`
}

// HypothesisGeneratedPayload is published when a hypothesis is generated.
type HypothesisGeneratedPayload struct {
	IncidentID uuid.UUID  `json:"incident_id"`
	Hypothesis Hypothesis `json:"hypothesis"`
}

// AgentStatusPayload is published when an agent's status changes.
type AgentStatusPayload struct {
	IncidentID uuid.UUID `json:"incident_id"`
	AgentID    uuid.UUID `json:"agent_id"`
	AgentName  string    `json:"agent_name"`
	Status     string    `json:"status"` // started, running, completed, failed
	Message    string    `json:"message"`
	Progress   float64   `json:"progress"`
}
