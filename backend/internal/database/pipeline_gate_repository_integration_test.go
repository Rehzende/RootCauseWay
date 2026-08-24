//go:build integration

package database

import (
	"context"
	"testing"

	"github.com/Rehzende/RootCauseway/backend/internal/crypto"
)

// TestPgPipelineGateRepository_TeamsSettings_EncryptsSecretAtRest covers
// the security-relevant half of the Teams integration settings feature:
// ClientSecret and RefreshToken must round-trip correctly through
// Get/SetOrgTeamsSettings, and what's actually stored in
// teams_client_secret_encrypted/teams_refresh_token_encrypted must not be
// the plaintext value -- callers only ever see plaintext, the DB only ever
// sees the envelope.
func TestPgPipelineGateRepository_TeamsSettings_EncryptsSecretAtRest(t *testing.T) {
	pool := setupTestDB(t)
	orgID := createTestOrg(t, pool)

	cipher, err := crypto.New("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=") // 32 zero bytes, base64
	if err != nil {
		t.Fatalf("failed to build test cipher: %v", err)
	}
	repo := NewPipelineGateRepository(pool, cipher)
	ctx := context.Background()

	want := TeamsSettings{
		TenantID:              "tenant-123",
		ClientID:              "client-456",
		ClientSecret:          "super-secret-app-password",
		RefreshToken:          "super-secret-refresh-token",
		ConnectedAccountEmail: "rootcauseway-bot@example.com",
	}
	if err := repo.SetOrgTeamsSettings(ctx, orgID, want); err != nil {
		t.Fatalf("SetOrgTeamsSettings failed: %v", err)
	}

	got, err := repo.GetOrgTeamsSettings(ctx, orgID)
	if err != nil {
		t.Fatalf("GetOrgTeamsSettings failed: %v", err)
	}
	if got != want {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, want)
	}
	if !got.Configured() {
		t.Fatalf("expected Configured() to be true once tenant/client/secret/refresh_token are all set")
	}

	var rawSecret, rawRefreshToken string
	if err := pool.QueryRow(ctx,
		`SELECT teams_client_secret_encrypted, teams_refresh_token_encrypted FROM organizations WHERE id = $1`, orgID,
	).Scan(&rawSecret, &rawRefreshToken); err != nil {
		t.Fatalf("failed to read raw stored values: %v", err)
	}
	if rawSecret == want.ClientSecret {
		t.Fatalf("client secret stored in plaintext at rest, expected the encrypted envelope")
	}
	if !crypto.IsEncrypted(rawSecret) {
		t.Fatalf("stored client secret doesn't carry the encryption envelope prefix: %q", rawSecret)
	}
	if rawRefreshToken == want.RefreshToken {
		t.Fatalf("refresh token stored in plaintext at rest, expected the encrypted envelope")
	}
	if !crypto.IsEncrypted(rawRefreshToken) {
		t.Fatalf("stored refresh token doesn't carry the encryption envelope prefix: %q", rawRefreshToken)
	}
}

// TestPgPipelineGateRepository_TeamsSettings_EmptyByDefault covers a
// fresh org (no Teams settings ever written): all fields empty,
// Configured() false, no decryption attempted (would error on an empty
// string since it's not a valid envelope).
func TestPgPipelineGateRepository_TeamsSettings_EmptyByDefault(t *testing.T) {
	pool := setupTestDB(t)
	orgID := createTestOrg(t, pool)

	repo := NewPipelineGateRepository(pool)
	got, err := repo.GetOrgTeamsSettings(context.Background(), orgID)
	if err != nil {
		t.Fatalf("GetOrgTeamsSettings failed: %v", err)
	}
	if got != (TeamsSettings{}) {
		t.Fatalf("expected zero-value settings for a fresh org, got %+v", got)
	}
	if got.Configured() {
		t.Fatalf("expected Configured() to be false for a fresh org")
	}
}

// TestPgPipelineGateRepository_UpdateTeamsRefreshToken covers the narrow
// rotation path the delegated Graph client uses (see
// teams.delegatedTokenProvider): only the refresh token column changes,
// every other Teams field (and the org's other settings) is untouched.
func TestPgPipelineGateRepository_UpdateTeamsRefreshToken(t *testing.T) {
	pool := setupTestDB(t)
	orgID := createTestOrg(t, pool)

	cipher, err := crypto.New("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		t.Fatalf("failed to build test cipher: %v", err)
	}
	repo := NewPipelineGateRepository(pool, cipher)
	ctx := context.Background()

	initial := TeamsSettings{
		TenantID:              "tenant-123",
		ClientID:              "client-456",
		ClientSecret:          "app-secret",
		RefreshToken:          "old-refresh-token",
		ConnectedAccountEmail: "rootcauseway-bot@example.com",
	}
	if err := repo.SetOrgTeamsSettings(ctx, orgID, initial); err != nil {
		t.Fatalf("SetOrgTeamsSettings failed: %v", err)
	}

	if err := repo.UpdateTeamsRefreshToken(ctx, orgID, "new-rotated-refresh-token"); err != nil {
		t.Fatalf("UpdateTeamsRefreshToken failed: %v", err)
	}

	got, err := repo.GetOrgTeamsSettings(ctx, orgID)
	if err != nil {
		t.Fatalf("GetOrgTeamsSettings failed: %v", err)
	}
	want := initial
	want.RefreshToken = "new-rotated-refresh-token"
	if got != want {
		t.Fatalf("expected only RefreshToken to change: got %+v, want %+v", got, want)
	}
}

// TestPgPipelineGateRepository_DisconnectTeams covers the "Disconnect"
// button's backend path: refresh token + connected account email are
// cleared, but tenant_id/client_id/client_secret stay saved so reconnecting
// doesn't require re-entering the Azure AD app registration.
func TestPgPipelineGateRepository_DisconnectTeams(t *testing.T) {
	pool := setupTestDB(t)
	orgID := createTestOrg(t, pool)

	cipher, err := crypto.New("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		t.Fatalf("failed to build test cipher: %v", err)
	}
	repo := NewPipelineGateRepository(pool, cipher)
	ctx := context.Background()

	initial := TeamsSettings{
		TenantID:              "tenant-123",
		ClientID:              "client-456",
		ClientSecret:          "app-secret",
		RefreshToken:          "old-refresh-token",
		ConnectedAccountEmail: "rootcauseway-bot@example.com",
	}
	if err := repo.SetOrgTeamsSettings(ctx, orgID, initial); err != nil {
		t.Fatalf("SetOrgTeamsSettings failed: %v", err)
	}

	if err := repo.DisconnectTeams(ctx, orgID); err != nil {
		t.Fatalf("DisconnectTeams failed: %v", err)
	}

	got, err := repo.GetOrgTeamsSettings(ctx, orgID)
	if err != nil {
		t.Fatalf("GetOrgTeamsSettings failed: %v", err)
	}
	want := initial
	want.RefreshToken = ""
	want.ConnectedAccountEmail = ""
	if got != want {
		t.Fatalf("expected only RefreshToken/ConnectedAccountEmail cleared: got %+v, want %+v", got, want)
	}
}
