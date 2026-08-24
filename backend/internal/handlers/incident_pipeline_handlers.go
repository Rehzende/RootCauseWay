package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Agent Runs ---

func (h *Handler) ListAgentRuns(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	runs, err := h.AgentRuns.ListByIncident(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, runs)
}

func (h *Handler) GetAgentRun(c *gin.Context) {
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid runId"})
		return
	}
	run, err := h.AgentRuns.GetByID(c.Request.Context(), runID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, run)
}

func (h *Handler) CreateAgentRun(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateAgentRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	run, err := h.AgentRuns.Create(c.Request.Context(), incidentID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, run)
}

func (h *Handler) UpdateAgentRun(c *gin.Context) {
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid runId"})
		return
	}
	var req models.UpdateAgentRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	run, err := h.AgentRuns.Update(c.Request.Context(), runID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, run)
}

func (h *Handler) RerunAgentRun(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	runID, err := uuid.Parse(c.Param("runId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid runId"})
		return
	}

	// 1. Get the original agent_run
	original, err := h.AgentRuns.GetByID(c.Request.Context(), runID)
	if err != nil {
		handleDBError(c, err, "agent_run")
		return
	}
	if original.IncidentID != incidentID {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "run does not belong to this incident"})
		return
	}

	// 2. Create a new agent_run with same agent_name/type but status "pending"
	newRun, err := h.AgentRuns.Create(c.Request.Context(), incidentID, models.CreateAgentRunRequest{
		AgentID:     original.AgentID,
		AgentName:   original.AgentName,
		AgentType:   original.AgentType,
		ParentRunID: original.ParentRunID,
		InputData:   original.InputData,
		ModelUsed:   original.ModelUsed,
	})
	if err != nil {
		handleDBError(c, err, "agent_run")
		return
	}

	// 3. Publish a Redis event for rerun
	if h.EventPublisher != nil {
		orgID := getOrgID(c)
		channel := fmt.Sprintf("rootcauseway:%s:agent.rerun", orgID.String())
		event := models.EventEnvelope{
			EventID:   uuid.New(),
			EventType: "agent.rerun",
			OrgID:     orgID,
			Timestamp: time.Now(),
			Payload: map[string]interface{}{
				"incident_id":     incidentID.String(),
				"original_run_id": runID.String(),
				"new_run_id":      newRun.ID.String(),
				"agent_name":      newRun.AgentName,
				"agent_type":      newRun.AgentType,
			},
		}
		if pubErr := h.EventPublisher.Publish(c.Request.Context(), channel, event); pubErr != nil {
			slog.Error("failed to publish rerun event", "error", pubErr.Error())
		}
	}

	// 4. Return the new agent_run
	c.JSON(http.StatusCreated, newRun)
}

func (h *Handler) GetIncidentDAG(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	dag, err := h.AgentRuns.GetDAG(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, dag)
}

// --- RCI ---

func (h *Handler) CreateRCI(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateRCIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	rci, err := h.RCI.Create(c.Request.Context(), incidentID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, rci)
}

func (h *Handler) GetRCI(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	rci, err := h.RCI.GetByIncidentID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, rci)
}

func (h *Handler) UpdateRCI(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateRCIRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	rci, err := h.RCI.Update(c.Request.Context(), incidentID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, rci)
}

// --- RCA ---

func (h *Handler) CreateRCA(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateRCARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	rca, err := h.RCA.Create(c.Request.Context(), incidentID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, rca)
}

func (h *Handler) GetRCA(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	rca, err := h.RCA.GetByIncidentID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, rca)
}

func (h *Handler) UpdateRCA(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateRCARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	rca, err := h.RCA.Update(c.Request.Context(), incidentID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, rca)
}

// --- Postmortem ---

func (h *Handler) CreatePostmortem(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreatePostmortemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	pm, err := h.Postmortem.Create(c.Request.Context(), incidentID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, pm)
}

func (h *Handler) GetPostmortem(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	pm, err := h.Postmortem.GetByIncidentID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, pm)
}

func (h *Handler) UpdatePostmortem(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreatePostmortemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	pm, err := h.Postmortem.Update(c.Request.Context(), incidentID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, pm)
}
