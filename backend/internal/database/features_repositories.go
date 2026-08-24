package database

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/embeddings"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- Feedback Repository ---

type PgFeedbackRepository struct{ pool *pgxpool.Pool }

func NewFeedbackRepository(pool *pgxpool.Pool) *PgFeedbackRepository {
	return &PgFeedbackRepository{pool: pool}
}

func (r *PgFeedbackRepository) Create(ctx context.Context, f *models.IncidentFeedback) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO incident_feedback (id, incident_id, user_id, target_type, rating, correction, original_data, corrected_data, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		f.ID, f.IncidentID, f.UserID, f.TargetType, f.Rating, f.Correction, f.OriginalData, f.CorrectedData, f.CreatedAt)
	return err
}

func (r *PgFeedbackRepository) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.IncidentFeedback, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, incident_id, user_id, target_type, rating, COALESCE(correction,''), COALESCE(original_data,'{}'::jsonb), COALESCE(corrected_data,'{}'::jsonb), created_at
		 FROM incident_feedback WHERE incident_id=$1 ORDER BY created_at DESC`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.IncidentFeedback
	for rows.Next() {
		var f models.IncidentFeedback
		if err := rows.Scan(&f.ID, &f.IncidentID, &f.UserID, &f.TargetType, &f.Rating, &f.Correction, &f.OriginalData, &f.CorrectedData, &f.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, f)
	}
	if items == nil {
		items = []models.IncidentFeedback{}
	}
	return items, nil
}

// --- Knowledge Base Repository ---

type PgKnowledgeBaseRepository struct{ pool *pgxpool.Pool }

func NewKnowledgeBaseRepository(pool *pgxpool.Pool) *PgKnowledgeBaseRepository {
	return &PgKnowledgeBaseRepository{pool: pool}
}

func (r *PgKnowledgeBaseRepository) Create(ctx context.Context, kb *models.KnowledgeBaseEntry) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO knowledge_base (id, org_id, incident_id, software_id, category, error_pattern, root_cause_summary, resolution_summary, lessons_learned, action_items, tags, human_validated, confidence, times_referenced, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		kb.ID, kb.OrgID, kb.IncidentID, kb.SoftwareID, kb.Category, kb.ErrorPattern, kb.RootCauseSummary, kb.ResolutionSummary, kb.LessonsLearned, kb.ActionItems, kb.Tags, kb.HumanValidated, kb.Confidence, kb.TimesReferenced, kb.CreatedAt, kb.UpdatedAt)
	return err
}

func (r *PgKnowledgeBaseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.KnowledgeBaseEntry, error) {
	var kb models.KnowledgeBaseEntry
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, incident_id, software_id, COALESCE(category,''), COALESCE(error_pattern,''), root_cause_summary, COALESCE(resolution_summary,''), COALESCE(lessons_learned,'[]'::jsonb), COALESCE(action_items,'[]'::jsonb), COALESCE(tags,'[]'::jsonb), COALESCE(human_validated,false), COALESCE(confidence,0), COALESCE(times_referenced,0), created_at, updated_at
		 FROM knowledge_base WHERE id=$1`, id).
		Scan(&kb.ID, &kb.OrgID, &kb.IncidentID, &kb.SoftwareID, &kb.Category, &kb.ErrorPattern, &kb.RootCauseSummary, &kb.ResolutionSummary, &kb.LessonsLearned, &kb.ActionItems, &kb.Tags, &kb.HumanValidated, &kb.Confidence, &kb.TimesReferenced, &kb.CreatedAt, &kb.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &kb, nil
}

func (r *PgKnowledgeBaseRepository) List(ctx context.Context, orgID uuid.UUID, category string) ([]models.KnowledgeBaseEntry, error) {
	query := `SELECT id, org_id, incident_id, software_id, COALESCE(category,''), COALESCE(error_pattern,''), root_cause_summary, COALESCE(resolution_summary,''), COALESCE(lessons_learned,'[]'::jsonb), COALESCE(action_items,'[]'::jsonb), COALESCE(tags,'[]'::jsonb), COALESCE(human_validated,false), COALESCE(confidence,0), COALESCE(times_referenced,0), created_at, updated_at FROM knowledge_base WHERE org_id=$1`
	args := []interface{}{orgID}
	if category != "" {
		query += ` AND category=$2`
		args = append(args, category)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.KnowledgeBaseEntry
	for rows.Next() {
		var kb models.KnowledgeBaseEntry
		if err := rows.Scan(&kb.ID, &kb.OrgID, &kb.IncidentID, &kb.SoftwareID, &kb.Category, &kb.ErrorPattern, &kb.RootCauseSummary, &kb.ResolutionSummary, &kb.LessonsLearned, &kb.ActionItems, &kb.Tags, &kb.HumanValidated, &kb.Confidence, &kb.TimesReferenced, &kb.CreatedAt, &kb.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, kb)
	}
	if items == nil {
		items = []models.KnowledgeBaseEntry{}
	}
	return items, nil
}

func (r *PgKnowledgeBaseRepository) Search(ctx context.Context, orgID uuid.UUID, softwareID *uuid.UUID, errorPattern string) ([]models.KnowledgeBaseEntry, error) {
	query := `SELECT id, org_id, incident_id, software_id, COALESCE(category,''), COALESCE(error_pattern,''), root_cause_summary, COALESCE(resolution_summary,''), COALESCE(lessons_learned,'[]'::jsonb), COALESCE(action_items,'[]'::jsonb), COALESCE(tags,'[]'::jsonb), COALESCE(human_validated,false), COALESCE(confidence,0), COALESCE(times_referenced,0), created_at, updated_at FROM knowledge_base WHERE org_id=$1`
	args := []interface{}{orgID}
	idx := 2
	if softwareID != nil {
		query += ` AND software_id=$` + itoa(idx)
		args = append(args, *softwareID)
		idx++
	}
	if errorPattern != "" {
		query += ` AND error_pattern ILIKE $` + itoa(idx)
		args = append(args, "%"+errorPattern+"%")
		idx++
	}
	query += ` ORDER BY confidence DESC, times_referenced DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.KnowledgeBaseEntry
	for rows.Next() {
		var kb models.KnowledgeBaseEntry
		if err := rows.Scan(&kb.ID, &kb.OrgID, &kb.IncidentID, &kb.SoftwareID, &kb.Category, &kb.ErrorPattern, &kb.RootCauseSummary, &kb.ResolutionSummary, &kb.LessonsLearned, &kb.ActionItems, &kb.Tags, &kb.HumanValidated, &kb.Confidence, &kb.TimesReferenced, &kb.CreatedAt, &kb.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, kb)
	}
	if items == nil {
		items = []models.KnowledgeBaseEntry{}
	}
	return items, nil
}

func (r *PgKnowledgeBaseRepository) Update(ctx context.Context, kb *models.KnowledgeBaseEntry) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE knowledge_base SET category=$1, error_pattern=$2, root_cause_summary=$3, resolution_summary=$4, lessons_learned=$5, action_items=$6, tags=$7, human_validated=$8, confidence=$9, updated_at=$10 WHERE id=$11`,
		kb.Category, kb.ErrorPattern, kb.RootCauseSummary, kb.ResolutionSummary, kb.LessonsLearned, kb.ActionItems, kb.Tags, kb.HumanValidated, kb.Confidence, kb.UpdatedAt, kb.ID)
	return err
}

