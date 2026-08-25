package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/auth"
	"github.com/Rehzende/RootCauseway/backend/internal/middleware"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/Rehzende/RootCauseway/backend/internal/services"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles authentication and authorization endpoints.
type AuthHandler struct {
	RoleSvc        *services.RoleService
	SSOProviderSvc *services.SSOProviderService
	APIKeySvc      *services.APIKeyService
	AuditSvc       *services.AuditService
	UserSvc        *services.UserService
	OIDCAuth       *auth.OIDCAuthenticator
	RBAC           *auth.RBACEnforcer
	JWTSecret      string
	UserRepo       auth.UserRepository
}

// --- Login (password) ---

func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid request body"})
		return
	}
	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "email and password are required"})
		return
	}

	user, err := h.UserRepo.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid credentials"})
		return
	}

	if !user.IsActive {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "account is disabled"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "invalid credentials"})
		return
	}

	_ = h.UserRepo.UpdateLastLogin(c.Request.Context(), user.ID)

	claims := middleware.Claims{
		UserID: user.ID,
		OrgID:  user.OrgID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenStr,
		"user":  user,
	})
}

// --- SSO ---

func (h *AuthHandler) SSOLogin(c *gin.Context) {
	providerID, err := uuid.Parse(c.Param("provider"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid provider id"})
		return
	}

	authURL, _, err := h.OIDCAuth.InitiateLogin(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

func (h *AuthHandler) SSOCallback(c *gin.Context) {
	state := c.Query("state")
	code := c.Query("code")
	if state == "" || code == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "missing state or code"})
		return
	}

	session, user, err := h.OIDCAuth.HandleCallback(c.Request.Context(), state, code, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Generate JWT for the SSO user
	claims := middleware.Claims{
		UserID: user.ID,
		OrgID:  user.OrgID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(session.ExpiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenStr,
		"user":  user,
	})
}

// --- Logout ---

func (h *AuthHandler) Logout(c *gin.Context) {
	// For JWT-based auth, logout is client-side (discard token).
	// We could add token blacklisting later.
	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// --- Me ---

func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	uid, ok := userID.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{Error: "not authenticated"})
		return
	}

	user, err := h.UserSvc.GetByID(c.Request.Context(), uid)
	if err != nil {
		handleDBError(c, err, "user")
		return
	}

	// Load permissions
	perms, _ := h.RBAC.GetUserPermissions(c.Request.Context(), uid)

	c.JSON(http.StatusOK, gin.H{
		"user":        user,
		"permissions": perms,
	})
}

// --- API Keys ---

