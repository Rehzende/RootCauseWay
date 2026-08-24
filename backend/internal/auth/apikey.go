package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// APIKeyRepository defines DB operations for API keys.
type APIKeyRepository interface {
	Create(ctx context.Context, k *models.APIKey) error
	GetByPrefix(ctx context.Context, prefix string) (*models.APIKey, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.APIKey, int, error)
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
	Deactivate(ctx context.Context, id uuid.UUID) error
}

// APIKeyAuthenticator handles API key authentication.
type APIKeyAuthenticator struct {
	keyRepo      APIKeyRepository
	userRoleRepo UserRoleRepository
	permRepo     PermissionRepository
}

// NewAPIKeyAuthenticator creates a new APIKeyAuthenticator.
func NewAPIKeyAuthenticator(keyRepo APIKeyRepository, userRoleRepo UserRoleRepository, permRepo PermissionRepository) *APIKeyAuthenticator {
	return &APIKeyAuthenticator{
		keyRepo:      keyRepo,
		userRoleRepo: userRoleRepo,
		permRepo:     permRepo,
	}
}

// Authenticate validates an API key and returns the auth context.
func (a *APIKeyAuthenticator) Authenticate(ctx context.Context, keyString string) (*models.AuthContext, error) {
	if len(keyString) < 10 {
		return nil, fmt.Errorf("invalid API key format")
	}

	// Extract prefix: "rootcauseway_" + first 8 hex chars
	prefix := keyString[:13] // "rootcauseway_" (5) + 8 chars
	apiKey, err := a.keyRepo.GetByPrefix(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("API key not found")
	}

	if !apiKey.IsActive {
		return nil, fmt.Errorf("API key is deactivated")
	}

	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("API key has expired")
	}

	// Verify hash
	if err := bcrypt.CompareHashAndPassword([]byte(apiKey.KeyHash), []byte(keyString)); err != nil {
		return nil, fmt.Errorf("invalid API key")
	}

	// Update last used
	_ = a.keyRepo.UpdateLastUsed(ctx, apiKey.ID)

	// Build auth context
	authCtx := &models.AuthContext{
		UserID: apiKey.UserID,
		OrgID:  apiKey.OrgID,
	}

	// Load permissions from roles
	roles, err := a.userRoleRepo.ListByUser(ctx, apiKey.UserID)
	if err == nil {
		perms := make(map[string][]string)
		seen := make(map[string]bool)
		for _, role := range roles {
			authCtx.Roles = append(authCtx.Roles, role.Slug)
			rolePerms, err := a.permRepo.ListByRole(ctx, role.ID)
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
		authCtx.Permissions = perms
	}

	return authCtx, nil
}

// GenerateKey creates a new API key.
func (a *APIKeyAuthenticator) GenerateKey(ctx context.Context, orgID, userID uuid.UUID, req models.CreateAPIKeyRequest) (*models.APIKeyResponse, error) {
	// Generate random key: "rootcauseway_" + 48 random hex chars
	keyBytes := make([]byte, 24)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	plainKey := "rootcauseway_" + hex.EncodeToString(keyBytes)
	prefix := plainKey[:13]

	hash, err := bcrypt.GenerateFromPassword([]byte(plainKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash key: %w", err)
	}

	apiKey := &models.APIKey{
		ID:        uuid.New(),
		OrgID:     orgID,
		UserID:    userID,
		Name:      req.Name,
		KeyHash:   string(hash),
		KeyPrefix: prefix,
		RoleID:    req.RoleID,
		Scopes:    req.Scopes,
		ExpiresAt: req.ExpiresAt,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	if err := a.keyRepo.Create(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("failed to store key: %w", err)
	}

	return &models.APIKeyResponse{
		ID:        apiKey.ID,
		Name:      apiKey.Name,
		Key:       plainKey,
		KeyPrefix: prefix,
		Scopes:    apiKey.Scopes,
		ExpiresAt: apiKey.ExpiresAt,
		CreatedAt: apiKey.CreatedAt,
	}, nil
}
