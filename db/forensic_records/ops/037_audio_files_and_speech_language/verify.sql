-- Read-only verification of migration 037. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  v text;
BEGIN
  FOREACH v IN ARRAY ARRAY['v_audio_files', 'v_speech_segments'] LOOP
    IF to_regclass('analyst.' || v) IS NULL THEN
      RAISE EXCEPTION 'analyst.% is missing (migration 037 is not applied)', v;
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
    IF pg_get_viewdef(('analyst.' || v)::regclass) NOT LIKE '%app.tenant_id%' OR pg_get_viewdef(('analyst.' || v)::regclass) NOT LIKE '%app.collection_id%' THEN
      RAISE EXCEPTION 'analyst.% has lost its tenant or collection predicate', v;
    END IF;
  END LOOP;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_speech_segments'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'evidence_id,original_filename,start_seconds,end_seconds,text,requested_language,detected_language,model,needs_review,created_at,language,language_name,reading_status,source_kind' THEN
    RAISE EXCEPTION 'analyst.v_speech_segments has other columns than migration 037 gave it';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_audio_files'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'evidence_id,original_filename,duration_seconds,sample_rate_hz,channels,language,language_name,reading_status,segments,size_bytes,processing_status,created_at,needs_review' THEN
    RAISE EXCEPTION 'analyst.v_audio_files has other columns than migration 037 gave it';
  END IF;
  RAISE NOTICE 'migration_037_verified';
END
$$;

ROLLBACK;
