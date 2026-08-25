package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Incidents ---

// verifyIncidentOwnership fetches an incident by ID and confirms it belongs
// to the caller's org, writing the 404 response itself on any failure (bad
// ID, not found, OR wrong org -- same response either way, so a caller
// probing IDs from another org can't distinguish "doesn't exist" from
// "exists but isn't yours"). Every handler below that reads or mutates a
// single incident by ID must call this first: GetIncident, GetIncidentFull,
// UpdateIncident, AddIncidentEvent and AddIncidentEvidence all fetched (or
// mutated) by ID alone with no org check at all until this was found during
// a platform audit -- any authenticated user from any org could read or
// write any other org's incidents by ID.
func (h *Handler) verifyIncidentOwnership(c *gin.Context, id uuid.UUID) (*models.Incident, bool) {
	incident, err := h.Incidents.GetByID(c.Request.Context(), id)
	if err != nil || incident.OrgID != getOrgID(c) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return nil, false
	}
	return incident, true
}

func (h *Handler) ListIncidents(c *gin.Context) {
	page, perPage := getPagination(c)
	status := c.Query("status")
	severity := c.Query("severity")
	var softwareID *uuid.UUID
	if sid := c.Query("software_id"); sid != "" {
		if id, err := uuid.Parse(sid); err == nil {
			softwareID = &id
		}
	}
	var from *time.Time
	if f := c.Query("from"); f != "" {
		if t, err := time.Parse(time.RFC3339, f); err == nil {
			from = &t
		}
	}
	items, total, err := h.Incidents.List(c.Request.Context(), getOrgID(c), status, severity, softwareID, from, page, perPage)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *Handler) GetIncident(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	incident, ok := h.verifyIncidentOwnership(c, id)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, incident)
}

func (h *Handler) UpdateIncident(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if _, ok := h.verifyIncidentOwnership(c, id); !ok {
		return
	}
	var req models.UpdateIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if req.AssigneeID != nil && *req.AssigneeID != "" {
		if _, err := uuid.Parse(*req.AssigneeID); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid assignee_id: " + err.Error()})
			return
		}
	}
	incident, justTerminalized, err := h.Incidents.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}

	if h.EventPublisher != nil {
		orgID := getOrgID(c)

		// Publish incident.resolved to trigger postmortem generation. Fires
		// on the transition into "resolved" OR "closed" (whichever the
		// operator picks first) but only once per incident --
		// justTerminalized is false on a later resolved->closed update, so
		// postmortem isn't re-triggered.
		if justTerminalized {
			channel := fmt.Sprintf("rootcauseway:%s:incident.resolved", orgID.String())
			_ = h.EventPublisher.Publish(c.Request.Context(), channel, models.EventEnvelope{
				EventID:   uuid.New(),
				EventType: "incident.resolved",
				OrgID:     orgID,
				Timestamp: time.Now(),
				Payload: map[string]interface{}{
					"incident_id": id.String(),
				},
			})
		}

		// incident.updated -- separate purpose from incident.resolved above:
		// drives the WebSocket bridge -> frontend's live incident
		// list/dashboard refresh on *any* update (severity, status,
		// assignee, ...), not just the terminal transition.
		updatedChannel := fmt.Sprintf("rootcauseway:%s:incident.updated", orgID.String())
		_ = h.EventPublisher.Publish(c.Request.Context(), updatedChannel, models.EventEnvelope{
			EventID:   uuid.New(),
			EventType: "incident.updated",
			OrgID:     orgID,
			Timestamp: time.Now(),
			Payload: models.IncidentUpdatedPayload{
				IncidentID: id,
				Title:      incident.Title,
				Severity:   incident.Severity,
				Status:     incident.Status,
			},
		})
	}

	c.JSON(http.StatusOK, incident)
}

func (h *Handler) AddIncidentEvent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if _, ok := h.verifyIncidentOwnership(c, id); !ok {
		return
	}
	var req models.CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	actor := "system"
	if v, exists := c.Get("user_id"); exists {
		actor = v.(uuid.UUID).String()
	}
	event, err := h.Incidents.AddEvent(c.Request.Context(), id, actor, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, event)
}

