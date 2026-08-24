-- war_room_created was added to WarRoomService.CreateWarRoom's timeline
-- event write (backend/internal/services/warroom_service.go) but never
-- added to this CHECK constraint -- every war room creation has been
-- silently failing to write its timeline event since the feature shipped
-- (best-effort insert, only logged as a warning, never surfaced).
ALTER TABLE incident_events
    DROP CONSTRAINT incident_events_type_check;

ALTER TABLE incident_events
    ADD CONSTRAINT incident_events_type_check
    CHECK (type IN (
        'alert_received', 'triage_started', 'triage_completed', 'evidence_collected',
        'hypothesis_generated', 'status_changed', 'comment', 'agent_action',
        'rci_started', 'rci_completed', 'rca_started', 'rca_completed',
        'postmortem_started', 'postmortem_completed', 'agent_run_started',
        'agent_run_completed', 'agent_run_failed', 'correlated_alert',
        'war_room_created'
    ));
