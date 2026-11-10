-- Read-only compatibility gate for migration 034 (conversations and the case layer). Any exception means: do not apply.
BEGIN TRANSACTION READ ONLY;
DO $$
DECLARE
  t text;
  k text;
BEGIN
  IF to_regnamespace('forensic') IS NULL THEN
    RAISE EXCEPTION 'schema forensic is missing';
  END IF;
  IF current_setting('server_version_num')::int < 130000 THEN
    RAISE EXCEPTION 'PostgreSQL 13 or later is required (gen_random_uuid, hashtextextended)';
  END IF;
  IF pg_encoding_to_char((SELECT encoding FROM pg_database WHERE datname = current_database())) <> 'UTF8' THEN
    RAISE EXCEPTION 'the database encoding must be UTF8 (search_normalize writes Arabic-script code points)';
  END IF;
  IF to_regclass('forensic.evidence_items') IS NULL THEN
    RAISE EXCEPTION 'migration 007 (the evidence registry) must be applied first';
  END IF;
  IF to_regprocedure('forensic.reject_append_only_mutation()') IS NULL THEN
    RAISE EXCEPTION 'migration 008 (the append-only function) must be applied first';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm') THEN
    RAISE EXCEPTION 'extension pg_trgm is not installed (migrations 001 and 012 install it)';
  END IF;
  -- a table of this migration that already exists must be the one the migration describes, never another with the same name
  FOR t, k IN SELECT * FROM (VALUES
      ('case_member', 'subject_id'), ('conversation_thread', 'owner_subject'), ('conversation_turn', 'root_turn_id'), ('turn_feedback', 'verdict'),
      ('case_data_version', 'data_version_hash'), ('case_note', 'category'), ('finding', 'review_state'), ('hypothesis', 'statement'), ('task', 'assignee_subject'),
      ('tag', 'name'), ('tag_assignment', 'object_kind'), ('saved_view', 'params_json'), ('annotation', 'target_key'), ('authority_record', 'issuing_body'),
      ('case_audit_event', 'event_sha256')) AS x(t, k)
  LOOP
    IF to_regclass('forensic.' || t) IS NOT NULL
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'forensic' AND table_name = t AND column_name = k) THEN
      RAISE EXCEPTION 'forensic.% exists but is not the table migration 034 describes (no column %)', t, k;
    END IF;
  END LOOP;
  IF to_regprocedure('forensic.case_role(text,text,text)') IS NOT NULL OR to_regprocedure('forensic.search_normalize(text)') IS NOT NULL THEN
    RAISE NOTICE 'forensic.case_role or forensic.search_normalize already exist: the migration replaces them';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    RAISE NOTICE 'the role forensic_runtime does not exist: its grants will be skipped (apply runtime_role.grants.sql afterwards, then run the migration again to grant)';
  END IF;
  RAISE NOTICE 'migration 034 preflight passed';
END
$$;
ROLLBACK;
