-- Correlation Rules
CREATE TABLE correlation_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    rule_type VARCHAR(30) NOT NULL CHECK (rule_type IN ('same_service', 'time_window', 'error_pattern', 'custom')),
    config JSONB NOT NULL DEFAULT '{}',
    time_window_seconds INTEGER DEFAULT 300,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Alert Groups (correlated alerts under one incident)
CREATE TABLE alert_groups (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    alert_snapshot_id UUID NOT NULL REFERENCES alert_snapshots(id),
    correlation_rule_id UUID REFERENCES correlation_rules(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_correlation_rules_org ON correlation_rules(org_id);
CREATE INDEX idx_alert_groups_incident ON alert_groups(incident_id);
CREATE INDEX idx_alert_groups_snapshot ON alert_groups(alert_snapshot_id);
