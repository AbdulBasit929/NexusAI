-- Read-only compatibility gate for migration 038 (image search). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
  installed text;
BEGIN
  IF to_regclass('forensic.evidence_items') IS NULL OR to_regclass('forensic.derived_artifacts') IS NULL
     OR to_regclass('forensic.evidence_items_tenant_identity_idx') IS NULL THEN
    RAISE EXCEPTION 'migration 008 must be applied before migration 038';
  END IF;
  IF to_regclass('forensic.content_embedding') IS NULL THEN
    RAISE EXCEPTION 'migration 015 must be applied before migration 038 (forensic.content_embedding is missing)';
  END IF;
  SELECT e.extversion INTO installed FROM pg_extension e WHERE e.extname = 'vector';
  IF installed IS NULL THEN
    RAISE EXCEPTION 'the vector extension is not installed (migration 015 installs it)';
  END IF;
  -- bit_count(bit) needs PostgreSQL 14 or later.
  IF current_setting('server_version_num')::int < 140000 THEN
    RAISE EXCEPTION 'PostgreSQL 14 or later is required (bit_count on bit strings); this server is %', current_setting('server_version');
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('evidence_id'), ('tenant_id'), ('collection_id'), ('case_id'), ('original_filename'), ('content_type'), ('sha256'), ('modality'),
               ('processing_status'), ('current_version_id'), ('created_at')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns k WHERE k.table_schema = 'forensic' AND k.table_name = 'evidence_items' AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.evidence_items lacks the columns the image search reads: %', missing;
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('artifact_id'), ('version_id'), ('tenant_id'), ('collection_id')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns k WHERE k.table_schema = 'forensic' AND k.table_name = 'derived_artifacts' AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.derived_artifacts lacks the columns the image search reads: %', missing;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = 'forensic' AND c.relname IN ('image_hash', 'image_search_log') AND c.relkind <> 'r') THEN
    RAISE EXCEPTION 'a relation named image_hash or image_search_log exists in schema forensic and is not a table';
  END IF;
  RAISE NOTICE 'migration 038 preflight passed (server %, vector %)', current_setting('server_version'), installed;
END
$$;

ROLLBACK;
