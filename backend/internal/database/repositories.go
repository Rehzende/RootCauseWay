package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Rehzende/RootCauseway/backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// --- Software Repository ---

type PgSoftwareRepository struct{ pool *pgxpool.Pool }

func NewSoftwareRepository(pool *pgxpool.Pool) *PgSoftwareRepository {
	return &PgSoftwareRepository{pool: pool}
}

const softwareColumns = `id, org_id, name, slug, COALESCE(description,''), owner_id, COALESCE(repository_url,''), COALESCE(tags,'[]'::jsonb), status,
		 COALESCE(pipeline_url,''), COALESCE(cloud_provider,''), COALESCE(cloud_resources,'{}'::jsonb), COALESCE(database_info,'{}'::jsonb), COALESCE(infra_details,'{}'::jsonb), COALESCE(stakeholders,'[]'::jsonb), COALESCE(sre_team,'[]'::jsonb), COALESCE(architects,'[]'::jsonb), COALESCE(runbook_url,''), COALESCE(dashboard_url,''), COALESCE(dependencies,'[]'::jsonb),
		 COALESCE(criticality,'medium'), COALESCE(type,'service'),
		 created_at, updated_at`

// scanSoftwareRow scans one software_catalog row (as selected by
// softwareColumns) via a pgx row-like scanner (*pgx.Row or pgx.Rows both
// satisfy this).
func scanSoftwareRow(scan func(dest ...any) error) (*models.SoftwareEntry, error) {
	var e models.SoftwareEntry
	var tags []byte
	if err := scan(&e.ID, &e.OrgID, &e.Name, &e.Slug, &e.Description, &e.OwnerID, &e.RepositoryURL, &tags, &e.Status,
		&e.PipelineURL, &e.CloudProvider, &e.CloudResources, &e.DatabaseInfo, &e.InfraDetails, &e.Stakeholders, &e.SreTeam, &e.Architects, &e.RunbookURL, &e.DashboardURL, &e.Dependencies,
		&e.Criticality, &e.Type,
		&e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal(tags, &e.Tags)
	if e.Tags == nil {
		e.Tags = []string{}
	}
	return &e, nil
}

func (r *PgSoftwareRepository) Create(ctx context.Context, e *models.SoftwareEntry) error {
	tags, _ := json.Marshal(e.Tags)
	_, err := r.pool.Exec(ctx,
		`INSERT INTO software_catalog (id, org_id, name, slug, description, owner_id, repository_url, tags, status, pipeline_url, cloud_provider, cloud_resources, database_info, infra_details, stakeholders, sre_team, architects, runbook_url, dashboard_url, dependencies, criticality, type, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)`,
		e.ID, e.OrgID, e.Name, e.Slug, e.Description, e.OwnerID, e.RepositoryURL, tags, e.Status,
		e.PipelineURL, e.CloudProvider, e.CloudResources, e.DatabaseInfo, e.InfraDetails, e.Stakeholders, e.SreTeam, e.Architects, e.RunbookURL, e.DashboardURL, e.Dependencies,
		e.Criticality, e.Type,
		e.CreatedAt, e.UpdatedAt)
	return err
}

func (r *PgSoftwareRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.SoftwareEntry, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+softwareColumns+` FROM software_catalog WHERE id=$1`, id)
	return scanSoftwareRow(row.Scan)
}

func (r *PgSoftwareRepository) FindBySlugOrTag(ctx context.Context, orgID uuid.UUID, label string) (*models.SoftwareEntry, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+softwareColumns+`
		 FROM software_catalog WHERE org_id=$1 AND (slug=$2 OR tags @> $3::jsonb)
		 LIMIT 1`, orgID, label, fmt.Sprintf(`[%q]`, label))
	return scanSoftwareRow(row.Scan)
}

func (r *PgSoftwareRepository) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.SoftwareEntry, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM software_catalog WHERE org_id=$1`, orgID).Scan(&total)

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx,
		`SELECT `+softwareColumns+`
		 FROM software_catalog WHERE org_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, orgID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.SoftwareEntry
	for rows.Next() {
		e, err := scanSoftwareRow(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *e)
	}
	if items == nil {
		items = []models.SoftwareEntry{}
	}
	return items, total, nil
}

func (r *PgSoftwareRepository) Update(ctx context.Context, e *models.SoftwareEntry) error {
	tags, _ := json.Marshal(e.Tags)
	_, err := r.pool.Exec(ctx,
		`UPDATE software_catalog SET name=$1, slug=$2, description=$3, owner_id=$4, repository_url=$5, tags=$6, status=$7,
		 pipeline_url=$8, cloud_provider=$9, cloud_resources=$10, database_info=$11, infra_details=$12, stakeholders=$13, sre_team=$14, architects=$15, runbook_url=$16, dashboard_url=$17, dependencies=$18,
		 criticality=$19, type=$20,
		 updated_at=$21 WHERE id=$22`,
		e.Name, e.Slug, e.Description, e.OwnerID, e.RepositoryURL, tags, e.Status,
		e.PipelineURL, e.CloudProvider, e.CloudResources, e.DatabaseInfo, e.InfraDetails, e.Stakeholders, e.SreTeam, e.Architects, e.RunbookURL, e.DashboardURL, e.Dependencies,
		e.Criticality, e.Type,
		e.UpdatedAt, e.ID)
	return err
}

