package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// Repository interfaces for cockpit services

type AgentRunRepository interface {
	Create(ctx context.Context, a *models.AgentRun) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.AgentRun, error)
	ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.AgentRun, error)
	Update(ctx context.Context, a *models.AgentRun) error
	GetDAG(ctx context.Context, incidentID uuid.UUID) ([]models.AgentRun, error)
}

type RCIRepository interface {
	Create(ctx context.Context, rci *models.IncidentRCI) error
	GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentRCI, error)
	Update(ctx context.Context, rci *models.IncidentRCI) error
}

type RCARepository interface {
	Create(ctx context.Context, rca *models.IncidentRCA) error
	GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentRCA, error)
	Update(ctx context.Context, rca *models.IncidentRCA) error
}

// IncidentRootCauseUpdater is the narrow slice of IncidentRepository that
// RCAService needs to backfill an incident's summary root_cause field once
// an RCA is produced. Kept minimal so RCAService doesn't need the full
// incident repository/service surface.
type IncidentRootCauseUpdater interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error)
	Update(ctx context.Context, i *models.Incident) error
}

type PostmortemRepository interface {
	Create(ctx context.Context, pm *models.IncidentPostmortem) error
	GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentPostmortem, error)
	Update(ctx context.Context, pm *models.IncidentPostmortem) error
}

// --- AgentRunService ---

type AgentRunService struct {
	repo AgentRunRepository
}

func NewAgentRunService(repo AgentRunRepository) *AgentRunService {
	return &AgentRunService{repo: repo}
}

func (s *AgentRunService) Create(ctx context.Context, incidentID uuid.UUID, req models.CreateAgentRunRequest) (*models.AgentRun, error) {
	now := time.Now()
	run := &models.AgentRun{
		ID:          uuid.New(),
		IncidentID:  incidentID,
		AgentID:     req.AgentID,
		AgentName:   req.AgentName,
		AgentType:   req.AgentType,
		Status:      "pending",
		ParentRunID: req.ParentRunID,
		InputData:   req.InputData,
		OutputData:  json.RawMessage("{}"),
		ModelUsed:   req.ModelUsed,
		// StartedAt used to only get set by Update() on a transition to
		// "running" -- but the orchestrator's real dispatch loop never sends
		// that transition, it goes straight from this "pending" create to a
		// "completed"/"failed" Update. StartedAt stayed nil forever, so the
		// frontend's RunsTimeline (new Date(run.started_at)) rendered
		// "Invalid Date" for every single stage. A run's tracking row is
		// created right as the orchestrator is about to dispatch it, so
		// "created" and "started" are the same moment in practice here.
		StartedAt: &now,
		CreatedAt: now,
	}
	if run.InputData == nil {
		run.InputData = json.RawMessage("{}")
	}
	if err := s.repo.Create(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *AgentRunService) GetByID(ctx context.Context, id uuid.UUID) (*models.AgentRun, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AgentRunService) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.AgentRun, error) {
	return s.repo.ListByIncident(ctx, incidentID)
}

func (s *AgentRunService) Update(ctx context.Context, id uuid.UUID, req models.UpdateAgentRunRequest) (*models.AgentRun, error) {
	run, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status != nil {
		run.Status = *req.Status
		if *req.Status == "running" {
			now := time.Now()
			run.StartedAt = &now
		}
		if *req.Status == "completed" || *req.Status == "failed" {
			now := time.Now()
			run.CompletedAt = &now
		}
	}
	if req.OutputData != nil {
		run.OutputData = req.OutputData
	}
	if req.ErrorMessage != nil {
		run.ErrorMessage = *req.ErrorMessage
	}
	if req.TokensUsed != nil {
		run.TokensUsed = *req.TokensUsed
	}
	if req.ModelUsed != nil {
		run.ModelUsed = *req.ModelUsed
	}
	if req.DurationMs != nil {
		run.DurationMs = *req.DurationMs
	}
	if req.CompletedAt != nil {
		run.CompletedAt = req.CompletedAt
	}
	if err := s.repo.Update(ctx, run); err != nil {
		return nil, err
	}
	return run, nil
}

func (s *AgentRunService) GetDAG(ctx context.Context, incidentID uuid.UUID) ([]models.AgentRun, error) {
	return s.repo.GetDAG(ctx, incidentID)
}

// --- RCIService ---

type RCIService struct {
	repo RCIRepository
}

func NewRCIService(repo RCIRepository) *RCIService {
	return &RCIService{repo: repo}
}

