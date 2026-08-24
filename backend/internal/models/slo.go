package models

import (
	"time"

	"github.com/google/uuid"
)

// SLOType enumerates the kinds of Service Level Objectives supported.
const (
	SLOTypeAvailability = "availability"
	SLOTypeLatency      = "latency"
	SLOTypeErrorRate    = "error_rate"
)

// SLOStatusHealthy / AtRisk / Exhausted are the computed error-budget states.
const (
	SLOStatusHealthy   = "healthy"
	SLOStatusAtRisk    = "at_risk"
	SLOStatusExhausted = "exhausted"
)

// SLODefinition is a target (e.g. 99.9% availability) for a software
// service, measured over a rolling window of days.
type SLODefinition struct {
	ID                    uuid.UUID `json:"id"`
	OrgID                 uuid.UUID `json:"org_id"`
	SoftwareID            uuid.UUID `json:"software_id"`
	Name                  string    `json:"name"`
	SLOType               string    `json:"slo_type"`
	TargetPercentage      float64   `json:"target_percentage"`
	MeasurementWindowDays int       `json:"measurement_window_days"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// CreateSLODefinitionRequest is the payload for creating an SLO definition.
type CreateSLODefinitionRequest struct {
	SoftwareID            uuid.UUID `json:"software_id" binding:"required"`
	Name                  string    `json:"name" binding:"required"`
	SLOType               string    `json:"slo_type" binding:"required,oneof=availability latency error_rate"`
	TargetPercentage      float64   `json:"target_percentage" binding:"required,gt=0,lte=100"`
	MeasurementWindowDays int       `json:"measurement_window_days"`
}

// UpdateSLODefinitionRequest is the payload for updating an SLO definition.
// All fields are optional; only non-zero-ish fields are applied by the
// service layer (validated there, since binding:"required" isn't
// appropriate for a partial update).
type UpdateSLODefinitionRequest struct {
	Name                  string  `json:"name"`
	SLOType               string  `json:"slo_type" binding:"omitempty,oneof=availability latency error_rate"`
	TargetPercentage      float64 `json:"target_percentage" binding:"omitempty,gt=0,lte=100"`
	MeasurementWindowDays int     `json:"measurement_window_days"`
}

// SLOStatus is the computed, point-in-time error-budget status for a single
// SLO definition. See slo_service.go for the exact formulas.
type SLOStatus struct {
	SLODefinitionID  uuid.UUID `json:"slo_definition_id"`
	SoftwareID       uuid.UUID `json:"software_id"`
	SLOType          string    `json:"slo_type"`
	TargetPercentage float64   `json:"target_percentage"`

	// WindowStart/WindowEnd bound the measurement window this status was
	// computed over ([now - measurement_window_days, now]).
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`

	// CurrentPercentage is the observed attainment over the window.
	CurrentPercentage float64 `json:"current_percentage"`

	// ErrorBudgetTotalMinutes is how many minutes of "badness" (downtime for
	// availability; see doc comment on slo_service.go for latency/error_rate)
	// are allowed by the target over the window.
	ErrorBudgetTotalMinutes float64 `json:"error_budget_total_minutes"`
	// ErrorBudgetConsumedMinutes is how many of those minutes have been used.
	ErrorBudgetConsumedMinutes float64 `json:"error_budget_consumed_minutes"`
	// ErrorBudgetRemainingPercentage is (total-consumed)/total * 100, floored at 0.
	ErrorBudgetRemainingPercentage float64 `json:"error_budget_remaining_percentage"`

	// Status is healthy (>=20% budget remaining), at_risk (<20% remaining
	// but >0), or exhausted (<=0 remaining).
	Status string `json:"status"`

	IncidentCount int `json:"incident_count"`

	// IsApproximated is true for latency/error_rate SLO types, which are
	// derived from incident-duration data as a proxy in the absence of raw
	// request/latency metric ingestion. See slo_service.go for details.
	IsApproximated bool `json:"is_approximated"`
}

// SoftwareSLOStatus bundles all SLO definitions + computed statuses for a
// single software entry (used by GET /software/:id/slo-status).
type SoftwareSLOStatus struct {
	SoftwareID uuid.UUID   `json:"software_id"`
	SLOs       []SLOStatus `json:"slos"`
}
