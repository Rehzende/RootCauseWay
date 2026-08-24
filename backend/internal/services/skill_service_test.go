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

type MockSkillRepo struct{ mock.Mock }

func (m *MockSkillRepo) Create(ctx context.Context, s *models.Skill) error {
	return m.Called(ctx, s).Error(0)
}
func (m *MockSkillRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Skill, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Skill), args.Error(1)
}
func (m *MockSkillRepo) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Skill, error) {
	args := m.Called(ctx, orgID, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Skill), args.Error(1)
}
func (m *MockSkillRepo) List(ctx context.Context, orgID uuid.UUID, category string, page, perPage int) ([]models.Skill, int, error) {
	args := m.Called(ctx, orgID, category, page, perPage)
	return args.Get(0).([]models.Skill), args.Int(1), args.Error(2)
}
func (m *MockSkillRepo) Update(ctx context.Context, s *models.Skill) error {
	return m.Called(ctx, s).Error(0)
}
func (m *MockSkillRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// TestSkillService_Update_TogglesEnabled covers the fix for a real bug a
// user hit live: SkillDetail's enable/disable toggle sends only
// {"enabled": false} -- before Enabled existed on CreateSkillRequest at
// all, that field was silently dropped by Gin's JSON binding (extra keys
// ignored) and the request also 400'd anyway (Name/Slug are
// binding:"required", never sent by the toggle). The toggle button did
// nothing but show an error toast.
func TestSkillService_Update_TogglesEnabled(t *testing.T) {
	repo := new(MockSkillRepo)
	svc := NewSkillService(repo)
	id := uuid.New()

	existing := &models.Skill{ID: id, Name: "Memory Leak Deep Dive", Slug: "memory-leak-deep-dive", Enabled: true}
	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *models.Skill) bool {
		return s.Enabled == false && s.Name == "Memory Leak Deep Dive"
	})).Return(nil)

	disabled := false
	got, err := svc.Update(context.Background(), id, models.CreateSkillRequest{
		Name: "Memory Leak Deep Dive", Slug: "memory-leak-deep-dive", Enabled: &disabled,
	})

	require.NoError(t, err)
	assert.False(t, got.Enabled)
	repo.AssertExpectations(t)
}

func TestSkillService_Update_PreservesEnabledWhenNotProvided(t *testing.T) {
	repo := new(MockSkillRepo)
	svc := NewSkillService(repo)
	id := uuid.New()

	existing := &models.Skill{ID: id, Name: "x", Slug: "x", Enabled: true}
	repo.On("GetByID", mock.Anything, id).Return(existing, nil)
	repo.On("Update", mock.Anything, mock.MatchedBy(func(s *models.Skill) bool {
		return s.Enabled == true
	})).Return(nil)

	got, err := svc.Update(context.Background(), id, models.CreateSkillRequest{Name: "x", Slug: "x"})

	require.NoError(t, err)
	assert.True(t, got.Enabled)
	repo.AssertExpectations(t)
}
