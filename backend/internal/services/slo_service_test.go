package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// --- Mock SLORepository ---

type MockSLORepo struct{ mock.Mock }

func (m *MockSLORepo) Create(ctx context.Context, s *models.SLODefinition) error {
	return m.Called(ctx, s).Error(0)
}

func (m *MockSLORepo) GetByID(ctx context.Context, id uuid.UUID) (*models.SLODefinition, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SLODefinition), args.Error(1)
}

func (m *MockSLORepo) List(ctx context.Context, orgID uuid.UUID) ([]models.SLODefinition, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).([]models.SLODefinition), args.Error(1)
}

func (m *MockSLORepo) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.SLODefinition, error) {
	args := m.Called(ctx, softwareID)
	return args.Get(0).([]models.SLODefinition), args.Error(1)
}

func (m *MockSLORepo) Update(ctx context.Context, s *models.SLODefinition) error {
	return m.Called(ctx, s).Error(0)
}

func (m *MockSLORepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockSLORepo) GetIncidentDowntimeMinutes(ctx context.Context, orgID, softwareID uuid.UUID, windowStart, windowEnd time.Time) (float64, int, error) {
	args := m.Called(ctx, orgID, softwareID, windowStart, windowEnd)
	return args.Get(0).(float64), args.Int(1), args.Error(2)
}

// --- Test helpers ---

// newTestSLOService builds an SLOService with a fixed clock so window
// boundaries are deterministic in assertions/mocks.
func newTestSLOService(repo *MockSLORepo, fixedNow time.Time) *SLOService {
	svc := NewSLOService(repo)
	svc.now = func() time.Time { return fixedNow }
	return svc
}

func sampleSLODef(orgID, softwareID uuid.UUID, targetPct float64, windowDays int) *models.SLODefinition {
	return &models.SLODefinition{
		ID:                    uuid.New(),
		OrgID:                 orgID,
		SoftwareID:            softwareID,
		Name:                  "API availability",
		SLOType:               models.SLOTypeAvailability,
		TargetPercentage:      targetPct,
		MeasurementWindowDays: windowDays,
	}
}

// --- CalculateSLOStatus tests ---
//
// Window = 30 days = 43200 minutes. target = 99.9% =>
// error_budget_total_minutes = 43200 * (1 - 0.999) = 43.2 minutes.

func TestCalculateSLOStatus_ZeroIncidents_Healthy(t *testing.T) {
	repo := new(MockSLORepo)
	fixedNow := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	svc := newTestSLOService(repo, fixedNow)

	orgID, softwareID := uuid.New(), uuid.New()
	def := sampleSLODef(orgID, softwareID, 99.9, 30)

	repo.On("GetByID", mock.Anything, def.ID).Return(def, nil)
	repo.On("GetIncidentDowntimeMinutes", mock.Anything, orgID, softwareID, mock.Anything, mock.Anything).
		Return(0.0, 0, nil)

	status, err := svc.CalculateSLOStatus(context.Background(), def.ID)
	require.NoError(t, err)

	assert.Equal(t, models.SLOStatusHealthy, status.Status)
	assert.InDelta(t, 100.0, status.CurrentPercentage, 1e-9)
	assert.InDelta(t, 43.2, status.ErrorBudgetTotalMinutes, 1e-6)
	assert.InDelta(t, 0.0, status.ErrorBudgetConsumedMinutes, 1e-9)
	assert.InDelta(t, 100.0, status.ErrorBudgetRemainingPercentage, 1e-9)
	assert.Equal(t, 0, status.IncidentCount)
	assert.False(t, status.IsApproximated)
	assert.Equal(t, fixedNow, status.WindowEnd)
	assert.Equal(t, fixedNow.Add(-30*24*time.Hour), status.WindowStart)
}

