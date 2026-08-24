package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

type MarketplaceServiceInterface interface {
	Browse(ctx context.Context, category, search string) ([]models.MarketplaceAgent, error)
	GetDetails(ctx context.Context, slug string) (*models.MarketplaceAgent, error)
	Install(ctx context.Context, orgID uuid.UUID, slug string, req models.InstallAgentRequest) (*models.InstalledAgent, error)
	Uninstall(ctx context.Context, id uuid.UUID) error
	ListInstalled(ctx context.Context, orgID uuid.UUID) ([]models.InstalledAgent, error)
}

type MarketplaceHandler struct {
	Marketplace MarketplaceServiceInterface
}

func (h *MarketplaceHandler) ListMarketplaceAgents(c *gin.Context) {
	category := c.Query("category")
	search := c.Query("search")
	items, err := h.Marketplace.Browse(c.Request.Context(), category, search)
	if err != nil {
		handleDBError(c, err, "marketplace agent")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *MarketplaceHandler) GetMarketplaceAgent(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "slug required"})
		return
	}
	agent, err := h.Marketplace.GetDetails(c.Request.Context(), slug)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "agent not found"})
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (h *MarketplaceHandler) InstallAgent(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "slug required"})
		return
	}
	var req models.InstallAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	ia, err := h.Marketplace.Install(c.Request.Context(), getOrgID(c), slug, req)
	if err != nil {
		handleDBError(c, err, "marketplace agent")
		return
	}
	c.JSON(http.StatusCreated, ia)
}

func (h *MarketplaceHandler) UninstallAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Marketplace.Uninstall(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "installed agent")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *MarketplaceHandler) ListInstalledAgents(c *gin.Context) {
	items, err := h.Marketplace.ListInstalled(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "installed agent")
		return
	}
	c.JSON(http.StatusOK, items)
}
