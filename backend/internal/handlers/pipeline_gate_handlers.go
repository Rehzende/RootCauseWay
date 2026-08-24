package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
)

// PipelineGateRepository persists the human-in-the-loop (HITL) pipeline gate
// state. Implemented by database.PgPipelineGateRepository; declared here as
// an interface so it can be mocked in handler tests.
type PipelineGateRepository interface {
	GetOrgHITLGateEnabled(ctx context.Context, orgID uuid.UUID) (bool, error)
	SetOrgHITLGateEnabled(ctx context.Context, orgID uuid.UUID, enabled bool) error
	MarkAwaitingApproval(ctx context.Context, incidentID uuid.UUID, stage string) error
	ApproveStage(ctx context.Context, incidentID uuid.UUID, approvedBy uuid.UUID) (orgID uuid.UUID, stage string, err error)
	GetOrgLLMSettings(ctx context.Context, orgID uuid.UUID) (database.LLMSettings, error)
	SetOrgLLMSettings(ctx context.Context, orgID uuid.UUID, settings database.LLMSettings) error
	GetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID) (database.TeamsSettings, error)
	SetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID, settings database.TeamsSettings) error
}

// PipelineGateHandler exposes the HITL gate endpoints. It is a standalone
// handler group (not a method set on *Handler) so it can be wired into
// main.go additively without modifying the shared Handler struct/
// constructor that other in-flight work also touches.
type PipelineGateHandler struct {
	Repo                PipelineGateRepository
	Publisher           EventPublisherInterface
	CredentialProviders CredentialProviderServiceInterface
}

func NewPipelineGateHandler(repo PipelineGateRepository, publisher EventPublisherInterface, credProviders CredentialProviderServiceInterface) *PipelineGateHandler {
	return &PipelineGateHandler{Repo: repo, Publisher: publisher, CredentialProviders: credProviders}
}

// getUserID extracts the authenticated user's ID from the gin context, set
// by the auth middleware (see middleware/auth.go: c.Set("user_id", ...)).
func getUserID(c *gin.Context) uuid.UUID {
	v, _ := c.Get("user_id")
	id, _ := v.(uuid.UUID)
	return id
}

// ApproveStageResponse is returned by POST /incidents/:id/approve-stage.
type ApproveStageResponse struct {
	IncidentID uuid.UUID `json:"incident_id"`
	Stage      string    `json:"stage"`
	ApprovedBy uuid.UUID `json:"approved_by"`
	Status     string    `json:"status"`
}

// ApproveStage handles POST /api/v1/incidents/:id/approve-stage.
//
// A human approves the stage the pipeline is currently paused on (e.g.
// "postmortem"). This clears the incident's awaiting_approval_stage,
// records approved_by/approved_at, and publishes a pipeline.stage_approved
// event so the agent-service's resume listener can pick the run back up.
func (h *PipelineGateHandler) ApproveStage(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid incident id"})
		return
	}

	userID := getUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "authentication required"})
		return
	}

	orgID, stage, err := h.Repo.ApproveStage(c.Request.Context(), incidentID, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusConflict, models.ErrorResponse{Error: "incident is not awaiting approval"})
			return
		}
		slog.Error("approve stage failed", "request_id", c.GetString("request_id"), "incident_id", incidentID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
		return
	}

	if h.Publisher != nil {
		envelope := models.EventEnvelope{
			EventID:   uuid.New(),
			EventType: "pipeline.stage_approved",
			OrgID:     orgID,
			Timestamp: time.Now(),
			Payload: map[string]any{
				"incident_id": incidentID,
				"stage":       stage,
				"approved_by": userID,
			},
		}
		channel := fmt.Sprintf("rootcauseway:%s:pipeline.stage_approved", orgID.String())
		if pubErr := h.Publisher.Publish(c.Request.Context(), channel, envelope); pubErr != nil {
			// Non-blocking: the approval itself already succeeded and was
			// persisted. Log and continue, matching the pattern used by
			// IngestionService for alert.received publishing.
			slog.Error("failed to publish pipeline.stage_approved", "incident_id", incidentID, "error", pubErr.Error())
		}
	}

	c.JSON(http.StatusOK, ApproveStageResponse{
		IncidentID: incidentID,
		Stage:      stage,
		ApprovedBy: userID,
		Status:     "approved",
	})
}

