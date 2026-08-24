package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/crypto"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Encryption-at-rest helpers ---

// pickCipher resolves the optional variadic cipher argument used by repository
// constructors. When no cipher is supplied, it falls back to the environment
// (ROOTCAUSEWAY_ENCRYPTION_KEY) so existing call sites keep working unchanged.
func pickCipher(c []crypto.Cipher) crypto.Cipher {
	if len(c) > 0 && c[0] != nil {
		return c[0]
	}
	ci, err := crypto.NewFromEnv()
	if err != nil {
		panic(fmt.Sprintf("invalid %s: %v", crypto.EnvKeyName, err))
	}
	return ci
}

// encryptJSONField encrypts a whole JSON document for storage in a JSONB
// column. The envelope is stored as a JSON string (quoted) so it remains
// valid JSONB. In no-op mode the original JSON is stored unchanged.
func encryptJSONField(c crypto.Cipher, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	enc, err := c.Encrypt(string(raw))
	if err != nil {
		return nil, err
	}
	if enc == string(raw) {
		// No-op cipher: keep original (valid) JSON as-is.
		return raw, nil
	}
	return json.Marshal(enc)
}

// decryptJSONField reverses encryptJSONField. Legacy plaintext JSON documents
// (anything that is not a JSON string carrying the enc: envelope) pass
// through unchanged.
func decryptJSONField(c crypto.Cipher, raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || raw[0] != '"' {
		return raw, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return raw, nil
	}
	if !crypto.IsEncrypted(s) {
		return raw, nil
	}
	dec, err := c.Decrypt(s)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(dec), nil
}

// --- Skill Repository ---

type PgSkillRepository struct{ pool *pgxpool.Pool }

func NewSkillRepository(pool *pgxpool.Pool) *PgSkillRepository {
	return &PgSkillRepository{pool: pool}
}

func (r *PgSkillRepository) Create(ctx context.Context, s *models.Skill) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO skills (id, org_id, name, slug, description, category, prompt_template, input_schema, output_schema, required_tools, required_resource_types, required_permissions, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		s.ID, s.OrgID, s.Name, s.Slug, s.Description, s.Category, s.PromptTemplate, s.InputSchema, s.OutputSchema, s.RequiredTools, s.RequiredResourceTypes, s.RequiredPermissions, s.Enabled, s.CreatedAt, s.UpdatedAt)
	return err
}

