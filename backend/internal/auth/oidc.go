package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// SSOProviderRepository defines DB operations for SSO providers.
type SSOProviderRepository interface {
	Create(ctx context.Context, p *models.SSOProvider) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.SSOProvider, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.SSOProvider, int, error)
	Update(ctx context.Context, p *models.SSOProvider) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// UserRepository defines DB operations for users.
type UserRepository interface {
	Create(ctx context.Context, u *models.UserWithRoles) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.UserWithRoles, error)
	GetByEmail(ctx context.Context, email string) (*models.UserWithRoles, error)
	GetBySSOSubject(ctx context.Context, provider, subject string) (*models.UserWithRoles, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.UserWithRoles, int, error)
	Update(ctx context.Context, u *models.UserWithRoles) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
}

// SessionRepository defines DB operations for sessions.
type SessionRepository interface {
	Create(ctx context.Context, s *models.Session) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.Session, error)
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteExpired(ctx context.Context) error
	DeleteByUser(ctx context.Context, userID uuid.UUID) error
}

// UserRoleRepository defines DB operations for user-role assignments.
type UserRoleRepository interface {
	Assign(ctx context.Context, userID, roleID uuid.UUID) error
	Unassign(ctx context.Context, userID, roleID uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Role, error)
}

// RoleRepository defines DB operations for roles.
type RoleRepository interface {
	Create(ctx context.Context, r *models.Role) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Role, error)
	List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.Role, int, error)
	Update(ctx context.Context, r *models.Role) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// OIDCAuthenticator handles OIDC authentication flows.
type OIDCAuthenticator struct {
	providerRepo SSOProviderRepository
	userRepo     UserRepository
	sessionRepo  SessionRepository
	userRoleRepo UserRoleRepository
	roleRepo     RoleRepository
	// stateStore maps state -> providerID for CSRF protection (in-memory for MVP)
	stateStore map[string]stateEntry
}

type stateEntry struct {
	providerID uuid.UUID
	orgID      uuid.UUID
	createdAt  time.Time
}

// oidcTokenResponse is the response from the OIDC token endpoint.
type oidcTokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// oidcUserInfo is the response from the OIDC userinfo endpoint.
type oidcUserInfo struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// NewOIDCAuthenticator creates a new OIDCAuthenticator.
func NewOIDCAuthenticator(
	providerRepo SSOProviderRepository,
	userRepo UserRepository,
	sessionRepo SessionRepository,
	userRoleRepo UserRoleRepository,
	roleRepo RoleRepository,
) *OIDCAuthenticator {
	return &OIDCAuthenticator{
		providerRepo: providerRepo,
		userRepo:     userRepo,
		sessionRepo:  sessionRepo,
		userRoleRepo: userRoleRepo,
		roleRepo:     roleRepo,
		stateStore:   make(map[string]stateEntry),
	}
}

// InitiateLogin returns the OIDC authorization URL and state.
func (a *OIDCAuthenticator) InitiateLogin(ctx context.Context, providerID uuid.UUID) (authURL, state string, err error) {
	provider, err := a.providerRepo.GetByID(ctx, providerID)
	if err != nil {
		return "", "", fmt.Errorf("provider not found: %w", err)
	}
	if !provider.Enabled {
		return "", "", fmt.Errorf("provider is disabled")
	}

	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate state: %w", err)
	}
	state = base64.URLEncoding.EncodeToString(stateBytes)

	a.stateStore[state] = stateEntry{
		providerID: provider.ID,
		orgID:      provider.OrgID,
		createdAt:  time.Now(),
	}

	scopes := provider.Scopes
	if scopes == "" {
		scopes = "openid email profile"
	}

	params := url.Values{
		"client_id":     {provider.ClientID},
		"response_type": {"code"},
		"scope":         {scopes},
		"state":         {state},
		"redirect_uri":  {fmt.Sprintf("/api/v1/auth/sso/%s/callback", provider.ID.String())},
	}

	authURL = provider.AuthorizationURL + "?" + params.Encode()
	return authURL, state, nil
}