func (r *PgKnowledgeBaseRepository) IncrementReferences(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `UPDATE knowledge_base SET times_referenced = times_referenced + 1 WHERE id=$1`, id)
	return err
}

// UpdateEmbedding stores the embedding vector for a knowledge base entry.
func (r *PgKnowledgeBaseRepository) UpdateEmbedding(ctx context.Context, id uuid.UUID, embedding []float32) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE knowledge_base SET embedding=$1::vector WHERE id=$2`,
		embeddings.VectorString(embedding), id)
	return err
}

// SearchByEmbedding returns entries ordered by cosine distance to the query
// embedding, filtered by a minimum cosine similarity threshold.
func (r *PgKnowledgeBaseRepository) SearchByEmbedding(ctx context.Context, orgID uuid.UUID, softwareID *uuid.UUID, embedding []float32, minSimilarity float64, limit int) ([]models.KnowledgeBaseEntry, error) {
	if limit <= 0 {
		limit = 10
	}
	vec := embeddings.VectorString(embedding)
	query := `SELECT id, org_id, incident_id, software_id, COALESCE(category,''), COALESCE(error_pattern,''), root_cause_summary, COALESCE(resolution_summary,''), COALESCE(lessons_learned,'[]'::jsonb), COALESCE(action_items,'[]'::jsonb), COALESCE(tags,'[]'::jsonb), COALESCE(human_validated,false), COALESCE(confidence,0), COALESCE(times_referenced,0), created_at, updated_at,
	          (1 - (embedding <=> $2::vector))::float8 AS similarity
	   FROM knowledge_base
	   WHERE org_id=$1 AND embedding IS NOT NULL AND (1 - (embedding <=> $2::vector)) >= $3`
	args := []interface{}{orgID, vec, minSimilarity}
	idx := 4
	if softwareID != nil {
		query += ` AND software_id=$` + itoa(idx)
		args = append(args, *softwareID)
		idx++
	}
	query += ` ORDER BY embedding <=> $2::vector LIMIT $` + itoa(idx)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.KnowledgeBaseEntry
	for rows.Next() {
		var kb models.KnowledgeBaseEntry
		var similarity float64
		if err := rows.Scan(&kb.ID, &kb.OrgID, &kb.IncidentID, &kb.SoftwareID, &kb.Category, &kb.ErrorPattern, &kb.RootCauseSummary, &kb.ResolutionSummary, &kb.LessonsLearned, &kb.ActionItems, &kb.Tags, &kb.HumanValidated, &kb.Confidence, &kb.TimesReferenced, &kb.CreatedAt, &kb.UpdatedAt, &similarity); err != nil {
			return nil, err
		}
		kb.Similarity = &similarity
		items = append(items, kb)
	}
	if items == nil {
		items = []models.KnowledgeBaseEntry{}
	}
	return items, nil
}

// --- Similar Incident Repository ---

type PgSimilarIncidentRepository struct{ pool *pgxpool.Pool }

func NewSimilarIncidentRepository(pool *pgxpool.Pool) *PgSimilarIncidentRepository {
	return &PgSimilarIncidentRepository{pool: pool}
}

func (r *PgSimilarIncidentRepository) Create(ctx context.Context, s *models.SimilarIncident) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO similar_incidents (id, incident_id, similar_incident_id, similarity_score, matched_on, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		s.ID, s.IncidentID, s.SimilarIncidentID, s.SimilarityScore, s.MatchedOn, s.CreatedAt)
	return err
}

func (r *PgSimilarIncidentRepository) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.SimilarIncident, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, incident_id, similar_incident_id, similarity_score, COALESCE(matched_on,'{}'::jsonb), created_at
		 FROM similar_incidents WHERE incident_id=$1 ORDER BY similarity_score DESC`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.SimilarIncident
	for rows.Next() {
		var s models.SimilarIncident
		if err := rows.Scan(&s.ID, &s.IncidentID, &s.SimilarIncidentID, &s.SimilarityScore, &s.MatchedOn, &s.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	if items == nil {
		items = []models.SimilarIncident{}
	}
	return items, nil
}

// --- Incident Vector Repository (pgvector similar-incident matching) ---

type PgIncidentVectorRepository struct{ pool *pgxpool.Pool }

func NewIncidentVectorRepository(pool *pgxpool.Pool) *PgIncidentVectorRepository {
	return &PgIncidentVectorRepository{pool: pool}
}

// UpdateEmbedding stores the embedding vector for an incident.
func (r *PgIncidentVectorRepository) UpdateEmbedding(ctx context.Context, incidentID uuid.UUID, embedding []float32) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE incidents SET embedding=$1::vector WHERE id=$2`,
		embeddings.VectorString(embedding), incidentID)
	return err
}

// FindSimilar returns the nearest incidents (by cosine distance) to the given
// incident's embedding, within the same organization, excluding the incident itself.
func (r *PgIncidentVectorRepository) FindSimilar(ctx context.Context, incidentID uuid.UUID, limit int) ([]models.SimilarIncidentMatch, error) {
	if limit <= 0 {
		limit = 5
	}
	rows, err := r.pool.Query(ctx,
		`SELECT i.id, i.title, i.severity, i.status,
		        (1 - (i.embedding <=> src.embedding))::float8 AS similarity,
		        i.created_at
		 FROM incidents i,
		      (SELECT org_id, embedding FROM incidents WHERE id=$1) src
		 WHERE i.org_id = src.org_id
		   AND i.id <> $1
		   AND i.embedding IS NOT NULL
		   AND src.embedding IS NOT NULL
		 ORDER BY i.embedding <=> src.embedding
		 LIMIT $2`, incidentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.SimilarIncidentMatch
	for rows.Next() {
		var m models.SimilarIncidentMatch
		if err := rows.Scan(&m.IncidentID, &m.Title, &m.Severity, &m.Status, &m.Similarity, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	if items == nil {
		items = []models.SimilarIncidentMatch{}
	}
	return items, nil
}

// --- Correlation Rule Repository ---

type PgCorrelationRuleRepository struct{ pool *pgxpool.Pool }

func NewCorrelationRuleRepository(pool *pgxpool.Pool) *PgCorrelationRuleRepository {
	return &PgCorrelationRuleRepository{pool: pool}
}

func (r *PgCorrelationRuleRepository) Create(ctx context.Context, cr *models.CorrelationRule) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO correlation_rules (id, org_id, name, description, rule_type, config, time_window_seconds, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		cr.ID, cr.OrgID, cr.Name, cr.Description, cr.RuleType, cr.Config, cr.TimeWindowSeconds, cr.Enabled, cr.CreatedAt, cr.UpdatedAt)
	return err
}

func (r *PgCorrelationRuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.CorrelationRule, error) {
	var cr models.CorrelationRule
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, COALESCE(description,''), rule_type, COALESCE(config,'{}'::jsonb), COALESCE(time_window_seconds,300), enabled, created_at, updated_at
		 FROM correlation_rules WHERE id=$1`, id).
		Scan(&cr.ID, &cr.OrgID, &cr.Name, &cr.Description, &cr.RuleType, &cr.Config, &cr.TimeWindowSeconds, &cr.Enabled, &cr.CreatedAt, &cr.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &cr, nil
}

