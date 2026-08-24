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
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type MockKnowledgeBaseSvc struct{ mock.Mock }

func (m *MockKnowledgeBaseSvc) Create(ctx context.Context, orgID uuid.UUID, req models.CreateKnowledgeBaseRequest) (*models.KnowledgeBaseEntry, error) {
	args := m.Called(ctx, orgID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.KnowledgeBaseEntry), args.Error(1)
}
func (m *MockKnowledgeBaseSvc) GetByID(ctx context.Context, id uuid.UUID) (*models.KnowledgeBaseEntry, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.KnowledgeBaseEntry), args.Error(1)
}
func (m *MockKnowledgeBaseSvc) List(ctx context.Context, orgID uuid.UUID, category string) ([]models.KnowledgeBaseEntry, error) {
	args := m.Called(ctx, orgID, category)
	return args.Get(0).([]models.KnowledgeBaseEntry), args.Error(1)
}
func (m *MockKnowledgeBaseSvc) Search(ctx context.Context, orgID uuid.UUID, softwareID *uuid.UUID, errorPattern string) ([]models.KnowledgeBaseEntry, error) {
	args := m.Called(ctx, orgID, softwareID, errorPattern)
	return args.Get(0).([]models.KnowledgeBaseEntry), args.Error(1)
}
func (m *MockKnowledgeBaseSvc) Update(ctx context.Context, id uuid.UUID, req models.CreateKnowledgeBaseRequest) (*models.KnowledgeBaseEntry, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.KnowledgeBaseEntry), args.Error(1)
}
func (m *MockKnowledgeBaseSvc) IncrementReferences(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockKnowledgeBaseSvc) CreateFromHumanCorrection(ctx context.Context, orgID, incidentID uuid.UUID, softwareID *uuid.UUID, rootCauseSummary string) (*models.KnowledgeBaseEntry, error) {
	args := m.Called(ctx, orgID, incidentID, softwareID, rootCauseSummary)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.KnowledgeBaseEntry), args.Error(1)
}

type MockFeedbackSvc struct{ mock.Mock }

func (m *MockFeedbackSvc) Create(ctx context.Context, incidentID uuid.UUID, userID *uuid.UUID, req models.CreateFeedbackRequest) (*models.IncidentFeedback, error) {
	args := m.Called(ctx, incidentID, userID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.IncidentFeedback), args.Error(1)
}
func (m *MockFeedbackSvc) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentFeedback, error) {
	args := m.Called(ctx, incidentID)
	return args.Get(0).([]models.IncidentFeedback), args.Error(1)
}

type MockNotifLogSvc struct{ mock.Mock }

func (m *MockNotifLogSvc) Create(ctx context.Context, orgID uuid.UUID, req models.CreateNotificationLogRequest) (*models.NotificationLogEntry, error) {
	args := m.Called(ctx, orgID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.NotificationLogEntry), args.Error(1)
}
func (m *MockNotifLogSvc) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.NotificationLogEntry, error) {
	args := m.Called(ctx, incidentID)
	return args.Get(0).([]models.NotificationLogEntry), args.Error(1)
}
func (m *MockNotifLogSvc) ListByOrg(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.NotificationLogEntry, int, error) {
	args := m.Called(ctx, orgID, page, perPage)
	return args.Get(0).([]models.NotificationLogEntry), args.Int(1), args.Error(2)
}
func (m *MockNotifLogSvc) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errMsg string) error {
	return m.Called(ctx, id, status, errMsg).Error(0)
}

type MockRunbookStepSvc struct{ mock.Mock }

func (m *MockRunbookStepSvc) Create(ctx context.Context, runbookID uuid.UUID, req models.CreateRunbookStepRequest) (*models.RunbookStep, error) {
	args := m.Called(ctx, runbookID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RunbookStep), args.Error(1)
}
func (m *MockRunbookStepSvc) ListByRunbook(ctx context.Context, runbookID uuid.UUID) ([]models.RunbookStep, error) {
	args := m.Called(ctx, runbookID)
	return args.Get(0).([]models.RunbookStep), args.Error(1)
}
func (m *MockRunbookStepSvc) Update(ctx context.Context, id uuid.UUID, req models.CreateRunbookStepRequest) (*models.RunbookStep, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RunbookStep), args.Error(1)
}
func (m *MockRunbookStepSvc) Reorder(ctx context.Context, orderedStepIDs []uuid.UUID) error {
	return m.Called(ctx, orderedStepIDs).Error(0)
}
func (m *MockRunbookStepSvc) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type MockRunbookExecSvc struct{ mock.Mock }

