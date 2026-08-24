package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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

type MockDependencyGraphProvider struct{ mock.Mock }

func (m *MockDependencyGraphProvider) GetDependencyGraph(ctx context.Context, id uuid.UUID) (*services.DependencyGraph, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.DependencyGraph), args.Error(1)
}

type MockCorrelationIncidentRepo struct{ mock.Mock }

func (m *MockCorrelationIncidentRepo) ListOpenBySoftwareIDs(ctx context.Context, orgID uuid.UUID, softwareIDs []uuid.UUID, since time.Time) ([]models.Incident, error) {
	args := m.Called(ctx, orgID, softwareIDs, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Incident), args.Error(1)
}

func (m *MockCorrelationIncidentRepo) FindByFingerprint(ctx context.Context, orgID uuid.UUID, fingerprint string, since time.Time) (*models.Incident, error) {
	args := m.Called(ctx, orgID, fingerprint, since)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Incident), args.Error(1)
}

func setupCorrelationExtraRouter(h *CorrelationExtraHandler, orgID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if orgID != uuid.Nil {
			c.Set("org_id", orgID)
		}
		c.Next()
	})
	api := r.Group("/internal")
	api.GET("/software/:id/dependency-graph", h.GetSoftwareDependencyGraph)
	api.GET("/incidents/open-by-software", h.ListOpenIncidentsBySoftware)
	api.GET("/incidents/by-fingerprint", h.FindIncidentByFingerprint)
	api.POST("/correlation/check", h.CorrelationCheck)
	return r
}

// --- Tests ---