func (r *PgCorrelationRuleRepository) List(ctx context.Context, orgID uuid.UUID) ([]models.CorrelationRule, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, COALESCE(description,''), rule_type, COALESCE(config,'{}'::jsonb), COALESCE(time_window_seconds,300), enabled, created_at, updated_at
		 FROM correlation_rules WHERE org_id=$1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.CorrelationRule
	for rows.Next() {
		var cr models.CorrelationRule
		if err := rows.Scan(&cr.ID, &cr.OrgID, &cr.Name, &cr.Description, &cr.RuleType, &cr.Config, &cr.TimeWindowSeconds, &cr.Enabled, &cr.CreatedAt, &cr.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, cr)
	}
	if items == nil {
		items = []models.CorrelationRule{}
	}
	return items, nil
}

func (r *PgCorrelationRuleRepository) Update(ctx context.Context, cr *models.CorrelationRule) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE correlation_rules SET name=$1, description=$2, rule_type=$3, config=$4, time_window_seconds=$5, enabled=$6, updated_at=$7 WHERE id=$8`,
		cr.Name, cr.Description, cr.RuleType, cr.Config, cr.TimeWindowSeconds, cr.Enabled, cr.UpdatedAt, cr.ID)
	return err
}

func (r *PgCorrelationRuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM correlation_rules WHERE id=$1`, id)
	return err
}

// --- Alert Group Repository ---

type PgAlertGroupRepository struct{ pool *pgxpool.Pool }

func NewAlertGroupRepository(pool *pgxpool.Pool) *PgAlertGroupRepository {
	return &PgAlertGroupRepository{pool: pool}
}

func (r *PgAlertGroupRepository) Create(ctx context.Context, ag *models.AlertGroup) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO alert_groups (id, incident_id, alert_snapshot_id, correlation_rule_id, created_at)
		 VALUES ($1,$2,$3,$4,$5)`,
		ag.ID, ag.IncidentID, ag.AlertSnapshotID, ag.CorrelationRuleID, ag.CreatedAt)
	return err
}

func (r *PgAlertGroupRepository) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.AlertGroup, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, incident_id, alert_snapshot_id, correlation_rule_id, created_at
		 FROM alert_groups WHERE incident_id=$1 ORDER BY created_at`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.AlertGroup
	for rows.Next() {
		var ag models.AlertGroup
		if err := rows.Scan(&ag.ID, &ag.IncidentID, &ag.AlertSnapshotID, &ag.CorrelationRuleID, &ag.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, ag)
	}
	if items == nil {
		items = []models.AlertGroup{}
	}
	return items, nil
}