func (s *RCIService) Create(ctx context.Context, incidentID uuid.UUID, req models.CreateRCIRequest) (*models.IncidentRCI, error) {
	now := time.Now()
	rci := &models.IncidentRCI{
		ID:                    uuid.New(),
		IncidentID:            incidentID,
		AgentRunID:            req.AgentRunID,
		Status:                req.Status,
		InvestigationSummary:  req.InvestigationSummary,
		ImpactAssessment:      req.ImpactAssessment,
		AffectedServices:      req.AffectedServices,
		AffectedUsersEstimate: req.AffectedUsersEstimate,
		DetectionMethod:       req.DetectionMethod,
		DetectionTime:         req.DetectionTime,
		AcknowledgmentTime:    req.AcknowledgmentTime,
		TimeToDetectSeconds:   req.TimeToDetectSeconds,
		EvidenceIDs:           req.EvidenceIDs,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if rci.Status == "" {
		rci.Status = "draft"
	}
	if rci.ImpactAssessment == nil {
		rci.ImpactAssessment = json.RawMessage("{}")
	}
	if rci.AffectedServices == nil {
		rci.AffectedServices = json.RawMessage("[]")
	}
	if rci.EvidenceIDs == nil {
		rci.EvidenceIDs = json.RawMessage("[]")
	}
	if err := s.repo.Create(ctx, rci); err != nil {
		return nil, err
	}
	return rci, nil
}

func (s *RCIService) GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentRCI, error) {
	return s.repo.GetByIncidentID(ctx, incidentID)
}

func (s *RCIService) Update(ctx context.Context, incidentID uuid.UUID, req models.CreateRCIRequest) (*models.IncidentRCI, error) {
	rci, err := s.repo.GetByIncidentID(ctx, incidentID)
	if err != nil {
		return nil, err
	}
	rci.Status = req.Status
	rci.InvestigationSummary = req.InvestigationSummary
	if req.ImpactAssessment != nil {
		rci.ImpactAssessment = req.ImpactAssessment
	}
	if req.AffectedServices != nil {
		rci.AffectedServices = req.AffectedServices
	}
	rci.AffectedUsersEstimate = req.AffectedUsersEstimate
	rci.DetectionMethod = req.DetectionMethod
	rci.DetectionTime = req.DetectionTime
	rci.AcknowledgmentTime = req.AcknowledgmentTime
	rci.TimeToDetectSeconds = req.TimeToDetectSeconds
	if req.EvidenceIDs != nil {
		rci.EvidenceIDs = req.EvidenceIDs
	}
	rci.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, rci); err != nil {
		return nil, err
	}
	return rci, nil
}

// --- RCAService ---

type RCAService struct {
	repo      RCARepository
	incidents IncidentRootCauseUpdater
}

func NewRCAService(repo RCARepository, incidents IncidentRootCauseUpdater) *RCAService {
	return &RCAService{repo: repo, incidents: incidents}
}

// backfillIncidentRootCause mirrors the RCA's root_cause_summary onto the
// parent incident's root_cause field, so GET /incidents/{id} reflects the
// finding without callers having to also know about the separate /rca
// sub-resource. Best-effort: an incident lookup/update failure here must
// never fail the RCA create/update itself.
func (s *RCAService) backfillIncidentRootCause(ctx context.Context, incidentID uuid.UUID, summary string) {
	if s.incidents == nil || summary == "" {
		return
	}
	inc, err := s.incidents.GetByID(ctx, incidentID)
	if err != nil {
		return
	}
	inc.RootCause = summary
	inc.UpdatedAt = time.Now()
	_ = s.incidents.Update(ctx, inc)
}

func (s *RCAService) Create(ctx context.Context, incidentID uuid.UUID, req models.CreateRCARequest) (*models.IncidentRCA, error) {
	now := time.Now()
	rca := &models.IncidentRCA{
		ID:                  uuid.New(),
		IncidentID:          incidentID,
		RCIID:               req.RCIID,
		AgentRunID:          req.AgentRunID,
		Status:              req.Status,
		RootCauseSummary:    req.RootCauseSummary,
		RootCauseCategory:   req.RootCauseCategory,
		ContributingFactors: req.ContributingFactors,
		FiveWhys:            req.FiveWhys,
		Confidence:          req.Confidence,
		EvidenceIDs:         req.EvidenceIDs,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if rca.Status == "" {
		rca.Status = "draft"
	}
	if rca.ContributingFactors == nil {
		rca.ContributingFactors = json.RawMessage("[]")
	}
	if rca.FiveWhys == nil {
		rca.FiveWhys = json.RawMessage("[]")
	}
	if rca.EvidenceIDs == nil {
		rca.EvidenceIDs = json.RawMessage("[]")
	}
	if err := s.repo.Create(ctx, rca); err != nil {
		return nil, err
	}
	s.backfillIncidentRootCause(ctx, incidentID, rca.RootCauseSummary)
	return rca, nil
}

func (s *RCAService) GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentRCA, error) {
	return s.repo.GetByIncidentID(ctx, incidentID)
}