func (r *PgSkillRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Skill, error) {
	var s models.Skill
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, slug, COALESCE(description,''), COALESCE(category,''), COALESCE(prompt_template,''), COALESCE(input_schema,'{}'::jsonb), COALESCE(output_schema,'{}'::jsonb), COALESCE(required_tools,'[]'::jsonb), COALESCE(required_resource_types,'[]'::jsonb), COALESCE(required_permissions,'[]'::jsonb), enabled, created_at, updated_at
		 FROM skills WHERE id=$1`, id).
		Scan(&s.ID, &s.OrgID, &s.Name, &s.Slug, &s.Description, &s.Category, &s.PromptTemplate, &s.InputSchema, &s.OutputSchema, &s.RequiredTools, &s.RequiredResourceTypes, &s.RequiredPermissions, &s.Enabled, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PgSkillRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Skill, error) {
	var s models.Skill
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, slug, COALESCE(description,''), COALESCE(category,''), COALESCE(prompt_template,''), COALESCE(input_schema,'{}'::jsonb), COALESCE(output_schema,'{}'::jsonb), COALESCE(required_tools,'[]'::jsonb), COALESCE(required_resource_types,'[]'::jsonb), COALESCE(required_permissions,'[]'::jsonb), enabled, created_at, updated_at
		 FROM skills WHERE org_id=$1 AND slug=$2`, orgID, slug).
		Scan(&s.ID, &s.OrgID, &s.Name, &s.Slug, &s.Description, &s.Category, &s.PromptTemplate, &s.InputSchema, &s.OutputSchema, &s.RequiredTools, &s.RequiredResourceTypes, &s.RequiredPermissions, &s.Enabled, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PgSkillRepository) List(ctx context.Context, orgID uuid.UUID, category string, page, perPage int) ([]models.Skill, int, error) {
	var total int
	countQ := `SELECT COUNT(*) FROM skills WHERE org_id=$1`
	countArgs := []interface{}{orgID}
	if category != "" {
		countQ += ` AND category=$2`
		countArgs = append(countArgs, category)
	}
	_ = r.pool.QueryRow(ctx, countQ, countArgs...).Scan(&total)

	offset := (page - 1) * perPage
	selectQ := `SELECT id, org_id, name, slug, COALESCE(description,''), COALESCE(category,''), COALESCE(prompt_template,''), COALESCE(input_schema,'{}'::jsonb), COALESCE(output_schema,'{}'::jsonb), COALESCE(required_tools,'[]'::jsonb), COALESCE(required_resource_types,'[]'::jsonb), COALESCE(required_permissions,'[]'::jsonb), enabled, created_at, updated_at FROM skills WHERE org_id=$1`
	selectArgs := []interface{}{orgID}
	if category != "" {
		selectQ += ` AND category=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		selectArgs = append(selectArgs, category, perPage, offset)
	} else {
		selectQ += ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		selectArgs = append(selectArgs, perPage, offset)
	}

	rows, err := r.pool.Query(ctx, selectQ, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.Skill
	for rows.Next() {
		var s models.Skill
		if err := rows.Scan(&s.ID, &s.OrgID, &s.Name, &s.Slug, &s.Description, &s.Category, &s.PromptTemplate, &s.InputSchema, &s.OutputSchema, &s.RequiredTools, &s.RequiredResourceTypes, &s.RequiredPermissions, &s.Enabled, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, s)
	}
	if items == nil {
		items = []models.Skill{}
	}
	if err := r.attachLinkedAgents(ctx, items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// attachLinkedAgents fills in Agents on each skill by joining agent_skills
// to a2a_agents (enabled agents only -- a disabled agent can't actually be
// dispatched, so it shouldn't make a skill look usable). One query for the
// whole page rather than N+1 per skill.
func (r *PgSkillRepository) attachLinkedAgents(ctx context.Context, skills []models.Skill) error {
	if len(skills) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(skills))
	bySkill := make(map[uuid.UUID]*models.Skill, len(skills))
	for i := range skills {
		ids[i] = skills[i].ID
		bySkill[skills[i].ID] = &skills[i]
	}

	rows, err := r.pool.Query(ctx,
		// NULLIF handles existing rows that store "" rather than NULL for
		// these columns (COALESCE alone only replaces NULL) -- found live
		// while validating this fix: every pre-existing a2a_agents row had
		// hosting_type/llm_provider as "", so the plain COALESCE version
		// silently produced "" instead of the intended default, which
		// downstream Python code (`call.get("hosting_type", "managed")`)
		// would NOT catch either, since the key would be present with an
		// empty-string value rather than absent.
		`SELECT ask.skill_id, a.id, COALESCE(a.endpoint_url,''), a.name,
		        COALESCE(NULLIF(a.hosting_type,''),'managed'), COALESCE(NULLIF(a.llm_provider,''),'platform'),
		        COALESCE(a.managed_config,'{}'::jsonb)
		 FROM agent_skills ask
		 JOIN a2a_agents a ON a.id = ask.agent_id
		 WHERE ask.skill_id = ANY($1) AND a.enabled = true`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var skillID uuid.UUID
		var ref models.SkillAgentRef
		if err := rows.Scan(&skillID, &ref.ID, &ref.URL, &ref.Name, &ref.HostingType, &ref.LLMProvider, &ref.ManagedConfig); err != nil {
			return err
		}
		if s, ok := bySkill[skillID]; ok {
			s.Agents = append(s.Agents, ref)
		}
	}
	return rows.Err()
}

func (r *PgSkillRepository) Update(ctx context.Context, s *models.Skill) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE skills SET name=$1, slug=$2, description=$3, category=$4, prompt_template=$5, input_schema=$6, output_schema=$7, required_tools=$8, required_resource_types=$9, required_permissions=$10, enabled=$11, updated_at=$12 WHERE id=$13`,
		s.Name, s.Slug, s.Description, s.Category, s.PromptTemplate, s.InputSchema, s.OutputSchema, s.RequiredTools, s.RequiredResourceTypes, s.RequiredPermissions, s.Enabled, s.UpdatedAt, s.ID)
	return err
}

