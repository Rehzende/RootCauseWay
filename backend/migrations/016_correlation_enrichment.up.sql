-- Correlation enrichment: dependency-graph cascade correlation + fingerprint dedup support.

-- Allow the correlation engine to record dedup/cascade matches on the incident timeline.
ALTER TABLE incident_events DROP CONSTRAINT IF EXISTS incident_events_type_check;
ALTER TABLE incident_events ADD CONSTRAINT incident_events_type_check
    CHECK (type IN (
        'alert_received', 'triage_started', 'triage_completed',
        'evidence_collected', 'hypothesis_generated',
        'status_changed', 'comment', 'agent_action',
        'rci_started', 'rci_completed',
        'rca_started', 'rca_completed',
        'postmortem_started', 'postmortem_completed',
        'agent_run_started', 'agent_run_completed', 'agent_run_failed',
        'correlated_alert'
    ));

-- Allow correlation rules to describe the two new match strategies.
ALTER TABLE correlation_rules DROP CONSTRAINT IF EXISTS correlation_rules_rule_type_check;
ALTER TABLE correlation_rules ADD CONSTRAINT correlation_rules_rule_type_check
    CHECK (rule_type IN ('same_service', 'time_window', 'error_pattern', 'custom', 'dependency_cascade', 'fingerprint_dedup'));

-- Speed up fingerprint-based incident lookup (alert.received payloads that carry a
-- "fingerprint" key get persisted into alert_snapshots.normalized as JSONB).
CREATE INDEX IF NOT EXISTS idx_alert_snapshots_fingerprint
    ON alert_snapshots ((normalized->>'fingerprint'))
    WHERE normalized->>'fingerprint' IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_alert_snapshots_incident_created
    ON alert_snapshots (incident_id, created_at DESC);

-- Speed up "open incidents on related services" cascade lookups.
CREATE INDEX IF NOT EXISTS idx_incidents_org_software_status
    ON incidents (org_id, software_id, status);
