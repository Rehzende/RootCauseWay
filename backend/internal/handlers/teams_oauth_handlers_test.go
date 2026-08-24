package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/integrations/teams"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockTeamsOAuthRepo satisfies whatever unexported settings interface
// services.NewTeamsOAuthService expects (GetOrgTeamsSettings/
// SetOrgTeamsSettings) structurally -- Go resolves that at the call site,
// no need to name the interface from this package.
type mockTeamsOAuthRepo struct{ mock.Mock }

func (m *mockTeamsOAuthRepo) GetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID) (database.TeamsSettings, error) {
	args := m.Called(ctx, orgID)
	s, _ := args.Get(0).(database.TeamsSettings)
	return s, args.Error(1)
}

func (m *mockTeamsOAuthRepo) SetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID, s database.TeamsSettings) error {
	args := m.Called(ctx, orgID, s)
	return args.Error(0)
}

func setupTeamsOAuthRouter(toh *TeamsOAuthHandler, callerOrg uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if callerOrg != uuid.Nil {
			c.Set("org_id", callerOrg)
		}
		c.Next()
	})
	api := r.Group("/api/v1")
	api.POST("/organizations/:id/integrations/teams/oauth/authorize", toh.Authorize)
	api.GET("/integrations/teams/oauth/callback", toh.Callback)
	return r
}

func TestTeamsOAuthHandler_Authorize_ReturnsURL(t *testing.T) {
	repo := new(mockTeamsOAuthRepo)
	orgID := uuid.New()
	repo.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{
		TenantID: "t", ClientID: "c", ClientSecret: "s",
	}, nil)

	svc := services.NewTeamsOAuthService(repo, "https://api.example.com/callback")
	toh := NewTeamsOAuthHandler(svc, "https://app.example.com")
	r := setupTeamsOAuthRouter(toh, orgID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/integrations/teams/oauth/authorize", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["authorize_url"], "https://login.microsoftonline.com/t/oauth2/v2.0/authorize")
}

func TestTeamsOAuthHandler_Authorize_RejectsOtherOrg(t *testing.T) {
	repo := new(mockTeamsOAuthRepo)
	orgID := uuid.New()
	callerOrg := uuid.New() // different org than the path param

	svc := services.NewTeamsOAuthService(repo, "https://api.example.com/callback")
	toh := NewTeamsOAuthHandler(svc, "https://app.example.com")
	r := setupTeamsOAuthRouter(toh, callerOrg)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/integrations/teams/oauth/authorize", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	repo.AssertNotCalled(t, "GetOrgTeamsSettings", mock.Anything, mock.Anything)
}

func TestTeamsOAuthHandler_Authorize_NotConfigured_ReturnsBadRequest(t *testing.T) {
	repo := new(mockTeamsOAuthRepo)
	orgID := uuid.New()
	repo.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{}, nil)

	svc := services.NewTeamsOAuthService(repo, "https://api.example.com/callback")
	toh := NewTeamsOAuthHandler(svc, "https://app.example.com")
	r := setupTeamsOAuthRouter(toh, orgID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/integrations/teams/oauth/authorize", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTeamsOAuthHandler_Callback_MicrosoftDeniedConsent_RedirectsWithError(t *testing.T) {
	repo := new(mockTeamsOAuthRepo)
	svc := services.NewTeamsOAuthService(repo, "https://api.example.com/callback")
	toh := NewTeamsOAuthHandler(svc, "https://app.example.com")
	r := setupTeamsOAuthRouter(toh, uuid.Nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/integrations/teams/oauth/callback?error=access_denied&error_description=User+declined", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusFound, w.Code)
	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/settings", loc.Path)
	assert.Contains(t, loc.Query().Get("teams_error"), "User declined")
}

func TestTeamsOAuthHandler_Callback_MissingCodeOrState_RedirectsWithError(t *testing.T) {
	repo := new(mockTeamsOAuthRepo)
	svc := services.NewTeamsOAuthService(repo, "https://api.example.com/callback")
	toh := NewTeamsOAuthHandler(svc, "https://app.example.com")
	r := setupTeamsOAuthRouter(toh, uuid.Nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/integrations/teams/oauth/callback?state=only-state", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusFound, w.Code)
	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	assert.NotEmpty(t, loc.Query().Get("teams_error"))
}

func TestTeamsOAuthHandler_Callback_ServiceError_RedirectsWithError(t *testing.T) {
	repo := new(mockTeamsOAuthRepo)
	svc := services.NewTeamsOAuthService(repo, "https://api.example.com/callback")
	toh := NewTeamsOAuthHandler(svc, "https://app.example.com")
	r := setupTeamsOAuthRouter(toh, uuid.Nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/integrations/teams/oauth/callback?state=unknown-state&code=abc", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusFound, w.Code)
	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/settings", loc.Path)
	assert.NotEmpty(t, loc.Query().Get("teams_error"))
}

func TestTeamsOAuthHandler_Callback_Success_RedirectsWithConnectedFlag(t *testing.T) {
	graphSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/tenant-1/oauth2/v2.0/token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-1", "refresh_token": "refresh-1", "expires_in": 3600,
			})
		case r.URL.Path == "/me":
			_ = json.NewEncoder(w).Encode(map[string]any{"userPrincipalName": "rootcauseway-bot@customer.com"})
		}
	}))
	defer graphSrv.Close()
	origGraph, origLogin := teams.GraphBaseURL, teams.MicrosoftLoginBaseURL
	teams.GraphBaseURL = graphSrv.URL
	teams.MicrosoftLoginBaseURL = graphSrv.URL
	t.Cleanup(func() {
		teams.GraphBaseURL = origGraph
		teams.MicrosoftLoginBaseURL = origLogin
	})

	repo := new(mockTeamsOAuthRepo)
	orgID := uuid.New()
	repo.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{
		TenantID: "tenant-1", ClientID: "client-1", ClientSecret: "secret-1",
	}, nil)
	repo.On("SetOrgTeamsSettings", mock.Anything, orgID, database.TeamsSettings{
		TenantID: "tenant-1", ClientID: "client-1", ClientSecret: "secret-1",
		RefreshToken: "refresh-1", ConnectedAccountEmail: "rootcauseway-bot@customer.com",
	}).Return(nil)

	svc := services.NewTeamsOAuthService(repo, "https://api.example.com/callback")
	toh := NewTeamsOAuthHandler(svc, "https://app.example.com")
	r := setupTeamsOAuthRouter(toh, orgID)

	// Drive InitiateConnect for real so a genuine, tracked state exists --
	// more realistic than reaching into the service's private state store.
	authorizeURL, err := svc.InitiateConnect(context.Background(), orgID)
	require.NoError(t, err)
	state := mustParseQueryParam(t, authorizeURL, "state")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/integrations/teams/oauth/callback?state="+url.QueryEscape(state)+"&code=auth-code", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusFound, w.Code)
	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/settings", loc.Path)
	assert.Equal(t, "true", loc.Query().Get("teams_connected"))
	repo.AssertExpectations(t)
}

func mustParseQueryParam(t *testing.T, rawURL, param string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	require.NoError(t, err)
	return u.Query().Get(param)
}
