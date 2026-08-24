package database

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Observability Source Repository ---

type PgObservabilitySourceRepository struct{ pool *pgxpool.Pool }

func NewObservabilitySourceRepository(pool *pgxpool.Pool) *PgObservabilitySourceRepository {
	return &PgObservabilitySourceRepository{pool: pool}
}

func (r *PgObservabilitySourceRepository) Create(ctx context.Context, s *models.ObservabilitySource) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO observability_sources (id, org_id, name, source_type, base_url, auth_type, auth_config, capabilities, monitored_software_ids, timeout_seconds, verify_ssl, custom_headers, enabled, health_status, last_health_check, description, environment, region, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		s.ID, s.OrgID, s.Name, s.SourceType, s.BaseURL, s.AuthType, s.AuthConfig, s.Capabilities, s.MonitoredSoftwareIDs, s.TimeoutSeconds, s.VerifySSL, s.CustomHeaders, s.Enabled, s.HealthStatus, s.LastHealthCheck, s.Description, s.Environment, s.Region, s.CreatedAt, s.UpdatedAt)
	return err
}

func (r *PgObservabilitySourceRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ObservabilitySource, error) {
	var s models.ObservabilitySource
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, source_type, base_url, COALESCE(auth_type,''), COALESCE(auth_config,'{}'::jsonb), COALESCE(capabilities,'[]'::jsonb), COALESCE(monitored_software_ids,'[]'::jsonb), COALESCE(timeout_seconds,30), COALESCE(verify_ssl,true), COALESCE(custom_headers,'{}'::jsonb), enabled, COALESCE(health_status,'unknown'), last_health_check, COALESCE(description,''), COALESCE(environment,''), COALESCE(region,''), created_at, updated_at
		 FROM observability_sources WHERE id=$1`, id).
		Scan(&s.ID, &s.OrgID, &s.Name, &s.SourceType, &s.BaseURL, &s.AuthType, &s.AuthConfig, &s.Capabilities, &s.MonitoredSoftwareIDs, &s.TimeoutSeconds, &s.VerifySSL, &s.CustomHeaders, &s.Enabled, &s.HealthStatus, &s.LastHealthCheck, &s.Description, &s.Environment, &s.Region, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PgObservabilitySourceRepository) List(ctx context.Context, orgID uuid.UUID, sourceType string) ([]models.ObservabilitySource, error) {
	query := `SELECT id, org_id, name, source_type, base_url, COALESCE(auth_type,''), COALESCE(auth_config,'{}'::jsonb), COALESCE(capabilities,'[]'::jsonb), COALESCE(monitored_software_ids,'[]'::jsonb), COALESCE(timeout_seconds,30), COALESCE(verify_ssl,true), COALESCE(custom_headers,'{}'::jsonb), enabled, COALESCE(health_status,'unknown'), last_health_check, COALESCE(description,''), COALESCE(environment,''), COALESCE(region,''), created_at, updated_at FROM observability_sources WHERE org_id=$1`
	args := []interface{}{orgID}
	if sourceType != "" {
		query += ` AND source_type=$2`
		args = append(args, sourceType)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.ObservabilitySource
	for rows.Next() {
		var s models.ObservabilitySource
		if err := rows.Scan(&s.ID, &s.OrgID, &s.Name, &s.SourceType, &s.BaseURL, &s.AuthType, &s.AuthConfig, &s.Capabilities, &s.MonitoredSoftwareIDs, &s.TimeoutSeconds, &s.VerifySSL, &s.CustomHeaders, &s.Enabled, &s.HealthStatus, &s.LastHealthCheck, &s.Description, &s.Environment, &s.Region, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	if items == nil {
		items = []models.ObservabilitySource{}
	}
	return items, nil
}

func (r *PgObservabilitySourceRepository) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.ObservabilitySource, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, source_type, base_url, COALESCE(auth_type,''), COALESCE(auth_config,'{}'::jsonb), COALESCE(capabilities,'[]'::jsonb), COALESCE(monitored_software_ids,'[]'::jsonb), COALESCE(timeout_seconds,30), COALESCE(verify_ssl,true), COALESCE(custom_headers,'{}'::jsonb), enabled, COALESCE(health_status,'unknown'), last_health_check, COALESCE(description,''), COALESCE(environment,''), COALESCE(region,''), created_at, updated_at
		 FROM observability_sources WHERE monitored_software_ids @> $1::jsonb ORDER BY created_at DESC`,
		`["`+softwareID.String()+`"]`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.ObservabilitySource
	for rows.Next() {
		var s models.ObservabilitySource
		if err := rows.Scan(&s.ID, &s.OrgID, &s.Name, &s.SourceType, &s.BaseURL, &s.AuthType, &s.AuthConfig, &s.Capabilities, &s.MonitoredSoftwareIDs, &s.TimeoutSeconds, &s.VerifySSL, &s.CustomHeaders, &s.Enabled, &s.HealthStatus, &s.LastHealthCheck, &s.Description, &s.Environment, &s.Region, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	if items == nil {
		items = []models.ObservabilitySource{}
	}
	return items, nil
}

func (r *PgObservabilitySourceRepository) Update(ctx context.Context, s *models.ObservabilitySource) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE observability_sources SET name=$1, source_type=$2, base_url=$3, auth_type=$4, auth_config=$5, capabilities=$6, monitored_software_ids=$7, timeout_seconds=$8, verify_ssl=$9, custom_headers=$10, enabled=$11, description=$12, environment=$13, region=$14, updated_at=$15 WHERE id=$16`,
		s.Name, s.SourceType, s.BaseURL, s.AuthType, s.AuthConfig, s.Capabilities, s.MonitoredSoftwareIDs, s.TimeoutSeconds, s.VerifySSL, s.CustomHeaders, s.Enabled, s.Description, s.Environment, s.Region, s.UpdatedAt, s.ID)
	return err
}

func (r *PgObservabilitySourceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM observability_sources WHERE id=$1`, id)
	return err
}

