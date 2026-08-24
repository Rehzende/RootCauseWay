package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/auth"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// --- Role Service ---

type RoleService struct {
	roleRepo     auth.RoleRepository
	rolePermRepo auth.RolePermissionRepository
	userRoleRepo auth.UserRoleRepository
	permRepo     auth.PermissionRepository
}

func NewRoleService(roleRepo auth.RoleRepository, rolePermRepo auth.RolePermissionRepository, userRoleRepo auth.UserRoleRepository, permRepo auth.PermissionRepository) *RoleService {
	return &RoleService{roleRepo: roleRepo, rolePermRepo: rolePermRepo, userRoleRepo: userRoleRepo, permRepo: permRepo}
}

// ListAllPermissions returns the full permission catalog (resource+action
// pairs) available to grant to a role -- distinct from ListPermissions
// above, which returns only the permissions a specific role already has.
// Found missing live: RolesPage's "grant permission" selector needs this
// catalog, but GET /permissions never existed end-to-end -- the repository
// method (PgPermissionRepository.List) was already there, wired only into
// the RBAC enforcer/API-key authenticator, never exposed via a service
// method or route.
func (s *RoleService) ListAllPermissions(ctx context.Context) ([]models.Permission, error) {
	return s.permRepo.List(ctx)
}

func (s *RoleService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateRoleRequest) (*models.Role, error) {
	role := &models.Role{
		ID:          uuid.New(),
		OrgID:       orgID,
		Name:        req.Name,
		Slug:        req.Slug,
		Description: req.Description,
		IsSystem:    false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.roleRepo.Create(ctx, role); err != nil {
		return nil, err
	}
	return role, nil
}

func (s *RoleService) GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error) {
	return s.roleRepo.GetByID(ctx, id)
}

func (s *RoleService) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.Role, int, error) {
	return s.roleRepo.List(ctx, orgID, page, perPage)
}

func (s *RoleService) Update(ctx context.Context, id uuid.UUID, req models.CreateRoleRequest) (*models.Role, error) {
	role, err := s.roleRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if role.IsSystem {
		return nil, fmt.Errorf("cannot modify system role")
	}
	role.Name = req.Name
	role.Slug = req.Slug
	role.Description = req.Description
	role.UpdatedAt = time.Now()
	if err := s.roleRepo.Update(ctx, role); err != nil {
		return nil, err
	}
	return role, nil
}

func (s *RoleService) Delete(ctx context.Context, id uuid.UUID) error {
	role, err := s.roleRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if role.IsSystem {
		return fmt.Errorf("cannot delete system role")
	}
	return s.roleRepo.Delete(ctx, id)
}

func (s *RoleService) GrantPermission(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return s.rolePermRepo.Grant(ctx, roleID, permissionID)
}

func (s *RoleService) RevokePermission(ctx context.Context, roleID, permissionID uuid.UUID) error {
	return s.rolePermRepo.Revoke(ctx, roleID, permissionID)
}

func (s *RoleService) ListPermissions(ctx context.Context, roleID uuid.UUID) ([]models.Permission, error) {
	return s.rolePermRepo.ListByRole(ctx, roleID)
}

func (s *RoleService) AssignUser(ctx context.Context, userID, roleID uuid.UUID) error {
	return s.userRoleRepo.Assign(ctx, userID, roleID)
}

func (s *RoleService) UnassignUser(ctx context.Context, userID, roleID uuid.UUID) error {
	return s.userRoleRepo.Unassign(ctx, userID, roleID)
}

func (s *RoleService) ListUserRoles(ctx context.Context, userID uuid.UUID) ([]models.Role, error) {
	return s.userRoleRepo.ListByUser(ctx, userID)
}

// --- SSO Provider Service ---

type SSOProviderService struct {
	repo auth.SSOProviderRepository
}

func NewSSOProviderService(repo auth.SSOProviderRepository) *SSOProviderService {
	return &SSOProviderService{repo: repo}
}

func (s *SSOProviderService) Create(ctx context.Context, orgID uuid.UUID, req models.CreateSSOProviderRequest) (*models.SSOProvider, error) {
	provider := &models.SSOProvider{
		ID:                 uuid.New(),
		OrgID:              orgID,
		Name:               req.Name,
		ProviderType:       req.ProviderType,
		ClientID:           req.ClientID,
		ClientSecret:       req.ClientSecret,
		IssuerURL:          req.IssuerURL,
		AuthorizationURL:   req.AuthorizationURL,
		TokenURL:           req.TokenURL,
		UserinfoURL:        req.UserinfoURL,
		Scopes:             req.Scopes,
		AutoProvisionUsers: req.AutoProvisionUsers,
		DefaultRoleID:      req.DefaultRoleID,
		Enabled:            true,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := s.repo.Create(ctx, provider); err != nil {
		return nil, err
	}
	return provider, nil
}

func (s *SSOProviderService) GetByID(ctx context.Context, id uuid.UUID) (*models.SSOProvider, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SSOProviderService) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.SSOProvider, int, error) {
	return s.repo.List(ctx, orgID, page, perPage)
}

