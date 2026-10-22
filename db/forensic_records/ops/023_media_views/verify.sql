-- Read-only verification of migration 023. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  v text;
BEGIN
  FOREACH v IN ARRAY ARRAY['v_ocr_regions', 'v_ocr_evidence_summary', 'v_speech_segments', 'v_plate_reads'] LOOP
    IF to_regclass('analyst.' || v) IS NULL THEN
      RAISE EXCEPTION 'analyst.% is missing (migration 023 is not applied)', v;
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
    IF pg_get_viewdef(('analyst.' || v)::regclass) NOT LIKE '%app.tenant_id%' AND v <> 'v_ocr_evidence_summary' THEN
      RAISE EXCEPTION 'analyst.% has lost its tenant predicate', v;
    END IF;
    IF pg_get_viewdef(('analyst.' || v)::regclass) NOT LIKE '%app.collection_id%' AND v <> 'v_ocr_evidence_summary' THEN
      RAISE EXCEPTION 'analyst.% has lost its collection predicate', v;
    END IF;
  END LOOP;
  IF pg_get_viewdef('analyst.v_ocr_evidence_summary'::regclass) NOT LIKE '%v_ocr_regions%' THEN
    RAISE EXCEPTION 'analyst.v_ocr_evidence_summary does not read analyst.v_ocr_regions (it takes its scope from it)';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_plate_reads'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'evidence_id,original_filename,source,plate,plate_as_read,detection_confidence,read_confidence,sightings,first_seen_seconds,last_seen_seconds,needs_review,created_at' THEN
    RAISE EXCEPTION 'analyst.v_plate_reads has other columns than the migration gave it';
  END IF;
  RAISE NOTICE 'migration_023_verified';
END
$$;

ROLLBACK;