func (r *PgSkillRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM skills WHERE id=$1`, id)
	return err
}

// --- Agent Skill Repository ---

type PgAgentSkillRepository struct{ pool *pgxpool.Pool }

func NewAgentSkillRepository(pool *pgxpool.Pool) *PgAgentSkillRepository {
	return &PgAgentSkillRepository{pool: pool}
}

func (r *PgAgentSkillRepository) Link(ctx context.Context, link *models.AgentSkillLink) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO agent_skills (id, agent_id, skill_id, priority, config_overrides, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		link.ID, link.AgentID, link.SkillID, link.Priority, link.ConfigOverrides, link.CreatedAt)
	return err
}

func (r *PgAgentSkillRepository) Unlink(ctx context.Context, agentID, skillID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM agent_skills WHERE agent_id=$1 AND skill_id=$2`, agentID, skillID)
	return err
}

func (r *PgAgentSkillRepository) ListByAgent(ctx context.Context, agentID uuid.UUID) ([]models.AgentSkillLink, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, agent_id, skill_id, COALESCE(priority,0), COALESCE(config_overrides,'{}'::jsonb), created_at
		 FROM agent_skills WHERE agent_id=$1 ORDER BY priority`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.AgentSkillLink
	for rows.Next() {
		var l models.AgentSkillLink
		if err := rows.Scan(&l.ID, &l.AgentID, &l.SkillID, &l.Priority, &l.ConfigOverrides, &l.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	if items == nil {
		items = []models.AgentSkillLink{}
	}
	return items, nil
}

func (r *PgAgentSkillRepository) ListBySkill(ctx context.Context, skillID uuid.UUID) ([]models.AgentSkillLink, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, agent_id, skill_id, COALESCE(priority,0), COALESCE(config_overrides,'{}'::jsonb), created_at
		 FROM agent_skills WHERE skill_id=$1 ORDER BY priority`, skillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.AgentSkillLink
	for rows.Next() {
		var l models.AgentSkillLink
		if err := rows.Scan(&l.ID, &l.AgentID, &l.SkillID, &l.Priority, &l.ConfigOverrides, &l.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	if items == nil {
		items = []models.AgentSkillLink{}
	}
	return items, nil
}

// --- Credential Provider Repository ---

type PgCredentialProviderRepository struct {
	pool   *pgxpool.Pool
	cipher crypto.Cipher
}

func NewCredentialProviderRepository(pool *pgxpool.Pool, cipher ...crypto.Cipher) *PgCredentialProviderRepository {
	return &PgCredentialProviderRepository{pool: pool, cipher: pickCipher(cipher)}
}

func (r *PgCredentialProviderRepository) Create(ctx context.Context, p *models.CredentialProvider) error {
	cfg, err := encryptJSONField(r.cipher, p.Config)
	if err != nil {
		return fmt.Errorf("failed to encrypt credential provider config: %w", err)
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO credential_providers (id, org_id, name, provider_type, config, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		p.ID, p.OrgID, p.Name, p.ProviderType, cfg, p.Enabled, p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *PgCredentialProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialProvider, error) {
	var p models.CredentialProvider
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, provider_type, COALESCE(config,'{}'::jsonb), enabled, created_at, updated_at
		 FROM credential_providers WHERE id=$1`, id).
		Scan(&p.ID, &p.OrgID, &p.Name, &p.ProviderType, &p.Config, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if p.Config, err = decryptJSONField(r.cipher, p.Config); err != nil {
		return nil, fmt.Errorf("failed to decrypt credential provider config: %w", err)
	}
	return &p, nil
}

func (r *PgCredentialProviderRepository) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.CredentialProvider, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM credential_providers WHERE org_id=$1`, orgID).Scan(&total)

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, provider_type, COALESCE(config,'{}'::jsonb), enabled, created_at, updated_at
		 FROM credential_providers WHERE org_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, orgID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []models.CredentialProvider
	for rows.Next() {
		var p models.CredentialProvider
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.ProviderType, &p.Config, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		if p.Config, err = decryptJSONField(r.cipher, p.Config); err != nil {
			return nil, 0, fmt.Errorf("failed to decrypt credential provider config: %w", err)
		}
		items = append(items, p)
	}
	if items == nil {
		items = []models.CredentialProvider{}
	}
	return items, total, nil
}

func (r *PgCredentialProviderRepository) Update(ctx context.Context, p *models.CredentialProvider) error {
	cfg, err := encryptJSONField(r.cipher, p.Config)
	if err != nil {
		return fmt.Errorf("failed to encrypt credential provider config: %w", err)
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE credential_providers SET name=$1, provider_type=$2, config=$3, enabled=$4, updated_at=$5 WHERE id=$6`,
		p.Name, p.ProviderType, cfg, p.Enabled, p.UpdatedAt, p.ID)
	return err
}

func (r *PgCredentialProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM credential_providers WHERE id=$1`, id)
	return err
}

// --- Resource Credential Repository ---

type PgResourceCredentialRepository struct {
	pool   *pgxpool.Pool
	cipher crypto.Cipher
}

func NewResourceCredentialRepository(pool *pgxpool.Pool, cipher ...crypto.Cipher) *PgResourceCredentialRepository {
	return &PgResourceCredentialRepository{pool: pool, cipher: pickCipher(cipher)}
}

func (r *PgResourceCredentialRepository) Create(ctx context.Context, rc *models.ResourceCredential) error {
	path, err := r.cipher.Encrypt(rc.CredentialPath)
	if err != nil {
		return fmt.Errorf("failed to encrypt credential path: %w", err)
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO resource_credentials (id, org_id, software_id, resource_name, resource_type, provider_id, credential_path, default_ttl_seconds, max_ttl_seconds, default_scope, metadata, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		rc.ID, rc.OrgID, rc.SoftwareID, rc.ResourceName, rc.ResourceType, rc.ProviderID, path, rc.DefaultTTL, rc.MaxTTL, rc.DefaultScope, rc.Metadata, rc.CreatedAt, rc.UpdatedAt)
	return err
}

func (r *PgResourceCredentialRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ResourceCredential, error) {
	var rc models.ResourceCredential
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, software_id, resource_name, resource_type, provider_id, COALESCE(credential_path,''), COALESCE(default_ttl_seconds,0), COALESCE(max_ttl_seconds,0), COALESCE(default_scope,'{}'::jsonb), COALESCE(metadata,'{}'::jsonb), created_at, updated_at
		 FROM resource_credentials WHERE id=$1`, id).
		Scan(&rc.ID, &rc.OrgID, &rc.SoftwareID, &rc.ResourceName, &rc.ResourceType, &rc.ProviderID, &rc.CredentialPath, &rc.DefaultTTL, &rc.MaxTTL, &rc.DefaultScope, &rc.Metadata, &rc.CreatedAt, &rc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if rc.CredentialPath, err = r.cipher.Decrypt(rc.CredentialPath); err != nil {
		return nil, fmt.Errorf("failed to decrypt credential path: %w", err)
	}
	return &rc, nil
}

func (r *PgResourceCredentialRepository) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.ResourceCredential, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, software_id, resource_name, resource_type, provider_id, COALESCE(credential_path,''), COALESCE(default_ttl_seconds,0), COALESCE(max_ttl_seconds,0), COALESCE(default_scope,'{}'::jsonb), COALESCE(metadata,'{}'::jsonb), created_at, updated_at
		 FROM resource_credentials WHERE software_id=$1 ORDER BY created_at`, softwareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.ResourceCredential
	for rows.Next() {
		var rc models.ResourceCredential
		if err := rows.Scan(&rc.ID, &rc.OrgID, &rc.SoftwareID, &rc.ResourceName, &rc.ResourceType, &rc.ProviderID, &rc.CredentialPath, &rc.DefaultTTL, &rc.MaxTTL, &rc.DefaultScope, &rc.Metadata, &rc.CreatedAt, &rc.UpdatedAt); err != nil {
			return nil, err
		}
		if rc.CredentialPath, err = r.cipher.Decrypt(rc.CredentialPath); err != nil {
			return nil, fmt.Errorf("failed to decrypt credential path: %w", err)
		}
		items = append(items, rc)
	}
	if items == nil {
		items = []models.ResourceCredential{}
	}
	return items, nil
}

func (r *PgResourceCredentialRepository) ListByProvider(ctx context.Context, providerID uuid.UUID) ([]models.ResourceCredential, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, software_id, resource_name, resource_type, provider_id, COALESCE(credential_path,''), COALESCE(default_ttl_seconds,0), COALESCE(max_ttl_seconds,0), COALESCE(default_scope,'{}'::jsonb), COALESCE(metadata,'{}'::jsonb), created_at, updated_at
		 FROM resource_credentials WHERE provider_id=$1 ORDER BY created_at`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.ResourceCredential
	for rows.Next() {
		var rc models.ResourceCredential
		if err := rows.Scan(&rc.ID, &rc.OrgID, &rc.SoftwareID, &rc.ResourceName, &rc.ResourceType, &rc.ProviderID, &rc.CredentialPath, &rc.DefaultTTL, &rc.MaxTTL, &rc.DefaultScope, &rc.Metadata, &rc.CreatedAt, &rc.UpdatedAt); err != nil {
			return nil, err
		}
		if rc.CredentialPath, err = r.cipher.Decrypt(rc.CredentialPath); err != nil {
			return nil, fmt.Errorf("failed to decrypt credential path: %w", err)
		}
		items = append(items, rc)
	}
	if items == nil {
		items = []models.ResourceCredential{}
	}
	return items, nil
}

func (r *PgResourceCredentialRepository) Update(ctx context.Context, rc *models.ResourceCredential) error {
	path, err := r.cipher.Encrypt(rc.CredentialPath)
	if err != nil {
		return fmt.Errorf("failed to encrypt credential path: %w", err)
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE resource_credentials SET resource_name=$1, resource_type=$2, provider_id=$3, credential_path=$4, default_ttl_seconds=$5, max_ttl_seconds=$6, default_scope=$7, metadata=$8, updated_at=$9 WHERE id=$10`,
		rc.ResourceName, rc.ResourceType, rc.ProviderID, path, rc.DefaultTTL, rc.MaxTTL, rc.DefaultScope, rc.Metadata, rc.UpdatedAt, rc.ID)
	return err
}

func (r *PgResourceCredentialRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM resource_credentials WHERE id=$1`, id)
	return err
}

// --- Access Policy Repository ---

type PgAccessPolicyRepository struct{ pool *pgxpool.Pool }

func NewAccessPolicyRepository(pool *pgxpool.Pool) *PgAccessPolicyRepository {
	return &PgAccessPolicyRepository{pool: pool}
}

func (r *PgAccessPolicyRepository) Create(ctx context.Context, p *models.AccessPolicy) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO access_policies (id, org_id, name, description, target_type, target_id, resource_type, allowed_actions, scope_restrictions, max_ttl, require_approval, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		p.ID, p.OrgID, p.Name, p.Description, p.TargetType, p.TargetID, p.ResourceType, p.AllowedActions, p.ScopeRestrictions, p.MaxTTL, p.RequireApproval, p.Enabled, p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *PgAccessPolicyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AccessPolicy, error) {
	var p models.AccessPolicy
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, COALESCE(description,''), target_type, target_id, resource_type, COALESCE(allowed_actions,'[]'::jsonb), COALESCE(scope_restrictions,'{}'::jsonb), COALESCE(max_ttl,0), require_approval, enabled, created_at, updated_at
		 FROM access_policies WHERE id=$1`, id).
		Scan(&p.ID, &p.OrgID, &p.Name, &p.Description, &p.TargetType, &p.TargetID, &p.ResourceType, &p.AllowedActions, &p.ScopeRestrictions, &p.MaxTTL, &p.RequireApproval, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PgAccessPolicyRepository) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.AccessPolicy, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM access_policies WHERE org_id=$1`, orgID).Scan(&total)

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, COALESCE(description,''), target_type, target_id, resource_type, COALESCE(allowed_actions,'[]'::jsonb), COALESCE(scope_restrictions,'{}'::jsonb), COALESCE(max_ttl,0), require_approval, enabled, created_at, updated_at
		 FROM access_policies WHERE org_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, orgID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []models.AccessPolicy
	for rows.Next() {
		var p models.AccessPolicy
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Description, &p.TargetType, &p.TargetID, &p.ResourceType, &p.AllowedActions, &p.ScopeRestrictions, &p.MaxTTL, &p.RequireApproval, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if items == nil {
		items = []models.AccessPolicy{}
	}
	return items, total, nil
}

func (r *PgAccessPolicyRepository) ListByTarget(ctx context.Context, targetType string, targetID uuid.UUID) ([]models.AccessPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, COALESCE(description,''), target_type, target_id, resource_type, COALESCE(allowed_actions,'[]'::jsonb), COALESCE(scope_restrictions,'{}'::jsonb), COALESCE(max_ttl,0), require_approval, enabled, created_at, updated_at
		 FROM access_policies WHERE target_type=$1 AND target_id=$2 AND enabled=true ORDER BY created_at`, targetType, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.AccessPolicy
	for rows.Next() {
		var p models.AccessPolicy
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.Description, &p.TargetType, &p.TargetID, &p.ResourceType, &p.AllowedActions, &p.ScopeRestrictions, &p.MaxTTL, &p.RequireApproval, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if items == nil {
		items = []models.AccessPolicy{}
	}
	return items, nil
}

func (r *PgAccessPolicyRepository) Update(ctx context.Context, p *models.AccessPolicy) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE access_policies SET name=$1, description=$2, target_type=$3, target_id=$4, resource_type=$5, allowed_actions=$6, scope_restrictions=$7, max_ttl=$8, require_approval=$9, enabled=$10, updated_at=$11 WHERE id=$12`,
		p.Name, p.Description, p.TargetType, p.TargetID, p.ResourceType, p.AllowedActions, p.ScopeRestrictions, p.MaxTTL, p.RequireApproval, p.Enabled, p.UpdatedAt, p.ID)
	return err
}

func (r *PgAccessPolicyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM access_policies WHERE id=$1`, id)
	return err
}

// --- Credential Lease Repository ---

type PgCredentialLeaseRepository struct{ pool *pgxpool.Pool }

func NewCredentialLeaseRepository(pool *pgxpool.Pool) *PgCredentialLeaseRepository {
	return &PgCredentialLeaseRepository{pool: pool}
}

func (r *PgCredentialLeaseRepository) Create(ctx context.Context, l *models.CredentialLease) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO credential_leases (id, org_id, incident_id, agent_id, skill_id, resource_credential_id, policy_id, status, scope, issued_at, expires_at, revoked_at, revoked_by, request_reason, actions_performed, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		l.ID, l.OrgID, l.IncidentID, l.AgentID, l.SkillID, l.ResourceCredentialID, l.PolicyID, l.Status, l.Scope, l.IssuedAt, l.ExpiresAt, l.RevokedAt, l.RevokedBy, l.RequestReason, l.ActionsPerformed, l.CreatedAt)
	return err
}

