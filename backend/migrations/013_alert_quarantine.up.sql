-- Alert quarantine: stores alerts that couldn't be matched to a software entry
CREATE TABLE IF NOT EXISTS alert_quarantine (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    webhook_id UUID NOT NULL REFERENCES webhooks(id),
    source VARCHAR(50) NOT NULL,
    raw_payload JSONB NOT NULL,
    normalized_title VARCHAR(500),
    normalized_severity VARCHAR(20),
    labels JSONB DEFAULT '{}',
    reason VARCHAR(200) NOT NULL DEFAULT 'no_software_match',
    resolved BOOLEAN NOT NULL DEFAULT FALSE,
    resolved_at TIMESTAMPTZ,
    resolved_software_id UUID REFERENCES software_catalog(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_quarantine_org_resolved ON alert_quarantine(org_id, resolved);
CREATE INDEX idx_quarantine_created ON alert_quarantine(created_at DESC);

-- Make software_id nullable on webhooks (one webhook per source, not per software)
ALTER TABLE webhooks ALTER COLUMN software_id DROP NOT NULL;
