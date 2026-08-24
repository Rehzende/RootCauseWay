package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// RetentionRepository is the persistence contract RunRetentionSweep depends
// on. Declared here (rather than reusing database.PgRetentionRepository
// directly) so it can be mocked in service tests, matching the pattern used
// by the other *Repository interfaces in this package.
type RetentionRepository interface {
	CreatePolicy(ctx context.Context, p *models.RetentionPolicy) error
	GetPolicy(ctx context.Context, id uuid.UUID) (*models.RetentionPolicy, error)
	ListPolicies(ctx context.Context, orgID uuid.UUID) ([]models.RetentionPolicy, error)
	ListEnabledPolicies(ctx context.Context, orgID uuid.UUID) ([]models.RetentionPolicy, error)
	UpdatePolicy(ctx context.Context, p *models.RetentionPolicy) error
	DeletePolicy(ctx context.Context, id uuid.UUID) error
	ListOrgIDs(ctx context.Context) ([]uuid.UUID, error)

	FindExpiredIncidents(ctx context.Context, orgID uuid.UUID, olderThanDays int) ([]database.ExpiredRecord, error)
	FindExpiredEvidence(ctx context.Context, orgID uuid.UUID, olderThanDays int) ([]database.ExpiredRecord, error)
	FindExpiredAgentRuns(ctx context.Context, orgID uuid.UUID, olderThanDays int) ([]database.ExpiredRecord, error)

	ArchiveRecord(ctx context.Context, orgID uuid.UUID, resourceType string, resourceID uuid.UUID, data json.RawMessage) error
	DeleteIncidentCascade(ctx context.Context, incidentID uuid.UUID) error
	DeleteEvidence(ctx context.Context, evidenceID uuid.UUID) error
	DeleteAgentRun(ctx context.Context, agentRunID uuid.UUID) error
}

// RetentionService runs configured retention policies (archive or hard
// delete) against expired evidence, closed incidents, and agent runs.
//
// v1 scheduling scope: there is no in-process cron/ticker here. Callers
// trigger a sweep on demand -- today that's the manual
// POST /api/v1/retention-policies/sweep endpoint (retention_handlers.go).
// RunRetentionSweep/RunAllOrgsSweep take no dependency on how they're
// invoked, so a future scheduled job (a ticker goroutine added to main.go,
// or an external cron hitting the same endpoint) can call them unchanged.
type RetentionService struct {
	repo RetentionRepository
}

func NewRetentionService(repo RetentionRepository) *RetentionService {
	return &RetentionService{repo: repo}
}

// --- Policy CRUD passthroughs ---

