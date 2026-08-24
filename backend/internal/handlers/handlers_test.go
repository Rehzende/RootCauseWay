package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type MockSoftwareSvc struct{ mock.Mock }

func (m *MockSoftwareSvc) Create(ctx context.Context, orgID uuid.UUID, req models.CreateSoftwareRequest) (*models.SoftwareEntry, error) {
	args := m.Called(ctx, orgID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SoftwareEntry), args.Error(1)
}
func (m *MockSoftwareSvc) GetByID(ctx context.Context, id uuid.UUID) (*models.SoftwareEntry, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SoftwareEntry), args.Error(1)
}
func (m *MockSoftwareSvc) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.SoftwareEntry, int, error) {
	args := m.Called(ctx, orgID, page, perPage)
	return args.Get(0).([]models.SoftwareEntry), args.Int(1), args.Error(2)
}
func (m *MockSoftwareSvc) Update(ctx context.Context, id uuid.UUID, req models.CreateSoftwareRequest) (*models.SoftwareEntry, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SoftwareEntry), args.Error(1)
}
func (m *MockSoftwareSvc) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type MockResourceCredentialsSvc struct{ mock.Mock }

func (m *MockResourceCredentialsSvc) Create(ctx context.Context, orgID uuid.UUID, req models.CreateResourceCredentialRequest) (*models.ResourceCredential, error) {
	args := m.Called(ctx, orgID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ResourceCredential), args.Error(1)
}
func (m *MockResourceCredentialsSvc) GetByID(ctx context.Context, id uuid.UUID) (*models.ResourceCredential, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ResourceCredential), args.Error(1)
}
func (m *MockResourceCredentialsSvc) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.ResourceCredential, error) {
	args := m.Called(ctx, softwareID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ResourceCredential), args.Error(1)
}
func (m *MockResourceCredentialsSvc) Update(ctx context.Context, id uuid.UUID, req models.CreateResourceCredentialRequest) (*models.ResourceCredential, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ResourceCredential), args.Error(1)
}
func (m *MockResourceCredentialsSvc) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type MockCredentialLeaseSvc struct{ mock.Mock }

func (m *MockCredentialLeaseSvc) RequestLease(ctx context.Context, orgID uuid.UUID, req models.RequestLeaseRequest) (*models.CredentialLease, error) {
	args := m.Called(ctx, orgID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CredentialLease), args.Error(1)
}
func (m *MockCredentialLeaseSvc) RevokeLease(ctx context.Context, id uuid.UUID, revokedBy string) (*models.CredentialLease, error) {
	args := m.Called(ctx, id, revokedBy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CredentialLease), args.Error(1)
}
func (m *MockCredentialLeaseSvc) GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialLease, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CredentialLease), args.Error(1)
}
func (m *MockCredentialLeaseSvc) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.CredentialLease, error) {
	args := m.Called(ctx, incidentID)
	return args.Get(0).([]models.CredentialLease), args.Error(1)
}
func (m *MockCredentialLeaseSvc) ListActive(ctx context.Context, orgID uuid.UUID) ([]models.CredentialLease, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).([]models.CredentialLease), args.Error(1)
}

type MockAgentSvc struct{ mock.Mock }

func (m *MockAgentSvc) Create(ctx context.Context, orgID uuid.UUID, req models.CreateAgentRequest) (*models.Agent, error) {
	args := m.Called(ctx, orgID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}
func (m *MockAgentSvc) GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}
func (m *MockAgentSvc) List(ctx context.Context, orgID uuid.UUID, agentType string, page, perPage int) ([]models.Agent, int, error) {
	args := m.Called(ctx, orgID, agentType, page, perPage)
	return args.Get(0).([]models.Agent), args.Int(1), args.Error(2)
}
func (m *MockAgentSvc) Update(ctx context.Context, id uuid.UUID, req models.CreateAgentRequest) (*models.Agent, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}
func (m *MockAgentSvc) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type MockWebhookSvc struct{ mock.Mock }

func (m *MockWebhookSvc) Create(ctx context.Context, orgID uuid.UUID, req models.CreateWebhookRequest) (*models.Webhook, error) {
	args := m.Called(ctx, orgID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Webhook), args.Error(1)
}
func (m *MockWebhookSvc) GetByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Webhook), args.Error(1)
}
func (m *MockWebhookSvc) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.Webhook, int, error) {
	args := m.Called(ctx, orgID, page, perPage)
	return args.Get(0).([]models.Webhook), args.Int(1), args.Error(2)
}
func (m *MockWebhookSvc) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type MockIncidentSvc struct{ mock.Mock }

