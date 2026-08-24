package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/crypto"
)

// PgPipelineGateRepository persists the human-in-the-loop (HITL) pipeline
// gate state: the per-org enable/disable toggle (organizations table) and
// the per-incident awaiting-approval/approved bookkeeping (incidents table).
// It is intentionally standalone (not part of repositories.go) so it can be
// added without touching the shared incident/org repositories owned by
// other in-flight work.
//
// Also owns the org's Teams integration settings (see TeamsSettings) --
// grouped here rather than a new file/table since it's the same "org
// settings" surface as the HITL gate and LLM settings, all read/written
// through PATCH/GET /api/v1/organizations/:id/settings.
type PgPipelineGateRepository struct {
	pool   *pgxpool.Pool
	cipher crypto.Cipher
}

func NewPipelineGateRepository(pool *pgxpool.Pool, cipher ...crypto.Cipher) *PgPipelineGateRepository {
	return &PgPipelineGateRepository{pool: pool, cipher: pickCipher(cipher)}
}

// GetOrgHITLGateEnabled returns the org's pipeline_hitl_gate_enabled flag.
func (r *PgPipelineGateRepository) GetOrgHITLGateEnabled(ctx context.Context, orgID uuid.UUID) (bool, error) {
	var enabled bool
	err := r.pool.QueryRow(ctx,
		`SELECT pipeline_hitl_gate_enabled FROM organizations WHERE id = $1`, orgID,
	).Scan(&enabled)
	return enabled, err
}

// SetOrgHITLGateEnabled toggles the HITL gate for an org.
func (r *PgPipelineGateRepository) SetOrgHITLGateEnabled(ctx context.Context, orgID uuid.UUID, enabled bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE organizations SET pipeline_hitl_gate_enabled = $1, updated_at = NOW() WHERE id = $2`,
		enabled, orgID,
	)
	return err
}

// LLMSettings is the org-level default LLM provider/model config, set via
// the LLM & Tokens settings UI. Individual agents can still override the
// model/temperature via their a2a_agents.managed_config -- this is the
// fallback every agent call resolves against when no per-agent override is
// set (see Orchestrator._resolve_llm_config in agent-service).
type LLMSettings struct {
	ProviderType string `json:"provider_type"`
	BaseURL      string `json:"base_url"`
	Model        string `json:"model"`
	APIKeyRef    string `json:"api_key_ref"`
	// CredentialProviderID, when set, means APIKeyRef is a credential_path
	// to resolve through that credential_providers row (see
	// CredentialLeaseService.resolveCredentialData) rather than the
	// literal API key value. nil preserves the original literal-ref
	// behavior for orgs that haven't configured a provider.
	CredentialProviderID *uuid.UUID `json:"credential_provider_id,omitempty"`
}

// GetOrgLLMSettings returns the org's default LLM provider settings.
func (r *PgPipelineGateRepository) GetOrgLLMSettings(ctx context.Context, orgID uuid.UUID) (LLMSettings, error) {
	var s LLMSettings
	err := r.pool.QueryRow(ctx,
		`SELECT default_llm_provider_type, default_llm_base_url, default_llm_model, default_llm_api_key_ref, default_llm_credential_provider_id
		 FROM organizations WHERE id = $1`, orgID,
	).Scan(&s.ProviderType, &s.BaseURL, &s.Model, &s.APIKeyRef, &s.CredentialProviderID)
	return s, err
}

// SetOrgLLMSettings updates the org's default LLM provider settings.
func (r *PgPipelineGateRepository) SetOrgLLMSettings(ctx context.Context, orgID uuid.UUID, s LLMSettings) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE organizations
		 SET default_llm_provider_type = $1, default_llm_base_url = $2, default_llm_model = $3, default_llm_api_key_ref = $4, default_llm_credential_provider_id = $5, updated_at = NOW()
		 WHERE id = $6`,
		s.ProviderType, s.BaseURL, s.Model, s.APIKeyRef, s.CredentialProviderID, orgID,
	)
	return err
}

// TeamsSettings is the org-level Microsoft Teams (Graph API) integration
// config, set via the Integrations settings UI. ClientSecret and
// RefreshToken are always the plaintext values on this struct --
// encryption/decryption happens inside Get/SetOrgTeamsSettings below, so
// callers (handlers, WarRoomService's client resolver) never handle the
// encrypted envelope directly.
//
// RefreshToken is what actually authenticates Graph calls now (delegated
// OAuth, see teams.NewGraphTeamsClientDelegated) -- ClientSecret is still
// needed too, since it's the org's app registration secret used to exchange/
// refresh that token, not a leftover from the old app-only flow.
// ConnectedAccountEmail is read-only/informational, auto-populated from
// Graph /me right after the OAuth connect flow completes -- never
// user-typed (unlike the app-only flow's old, manually-typed
// OrganizerUserID, which this field replaces).
type TeamsSettings struct {
	TenantID              string `json:"tenant_id"`
	ClientID              string `json:"client_id"`
	ClientSecret          string `json:"-"`
	RefreshToken          string `json:"-"`
	ConnectedAccountEmail string `json:"connected_account_email"`
}

// Configured reports whether every field needed for a real, working
// delegated Graph API client is set -- including RefreshToken, which only
// exists once someone has completed the OAuth connect flow. Tenant/client/
// secret alone (the app registration, configurable before connecting) are
// not enough on their own.
func (s TeamsSettings) Configured() bool {
	return s.TenantID != "" && s.ClientID != "" && s.ClientSecret != "" && s.RefreshToken != ""
}