func (s *RCAService) Update(ctx context.Context, incidentID uuid.UUID, req models.CreateRCARequest) (*models.IncidentRCA, error) {
	rca, err := s.repo.GetByIncidentID(ctx, incidentID)
	if err != nil {
		return nil, err
	}
	rca.Status = req.Status
	rca.RootCauseSummary = req.RootCauseSummary
	rca.RootCauseCategory = req.RootCauseCategory
	if req.ContributingFactors != nil {
		rca.ContributingFactors = req.ContributingFactors
	}
	if req.FiveWhys != nil {
		rca.FiveWhys = req.FiveWhys
	}
	rca.Confidence = req.Confidence
	if req.EvidenceIDs != nil {
		rca.EvidenceIDs = req.EvidenceIDs
	}
	rca.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, rca); err != nil {
		return nil, err
	}
	s.backfillIncidentRootCause(ctx, incidentID, rca.RootCauseSummary)
	return rca, nil
}

// --- PostmortemService ---

type PostmortemService struct {
	repo PostmortemRepository
}

func NewPostmortemService(repo PostmortemRepository) *PostmortemService {
	return &PostmortemService{repo: repo}
}

func (s *PostmortemService) Create(ctx context.Context, incidentID uuid.UUID, req models.CreatePostmortemRequest) (*models.IncidentPostmortem, error) {
	now := time.Now()
	pm := &models.IncidentPostmortem{
		ID:                        uuid.New(),
		IncidentID:                incidentID,
		RootCausewayD:                     req.RootCausewayD,
		AgentRunID:                req.AgentRunID,
		Status:                    req.Status,
		Title:                     req.Title,
		ExecutiveSummary:          req.ExecutiveSummary,
		IncidentTimelineNarrative: req.IncidentTimelineNarrative,
		RootCauseDetail:           req.RootCauseDetail,
		ImpactDetail:              req.ImpactDetail,
		LessonsLearned:            req.LessonsLearned,
		ActionItems:               req.ActionItems,
		WhatWentWell:              req.WhatWentWell,
		WhatWentWrong:             req.WhatWentWrong,
		PreventionMeasures:        req.PreventionMeasures,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if pm.Status == "" {
		pm.Status = "draft"
	}
	for _, field := range []*json.RawMessage{&pm.LessonsLearned, &pm.ActionItems, &pm.WhatWentWell, &pm.WhatWentWrong, &pm.PreventionMeasures} {
		if *field == nil {
			*field = json.RawMessage("[]")
		}
	}
	if err := s.repo.Create(ctx, pm); err != nil {
		return nil, err
	}
	return pm, nil
}

func (s *PostmortemService) GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentPostmortem, error) {
	return s.repo.GetByIncidentID(ctx, incidentID)
}

func (s *PostmortemService) Update(ctx context.Context, incidentID uuid.UUID, req models.CreatePostmortemRequest) (*models.IncidentPostmortem, error) {
	pm, err := s.repo.GetByIncidentID(ctx, incidentID)
	if err != nil {
		return nil, err
	}
	pm.Status = req.Status
	pm.Title = req.Title
	pm.ExecutiveSummary = req.ExecutiveSummary
	pm.IncidentTimelineNarrative = req.IncidentTimelineNarrative
	pm.RootCauseDetail = req.RootCauseDetail
	pm.ImpactDetail = req.ImpactDetail
	if req.LessonsLearned != nil {
		pm.LessonsLearned = req.LessonsLearned
	}
	if req.ActionItems != nil {
		pm.ActionItems = req.ActionItems
	}
	if req.WhatWentWell != nil {
		pm.WhatWentWell = req.WhatWentWell
	}
	if req.WhatWentWrong != nil {
		pm.WhatWentWrong = req.WhatWentWrong
	}
	if req.PreventionMeasures != nil {
		pm.PreventionMeasures = req.PreventionMeasures
	}
	if pm.Status == "published" {
		now := time.Now()
		pm.PublishedAt = &now
	}
	pm.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, pm); err != nil {
		return nil, err
	}
	return pm, nil
}
