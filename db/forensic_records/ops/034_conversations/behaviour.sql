-- Behaviour test of migration 034 inside ONE rolled-back transaction: it proves what the policies, constraints and triggers do, as the application role, and leaves nothing behind.
-- Run as the owner of the tables (a superuser can switch to forensic_runtime); needs the role forensic_runtime with its grants (runtime_role.grants.sql, then the migration).
-- Writes only synthetic rows of the tenant t034; touches no retained data. Prints behaviour_ok at the end; any exception means a rule is broken.
BEGIN;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    RAISE EXCEPTION 'the role forensic_runtime does not exist: apply runtime_role.grants.sql and migration 034 first';
  END IF;
  IF to_regclass('forensic.conversation_turn') IS NULL THEN
    RAISE EXCEPTION 'migration 034 is not applied';
  END IF;
END
$$;

-- 1. The folding of text for search (as the owner: a pure function)
DO $$
BEGIN
  IF forensic.search_normalize(U&'\0643\064A\0627') <> U&'\06A9\06CC\0627' THEN RAISE EXCEPTION 'step 1: the Arabic kaf and yeh are not folded to the Urdu letters'; END IF;
  IF forensic.search_normalize(U&'\0661\0662\0663\06F4\06F5\06F6') <> '123456' THEN RAISE EXCEPTION 'step 1: Arabic-Indic digits are not folded to Latin digits'; END IF;
  IF forensic.search_normalize(U&'\0627\064E\0644\0633\0640\0644\0627\0645') <> U&'\0627\0644\0633\0644\0627\0645' THEN RAISE EXCEPTION 'step 1: marks and the tatweel are not removed'; END IF;
  IF forensic.search_normalize('Kya HAI 0300-1234567') <> 'kya hai 0300-1234567' THEN RAISE EXCEPTION 'step 1: Latin text is not lower-cased as it is'; END IF;
  IF forensic.search_normalize(NULL) <> '' THEN RAISE EXCEPTION 'step 1: NULL is not the empty string'; END IF;
  IF forensic.search_normalize(forensic.search_normalize(U&'\0643\064A\0627 \0661')) <> forensic.search_normalize(U&'\0643\064A\0627 \0661') THEN RAISE EXCEPTION 'step 1: the folding is not idempotent'; END IF;
  RAISE NOTICE 'step 1 ok: search_normalize';
END
$$;

SET LOCAL ROLE forensic_runtime;
SELECT set_config('app.tenant_id', 't034', true), set_config('app.actor_role', 'user', true), set_config('app.subject_id', 'alice', true);

DO $$
DECLARE
  th uuid;
  th2 uuid;
  tn uuid := gen_random_uuid();
  tn2 uuid := gen_random_uuid();
  fid uuid;
  n bigint;
  evs bigint;
  good boolean;
  broken bigint;
  seen text;
  th3 uuid;
  th4 uuid;
  th5 uuid;
  ct1 uuid := gen_random_uuid();
  ct2 uuid := gen_random_uuid();
  grp uuid := gen_random_uuid();
