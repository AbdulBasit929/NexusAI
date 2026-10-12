-- Read-only compatibility gate for migration 014 (entities, relationships, evidence). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  rel text;
  kind "char";
BEGIN
  IF to_regclass('forensic.evidence_items') IS NULL
     OR to_regclass('forensic.derived_artifacts') IS NULL
     OR to_regclass('forensic.evidence_items_tenant_identity_idx') IS NULL THEN
    RAISE EXCEPTION 'migration 008 must be applied before migration 014';
  END IF;

  IF to_regprocedure('forensic.touch_updated_at()') IS NULL THEN
    RAISE EXCEPTION 'forensic.touch_updated_at() (migration 007) is missing';
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

  IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm') THEN
    RAISE EXCEPTION 'the pg_trgm extension is not installed (migration 001 installs it)';
  END IF;

  IF NOT has_schema_privilege(current_user, 'forensic', 'CREATE') THEN
    RAISE EXCEPTION 'role % cannot create objects in schema forensic', current_user;
  END IF;

  FOREACH rel IN ARRAY ARRAY['derivation', 'entity', 'entity_mention', 'relationship', 'relationship_evidence', 'ip_assignment']
  LOOP
    SELECT c.relkind INTO kind FROM pg_class c WHERE c.relnamespace = 'forensic'::regnamespace AND c.relname = rel;
    IF kind IS NOT NULL AND kind <> 'r' THEN
      RAISE EXCEPTION 'forensic.% exists but is not an ordinary table (relkind %)', rel, kind;
    END IF;
  END LOOP;

  RAISE NOTICE 'database % size: %', current_database(), pg_size_pretty(pg_database_size(current_database()));
END
$$;

ROLLBACK;
