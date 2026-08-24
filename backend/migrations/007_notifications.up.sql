-- Notification Channels
CREATE TABLE notification_channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    channel_type VARCHAR(30) NOT NULL CHECK (channel_type IN ('slack', 'teams', 'pagerduty', 'email', 'webhook', 'custom')),
    config JSONB NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Escalation Policies
CREATE TABLE escalation_policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    software_id UUID REFERENCES software_catalog(id),
    severity_filter JSONB DEFAULT '["critical","high","medium","low"]',
    steps JSONB NOT NULL DEFAULT '[]',
    repeat_after_seconds INTEGER,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Notification Log
CREATE TABLE notification_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    incident_id UUID REFERENCES incidents(id),
    channel_id UUID REFERENCES notification_channels(id),
    policy_id UUID REFERENCES escalation_policies(id),
    event_type VARCHAR(50) NOT NULL,
    recipient VARCHAR(500),
    payload JSONB DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed', 'delivered')),
    error_message TEXT,
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notification_channels_org ON notification_channels(org_id);
CREATE INDEX idx_escalation_policies_org ON escalation_policies(org_id);
CREATE INDEX idx_escalation_policies_software ON escalation_policies(software_id);
CREATE INDEX idx_notification_log_incident ON notification_log(incident_id);
CREATE INDEX idx_notification_log_status ON notification_log(status);