func (m *MockIncidentSvc) Create(ctx context.Context, incident *models.Incident) error {
	return m.Called(ctx, incident).Error(0)
}
func (m *MockIncidentSvc) GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Incident), args.Error(1)
}
func (m *MockIncidentSvc) List(ctx context.Context, orgID uuid.UUID, status, severity string, softwareID *uuid.UUID, from *time.Time, page, perPage int) ([]models.Incident, int, error) {
	args := m.Called(ctx, orgID, status, severity, softwareID, from, page, perPage)
	return args.Get(0).([]models.Incident), args.Int(1), args.Error(2)
}
func (m *MockIncidentSvc) Update(ctx context.Context, id uuid.UUID, req models.UpdateIncidentRequest) (*models.Incident, bool, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).(*models.Incident), args.Bool(1), args.Error(2)
}
func (m *MockIncidentSvc) AddEvent(ctx context.Context, incidentID uuid.UUID, actor string, req models.CreateEventRequest) (*models.IncidentEvent, error) {
	args := m.Called(ctx, incidentID, actor, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.IncidentEvent), args.Error(1)
}
func (m *MockIncidentSvc) AddEvidence(ctx context.Context, incidentID uuid.UUID, req models.CreateEvidenceRequest) (*models.IncidentEvidence, error) {
	args := m.Called(ctx, incidentID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.IncidentEvidence), args.Error(1)
}
func (m *MockIncidentSvc) ListEvidence(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentEvidence, error) {
	args := m.Called(ctx, incidentID)
	return args.Get(0).([]models.IncidentEvidence), args.Error(1)
}

type MockIngestionSvc struct{ mock.Mock }

func (m *MockIngestionSvc) IngestAlert(ctx context.Context, token string, rawPayload json.RawMessage) (*services.IngestionResult, error) {
	args := m.Called(ctx, token, rawPayload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.IngestionResult), args.Error(1)
}

// --- Test helpers ---

func setupRouter(h *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Set("org_id", uuid.MustParse("00000000-0000-0000-0000-000000000001"))
		c.Set("user_id", uuid.MustParse("00000000-0000-0000-0000-000000000002"))
		c.Next()
	})

	api := r.Group("/api/v1")
	api.GET("/software", h.ListSoftware)
	api.POST("/software", h.CreateSoftware)
	api.GET("/software/:id", h.GetSoftware)
	api.PUT("/software/:id", h.UpdateSoftware)
	api.DELETE("/software/:id", h.DeleteSoftware)
	api.GET("/software/:id/credentials", h.ListResourceCredentials)
	api.POST("/credentials/lease", h.RequestLease)
	api.POST("/ingest/:token", h.IngestAlert)
	api.GET("/incidents/:id", h.GetIncident)
	api.PATCH("/incidents/:id", h.UpdateIncident)
	api.POST("/incidents/:id/events", h.AddIncidentEvent)
	api.POST("/incidents/:id/evidence", h.AddIncidentEvidence)

	return r
}

// --- Tests ---

