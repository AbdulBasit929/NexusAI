-- Synthetic acceptance checks for migration 039. DISPOSABLE DATABASE ONLY; one rolled-back transaction.
-- Proves: the flags come out as stated (identical file by SHA-256 with no hash row needed; the same image within the stated distance; nothing beyond it), a set filter and a case filter narrow them,
-- the coverage counts are right, a retired set stops flagging and frees its name, a set of another tenant is invisible (functions and, for the runtime role, the tables), the runtime role can create
-- sets, add hashes and write events but cannot change or delete a hash or an event or a set's name or kind, no table can hold picture bytes, and the constraints refuse what they should.
BEGIN;

CREATE FUNCTION pg_temp.expect_error(label text, stmt text, expected text) RETURNS void
LANGUAGE plpgsql AS $$
BEGIN
  BEGIN
    EXECUTE stmt;
  EXCEPTION WHEN OTHERS THEN
    IF SQLSTATE = expected THEN
      RETURN;
    END IF;
    RAISE EXCEPTION 'smoke "%": expected SQLSTATE %, got % (%)', label, expected, SQLSTATE, SQLERRM;
  END;
  RAISE EXCEPTION 'smoke "%": the statement unexpectedly succeeded', label;
END
$$;

-- One image evidence item with its processing run and one completed artifact; returns the artifact id. The registration trigger of migration 008 creates the evidence version.
CREATE FUNCTION pg_temp.mk_image(p_tenant text, p_collection text, p_case text, p_evidence uuid, p_sha text, p_status text, OUT version_id uuid, OUT artifact_id uuid)
LANGUAGE plpgsql AS $$
DECLARE
  run uuid := gen_random_uuid();
BEGIN
  INSERT INTO forensic.evidence_items (
    evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension,
    size_bytes, sha256, modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata
  ) VALUES (
    p_evidence, p_tenant, p_collection, p_case, 'known-smoke-user', 'photo-' || substr(p_evidence::text, 1, 8) || '.jpg', 'photo.jpg', 'image/jpeg', 'jpg',
    1000, p_sha, 'image', 'image', 1, 'unified_media_worker', p_status, 'immutable://known-smoke/sha256/' || p_sha,
    jsonb_build_object('request_id', 'known-smoke-request'));
  SELECT e.current_version_id INTO version_id FROM forensic.evidence_items e WHERE e.evidence_id = p_evidence;
  INSERT INTO forensic.processing_runs (run_id, tenant_id, collection_id, evidence_id, version_id, run_kind, pipeline_id, idempotency_key, status, input_sha256)
  VALUES (run, p_tenant, p_collection, p_evidence, version_id, 'ingest', 'known-smoke', 'known-smoke:' || run::text, 'succeeded', p_sha);
  artifact_id := gen_random_uuid();
  INSERT INTO forensic.derived_artifacts (artifact_id, tenant_id, collection_id, evidence_id, version_id, run_id, artifact_type, media_type, processing_status)
  VALUES (artifact_id, p_tenant, p_collection, p_evidence, version_id, run, 'forensics.image-hash-observation/v1', 'application/json', 'completed');
END
$$;

CREATE FUNCTION pg_temp.put_hash(p_tenant text, p_collection text, p_evidence uuid, p_version uuid, p_artifact uuid, p_bits text) RETURNS void
LANGUAGE sql AS $$
  INSERT INTO forensic.image_hash (tenant_id, collection_id, evidence_id, version_id, derived_artifact_id, chunk_ref, algorithm, pdq256, image_width, image_height)
  VALUES (p_tenant, p_collection, p_evidence, p_version, p_artifact, 'v:' || p_version::text || ':full', 'pdq-style-256-nexusai-v1', p_bits::bit(256), 640, 480)
$$;

SELECT set_config('app.tenant_id', 'known-smoke', true);