func (s *RetentionService) CreatePolicy(ctx context.Context, orgID uuid.UUID, req models.CreateRetentionPolicyRequest) (*models.RetentionPolicy, error) {
	now := time.Now()
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	p := &models.RetentionPolicy{
		ID:            uuid.New(),
		OrgID:         orgID,
		ResourceType:  req.ResourceType,
		RetentionDays: req.RetentionDays,
		Action:        req.Action,
		Enabled:       enabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreatePolicy(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *RetentionService) GetPolicy(ctx context.Context, id uuid.UUID) (*models.RetentionPolicy, error) {
	return s.repo.GetPolicy(ctx, id)
}

func (s *RetentionService) ListPolicies(ctx context.Context, orgID uuid.UUID) ([]models.RetentionPolicy, error) {
	return s.repo.ListPolicies(ctx, orgID)
}

func (s *RetentionService) UpdatePolicy(ctx context.Context, id uuid.UUID, req models.UpdateRetentionPolicyRequest) (*models.RetentionPolicy, error) {
	p, err := s.repo.GetPolicy(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.RetentionDays != nil {
		p.RetentionDays = *req.RetentionDays
	}
	if req.Action != nil {
		p.Action = *req.Action
	}
	if req.Enabled != nil {
		p.Enabled = *req.Enabled
	}
	p.UpdatedAt = time.Now()
	if err := s.repo.UpdatePolicy(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *RetentionService) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeletePolicy(ctx, id)
}

// --- Sweep ---

// RunRetentionSweep loads the enabled retention policies for orgID and, for
// each one, finds the expired records for its resource_type and either
// archives (snapshot to archived_records, then delete) or hard-deletes
// them. It returns a per-policy summary of what happened.
//
// A per-record archive/delete failure is recorded in that policy's
// RetentionSweepResult.Errors and the sweep continues with the remaining
// records/policies -- one bad row shouldn't abort the whole org's sweep.
func (s *RetentionService) RunRetentionSweep(ctx context.Context, orgID uuid.UUID) (*models.RetentionSweepSummary, error) {
	summary := &models.RetentionSweepSummary{
		OrgID:     orgID,
		StartedAt: time.Now(),
		Results:   []models.RetentionSweepResult{},
	}

	policies, err := s.repo.ListEnabledPolicies(ctx, orgID)
	if err != nil {
		return nil, err
	}

	for _, policy := range policies {
		result := models.RetentionSweepResult{
			PolicyID:     policy.ID,
			ResourceType: policy.ResourceType,
			Action:       policy.Action,
		}

		var expired []database.ExpiredRecord
		switch policy.ResourceType {
		case models.RetentionResourceIncidents:
			expired, err = s.repo.FindExpiredIncidents(ctx, orgID, policy.RetentionDays)
		case models.RetentionResourceEvidence:
			expired, err = s.repo.FindExpiredEvidence(ctx, orgID, policy.RetentionDays)
		case models.RetentionResourceAgentRuns:
			expired, err = s.repo.FindExpiredAgentRuns(ctx, orgID, policy.RetentionDays)
		default:
			slog.Warn("retention sweep: unknown resource_type, skipping policy", "org_id", orgID, "policy_id", policy.ID, "resource_type", policy.ResourceType)
			continue
		}
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			summary.Results = append(summary.Results, result)
			continue
		}
		result.MatchedCount = len(expired)

		for _, rec := range expired {
			if policy.Action == models.RetentionActionArchive {
				if err := s.repo.ArchiveRecord(ctx, orgID, policy.ResourceType, rec.ID, rec.Data); err != nil {
					result.Errors = append(result.Errors, "archive "+rec.ID.String()+": "+err.Error())
					continue
				}
				result.ArchivedCount++
			}
			// Both "archive" and "delete" remove the source row; "archive"
			// only differs in that a snapshot was written first above.
			if err := s.deleteResource(ctx, policy.ResourceType, rec.ID); err != nil {
				result.Errors = append(result.Errors, "delete "+rec.ID.String()+": "+err.Error())
				continue
			}
			result.DeletedCount++
		}

		summary.Results = append(summary.Results, result)
	}

	return summary, nil
}

func (s *RetentionService) deleteResource(ctx context.Context, resourceType string, id uuid.UUID) error {
	switch resourceType {
	case models.RetentionResourceIncidents:
		return s.repo.DeleteIncidentCascade(ctx, id)
	case models.RetentionResourceEvidence:
		return s.repo.DeleteEvidence(ctx, id)
	case models.RetentionResourceAgentRuns:
		return s.repo.DeleteAgentRun(ctx, id)
	}
	return nil
}

// RunAllOrgsSweep runs RunRetentionSweep for every organization. It is not
// wired to any scheduler in v1 (see package doc comment above) but exists
// so a future ticker/cron only needs to call this one method. Per-org
// errors are logged and do not stop the sweep for the remaining orgs.
func (s *RetentionService) RunAllOrgsSweep(ctx context.Context) ([]models.RetentionSweepSummary, error) {
	orgIDs, err := s.repo.ListOrgIDs(ctx)
	if err != nil {
		return nil, err
	}

	summaries := make([]models.RetentionSweepSummary, 0, len(orgIDs))
	for _, orgID := range orgIDs {
		summary, err := s.RunRetentionSweep(ctx, orgID)
		if err != nil {
			slog.Error("retention sweep failed for org", "org_id", orgID, "error", err.Error())
			continue
		}
		summaries = append(summaries, *summary)
	}
	return summaries, nil
}
