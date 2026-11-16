-- Read-only compatibility gate for migration 037 (analyst.v_audio_files and the four columns appended to analyst.v_speech_segments). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
BEGIN
  IF to_regclass('analyst.v_speech_segments') IS NULL OR to_regclass('analyst.v_video_files') IS NULL THEN
    RAISE EXCEPTION 'migration 023 must be applied before migration 037 (analyst.v_speech_segments or analyst.v_video_files is missing)';
  END IF;
  IF (SELECT string_agg(a.attname, ',' ORDER BY a.attnum) FROM pg_attribute a WHERE a.attrelid = 'analyst.v_speech_segments'::regclass AND a.attnum > 0 AND NOT a.attisdropped)
     IS DISTINCT FROM 'evidence_id,original_filename,start_seconds,end_seconds,text,requested_language,detected_language,model,needs_review,created_at' THEN
    RAISE EXCEPTION 'analyst.v_speech_segments has other columns than migration 023 gave it (or migration 037 is already applied): CREATE OR REPLACE VIEW could not append to it safely';
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('evidence_id'), ('tenant_id'), ('collection_id'), ('original_filename'), ('modality'), ('media_metadata'), ('size_bytes'), ('processing_status'), ('created_at')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns k WHERE k.table_schema = 'forensic' AND k.table_name = 'evidence_items' AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.evidence_items lacks the columns the audio view reads: %', missing;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    RAISE EXCEPTION 'the role forensic_analyst_ro is missing (migration 017)';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = 'analyst' AND c.relname = 'v_audio_files' AND c.relkind <> 'v') THEN
    RAISE EXCEPTION 'a relation named v_audio_files exists in schema analyst and is not a view';
  END IF;
  RAISE NOTICE 'migration 037 preflight passed';
END
$$;

ROLLBACK;