func (r *PgSoftwareRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM software_catalog WHERE id=$1`, id)
	return err
}

// ListDependents returns software catalog entries in the org whose declared
// `dependencies` array contains an entry for the given slug -- i.e. services
// that depend on (are downstream of) the service identified by slug. Used
// for dependency-graph cascade correlation. Matches regardless of the
// dependency's `relation` value: jsonb `@>` containment on
// `[{"slug": "..."}]` matches any array element whose object is a superset
// of that (i.e. has that slug, whatever its relation).
func (r *PgSoftwareRepository) ListDependents(ctx context.Context, orgID uuid.UUID, slug string) ([]models.SoftwareEntry, error) {
	dep, err := json.Marshal([]map[string]string{{"slug": slug}})
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+softwareColumns+`
		 FROM software_catalog WHERE org_id=$1 AND dependencies @> $2::jsonb`, orgID, dep)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.SoftwareEntry
	for rows.Next() {
		e, err := scanSoftwareRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, *e)
	}
	if items == nil {
		items = []models.SoftwareEntry{}
	}
	return items, nil
}

// --- Agent Repository ---

type PgAgentRepository struct{ pool *pgxpool.Pool }

func NewAgentRepository(pool *pgxpool.Pool) *PgAgentRepository {
	return &PgAgentRepository{pool: pool}
}

func (r *PgAgentRepository) Create(ctx context.Context, a *models.Agent) error {
	config, _ := json.Marshal(a.Config)
	_, err := r.pool.Exec(ctx,
		`INSERT INTO agents (id, org_id, name, type, description, config, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		a.ID, a.OrgID, a.Name, a.Type, a.Description, config, a.Enabled, a.CreatedAt, a.UpdatedAt)
	return err
}

func (r *PgAgentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Agent, error) {
	var a models.Agent
	var config []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, type, description, config, enabled, created_at, updated_at FROM agents WHERE id=$1`, id).
		Scan(&a.ID, &a.OrgID, &a.Name, &a.Type, &a.Description, &config, &a.Enabled, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(config, &a.Config)
	return &a, nil
}

func (r *PgAgentRepository) List(ctx context.Context, orgID uuid.UUID, agentType string, page, perPage int) ([]models.Agent, int, error) {
	var total int
	query := `SELECT COUNT(*) FROM agents WHERE org_id=$1`
	args := []interface{}{orgID}
	if agentType != "" {
		query += ` AND type=$2`
		args = append(args, agentType)
	}
	_ = r.pool.QueryRow(ctx, query, args...).Scan(&total)

	offset := (page - 1) * perPage
	selectQ := `SELECT id, org_id, name, type, description, config, enabled, created_at, updated_at FROM agents WHERE org_id=$1`
	selectArgs := []interface{}{orgID}
	if agentType != "" {
		selectQ += ` AND type=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		selectArgs = append(selectArgs, agentType, perPage, offset)
	} else {
		selectQ += ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		selectArgs = append(selectArgs, perPage, offset)
	}

	rows, err := r.pool.Query(ctx, selectQ, selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.Agent
	for rows.Next() {
		var a models.Agent
		var config []byte
		if err := rows.Scan(&a.ID, &a.OrgID, &a.Name, &a.Type, &a.Description, &config, &a.Enabled, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, err
		}
		_ = json.Unmarshal(config, &a.Config)
		items = append(items, a)
	}
	if items == nil {
		items = []models.Agent{}
	}
	return items, total, nil
}

func (r *PgAgentRepository) Update(ctx context.Context, a *models.Agent) error {
	config, _ := json.Marshal(a.Config)
	_, err := r.pool.Exec(ctx,
		`UPDATE agents SET name=$1, type=$2, description=$3, config=$4, enabled=$5, updated_at=$6 WHERE id=$7`,
		a.Name, a.Type, a.Description, config, a.Enabled, a.UpdatedAt, a.ID)
	return err
}