func (m *MockRunbookExecSvc) Create(ctx context.Context, req models.CreateRunbookExecutionRequest) (*models.RunbookExecution, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RunbookExecution), args.Error(1)
}
func (m *MockRunbookExecSvc) GetByID(ctx context.Context, id uuid.UUID) (*models.RunbookExecution, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RunbookExecution), args.Error(1)
}
func (m *MockRunbookExecSvc) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.RunbookExecution, error) {
	args := m.Called(ctx, incidentID)
	return args.Get(0).([]models.RunbookExecution), args.Error(1)
}
func (m *MockRunbookExecSvc) ListByRunbook(ctx context.Context, runbookID uuid.UUID) ([]models.RunbookExecution, error) {
	args := m.Called(ctx, runbookID)
	return args.Get(0).([]models.RunbookExecution), args.Error(1)
}
func (m *MockRunbookExecSvc) Update(ctx context.Context, id uuid.UUID, req models.UpdateRunbookExecutionRequest) (*models.RunbookExecution, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.RunbookExecution), args.Error(1)
}

// --- Test helpers ---

func setupFeaturesRouter(fh *FeaturesHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("org_id", uuid.MustParse("00000000-0000-0000-0000-000000000001"))
		c.Next()
	})
	internal := r.Group("/internal")
	internal.GET("/knowledge-base/search", fh.SearchKnowledgeBase)
	internal.POST("/knowledge-base/search", fh.SearchKnowledgeBase)
	internal.POST("/knowledge-base/:id/increment-references", fh.IncrementKnowledgeBaseReferences)
	protected := r.Group("/api/v1")
	protected.POST("/incidents/:id/feedback", fh.CreateFeedback)
	protected.GET("/notifications/logs", fh.ListNotificationLogsGlobal)
	protected.POST("/runbooks/:id/steps/reorder", fh.ReorderRunbookSteps)
	protected.GET("/runbooks/:id/executions", fh.ListRunbookExecutionsByRunbook)
	protected.POST("/runbook-executions/:execId/steps/:stepId/complete", fh.CompleteExecutionStep)
	return r
}

// --- Tests ---

// TestSearchKnowledgeBase_GETWithQueryParams guards the actual production
// bug: agent-service's search_knowledge_base sends a GET with software_id/
// query/limit as query params (see backend_client.py), but the handler
// used ShouldBindJSON (body-only) -- every internal search silently came
// back empty/400 instead of finding anything.
func TestSearchKnowledgeBase_GETWithQueryParams(t *testing.T) {
	kb := new(MockKnowledgeBaseSvc)
	fh := &FeaturesHandler{KnowledgeBase: kb}
	r := setupFeaturesRouter(fh)

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	softwareID := uuid.New()
	entries := []models.KnowledgeBaseEntry{
		{ID: uuid.New(), OrgID: orgID, ErrorPattern: "connection pool exhausted"},
		{ID: uuid.New(), OrgID: orgID, ErrorPattern: "connection timeout"},
	}
	kb.On("Search", mock.Anything, orgID, &softwareID, "pool exhaustion").Return(entries, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/internal/knowledge-base/search?software_id="+softwareID.String()+"&query=pool+exhaustion&limit=1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var got []models.KnowledgeBaseEntry
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1, "limit=1 query param should truncate the 2 matches down to 1")
}

