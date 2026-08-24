package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- A2A Tasks ---

func (h *Handler) ListA2ATasks(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	tasks, err := h.A2ATasks.ListByIncident(c.Request.Context(), incidentID)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, tasks)
}

func (h *Handler) CreateA2ATask(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateA2ATaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	task, err := h.A2ATasks.Create(c.Request.Context(), incidentID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, task)
}

func (h *Handler) GetA2ATask(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("taskId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid taskId"})
		return
	}
	task, err := h.A2ATasks.GetByID(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (h *Handler) UpdateA2ATask(c *gin.Context) {
	taskID, err := uuid.Parse(c.Param("taskId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid taskId"})
		return
	}
	var req models.UpdateA2ATaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	task, err := h.A2ATasks.Update(c.Request.Context(), taskID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, task)
}

// --- Orchestrator Decisions ---

func (h *Handler) ListOrchestratorDecisions(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	decisions, err := h.OrchestratorDecisions.ListByIncident(c.Request.Context(), incidentID)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, decisions)
}

func (h *Handler) CreateOrchestratorDecision(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateOrchestratorDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	decision, err := h.OrchestratorDecisions.Create(c.Request.Context(), incidentID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, decision)
}
