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

type MockKnowledgeBaseRepo struct{ mock.Mock }

func (m *MockKnowledgeBaseRepo) Create(ctx context.Context, kb *models.KnowledgeBaseEntry) error {
	return m.Called(ctx, kb).Error(0)
}
func (m *MockKnowledgeBaseRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.KnowledgeBaseEntry, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.KnowledgeBaseEntry), args.Error(1)
}
func (m *MockKnowledgeBaseRepo) List(ctx context.Context, orgID uuid.UUID, category string) ([]models.KnowledgeBaseEntry, error) {
	args := m.Called(ctx, orgID, category)
	return args.Get(0).([]models.KnowledgeBaseEntry), args.Error(1)
}
func (m *MockKnowledgeBaseRepo) Search(ctx context.Context, orgID uuid.UUID, softwareID *uuid.UUID, errorPattern string) ([]models.KnowledgeBaseEntry, error) {
	args := m.Called(ctx, orgID, softwareID, errorPattern)
	return args.Get(0).([]models.KnowledgeBaseEntry), args.Error(1)
}
func (m *MockKnowledgeBaseRepo) Update(ctx context.Context, kb *models.KnowledgeBaseEntry) error {
	return m.Called(ctx, kb).Error(0)
}
func (m *MockKnowledgeBaseRepo) IncrementReferences(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// TestKnowledgeBaseService_Create_PersistsIncidentID covers the fix for a
// platform audit finding: CreateKnowledgeBaseRequest had no IncidentID
// field at all, so agent-service's automatic post-incident write (which
// does send an incident_id) had that association silently dropped even
// once the request otherwise succeeded -- the entity itself
// (KnowledgeBaseEntry.IncidentID) was always able to store it, only the
// request struct and this Create wiring were missing it.
func TestKnowledgeBaseService_Create_PersistsIncidentID(t *testing.T) {
	repo := new(MockKnowledgeBaseRepo)
	svc := NewKnowledgeBaseService(repo)

	orgID := uuid.New()
	incidentID := uuid.New()
	repo.On("Create", mock.Anything, mock.MatchedBy(func(kb *models.KnowledgeBaseEntry) bool {
		return kb.IncidentID != nil && *kb.IncidentID == incidentID && kb.OrgID == orgID
	})).Return(nil)

	req := models.CreateKnowledgeBaseRequest{
		IncidentID:       &incidentID,
		Category:         "infrastructure",
		RootCauseSummary: "Connection pool exhausted",
	}
	got, err := svc.Create(context.Background(), orgID, req)

	require.NoError(t, err)
	require.NotNil(t, got.IncidentID)
	assert.Equal(t, incidentID, *got.IncidentID)
	repo.AssertExpectations(t)
}

// TestKnowledgeBaseService_CreateFromHumanCorrection_SetsHumanValidated
// covers the human-feedback-loop fix: a human's correction on a wrong RCA
// must land as human_validated=true, confidence=1.0 -- the generic
// CreateKnowledgeBaseRequest path used by agent-service's automatic
// post-incident write has no field for either, and must never accidentally
// default to human_validated for an unreviewed LLM output.
func TestKnowledgeBaseService_CreateFromHumanCorrection_SetsHumanValidated(t *testing.T) {
	repo := new(MockKnowledgeBaseRepo)
	svc := NewKnowledgeBaseService(repo)

	orgID := uuid.New()
	incidentID := uuid.New()
	repo.On("Create", mock.Anything, mock.MatchedBy(func(kb *models.KnowledgeBaseEntry) bool {
		return kb.OrgID == orgID &&
			kb.IncidentID != nil && *kb.IncidentID == incidentID &&
			kb.HumanValidated == true &&
			kb.Confidence == 1.0 &&
			kb.RootCauseSummary == "the real root cause"
	})).Return(nil)

	got, err := svc.CreateFromHumanCorrection(context.Background(), orgID, incidentID, nil, "the real root cause")

	require.NoError(t, err)
	assert.True(t, got.HumanValidated)
	assert.Equal(t, 1.0, got.Confidence)
	repo.AssertExpectations(t)
}

func TestKnowledgeBaseService_Create_DefaultsEmptyJSONArrays(t *testing.T) {
	repo := new(MockKnowledgeBaseRepo)
	svc := NewKnowledgeBaseService(repo)
	orgID := uuid.New()

	repo.On("Create", mock.Anything, mock.MatchedBy(func(kb *models.KnowledgeBaseEntry) bool {
		return string(kb.LessonsLearned) == "[]" && string(kb.ActionItems) == "[]" && string(kb.Tags) == "[]"
	})).Return(nil)

	_, err := svc.Create(context.Background(), orgID, models.CreateKnowledgeBaseRequest{
		RootCauseSummary: "leak",
	})

	require.NoError(t, err)
	repo.AssertExpectations(t)
}
