package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// Service interfaces for features

type FeedbackServiceInterface interface {
	Create(ctx context.Context, incidentID uuid.UUID, userID *uuid.UUID, req models.CreateFeedbackRequest) (*models.IncidentFeedback, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentFeedback, error)
}

type KnowledgeBaseServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateKnowledgeBaseRequest) (*models.KnowledgeBaseEntry, error)
	CreateFromHumanCorrection(ctx context.Context, orgID, incidentID uuid.UUID, softwareID *uuid.UUID, rootCauseSummary string) (*models.KnowledgeBaseEntry, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.KnowledgeBaseEntry, error)
	List(ctx context.Context, orgID uuid.UUID, category string) ([]models.KnowledgeBaseEntry, error)
	Search(ctx context.Context, orgID uuid.UUID, softwareID *uuid.UUID, errorPattern string) ([]models.KnowledgeBaseEntry, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateKnowledgeBaseRequest) (*models.KnowledgeBaseEntry, error)
	IncrementReferences(ctx context.Context, id uuid.UUID) error
}

type SimilarIncidentServiceInterface interface {
	Create(ctx context.Context, incidentID uuid.UUID, req models.CreateSimilarIncidentRequest) (*models.SimilarIncident, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.SimilarIncident, error)
}

// SemanticSimilarServiceInterface provides embedding-based similar-incident
// matching. Optional: a nil handler field disables semantic enrichment.
type SemanticSimilarServiceInterface interface {
	FindSimilarByEmbedding(ctx context.Context, incidentID uuid.UUID, limit int) ([]models.SimilarIncidentMatch, error)
}

type CorrelationRuleServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateCorrelationRuleRequest) (*models.CorrelationRule, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.CorrelationRule, error)
	List(ctx context.Context, orgID uuid.UUID) ([]models.CorrelationRule, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateCorrelationRuleRequest) (*models.CorrelationRule, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type AlertGroupServiceInterface interface {
	Create(ctx context.Context, incidentID uuid.UUID, req models.CreateAlertGroupRequest) (*models.AlertGroup, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.AlertGroup, error)
}

type NotificationChannelServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateNotificationChannelRequest) (*models.NotificationChannel, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.NotificationChannel, error)
	List(ctx context.Context, orgID uuid.UUID) ([]models.NotificationChannel, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateNotificationChannelRequest) (*models.NotificationChannel, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type EscalationPolicyServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateEscalationPolicyRequest) (*models.EscalationPolicy, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.EscalationPolicy, error)
	List(ctx context.Context, orgID uuid.UUID) ([]models.EscalationPolicy, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateEscalationPolicyRequest) (*models.EscalationPolicy, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type NotificationLogServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateNotificationLogRequest) (*models.NotificationLogEntry, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.NotificationLogEntry, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.NotificationLogEntry, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string, errMsg string) error
}

type RunbookServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateRunbookRequest) (*models.Runbook, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Runbook, error)
	List(ctx context.Context, orgID uuid.UUID) ([]models.Runbook, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateRunbookRequest) (*models.Runbook, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type RunbookStepServiceInterface interface {
	Create(ctx context.Context, runbookID uuid.UUID, req models.CreateRunbookStepRequest) (*models.RunbookStep, error)
	ListByRunbook(ctx context.Context, runbookID uuid.UUID) ([]models.RunbookStep, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateRunbookStepRequest) (*models.RunbookStep, error)
	Reorder(ctx context.Context, orderedStepIDs []uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type RunbookExecutionServiceInterface interface {
	Create(ctx context.Context, req models.CreateRunbookExecutionRequest) (*models.RunbookExecution, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.RunbookExecution, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.RunbookExecution, error)
	ListByRunbook(ctx context.Context, runbookID uuid.UUID) ([]models.RunbookExecution, error)
	Update(ctx context.Context, id uuid.UUID, req models.UpdateRunbookExecutionRequest) (*models.RunbookExecution, error)
}

type ChangeEventServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateChangeEventRequest) (*models.ChangeEvent, error)
	ListBySoftware(ctx context.Context, softwareID uuid.UUID, since time.Time) ([]models.ChangeEvent, error)
}

type AnalyticsServiceInterface interface {
	GetMTTR(ctx context.Context, orgID uuid.UUID, days int) ([]models.AnalyticsMTTR, error)
	GetIncidentTrends(ctx context.Context, orgID uuid.UUID, days int) ([]models.AnalyticsIncidentTrend, error)
	GetAgentEffectiveness(ctx context.Context, orgID uuid.UUID) ([]models.AnalyticsAgentEffectiveness, error)
	GetCostByModel(ctx context.Context, orgID uuid.UUID) ([]models.AnalyticsCostByModel, error)
	GetCostByIncident(ctx context.Context, orgID uuid.UUID, limit int) ([]models.AnalyticsCostByIncident, error)
}

type FeaturesHandler struct {
	Feedback         FeedbackServiceInterface
	KnowledgeBase    KnowledgeBaseServiceInterface
	// Incidents is used to resolve an incident's software_id when
	// promoting a human feedback correction to the knowledge base (see
	// CreateFeedback), so the resulting entry can be filtered by service
	// in a future search. Optional -- nil just means the promoted entry
	// carries no software_id, same as before this field existed.
	Incidents IncidentServiceInterface
	SimilarIncidents SimilarIncidentServiceInterface
	SemanticSimilar  SemanticSimilarServiceInterface // optional: embedding-based matches
	CorrelationRules CorrelationRuleServiceInterface
	AlertGroups      AlertGroupServiceInterface
	NotifChannels    NotificationChannelServiceInterface
	EscalationPols   EscalationPolicyServiceInterface
	NotifLog         NotificationLogServiceInterface
	Runbooks         RunbookServiceInterface
	RunbookSteps     RunbookStepServiceInterface
	RunbookExecs     RunbookExecutionServiceInterface
	ChangeEvents     ChangeEventServiceInterface
	Analytics        AnalyticsServiceInterface
	EventPublisher   EventPublisherInterface
}

// --- Feedback ---

func (h *FeaturesHandler) ListFeedback(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	items, err := h.Feedback.ListByIncident(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) CreateFeedback(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	var userID *uuid.UUID
	if v, exists := c.Get("user_id"); exists {
		uid := v.(uuid.UUID)
		userID = &uid
	}
	feedback, err := h.Feedback.Create(c.Request.Context(), incidentID, userID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}

	// Outer loop, human side: a negative rating with a correction is a
	// human telling us the RCA was wrong and what the real answer is --
	// promote it straight into the knowledge base as human_validated so
	// the next similar incident gets it as a few-shot example. Best-effort
	// and never fails the feedback response itself: the feedback row is
	// already durably saved above regardless of what happens here.
	if req.Rating == "negative" && strings.TrimSpace(req.Correction) != "" && h.KnowledgeBase != nil {
		var softwareID *uuid.UUID
		if h.Incidents != nil {
			if incident, incErr := h.Incidents.GetByID(c.Request.Context(), incidentID); incErr == nil {
				softwareID = &incident.SoftwareID
			} else {
				slog.Warn("failed to resolve incident software_id for KB promotion, storing without it",
					"error", incErr.Error(), "incident_id", incidentID.String())
			}
		}
		if _, kbErr := h.KnowledgeBase.CreateFromHumanCorrection(
			c.Request.Context(), getOrgID(c), incidentID, softwareID, req.Correction,
		); kbErr != nil {
			slog.Error("failed to promote feedback correction to knowledge base",
				"error", kbErr.Error(), "incident_id", incidentID.String())
		}
	}

	c.JSON(http.StatusCreated, feedback)
}

// --- Knowledge Base ---

func (h *FeaturesHandler) ListKnowledgeBase(c *gin.Context) {
	category := c.Query("category")
	items, err := h.KnowledgeBase.List(c.Request.Context(), getOrgID(c), category)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) CreateKnowledgeBase(c *gin.Context) {
	var req models.CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	kb, err := h.KnowledgeBase.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, kb)
}

func (h *FeaturesHandler) GetKnowledgeBase(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	kb, err := h.KnowledgeBase.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, kb)
}

func (h *FeaturesHandler) UpdateKnowledgeBase(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	kb, err := h.KnowledgeBase.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, kb)
}