// UpdateOrgSettingsRequest is the request body for PATCH
// /api/v1/organizations/:id/settings.
type UpdateOrgSettingsRequest struct {
	PipelineHITLGateEnabled *bool   `json:"pipeline_hitl_gate_enabled,omitempty"`
	DefaultLLMProviderType  *string `json:"default_llm_provider_type,omitempty"`
	DefaultLLMBaseURL       *string `json:"default_llm_base_url,omitempty"`
	DefaultLLMModel         *string `json:"default_llm_model,omitempty"`
	DefaultLLMAPIKeyRef     *string `json:"default_llm_api_key_ref,omitempty"`
	// DefaultLLMCredentialProviderID, when set to a credential_providers
	// UUID, makes DefaultLLMAPIKeyRef a credential_path resolved through
	// that provider (see GetOrgSettingsInternal) instead of a literal key.
	// An explicitly-empty string clears it back to the literal-ref
	// behavior; omitted (nil) leaves whatever was set before untouched, per
	// this handler's usual partial-update semantics.
	DefaultLLMCredentialProviderID *string `json:"default_llm_credential_provider_id,omitempty"`

	// Microsoft Teams (Graph API) integration, used by War Room. Nil means
	// "leave unchanged" for every field below, same partial-update
	// semantics as the LLM fields above. TeamsClientSecret is write-only --
	// it is never echoed back by GetOrgSettings/UpdateOrgSettings, only a
	// teams_client_secret_set boolean is.
	//
	// Deliberately no TeamsConnectedAccount/refresh-token field here --
	// those are never user-typed, only set by the OAuth connect flow (see
	// TeamsOAuthHandler), so there's nothing for a PATCH to accept.
	TeamsTenantID     *string `json:"teams_tenant_id,omitempty"`
	TeamsClientID     *string `json:"teams_client_id,omitempty"`
	TeamsClientSecret *string `json:"teams_client_secret,omitempty"`
}