-- Scenario (tenant known-smoke, collection c1; case k1 unless said):
--   a1 hash 0^256            a2 hash with 3 bits set (distance 3)      a3 hash with 60 bits set (distance 60: beyond the 56 of the copy search)
--   a4 NO hash row, file sha 4...4                                      a5 hash with 120 bits set
--   a6 case k2, hash 0^256                                              f  tenant known-other, collection c1, hash 0^256
-- Sets: S1 watch (of_interest): pdq 0^256 and the sha 4...4;  S2 harmless (known_harmless): pdq of a5 (120 ones);  S3 (tenant known-other, of_interest): pdq 0^256
DO $$
DECLARE
  z256 text := repeat('0', 256);
  near text := repeat('0', 253) || '111';
  sixty text := repeat('1', 60) || repeat('0', 196);
  ones120 text := repeat('1', 120) || repeat('0', 136);
  a1 uuid := '39000000-0000-4000-8000-0000000000a1'; a2 uuid := '39000000-0000-4000-8000-0000000000a2'; a3 uuid := '39000000-0000-4000-8000-0000000000a3';
  a4 uuid := '39000000-0000-4000-8000-0000000000a4'; a5 uuid := '39000000-0000-4000-8000-0000000000a5'; a6 uuid := '39000000-0000-4000-8000-0000000000a6';
  f uuid := '39000000-0000-4000-8000-0000000000f0';
  m record;
BEGIN
  SELECT * INTO m FROM pg_temp.mk_image('known-smoke', 'c1', 'k1', a1, repeat('1', 64), 'completed');
  PERFORM pg_temp.put_hash('known-smoke', 'c1', a1, m.version_id, m.artifact_id, z256);
  SELECT * INTO m FROM pg_temp.mk_image('known-smoke', 'c1', 'k1', a2, repeat('2', 64), 'completed');
  PERFORM pg_temp.put_hash('known-smoke', 'c1', a2, m.version_id, m.artifact_id, near);
  SELECT * INTO m FROM pg_temp.mk_image('known-smoke', 'c1', 'k1', a3, repeat('3', 64), 'completed');
  PERFORM pg_temp.put_hash('known-smoke', 'c1', a3, m.version_id, m.artifact_id, sixty);
  SELECT * INTO m FROM pg_temp.mk_image('known-smoke', 'c1', 'k1', a4, repeat('4', 64), 'completed');
  SELECT * INTO m FROM pg_temp.mk_image('known-smoke', 'c1', 'k1', a5, repeat('5', 64), 'completed');
  PERFORM pg_temp.put_hash('known-smoke', 'c1', a5, m.version_id, m.artifact_id, ones120);
  SELECT * INTO m FROM pg_temp.mk_image('known-smoke', 'c1', 'k2', a6, repeat('6', 64), 'completed');
  PERFORM pg_temp.put_hash('known-smoke', 'c1', a6, m.version_id, m.artifact_id, z256);
  SELECT * INTO m FROM pg_temp.mk_image('known-other', 'c1', 'k1', f, repeat('f', 64), 'completed');
  PERFORM pg_temp.put_hash('known-other', 'c1', f, m.version_id, m.artifact_id, z256);

  INSERT INTO forensic.image_known_set (set_id, tenant_id, name, kind, created_by) VALUES
    ('39000000-0000-4000-8000-000000000001', 'known-smoke', 'watch', 'of_interest', 'smoke-user'),
    ('39000000-0000-4000-8000-000000000002', 'known-smoke', 'harmless', 'known_harmless', 'smoke-user'),
    ('39000000-0000-4000-8000-000000000003', 'known-other', 'other watch', 'of_interest', 'smoke-user');
  INSERT INTO forensic.image_known_hash (tenant_id, set_id, sha256, pdq256, label, source, added_by) VALUES
    ('known-smoke', '39000000-0000-4000-8000-000000000001', NULL, z256::bit(256), 'seed', 'api', 'smoke-user'),
    ('known-smoke', '39000000-0000-4000-8000-000000000001', repeat('4', 64), NULL, 'by file', 'api', 'smoke-user'),
    ('known-smoke', '39000000-0000-4000-8000-000000000002', NULL, ones120::bit(256), 'logo', 'csv', 'smoke-user'),
    ('known-other', '39000000-0000-4000-8000-000000000003', NULL, z256::bit(256), 'seed', 'api', 'smoke-user');
END
$$;

-- 1. the flags
DO $$
DECLARE
  n integer;
