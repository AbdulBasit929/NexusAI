-- Read-only verification of migration 034. Any exception means the migration is incomplete or was altered.
BEGIN TRANSACTION READ ONLY;
DO $$
DECLARE
  t text;
  problem text;
  expected_policies text[] := ARRAY[
    'case_member_select', 'case_member_insert', 'case_member_update', 'case_member_delete',
    'conversation_thread_select', 'conversation_thread_insert', 'conversation_thread_update',
    'conversation_turn_select', 'conversation_turn_insert', 'conversation_turn_update',
    'turn_feedback_select', 'turn_feedback_insert', 'turn_feedback_update',
    'case_data_version_select', 'case_data_version_insert',
    'case_note_select', 'case_note_insert', 'case_note_update',
    'finding_select', 'finding_insert', 'finding_update',
    'hypothesis_select', 'hypothesis_insert', 'hypothesis_update',
    'task_select', 'task_insert', 'task_update',
    'tag_select', 'tag_insert', 'tag_update',
    'tag_assignment_select', 'tag_assignment_insert', 'tag_assignment_delete',
    'saved_view_select', 'saved_view_insert', 'saved_view_update',
    'annotation_select', 'annotation_insert', 'annotation_update',
    'authority_record_select', 'authority_record_insert', 'authority_record_update',
    'case_audit_event_select', 'case_audit_event_insert'];
  p text;
