-- Read-only compatibility gate for migration 015 (vectors and earth distance). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  ext text;
  installed text;
  vector_version text;
BEGIN
  IF to_regclass('forensic.evidence_items') IS NULL
     OR to_regclass('forensic.derived_artifacts') IS NULL
     OR to_regclass('forensic.evidence_items_tenant_identity_idx') IS NULL THEN
    RAISE EXCEPTION 'migration 008 must be applied before migration 015';
  END IF;

  IF to_regclass('forensic.cell_site') IS NULL OR to_regclass('forensic.camera_site') IS NULL THEN
    RAISE EXCEPTION 'migration 013 must be applied before migration 015 (earth indexes sit on cell_site and camera_site)';
  END IF;

  -- The three extensions must be available in this image (an installed one is fine).
  FOREACH ext IN ARRAY ARRAY['vector', 'cube', 'earthdistance']
  LOOP
    SELECT e.extversion INTO installed FROM pg_extension e WHERE e.extname = ext;
    IF installed IS NULL AND NOT EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = ext) THEN
      RAISE EXCEPTION 'extension % is neither installed nor available in this image', ext;
    END IF;
  END LOOP;

  SELECT coalesce(
           (SELECT e.extversion FROM pg_extension e WHERE e.extname = 'vector'),
           (SELECT a.default_version FROM pg_available_extensions a WHERE a.name = 'vector')) INTO vector_version;
  IF string_to_array(vector_version, '.')::int[] < ARRAY[0, 5, 0] THEN
    RAISE EXCEPTION 'pgvector % is too old (need 0.5.0 or newer for vector_dims and HNSW)', vector_version;
  END IF;

  IF NOT has_schema_privilege(current_user, 'forensic', 'CREATE') THEN
    RAISE EXCEPTION 'role % cannot create objects in schema forensic', current_user;
  END IF;

  IF NOT (SELECT rolsuper FROM pg_roles WHERE rolname = current_user)
     AND NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname IN ('vector', 'cube', 'earthdistance')) THEN
    RAISE NOTICE 'role % is not a superuser: CREATE EXTENSION may require a trusted extension or another role', current_user;
  END IF;

  RAISE NOTICE 'pgvector version %', vector_version;
END
$$;

ROLLBACK;
