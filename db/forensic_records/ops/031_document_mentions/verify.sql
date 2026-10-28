-- Read-only verification of migration 031. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  v text;
BEGIN
  IF to_regclass('forensic.document_mention') IS NULL THEN
    RAISE EXCEPTION 'forensic.document_mention is missing (migration 031 is not applied)';
  END IF;
  IF NOT (SELECT relrowsecurity FROM pg_class WHERE oid = 'forensic.document_mention'::regclass) THEN
    RAISE EXCEPTION 'row-level security is off on forensic.document_mention';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = 'document_mention' AND policyname = 'tenant_isolation_document_mention') THEN
    RAISE EXCEPTION 'the tenant policy of forensic.document_mention is missing';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'document_mention_withheld_chk' AND conrelid = 'forensic.document_mention'::regclass) THEN
    RAISE EXCEPTION 'the constraint that withholds national ID numbers is missing';
  END IF;
  IF EXISTS (SELECT 1 FROM forensic.document_mention WHERE kind = 'national_id' AND (value IS NOT NULL OR surface IS NOT NULL)) THEN
    RAISE EXCEPTION 'a national ID number is stored in forensic.document_mention: they are counted and withheld until the masking design is approved';
  END IF;
  FOREACH v IN ARRAY ARRAY['v_document_mentions', 'v_document_values', 'v_document_links'] LOOP
    IF to_regclass('analyst.' || v) IS NULL THEN
      RAISE EXCEPTION 'analyst.% is missing (migration 031 is not applied)', v;
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
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_document_mentions'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'mention_id,evidence_id,document,kind,value,surface,tier,rule,position_in_document,char_start,char_end' THEN
    RAISE EXCEPTION 'analyst.v_document_mentions has other columns than the migration gave it';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_document_links'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'kind,value,family,records' THEN
    RAISE EXCEPTION 'analyst.v_document_links has other columns than the migration gave it';
  END IF;
  IF pg_get_viewdef('analyst.v_document_mentions'::regclass) NOT LIKE '%app.tenant_id%' OR pg_get_viewdef('analyst.v_document_mentions'::regclass) NOT LIKE '%app.collection_id%' THEN
    RAISE EXCEPTION 'analyst.v_document_mentions no longer carries its own tenant and collection scope';
  END IF;
  FOREACH v IN ARRAY ARRAY['v_document_values', 'v_document_links'] LOOP
    IF pg_get_viewdef(('analyst.' || v)::regclass) NOT LIKE '%v_document_%' THEN
      RAISE EXCEPTION 'analyst.% does not read the document views (it takes its scope from them)', v;
    END IF;
  END LOOP;
  IF has_table_privilege('forensic_analyst_ro', 'forensic.document_mention', 'SELECT') THEN
    RAISE EXCEPTION 'forensic_analyst_ro can read forensic.document_mention directly: it may read only through the views';
  END IF;
  RAISE NOTICE 'migration_031_verified';
END
$$;

ROLLBACK;
