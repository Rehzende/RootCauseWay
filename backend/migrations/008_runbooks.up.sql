-- Runbooks
CREATE TABLE runbooks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    software_id UUID REFERENCES software_catalog(id),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    description TEXT,
    trigger_conditions JSONB DEFAULT '{}',
    auto_trigger BOOLEAN DEFAULT false,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, slug)
);

-- Runbook Steps
CREATE TABLE runbook_steps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    runbook_id UUID NOT NULL REFERENCES runbooks(id) ON DELETE CASCADE,
    step_order INTEGER NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    step_type VARCHAR(20) NOT NULL CHECK (step_type IN ('manual', 'automated', 'approval', 'notification', 'condition')),
    config JSONB NOT NULL DEFAULT '{}',
    skill_id UUID REFERENCES skills(id),
    timeout_seconds INTEGER DEFAULT 300,
    on_failure VARCHAR(20) DEFAULT 'stop' CHECK (on_failure IN ('stop', 'continue', 'retry')),
    max_retries INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Runbook Executions
CREATE TABLE runbook_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    runbook_id UUID NOT NULL REFERENCES runbooks(id),
    incident_id UUID REFERENCES incidents(id),
    triggered_by VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'canceled', 'waiting_approval')),
    current_step INTEGER DEFAULT 0,
    step_results JSONB DEFAULT '[]',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_runbooks_org ON runbooks(org_id);
CREATE INDEX idx_runbooks_software ON runbooks(software_id);
CREATE INDEX idx_runbook_steps_runbook ON runbook_steps(runbook_id);
CREATE INDEX idx_runbook_executions_incident ON runbook_executions(incident_id);
CREATE INDEX idx_runbook_executions_status ON runbook_executions(status);
