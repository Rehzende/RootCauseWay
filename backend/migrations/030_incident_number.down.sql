DROP INDEX IF EXISTS incidents_org_incident_number_idx;
ALTER TABLE incidents DROP COLUMN IF EXISTS incident_number;
ALTER TABLE organizations DROP COLUMN IF EXISTS next_incident_number;
