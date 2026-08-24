package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Role Repository ---

type PgRoleRepository struct{ pool *pgxpool.Pool }

func NewRoleRepository(pool *pgxpool.Pool) *PgRoleRepository {
	return &PgRoleRepository{pool: pool}
}

func (r *PgRoleRepository) Create(ctx context.Context, role *models.Role) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO roles (id, org_id, name, slug, description, is_system, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		role.ID, role.OrgID, role.Name, role.Slug, role.Description, role.IsSystem, role.CreatedAt, role.UpdatedAt)
	return err
}

func (r *PgRoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error) {
	var role models.Role
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, slug, COALESCE(description,''), is_system, created_at, updated_at
		 FROM roles WHERE id=$1`, id).
		Scan(&role.ID, &role.OrgID, &role.Name, &role.Slug, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *PgRoleRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Role, error) {
	var role models.Role
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, slug, COALESCE(description,''), is_system, created_at, updated_at
		 FROM roles WHERE org_id=$1 AND slug=$2`, orgID, slug).
		Scan(&role.ID, &role.OrgID, &role.Name, &role.Slug, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *PgRoleRepository) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.Role, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM roles WHERE org_id=$1`, orgID).Scan(&total)

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, slug, COALESCE(description,''), is_system, created_at, updated_at
		 FROM roles WHERE org_id=$1 ORDER BY created_at ASC LIMIT $2 OFFSET $3`, orgID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.Role
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(&role.ID, &role.OrgID, &role.Name, &role.Slug, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, role)
	}
	if items == nil {
		items = []models.Role{}
	}
	return items, total, nil
}

func (r *PgRoleRepository) Update(ctx context.Context, role *models.Role) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE roles SET name=$1, slug=$2, description=$3, updated_at=$4 WHERE id=$5`,
		role.Name, role.Slug, role.Description, role.UpdatedAt, role.ID)
	return err
}

func (r *PgRoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM roles WHERE id=$1 AND is_system=false`, id)
	return err
}

// --- Permission Repository ---

type PgPermissionRepository struct{ pool *pgxpool.Pool }

func NewPermissionRepository(pool *pgxpool.Pool) *PgPermissionRepository {
	return &PgPermissionRepository{pool: pool}
}

func (r *PgPermissionRepository) List(ctx context.Context) ([]models.Permission, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, resource, action, COALESCE(description,'') FROM permissions ORDER BY resource, action`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Permission
	for rows.Next() {
		var p models.Permission
		if err := rows.Scan(&p.ID, &p.Resource, &p.Action, &p.Description); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if items == nil {
		items = []models.Permission{}
	}
	return items, nil
}

func (r *PgPermissionRepository) GetByResourceAction(ctx context.Context, resource, action string) (*models.Permission, error) {
	var p models.Permission
	err := r.pool.QueryRow(ctx,
		`SELECT id, resource, action, COALESCE(description,'') FROM permissions WHERE resource=$1 AND action=$2`,
		resource, action).
		Scan(&p.ID, &p.Resource, &p.Action, &p.Description)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PgPermissionRepository) ListByRole(ctx context.Context, roleID uuid.UUID) ([]models.Permission, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.resource, p.action, COALESCE(p.description,'')
		 FROM permissions p
		 JOIN role_permissions rp ON rp.permission_id = p.id
		 WHERE rp.role_id = $1
		 ORDER BY p.resource, p.action`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Permission
	for rows.Next() {
		var p models.Permission
		if err := rows.Scan(&p.ID, &p.Resource, &p.Action, &p.Description); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if items == nil {
		items = []models.Permission{}
	}
	return items, nil
}

// --- RolePermission Repository ---

type PgRolePermissionRepository struct{ pool *pgxpool.Pool }

func NewRolePermissionRepository(pool *pgxpool.Pool) *PgRolePermissionRepository {
	return &PgRolePermissionRepository{pool: pool}
}

func (r *PgRolePermissionRepository) Grant(ctx context.Context, roleID, permissionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		roleID, permissionID)
	return err
}