func (r *PgCredentialLeaseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.CredentialLease, error) {
	var l models.CredentialLease
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, incident_id, agent_id, skill_id, resource_credential_id, policy_id, status, COALESCE(scope,'{}'::jsonb), issued_at, expires_at, revoked_at, COALESCE(revoked_by,''), COALESCE(request_reason,''), COALESCE(actions_performed,'[]'::jsonb), created_at
		 FROM credential_leases WHERE id=$1`, id).
		Scan(&l.ID, &l.OrgID, &l.IncidentID, &l.AgentID, &l.SkillID, &l.ResourceCredentialID, &l.PolicyID, &l.Status, &l.Scope, &l.IssuedAt, &l.ExpiresAt, &l.RevokedAt, &l.RevokedBy, &l.RequestReason, &l.ActionsPerformed, &l.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *PgCredentialLeaseRepository) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.CredentialLease, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, incident_id, agent_id, skill_id, resource_credential_id, policy_id, status, COALESCE(scope,'{}'::jsonb), issued_at, expires_at, revoked_at, COALESCE(revoked_by,''), COALESCE(request_reason,''), COALESCE(actions_performed,'[]'::jsonb), created_at
		 FROM credential_leases WHERE incident_id=$1 ORDER BY created_at`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.CredentialLease
	for rows.Next() {
		var l models.CredentialLease
		if err := rows.Scan(&l.ID, &l.OrgID, &l.IncidentID, &l.AgentID, &l.SkillID, &l.ResourceCredentialID, &l.PolicyID, &l.Status, &l.Scope, &l.IssuedAt, &l.ExpiresAt, &l.RevokedAt, &l.RevokedBy, &l.RequestReason, &l.ActionsPerformed, &l.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	if items == nil {
		items = []models.CredentialLease{}
	}
	return items, nil
}

func (r *PgCredentialLeaseRepository) ListActive(ctx context.Context, orgID uuid.UUID) ([]models.CredentialLease, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, incident_id, agent_id, skill_id, resource_credential_id, policy_id, status, COALESCE(scope,'{}'::jsonb), issued_at, expires_at, revoked_at, COALESCE(revoked_by,''), COALESCE(request_reason,''), COALESCE(actions_performed,'[]'::jsonb), created_at
		 FROM credential_leases WHERE org_id=$1 AND status='active' ORDER BY created_at`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.CredentialLease
	for rows.Next() {
		var l models.CredentialLease
		if err := rows.Scan(&l.ID, &l.OrgID, &l.IncidentID, &l.AgentID, &l.SkillID, &l.ResourceCredentialID, &l.PolicyID, &l.Status, &l.Scope, &l.IssuedAt, &l.ExpiresAt, &l.RevokedAt, &l.RevokedBy, &l.RequestReason, &l.ActionsPerformed, &l.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, l)
	}
	if items == nil {
		items = []models.CredentialLease{}
	}
	return items, nil
}

func (r *PgCredentialLeaseRepository) Update(ctx context.Context, l *models.CredentialLease) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE credential_leases SET status=$1, revoked_at=$2, revoked_by=$3, actions_performed=$4 WHERE id=$5`,
		l.Status, l.RevokedAt, l.RevokedBy, l.ActionsPerformed, l.ID)
	return err
}

func (r *PgCredentialLeaseRepository) ExpireLeases(ctx context.Context) (int64, error) {
	now := time.Now()
	tag, err := r.pool.Exec(ctx,
		`UPDATE credential_leases SET status='expired' WHERE status='active' AND expires_at < $1`, now)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
