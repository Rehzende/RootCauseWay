-- Human-friendly, sequential-per-org incident code (INC-0001, INC-0002, ...)
-- shown alongside the incident's title everywhere it's displayed -- the
-- UUID primary key stays the real identifier for foreign keys and API
-- paths, this is purely a display/reference number.
--
-- next_incident_number lives on organizations (not a global SEQUENCE)
-- because numbering resets per org, matching how every other tool with
-- this feature (Jira, Linear, PagerDuty) scopes it. Assignment is a
-- transactional "UPDATE organizations ... RETURNING" immediately before
-- the incidents INSERT (see PgIncidentRepository.Create) -- Postgres's
-- row-level locking on the UPDATE serializes concurrent incident creates
-- for the same org, so two incidents can never be assigned the same
-- number.
ALTER TABLE organizations ADD COLUMN next_incident_number BIGINT NOT NULL DEFAULT 1;
ALTER TABLE incidents ADD COLUMN incident_number BIGINT;

-- Backfill existing incidents, oldest first per org, and advance each
-- org's counter past whatever was just backfilled so newly created
-- incidents don't collide with these numbers.
WITH numbered AS (
    SELECT id, org_id, ROW_NUMBER() OVER (PARTITION BY org_id ORDER BY created_at, id) AS rn
    FROM incidents
)
UPDATE incidents i
SET incident_number = numbered.rn
FROM numbered
WHERE i.id = numbered.id;

UPDATE organizations o
SET next_incident_number = COALESCE(
    (SELECT max(incident_number) + 1 FROM incidents WHERE org_id = o.id),
    1
);

ALTER TABLE incidents ALTER COLUMN incident_number SET NOT NULL;
CREATE UNIQUE INDEX incidents_org_incident_number_idx ON incidents(org_id, incident_number);
