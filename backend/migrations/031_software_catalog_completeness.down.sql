-- Revert dependencies to flat slug-string arrays.
UPDATE software_catalog
SET dependencies = (
    SELECT jsonb_agg(elem->>'slug')
    FROM jsonb_array_elements(dependencies) AS elem
)
WHERE jsonb_typeof(dependencies) = 'array'
  AND jsonb_array_length(dependencies) > 0
  AND jsonb_typeof(dependencies->0) = 'object';

ALTER TABLE software_catalog DROP COLUMN IF EXISTS type;
ALTER TABLE software_catalog DROP COLUMN IF EXISTS criticality;
