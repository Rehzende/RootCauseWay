package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type MockPipelineGateRepo struct{ mock.Mock }

func (m *MockPipelineGateRepo) GetOrgHITLGateEnabled(ctx context.Context, orgID uuid.UUID) (bool, error) {
	args := m.Called(ctx, orgID)
	return args.Bool(0), args.Error(1)
}
func (m *MockPipelineGateRepo) SetOrgHITLGateEnabled(ctx context.Context, orgID uuid.UUID, enabled bool) error {
	return m.Called(ctx, orgID, enabled).Error(0)
}
func (m *MockPipelineGateRepo) MarkAwaitingApproval(ctx context.Context, incidentID uuid.UUID, stage string) error {
	return m.Called(ctx, incidentID, stage).Error(0)
}
func (m *MockPipelineGateRepo) ApproveStage(ctx context.Context, incidentID uuid.UUID, approvedBy uuid.UUID) (uuid.UUID, string, error) {
	args := m.Called(ctx, incidentID, approvedBy)
	orgID, _ := args.Get(0).(uuid.UUID)
	return orgID, args.String(1), args.Error(2)
}
func (m *MockPipelineGateRepo) GetOrgLLMSettings(ctx context.Context, orgID uuid.UUID) (database.LLMSettings, error) {
	args := m.Called(ctx, orgID)
	s, _ := args.Get(0).(database.LLMSettings)
	return s, args.Error(1)
}
func (m *MockPipelineGateRepo) SetOrgLLMSettings(ctx context.Context, orgID uuid.UUID, settings database.LLMSettings) error {
	return m.Called(ctx, orgID, settings).Error(0)
}
func (m *MockPipelineGateRepo) GetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID) (database.TeamsSettings, error) {
	args := m.Called(ctx, orgID)
	s, _ := args.Get(0).(database.TeamsSettings)
	return s, args.Error(1)
}
func (m *MockPipelineGateRepo) SetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID, settings database.TeamsSettings) error {
	return m.Called(ctx, orgID, settings).Error(0)
}

type MockGatePublisher struct{ mock.Mock }

func (m *MockGatePublisher) Publish(ctx context.Context, channel string, event models.EventEnvelope) error {
	return m.Called(ctx, channel, event).Error(0)
}

type MockCredentialProviderSvc struct{ mock.Mock }

func (m *MockCredentialProviderSvc) Create(ctx context.Context, orgID uuid.UUID, req models.CreateCredentialProviderRequest) (*models.CredentialProvider, error) {
	args := m.Called(ctx, orgID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CredentialProvider), args.Error(1)
}
func (m *MockCredentialProviderSvc) GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialProvider, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CredentialProvider), args.Error(1)
}
func (m *MockCredentialProviderSvc) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.CredentialProvider, int, error) {
	args := m.Called(ctx, orgID, page, perPage)
	return args.Get(0).([]models.CredentialProvider), args.Int(1), args.Error(2)
}
func (m *MockCredentialProviderSvc) Update(ctx context.Context, id uuid.UUID, req models.CreateCredentialProviderRequest) (*models.CredentialProvider, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CredentialProvider), args.Error(1)
}
func (m *MockCredentialProviderSvc) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// --- Test helpers ---

var (
	gateTestOrgID  = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	gateTestUserID = uuid.MustParse("00000000-0000-0000-0000-000000000002")
)

func setupPipelineGateRouter(pgh *PipelineGateHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Set("org_id", gateTestOrgID)
		c.Set("user_id", gateTestUserID)
		c.Next()
	})

	api := r.Group("/api/v1")
	api.POST("/incidents/:id/approve-stage", pgh.ApproveStage)
	api.GET("/organizations/:id/settings", pgh.GetOrgSettings)
	api.PATCH("/organizations/:id/settings", pgh.UpdateOrgSettings)

	internal := r.Group("/api/v1/internal")
	internal.GET("/organizations/:id/settings", pgh.GetOrgSettingsInternal)
	internal.POST("/incidents/:id/awaiting-approval", pgh.MarkAwaitingApprovalInternal)

	return r
}

// --- Tests ---

