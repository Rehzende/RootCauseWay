package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/Rehzende/RootCauseway/backend/internal/models"
)

// --- A2A Agent Repository ---

type PgA2AAgentRepository struct{ pool *pgxpool.Pool }

func NewA2AAgentRepository(pool *pgxpool.Pool) *PgA2AAgentRepository {
	return &PgA2AAgentRepository{pool: pool}
}

func (r *PgA2AAgentRepository) Create(ctx context.Context, a *models.A2AAgent) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO a2a_agents (id, org_id, name, description, agent_type, endpoint_url, agent_card, skills, allowed_software_ids, auth_type, auth_credentials, enabled, hosting_type, managed_config, llm_provider, llm_api_key_ref, auto_scale, min_replicas, max_replicas, health_status, last_health_check, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`,
		a.ID, a.OrgID, a.Name, a.Description, a.AgentType, a.EndpointURL, a.AgentCard, a.Skills, a.AllowedSoftwareIDs, a.AuthType, a.AuthCredentials, a.Enabled, a.HostingType, a.ManagedConfig, a.LLMProvider, a.LLMAPIKeyRef, a.AutoScale, a.MinReplicas, a.MaxReplicas, a.HealthStatus, a.LastHealthCheck, a.CreatedAt, a.UpdatedAt)
	return err
}

func (r *PgA2AAgentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.A2AAgent, error) {
	var a models.A2AAgent
	err := r.pool.QueryRow(ctx,
		`SELECT id, org_id, name, COALESCE(description,''), agent_type, COALESCE(endpoint_url,''), COALESCE(agent_card,'{}'::jsonb), COALESCE(skills,'[]'::jsonb), COALESCE(allowed_software_ids,'[]'::jsonb), COALESCE(auth_type,''), COALESCE(auth_credentials,''), enabled, COALESCE(is_system,false), COALESCE(hosting_type,'managed'), COALESCE(managed_config,'{}'::jsonb), COALESCE(llm_provider,'platform'), COALESCE(llm_api_key_ref,''), COALESCE(auto_scale,false), COALESCE(min_replicas,1), COALESCE(max_replicas,3), COALESCE(health_status,'unknown'), last_health_check, created_at, updated_at
		 FROM a2a_agents WHERE id=$1`, id).
		Scan(&a.ID, &a.OrgID, &a.Name, &a.Description, &a.AgentType, &a.EndpointURL, &a.AgentCard, &a.Skills, &a.AllowedSoftwareIDs, &a.AuthType, &a.AuthCredentials, &a.Enabled, &a.IsSystem, &a.HostingType, &a.ManagedConfig, &a.LLMProvider, &a.LLMAPIKeyRef, &a.AutoScale, &a.MinReplicas, &a.MaxReplicas, &a.HealthStatus, &a.LastHealthCheck, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *PgA2AAgentRepository) List(ctx context.Context, orgID uuid.UUID, agentType string, page, perPage int) ([]models.A2AAgent, int, error) {
	var total int
	query := `SELECT COUNT(*) FROM a2a_agents WHERE org_id=$1`
	args := []interface{}{orgID}
	if agentType != "" {
		query += ` AND agent_type=$2`
		args = append(args, agentType)
	}
	_ = r.pool.QueryRow(ctx, query, args...).Scan(&total)

	offset := (page - 1) * perPage
	selectQ := `SELECT id, org_id, name, COALESCE(description,''), agent_type, COALESCE(endpoint_url,''), COALESCE(agent_card,'{}'::jsonb), COALESCE(skills,'[]'::jsonb), COALESCE(allowed_software_ids,'[]'::jsonb), COALESCE(auth_type,''), COALESCE(auth_credentials,''), enabled, COALESCE(is_system,false), COALESCE(hosting_type,'managed'), COALESCE(managed_config,'{}'::jsonb), COALESCE(llm_provider,'platform'), COALESCE(llm_api_key_ref,''), COALESCE(auto_scale,false), COALESCE(min_replicas,1), COALESCE(max_replicas,3), COALESCE(health_status,'unknown'), last_health_check, created_at, updated_at FROM a2a_agents WHERE org_id=$1`
	selectArgs := []interface{}{orgID}
	if agentType != "" {
		selectQ += ` AND agent_type=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
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

	var items []models.A2AAgent
	for rows.Next() {
		var a models.A2AAgent
		if err := rows.Scan(&a.ID, &a.OrgID, &a.Name, &a.Description, &a.AgentType, &a.EndpointURL, &a.AgentCard, &a.Skills, &a.AllowedSoftwareIDs, &a.AuthType, &a.AuthCredentials, &a.Enabled, &a.IsSystem, &a.HostingType, &a.ManagedConfig, &a.LLMProvider, &a.LLMAPIKeyRef, &a.AutoScale, &a.MinReplicas, &a.MaxReplicas, &a.HealthStatus, &a.LastHealthCheck, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, a)
	}
	if items == nil {
		items = []models.A2AAgent{}
	}
	return items, total, nil
}