BEGIN
  SELECT count(*) INTO n FROM forensic.image_known_matches('c1');
  IF n <> 5 THEN RAISE EXCEPTION 'expected 5 flags in the collection (a1, a2, a4, a5, a6), got %', n; END IF;
  SELECT count(*) INTO n FROM forensic.image_known_matches('c1', 56, 'k1');
  IF n <> 4 THEN RAISE EXCEPTION 'expected 4 flags in case k1, got %', n; END IF;
  IF NOT EXISTS (SELECT 1 FROM forensic.image_known_matches('c1', 56, 'k1') m WHERE m.evidence_id = '39000000-0000-4000-8000-0000000000a4' AND m.tier = 'identical' AND m.distance = 0 AND m.set_name = 'watch' AND m.set_kind = 'of_interest') THEN
    RAISE EXCEPTION 'a4 should be flagged identical by its SHA-256 although it has no hash row';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM forensic.image_known_matches('c1', 56, 'k1') m WHERE m.evidence_id = '39000000-0000-4000-8000-0000000000a1' AND m.tier = 'same_image' AND m.distance = 0 AND m.label = 'seed') THEN
    RAISE EXCEPTION 'a1 should be flagged same_image at distance 0';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM forensic.image_known_matches('c1', 56, 'k1') m WHERE m.evidence_id = '39000000-0000-4000-8000-0000000000a2' AND m.tier = 'same_image' AND m.distance = 3) THEN
    RAISE EXCEPTION 'a2 should be flagged same_image at distance 3';
  END IF;
  IF EXISTS (SELECT 1 FROM forensic.image_known_matches('c1', 56) m WHERE m.evidence_id = '39000000-0000-4000-8000-0000000000a3') THEN
    RAISE EXCEPTION 'a3 (distance 60) must not be flagged at 56';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM forensic.image_known_matches('c1', 60, 'k1') m WHERE m.evidence_id = '39000000-0000-4000-8000-0000000000a3' AND m.distance = 60) THEN
    RAISE EXCEPTION 'a3 should be flagged when the distance is stated as 60';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM forensic.image_known_matches('c1', 56, 'k1') m WHERE m.evidence_id = '39000000-0000-4000-8000-0000000000a5' AND m.set_name = 'harmless' AND m.set_kind = 'known_harmless') THEN
    RAISE EXCEPTION 'a5 should be flagged by the harmless set';
  END IF;
  -- a set filter
  SELECT count(*) INTO n FROM forensic.image_known_matches('c1', 56, 'k1', ARRAY['39000000-0000-4000-8000-000000000002'::uuid]);
  IF n <> 1 THEN RAISE EXCEPTION 'the set filter should leave exactly the flag of a5, got %', n; END IF;
  -- another collection sees nothing
  SELECT count(*) INTO n FROM forensic.image_known_matches('c2');
  IF n <> 0 THEN RAISE EXCEPTION 'an empty collection should have no flags, got %', n; END IF;
END
$$;

-- 2. the coverage line: 2 active sets, 3 hashes, 5 images in case k1, 4 of them with a hash (a4 has none)
DO $$
DECLARE
  s record;
BEGIN
  SELECT * INTO s FROM forensic.image_known_state('c1', 'k1');
  IF s.active_sets <> 2 OR s.known_hashes <> 3 OR s.images_in_case <> 5 OR s.images_with_hash <> 4 THEN
    RAISE EXCEPTION 'coverage line wrong: sets %, hashes %, images %, with hash %', s.active_sets, s.known_hashes, s.images_in_case, s.images_with_hash;
  END IF;
END
$$;

-- 3. retirement: the retired set stops flagging and frees its name; two active sets of one name are refused
UPDATE forensic.image_known_set SET retired_at = now(), retired_by = 'smoke-user' WHERE set_id = '39000000-0000-4000-8000-000000000001';
DO $$
DECLARE
  n integer;
  s record;
BEGIN
  SELECT count(*) INTO n FROM forensic.image_known_matches('c1', 56, 'k1');
  IF n <> 1 THEN RAISE EXCEPTION 'after retiring the watch set only the harmless flag should remain, got %', n; END IF;
  SELECT * INTO s FROM forensic.image_known_state('c1', 'k1');
  IF s.active_sets <> 1 OR s.known_hashes <> 1 THEN RAISE EXCEPTION 'coverage after retirement wrong: sets %, hashes %', s.active_sets, s.known_hashes; END IF;
  INSERT INTO forensic.image_known_set (tenant_id, name, kind, created_by) VALUES ('known-smoke', 'watch', 'of_interest', 'smoke-user');