func (r *PgRolePermissionRepository) Revoke(ctx context.Context, roleID, permissionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM role_permissions WHERE role_id=$1 AND permission_id=$2`,
		roleID, permissionID)
	return err
}

func (r *PgRolePermissionRepository) ListByRole(ctx context.Context, roleID uuid.UUID) ([]models.Permission, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.resource, p.action, COALESCE(p.description,'')
		 FROM permissions p
		 JOIN role_permissions rp ON rp.permission_id = p.id
		 WHERE rp.role_id = $1`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Permission
	for rows.Next() {
		var p models.Permission
		if err := rows.Scan(&p.ID, &p.Resource, &p.Action, &p.Description); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if items == nil {
		items = []models.Permission{}
	}
	return items, nil
}

// --- UserRole Repository ---

type PgUserRoleRepository struct{ pool *pgxpool.Pool }

func NewUserRoleRepository(pool *pgxpool.Pool) *PgUserRoleRepository {
	return &PgUserRoleRepository{pool: pool}
}

func (r *PgUserRoleRepository) Assign(ctx context.Context, userID, roleID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userID, roleID)
	return err
}

func (r *PgUserRoleRepository) Unassign(ctx context.Context, userID, roleID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM user_roles WHERE user_id=$1 AND role_id=$2`,
		userID, roleID)
	return err
}

func (r *PgUserRoleRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Role, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT r.id, r.org_id, r.name, r.slug, COALESCE(r.description,''), r.is_system, r.created_at, r.updated_at
		 FROM roles r
		 JOIN user_roles ur ON ur.role_id = r.id
		 WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Role
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(&role.ID, &role.OrgID, &role.Name, &role.Slug, &role.Description, &role.IsSystem, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, role)
	}
	if items == nil {
		items = []models.Role{}
	}
	return items, nil
}

// --- SSO Provider Repository ---

type PgSSOProviderRepository struct{ pool *pgxpool.Pool }

func NewSSOProviderRepository(pool *pgxpool.Pool) *PgSSOProviderRepository {
	return &PgSSOProviderRepository{pool: pool}
}

func (r *PgSSOProviderRepository) Create(ctx context.Context, p *models.SSOProvider) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sso_providers (id, org_id, name, provider_type, client_id, client_secret, issuer_url, authorization_url, token_url, userinfo_url, scopes, auto_provision_users, default_role_id, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		p.ID, p.OrgID, p.Name, p.ProviderType, p.ClientID, p.ClientSecret, p.IssuerURL, p.AuthorizationURL, p.TokenURL, p.UserinfoURL, p.Scopes, p.AutoProvisionUsers, p.DefaultRoleID, p.Enabled, p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *PgSSOProviderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.SSOProvider, error) {
	var p models.SSOProvider
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, provider_type, client_id, client_secret, COALESCE(issuer_url,''), authorization_url, token_url, COALESCE(userinfo_url,''), COALESCE(scopes,''), auto_provision_users, default_role_id, enabled, created_at, updated_at
		 FROM sso_providers WHERE id=$1`, id).
		Scan(&p.ID, &p.OrgID, &p.Name, &p.ProviderType, &p.ClientID, &p.ClientSecret, &p.IssuerURL, &p.AuthorizationURL, &p.TokenURL, &p.UserinfoURL, &p.Scopes, &p.AutoProvisionUsers, &p.DefaultRoleID, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *PgSSOProviderRepository) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.SSOProvider, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sso_providers WHERE org_id=$1`, orgID).Scan(&total)

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, provider_type, client_id, COALESCE(issuer_url,''), authorization_url, token_url, COALESCE(userinfo_url,''), COALESCE(scopes,''), auto_provision_users, default_role_id, enabled, created_at, updated_at
		 FROM sso_providers WHERE org_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, orgID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.SSOProvider
	for rows.Next() {
		var p models.SSOProvider
		if err := rows.Scan(&p.ID, &p.OrgID, &p.Name, &p.ProviderType, &p.ClientID, &p.IssuerURL, &p.AuthorizationURL, &p.TokenURL, &p.UserinfoURL, &p.Scopes, &p.AutoProvisionUsers, &p.DefaultRoleID, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if items == nil {
		items = []models.SSOProvider{}
	}
	return items, total, nil
}

func (r *PgSSOProviderRepository) Update(ctx context.Context, p *models.SSOProvider) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sso_providers SET name=$1, provider_type=$2, client_id=$3, client_secret=$4, issuer_url=$5, authorization_url=$6, token_url=$7, userinfo_url=$8, scopes=$9, auto_provision_users=$10, default_role_id=$11, enabled=$12, updated_at=$13 WHERE id=$14`,
		p.Name, p.ProviderType, p.ClientID, p.ClientSecret, p.IssuerURL, p.AuthorizationURL, p.TokenURL, p.UserinfoURL, p.Scopes, p.AutoProvisionUsers, p.DefaultRoleID, p.Enabled, p.UpdatedAt, p.ID)
	return err
}

