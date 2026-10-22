-- Read-only compatibility gate for migration 023 (the media views v_ocr_regions, v_ocr_evidence_summary, v_speech_segments, v_plate_reads). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
BEGIN
  IF to_regclass('analyst.v_evidence_inventory') IS NULL THEN
    RAISE EXCEPTION 'migration 020 must be applied before migration 023 (analyst.v_evidence_inventory is missing)';
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('artifact_id'), ('tenant_id'), ('collection_id'), ('evidence_id'), ('artifact_type'), ('processing_status'), ('confidence'), ('citation_locator'), ('metadata'), ('created_at')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns k WHERE k.table_schema = 'forensic' AND k.table_name = 'derived_artifacts' AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.derived_artifacts lacks the columns the media views read: %', missing;
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('evidence_id'), ('tenant_id'), ('collection_id'), ('original_filename')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns k WHERE k.table_schema = 'forensic' AND k.table_name = 'evidence_items' AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.evidence_items lacks the columns the media views read: %', missing;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    RAISE EXCEPTION 'the role forensic_analyst_ro is missing (migration 017)';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = 'analyst' AND c.relname IN ('v_ocr_regions', 'v_ocr_evidence_summary', 'v_speech_segments', 'v_plate_reads') AND c.relkind <> 'v') THEN
    RAISE EXCEPTION 'a relation named like a new view exists in schema analyst and is not a view';
  END IF;
  RAISE NOTICE 'migration 023 preflight passed';
END
$$;

ROLLBACK;
