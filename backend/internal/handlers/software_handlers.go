package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Software ---

// verifySoftwareOwnership: see verifyIncidentOwnership's doc comment in
// incident_handlers.go for the rationale -- GetSoftware, UpdateSoftware and
// DeleteSoftware all had this exact gap (fetch/mutate by ID with no org
// check at all) until a platform audit found it.
func (h *Handler) verifySoftwareOwnership(c *gin.Context, id uuid.UUID) (*models.SoftwareEntry, bool) {
	entry, err := h.Software.GetByID(c.Request.Context(), id)
	if err != nil || entry.OrgID != getOrgID(c) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return nil, false
	}
	return entry, true
}

func (h *Handler) ListSoftware(c *gin.Context) {
	page, perPage := getPagination(c)
	items, total, err := h.Software.List(c.Request.Context(), getOrgID(c), page, perPage)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *Handler) CreateSoftware(c *gin.Context) {
	var req models.CreateSoftwareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	entry, err := h.Software.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, entry)
}

func (h *Handler) GetSoftware(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	entry, ok := h.verifySoftwareOwnership(c, id)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, entry)
}

func (h *Handler) UpdateSoftware(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if _, ok := h.verifySoftwareOwnership(c, id); !ok {
		return
	}
	var req models.CreateSoftwareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	entry, err := h.Software.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, entry)
}

func (h *Handler) DeleteSoftware(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if _, ok := h.verifySoftwareOwnership(c, id); !ok {
		return
	}
	if err := h.Software.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}
