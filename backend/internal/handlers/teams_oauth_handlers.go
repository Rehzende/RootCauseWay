package handlers

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
)

// TeamsOAuthHandler exposes the delegated Teams OAuth connect flow (see
// services.TeamsOAuthService and migration 027_teams_oauth) -- replaces the
// old app-only auth path that needed a tenant admin to grant a Microsoft
// Application Access Policy via PowerShell.
type TeamsOAuthHandler struct {
	Svc             *services.TeamsOAuthService
	FrontendBaseURL string // e.g. "https://rootcauseway.example.com", no trailing slash
}

func NewTeamsOAuthHandler(svc *services.TeamsOAuthService, frontendBaseURL string) *TeamsOAuthHandler {
	return &TeamsOAuthHandler{Svc: svc, FrontendBaseURL: frontendBaseURL}
}

// Authorize handles POST /api/v1/organizations/:id/integrations/teams/oauth/authorize.
// Protected (JWT), same tenant-isolation rule as UpdateOrgSettings. Returns
// the Microsoft authorize URL for the frontend to navigate the browser to
// (a plain fetch/XHR can't be redirected through Microsoft's consent
// screen itself -- window.location.href has to do that part).
func (h *TeamsOAuthHandler) Authorize(c *gin.Context) {
	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid organization id"})
		return
	}

	if callerOrg := getOrgID(c); callerOrg != uuid.Nil && callerOrg != orgID {
		c.JSON(http.StatusForbidden, models.ErrorResponse{Error: "cannot connect Teams for another organization"})
		return
	}

	authorizeURL, err := h.Svc.InitiateConnect(c.Request.Context(), orgID)
	if err != nil {
		slog.Error("initiate Teams OAuth connect failed", "request_id", c.GetString("request_id"), "org_id", orgID, "error", err.Error())
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"authorize_url": authorizeURL})
}

// Callback handles GET /api/v1/integrations/teams/oauth/callback. Public --
// Microsoft's redirect carries no RootCauseway JWT, same reasoning as SSOCallback.
// Always redirects the browser back to the frontend Settings page (never
// raw JSON), on both success and failure, so the user lands somewhere
// meaningful either way.
func (h *TeamsOAuthHandler) Callback(c *gin.Context) {
	if oauthErr := c.Query("error"); oauthErr != "" {
		desc := c.Query("error_description")
		h.redirectWithError(c, "Microsoft declined the connection: "+firstNonEmpty(desc, oauthErr))
		return
	}

	state := c.Query("state")
	code := c.Query("code")
	if state == "" || code == "" {
		h.redirectWithError(c, "missing state or code in Microsoft's response")
		return
	}

	if _, err := h.Svc.HandleCallback(c.Request.Context(), state, code); err != nil {
		slog.Error("Teams OAuth callback failed", "request_id", c.GetString("request_id"), "error", err.Error())
		h.redirectWithError(c, "failed to complete the connection, please try again")
		return
	}

	c.Redirect(http.StatusFound, h.FrontendBaseURL+"/settings?tab=integrations&teams_connected=true")
}

func (h *TeamsOAuthHandler) redirectWithError(c *gin.Context, message string) {
	c.Redirect(http.StatusFound, h.FrontendBaseURL+"/settings?tab=integrations&teams_error="+url.QueryEscape(message))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