func TestListSoftware(t *testing.T) {
	swSvc := new(MockSoftwareSvc)
	h := &Handler{Software: swSvc}
	r := setupRouter(h)

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	entries := []models.SoftwareEntry{{Name: "API"}}
	swSvc.On("List", mock.Anything, orgID, 1, 20).Return(entries, 1, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/software", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.PaginatedResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Total)
}

func TestCreateSoftware(t *testing.T) {
	swSvc := new(MockSoftwareSvc)
	h := &Handler{Software: swSvc}
	r := setupRouter(h)

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	expected := &models.SoftwareEntry{ID: uuid.New(), Name: "API", Slug: "api"}
	swSvc.On("Create", mock.Anything, orgID, mock.AnythingOfType("models.CreateSoftwareRequest")).Return(expected, nil)

	body, _ := json.Marshal(models.CreateSoftwareRequest{Name: "API", Slug: "api"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/software", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestGetSoftware_InvalidID(t *testing.T) {
	h := &Handler{Software: new(MockSoftwareSvc)}
	r := setupRouter(h)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/software/not-a-uuid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestIngestAlert(t *testing.T) {
	ingSvc := new(MockIngestionSvc)
	h := &Handler{Ingestion: ingSvc}
	r := setupRouter(h)

	result := &services.IngestionResult{IncidentID: uuid.New(), AlertSnapshotID: uuid.New()}
	ingSvc.On("IngestAlert", mock.Anything, "tok123", mock.Anything).Return(result, nil)

	body := []byte(`{"alert_id":"1","alert_title":"test"}`)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/ingest/tok123", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

// --- Multi-tenant isolation ---
//
// A platform audit found GetIncident, UpdateIncident, AddIncidentEvent,
// AddIncidentEvidence, GetIncidentFull, GetSoftware, UpdateSoftware and
// DeleteSoftware all fetched/mutated a resource by ID alone, with no check
// that it belonged to the caller's org -- any authenticated user from any
// org could read or write any other org's incidents/software by ID. These
// tests guard the fix (verifyIncidentOwnership / verifySoftwareOwnership).

func TestGetIncident_DifferentOrg_Returns404(t *testing.T) {
	incSvc := new(MockIncidentSvc)
	h := &Handler{Incidents: incSvc}
	r := setupRouter(h)

	otherOrgID := uuid.New() // NOT the caller's org (00000000-...-000000000001)
	id := uuid.New()
	incSvc.On("GetByID", mock.Anything, id).Return(&models.Incident{ID: id, OrgID: otherOrgID}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/incidents/"+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetIncident_SameOrg_Returns200(t *testing.T) {
	incSvc := new(MockIncidentSvc)
	h := &Handler{Incidents: incSvc}
	r := setupRouter(h)

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id := uuid.New()
	incSvc.On("GetByID", mock.Anything, id).Return(&models.Incident{ID: id, OrgID: orgID}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/incidents/"+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateIncident_DifferentOrg_Returns404AndDoesNotCallUpdate(t *testing.T) {
	incSvc := new(MockIncidentSvc)
	h := &Handler{Incidents: incSvc}
	r := setupRouter(h)

	otherOrgID := uuid.New()
	id := uuid.New()
	incSvc.On("GetByID", mock.Anything, id).Return(&models.Incident{ID: id, OrgID: otherOrgID}, nil)

	body, _ := json.Marshal(map[string]string{"status": "resolved"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/incidents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	incSvc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
}

// TestUpdateIncident_Publishes* pin the WS-bridge event wiring added to
// close a live-found gap: nothing was ever publishing "incident.created"/
// "incident.updated", so the frontend's live incident list/dashboard toast
// (IncidentsPage/DashboardPage, subscribed to exactly these two topics) never
// fired in production even after the WebSocket connection itself got fixed.
// "incident.resolved" already existed (drives postmortem generation) but had
// zero test coverage before this.

func TestUpdateIncident_PublishesIncidentUpdatedOnEveryUpdate(t *testing.T) {
	incSvc := new(MockIncidentSvc)
	pub := new(MockGatePublisher)
	h := &Handler{Incidents: incSvc, EventPublisher: pub}
	r := setupRouter(h)

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id := uuid.New()
	incSvc.On("GetByID", mock.Anything, id).Return(&models.Incident{ID: id, OrgID: orgID}, nil)
	updated := &models.Incident{ID: id, OrgID: orgID, Title: "High CPU", Severity: "high", Status: "investigating"}
	incSvc.On("Update", mock.Anything, id, mock.Anything).Return(updated, false, nil)
	pub.On("Publish", mock.Anything, "rootcauseway:"+orgID.String()+":incident.updated", mock.MatchedBy(func(e models.EventEnvelope) bool {
		payload, ok := e.Payload.(models.IncidentUpdatedPayload)
		return ok && e.EventType == "incident.updated" && payload.IncidentID == id && payload.Status == "investigating"
	})).Return(nil)

	body, _ := json.Marshal(map[string]string{"status": "investigating"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/incidents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	pub.AssertCalled(t, "Publish", mock.Anything, "rootcauseway:"+orgID.String()+":incident.updated", mock.Anything)
	// Non-terminal transition -- incident.resolved must NOT also fire.
	pub.AssertNotCalled(t, "Publish", mock.Anything, "rootcauseway:"+orgID.String()+":incident.resolved", mock.Anything)
}

func TestUpdateIncident_PublishesBothResolvedAndUpdatedOnTerminalTransition(t *testing.T) {
	incSvc := new(MockIncidentSvc)
	pub := new(MockGatePublisher)
	h := &Handler{Incidents: incSvc, EventPublisher: pub}
	r := setupRouter(h)

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id := uuid.New()
	incSvc.On("GetByID", mock.Anything, id).Return(&models.Incident{ID: id, OrgID: orgID}, nil)
	resolved := &models.Incident{ID: id, OrgID: orgID, Title: "High CPU", Severity: "high", Status: "resolved"}
	incSvc.On("Update", mock.Anything, id, mock.Anything).Return(resolved, true, nil)
	pub.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	body, _ := json.Marshal(map[string]string{"status": "resolved"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PATCH", "/api/v1/incidents/"+id.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	pub.AssertCalled(t, "Publish", mock.Anything, "rootcauseway:"+orgID.String()+":incident.resolved", mock.Anything)
	pub.AssertCalled(t, "Publish", mock.Anything, "rootcauseway:"+orgID.String()+":incident.updated", mock.Anything)
}

func TestAddIncidentEvidence_DifferentOrg_Returns404(t *testing.T) {
	incSvc := new(MockIncidentSvc)
	h := &Handler{Incidents: incSvc}
	r := setupRouter(h)

	otherOrgID := uuid.New()
	id := uuid.New()
	incSvc.On("GetByID", mock.Anything, id).Return(&models.Incident{ID: id, OrgID: otherOrgID}, nil)

	body, _ := json.Marshal(models.CreateEvidenceRequest{Type: "manual", Title: "x", Content: json.RawMessage(`{}`)})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/incidents/"+id.String()+"/evidence", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	incSvc.AssertNotCalled(t, "AddEvidence", mock.Anything, mock.Anything, mock.Anything)
}

func TestGetSoftware_DifferentOrg_Returns404(t *testing.T) {
	swSvc := new(MockSoftwareSvc)
	h := &Handler{Software: swSvc}
	r := setupRouter(h)

	otherOrgID := uuid.New()
	id := uuid.New()
	swSvc.On("GetByID", mock.Anything, id).Return(&models.SoftwareEntry{ID: id, OrgID: otherOrgID}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/software/"+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListResourceCredentials_DifferentOrg_Returns404AndDoesNotList(t *testing.T) {
	// Found live: ListResourceCredentials had no org ownership check at
	// all -- any internal caller passing another org's software UUID got
	// that software's resource credentials back. Same gap
	// verifyIncidentOwnership/verifySoftwareOwnership already closed on
	// incidents/software; this call site was missed.
	swSvc := new(MockSoftwareSvc)
	rcSvc := new(MockResourceCredentialsSvc)
	h := &Handler{Software: swSvc, ResourceCredentials: rcSvc}
	r := setupRouter(h)

	otherOrgID := uuid.New()
	id := uuid.New()
	swSvc.On("GetByID", mock.Anything, id).Return(&models.SoftwareEntry{ID: id, OrgID: otherOrgID}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/software/"+id.String()+"/credentials", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	rcSvc.AssertNotCalled(t, "ListBySoftware", mock.Anything, mock.Anything)
}

func TestListResourceCredentials_SameOrg_ReturnsCredentials(t *testing.T) {
	swSvc := new(MockSoftwareSvc)
	rcSvc := new(MockResourceCredentialsSvc)
	h := &Handler{Software: swSvc, ResourceCredentials: rcSvc}
	r := setupRouter(h)

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id := uuid.New()
	swSvc.On("GetByID", mock.Anything, id).Return(&models.SoftwareEntry{ID: id, OrgID: orgID}, nil)
	rcSvc.On("ListBySoftware", mock.Anything, id).Return([]models.ResourceCredential{{ID: uuid.New()}}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/software/"+id.String()+"/credentials", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRequestLease_UnimplementedProvider_Returns501 guards the loud-failure
// mapping for the credential vault gap found live: a provider type this
// codebase can't actually generate a real secret for yet (only "static" is
// wired) must surface as an explicit, actionable 501 -- not a masked
// generic 500 indistinguishable from an unrelated DB error.
func TestRequestLease_UnimplementedProvider_Returns501(t *testing.T) {
	cl := new(MockCredentialLeaseSvc)
	h := &Handler{CredentialLeases: cl}
	r := setupRouter(h)

	cl.On("RequestLease", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("%w: %q", services.ErrCredentialProviderNotImplemented, "hashicorp_vault"))

	body, _ := json.Marshal(map[string]any{
		"incident_id": uuid.New().String(), "agent_id": uuid.New().String(),
		"resource_credential_id": uuid.New().String(),
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/credentials/lease", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestDeleteSoftware_DifferentOrg_Returns404AndDoesNotDelete(t *testing.T) {
	swSvc := new(MockSoftwareSvc)
	h := &Handler{Software: swSvc}
	r := setupRouter(h)

	otherOrgID := uuid.New()
	id := uuid.New()
	swSvc.On("GetByID", mock.Anything, id).Return(&models.SoftwareEntry{ID: id, OrgID: otherOrgID}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/software/"+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	swSvc.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
}

func TestDeleteSoftware(t *testing.T) {
	swSvc := new(MockSoftwareSvc)
	h := &Handler{Software: swSvc}
	r := setupRouter(h)

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	id := uuid.New()
	swSvc.On("GetByID", mock.Anything, id).Return(&models.SoftwareEntry{ID: id, OrgID: orgID}, nil)
	swSvc.On("Delete", mock.Anything, id).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/software/"+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}
