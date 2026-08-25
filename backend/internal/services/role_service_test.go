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

// No test coverage existed for RoleService at all before this file -- the
// List/GetByID-never-populate-Permissions bug (below) went unnoticed
// because of that.

type MockRoleRepo struct{ mock.Mock }

func (m *MockRoleRepo) Create(ctx context.Context, r *models.Role) error {
	return m.Called(ctx, r).Error(0)
}
func (m *MockRoleRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Role), args.Error(1)
}
func (m *MockRoleRepo) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Role, error) {
	args := m.Called(ctx, orgID, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Role), args.Error(1)
}
func (m *MockRoleRepo) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.Role, int, error) {
	args := m.Called(ctx, orgID, page, perPage)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]models.Role), args.Int(1), args.Error(2)
}
func (m *MockRoleRepo) Update(ctx context.Context, r *models.Role) error {
	return m.Called(ctx, r).Error(0)
}
func (m *MockRoleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

type MockRolePermRepo struct{ mock.Mock }

func (m *MockRolePermRepo) Grant(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return m.Called(ctx, roleID, permissionID).Error(0)
}
func (m *MockRolePermRepo) Revoke(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return m.Called(ctx, roleID, permissionID).Error(0)
}
func (m *MockRolePermRepo) ListByRole(ctx context.Context, roleID uuid.UUID) ([]models.Permission, error) {
	args := m.Called(ctx, roleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Permission), args.Error(1)
}

type MockPermissionRepo struct{ mock.Mock }

func (m *MockPermissionRepo) List(ctx context.Context) ([]models.Permission, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Permission), args.Error(1)
}
func (m *MockPermissionRepo) GetByResourceAction(ctx context.Context, resource, action string) (*models.Permission, error) {
	args := m.Called(ctx, resource, action)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Permission), args.Error(1)
}
func (m *MockPermissionRepo) ListByRole(ctx context.Context, roleID uuid.UUID) ([]models.Permission, error) {
	args := m.Called(ctx, roleID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Permission), args.Error(1)
}

func newTestRoleService(roleRepo *MockRoleRepo, rolePermRepo *MockRolePermRepo) *RoleService {
	return NewRoleService(roleRepo, rolePermRepo, new(MockUserRoleRepo), new(MockPermissionRepo))
}

// TestRoleService_List_PopulatesPermissionsPerRole pins a real live-found
// bug: List (what GET /roles -- the endpoint RolesPage actually uses --
// calls) never populated Permissions, so every role card always showed
// "0 permissions" and every checkbox in the permission matrix rendered
// unchecked, regardless of what role_permissions actually held. Confirmed
// against real Postgres: Admin had 24 real grants, the UI showed none.
func TestRoleService_List_PopulatesPermissionsPerRole(t *testing.T) {
	roleRepo := new(MockRoleRepo)
	rolePermRepo := new(MockRolePermRepo)
	svc := newTestRoleService(roleRepo, rolePermRepo)

	orgID := uuid.New()
	adminRole := uuid.New()
	viewerRole := uuid.New()
	readPerm := models.Permission{ID: uuid.New(), Resource: "incidents", Action: "read"}
	writePerm := models.Permission{ID: uuid.New(), Resource: "incidents", Action: "write"}

	roleRepo.On("List", mock.Anything, orgID, 1, 20).Return(
		[]models.Role{{ID: adminRole, Name: "Admin"}, {ID: viewerRole, Name: "Viewer"}}, 2, nil,
	)
	rolePermRepo.On("ListByRole", mock.Anything, adminRole).Return([]models.Permission{readPerm, writePerm}, nil)
	rolePermRepo.On("ListByRole", mock.Anything, viewerRole).Return([]models.Permission{readPerm}, nil)

	roles, total, err := svc.List(context.Background(), orgID, 1, 20)

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, roles, 2)
	assert.Len(t, roles[0].Permissions, 2)
	assert.Len(t, roles[1].Permissions, 1)
}

func TestRoleService_GetByID_PopulatesPermissions(t *testing.T) {
	roleRepo := new(MockRoleRepo)
	rolePermRepo := new(MockRolePermRepo)
	svc := newTestRoleService(roleRepo, rolePermRepo)

	roleID := uuid.New()
	perm := models.Permission{ID: uuid.New(), Resource: "software", Action: "write"}

	roleRepo.On("GetByID", mock.Anything, roleID).Return(&models.Role{ID: roleID, Name: "Operator"}, nil)
	rolePermRepo.On("ListByRole", mock.Anything, roleID).Return([]models.Permission{perm}, nil)

	role, err := svc.GetByID(context.Background(), roleID)

	require.NoError(t, err)
	require.Len(t, role.Permissions, 1)
	assert.Equal(t, "software", role.Permissions[0].Resource)
}

func TestRoleService_List_RepositoryErrorPropagates(t *testing.T) {
	roleRepo := new(MockRoleRepo)
	rolePermRepo := new(MockRolePermRepo)
	svc := newTestRoleService(roleRepo, rolePermRepo)

	orgID := uuid.New()
	roleRepo.On("List", mock.Anything, orgID, 1, 20).Return(nil, 0, assert.AnError)

	roles, total, err := svc.List(context.Background(), orgID, 1, 20)

	require.Error(t, err)
	assert.Nil(t, roles)
	assert.Equal(t, 0, total)
	rolePermRepo.AssertNotCalled(t, "ListByRole", mock.Anything, mock.Anything)
}

func TestRoleService_List_ToleratesPermissionLookupFailure(t *testing.T) {
	roleRepo := new(MockRoleRepo)
	rolePermRepo := new(MockRolePermRepo)
	svc := newTestRoleService(roleRepo, rolePermRepo)

	orgID := uuid.New()
	roleID := uuid.New()
	roleRepo.On("List", mock.Anything, orgID, 1, 20).Return([]models.Role{{ID: roleID, Name: "Viewer"}}, 1, nil)
	rolePermRepo.On("ListByRole", mock.Anything, roleID).Return(nil, assert.AnError)

	roles, total, err := svc.List(context.Background(), orgID, 1, 20)

	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, roles, 1)
	assert.Empty(t, roles[0].Permissions)
}