// --- Notification Channel Repository ---

type PgNotificationChannelRepository struct{ pool *pgxpool.Pool }

func NewNotificationChannelRepository(pool *pgxpool.Pool) *PgNotificationChannelRepository {
	return &PgNotificationChannelRepository{pool: pool}
}

func (r *PgNotificationChannelRepository) Create(ctx context.Context, nc *models.NotificationChannel) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notification_channels (id, org_id, name, channel_type, config, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		nc.ID, nc.OrgID, nc.Name, nc.ChannelType, nc.Config, nc.Enabled, nc.CreatedAt, nc.UpdatedAt)
	return err
}

func (r *PgNotificationChannelRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.NotificationChannel, error) {
	var nc models.NotificationChannel
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, channel_type, COALESCE(config,'{}'::jsonb), enabled, created_at, updated_at
		 FROM notification_channels WHERE id=$1`, id).
		Scan(&nc.ID, &nc.OrgID, &nc.Name, &nc.ChannelType, &nc.Config, &nc.Enabled, &nc.CreatedAt, &nc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &nc, nil
}

func (r *PgNotificationChannelRepository) List(ctx context.Context, orgID uuid.UUID) ([]models.NotificationChannel, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, channel_type, COALESCE(config,'{}'::jsonb), enabled, created_at, updated_at
		 FROM notification_channels WHERE org_id=$1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.NotificationChannel
	for rows.Next() {
		var nc models.NotificationChannel
		if err := rows.Scan(&nc.ID, &nc.OrgID, &nc.Name, &nc.ChannelType, &nc.Config, &nc.Enabled, &nc.CreatedAt, &nc.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, nc)
	}
	if items == nil {
		items = []models.NotificationChannel{}
	}
	return items, nil
}

func (r *PgNotificationChannelRepository) Update(ctx context.Context, nc *models.NotificationChannel) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notification_channels SET name=$1, channel_type=$2, config=$3, enabled=$4, updated_at=$5 WHERE id=$6`,
		nc.Name, nc.ChannelType, nc.Config, nc.Enabled, nc.UpdatedAt, nc.ID)
	return err
}

func (r *PgNotificationChannelRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM notification_channels WHERE id=$1`, id)
	return err
}

// --- Escalation Policy Repository ---

type PgEscalationPolicyRepository struct{ pool *pgxpool.Pool }

func NewEscalationPolicyRepository(pool *pgxpool.Pool) *PgEscalationPolicyRepository {
	return &PgEscalationPolicyRepository{pool: pool}
}

func (r *PgEscalationPolicyRepository) Create(ctx context.Context, ep *models.EscalationPolicy) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO escalation_policies (id, org_id, name, description, software_id, severity_filter, steps, repeat_after_seconds, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		ep.ID, ep.OrgID, ep.Name, ep.Description, ep.SoftwareID, ep.SeverityFilter, ep.Steps, ep.RepeatAfterSeconds, ep.Enabled, ep.CreatedAt, ep.UpdatedAt)
	return err
}

func (r *PgEscalationPolicyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.EscalationPolicy, error) {
	var ep models.EscalationPolicy
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, COALESCE(description,''), software_id, COALESCE(severity_filter,'[]'::jsonb), COALESCE(steps,'[]'::jsonb), repeat_after_seconds, enabled, created_at, updated_at
		 FROM escalation_policies WHERE id=$1`, id).
		Scan(&ep.ID, &ep.OrgID, &ep.Name, &ep.Description, &ep.SoftwareID, &ep.SeverityFilter, &ep.Steps, &ep.RepeatAfterSeconds, &ep.Enabled, &ep.CreatedAt, &ep.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &ep, nil
}

func (r *PgEscalationPolicyRepository) List(ctx context.Context, orgID uuid.UUID) ([]models.EscalationPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, COALESCE(description,''), software_id, COALESCE(severity_filter,'[]'::jsonb), COALESCE(steps,'[]'::jsonb), repeat_after_seconds, enabled, created_at, updated_at
		 FROM escalation_policies WHERE org_id=$1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.EscalationPolicy
	for rows.Next() {
		var ep models.EscalationPolicy
		if err := rows.Scan(&ep.ID, &ep.OrgID, &ep.Name, &ep.Description, &ep.SoftwareID, &ep.SeverityFilter, &ep.Steps, &ep.RepeatAfterSeconds, &ep.Enabled, &ep.CreatedAt, &ep.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, ep)
	}
	if items == nil {
		items = []models.EscalationPolicy{}
	}
	return items, nil
}

func (r *PgEscalationPolicyRepository) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.EscalationPolicy, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, COALESCE(description,''), software_id, COALESCE(severity_filter,'[]'::jsonb), COALESCE(steps,'[]'::jsonb), repeat_after_seconds, enabled, created_at, updated_at
		 FROM escalation_policies WHERE software_id=$1 ORDER BY created_at DESC`, softwareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.EscalationPolicy
	for rows.Next() {
		var ep models.EscalationPolicy
		if err := rows.Scan(&ep.ID, &ep.OrgID, &ep.Name, &ep.Description, &ep.SoftwareID, &ep.SeverityFilter, &ep.Steps, &ep.RepeatAfterSeconds, &ep.Enabled, &ep.CreatedAt, &ep.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, ep)
	}
	if items == nil {
		items = []models.EscalationPolicy{}
	}
	return items, nil
}

func (r *PgEscalationPolicyRepository) Update(ctx context.Context, ep *models.EscalationPolicy) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE escalation_policies SET name=$1, description=$2, software_id=$3, severity_filter=$4, steps=$5, repeat_after_seconds=$6, enabled=$7, updated_at=$8 WHERE id=$9`,
		ep.Name, ep.Description, ep.SoftwareID, ep.SeverityFilter, ep.Steps, ep.RepeatAfterSeconds, ep.Enabled, ep.UpdatedAt, ep.ID)
	return err
}

