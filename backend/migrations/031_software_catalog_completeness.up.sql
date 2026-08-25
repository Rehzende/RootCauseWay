-- Software catalog completeness: a business-impact tier ("criticality") and
-- an entry kind ("type"), plus upgrading `dependencies` from a flat array of
-- slug strings to typed relations -- so the correlation engine and RCA
-- context can distinguish a hard runtime dependency from something looser
-- (e.g. "uses_api_of", "shares_database_with"), not just "these are related".
ALTER TABLE software_catalog ADD COLUMN criticality VARCHAR(20) NOT NULL DEFAULT 'medium'
    CHECK (criticality IN ('critical', 'high', 'medium', 'low'));
ALTER TABLE software_catalog ADD COLUMN type VARCHAR(20) NOT NULL DEFAULT 'service'
    CHECK (type IN ('service', 'library', 'database', 'job', 'website', 'other'));

-- Backfill: the old `dependencies` shape was a flat JSON array of slug
-- strings (["pulse-postgres", ...]); the new shape is typed objects
-- ([{"slug": "pulse-postgres", "relation": "depends_on"}]). Only rewrite
-- rows still in the old flat-string shape -- idempotent / safe to rerun,
-- and a no-op for any row already migrated or created after this shipped.
UPDATE software_catalog
SET dependencies = (
    SELECT jsonb_agg(jsonb_build_object('slug', elem, 'relation', 'depends_on'))
    FROM jsonb_array_elements_text(dependencies) AS elem
)
WHERE jsonb_typeof(dependencies) = 'array'
  AND jsonb_array_length(dependencies) > 0
  AND jsonb_typeof(dependencies->0) = 'string';
