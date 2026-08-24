package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Repository Interfaces ---

type ObservabilitySourceRepository interface {
	Create(ctx context.Context, s *models.ObservabilitySource) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.ObservabilitySource, error)
	List(ctx context.Context, orgID uuid.UUID, sourceType string) ([]models.ObservabilitySource, error)
	ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.ObservabilitySource, error)
	Update(ctx context.Context, s *models.ObservabilitySource) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateHealthStatus(ctx context.Context, id uuid.UUID, status string) error
}

type SnapshotConfigRepository interface {
	Create(ctx context.Context, sc *models.SnapshotConfig) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.SnapshotConfig, error)
	ListBySource(ctx context.Context, sourceID uuid.UUID) ([]models.SnapshotConfig, error)
	ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.SnapshotConfig, error)
	Update(ctx context.Context, sc *models.SnapshotConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// --- ObservabilitySourceService ---

type ObservabilitySourceService struct {
	repo ObservabilitySourceRepository
}

func NewObservabilitySourceService(repo ObservabilitySourceRepository) *ObservabilitySourceService {
	return &ObservabilitySourceService{repo: repo}
}

func (s *ObservabilitySourceService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateObservabilitySourceRequest) (*models.ObservabilitySource, error) {
	now := time.Now()
	verifySSL := true
	if req.VerifySSL != nil {
		verifySSL = *req.VerifySSL
	}
	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	src := &models.ObservabilitySource{
		ID:                   uuid.New(),
		OrgID:                orgID,
		Name:                 req.Name,
		SourceType:           req.SourceType,
		BaseURL:              req.BaseURL,
		AuthType:             req.AuthType,
		AuthConfig:           req.AuthConfig,
		Capabilities:         req.Capabilities,
		MonitoredSoftwareIDs: req.MonitoredSoftwareIDs,
		TimeoutSeconds:       timeoutSeconds,
		VerifySSL:            verifySSL,
		CustomHeaders:        req.CustomHeaders,
		Enabled:              true,
		HealthStatus:         "unknown",
		Description:          req.Description,
		Environment:          req.Environment,
		Region:               req.Region,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if src.AuthConfig == nil {
		src.AuthConfig = json.RawMessage("{}")
	}
	if src.Capabilities == nil {
		src.Capabilities = json.RawMessage("[]")
	}
	if src.MonitoredSoftwareIDs == nil {
		src.MonitoredSoftwareIDs = json.RawMessage("[]")
	}
	if src.CustomHeaders == nil {
		src.CustomHeaders = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, src); err != nil {
		return nil, err
	}
	return src, nil
}

func (s *ObservabilitySourceService) GetByID(ctx context.Context, id uuid.UUID) (*models.ObservabilitySource, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ObservabilitySourceService) List(ctx context.Context, orgID uuid.UUID, sourceType string) ([]models.ObservabilitySource, error) {
	return s.repo.List(ctx, orgID, sourceType)
}

func (s *ObservabilitySourceService) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.ObservabilitySource, error) {
	return s.repo.ListBySoftware(ctx, softwareID)
}

func (s *ObservabilitySourceService) Update(ctx context.Context, id uuid.UUID, req models.CreateObservabilitySourceRequest) (*models.ObservabilitySource, error) {
	src, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	src.Name = req.Name
	src.SourceType = req.SourceType
	src.BaseURL = req.BaseURL
	src.AuthType = req.AuthType
	src.Description = req.Description
	src.Environment = req.Environment
	src.Region = req.Region
	src.UpdatedAt = time.Now()
	if req.AuthConfig != nil {
		src.AuthConfig = req.AuthConfig
	}
	if req.Capabilities != nil {
		src.Capabilities = req.Capabilities
	}
	if req.MonitoredSoftwareIDs != nil {
		src.MonitoredSoftwareIDs = req.MonitoredSoftwareIDs
	}
	if req.CustomHeaders != nil {
		src.CustomHeaders = req.CustomHeaders
	}
	if req.TimeoutSeconds > 0 {
		src.TimeoutSeconds = req.TimeoutSeconds
	}
	if req.VerifySSL != nil {
		src.VerifySSL = *req.VerifySSL
	}
	if err := s.repo.Update(ctx, src); err != nil {
		return nil, err
	}
	return src, nil
}

func (s *ObservabilitySourceService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *ObservabilitySourceService) CheckHealth(ctx context.Context, id uuid.UUID) (*models.ObservabilitySource, error) {
	// Stub: mark as healthy. Real implementation would probe the source.
	if err := s.repo.UpdateHealthStatus(ctx, id, "healthy"); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// --- SnapshotConfigService ---

type SnapshotConfigService struct {
	repo SnapshotConfigRepository
}

func NewSnapshotConfigService(repo SnapshotConfigRepository) *SnapshotConfigService {
	return &SnapshotConfigService{repo: repo}
}

func (s *SnapshotConfigService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateSnapshotConfigRequest) (*models.SnapshotConfig, error) {
	now := time.Now()
	timeRange := req.TimeRangeSeconds
	if timeRange <= 0 {
		timeRange = 3600
	}
	sc := &models.SnapshotConfig{
		ID:               uuid.New(),
		OrgID:            orgID,
		SourceID:         req.SourceID,
		SoftwareID:       req.SoftwareID,
		Name:             req.Name,
		SnapshotType:     req.SnapshotType,
		QueryTemplate:    req.QueryTemplate,
		TimeRangeSeconds: timeRange,
		Parameters:       req.Parameters,
		Enabled:          true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if sc.Parameters == nil {
		sc.Parameters = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, sc); err != nil {
		return nil, err
	}
	return sc, nil
}

func (s *SnapshotConfigService) GetByID(ctx context.Context, id uuid.UUID) (*models.SnapshotConfig, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SnapshotConfigService) ListBySource(ctx context.Context, sourceID uuid.UUID) ([]models.SnapshotConfig, error) {
	return s.repo.ListBySource(ctx, sourceID)
}

func (s *SnapshotConfigService) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.SnapshotConfig, error) {
	return s.repo.ListBySoftware(ctx, softwareID)
}

func (s *SnapshotConfigService) Update(ctx context.Context, id uuid.UUID, req models.CreateSnapshotConfigRequest) (*models.SnapshotConfig, error) {
	sc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	sc.SourceID = req.SourceID
	sc.SoftwareID = req.SoftwareID
	sc.Name = req.Name
	sc.SnapshotType = req.SnapshotType
	sc.QueryTemplate = req.QueryTemplate
	sc.UpdatedAt = time.Now()
	if req.TimeRangeSeconds > 0 {
		sc.TimeRangeSeconds = req.TimeRangeSeconds
	}
	if req.Parameters != nil {
		sc.Parameters = req.Parameters
	}
	if err := s.repo.Update(ctx, sc); err != nil {
		return nil, err
	}
	return sc, nil
}

func (s *SnapshotConfigService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