func (r *PgEscalationPolicyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM escalation_policies WHERE id=$1`, id)
	return err
}

// --- Notification Log Repository ---

type PgNotificationLogRepository struct{ pool *pgxpool.Pool }

func NewNotificationLogRepository(pool *pgxpool.Pool) *PgNotificationLogRepository {
	return &PgNotificationLogRepository{pool: pool}
}

func (r *PgNotificationLogRepository) Create(ctx context.Context, nl *models.NotificationLogEntry) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notification_log (id, org_id, incident_id, channel_id, policy_id, event_type, recipient, payload, status, error_message, sent_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		nl.ID, nl.OrgID, nl.IncidentID, nl.ChannelID, nl.PolicyID, nl.EventType, nl.Recipient, nl.Payload, nl.Status, nl.ErrorMessage, nl.SentAt, nl.CreatedAt)
	return err
}

func (r *PgNotificationLogRepository) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.NotificationLogEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, incident_id, channel_id, policy_id, event_type, COALESCE(recipient,''), COALESCE(payload,'{}'::jsonb), status, COALESCE(error_message,''), sent_at, created_at
		 FROM notification_log WHERE incident_id=$1 ORDER BY created_at DESC`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.NotificationLogEntry
	for rows.Next() {
		var nl models.NotificationLogEntry
		if err := rows.Scan(&nl.ID, &nl.OrgID, &nl.IncidentID, &nl.ChannelID, &nl.PolicyID, &nl.EventType, &nl.Recipient, &nl.Payload, &nl.Status, &nl.ErrorMessage, &nl.SentAt, &nl.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, nl)
	}
	if items == nil {
		items = []models.NotificationLogEntry{}
	}
	return items, nil
}

// ListByOrg returns notification log entries across all of an org's
// incidents, paginated -- the global "Logs" tab in the frontend's
// Notifications page. notification_log carries org_id directly (denormalized
// at write time in Create above), so this is a plain filter, no join needed.
func (r *PgNotificationLogRepository) ListByOrg(ctx context.Context, orgID uuid.UUID, page, perPage int) ([]models.NotificationLogEntry, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM notification_log WHERE org_id=$1`, orgID).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, incident_id, channel_id, policy_id, event_type, COALESCE(recipient,''), COALESCE(payload,'{}'::jsonb), status, COALESCE(error_message,''), sent_at, created_at
		 FROM notification_log WHERE org_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, orgID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var items []models.NotificationLogEntry
	for rows.Next() {
		var nl models.NotificationLogEntry
		if err := rows.Scan(&nl.ID, &nl.OrgID, &nl.IncidentID, &nl.ChannelID, &nl.PolicyID, &nl.EventType, &nl.Recipient, &nl.Payload, &nl.Status, &nl.ErrorMessage, &nl.SentAt, &nl.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, nl)
	}
	if items == nil {
		items = []models.NotificationLogEntry{}
	}
	return items, total, nil
}

func (r *PgNotificationLogRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, errMsg string) error {
	var sentAt *time.Time
	if status == "sent" || status == "delivered" {
		now := time.Now()
		sentAt = &now
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE notification_log SET status=$1, error_message=$2, sent_at=COALESCE($3, sent_at) WHERE id=$4`,
		status, errMsg, sentAt, id)
	return err
}

// --- Runbook Repository ---

type PgRunbookRepository struct{ pool *pgxpool.Pool }

func NewRunbookRepository(pool *pgxpool.Pool) *PgRunbookRepository {
	return &PgRunbookRepository{pool: pool}
}

func (r *PgRunbookRepository) Create(ctx context.Context, rb *models.Runbook) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO runbooks (id, org_id, software_id, name, slug, description, trigger_conditions, auto_trigger, enabled, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		rb.ID, rb.OrgID, rb.SoftwareID, rb.Name, rb.Slug, rb.Description, rb.TriggerConditions, rb.AutoTrigger, rb.Enabled, rb.CreatedAt, rb.UpdatedAt)
	return err
}

func (r *PgRunbookRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Runbook, error) {
	var rb models.Runbook
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, software_id, name, slug, COALESCE(description,''), COALESCE(trigger_conditions,'{}'::jsonb), COALESCE(auto_trigger,false), enabled, created_at, updated_at
		 FROM runbooks WHERE id=$1`, id).
		Scan(&rb.ID, &rb.OrgID, &rb.SoftwareID, &rb.Name, &rb.Slug, &rb.Description, &rb.TriggerConditions, &rb.AutoTrigger, &rb.Enabled, &rb.CreatedAt, &rb.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &rb, nil
}

func (r *PgRunbookRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*models.Runbook, error) {
	var rb models.Runbook
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, software_id, name, slug, COALESCE(description,''), COALESCE(trigger_conditions,'{}'::jsonb), COALESCE(auto_trigger,false), enabled, created_at, updated_at
		 FROM runbooks WHERE org_id=$1 AND slug=$2`, orgID, slug).
		Scan(&rb.ID, &rb.OrgID, &rb.SoftwareID, &rb.Name, &rb.Slug, &rb.Description, &rb.TriggerConditions, &rb.AutoTrigger, &rb.Enabled, &rb.CreatedAt, &rb.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &rb, nil
}