// isOrchestratorWrittenEvidence reports whether an evidence row's Source
// identifies it as something the orchestrator (agent-service) itself wrote
// -- either a skill agent's own result ("agent:<skill_id>", any skill, not
// just the 4 built-ins) or its MLflow trace link ("mlflow"). Evidence from
// these sources must never re-trigger evidence.uploaded, or a re-analysis
// writes more of the same evidence, which re-triggers again, forever.
func isOrchestratorWrittenEvidence(source string) bool {
	return source == "mlflow" || strings.HasPrefix(source, "agent:")
}

func (h *Handler) AddIncidentEvidence(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if _, ok := h.verifyIncidentOwnership(c, id); !ok {
		return
	}
	var req models.CreateEvidenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	evidence, err := h.Incidents.AddEvidence(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}

	// Publish event so orchestrator can re-analyze with new evidence -- but
	// never for evidence the orchestrator itself just wrote (its own agent
	// results, source "agent:<skill_id>", or its MLflow trace link, source
	// "mlflow"). A hardcoded list of 4 literal skill-id sources used to gate
	// this ("agent:triage"/"agent:evidence-collection"/"agent:rca"/
	// "agent:postmortem") only matched the built-in agents' skill IDs -- a
	// custom skill's source is "agent:<uuid>", which never matched, so every
	// evidence write for a custom skill (and the "mlflow" trace-link write,
	// unconditionally on ALL skills) republished evidence.uploaded, which
	// re-ran analysis, which wrote more evidence, forever. Confirmed live:
	// the same incident re-analyzed 4 times in under 5 minutes before this
	// was caught. isOrchestratorWrittenEvidence below matches any source the
	// orchestrator itself produces, not just the 4 built-ins.
	if h.EventPublisher != nil && !isOrchestratorWrittenEvidence(req.Source) {
		orgID := getOrgID(c)
		channel := fmt.Sprintf("rootcauseway:%s:evidence.uploaded", orgID.String())
		_ = h.EventPublisher.Publish(c.Request.Context(), channel, models.EventEnvelope{
			EventID:   uuid.New(),
			EventType: "evidence.uploaded",
			OrgID:     orgID,
			Timestamp: time.Now(),
			Payload: map[string]interface{}{
				"incident_id": id.String(),
				"evidence_id": evidence.ID.String(),
				"type":        req.Type,
				"title":       req.Title,
			},
		})
	}

	c.JSON(http.StatusCreated, evidence)
}

// --- Ingestion (public endpoint) ---

func (h *Handler) IngestAlert(c *gin.Context) {
	token := c.Param("token")
	var rawPayload json.RawMessage
	if err := c.ShouldBindJSON(&rawPayload); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	result, err := h.Ingestion.IngestAlert(c.Request.Context(), token, rawPayload)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, result)
}

// --- Evidence Upload ---

func (h *Handler) UploadEvidence(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "file required"})
		return
	}
	defer file.Close()

	evidenceDir := filepath.Join(".", "data", "evidence", incidentID.String())
	if err := os.MkdirAll(evidenceDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to create directory"})
		return
	}

	evidenceID := uuid.New()
	ext := filepath.Ext(header.Filename)
	blobPath := filepath.Join(evidenceDir, evidenceID.String()+ext)

	out, err := os.Create(blobPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to save file"})
		return
	}
	defer out.Close()

	written, err := io.Copy(out, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to write file"})
		return
	}

	title := c.PostForm("title")
	if title == "" {
		title = header.Filename
	}
	evidenceType := c.PostForm("type")
	if evidenceType == "" {
		evidenceType = "manual"
	}

	evidence := models.IncidentEvidence{
		ID:            evidenceID,
		IncidentID:    incidentID,
		Type:          evidenceType,
		Title:         title,
		Content:       json.RawMessage(fmt.Sprintf(`{"filename":"%s"}`, header.Filename)),
		Source:        c.PostForm("source"),
		CollectedAt:   time.Now(),
		BlobPath:      blobPath,
		BlobSizeBytes: written,
		MimeType:      header.Header.Get("Content-Type"),
	}

	saved, err := h.Incidents.AddEvidence(c.Request.Context(), incidentID, models.CreateEvidenceRequest{
		Type:    evidence.Type,
		Title:   evidence.Title,
		Content: evidence.Content,
		Source:  evidence.Source,
	})
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}

	// Publish event so orchestrator can re-analyze with new evidence
	if h.EventPublisher != nil {
		orgID := getOrgID(c)
		channel := fmt.Sprintf("rootcauseway:%s:evidence.uploaded", orgID.String())
		_ = h.EventPublisher.Publish(c.Request.Context(), channel, models.EventEnvelope{
			EventID:   uuid.New(),
			EventType: "evidence.uploaded",
			OrgID:     orgID,
			Timestamp: time.Now(),
			Payload: map[string]interface{}{
				"incident_id": incidentID.String(),
				"evidence_id": saved.ID.String(),
				"type":        evidence.Type,
				"title":       evidence.Title,
				"filename":    header.Filename,
			},
		})
	}

	c.JSON(http.StatusCreated, evidence)
}

