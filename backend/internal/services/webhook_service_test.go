package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockWebhookRepo struct {
	mock.Mock
}

func (m *MockWebhookRepo) Create(ctx context.Context, webhook *models.Webhook) error {
	return m.Called(ctx, webhook).Error(0)
}

func (m *MockWebhookRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Webhook), args.Error(1)
}

func (m *MockWebhookRepo) GetByToken(ctx context.Context, token string) (*models.Webhook, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Webhook), args.Error(1)
}

func (m *MockWebhookRepo) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.Webhook, int, error) {
	args := m.Called(ctx, orgID, page, perPage)
	return args.Get(0).([]models.Webhook), args.Int(1), args.Error(2)
}

func (m *MockWebhookRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func TestWebhookService_Create(t *testing.T) {
	repo := new(MockWebhookRepo)
	svc := NewWebhookService(repo)

	orgID := uuid.New()
	softwareID := uuid.New()
	req := models.CreateWebhookRequest{
		Name:       "Datadog Webhook",
		Source:     "datadog",
		SoftwareID: softwareID,
	}

	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.Webhook")).Return(nil)

	result, err := svc.Create(context.Background(), orgID, req)

	require.NoError(t, err)
	assert.Equal(t, "Datadog Webhook", result.Name)
	assert.Equal(t, "datadog", result.Source)
	assert.Equal(t, softwareID, result.SoftwareID)
	assert.NotEmpty(t, result.EndpointToken)
	assert.NotEmpty(t, result.Secret)
	assert.True(t, result.Enabled)
	assert.Len(t, result.EndpointToken, 64) // 32 bytes = 64 hex chars
	repo.AssertExpectations(t)
}

func TestWebhookService_GetByToken(t *testing.T) {
	repo := new(MockWebhookRepo)
	svc := NewWebhookService(repo)

	token := "abc123"
	expected := &models.Webhook{EndpointToken: token}
	repo.On("GetByToken", mock.Anything, token).Return(expected, nil)

	result, err := svc.GetByToken(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, token, result.EndpointToken)
}

func TestWebhookService_Delete(t *testing.T) {
	repo := new(MockWebhookRepo)
	svc := NewWebhookService(repo)

	id := uuid.New()
	repo.On("Delete", mock.Anything, id).Return(nil)

	err := svc.Delete(context.Background(), id)
	assert.NoError(t, err)
}
