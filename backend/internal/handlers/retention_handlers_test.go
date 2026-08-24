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
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mock ---

type MockRetentionService struct{ mock.Mock }

func (m *MockRetentionService) CreatePolicy(ctx context.Context, orgID uuid.UUID, req models.CreateRetentionPolicyRequest) (*models.RetentionPolicy, error) {
	args := m.Called(ctx, orgID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RetentionPolicy), args.Error(1)
}
func (m *MockRetentionService) GetPolicy(ctx context.Context, id uuid.UUID) (*models.RetentionPolicy, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RetentionPolicy), args.Error(1)
}
func (m *MockRetentionService) ListPolicies(ctx context.Context, orgID uuid.UUID) ([]models.RetentionPolicy, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).([]models.RetentionPolicy), args.Error(1)
}
func (m *MockRetentionService) UpdatePolicy(ctx context.Context, id uuid.UUID, req models.UpdateRetentionPolicyRequest) (*models.RetentionPolicy, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RetentionPolicy), args.Error(1)
}
func (m *MockRetentionService) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockRetentionService) RunRetentionSweep(ctx context.Context, orgID uuid.UUID) (*models.RetentionSweepSummary, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RetentionSweepSummary), args.Error(1)
}

// --- Test helpers ---

var retentionTestOrgID = uuid.MustParse("00000000-0000-0000-0000-000000000003")

func setupRetentionRouter(rh *RetentionHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("org_id", retentionTestOrgID)
		c.Next()
	})

	api := r.Group("/api/v1")
	api.GET("/retention-policies", rh.ListRetentionPolicies)
	api.POST("/retention-policies", rh.CreateRetentionPolicy)
	api.PUT("/retention-policies/:id", rh.UpdateRetentionPolicy)
	api.DELETE("/retention-policies/:id", rh.DeleteRetentionPolicy)
	api.POST("/retention-policies/sweep", rh.TriggerSweep)

	return r
}

// --- Tests ---

func TestListRetentionPolicies_Success(t *testing.T) {
	svc := new(MockRetentionService)
	rh := NewRetentionHandler(svc)
	r := setupRetentionRouter(rh)

	svc.On("ListPolicies", mock.Anything, retentionTestOrgID).
		Return([]models.RetentionPolicy{{ID: uuid.New(), OrgID: retentionTestOrgID, ResourceType: "evidence", RetentionDays: 90, Action: "archive", Enabled: true}}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/retention-policies", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestCreateRetentionPolicy_Success(t *testing.T) {
	svc := new(MockRetentionService)
	rh := NewRetentionHandler(svc)
	r := setupRetentionRouter(rh)

	req := models.CreateRetentionPolicyRequest{ResourceType: "evidence", RetentionDays: 90, Action: "archive"}
	created := &models.RetentionPolicy{ID: uuid.New(), OrgID: retentionTestOrgID, ResourceType: "evidence", RetentionDays: 90, Action: "archive", Enabled: true}
	svc.On("CreatePolicy", mock.Anything, retentionTestOrgID, req).Return(created, nil)

	body, _ := json.Marshal(req)
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/retention-policies", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp models.RetentionPolicy
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 90, resp.RetentionDays)
	svc.AssertExpectations(t)
}

