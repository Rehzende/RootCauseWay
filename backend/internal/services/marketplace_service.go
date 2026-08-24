package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// Repository interfaces for marketplace

type MarketplaceAgentRepository interface {
	Create(ctx context.Context, a *models.MarketplaceAgent) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.MarketplaceAgent, error)
	GetBySlug(ctx context.Context, slug string) (*models.MarketplaceAgent, error)
	List(ctx context.Context, category, search string) ([]models.MarketplaceAgent, error)
	Update(ctx context.Context, a *models.MarketplaceAgent) error
	IncrementDownloads(ctx context.Context, id uuid.UUID) error
}

type InstalledAgentRepository interface {
	Install(ctx context.Context, ia *models.InstalledAgent) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.InstalledAgent, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]models.InstalledAgent, error)
	Update(ctx context.Context, ia *models.InstalledAgent) error
	Uninstall(ctx context.Context, id uuid.UUID) error
}

// --- MarketplaceService ---

type MarketplaceService struct {
	marketplaceRepo MarketplaceAgentRepository
	installedRepo   InstalledAgentRepository
	a2aRepo         A2AAgentRepository
}

func NewMarketplaceService(marketplaceRepo MarketplaceAgentRepository, installedRepo InstalledAgentRepository, a2aRepo A2AAgentRepository) *MarketplaceService {
	return &MarketplaceService{
		marketplaceRepo: marketplaceRepo,
		installedRepo:   installedRepo,
		a2aRepo:         a2aRepo,
	}
}

func (s *MarketplaceService) Browse(ctx context.Context, category, search string) ([]models.MarketplaceAgent, error) {
	return s.marketplaceRepo.List(ctx, category, search)
}

func (s *MarketplaceService) GetDetails(ctx context.Context, slug string) (*models.MarketplaceAgent, error) {
	return s.marketplaceRepo.GetBySlug(ctx, slug)
}

func (s *MarketplaceService) Install(ctx context.Context, orgID uuid.UUID, slug string, req models.InstallAgentRequest) (*models.InstalledAgent, error) {
	mAgent, err := s.marketplaceRepo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("marketplace agent not found: %w", err)
	}

	// Determine hosting type from install config
	hostingType := "managed"
	llmProvider := "platform"
	endpointURL := ""
	llmAPIKeyRef := ""
	var managedConfig json.RawMessage

	// Check if config specifies BYOA
	if req.Config != nil {
		var cfgMap map[string]interface{}
		if json.Unmarshal(req.Config, &cfgMap) == nil {
			if ht, ok := cfgMap["hosting_type"].(string); ok && ht == "byoa" {
				hostingType = "byoa"
				llmProvider = "custom"
				if ep, ok := cfgMap["endpoint_url"].(string); ok {
					endpointURL = ep
				}
				if ref, ok := cfgMap["llm_api_key_ref"].(string); ok {
					llmAPIKeyRef = ref
				}
			}
			if lp, ok := cfgMap["llm_provider"].(string); ok {
				llmProvider = lp
			}
		}
	}

	if hostingType == "managed" {
		managedConfig, _ = json.Marshal(map[string]interface{}{
			"docker_image": mAgent.DockerImage,
		})
	}

	// Create a2a_agent record from marketplace agent card
	now := time.Now()
	a2aAgent := &models.A2AAgent{
		ID:              uuid.New(),
		OrgID:           orgID,
		Name:            mAgent.Name,
		Description:     mAgent.Description,
		AgentType:       "marketplace",
		EndpointURL:     endpointURL,
		AgentCard:       mAgent.AgentCard,
		Skills:          mAgent.Skills,
		Enabled:         true,
		HostingType:     hostingType,
		ManagedConfig:   managedConfig,
		LLMProvider:     llmProvider,
		LLMAPIKeyRef:    llmAPIKeyRef,
		AutoScale:       false,
		MinReplicas:     1,
		MaxReplicas:     3,
		HealthStatus:    "unknown",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.a2aRepo.Create(ctx, a2aAgent); err != nil {
		return nil, fmt.Errorf("failed to create a2a agent: %w", err)
	}

	config := req.Config
	if config == nil {
		config = json.RawMessage("{}")
	}

	ia := &models.InstalledAgent{
		ID:                 uuid.New(),
		OrgID:              orgID,
		MarketplaceAgentID: mAgent.ID,
		A2AAgentID:         &a2aAgent.ID,
		Config:             config,
		Version:            mAgent.Version,
		Status:             "installed",
		InstalledAt:        now,
		UpdatedAt:          now,
	}

	if err := s.installedRepo.Install(ctx, ia); err != nil {
		return nil, fmt.Errorf("failed to install agent: %w", err)
	}

	// Increment download count
	_ = s.marketplaceRepo.IncrementDownloads(ctx, mAgent.ID)

	return ia, nil
}

func (s *MarketplaceService) Uninstall(ctx context.Context, id uuid.UUID) error {
	return s.installedRepo.Uninstall(ctx, id)
}

func (s *MarketplaceService) ListInstalled(ctx context.Context, orgID uuid.UUID) ([]models.InstalledAgent, error) {
	return s.installedRepo.ListByOrg(ctx, orgID)
}