func TestCalculateSLOStatus_Healthy(t *testing.T) {
	repo := new(MockSLORepo)
	fixedNow := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	svc := newTestSLOService(repo, fixedNow)

	orgID, softwareID := uuid.New(), uuid.New()
	def := sampleSLODef(orgID, softwareID, 99.9, 30)

	// 10 minutes of downtime out of a 43.2 minute budget -> ~76.85% remaining.
	repo.On("GetByID", mock.Anything, def.ID).Return(def, nil)
	repo.On("GetIncidentDowntimeMinutes", mock.Anything, orgID, softwareID, mock.Anything, mock.Anything).
		Return(10.0, 2, nil)

	status, err := svc.CalculateSLOStatus(context.Background(), def.ID)
	require.NoError(t, err)

	assert.Equal(t, models.SLOStatusHealthy, status.Status)
	assert.InDelta(t, 99.976851851, status.CurrentPercentage, 1e-6)
	assert.InDelta(t, 43.2, status.ErrorBudgetTotalMinutes, 1e-6)
	assert.InDelta(t, 10.0, status.ErrorBudgetConsumedMinutes, 1e-9)
	assert.InDelta(t, 76.851851851, status.ErrorBudgetRemainingPercentage, 1e-6)
	assert.Equal(t, 2, status.IncidentCount)
}

func TestCalculateSLOStatus_AtRisk(t *testing.T) {
	repo := new(MockSLORepo)
	fixedNow := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	svc := newTestSLOService(repo, fixedNow)

	orgID, softwareID := uuid.New(), uuid.New()
	def := sampleSLODef(orgID, softwareID, 99.9, 30)

	// 40 minutes of downtime out of a 43.2 minute budget -> remaining = 3.2
	// minutes = ~7.4% remaining, which is < 20% but > 0 -> at_risk.
	repo.On("GetByID", mock.Anything, def.ID).Return(def, nil)
	repo.On("GetIncidentDowntimeMinutes", mock.Anything, orgID, softwareID, mock.Anything, mock.Anything).
		Return(40.0, 5, nil)

	status, err := svc.CalculateSLOStatus(context.Background(), def.ID)
	require.NoError(t, err)

	assert.Equal(t, models.SLOStatusAtRisk, status.Status)
	assert.InDelta(t, 3.2, status.ErrorBudgetTotalMinutes-status.ErrorBudgetConsumedMinutes, 1e-6)
	assert.InDelta(t, 7.407407, status.ErrorBudgetRemainingPercentage, 1e-4)
}

func TestCalculateSLOStatus_Exhausted(t *testing.T) {
	repo := new(MockSLORepo)
	fixedNow := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	svc := newTestSLOService(repo, fixedNow)

	orgID, softwareID := uuid.New(), uuid.New()
	def := sampleSLODef(orgID, softwareID, 99.9, 30)

	// 100 minutes of downtime exceeds the 43.2 minute budget entirely.
	repo.On("GetByID", mock.Anything, def.ID).Return(def, nil)
	repo.On("GetIncidentDowntimeMinutes", mock.Anything, orgID, softwareID, mock.Anything, mock.Anything).
		Return(100.0, 8, nil)

	status, err := svc.CalculateSLOStatus(context.Background(), def.ID)
	require.NoError(t, err)

	assert.Equal(t, models.SLOStatusExhausted, status.Status)
	assert.InDelta(t, 0.0, status.ErrorBudgetRemainingPercentage, 1e-9)
	assert.InDelta(t, 99.768518, status.CurrentPercentage, 1e-4)
}

func TestCalculateSLOStatus_HundredPercentTarget_AnyDowntimeExhausts(t *testing.T) {
	repo := new(MockSLORepo)
	fixedNow := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	svc := newTestSLOService(repo, fixedNow)

	orgID, softwareID := uuid.New(), uuid.New()
	def := sampleSLODef(orgID, softwareID, 100.0, 30)

	repo.On("GetByID", mock.Anything, def.ID).Return(def, nil)
	repo.On("GetIncidentDowntimeMinutes", mock.Anything, orgID, softwareID, mock.Anything, mock.Anything).
		Return(1.0, 1, nil)

	status, err := svc.CalculateSLOStatus(context.Background(), def.ID)
	require.NoError(t, err)

	assert.Equal(t, models.SLOStatusExhausted, status.Status)
	assert.InDelta(t, 0.0, status.ErrorBudgetTotalMinutes, 1e-9)
}

