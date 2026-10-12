-- Read-only verification of migration 015. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  problem text;
  metres double precision;
BEGIN
  SELECT string_agg(e.name, ', ')
    INTO problem
  FROM (VALUES ('vector'), ('cube'), ('earthdistance')) AS e(name)
  WHERE NOT EXISTS (SELECT 1 FROM pg_extension x WHERE x.extname = e.name);
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'extensions not installed: %', problem;
  END IF;

  IF to_regclass('forensic.content_embedding') IS NULL THEN
    RAISE EXCEPTION 'forensic.content_embedding is missing';
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = 'forensic.content_embedding'::regclass AND c.relrowsecurity)
     OR NOT EXISTS (
       SELECT 1 FROM pg_policies p
       WHERE p.schemaname = 'forensic' AND p.tablename = 'content_embedding'
         AND p.policyname = 'tenant_isolation_content_embedding' AND p.cmd = 'ALL'
         AND p.qual IS NOT NULL AND p.with_check IS NOT NULL) THEN
    RAISE EXCEPTION 'row-level security or tenant policy missing on content_embedding';
  END IF;

  SELECT string_agg(c.conname, ', ')
    INTO problem
  FROM (VALUES ('content_embedding_evidence_fk'), ('content_embedding_artifact_fk'), ('content_embedding_modality_chk'),
               ('content_embedding_dim_chk')) AS c(conname)
  WHERE NOT EXISTS (
    SELECT 1 FROM pg_constraint k
    WHERE k.conrelid = 'forensic.content_embedding'::regclass AND k.conname = c.conname AND k.convalidated);
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'missing or unvalidated constraints: %', problem;
  END IF;

  SELECT string_agg(i.name, ', ')
    INTO problem
  FROM (VALUES ('cell_site_earth_idx'), ('camera_site_earth_idx')) AS i(name)
  WHERE NOT EXISTS (
    SELECT 1 FROM pg_indexes x WHERE x.schemaname = 'forensic' AND x.indexname = i.name AND x.indexdef LIKE '%USING gist%ll_to_earth%');
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'earth indexes missing: %', problem;
  END IF;

  -- Pure checks of the extension functions the lane will rely on.
  metres := earth_distance(ll_to_earth(0, 0), ll_to_earth(0, 1));
  IF metres NOT BETWEEN 110000 AND 112500 THEN
    RAISE EXCEPTION 'earth_distance for one degree of longitude at the equator is %, expected about 111 km', metres;
  END IF;
  IF round(('[1,2,3]'::vector <-> '[4,5,6]'::vector)::numeric, 3) <> 5.196 THEN
    RAISE EXCEPTION 'pgvector distance operator returned an unexpected value';
  END IF;
  IF vector_dims('[1,2,3]'::vector) <> 3 THEN
    RAISE EXCEPTION 'vector_dims returned an unexpected value';
  END IF;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    IF NOT has_table_privilege('forensic_runtime', 'forensic.content_embedding', 'SELECT')
       OR NOT has_table_privilege('forensic_runtime', 'forensic.content_embedding', 'INSERT')
       OR has_table_privilege('forensic_runtime', 'forensic.content_embedding', 'UPDATE')
       OR has_table_privilege('forensic_runtime', 'forensic.content_embedding', 'DELETE') THEN
      RAISE EXCEPTION 'runtime role privileges are wrong on content_embedding';
    END IF;
  END IF;
END
$$;

SELECT 'migration_015_verified' AS result;

ROLLBACK;