func TestSearchKnowledgeBase_POSTWithJSONBody(t *testing.T) {
	kb := new(MockKnowledgeBaseSvc)
	fh := &FeaturesHandler{KnowledgeBase: kb}
	r := setupFeaturesRouter(fh)

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	kb.On("Search", mock.Anything, orgID, (*uuid.UUID)(nil), "timeout").
		Return([]models.KnowledgeBaseEntry{}, nil)

	body, _ := json.Marshal(models.KnowledgeBaseSearchRequest{Query: "timeout"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/knowledge-base/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIncrementKnowledgeBaseReferences(t *testing.T) {
	kb := new(MockKnowledgeBaseSvc)
	fh := &FeaturesHandler{KnowledgeBase: kb}
	r := setupFeaturesRouter(fh)

	entryID := uuid.New()
	kb.On("IncrementReferences", mock.Anything, entryID).Return(nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/knowledge-base/"+entryID.String()+"/increment-references", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	kb.AssertCalled(t, "IncrementReferences", mock.Anything, entryID)
}

// TestCreateFeedback_NegativeWithCorrection_PromotesToKnowledgeBase covers
// the fix for the human-feedback loop being dead code end-to-end: a
// negative rating with a correction must reach the knowledge base as a
// human_validated entry, not just sit in incident_feedback unused. Also
// covers resolving the incident's software_id along the way, so the
// promoted entry isn't orphaned from service-scoped searches.
func TestCreateFeedback_NegativeWithCorrection_PromotesToKnowledgeBase(t *testing.T) {
	fb := new(MockFeedbackSvc)
	kb := new(MockKnowledgeBaseSvc)
	inc := new(MockIncidentSvc)
	fh := &FeaturesHandler{Feedback: fb, KnowledgeBase: kb, Incidents: inc}
	r := setupFeaturesRouter(fh)

	incidentID := uuid.New()
	softwareID := uuid.New()
	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	fb.On("Create", mock.Anything, incidentID, (*uuid.UUID)(nil), mock.Anything).
		Return(&models.IncidentFeedback{ID: uuid.New(), IncidentID: incidentID}, nil)
	inc.On("GetByID", mock.Anything, incidentID).
		Return(&models.Incident{ID: incidentID, SoftwareID: softwareID}, nil)
	kb.On("CreateFromHumanCorrection", mock.Anything, orgID, incidentID, &softwareID, "actual root cause was X").
		Return(&models.KnowledgeBaseEntry{ID: uuid.New()}, nil)

	body, _ := json.Marshal(models.CreateFeedbackRequest{
		TargetType: "rca", Rating: "negative", Correction: "actual root cause was X",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/incidents/"+incidentID.String()+"/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	kb.AssertCalled(t, "CreateFromHumanCorrection", mock.Anything, orgID, incidentID, &softwareID, "actual root cause was X")
}

// TestCreateFeedback_IncidentLookupFails_StillPromotesWithoutSoftwareID
// makes sure a failure resolving software_id degrades gracefully (nil
// software_id) instead of blocking the promotion entirely.
func TestCreateFeedback_IncidentLookupFails_StillPromotesWithoutSoftwareID(t *testing.T) {
	fb := new(MockFeedbackSvc)
	kb := new(MockKnowledgeBaseSvc)
	inc := new(MockIncidentSvc)
	fh := &FeaturesHandler{Feedback: fb, KnowledgeBase: kb, Incidents: inc}
	r := setupFeaturesRouter(fh)

	incidentID := uuid.New()
	fb.On("Create", mock.Anything, incidentID, (*uuid.UUID)(nil), mock.Anything).
		Return(&models.IncidentFeedback{ID: uuid.New(), IncidentID: incidentID}, nil)
	inc.On("GetByID", mock.Anything, incidentID).
		Return(nil, assert.AnError)
	kb.On("CreateFromHumanCorrection", mock.Anything, mock.Anything, incidentID, (*uuid.UUID)(nil), "X").
		Return(&models.KnowledgeBaseEntry{ID: uuid.New()}, nil)

	body, _ := json.Marshal(models.CreateFeedbackRequest{
		TargetType: "rca", Rating: "negative", Correction: "X",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/incidents/"+incidentID.String()+"/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	kb.AssertCalled(t, "CreateFromHumanCorrection", mock.Anything, mock.Anything, incidentID, (*uuid.UUID)(nil), "X")
}

func TestCreateFeedback_PositiveRating_DoesNotPromoteToKnowledgeBase(t *testing.T) {
	fb := new(MockFeedbackSvc)
	kb := new(MockKnowledgeBaseSvc)
	fh := &FeaturesHandler{Feedback: fb, KnowledgeBase: kb}
	r := setupFeaturesRouter(fh)

	incidentID := uuid.New()
	fb.On("Create", mock.Anything, incidentID, (*uuid.UUID)(nil), mock.Anything).
		Return(&models.IncidentFeedback{ID: uuid.New(), IncidentID: incidentID}, nil)

	body, _ := json.Marshal(models.CreateFeedbackRequest{
		TargetType: "rca", Rating: "positive",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/incidents/"+incidentID.String()+"/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	kb.AssertNotCalled(t, "CreateFromHumanCorrection", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCreateFeedback_NegativeWithoutCorrection_DoesNotPromoteToKnowledgeBase(t *testing.T) {
	fb := new(MockFeedbackSvc)
	kb := new(MockKnowledgeBaseSvc)
	fh := &FeaturesHandler{Feedback: fb, KnowledgeBase: kb}
	r := setupFeaturesRouter(fh)

	incidentID := uuid.New()
	fb.On("Create", mock.Anything, incidentID, (*uuid.UUID)(nil), mock.Anything).
		Return(&models.IncidentFeedback{ID: uuid.New(), IncidentID: incidentID}, nil)

	body, _ := json.Marshal(models.CreateFeedbackRequest{
		TargetType: "rca", Rating: "negative", Correction: "   ",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/incidents/"+incidentID.String()+"/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	kb.AssertNotCalled(t, "CreateFromHumanCorrection", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// TestCreateFeedback_KnowledgeBasePromotionFails_StillReturns201 makes sure
// the promotion is truly best-effort: the feedback itself was already
// durably saved, so a KB failure must not turn into a client-facing error.
func TestCreateFeedback_KnowledgeBasePromotionFails_StillReturns201(t *testing.T) {
	fb := new(MockFeedbackSvc)
	kb := new(MockKnowledgeBaseSvc)
	fh := &FeaturesHandler{Feedback: fb, KnowledgeBase: kb}
	r := setupFeaturesRouter(fh)

	incidentID := uuid.New()
	fb.On("Create", mock.Anything, incidentID, (*uuid.UUID)(nil), mock.Anything).
		Return(&models.IncidentFeedback{ID: uuid.New(), IncidentID: incidentID}, nil)
	kb.On("CreateFromHumanCorrection", mock.Anything, mock.Anything, incidentID, (*uuid.UUID)(nil), "X").
		Return(nil, assert.AnError)

	body, _ := json.Marshal(models.CreateFeedbackRequest{
		TargetType: "rca", Rating: "negative", Correction: "X",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/incidents/"+incidentID.String()+"/feedback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
}

func TestIncrementKnowledgeBaseReferences_InvalidID(t *testing.T) {
	fh := &FeaturesHandler{KnowledgeBase: new(MockKnowledgeBaseSvc)}
	r := setupFeaturesRouter(fh)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/internal/knowledge-base/not-a-uuid/increment-references", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestListNotificationLogsGlobal guards a capability that never existed:
// the frontend's Notifications "Logs" tab calls GET /notifications/logs
// with no incident scoping, but only the per-incident ListByIncident path
// was ever wired end-to-end -- this route 404'd and the tab always
// rendered empty.
func TestListNotificationLogsGlobal_ReturnsOrgScopedPage(t *testing.T) {
	nl := new(MockNotifLogSvc)
	fh := &FeaturesHandler{NotifLog: nl}
	r := setupFeaturesRouter(fh)

	orgID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	entries := []models.NotificationLogEntry{{ID: uuid.New(), OrgID: orgID}}
	nl.On("ListByOrg", mock.Anything, orgID, 1, 20).Return(entries, 1, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/notifications/logs", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.PaginatedResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 1, resp.Total)
}

// TestReorderRunbookSteps guards a capability that was a no-op end to end:
// the frontend's reorderRunbookSteps() called this route already, but it
// never existed on the backend.
func TestReorderRunbookSteps_OK(t *testing.T) {
	rs := new(MockRunbookStepSvc)
	fh := &FeaturesHandler{RunbookSteps: rs}
	r := setupFeaturesRouter(fh)

	step1, step2 := uuid.New(), uuid.New()
	rs.On("Reorder", mock.Anything, []uuid.UUID{step2, step1}).Return(nil)

	body, _ := json.Marshal(map[string]any{"step_ids": []string{step2.String(), step1.String()}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/runbooks/"+uuid.New().String()+"/steps/reorder", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	rs.AssertExpectations(t)
}

func TestReorderRunbookSteps_InvalidStepID_Returns400(t *testing.T) {
	rs := new(MockRunbookStepSvc)
	fh := &FeaturesHandler{RunbookSteps: rs}
	r := setupFeaturesRouter(fh)

	body, _ := json.Marshal(map[string]any{"step_ids": []string{"not-a-uuid"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/runbooks/"+uuid.New().String()+"/steps/reorder", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	rs.AssertNotCalled(t, "Reorder", mock.Anything, mock.Anything)
}

// TestListRunbookExecutionsByRunbook guards a capability that never
// existed: RunbookDetailPage's execution history is scoped by runbook_id,
// but only the by-incident listing route was ever wired.
func TestListRunbookExecutionsByRunbook_OK(t *testing.T) {
	re := new(MockRunbookExecSvc)
	fh := &FeaturesHandler{RunbookExecs: re}
	r := setupFeaturesRouter(fh)

	runbookID := uuid.New()
	re.On("ListByRunbook", mock.Anything, runbookID).
		Return([]models.RunbookExecution{{ID: uuid.New(), RunbookID: runbookID}}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/runbooks/"+runbookID.String()+"/executions", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var items []models.RunbookExecution
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	assert.Len(t, items, 1)
}

// TestCompleteExecutionStep_* guards two things: the pre-existing behavior
// (a human "Mark Complete" click, no body, always marks "completed") stays
// unchanged, and the new behavior RunbookExecutor's automation loop needs
// (an explicit status/output for an automated step, including "failed") now
// works -- this handler used to hardcode "completed" unconditionally.
func TestCompleteExecutionStep_NoBody_DefaultsToCompleted(t *testing.T) {
	re := new(MockRunbookExecSvc)
	fh := &FeaturesHandler{RunbookExecs: re}
	r := setupFeaturesRouter(fh)

	execID := uuid.New()
	stepResults, _ := json.Marshal([]models.ExecutionStepResult{
		{StepID: "s1", StepType: "manual", Status: "pending_action"},
	})
	re.On("GetByID", mock.Anything, execID).
		Return(&models.RunbookExecution{ID: execID, StepResults: stepResults, CurrentStep: 0}, nil)
	re.On("Update", mock.Anything, execID, mock.MatchedBy(func(req models.UpdateRunbookExecutionRequest) bool {
		var results []models.ExecutionStepResult
		_ = json.Unmarshal(req.StepResults, &results)
		return len(results) == 1 && results[0].Status == "completed"
	})).Return(&models.RunbookExecution{ID: execID}, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/runbook-executions/"+execID.String()+"/steps/s1/complete", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	re.AssertExpectations(t)
}

func TestCompleteExecutionStep_ExplicitFailedStatus_RecordsFailureAndOutput(t *testing.T) {
	re := new(MockRunbookExecSvc)
	fh := &FeaturesHandler{RunbookExecs: re}
	r := setupFeaturesRouter(fh)

	execID := uuid.New()
	stepResults, _ := json.Marshal([]models.ExecutionStepResult{
		{StepID: "s1", StepType: "automated", Status: "running"},
	})
	re.On("GetByID", mock.Anything, execID).
		Return(&models.RunbookExecution{ID: execID, StepResults: stepResults, CurrentStep: 0}, nil)
	re.On("Update", mock.Anything, execID, mock.MatchedBy(func(req models.UpdateRunbookExecutionRequest) bool {
		var results []models.ExecutionStepResult
		_ = json.Unmarshal(req.StepResults, &results)
		return len(results) == 1 && results[0].Status == "failed" && results[0].Output["error"] == "agent unreachable"
	})).Return(&models.RunbookExecution{ID: execID}, nil)

	body, _ := json.Marshal(map[string]any{"status": "failed", "output": map[string]any{"error": "agent unreachable"}})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/runbook-executions/"+execID.String()+"/steps/s1/complete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	re.AssertExpectations(t)
}