func (r *PgObservabilitySourceRepository) UpdateHealthStatus(ctx context.Context, id uuid.UUID, status string) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE observability_sources SET health_status=$1, last_health_check=$2, updated_at=$2 WHERE id=$3`,
		status, now, id)
	return err
}

// --- Snapshot Config Repository ---

type PgSnapshotConfigRepository struct{ pool *pgxpool.Pool }

func NewSnapshotConfigRepository(pool *pgxpool.Pool) *PgSnapshotConfigRepository {
	return &PgSnapshotConfigRepository{pool: pool}
}

func (r *PgSnapshotConfigRepository) Create(ctx context.Context, sc *models.SnapshotConfig) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO snapshot_configs (id, org_id, source_id, software_id, name, snapshot_type, query_template, time_range_seconds, parameters, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		sc.ID, sc.OrgID, sc.SourceID, sc.SoftwareID, sc.Name, sc.SnapshotType, sc.QueryTemplate, sc.TimeRangeSeconds, sc.Parameters, sc.Enabled, sc.CreatedAt, sc.UpdatedAt)
	return err
}

func (r *PgSnapshotConfigRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.SnapshotConfig, error) {
	var sc models.SnapshotConfig
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, source_id, software_id, name, snapshot_type, COALESCE(query_template,''), COALESCE(time_range_seconds,3600), COALESCE(parameters,'{}'::jsonb), enabled, created_at, updated_at
		 FROM snapshot_configs WHERE id=$1`, id).
		Scan(&sc.ID, &sc.OrgID, &sc.SourceID, &sc.SoftwareID, &sc.Name, &sc.SnapshotType, &sc.QueryTemplate, &sc.TimeRangeSeconds, &sc.Parameters, &sc.Enabled, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

func (r *PgSnapshotConfigRepository) ListBySource(ctx context.Context, sourceID uuid.UUID) ([]models.SnapshotConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, source_id, software_id, name, snapshot_type, COALESCE(query_template,''), COALESCE(time_range_seconds,3600), COALESCE(parameters,'{}'::jsonb), enabled, created_at, updated_at
		 FROM snapshot_configs WHERE source_id=$1 ORDER BY created_at DESC`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.SnapshotConfig
	for rows.Next() {
		var sc models.SnapshotConfig
		if err := rows.Scan(&sc.ID, &sc.OrgID, &sc.SourceID, &sc.SoftwareID, &sc.Name, &sc.SnapshotType, &sc.QueryTemplate, &sc.TimeRangeSeconds, &sc.Parameters, &sc.Enabled, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, sc)
	}
	if items == nil {
		items = []models.SnapshotConfig{}
	}
	return items, nil
}

func (r *PgSnapshotConfigRepository) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.SnapshotConfig, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, source_id, software_id, name, snapshot_type, COALESCE(query_template,''), COALESCE(time_range_seconds,3600), COALESCE(parameters,'{}'::jsonb), enabled, created_at, updated_at
		 FROM snapshot_configs WHERE software_id=$1 ORDER BY created_at DESC`, softwareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.SnapshotConfig
	for rows.Next() {
		var sc models.SnapshotConfig
		if err := rows.Scan(&sc.ID, &sc.OrgID, &sc.SourceID, &sc.SoftwareID, &sc.Name, &sc.SnapshotType, &sc.QueryTemplate, &sc.TimeRangeSeconds, &sc.Parameters, &sc.Enabled, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, sc)
	}
	if items == nil {
		items = []models.SnapshotConfig{}
	}
	return items, nil
}

func (r *PgSnapshotConfigRepository) Update(ctx context.Context, sc *models.SnapshotConfig) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE snapshot_configs SET source_id=$1, software_id=$2, name=$3, snapshot_type=$4, query_template=$5, time_range_seconds=$6, parameters=$7, enabled=$8, updated_at=$9 WHERE id=$10`,
		sc.SourceID, sc.SoftwareID, sc.Name, sc.SnapshotType, sc.QueryTemplate, sc.TimeRangeSeconds, sc.Parameters, sc.Enabled, sc.UpdatedAt, sc.ID)
	return err
}

func (r *PgSnapshotConfigRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM snapshot_configs WHERE id=$1`, id)
	return err
}
