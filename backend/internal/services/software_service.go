package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// SoftwareRepository defines the DB operations for software catalog
type SoftwareRepository interface {
	Create(ctx context.Context, entry *models.SoftwareEntry) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.SoftwareEntry, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.SoftwareEntry, int, error)
	Update(ctx context.Context, entry *models.SoftwareEntry) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindBySlugOrTag(ctx context.Context, orgID uuid.UUID, label string) (*models.SoftwareEntry, error)
	// ListDependents returns software entries whose declared `dependencies` array
	// contains the given slug (i.e. services downstream of/dependent on it).
	ListDependents(ctx context.Context, orgID uuid.UUID, slug string) ([]models.SoftwareEntry, error)
}

// RelatedService is a lightweight reference to a software catalog entry related
// to another one via the dependency graph (upstream dependency or downstream
// dependent).
type RelatedService struct {
	SoftwareID uuid.UUID `json:"software_id"`
	Slug       string    `json:"slug"`
	Name       string    `json:"name"`
}

// DependencyGraph describes the services a software entry depends on (upstream)
// and the services that depend on it (downstream). Used by the correlation
// engine to cascade-correlate alerts across related services (e.g. a database
// outage that trips alerts on every dependent service).
type DependencyGraph struct {
	SoftwareID uuid.UUID        `json:"software_id"`
	Slug       string           `json:"slug"`
	Upstream   []RelatedService `json:"upstream"`
	Downstream []RelatedService `json:"downstream"`
}

type SoftwareService struct {
	repo SoftwareRepository
}

func NewSoftwareService(repo SoftwareRepository) *SoftwareService {
	return &SoftwareService{repo: repo}
}

func (s *SoftwareService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateSoftwareRequest) (*models.SoftwareEntry, error) {
	entry := &models.SoftwareEntry{
		ID:             uuid.New(),
		OrgID:          orgID,
		Name:           req.Name,
		Slug:           req.Slug,
		Description:    req.Description,
		OwnerID:        req.OwnerID,
		RepositoryURL:  req.RepositoryURL,
		Tags:           req.Tags,
		Status:         "active",
		PipelineURL:    req.PipelineURL,
		CloudProvider:  req.CloudProvider,
		CloudResources: req.CloudResources,
		DatabaseInfo:   req.DatabaseInfo,
		InfraDetails:   req.InfraDetails,
		Stakeholders:   req.Stakeholders,
		SreTeam:        req.SreTeam,
		Architects:     req.Architects,
		RunbookURL:     req.RunbookURL,
		DashboardURL:   req.DashboardURL,
		Dependencies:   req.Dependencies,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if entry.Tags == nil {
		entry.Tags = []string{}
	}
	for _, field := range []*json.RawMessage{&entry.CloudResources, &entry.DatabaseInfo, &entry.InfraDetails} {
		if *field == nil {
			*field = json.RawMessage("{}")
		}
	}
	for _, field := range []*json.RawMessage{&entry.Stakeholders, &entry.SreTeam, &entry.Architects, &entry.Dependencies} {
		if *field == nil {
			*field = json.RawMessage("[]")
		}
	}
	if err := s.repo.Create(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *SoftwareService) GetByID(ctx context.Context, id uuid.UUID) (*models.SoftwareEntry, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SoftwareService) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.SoftwareEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	return s.repo.List(ctx, orgID, page, perPage)
}

func (s *SoftwareService) Update(ctx context.Context, id uuid.UUID, req models.CreateSoftwareRequest) (*models.SoftwareEntry, error) {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	entry.Name = req.Name
	entry.Slug = req.Slug
	entry.Description = req.Description
	entry.OwnerID = req.OwnerID
	entry.RepositoryURL = req.RepositoryURL
	entry.Tags = req.Tags
	entry.PipelineURL = req.PipelineURL
	entry.CloudProvider = req.CloudProvider
	if req.CloudResources != nil {
		entry.CloudResources = req.CloudResources
	}
	if req.DatabaseInfo != nil {
		entry.DatabaseInfo = req.DatabaseInfo
	}
	if req.InfraDetails != nil {
		entry.InfraDetails = req.InfraDetails
	}
	if req.Stakeholders != nil {
		entry.Stakeholders = req.Stakeholders
	}
	if req.SreTeam != nil {
		entry.SreTeam = req.SreTeam
	}
	if req.Architects != nil {
		entry.Architects = req.Architects
	}
	entry.RunbookURL = req.RunbookURL
	entry.DashboardURL = req.DashboardURL
	if req.Dependencies != nil {
		entry.Dependencies = req.Dependencies
	}
	entry.UpdatedAt = time.Now()
	if entry.Tags == nil {
		entry.Tags = []string{}
	}
	if err := s.repo.Update(ctx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *SoftwareService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// GetDependencyGraph resolves the upstream (declared `dependencies`) and
// downstream (services whose `dependencies` list this one) relations for a
// software catalog entry. Dependencies are stored as a JSON array of slugs;
// slugs that don't resolve to a known catalog entry (e.g. external systems)
// are skipped rather than treated as an error.
func (s *SoftwareService) GetDependencyGraph(ctx context.Context, id uuid.UUID) (*DependencyGraph, error) {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var upstreamSlugs []string
	if len(entry.Dependencies) > 0 {
		_ = json.Unmarshal(entry.Dependencies, &upstreamSlugs)
	}

	upstream := make([]RelatedService, 0, len(upstreamSlugs))
	for _, slug := range upstreamSlugs {
		if slug == "" {
			continue
		}
		dep, err := s.repo.FindBySlugOrTag(ctx, entry.OrgID, slug)
		if err != nil || dep == nil {
			continue
		}
		upstream = append(upstream, RelatedService{SoftwareID: dep.ID, Slug: dep.Slug, Name: dep.Name})
	}

	dependents, err := s.repo.ListDependents(ctx, entry.OrgID, entry.Slug)
	if err != nil {
		return nil, err
	}
	downstream := make([]RelatedService, 0, len(dependents))
	for _, dep := range dependents {
		if dep.ID == entry.ID {
			continue
		}
		downstream = append(downstream, RelatedService{SoftwareID: dep.ID, Slug: dep.Slug, Name: dep.Name})
	}

	return &DependencyGraph{
		SoftwareID: entry.ID,
		Slug:       entry.Slug,
		Upstream:   upstream,
		Downstream: downstream,
	}, nil
}