// --- Quarantine ---

func (h *Handler) ListQuarantine(c *gin.Context) {
	orgID := getOrgID(c)
	resolved := c.Query("resolved") == "true"
	page, perPage := getPagination(c)

	if h.QuarantineRepo == nil {
		c.JSON(http.StatusOK, models.PaginatedResponse{Data: []interface{}{}, Total: 0, Page: page, PerPage: perPage})
		return
	}

	items, total, err := h.QuarantineRepo.List(c.Request.Context(), orgID, resolved, page, perPage)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *Handler) ResolveQuarantine(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	var req models.ResolveQuarantineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	if h.QuarantineRepo == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "quarantine not configured"})
		return
	}

	// Resolve: link to software and optionally re-ingest
	if err := h.QuarantineRepo.Resolve(c.Request.Context(), id, req.SoftwareID); err != nil {
		handleDBError(c, err, "resource")
		return
	}

	// Get the quarantined alert and re-ingest it
	q, err := h.QuarantineRepo.GetByID(c.Request.Context(), id)
	if err == nil && q != nil {
		result, err := h.Ingestion.IngestAlert(c.Request.Context(), "", q.RawPayload)
		if err == nil && result != nil && !result.Quarantined {
			c.JSON(http.StatusOK, gin.H{"resolved": true, "incident_id": result.IncidentID})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"resolved": true})
}

func (h *Handler) GetOnboardingStatus(c *gin.Context) {
	orgID := getOrgID(c)
	ctx := c.Request.Context()

	software, _, _ := h.Software.List(ctx, orgID, 1, 1)
	webhooks, _, _ := h.Webhooks.List(ctx, orgID, 1, 1)
	incidents, _, _ := h.Incidents.List(ctx, orgID, "", "", nil, nil, 1, 1)

	completed := len(software) > 0 && len(webhooks) > 0

	c.JSON(http.StatusOK, gin.H{
		"completed":     completed,
		"has_software":  len(software) > 0,
		"has_webhooks":  len(webhooks) > 0,
		"has_incidents": len(incidents) > 0,
	})
}

func (h *Handler) GetIncidentFull(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	ctx := c.Request.Context()

	incident, ok := h.verifyIncidentOwnership(c, id)
	if !ok {
		return
	}

	dag, _ := h.AgentRuns.GetDAG(ctx, id)
	if dag == nil {
		dag = []models.AgentRun{}
	}

	rci, _ := h.RCI.GetByIncidentID(ctx, id)
	rca, _ := h.RCA.GetByIncidentID(ctx, id)
	pm, _ := h.Postmortem.GetByIncidentID(ctx, id)

	var sw *models.SoftwareEntry
	if incident.SoftwareID != uuid.Nil {
		sw, _ = h.Software.GetByID(ctx, incident.SoftwareID)
	}

	evidence, _ := h.Incidents.ListEvidence(ctx, id)
	if evidence == nil {
		evidence = []models.IncidentEvidence{}
	}

	full := models.IncidentFull{
		Incident: *incident,
		Software: sw,
		DAG:      dag,
		Evidence: evidence,
		RCIData:  rci,
		RCAData:  rca,
		PMData:   pm,
	}

	c.JSON(http.StatusOK, full)
}