// UpdateOrgSettings handles PATCH /api/v1/organizations/:id/settings.
//
// Toggles pipeline_hitl_gate_enabled and/or the default LLM provider
// settings; additional org-level settings can be added to
// UpdateOrgSettingsRequest additively later.
func (h *PipelineGateHandler) UpdateOrgSettings(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid organization id"})
		return
	}

	// Tenant isolation: callers may only update their own org's settings.
	if callerOrg := getOrgID(c); callerOrg != uuid.Nil && callerOrg != orgID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "cannot modify another organization's settings"})
		return
	}

	var req UpdateOrgSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	llmFieldsProvided := req.DefaultLLMProviderType != nil || req.DefaultLLMBaseURL != nil ||
		req.DefaultLLMModel != nil || req.DefaultLLMAPIKeyRef != nil || req.DefaultLLMCredentialProviderID != nil
	teamsFieldsProvided := req.TeamsTenantID != nil || req.TeamsClientID != nil ||
		req.TeamsClientSecret != nil
	if req.PipelineHITLGateEnabled == nil && !llmFieldsProvided && !teamsFieldsProvided {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "no settings provided"})
		return
	}

	resp := gin.H{"org_id": orgID}

	if req.PipelineHITLGateEnabled != nil {
		if err := h.Repo.SetOrgHITLGateEnabled(c.Request.Context(), orgID, *req.PipelineHITLGateEnabled); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "organization not found"})
				return
			}
			slog.Error("update org settings failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
			return
		}
		resp["pipeline_hitl_gate_enabled"] = *req.PipelineHITLGateEnabled
	}

	if llmFieldsProvided {
		current, err := h.Repo.GetOrgLLMSettings(c.Request.Context(), orgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "organization not found"})
				return
			}
			slog.Error("get org llm settings failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
			return
		}
		if req.DefaultLLMProviderType != nil {
			current.ProviderType = *req.DefaultLLMProviderType
		}
		if req.DefaultLLMBaseURL != nil {
			current.BaseURL = *req.DefaultLLMBaseURL
		}
		if req.DefaultLLMModel != nil {
			current.Model = *req.DefaultLLMModel
		}
		if req.DefaultLLMAPIKeyRef != nil {
			current.APIKeyRef = *req.DefaultLLMAPIKeyRef
		}
		if req.DefaultLLMCredentialProviderID != nil {
			if *req.DefaultLLMCredentialProviderID == "" {
				current.CredentialProviderID = nil
			} else {
				id, err := uuid.Parse(*req.DefaultLLMCredentialProviderID)
				if err != nil {
					c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid default_llm_credential_provider_id"})
					return
				}
				current.CredentialProviderID = &id
			}
		}
		if err := h.Repo.SetOrgLLMSettings(c.Request.Context(), orgID, current); err != nil {
			slog.Error("update org llm settings failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
			return
		}
		resp["default_llm_provider_type"] = current.ProviderType
		resp["default_llm_base_url"] = current.BaseURL
		resp["default_llm_model"] = current.Model
		resp["default_llm_api_key_ref"] = current.APIKeyRef
		resp["default_llm_credential_provider_id"] = current.CredentialProviderID
	}

	if teamsFieldsProvided {
		current, err := h.Repo.GetOrgTeamsSettings(c.Request.Context(), orgID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "organization not found"})
				return
			}
			slog.Error("get org teams settings failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
			return
		}
		if req.TeamsTenantID != nil {
			current.TenantID = *req.TeamsTenantID
		}
		if req.TeamsClientID != nil {
			current.ClientID = *req.TeamsClientID
		}
		if req.TeamsClientSecret != nil {
			current.ClientSecret = *req.TeamsClientSecret
		}
		if err := h.Repo.SetOrgTeamsSettings(c.Request.Context(), orgID, current); err != nil {
			slog.Error("update org teams settings failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
			return
		}
		resp["teams_tenant_id"] = current.TenantID
		resp["teams_client_id"] = current.ClientID
		resp["teams_client_secret_set"] = current.ClientSecret != ""
		resp["teams_connected_account"] = current.ConnectedAccountEmail
		resp["teams_refresh_token_set"] = current.RefreshToken != ""
		resp["teams_configured"] = current.Configured()
	}

	c.JSON(http.StatusOK, resp)
}

// GetOrgSettings handles GET /api/v1/organizations/:id/settings.
//
// Public, JWT-authenticated counterpart to GetOrgSettingsInternal, used by
// the frontend to load the current pipeline_hitl_gate_enabled value (e.g.
// on Settings page mount) rather than only being able to write it blind.
func (h *PipelineGateHandler) GetOrgSettings(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid organization id"})
		return
	}

	// Tenant isolation: callers may only read their own org's settings.
	if callerOrg := getOrgID(c); callerOrg != uuid.Nil && callerOrg != orgID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "cannot read another organization's settings"})
		return
	}

	enabled, err := h.Repo.GetOrgHITLGateEnabled(c.Request.Context(), orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "organization not found"})
			return
		}
		slog.Error("get org settings failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
		return
	}
	llm, err := h.Repo.GetOrgLLMSettings(c.Request.Context(), orgID)
	if err != nil {
		slog.Error("get org llm settings failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
		return
	}
	teamsSettings, err := h.Repo.GetOrgTeamsSettings(c.Request.Context(), orgID)
	if err != nil {
		slog.Error("get org teams settings failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"org_id":                     orgID,
		"pipeline_hitl_gate_enabled": enabled,
		"default_llm_provider_type":  llm.ProviderType,
		"default_llm_base_url":       llm.BaseURL,
		"default_llm_model":          llm.Model,
		// The literal ref/path only, never a resolved secret -- this is
		// the public/JWT endpoint the frontend Settings page reads from.
		// GetOrgSettingsInternal below is where resolution happens, for
		// agent-service only.
		"default_llm_api_key_ref":            llm.APIKeyRef,
		"default_llm_credential_provider_id": llm.CredentialProviderID,
		// Same redaction rule as the LLM key, but stricter: the client
		// secret is never echoed at all, encrypted or not -- only whether
		// one is set.
		"teams_tenant_id":         teamsSettings.TenantID,
		"teams_client_id":         teamsSettings.ClientID,
		"teams_client_secret_set": teamsSettings.ClientSecret != "",
		"teams_connected_account": teamsSettings.ConnectedAccountEmail,
		"teams_refresh_token_set": teamsSettings.RefreshToken != "",
		"teams_configured":        teamsSettings.Configured(),
	})
}

