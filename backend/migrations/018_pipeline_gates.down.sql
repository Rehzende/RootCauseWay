DROP INDEX IF EXISTS idx_incidents_awaiting_approval;

ALTER TABLE incidents
    DROP COLUMN IF EXISTS awaiting_approval_stage,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS approved_at;

ALTER TABLE organizations
    DROP COLUMN IF EXISTS pipeline_hitl_gate_enabled;