func (h *FeaturesHandler) SearchKnowledgeBase(c *gin.Context) {
	var req models.KnowledgeBaseSearchRequest
	if c.Request.Method == http.MethodGet {
		// agent-service's internal route (see backend_client.py's
		// search_knowledge_base) sends software_id/query/limit as query
		// params, not a JSON body. Bind those two manually instead of via
		// Gin's ShouldBind(form): gin's form binder can't decode a
		// *uuid.UUID field from a query string (works fine for JSON via
		// uuid.UUID's UnmarshalJSON, but the form/query binder uses a
		// different reflection path that 400s on it).
		req.Query = c.Query("query")
		req.ErrorPattern = c.Query("error_pattern")
		if v := c.Query("software_id"); v != "" {
			if id, err := uuid.Parse(v); err == nil {
				req.SoftwareID = &id
			}
		}
		if v := c.Query("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				req.Limit = n
			}
		}
	} else if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	// Free-text `query` takes precedence for semantic search; falls back to
	// error_pattern (which is also what the ILIKE fallback matches against).
	query := req.Query
	if query == "" {
		query = req.ErrorPattern
	}
	items, err := h.KnowledgeBase.Search(c.Request.Context(), getOrgID(c), req.SoftwareID, query)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	if req.Limit > 0 && len(items) > req.Limit {
		items = items[:req.Limit]
	}
	c.JSON(http.StatusOK, items)
}

// IncrementKnowledgeBaseReferences bumps times_referenced when a pipeline
// run cites an existing entry (see backend_client.py's
// increment_kb_references) -- previously had no route at all, so
// agent-service's outer_loop calls to it always 404ed.
func (h *FeaturesHandler) IncrementKnowledgeBaseReferences(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.KnowledgeBase.IncrementReferences(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// --- Similar Incidents ---

func (h *FeaturesHandler) ListSimilarIncidents(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	items, err := h.SimilarIncidents.ListByIncident(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	// Opt-in semantic enrichment: ?semantic=true merges embedding-based
	// matches (same response shape) with the stored links. Best-effort.
	if c.Query("semantic") == "true" && h.SemanticSimilar != nil {
		items = h.mergeSemanticMatches(c.Request.Context(), id, items)
	}
	c.JSON(http.StatusOK, items)
}

// mergeSemanticMatches appends embedding-based matches not already present as
// stored links. Failures are ignored so the stored links are always returned.
func (h *FeaturesHandler) mergeSemanticMatches(ctx context.Context, incidentID uuid.UUID, items []models.SimilarIncident) []models.SimilarIncident {
	matches, err := h.SemanticSimilar.FindSimilarByEmbedding(ctx, incidentID, 5)
	if err != nil {
		return items
	}
	seen := make(map[uuid.UUID]bool, len(items))
	for _, it := range items {
		seen[it.SimilarIncidentID] = true
	}
	for _, m := range matches {
		if seen[m.IncidentID] {
			continue
		}
		matchedOn, _ := json.Marshal(map[string]string{"method": "embedding", "title": m.Title})
		items = append(items, models.SimilarIncident{
			ID:                uuid.New(), // ephemeral: not persisted
			IncidentID:        incidentID,
			SimilarIncidentID: m.IncidentID,
			SimilarityScore:   m.Similarity,
			MatchedOn:         matchedOn,
			CreatedAt:         m.CreatedAt,
		})
	}
	return items
}

func (h *FeaturesHandler) CreateSimilarIncident(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateSimilarIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	si, err := h.SimilarIncidents.Create(c.Request.Context(), incidentID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, si)
}

// --- Correlation Rules ---

func (h *FeaturesHandler) ListCorrelationRules(c *gin.Context) {
	items, err := h.CorrelationRules.List(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) CreateCorrelationRule(c *gin.Context) {
	var req models.CreateCorrelationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	cr, err := h.CorrelationRules.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, cr)
}

func (h *FeaturesHandler) GetCorrelationRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	cr, err := h.CorrelationRules.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, cr)
}

func (h *FeaturesHandler) UpdateCorrelationRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateCorrelationRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	cr, err := h.CorrelationRules.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, cr)
}