func (r *PgRunbookRepository) List(ctx context.Context, orgID uuid.UUID) ([]models.Runbook, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, software_id, name, slug, COALESCE(description,''), COALESCE(trigger_conditions,'{}'::jsonb), COALESCE(auto_trigger,false), enabled, created_at, updated_at
		 FROM runbooks WHERE org_id=$1 ORDER BY created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.Runbook
	for rows.Next() {
		var rb models.Runbook
		if err := rows.Scan(&rb.ID, &rb.OrgID, &rb.SoftwareID, &rb.Name, &rb.Slug, &rb.Description, &rb.TriggerConditions, &rb.AutoTrigger, &rb.Enabled, &rb.CreatedAt, &rb.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, rb)
	}
	if items == nil {
		items = []models.Runbook{}
	}
	return items, nil
}

func (r *PgRunbookRepository) ListBySoftware(ctx context.Context, softwareID uuid.UUID) ([]models.Runbook, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, software_id, name, slug, COALESCE(description,''), COALESCE(trigger_conditions,'{}'::jsonb), COALESCE(auto_trigger,false), enabled, created_at, updated_at
		 FROM runbooks WHERE software_id=$1 ORDER BY created_at DESC`, softwareID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.Runbook
	for rows.Next() {
		var rb models.Runbook
		if err := rows.Scan(&rb.ID, &rb.OrgID, &rb.SoftwareID, &rb.Name, &rb.Slug, &rb.Description, &rb.TriggerConditions, &rb.AutoTrigger, &rb.Enabled, &rb.CreatedAt, &rb.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, rb)
	}
	if items == nil {
		items = []models.Runbook{}
	}
	return items, nil
}

func (r *PgRunbookRepository) Update(ctx context.Context, rb *models.Runbook) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE runbooks SET software_id=$1, name=$2, slug=$3, description=$4, trigger_conditions=$5, auto_trigger=$6, enabled=$7, updated_at=$8 WHERE id=$9`,
		rb.SoftwareID, rb.Name, rb.Slug, rb.Description, rb.TriggerConditions, rb.AutoTrigger, rb.Enabled, rb.UpdatedAt, rb.ID)
	return err
}

func (r *PgRunbookRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM runbooks WHERE id=$1`, id)
	return err
}

// --- Runbook Step Repository ---

type PgRunbookStepRepository struct{ pool *pgxpool.Pool }

func NewRunbookStepRepository(pool *pgxpool.Pool) *PgRunbookStepRepository {
	return &PgRunbookStepRepository{pool: pool}
}

func (r *PgRunbookStepRepository) Create(ctx context.Context, s *models.RunbookStep) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO runbook_steps (id, runbook_id, step_order, name, description, step_type, config, skill_id, timeout_seconds, on_failure, max_retries, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		s.ID, s.RunbookID, s.StepOrder, s.Name, s.Description, s.StepType, s.Config, s.SkillID, s.TimeoutSeconds, s.OnFailure, s.MaxRetries, s.CreatedAt)
	return err
}

func (r *PgRunbookStepRepository) ListByRunbook(ctx context.Context, runbookID uuid.UUID) ([]models.RunbookStep, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, runbook_id, step_order, name, COALESCE(description,''), step_type, COALESCE(config,'{}'::jsonb), skill_id, COALESCE(timeout_seconds,300), COALESCE(on_failure,'stop'), COALESCE(max_retries,0), created_at
		 FROM runbook_steps WHERE runbook_id=$1 ORDER BY step_order`, runbookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.RunbookStep
	for rows.Next() {
		var s models.RunbookStep
		if err := rows.Scan(&s.ID, &s.RunbookID, &s.StepOrder, &s.Name, &s.Description, &s.StepType, &s.Config, &s.SkillID, &s.TimeoutSeconds, &s.OnFailure, &s.MaxRetries, &s.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, s)
	}
	if items == nil {
		items = []models.RunbookStep{}
	}
	return items, nil
}

func (r *PgRunbookStepRepository) Update(ctx context.Context, s *models.RunbookStep) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE runbook_steps SET step_order=$1, name=$2, description=$3, step_type=$4, config=$5, skill_id=$6, timeout_seconds=$7, on_failure=$8, max_retries=$9 WHERE id=$10`,
		s.StepOrder, s.Name, s.Description, s.StepType, s.Config, s.SkillID, s.TimeoutSeconds, s.OnFailure, s.MaxRetries, s.ID)
	return err
}

func (r *PgRunbookStepRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM runbook_steps WHERE id=$1`, id)
	return err
}

// UpdateOrder sets a single step's position without touching its other
// fields (unlike Update, which requires the full row) -- the primitive
// reordering needs.
func (r *PgRunbookStepRepository) UpdateOrder(ctx context.Context, id uuid.UUID, order int) error {
	_, err := r.pool.Exec(ctx, `UPDATE runbook_steps SET step_order=$1 WHERE id=$2`, order, id)
	return err
}

// --- Runbook Execution Repository ---

type PgRunbookExecutionRepository struct{ pool *pgxpool.Pool }

func NewRunbookExecutionRepository(pool *pgxpool.Pool) *PgRunbookExecutionRepository {
	return &PgRunbookExecutionRepository{pool: pool}
}

func (r *PgRunbookExecutionRepository) Create(ctx context.Context, re *models.RunbookExecution) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO runbook_executions (id, runbook_id, incident_id, triggered_by, status, current_step, step_results, started_at, completed_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		re.ID, re.RunbookID, re.IncidentID, re.TriggeredBy, re.Status, re.CurrentStep, re.StepResults, re.StartedAt, re.CompletedAt, re.CreatedAt)
	return err
}

