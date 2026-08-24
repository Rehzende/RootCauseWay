package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// ExpiredRecord is a lightweight (id, JSON snapshot) pair returned by the
// Find* lookups below. The JSON snapshot is what gets written to
// archived_records.archived_data when a policy's action is "archive"; it is
// built directly from the SELECT rather than round-tripping through the
// full models.Incident / models.IncidentEvidence structs (which carry
// nested slices this repository doesn't load).
type ExpiredRecord struct {
	ID   uuid.UUID
	Data json.RawMessage
}

// PgRetentionRepository persists retention_policies / archived_records and
// implements the expired-record lookups the retention sweep needs. It is
// intentionally standalone (not part of repositories.go) so it can be added
// without touching the shared incident/evidence repositories owned by other
// in-flight work.
type PgRetentionRepository struct{ pool *pgxpool.Pool }

func NewRetentionRepository(pool *pgxpool.Pool) *PgRetentionRepository {
	return &PgRetentionRepository{pool: pool}
}

// --- Retention Policy CRUD ---

func (r *PgRetentionRepository) CreatePolicy(ctx context.Context, p *models.RetentionPolicy) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO retention_policies (id, org_id, resource_type, retention_days, action, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		p.ID, p.OrgID, p.ResourceType, p.RetentionDays, p.Action, p.Enabled, p.CreatedAt, p.UpdatedAt)
	return err
}

