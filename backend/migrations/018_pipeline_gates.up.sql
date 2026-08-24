-- Human-in-the-loop (HITL) approval gate before the postmortem pipeline stage.

-- Per-org toggle: when enabled, the orchestrator pauses after RCA and waits
-- for a human to approve the postmortem stage instead of auto-generating it.
ALTER TABLE organizations
    ADD COLUMN pipeline_hitl_gate_enabled BOOLEAN NOT NULL DEFAULT false;

-- Per-incident gate state. awaiting_approval_stage is set (e.g. 'postmortem')
-- while the pipeline is paused; approved_by/approved_at record who cleared
-- the gate and when. Mirrors the existing DAG/status modeling on incidents
-- rather than agent_runs, since the gate applies to the pipeline as a whole
-- (between stages), not to a single agent execution.
ALTER TABLE incidents
    ADD COLUMN awaiting_approval_stage TEXT NULL,
    ADD COLUMN approved_by UUID NULL REFERENCES users(id),
    ADD COLUMN approved_at TIMESTAMPTZ NULL;

CREATE INDEX idx_incidents_awaiting_approval ON incidents(awaiting_approval_stage) WHERE awaiting_approval_stage IS NOT NULL;