func TestCalculateSLOStatus_LatencyType_IsApproximated(t *testing.T) {
	repo := new(MockSLORepo)
	fixedNow := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	svc := newTestSLOService(repo, fixedNow)

	orgID, softwareID := uuid.New(), uuid.New()
	def := sampleSLODef(orgID, softwareID, 99.9, 30)
	def.SLOType = models.SLOTypeLatency

	repo.On("GetByID", mock.Anything, def.ID).Return(def, nil)
	repo.On("GetIncidentDowntimeMinutes", mock.Anything, orgID, softwareID, mock.Anything, mock.Anything).
		Return(0.0, 0, nil)

	status, err := svc.CalculateSLOStatus(context.Background(), def.ID)
	require.NoError(t, err)

	assert.True(t, status.IsApproximated)
	assert.Equal(t, models.SLOTypeLatency, status.SLOType)
}

func TestCalculateSLOStatus_NotFound(t *testing.T) {
	repo := new(MockSLORepo)
	svc := newTestSLOService(repo, time.Now())

	id := uuid.New()
	repo.On("GetByID", mock.Anything, id).Return(nil, assert.AnError)

	_, err := svc.CalculateSLOStatus(context.Background(), id)
	assert.Error(t, err)
}

// --- SoftwareSLOStatus ---

func TestSoftwareSLOStatus_AggregatesAllDefinitions(t *testing.T) {
	repo := new(MockSLORepo)
	fixedNow := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	svc := newTestSLOService(repo, fixedNow)

	orgID, softwareID := uuid.New(), uuid.New()
	def1 := sampleSLODef(orgID, softwareID, 99.9, 30)
	def2 := sampleSLODef(orgID, softwareID, 99.0, 7)
	def2.SLOType = models.SLOTypeErrorRate

	repo.On("ListBySoftware", mock.Anything, softwareID).Return([]models.SLODefinition{*def1, *def2}, nil)
	repo.On("GetIncidentDowntimeMinutes", mock.Anything, orgID, softwareID, mock.Anything, mock.Anything).
		Return(0.0, 0, nil)

	result, err := svc.SoftwareSLOStatus(context.Background(), softwareID)
	require.NoError(t, err)
	assert.Equal(t, softwareID, result.SoftwareID)
	require.Len(t, result.SLOs, 2)
	assert.False(t, result.SLOs[0].IsApproximated)
	assert.True(t, result.SLOs[1].IsApproximated)
}

// --- CRUD ---

func TestSLOService_Create_DefaultsWindowTo30Days(t *testing.T) {
	repo := new(MockSLORepo)
	svc := NewSLOService(repo)

	orgID := uuid.New()
	req := models.CreateSLODefinitionRequest{
		SoftwareID:       uuid.New(),
		Name:             "API availability",
		SLOType:          models.SLOTypeAvailability,
		TargetPercentage: 99.9,
	}

	repo.On("Create", mock.Anything, mock.AnythingOfType("*models.SLODefinition")).Return(nil)

	def, err := svc.Create(context.Background(), orgID, req)
	require.NoError(t, err)
	assert.Equal(t, 30, def.MeasurementWindowDays)
	assert.Equal(t, orgID, def.OrgID)
}

func TestSLOService_Update_PartialUpdateOnlyAppliesProvidedFields(t *testing.T) {
	repo := new(MockSLORepo)
	svc := NewSLOService(repo)

	existing := sampleSLODef(uuid.New(), uuid.New(), 99.9, 30)
	repo.On("GetByID", mock.Anything, existing.ID).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.AnythingOfType("*models.SLODefinition")).Return(nil)

	updated, err := svc.Update(context.Background(), existing.ID, models.UpdateSLODefinitionRequest{
		TargetPercentage: 99.95,
	})
	require.NoError(t, err)
	assert.Equal(t, 99.95, updated.TargetPercentage)
	assert.Equal(t, "API availability", updated.Name) // unchanged
}

func TestSLOService_Delete(t *testing.T) {
	repo := new(MockSLORepo)
	svc := NewSLOService(repo)

	id := uuid.New()
	repo.On("Delete", mock.Anything, id).Return(nil)

	err := svc.Delete(context.Background(), id)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}