// HandleCallback processes the OIDC callback, creates/updates user, returns session.
func (a *OIDCAuthenticator) HandleCallback(ctx context.Context, state, code, ipAddress, userAgent string) (*models.Session, *models.UserWithRoles, error) {
	entry, ok := a.stateStore[state]
	if !ok {
		return nil, nil, fmt.Errorf("invalid state")
	}
	delete(a.stateStore, state)

	if time.Since(entry.createdAt) > 10*time.Minute {
		return nil, nil, fmt.Errorf("state expired")
	}

	provider, err := a.providerRepo.GetByID(ctx, entry.providerID)
	if err != nil {
		return nil, nil, fmt.Errorf("provider not found: %w", err)
	}

	// Exchange code for tokens
	tokenResp, err := a.exchangeCode(provider, code)
	if err != nil {
		return nil, nil, fmt.Errorf("token exchange failed: %w", err)
	}

	// Get user info
	userInfo, err := a.getUserInfo(provider, tokenResp.AccessToken)
	if err != nil {
		return nil, nil, fmt.Errorf("userinfo failed: %w", err)
	}

	if userInfo.Email == "" {
		return nil, nil, fmt.Errorf("email not provided by OIDC provider")
	}

	// Find or create user
	providerName := provider.Name
	user, err := a.userRepo.GetBySSOSubject(ctx, providerName, userInfo.Sub)
	if err != nil {
		// Try by email
		user, err = a.userRepo.GetByEmail(ctx, userInfo.Email)
		if err != nil {
			// Auto-provision if enabled
			if !provider.AutoProvisionUsers {
				return nil, nil, fmt.Errorf("user not found and auto-provisioning is disabled")
			}

			user = &models.UserWithRoles{
				User: models.User{
					ID:    uuid.New(),
					OrgID: provider.OrgID,
					Name:  userInfo.Name,
					Email: userInfo.Email,
					Role:  "viewer",
				},
				SSOProvider: &providerName,
				SSOSubject:  userInfo.Sub,
				AvatarURL:   userInfo.Picture,
				IsActive:    true,
			}
			if userInfo.Name == "" {
				user.Name = userInfo.Email
			}

			if err := a.userRepo.Create(ctx, user); err != nil {
				return nil, nil, fmt.Errorf("failed to create user: %w", err)
			}

			// Assign default role
			if provider.DefaultRoleID != nil {
				_ = a.userRoleRepo.Assign(ctx, user.ID, *provider.DefaultRoleID)
			}
		} else {
			// Link SSO to existing user
			user.SSOProvider = &providerName
			user.SSOSubject = userInfo.Sub
			if userInfo.Picture != "" {
				user.AvatarURL = userInfo.Picture
			}
			_ = a.userRepo.Update(ctx, user)
		}
	}

	_ = a.userRepo.UpdateLastLogin(ctx, user.ID)

	// Create session
	session := &models.Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	if err := a.sessionRepo.Create(ctx, session); err != nil {
		return nil, nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Load roles
	roles, err := a.userRoleRepo.ListByUser(ctx, user.ID)
	if err == nil {
		user.Roles = roles
	}

	return session, user, nil
}

func (a *OIDCAuthenticator) exchangeCode(provider *models.SSOProvider, code string) (*oidcTokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {provider.ClientID},
		"client_secret": {provider.ClientSecret},
		"redirect_uri":  {fmt.Sprintf("/api/v1/auth/sso/%s/callback", provider.ID.String())},
	}

	resp, err := http.Post(provider.TokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp oidcTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, err
	}
	return &tokenResp, nil
}

func (a *OIDCAuthenticator) getUserInfo(provider *models.SSOProvider, accessToken string) (*oidcUserInfo, error) {
	userinfoURL := provider.UserinfoURL
	if userinfoURL == "" {
		userinfoURL = strings.TrimSuffix(provider.IssuerURL, "/") + "/userinfo"
	}

	req, err := http.NewRequest("GET", userinfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var info oidcUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}
