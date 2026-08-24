-- Enrich Software Catalog with operational context
ALTER TABLE software_catalog ADD COLUMN IF NOT EXISTS pipeline_url VARCHAR(500);
ALTER TABLE software_catalog ADD COLUMN IF NOT EXISTS cloud_provider VARCHAR(20) CHECK (cloud_provider IN ('aws', 'azure', 'gcp', 'on_prem', 'hybrid'));
ALTER TABLE software_catalog ADD COLUMN IF NOT EXISTS cloud_resources JSONB DEFAULT '[]';
ALTER TABLE software_catalog ADD COLUMN IF NOT EXISTS database_info JSONB DEFAULT '[]';
ALTER TABLE software_catalog ADD COLUMN IF NOT EXISTS infra_details JSONB DEFAULT '{}';
ALTER TABLE software_catalog ADD COLUMN IF NOT EXISTS stakeholders JSONB DEFAULT '[]';
ALTER TABLE software_catalog ADD COLUMN IF NOT EXISTS sre_team JSONB DEFAULT '[]';
ALTER TABLE software_catalog ADD COLUMN IF NOT EXISTS architects JSONB DEFAULT '[]';
ALTER TABLE software_catalog ADD COLUMN IF NOT EXISTS runbook_url VARCHAR(500);
ALTER TABLE software_catalog ADD COLUMN IF NOT EXISTS dashboard_url VARCHAR(500);
ALTER TABLE software_catalog ADD COLUMN IF NOT EXISTS dependencies JSONB DEFAULT '[]';

-- A2A Agent Registry (replaces simple agents table for A2A-compatible agents)
CREATE TABLE a2a_agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    agent_type VARCHAR(50) NOT NULL,
    endpoint_url VARCHAR(500) NOT NULL,
    -- Agent Card fields (Google A2A spec)
    agent_card JSONB NOT NULL DEFAULT '{}',
    -- Skills this agent can handle
    skills JSONB NOT NULL DEFAULT '[]',
    -- Which software catalog entries this agent is allowed to work on
    allowed_software_ids JSONB DEFAULT '[]',
    -- Auth for calling this agent
    auth_type VARCHAR(20) DEFAULT 'none' CHECK (auth_type IN ('none', 'bearer', 'api_key', 'mtls')),
    auth_credentials VARCHAR(500),
    enabled BOOLEAN NOT NULL DEFAULT true,
    health_status VARCHAR(20) DEFAULT 'unknown' CHECK (health_status IN ('healthy', 'unhealthy', 'unknown')),
    last_health_check TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- A2A Tasks (tracks task dispatched to agents)
CREATE TABLE a2a_tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES a2a_agents(id),
    agent_run_id UUID REFERENCES agent_runs(id),
    -- A2A Task fields
    task_type VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'submitted', 'working', 'input_required', 'completed', 'failed', 'canceled')),
    input_message JSONB NOT NULL DEFAULT '{}',
    output_artifacts JSONB DEFAULT '[]',
    error_message TEXT,
    -- Orchestrator context
    orchestrator_reasoning TEXT,
    priority INTEGER DEFAULT 0,
    depends_on UUID REFERENCES a2a_tasks(id),
    -- Timing
    submitted_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Orchestrator decisions log
CREATE TABLE orchestrator_decisions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    decision_type VARCHAR(50) NOT NULL CHECK (decision_type IN ('triage', 'agent_selection', 'escalation', 'resolution')),
    reasoning TEXT NOT NULL,
    selected_agents JSONB DEFAULT '[]',
    context_used JSONB DEFAULT '{}',
    confidence NUMERIC(3,2) DEFAULT 0.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_a2a_agents_org ON a2a_agents(org_id);
CREATE INDEX idx_a2a_agents_type ON a2a_agents(agent_type);
CREATE INDEX idx_a2a_tasks_incident ON a2a_tasks(incident_id);
CREATE INDEX idx_a2a_tasks_agent ON a2a_tasks(agent_id);
CREATE INDEX idx_a2a_tasks_status ON a2a_tasks(status);
CREATE INDEX idx_orchestrator_decisions_incident ON orchestrator_decisions(incident_id);
