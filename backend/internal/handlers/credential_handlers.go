package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
)

// --- Credential Providers ---

func (h *Handler) ListProviders(c *gin.Context) {
	page, perPage := getPagination(c)
	items, total, err := h.CredentialProviders.List(c.Request.Context(), getOrgID(c), page, perPage)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *Handler) CreateProvider(c *gin.Context) {
	var req models.CreateCredentialProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	provider, err := h.CredentialProviders.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, provider)
}

func (h *Handler) GetProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	provider, err := h.CredentialProviders.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, provider)
}

func (h *Handler) UpdateProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateCredentialProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	provider, err := h.CredentialProviders.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, provider)
}

func (h *Handler) DeleteProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.CredentialProviders.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Resource Credentials ---

func (h *Handler) ListResourceCredentials(c *gin.Context) {
	softwareID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	// Found live during a full-pipeline test: this had no org ownership
	// check at all -- any caller passing another org's software UUID got
	// that software's resource credentials back. Same gap the
	// verifyIncidentOwnership/verifySoftwareOwnership pattern already
	// closed on incidents/software; this call site was missed.
	if _, ok := h.verifySoftwareOwnership(c, softwareID); !ok {
		return
	}
	items, err := h.ResourceCredentials.ListBySoftware(c.Request.Context(), softwareID)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) CreateResourceCredential(c *gin.Context) {
	var req models.CreateResourceCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	// Override software_id from URL param if present
	if sid := c.Param("id"); sid != "" {
		if id, err := uuid.Parse(sid); err == nil {
			req.SoftwareID = id
		}
	}
	rc, err := h.ResourceCredentials.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, rc)
}

func (h *Handler) GetResourceCredential(c *gin.Context) {
	id, err := uuid.Parse(c.Param("credId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid credId"})
		return
	}
	rc, err := h.ResourceCredentials.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, rc)
}

func (h *Handler) UpdateResourceCredential(c *gin.Context) {
	id, err := uuid.Parse(c.Param("credId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid credId"})
		return
	}
	var req models.CreateResourceCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	rc, err := h.ResourceCredentials.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, rc)
}

func (h *Handler) DeleteResourceCredential(c *gin.Context) {
	id, err := uuid.Parse(c.Param("credId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid credId"})
		return
	}
	if err := h.ResourceCredentials.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Access Policies ---

func (h *Handler) ListPolicies(c *gin.Context) {
	page, perPage := getPagination(c)
	items, total, err := h.AccessPolicies.List(c.Request.Context(), getOrgID(c), page, perPage)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *Handler) CreatePolicy(c *gin.Context) {
	var req models.CreateAccessPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	policy, err := h.AccessPolicies.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, policy)
}

func (h *Handler) GetPolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	policy, err := h.AccessPolicies.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "not found"})
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *Handler) UpdatePolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateAccessPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	policy, err := h.AccessPolicies.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *Handler) DeletePolicy(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.AccessPolicies.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Credential Leases ---

func (h *Handler) RequestLease(c *gin.Context) {
	var req models.RequestLeaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	lease, err := h.CredentialLeases.RequestLease(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		if errors.Is(err, services.ErrCredentialProviderNotImplemented) {
			// Fail loud and specific rather than falling through to a
			// generic masked 500 -- see resolveCredentialData's comment
			// for why this provider type isn't wired yet.
			c.JSON(http.StatusNotImplemented, models.ErrorResponse{Error: err.Error()})
			return
		}
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusCreated, lease)
}

func (h *Handler) RevokeLease(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	revokedBy := "system"
	if v, exists := c.Get("user_id"); exists {
		revokedBy = v.(uuid.UUID).String()
	}
	lease, err := h.CredentialLeases.RevokeLease(c.Request.Context(), id, revokedBy)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, lease)
}

func (h *Handler) ListLeases(c *gin.Context) {
	incidentID := c.Query("incident_id")
	if incidentID != "" {
		id, err := uuid.Parse(incidentID)
		if err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid incident_id"})
			return
		}
		leases, err := h.CredentialLeases.ListByIncident(c.Request.Context(), id)
		if err != nil {
			handleDBError(c, err, "resource")
			return
		}
		c.JSON(http.StatusOK, leases)
		return
	}
	// Default: list active for org
	leases, err := h.CredentialLeases.ListActive(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, leases)
}

func (h *Handler) ListActiveLeases(c *gin.Context) {
	leases, err := h.CredentialLeases.ListActive(c.Request.Context(), getOrgID(c))
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, leases)
}

// --- Internal: Evaluate Policies ---

func (h *Handler) EvaluatePolicies(c *gin.Context) {
	var req models.EvaluatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	policies, err := h.AccessPolicies.Evaluate(c.Request.Context(), getOrgID(c), req.AgentID, req.SkillID, req.ResourceType)
	if err != nil {
		handleDBError(c, err, "resource")
		return
	}
	c.JSON(http.StatusOK, policies)
}
