-- Change Events (deploys, config changes, infra changes)
CREATE TABLE change_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    software_id UUID REFERENCES software_catalog(id),
    change_type VARCHAR(30) NOT NULL CHECK (change_type IN ('deploy', 'config_change', 'infra_change', 'rollback', 'scale', 'custom')),
    title VARCHAR(500) NOT NULL,
    description TEXT,
    source VARCHAR(50),
    source_url VARCHAR(500),
    commit_sha VARCHAR(64),
    author VARCHAR(255),
    environment VARCHAR(50),
    metadata JSONB DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_change_events_org ON change_events(org_id);
CREATE INDEX idx_change_events_software ON change_events(software_id);
CREATE INDEX idx_change_events_occurred ON change_events(software_id, occurred_at);
CREATE INDEX idx_change_events_type ON change_events(change_type);
