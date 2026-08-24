package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/Rehzende/RootCauseway/backend/internal/database"
	"github.com/Rehzende/RootCauseway/backend/internal/integrations/teams"
)

// teamsOAuthSettings is the read+write surface TeamsOAuthService needs from
// org settings storage. Narrower than the full PgPipelineGateRepository,
// same style as TeamsSettingsReader/TeamsRefreshTokenUpdater above.
type teamsOAuthSettings interface {
	GetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID) (database.TeamsSettings, error)
	SetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID, s database.TeamsSettings) error
	DisconnectTeams(ctx context.Context, orgID uuid.UUID) error
}

type teamsOAuthStateEntry struct {
	orgID     uuid.UUID
	createdAt time.Time
}

// TeamsOAuthService runs the delegated OAuth connect flow that replaces the
// old app-only Teams integration (see migration 027_teams_oauth and
// teams.NewGraphTeamsClientDelegated for why: app-only auth needs a tenant
// admin to grant a Microsoft Application Access Policy via PowerShell,
// delegated auth as a connected service/bot account needs neither).
//
// Structurally modeled on auth.OIDCAuthenticator's InitiateLogin/
// HandleCallback (same state-store-then-exchange-code shape), but kept
// separate rather than sharing its state store: this one is single-provider
// (Microsoft Graph, not multi-IdP SSO) and org-scoped rather than
// provider-scoped, and it additionally needs to persist a rotating refresh
// token going forward -- different enough lifecycle that merging the two
// would couple unrelated features.
type TeamsOAuthService struct {
	settings    teamsOAuthSettings
	redirectURI string // absolute URL, must match the app registration's registered redirect URI

	mu         sync.Mutex
	stateStore map[string]teamsOAuthStateEntry
}

// NewTeamsOAuthService builds a TeamsOAuthService. redirectURI must be the
// full absolute callback URL (e.g.
// "https://api.rootcauseway.example.com/api/v1/integrations/teams/oauth/callback"),
// not a relative path -- Microsoft requires an absolute, exactly-matching
// redirect_uri at both the authorize and token-exchange steps.
func NewTeamsOAuthService(settings teamsOAuthSettings, redirectURI string) *TeamsOAuthService {
	return &TeamsOAuthService{
		settings:    settings,
		redirectURI: redirectURI,
		stateStore:  make(map[string]teamsOAuthStateEntry),
	}
}

// InitiateConnect builds the Microsoft authorize URL for orgID's Teams
// connect flow. The org must already have tenant_id/client_id/client_secret
// saved (its Azure AD app registration) -- this doesn't create one, it just
// starts the delegated-auth handshake against it.
func (s *TeamsOAuthService) InitiateConnect(ctx context.Context, orgID uuid.UUID) (authorizeURL string, err error) {
	current, err := s.settings.GetOrgTeamsSettings(ctx, orgID)
	if err != nil {
		return "", fmt.Errorf("load org Teams settings: %w", err)
	}
	if current.TenantID == "" || current.ClientID == "" || current.ClientSecret == "" {
		return "", fmt.Errorf("configure Teams tenant ID, client ID and client secret before connecting an account")
	}

	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	state := base64.URLEncoding.EncodeToString(stateBytes)

	s.mu.Lock()
	s.stateStore[state] = teamsOAuthStateEntry{orgID: orgID, createdAt: time.Now()}
	s.mu.Unlock()

	return teams.BuildAuthorizeURL(current.TenantID, current.ClientID, s.redirectURI, state), nil
}

// HandleCallback validates the state token, exchanges the authorization
// code for tokens, records which account connected, and persists the
// refresh token (encrypted) against the org the state was issued for. It
// returns that orgID so the caller (the public callback handler) can decide
// where to redirect the browser back to even on a partial failure.
func (s *TeamsOAuthService) HandleCallback(ctx context.Context, state, code string) (orgID uuid.UUID, err error) {
	s.mu.Lock()
	entry, ok := s.stateStore[state]
	if ok {
		delete(s.stateStore, state) // single-use, regardless of outcome below
	}
	s.mu.Unlock()

	if !ok {
		return uuid.Nil, fmt.Errorf("invalid or already-used state")
	}
	if time.Since(entry.createdAt) > 10*time.Minute {
		return entry.orgID, fmt.Errorf("state expired, please try connecting again")
	}

	current, err := s.settings.GetOrgTeamsSettings(ctx, entry.orgID)
	if err != nil {
		return entry.orgID, fmt.Errorf("load org Teams settings: %w", err)
	}
	if current.TenantID == "" || current.ClientID == "" || current.ClientSecret == "" {
		return entry.orgID, fmt.Errorf("org Teams settings changed mid-flow, please try connecting again")
	}

	accessToken, refreshToken, err := teams.ExchangeCode(ctx, current.TenantID, current.ClientID, current.ClientSecret, code, s.redirectURI)
	if err != nil {
		return entry.orgID, fmt.Errorf("exchange authorization code: %w", err)
	}

	connectedAccountEmail, err := teams.FetchMe(ctx, accessToken)
	if err != nil {
		// Not fatal -- the refresh token is still good and the integration
		// will work; only the "Connected as: ..." display label is
		// unknown. Persist what we have rather than losing the connect.
		connectedAccountEmail = ""
	}

	current.RefreshToken = refreshToken
	current.ConnectedAccountEmail = connectedAccountEmail
	if err := s.settings.SetOrgTeamsSettings(ctx, entry.orgID, current); err != nil {
		return entry.orgID, fmt.Errorf("save Teams connection: %w", err)
	}

	return entry.orgID, nil
}

// Disconnect clears orgID's connected Teams account (refresh token +
// connected email), leaving the saved tenant_id/client_id/client_secret in
// place so reconnecting later doesn't require re-entering the Azure AD app
// registration. Idempotent -- disconnecting an already-disconnected org is
// not an error.
func (s *TeamsOAuthService) Disconnect(ctx context.Context, orgID uuid.UUID) error {
	return s.settings.DisconnectTeams(ctx, orgID)
}
