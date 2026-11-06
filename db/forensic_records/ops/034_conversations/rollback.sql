-- DESTRUCTIVE rollback for 034_conversations_and_case_layer.sql: drops the 15 tables AND every conversation, turn, note, finding, tag, view, member, authority record and audit row in them.
-- Export what matters first (pg_dump of the 15 tables). Nothing else references these tables. Run only from a verified backup, with ON_ERROR_STOP enabled and an explicit:
--   SET app.caselayer_allow_destructive_rollback = 'true';
-- The shared pieces of earlier migrations (forensic.reject_append_only_mutation, pg_trgm) are left alone.
BEGIN;
DO $$
DECLARE
  t text;
  n bigint;
  total bigint := 0;
BEGIN
  IF current_setting('app.caselayer_allow_destructive_rollback', true) IS DISTINCT FROM 'true' THEN
    RAISE EXCEPTION 'the 034 rollback is destructive; set app.caselayer_allow_destructive_rollback=true explicitly' USING ERRCODE = '55000';
  END IF;
  FOREACH t IN ARRAY ARRAY['conversation_thread', 'conversation_turn', 'turn_feedback', 'case_data_version', 'case_note', 'finding', 'hypothesis', 'task', 'tag', 'tag_assignment',
                           'saved_view', 'annotation', 'case_member', 'authority_record', 'case_audit_event']
  LOOP
    IF to_regclass('forensic.' || t) IS NOT NULL THEN
      EXECUTE format('SELECT count(*) FROM forensic.%I', t) INTO n;
      total := total + n;
      RAISE NOTICE 'dropping forensic.% with % rows', t, n;
    END IF;
  END LOOP;
  RAISE NOTICE 'dropping % rows in all', total;
END
$$;
DROP TABLE IF EXISTS forensic.tag_assignment;
DROP TABLE IF EXISTS forensic.tag;
DROP TABLE IF EXISTS forensic.turn_feedback;
DROP TABLE IF EXISTS forensic.finding;
DROP TABLE IF EXISTS forensic.case_note;
DROP TABLE IF EXISTS forensic.hypothesis;
DROP TABLE IF EXISTS forensic.task;
DROP TABLE IF EXISTS forensic.saved_view;
DROP TABLE IF EXISTS forensic.annotation;
DROP TABLE IF EXISTS forensic.authority_record;
DROP TABLE IF EXISTS forensic.case_data_version;
DROP TABLE IF EXISTS forensic.conversation_turn;
DROP TABLE IF EXISTS forensic.conversation_thread;
DROP TABLE IF EXISTS forensic.case_audit_event;
DROP TABLE IF EXISTS forensic.case_member;
DROP FUNCTION IF EXISTS forensic.case_audit_chain_status(text, text);
DROP FUNCTION IF EXISTS forensic.prepare_case_audit_event();
DROP FUNCTION IF EXISTS forensic.guard_conversation_turn();
DROP FUNCTION IF EXISTS forensic.check_conversation_turn_scope();
DROP FUNCTION IF EXISTS forensic.case_role_in_all(text, text[], text, text[]);
DROP FUNCTION IF EXISTS forensic.sorted_distinct_text(text[]);
DROP FUNCTION IF EXISTS forensic.case_role(text, text, text);
DROP FUNCTION IF EXISTS forensic.search_normalize(text);
COMMIT;
