DROP INDEX IF EXISTS idx_incident_evidence_retention;
DROP INDEX IF EXISTS idx_incidents_retention;

DROP INDEX IF EXISTS idx_archived_records_resource;
DROP INDEX IF EXISTS idx_archived_records_org;

DROP INDEX IF EXISTS idx_retention_policies_org_enabled;
DROP INDEX IF EXISTS idx_retention_policies_org;

DROP TABLE IF EXISTS archived_records;
DROP TABLE IF EXISTS retention_policies;