func (r *PgRunbookExecutionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.RunbookExecution, error) {
	var re models.RunbookExecution
	err := r.pool.QueryRow(ctx,
		`SELECT id, runbook_id, incident_id, COALESCE(triggered_by,''), status, COALESCE(current_step,0), COALESCE(step_results,'[]'::jsonb), started_at, completed_at, created_at
		 FROM runbook_executions WHERE id=$1`, id).
		Scan(&re.ID, &re.RunbookID, &re.IncidentID, &re.TriggeredBy, &re.Status, &re.CurrentStep, &re.StepResults, &re.StartedAt, &re.CompletedAt, &re.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &re, nil
}

func (r *PgRunbookExecutionRepository) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.RunbookExecution, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, runbook_id, incident_id, COALESCE(triggered_by,''), status, COALESCE(current_step,0), COALESCE(step_results,'[]'::jsonb), started_at, completed_at, created_at
		 FROM runbook_executions WHERE incident_id=$1 ORDER BY created_at DESC`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.RunbookExecution
	for rows.Next() {
		var re models.RunbookExecution
		if err := rows.Scan(&re.ID, &re.RunbookID, &re.IncidentID, &re.TriggeredBy, &re.Status, &re.CurrentStep, &re.StepResults, &re.StartedAt, &re.CompletedAt, &re.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, re)
	}
	if items == nil {
		items = []models.RunbookExecution{}
	}
	return items, nil
}

// ListByRunbook returns all executions of a given runbook, across every
// incident -- the RunbookDetailPage's execution history tab, which is
// scoped by runbook_id, not incident_id (that's ListByIncident above).
func (r *PgRunbookExecutionRepository) ListByRunbook(ctx context.Context, runbookID uuid.UUID) ([]models.RunbookExecution, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, runbook_id, incident_id, COALESCE(triggered_by,''), status, COALESCE(current_step,0), COALESCE(step_results,'[]'::jsonb), started_at, completed_at, created_at
		 FROM runbook_executions WHERE runbook_id=$1 ORDER BY created_at DESC`, runbookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.RunbookExecution
	for rows.Next() {
		var re models.RunbookExecution
		if err := rows.Scan(&re.ID, &re.RunbookID, &re.IncidentID, &re.TriggeredBy, &re.Status, &re.CurrentStep, &re.StepResults, &re.StartedAt, &re.CompletedAt, &re.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, re)
	}
	if items == nil {
		items = []models.RunbookExecution{}
	}
	return items, nil
}

func (r *PgRunbookExecutionRepository) Update(ctx context.Context, re *models.RunbookExecution) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE runbook_executions SET status=$1, current_step=$2, step_results=$3, started_at=$4, completed_at=$5 WHERE id=$6`,
		re.Status, re.CurrentStep, re.StepResults, re.StartedAt, re.CompletedAt, re.ID)
	return err
}

// --- Change Event Repository ---

type PgChangeEventRepository struct{ pool *pgxpool.Pool }

func NewChangeEventRepository(pool *pgxpool.Pool) *PgChangeEventRepository {
	return &PgChangeEventRepository{pool: pool}
}

func (r *PgChangeEventRepository) Create(ctx context.Context, ce *models.ChangeEvent) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO change_events (id, org_id, software_id, change_type, title, description, source, source_url, commit_sha, author, environment, metadata, occurred_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		ce.ID, ce.OrgID, ce.SoftwareID, ce.ChangeType, ce.Title, ce.Description, ce.Source, ce.SourceURL, ce.CommitSHA, ce.Author, ce.Environment, ce.Metadata, ce.OccurredAt, ce.CreatedAt)
	return err
}

func (r *PgChangeEventRepository) ListBySoftware(ctx context.Context, softwareID uuid.UUID, since time.Time) ([]models.ChangeEvent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, software_id, change_type, title, COALESCE(description,''), COALESCE(source,''), COALESCE(source_url,''), COALESCE(commit_sha,''), COALESCE(author,''), COALESCE(environment,''), COALESCE(metadata,'{}'::jsonb), occurred_at, created_at
		 FROM change_events WHERE software_id=$1 AND occurred_at>=$2 ORDER BY occurred_at DESC`, softwareID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.ChangeEvent
	for rows.Next() {
		var ce models.ChangeEvent
		if err := rows.Scan(&ce.ID, &ce.OrgID, &ce.SoftwareID, &ce.ChangeType, &ce.Title, &ce.Description, &ce.Source, &ce.SourceURL, &ce.CommitSHA, &ce.Author, &ce.Environment, &ce.Metadata, &ce.OccurredAt, &ce.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, ce)
	}
	if items == nil {
		items = []models.ChangeEvent{}
	}
	return items, nil
}

func (r *PgChangeEventRepository) ListRecent(ctx context.Context, softwareID uuid.UUID, minutes int) ([]models.ChangeEvent, error) {
	since := time.Now().Add(-time.Duration(minutes) * time.Minute)
	return r.ListBySoftware(ctx, softwareID, since)
}

// --- Analytics Repository ---

type PgAnalyticsRepository struct{ pool *pgxpool.Pool }

func NewAnalyticsRepository(pool *pgxpool.Pool) *PgAnalyticsRepository {
	return &PgAnalyticsRepository{pool: pool}
}

