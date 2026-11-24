-- Read-only verification of migration 038. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  problem text;
  functions text[] := ARRAY['image_index_state(text,text)', 'image_search_hash(text,bit,integer,text,uuid,integer)',
                            'image_search_vector(text,text,vector,double precision,text,uuid,integer)', 'image_exact_matches(text,text,text,uuid,integer)'];
  f text;
  t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['image_hash', 'image_search_log']
  LOOP
    IF to_regclass('forensic.' || t) IS NULL THEN
      RAISE EXCEPTION 'forensic.% is missing', t;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = ('forensic.' || t)::regclass AND c.relrowsecurity) THEN
      RAISE EXCEPTION 'row-level security is not enabled on forensic.%', t;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_policies p WHERE p.schemaname = 'forensic' AND p.tablename = t AND p.policyname = 'tenant_isolation_' || t
                   AND p.cmd = 'ALL' AND p.qual IS NOT NULL AND p.with_check IS NOT NULL) THEN
      RAISE EXCEPTION 'tenant policy missing on forensic.%', t;
    END IF;
  END LOOP;

  SELECT string_agg(c.conname, ', ') INTO problem
  FROM (VALUES ('image_hash_evidence_fk'), ('image_hash_artifact_fk'), ('image_hash_size_chk'), ('image_hash_sha_chk')) AS c(conname)
  WHERE NOT EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conrelid = 'forensic.image_hash'::regclass AND k.conname = c.conname AND k.convalidated);
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'missing or unvalidated constraints on image_hash: %', problem;
  END IF;
  SELECT string_agg(c.conname, ', ') INTO problem
  FROM (VALUES ('image_search_log_kind_chk'), ('image_search_log_params_chk'), ('image_search_log_sha_chk'), ('image_search_log_query_chk')) AS c(conname)
  WHERE NOT EXISTS (SELECT 1 FROM pg_constraint k WHERE k.conrelid = 'forensic.image_search_log'::regclass AND k.conname = c.conname AND k.convalidated);
  IF problem IS NOT NULL THEN
    RAISE EXCEPTION 'missing or unvalidated constraints on image_search_log: %', problem;
  END IF;

  FOREACH f IN ARRAY functions
  LOOP
    IF to_regprocedure('forensic.' || f) IS NULL THEN
      RAISE EXCEPTION 'function forensic.% is missing', f;
    END IF;
    -- SECURITY INVOKER (prosecdef false) is what keeps row-level security in force inside the function.
    IF EXISTS (SELECT 1 FROM pg_proc p WHERE p.oid = to_regprocedure('forensic.' || f) AND p.prosecdef) THEN
      RAISE EXCEPTION 'function forensic.% is SECURITY DEFINER; it must be SECURITY INVOKER', f;
    END IF;
  END LOOP;

  -- The pure parts: the hash distance and the cosine distance behave as the search assumes.
  IF bit_count(B'1010' # B'0110') <> 2 THEN
    RAISE EXCEPTION 'bit_count/xor returned an unexpected value';
  END IF;
  IF round(('[1,0,0]'::vector <=> '[0,1,0]'::vector)::numeric, 6) <> 1.000000 THEN
    RAISE EXCEPTION 'pgvector cosine distance returned an unexpected value';
  END IF;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    IF NOT has_table_privilege('forensic_runtime', 'forensic.image_hash', 'SELECT')
       OR NOT has_table_privilege('forensic_runtime', 'forensic.image_hash', 'INSERT')
       OR has_table_privilege('forensic_runtime', 'forensic.image_hash', 'UPDATE')
       OR has_table_privilege('forensic_runtime', 'forensic.image_hash', 'DELETE')
       OR NOT has_table_privilege('forensic_runtime', 'forensic.image_search_log', 'SELECT')
       OR NOT has_table_privilege('forensic_runtime', 'forensic.image_search_log', 'INSERT')
       OR has_table_privilege('forensic_runtime', 'forensic.image_search_log', 'UPDATE')
       OR has_table_privilege('forensic_runtime', 'forensic.image_search_log', 'DELETE') THEN
      RAISE EXCEPTION 'runtime role privileges are wrong on image_hash or image_search_log (SELECT and INSERT only)';
    END IF;
  END IF;
END
$$;

SELECT 'migration_038_verified' AS result;

ROLLBACK;
