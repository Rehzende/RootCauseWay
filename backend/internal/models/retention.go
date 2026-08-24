package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- Retention & Archival ---

// RetentionResourceType enumerates the resource families a retention policy
// can govern. Kept as plain strings (validated at the DB layer via CHECK
// constraints, and at the handler layer below) rather than a Go enum type,
// matching the convention used elsewhere in this package (see
// CorrelationRule.RuleType, Runbook step_type, etc).
const (
	RetentionResourceEvidence  = "evidence"
	RetentionResourceIncidents = "incidents"
	RetentionResourceAgentRuns = "agent_runs"
)

// RetentionAction enumerates what a sweep does with expired records.
const (
	RetentionActionArchive = "archive"
	RetentionActionDelete  = "delete"
)

// RetentionPolicy configures, per org and resource type, how long records
// are kept before a sweep archives or deletes them.
//
// Default policy guidance (v1 -- documented, not hardcoded): evidence 90
// days, closed incidents 365 days. These are sensible seed/example values
// for onboarding; every org configures its own policies via the CRUD
// endpoints below rather than the service enforcing a built-in default.
type RetentionPolicy struct {
	ID            uuid.UUID `json:"id"`
	OrgID         uuid.UUID `json:"org_id"`
	ResourceType  string    `json:"resource_type"`
	RetentionDays int       `json:"retention_days"`
	Action        string    `json:"action"`
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateRetentionPolicyRequest is the request body for creating a policy.
type CreateRetentionPolicyRequest struct {
	ResourceType  string `json:"resource_type" binding:"required,oneof=evidence incidents agent_runs"`
	RetentionDays int    `json:"retention_days" binding:"required,gt=0"`
	Action        string `json:"action" binding:"required,oneof=archive delete"`
	Enabled       *bool  `json:"enabled,omitempty"`
}

// UpdateRetentionPolicyRequest is the request body for updating a policy.
// All fields are optional; only provided fields are changed.
type UpdateRetentionPolicyRequest struct {
	RetentionDays *int    `json:"retention_days,omitempty" binding:"omitempty,gt=0"`
	Action        *string `json:"action,omitempty" binding:"omitempty,oneof=archive delete"`
	Enabled       *bool   `json:"enabled,omitempty"`
}

// ArchivedRecord is an append-only snapshot of a row removed by a retention
// sweep with action="archive". There is no update path -- only inserts (by
// the sweep) and reads (audit / future restore tooling).
type ArchivedRecord struct {
	ID           uuid.UUID       `json:"id"`
	OrgID        uuid.UUID       `json:"org_id"`
	ResourceType string          `json:"resource_type"`
	ResourceID   uuid.UUID       `json:"resource_id"`
	ArchivedData json.RawMessage `json:"archived_data"`
	ArchivedAt   time.Time       `json:"archived_at"`
}

// RetentionSweepResult summarizes what a sweep run did for a single policy.
type RetentionSweepResult struct {
	PolicyID      uuid.UUID `json:"policy_id"`
	ResourceType  string    `json:"resource_type"`
	Action        string    `json:"action"`
	MatchedCount  int       `json:"matched_count"`
	ArchivedCount int       `json:"archived_count"`
	DeletedCount  int       `json:"deleted_count"`
	Errors        []string  `json:"errors,omitempty"`
}

// RetentionSweepSummary is the response of a manual (or, later, scheduled)
// sweep trigger for a single org.
type RetentionSweepSummary struct {
	OrgID     uuid.UUID              `json:"org_id"`
	StartedAt time.Time              `json:"started_at"`
	Results   []RetentionSweepResult `json:"results"`
}