func (r *PgAnalyticsRepository) GetMTTR(ctx context.Context, orgID uuid.UUID, days int) ([]models.AnalyticsMTTR, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT i.software_id, COALESCE(s.name,'unknown'),
		        AVG(EXTRACT(EPOCH FROM (i.resolved_at - i.created_at)))::float8 as avg_mttr,
		        COUNT(*)::int as cnt
		 FROM incidents i
		 LEFT JOIN software_catalog s ON s.id = i.software_id
		 WHERE i.org_id=$1 AND i.resolved_at IS NOT NULL AND i.created_at >= NOW() - ($2 || ' days')::interval
		 GROUP BY i.software_id, s.name
		 ORDER BY avg_mttr DESC`, orgID, itoa(days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	period := itoa(days) + "d"
	var items []models.AnalyticsMTTR
	for rows.Next() {
		var m models.AnalyticsMTTR
		if err := rows.Scan(&m.SoftwareID, &m.SoftwareName, &m.AvgMTTRSeconds, &m.IncidentCount); err != nil {
			return nil, err
		}
		m.Period = period
		items = append(items, m)
	}
	if items == nil {
		items = []models.AnalyticsMTTR{}
	}
	return items, nil
}

func (r *PgAnalyticsRepository) GetIncidentTrends(ctx context.Context, orgID uuid.UUID, days int) ([]models.AnalyticsIncidentTrend, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT TO_CHAR(created_at::date, 'YYYY-MM-DD') as dt, COUNT(*)::int, COALESCE(severity,'unknown')
		 FROM incidents
		 WHERE org_id=$1 AND created_at >= NOW() - ($2 || ' days')::interval
		 GROUP BY dt, severity
		 ORDER BY dt`, orgID, itoa(days))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.AnalyticsIncidentTrend
	for rows.Next() {
		var t models.AnalyticsIncidentTrend
		if err := rows.Scan(&t.Date, &t.Count, &t.Severity); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	if items == nil {
		items = []models.AnalyticsIncidentTrend{}
	}
	return items, nil
}

func (r *PgAnalyticsRepository) GetAgentEffectiveness(ctx context.Context, orgID uuid.UUID) ([]models.AnalyticsAgentEffectiveness, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT COALESCE(ar.agent_name,'unknown'),
		        COUNT(*)::int as total,
		        (COUNT(*) FILTER (WHERE ar.status='completed')::float8 / GREATEST(COUNT(*),1)::float8) as success_rate,
		        AVG(COALESCE(ar.duration_ms,0))::float8 as avg_dur
		 FROM agent_runs ar
		 JOIN incidents i ON i.id = ar.incident_id
		 WHERE i.org_id=$1
		 GROUP BY ar.agent_name
		 ORDER BY total DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.AnalyticsAgentEffectiveness
	for rows.Next() {
		var a models.AnalyticsAgentEffectiveness
		if err := rows.Scan(&a.AgentName, &a.TotalTasks, &a.SuccessRate, &a.AvgDurationMs); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	if items == nil {
		items = []models.AnalyticsAgentEffectiveness{}
	}
	return items, nil
}

func (r *PgAnalyticsRepository) GetCostByModel(ctx context.Context, orgID uuid.UUID) ([]models.AnalyticsCostByModel, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT COALESCE(ar.model_used,'unknown'), COUNT(*), COALESCE(SUM(ar.tokens_used),0)
		 FROM agent_runs ar
		 JOIN incidents i ON ar.incident_id = i.id
		 WHERE i.org_id=$1 AND ar.status='completed'
		 GROUP BY ar.model_used ORDER BY SUM(ar.tokens_used) DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.AnalyticsCostByModel
	for rows.Next() {
		var a models.AnalyticsCostByModel
		if err := rows.Scan(&a.Model, &a.TotalRuns, &a.TotalTokens); err != nil {
			return nil, err
		}
		a.EstCostUSD = estimateCostUSD(a.Model, a.TotalTokens)
		items = append(items, a)
	}
	if items == nil {
		items = []models.AnalyticsCostByModel{}
	}
	return items, nil
}

func (r *PgAnalyticsRepository) GetCostByIncident(ctx context.Context, orgID uuid.UUID, limit int) ([]models.AnalyticsCostByIncident, error) {
	if limit <= 0 {
		limit = 20
	}
	// Grouped by (incident, model_used) rather than just incident -- an
	// incident's runs can span more than one model (e.g. a cheap local
	// model for triage, a stronger one for RCA), and estimateCostUSD needs
	// the real per-group model to price each slice correctly instead of a
	// single flat rate applied to the incident's total tokens regardless of
	// which model actually produced them. Aggregated back down to one row
	// per incident in Go below; LIMIT is applied there too since GROUP BY
	// means more than one row can share an incident.
	rows, err := r.pool.Query(ctx,
		`SELECT i.id, COALESCE(i.title,''), i.created_at,
		        COALESCE(ar.model_used,'unknown'),
		        COUNT(ar.id)::int, COALESCE(SUM(ar.tokens_used),0)::int, COALESCE(SUM(ar.duration_ms),0)::int
		 FROM incidents i
		 LEFT JOIN agent_runs ar ON ar.incident_id = i.id AND ar.status='completed'
		 WHERE i.org_id=$1
		 GROUP BY i.id, i.title, i.created_at, ar.model_used
		 ORDER BY i.created_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type incidentAgg struct {
		title     string
		createdAt time.Time
		runs      int
		tokens    int
		durMs     int
		costUSD   float64
	}
	order := make([]uuid.UUID, 0, limit)
	byID := make(map[uuid.UUID]*incidentAgg, limit)
	for rows.Next() {
		var id uuid.UUID
		var title, model string
		var createdAt time.Time
		var runCount, tokens, durMs int
		if err := rows.Scan(&id, &title, &createdAt, &model, &runCount, &tokens, &durMs); err != nil {
			return nil, err
		}
		a, ok := byID[id]
		if !ok {
			a = &incidentAgg{title: title, createdAt: createdAt}
			byID[id] = a
			order = append(order, id)
		}
		a.runs += runCount
		a.tokens += tokens
		a.durMs += durMs
		a.costUSD += estimateCostUSD(model, tokens)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	items := make([]models.AnalyticsCostByIncident, 0, len(order))
	for _, id := range order {
		a := byID[id]
		items = append(items, models.AnalyticsCostByIncident{
			IncidentID: id, IncidentTitle: a.title, TotalRuns: a.runs,
			TotalTokens: a.tokens, EstCostUSD: a.costUSD,
			TotalDurationMs: a.durMs, CreatedAt: a.createdAt,
		})
	}
	if len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// helper to convert int to string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}