func TestGetSoftwareDependencyGraph_OK(t *testing.T) {
	sw := new(MockDependencyGraphProvider)
	h := &CorrelationExtraHandler{Software: sw}
	orgID := uuid.New()
	r := setupCorrelationExtraRouter(h, orgID)

	id := uuid.New()
	graph := &services.DependencyGraph{
		SoftwareID: id, Slug: "api-service",
		Upstream:   []services.RelatedService{{Slug: "postgres-primary"}},
		Downstream: []services.RelatedService{{Slug: "checkout-service"}},
	}
	sw.On("GetDependencyGraph", mock.Anything, id).Return(graph, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/software/"+id.String()+"/dependency-graph", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]any)
	assert.Equal(t, "api-service", data["slug"])
}

func TestGetSoftwareDependencyGraph_InvalidID(t *testing.T) {
	h := &CorrelationExtraHandler{Software: new(MockDependencyGraphProvider)}
	r := setupCorrelationExtraRouter(h, uuid.New())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/software/not-a-uuid/dependency-graph", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetSoftwareDependencyGraph_NotFound(t *testing.T) {
	sw := new(MockDependencyGraphProvider)
	h := &CorrelationExtraHandler{Software: sw}
	r := setupCorrelationExtraRouter(h, uuid.New())

	id := uuid.New()
	sw.On("GetDependencyGraph", mock.Anything, id).Return(nil, assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/software/"+id.String()+"/dependency-graph", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListOpenIncidentsBySoftware_OK(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	orgID := uuid.New()
	r := setupCorrelationExtraRouter(h, orgID)

	sid1, sid2 := uuid.New(), uuid.New()
	incidentID := uuid.New()
	repo.On("ListOpenBySoftwareIDs", mock.Anything, orgID, mock.MatchedBy(func(ids []uuid.UUID) bool {
		return len(ids) == 2
	}), mock.AnythingOfType("time.Time")).
		Return([]models.Incident{{ID: incidentID, SoftwareID: sid1}}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/incidents/open-by-software?software_id="+sid1.String()+"&software_id="+sid2.String()+"&window_seconds=300", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	repo.AssertExpectations(t)
}

func TestListOpenIncidentsBySoftware_MissingOrgID(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	r := setupCorrelationExtraRouter(h, uuid.Nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/incidents/open-by-software?software_id="+uuid.New().String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListOpenIncidentsBySoftware_MissingSoftwareID(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	r := setupCorrelationExtraRouter(h, uuid.New())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/incidents/open-by-software", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFindIncidentByFingerprint_Found(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	orgID := uuid.New()
	r := setupCorrelationExtraRouter(h, orgID)

	incidentID := uuid.New()
	repo.On("FindByFingerprint", mock.Anything, orgID, "fp-123", mock.AnythingOfType("time.Time")).
		Return(&models.Incident{ID: incidentID}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/incidents/by-fingerprint?fingerprint=fp-123", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]any)
	assert.Equal(t, incidentID.String(), data["id"])
}

func TestFindIncidentByFingerprint_NoneFound(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	orgID := uuid.New()
	r := setupCorrelationExtraRouter(h, orgID)

	repo.On("FindByFingerprint", mock.Anything, orgID, "fp-404", mock.AnythingOfType("time.Time")).
		Return(nil, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/incidents/by-fingerprint?fingerprint=fp-404", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Nil(t, body["data"])
}

func TestFindIncidentByFingerprint_MissingFingerprint(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	r := setupCorrelationExtraRouter(h, uuid.New())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/incidents/by-fingerprint", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestCorrelationCheck_* pins the fix for a live-found bug: this endpoint
// used to be a stub (FeaturesHandler.CorrelationCheck) requiring a field
// (alert_snapshot_id) that agent-service's real caller
// (BackendClient.check_correlation) never sends -- every real call 400'd,
// so this leg of correlation always silently fell back to "treat as new
// incident". Confirmed live during a full-pipeline test against a real
// Pulso alert.

func TestCorrelationCheck_OpenIncidentExists_ReturnsCorrelated(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	orgID := uuid.New()
	softwareID := uuid.New()
	existingID := uuid.New()
	r := setupCorrelationExtraRouter(h, orgID)

	repo.On("ListOpenBySoftwareIDs", mock.Anything, orgID, []uuid.UUID{softwareID}, mock.AnythingOfType("time.Time")).
		Return([]models.Incident{{ID: existingID, SoftwareID: softwareID}}, nil)

	body, _ := json.Marshal(map[string]any{
		"software_id":         softwareID.String(),
		"alert":               map[string]any{"title": "5xx spike"},
		"time_window_seconds": 300,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/correlation/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.CorrelationCheckResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Correlated)
	require.NotNil(t, resp.IncidentID)
	assert.Equal(t, existingID, *resp.IncidentID)
}

func TestCorrelationCheck_NoOpenIncident_ReturnsNotCorrelated(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	orgID := uuid.New()
	softwareID := uuid.New()
	r := setupCorrelationExtraRouter(h, orgID)

	repo.On("ListOpenBySoftwareIDs", mock.Anything, orgID, []uuid.UUID{softwareID}, mock.AnythingOfType("time.Time")).
		Return([]models.Incident{}, nil)

	body, _ := json.Marshal(map[string]any{
		"software_id":         softwareID.String(),
		"alert":               map[string]any{"title": "5xx spike"},
		"time_window_seconds": 300,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/correlation/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.CorrelationCheckResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Correlated)
	assert.Nil(t, resp.IncidentID)
}

// TestCorrelationCheck_ExcludesSelf_* pins a second live-found bug on top of
// the one above: IngestAlert (Go) creates and commits the incident row
// *before* agent-service's alert.received handler ever calls this endpoint,
// so for the very first alert of a brand-new incident, "an open incident on
// this software_id within the window" trivially matches the incident this
// alert itself just created. Confirmed live: a real fired Pulso alert
// self-correlated and the pipeline never ran for it, immediately after the
// fix above went in -- this endpoint went from "always 400s" straight to
// "always self-matches", never actually exercising the real case in between.

func TestCorrelationCheck_ExcludesSelf_NoOtherOpenIncident_ReturnsNotCorrelated(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	orgID := uuid.New()
	softwareID := uuid.New()
	selfID := uuid.New()
	r := setupCorrelationExtraRouter(h, orgID)

	// The only "open incident" found is the one this exact alert created.
	repo.On("ListOpenBySoftwareIDs", mock.Anything, orgID, []uuid.UUID{softwareID}, mock.AnythingOfType("time.Time")).
		Return([]models.Incident{{ID: selfID, SoftwareID: softwareID}}, nil)

	body, _ := json.Marshal(map[string]any{
		"software_id":         softwareID.String(),
		"alert":               map[string]any{"title": "5xx spike"},
		"time_window_seconds": 300,
		"exclude_incident_id": selfID.String(),
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/correlation/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.CorrelationCheckResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Correlated)
	assert.Nil(t, resp.IncidentID)
}

func TestCorrelationCheck_ExcludesSelf_OtherOpenIncidentStillMatches(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	orgID := uuid.New()
	softwareID := uuid.New()
	selfID := uuid.New()
	otherID := uuid.New()
	r := setupCorrelationExtraRouter(h, orgID)

	// A genuinely different open incident exists alongside the self one --
	// exclusion must not blind the check to real correlation targets.
	repo.On("ListOpenBySoftwareIDs", mock.Anything, orgID, []uuid.UUID{softwareID}, mock.AnythingOfType("time.Time")).
		Return([]models.Incident{{ID: selfID, SoftwareID: softwareID}, {ID: otherID, SoftwareID: softwareID}}, nil)

	body, _ := json.Marshal(map[string]any{
		"software_id":         softwareID.String(),
		"alert":               map[string]any{"title": "5xx spike"},
		"time_window_seconds": 300,
		"exclude_incident_id": selfID.String(),
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/correlation/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.CorrelationCheckResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Correlated)
	require.NotNil(t, resp.IncidentID)
	assert.Equal(t, otherID, *resp.IncidentID)
}

func TestFindIncidentByFingerprint_ExcludesSelf_ReturnsNoneWhenOnlySelfMatches(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	orgID := uuid.New()
	selfID := uuid.New()
	r := setupCorrelationExtraRouter(h, orgID)

	// The incident this exact alert created shares its own fingerprint.
	repo.On("FindByFingerprint", mock.Anything, orgID, "fp-self", mock.AnythingOfType("time.Time")).
		Return(&models.Incident{ID: selfID}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/incidents/by-fingerprint?fingerprint=fp-self&exclude_incident_id="+selfID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Nil(t, body["data"])
}

func TestCorrelationCheck_MissingSoftwareID_Returns400(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	r := setupCorrelationExtraRouter(h, uuid.New())

	body, _ := json.Marshal(map[string]any{"alert": map[string]any{}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/correlation/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	repo.AssertNotCalled(t, "ListOpenBySoftwareIDs", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCorrelationCheck_MissingOrgHeader_Returns400(t *testing.T) {
	repo := new(MockCorrelationIncidentRepo)
	h := &CorrelationExtraHandler{IncidentRepo: repo}
	r := setupCorrelationExtraRouter(h, uuid.Nil)

	body, _ := json.Marshal(map[string]any{"software_id": uuid.New().String()})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/correlation/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
