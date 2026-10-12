-- Read-only compatibility gate for migration 012 (typed event and registration tables).
-- Run it before 012 with ON_ERROR_STOP enabled. Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  rel text;
  kind "char";
  has_col boolean;
BEGIN
  IF to_regclass('forensic.evidence_items') IS NULL
     OR to_regclass('forensic.derived_artifacts') IS NULL
     OR to_regclass('forensic.evidence_items_tenant_identity_idx') IS NULL THEN
    RAISE EXCEPTION 'migration 008 must be applied before migration 012';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM pg_index i
    WHERE i.indrelid = 'forensic.derived_artifacts'::regclass
      AND i.indisunique
      AND (
        SELECT array_agg(a.attname::text ORDER BY k.ord)
        FROM unnest(i.indkey::int2[]) WITH ORDINALITY AS k(attnum, ord)
        JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = k.attnum
      ) = ARRAY['tenant_id', 'collection_id', 'artifact_id']
  ) THEN
    RAISE EXCEPTION 'forensic.derived_artifacts has no unique key on (tenant_id, collection_id, artifact_id)';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'timescaledb') THEN
    RAISE EXCEPTION 'the timescaledb extension is not installed in this database';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm')
     AND NOT EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'pg_trgm') THEN
    RAISE EXCEPTION 'the pg_trgm extension is neither installed nor available';
  END IF;

  IF current_setting('server_encoding') <> 'UTF8' THEN
    RAISE EXCEPTION 'server_encoding is %, but forensic.normalize_plate needs UTF8 for Unicode normalization', current_setting('server_encoding');
  END IF;

  IF NOT has_schema_privilege(current_user, 'forensic', 'CREATE') THEN
    RAISE EXCEPTION 'role % cannot create objects in schema forensic', current_user;
  END IF;

  -- A same-named object that migration 012 did not create would be skipped silently by CREATE ... IF NOT EXISTS.
  FOREACH rel IN ARRAY ARRAY['layout_versions', 'comm_event', 'data_session', 'app_usage', 'plate_sighting',
                             'subscriber_identity', 'subscriber_registration']
  LOOP
    SELECT c.relkind INTO kind
    FROM pg_class c
    WHERE c.relnamespace = 'forensic'::regnamespace AND c.relname = rel;

    IF kind IS NULL THEN
      CONTINUE;
    END IF;
    IF kind <> 'r' THEN
      RAISE EXCEPTION 'forensic.% exists but is not an ordinary table (relkind %)', rel, kind;
    END IF;

    SELECT EXISTS (
      SELECT 1 FROM pg_attribute a
      WHERE a.attrelid = ('forensic.' || rel)::regclass AND a.attname IN ('layout_version', 'definition_sha256', 'identity_ref') AND NOT a.attisdropped
    ) INTO has_col;
    IF NOT has_col THEN
      RAISE EXCEPTION 'forensic.% already exists with a shape migration 012 does not recognise', rel;
    END IF;
  END LOOP;

  IF to_regprocedure('forensic.normalize_plate(text)') IS NOT NULL
     AND (SELECT p.prorettype <> 'text'::regtype FROM pg_proc p WHERE p.oid = to_regprocedure('forensic.normalize_plate(text)')) THEN
    RAISE EXCEPTION 'forensic.normalize_plate(text) exists with a different return type';
  END IF;

  RAISE NOTICE 'database % size: %', current_database(), pg_size_pretty(pg_database_size(current_database()));
END
$$;

ROLLBACK;