// --- Internal endpoints (called by agent-service, no JWT; see main.go's
// /api/v1/internal group which trusts X-Org-ID instead) ---

// GetOrgSettingsInternal handles GET /internal/organizations/:id/settings.
// Used by agent-service's BackendClient.get_organization_settings() to
// decide whether to pause the pipeline before postmortem, and (as of the
// LLM & Tokens settings feature) to resolve the org's default LLM provider
// for each agent dispatch.
func (h *PipelineGateHandler) GetOrgSettingsInternal(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid organization id"})
		return
	}

	enabled, err := h.Repo.GetOrgHITLGateEnabled(c.Request.Context(), orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{Error: "organization not found"})
			return
		}
		slog.Error("get org settings failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
		return
	}
	llm, err := h.Repo.GetOrgLLMSettings(c.Request.Context(), orgID)
	if err != nil {
		slog.Error("get org llm settings failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
		return
	}

	// Resolve the real key through the credential vault when the org has
	// configured a provider for it -- previously default_llm_api_key_ref
	// was always used as-is here, i.e. handed to agent-service as a
	// literal secret value rather than a reference. Orgs that haven't set
	// default_llm_credential_provider_id keep the original literal-ref
	// behavior (nil check below) -- additive, not a breaking change.
	// Deliberately only done on this *internal* endpoint, never on the
	// public/JWT GetOrgSettings above -- that one serves the frontend
	// Settings page and must keep showing the reference/path, not a
	// resolved secret.
	apiKey := llm.APIKeyRef
	if llm.CredentialProviderID != nil {
		provider, err := h.CredentialProviders.GetByID(c.Request.Context(), *llm.CredentialProviderID)
		if err != nil {
			slog.Error("resolve org llm credential provider failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "failed to resolve LLM credential provider"})
			return
		}
		credentialData, err := services.ResolveCredentialData(c.Request.Context(), provider, llm.APIKeyRef, nil, 0)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, services.ErrCredentialProviderNotImplemented) {
				status = http.StatusNotImplemented
			}
			slog.Error("resolve org llm credential failed", "request_id", c.GetString("request_id"), "org_id", orgID, "provider_type", provider.ProviderType, "error", err.Error())
			c.JSON(status, models.ErrorResponse{Error: err.Error()})
			return
		}
		v, ok := credentialData["api_key"].(string)
		if !ok || v == "" {
			slog.Error("resolved credential has no api_key field", "request_id", c.GetString("request_id"), "org_id", orgID)
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "resolved credential missing api_key"})
			return
		}
		apiKey = v
	}

	c.JSON(http.StatusOK, gin.H{
		"org_id":                     orgID,
		"pipeline_hitl_gate_enabled": enabled,
		"default_llm_provider_type":  llm.ProviderType,
		"default_llm_base_url":       llm.BaseURL,
		"default_llm_model":          llm.Model,
		"default_llm_api_key_ref":    apiKey,
	})
}

// MarkAwaitingApprovalRequest is the body for POST
// /internal/incidents/:id/awaiting-approval.
type MarkAwaitingApprovalRequest struct {
	Stage string `json:"stage" binding:"required"`
}

// MarkAwaitingApprovalInternal handles POST
// /internal/incidents/:id/awaiting-approval. Used by agent-service's
// BackendClient.mark_awaiting_approval() when the orchestrator pauses the
// pipeline for human approval.
func (h *PipelineGateHandler) MarkAwaitingApprovalInternal(c *gin.Context) {
	incidentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid incident id"})
		return
	}

	var req MarkAwaitingApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}

	if err := h.Repo.MarkAwaitingApproval(c.Request.Context(), incidentID, req.Stage); err != nil {
		slog.Error("mark awaiting approval failed", "request_id", c.GetString("request_id"), "incident_id", incidentID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"incident_id":             incidentID,
		"awaiting_approval_stage": req.Stage,
	})
}