func (r *PgSSOProviderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sso_providers WHERE id=$1`, id)
	return err
}

// --- API Key Repository ---

type PgAPIKeyRepository struct{ pool *pgxpool.Pool }

func NewAPIKeyRepository(pool *pgxpool.Pool) *PgAPIKeyRepository {
	return &PgAPIKeyRepository{pool: pool}
}

func (r *PgAPIKeyRepository) Create(ctx context.Context, k *models.APIKey) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO api_keys (id, org_id, user_id, name, key_hash, key_prefix, role_id, scopes, expires_at, is_active, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		k.ID, k.OrgID, k.UserID, k.Name, k.KeyHash, k.KeyPrefix, k.RoleID, k.Scopes, k.ExpiresAt, k.IsActive, k.CreatedAt)
	return err
}

func (r *PgAPIKeyRepository) GetByPrefix(ctx context.Context, prefix string) (*models.APIKey, error) {
	var k models.APIKey
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, user_id, name, key_hash, key_prefix, role_id, COALESCE(scopes,'[]'::jsonb), expires_at, last_used_at, is_active, created_at
		 FROM api_keys WHERE key_prefix=$1`, prefix).
		Scan(&k.ID, &k.OrgID, &k.UserID, &k.Name, &k.KeyHash, &k.KeyPrefix, &k.RoleID, &k.Scopes, &k.ExpiresAt, &k.LastUsedAt, &k.IsActive, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (r *PgAPIKeyRepository) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.APIKey, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_keys WHERE org_id=$1`, orgID).Scan(&total)

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, user_id, name, key_prefix, role_id, COALESCE(scopes,'[]'::jsonb), expires_at, last_used_at, is_active, created_at
		 FROM api_keys WHERE org_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, orgID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.APIKey
	for rows.Next() {
		var k models.APIKey
		if err := rows.Scan(&k.ID, &k.OrgID, &k.UserID, &k.Name, &k.KeyPrefix, &k.RoleID, &k.Scopes, &k.ExpiresAt, &k.LastUsedAt, &k.IsActive, &k.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, k)
	}
	if items == nil {
		items = []models.APIKey{}
	}
	return items, total, nil
}

func (r *PgAPIKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE api_keys SET last_used_at=$1 WHERE id=$2`, time.Now(), id)
	return err
}

func (r *PgAPIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM api_keys WHERE id=$1`, id)
	return err
}

func (r *PgAPIKeyRepository) Deactivate(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE api_keys SET is_active=false WHERE id=$1`, id)
	return err
}

// --- Audit Log Repository ---

type PgAuditLogRepository struct{ pool *pgxpool.Pool }

