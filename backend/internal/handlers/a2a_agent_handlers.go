package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- A2A Agents ---

func (h *Handler) ListA2AAgents(c *gin.Context) {
	page, perPage := getPagination(c)
	agentType := c.Query("type")
	items, total, err := h.A2AAgents.List(c.Request.Context(), getOrgID(c), agentType, page, perPage)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *Handler) CreateA2AAgent(c *gin.Context) {
	var req models.CreateA2AAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	agent, err := h.A2AAgents.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, agent)
}

func (h *Handler) GetA2AAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	agent, err := h.A2AAgents.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (h *Handler) UpdateA2AAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateA2AAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	agent, err := h.A2AAgents.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (h *Handler) DeleteA2AAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	agent, err := h.A2AAgents.GetByID(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "agent")
		return
	}
	if agent.IsSystem {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "system agents cannot be deleted"})
		return
	}
	if err := h.A2AAgents.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) GetA2AAgentCard(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	agent, err := h.A2AAgents.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.Data(http.StatusOK, "application/json", agent.AgentCard)
}

func (h *Handler) HealthCheckA2AAgent(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	agent, err := h.A2AAgents.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}

	// Try to reach the agent's health endpoint
	healthStatus := "unhealthy"
	client := &http.Client{Timeout: 5 * time.Second}
	healthURL := agent.EndpointURL + "/health"
	resp, err := client.Get(healthURL)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			healthStatus = "healthy"
		}
	}

	// Update health status
	if err := h.A2AAgents.HealthCheck(c.Request.Context(), id, healthStatus); err != nil {
		handleDBError(c, err, "agent")
		return
	}

	// Return updated agent
	agent, err = h.A2AAgents.GetByID(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "agent")
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (h *Handler) HealthCheckAllA2AAgents(c *gin.Context) {
	agents, _, err := h.A2AAgents.List(c.Request.Context(), getOrgID(c), "", 1, 100)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	results := make([]models.A2AAgent, 0, len(agents))

	for _, agent := range agents {
		healthStatus := "unhealthy"
		healthURL := agent.EndpointURL + "/health"
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				healthStatus = "healthy"
			}
		}
		_ = h.A2AAgents.HealthCheck(c.Request.Context(), agent.ID, healthStatus)
		agent.HealthStatus = healthStatus
		now := time.Now()
		agent.LastHealthCheck = &now
		results = append(results, agent)
	}

	c.JSON(http.StatusOK, results)
}
