package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// SLOServiceInterface is the service contract the handler depends on. Kept
// as a narrow interface (rather than importing *services.SLOService
// directly) so handler tests can mock it, matching the convention used by
// the other feature handlers in this package.
type SLOServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateSLODefinitionRequest) (*models.SLODefinition, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.SLODefinition, error)
	List(ctx context.Context, orgID uuid.UUID) ([]models.SLODefinition, error)
	Update(ctx context.Context, id uuid.UUID, req models.UpdateSLODefinitionRequest) (*models.SLODefinition, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CalculateSLOStatus(ctx context.Context, sloDefinitionID uuid.UUID) (*models.SLOStatus, error)
	SoftwareSLOStatus(ctx context.Context, softwareID uuid.UUID) (*models.SoftwareSLOStatus, error)
}

type SLOHandler struct {
	SLOs SLOServiceInterface
}

func NewSLOHandler(svc SLOServiceInterface) *SLOHandler {
	return &SLOHandler{SLOs: svc}
}

// --- CRUD ---

func (h *SLOHandler) ListSLODefinitions(c *gin.Context) {
	items, err := h.SLOs.List(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "slo_definition")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *SLOHandler) CreateSLODefinition(c *gin.Context) {
	var req models.CreateSLODefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	def, err := h.SLOs.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "slo_definition")
		return
	}
	c.JSON(http.StatusCreated, def)
}

func (h *SLOHandler) GetSLODefinition(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	def, err := h.SLOs.GetByID(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "slo_definition")
		return
	}
	c.JSON(http.StatusOK, def)
}

func (h *SLOHandler) UpdateSLODefinition(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.UpdateSLODefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	def, err := h.SLOs.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "slo_definition")
		return
	}
	c.JSON(http.StatusOK, def)
}

func (h *SLOHandler) DeleteSLODefinition(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.SLOs.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "slo_definition")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Status ---

func (h *SLOHandler) GetSLOStatus(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	status, err := h.SLOs.CalculateSLOStatus(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "slo_definition")
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *SLOHandler) GetSoftwareSLOStatus(c *gin.Context) {
	softwareID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	status, err := h.SLOs.SoftwareSLOStatus(c.Request.Context(), softwareID)
	if err != nil {
		handleDBError(c, err, "software")
		return
	}
	c.JSON(http.StatusOK, status)
}