BEGIN
  -- tables, row-level security, no table left without a policy
  FOREACH t IN ARRAY ARRAY['case_member', 'conversation_thread', 'conversation_turn', 'turn_feedback', 'case_data_version', 'case_note', 'finding', 'hypothesis', 'task',
                           'tag', 'tag_assignment', 'saved_view', 'annotation', 'authority_record', 'case_audit_event']
  LOOP
    IF to_regclass('forensic.' || t) IS NULL THEN
      RAISE EXCEPTION 'forensic.% is missing (migration 034 is not applied)', t;
    END IF;
    IF NOT (SELECT relrowsecurity FROM pg_class WHERE oid = ('forensic.' || t)::regclass) THEN
      RAISE EXCEPTION 'row-level security is off on forensic.%', t;
    END IF;
    IF obj_description(('forensic.' || t)::regclass, 'pg_class') IS NULL THEN
      RAISE EXCEPTION 'forensic.% has no description', t;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_schema = 'forensic' AND table_name = t AND column_name = 'tenant_id' AND column_default IS NULL) THEN
      RAISE EXCEPTION 'forensic.%.tenant_id has no default', t;
    END IF;
  END LOOP;
  -- every constraint validated
  SELECT string_agg(c.conrelid::regclass::text || '.' || c.conname, ', ') INTO problem
  FROM pg_constraint c WHERE c.conrelid IN (SELECT ('forensic.' || x)::regclass FROM unnest(ARRAY['case_member', 'conversation_thread', 'conversation_turn', 'turn_feedback', 'case_data_version', 'case_note', 'finding',
      'hypothesis', 'task', 'tag', 'tag_assignment', 'saved_view', 'annotation', 'authority_record', 'case_audit_event']) AS x)
    AND NOT c.convalidated;
  IF problem IS NOT NULL THEN RAISE EXCEPTION 'unvalidated constraints: %', problem; END IF;
  -- policies: each expected one exists, is permissive, and none of them is missing its expression
  FOREACH p IN ARRAY expected_policies LOOP
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND policyname = p AND permissive = 'PERMISSIVE' AND (qual IS NOT NULL OR with_check IS NOT NULL)) THEN
      RAISE EXCEPTION 'the policy % is missing or altered', p;
    END IF;
  END LOOP;
  IF EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND policyname = ANY (expected_policies) AND roles <> '{public}') THEN
    RAISE EXCEPTION 'a policy of migration 034 is restricted to a role; they apply to every role';
  END IF;
  -- tenant isolation is in every policy that has an expression on a tenant-carrying table
  IF EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND policyname = ANY (expected_policies)
             AND coalesce(qual, '') NOT LIKE '%app.tenant_id%' AND coalesce(with_check, '') NOT LIKE '%app.tenant_id%') THEN
    RAISE EXCEPTION 'a policy of migration 034 does not test the tenant';
  END IF;
  -- functions
  FOREACH p IN ARRAY ARRAY['forensic.search_normalize(text)', 'forensic.case_role(text,text,text)', 'forensic.prepare_case_audit_event()', 'forensic.case_audit_chain_status(text,text)', 'forensic.guard_conversation_turn()']
  LOOP
    IF to_regprocedure(p) IS NULL THEN RAISE EXCEPTION 'the function % is missing', p; END IF;
  END LOOP;
  IF NOT (SELECT provolatile = 'i' FROM pg_proc WHERE oid = 'forensic.search_normalize(text)'::regprocedure) THEN
    RAISE EXCEPTION 'forensic.search_normalize must be IMMUTABLE (the search columns are generated from it)';
  END IF;
  IF NOT (SELECT prosecdef FROM pg_proc WHERE oid = 'forensic.case_role(text,text,text)'::regprocedure)
     OR NOT (SELECT prosecdef FROM pg_proc WHERE oid = 'forensic.prepare_case_audit_event()'::regprocedure) THEN
    RAISE EXCEPTION 'forensic.case_role and forensic.prepare_case_audit_event must be SECURITY DEFINER';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_proc WHERE oid = 'forensic.case_role(text,text,text)'::regprocedure AND proconfig::text LIKE '%search_path=forensic, pg_catalog, pg_temp%') THEN
    RAISE EXCEPTION 'forensic.case_role must have a fixed search path';
  END IF;
  -- triggers
  FOREACH p IN ARRAY ARRAY['case_audit_event_prepare', 'case_audit_event_append_only', 'conversation_turn_guard', 'conversation_turn_no_delete'] LOOP
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = p AND NOT tgisinternal) THEN RAISE EXCEPTION 'the trigger % is missing', p; END IF;
  END LOOP;
  -- the search: generated columns and both indexes
  FOREACH p IN ARRAY ARRAY['search_text', 'search_tsv'] LOOP
    IF NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid = 'forensic.conversation_turn'::regclass AND attname = p AND attgenerated = 's') THEN
      RAISE EXCEPTION 'conversation_turn.% is not a stored generated column', p;
    END IF;
  END LOOP;
  FOREACH p IN ARRAY ARRAY['conversation_turn_search_tsv_idx', 'conversation_turn_search_trgm_idx', 'conversation_turn_client_request_idx', 'finding_source_turn_idx', 'conversation_thread_rail_idx'] LOOP
    IF NOT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'forensic' AND indexname = p) THEN RAISE EXCEPTION 'the index % is missing', p; END IF;
  END LOOP;
  -- the folding is what the search relies on (a few fixed cases; behaviour.sql has the rest)
  IF forensic.search_normalize(U&'\0643\064A\0627 \0661\0662\0663 ABC') IS DISTINCT FROM U&'\06A9\06CC\0627 123 abc' THEN
    RAISE EXCEPTION 'forensic.search_normalize does not fold the Arabic yeh and kaf and the Arabic-Indic digits';
  END IF;
  -- grants of the application role
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    FOREACH t IN ARRAY ARRAY['case_member', 'conversation_thread', 'conversation_turn', 'turn_feedback', 'case_data_version', 'case_note', 'finding', 'hypothesis', 'task',
                             'tag', 'tag_assignment', 'saved_view', 'annotation', 'authority_record', 'case_audit_event']
    LOOP
      IF NOT has_table_privilege('forensic_runtime', 'forensic.' || t, 'SELECT') OR NOT has_table_privilege('forensic_runtime', 'forensic.' || t, 'INSERT') THEN
        RAISE EXCEPTION 'forensic_runtime lacks SELECT or INSERT on forensic.%', t;
      END IF;
      IF has_table_privilege('forensic_runtime', 'forensic.' || t, 'DELETE') AND t NOT IN ('case_member', 'tag_assignment') THEN
        RAISE EXCEPTION 'forensic_runtime may delete from forensic.%; only links (case_member, tag_assignment) may be deleted', t;
      END IF;
      IF has_table_privilege('forensic_runtime', 'forensic.' || t, 'UPDATE') THEN
        RAISE EXCEPTION 'forensic_runtime has a table-wide UPDATE on forensic.%; updates are column by column', t;
      END IF;
      IF has_table_privilege('forensic_runtime', 'forensic.' || t, 'TRUNCATE') THEN
        RAISE EXCEPTION 'forensic_runtime may truncate forensic.%', t;
      END IF;
    END LOOP;
    -- identity and content that must never change
    FOREACH p IN ARRAY ARRAY['conversation_turn.question_text', 'conversation_turn.thread_id', 'conversation_turn.created_by', 'conversation_thread.owner_subject', 'conversation_thread.collection_id',
                             'case_audit_event.details', 'case_audit_event.event_sha256', 'finding.source_turn_id', 'finding.created_by', 'case_member.subject_id', 'case_note.created_by']
    LOOP
      IF has_column_privilege('forensic_runtime', 'forensic.' || split_part(p, '.', 1), split_part(p, '.', 2), 'UPDATE') THEN
        RAISE EXCEPTION 'forensic_runtime may update %; it must not', p;
      END IF;
    END LOOP;
    IF NOT has_column_privilege('forensic_runtime', 'forensic.conversation_turn', 'state', 'UPDATE') OR NOT has_column_privilege('forensic_runtime', 'forensic.conversation_thread', 'hidden_at', 'UPDATE') THEN
      RAISE EXCEPTION 'forensic_runtime cannot complete a turn or hide a thread';
    END IF;
  END IF;
  -- the engine's read-only role must not see these tables
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    FOREACH t IN ARRAY ARRAY['case_member', 'conversation_thread', 'conversation_turn', 'turn_feedback', 'case_data_version', 'case_note', 'finding', 'hypothesis', 'task',
                             'tag', 'tag_assignment', 'saved_view', 'annotation', 'authority_record', 'case_audit_event']
    LOOP
      IF has_table_privilege('forensic_analyst_ro', 'forensic.' || t, 'SELECT') THEN
        RAISE EXCEPTION 'forensic_analyst_ro can read forensic.%: the engine must never see the case layer', t;
      END IF;
    END LOOP;
  END IF;
  IF has_table_privilege('public', 'forensic.conversation_turn', 'SELECT') THEN
    RAISE EXCEPTION 'PUBLIC can read forensic.conversation_turn';
  END IF;
  RAISE NOTICE 'migration 034 verified';
END
$$;
ROLLBACK;