func (s *SSOProviderService) Update(ctx context.Context, id uuid.UUID, req models.CreateSSOProviderRequest) (*models.SSOProvider, error) {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	provider.Name = req.Name
	provider.ProviderType = req.ProviderType
	provider.ClientID = req.ClientID
	if req.ClientSecret != "" {
		provider.ClientSecret = req.ClientSecret
	}
	provider.IssuerURL = req.IssuerURL
	provider.AuthorizationURL = req.AuthorizationURL
	provider.TokenURL = req.TokenURL
	provider.UserinfoURL = req.UserinfoURL
	provider.Scopes = req.Scopes
	provider.AutoProvisionUsers = req.AutoProvisionUsers
	provider.DefaultRoleID = req.DefaultRoleID
	provider.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, provider); err != nil {
		return nil, err
	}
	return provider, nil
}

func (s *SSOProviderService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

// --- API Key Service ---

type APIKeyService struct {
	authenticator *auth.APIKeyAuthenticator
	keyRepo       auth.APIKeyRepository
}

func NewAPIKeyService(authenticator *auth.APIKeyAuthenticator, keyRepo auth.APIKeyRepository) *APIKeyService {
	return &APIKeyService{authenticator: authenticator, keyRepo: keyRepo}
}

func (s *APIKeyService) Generate(ctx context.Context, orgID, userID uuid.UUID, req models.CreateAPIKeyRequest) (*models.APIKeyResponse, error) {
	return s.authenticator.GenerateKey(ctx, orgID, userID, req)
}

func (s *APIKeyService) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.APIKey, int, error) {
	return s.keyRepo.List(ctx, orgID, page, perPage)
}

func (s *APIKeyService) Revoke(ctx context.Context, id uuid.UUID) error {
	return s.keyRepo.Deactivate(ctx, id)
}

// --- Audit Service ---

type AuditService struct {
	repo AuditLogFullRepository
}

// AuditLogFullRepository extends the middleware interface with query methods.
type AuditLogFullRepository interface {
	Create(ctx context.Context, entry *models.AuditLogEntry) error
	List(ctx context.Context, orgID uuid.UUID, filter models.AuditLogFilter) ([]models.AuditLogEntry, int, error)
	Count(ctx context.Context, orgID uuid.UUID) (int, error)
}

func NewAuditService(repo AuditLogFullRepository) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) List(ctx context.Context, orgID uuid.UUID, filter models.AuditLogFilter) ([]models.AuditLogEntry, int, error) {
	return s.repo.List(ctx, orgID, filter)
}

// --- User Service ---

type UserService struct {
	userRepo     auth.UserRepository
	userRoleRepo auth.UserRoleRepository
}

func NewUserService(userRepo auth.UserRepository, userRoleRepo auth.UserRoleRepository) *UserService {
	return &UserService{userRepo: userRepo, userRoleRepo: userRoleRepo}
}

func (s *UserService) Create(ctx context.Context, orgID uuid.UUID, name, email, password string, roleIDs []uuid.UUID) (*models.UserWithRoles, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &models.UserWithRoles{
		User: models.User{
			ID:           uuid.New(),
			OrgID:        orgID,
			Name:         name,
			Email:        email,
			PasswordHash: string(hash),
			Role:         "viewer",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		IsActive: true,
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}
	for _, roleID := range roleIDs {
		_ = s.userRoleRepo.Assign(ctx, user.ID, roleID)
	}
	roles, _ := s.userRoleRepo.ListByUser(ctx, user.ID)
	user.Roles = roles
	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*models.UserWithRoles, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	roles, err := s.userRoleRepo.ListByUser(ctx, id)
	if err == nil {
		user.Roles = roles
	}
	return user, nil
}

func (s *UserService) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.UserWithRoles, int, error) {
	return s.userRepo.List(ctx, orgID, page, perPage)
}

func (s *UserService) Update(ctx context.Context, id uuid.UUID, u *models.UserWithRoles) (*models.UserWithRoles, error) {
	existing, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if u.Name != "" {
		existing.Name = u.Name
	}
	if u.Email != "" {
		existing.Email = u.Email
	}
	existing.IsActive = u.IsActive
	if err := s.userRepo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}

func (s *UserService) Delete(ctx context.Context, id uuid.UUID) error {
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	user.IsActive = false
	return s.userRepo.Update(ctx, user)
}
