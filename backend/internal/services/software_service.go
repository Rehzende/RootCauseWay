package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/google/uuid"
)

// ValidCriticalities / ValidSoftwareTypes are the allowed values for
// SoftwareEntry.Criticality / .Type -- enforced here (not just a DB CHECK
// constraint) so a bad value 400s with a clear message instead of a raw
// Postgres constraint-violation error.
var (
	ValidCriticalities = []string{"critical", "high", "medium", "low"}
	ValidSoftwareTypes = []string{"service", "library", "database", "job", "website", "other"}
)

func isValidEnum(value string, allowed []string) bool {
	for _, v := range allowed {
		if value == v {
			return true
		}
	}
	return false
}

// ValidateSoftwareRequest checks the enum-valued fields of a
// CreateSoftwareRequest (used for both create and update), returning a
// caller-facing error naming the bad field and its allowed values. Empty
// Criticality/Type are allowed here -- Create/Update fill in the defaults --
// only an explicitly-set-but-wrong value is rejected.
func ValidateSoftwareRequest(req models.CreateSoftwareRequest) error {
	if req.Criticality != "" && !isValidEnum(req.Criticality, ValidCriticalities) {
		return fmt.Errorf("invalid criticality %q: must be one of %v", req.Criticality, ValidCriticalities)
	}
	if req.Type != "" && !isValidEnum(req.Type, ValidSoftwareTypes) {
		return fmt.Errorf("invalid type %q: must be one of %v", req.Type, ValidSoftwareTypes)
	}
	for _, d := range parseDependencies(req.Dependencies) {
		if d.Relation != "" && !isValidEnum(d.Relation, ValidDependencyRelations) {
			return fmt.Errorf("invalid dependency relation %q for %q: must be one of %v", d.Relation, d.Slug, ValidDependencyRelations)
		}
	}
	return nil
}

// DependencyRelation values describe *why* one software entry depends on
// another -- a hard runtime dependency vs. something looser -- so the
// correlation engine and RCA context have more to work with than "these are
// somehow related".
const (
	DependencyRelationDependsOn          = "depends_on"
	DependencyRelationUsesAPIOf          = "uses_api_of"
	DependencyRelationSharesDatabaseWith = "shares_database_with"
)

// ValidDependencyRelations is exported for handler-side validation.
var ValidDependencyRelations = []string{
	DependencyRelationDependsOn, DependencyRelationUsesAPIOf, DependencyRelationSharesDatabaseWith,
}

// SoftwareDependency is one entry in a SoftwareEntry.Dependencies array.
// UnmarshalJSON accepts both the current shape ({"slug": "...", "relation":
// "..."}) and the original shape (a bare slug string) so data written before
// this migration -- or a row migration 031's backfill somehow missed --
// still parses instead of silently dropping the dependency.
type SoftwareDependency struct {
	Slug     string `json:"slug"`
	Relation string `json:"relation,omitempty"`
}

func (d *SoftwareDependency) UnmarshalJSON(data []byte) error {
	type alias SoftwareDependency
	var obj alias
	if err := json.Unmarshal(data, &obj); err == nil && obj.Slug != "" {
		*d = SoftwareDependency(obj)
		if d.Relation == "" {
			d.Relation = DependencyRelationDependsOn
		}
		return nil
	}
	var slug string
	if err := json.Unmarshal(data, &slug); err != nil {
		return fmt.Errorf("dependency entry is neither an object with a slug nor a string: %w", err)
	}
	*d = SoftwareDependency{Slug: slug, Relation: DependencyRelationDependsOn}
	return nil
}

// parseDependencies best-effort parses a SoftwareEntry.Dependencies
// json.RawMessage into typed entries, skipping any element that fails to
// parse rather than failing the whole graph lookup over one bad entry.
func parseDependencies(raw json.RawMessage) []SoftwareDependency {
	if len(raw) == 0 {
		return nil
	}
	var rawElems []json.RawMessage
	if err := json.Unmarshal(raw, &rawElems); err != nil {
		return nil
	}
	deps := make([]SoftwareDependency, 0, len(rawElems))
	for _, elem := range rawElems {
		var d SoftwareDependency
		if err := json.Unmarshal(elem, &d); err != nil || d.Slug == "" {
			continue
		}
		deps = append(deps, d)
	}
	return deps
}

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
	// Relation is the dependency type from the *depending* side's
	// perspective, e.g. an upstream entry with Relation "uses_api_of" means
	// "this entry uses the API of" that upstream service; a downstream
	// entry's Relation describes how IT depends on the entry being queried.
	Relation string `json:"relation"`
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
		Criticality:    req.Criticality,
		Type:           req.Type,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if entry.Tags == nil {
		entry.Tags = []string{}
	}
	if entry.Criticality == "" {
		entry.Criticality = "medium"
	}
	if entry.Type == "" {
		entry.Type = "service"
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
	// OwnerID only overwritten when the request actually carries one -- the
	// catalog UI's edit form has no owner picker yet and never sends this
	// field at all, so unconditionally assigning req.OwnerID (nil whenever
	// it's absent from the JSON body) silently wiped any owner set some
	// other way (e.g. direct API call) on every single edit.
	if req.OwnerID != nil {
		entry.OwnerID = req.OwnerID
	}
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
	if req.Criticality != "" {
		entry.Criticality = req.Criticality
	}
	if req.Type != "" {
		entry.Type = req.Type
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
// software catalog entry. Dependencies not resolving to a known catalog
// entry (e.g. external systems) are skipped rather than treated as an error.
func (s *SoftwareService) GetDependencyGraph(ctx context.Context, id uuid.UUID) (*DependencyGraph, error) {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	upstreamDeps := parseDependencies(entry.Dependencies)

	upstream := make([]RelatedService, 0, len(upstreamDeps))
	for _, d := range upstreamDeps {
		dep, err := s.repo.FindBySlugOrTag(ctx, entry.OrgID, d.Slug)
		if err != nil || dep == nil {
			continue
		}
		upstream = append(upstream, RelatedService{SoftwareID: dep.ID, Slug: dep.Slug, Name: dep.Name, Relation: d.Relation})
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
		// Find the specific relation *this dependent* declared toward
		// `entry`, rather than just "some dependency exists" -- e.g.
		// checkout-service might declare "uses_api_of" toward this entry
		// while some other dependent declares "depends_on".
		relation := DependencyRelationDependsOn
		for _, dd := range parseDependencies(dep.Dependencies) {
			if dd.Slug == entry.Slug {
				relation = dd.Relation
				break
			}
		}
		downstream = append(downstream, RelatedService{SoftwareID: dep.ID, Slug: dep.Slug, Name: dep.Name, Relation: relation})
	}

	return &DependencyGraph{
		SoftwareID: entry.ID,
		Slug:       entry.Slug,
		Upstream:   upstream,
		Downstream: downstream,
	}, nil
}