END
$$;
SELECT pg_temp.expect_error('two active sets with one name (case and spaces ignored)', $q$INSERT INTO forensic.image_known_set (tenant_id, name, kind, created_by) VALUES ('known-smoke', ' WATCH ', 'of_interest', 'smoke-user')$q$, '23505');

-- 4. tenant isolation through the functions: as known-other only its own set flags only its own picture
SELECT set_config('app.tenant_id', 'known-other', true);
DO $$
BEGIN
  IF (SELECT count(*) FROM forensic.image_known_matches('c1')) <> 1
     OR NOT EXISTS (SELECT 1 FROM forensic.image_known_matches('c1') m WHERE m.evidence_id = '39000000-0000-4000-8000-0000000000f0' AND m.set_name = 'other watch') THEN
    RAISE EXCEPTION 'tenant known-other should see exactly its own flag';
  END IF;
  IF (SELECT active_sets FROM forensic.image_known_state('c1')) <> 1 THEN RAISE EXCEPTION 'tenant known-other should count exactly its own set'; END IF;
END
$$;
SELECT set_config('app.tenant_id', 'known-smoke', true);

-- 5. the runtime role (when it exists, with the grants of migration 039)
DO $$
DECLARE
  rt uuid := '39000000-0000-4000-8000-0000000000b1';
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    SET LOCAL ROLE forensic_runtime;
    PERFORM set_config('app.tenant_id', 'known-smoke', true);
    INSERT INTO forensic.image_known_set (set_id, tenant_id, name, kind, created_by) VALUES (rt, 'known-smoke', 'runtime set', 'of_interest', 'smoke-user');
    INSERT INTO forensic.image_known_hash (tenant_id, set_id, sha256, label, source, source_ref, added_by) VALUES ('known-smoke', rt, repeat('9', 64), 'x', 'evidence', 'evidence-id', 'smoke-user');
    INSERT INTO forensic.image_known_event (tenant_id, set_id, kind, actor, collection_id, case_id, detail) VALUES ('known-smoke', rt, 'hashes_added', 'smoke-user', 'c1', 'k1', '{"added": 1}');
    IF (SELECT count(*) FROM forensic.image_known_matches('c1', 56, 'k1')) <> 1 THEN RAISE EXCEPTION 'the runtime role should still see the one harmless flag'; END IF;
    PERFORM pg_temp.expect_error('runtime role may not rename a set', $q$UPDATE forensic.image_known_set SET name = 'x' WHERE name = 'runtime set'$q$, '42501');
    PERFORM pg_temp.expect_error('runtime role may not change a set kind', $q$UPDATE forensic.image_known_set SET kind = 'known_harmless' WHERE name = 'runtime set'$q$, '42501');
    PERFORM pg_temp.expect_error('runtime role may not delete a set', $q$DELETE FROM forensic.image_known_set$q$, '42501');
    PERFORM pg_temp.expect_error('runtime role may not update a hash', $q$UPDATE forensic.image_known_hash SET label = 'y'$q$, '42501');
    PERFORM pg_temp.expect_error('runtime role may not delete a hash', $q$DELETE FROM forensic.image_known_hash$q$, '42501');
    PERFORM pg_temp.expect_error('runtime role may not update an event', $q$UPDATE forensic.image_known_event SET actor = 'y'$q$, '42501');
    PERFORM pg_temp.expect_error('runtime role may not delete an event', $q$DELETE FROM forensic.image_known_event$q$, '42501');
    PERFORM pg_temp.expect_error('a set for another tenant', $q$INSERT INTO forensic.image_known_set (tenant_id, name, kind, created_by) VALUES ('known-other', 'sneaky', 'of_interest', 'smoke-user')$q$, '42501');
    UPDATE forensic.image_known_set SET retired_at = now(), retired_by = 'smoke-user' WHERE set_id = rt;
    IF (SELECT count(*) FROM forensic.image_known_set) <> 4 THEN RAISE EXCEPTION 'the runtime role should see the four sets of its tenant (watch retired, harmless, new watch, runtime set), got %', (SELECT count(*) FROM forensic.image_known_set); END IF;
    PERFORM set_config('app.tenant_id', 'known-other', true);
    IF (SELECT count(*) FROM forensic.image_known_set) <> 1 THEN RAISE EXCEPTION 'row-level security should leave exactly the one set of known-other'; END IF;
    IF (SELECT count(*) FROM forensic.image_known_hash) <> 1 THEN RAISE EXCEPTION 'row-level security should leave exactly the one hash of known-other'; END IF;
    RESET ROLE;
  END IF;
