package database

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Marketplace Agent Repository ---

type PgMarketplaceAgentRepository struct{ pool *pgxpool.Pool }

func NewMarketplaceAgentRepository(pool *pgxpool.Pool) *PgMarketplaceAgentRepository {
	return &PgMarketplaceAgentRepository{pool: pool}
}

func (r *PgMarketplaceAgentRepository) Create(ctx context.Context, a *models.MarketplaceAgent) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO marketplace_agents (id, name, slug, description, long_description, author, author_url, version, category, icon_url, docker_image, agent_card, skills, required_credentials, config_schema, readme, downloads, rating, verified, published, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`,
		a.ID, a.Name, a.Slug, a.Description, a.LongDescription, a.Author, a.AuthorURL, a.Version, a.Category, a.IconURL, a.DockerImage, a.AgentCard, a.Skills, a.RequiredCredentials, a.ConfigSchema, a.Readme, a.Downloads, a.Rating, a.Verified, a.Published, a.CreatedAt, a.UpdatedAt)
	return err
}

func (r *PgMarketplaceAgentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.MarketplaceAgent, error) {
	var a models.MarketplaceAgent
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, COALESCE(description,''), COALESCE(long_description,''), COALESCE(author,''), COALESCE(author_url,''), version, COALESCE(category,''), COALESCE(icon_url,''), COALESCE(docker_image,''), COALESCE(agent_card,'{}'::jsonb), COALESCE(skills,'[]'::jsonb), COALESCE(required_credentials,'[]'::jsonb), COALESCE(config_schema,'{}'::jsonb), COALESCE(readme,''), COALESCE(downloads,0), COALESCE(rating,0), COALESCE(verified,false), COALESCE(published,false), created_at, updated_at
		 FROM marketplace_agents WHERE id=$1`, id).
		Scan(&a.ID, &a.Name, &a.Slug, &a.Description, &a.LongDescription, &a.Author, &a.AuthorURL, &a.Version, &a.Category, &a.IconURL, &a.DockerImage, &a.AgentCard, &a.Skills, &a.RequiredCredentials, &a.ConfigSchema, &a.Readme, &a.Downloads, &a.Rating, &a.Verified, &a.Published, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *PgMarketplaceAgentRepository) GetBySlug(ctx context.Context, slug string) (*models.MarketplaceAgent, error) {
	var a models.MarketplaceAgent
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, COALESCE(description,''), COALESCE(long_description,''), COALESCE(author,''), COALESCE(author_url,''), version, COALESCE(category,''), COALESCE(icon_url,''), COALESCE(docker_image,''), COALESCE(agent_card,'{}'::jsonb), COALESCE(skills,'[]'::jsonb), COALESCE(required_credentials,'[]'::jsonb), COALESCE(config_schema,'{}'::jsonb), COALESCE(readme,''), COALESCE(downloads,0), COALESCE(rating,0), COALESCE(verified,false), COALESCE(published,false), created_at, updated_at
		 FROM marketplace_agents WHERE slug=$1`, slug).
		Scan(&a.ID, &a.Name, &a.Slug, &a.Description, &a.LongDescription, &a.Author, &a.AuthorURL, &a.Version, &a.Category, &a.IconURL, &a.DockerImage, &a.AgentCard, &a.Skills, &a.RequiredCredentials, &a.ConfigSchema, &a.Readme, &a.Downloads, &a.Rating, &a.Verified, &a.Published, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *PgMarketplaceAgentRepository) List(ctx context.Context, category, search string) ([]models.MarketplaceAgent, error) {
	query := `SELECT id, name, slug, COALESCE(description,''), COALESCE(long_description,''), COALESCE(author,''), COALESCE(author_url,''), version, COALESCE(category,''), COALESCE(icon_url,''), COALESCE(docker_image,''), COALESCE(agent_card,'{}'::jsonb), COALESCE(skills,'[]'::jsonb), COALESCE(required_credentials,'[]'::jsonb), COALESCE(config_schema,'{}'::jsonb), COALESCE(readme,''), COALESCE(downloads,0), COALESCE(rating,0), COALESCE(verified,false), COALESCE(published,false), created_at, updated_at FROM marketplace_agents WHERE published=true`
	args := []interface{}{}
	idx := 1
	if category != "" {
		query += ` AND category=$` + itoa(idx)
		args = append(args, category)
		idx++
	}
	if search != "" {
		query += ` AND (name ILIKE $` + itoa(idx) + ` OR description ILIKE $` + itoa(idx) + `)`
		args = append(args, "%"+search+"%")
		idx++
	}
	query += ` ORDER BY downloads DESC, rating DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.MarketplaceAgent
	for rows.Next() {
		var a models.MarketplaceAgent
		if err := rows.Scan(&a.ID, &a.Name, &a.Slug, &a.Description, &a.LongDescription, &a.Author, &a.AuthorURL, &a.Version, &a.Category, &a.IconURL, &a.DockerImage, &a.AgentCard, &a.Skills, &a.RequiredCredentials, &a.ConfigSchema, &a.Readme, &a.Downloads, &a.Rating, &a.Verified, &a.Published, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	if items == nil {
		items = []models.MarketplaceAgent{}
	}
	return items, nil
}

func (r *PgMarketplaceAgentRepository) Update(ctx context.Context, a *models.MarketplaceAgent) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE marketplace_agents SET name=$1, description=$2, long_description=$3, author=$4, author_url=$5, version=$6, category=$7, icon_url=$8, docker_image=$9, agent_card=$10, skills=$11, required_credentials=$12, config_schema=$13, readme=$14, verified=$15, published=$16, updated_at=$17 WHERE id=$18`,
		a.Name, a.Description, a.LongDescription, a.Author, a.AuthorURL, a.Version, a.Category, a.IconURL, a.DockerImage, a.AgentCard, a.Skills, a.RequiredCredentials, a.ConfigSchema, a.Readme, a.Verified, a.Published, a.UpdatedAt, a.ID)
	return err
}

func (r *PgMarketplaceAgentRepository) IncrementDownloads(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE marketplace_agents SET downloads = downloads + 1 WHERE id=$1`, id)
	return err
}

