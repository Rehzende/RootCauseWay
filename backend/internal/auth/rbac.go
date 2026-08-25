package auth

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// PermissionRepository defines DB operations for permissions.
type PermissionRepository interface {
	List(ctx context.Context) ([]models.Permission, error)
	GetByResourceAction(ctx context.Context, resource, action string) (*models.Permission, error)
	ListByRole(ctx context.Context, roleID uuid.UUID) ([]models.Permission, error)
}

// RolePermissionRepository defines DB operations for role-permission links.
type RolePermissionRepository interface {
	Grant(ctx context.Context, roleID, permissionID uuid.UUID) error
	Revoke(ctx context.Context, roleID, permissionID uuid.UUID) error
	ListByRole(ctx context.Context, roleID uuid.UUID) ([]models.Permission, error)
}

// RBACEnforcer provides RBAC enforcement.
type RBACEnforcer struct {
	roleRepo       RoleRepository
	permissionRepo PermissionRepository
	userRoleRepo   UserRoleRepository
}

// NewRBACEnforcer creates a new RBACEnforcer.
func NewRBACEnforcer(roleRepo RoleRepository, permissionRepo PermissionRepository, userRoleRepo UserRoleRepository) *RBACEnforcer {
	return &RBACEnforcer{
		roleRepo:       roleRepo,
		permissionRepo: permissionRepo,
		userRoleRepo:   userRoleRepo,
	}
}

// HasPermission checks if a user has a specific permission via any of their roles.
func (r *RBACEnforcer) HasPermission(ctx context.Context, userID uuid.UUID, resource, action string) (bool, error) {
	perms, err := r.GetUserPermissions(ctx, userID)
	if err != nil {
		return false, err
	}

	actions, ok := perms[resource]
	if !ok {
		return false, nil
	}
	for _, a := range actions {
		if a == action || a == "*" {
			return true, nil
		}
	}
	return false, nil
}

// GetUserPermissions returns all permissions for a user grouped by resource.
func (r *RBACEnforcer) GetUserPermissions(ctx context.Context, userID uuid.UUID) (map[string][]string, error) {
	roles, err := r.userRoleRepo.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	perms := make(map[string][]string)
	seen := make(map[string]bool)

	for _, role := range roles {
		rolePerms, err := r.permissionRepo.ListByRole(ctx, role.ID)
		if err != nil {
			continue
		}
		for _, p := range rolePerms {
			key := p.Resource + ":" + p.Action
			if !seen[key] {
				seen[key] = true
				perms[p.Resource] = append(perms[p.Resource], p.Action)
			}
		}
	}

	return perms, nil
}

// RequirePermission returns Gin middleware that checks a specific permission.
func (r *RBACEnforcer) RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		v, exists := c.Get("auth_context")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, models.ErrorResponse{Error: "authentication required"})
			return
		}

		authCtx, ok := v.(*models.AuthContext)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, models.ErrorResponse{Error: "invalid auth context"})
			return
		}

		// Check permissions from auth context
		actions, ok := authCtx.Permissions[resource]
		if ok {
			for _, a := range actions {
				if a == action || a == "*" {
					c.Next()
					return
				}
			}
		}

		// Check wildcard resource
		actions, ok = authCtx.Permissions["*"]
		if ok {
			for _, a := range actions {
				if a == action || a == "*" {
					c.Next()
					return
				}
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, models.ErrorResponse{Error: "insufficient permissions"})
	}
}

// RequireResourcePermission returns Gin middleware that requires
// <resource>:<action> where action is inferred from the HTTP method
// (GET/HEAD -> read, everything else -> write) -- the convention nearly
// every route in this app already follows, since the permission catalog
// only defines a "delete" action for the "incidents" resource and an
// "execute" action for "runbooks". Routes needing one of those two, or any
// other non-default action, use RequirePermission directly instead (see
// cmd/api/main.go's route registration for exactly which ones).
func (r *RBACEnforcer) RequireResourcePermission(resource string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Computed fresh per-request, not captured once at registration
		// time -- this handler is shared across every request through the
		// route, so a var declared outside this closure would leak its
		// value (e.g. "read" from a GET) into the next, unrelated request.
		action := "write"
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			action = "read"
		}
		r.RequirePermission(resource, action)(c)
	}
}
