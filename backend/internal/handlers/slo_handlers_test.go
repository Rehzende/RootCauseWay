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
	"github.com/jackc/pgx/v5"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mock SLOServiceInterface ---

type MockSLOSvc struct{ mock.Mock }

func (m *MockSLOSvc) Create(ctx context.Context, orgID uuid.UUID, req models.CreateSLODefinitionRequest) (*models.SLODefinition, error) {
	args := m.Called(ctx, orgID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SLODefinition), args.Error(1)
}

func (m *MockSLOSvc) GetByID(ctx context.Context, id uuid.UUID) (*models.SLODefinition, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SLODefinition), args.Error(1)
}

func (m *MockSLOSvc) List(ctx context.Context, orgID uuid.UUID) ([]models.SLODefinition, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.SLODefinition), args.Error(1)
}

func (m *MockSLOSvc) Update(ctx context.Context, id uuid.UUID, req models.UpdateSLODefinitionRequest) (*models.SLODefinition, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SLODefinition), args.Error(1)
}

func (m *MockSLOSvc) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockSLOSvc) CalculateSLOStatus(ctx context.Context, sloDefinitionID uuid.UUID) (*models.SLOStatus, error) {
	args := m.Called(ctx, sloDefinitionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SLOStatus), args.Error(1)
}

func (m *MockSLOSvc) SoftwareSLOStatus(ctx context.Context, softwareID uuid.UUID) (*models.SoftwareSLOStatus, error) {
	args := m.Called(ctx, softwareID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SoftwareSLOStatus), args.Error(1)
}

// --- Test helpers ---

var sloTestOrgID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

func setupSLORouter(sh *SLOHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	r.Use(func(c *gin.Context) {
		c.Set("org_id", sloTestOrgID)
		c.Next()
	})

	api := r.Group("/api/v1")
	api.GET("/slo-definitions", sh.ListSLODefinitions)
	api.POST("/slo-definitions", sh.CreateSLODefinition)
	api.GET("/slo-definitions/:id", sh.GetSLODefinition)
	api.PUT("/slo-definitions/:id", sh.UpdateSLODefinition)
	api.DELETE("/slo-definitions/:id", sh.DeleteSLODefinition)
	api.GET("/slo-definitions/:id/status", sh.GetSLOStatus)
	api.GET("/software/:id/slo-status", sh.GetSoftwareSLOStatus)

	return r
}

func sampleSLODefModel() *models.SLODefinition {
	return &models.SLODefinition{
		ID:                    uuid.New(),
		OrgID:                 sloTestOrgID,
		SoftwareID:            uuid.New(),
		Name:                  "API availability",
		SLOType:               models.SLOTypeAvailability,
		TargetPercentage:      99.9,
		MeasurementWindowDays: 30,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}
}

// --- CRUD tests ---

func TestListSLODefinitions_Success(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	items := []models.SLODefinition{*sampleSLODefModel()}
	svc.On("List", mock.Anything, sloTestOrgID).Return(items, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/slo-definitions", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp []models.SLODefinition
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 1)
}

func TestCreateSLODefinition_Success(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	created := sampleSLODefModel()
	svc.On("Create", mock.Anything, sloTestOrgID, mock.AnythingOfType("models.CreateSLODefinitionRequest")).
		Return(created, nil)

	body, _ := json.Marshal(models.CreateSLODefinitionRequest{
		SoftwareID:       created.SoftwareID,
		Name:             created.Name,
		SLOType:          created.SLOType,
		TargetPercentage: created.TargetPercentage,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/slo-definitions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	svc.AssertExpectations(t)
}

func TestCreateSLODefinition_InvalidBody(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/slo-definitions", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateSLODefinition_InvalidSLOType(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	body, _ := json.Marshal(map[string]interface{}{
		"software_id":       uuid.New().String(),
		"name":              "Bad SLO",
		"slo_type":          "not_a_real_type",
		"target_percentage": 99.9,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/slo-definitions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetSLODefinition_Success(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	def := sampleSLODefModel()
	svc.On("GetByID", mock.Anything, def.ID).Return(def, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/slo-definitions/"+def.ID.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetSLODefinition_NotFound(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	id := uuid.New()
	svc.On("GetByID", mock.Anything, id).Return(nil, pgx.ErrNoRows)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/slo-definitions/"+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetSLODefinition_InvalidID(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/slo-definitions/not-a-uuid", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateSLODefinition_Success(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	def := sampleSLODefModel()
	updateReq := models.UpdateSLODefinitionRequest{TargetPercentage: 99.95}
	svc.On("Update", mock.Anything, def.ID, updateReq).Return(def, nil)

	body, _ := json.Marshal(updateReq)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("PUT", "/api/v1/slo-definitions/"+def.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestDeleteSLODefinition_Success(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	id := uuid.New()
	svc.On("Delete", mock.Anything, id).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("DELETE", "/api/v1/slo-definitions/"+id.String(), nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

// --- Status endpoint tests ---

func TestGetSLOStatus_Success(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	def := sampleSLODefModel()
	status := &models.SLOStatus{
		SLODefinitionID:   def.ID,
		SoftwareID:        def.SoftwareID,
		SLOType:           def.SLOType,
		CurrentPercentage: 99.98,
		Status:            models.SLOStatusHealthy,
	}
	svc.On("CalculateSLOStatus", mock.Anything, def.ID).Return(status, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/slo-definitions/"+def.ID.String()+"/status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.SLOStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, models.SLOStatusHealthy, resp.Status)
}

func TestGetSLOStatus_InvalidID(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/slo-definitions/not-a-uuid/status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetSoftwareSLOStatus_Success(t *testing.T) {
	svc := new(MockSLOSvc)
	sh := NewSLOHandler(svc)
	r := setupSLORouter(sh)

	softwareID := uuid.New()
	result := &models.SoftwareSLOStatus{
		SoftwareID: softwareID,
		SLOs: []models.SLOStatus{
			{SLODefinitionID: uuid.New(), Status: models.SLOStatusHealthy},
		},
	}
	svc.On("SoftwareSLOStatus", mock.Anything, softwareID).Return(result, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/software/"+softwareID.String()+"/slo-status", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.SoftwareSLOStatus
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, softwareID, resp.SoftwareID)
	assert.Len(t, resp.SLOs, 1)
}
