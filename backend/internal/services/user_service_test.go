package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// MockUserRepo / MockUserRoleRepo -- no test coverage existed for
// UserService at all before this file; the List-never-populates-Roles bug
// (below) went unnoticed because of that, not because a test was wrong.

type MockUserRepo struct{ mock.Mock }

func (m *MockUserRepo) Create(ctx context.Context, u *models.UserWithRoles) error {
	return m.Called(ctx, u).Error(0)
}
func (m *MockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.UserWithRoles, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserWithRoles), args.Error(1)
}
func (m *MockUserRepo) GetByEmail(ctx context.Context, email string) (*models.UserWithRoles, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserWithRoles), args.Error(1)
}
func (m *MockUserRepo) GetBySSOSubject(ctx context.Context, provider, subject string) (*models.UserWithRoles, error) {
	args := m.Called(ctx, provider, subject)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.UserWithRoles), args.Error(1)
}
func (m *MockUserRepo) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.UserWithRoles, int, error) {
	args := m.Called(ctx, orgID, page, perPage)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]models.UserWithRoles), args.Int(1), args.Error(2)
}
func (m *MockUserRepo) Update(ctx context.Context, u *models.UserWithRoles) error {
	return m.Called(ctx, u).Error(0)
}
func (m *MockUserRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type MockUserRoleRepo struct{ mock.Mock }

func (m *MockUserRoleRepo) Assign(ctx context.Context, userID, roleID uuid.UUID) error {
	return m.Called(ctx, userID, roleID).Error(0)
}
func (m *MockUserRoleRepo) Unassign(ctx context.Context, userID, roleID uuid.UUID) error {
	return m.Called(ctx, userID, roleID).Error(0)
}
func (m *MockUserRoleRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Role, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Role), args.Error(1)
}

// TestUserService_List_PopulatesRolesPerUser pins a real live-found bug:
// List was a bare passthrough to the repository (which never joins
// user_roles), so every user on the Users page always showed zero roles
// and every role checkbox rendered unchecked -- regardless of what
// AssignRole/UnassignRole (which worked correctly) had actually persisted.
// Confirmed live against real data: the user_roles row existed in Postgres
// after clicking "assign", but the list response never carried it back.
func TestUserService_List_PopulatesRolesPerUser(t *testing.T) {
	userRepo := new(MockUserRepo)
	userRoleRepo := new(MockUserRoleRepo)
	svc := NewUserService(userRepo, userRoleRepo)

	orgID := uuid.New()
	user1 := uuid.New()
	user2 := uuid.New()
	adminRole := models.Role{ID: uuid.New(), Name: "Admin"}
	viewerRole := models.Role{ID: uuid.New(), Name: "Viewer"}

	userRepo.On("List", mock.Anything, orgID, 1, 20).Return(
		[]models.UserWithRoles{
			{User: models.User{ID: user1}},
			{User: models.User{ID: user2}},
		}, 2, nil,
	)
	userRoleRepo.On("ListByUser", mock.Anything, user1).Return([]models.Role{adminRole}, nil)
	userRoleRepo.On("ListByUser", mock.Anything, user2).Return([]models.Role{viewerRole}, nil)

	users, total, err := svc.List(context.Background(), orgID, 1, 20)

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, users, 2)
	require.Len(t, users[0].Roles, 1)
	assert.Equal(t, "Admin", users[0].Roles[0].Name)
	require.Len(t, users[1].Roles, 1)
	assert.Equal(t, "Viewer", users[1].Roles[0].Name)
}

func TestUserService_List_RepositoryErrorPropagates(t *testing.T) {
	userRepo := new(MockUserRepo)
	userRoleRepo := new(MockUserRoleRepo)
	svc := NewUserService(userRepo, userRoleRepo)

	orgID := uuid.New()
	userRepo.On("List", mock.Anything, orgID, 1, 20).Return(nil, 0, assert.AnError)

	users, total, err := svc.List(context.Background(), orgID, 1, 20)

	require.Error(t, err)
	assert.Nil(t, users)
	assert.Equal(t, 0, total)
	userRoleRepo.AssertNotCalled(t, "ListByUser", mock.Anything, mock.Anything)
}

// TestUserService_List_ToleratesRoleLookupFailure ensures one user's
// ListByUser failure doesn't fail the whole list -- that user just shows
// no roles for this response, rather than 500ing everyone else's data too.
func TestUserService_List_ToleratesRoleLookupFailure(t *testing.T) {
	userRepo := new(MockUserRepo)
	userRoleRepo := new(MockUserRoleRepo)
	svc := NewUserService(userRepo, userRoleRepo)

	orgID := uuid.New()
	user1 := uuid.New()
	userRepo.On("List", mock.Anything, orgID, 1, 20).Return(
		[]models.UserWithRoles{{User: models.User{ID: user1}}}, 1, nil,
	)
	userRoleRepo.On("ListByUser", mock.Anything, user1).Return(nil, assert.AnError)

	users, total, err := svc.List(context.Background(), orgID, 1, 20)

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, users, 1)
	assert.Empty(t, users[0].Roles)
}