func (r *PgA2AAgentRepository) Update(ctx context.Context, a *models.A2AAgent) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE a2a_agents SET name=$1, description=$2, agent_type=$3, endpoint_url=$4, agent_card=$5, skills=$6, allowed_software_ids=$7, auth_type=$8, auth_credentials=$9, enabled=$10, hosting_type=$11, managed_config=$12, llm_provider=$13, llm_api_key_ref=$14, auto_scale=$15, min_replicas=$16, max_replicas=$17, health_status=$18, last_health_check=$19, updated_at=$20 WHERE id=$21`,
		a.Name, a.Description, a.AgentType, a.EndpointURL, a.AgentCard, a.Skills, a.AllowedSoftwareIDs, a.AuthType, a.AuthCredentials, a.Enabled, a.HostingType, a.ManagedConfig, a.LLMProvider, a.LLMAPIKeyRef, a.AutoScale, a.MinReplicas, a.MaxReplicas, a.HealthStatus, a.LastHealthCheck, a.UpdatedAt, a.ID)
	return err
}

func (r *PgA2AAgentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM a2a_agents WHERE id=$1`, id)
	return err
}

func (r *PgA2AAgentRepository) GetBySkill(ctx context.Context, orgID uuid.UUID, skill string) ([]models.A2AAgent, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, org_id, name, COALESCE(description,''), agent_type, COALESCE(endpoint_url,''), COALESCE(agent_card,'{}'::jsonb), COALESCE(skills,'[]'::jsonb), COALESCE(allowed_software_ids,'[]'::jsonb), COALESCE(auth_type,''), COALESCE(auth_credentials,''), enabled, COALESCE(is_system,false), COALESCE(hosting_type,'managed'), COALESCE(managed_config,'{}'::jsonb), COALESCE(llm_provider,'platform'), COALESCE(llm_api_key_ref,''), COALESCE(auto_scale,false), COALESCE(min_replicas,1), COALESCE(max_replicas,3), COALESCE(health_status,'unknown'), last_health_check, created_at, updated_at
		 FROM a2a_agents WHERE org_id=$1 AND enabled=true AND skills @> $2::jsonb`, orgID, fmt.Sprintf(`[{"id":"%s"}]`, skill))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.A2AAgent
	for rows.Next() {
		var a models.A2AAgent
		if err := rows.Scan(&a.ID, &a.OrgID, &a.Name, &a.Description, &a.AgentType, &a.EndpointURL, &a.AgentCard, &a.Skills, &a.AllowedSoftwareIDs, &a.AuthType, &a.AuthCredentials, &a.Enabled, &a.IsSystem, &a.HostingType, &a.ManagedConfig, &a.LLMProvider, &a.LLMAPIKeyRef, &a.AutoScale, &a.MinReplicas, &a.MaxReplicas, &a.HealthStatus, &a.LastHealthCheck, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	if items == nil {
		items = []models.A2AAgent{}
	}
	return items, nil
}

func (r *PgA2AAgentRepository) HealthCheck(ctx context.Context, id uuid.UUID, status string) error {
	now := time.Now()
	_, err := r.pool.Exec(ctx,
		`UPDATE a2a_agents SET health_status=$1, last_health_check=$2, updated_at=$3 WHERE id=$4`,
		status, now, now, id)
	return err
}

// --- A2A Task Repository ---

type PgA2ATaskRepository struct{ pool *pgxpool.Pool }

func NewA2ATaskRepository(pool *pgxpool.Pool) *PgA2ATaskRepository {
	return &PgA2ATaskRepository{pool: pool}
}

func (r *PgA2ATaskRepository) Create(ctx context.Context, t *models.A2ATask) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO a2a_tasks (id, incident_id, agent_id, agent_run_id, task_type, status, input_message, output_artifacts, error_message, orchestrator_reasoning, priority, depends_on, submitted_at, started_at, completed_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		t.ID, t.IncidentID, t.AgentID, t.AgentRunID, t.TaskType, t.Status, t.InputMessage, t.OutputArtifacts, t.ErrorMessage, t.OrchestratorReasoning, t.Priority, t.DependsOn, t.SubmittedAt, t.StartedAt, t.CompletedAt, t.CreatedAt)
	return err
}

func (r *PgA2ATaskRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.A2ATask, error) {
	var t models.A2ATask
	err := r.pool.QueryRow(ctx,
		`SELECT id, incident_id, agent_id, agent_run_id, task_type, status, COALESCE(input_message,'{}'::jsonb), COALESCE(output_artifacts,'[]'::jsonb), COALESCE(error_message,''), COALESCE(orchestrator_reasoning,''), COALESCE(priority,0), depends_on, submitted_at, started_at, completed_at, created_at
		 FROM a2a_tasks WHERE id=$1`, id).
		Scan(&t.ID, &t.IncidentID, &t.AgentID, &t.AgentRunID, &t.TaskType, &t.Status, &t.InputMessage, &t.OutputArtifacts, &t.ErrorMessage, &t.OrchestratorReasoning, &t.Priority, &t.DependsOn, &t.SubmittedAt, &t.StartedAt, &t.CompletedAt, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *PgA2ATaskRepository) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.A2ATask, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, incident_id, agent_id, agent_run_id, task_type, status, COALESCE(input_message,'{}'::jsonb), COALESCE(output_artifacts,'[]'::jsonb), COALESCE(error_message,''), COALESCE(orchestrator_reasoning,''), COALESCE(priority,0), depends_on, submitted_at, started_at, completed_at, created_at
		 FROM a2a_tasks WHERE incident_id=$1 ORDER BY created_at`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.A2ATask
	for rows.Next() {
		var t models.A2ATask
		if err := rows.Scan(&t.ID, &t.IncidentID, &t.AgentID, &t.AgentRunID, &t.TaskType, &t.Status, &t.InputMessage, &t.OutputArtifacts, &t.ErrorMessage, &t.OrchestratorReasoning, &t.Priority, &t.DependsOn, &t.SubmittedAt, &t.StartedAt, &t.CompletedAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	if items == nil {
		items = []models.A2ATask{}
	}
	return items, nil
}

func (r *PgA2ATaskRepository) Update(ctx context.Context, t *models.A2ATask) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE a2a_tasks SET status=$1, output_artifacts=$2, error_message=$3, started_at=$4, completed_at=$5 WHERE id=$6`,
		t.Status, t.OutputArtifacts, t.ErrorMessage, t.StartedAt, t.CompletedAt, t.ID)
	return err
}

func (r *PgA2ATaskRepository) ListByAgent(ctx context.Context, agentID uuid.UUID) ([]models.A2ATask, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, incident_id, agent_id, agent_run_id, task_type, status, COALESCE(input_message,'{}'::jsonb), COALESCE(output_artifacts,'[]'::jsonb), COALESCE(error_message,''), COALESCE(orchestrator_reasoning,''), COALESCE(priority,0), depends_on, submitted_at, started_at, completed_at, created_at
		 FROM a2a_tasks WHERE agent_id=$1 ORDER BY created_at`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.A2ATask
	for rows.Next() {
		var t models.A2ATask
		if err := rows.Scan(&t.ID, &t.IncidentID, &t.AgentID, &t.AgentRunID, &t.TaskType, &t.Status, &t.InputMessage, &t.OutputArtifacts, &t.ErrorMessage, &t.OrchestratorReasoning, &t.Priority, &t.DependsOn, &t.SubmittedAt, &t.StartedAt, &t.CompletedAt, &t.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	if items == nil {
		items = []models.A2ATask{}
	}
	return items, nil
}

// --- Orchestrator Decision Repository ---

type PgOrchestratorDecisionRepository struct{ pool *pgxpool.Pool }

func NewOrchestratorDecisionRepository(pool *pgxpool.Pool) *PgOrchestratorDecisionRepository {
	return &PgOrchestratorDecisionRepository{pool: pool}
}

func (r *PgOrchestratorDecisionRepository) Create(ctx context.Context, d *models.OrchestratorDecision) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO orchestrator_decisions (id, incident_id, decision_type, reasoning, selected_agents, context_used, confidence, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		d.ID, d.IncidentID, d.DecisionType, d.Reasoning, d.SelectedAgents, d.ContextUsed, d.Confidence, d.CreatedAt)
	return err
}

func (r *PgOrchestratorDecisionRepository) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]models.OrchestratorDecision, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, incident_id, decision_type, COALESCE(reasoning,''), COALESCE(selected_agents,'[]'::jsonb), COALESCE(context_used,'{}'::jsonb), COALESCE(confidence,0), created_at
		 FROM orchestrator_decisions WHERE incident_id=$1 ORDER BY created_at`, incidentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.OrchestratorDecision
	for rows.Next() {
		var d models.OrchestratorDecision
		if err := rows.Scan(&d.ID, &d.IncidentID, &d.DecisionType, &d.Reasoning, &d.SelectedAgents, &d.ContextUsed, &d.Confidence, &d.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	if items == nil {
		items = []models.OrchestratorDecision{}
	}
	return items, nil
}
