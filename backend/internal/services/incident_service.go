package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/embeddings"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// IncidentRepository defines the DB operations for incidents
type IncidentRepository interface {
	Create(ctx context.Context, incident *models.Incident) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error)
	List(ctx context.Context, orgID uuid.UUID, status, severity string, softwareID *uuid.UUID, from *time.Time, page, perPage int) ([]models.Incident, int, error)
	Update(ctx context.Context, incident *models.Incident) error
	AddEvent(ctx context.Context, event *models.IncidentEvent) error
	AddEvidence(ctx context.Context, evidence *models.IncidentEvidence) error
	GetEvents(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentEvent, error)
	GetEvidence(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentEvidence, error)
}

// AlertSnapshotRepository defines the DB operations for alert snapshots
type AlertSnapshotRepository interface {
	Create(ctx context.Context, snapshot *models.AlertSnapshot) error
}

type IncidentService struct {
	repo         IncidentRepository
	snapshotRepo AlertSnapshotRepository
	embedder     embeddings.Embedder      // nil when embeddings are disabled
	vectorRepo   IncidentVectorRepository // nil when vector storage is unavailable
}

func NewIncidentService(repo IncidentRepository, snapshotRepo AlertSnapshotRepository) *IncidentService {
	return &IncidentService{repo: repo, snapshotRepo: snapshotRepo}
}

// SetEmbedder enables best-effort incident embedding on creation. Either
// argument being nil disables embedding (graceful fallback).
func (s *IncidentService) SetEmbedder(e embeddings.Embedder, vr IncidentVectorRepository) {
	s.embedder = e
	s.vectorRepo = vr
}

func (s *IncidentService) Create(ctx context.Context, incident *models.Incident) error {
	incident.ID = uuid.New()
	incident.Status = "open"
	incident.CreatedAt = time.Now()
	incident.UpdatedAt = time.Now()
	if err := s.repo.Create(ctx, incident); err != nil {
		return err
	}
	s.EmbedIncident(ctx, incident.ID, incident.Title, incident.Description)
	return nil
}

// EmbedIncident computes and stores the incident embedding from title +
// description. Best-effort: failures are logged and never propagate — this is
// the seam other write paths (e.g. RCA completion) can call to (re)embed.
func (s *IncidentService) EmbedIncident(ctx context.Context, id uuid.UUID, title, description string) {
	if s.embedder == nil || s.vectorRepo == nil {
		return
	}
	text := strings.TrimSpace(title + "\n" + description)
	if text == "" {
		return
	}
	vec, err := s.embedder.Embed(ctx, text)
	if err != nil {
		log.Printf("WARN: incident %s: embedding computation failed: %v", id, err)
		return
	}
	if err := s.vectorRepo.UpdateEmbedding(ctx, id, vec); err != nil {
		log.Printf("WARN: incident %s: embedding store failed: %v", id, err)
	}
}

func (s *IncidentService) GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error) {
	incident, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	events, _ := s.repo.GetEvents(ctx, id)
	evidence, _ := s.repo.GetEvidence(ctx, id)
	incident.Timeline = events
	incident.Evidence = evidence
	return incident, nil
}

func (s *IncidentService) List(ctx context.Context, orgID uuid.UUID, status, severity string, softwareID *uuid.UUID, from *time.Time, page, perPage int) ([]models.Incident, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 20
	}
	return s.repo.List(ctx, orgID, status, severity, softwareID, from, page, perPage)
}

// terminalStatuses are the incident statuses that mark an incident as done
// for postmortem-triggering purposes. "closed" is included alongside
// "resolved" because the incident detail UI lets an operator pick either
// one directly from a flat status list -- there's no enforced progression
// through "resolved" first, so an operator who picks "closed" still expects
// the same finalization (resolved_at stamped, postmortem generation kicked
// off) that picking "resolved" would have given them.
var terminalStatuses = map[string]bool{"resolved": true, "closed": true}

// Update applies a partial update to an incident. The second return value
// reports whether this specific call is what moved the incident into a
// terminal status for the first time (i.e. resolved_at was previously nil
// and just got set) -- callers use it to decide whether to fire the
// incident.resolved event exactly once, even if an incident later goes
// resolved -> closed.
func (s *IncidentService) Update(ctx context.Context, id uuid.UUID, req models.UpdateIncidentRequest) (*models.Incident, bool, error) {
	incident, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, false, err
	}

	wasAlreadyTerminal := incident.ResolvedAt != nil

	if req.Status != nil {
		incident.Status = *req.Status
	}
	if req.Severity != nil {
		incident.Severity = *req.Severity
	}
	if req.AssigneeID != nil {
		if *req.AssigneeID == "" {
			incident.AssigneeID = nil // explicit unassign, see UpdateIncidentRequest's doc comment
		} else {
			parsed, err := uuid.Parse(*req.AssigneeID)
			if err != nil {
				return nil, false, fmt.Errorf("invalid assignee_id: %w", err)
			}
			incident.AssigneeID = &parsed
		}
	}
	if req.RootCause != nil {
		incident.RootCause = *req.RootCause
	}
	if req.Mitigation != nil {
		incident.Mitigation = *req.Mitigation
	}

	justTerminalized := false
	if req.Status != nil && terminalStatuses[*req.Status] && !wasAlreadyTerminal {
		now := time.Now()
		incident.ResolvedAt = &now
		justTerminalized = true
	}

	incident.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, incident); err != nil {
		return nil, false, err
	}
	return incident, justTerminalized, nil
}

func (s *IncidentService) AddEvent(ctx context.Context, incidentID uuid.UUID, actor string, req models.CreateEventRequest) (*models.IncidentEvent, error) {
	event := &models.IncidentEvent{
		ID:         uuid.New(),
		IncidentID: incidentID,
		Type:       req.Type,
		Actor:      actor,
		Data:       req.Data,
		CreatedAt:  time.Now(),
	}
	if event.Data == nil {
		event.Data = json.RawMessage("{}")
	}
	if err := s.repo.AddEvent(ctx, event); err != nil {
		return nil, err
	}
	return event, nil
}

func (s *IncidentService) AddEvidence(ctx context.Context, incidentID uuid.UUID, req models.CreateEvidenceRequest) (*models.IncidentEvidence, error) {
	evidence := &models.IncidentEvidence{
		ID:          uuid.New(),
		IncidentID:  incidentID,
		Type:        req.Type,
		Title:       req.Title,
		Content:     req.Content,
		Source:      req.Source,
		CollectedAt: time.Now(),
	}
	if err := s.repo.AddEvidence(ctx, evidence); err != nil {
		return nil, err
	}
	return evidence, nil
}

func (s *IncidentService) ListEvidence(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentEvidence, error) {
	return s.repo.GetEvidence(ctx, incidentID)
}

func (s *IncidentService) CreateAlertSnapshot(ctx context.Context, snapshot *models.AlertSnapshot) error {
	snapshot.ID = uuid.New()
	snapshot.CreatedAt = time.Now()
	return s.snapshotRepo.Create(ctx, snapshot)
}
