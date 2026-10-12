-- Read-only compatibility gate for migration 013 (reference tables). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  rel text;
  kind "char";
BEGIN
  IF to_regclass('forensic.evidence_items') IS NULL THEN
    RAISE EXCEPTION 'migration 007 (forensic.evidence_items) must be applied before migration 013';
  END IF;

  IF NOT has_schema_privilege(current_user, 'forensic', 'CREATE') THEN
    RAISE EXCEPTION 'role % cannot create objects in schema forensic', current_user;
  END IF;

  -- inet_ops for GiST on cidr columns is part of core PostgreSQL 9.4 and later.
  IF NOT EXISTS (
    SELECT 1 FROM pg_opclass oc JOIN pg_am am ON am.oid = oc.opcmethod
    WHERE am.amname = 'gist' AND oc.opcname = 'inet_ops'
  ) THEN
    RAISE EXCEPTION 'the gist inet_ops operator class is not available';
  END IF;

  FOREACH rel IN ARRAY ARRAY['cell_site', 'camera_site', 'device_tac', 'ip_range_reference']
  LOOP
    SELECT c.relkind INTO kind FROM pg_class c WHERE c.relnamespace = 'forensic'::regnamespace AND c.relname = rel;
    IF kind IS NOT NULL AND kind <> 'r' THEN
      RAISE EXCEPTION 'forensic.% exists but is not an ordinary table (relkind %)', rel, kind;
    END IF;
  END LOOP;

  IF to_regprocedure('forensic.make_cell_key(text,text,text)') IS NOT NULL
     AND (SELECT p.prorettype <> 'text'::regtype FROM pg_proc p WHERE p.oid = to_regprocedure('forensic.make_cell_key(text,text,text)')) THEN
    RAISE EXCEPTION 'forensic.make_cell_key exists with a different return type';
  END IF;

  RAISE NOTICE 'database % size: %', current_database(), pg_size_pretty(pg_database_size(current_database()));
END
$$;

ROLLBACK;
