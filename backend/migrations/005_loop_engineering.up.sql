-- Incident Feedback (Human-in-the-Loop)
CREATE TABLE incident_feedback (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id),
    target_type VARCHAR(20) NOT NULL CHECK (target_type IN ('rci', 'rca', 'postmortem', 'triage', 'evidence')),
    rating VARCHAR(10) NOT NULL CHECK (rating IN ('positive', 'negative', 'neutral')),
    correction TEXT,
    original_data JSONB DEFAULT '{}',
    corrected_data JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Knowledge Base (Outer Loop)
CREATE TABLE knowledge_base (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    incident_id UUID REFERENCES incidents(id),
    software_id UUID REFERENCES software_catalog(id),
    category VARCHAR(100),
    error_pattern VARCHAR(500),
    root_cause_summary TEXT NOT NULL,
    resolution_summary TEXT,
    lessons_learned JSONB DEFAULT '[]',
    action_items JSONB DEFAULT '[]',
    tags JSONB DEFAULT '[]',
    human_validated BOOLEAN DEFAULT false,
    confidence NUMERIC(3,2) DEFAULT 0.0,
    times_referenced INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Similar Incidents tracking
CREATE TABLE similar_incidents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    similar_incident_id UUID NOT NULL REFERENCES incidents(id),
    similarity_score NUMERIC(3,2) NOT NULL,
    matched_on JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_incident_feedback_incident ON incident_feedback(incident_id);
CREATE INDEX idx_knowledge_base_org ON knowledge_base(org_id);
CREATE INDEX idx_knowledge_base_software ON knowledge_base(software_id);
CREATE INDEX idx_knowledge_base_category ON knowledge_base(org_id, category);
CREATE INDEX idx_knowledge_base_pattern ON knowledge_base(error_pattern);
CREATE INDEX idx_similar_incidents_incident ON similar_incidents(incident_id);
