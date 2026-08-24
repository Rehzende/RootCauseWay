package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/integrations/teams"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type MockWarRoomRepo struct{ mock.Mock }

func (m *MockWarRoomRepo) Create(ctx context.Context, meeting *models.WarRoomMeeting) error {
	return m.Called(ctx, meeting).Error(0)
}
func (m *MockWarRoomRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.WarRoomMeeting, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarRoomMeeting), args.Error(1)
}
func (m *MockWarRoomRepo) GetLatestByIncident(ctx context.Context, incidentID uuid.UUID) (*models.WarRoomMeeting, error) {
	args := m.Called(ctx, incidentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.WarRoomMeeting), args.Error(1)
}
func (m *MockWarRoomRepo) Update(ctx context.Context, meeting *models.WarRoomMeeting) error {
	return m.Called(ctx, meeting).Error(0)
}

type MockIncidentReader struct{ mock.Mock }

func (m *MockIncidentReader) GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Incident), args.Error(1)
}

type MockEventAdder struct{ mock.Mock }

func (m *MockEventAdder) AddEvent(ctx context.Context, incidentID uuid.UUID, actor string, req models.CreateEventRequest) (*models.IncidentEvent, error) {
	args := m.Called(ctx, incidentID, actor, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.IncidentEvent), args.Error(1)
}

type MockPublisher struct{ mock.Mock }

func (m *MockPublisher) Publish(ctx context.Context, channel string, event models.EventEnvelope) error {
	return m.Called(ctx, channel, event).Error(0)
}

// fixedTeamsResolver wraps an already-constructed teams.TeamsClient as a
// TeamsClientResolver that ignores orgID and always returns it -- these
// tests exercise WarRoomService's own logic, not per-org resolution
// (that's covered separately in teams_client_resolver_test.go).
func fixedTeamsResolver(client teams.TeamsClient) TeamsClientResolver {
	return func(ctx context.Context, orgID uuid.UUID) (teams.TeamsClient, error) {
		return client, nil
	}
}

// --- Tests ---

