-- Agent Execution Runs (DAG nodes)
CREATE TABLE agent_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id),
    agent_name VARCHAR(255) NOT NULL,
    agent_type VARCHAR(30) NOT NULL CHECK (agent_type IN ('triage', 'evidence_analysis', 'hypothesis', 'rci_generator', 'rca_generator', 'postmortem_generator', 'debug', 'custom')),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'skipped')),
    parent_run_id UUID REFERENCES agent_runs(id),
    input_data JSONB DEFAULT '{}',
    output_data JSONB DEFAULT '{}',
    error_message TEXT,
    model_used VARCHAR(255),
    tokens_used INTEGER DEFAULT 0,
    duration_ms INTEGER DEFAULT 0,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Evidence Blobs (binary/large evidence files)
ALTER TABLE incident_evidence ADD COLUMN blob_path VARCHAR(1000);
ALTER TABLE incident_evidence ADD COLUMN blob_size_bytes BIGINT;
ALTER TABLE incident_evidence ADD COLUMN mime_type VARCHAR(100);
ALTER TABLE incident_evidence DROP CONSTRAINT IF EXISTS incident_evidence_type_check;
ALTER TABLE incident_evidence ADD CONSTRAINT incident_evidence_type_check
    CHECK (type IN ('log', 'metric', 'trace', 'snapshot', 'agent_output', 'manual', 'screenshot', 'dashboard', 'config', 'heap_dump'));

-- RCI (Root Cause Investigation)
CREATE TABLE incident_rci (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    agent_run_id UUID REFERENCES agent_runs(id),
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'in_progress', 'completed', 'reviewed')),
    investigation_summary TEXT,
    impact_assessment JSONB DEFAULT '{}',
    affected_services JSONB DEFAULT '[]',
    affected_users_estimate INTEGER,
    detection_method VARCHAR(100),
    detection_time TIMESTAMPTZ,
    acknowledgment_time TIMESTAMPTZ,
    time_to_detect_seconds INTEGER,
    evidence_ids JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- RCA (Root Cause Analysis)
CREATE TABLE incident_rca (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    rci_id UUID REFERENCES incident_rci(id),
    agent_run_id UUID REFERENCES agent_runs(id),
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'in_progress', 'completed', 'reviewed')),
    root_cause_summary TEXT NOT NULL,
    root_cause_category VARCHAR(100),
    contributing_factors JSONB DEFAULT '[]',
    five_whys JSONB DEFAULT '[]',
    confidence NUMERIC(3,2) DEFAULT 0.0,
    evidence_ids JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Postmortem
CREATE TABLE incident_postmortem (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    rca_id UUID REFERENCES incident_rca(id),
    agent_run_id UUID REFERENCES agent_runs(id),
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'in_review', 'published')),
    title VARCHAR(500),
    executive_summary TEXT,
    incident_timeline_narrative TEXT,
    root_cause_detail TEXT,
    impact_detail TEXT,
    lessons_learned JSONB DEFAULT '[]',
    action_items JSONB DEFAULT '[]',
    what_went_well JSONB DEFAULT '[]',
    what_went_wrong JSONB DEFAULT '[]',
    prevention_measures JSONB DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

-- Update incident_events to support new event types
ALTER TABLE incident_events DROP CONSTRAINT IF EXISTS incident_events_type_check;
ALTER TABLE incident_events ADD CONSTRAINT incident_events_type_check
    CHECK (type IN (
        'alert_received', 'triage_started', 'triage_completed',
        'evidence_collected', 'hypothesis_generated',
        'status_changed', 'comment', 'agent_action',
        'rci_started', 'rci_completed',
        'rca_started', 'rca_completed',
        'postmortem_started', 'postmortem_completed',
        'agent_run_started', 'agent_run_completed', 'agent_run_failed'
    ));

-- Indexes
CREATE INDEX idx_agent_runs_incident ON agent_runs(incident_id);
CREATE INDEX idx_agent_runs_parent ON agent_runs(parent_run_id);
CREATE INDEX idx_agent_runs_status ON agent_runs(incident_id, status);
CREATE INDEX idx_incident_rci_incident ON incident_rci(incident_id);
CREATE INDEX idx_incident_rca_incident ON incident_rca(incident_id);
CREATE INDEX idx_incident_postmortem_incident ON incident_postmortem(incident_id);
