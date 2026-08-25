package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockIncidentRepo struct {
	mock.Mock
}

func (m *MockIncidentRepo) Create(ctx context.Context, incident *models.Incident) error {
	return m.Called(ctx, incident).Error(0)
}

func (m *MockIncidentRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Incident), args.Error(1)
}

func (m *MockIncidentRepo) List(ctx context.Context, orgID uuid.UUID, status, severity string, softwareID *uuid.UUID, from *time.Time, page, perPage int) ([]models.Incident, int, error) {
	args := m.Called(ctx, orgID, status, severity, softwareID, from, page, perPage)
	return args.Get(0).([]models.Incident), args.Int(1), args.Error(2)
}

func (m *MockIncidentRepo) Update(ctx context.Context, incident *models.Incident) error {
	return m.Called(ctx, incident).Error(0)
}

func (m *MockIncidentRepo) AddEvent(ctx context.Context, event *models.IncidentEvent) error {
	return m.Called(ctx, event).Error(0)
}

func (m *MockIncidentRepo) AddEvidence(ctx context.Context, evidence *models.IncidentEvidence) error {
	return m.Called(ctx, evidence).Error(0)
}

func (m *MockIncidentRepo) GetEvents(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentEvent, error) {
	args := m.Called(ctx, incidentID)
	return args.Get(0).([]models.IncidentEvent), args.Error(1)
}

func (m *MockIncidentRepo) GetEvidence(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentEvidence, error) {
	args := m.Called(ctx, incidentID)
	return args.Get(0).([]models.IncidentEvidence), args.Error(1)
}

func (m *MockIncidentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type MockAlertSnapshotRepo struct {
	mock.Mock
}

func (m *MockAlertSnapshotRepo) Create(ctx context.Context, snapshot *models.AlertSnapshot) error {
	return m.Called(ctx, snapshot).Error(0)
}

func TestIncidentService_Create(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	incident := &models.Incident{
		OrgID:      uuid.New(),
		SoftwareID: uuid.New(),
		Title:      "High CPU Alert",
		Severity:   "critical",
	}

	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.Incident")).Return(nil)

	err := svc.Create(context.Background(), incident)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, incident.ID)
	assert.Equal(t, "open", incident.Status)
}

func TestIncidentService_Update_Resolved(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	id := uuid.New()
	existing := &models.Incident{ID: id, Status: "open", Severity: "high"}

	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.Incident")).Return(nil)

	status := "resolved"
	result, justTerminalized, err := svc.Update(context.Background(), id, models.UpdateIncidentRequest{Status: &status})

	require.NoError(t, err)
	assert.Equal(t, "resolved", result.Status)
	assert.NotNil(t, result.ResolvedAt)
	assert.True(t, justTerminalized)
}

// TestIncidentService_Update_ClosedDirectly guards against the gap found
// when a real "fechei o incidente" (closed via the flat status picker,
// skipping "resolved") never stamped resolved_at and so never triggered
// postmortem generation.
func TestIncidentService_Update_ClosedDirectly(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	id := uuid.New()
	existing := &models.Incident{ID: id, Status: "investigating", Severity: "high"}

	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.Incident")).Return(nil)

	status := "closed"
	result, justTerminalized, err := svc.Update(context.Background(), id, models.UpdateIncidentRequest{Status: &status})

	require.NoError(t, err)
	assert.Equal(t, "closed", result.Status)
	assert.NotNil(t, result.ResolvedAt)
	assert.True(t, justTerminalized)
}

// TestIncidentService_Update_ClosedAfterResolved_DoesNotRefireTerminal
// covers an incident that goes resolved -> closed later: postmortem should
// not be triggered a second time.
func TestIncidentService_Update_ClosedAfterResolved_DoesNotRefireTerminal(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	id := uuid.New()
	alreadyResolvedAt := time.Now().Add(-time.Hour)
	existing := &models.Incident{ID: id, Status: "resolved", Severity: "high", ResolvedAt: &alreadyResolvedAt}

	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.Incident")).Return(nil)

	status := "closed"
	result, justTerminalized, err := svc.Update(context.Background(), id, models.UpdateIncidentRequest{Status: &status})

	require.NoError(t, err)
	assert.Equal(t, "closed", result.Status)
	assert.False(t, justTerminalized)
	assert.Equal(t, alreadyResolvedAt, *result.ResolvedAt)
}

// TestIncidentService_Update_AssignsIncident pins the fix for the "Assign"
// button on the incident detail page: it had no onClick at all (a
// completely dead button), and separately -- with AssigneeID typed
// *uuid.UUID -- there was no way to distinguish "field absent" from
// "please clear the assignee" over JSON (both unmarshal to a nil pointer).
func TestIncidentService_Update_AssignsIncident(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	id := uuid.New()
	existing := &models.Incident{ID: id, Status: "open", Severity: "high"}

	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.Incident")).Return(nil)

	userID := uuid.New().String()
	result, _, err := svc.Update(context.Background(), id, models.UpdateIncidentRequest{AssigneeID: &userID})

	require.NoError(t, err)
	require.NotNil(t, result.AssigneeID)
	assert.Equal(t, userID, result.AssigneeID.String())
}

