-- Revert correlation enrichment additions.

DROP INDEX IF EXISTS idx_incidents_org_software_status;
DROP INDEX IF EXISTS idx_alert_snapshots_incident_created;
DROP INDEX IF EXISTS idx_alert_snapshots_fingerprint;

ALTER TABLE correlation_rules DROP CONSTRAINT IF EXISTS correlation_rules_rule_type_check;
ALTER TABLE correlation_rules ADD CONSTRAINT correlation_rules_rule_type_check
    CHECK (rule_type IN ('same_service', 'time_window', 'error_pattern', 'custom'));

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
