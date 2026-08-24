package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
)

// DependencyGraphProvider is satisfied by *services.SoftwareService. Declared
// locally (rather than reusing handlers.SoftwareServiceInterface) so this file
// stays additive/self-contained and easily unit-testable with a small mock.
type DependencyGraphProvider interface {
	GetDependencyGraph(ctx context.Context, id uuid.UUID) (*services.DependencyGraph, error)
}

// CorrelationIncidentRepo is satisfied by *database.PgIncidentRepository.
// Declared locally so this handler can be unit-tested without a real DB.
type CorrelationIncidentRepo interface {
	ListOpenBySoftwareIDs(ctx context.Context, orgID uuid.UUID, softwareIDs []uuid.UUID, since time.Time) ([]models.Incident, error)
	FindByFingerprint(ctx context.Context, orgID uuid.UUID, fingerprint string, since time.Time) (*models.Incident, error)
}

// CorrelationExtraHandler exposes additive internal endpoints supporting richer
// alert correlation in agent-service: dependency-graph cascade correlation
// (checking open incidents on upstream/downstream services) and fingerprint-based
// alert dedup. It intentionally talks to the incident repository directly
// (rather than through IncidentService) to keep this addition self-contained.
type CorrelationExtraHandler struct {
	Software     DependencyGraphProvider
	IncidentRepo CorrelationIncidentRepo
}

const (
	defaultCascadeWindowSeconds = 300
	defaultDedupWindowSeconds   = 900
)

// GetSoftwareDependencyGraph returns the upstream (declared dependencies) and
// downstream (dependents) services for a software catalog entry, so the
// correlation engine can look for open incidents on related services.
func (h *CorrelationExtraHandler) GetSoftwareDependencyGraph(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid software id"})
		return
	}
	graph, err := h.Software.GetDependencyGraph(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "software not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": graph})
}

// ListOpenIncidentsBySoftware returns open (non-resolved/closed) incidents for
// any of the given software ids, created within a recency window. Used for
// dependency-graph cascade correlation.
func (h *CorrelationExtraHandler) ListOpenIncidentsBySoftware(c *gin.Context) {
	orgID, ok := orgIDFromContext(c)
	if !ok {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "missing X-Org-ID header"})
		return
	}

	idsParam := c.QueryArray("software_id")
	if len(idsParam) == 0 {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "software_id query param required"})
		return
	}
	ids := make([]uuid.UUID, 0, len(idsParam))
	for _, s := range idsParam {
		if id, err := uuid.Parse(s); err == nil {
			ids = append(ids, id)
		}
	}

	since := time.Now().Add(-time.Duration(windowSecondsParam(c, defaultCascadeWindowSeconds)) * time.Second)

	incidents, err := h.IncidentRepo.ListOpenBySoftwareIDs(c.Request.Context(), orgID, ids, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to list incidents"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": incidents})
}

// CorrelationCheck implements the "same-service" leg of agent-service's
// CorrelationEngine.check_correlation: is there already an open incident on
// this exact software_id within the caller-supplied time window? (Rule
// lookup/window resolution happens client-side, in
// CorrelationEngine._check_same_service, before this call.)
//
// This used to be a stub (FeaturesHandler.CorrelationCheck, always returned
// Correlated: false) that also required a field
// (alert_snapshot_id) the real caller never sends -- ShouldBindJSON 400'd
// on every real call, so correlation always fell back to "treat as new
// incident" and this leg of the pipeline never actually ran. Moved here to
// reuse the same CorrelationIncidentRepo already wired for the
// dependency-cascade leg above.
//
// Found live testing that exact fix: IngestAlert (Go) creates the incident
// row and commits it *before* agent-service's alert.received handler ever
// calls this endpoint -- so for the very first alert of a brand-new
// incident, "an open incident on this software_id within the window"
// trivially matches the incident this alert itself just created.
// ExcludeIncidentID is that self-reference; every real new incident
// self-correlated and silently never ran the pipeline until this was
// filtered out.
func (h *CorrelationExtraHandler) CorrelationCheck(c *gin.Context) {
	orgID, ok := orgIDFromContext(c)
	if !ok {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "missing X-Org-ID header"})
		return
	}
	var req models.CorrelationCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	windowSeconds := req.TimeWindowSeconds
	if windowSeconds <= 0 {
		windowSeconds = defaultCascadeWindowSeconds
	}
	since := time.Now().Add(-time.Duration(windowSeconds) * time.Second)

	incidents, err := h.IncidentRepo.ListOpenBySoftwareIDs(c.Request.Context(), orgID, []uuid.UUID{req.SoftwareID}, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to check correlation"})
		return
	}
	for _, inc := range incidents {
		if req.ExcludeIncidentID != nil && inc.ID == *req.ExcludeIncidentID {
			continue
		}
		incidentID := inc.ID
		c.JSON(http.StatusOK, models.CorrelationCheckResponse{Correlated: true, IncidentID: &incidentID})
		return
	}
	c.JSON(http.StatusOK, models.CorrelationCheckResponse{Correlated: false})
}

// FindIncidentByFingerprint looks up the most recent incident whose alert
// fingerprint matches, within a recency window. Used for literal-repeat alert
// dedup: an alert firing again with the same fingerprint should attach to the
// existing incident instead of re-running full correlation logic.
//
// Same self-match risk as CorrelationCheck above, for the same reason: an
// incident's own creating alert shares its fingerprint by definition, and
// IngestAlert commits the incident before this ever gets called. exclude_incident_id
// is optional (not every caller has an incident context yet) but must be
// honored when present.
func (h *CorrelationExtraHandler) FindIncidentByFingerprint(c *gin.Context) {
	orgID, ok := orgIDFromContext(c)
	if !ok {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "missing X-Org-ID header"})
		return
	}
	fingerprint := c.Query("fingerprint")
	if fingerprint == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "fingerprint query param required"})
		return
	}
	since := time.Now().Add(-time.Duration(windowSecondsParam(c, defaultDedupWindowSeconds)) * time.Second)

	incident, err := h.IncidentRepo.FindByFingerprint(c.Request.Context(), orgID, fingerprint, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to look up incident by fingerprint"})
		return
	}
	if incident != nil {
		if excludeStr := c.Query("exclude_incident_id"); excludeStr != "" {
			if excludeID, parseErr := uuid.Parse(excludeStr); parseErr == nil && incident.ID == excludeID {
				incident = nil
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": incident})
}

func orgIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get("org_id")
	if !exists {
		return uuid.UUID{}, false
	}
	orgID, ok := val.(uuid.UUID)
	if !ok || orgID == uuid.Nil {
		return uuid.UUID{}, false
	}
	return orgID, true
}

func windowSecondsParam(c *gin.Context, def int) int {
	w := c.Query("window_seconds")
	if w == "" {
		return def
	}
	parsed, err := strconv.Atoi(w)
	if err != nil || parsed <= 0 {
		return def
	}
	return parsed
}