func TestWarRoomService_CreateWarRoom(t *testing.T) {
	repo := new(MockWarRoomRepo)
	incidents := new(MockIncidentReader)
	events := new(MockEventAdder)
	teamsClient := teams.NewMockTeamsClient()

	svc := NewWarRoomService(repo, fixedTeamsResolver(teamsClient), incidents)
	svc.SetIncidentEventAdder(events)

	incidentID := uuid.New()
	orgID := uuid.New()
	incident := &models.Incident{ID: incidentID, OrgID: orgID, Title: "Checkout outage"}

	incidents.On("GetByID", mock.Anything, incidentID).Return(incident, nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.WarRoomMeeting")).Return(nil)
	events.On("AddEvent", mock.Anything, incidentID, "system", mock.AnythingOfType("models.CreateEventRequest")).
		Return(&models.IncidentEvent{}, nil)

	meeting, err := svc.CreateWarRoom(context.Background(), incidentID)

	require.NoError(t, err)
	assert.Equal(t, orgID, meeting.OrgID)
	assert.Equal(t, incidentID, meeting.IncidentID)
	assert.Equal(t, "teams", meeting.Provider)
	assert.Equal(t, "scheduled", meeting.Status)
	assert.NotEmpty(t, meeting.ExternalMeetingID)
	assert.NotEmpty(t, meeting.JoinURL)
	repo.AssertExpectations(t)
	events.AssertExpectations(t)
}

func TestWarRoomService_CreateWarRoom_TeamsClientError(t *testing.T) {
	repo := new(MockWarRoomRepo)
	incidents := new(MockIncidentReader)
	noopClient := teams.NewNoopTeamsClient()

	svc := NewWarRoomService(repo, fixedTeamsResolver(noopClient), incidents)

	incidentID := uuid.New()
	incident := &models.Incident{ID: incidentID, OrgID: uuid.New(), Title: "Checkout outage"}
	incidents.On("GetByID", mock.Anything, incidentID).Return(incident, nil)

	_, err := svc.CreateWarRoom(context.Background(), incidentID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestWarRoomService_EndWarRoom(t *testing.T) {
	repo := new(MockWarRoomRepo)
	incidents := new(MockIncidentReader)
	publisher := new(MockPublisher)
	teamsClient := teams.NewMockTeamsClient()

	svc := NewWarRoomService(repo, fixedTeamsResolver(teamsClient), incidents)
	svc.SetEventPublisher(publisher)

	meetingID := uuid.New()
	incidentID := uuid.New()
	orgID := uuid.New()
	started := time.Now().Add(-15 * time.Minute)
	meeting := &models.WarRoomMeeting{
		ID:                meetingID,
		OrgID:             orgID,
		IncidentID:        incidentID,
		Provider:          "teams",
		ExternalMeetingID: "mock-meeting-1",
		Status:            "scheduled",
		StartedAt:         &started,
	}

	repo.On("GetByID", mock.Anything, meetingID).Return(meeting, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.WarRoomMeeting")).Return(nil)
	publisher.On("Publish", mock.Anything, "rootcauseway:"+orgID.String()+":warroom.meeting.ended", mock.AnythingOfType("models.EventEnvelope")).
		Return(nil)

	result, err := svc.EndWarRoom(context.Background(), meetingID)

	require.NoError(t, err)
	assert.Equal(t, "ended", result.Status)
	assert.NotNil(t, result.EndedAt)
	require.NotNil(t, result.RawTranscript)
	assert.Contains(t, *result.RawTranscript, "war room")
	assert.NotNil(t, result.Attendance)

	var attendees []teams.Attendee
	require.NoError(t, json.Unmarshal(result.Attendance, &attendees))
	assert.Len(t, attendees, 3)

	repo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestWarRoomService_AttachSummary(t *testing.T) {
	repo := new(MockWarRoomRepo)
	incidents := new(MockIncidentReader)
	teamsClient := teams.NewMockTeamsClient()

	svc := NewWarRoomService(repo, fixedTeamsResolver(teamsClient), incidents)

	meetingID := uuid.New()
	meeting := &models.WarRoomMeeting{
		ID:     meetingID,
		Status: "ended",
	}
	repo.On("GetByID", mock.Anything, meetingID).Return(meeting, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.WarRoomMeeting")).Return(nil)

	summary := models.WarRoomSummary{
		ExecutiveSummary: "Checkout outage caused by bad deploy, rolled back.",
		KeyPoints:        []string{"502s spiked at 14:02 UTC", "rollback resolved the issue"},
		ActionItems: []models.WarRoomActionItem{
			{Description: "Add canary check before deploys", OwnerHint: "Bob"},
		},
	}
	participants := []models.WarRoomAttendee{
		{Name: "Alice", Email: "alice@example.com"},
	}

	result, err := svc.AttachSummary(context.Background(), meetingID, summary, participants)

	require.NoError(t, err)
	assert.Equal(t, "summarized", result.Status)

	var gotSummary models.WarRoomSummary
	require.NoError(t, json.Unmarshal(result.Summary, &gotSummary))
	assert.Equal(t, summary.ExecutiveSummary, gotSummary.ExecutiveSummary)
	assert.Len(t, gotSummary.ActionItems, 1)

	var gotAttendance []models.WarRoomAttendee
	require.NoError(t, json.Unmarshal(result.Attendance, &gotAttendance))
	assert.Len(t, gotAttendance, 1)
	assert.Equal(t, "Alice", gotAttendance[0].Name)

	repo.AssertExpectations(t)
}

// TestWarRoomService_CreateWarRoom_ResolvesClientByIncidentOrg proves
// CreateWarRoom actually passes the incident's own OrgID through to the
// resolver -- not a fixed org, not the zero value -- since that's the
// entire point of moving off the old single-client-for-the-process setup.
func TestWarRoomService_CreateWarRoom_ResolvesClientByIncidentOrg(t *testing.T) {
	repo := new(MockWarRoomRepo)
	incidents := new(MockIncidentReader)

	incidentID := uuid.New()
	orgID := uuid.New()
	incident := &models.Incident{ID: incidentID, OrgID: orgID, Title: "Checkout outage"}
	incidents.On("GetByID", mock.Anything, incidentID).Return(incident, nil)
	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.WarRoomMeeting")).Return(nil)

	var resolvedFor uuid.UUID
	resolver := TeamsClientResolver(func(ctx context.Context, gotOrgID uuid.UUID) (teams.TeamsClient, error) {
		resolvedFor = gotOrgID
		return teams.NewMockTeamsClient(), nil
	})
	svc := NewWarRoomService(repo, resolver, incidents)

	_, err := svc.CreateWarRoom(context.Background(), incidentID)

	require.NoError(t, err)
	assert.Equal(t, orgID, resolvedFor)
}

// TestWarRoomService_EndWarRoom_ResolverError_StillMarksEnded covers the
// best-effort degradation: if the org's Teams client can't be resolved
// (e.g. a transient DB error looking up settings), the meeting still gets
// marked ended -- transcript/attendance just aren't attached, same as any
// other transcript-fetch failure.
func TestWarRoomService_EndWarRoom_ResolverError_StillMarksEnded(t *testing.T) {
	repo := new(MockWarRoomRepo)
	incidents := new(MockIncidentReader)

	meetingID := uuid.New()
	meeting := &models.WarRoomMeeting{
		ID:                meetingID,
		OrgID:             uuid.New(),
		ExternalMeetingID: "mock-meeting-1",
		Status:            "scheduled",
	}
	repo.On("GetByID", mock.Anything, meetingID).Return(meeting, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.WarRoomMeeting")).Return(nil)

	resolver := TeamsClientResolver(func(ctx context.Context, orgID uuid.UUID) (teams.TeamsClient, error) {
		return nil, errors.New("settings lookup failed")
	})
	svc := NewWarRoomService(repo, resolver, incidents)

	result, err := svc.EndWarRoom(context.Background(), meetingID)

	require.NoError(t, err)
	assert.Equal(t, "ended", result.Status)
	assert.Nil(t, result.RawTranscript)
	assert.Nil(t, result.Attendance)
}

func TestWarRoomService_GetByIncident(t *testing.T) {
	repo := new(MockWarRoomRepo)
	incidents := new(MockIncidentReader)
	teamsClient := teams.NewMockTeamsClient()
	svc := NewWarRoomService(repo, fixedTeamsResolver(teamsClient), incidents)

	incidentID := uuid.New()
	expected := &models.WarRoomMeeting{ID: uuid.New(), IncidentID: incidentID}
	repo.On("GetLatestByIncident", mock.Anything, incidentID).Return(expected, nil)

	got, err := svc.GetByIncident(context.Background(), incidentID)

	require.NoError(t, err)
	assert.Equal(t, expected.ID, got.ID)
}
