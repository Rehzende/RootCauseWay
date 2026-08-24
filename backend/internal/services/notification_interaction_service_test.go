package services

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mocks ---

type MockNotifIncidentReader struct{ mock.Mock }

func (m *MockNotifIncidentReader) GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Incident), args.Error(1)
}

type MockNotifIncidentUpdater struct{ mock.Mock }

func (m *MockNotifIncidentUpdater) Update(ctx context.Context, id uuid.UUID, req models.UpdateIncidentRequest) (*models.Incident, bool, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).(*models.Incident), args.Bool(1), args.Error(2)
}

type MockNotifRCAReader struct{ mock.Mock }

func (m *MockNotifRCAReader) GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentRCA, error) {
	args := m.Called(ctx, incidentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.IncidentRCA), args.Error(1)
}

type MockNotifChannelReader struct{ mock.Mock }

func (m *MockNotifChannelReader) GetByID(ctx context.Context, id uuid.UUID) (*models.NotificationChannel, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.NotificationChannel), args.Error(1)
}

type MockNotifInteractionRecorder struct{ mock.Mock }

func (m *MockNotifInteractionRecorder) Create(ctx context.Context, interaction *models.NotificationInteraction) error {
	return m.Called(ctx, interaction).Error(0)
}

// --- Tests ---

func newTestNotificationInteractionService() (*NotificationInteractionService, *MockNotifIncidentReader, *MockNotifIncidentUpdater, *MockNotifRCAReader, *MockNotifChannelReader, *MockNotifInteractionRecorder) {
	reader := new(MockNotifIncidentReader)
	updater := new(MockNotifIncidentUpdater)
	rca := new(MockNotifRCAReader)
	channels := new(MockNotifChannelReader)
	interactions := new(MockNotifInteractionRecorder)
	svc := NewNotificationInteractionService(reader, updater, rca, channels, interactions)
	return svc, reader, updater, rca, channels, interactions
}

func TestNotificationInteractionService_Acknowledge(t *testing.T) {
	svc, reader, updater, _, channels, interactions := newTestNotificationInteractionService()

	incidentID := uuid.New()
	channelID := uuid.New()
	orgID := uuid.New()
	updated := &models.Incident{ID: incidentID, OrgID: orgID, Status: "investigating"}

	updater.On("Update", mock.Anything, incidentID, mock.MatchedBy(func(req models.UpdateIncidentRequest) bool {
		return req.Status != nil && *req.Status == "investigating"
	})).Return(updated, true, nil)
	channels.On("GetByID", mock.Anything, channelID).Return(&models.NotificationChannel{ID: channelID, OrgID: orgID, ChannelType: "slack"}, nil)
	interactions.On("Create", mock.Anything, mock.MatchedBy(func(ni *models.NotificationInteraction) bool {
		return ni.Action == InteractionAcknowledge && ni.Status == "ok" && ni.IncidentID == incidentID
	})).Return(nil)

	result, err := svc.Dispatch(context.Background(), incidentID, &channelID, "slack", InteractionAcknowledge, "alice", "ts-1")

	require.NoError(t, err)
	assert.Equal(t, "investigating", result.Incident.Status)
	assert.Contains(t, result.Message, "acknowledged")
	reader.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	interactions.AssertExpectations(t)
}

func TestNotificationInteractionService_Resolve(t *testing.T) {
	svc, _, updater, _, channels, interactions := newTestNotificationInteractionService()

	incidentID := uuid.New()
	channelID := uuid.New()
	orgID := uuid.New()
	updated := &models.Incident{ID: incidentID, OrgID: orgID, Status: "resolved"}

	updater.On("Update", mock.Anything, incidentID, mock.MatchedBy(func(req models.UpdateIncidentRequest) bool {
		return req.Status != nil && *req.Status == "resolved"
	})).Return(updated, true, nil)
	channels.On("GetByID", mock.Anything, channelID).Return(&models.NotificationChannel{ID: channelID, OrgID: orgID, ChannelType: "teams"}, nil)
	interactions.On("Create", mock.Anything, mock.AnythingOfType("*models.NotificationInteraction")).Return(nil)

	result, err := svc.Dispatch(context.Background(), incidentID, &channelID, "teams", InteractionResolve, "bob", "ts-2")

	require.NoError(t, err)
	assert.Equal(t, "resolved", result.Incident.Status)
	assert.Contains(t, result.Message, "resolved")
}

func TestNotificationInteractionService_ViewRCA(t *testing.T) {
	svc, reader, _, rca, channels, interactions := newTestNotificationInteractionService()

	incidentID := uuid.New()
	channelID := uuid.New()
	orgID := uuid.New()
	incident := &models.Incident{ID: incidentID, OrgID: orgID, Title: "Checkout down"}
	rcaRecord := &models.IncidentRCA{ID: uuid.New(), IncidentID: incidentID, Status: "completed", RootCauseSummary: "DB connection pool exhaustion", Confidence: 0.8}

	reader.On("GetByID", mock.Anything, incidentID).Return(incident, nil)
	rca.On("GetByIncidentID", mock.Anything, incidentID).Return(rcaRecord, nil)
	channels.On("GetByID", mock.Anything, channelID).Return(&models.NotificationChannel{ID: channelID, OrgID: orgID, ChannelType: "slack"}, nil)
	interactions.On("Create", mock.Anything, mock.AnythingOfType("*models.NotificationInteraction")).Return(nil)

	result, err := svc.Dispatch(context.Background(), incidentID, &channelID, "slack", InteractionViewRCA, "carol", "ts-3")

	require.NoError(t, err)
	assert.Equal(t, rcaRecord, result.RCA)
	assert.Contains(t, result.Message, "DB connection pool exhaustion")
}

