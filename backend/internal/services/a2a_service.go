package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// Repository interfaces for A2A services

type A2AAgentRepository interface {
	Create(ctx context.Context, a *models.A2AAgent) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.A2AAgent, error)
	List(ctx context.Context, orgID uuid.UUID, agentType string, page, perPage int) ([]models.A2AAgent, int, error)
	Update(ctx context.Context, a *models.A2AAgent) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetBySkill(ctx context.Context, orgID uuid.UUID, skill string) ([]models.A2AAgent, error)
	HealthCheck(ctx context.Context, id uuid.UUID, status string) error
}

type A2ATaskRepository interface {
	Create(ctx context.Context, t *models.A2ATask) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.A2ATask, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.A2ATask, error)
	Update(ctx context.Context, t *models.A2ATask) error
	ListByAgent(ctx context.Context, agentID uuid.UUID) ([]models.A2ATask, error)
}

type OrchestratorDecisionRepository interface {
	Create(ctx context.Context, d *models.OrchestratorDecision) error
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.OrchestratorDecision, error)
}

// --- A2AAgentService ---

type A2AAgentService struct {
	repo A2AAgentRepository
}

func NewA2AAgentService(repo A2AAgentRepository) *A2AAgentService {
	return &A2AAgentService{repo: repo}
}

func (s *A2AAgentService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateA2AAgentRequest) (*models.A2AAgent, error) {
	now := time.Now()
	agent := &models.A2AAgent{
		ID:                 uuid.New(),
		OrgID:              orgID,
		Name:               req.Name,
		Description:        req.Description,
		AgentType:          req.AgentType,
		EndpointURL:        req.EndpointURL,
		AgentCard:          req.AgentCard,
		Skills:             req.Skills,
		AllowedSoftwareIDs: req.AllowedSoftwareIDs,
		AuthType:           req.AuthType,
		AuthCredentials:    req.AuthCredentials,
		Enabled:            true,
		HealthStatus:       "unknown",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if agent.AgentCard == nil {
		agent.AgentCard = json.RawMessage("{}")
	}
	if agent.Skills == nil {
		agent.Skills = json.RawMessage("[]")
	}
	if agent.AllowedSoftwareIDs == nil {
		agent.AllowedSoftwareIDs = json.RawMessage("[]")
	}
	if err := s.repo.Create(ctx, agent); err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *A2AAgentService) GetByID(ctx context.Context, id uuid.UUID) (*models.A2AAgent, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *A2AAgentService) List(ctx context.Context, orgID uuid.UUID, agentType string, page, perPage int) ([]models.A2AAgent, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	return s.repo.List(ctx, orgID, agentType, page, perPage)
}

func (s *A2AAgentService) Update(ctx context.Context, id uuid.UUID, req models.CreateA2AAgentRequest) (*models.A2AAgent, error) {
	agent, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	agent.Name = req.Name
	agent.Description = req.Description
	agent.AgentType = req.AgentType
	agent.EndpointURL = req.EndpointURL
	if req.AgentCard != nil {
		agent.AgentCard = req.AgentCard
	}
	if req.Skills != nil {
		agent.Skills = req.Skills
	}
	if req.AllowedSoftwareIDs != nil {
		agent.AllowedSoftwareIDs = req.AllowedSoftwareIDs
	}
	if req.ManagedConfig != nil {
		agent.ManagedConfig = req.ManagedConfig
	}
	agent.AuthType = req.AuthType
	agent.AuthCredentials = req.AuthCredentials
	agent.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, agent); err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *A2AAgentService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *A2AAgentService) HealthCheck(ctx context.Context, id uuid.UUID, status string) error {
	return s.repo.HealthCheck(ctx, id, status)
}

func (s *A2AAgentService) GetBySkill(ctx context.Context, orgID uuid.UUID, skill string) ([]models.A2AAgent, error) {
	return s.repo.GetBySkill(ctx, orgID, skill)
}

// --- A2ATaskService ---

type A2ATaskService struct {
	repo A2ATaskRepository
}

func NewA2ATaskService(repo A2ATaskRepository) *A2ATaskService {
	return &A2ATaskService{repo: repo}
}

func (s *A2ATaskService) Create(ctx context.Context, incidentID uuid.UUID, req models.CreateA2ATaskRequest) (*models.A2ATask, error) {
	now := time.Now()
	task := &models.A2ATask{
		ID:              uuid.New(),
		IncidentID:      incidentID,
		AgentID:         req.AgentID,
		TaskType:        req.TaskType,
		Status:          "submitted",
		InputMessage:    req.InputMessage,
		OutputArtifacts: json.RawMessage("[]"),
		Priority:        req.Priority,
		DependsOn:       req.DependsOn,
		SubmittedAt:     &now,
		CreatedAt:       now,
	}
	if task.InputMessage == nil {
		task.InputMessage = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *A2ATaskService) GetByID(ctx context.Context, id uuid.UUID) (*models.A2ATask, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *A2ATaskService) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.A2ATask, error) {
	return s.repo.ListByIncident(ctx, incidentID)
}

func (s *A2ATaskService) Update(ctx context.Context, id uuid.UUID, req models.UpdateA2ATaskRequest) (*models.A2ATask, error) {
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status != nil {
		task.Status = *req.Status
		if *req.Status == "working" {
			now := time.Now()
			task.StartedAt = &now
		}
		if *req.Status == "completed" || *req.Status == "failed" {
			now := time.Now()
			task.CompletedAt = &now
		}
	}
	if req.OutputArtifacts != nil {
		task.OutputArtifacts = req.OutputArtifacts
	}
	if req.ErrorMessage != nil {
		task.ErrorMessage = *req.ErrorMessage
	}
	if err := s.repo.Update(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *A2ATaskService) ListByAgent(ctx context.Context, agentID uuid.UUID) ([]models.A2ATask, error) {
	return s.repo.ListByAgent(ctx, agentID)
}

// --- OrchestratorDecisionService ---

type OrchestratorDecisionService struct {
	repo OrchestratorDecisionRepository
}

func NewOrchestratorDecisionService(repo OrchestratorDecisionRepository) *OrchestratorDecisionService {
	return &OrchestratorDecisionService{repo: repo}
}

func (s *OrchestratorDecisionService) Create(ctx context.Context, incidentID uuid.UUID, req models.CreateOrchestratorDecisionRequest) (*models.OrchestratorDecision, error) {
	decision := &models.OrchestratorDecision{
		ID:             uuid.New(),
		IncidentID:     incidentID,
		DecisionType:   req.DecisionType,
		Reasoning:      req.Reasoning,
		SelectedAgents: req.SelectedAgents,
		ContextUsed:    req.ContextUsed,
		Confidence:     req.Confidence,
		CreatedAt:      time.Now(),
	}
	if decision.SelectedAgents == nil {
		decision.SelectedAgents = json.RawMessage("[]")
	}
	if decision.ContextUsed == nil {
		decision.ContextUsed = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, decision); err != nil {
		return nil, err
	}
	return decision, nil
}

func (s *OrchestratorDecisionService) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.OrchestratorDecision, error) {
	return s.repo.ListByIncident(ctx, incidentID)
}