func NewAuditLogRepository(pool *pgxpool.Pool) *PgAuditLogRepository {
	return &PgAuditLogRepository{pool: pool}
}

func (r *PgAuditLogRepository) Create(ctx context.Context, entry *models.AuditLogEntry) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO audit_log (id, org_id, user_id, action, resource_type, resource_id, details, ip_address, user_agent, request_id, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		entry.ID, entry.OrgID, entry.UserID, entry.Action, entry.ResourceType, entry.ResourceID, entry.Details, entry.IPAddress, entry.UserAgent, entry.RequestID, entry.CreatedAt)
	return err
}

func (r *PgAuditLogRepository) List(ctx context.Context, orgID uuid.UUID, filter models.AuditLogFilter) ([]models.AuditLogEntry, int, error) {
	// Build dynamic query
	query := `SELECT id, org_id, user_id, action, COALESCE(resource_type,''), COALESCE(resource_id,''), COALESCE(details,'{}'::jsonb), COALESCE(ip_address,''), COALESCE(user_agent,''), COALESCE(request_id,''), created_at FROM audit_log WHERE org_id=$1`
	countQuery := `SELECT COUNT(*) FROM audit_log WHERE org_id=$1`
	args := []interface{}{orgID}
	argIdx := 2

	if filter.UserID != nil {
		query += fmt.Sprintf(` AND user_id=$%d`, argIdx)
		countQuery += fmt.Sprintf(` AND user_id=$%d`, argIdx)
		args = append(args, *filter.UserID)
		argIdx++
	}
	if filter.Action != "" {
		query += fmt.Sprintf(` AND action ILIKE $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND action ILIKE $%d`, argIdx)
		args = append(args, "%"+filter.Action+"%")
		argIdx++
	}
	if filter.ResourceType != "" {
		query += fmt.Sprintf(` AND resource_type=$%d`, argIdx)
		countQuery += fmt.Sprintf(` AND resource_type=$%d`, argIdx)
		args = append(args, filter.ResourceType)
		argIdx++
	}
	if filter.From != nil {
		query += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND created_at >= $%d`, argIdx)
		args = append(args, *filter.From)
		argIdx++
	}
	if filter.To != nil {
		query += fmt.Sprintf(` AND created_at <= $%d`, argIdx)
		countQuery += fmt.Sprintf(` AND created_at <= $%d`, argIdx)
		args = append(args, *filter.To)
		argIdx++
	}

	var total int
	_ = r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)

	page := filter.Page
	if page < 1 {
		page = 1
	}
	perPage := filter.PerPage
	if perPage < 1 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	query += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, perPage, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.AuditLogEntry
	for rows.Next() {
		var e models.AuditLogEntry
		if err := rows.Scan(&e.ID, &e.OrgID, &e.UserID, &e.Action, &e.ResourceType, &e.ResourceID, &e.Details, &e.IPAddress, &e.UserAgent, &e.RequestID, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, e)
	}
	if items == nil {
		items = []models.AuditLogEntry{}
	}
	return items, total, nil
}

func (r *PgAuditLogRepository) Count(ctx context.Context, orgID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log WHERE org_id=$1`, orgID).Scan(&count)
	return count, err
}

// --- Session Repository ---

type PgSessionRepository struct{ pool *pgxpool.Pool }

func NewSessionRepository(pool *pgxpool.Pool) *PgSessionRepository {
	return &PgSessionRepository{pool: pool}
}

func (r *PgSessionRepository) Create(ctx context.Context, s *models.Session) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO sessions (id, user_id, token_hash, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		s.ID, s.UserID, s.TokenHash, s.RefreshTokenHash, s.IPAddress, s.UserAgent, s.ExpiresAt, s.CreatedAt)
	return err
}

