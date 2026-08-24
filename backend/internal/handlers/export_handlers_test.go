package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPostmortemSvc mocks PostmortemServiceInterface for handler tests.
// Not defined in handlers_test.go to avoid touching that shared file, which
// other in-flight agents are also editing.
type MockPostmortemSvc struct{ mock.Mock }

func (m *MockPostmortemSvc) Create(ctx context.Context, incidentID uuid.UUID, req models.CreatePostmortemRequest) (*models.IncidentPostmortem, error) {
	args := m.Called(ctx, incidentID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.IncidentPostmortem), args.Error(1)
}
func (m *MockPostmortemSvc) GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentPostmortem, error) {
	args := m.Called(ctx, incidentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.IncidentPostmortem), args.Error(1)
}
func (m *MockPostmortemSvc) Update(ctx context.Context, incidentID uuid.UUID, req models.CreatePostmortemRequest) (*models.IncidentPostmortem, error) {
	args := m.Called(ctx, incidentID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.IncidentPostmortem), args.Error(1)
}

// mockIncidentSvcForExport mocks IncidentServiceInterface for export handler
// tests. It is deliberately NOT named MockIncidentSvc and NOT defined in
// handlers_test.go: the existing MockIncidentSvc there has a stale List()
// signature (missing the `from *time.Time` param the interface now
// declares) which is a pre-existing mismatch on main, unrelated to this
// change. Reusing it would fail to compile; handlers_test.go is a shared
// file other in-flight agents are editing, so it's left untouched here.
type mockIncidentSvcForExport struct{ mock.Mock }

func (m *mockIncidentSvcForExport) Create(ctx context.Context, incident *models.Incident) error {
	return m.Called(ctx, incident).Error(0)
}
func (m *mockIncidentSvcForExport) GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Incident), args.Error(1)
}
func (m *mockIncidentSvcForExport) List(ctx context.Context, orgID uuid.UUID, status, severity string, softwareID *uuid.UUID, from *time.Time, page, perPage int) ([]models.Incident, int, error) {
	args := m.Called(ctx, orgID, status, severity, softwareID, from, page, perPage)
	return args.Get(0).([]models.Incident), args.Int(1), args.Error(2)
}
func (m *mockIncidentSvcForExport) Update(ctx context.Context, id uuid.UUID, req models.UpdateIncidentRequest) (*models.Incident, bool, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).(*models.Incident), args.Bool(1), args.Error(2)
}
func (m *mockIncidentSvcForExport) AddEvent(ctx context.Context, incidentID uuid.UUID, actor string, req models.CreateEventRequest) (*models.IncidentEvent, error) {
	args := m.Called(ctx, incidentID, actor, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.IncidentEvent), args.Error(1)
}
func (m *mockIncidentSvcForExport) AddEvidence(ctx context.Context, incidentID uuid.UUID, req models.CreateEvidenceRequest) (*models.IncidentEvidence, error) {
	args := m.Called(ctx, incidentID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.IncidentEvidence), args.Error(1)
}
func (m *mockIncidentSvcForExport) ListEvidence(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentEvidence, error) {
	args := m.Called(ctx, incidentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.IncidentEvidence), args.Error(1)
}

func setupExportRouter(eh *ExportHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Set("org_id", uuid.MustParse("00000000-0000-0000-0000-000000000001"))
		c.Set("user_id", uuid.MustParse("00000000-0000-0000-0000-000000000002"))
		c.Next()
	})

	api := r.Group("/api/v1")
	api.GET("/incidents/:id/postmortem/export", eh.ExportPostmortem)

	return r
}

func samplePM(incidentID uuid.UUID) *models.IncidentPostmortem {
	return &models.IncidentPostmortem{
		ID:               uuid.New(),
		IncidentID:       incidentID,
		Title:            "Test Postmortem",
		ExecutiveSummary: "Summary text.",
	}
}

func TestExportPostmortem_Markdown(t *testing.T) {
	pmSvc := new(MockPostmortemSvc)
	incSvc := new(mockIncidentSvcForExport)
	eh := NewExportHandler(pmSvc, incSvc)
	r := setupExportRouter(eh)

	incidentID := uuid.New()
	pm := samplePM(incidentID)
	incident := &models.Incident{ID: incidentID, Title: "Outage"}

	pmSvc.On("GetByIncidentID", mock.Anything, incidentID).Return(pm, nil)
	incSvc.On("GetByID", mock.Anything, incidentID).Return(incident, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/incidents/"+incidentID.String()+"/postmortem/export?format=markdown", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/markdown")
	assert.Equal(t, `attachment; filename="postmortem-`+incidentID.String()+`.md"`, w.Header().Get("Content-Disposition"))
	assert.Contains(t, w.Body.String(), "# Test Postmortem")
}

func TestExportPostmortem_DefaultFormatIsMarkdown(t *testing.T) {
	pmSvc := new(MockPostmortemSvc)
	incSvc := new(mockIncidentSvcForExport)
	eh := NewExportHandler(pmSvc, incSvc)
	r := setupExportRouter(eh)

	incidentID := uuid.New()
	pm := samplePM(incidentID)

	pmSvc.On("GetByIncidentID", mock.Anything, incidentID).Return(pm, nil)
	incSvc.On("GetByID", mock.Anything, incidentID).Return(&models.Incident{ID: incidentID}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/incidents/"+incidentID.String()+"/postmortem/export", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/markdown")
}

func TestExportPostmortem_PDF(t *testing.T) {
	pmSvc := new(MockPostmortemSvc)
	incSvc := new(mockIncidentSvcForExport)
	eh := NewExportHandler(pmSvc, incSvc)
	r := setupExportRouter(eh)

	incidentID := uuid.New()
	pm := samplePM(incidentID)

	pmSvc.On("GetByIncidentID", mock.Anything, incidentID).Return(pm, nil)
	incSvc.On("GetByID", mock.Anything, incidentID).Return(&models.Incident{ID: incidentID}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/incidents/"+incidentID.String()+"/postmortem/export?format=pdf", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="postmortem-`+incidentID.String()+`.pdf"`, w.Header().Get("Content-Disposition"))
	assert.True(t, len(w.Body.Bytes()) > 4 && string(w.Body.Bytes()[:5]) == "%PDF-")
}

func TestExportPostmortem_UnknownFormat(t *testing.T) {
	pmSvc := new(MockPostmortemSvc)
	incSvc := new(mockIncidentSvcForExport)
	eh := NewExportHandler(pmSvc, incSvc)
	r := setupExportRouter(eh)

	incidentID := uuid.New()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/incidents/"+incidentID.String()+"/postmortem/export?format=docx", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportPostmortem_NotFound(t *testing.T) {
	pmSvc := new(MockPostmortemSvc)
	incSvc := new(mockIncidentSvcForExport)
	eh := NewExportHandler(pmSvc, incSvc)
	r := setupExportRouter(eh)

	incidentID := uuid.New()
	pmSvc.On("GetByIncidentID", mock.Anything, incidentID).Return(nil, assert.AnError)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/incidents/"+incidentID.String()+"/postmortem/export", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestExportPostmortem_InvalidID(t *testing.T) {
	eh := NewExportHandler(new(MockPostmortemSvc), new(mockIncidentSvcForExport))
	r := setupExportRouter(eh)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/incidents/not-a-uuid/postmortem/export", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