func (r *PgRetentionRepository) GetPolicy(ctx context.Context, id uuid.UUID) (*models.RetentionPolicy, error) {
	var p models.RetentionPolicy
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, resource_type, retention_days, action, enabled, created_at, updated_at
		 FROM retention_policies WHERE id=$1`, id).
		Scan(&p.ID, &p.OrgID, &p.ResourceType, &p.RetentionDays, &p.Action, &p.Enabled, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListPolicies returns every retention policy for an org, regardless of
// enabled state (used by the CRUD "list" endpoint).
func (r *PgRetentionRepository) ListPolicies(ctx context.Context, orgID uuid.UUID) ([]models.RetentionPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, resource_type, retention_days, action, enabled, created_at, updated_at
		 FROM retention_policies WHERE org_id=$1 ORDER BY resource_type`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.RetentionPolicy
	for rows.Next() {
		var p models.RetentionPolicy
		if err := rows.Scan(&p.ID, &p.OrgID, &p.ResourceType, &p.RetentionDays, &p.Action, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if items == nil {
		items = []models.RetentionPolicy{}
	}
	return items, nil
}

// ListEnabledPolicies returns only the enabled policies for an org (used by
// the sweep, which should skip disabled policies entirely).
func (r *PgRetentionRepository) ListEnabledPolicies(ctx context.Context, orgID uuid.UUID) ([]models.RetentionPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, resource_type, retention_days, action, enabled, created_at, updated_at
		 FROM retention_policies WHERE org_id=$1 AND enabled=true ORDER BY resource_type`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.RetentionPolicy
	for rows.Next() {
		var p models.RetentionPolicy
		if err := rows.Scan(&p.ID, &p.OrgID, &p.ResourceType, &p.RetentionDays, &p.Action, &p.Enabled, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	if items == nil {
		items = []models.RetentionPolicy{}
	}
	return items, nil
}

func (r *PgRetentionRepository) UpdatePolicy(ctx context.Context, p *models.RetentionPolicy) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE retention_policies SET retention_days=$1, action=$2, enabled=$3, updated_at=$4 WHERE id=$5`,
		p.RetentionDays, p.Action, p.Enabled, p.UpdatedAt, p.ID)
	return err
}

func (r *PgRetentionRepository) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM retention_policies WHERE id=$1`, id)
	return err
}

// ListOrgIDs returns every organization id, used by RunAllOrgsSweep to
// iterate orgs for a scheduled (future) sweep run.
func (r *PgRetentionRepository) ListOrgIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `SELECT id FROM organizations ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// --- Expired record lookups ---

// incidentSnapshot mirrors the subset of the incidents columns that get
// captured into archived_records.archived_data. It intentionally excludes
// nested timeline/evidence/agent-run data (those are archived separately,
// under their own resource_type, if/when their own policy expires them).
type incidentSnapshot struct {
	ID            uuid.UUID  `json:"id"`
	OrgID         uuid.UUID  `json:"org_id"`
	SoftwareID    uuid.UUID  `json:"software_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Severity      string     `json:"severity"`
	Status        string     `json:"status"`
	SourceAlertID string     `json:"source_alert_id"`
	RootCause     string     `json:"root_cause"`
	Mitigation    string     `json:"mitigation"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
}

// FindExpiredIncidents returns resolved/closed incidents for orgID whose
// resolved_at is older than olderThanDays. Only incidents in a terminal
// state are eligible -- an incident stuck open for a long time is never
// swept, regardless of age.
func (r *PgRetentionRepository) FindExpiredIncidents(ctx context.Context, orgID uuid.UUID, olderThanDays int) ([]ExpiredRecord, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, software_id, title, COALESCE(description,''), severity, status,
		        COALESCE(source_alert_id,''), COALESCE(root_cause,''), COALESCE(mitigation,''),
		        created_at, updated_at, resolved_at
		 FROM incidents
		 WHERE org_id=$1 AND status IN ('resolved','closed') AND resolved_at IS NOT NULL AND resolved_at < $2
		 ORDER BY resolved_at`, orgID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExpiredRecord
	for rows.Next() {
		var s incidentSnapshot
		if err := rows.Scan(&s.ID, &s.OrgID, &s.SoftwareID, &s.Title, &s.Description, &s.Severity, &s.Status,
			&s.SourceAlertID, &s.RootCause, &s.Mitigation, &s.CreatedAt, &s.UpdatedAt, &s.ResolvedAt); err != nil {
			return nil, err
		}
		data, err := json.Marshal(s)
		if err != nil {
			return nil, err
		}
		out = append(out, ExpiredRecord{ID: s.ID, Data: data})
	}
	return out, nil
}

// evidenceSnapshot mirrors incident_evidence for archival purposes.
type evidenceSnapshot struct {
	ID            uuid.UUID       `json:"id"`
	IncidentID    uuid.UUID       `json:"incident_id"`
	Type          string          `json:"type"`
	Title         string          `json:"title"`
	Content       json.RawMessage `json:"content"`
	Source        string          `json:"source"`
	CollectedAt   time.Time       `json:"collected_at"`
	BlobPath      string          `json:"blob_path,omitempty"`
	BlobSizeBytes int64           `json:"blob_size_bytes,omitempty"`
	MimeType      string          `json:"mime_type,omitempty"`
}

// FindExpiredEvidence returns evidence rows belonging to orgID (scoped via
// their parent incident, since incident_evidence has no org_id column of
// its own) whose collected_at is older than olderThanDays.
//
// Unlike FindExpiredIncidents, this is intentionally NOT gated on the
// parent incident's status: evidence (including large blobs) is the
// primary storage-cost driver this feature targets, and an org may want to
// prune old evidence attached to long-running incidents independently of
// whether/when the incident itself closes. The "incidents" and "evidence"
// resource types are separate policies for exactly this reason.
func (r *PgRetentionRepository) FindExpiredEvidence(ctx context.Context, orgID uuid.UUID, olderThanDays int) ([]ExpiredRecord, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	rows, err := r.pool.Query(ctx,
		`SELECT e.id, e.incident_id, e.type, e.title, e.content, COALESCE(e.source,''), e.collected_at,
		        COALESCE(e.blob_path,''), COALESCE(e.blob_size_bytes,0), COALESCE(e.mime_type,'')
		 FROM incident_evidence e
		 JOIN incidents i ON i.id = e.incident_id
		 WHERE i.org_id=$1 AND e.collected_at < $2
		 ORDER BY e.collected_at`, orgID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExpiredRecord
	for rows.Next() {
		var s evidenceSnapshot
		if err := rows.Scan(&s.ID, &s.IncidentID, &s.Type, &s.Title, &s.Content, &s.Source, &s.CollectedAt,
			&s.BlobPath, &s.BlobSizeBytes, &s.MimeType); err != nil {
			return nil, err
		}
		data, err := json.Marshal(s)
		if err != nil {
			return nil, err
		}
		out = append(out, ExpiredRecord{ID: s.ID, Data: data})
	}
	return out, nil
}

// agentRunSnapshot mirrors agent_runs for archival purposes.
type agentRunSnapshot struct {
	ID          uuid.UUID       `json:"id"`
	IncidentID  uuid.UUID       `json:"incident_id"`
	AgentName   string          `json:"agent_name"`
	AgentType   string          `json:"agent_type"`
	Status      string          `json:"status"`
	InputData   json.RawMessage `json:"input_data"`
	OutputData  json.RawMessage `json:"output_data"`
	ModelUsed   string          `json:"model_used"`
	TokensUsed  int             `json:"tokens_used"`
	DurationMs  int             `json:"duration_ms"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// FindExpiredAgentRuns returns agent_runs rows belonging to orgID (scoped
// via their parent incident) whose created_at is older than olderThanDays.
// Same independence-from-incident-status rationale as FindExpiredEvidence.
func (r *PgRetentionRepository) FindExpiredAgentRuns(ctx context.Context, orgID uuid.UUID, olderThanDays int) ([]ExpiredRecord, error) {
	cutoff := time.Now().AddDate(0, 0, -olderThanDays)
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.incident_id, a.agent_name, a.agent_type, a.status,
		        COALESCE(a.input_data,'{}'::jsonb), COALESCE(a.output_data,'{}'::jsonb),
		        COALESCE(a.model_used,''), COALESCE(a.tokens_used,0), COALESCE(a.duration_ms,0),
		        a.created_at, a.completed_at
		 FROM agent_runs a
		 JOIN incidents i ON i.id = a.incident_id
		 WHERE i.org_id=$1 AND a.created_at < $2
		 ORDER BY a.created_at`, orgID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ExpiredRecord
	for rows.Next() {
		var s agentRunSnapshot
		if err := rows.Scan(&s.ID, &s.IncidentID, &s.AgentName, &s.AgentType, &s.Status,
			&s.InputData, &s.OutputData, &s.ModelUsed, &s.TokensUsed, &s.DurationMs, &s.CreatedAt, &s.CompletedAt); err != nil {
			return nil, err
		}
		data, err := json.Marshal(s)
		if err != nil {
			return nil, err
		}
		out = append(out, ExpiredRecord{ID: s.ID, Data: data})
	}
	return out, nil
}

// --- Archive / delete actions ---

// ArchiveRecord writes a snapshot of a row to archived_records. Called
// before deleting the source row when a policy's action is "archive".
func (r *PgRetentionRepository) ArchiveRecord(ctx context.Context, orgID uuid.UUID, resourceType string, resourceID uuid.UUID, data json.RawMessage) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO archived_records (id, org_id, resource_type, resource_id, archived_data, archived_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		uuid.New(), orgID, resourceType, resourceID, data, time.Now())
	return err
}

// DeleteIncidentCascade deletes an incident. incident_events, incident_evidence,
// agent_runs, incident_rci, incident_rca and incident_postmortem all carry
// "ON DELETE CASCADE incidents(id)" FKs (see 001_initial.up.sql,
// 002_incident_cockpit.up.sql, 003_enriched_catalog_a2a.up.sql), so a single
// DELETE here is sufficient -- there is no separate evidence-delete step
// needed when the "incidents" policy fires.
func (r *PgRetentionRepository) DeleteIncidentCascade(ctx context.Context, incidentID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM incidents WHERE id=$1`, incidentID)
	return err
}

// DeleteEvidence deletes a single incident_evidence row (used by the
// "evidence" policy, which prunes evidence independently of its parent
// incident's lifecycle).
func (r *PgRetentionRepository) DeleteEvidence(ctx context.Context, evidenceID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM incident_evidence WHERE id=$1`, evidenceID)
	return err
}

// DeleteAgentRun deletes a single agent_runs row (used by the "agent_runs"
// policy).
func (r *PgRetentionRepository) DeleteAgentRun(ctx context.Context, agentRunID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM agent_runs WHERE id=$1`, agentRunID)
	return err
}
