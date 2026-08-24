package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/integrations/teams"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockTeamsOAuthSettings struct{ mock.Mock }

func (m *mockTeamsOAuthSettings) GetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID) (database.TeamsSettings, error) {
	args := m.Called(ctx, orgID)
	s, _ := args.Get(0).(database.TeamsSettings)
	return s, args.Error(1)
}

func (m *mockTeamsOAuthSettings) SetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID, s database.TeamsSettings) error {
	args := m.Called(ctx, orgID, s)
	return args.Error(0)
}

// withStubbedGraph mirrors the same-named helper in the teams package's own
// tests -- these are exported package vars (teams.GraphBaseURL /
// teams.MicrosoftLoginBaseURL), reachable from here specifically so
// TeamsOAuthService's HandleCallback can be exercised end to end without a
// real Microsoft tenant.
func withStubbedGraph(t *testing.T, srv *httptest.Server) {
	t.Helper()
	origGraph, origLogin := teams.GraphBaseURL, teams.MicrosoftLoginBaseURL
	teams.GraphBaseURL = srv.URL
	teams.MicrosoftLoginBaseURL = srv.URL
	t.Cleanup(func() {
		teams.GraphBaseURL = origGraph
		teams.MicrosoftLoginBaseURL = origLogin
	})
}

func TestTeamsOAuthService_InitiateConnect_ErrorsWhenAppRegistrationNotConfigured(t *testing.T) {
	settings := new(mockTeamsOAuthSettings)
	orgID := uuid.New()
	settings.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{}, nil)

	svc := NewTeamsOAuthService(settings, "https://api.example.com/callback")
	_, err := svc.InitiateConnect(context.Background(), orgID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "tenant ID, client ID and client secret")
}

func TestTeamsOAuthService_InitiateConnect_Success(t *testing.T) {
	settings := new(mockTeamsOAuthSettings)
	orgID := uuid.New()
	settings.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{
		TenantID: "my-tenant", ClientID: "my-client", ClientSecret: "shh",
	}, nil)

	svc := NewTeamsOAuthService(settings, "https://api.example.com/callback")
	authorizeURL, err := svc.InitiateConnect(context.Background(), orgID)

	require.NoError(t, err)
	u, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	assert.Equal(t, "/my-tenant/oauth2/v2.0/authorize", u.Path)
	assert.Equal(t, "my-client", u.Query().Get("client_id"))
	assert.Equal(t, "https://api.example.com/callback", u.Query().Get("redirect_uri"))
	assert.NotEmpty(t, u.Query().Get("state"))

	// The state must be tracked so a matching callback can complete.
	svc.mu.Lock()
	_, tracked := svc.stateStore[u.Query().Get("state")]
	svc.mu.Unlock()
	assert.True(t, tracked)
}

func TestTeamsOAuthService_HandleCallback_UnknownState_Errors(t *testing.T) {
	settings := new(mockTeamsOAuthSettings)
	svc := NewTeamsOAuthService(settings, "https://api.example.com/callback")

	_, err := svc.HandleCallback(context.Background(), "never-issued-state", "some-code")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or already-used state")
}

func TestTeamsOAuthService_HandleCallback_StateIsSingleUse(t *testing.T) {
	settings := new(mockTeamsOAuthSettings)
	orgID := uuid.New()
	settings.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{}, nil)
	svc := NewTeamsOAuthService(settings, "https://api.example.com/callback")

	svc.mu.Lock()
	svc.stateStore["reused-state"] = teamsOAuthStateEntry{orgID: orgID, createdAt: time.Now()}
	svc.mu.Unlock()

	// First use fails downstream (org settings changed mid-flow, since
	// GetOrgTeamsSettings returns an empty struct) but must still consume
	// the state.
	_, err := svc.HandleCallback(context.Background(), "reused-state", "code")
	require.Error(t, err)

	_, err = svc.HandleCallback(context.Background(), "reused-state", "code")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid or already-used state")
}