END
$$;
SELECT set_config('app.tenant_id', 'known-smoke', true);

-- 6. constraints and hash-only
SELECT pg_temp.expect_error('a set kind that does not exist', $q$INSERT INTO forensic.image_known_set (tenant_id, name, kind, created_by) VALUES ('known-smoke', 'k', 'face_set', 'smoke-user')$q$, '23514');
SELECT pg_temp.expect_error('an empty set name', $q$INSERT INTO forensic.image_known_set (tenant_id, name, kind, created_by) VALUES ('known-smoke', '   ', 'of_interest', 'smoke-user')$q$, '23514');
SELECT pg_temp.expect_error('retired_by without retired_at', $q$INSERT INTO forensic.image_known_set (tenant_id, name, kind, created_by, retired_by) VALUES ('known-smoke', 'r', 'of_interest', 'smoke-user', 'x')$q$, '23514');
SELECT pg_temp.expect_error('a hash row with no hash', $q$INSERT INTO forensic.image_known_hash (tenant_id, set_id, label, source, added_by) VALUES ('known-smoke', '39000000-0000-4000-8000-000000000002', 'x', 'api', 'smoke-user')$q$, '23514');
SELECT pg_temp.expect_error('a malformed sha256', $q$INSERT INTO forensic.image_known_hash (tenant_id, set_id, sha256, source, added_by) VALUES ('known-smoke', '39000000-0000-4000-8000-000000000002', 'ABC', 'api', 'smoke-user')$q$, '23514');
SELECT pg_temp.expect_error('a source that does not exist', $q$INSERT INTO forensic.image_known_hash (tenant_id, set_id, sha256, source, added_by) VALUES ('known-smoke', '39000000-0000-4000-8000-000000000002', repeat('7', 64), 'paste', 'smoke-user')$q$, '23514');
SELECT pg_temp.expect_error('a label longer than 512 characters', $q$INSERT INTO forensic.image_known_hash (tenant_id, set_id, sha256, label, source, added_by) VALUES ('known-smoke', '39000000-0000-4000-8000-000000000002', repeat('7', 64), repeat('x', 513), 'api', 'smoke-user')$q$, '23514');
SELECT pg_temp.expect_error('the same hash twice in one set', $q$INSERT INTO forensic.image_known_hash (tenant_id, set_id, sha256, pdq256, source, added_by) SELECT tenant_id, set_id, sha256, pdq256, source, added_by FROM forensic.image_known_hash WHERE label = 'logo'$q$, '23505');
SELECT pg_temp.expect_error('a hash in a set of another tenant', $q$INSERT INTO forensic.image_known_hash (tenant_id, set_id, sha256, source, added_by) VALUES ('known-smoke', '39000000-0000-4000-8000-000000000003', repeat('8', 64), 'api', 'smoke-user')$q$, '23503');
SELECT pg_temp.expect_error('an event kind that does not exist', $q$INSERT INTO forensic.image_known_event (tenant_id, kind, actor) VALUES ('known-smoke', 'peek', 'smoke-user')$q$, '23514');
SELECT pg_temp.expect_error('an event without an actor', $q$INSERT INTO forensic.image_known_event (tenant_id, kind, actor) VALUES ('known-smoke', 'check', ' ')$q$, '23514');
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns k WHERE k.table_schema = 'forensic' AND k.table_name IN ('image_known_set', 'image_known_hash', 'image_known_event') AND k.data_type = 'bytea') THEN
    RAISE EXCEPTION 'a known-set table has a bytea column';
  END IF;
END
$$;

SELECT 'migration_039_smoke_passed' AS result;

ROLLBACK;
