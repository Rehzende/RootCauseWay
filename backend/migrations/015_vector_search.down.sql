DROP INDEX IF EXISTS idx_incidents_embedding;
DROP INDEX IF EXISTS idx_knowledge_base_embedding;
ALTER TABLE incidents DROP COLUMN IF EXISTS embedding;
ALTER TABLE knowledge_base DROP COLUMN IF EXISTS embedding;
-- Extension is left installed: other objects may depend on it.
