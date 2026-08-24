package database

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// PgSLORepository persists slo_definitions (CRUD) and, separately, computes
// incident-derived downtime data needed by the SLO service. It is
// intentionally standalone (not part of repositories.go) so it can be added
// without touching the shared incident/software repositories owned by other
// in-flight work. It only reads from `incidents` (no writes) for the
// downtime query.
type PgSLORepository struct{ pool *pgxpool.Pool }

func NewSLORepository(pool *pgxpool.Pool) *PgSLORepository {
	return &PgSLORepository{pool: pool}
}

// --- slo_definitions CRUD ---

func (r *PgSLORepository) Create(ctx context.Context, s *models.SLODefinition) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO slo_definitions (id, org_id, software_id, name, slo_type, target_percentage, measurement_window_days, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		s.ID, s.OrgID, s.SoftwareID, s.Name, s.SLOType, s.TargetPercentage, s.MeasurementWindowDays, s.CreatedAt, s.UpdatedAt)
	return err
}

func (r *PgSLORepository) GetByID(ctx context.Context, id uuid.UUID) (*models.SLODefinition, error) {
	var s models.SLODefinition
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, software_id, name, slo_type, target_percentage, measurement_window_days, created_at, updated_at
		 FROM slo_definitions WHERE id=$1`, id).
		Scan(&s.ID, &s.OrgID, &s.SoftwareID, &s.Name, &s.SLOType, &s.TargetPercentage, &s.MeasurementWindowDays, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *PgSLORepository) List(ctx context.Context, orgID uuid.UUID) ([]models.SLODefinition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, software_id, name, slo_type, target_percentage, measurement_window_days, created_at, updated_at
		 FROM slo_definitions WHERE org_id=$1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.SLODefinition
	for rows.Next() {
		var s models.SLODefinition
		if err := rows.Scan(&s.ID, &s.OrgID, &s.SoftwareID, &s.Name, &s.SLOType, &s.TargetPercentage, &s.MeasurementWindowDays, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	if items == nil {
		items = []models.SLODefinition{}
	}
	return items, rows.Err()
}

func (r *PgSLORepository) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.SLODefinition, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, software_id, name, slo_type, target_percentage, measurement_window_days, created_at, updated_at
		 FROM slo_definitions WHERE software_id=$1 ORDER BY created_at DESC`, softwareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.SLODefinition
	for rows.Next() {
		var s models.SLODefinition
		if err := rows.Scan(&s.ID, &s.OrgID, &s.SoftwareID, &s.Name, &s.SLOType, &s.TargetPercentage, &s.MeasurementWindowDays, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	if items == nil {
		items = []models.SLODefinition{}
	}
	return items, rows.Err()
}

func (r *PgSLORepository) Update(ctx context.Context, s *models.SLODefinition) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE slo_definitions SET name=$1, slo_type=$2, target_percentage=$3, measurement_window_days=$4, updated_at=$5 WHERE id=$6`,
		s.Name, s.SLOType, s.TargetPercentage, s.MeasurementWindowDays, s.UpdatedAt, s.ID)
	return err
}

func (r *PgSLORepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM slo_definitions WHERE id=$1`, id)
	return err
}

// --- Incident-derived downtime signal ---

// GetIncidentDowntimeMinutes sums, over incidents for the given org+software
// that overlap [windowStart, windowEnd], the portion of each incident's
// lifetime (created_at -> resolved_at, or created_at -> now if still open)
// that falls inside the window. This is a deliberate approximation: it sums
// per-incident overlaps rather than computing the union of overlapping
// incident intervals, so two concurrent incidents on the same service will
// double-count their overlapping minutes. In practice concurrent incidents
// on a single service are rare, and this keeps the query simple; documented
// here and in slo_service.go per the task's request for an explicit,
// reasonable interpretation rather than a silently-wrong precise-looking
// number.
//
// It considers incidents of any severity/status "downtime" for the duration
// they were open (i.e. from created_at until resolved_at, or until now if
// still unresolved) -- this is the "total minutes where an open incident
// existed for that software within the window" interpretation.
func (r *PgSLORepository) GetIncidentDowntimeMinutes(ctx context.Context, orgID, softwareID uuid.UUID, windowStart, windowEnd time.Time) (float64, int, error) {
	var downtimeMinutes float64
	var incidentCount int
	err := r.pool.QueryRow(ctx,
		`SELECT
		    COALESCE(SUM(
		        GREATEST(0, EXTRACT(EPOCH FROM (
		            LEAST(COALESCE(resolved_at, NOW()), $4) - GREATEST(created_at, $3)
		        )) / 60.0)
		    ), 0) AS downtime_minutes,
		    COUNT(*)::int AS incident_count
		 FROM incidents
		 WHERE org_id = $1
		   AND software_id = $2
		   AND created_at < $4
		   AND COALESCE(resolved_at, NOW()) > $3`,
		orgID, softwareID, windowStart, windowEnd,
	).Scan(&downtimeMinutes, &incidentCount)
	if err != nil {
		return 0, 0, err
	}
	return downtimeMinutes, incidentCount, nil
}
