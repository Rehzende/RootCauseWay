//go:build integration

package database

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/crypto"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

func getTestDBURL() string {
	if url := os.Getenv("ROOTCAUSEWAY_TEST_DB_URL"); url != "" {
		return url
	}
	return "postgres://rootcauseway:rootcauseway_dev_password@localhost:5432/test_rootcauseway?sslmode=disable"
}

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, getTestDBURL())
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("failed to ping test database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func createTestOrg(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	orgID := uuid.New()
	name := "test-org-" + orgID.String()[:8]
	_, err := pool.Exec(context.Background(),
		`INSERT INTO organizations (id, name, slug) VALUES ($1, $2, $3)`, orgID, name, name)
	if err != nil {
		t.Fatalf("failed to create test org: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM organizations WHERE id=$1`, orgID)
	})
	return orgID
}

func createTestSoftware(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID) uuid.UUID {
	t.Helper()
	swID := uuid.New()
	slug := "test-sw-" + swID.String()[:8]
	_, err := pool.Exec(context.Background(),
		`INSERT INTO software_catalog (id, org_id, name, slug) VALUES ($1, $2, $3, $4)`,
		swID, orgID, slug, slug)
	if err != nil {
		t.Fatalf("failed to create test software: %v", err)
	}
	return swID
}

func integrationTestCipher(t *testing.T) crypto.Cipher {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	c, err := crypto.New(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("crypto.New error: %v", err)
	}
	return c
}

