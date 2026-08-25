package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// --- RBAC Models ---

// Role represents a role in the RBAC system.
type Role struct {
	ID          uuid.UUID `json:"id"`
	OrgID       uuid.UUID `json:"org_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"is_system"`
	// Permissions is populated by RoleService.List/GetByID (via
	// RolePermissionRepository.ListByRole) -- never set by the repository
	// layer directly, since PgRoleRepository's queries only touch the
	// roles table itself.
	Permissions []Permission `json:"permissions,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// CreateRoleRequest is the request body for creating a role.
type CreateRoleRequest struct {
	Name        string `json:"name" binding:"required"`
	Slug        string `json:"slug" binding:"required"`
	Description string `json:"description"`
	// PermissionIDs: grant these permissions to the role at creation time.
	// Also accepted (but currently ignored) by UpdateRole, which reuses
	// this same request shape -- permission changes on an existing role go
	// through the dedicated Grant/RevokePermission endpoints instead,
	// same as the rest of the Permission Matrix UI.
	PermissionIDs []uuid.UUID `json:"permission_ids,omitempty"`
}

// Permission represents a granular permission.
type Permission struct {
	ID          uuid.UUID `json:"id"`
	Resource    string    `json:"resource"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
}

// RolePermission links a role to a permission.
type RolePermission struct {
	RoleID       uuid.UUID `json:"role_id"`
	PermissionID uuid.UUID `json:"permission_id"`
}

// UserRole links a user to a role.
type UserRole struct {
	UserID uuid.UUID `json:"user_id"`
	RoleID uuid.UUID `json:"role_id"`
}

// --- SSO Models ---

// SSOProvider represents an SSO/OIDC provider configuration.
type SSOProvider struct {
	ID                 uuid.UUID `json:"id"`
	OrgID              uuid.UUID `json:"org_id"`
	Name               string    `json:"name"`
	ProviderType       string    `json:"provider_type"` // oidc, saml
	ClientID           string    `json:"client_id"`
	ClientSecret       string    `json:"-"`
	IssuerURL          string    `json:"issuer_url"`
	AuthorizationURL   string    `json:"authorization_url"`
	TokenURL           string    `json:"token_url"`
	UserinfoURL        string    `json:"userinfo_url"`
	Scopes             string    `json:"scopes"`
	AutoProvisionUsers bool      `json:"auto_provision_users"`
	DefaultRoleID      *uuid.UUID `json:"default_role_id,omitempty"`
	Enabled            bool      `json:"enabled"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// CreateSSOProviderRequest is the request body for creating an SSO provider.
type CreateSSOProviderRequest struct {
	Name               string     `json:"name" binding:"required"`
	ProviderType       string     `json:"provider_type" binding:"required"`
	ClientID           string     `json:"client_id" binding:"required"`
	ClientSecret       string     `json:"client_secret" binding:"required"`
	IssuerURL          string     `json:"issuer_url"`
	AuthorizationURL   string     `json:"authorization_url" binding:"required"`
	TokenURL           string     `json:"token_url" binding:"required"`
	UserinfoURL        string     `json:"userinfo_url"`
	Scopes             string     `json:"scopes"`
	AutoProvisionUsers bool       `json:"auto_provision_users"`
	DefaultRoleID      *uuid.UUID `json:"default_role_id,omitempty"`
}

// --- API Key Models ---

// APIKey represents an API key for programmatic access.
type APIKey struct {
	ID         uuid.UUID        `json:"id"`
	OrgID      uuid.UUID        `json:"org_id"`
	UserID     uuid.UUID        `json:"user_id"`
	Name       string           `json:"name"`
	KeyHash    string           `json:"-"`
	KeyPrefix  string           `json:"key_prefix"`
	RoleID     *uuid.UUID       `json:"role_id,omitempty"`
	Scopes     json.RawMessage  `json:"scopes"`
	ExpiresAt  *time.Time       `json:"expires_at,omitempty"`
	LastUsedAt *time.Time       `json:"last_used_at,omitempty"`
	IsActive   bool             `json:"is_active"`
	CreatedAt  time.Time        `json:"created_at"`
}

// CreateAPIKeyRequest is the request body for creating an API key.
type CreateAPIKeyRequest struct {
	Name      string          `json:"name" binding:"required"`
	RoleID    *uuid.UUID      `json:"role_id,omitempty"`
	Scopes    json.RawMessage `json:"scopes"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
}

// APIKeyResponse is returned after creating an API key (includes plaintext key).
type APIKeyResponse struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Key       string          `json:"key,omitempty"` // Only set on create
	KeyPrefix string          `json:"key_prefix"`
	Scopes    json.RawMessage `json:"scopes"`
	ExpiresAt *time.Time      `json:"expires_at,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

// --- Audit Log Models ---

// AuditLogEntry represents an entry in the audit log.
type AuditLogEntry struct {
	ID           uuid.UUID       `json:"id"`
	OrgID        uuid.UUID       `json:"org_id"`
	UserID       *uuid.UUID      `json:"user_id,omitempty"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	ResourceID   string          `json:"resource_id"`
	Details      json.RawMessage `json:"details"`
	IPAddress    string          `json:"ip_address"`
	UserAgent    string          `json:"user_agent"`
	RequestID    string          `json:"request_id"`
	CreatedAt    time.Time       `json:"created_at"`
}

// AuditLogFilter contains filters for querying audit logs.
type AuditLogFilter struct {
	UserID       *uuid.UUID `json:"user_id,omitempty"`
	Action       string     `json:"action,omitempty"`
	ResourceType string     `json:"resource_type,omitempty"`
	From         *time.Time `json:"from,omitempty"`
	To           *time.Time `json:"to,omitempty"`
	Page         int        `json:"page"`
	PerPage      int        `json:"per_page"`
}

// --- Session Models ---

// Session represents an authenticated session.
type Session struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	TokenHash        string     `json:"-"`
	RefreshTokenHash string     `json:"-"`
	IPAddress        string     `json:"ip_address"`
	UserAgent        string     `json:"user_agent"`
	ExpiresAt        time.Time  `json:"expires_at"`
	CreatedAt        time.Time  `json:"created_at"`
}

// --- Auth Context ---

// AuthContext is the authentication context passed through middleware.
type AuthContext struct {
	UserID      uuid.UUID            `json:"user_id"`
	OrgID       uuid.UUID            `json:"org_id"`
	Roles       []string             `json:"roles"`
	Permissions map[string][]string  `json:"permissions"` // resource -> []action
}

// UserWithRoles extends User with role information.
type UserWithRoles struct {
	User
	SSOProvider *string    `json:"sso_provider,omitempty"`
	SSOSubject  string     `json:"sso_subject,omitempty"`
	AvatarURL   string     `json:"avatar_url,omitempty"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	IsActive    bool       `json:"is_active"`
	Roles       []Role     `json:"roles,omitempty"`
}

// GrantPermissionRequest is the request body for granting a permission to a role.
type GrantPermissionRequest struct {
	PermissionID uuid.UUID `json:"permission_id" binding:"required"`
}

// AssignRoleRequest is the request body for assigning a role to a user.
type AssignRoleRequest struct {
	RoleID uuid.UUID `json:"role_id" binding:"required"`
}