func TestNotificationInteractionService_ViewRCA_NoRCAYet(t *testing.T) {
	svc, reader, _, rca, channels, interactions := newTestNotificationInteractionService()

	incidentID := uuid.New()
	channelID := uuid.New()
	incident := &models.Incident{ID: incidentID, OrgID: uuid.New(), Title: "Checkout down"}

	reader.On("GetByID", mock.Anything, incidentID).Return(incident, nil)
	rca.On("GetByIncidentID", mock.Anything, incidentID).Return(nil, assert.AnError)
	channels.On("GetByID", mock.Anything, channelID).Return(&models.NotificationChannel{ID: channelID, OrgID: incident.OrgID, ChannelType: "slack"}, nil)
	interactions.On("Create", mock.Anything, mock.AnythingOfType("*models.NotificationInteraction")).Return(nil)

	result, err := svc.Dispatch(context.Background(), incidentID, &channelID, "slack", InteractionViewRCA, "carol", "ts-4")

	require.NoError(t, err)
	assert.Nil(t, result.RCA)
	assert.Contains(t, result.Message, "No RCA is available yet")
}

func TestNotificationInteractionService_UnknownAction(t *testing.T) {
	svc, _, _, _, channels, interactions := newTestNotificationInteractionService()

	incidentID := uuid.New()
	channelID := uuid.New()

	channels.On("GetByID", mock.Anything, channelID).Return(&models.NotificationChannel{ID: channelID, OrgID: uuid.New(), ChannelType: "slack"}, nil)
	interactions.On("Create", mock.Anything, mock.MatchedBy(func(ni *models.NotificationInteraction) bool {
		return ni.Status == "error"
	})).Return(nil)

	_, err := svc.Dispatch(context.Background(), incidentID, &channelID, "slack", "delete_everything", "eve", "ts-5")

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownInteractionAction)
}

func TestNotificationInteractionService_AcknowledgeError_StillRecordsInteraction(t *testing.T) {
	svc, _, updater, _, channels, interactions := newTestNotificationInteractionService()

	incidentID := uuid.New()
	channelID := uuid.New()

	updater.On("Update", mock.Anything, incidentID, mock.Anything).Return(nil, false, assert.AnError)
	channels.On("GetByID", mock.Anything, channelID).Return(&models.NotificationChannel{ID: channelID, OrgID: uuid.New(), ChannelType: "slack"}, nil)
	interactions.On("Create", mock.Anything, mock.MatchedBy(func(ni *models.NotificationInteraction) bool {
		return ni.Status == "error" && ni.ErrorMessage != ""
	})).Return(nil)

	_, err := svc.Dispatch(context.Background(), incidentID, &channelID, "slack", InteractionAcknowledge, "dave", "ts-6")

	require.Error(t, err)
	interactions.AssertExpectations(t)
}

func TestNotificationInteractionService_SlackSigningSecret(t *testing.T) {
	svc, _, _, _, channels, _ := newTestNotificationInteractionService()

	channelID := uuid.New()
	cfg, _ := json.Marshal(models.SlackChannelConfig{WebhookURL: "https://hooks.slack.com/x", SigningSecret: "shh-secret"})
	channels.On("GetByID", mock.Anything, channelID).Return(&models.NotificationChannel{ID: channelID, ChannelType: "slack", Config: cfg}, nil)

	secret, err := svc.SlackSigningSecret(context.Background(), channelID)

	require.NoError(t, err)
	assert.Equal(t, "shh-secret", secret)
}

func TestNotificationInteractionService_SlackSigningSecret_WrongChannelType(t *testing.T) {
	svc, _, _, _, channels, _ := newTestNotificationInteractionService()

	channelID := uuid.New()
	channels.On("GetByID", mock.Anything, channelID).Return(&models.NotificationChannel{ID: channelID, ChannelType: "teams"}, nil)

	_, err := svc.SlackSigningSecret(context.Background(), channelID)

	require.Error(t, err)
}

func TestNotificationInteractionService_TeamsVerificationToken(t *testing.T) {
	svc, _, _, _, channels, _ := newTestNotificationInteractionService()

	channelID := uuid.New()
	cfg, _ := json.Marshal(models.TeamsChannelConfig{WebhookURL: "https://outlook.office.com/x", VerificationToken: "tok-123"})
	channels.On("GetByID", mock.Anything, channelID).Return(&models.NotificationChannel{ID: channelID, ChannelType: "teams", Config: cfg}, nil)

	token, err := svc.TeamsVerificationToken(context.Background(), channelID)

	require.NoError(t, err)
	assert.Equal(t, "tok-123", token)
}
