-- Configurable data retention & archival for evidence and closed incidents.
--
-- v1 scope: this migration only adds the schema. There is no built-in cron
-- runner yet; sweeps are triggered manually via
-- POST /api/v1/retention-policies/sweep (see retention_handlers.go). The
-- service layer (RunRetentionSweep / RunAllOrgsSweep) is structured so a
-- future scheduled job (a ticker goroutine in main.go, or an external cron
-- hitting the sweep endpoint) can reuse it without any changes.

-- Per-org, per-resource-type retention configuration.
CREATE TABLE retention_policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    resource_type VARCHAR(20) NOT NULL CHECK (resource_type IN ('evidence', 'incidents', 'agent_runs')),
    retention_days INTEGER NOT NULL CHECK (retention_days > 0),
    action VARCHAR(10) NOT NULL DEFAULT 'archive' CHECK (action IN ('archive', 'delete')),
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- One policy per resource type per org keeps sweep logic unambiguous.
    UNIQUE(org_id, resource_type)
);

-- Append-only audit-style snapshot of rows removed (or about to be removed)
-- by a retention sweep with action='archive'. Not a working table -- there
-- is no update path, only inserts (by the sweep) and reads (for audit /
-- restore tooling, out of scope for v1).
CREATE TABLE archived_records (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    resource_type VARCHAR(20) NOT NULL CHECK (resource_type IN ('evidence', 'incidents', 'agent_runs')),
    resource_id UUID NOT NULL,
    archived_data JSONB NOT NULL,
    archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_retention_policies_org ON retention_policies(org_id);
CREATE INDEX idx_retention_policies_org_enabled ON retention_policies(org_id, enabled);

CREATE INDEX idx_archived_records_org ON archived_records(org_id);
CREATE INDEX idx_archived_records_resource ON archived_records(org_id, resource_type, resource_id);

-- Support "find rows older than N days for org X" retention sweeps.
-- idx_incidents_status already covers (org_id, status); this adds
-- resolved_at so the sweep's cutoff filter can use the index directly.
CREATE INDEX IF NOT EXISTS idx_incidents_retention ON incidents(org_id, status, resolved_at);

-- incident_evidence only had an index on (incident_id); add collected_at
-- (the evidence table's actual timestamp column -- there is no
-- incident_evidence.created_at) so expired-evidence lookups joined through
-- incidents(org_id) can range-scan efficiently per incident.
CREATE INDEX IF NOT EXISTS idx_incident_evidence_retention ON incident_evidence(incident_id, collected_at);
