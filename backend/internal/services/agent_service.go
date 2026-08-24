package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// AgentRepository defines the DB operations for agents
type AgentRepository interface {
	Create(ctx context.Context, agent *models.Agent) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error)
	List(ctx context.Context, orgID uuid.UUID, agentType string, page, perPage int) ([]models.Agent, int, error)
	Update(ctx context.Context, agent *models.Agent) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type AgentService struct {
	repo AgentRepository
}

func NewAgentService(repo AgentRepository) *AgentService {
	return &AgentService{repo: repo}
}

func (s *AgentService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateAgentRequest) (*models.Agent, error) {
	agent := &models.Agent{
		ID:          uuid.New(),
		OrgID:       orgID,
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Config:      req.Config,
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.repo.Create(ctx, agent); err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *AgentService) GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AgentService) List(ctx context.Context, orgID uuid.UUID, agentType string, page, perPage int) ([]models.Agent, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	return s.repo.List(ctx, orgID, agentType, page, perPage)
}

func (s *AgentService) Update(ctx context.Context, id uuid.UUID, req models.CreateAgentRequest) (*models.Agent, error) {
	agent, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	agent.Name = req.Name
	agent.Type = req.Type
	agent.Description = req.Description
	agent.Config = req.Config
	agent.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, agent); err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *AgentService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