func TestTeamsOAuthService_HandleCallback_ExpiredState_Errors(t *testing.T) {
	settings := new(mockTeamsOAuthSettings)
	orgID := uuid.New()
	svc := NewTeamsOAuthService(settings, "https://api.example.com/callback")

	svc.mu.Lock()
	svc.stateStore["old-state"] = teamsOAuthStateEntry{orgID: orgID, createdAt: time.Now().Add(-11 * time.Minute)}
	svc.mu.Unlock()

	_, err := svc.HandleCallback(context.Background(), "old-state", "code")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
	settings.AssertNotCalled(t, "GetOrgTeamsSettings", mock.Anything, mock.Anything)
}

func TestTeamsOAuthService_HandleCallback_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/tenant-1/oauth2/v2.0/token":
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "authorization_code", r.PostForm.Get("grant_type"))
			assert.Equal(t, "https://api.example.com/callback", r.PostForm.Get("redirect_uri"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "access-1", "refresh_token": "refresh-1", "expires_in": 3600,
			})
		case r.URL.Path == "/me":
			_ = json.NewEncoder(w).Encode(map[string]any{"userPrincipalName": "rootcauseway-bot@customer.com"})
		default:
			t.Fatalf("unexpected request to %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	settings := new(mockTeamsOAuthSettings)
	orgID := uuid.New()
	svc := NewTeamsOAuthService(settings, "https://api.example.com/callback")

	svc.mu.Lock()
	svc.stateStore["good-state"] = teamsOAuthStateEntry{orgID: orgID, createdAt: time.Now()}
	svc.mu.Unlock()

	settings.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{
		TenantID: "tenant-1", ClientID: "client-1", ClientSecret: "secret-1",
	}, nil)
	settings.On("SetOrgTeamsSettings", mock.Anything, orgID, database.TeamsSettings{
		TenantID: "tenant-1", ClientID: "client-1", ClientSecret: "secret-1",
		RefreshToken: "refresh-1", ConnectedAccountEmail: "rootcauseway-bot@customer.com",
	}).Return(nil)

	gotOrgID, err := svc.HandleCallback(context.Background(), "good-state", "auth-code")

	require.NoError(t, err)
	assert.Equal(t, orgID, gotOrgID)
	settings.AssertExpectations(t)
}

func TestTeamsOAuthService_HandleCallback_ExchangeFailure_DoesNotPersist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer srv.Close()
	withStubbedGraph(t, srv)

	settings := new(mockTeamsOAuthSettings)
	orgID := uuid.New()
	svc := NewTeamsOAuthService(settings, "https://api.example.com/callback")

	svc.mu.Lock()
	svc.stateStore["state-1"] = teamsOAuthStateEntry{orgID: orgID, createdAt: time.Now()}
	svc.mu.Unlock()

	settings.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{
		TenantID: "t", ClientID: "c", ClientSecret: "s",
	}, nil)

	_, err := svc.HandleCallback(context.Background(), "state-1", "bad-code")

	require.Error(t, err)
	settings.AssertNotCalled(t, "SetOrgTeamsSettings", mock.Anything, mock.Anything, mock.Anything)
}

func TestTeamsOAuthService_HandleCallback_SettingsSaveFailure_Propagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer srv.Close()
	withStubbedGraph(t, srv)

	settings := new(mockTeamsOAuthSettings)
	orgID := uuid.New()
	svc := NewTeamsOAuthService(settings, "https://api.example.com/callback")

	svc.mu.Lock()
	svc.stateStore["state-1"] = teamsOAuthStateEntry{orgID: orgID, createdAt: time.Now()}
	svc.mu.Unlock()

	settings.On("GetOrgTeamsSettings", mock.Anything, orgID).Return(database.TeamsSettings{
		TenantID: "tenant-1", ClientID: "c", ClientSecret: "s",
	}, nil)
	settings.On("SetOrgTeamsSettings", mock.Anything, orgID, mock.Anything).Return(errors.New("db unavailable"))

	_, err := svc.HandleCallback(context.Background(), "state-1", "auth-code")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "save Teams connection")
}
