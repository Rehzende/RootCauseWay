package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Webhooks ---

func (h *Handler) ListWebhooks(c *gin.Context) {
	page, perPage := getPagination(c)
	items, total, err := h.Webhooks.List(c.Request.Context(), getOrgID(c), page, perPage)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *Handler) CreateWebhook(c *gin.Context) {
	var req models.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	webhook, err := h.Webhooks.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, webhook)
}

func (h *Handler) GetWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	webhook, err := h.Webhooks.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, webhook)
}

func (h *Handler) DeleteWebhook(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Webhooks.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}