func (r *PgSessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*models.Session, error) {
	var s models.Session
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, token_hash, COALESCE(refresh_token_hash,''), COALESCE(ip_address,''), COALESCE(user_agent,''), expires_at, created_at
		 FROM sessions WHERE token_hash=$1 AND expires_at > NOW()`, tokenHash).
		Scan(&s.ID, &s.UserID, &s.TokenHash, &s.RefreshTokenHash, &s.IPAddress, &s.UserAgent, &s.ExpiresAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PgSessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE id=$1`, id)
	return err
}

func (r *PgSessionRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
	return err
}

func (r *PgSessionRepository) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, userID)
	return err
}

// --- User Repository ---

type PgUserRepository struct{ pool *pgxpool.Pool }

func NewUserRepository(pool *pgxpool.Pool) *PgUserRepository {
	return &PgUserRepository{pool: pool}
}

func (r *PgUserRepository) Create(ctx context.Context, u *models.UserWithRoles) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, org_id, name, email, password_hash, role, sso_provider, sso_subject, avatar_url, is_active, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		u.ID, u.OrgID, u.Name, u.Email, u.PasswordHash, u.Role, u.SSOProvider, u.SSOSubject, u.AvatarURL, u.IsActive, u.CreatedAt, u.UpdatedAt)
	return err
}

func (r *PgUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.UserWithRoles, error) {
	var u models.UserWithRoles
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, email, COALESCE(password_hash,''), role, sso_provider, COALESCE(sso_subject,''), COALESCE(avatar_url,''), last_login_at, COALESCE(is_active,true), created_at, updated_at
		 FROM users WHERE id=$1`, id).
		Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.SSOProvider, &u.SSOSubject, &u.AvatarURL, &u.LastLoginAt, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *PgUserRepository) GetByEmail(ctx context.Context, email string) (*models.UserWithRoles, error) {
	var u models.UserWithRoles
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, email, COALESCE(password_hash,''), role, sso_provider, COALESCE(sso_subject,''), COALESCE(avatar_url,''), last_login_at, COALESCE(is_active,true), created_at, updated_at
		 FROM users WHERE email=$1`, email).
		Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.SSOProvider, &u.SSOSubject, &u.AvatarURL, &u.LastLoginAt, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *PgUserRepository) GetBySSOSubject(ctx context.Context, provider, subject string) (*models.UserWithRoles, error) {
	var u models.UserWithRoles
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, email, COALESCE(password_hash,''), role, sso_provider, COALESCE(sso_subject,''), COALESCE(avatar_url,''), last_login_at, COALESCE(is_active,true), created_at, updated_at
		 FROM users WHERE sso_provider=$1 AND sso_subject=$2`, provider, subject).
		Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.SSOProvider, &u.SSOSubject, &u.AvatarURL, &u.LastLoginAt, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *PgUserRepository) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.UserWithRoles, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE org_id=$1`, orgID).Scan(&total)

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, email, role, sso_provider, COALESCE(sso_subject,''), COALESCE(avatar_url,''), last_login_at, COALESCE(is_active,true), created_at, updated_at
		 FROM users WHERE org_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, orgID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.UserWithRoles
	for rows.Next() {
		var u models.UserWithRoles
		if err := rows.Scan(&u.ID, &u.OrgID, &u.Name, &u.Email, &u.Role, &u.SSOProvider, &u.SSOSubject, &u.AvatarURL, &u.LastLoginAt, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, u)
	}
	if items == nil {
		items = []models.UserWithRoles{}
	}
	return items, total, nil
}

func (r *PgUserRepository) Update(ctx context.Context, u *models.UserWithRoles) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET name=$1, email=$2, role=$3, sso_provider=$4, sso_subject=$5, avatar_url=$6, is_active=$7, updated_at=$8 WHERE id=$9`,
		u.Name, u.Email, u.Role, u.SSOProvider, u.SSOSubject, u.AvatarURL, u.IsActive, time.Now(), u.ID)
	return err
}

func (r *PgUserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET last_login_at=$1 WHERE id=$2`, time.Now(), id)
	return err
}