// GetOrgTeamsSettings returns the org's Teams integration settings, with
// ClientSecret and RefreshToken already decrypted.
func (r *PgPipelineGateRepository) GetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID) (TeamsSettings, error) {
	var s TeamsSettings
	var encSecret, encRefreshToken string
	err := r.pool.QueryRow(ctx,
		`SELECT teams_tenant_id, teams_client_id, teams_client_secret_encrypted, teams_refresh_token_encrypted, teams_connected_account_email
		 FROM organizations WHERE id = $1`, orgID,
	).Scan(&s.TenantID, &s.ClientID, &encSecret, &encRefreshToken, &s.ConnectedAccountEmail)
	if err != nil {
		return s, err
	}
	if encSecret != "" {
		plain, err := r.cipher.Decrypt(encSecret)
		if err != nil {
			return s, fmt.Errorf("decrypt teams client secret: %w", err)
		}
		s.ClientSecret = plain
	}
	if encRefreshToken != "" {
		plain, err := r.cipher.Decrypt(encRefreshToken)
		if err != nil {
			return s, fmt.Errorf("decrypt teams refresh token: %w", err)
		}
		s.RefreshToken = plain
	}
	return s, nil
}

// SetOrgTeamsSettings updates the org's Teams integration settings,
// encrypting ClientSecret and RefreshToken before they touch the database.
func (r *PgPipelineGateRepository) SetOrgTeamsSettings(ctx context.Context, orgID uuid.UUID, s TeamsSettings) error {
	encSecret := ""
	if s.ClientSecret != "" {
		enc, err := r.cipher.Encrypt(s.ClientSecret)
		if err != nil {
			return fmt.Errorf("encrypt teams client secret: %w", err)
		}
		encSecret = enc
	}
	encRefreshToken := ""
	if s.RefreshToken != "" {
		enc, err := r.cipher.Encrypt(s.RefreshToken)
		if err != nil {
			return fmt.Errorf("encrypt teams refresh token: %w", err)
		}
		encRefreshToken = enc
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE organizations
		 SET teams_tenant_id = $1, teams_client_id = $2, teams_client_secret_encrypted = $3, teams_refresh_token_encrypted = $4, teams_connected_account_email = $5, updated_at = NOW()
		 WHERE id = $6`,
		s.TenantID, s.ClientID, encSecret, encRefreshToken, s.ConnectedAccountEmail, orgID,
	)
	return err
}

// UpdateTeamsRefreshToken persists a rotated refresh token in isolation --
// used only by the delegated Graph client's token provider right after a
// successful refresh grant (Microsoft rotates the refresh token on every
// use). Deliberately narrower than SetOrgTeamsSettings: that method does a
// full read-then-overwrite of every Teams field, which risks clobbering a
// concurrent settings edit (e.g. someone re-saving the client secret) with
// stale data if both happen close together. This is a single encrypted
// column UPDATE, nothing else touched.
func (r *PgPipelineGateRepository) UpdateTeamsRefreshToken(ctx context.Context, orgID uuid.UUID, newRefreshToken string) error {
	enc, err := r.cipher.Encrypt(newRefreshToken)
	if err != nil {
		return fmt.Errorf("encrypt teams refresh token: %w", err)
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE organizations SET teams_refresh_token_encrypted = $1, updated_at = NOW() WHERE id = $2`,
		enc, orgID,
	)
	return err
}

// DisconnectTeams clears the connected account (refresh token + connected
// email) in isolation -- same narrow single-column-set UPDATE shape as
// UpdateTeamsRefreshToken above, for the same reason: a full
// SetOrgTeamsSettings read-then-overwrite risks clobbering a concurrent
// settings edit. Deliberately leaves tenant_id/client_id/
// client_secret_encrypted untouched -- disconnecting the account shouldn't
// force re-entering the Azure AD app registration to reconnect later.
// Before this existed, the only way to clear a stale/wrong connection was a
// direct UPDATE against Postgres (see project backlog).
func (r *PgPipelineGateRepository) DisconnectTeams(ctx context.Context, orgID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE organizations SET teams_refresh_token_encrypted = '', teams_connected_account_email = '', updated_at = NOW() WHERE id = $1`,
		orgID,
	)
	return err
}

// MarkAwaitingApproval records that the incident's pipeline is paused before
// the given stage, waiting for a human to approve it.
func (r *PgPipelineGateRepository) MarkAwaitingApproval(ctx context.Context, incidentID uuid.UUID, stage string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE incidents SET awaiting_approval_stage = $1, updated_at = NOW() WHERE id = $2`,
		stage, incidentID,
	)
	return err
}

// ApproveStage clears the awaiting-approval state for an incident and
// records who approved it. It returns the incident's org_id (needed to
// publish the resulting event on the right org channel) and the stage that
// was approved. If the incident isn't currently awaiting approval, it
// returns pgx.ErrNoRows.
func (r *PgPipelineGateRepository) ApproveStage(
	ctx context.Context, incidentID uuid.UUID, approvedBy uuid.UUID,
) (orgID uuid.UUID, stage string, err error) {
	now := time.Now()
	err = r.pool.QueryRow(ctx,
		`WITH prior AS (
			SELECT id, org_id, awaiting_approval_stage
			FROM incidents
			WHERE id = $1
			FOR UPDATE
		)
		UPDATE incidents i
		SET awaiting_approval_stage = NULL,
		    approved_by = $2,
		    approved_at = $3,
		    updated_at = $3
		FROM prior
		WHERE i.id = prior.id AND prior.awaiting_approval_stage IS NOT NULL
		RETURNING prior.org_id, prior.awaiting_approval_stage`,
		incidentID, approvedBy, now,
	).Scan(&orgID, &stage)
	return orgID, stage, err
}