func (h *AuthHandler) CreateAPIKey(c *gin.Context) {
	var req models.CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	uid := userID.(uuid.UUID)
	orgID := getOrgID(c)

	resp, err := h.APIKeySvc.Generate(c.Request.Context(), orgID, uid, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) ListAPIKeys(c *gin.Context) {
	page, perPage := getPagination(c)
	items, total, err := h.APIKeySvc.List(c.Request.Context(), getOrgID(c), page, perPage)
	if err != nil {
		handleDBError(c, err, "api_key")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *AuthHandler) RevokeAPIKey(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.APIKeySvc.Revoke(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "api_key")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Users ---

func (h *AuthHandler) CreateUser(c *gin.Context) {
	var req struct {
		Name     string      `json:"name" binding:"required"`
		Email    string      `json:"email" binding:"required"`
		Password string      `json:"password" binding:"required"`
		RoleIDs  []uuid.UUID `json:"role_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	user, err := h.UserSvc.Create(c.Request.Context(), getOrgID(c), req.Name, req.Email, req.Password, req.RoleIDs)
	if err != nil {
		handleDBError(c, err, "user")
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (h *AuthHandler) ListUsers(c *gin.Context) {
	page, perPage := getPagination(c)
	items, total, err := h.UserSvc.List(c.Request.Context(), getOrgID(c), page, perPage)
	if err != nil {
		handleDBError(c, err, "user")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *AuthHandler) GetUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	user, err := h.UserSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "user")
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) UpdateUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.UserWithRoles
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	user, err := h.UserSvc.Update(c.Request.Context(), id, &req)
	if err != nil {
		handleDBError(c, err, "user")
		return
	}
	c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) DeleteUser(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.UserSvc.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "user")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Permissions ---

// ListPermissions returns the full permission catalog -- backs the "grant
// permission to this role" selector in RolesPage. Found missing live: the
// grant/revoke endpoints below already worked, but nothing populated the
// catalog to choose from.
func (h *AuthHandler) ListPermissions(c *gin.Context) {
	items, err := h.RoleSvc.ListAllPermissions(c.Request.Context())
	if err != nil {
		handleDBError(c, err, "permission")
		return
	}
	c.JSON(http.StatusOK, items)
}

// --- Roles ---

func (h *AuthHandler) ListRoles(c *gin.Context) {
	page, perPage := getPagination(c)
	items, total, err := h.RoleSvc.List(c.Request.Context(), getOrgID(c), page, perPage)
	if err != nil {
		handleDBError(c, err, "role")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *AuthHandler) CreateRole(c *gin.Context) {
	var req models.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	role, err := h.RoleSvc.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "role")
		return
	}
	c.JSON(http.StatusCreated, role)
}

func (h *AuthHandler) GetRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	role, err := h.RoleSvc.GetByID(c.Request.Context(), id)
	if err != nil {
		handleDBError(c, err, "role")
		return
	}
	c.JSON(http.StatusOK, role)
}

func (h *AuthHandler) UpdateRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	role, err := h.RoleSvc.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "role")
		return
	}
	c.JSON(http.StatusOK, role)
}

func (h *AuthHandler) DeleteRole(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.RoleSvc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Role Permissions ---

func (h *AuthHandler) GrantPermission(c *gin.Context) {
	roleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.GrantPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.RoleSvc.GrantPermission(c.Request.Context(), roleID, req.PermissionID); err != nil {
		handleDBError(c, err, "role_permission")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) RevokePermission(c *gin.Context) {
	roleID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	permID, err := uuid.Parse(c.Param("permId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid permId"})
		return
	}
	if err := h.RoleSvc.RevokePermission(c.Request.Context(), roleID, permID); err != nil {
		handleDBError(c, err, "role_permission")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- User Roles ---

func (h *AuthHandler) AssignRole(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	if err := h.RoleSvc.AssignUser(c.Request.Context(), userID, req.RoleID); err != nil {
		handleDBError(c, err, "user_role")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) UnassignRole(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	roleID, err := uuid.Parse(c.Param("roleId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid roleId"})
		return
	}
	if err := h.RoleSvc.UnassignUser(c.Request.Context(), userID, roleID); err != nil {
		handleDBError(c, err, "user_role")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- SSO Providers ---

func (h *AuthHandler) ListSSOProviders(c *gin.Context) {
	page, perPage := getPagination(c)
	items, total, err := h.SSOProviderSvc.List(c.Request.Context(), getOrgID(c), page, perPage)
	if err != nil {
		handleDBError(c, err, "sso_provider")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

func (h *AuthHandler) CreateSSOProvider(c *gin.Context) {
	var req models.CreateSSOProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	provider, err := h.SSOProviderSvc.Create(c.Request.Context(), getOrgID(c), req)
	if err != nil {
		handleDBError(c, err, "sso_provider")
		return
	}
	c.JSON(http.StatusCreated, provider)
}

func (h *AuthHandler) UpdateSSOProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	var req models.CreateSSOProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}
	provider, err := h.SSOProviderSvc.Update(c.Request.Context(), id, req)
	if err != nil {
		handleDBError(c, err, "sso_provider")
		return
	}
	c.JSON(http.StatusOK, provider)
}

func (h *AuthHandler) DeleteSSOProvider(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: "invalid id"})
		return
	}
	if err := h.SSOProviderSvc.Delete(c.Request.Context(), id); err != nil {
		handleDBError(c, err, "sso_provider")
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Audit Log ---

func (h *AuthHandler) ListAuditLog(c *gin.Context) {
	page, perPage := getPagination(c)
	filter := models.AuditLogFilter{
		Action:       c.Query("action"),
		ResourceType: c.Query("resource_type"),
		Page:         page,
		PerPage:      perPage,
	}

	if uid := c.Query("user_id"); uid != "" {
		if id, err := uuid.Parse(uid); err == nil {
			filter.UserID = &id
		}
	}
	if from := c.Query("from"); from != "" {
		if t, err := time.Parse(time.RFC3339, from); err == nil {
			filter.From = &t
		}
	}
	if to := c.Query("to"); to != "" {
		if t, err := time.Parse(time.RFC3339, to); err == nil {
			filter.To = &t
		}
	}

	items, total, err := h.AuditSvc.List(c.Request.Context(), getOrgID(c), filter)
	if err != nil {
		handleDBError(c, err, "audit_log")
		return
	}
	c.JSON(http.StatusOK, models.PaginatedResponse{Data: items, Total: total, Page: page, PerPage: perPage})
}

// SeedDefaultRoles creates system roles if they don't exist.
func SeedDefaultRoles(ctx context.Context, roleRepo auth.RoleRepository, permRepo auth.PermissionRepository, rolePermRepo auth.RolePermissionRepository, orgID uuid.UUID) error {
	type roleDef struct {
		Name, Slug, Description string
		Permissions              []string // "resource:action" or "*:*"
	}
	roles := []roleDef{
		{"Admin", "admin", "Full access to all resources", []string{"*:*"}},
		// Operational/day-to-day resources only -- same tier the 032
		// migration applies to existing orgs' Operator role. No users,
		// roles, credentials, audit, or settings: those stay Admin-only.
		{"Operator", "operator", "Manage incidents, software, agents, and runbooks", []string{
			"incidents:read", "incidents:write", "incidents:delete",
			"software:read", "software:write",
			"agents:read", "agents:write",
			"runbooks:read", "runbooks:write", "runbooks:execute",
			"webhooks:read", "webhooks:write",
			"knowledge_base:read", "knowledge_base:write",
			"marketplace:read", "marketplace:write",
			"observability:read", "observability:write",
			"slo:read", "slo:write",
		}},
		// Read-only, and narrowly scoped to the incident-facing/reporting
		// surface only: incidents, knowledge base, analytics. Not
		// administration (users, roles, credentials, audit, settings --
		// see migration 033), and not catalog/integrations either
		// (software, runbooks, slo, agents, webhooks, observability,
		// marketplace -- see migration 034). A reader needs to see what
		// happened, not manage the catalog of things that can happen or
		// how alerts get wired in.
		{"Viewer", "viewer", "Read-only access to incidents, knowledge base, and analytics", []string{
			"incidents:read", "knowledge_base:read", "analytics:read",
		}},
	}

	allPerms, _ := permRepo.List(ctx)
	permMap := make(map[string]uuid.UUID)
	for _, p := range allPerms {
		permMap[p.Resource+":"+p.Action] = p.ID
	}

	for _, rd := range roles {
		existing, err := roleRepo.GetBySlug(ctx, orgID, rd.Slug)
		if err == nil && existing != nil {
			continue
		}

		role := &models.Role{
			ID:          uuid.New(),
			OrgID:       orgID,
			Name:        rd.Name,
			Slug:        rd.Slug,
			Description: rd.Description,
			IsSystem:    true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := roleRepo.Create(ctx, role); err != nil {
			continue
		}

		for _, permKey := range rd.Permissions {
			if permKey == "*:*" {
				// Grant all permissions
				for _, pid := range permMap {
					_ = rolePermRepo.Grant(ctx, role.ID, pid)
				}
			} else if pid, ok := permMap[permKey]; ok {
				_ = rolePermRepo.Grant(ctx, role.ID, pid)
			}
		}
	}
	return nil
}
