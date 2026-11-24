-- Read-only compatibility gate for migration 039 (known sets for images). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  missing text;
BEGIN
  IF to_regclass('forensic.evidence_items') IS NULL THEN
    RAISE EXCEPTION 'migration 008 must be applied before migration 039';
  END IF;
  IF to_regclass('forensic.image_hash') IS NULL OR to_regprocedure('forensic.image_index_state(text,text)') IS NULL THEN
    RAISE EXCEPTION 'migration 038 (image search) must be applied before migration 039 (forensic.image_hash or forensic.image_index_state is missing)';
  END IF;
  -- bit_count(bit) needs PostgreSQL 14 or later; gen_random_uuid() is built in from 13.
  IF current_setting('server_version_num')::int < 140000 THEN
    RAISE EXCEPTION 'PostgreSQL 14 or later is required (bit_count on bit strings); this server is %', current_setting('server_version');
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('evidence_id'), ('tenant_id'), ('collection_id'), ('case_id'), ('sha256'), ('modality'), ('current_version_id')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns k WHERE k.table_schema = 'forensic' AND k.table_name = 'evidence_items' AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.evidence_items lacks the columns the known-set check reads: %', missing;
  END IF;
  SELECT string_agg(c.name, ', ') INTO missing
  FROM (VALUES ('tenant_id'), ('collection_id'), ('evidence_id'), ('version_id'), ('pdq256')) AS c(name)
  WHERE NOT EXISTS (SELECT 1 FROM information_schema.columns k WHERE k.table_schema = 'forensic' AND k.table_name = 'image_hash' AND k.column_name = c.name);
  IF missing IS NOT NULL THEN
    RAISE EXCEPTION 'forensic.image_hash lacks the columns the known-set check reads: %', missing;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
             WHERE n.nspname = 'forensic' AND c.relname IN ('image_known_set', 'image_known_hash', 'image_known_event') AND c.relkind <> 'r') THEN
    RAISE EXCEPTION 'a relation named image_known_set, image_known_hash or image_known_event exists in schema forensic and is not a table';
  END IF;
  RAISE NOTICE 'migration 039 preflight passed (server %)', current_setting('server_version');
END
$$;

ROLLBACK;