func (h *FeaturesHandler) DeleteCorrelationRule(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.CorrelationRules.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Alert Groups ---

func (h *FeaturesHandler) ListAlertGroups(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	items, err := h.AlertGroups.ListByIncident(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

// --- Notification Channels ---

func (h *FeaturesHandler) ListNotificationChannels(c *gin.Context) {
	items, err := h.NotifChannels.List(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) CreateNotificationChannel(c *gin.Context) {
	var req models.CreateNotificationChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	nc, err := h.NotifChannels.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, nc)
}

func (h *FeaturesHandler) GetNotificationChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	nc, err := h.NotifChannels.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, nc)
}

func (h *FeaturesHandler) UpdateNotificationChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateNotificationChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	nc, err := h.NotifChannels.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, nc)
}

func (h *FeaturesHandler) DeleteNotificationChannel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.NotifChannels.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Escalation Policies ---

func (h *FeaturesHandler) ListEscalationPolicies(c *gin.Context) {
	items, err := h.EscalationPols.List(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) CreateEscalationPolicy(c *gin.Context) {
	var req models.CreateEscalationPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	ep, err := h.EscalationPols.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, ep)
}

func (h *FeaturesHandler) GetEscalationPolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	ep, err := h.EscalationPols.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, ep)
}

func (h *FeaturesHandler) UpdateEscalationPolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateEscalationPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	ep, err := h.EscalationPols.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, ep)
}

func (h *FeaturesHandler) DeleteEscalationPolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.EscalationPols.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Notification Log ---

// ListNotificationLogsGlobal serves the frontend's "Logs" tab (Notifications
// page), which lists a page of an org's notification log entries across all
// incidents -- not scoped to one incident (see ListNotificationLog below for
// that). This capability didn't exist at all before: only ListByIncident was
// wired end-to-end (repo/service/handler/route), so GET /notifications/logs
// 404'd and the Logs tab always rendered empty.
func (h *FeaturesHandler) ListNotificationLogsGlobal(c *gin.Context) {
	page, perPage := getPagination(c)
	items, total, err := h.NotifLog.ListByOrg(c.Request.Context(), getOrgID(c), page, perPage)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *FeaturesHandler) ListNotificationLog(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	items, err := h.NotifLog.ListByIncident(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

// --- Runbooks ---

func (h *FeaturesHandler) ListRunbooks(c *gin.Context) {
	items, err := h.Runbooks.List(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) CreateRunbook(c *gin.Context) {
	var req models.CreateRunbookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	rb, err := h.Runbooks.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, rb)
}

func (h *FeaturesHandler) GetRunbook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	rb, err := h.Runbooks.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, rb)
}

func (h *FeaturesHandler) UpdateRunbook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateRunbookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	rb, err := h.Runbooks.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, rb)
}

func (h *FeaturesHandler) DeleteRunbook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Runbooks.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Runbook Steps ---

func (h *FeaturesHandler) ListRunbookSteps(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	items, err := h.RunbookSteps.ListByRunbook(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) CreateRunbookStep(c *gin.Context) {
	runbookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateRunbookStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	step, err := h.RunbookSteps.Create(c.Request.Context(), runbookID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, step)
}

func (h *FeaturesHandler) UpdateRunbookStep(c *gin.Context) {
	stepID, err := uuid.Parse(c.Param("stepId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid stepId"})
		return
	}
	var req models.CreateRunbookStepRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	step, err := h.RunbookSteps.Update(c.Request.Context(), stepID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, step)
}

func (h *FeaturesHandler) DeleteRunbookStep(c *gin.Context) {
	stepID, err := uuid.Parse(c.Param("stepId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid stepId"})
		return
	}
	if err := h.RunbookSteps.Delete(c.Request.Context(), stepID); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

// ReorderRunbookSteps sets each step's position to its index in the
// request's step_ids list. Found missing live: the frontend's
// reorderRunbookSteps() already called this route, but it never existed on
// the backend -- drag-to-reorder was a no-op end to end.
func (h *FeaturesHandler) ReorderRunbookSteps(c *gin.Context) {
	var req models.ReorderRunbookStepsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	stepIDs := make([]uuid.UUID, 0, len(req.StepIDs))
	for _, s := range req.StepIDs {
		id, err := uuid.Parse(s)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid step id: " + s})
			return
		}
		stepIDs = append(stepIDs, id)
	}
	if err := h.RunbookSteps.Reorder(c.Request.Context(), stepIDs); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Runbook Executions ---