func TestCredentialProviderConfigEncryptedAtRest(t *testing.T) {
	pool := setupTestDB(t)
	orgID := createTestOrg(t, pool)
	cipher := integrationTestCipher(t)
	repo := NewCredentialProviderRepository(pool, cipher)
	ctx := context.Background()

	now := time.Now().UTC()
	p := &models.CredentialProvider{
		ID:           uuid.New(),
		OrgID:        orgID,
		Name:         "static-secrets",
		ProviderType: "static",
		Config:       json.RawMessage(`{"api_key":"sk-super-secret-static-key"}`),
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// Raw DB value must be an enc:v1 envelope, not the plaintext secret.
	var rawConfig string
	if err := pool.QueryRow(ctx,
		`SELECT config::text FROM credential_providers WHERE id=$1`, p.ID).Scan(&rawConfig); err != nil {
		t.Fatalf("raw select error: %v", err)
	}
	if strings.Contains(rawConfig, "sk-super-secret-static-key") {
		t.Errorf("config stored in plaintext: %s", rawConfig)
	}
	if !strings.Contains(rawConfig, "enc:v1:") {
		t.Errorf("config missing enc:v1 envelope: %s", rawConfig)
	}

	// Repository read must return the decrypted original.
	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if string(got.Config) != `{"api_key":"sk-super-secret-static-key"}` {
		t.Errorf("GetByID config mismatch: %s", got.Config)
	}

	// List must decrypt as well.
	items, _, err := repo.List(ctx, orgID, 1, 10)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	found := false
	for _, it := range items {
		if it.ID == p.ID {
			found = true
			if strings.Contains(string(it.Config), "enc:v1:") {
				t.Errorf("List returned encrypted config: %s", it.Config)
			}
		}
	}
	if !found {
		t.Error("created provider not found in List")
	}

	// Update re-encrypts.
	p.Config = json.RawMessage(`{"api_key":"rotated-secret"}`)
	p.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, p); err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT config::text FROM credential_providers WHERE id=$1`, p.ID).Scan(&rawConfig); err != nil {
		t.Fatalf("raw select error: %v", err)
	}
	if strings.Contains(rawConfig, "rotated-secret") {
		t.Errorf("updated config stored in plaintext: %s", rawConfig)
	}
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID after update error: %v", err)
	}
	if string(got.Config) != `{"api_key":"rotated-secret"}` {
		t.Errorf("config after update mismatch: %s", got.Config)
	}
}

func TestCredentialProviderLegacyPlaintextRowStillReadable(t *testing.T) {
	pool := setupTestDB(t)
	orgID := createTestOrg(t, pool)
	cipher := integrationTestCipher(t)
	repo := NewCredentialProviderRepository(pool, cipher)
	ctx := context.Background()

	// Simulate a pre-encryption row written directly as plaintext JSONB.
	id := uuid.New()
	legacy := `{"vault_addr": "https://vault.example.com"}`
	if _, err := pool.Exec(ctx,
		`INSERT INTO credential_providers (id, org_id, name, provider_type, config, enabled)
		 VALUES ($1,$2,$3,'hashicorp_vault',$4::jsonb,true)`, id, orgID, "legacy", legacy); err != nil {
		t.Fatalf("failed to insert legacy row: %v", err)
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID legacy row error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(got.Config, &m); err != nil {
		t.Fatalf("legacy config not valid JSON: %v", err)
	}
	if m["vault_addr"] != "https://vault.example.com" {
		t.Errorf("legacy config mismatch: %s", got.Config)
	}
}

// TestSkillList_AttachesLinkedAgents covers the fix for a platform audit
// finding: PgSkillRepository.List used to return skills with no agent
// info at all (a bare `SELECT * FROM skills`, no join), which meant
// agent-service's Orchestrator._discover_skills couldn't resolve an
// agent_url for any skill -- and worse, a non-empty (but agent-less)
// skill list silently bypassed the working agent-card-discovery fallback
// for the rest of the org's pipeline. See orchestrator.py's
// _discover_skills docstring/comment for the consuming side.
func TestSkillList_AttachesLinkedAgents(t *testing.T) {
	pool := setupTestDB(t)
	orgID := createTestOrg(t, pool)
	skillRepo := NewSkillRepository(pool)
	agentSkillRepo := NewAgentSkillRepository(pool)
	ctx := context.Background()
	now := time.Now().UTC()

	skill := &models.Skill{
		ID: uuid.New(), OrgID: orgID, Name: "custom-triage", Slug: "custom-triage-" + uuid.NewString()[:8],
		Category:    "custom",
		InputSchema: json.RawMessage(`{}`), OutputSchema: json.RawMessage(`{}`),
		RequiredTools: json.RawMessage(`[]`), RequiredResourceTypes: json.RawMessage(`[]`), RequiredPermissions: json.RawMessage(`[]`),
		Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := skillRepo.Create(ctx, skill); err != nil {
		t.Fatalf("Create skill error: %v", err)
	}

	// Before any agent is linked, the skill must come back with no agents
	// -- this is the state a brand-new custom skill is in immediately
	// after creation via the UI, before the user links an agent.
	items, _, err := skillRepo.List(ctx, orgID, "", 1, 20)
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	found := findSkill(items, skill.ID)
	if found == nil {
		t.Fatal("created skill not found in List")
	}
	if len(found.Agents) != 0 {
		t.Errorf("expected no agents before linking, got %+v", found.Agents)
	}

	// Link a real, enabled agent -- hosting_type/llm_provider deliberately
	// left as "" (not NULL, not omitted) to match what every pre-existing
	// row in the live database actually has, found while validating this
	// fix live: a plain COALESCE (which only replaces NULL) silently
	// returned "" instead of the intended "managed"/"platform" default.
	agentID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO a2a_agents (id, org_id, name, agent_type, endpoint_url, hosting_type, llm_provider, managed_config, enabled)
		 VALUES ($1,$2,'triage-agent','triage','http://triage-agent:8090','','','{"model":"custom-model"}'::jsonb,true)`,
		agentID, orgID); err != nil {
		t.Fatalf("insert a2a_agent error: %v", err)
	}
	link := &models.AgentSkillLink{ID: uuid.New(), AgentID: agentID, SkillID: skill.ID, ConfigOverrides: json.RawMessage(`{}`), CreatedAt: now}
	if err := agentSkillRepo.Link(ctx, link); err != nil {
		t.Fatalf("Link error: %v", err)
	}

	items, _, err = skillRepo.List(ctx, orgID, "", 1, 20)
	if err != nil {
		t.Fatalf("List error after link: %v", err)
	}
	found = findSkill(items, skill.ID)
	if found == nil {
		t.Fatal("skill not found in List after linking")
	}
	if len(found.Agents) != 1 {
		t.Fatalf("expected exactly 1 linked agent, got %d: %+v", len(found.Agents), found.Agents)
	}
	got := found.Agents[0]
	if got.ID != agentID || got.URL != "http://triage-agent:8090" || got.Name != "triage-agent" {
		t.Errorf("agent ref mismatch: %+v", got)
	}
	if got.HostingType != "managed" || got.LLMProvider != "platform" {
		t.Errorf("agent ref hosting/provider mismatch: %+v", got)
	}
	if string(got.ManagedConfig) != `{"model": "custom-model"}` && string(got.ManagedConfig) != `{"model":"custom-model"}` {
		t.Errorf("agent ref managed_config mismatch: %s", got.ManagedConfig)
	}

	// A disabled agent must not make the skill look usable -- it can't
	// actually be dispatched to.
	if _, err := pool.Exec(ctx, `UPDATE a2a_agents SET enabled=false WHERE id=$1`, agentID); err != nil {
		t.Fatalf("disable agent error: %v", err)
	}
	items, _, err = skillRepo.List(ctx, orgID, "", 1, 20)
	if err != nil {
		t.Fatalf("List error after disabling agent: %v", err)
	}
	found = findSkill(items, skill.ID)
	if found == nil {
		t.Fatal("skill not found in List after disabling its agent")
	}
	if len(found.Agents) != 0 {
		t.Errorf("expected disabled agent to be excluded, got %+v", found.Agents)
	}
}

