package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// RetentionServiceInterface is implemented by services.RetentionService;
// declared here as an interface so it can be mocked in handler tests,
// matching the *ServiceInterface pattern used throughout this package (see
// features_handlers.go).
type RetentionServiceInterface interface {
	CreatePolicy(ctx context.Context, orgID uuid.UUID, req models.CreateRetentionPolicyRequest) (*models.RetentionPolicy, error)
	GetPolicy(ctx context.Context, id uuid.UUID) (*models.RetentionPolicy, error)
	ListPolicies(ctx context.Context, orgID uuid.UUID) ([]models.RetentionPolicy, error)
	UpdatePolicy(ctx context.Context, id uuid.UUID, req models.UpdateRetentionPolicyRequest) (*models.RetentionPolicy, error)
	DeletePolicy(ctx context.Context, id uuid.UUID) error
	RunRetentionSweep(ctx context.Context, orgID uuid.UUID) (*models.RetentionSweepSummary, error)
}

// RetentionHandler exposes retention policy CRUD and the manual sweep
// trigger. It is a standalone handler group (not a method on *Handler) so
// it can be wired into main.go additively without touching the shared
// Handler struct/constructor that other in-flight work also touches.
type RetentionHandler struct {
	Service RetentionServiceInterface
}

func NewRetentionHandler(service RetentionServiceInterface) *RetentionHandler {
	return &RetentionHandler{Service: service}
}

// ListRetentionPolicies handles GET /api/v1/retention-policies.
func (h *RetentionHandler) ListRetentionPolicies(c *gin.Context) {
	policies, err := h.Service.ListPolicies(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "retention policies")
		return
	}
	c.JSON(http.StatusOK, gin.H{"policies": policies})
}

// CreateRetentionPolicy handles POST /api/v1/retention-policies.
func (h *RetentionHandler) CreateRetentionPolicy(c *gin.Context) {
	var req models.CreateRetentionPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	policy, err := h.Service.CreatePolicy(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "retention policy")
		return
	}
	c.JSON(http.StatusCreated, policy)
}

// UpdateRetentionPolicy handles PUT /api/v1/retention-policies/:id.
func (h *RetentionHandler) UpdateRetentionPolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	existing, err := h.Service.GetPolicy(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "retention policy")
		return
	}
	if callerOrg := getOrgID(c); callerOrg != uuid.Nil && callerOrg != existing.OrgID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "cannot modify another organization's retention policy"})
		return
	}

	var req models.UpdateRetentionPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	policy, err := h.Service.UpdatePolicy(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "retention policy")
		return
	}
	c.JSON(http.StatusOK, policy)
}

// DeleteRetentionPolicy handles DELETE /api/v1/retention-policies/:id.
func (h *RetentionHandler) DeleteRetentionPolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}

	existing, err := h.Service.GetPolicy(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "retention policy")
		return
	}
	if callerOrg := getOrgID(c); callerOrg != uuid.Nil && callerOrg != existing.OrgID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "cannot modify another organization's retention policy"})
		return
	}

	if err := h.Service.DeletePolicy(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "retention policy")
		return
	}
	c.Status(http.StatusNoContent)
}

// TriggerSweep handles POST /api/v1/retention-policies/sweep.
//
// v1 scope: this manual trigger IS the sweep mechanism -- there is no cron
// infra wired up in this environment to test against. RunRetentionSweep is
// written so a future scheduled job (an in-process ticker goroutine added
// to main.go, or an external cron hitting this same endpoint) can reuse it
// unchanged; only the trigger mechanism is missing, not the sweep logic.
func (h *RetentionHandler) TriggerSweep(c *gin.Context) {
	summary, err := h.Service.RunRetentionSweep(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "retention sweep")
		return
	}
	c.JSON(http.StatusOK, summary)
}