func (h *FeaturesHandler) ExecuteRunbook(c *gin.Context) {
	runbookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateRunbookExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	req.RunbookID = runbookID

	// 1. Fetch runbook steps
	steps, err := h.RunbookSteps.ListByRunbook(c.Request.Context(), runbookID)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}

	// 2. Build step results based on step types
	stepResults := make([]models.ExecutionStepResult, 0, len(steps))
	for _, step := range steps {
		sr := models.ExecutionStepResult{
			StepID:   step.ID.String(),
			StepName: step.Name,
			StepType: step.StepType,
			Status:   "pending",
		}
		switch step.StepType {
		case "manual":
			sr.Status = "pending_action"
		case "automated":
			sr.Status = "pending"
		case "approval":
			sr.Status = "pending_approval"
		default:
			sr.Status = "pending"
		}
		stepResults = append(stepResults, sr)
	}

	// Set first step as active
	if len(stepResults) > 0 {
		if stepResults[0].Status == "pending" {
			stepResults[0].Status = "running"
		}
	}

	// 3. Marshal step results
	stepResultsJSON, _ := json.Marshal(stepResults)

	// 4. Create execution record with status "running"
	req.TriggeredBy = "manual"
	exec, err := h.RunbookExecs.Create(c.Request.Context(), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}

	// Update with computed step results
	status := "running"
	currentStep := 1
	updateReq := models.UpdateRunbookExecutionRequest{
		Status:      &status,
		CurrentStep: &currentStep,
		StepResults: stepResultsJSON,
	}
	exec, err = h.RunbookExecs.Update(c.Request.Context(), exec.ID, updateReq)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}

	// 5. Publish Redis event if publisher is available
	if h.EventPublisher != nil {
		orgID := getOrgID(c)
		channel := fmt.Sprintf("rootcauseway:%s:runbook.execution.started", orgID.String())
		_ = h.EventPublisher.Publish(c.Request.Context(), channel, models.EventEnvelope{
			EventID:   uuid.New(),
			EventType: "runbook.execution.started",
			OrgID:     orgID,
			Timestamp: time.Now(),
			Payload: map[string]interface{}{
				"execution_id": exec.ID.String(),
				"runbook_id":   runbookID.String(),
			},
		})
	}

	c.JSON(http.StatusCreated, exec)
}

func (h *FeaturesHandler) CompleteExecutionStep(c *gin.Context) {
	execID, err := uuid.Parse(c.Param("execId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid execId"})
		return
	}
	stepID := c.Param("stepId")

	// Optional body -- {status, output}. Human "Mark Complete" clicks in
	// the UI send no body at all (status defaults to "completed" below),
	// but RunbookExecutor's automation loop needs to record a failed
	// automated step (and its result payload) rather than always claiming
	// success, so this now accepts an explicit status/output instead of
	// hardcoding "completed" unconditionally.
	var req models.CompleteExecutionStepRequest
	_ = c.ShouldBindJSON(&req) // empty/absent body is valid; ignore bind errors
	stepStatus := "completed"
	if req.Status != "" {
		stepStatus = req.Status
	}

	// Get execution
	exec, err := h.RunbookExecs.GetByID(c.Request.Context(), execID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}

	// Parse step results
	var stepResults []models.ExecutionStepResult
	if err := json.Unmarshal(exec.StepResults, &stepResults); err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "invalid step results"})
		return
	}

	// Find and complete the step
	found := false
	allDone := true
	nextIdx := -1
	for i, sr := range stepResults {
		if sr.StepID == stepID {
			found = true
			now := time.Now().Format(time.RFC3339)
			stepResults[i].Status = stepStatus
			stepResults[i].CompletedAt = &now
			stepResults[i].Output = req.Output
			nextIdx = i + 1
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "step not found in execution"})
		return
	}

	// Advance to next step
	if nextIdx >= 0 && nextIdx < len(stepResults) {
		sr := &stepResults[nextIdx]
		switch sr.StepType {
		case "manual":
			sr.Status = "pending_action"
		case "automated":
			sr.Status = "running"
		case "approval":
			sr.Status = "pending_approval"
		default:
			sr.Status = "running"
		}
	}

	// Check if all done
	for _, sr := range stepResults {
		if sr.Status != "completed" && sr.Status != "failed" && sr.Status != "skipped" {
			allDone = false
			break
		}
	}

	stepResultsJSON, _ := json.Marshal(stepResults)
	status := "running"
	currentStep := exec.CurrentStep + 1
	if allDone {
		status = "completed"
	}
	updateReq := models.UpdateRunbookExecutionRequest{
		Status:      &status,
		CurrentStep: &currentStep,
		StepResults: stepResultsJSON,
	}
	exec, err = h.RunbookExecs.Update(c.Request.Context(), execID, updateReq)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}

	c.JSON(http.StatusOK, exec)
}