func (r *PgAgentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM agents WHERE id=$1`, id)
	return err
}

// --- Webhook Repository ---

type PgWebhookRepository struct{ pool *pgxpool.Pool }

func NewWebhookRepository(pool *pgxpool.Pool) *PgWebhookRepository {
	return &PgWebhookRepository{pool: pool}
}

func (r *PgWebhookRepository) Create(ctx context.Context, w *models.Webhook) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO webhooks (id, org_id, name, source, software_id, endpoint_token, secret, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		w.ID, w.OrgID, w.Name, w.Source, w.SoftwareID, w.EndpointToken, w.Secret, w.Enabled, w.CreatedAt, w.UpdatedAt)
	return err
}

func (r *PgWebhookRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
	var w models.Webhook
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, source, software_id, endpoint_token, secret, enabled, created_at, updated_at FROM webhooks WHERE id=$1`, id).
		Scan(&w.ID, &w.OrgID, &w.Name, &w.Source, &w.SoftwareID, &w.EndpointToken, &w.Secret, &w.Enabled, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *PgWebhookRepository) GetByToken(ctx context.Context, token string) (*models.Webhook, error) {
	var w models.Webhook
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, source, COALESCE(software_id, '00000000-0000-0000-0000-000000000000'), endpoint_token, secret, enabled, created_at, updated_at FROM webhooks WHERE endpoint_token=$1`, token).
		Scan(&w.ID, &w.OrgID, &w.Name, &w.Source, &w.SoftwareID, &w.EndpointToken, &w.Secret, &w.Enabled, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *PgWebhookRepository) List(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.Webhook, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhooks WHERE org_id=$1`, orgID).Scan(&total)

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, source, software_id, endpoint_token, secret, enabled, created_at, updated_at
		 FROM webhooks WHERE org_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, orgID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.Webhook
	for rows.Next() {
		var w models.Webhook
		if err := rows.Scan(&w.ID, &w.OrgID, &w.Name, &w.Source, &w.SoftwareID, &w.EndpointToken, &w.Secret, &w.Enabled, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, w)
	}
	if items == nil {
		items = []models.Webhook{}
	}
	return items, total, nil
}

func (r *PgWebhookRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM webhooks WHERE id=$1`, id)
	return err
}

// --- Incident Repository ---

type PgIncidentRepository struct{ pool *pgxpool.Pool }

func NewIncidentRepository(pool *pgxpool.Pool) *PgIncidentRepository {
	return &PgIncidentRepository{pool: pool}
}

// Create assigns the incident a sequential per-org number (see
// FormatIncidentCode / migration 030_incident_number) and inserts the row
// inside one transaction: the UPDATE ... RETURNING against
// organizations.next_incident_number takes a row lock, so two concurrent
// Create calls for the same org can never be handed the same number --
// Postgres serializes them.
func (r *PgIncidentRepository) Create(ctx context.Context, i *models.Incident) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful Commit

	if err := tx.QueryRow(ctx,
		`UPDATE organizations SET next_incident_number = next_incident_number + 1
		 WHERE id = $1 RETURNING next_incident_number - 1`,
		i.OrgID,
	).Scan(&i.IncidentNumber); err != nil {
		return fmt.Errorf("reserve incident number: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO incidents (id, org_id, software_id, incident_number, title, description, severity, status, assignee_id, source_alert_id, root_cause, mitigation, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		i.ID, i.OrgID, i.SoftwareID, i.IncidentNumber, i.Title, i.Description, i.Severity, i.Status, i.AssigneeID, i.SourceAlertID, i.RootCause, i.Mitigation, i.CreatedAt, i.UpdatedAt,
	); err != nil {
		return fmt.Errorf("insert incident: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *PgIncidentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Incident, error) {
	var i models.Incident
	err := r.pool.QueryRow(ctx,
		`SELECT id, incident_number, org_id, software_id, title, COALESCE(description,''), severity, status, assignee_id, COALESCE(source_alert_id,''), COALESCE(root_cause,''), COALESCE(mitigation,''), created_at, updated_at, resolved_at
		 FROM incidents WHERE id=$1`, id).
		Scan(&i.ID, &i.IncidentNumber, &i.OrgID, &i.SoftwareID, &i.Title, &i.Description, &i.Severity, &i.Status, &i.AssigneeID, &i.SourceAlertID, &i.RootCause, &i.Mitigation, &i.CreatedAt, &i.UpdatedAt, &i.ResolvedAt)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *PgIncidentRepository) List(ctx context.Context, orgID uuid.UUID, status, severity string, softwareID *uuid.UUID, from *time.Time, page, perPage int) ([]models.Incident, int, error) {
	baseWhere := `WHERE org_id=$1`
	args := []interface{}{orgID}
	idx := 2

	if status != "" {
		baseWhere += fmt.Sprintf(` AND status=$%d`, idx)
		args = append(args, status)
		idx++
	}
	if severity != "" {
		baseWhere += fmt.Sprintf(` AND severity=$%d`, idx)
		args = append(args, severity)
		idx++
	}
	if softwareID != nil {
		baseWhere += fmt.Sprintf(` AND software_id=$%d`, idx)
		args = append(args, *softwareID)
		idx++
	}
	if from != nil {
		baseWhere += fmt.Sprintf(` AND created_at >= $%d`, idx)
		args = append(args, *from)
		idx++
	}

	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM incidents `+baseWhere, args...).Scan(&total)

	offset := (page - 1) * perPage
	selectArgs := append(args, perPage, offset)
	rows, err := r.pool.Query(ctx,
		fmt.Sprintf(`SELECT id, incident_number, org_id, software_id, title, COALESCE(description,''), severity, status, assignee_id, COALESCE(source_alert_id,''), COALESCE(root_cause,''), COALESCE(mitigation,''), created_at, updated_at, resolved_at
		 FROM incidents %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, baseWhere, idx, idx+1), selectArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.Incident
	for rows.Next() {
		var i models.Incident
		if err := rows.Scan(&i.ID, &i.IncidentNumber, &i.OrgID, &i.SoftwareID, &i.Title, &i.Description, &i.Severity, &i.Status, &i.AssigneeID, &i.SourceAlertID, &i.RootCause, &i.Mitigation, &i.CreatedAt, &i.UpdatedAt, &i.ResolvedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, i)
	}
	if items == nil {
		items = []models.Incident{}
	}
	return items, total, nil
}

func (r *PgIncidentRepository) Update(ctx context.Context, i *models.Incident) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE incidents SET title=$1, description=$2, severity=$3, status=$4, assignee_id=$5, root_cause=$6, mitigation=$7, updated_at=$8, resolved_at=$9 WHERE id=$10`,
		i.Title, i.Description, i.Severity, i.Status, i.AssigneeID, i.RootCause, i.Mitigation, i.UpdatedAt, i.ResolvedAt, i.ID)
	return err
}