func findSkill(items []models.Skill, id uuid.UUID) *models.Skill {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

func TestResourceCredentialPathEncryptedAtRest(t *testing.T) {
	pool := setupTestDB(t)
	orgID := createTestOrg(t, pool)
	swID := createTestSoftware(t, pool, orgID)
	cipher := integrationTestCipher(t)

	provRepo := NewCredentialProviderRepository(pool, cipher)
	repo := NewResourceCredentialRepository(pool, cipher)
	ctx := context.Background()
	now := time.Now().UTC()

	prov := &models.CredentialProvider{
		ID: uuid.New(), OrgID: orgID, Name: "vault", ProviderType: "hashicorp_vault",
		Config: json.RawMessage(`{}`), Enabled: true, CreatedAt: now, UpdatedAt: now,
	}
	if err := provRepo.Create(ctx, prov); err != nil {
		t.Fatalf("provider Create error: %v", err)
	}

	rc := &models.ResourceCredential{
		ID: uuid.New(), OrgID: orgID, SoftwareID: swID,
		ResourceName: "prod-db", ResourceType: "database", ProviderID: prov.ID,
		CredentialPath: "secret/data/prod/db-password",
		DefaultTTL:     900, MaxTTL: 3600,
		DefaultScope: json.RawMessage(`{}`), Metadata: json.RawMessage(`{}`),
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.Create(ctx, rc); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	// Raw DB value must be encrypted.
	var rawPath string
	if err := pool.QueryRow(ctx,
		`SELECT credential_path FROM resource_credentials WHERE id=$1`, rc.ID).Scan(&rawPath); err != nil {
		t.Fatalf("raw select error: %v", err)
	}
	if !strings.HasPrefix(rawPath, "enc:v1:") {
		t.Errorf("credential_path not encrypted at rest: %q", rawPath)
	}
	if strings.Contains(rawPath, "db-password") {
		t.Errorf("credential_path leaks plaintext: %q", rawPath)
	}

	// Reads decrypt.
	got, err := repo.GetByID(ctx, rc.ID)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if got.CredentialPath != "secret/data/prod/db-password" {
		t.Errorf("GetByID path mismatch: %q", got.CredentialPath)
	}
	bySw, err := repo.ListBySoftware(ctx, swID)
	if err != nil {
		t.Fatalf("ListBySoftware error: %v", err)
	}
	if len(bySw) != 1 || bySw[0].CredentialPath != "secret/data/prod/db-password" {
		t.Errorf("ListBySoftware mismatch: %+v", bySw)
	}
	byProv, err := repo.ListByProvider(ctx, prov.ID)
	if err != nil {
		t.Fatalf("ListByProvider error: %v", err)
	}
	if len(byProv) != 1 || byProv[0].CredentialPath != "secret/data/prod/db-password" {
		t.Errorf("ListByProvider mismatch: %+v", byProv)
	}

	// Update re-encrypts.
	rc.CredentialPath = "secret/data/prod/db-password-v2"
	rc.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, rc); err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT credential_path FROM resource_credentials WHERE id=$1`, rc.ID).Scan(&rawPath); err != nil {
		t.Fatalf("raw select error: %v", err)
	}
	if !strings.HasPrefix(rawPath, "enc:v1:") || strings.Contains(rawPath, "db-password") {
		t.Errorf("updated credential_path not encrypted: %q", rawPath)
	}
	got, err = repo.GetByID(ctx, rc.ID)
	if err != nil {
		t.Fatalf("GetByID after update error: %v", err)
	}
	if got.CredentialPath != "secret/data/prod/db-password-v2" {
		t.Errorf("path after update mismatch: %q", got.CredentialPath)
	}
}