func TestCreateRetentionPolicy_InvalidBody(t *testing.T) {
	svc := new(MockRetentionService)
	rh := NewRetentionHandler(svc)
	r := setupRetentionRouter(rh)

	// resource_type missing -> binding:"required,oneof=..." fails
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/retention-policies", bytes.NewReader([]byte(`{"retention_days":90,"action":"archive"}`)))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	svc.AssertNotCalled(t, "CreatePolicy", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateRetentionPolicy_Success(t *testing.T) {
	svc := new(MockRetentionService)
	rh := NewRetentionHandler(svc)
	r := setupRetentionRouter(rh)

	id := uuid.New()
	existing := &models.RetentionPolicy{ID: id, OrgID: retentionTestOrgID, ResourceType: "incidents", RetentionDays: 365, Action: "archive", Enabled: true}
	svc.On("GetPolicy", mock.Anything, id).Return(existing, nil)

	newDays := 180
	updateReq := models.UpdateRetentionPolicyRequest{RetentionDays: &newDays}
	updated := &models.RetentionPolicy{ID: id, OrgID: retentionTestOrgID, ResourceType: "incidents", RetentionDays: 180, Action: "archive", Enabled: true}
	svc.On("UpdatePolicy", mock.Anything, id, updateReq).Return(updated, nil)

	body, _ := json.Marshal(updateReq)
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("PUT", "/api/v1/retention-policies/"+id.String(), bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestUpdateRetentionPolicy_ForbiddenCrossOrg(t *testing.T) {
	svc := new(MockRetentionService)
	rh := NewRetentionHandler(svc)
	r := setupRetentionRouter(rh)

	id := uuid.New()
	otherOrg := uuid.New()
	existing := &models.RetentionPolicy{ID: id, OrgID: otherOrg, ResourceType: "incidents", RetentionDays: 365, Action: "archive", Enabled: true}
	svc.On("GetPolicy", mock.Anything, id).Return(existing, nil)

	newDays := 180
	body, _ := json.Marshal(models.UpdateRetentionPolicyRequest{RetentionDays: &newDays})
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("PUT", "/api/v1/retention-policies/"+id.String(), bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertNotCalled(t, "UpdatePolicy", mock.Anything, mock.Anything, mock.Anything)
}

func TestUpdateRetentionPolicy_NotFound(t *testing.T) {
	svc := new(MockRetentionService)
	rh := NewRetentionHandler(svc)
	r := setupRetentionRouter(rh)

	id := uuid.New()
	svc.On("GetPolicy", mock.Anything, id).Return(nil, pgx.ErrNoRows)

	newDays := 180
	body, _ := json.Marshal(models.UpdateRetentionPolicyRequest{RetentionDays: &newDays})
	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("PUT", "/api/v1/retention-policies/"+id.String(), bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDeleteRetentionPolicy_Success(t *testing.T) {
	svc := new(MockRetentionService)
	rh := NewRetentionHandler(svc)
	r := setupRetentionRouter(rh)

	id := uuid.New()
	existing := &models.RetentionPolicy{ID: id, OrgID: retentionTestOrgID, ResourceType: "evidence", RetentionDays: 90, Action: "delete", Enabled: true}
	svc.On("GetPolicy", mock.Anything, id).Return(existing, nil)
	svc.On("DeletePolicy", mock.Anything, id).Return(nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("DELETE", "/api/v1/retention-policies/"+id.String(), nil)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusNoContent, w.Code)
	svc.AssertExpectations(t)
}

func TestDeleteRetentionPolicy_ForbiddenCrossOrg(t *testing.T) {
	svc := new(MockRetentionService)
	rh := NewRetentionHandler(svc)
	r := setupRetentionRouter(rh)

	id := uuid.New()
	otherOrg := uuid.New()
	existing := &models.RetentionPolicy{ID: id, OrgID: otherOrg, ResourceType: "evidence", RetentionDays: 90, Action: "delete", Enabled: true}
	svc.On("GetPolicy", mock.Anything, id).Return(existing, nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("DELETE", "/api/v1/retention-policies/"+id.String(), nil)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusForbidden, w.Code)
	svc.AssertNotCalled(t, "DeletePolicy", mock.Anything, mock.Anything)
}

func TestTriggerSweep_Success(t *testing.T) {
	svc := new(MockRetentionService)
	rh := NewRetentionHandler(svc)
	r := setupRetentionRouter(rh)

	summary := &models.RetentionSweepSummary{OrgID: retentionTestOrgID, Results: []models.RetentionSweepResult{}}
	svc.On("RunRetentionSweep", mock.Anything, retentionTestOrgID).Return(summary, nil)

	w := httptest.NewRecorder()
	httpReq, _ := http.NewRequest("POST", "/api/v1/retention-policies/sweep", nil)
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}
