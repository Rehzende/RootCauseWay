package services

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, channel string, event models.EventEnvelope) error {
	return m.Called(ctx, channel, event).Error(0)
}

func TestIngestionService_IngestAlert_Datadog(t *testing.T) {
	webhookRepo := new(MockWebhookRepo)
	incidentRepo := new(MockIncidentRepo)
	snapshotRepo := new(MockAlertSnapshotRepo)
	publisher := new(MockEventPublisher)

	svc := NewIngestionService(webhookRepo, incidentRepo, snapshotRepo, publisher)

	orgID := uuid.New()
	softwareID := uuid.New()
	token := "test-token-123"

	webhook := &models.Webhook{
		ID:            uuid.New(),
		OrgID:         orgID,
		Source:        "datadog",
		SoftwareID:    softwareID,
		EndpointToken: token,
		Enabled:       true,
	}

	webhookRepo.On("GetByToken", mock.Anything, token).Return(webhook, nil)
	incidentRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Incident")).Return(nil)
	snapshotRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertSnapshot")).Return(nil)
	publisher.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	raw := json.RawMessage(`{"alert_id":"123","alert_title":"High CPU","alert_type":"error","hostname":"web-1","priority":"critical","tags":"service:api,env:prod"}`)

	result, err := svc.IngestAlert(context.Background(), token, raw)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, result.IncidentID)
	assert.NotEqual(t, uuid.Nil, result.AlertSnapshotID)

	webhookRepo.AssertExpectations(t)
	incidentRepo.AssertExpectations(t)
	snapshotRepo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestIngestionService_IngestAlert_WebhookNotFound(t *testing.T) {
	webhookRepo := new(MockWebhookRepo)
	incidentRepo := new(MockIncidentRepo)
	snapshotRepo := new(MockAlertSnapshotRepo)
	publisher := new(MockEventPublisher)

	svc := NewIngestionService(webhookRepo, incidentRepo, snapshotRepo, publisher)

	webhookRepo.On("GetByToken", mock.Anything, "bad-token").Return(nil, fmt.Errorf("not found"))

	_, err := svc.IngestAlert(context.Background(), "bad-token", json.RawMessage(`{}`))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "webhook not found")
}

func TestIngestionService_IngestAlert_DisabledWebhook(t *testing.T) {
	webhookRepo := new(MockWebhookRepo)
	incidentRepo := new(MockIncidentRepo)
	snapshotRepo := new(MockAlertSnapshotRepo)
	publisher := new(MockEventPublisher)

	svc := NewIngestionService(webhookRepo, incidentRepo, snapshotRepo, publisher)

	webhook := &models.Webhook{Enabled: false}
	webhookRepo.On("GetByToken", mock.Anything, "disabled").Return(webhook, nil)

	_, err := svc.IngestAlert(context.Background(), "disabled", json.RawMessage(`{}`))

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestIngestionService_IngestAlert_Prometheus(t *testing.T) {
	webhookRepo := new(MockWebhookRepo)
	incidentRepo := new(MockIncidentRepo)
	snapshotRepo := new(MockAlertSnapshotRepo)
	publisher := new(MockEventPublisher)

	svc := NewIngestionService(webhookRepo, incidentRepo, snapshotRepo, publisher)

	orgID := uuid.New()
	webhook := &models.Webhook{
		OrgID:      orgID,
		Source:     "prometheus_alertmanager",
		SoftwareID: uuid.New(),
		Enabled:    true,
	}

	webhookRepo.On("GetByToken", mock.Anything, "prom-token").Return(webhook, nil)
	incidentRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Incident")).Return(nil)
	snapshotRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertSnapshot")).Return(nil)
	publisher.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	raw := json.RawMessage(`{"alerts":[{"status":"firing","labels":{"alertname":"HighCPU","severity":"critical","service":"api"},"annotations":{"summary":"CPU > 90%"},"startsAt":"2026-01-01T00:00:00Z"}]}`)

	result, err := svc.IngestAlert(context.Background(), "prom-token", raw)

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, result.IncidentID)
}

// TestIngestionService_IngestAlert_PublishesIncidentCreated pins a live-found
// gap: nothing published an "incident.created" event, so the WebSocket
// bridge (internal/ws/redis_bridge.go) never had anything to forward for the
// frontend's live incident list/dashboard toast (IncidentsPage/DashboardPage,
// subscribed to exactly this topic) to react to -- confirmed by grep finding
// zero references to the string "incident.created" anywhere in the backend
// before this. Separate from "alert.received" (agent-service's pipeline
// trigger, unrelated to the WS bridge) published just above it.
func TestIngestionService_IngestAlert_PublishesIncidentCreated(t *testing.T) {
	webhookRepo := new(MockWebhookRepo)
	incidentRepo := new(MockIncidentRepo)
	snapshotRepo := new(MockAlertSnapshotRepo)
	publisher := new(MockEventPublisher)

	svc := NewIngestionService(webhookRepo, incidentRepo, snapshotRepo, publisher)

	orgID := uuid.New()
	softwareID := uuid.New()
	webhook := &models.Webhook{
		OrgID:      orgID,
		Source:     "prometheus_alertmanager",
		SoftwareID: softwareID,
		Enabled:    true,
	}

	webhookRepo.On("GetByToken", mock.Anything, "prom-token").Return(webhook, nil)
	incidentRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.Incident")).Return(nil)
	snapshotRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AlertSnapshot")).Return(nil)
	publisher.On("Publish", mock.Anything, "rootcauseway:"+orgID.String()+":alert.received", mock.Anything).Return(nil)
	publisher.On("Publish", mock.Anything, "rootcauseway:"+orgID.String()+":incident.created", mock.MatchedBy(func(e models.EventEnvelope) bool {
		payload, ok := e.Payload.(models.IncidentCreatedPayload)
		return ok && e.EventType == "incident.created" && payload.Title == "HighCPU" && payload.Severity == "critical"
	})).Return(nil)

	raw := json.RawMessage(`{"alerts":[{"status":"firing","labels":{"alertname":"HighCPU","severity":"critical","service":"api"},"annotations":{"summary":"CPU > 90%"},"startsAt":"2026-01-01T00:00:00Z"}]}`)

	_, err := svc.IngestAlert(context.Background(), "prom-token", raw)

	require.NoError(t, err)
	publisher.AssertExpectations(t)
}