func TestApproveStage_Success(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	pub := new(MockGatePublisher)
	pgh := NewPipelineGateHandler(repo, pub, nil)
	r := setupPipelineGateRouter(pgh)

	incidentID := uuid.New()
	repo.On("ApproveStage", mock.Anything, incidentID, gateTestUserID).
		Return(gateTestOrgID, "postmortem", nil)
	pub.On("Publish", mock.Anything, "rootcauseway:"+gateTestOrgID.String()+":pipeline.stage_approved", mock.AnythingOfType("models.EventEnvelope")).
		Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/incidents/"+incidentID.String()+"/approve-stage", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp ApproveStageResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, incidentID, resp.IncidentID)
	assert.Equal(t, "postmortem", resp.Stage)
	assert.Equal(t, gateTestUserID, resp.ApprovedBy)
	assert.Equal(t, "approved", resp.Status)

	repo.AssertExpectations(t)
	pub.AssertExpectations(t)
}

func TestApproveStage_NotAwaitingApproval(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	pub := new(MockGatePublisher)
	pgh := NewPipelineGateHandler(repo, pub, nil)
	r := setupPipelineGateRouter(pgh)

	incidentID := uuid.New()
	repo.On("ApproveStage", mock.Anything, incidentID, gateTestUserID).
		Return(uuid.Nil, "", pgx.ErrNoRows)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/incidents/"+incidentID.String()+"/approve-stage", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	pub.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything)
}

