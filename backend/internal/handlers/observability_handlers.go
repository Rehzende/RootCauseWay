package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// Service interfaces for observability

type ObservabilitySourceServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateObservabilitySourceRequest) (*models.ObservabilitySource, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.ObservabilitySource, error)
	List(ctx context.Context, orgID uuid.UUID, sourceType string) ([]models.ObservabilitySource, error)
	ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.ObservabilitySource, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateObservabilitySourceRequest) (*models.ObservabilitySource, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CheckHealth(ctx context.Context, id uuid.UUID) (*models.ObservabilitySource, error)
}

type SnapshotConfigServiceInterface interface {
	Create(ctx context.Context, orgID uuid.UUID, req models.CreateSnapshotConfigRequest) (*models.SnapshotConfig, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.SnapshotConfig, error)
	ListBySource(ctx context.Context, sourceID uuid.UUID) ([]models.SnapshotConfig, error)
	ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.SnapshotConfig, error)
	Update(ctx context.Context, id uuid.UUID, req models.CreateSnapshotConfigRequest) (*models.SnapshotConfig, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type ObservabilityHandler struct {
	Sources   ObservabilitySourceServiceInterface
	Snapshots SnapshotConfigServiceInterface
}

// --- Sources ---

func (h *ObservabilityHandler) ListSources(c *gin.Context) {
	sourceType := c.Query("source_type")
	items, err := h.Sources.List(c.Request.Context(), getOrgID(c), sourceType)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *ObservabilityHandler) CreateSource(c *gin.Context) {
	var req models.CreateObservabilitySourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	src, err := h.Sources.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, src)
}

func (h *ObservabilityHandler) GetSource(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	src, err := h.Sources.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, src)
}

func (h *ObservabilityHandler) UpdateSource(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateObservabilitySourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	src, err := h.Sources.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, src)
}

func (h *ObservabilityHandler) DeleteSource(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Sources.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ObservabilityHandler) CheckSourceHealth(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	src, err := h.Sources.CheckHealth(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, src)
}

func (h *ObservabilityHandler) ListSoftwareObservability(c *gin.Context) {
	softwareID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	items, err := h.Sources.ListBySoftware(c.Request.Context(), softwareID)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

// --- Snapshot Configs ---

func (h *ObservabilityHandler) ListSnapshotConfigs(c *gin.Context) {
	sourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	items, err := h.Snapshots.ListBySource(c.Request.Context(), sourceID)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *ObservabilityHandler) CreateSnapshotConfig(c *gin.Context) {
	sourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateSnapshotConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	req.SourceID = sourceID
	sc, err := h.Snapshots.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, sc)
}

func (h *ObservabilityHandler) GetSnapshotConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	sc, err := h.Snapshots.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, sc)
}

func (h *ObservabilityHandler) UpdateSnapshotConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateSnapshotConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	sc, err := h.Snapshots.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, sc)
}

func (h *ObservabilityHandler) DeleteSnapshotConfig(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Snapshots.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}