func (h *FeaturesHandler) GetRunbookExecution(c *gin.Context) {
	execID, err := uuid.Parse(c.Param("execId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid execId"})
		return
	}
	exec, err := h.RunbookExecs.GetByID(c.Request.Context(), execID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, exec)
}

func (h *FeaturesHandler) UpdateRunbookExecution(c *gin.Context) {
	execID, err := uuid.Parse(c.Param("execId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid execId"})
		return
	}
	var req models.UpdateRunbookExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	exec, err := h.RunbookExecs.Update(c.Request.Context(), execID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, exec)
}

func (h *FeaturesHandler) ListIncidentRunbookExecutions(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	items, err := h.RunbookExecs.ListByIncident(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

// ListRunbookExecutionsByRunbook serves RunbookDetailPage's execution
// history tab, scoped to one runbook across every incident it ran on --
// distinct from ListIncidentRunbookExecutions above, which is scoped to one
// incident. Found missing live: the frontend's listRunbookExecutions()
// already called GET /runbooks/:id/executions; only the by-incident route
// existed on the backend, so this always 404'd.
func (h *FeaturesHandler) ListRunbookExecutionsByRunbook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	items, err := h.RunbookExecs.ListByRunbook(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

// --- Change Events ---

func (h *FeaturesHandler) CreateChangeEvent(c *gin.Context) {
	var req models.CreateChangeEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	ce, err := h.ChangeEvents.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, ce)
}

func (h *FeaturesHandler) ListChangeEvents(c *gin.Context) {
	softwareIDStr := c.Query("software_id")
	if softwareIDStr == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "software_id required"})
		return
	}
	softwareID, err := uuid.Parse(softwareIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid software_id"})
		return
	}
	daysStr := c.DefaultQuery("days", "7")
	days, _ := strconv.Atoi(daysStr)
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}
	since := time.Now().AddDate(0, 0, -days)
	items, err := h.ChangeEvents.ListBySoftware(c.Request.Context(), softwareID, since)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) ListSoftwareChanges(c *gin.Context) {
	softwareID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	daysStr := c.DefaultQuery("days", "7")
	days, _ := strconv.Atoi(daysStr)
	if days <= 0 {
		days = 7
	}
	if days > 365 {
		days = 365
	}
	since := time.Now().AddDate(0, 0, -days)
	items, err := h.ChangeEvents.ListBySoftware(c.Request.Context(), softwareID, since)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

// --- Analytics ---

func (h *FeaturesHandler) GetMTTR(c *gin.Context) {
	periodStr := c.DefaultQuery("period", "7d")
	days := 7
	if len(periodStr) > 1 && periodStr[len(periodStr)-1] == 'd' {
		if d, err := strconv.Atoi(periodStr[:len(periodStr)-1]); err == nil && d > 0 {
			days = d
		}
	}
	items, err := h.Analytics.GetMTTR(c.Request.Context(), getOrgID(c), days)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) GetIncidentTrends(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, _ := strconv.Atoi(daysStr)
	if days <= 0 {
		days = 30
	}
	items, err := h.Analytics.GetIncidentTrends(c.Request.Context(), getOrgID(c), days)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) GetAgentEffectiveness(c *gin.Context) {
	items, err := h.Analytics.GetAgentEffectiveness(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) GetCostByModel(c *gin.Context) {
	items, err := h.Analytics.GetCostByModel(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *FeaturesHandler) GetCostByIncident(c *gin.Context) {
	items, err := h.Analytics.GetCostByIncident(c.Request.Context(), getOrgID(c), 20)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