func TestApproveStage_InvalidIncidentID(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	pgh := NewPipelineGateHandler(repo, nil, nil)
	r := setupPipelineGateRouter(pgh)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/incidents/not-a-uuid/approve-stage", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateOrgSettings_Success(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	pgh := NewPipelineGateHandler(repo, nil, nil)
	r := setupPipelineGateRouter(pgh)

	repo.On("SetOrgHITLGateEnabled", mock.Anything, gateTestOrgID, true).Return(nil)

	body, _ := json.Marshal(UpdateOrgSettingsRequest{PipelineHITLGateEnabled: boolPtr(true)})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/organizations/"+gateTestOrgID.String()+"/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestUpdateOrgSettings_ForbiddenCrossOrg(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	pgh := NewPipelineGateHandler(repo, nil, nil)
	r := setupPipelineGateRouter(pgh)

	otherOrgID := uuid.New()
	body, _ := json.Marshal(UpdateOrgSettingsRequest{PipelineHITLGateEnabled: boolPtr(true)})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/organizations/"+otherOrgID.String()+"/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	repo.AssertNotCalled(t, "SetOrgHITLGateEnabled", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateOrgSettings_NoFieldsProvided(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	pgh := NewPipelineGateHandler(repo, nil, nil)
	r := setupPipelineGateRouter(pgh)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/organizations/"+gateTestOrgID.String()+"/settings", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetOrgSettingsInternal_Success(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	pgh := NewPipelineGateHandler(repo, nil, nil)
	r := setupPipelineGateRouter(pgh)

	repo.On("GetOrgHITLGateEnabled", mock.Anything, gateTestOrgID).Return(true, nil)
	repo.On("GetOrgLLMSettings", mock.Anything, gateTestOrgID).Return(database.LLMSettings{
		ProviderType: "lm_studio", BaseURL: "http://lm-studio:1234/v1", Model: "qwen2.5-coder-14b",
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/internal/organizations/"+gateTestOrgID.String()+"/settings", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["pipeline_hitl_gate_enabled"])
	assert.Equal(t, "lm_studio", body["default_llm_provider_type"])
	assert.Equal(t, "qwen2.5-coder-14b", body["default_llm_model"])
	assert.Equal(t, "", body["default_llm_api_key_ref"], "no credential provider configured -- literal ref (empty in this fixture) passes through unchanged")
}

// TestGetOrgSettingsInternal_ResolvesLLMKeyThroughCredentialProvider guards
// the fix for a bug found live: default_llm_api_key_ref used to always be
// handed to agent-service as-is, i.e. as a literal secret value, never
// resolved through the credential vault. When the org has configured
// default_llm_credential_provider_id, the response's
// default_llm_api_key_ref must be the *resolved* key (from the provider's
// config), not the stored credential_path.
func TestGetOrgSettingsInternal_ResolvesLLMKeyThroughCredentialProvider(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	credProviders := new(MockCredentialProviderSvc)
	pgh := NewPipelineGateHandler(repo, nil, credProviders)
	r := setupPipelineGateRouter(pgh)

	providerID := uuid.New()
	repo.On("GetOrgHITLGateEnabled", mock.Anything, gateTestOrgID).Return(true, nil)
	repo.On("GetOrgLLMSettings", mock.Anything, gateTestOrgID).Return(database.LLMSettings{
		ProviderType: "openai", BaseURL: "https://openrouter.ai/api/v1", Model: "anthropic/claude-sonnet-4-6",
		APIKeyRef: "org/default-llm-key", CredentialProviderID: &providerID,
	}, nil)
	credProviders.On("GetByID", mock.Anything, providerID).Return(&models.CredentialProvider{
		ID: providerID, ProviderType: "static",
		Config: json.RawMessage(`{"credentials":{"api_key":"sk-or-v1-real-secret-value"}}`),
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/internal/organizations/"+gateTestOrgID.String()+"/settings", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "sk-or-v1-real-secret-value", body["default_llm_api_key_ref"])
}

// TestGetOrgSettingsInternal_UnimplementedProvider_Returns501 guards the
// loud-failure path: a provider type this codebase can't actually resolve
// yet (anything but "static") must surface as an explicit 501, not
// silently fall back to the (wrong, unresolved) credential_path string.
func TestGetOrgSettingsInternal_UnimplementedProvider_Returns501(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	credProviders := new(MockCredentialProviderSvc)
	pgh := NewPipelineGateHandler(repo, nil, credProviders)
	r := setupPipelineGateRouter(pgh)

	providerID := uuid.New()
	repo.On("GetOrgHITLGateEnabled", mock.Anything, gateTestOrgID).Return(true, nil)
	repo.On("GetOrgLLMSettings", mock.Anything, gateTestOrgID).Return(database.LLMSettings{
		APIKeyRef: "secret/llm-key", CredentialProviderID: &providerID,
	}, nil)
	credProviders.On("GetByID", mock.Anything, providerID).Return(&models.CredentialProvider{
		ID: providerID, ProviderType: "hashicorp_vault", Config: json.RawMessage(`{}`),
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/internal/organizations/"+gateTestOrgID.String()+"/settings", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestUpdateOrgSettings_LLMFields(t *testing.T) {
	// This is the write path the LLM & Tokens settings UI uses -- the
	// handler must read-modify-write (GetOrgLLMSettings then
	// SetOrgLLMSettings with only the provided fields overlaid), not
	// blindly overwrite fields the request didn't include.
	repo := new(MockPipelineGateRepo)
	pgh := NewPipelineGateHandler(repo, nil, nil)
	r := setupPipelineGateRouter(pgh)

	repo.On("GetOrgLLMSettings", mock.Anything, gateTestOrgID).Return(database.LLMSettings{
		ProviderType: "lm_studio", BaseURL: "http://old:1234/v1", Model: "old-model", APIKeyRef: "old-ref",
	}, nil)
	repo.On("SetOrgLLMSettings", mock.Anything, gateTestOrgID, database.LLMSettings{
		ProviderType: "openrouter", BaseURL: "http://old:1234/v1", Model: "anthropic/claude-sonnet-4-6", APIKeyRef: "old-ref",
	}).Return(nil)

	body, _ := json.Marshal(UpdateOrgSettingsRequest{
		DefaultLLMProviderType: strPtr("openrouter"),
		DefaultLLMModel:        strPtr("anthropic/claude-sonnet-4-6"),
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/organizations/"+gateTestOrgID.String()+"/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "openrouter", resp["default_llm_provider_type"])
	assert.Equal(t, "anthropic/claude-sonnet-4-6", resp["default_llm_model"])
	repo.AssertExpectations(t)
	repo.AssertNotCalled(t, "SetOrgHITLGateEnabled", mock.Anything, mock.Anything, mock.Anything)
}

// TestUpdateOrgSettings_TeamsFields covers the write path the
// Integrations settings UI uses for War Room's Teams config. Same
// read-modify-write semantics as the LLM fields, plus: the response must
// never echo the client secret back, only whether one is set.
func TestUpdateOrgSettings_TeamsFields(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	pgh := NewPipelineGateHandler(repo, nil, nil)
	r := setupPipelineGateRouter(pgh)

	repo.On("GetOrgTeamsSettings", mock.Anything, gateTestOrgID).Return(database.TeamsSettings{
		TenantID: "old-tenant", ClientID: "old-client",
	}, nil)
	repo.On("SetOrgTeamsSettings", mock.Anything, gateTestOrgID, database.TeamsSettings{
		TenantID: "new-tenant", ClientID: "old-client", ClientSecret: "shh",
	}).Return(nil)

	body, _ := json.Marshal(UpdateOrgSettingsRequest{
		TeamsTenantID:     strPtr("new-tenant"),
		TeamsClientSecret: strPtr("shh"),
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/organizations/"+gateTestOrgID.String()+"/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "new-tenant", resp["teams_tenant_id"])
	assert.Equal(t, "old-client", resp["teams_client_id"])
	assert.Equal(t, true, resp["teams_client_secret_set"])
	// No refresh token yet (that only exists once the OAuth connect flow
	// completes) -- Configured() must stay false even with tenant/client/
	// secret all set.
	assert.Equal(t, false, resp["teams_configured"])
	assert.Equal(t, false, resp["teams_refresh_token_set"])
	assert.NotContains(t, resp, "teams_client_secret", "the raw secret must never be echoed back")
	assert.NotContains(t, w.Body.String(), "shh", "the raw secret must not appear anywhere in the response body")
	repo.AssertExpectations(t)
	repo.AssertNotCalled(t, "SetOrgHITLGateEnabled", mock.Anything, mock.Anything, mock.Anything)
	repo.AssertNotCalled(t, "SetOrgLLMSettings", mock.Anything, mock.Anything, mock.Anything)
}

// TestGetOrgSettings_IncludesRedactedTeamsSettings covers the read path
// the Settings page loads on mount.
func TestGetOrgSettings_IncludesRedactedTeamsSettings(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	pgh := NewPipelineGateHandler(repo, nil, nil)
	r := setupPipelineGateRouter(pgh)

	repo.On("GetOrgHITLGateEnabled", mock.Anything, gateTestOrgID).Return(false, nil)
	repo.On("GetOrgLLMSettings", mock.Anything, gateTestOrgID).Return(database.LLMSettings{}, nil)
	repo.On("GetOrgTeamsSettings", mock.Anything, gateTestOrgID).Return(database.TeamsSettings{
		TenantID: "t", ClientID: "c", ClientSecret: "top-secret",
		RefreshToken: "top-secret-refresh-token", ConnectedAccountEmail: "rootcauseway-bot@example.com",
	}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/organizations/"+gateTestOrgID.String()+"/settings", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "t", resp["teams_tenant_id"])
	assert.Equal(t, "c", resp["teams_client_id"])
	assert.Equal(t, "rootcauseway-bot@example.com", resp["teams_connected_account"])
	assert.Equal(t, true, resp["teams_client_secret_set"])
	assert.Equal(t, true, resp["teams_refresh_token_set"])
	assert.Equal(t, true, resp["teams_configured"])
	assert.NotContains(t, w.Body.String(), "top-secret")
	assert.NotContains(t, w.Body.String(), "top-secret-refresh-token")
}

func TestMarkAwaitingApprovalInternal_Success(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	pgh := NewPipelineGateHandler(repo, nil, nil)
	r := setupPipelineGateRouter(pgh)

	incidentID := uuid.New()
	repo.On("MarkAwaitingApproval", mock.Anything, incidentID, "postmortem").Return(nil)

	body, _ := json.Marshal(MarkAwaitingApprovalRequest{Stage: "postmortem"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/internal/incidents/"+incidentID.String()+"/awaiting-approval", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestMarkAwaitingApprovalInternal_MissingStage(t *testing.T) {
	repo := new(MockPipelineGateRepo)
	pgh := NewPipelineGateHandler(repo, nil, nil)
	r := setupPipelineGateRouter(pgh)

	incidentID := uuid.New()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/internal/incidents/"+incidentID.String()+"/awaiting-approval", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
