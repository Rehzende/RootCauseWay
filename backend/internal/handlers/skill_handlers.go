package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Skills ---

func (h *Handler) ListSkills(c *gin.Context) {
	page, perPage := getPagination(c)
	category := c.Query("category")
	items, total, err := h.Skills.List(c.Request.Context(), getOrgID(c), category, page, perPage)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *Handler) CreateSkill(c *gin.Context) {
	var req models.CreateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	skill, err := h.Skills.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, skill)
}

func (h *Handler) GetSkill(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	skill, err := h.Skills.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, skill)
}

func (h *Handler) UpdateSkill(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateSkillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	skill, err := h.Skills.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, skill)
}

func (h *Handler) DeleteSkill(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.Skills.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Agent Skills ---

func (h *Handler) LinkSkillToAgent(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateAgentSkillLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	link, err := h.AgentSkills.Link(c.Request.Context(), agentID, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, link)
}

func (h *Handler) UnlinkSkillFromAgent(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	skillID, err := uuid.Parse(c.Param("skillId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid skillId"})
		return
	}
	if err := h.AgentSkills.Unlink(c.Request.Context(), agentID, skillID); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListAgentSkills(c *gin.Context) {
	agentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	links, err := h.AgentSkills.ListByAgent(c.Request.Context(), agentID)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, links)
}

func (h *Handler) ListSkillAgents(c *gin.Context) {
	skillID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	links, err := h.AgentSkills.ListBySkill(c.Request.Context(), skillID)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, links)
}