// --- Installed Agent Repository ---

type PgInstalledAgentRepository struct{ pool *pgxpool.Pool }

func NewInstalledAgentRepository(pool *pgxpool.Pool) *PgInstalledAgentRepository {
	return &PgInstalledAgentRepository{pool: pool}
}

func (r *PgInstalledAgentRepository) Install(ctx context.Context, ia *models.InstalledAgent) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO installed_agents (id, org_id, marketplace_agent_id, a2a_agent_id, config, version, status, installed_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		ia.ID, ia.OrgID, ia.MarketplaceAgentID, ia.A2AAgentID, ia.Config, ia.Version, ia.Status, ia.InstalledAt, ia.UpdatedAt)
	return err
}

func (r *PgInstalledAgentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.InstalledAgent, error) {
	var ia models.InstalledAgent
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, marketplace_agent_id, a2a_agent_id, COALESCE(config,'{}'::jsonb), version, status, installed_at, updated_at
		 FROM installed_agents WHERE id=$1`, id).
		Scan(&ia.ID, &ia.OrgID, &ia.MarketplaceAgentID, &ia.A2AAgentID, &ia.Config, &ia.Version, &ia.Status, &ia.InstalledAt, &ia.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ia, nil
}

func (r *PgInstalledAgentRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]models.InstalledAgent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, marketplace_agent_id, a2a_agent_id, COALESCE(config,'{}'::jsonb), version, status, installed_at, updated_at
		 FROM installed_agents WHERE org_id=$1 ORDER BY installed_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.InstalledAgent
	for rows.Next() {
		var ia models.InstalledAgent
		if err := rows.Scan(&ia.ID, &ia.OrgID, &ia.MarketplaceAgentID, &ia.A2AAgentID, &ia.Config, &ia.Version, &ia.Status, &ia.InstalledAt, &ia.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, ia)
	}
	if items == nil {
		items = []models.InstalledAgent{}
	}
	return items, nil
}

func (r *PgInstalledAgentRepository) Update(ctx context.Context, ia *models.InstalledAgent) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE installed_agents SET config=$1, version=$2, status=$3, updated_at=$4 WHERE id=$5`,
		ia.Config, ia.Version, ia.Status, ia.UpdatedAt, ia.ID)
	return err
}

func (r *PgInstalledAgentRepository) Uninstall(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx, `UPDATE installed_agents SET status='uninstalled', updated_at=$1 WHERE id=$2`, now, id)
	return err
}
