package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePermissionRepo is a minimal auth.PermissionRepository double -- the
// full RoleService constructor also needs auth.RoleRepository/
// RolePermissionRepository/UserRoleRepository, which these tests don't
// exercise, so they're left nil (fine as long as the test only calls
// methods that route through permRepo).
type fakePermissionRepo struct {
	items []models.Permission
	err   error
}

func (f *fakePermissionRepo) List(ctx context.Context) ([]models.Permission, error) {
	return f.items, f.err
}
func (f *fakePermissionRepo) GetByResourceAction(ctx context.Context, resource, action string) (*models.Permission, error) {
	return nil, nil
}
func (f *fakePermissionRepo) ListByRole(ctx context.Context, roleID uuid.UUID) ([]models.Permission, error) {
	return nil, nil
}

// TestRoleService_ListAllPermissions guards a capability found missing
// live: RolesPage's "grant permission" selector needs the full permission
// catalog, but no service method exposed PgPermissionRepository.List at
// all before this -- only ListPermissions(roleID), which returns a
// specific role's already-granted permissions, existed.
func TestRoleService_ListAllPermissions(t *testing.T) {
	repo := &fakePermissionRepo{items: []models.Permission{
		{ID: uuid.New(), Resource: "incidents", Action: "read"},
		{ID: uuid.New(), Resource: "incidents", Action: "write"},
	}}
	svc := NewRoleService(nil, nil, nil, repo)

	items, err := svc.ListAllPermissions(context.Background())

	require.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "incidents", items[0].Resource)
}
