-- Read-only compatibility gate for migration 019 (analyst.v_cells knows the positions the records carry). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
BEGIN
  IF to_regclass('analyst.v_cells') IS NULL THEN
    RAISE EXCEPTION 'migration 017 must be applied before migration 019 (analyst.v_cells is missing)';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'analyst' AND c.relname = 'v_cells' AND c.relkind = 'v') THEN
    RAISE EXCEPTION 'analyst.v_cells exists and is not a view';
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('tenant_id'), ('collection_id'), ('cell_key'), ('lat'), ('lon'), ('loc_uncertainty_m'), ('quality_flags')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns k WHERE k.table_schema = 'forensic' AND k.table_name = 'comm_event' AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.comm_event lacks the columns the view reads: %', missing;
  END IF;
  -- the columns of v_cells must stay as they are (CREATE OR REPLACE VIEW refuses another shape, and the Ask engine's catalogue lists them)
  IF (SELECT string_agg(a.attname || ':' || format_type(a.atttypid, a.atttypmod), ',' ORDER BY a.attnum) FROM pg_attribute a
      WHERE a.attrelid = 'analyst.v_cells'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'cell_key:text,site_name:text,operator:text,lat:double precision,lon:double precision,radius_m:integer' THEN
    RAISE EXCEPTION 'analyst.v_cells does not have the columns migration 017 gave it';
  END IF;
  RAISE NOTICE 'migration 019 preflight passed';
END
$$;

ROLLBACK;
