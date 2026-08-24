package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// SLORepository is the persistence contract needed by SLOService. It covers
// both slo_definitions CRUD and the incident-derived downtime signal used to
// compute error budget burn.
type SLORepository interface {
	Create(ctx context.Context, s *models.SLODefinition) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.SLODefinition, error)
	List(ctx context.Context, orgID uuid.UUID) ([]models.SLODefinition, error)
	ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.SLODefinition, error)
	Update(ctx context.Context, s *models.SLODefinition) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetIncidentDowntimeMinutes(ctx context.Context, orgID, softwareID uuid.UUID, windowStart, windowEnd time.Time) (downtimeMinutes float64, incidentCount int, err error)
}

// atRiskThresholdPct is the fraction of remaining error budget below which
// status flips from "healthy" to "at_risk" (20%, per spec).
const atRiskThresholdPct = 20.0

type SLOService struct {
	repo SLORepository
	// now is overridable in tests so window boundaries are deterministic.
	now func() time.Time
}

func NewSLOService(repo SLORepository) *SLOService {
	return &SLOService{repo: repo, now: time.Now}
}

// --- CRUD ---

func (s *SLOService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateSLODefinitionRequest) (*models.SLODefinition, error) {
	now := s.now()
	windowDays := req.MeasurementWindowDays
	if windowDays <= 0 {
		windowDays = 30
	}
	def := &models.SLODefinition{
		ID:                    uuid.New(),
		OrgID:                 orgID,
		SoftwareID:            req.SoftwareID,
		Name:                  req.Name,
		SLOType:               req.SLOType,
		TargetPercentage:      req.TargetPercentage,
		MeasurementWindowDays: windowDays,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := s.repo.Create(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

func (s *SLOService) GetByID(ctx context.Context, id uuid.UUID) (*models.SLODefinition, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SLOService) List(ctx context.Context, orgID uuid.UUID) ([]models.SLODefinition, error) {
	return s.repo.List(ctx, orgID)
}

func (s *SLOService) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.SLODefinition, error) {
	return s.repo.ListBySoftware(ctx, softwareID)
}

func (s *SLOService) Update(ctx context.Context, id uuid.UUID, req models.UpdateSLODefinitionRequest) (*models.SLODefinition, error) {
	def, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != "" {
		def.Name = req.Name
	}
	if req.SLOType != "" {
		def.SLOType = req.SLOType
	}
	if req.TargetPercentage > 0 {
		def.TargetPercentage = req.TargetPercentage
	}
	if req.MeasurementWindowDays > 0 {
		def.MeasurementWindowDays = req.MeasurementWindowDays
	}
	def.UpdatedAt = s.now()
	if err := s.repo.Update(ctx, def); err != nil {
		return nil, err
	}
	return def, nil
}

func (s *SLOService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// --- Error budget computation ---
//
// Math (documented per task spec):
//
//	window_total_minutes         = measurement_window_days * 24 * 60
//	downtime_minutes             = sum of incident open-duration overlapping
//	                                the window (see SLORepository.GetIncidentDowntimeMinutes)
//	current_percentage           = 100 * (1 - downtime_minutes / window_total_minutes)
//	error_budget_total_minutes   = window_total_minutes * (1 - target_percentage/100)
//	error_budget_consumed_minutes = downtime_minutes
//	error_budget_remaining_minutes = max(0, error_budget_total_minutes - error_budget_consumed_minutes)
//	error_budget_remaining_percentage = 100 * remaining_minutes / error_budget_total_minutes
//
//	status = "exhausted" if remaining_minutes <= 0 (and a budget exists to exhaust)
//	       = "at_risk"   if remaining_percentage < 20%
//	       = "healthy"   otherwise
//
// For slo_type == "availability", downtime_minutes is a literal reading:
// total minutes where an open incident existed for that software within the
// window (see repository doc comment for how overlapping incidents are
// handled).
//
// For slo_type == "latency" or "error_rate": this codebase has no raw
// request/latency metric ingestion (no per-request or per-endpoint metrics
// table -- confirmed by grep across backend/ and contracts/ for
// latency_ms/error_rate/request_count-style columns). Rather than fabricate
// a precise-looking number from data that doesn't exist, we reuse the same
// incident-overlap-minutes signal as a best-effort proxy: incidents are
// treated as "SLO-impacting minutes" regardless of type, since this
// codebase does not tag incidents by which SLO dimension (availability vs.
// latency vs. error rate) they affected. The resulting SLOStatus is
// returned with IsApproximated=true for these two types so callers/UI can
// visually flag it as an approximation rather than a precise measurement.
// A real implementation would wire this to the existing observability_sources
// / snapshot_configs metric-query infrastructure (backend/internal/services/observability_service.go)
// once that pipeline captures raw latency/error-rate time series.
func (s *SLOService) CalculateSLOStatus(ctx context.Context, sloDefinitionID uuid.UUID) (*models.SLOStatus, error) {
	def, err := s.repo.GetByID(ctx, sloDefinitionID)
	if err != nil {
		return nil, err
	}
	return s.calculateForDefinition(ctx, def)
}

func (s *SLOService) calculateForDefinition(ctx context.Context, def *models.SLODefinition) (*models.SLOStatus, error) {
	windowEnd := s.now()
	windowStart := windowEnd.Add(-time.Duration(def.MeasurementWindowDays) * 24 * time.Hour)

	downtimeMinutes, incidentCount, err := s.repo.GetIncidentDowntimeMinutes(ctx, def.OrgID, def.SoftwareID, windowStart, windowEnd)
	if err != nil {
		return nil, err
	}

	windowTotalMinutes := float64(def.MeasurementWindowDays) * 24 * 60

	currentPercentage := 100.0
	if windowTotalMinutes > 0 {
		currentPercentage = 100 * (1 - downtimeMinutes/windowTotalMinutes)
	}
	if currentPercentage < 0 {
		currentPercentage = 0
	}

	errorBudgetTotal := windowTotalMinutes * (1 - def.TargetPercentage/100)
	errorBudgetConsumed := downtimeMinutes

	remaining := errorBudgetTotal - errorBudgetConsumed
	if remaining < 0 {
		remaining = 0
	}

	var remainingPct float64
	var status string
	switch {
	case errorBudgetTotal <= 0:
		// A 100% target has zero allowed budget: any downtime at all
		// exhausts it immediately; zero downtime keeps it (trivially) healthy.
		if errorBudgetConsumed > 0 {
			remainingPct = 0
			status = models.SLOStatusExhausted
		} else {
			remainingPct = 100
			status = models.SLOStatusHealthy
		}
	default:
		remainingPct = 100 * remaining / errorBudgetTotal
		switch {
		case remaining <= 0:
			status = models.SLOStatusExhausted
		case remainingPct < atRiskThresholdPct:
			status = models.SLOStatusAtRisk
		default:
			status = models.SLOStatusHealthy
		}
	}

	return &models.SLOStatus{
		SLODefinitionID:                def.ID,
		SoftwareID:                     def.SoftwareID,
		SLOType:                        def.SLOType,
		TargetPercentage:               def.TargetPercentage,
		WindowStart:                    windowStart,
		WindowEnd:                      windowEnd,
		CurrentPercentage:              currentPercentage,
		ErrorBudgetTotalMinutes:        errorBudgetTotal,
		ErrorBudgetConsumedMinutes:     errorBudgetConsumed,
		ErrorBudgetRemainingPercentage: remainingPct,
		Status:                         status,
		IncidentCount:                  incidentCount,
		IsApproximated:                 def.SLOType != models.SLOTypeAvailability,
	}, nil
}

// SoftwareSLOStatus computes SLOStatus for every SLO definition attached to
// a software entry (used by GET /software/:id/slo-status).
func (s *SLOService) SoftwareSLOStatus(ctx context.Context, softwareID uuid.UUID) (*models.SoftwareSLOStatus, error) {
	defs, err := s.repo.ListBySoftware(ctx, softwareID)
	if err != nil {
		return nil, err
	}
	statuses := make([]models.SLOStatus, 0, len(defs))
	for i := range defs {
		st, err := s.calculateForDefinition(ctx, &defs[i])
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, *st)
	}
	return &models.SoftwareSLOStatus{SoftwareID: softwareID, SLOs: statuses}, nil
}
