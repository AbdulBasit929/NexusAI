-- Read-only verification of migration 019. Any exception means the migration is incomplete or was altered. Run ops/017_analyst_views/verify.sql as well: it still holds.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  def text;
BEGIN
  def := pg_get_viewdef('analyst.v_cells'::regclass);
  IF def NOT LIKE '%comm_event%' THEN
    RAISE EXCEPTION 'analyst.v_cells does not read the positions of the records (migration 019 is not applied)';
  END IF;
  IF def NOT LIKE '%app.tenant_id%' OR def NOT LIKE '%app.collection_id%' THEN
    RAISE EXCEPTION 'analyst.v_cells has lost its tenant or collection predicate';
  END IF;
  IF def NOT LIKE '%duplicate_row%' THEN
    RAISE EXCEPTION 'analyst.v_cells counts repeated rows';
  END IF;
  IF (SELECT string_agg(a.attname || ':' || format_type(a.atttypid, a.atttypmod), ',' ORDER BY a.attnum) FROM pg_attribute a
      WHERE a.attrelid = 'analyst.v_cells'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'cell_key:text,site_name:text,operator:text,lat:double precision,lon:double precision,radius_m:integer' THEN
    RAISE EXCEPTION 'analyst.v_cells has other columns than before';
  END IF;
  IF obj_description('analyst.v_cells'::regclass, 'pg_class') NOT LIKE '%records themselves%' THEN
    RAISE EXCEPTION 'the description of analyst.v_cells does not say where a position can come from';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = 'analyst.v_cells'::regclass AND a.attnum > 0 AND NOT a.attisdropped AND col_description(a.attrelid, a.attnum) IS NULL) THEN
    RAISE EXCEPTION 'a column of analyst.v_cells lost its description';
  END IF;
  IF has_table_privilege('forensic_analyst_ro', 'analyst.v_cells'::regclass, 'SELECT') IS NOT TRUE THEN
    RAISE EXCEPTION 'forensic_analyst_ro cannot read analyst.v_cells';
  END IF;
  RAISE NOTICE 'migration_019_verified';
END
$$;

ROLLBACK;