// TestIncidentService_Update_UnassignsViaEmptyString pins the "" sentinel:
// AssigneeID is now *string specifically so an explicit empty string ("",
// never a valid UUID) can mean "clear it" while a nil pointer (key absent)
// still means "don't touch assignment".
func TestIncidentService_Update_UnassignsViaEmptyString(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	id := uuid.New()
	existingAssignee := uuid.New()
	existing := &models.Incident{ID: id, Status: "open", Severity: "high", AssigneeID: &existingAssignee}

	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.Incident")).Return(nil)

	empty := ""
	result, _, err := svc.Update(context.Background(), id, models.UpdateIncidentRequest{AssigneeID: &empty})

	require.NoError(t, err)
	assert.Nil(t, result.AssigneeID)
}

func TestIncidentService_Update_NilAssigneeLeavesExistingUntouched(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	id := uuid.New()
	existingAssignee := uuid.New()
	existing := &models.Incident{ID: id, Status: "open", Severity: "high", AssigneeID: &existingAssignee}

	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.Incident")).Return(nil)

	status := "investigating"
	result, _, err := svc.Update(context.Background(), id, models.UpdateIncidentRequest{Status: &status})

	require.NoError(t, err)
	require.NotNil(t, result.AssigneeID)
	assert.Equal(t, existingAssignee, *result.AssigneeID)
}

func TestIncidentService_Update_InvalidAssigneeIDReturnsError(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	id := uuid.New()
	existing := &models.Incident{ID: id, Status: "open", Severity: "high"}
	repo.On("GetByID", mock.Anything, id).Return(existing, nil)

	notAUUID := "not-a-uuid"
	_, _, err := svc.Update(context.Background(), id, models.UpdateIncidentRequest{AssigneeID: &notAUUID})

	require.Error(t, err)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

// TestIncidentService_Delete pins the fix for "nao tem opcao de deletar
// incidente, mas tem permissao" -- the incidents:delete permission has
// existed in the RBAC catalog since migration 010, but no repository
// method, service method, handler, or route ever backed it.
func TestIncidentService_Delete(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	id := uuid.New()
	repo.On("Delete", mock.Anything, id).Return(nil)

	err := svc.Delete(context.Background(), id)

	require.NoError(t, err)
	repo.AssertCalled(t, "Delete", mock.Anything, id)
}

func TestIncidentService_AddEvent(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	incidentID := uuid.New()
	req := models.CreateEventRequest{
		Type: "comment",
		Data: json.RawMessage(`{"text": "investigating"}`),
	}

	repo.On("AddEvent", mock.Anything, mock.AnythingOfType("*models.IncidentEvent")).Return(nil)

	event, err := svc.AddEvent(context.Background(), incidentID, "user@test.com", req)

	require.NoError(t, err)
	assert.Equal(t, "comment", event.Type)
	assert.Equal(t, "user@test.com", event.Actor)
}

func TestIncidentService_AddEvidence(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	incidentID := uuid.New()
	req := models.CreateEvidenceRequest{
		Type:    "log",
		Title:   "Error logs",
		Content: json.RawMessage(`{"lines": ["error: OOM"]}`),
		Source:  "cloudwatch",
	}

	repo.On("AddEvidence", mock.Anything, mock.AnythingOfType("*models.IncidentEvidence")).Return(nil)

	evidence, err := svc.AddEvidence(context.Background(), incidentID, req)

	require.NoError(t, err)
	assert.Equal(t, "log", evidence.Type)
	assert.Equal(t, "Error logs", evidence.Title)
}

func TestIncidentService_GetByID_WithTimeline(t *testing.T) {
	repo := new(MockIncidentRepo)
	snapRepo := new(MockAlertSnapshotRepo)
	svc := NewIncidentService(repo, snapRepo)

	id := uuid.New()
	incident := &models.Incident{ID: id, Title: "Test"}
	events := []models.IncidentEvent{{Type: "comment"}}
	evidence := []models.IncidentEvidence{{Type: "log"}}

	repo.On("GetByID", mock.Anything, id).Return(incident, nil)
	repo.On("GetEvents", mock.Anything, id).Return(events, nil)
	repo.On("GetEvidence", mock.Anything, id).Return(evidence, nil)

	result, err := svc.GetByID(context.Background(), id)

	require.NoError(t, err)
	assert.Len(t, result.Timeline, 1)
	assert.Len(t, result.Evidence, 1)
}