// Delete removes an incident and everything under it. Most
// incident-referencing tables cascade automatically, but four are
// ON DELETE NO ACTION by design (credential_leases, knowledge_base,
// notification_log, runbook_executions -- audit/operational trails not
// meant to silently vanish just because the incident that spawned them
// did) plus similar_incidents.similar_incident_id (another incident's
// "this looked like X" pointer at this one) -- all cleared explicitly
// first, in the same transaction, or the FK constraint blocks the delete
// outright.
func (r *PgIncidentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after a successful Commit

	for _, stmt := range []string{
		`DELETE FROM credential_leases WHERE incident_id = $1`,
		`DELETE FROM knowledge_base WHERE incident_id = $1`,
		`DELETE FROM notification_log WHERE incident_id = $1`,
		`DELETE FROM runbook_executions WHERE incident_id = $1`,
		`DELETE FROM similar_incidents WHERE similar_incident_id = $1`,
	} {
		if _, err := tx.Exec(ctx, stmt, id); err != nil {
			return fmt.Errorf("clear NO ACTION dependents: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM incidents WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete incident: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *PgIncidentRepository) AddEvent(ctx context.Context, e *models.IncidentEvent) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO incident_events (id, incident_id, type, actor, data, created_at) VALUES ($1,$2,$3,$4,$5,$6)`,
		e.ID, e.IncidentID, e.Type, e.Actor, e.Data, e.CreatedAt)
	return err
}

func (r *PgIncidentRepository) AddEvidence(ctx context.Context, e *models.IncidentEvidence) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO incident_evidence (id, incident_id, type, title, content, source, collected_at, blob_path, blob_size_bytes, mime_type) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		e.ID, e.IncidentID, e.Type, e.Title, e.Content, e.Source, e.CollectedAt, e.BlobPath, e.BlobSizeBytes, e.MimeType)
	return err
}

