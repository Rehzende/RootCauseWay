package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockRCARepo struct {
	mock.Mock
}

func (m *MockRCARepo) Create(ctx context.Context, rca *models.IncidentRCA) error {
	return m.Called(ctx, rca).Error(0)
}

func (m *MockRCARepo) GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentRCA, error) {
	args := m.Called(ctx, incidentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.IncidentRCA), args.Error(1)
}

func (m *MockRCARepo) Update(ctx context.Context, rca *models.IncidentRCA) error {
	return m.Called(ctx, rca).Error(0)
}

// TestRCAService_Create_BackfillsIncidentRootCause guards against the gap
// found while validating the automated alert->RCA pipeline under load: the
// RCI/RCA sub-resources persisted correctly, but GET /incidents/{id} kept
// root_cause empty because nothing mirrored it back onto the incident row.
func TestRCAService_Create_BackfillsIncidentRootCause(t *testing.T) {
	rcaRepo := new(MockRCARepo)
	incRepo := new(MockIncidentRepo)
	svc := NewRCAService(rcaRepo, incRepo)

	incidentID := uuid.New()
	existing := &models.Incident{ID: incidentID, RootCause: ""}

	rcaRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.IncidentRCA")).Return(nil)
	incRepo.On("GetByID", mock.Anything, incidentID).Return(existing, nil)
	incRepo.On("Update", mock.Anything, mock.MatchedBy(func(i *models.Incident) bool {
		return i.RootCause == "misconfigured database connection pool"
	})).Return(nil)

	req := models.CreateRCARequest{RootCauseSummary: "misconfigured database connection pool"}
	rca, err := svc.Create(context.Background(), incidentID, req)

	require.NoError(t, err)
	assert.Equal(t, "misconfigured database connection pool", rca.RootCauseSummary)
	incRepo.AssertCalled(t, "Update", mock.Anything, mock.MatchedBy(func(i *models.Incident) bool {
		return i.RootCause == "misconfigured database connection pool"
	}))
}

// TestRCAService_Update_BackfillsIncidentRootCause covers the /rca PUT path
// (e.g. an analyst editing the LLM-generated summary) mirrors the same way.
func TestRCAService_Update_BackfillsIncidentRootCause(t *testing.T) {
	rcaRepo := new(MockRCARepo)
	incRepo := new(MockIncidentRepo)
	svc := NewRCAService(rcaRepo, incRepo)

	incidentID := uuid.New()
	existingRCA := &models.IncidentRCA{IncidentID: incidentID, RootCauseSummary: "old summary"}
	existingIncident := &models.Incident{ID: incidentID, RootCause: "old summary"}

	rcaRepo.On("GetByIncidentID", mock.Anything, incidentID).Return(existingRCA, nil)
	rcaRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.IncidentRCA")).Return(nil)
	incRepo.On("GetByID", mock.Anything, incidentID).Return(existingIncident, nil)
	incRepo.On("Update", mock.MatchedBy(func(ctx context.Context) bool { return true }), mock.MatchedBy(func(i *models.Incident) bool {
		return i.RootCause == "revised root cause"
	})).Return(nil)

	req := models.CreateRCARequest{RootCauseSummary: "revised root cause"}
	rca, err := svc.Update(context.Background(), incidentID, req)

	require.NoError(t, err)
	assert.Equal(t, "revised root cause", rca.RootCauseSummary)
}

// TestRCAService_Create_BackfillFailureDoesNotFailRequest ensures the
// mirroring stays best-effort: an incident lookup error must not turn a
// successful RCA persist into a 500 for the caller.
func TestRCAService_Create_BackfillFailureDoesNotFailRequest(t *testing.T) {
	rcaRepo := new(MockRCARepo)
	incRepo := new(MockIncidentRepo)
	svc := NewRCAService(rcaRepo, incRepo)

	incidentID := uuid.New()
	rcaRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.IncidentRCA")).Return(nil)
	incRepo.On("GetByID", mock.Anything, incidentID).Return(nil, assertAnError)

	req := models.CreateRCARequest{RootCauseSummary: "some cause"}
	rca, err := svc.Create(context.Background(), incidentID, req)

	require.NoError(t, err)
	assert.NotNil(t, rca)
}

var assertAnError = context.DeadlineExceeded
