package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// SoftwareSummarySoftwareProvider is satisfied by *services.SoftwareService.
type SoftwareSummarySoftwareProvider interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.SoftwareEntry, error)
}

// SoftwareSummarySLOProvider is satisfied by *database.PgSLORepository.
type SoftwareSummarySLOProvider interface {
	ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.SLODefinition, error)
}

// SoftwareSummaryEscalationProvider is satisfied by
// *database.PgEscalationPolicyRepository.
type SoftwareSummaryEscalationProvider interface {
	ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.EscalationPolicy, error)
}

// SoftwareSummaryIncidentProvider is satisfied by *database.PgIncidentRepository.
type SoftwareSummaryIncidentProvider interface {
	GetIncidentStatsBySoftware(ctx context.Context, orgID, softwareID uuid.UUID) (total, open int, lastIncidentAt *time.Time, err error)
}

// SoftwareSummaryHandler rolls up everything already tied to a software
// catalog entry (SLOs, escalation policies, incident history) plus a
// completeness score, onto a single response -- these all already exist as
// separate tables scoped by software_id, but the catalog page itself never
// surfaced any of it, so "is this service actually production-ready" meant
// checking 3+ different pages by hand.
type SoftwareSummaryHandler struct {
	Software   SoftwareSummarySoftwareProvider
	SLO        SoftwareSummarySLOProvider
	Escalation SoftwareSummaryEscalationProvider
	Incidents  SoftwareSummaryIncidentProvider
}

// SoftwareSummary is the response shape for GET /software/:id/summary.
type SoftwareSummary struct {
	SoftwareID            uuid.UUID  `json:"software_id"`
	CompletenessScore     int        `json:"completeness_score"`
	CompletenessTotal     int        `json:"completeness_total"`
	MissingChecks         []string   `json:"missing_checks"`
	SLOCount              int        `json:"slo_count"`
	EscalationPolicyCount int        `json:"escalation_policy_count"`
	TotalIncidents        int        `json:"total_incidents"`
	OpenIncidents         int        `json:"open_incidents"`
	LastIncidentAt        *time.Time `json:"last_incident_at,omitempty"`
}

func (h *SoftwareSummaryHandler) GetSoftwareSummary(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid software id"})
		return
	}

	entry, err := h.Software.GetByID(c.Request.Context(), id)
	if err != nil || entry.OrgID != getOrgID(c) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}

	// Best-effort: a failure to fetch SLOs/escalation policies shouldn't 500
	// the whole summary -- those sections just report as empty/unconfigured,
	// which is an accurate (if incomplete) completeness signal anyway.
	slos, _ := h.SLO.ListBySoftware(c.Request.Context(), id)
	escalations, _ := h.Escalation.ListBySoftware(c.Request.Context(), id)
	total, open, lastAt, _ := h.Incidents.GetIncidentStatsBySoftware(c.Request.Context(), entry.OrgID, id)

	checks := []struct {
		label string
		ok    bool
	}{
		{"description", entry.Description != ""},
		{"repository_url", entry.RepositoryURL != ""},
		{"runbook_url", entry.RunbookURL != ""},
		{"dashboard_url", entry.DashboardURL != ""},
		{"cloud_resources", hasJSONArrayItems(entry.CloudResources)},
		{"sre_team", hasJSONArrayItems(entry.SreTeam)},
		{"stakeholders", hasJSONArrayItems(entry.Stakeholders)},
		{"dependencies", hasJSONArrayItems(entry.Dependencies)},
		{"escalation_policy", len(escalations) > 0},
		{"slo", len(slos) > 0},
	}

	score := 0
	missing := make([]string, 0, len(checks))
	for _, ch := range checks {
		if ch.ok {
			score++
		} else {
			missing = append(missing, ch.label)
		}
	}

	c.JSON(http.StatusOK, SoftwareSummary{
		SoftwareID:            id,
		CompletenessScore:     score,
		CompletenessTotal:     len(checks),
		MissingChecks:         missing,
		SLOCount:              len(slos),
		EscalationPolicyCount: len(escalations),
		TotalIncidents:        total,
		OpenIncidents:         open,
		LastIncidentAt:        lastAt,
	})
}

// hasJSONArrayItems reports whether a json.RawMessage holds a non-empty
// JSON array. Used for the free-form array-shaped SoftwareEntry fields
// (cloud_resources, sre_team, stakeholders, dependencies) that are opaque
// json.RawMessage at the Go layer.
func hasJSONArrayItems(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err != nil {
		return false
	}
	return len(arr) > 0
}