BEGIN
  -- 2. The case c1 has no members: it is open. Alice's private thread is hers alone.
  INSERT INTO forensic.conversation_thread (tenant_id, collection_id, owner_subject, title) VALUES ('t034', 'c1', 'alice', 'alice private') RETURNING thread_id INTO th;
  PERFORM set_config('app.subject_id', 'bob', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th;
  IF n <> 0 THEN RAISE EXCEPTION 'step 2: bob can read alice''s private thread'; END IF;
  UPDATE forensic.conversation_thread SET title = 'bob was here' WHERE thread_id = th;
  GET DIAGNOSTICS n = ROW_COUNT;
  IF n <> 0 THEN RAISE EXCEPTION 'step 2: bob changed alice''s thread'; END IF;
  BEGIN
    INSERT INTO forensic.conversation_thread (tenant_id, collection_id, owner_subject) VALUES ('t034', 'c1', 'alice');
    RAISE EXCEPTION 'step 2: bob created a thread owned by alice';
  EXCEPTION WHEN insufficient_privilege THEN NULL;
  END;
  RAISE NOTICE 'step 2 ok: a private thread is private';

  -- 3. Sharing: the whole case, then selected subjects
  PERFORM set_config('app.subject_id', 'alice', true);
  UPDATE forensic.conversation_thread SET visibility = 'case' WHERE thread_id = th;
  PERFORM set_config('app.subject_id', 'bob', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th;
  IF n <> 1 THEN RAISE EXCEPTION 'step 3: bob cannot read a thread shared with the case'; END IF;
  PERFORM set_config('app.subject_id', 'alice', true);
  UPDATE forensic.conversation_thread SET visibility = 'selected', shared_with = ARRAY['carol'] WHERE thread_id = th;
  PERFORM set_config('app.subject_id', 'bob', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th;
  IF n <> 0 THEN RAISE EXCEPTION 'step 3: bob can read a thread shared with carol only'; END IF;
  PERFORM set_config('app.subject_id', 'carol', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th;
  IF n <> 1 THEN RAISE EXCEPTION 'step 3: carol cannot read a thread shared with her'; END IF;
  RAISE NOTICE 'step 3 ok: sharing';

  -- 4. Turns follow the thread: only its owner adds one; a finished turn cannot change
  PERFORM set_config('app.subject_id', 'carol', true);
  BEGIN
    INSERT INTO forensic.conversation_turn (turn_id, tenant_id, collection_id, thread_id, idx, root_turn_id, question_text, created_by) VALUES (gen_random_uuid(), 't034', 'c1', th, 1, tn, 'x', 'carol');
    RAISE EXCEPTION 'step 4: carol added a turn to alice''s thread';
  EXCEPTION WHEN insufficient_privilege OR foreign_key_violation THEN NULL;
  END;
  PERFORM set_config('app.subject_id', 'alice', true);
  INSERT INTO forensic.conversation_turn (turn_id, tenant_id, collection_id, thread_id, idx, root_turn_id, question_text, created_by)
    VALUES (tn, 't034', 'c1', th, 1, tn, 'how many calls did 923001234567 make?', 'alice');
  UPDATE forensic.conversation_turn SET state = 'completed', outcome = 'answered', headline = '14 calls', answer_text = '14 calls', completed_at = now() WHERE turn_id = tn;
  BEGIN
    UPDATE forensic.conversation_turn SET headline = '15 calls' WHERE turn_id = tn;
    RAISE EXCEPTION 'step 4: a finished turn was changed';
  EXCEPTION WHEN SQLSTATE '55000' THEN NULL;
  END;
  BEGIN
    UPDATE forensic.conversation_turn SET question_text = 'something else' WHERE turn_id = tn;
    RAISE EXCEPTION 'step 4: the question of a turn was changed';
  EXCEPTION WHEN insufficient_privilege OR SQLSTATE '55000' THEN NULL;
  END;
  BEGIN
    DELETE FROM forensic.conversation_turn WHERE turn_id = tn;
    RAISE EXCEPTION 'step 4: a turn was deleted';
  EXCEPTION WHEN insufficient_privilege OR SQLSTATE '55000' THEN NULL;
  END;
  BEGIN
    INSERT INTO forensic.conversation_turn (turn_id, tenant_id, collection_id, thread_id, idx, root_turn_id, question_text, created_by)
      VALUES (tn2, 't034', 'c2', th, 2, tn2, 'a turn of another case in this thread', 'alice');
    RAISE EXCEPTION 'step 4: a turn of case c2 was added to a thread of case c1';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  -- a turn that is still running can be completed once, then it is finished
  INSERT INTO forensic.conversation_turn (turn_id, tenant_id, collection_id, thread_id, idx, root_turn_id, parent_turn_id, question_text, created_by, client_request_id)
    VALUES (tn2, 't034', 'c1', th, 2, tn2, tn, 'and for february?', 'alice', 'req-1');
  BEGIN
    INSERT INTO forensic.conversation_turn (tenant_id, collection_id, thread_id, idx, root_turn_id, question_text, created_by, client_request_id)
      VALUES ('t034', 'c1', th, 3, tn2, 'the same request again', 'alice', 'req-1');
    RAISE EXCEPTION 'step 4: a second turn with the same client request id was accepted';
  EXCEPTION WHEN unique_violation THEN NULL;
  END;
  UPDATE forensic.conversation_turn SET state = 'interrupted', error_reason = 'the service restarted', completed_at = now() WHERE turn_id = tn2;
  RAISE NOTICE 'step 4 ok: turns';

  -- 5. Search: the question as typed with the Arabic yeh is found by the Urdu yeh, a number typed in Urdu digits finds the Latin digits
  UPDATE forensic.conversation_thread SET visibility = 'private', shared_with = '{}' WHERE thread_id = th;
  SELECT count(*) INTO n FROM forensic.conversation_turn WHERE search_text LIKE '%' || forensic.search_normalize(U&'\0669\0662\0663\0660\0660\0661\0662\0663\0664\0665\0666\0668') || '%' AND thread_id = th;
  IF n <> 0 THEN RAISE EXCEPTION 'step 5: a different number matched'; END IF;
  SELECT count(*) INTO n FROM forensic.conversation_turn WHERE search_text LIKE '%' || forensic.search_normalize(U&'\0669\0662\0663\0660\0660\0661\0662\0663\0664\0665\0666\0667') || '%' AND thread_id = th;
  IF n <> 1 THEN RAISE EXCEPTION 'step 5: a number typed in Arabic-Indic digits did not find the same number in Latin digits (found %)', n; END IF;
  SELECT count(*) INTO n FROM forensic.conversation_turn WHERE search_tsv @@ plainto_tsquery('simple', forensic.search_normalize('CALLS')) AND thread_id = th;
  IF n <> 1 THEN RAISE EXCEPTION 'step 5: the tsvector did not find the word calls (found %)', n; END IF;
  RAISE NOTICE 'step 5 ok: search';

  -- 6. Tenant isolation, and failing closed when no subject is set
  PERFORM set_config('app.tenant_id', 'other', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread;
  IF n <> 0 THEN RAISE EXCEPTION 'step 6: another tenant reads threads'; END IF;
  BEGIN
    INSERT INTO forensic.conversation_thread (tenant_id, collection_id, owner_subject) VALUES ('t034', 'c1', 'alice');
    RAISE EXCEPTION 'step 6: a thread of tenant t034 was written under tenant other';
  EXCEPTION WHEN insufficient_privilege THEN NULL;
  END;
  PERFORM set_config('app.tenant_id', 't034', true);
  PERFORM set_config('app.subject_id', '', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread;
  IF n <> 0 THEN RAISE EXCEPTION 'step 6: threads are readable with no subject set'; END IF;
  BEGIN
    INSERT INTO forensic.case_note (tenant_id, collection_id, body, created_by) VALUES ('t034', 'c1', 'a note with no subject', 'nobody');
    RAISE EXCEPTION 'step 6: a note was written with no subject set';
  EXCEPTION WHEN insufficient_privilege THEN NULL;
  END;
  RAISE NOTICE 'step 6 ok: tenant isolation and fail-closed';

  -- 7. Knowledge: one finding per answer, tags fold spelling, constraints refuse bad values
  PERFORM set_config('app.subject_id', 'alice', true);
  INSERT INTO forensic.finding (tenant_id, collection_id, title, source_turn_id, created_by) VALUES ('t034', 'c1', 'Calls of the number', tn, 'alice') RETURNING finding_id INTO fid;
  BEGIN
    INSERT INTO forensic.finding (tenant_id, collection_id, title, source_turn_id, created_by) VALUES ('t034', 'c1', 'The same answer again', tn, 'alice');
    RAISE EXCEPTION 'step 7: the same answer was pinned twice';
  EXCEPTION WHEN unique_violation THEN NULL;
  END;
  UPDATE forensic.finding SET deleted_at = now(), deleted_by = 'alice', delete_reason = 'test' WHERE finding_id = fid;
  INSERT INTO forensic.finding (tenant_id, collection_id, title, source_turn_id, created_by) VALUES ('t034', 'c1', 'Pinned again after the first was removed', tn, 'alice');
  BEGIN
    UPDATE forensic.finding SET review_state = 'confirmed' WHERE source_turn_id = tn AND deleted_at IS NULL;
    RAISE EXCEPTION 'step 7: a finding was confirmed with no reviewer';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  INSERT INTO forensic.tag (tenant_id, collection_id, name, created_by) VALUES ('t034', 'c1', 'Urgent', 'alice');
  BEGIN
    INSERT INTO forensic.tag (tenant_id, collection_id, name, created_by) VALUES ('t034', 'c1', ' URGENT ', 'alice');
    RAISE EXCEPTION 'step 7: a second spelling of a tag was accepted';
  EXCEPTION WHEN unique_violation THEN NULL;
  END;
  INSERT INTO forensic.tag (tenant_id, collection_id, name, created_by) VALUES ('t034', 'c1', U&'\0643\064A\0627', 'alice');
  BEGIN
    INSERT INTO forensic.tag (tenant_id, collection_id, name, created_by) VALUES ('t034', 'c1', U&'\06A9\06CC\0627', 'alice');
    RAISE EXCEPTION 'step 7: the Arabic and the Urdu spelling of a tag are two tags';
  EXCEPTION WHEN unique_violation THEN NULL;
  END;
  BEGIN
    UPDATE forensic.conversation_thread SET visibility = 'everyone' WHERE thread_id = th;
    RAISE EXCEPTION 'step 7: a visibility outside private, selected, case was accepted';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  BEGIN
    INSERT INTO forensic.turn_feedback (turn_id, tenant_id, collection_id, subject_id, verdict) VALUES (tn, 't034', 'c1', 'alice', 'maybe');
    RAISE EXCEPTION 'step 7: a verdict outside correct, wrong, incomplete, unclear was accepted';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  INSERT INTO forensic.turn_feedback (turn_id, tenant_id, collection_id, subject_id, verdict, note) VALUES (tn, 't034', 'c1', 'alice', 'wrong', 'the figure is for the other number');
  UPDATE forensic.turn_feedback SET verdict = 'incomplete' WHERE turn_id = tn AND subject_id = 'alice';
  INSERT INTO forensic.saved_view (tenant_id, collection_id, owner_subject, name, kind, params_json) VALUES ('t034', 'c1', 'alice', 'Links in February', 'connections', '{"from":"2026-02-01"}');
  INSERT INTO forensic.annotation (tenant_id, collection_id, target_kind, target_key, body, created_by) VALUES ('t034', 'c1', 'timeline_day', '2026-02-03', 'the day of the meeting', 'alice');
  INSERT INTO forensic.case_note (tenant_id, collection_id, category, body, created_by) VALUES ('t034', 'c1', 'lead', 'check the second handset', 'alice');
  RAISE NOTICE 'step 7 ok: knowledge';

  -- 8. A first member makes the case controlled; roles decide
  INSERT INTO forensic.case_member (tenant_id, collection_id, subject_id, role, added_by) VALUES ('t034', 'c1', 'alice', 'owner', 'alice');
  UPDATE forensic.conversation_thread SET visibility = 'case' WHERE thread_id = th;
  PERFORM set_config('app.subject_id', 'bob', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th;
  IF n <> 0 THEN RAISE EXCEPTION 'step 8: a non-member reads a thread of a controlled case'; END IF;
  SELECT count(*) INTO n FROM forensic.case_note;
  SELECT count(*) INTO evs FROM forensic.case_member;
  IF n <> 0 OR evs <> 0 THEN RAISE EXCEPTION 'step 8: a non-member reads notes or members of a controlled case'; END IF;
  PERFORM set_config('app.subject_id', 'alice', true);
  INSERT INTO forensic.case_member (tenant_id, collection_id, subject_id, role, added_by) VALUES ('t034', 'c1', 'bob', 'viewer', 'alice');
  INSERT INTO forensic.case_member (tenant_id, collection_id, subject_id, role, added_by) VALUES ('t034', 'c1', 'dave', 'auditor', 'alice');
  INSERT INTO forensic.case_member (tenant_id, collection_id, subject_id, role, added_by) VALUES ('t034', 'c1', 'erin', 'analyst', 'alice');
  PERFORM set_config('app.subject_id', 'bob', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th;
  IF n <> 1 THEN RAISE EXCEPTION 'step 8: a viewer cannot read a thread shared with the case'; END IF;
  BEGIN
    INSERT INTO forensic.case_note (tenant_id, collection_id, body, created_by) VALUES ('t034', 'c1', 'a viewer cannot write', 'bob');
    RAISE EXCEPTION 'step 8: a viewer wrote a note';
  EXCEPTION WHEN insufficient_privilege THEN NULL;
  END;
  BEGIN
    INSERT INTO forensic.case_member (tenant_id, collection_id, subject_id, role, added_by) VALUES ('t034', 'c1', 'mallory', 'owner', 'bob');
    RAISE EXCEPTION 'step 8: a viewer added a member';
  EXCEPTION WHEN insufficient_privilege THEN NULL;
  END;
  PERFORM set_config('app.subject_id', 'erin', true);
  INSERT INTO forensic.case_note (tenant_id, collection_id, body, created_by) VALUES ('t034', 'c1', 'an analyst writes', 'erin');
  INSERT INTO forensic.conversation_thread (tenant_id, collection_id, owner_subject, title) VALUES ('t034', 'c1', 'erin', 'erin private') RETURNING thread_id INTO th2;
  PERFORM set_config('app.subject_id', 'alice', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th2;
  IF n <> 0 THEN RAISE EXCEPTION 'step 8: the owner of the case reads a private thread of an analyst'; END IF;
  PERFORM set_config('app.subject_id', 'dave', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th2;
  IF n <> 1 THEN RAISE EXCEPTION 'step 8: an auditor cannot read a private thread'; END IF;
  BEGIN
    INSERT INTO forensic.case_note (tenant_id, collection_id, body, created_by) VALUES ('t034', 'c1', 'an auditor does not write', 'dave');
    RAISE EXCEPTION 'step 8: an auditor wrote a note';
  EXCEPTION WHEN insufficient_privilege THEN NULL;
  END;
  PERFORM set_config('app.subject_id', 'root', true);
  PERFORM set_config('app.actor_role', 'admin', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th2;
  IF n <> 1 THEN RAISE EXCEPTION 'step 8: a global admin cannot read a private thread'; END IF;
  UPDATE forensic.conversation_thread SET hidden_at = now(), hidden_by = 'root', hide_reason = 'test' WHERE thread_id = th2;
  GET DIAGNOSTICS n = ROW_COUNT;
  IF n <> 1 THEN RAISE EXCEPTION 'step 8: a global admin cannot hide a thread'; END IF;
  PERFORM set_config('app.actor_role', 'user', true);
  PERFORM set_config('app.subject_id', 'erin', true);
  BEGIN
    INSERT INTO forensic.conversation_turn (tenant_id, collection_id, thread_id, idx, root_turn_id, question_text, created_by)
      VALUES ('t034', 'c1', th2, 1, gen_random_uuid(), 'a turn in a hidden thread', 'erin');
    RAISE EXCEPTION 'step 8: a turn was added to a hidden thread';
  EXCEPTION WHEN insufficient_privilege OR foreign_key_violation THEN NULL;
  END;
  RAISE NOTICE 'step 8 ok: members and roles';

  -- 9. The audit chain: three rows chain; the trigger sets the hashes; a change is refused, and a change made behind the trigger is found
  PERFORM set_config('app.subject_id', 'alice', true);
  INSERT INTO forensic.case_audit_event (tenant_id, collection_id, actor_subject, actor_role, action, object_kind, object_id, details, event_sha256, seq)
    VALUES ('t034', 'c1', 'alice', 'owner', 'thread.created', 'thread', th::text, '{"a":1}', repeat('f', 64), 999);
  INSERT INTO forensic.case_audit_event (tenant_id, collection_id, actor_subject, actor_role, action, object_kind, object_id, details) VALUES ('t034', 'c1', 'alice', 'owner', 'turn.started', 'turn', tn::text, '{"b":2}');
  INSERT INTO forensic.case_audit_event (tenant_id, collection_id, actor_subject, actor_role, action, object_kind, object_id, details) VALUES ('t034', 'c1', 'alice', 'owner', 'turn.completed', 'turn', tn::text, '{"c":[1,2,3]}');
  SELECT string_agg(seq::text, ',' ORDER BY seq) INTO seen FROM forensic.case_audit_event WHERE tenant_id = 't034' AND collection_id = 'c1';
  IF seen <> '1,2,3' THEN RAISE EXCEPTION 'step 9: the sequence numbers are % (the application cannot choose them)', seen; END IF;
  IF EXISTS (SELECT 1 FROM forensic.case_audit_event WHERE event_sha256 = repeat('f', 64)) THEN RAISE EXCEPTION 'step 9: the application set a hash'; END IF;
  SELECT s.events, s.ok, s.first_broken_seq INTO evs, good, broken FROM forensic.case_audit_chain_status('t034', 'c1') s;
  IF evs <> 3 OR NOT good OR broken IS NOT NULL THEN RAISE EXCEPTION 'step 9: a clean chain of 3 is reported as events=%, ok=%, broken=%', evs, good, broken; END IF;
  BEGIN
    UPDATE forensic.case_audit_event SET action = 'thread.deleted' WHERE tenant_id = 't034' AND seq = 2;
    RAISE EXCEPTION 'step 9: an audit row was changed';
  EXCEPTION WHEN insufficient_privilege OR SQLSTATE '55000' THEN NULL;
  END;
  RESET ROLE;
  BEGIN
    UPDATE forensic.case_audit_event SET action = 'thread.deleted' WHERE tenant_id = 't034' AND seq = 2;
    RAISE EXCEPTION 'step 9: an audit row was changed by the owner of the table';
  EXCEPTION WHEN SQLSTATE '55000' THEN NULL;
  END;
  ALTER TABLE forensic.case_audit_event DISABLE TRIGGER case_audit_event_append_only;
  UPDATE forensic.case_audit_event SET details = '{"tampered":true}' WHERE tenant_id = 't034' AND seq = 2;
  ALTER TABLE forensic.case_audit_event ENABLE TRIGGER case_audit_event_append_only;
  SELECT s.events, s.ok, s.first_broken_seq INTO evs, good, broken FROM forensic.case_audit_chain_status('t034', 'c1') s;
  IF evs <> 3 OR good OR broken <> 2 THEN RAISE EXCEPTION 'step 9: a changed row is reported as events=%, ok=%, broken=% (expected 3, false, 2)', evs, good, broken; END IF;
  RAISE NOTICE 'step 9 ok: audit chain';

  -- 10. The engine's role sees none of it
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') AND has_table_privilege('forensic_analyst_ro', 'forensic.conversation_turn', 'SELECT') THEN
    RAISE EXCEPTION 'step 10: the read-only analyst role can read conversation_turn';
  END IF;
  RAISE NOTICE 'step 10 ok: analyst role';

  -- 11. A conversation across cases is seen only by someone who may open EVERY one of its cases. c1 is controlled (alice owner, bob viewer, dave auditor, erin analyst), c2 is open,
  -- c4 becomes controlled here (alice owner, erin viewer, dave auditor).
  SET LOCAL ROLE forensic_runtime;
  PERFORM set_config('app.tenant_id', 't034', true);
  PERFORM set_config('app.actor_role', 'user', true);
  PERFORM set_config('app.subject_id', 'alice', true);
  INSERT INTO forensic.case_member (tenant_id, collection_id, subject_id, role, added_by) VALUES ('t034', 'c4', 'alice', 'owner', 'alice');
  INSERT INTO forensic.case_member (tenant_id, collection_id, subject_id, role, added_by) VALUES ('t034', 'c4', 'erin', 'viewer', 'alice');
  INSERT INTO forensic.case_member (tenant_id, collection_id, subject_id, role, added_by) VALUES ('t034', 'c4', 'dave', 'auditor', 'alice');
  -- the shape of the list of cases: two to ten, each once, sorted, none empty, and never together with a single case
  BEGIN
    INSERT INTO forensic.conversation_thread (tenant_id, collection_id, scope_collection_ids, owner_subject) VALUES ('t034', NULL, ARRAY['c1'], 'alice');
    RAISE EXCEPTION 'step 11: a conversation across cases with one case was accepted';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  BEGIN
    INSERT INTO forensic.conversation_thread (tenant_id, collection_id, scope_collection_ids, owner_subject) VALUES ('t034', NULL, ARRAY['c2', 'c1'], 'alice');
    RAISE EXCEPTION 'step 11: a list of cases that is not sorted was accepted';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  BEGIN
    INSERT INTO forensic.conversation_thread (tenant_id, collection_id, scope_collection_ids, owner_subject) VALUES ('t034', NULL, ARRAY['c1', 'c1'], 'alice');
    RAISE EXCEPTION 'step 11: a list that names a case twice was accepted';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  BEGIN
    INSERT INTO forensic.conversation_thread (tenant_id, collection_id, scope_collection_ids, owner_subject) VALUES ('t034', 'c1', ARRAY['c1', 'c2'], 'alice');
    RAISE EXCEPTION 'step 11: a conversation with a case AND a list of cases was accepted';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  BEGIN
    INSERT INTO forensic.conversation_thread (tenant_id, collection_id, scope_collection_ids, owner_subject) VALUES ('t034', NULL, ARRAY['c1', ''], 'alice');
    RAISE EXCEPTION 'step 11: a list with an empty case name was accepted';
  EXCEPTION WHEN insufficient_privilege OR check_violation THEN NULL;
  END;
  BEGIN
    INSERT INTO forensic.conversation_thread (tenant_id, collection_id, scope_collection_ids, owner_subject)
      VALUES ('t034', NULL, (SELECT array_agg('x' || lpad(i::text, 2, '0') ORDER BY i) FROM generate_series(1, 11) AS i), 'alice');
    RAISE EXCEPTION 'step 11: a list of eleven cases was accepted';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  -- starting one: the writer's role must allow it in every case (erin may write in c1 and in the open c2, but only read in c4)
  INSERT INTO forensic.conversation_thread (tenant_id, collection_id, scope_collection_ids, owner_subject, title) VALUES ('t034', NULL, ARRAY['c1', 'c2'], 'alice', 'across c1 and c2') RETURNING thread_id INTO th3;
  PERFORM set_config('app.subject_id', 'erin', true);
  BEGIN
    INSERT INTO forensic.conversation_thread (tenant_id, collection_id, scope_collection_ids, owner_subject) VALUES ('t034', NULL, ARRAY['c1', 'c4'], 'erin');
    RAISE EXCEPTION 'step 11: a viewer of c4 started a conversation across c1 and c4';
  EXCEPTION WHEN insufficient_privilege THEN NULL;
  END;
  INSERT INTO forensic.conversation_thread (tenant_id, collection_id, scope_collection_ids, owner_subject) VALUES ('t034', NULL, ARRAY['c1', 'c2'], 'erin') RETURNING thread_id INTO th4;
  -- turns: each in one case of the conversation, the questions asked together share a group; a case outside the list, or a group in a conversation about one case, is refused
  PERFORM set_config('app.subject_id', 'alice', true);
  INSERT INTO forensic.conversation_turn (turn_id, tenant_id, collection_id, thread_id, idx, group_id, root_turn_id, question_text, created_by)
    VALUES (ct1, 't034', 'c1', th3, 1, grp, ct1, 'who called whom?', 'alice');
  INSERT INTO forensic.conversation_turn (turn_id, tenant_id, collection_id, thread_id, idx, group_id, root_turn_id, question_text, created_by)
    VALUES (ct2, 't034', 'c2', th3, 2, grp, ct2, 'who called whom?', 'alice');
  BEGIN
    INSERT INTO forensic.conversation_turn (tenant_id, collection_id, thread_id, idx, root_turn_id, question_text, created_by)
      VALUES ('t034', 'c4', th3, 3, gen_random_uuid(), 'a turn of a case the conversation does not span', 'alice');
    RAISE EXCEPTION 'step 11: a turn of c4 was added to a conversation across c1 and c2';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  BEGIN
    INSERT INTO forensic.conversation_turn (tenant_id, collection_id, thread_id, idx, group_id, root_turn_id, question_text, created_by)
      VALUES ('t034', 'c1', th, 9, grp, gen_random_uuid(), 'a group in a conversation about one case', 'alice');
    RAISE EXCEPTION 'step 11: a grouped turn was added to a conversation about one case';
  EXCEPTION WHEN check_violation OR insufficient_privilege THEN NULL;
  END;
  BEGIN
    UPDATE forensic.conversation_turn SET group_id = gen_random_uuid() WHERE turn_id = ct1;
    RAISE EXCEPTION 'step 11: the group of a turn was changed';
  EXCEPTION WHEN insufficient_privilege OR SQLSTATE '55000' THEN NULL;
  END;
  -- who sees it: private to its owner; with visibility case, to everyone who may open both c1 and c2, never to someone who may not open c1
  PERFORM set_config('app.subject_id', 'bob', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th3;
  SELECT count(*) INTO evs FROM forensic.conversation_turn WHERE thread_id = th3;
  IF n <> 0 OR evs <> 0 THEN RAISE EXCEPTION 'step 11: bob reads a private conversation across cases or its turns (threads %, turns %)', n, evs; END IF;
  PERFORM set_config('app.subject_id', 'alice', true);
  UPDATE forensic.conversation_thread SET visibility = 'case' WHERE thread_id = th3;
  PERFORM set_config('app.subject_id', 'bob', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th3;
  SELECT count(*) INTO evs FROM forensic.conversation_turn WHERE thread_id = th3;
  IF n <> 1 OR evs <> 2 THEN RAISE EXCEPTION 'step 11: bob (viewer of c1, open c2) cannot read a conversation shared with the cases (threads %, turns %)', n, evs; END IF;
  PERFORM set_config('app.subject_id', 'zed', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th3;
  SELECT count(*) INTO evs FROM forensic.conversation_turn WHERE thread_id = th3;
  IF n <> 0 OR evs <> 0 THEN RAISE EXCEPTION 'step 11: someone who may not open c1 reads a conversation across c1 and c2 (threads %, turns %)', n, evs; END IF;
  -- an auditor of c1 only (c2 is open, where dave is an analyst) does not read the private threads of a conversation across c1 and c2; an auditor of every case does
  PERFORM set_config('app.subject_id', 'alice', true);
  UPDATE forensic.conversation_thread SET visibility = 'private' WHERE thread_id = th3;
  INSERT INTO forensic.conversation_thread (tenant_id, collection_id, scope_collection_ids, owner_subject, title) VALUES ('t034', NULL, ARRAY['c1', 'c4'], 'alice', 'across c1 and c4') RETURNING thread_id INTO th5;
  PERFORM set_config('app.subject_id', 'dave', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th3;
  IF n <> 0 THEN RAISE EXCEPTION 'step 11: an auditor of c1 only reads a private conversation across c1 and the open c2'; END IF;
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th5;
  IF n <> 1 THEN RAISE EXCEPTION 'step 11: an auditor of c1 and c4 cannot read a private conversation across c1 and c4'; END IF;
  PERFORM set_config('app.subject_id', 'erin', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id = th5;
  IF n <> 0 THEN RAISE EXCEPTION 'step 11: erin reads alice''s private conversation across c1 and c4'; END IF;
  PERFORM set_config('app.subject_id', 'root', true);
  PERFORM set_config('app.actor_role', 'admin', true);
  SELECT count(*) INTO n FROM forensic.conversation_thread WHERE thread_id IN (th3, th5);
  IF n <> 2 THEN RAISE EXCEPTION 'step 11: a global admin cannot read the private conversations across cases (found %)', n; END IF;
  PERFORM set_config('app.actor_role', 'user', true);
  -- the function the policies use
  PERFORM set_config('app.subject_id', 'alice', true);
  IF forensic.case_role_in_all('t034', ARRAY[]::text[], 'alice', ARRAY['owner', 'open']) THEN RAISE EXCEPTION 'step 11: no case at all counted as all cases'; END IF;
  IF forensic.case_role_in_all('t034', ARRAY['c1', NULL], 'alice', ARRAY['owner', 'open']) THEN RAISE EXCEPTION 'step 11: a NULL case name counted'; END IF;
  IF NOT forensic.case_role_in_all('t034', ARRAY['c1', 'c2'], 'alice', ARRAY['owner', 'open']) THEN RAISE EXCEPTION 'step 11: owner of c1 and open c2 was refused'; END IF;
  IF forensic.case_role_in_all('t034', ARRAY['c1', 'c2'], 'alice', ARRAY['owner']) THEN RAISE EXCEPTION 'step 11: the open case counted as owner'; END IF;
  IF forensic.case_role_in_all('other', ARRAY['c1', 'c2'], 'alice', ARRAY['owner', 'open']) THEN RAISE EXCEPTION 'step 11: another tenant counted'; END IF;
  RESET ROLE;
  IF forensic.sorted_distinct_text(ARRAY['b', 'a', 'b', 'C']) <> ARRAY['C', 'a', 'b'] THEN RAISE EXCEPTION 'step 11: sorted_distinct_text did not sort in byte order and drop the repeat'; END IF;
  RAISE NOTICE 'step 11 ok: conversations across cases';
  RAISE NOTICE 'behaviour_ok';
END
$$;

ROLLBACK;
