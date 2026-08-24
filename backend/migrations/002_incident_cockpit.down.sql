DROP TABLE IF EXISTS incident_postmortem;
DROP TABLE IF EXISTS incident_rca;
DROP TABLE IF EXISTS incident_rci;
DROP TABLE IF EXISTS agent_runs;
ALTER TABLE incident_evidence DROP COLUMN IF EXISTS blob_path;
ALTER TABLE incident_evidence DROP COLUMN IF EXISTS blob_size_bytes;
ALTER TABLE incident_evidence DROP COLUMN IF EXISTS mime_type;