func (r *PgIncidentRepository) GetEvents(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, incident_id, type, actor, data, created_at FROM incident_events WHERE incident_id=$1 ORDER BY created_at`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.IncidentEvent
	for rows.Next() {
		var e models.IncidentEvent
		if err := rows.Scan(&e.ID, &e.IncidentID, &e.Type, &e.Actor, &e.Data, &e.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, nil
}

// ListOpenBySoftwareIDs returns non-resolved/closed incidents on any of the given
// software ids, created within the recency window. Used for dependency-graph
// cascade correlation (checking open incidents on upstream/downstream services).
func (r *PgIncidentRepository) ListOpenBySoftwareIDs(ctx context.Context, orgID uuid.UUID, softwareIDs []uuid.UUID, since time.Time) ([]models.Incident, error) {
	if len(softwareIDs) == 0 {
		return []models.Incident{}, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, software_id, title, COALESCE(description,''), severity, status, assignee_id, COALESCE(source_alert_id,''), COALESCE(root_cause,''), COALESCE(mitigation,''), created_at, updated_at, resolved_at
		 FROM incidents
		 WHERE org_id=$1 AND software_id = ANY($2) AND status NOT IN ('resolved', 'closed') AND created_at >= $3
		 ORDER BY created_at DESC`, orgID, softwareIDs, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Incident
	for rows.Next() {
		var i models.Incident
		if err := rows.Scan(&i.ID, &i.OrgID, &i.SoftwareID, &i.Title, &i.Description, &i.Severity, &i.Status, &i.AssigneeID, &i.SourceAlertID, &i.RootCause, &i.Mitigation, &i.CreatedAt, &i.UpdatedAt, &i.ResolvedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if items == nil {
		items = []models.Incident{}
	}
	return items, nil
}

// FindByFingerprint returns the incident behind the most recent alert snapshot
// carrying the given fingerprint, or nil if none is found. Used for
// literal-repeat alert dedup. Matches when EITHER the alert snapshot is
// within the recency window OR the incident is still unresolved -- a
// flapping alert (fires, resolves, fires again) whose gaps exceed the
// window must keep landing on the same incident for as long as that
// incident stays open, otherwise a slow-flapping root cause spawns a new
// incident (and re-runs the whole agent pipeline) every cycle instead of
// just adding a timeline event to the one already tracking it. Found live:
// a stale LLM endpoint made k8s-agent's swallowed-error alert flap on a
// 10-30min cadence, wider than the old 15min window, creating 20+ duplicate
// incidents before this was caught.
func (r *PgIncidentRepository) FindByFingerprint(ctx context.Context, orgID uuid.UUID, fingerprint string, since time.Time) (*models.Incident, error) {
	var i models.Incident
	err := r.pool.QueryRow(ctx,
		`SELECT i.id, i.org_id, i.software_id, i.title, COALESCE(i.description,''), i.severity, i.status, i.assignee_id, COALESCE(i.source_alert_id,''), COALESCE(i.root_cause,''), COALESCE(i.mitigation,''), i.created_at, i.updated_at, i.resolved_at
		 FROM incidents i
		 JOIN alert_snapshots a ON a.incident_id = i.id
		 WHERE i.org_id=$1 AND a.normalized->>'fingerprint' = $2
		   AND (a.created_at >= $3 OR i.status NOT IN ('resolved', 'closed'))
		 ORDER BY a.created_at DESC
		 LIMIT 1`, orgID, fingerprint, since).
		Scan(&i.ID, &i.OrgID, &i.SoftwareID, &i.Title, &i.Description, &i.Severity, &i.Status, &i.AssigneeID, &i.SourceAlertID, &i.RootCause, &i.Mitigation, &i.CreatedAt, &i.UpdatedAt, &i.ResolvedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &i, nil
}

func (r *PgIncidentRepository) GetEvidence(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentEvidence, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, incident_id, type, title, content, COALESCE(source,''), collected_at, COALESCE(blob_path,''), COALESCE(blob_size_bytes,0), COALESCE(mime_type,'') FROM incident_evidence WHERE incident_id=$1 ORDER BY collected_at`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.IncidentEvidence
	for rows.Next() {
		var e models.IncidentEvidence
		if err := rows.Scan(&e.ID, &e.IncidentID, &e.Type, &e.Title, &e.Content, &e.Source, &e.CollectedAt, &e.BlobPath, &e.BlobSizeBytes, &e.MimeType); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, nil
}

// GetIncidentStatsBySoftware returns the total incident count, open
// (non-resolved/closed) incident count, and the most recent incident's
// created_at for a software catalog entry -- feeds the catalog's
// completeness/health summary (see SoftwareSummaryHandler) so "how healthy
// is this service" doesn't require a separate trip to the incidents list.
func (r *PgIncidentRepository) GetIncidentStatsBySoftware(ctx context.Context, orgID, softwareID uuid.UUID) (total, open int, lastIncidentAt *time.Time, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT
		   COUNT(*),
		   COUNT(*) FILTER (WHERE status NOT IN ('resolved', 'closed')),
		   MAX(created_at)
		 FROM incidents WHERE org_id=$1 AND software_id=$2`,
		orgID, softwareID,
	).Scan(&total, &open, &lastIncidentAt)
	return total, open, lastIncidentAt, err
}

// --- Alert Quarantine Repository ---

type PgAlertQuarantineRepository struct{ pool *pgxpool.Pool }

func NewAlertQuarantineRepository(pool *pgxpool.Pool) *PgAlertQuarantineRepository {
	return &PgAlertQuarantineRepository{pool: pool}
}

func (r *PgAlertQuarantineRepository) Create(ctx context.Context, q *models.AlertQuarantine) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO alert_quarantine (id, org_id, webhook_id, source, raw_payload, normalized_title, normalized_severity, labels, reason, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		q.ID, q.OrgID, q.WebhookID, q.Source, q.RawPayload, q.NormalizedTitle, q.NormalizedSeverity, q.Labels, q.Reason, q.CreatedAt)
	return err
}

func (r *PgAlertQuarantineRepository) List(ctx context.Context, orgID uuid.UUID, resolved bool, page, perPage int) ([]models.AlertQuarantine, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM alert_quarantine WHERE org_id=$1 AND resolved=$2`, orgID, resolved).Scan(&total)

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, webhook_id, source, raw_payload, normalized_title, normalized_severity, COALESCE(labels,'{}'), reason, resolved, resolved_at, resolved_software_id, created_at
		 FROM alert_quarantine WHERE org_id=$1 AND resolved=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`,
		orgID, resolved, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.AlertQuarantine
	for rows.Next() {
		var q models.AlertQuarantine
		if err := rows.Scan(&q.ID, &q.OrgID, &q.WebhookID, &q.Source, &q.RawPayload, &q.NormalizedTitle, &q.NormalizedSeverity, &q.Labels, &q.Reason, &q.Resolved, &q.ResolvedAt, &q.ResolvedSoftwareID, &q.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, q)
	}
	if items == nil {
		items = []models.AlertQuarantine{}
	}
	return items, total, nil
}

func (r *PgAlertQuarantineRepository) Resolve(ctx context.Context, id uuid.UUID, softwareID uuid.UUID) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE alert_quarantine SET resolved=true, resolved_at=$2, resolved_software_id=$3 WHERE id=$1`,
		id, now, softwareID)
	return err
}

func (r *PgAlertQuarantineRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AlertQuarantine, error) {
	var q models.AlertQuarantine
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, webhook_id, source, raw_payload, normalized_title, normalized_severity, COALESCE(labels,'{}'), reason, resolved, resolved_at, resolved_software_id, created_at
		 FROM alert_quarantine WHERE id=$1`, id).
		Scan(&q.ID, &q.OrgID, &q.WebhookID, &q.Source, &q.RawPayload, &q.NormalizedTitle, &q.NormalizedSeverity, &q.Labels, &q.Reason, &q.Resolved, &q.ResolvedAt, &q.ResolvedSoftwareID, &q.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// --- Alert Snapshot Repository ---

type PgAlertSnapshotRepository struct{ pool *pgxpool.Pool }

func NewAlertSnapshotRepository(pool *pgxpool.Pool) *PgAlertSnapshotRepository {
	return &PgAlertSnapshotRepository{pool: pool}
}

func (r *PgAlertSnapshotRepository) Create(ctx context.Context, s *models.AlertSnapshot) error {
	normalized, _ := json.Marshal(s.Normalized)
	_, err := r.pool.Exec(ctx,
		`INSERT INTO alert_snapshots (id, incident_id, software_id, raw_payload, normalized, snapshots, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		s.ID, s.IncidentID, s.SoftwareID, s.RawPayload, normalized, s.Snapshots, s.CreatedAt)
	return err
}

// --- Redis Event Publisher ---

type RedisEventPublisher struct {
	rdb          eventRedisClient
	streamName   string
	streamMaxLen int64
}

func NewRedisEventPublisher(rdb *redis.Client) *RedisEventPublisher {
	return &RedisEventPublisher{
		rdb:          rdb,
		streamName:   DefaultEventStream,
		streamMaxLen: DefaultEventStreamMaxLen,
	}
}

// --- Agent Run Repository ---

type PgAgentRunRepository struct{ pool *pgxpool.Pool }

func NewAgentRunRepository(pool *pgxpool.Pool) *PgAgentRunRepository {
	return &PgAgentRunRepository{pool: pool}
}

func (r *PgAgentRunRepository) Create(ctx context.Context, a *models.AgentRun) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO agent_runs (id, incident_id, agent_id, agent_name, agent_type, status, parent_run_id, input_data, output_data, error_message, model_used, tokens_used, duration_ms, started_at, completed_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		a.ID, a.IncidentID, a.AgentID, a.AgentName, a.AgentType, a.Status, a.ParentRunID, a.InputData, a.OutputData, a.ErrorMessage, a.ModelUsed, a.TokensUsed, a.DurationMs, a.StartedAt, a.CompletedAt, a.CreatedAt)
	return err
}

func (r *PgAgentRunRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AgentRun, error) {
	var a models.AgentRun
	err := r.pool.QueryRow(ctx,
		`SELECT id, incident_id, agent_id, agent_name, agent_type, status, parent_run_id, COALESCE(input_data,'{}'::jsonb), COALESCE(output_data,'{}'::jsonb), COALESCE(error_message,''), COALESCE(model_used,''), COALESCE(tokens_used,0), COALESCE(duration_ms,0), started_at, completed_at, created_at
		 FROM agent_runs WHERE id=$1`, id).
		Scan(&a.ID, &a.IncidentID, &a.AgentID, &a.AgentName, &a.AgentType, &a.Status, &a.ParentRunID, &a.InputData, &a.OutputData, &a.ErrorMessage, &a.ModelUsed, &a.TokensUsed, &a.DurationMs, &a.StartedAt, &a.CompletedAt, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *PgAgentRunRepository) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.AgentRun, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, incident_id, agent_id, agent_name, agent_type, status, parent_run_id, COALESCE(input_data,'{}'::jsonb), COALESCE(output_data,'{}'::jsonb), COALESCE(error_message,''), COALESCE(model_used,''), COALESCE(tokens_used,0), COALESCE(duration_ms,0), started_at, completed_at, created_at
		 FROM agent_runs WHERE incident_id=$1 ORDER BY created_at`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.AgentRun
	for rows.Next() {
		var a models.AgentRun
		if err := rows.Scan(&a.ID, &a.IncidentID, &a.AgentID, &a.AgentName, &a.AgentType, &a.Status, &a.ParentRunID, &a.InputData, &a.OutputData, &a.ErrorMessage, &a.ModelUsed, &a.TokensUsed, &a.DurationMs, &a.StartedAt, &a.CompletedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	if items == nil {
		items = []models.AgentRun{}
	}
	return items, nil
}

func (r *PgAgentRunRepository) Update(ctx context.Context, a *models.AgentRun) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE agent_runs SET status=$1, output_data=$2, error_message=$3, model_used=$4, tokens_used=$5, duration_ms=$6, completed_at=$7 WHERE id=$8`,
		a.Status, a.OutputData, a.ErrorMessage, a.ModelUsed, a.TokensUsed, a.DurationMs, a.CompletedAt, a.ID)
	return err
}

func (r *PgAgentRunRepository) GetDAG(ctx context.Context, incidentID uuid.UUID) ([]models.AgentRun, error) {
	return r.ListByIncident(ctx, incidentID)
}

// --- RCI Repository ---

type PgRCIRepository struct{ pool *pgxpool.Pool }

func NewRCIRepository(pool *pgxpool.Pool) *PgRCIRepository {
	return &PgRCIRepository{pool: pool}
}

func (r *PgRCIRepository) Create(ctx context.Context, rci *models.IncidentRCI) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO incident_rci (id, incident_id, agent_run_id, status, investigation_summary, impact_assessment, affected_services, affected_users_estimate, detection_method, detection_time, acknowledgment_time, time_to_detect_seconds, evidence_ids, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		rci.ID, rci.IncidentID, rci.AgentRunID, rci.Status, rci.InvestigationSummary, rci.ImpactAssessment, rci.AffectedServices, rci.AffectedUsersEstimate, rci.DetectionMethod, rci.DetectionTime, rci.AcknowledgmentTime, rci.TimeToDetectSeconds, rci.EvidenceIDs, rci.CreatedAt, rci.UpdatedAt)
	return err
}

func (r *PgRCIRepository) GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentRCI, error) {
	var rci models.IncidentRCI
	err := r.pool.QueryRow(ctx,
		`SELECT id, incident_id, agent_run_id, status, COALESCE(investigation_summary,''), COALESCE(impact_assessment,'{}'::jsonb), COALESCE(affected_services,'[]'::jsonb), affected_users_estimate, COALESCE(detection_method,''), detection_time, acknowledgment_time, time_to_detect_seconds, COALESCE(evidence_ids,'[]'::jsonb), created_at, updated_at
		 FROM incident_rci WHERE incident_id=$1 ORDER BY created_at DESC LIMIT 1`, incidentID).
		Scan(&rci.ID, &rci.IncidentID, &rci.AgentRunID, &rci.Status, &rci.InvestigationSummary, &rci.ImpactAssessment, &rci.AffectedServices, &rci.AffectedUsersEstimate, &rci.DetectionMethod, &rci.DetectionTime, &rci.AcknowledgmentTime, &rci.TimeToDetectSeconds, &rci.EvidenceIDs, &rci.CreatedAt, &rci.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &rci, nil
}

func (r *PgRCIRepository) Update(ctx context.Context, rci *models.IncidentRCI) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE incident_rci SET status=$1, investigation_summary=$2, impact_assessment=$3, affected_services=$4, affected_users_estimate=$5, detection_method=$6, detection_time=$7, acknowledgment_time=$8, time_to_detect_seconds=$9, evidence_ids=$10, updated_at=$11 WHERE id=$12`,
		rci.Status, rci.InvestigationSummary, rci.ImpactAssessment, rci.AffectedServices, rci.AffectedUsersEstimate, rci.DetectionMethod, rci.DetectionTime, rci.AcknowledgmentTime, rci.TimeToDetectSeconds, rci.EvidenceIDs, rci.UpdatedAt, rci.ID)
	return err
}

// --- RCA Repository ---

type PgRCARepository struct{ pool *pgxpool.Pool }

func NewRCARepository(pool *pgxpool.Pool) *PgRCARepository {
	return &PgRCARepository{pool: pool}
}

func (r *PgRCARepository) Create(ctx context.Context, rca *models.IncidentRCA) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO incident_rca (id, incident_id, rci_id, agent_run_id, status, root_cause_summary, root_cause_category, contributing_factors, five_whys, confidence, evidence_ids, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		rca.ID, rca.IncidentID, rca.RCIID, rca.AgentRunID, rca.Status, rca.RootCauseSummary, rca.RootCauseCategory, rca.ContributingFactors, rca.FiveWhys, rca.Confidence, rca.EvidenceIDs, rca.CreatedAt, rca.UpdatedAt)
	return err
}

func (r *PgRCARepository) GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentRCA, error) {
	var rca models.IncidentRCA
	err := r.pool.QueryRow(ctx,
		`SELECT id, incident_id, rci_id, agent_run_id, status, COALESCE(root_cause_summary,''), COALESCE(root_cause_category,''), COALESCE(contributing_factors,'[]'::jsonb), COALESCE(five_whys,'[]'::jsonb), COALESCE(confidence,0), COALESCE(evidence_ids,'[]'::jsonb), created_at, updated_at
		 FROM incident_rca WHERE incident_id=$1 ORDER BY created_at DESC LIMIT 1`, incidentID).
		Scan(&rca.ID, &rca.IncidentID, &rca.RCIID, &rca.AgentRunID, &rca.Status, &rca.RootCauseSummary, &rca.RootCauseCategory, &rca.ContributingFactors, &rca.FiveWhys, &rca.Confidence, &rca.EvidenceIDs, &rca.CreatedAt, &rca.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &rca, nil
}

func (r *PgRCARepository) Update(ctx context.Context, rca *models.IncidentRCA) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE incident_rca SET status=$1, root_cause_summary=$2, root_cause_category=$3, contributing_factors=$4, five_whys=$5, confidence=$6, evidence_ids=$7, updated_at=$8 WHERE id=$9`,
		rca.Status, rca.RootCauseSummary, rca.RootCauseCategory, rca.ContributingFactors, rca.FiveWhys, rca.Confidence, rca.EvidenceIDs, rca.UpdatedAt, rca.ID)
	return err
}

// --- Postmortem Repository ---

type PgPostmortemRepository struct{ pool *pgxpool.Pool }

func NewPostmortemRepository(pool *pgxpool.Pool) *PgPostmortemRepository {
	return &PgPostmortemRepository{pool: pool}
}

func (r *PgPostmortemRepository) Create(ctx context.Context, pm *models.IncidentPostmortem) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO incident_postmortem (id, incident_id, rca_id, agent_run_id, status, title, executive_summary, incident_timeline_narrative, root_cause_detail, impact_detail, lessons_learned, action_items, what_went_well, what_went_wrong, prevention_measures, created_at, updated_at, published_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
		pm.ID, pm.IncidentID, pm.RootCausewayD, pm.AgentRunID, pm.Status, pm.Title, pm.ExecutiveSummary, pm.IncidentTimelineNarrative, pm.RootCauseDetail, pm.ImpactDetail, pm.LessonsLearned, pm.ActionItems, pm.WhatWentWell, pm.WhatWentWrong, pm.PreventionMeasures, pm.CreatedAt, pm.UpdatedAt, pm.PublishedAt)
	return err
}

func (r *PgPostmortemRepository) GetByIncidentID(ctx context.Context, incidentID uuid.UUID) (*models.IncidentPostmortem, error) {
	var pm models.IncidentPostmortem
	err := r.pool.QueryRow(ctx,
		`SELECT id, incident_id, rca_id, agent_run_id, status, COALESCE(title,''), COALESCE(executive_summary,''), COALESCE(incident_timeline_narrative,''), COALESCE(root_cause_detail,''), COALESCE(impact_detail,''), COALESCE(lessons_learned,'[]'::jsonb), COALESCE(action_items,'[]'::jsonb), COALESCE(what_went_well,'[]'::jsonb), COALESCE(what_went_wrong,'[]'::jsonb), COALESCE(prevention_measures,'[]'::jsonb), created_at, updated_at, published_at
		 FROM incident_postmortem WHERE incident_id=$1 ORDER BY created_at DESC LIMIT 1`, incidentID).
		Scan(&pm.ID, &pm.IncidentID, &pm.RootCausewayD, &pm.AgentRunID, &pm.Status, &pm.Title, &pm.ExecutiveSummary, &pm.IncidentTimelineNarrative, &pm.RootCauseDetail, &pm.ImpactDetail, &pm.LessonsLearned, &pm.ActionItems, &pm.WhatWentWell, &pm.WhatWentWrong, &pm.PreventionMeasures, &pm.CreatedAt, &pm.UpdatedAt, &pm.PublishedAt)
	if err != nil {
		return nil, err
	}
	return &pm, nil
}

func (r *PgPostmortemRepository) Update(ctx context.Context, pm *models.IncidentPostmortem) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE incident_postmortem SET status=$1, title=$2, executive_summary=$3, incident_timeline_narrative=$4, root_cause_detail=$5, impact_detail=$6, lessons_learned=$7, action_items=$8, what_went_well=$9, what_went_wrong=$10, prevention_measures=$11, updated_at=$12, published_at=$13 WHERE id=$14`,
		pm.Status, pm.Title, pm.ExecutiveSummary, pm.IncidentTimelineNarrative, pm.RootCauseDetail, pm.ImpactDetail, pm.LessonsLearned, pm.ActionItems, pm.WhatWentWell, pm.WhatWentWrong, pm.PreventionMeasures, pm.UpdatedAt, pm.PublishedAt, pm.ID)
	return err
}

// Publish dual-writes the event: durably to the rootcauseway:events Redis Stream
// (XADD, consumed by agent-service) and to the legacy pub/sub channel
// (kept for the WebSocket bridge). See stream_publisher.go.
func (p *RedisEventPublisher) Publish(ctx context.Context, channel string, event models.EventEnvelope) error {
	return p.publishDual(ctx, channel, event)
}
