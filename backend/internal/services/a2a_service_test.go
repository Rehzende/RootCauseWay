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

type MockA2AAgentRepo struct{ mock.Mock }

func (m *MockA2AAgentRepo) Create(ctx context.Context, a *models.A2AAgent) error {
	return m.Called(ctx, a).Error(0)
}
func (m *MockA2AAgentRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.A2AAgent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.A2AAgent), args.Error(1)
}
func (m *MockA2AAgentRepo) List(ctx context.Context, orgID uuid.UUID, agentType string, page, perPage int) ([]models.A2AAgent, int, error) {
	args := m.Called(ctx, orgID, agentType, page, perPage)
	return args.Get(0).([]models.A2AAgent), args.Int(1), args.Error(2)
}
func (m *MockA2AAgentRepo) Update(ctx context.Context, a *models.A2AAgent) error {
	return m.Called(ctx, a).Error(0)
}
func (m *MockA2AAgentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockA2AAgentRepo) GetBySkill(ctx context.Context, orgID uuid.UUID, skill string) ([]models.A2AAgent, error) {
	args := m.Called(ctx, orgID, skill)
	return args.Get(0).([]models.A2AAgent), args.Error(1)
}
func (m *MockA2AAgentRepo) HealthCheck(ctx context.Context, id uuid.UUID, status string) error {
	return m.Called(ctx, id, status).Error(0)
}

// TestA2AAgentService_Update_AppliesManagedConfigOverride covers the
// per-agent LLM override (model/temperature) written by the Agents page's
// edit form -- see the LLM & Tokens settings feature. Update fetches the
// existing agent and overlays only the fields the request actually sends;
// this pins that ManagedConfig is one of them, since it was missing
// entirely from CreateA2AAgentRequest until this feature (meaning the
// override could never actually be saved through this endpoint before).
func TestA2AAgentService_Update_AppliesManagedConfigOverride(t *testing.T) {
	repo := new(MockA2AAgentRepo)
	svc := NewA2AAgentService(repo)

	agentID := uuid.New()
	existing := &models.A2AAgent{
		ID: agentID, Name: "rca-agent", HostingType: "managed", LLMProvider: "platform",
		ManagedConfig: json.RawMessage(`{}`),
	}
	repo.On("GetByID", mock.Anything, agentID).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(a *models.A2AAgent) bool {
		return string(a.ManagedConfig) == `{"model":"anthropic/claude-sonnet-4-6","temperature":0.4}`
	})).Return(nil)

	req := models.CreateA2AAgentRequest{
		Name: "rca-agent", AgentType: "rca",
		ManagedConfig: json.RawMessage(`{"model":"anthropic/claude-sonnet-4-6","temperature":0.4}`),
	}
	updated, err := svc.Update(context.Background(), agentID, req)

	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"anthropic/claude-sonnet-4-6","temperature":0.4}`, string(updated.ManagedConfig))
	repo.AssertExpectations(t)
}

// TestA2AAgentService_Update_PreservesManagedConfigWhenNotProvided ensures
// a request that doesn't touch managed_config (e.g. the plain "edit name/
// endpoint" flow that predates this feature) doesn't silently wipe out an
// existing per-agent override.
func TestA2AAgentService_Update_PreservesManagedConfigWhenNotProvided(t *testing.T) {
	repo := new(MockA2AAgentRepo)
	svc := NewA2AAgentService(repo)

	agentID := uuid.New()
	existing := &models.A2AAgent{
		ID: agentID, Name: "rca-agent",
		ManagedConfig: json.RawMessage(`{"model":"existing-override"}`),
	}
	repo.On("GetByID", mock.Anything, agentID).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(a *models.A2AAgent) bool {
		return string(a.ManagedConfig) == `{"model":"existing-override"}`
	})).Return(nil)

	req := models.CreateA2AAgentRequest{Name: "rca-agent renamed", AgentType: "rca"}
	updated, err := svc.Update(context.Background(), agentID, req)

	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"existing-override"}`, string(updated.ManagedConfig))
	repo.AssertExpectations(t)
}
