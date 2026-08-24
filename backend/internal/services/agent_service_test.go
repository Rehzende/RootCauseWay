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

type MockAgentRepo struct {
	mock.Mock
}

func (m *MockAgentRepo) Create(ctx context.Context, agent *models.Agent) error {
	return m.Called(ctx, agent).Error(0)
}

func (m *MockAgentRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}

func (m *MockAgentRepo) List(ctx context.Context, orgID uuid.UUID, agentType string, page, perPage int) ([]models.Agent, int, error) {
	args := m.Called(ctx, orgID, agentType, page, perPage)
	return args.Get(0).([]models.Agent), args.Int(1), args.Error(2)
}

func (m *MockAgentRepo) Update(ctx context.Context, agent *models.Agent) error {
	return m.Called(ctx, agent).Error(0)
}

func (m *MockAgentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func TestAgentService_Create(t *testing.T) {
	repo := new(MockAgentRepo)
	svc := NewAgentService(repo)

	orgID := uuid.New()
	req := models.CreateAgentRequest{
		Name:   "Triage Agent",
		Type:   "triage",
		Config: models.AgentConfig{Model: "claude-sonnet-4-20250514"},
	}

	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.Agent")).Return(nil)

	result, err := svc.Create(context.Background(), orgID, req)

	require.NoError(t, err)
	assert.Equal(t, "Triage Agent", result.Name)
	assert.Equal(t, "triage", result.Type)
	assert.True(t, result.Enabled)
	repo.AssertExpectations(t)
}

func TestAgentService_GetByID(t *testing.T) {
	repo := new(MockAgentRepo)
	svc := NewAgentService(repo)

	id := uuid.New()
	expected := &models.Agent{ID: id, Name: "Test"}
	repo.On("GetByID", mock.Anything, id).Return(expected, nil)

	result, err := svc.GetByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestAgentService_List(t *testing.T) {
	repo := new(MockAgentRepo)
	svc := NewAgentService(repo)

	orgID := uuid.New()
	agents := []models.Agent{{Name: "A"}}
	repo.On("List", mock.Anything, orgID, "triage", 1, 20).Return(agents, 1, nil)

	result, total, err := svc.List(context.Background(), orgID, "triage", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, result, 1)
}

func TestAgentService_Delete(t *testing.T) {
	repo := new(MockAgentRepo)
	svc := NewAgentService(repo)

	id := uuid.New()
	repo.On("Delete", mock.Anything, id).Return(nil)

	err := svc.Delete(context.Background(), id)
	assert.NoError(t, err)
}
