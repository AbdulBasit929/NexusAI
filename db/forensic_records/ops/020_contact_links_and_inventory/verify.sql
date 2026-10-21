-- Read-only verification of migration 020. Any exception means the migration is incomplete or was altered. Run ops/017_analyst_views/verify.sql as well: it still holds for the views it names.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  v text;
BEGIN
  FOREACH v IN ARRAY ARRAY['v_contact_links', 'v_connection_edges', 'v_evidence_inventory'] LOOP
    IF to_regclass('analyst.' || v) IS NULL THEN
      RAISE EXCEPTION 'analyst.% is missing (migration 020 is not applied)', v;
    END IF;
    IF obj_description(('analyst.' || v)::regclass, 'pg_class') IS NULL THEN
      RAISE EXCEPTION 'analyst.% has no description', v;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_attribute a WHERE a.attrelid = ('analyst.' || v)::regclass AND a.attnum > 0 AND NOT a.attisdropped AND col_description(a.attrelid, a.attnum) IS NULL) THEN
      RAISE EXCEPTION 'a column of analyst.% has no description', v;
    END IF;
    IF has_table_privilege('forensic_analyst_ro', ('analyst.' || v)::regclass, 'SELECT') IS NOT TRUE THEN
      RAISE EXCEPTION 'forensic_analyst_ro cannot read analyst.%', v;
    END IF;
    IF has_table_privilege('public', ('analyst.' || v)::regclass, 'SELECT') THEN
      RAISE EXCEPTION 'PUBLIC can read analyst.%', v;
    END IF;
  END LOOP;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_connection_edges'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'a,b,src,dst,hops,ordinary,rank_any,rank_ordinary' THEN
    RAISE EXCEPTION 'analyst.v_connection_edges has other columns than a, b, src, dst, hops, ordinary, rank_any, rank_ordinary';
  END IF;
  IF pg_get_viewdef('analyst.v_connection_edges'::regclass) NOT LIKE '%v_contact_links%' THEN
    RAISE EXCEPTION 'analyst.v_connection_edges does not read analyst.v_contact_links';
  END IF;
  IF pg_get_viewdef('analyst.v_evidence_inventory'::regclass) NOT LIKE '%app.tenant_id%' OR pg_get_viewdef('analyst.v_evidence_inventory'::regclass) NOT LIKE '%app.collection_id%' THEN
    RAISE EXCEPTION 'analyst.v_evidence_inventory has lost its tenant or collection predicate';
  END IF;
  RAISE NOTICE 'migration_020_verified';
END
$$;

ROLLBACK;
