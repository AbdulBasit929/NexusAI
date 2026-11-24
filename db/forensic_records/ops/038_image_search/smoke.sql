-- Synthetic acceptance checks for migration 038. DISPOSABLE DATABASE ONLY; one rolled-back transaction.
-- Proves: the coverage line counts what it should (in case, indexed, failed, identical re-uploads), the hash search and the vector search return exactly the images inside the stated distance and nothing from another
-- collection or tenant, a superseded version is never returned, a vector of another model or dimension is never mixed in, the exact-file search finds re-uploads, and (when the runtime role exists) that role can
-- search and log but cannot change or delete a hash or a log row, and cannot write a row for another tenant.
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
    p_evidence, p_tenant, p_collection, p_case, 'img-smoke-user', 'photo-' || substr(p_evidence::text, 1, 8) || '.jpg', 'photo.jpg', 'image/jpeg', 'jpg',
    1000, p_sha, 'image', 'image', 1, 'unified_media_worker', p_status, 'immutable://img-smoke/sha256/' || p_sha,
    jsonb_build_object('request_id', 'img-smoke-request'));
  SELECT e.current_version_id INTO version_id FROM forensic.evidence_items e WHERE e.evidence_id = p_evidence;
  INSERT INTO forensic.processing_runs (run_id, tenant_id, collection_id, evidence_id, version_id, run_kind, pipeline_id, idempotency_key, status, input_sha256)
  VALUES (run, p_tenant, p_collection, p_evidence, version_id, 'ingest', 'img-smoke', 'img-smoke:' || run::text, 'succeeded', p_sha);
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

CREATE FUNCTION pg_temp.put_vec(p_tenant text, p_collection text, p_evidence uuid, p_version uuid, p_artifact uuid, p_model text, p_vec text) RETURNS void
LANGUAGE sql AS $$
  INSERT INTO forensic.content_embedding (tenant_id, collection_id, evidence_id, derived_artifact_id, modality, chunk_ref, model_id, dim, embedding)
  VALUES (p_tenant, p_collection, p_evidence, p_artifact, 'image', 'v:' || p_version::text || ':full', p_model, vector_dims(p_vec::vector), p_vec::vector)
$$;

SELECT set_config('app.tenant_id', 'img-smoke', true);

-- Scenario (tenant img-smoke, collection c1, case k1):
--   a1  completed, hash 0^256, dino [1,0,0], siglip2 [0,1,0,0]
--   a2  completed, hash with 3 bits set (distance 3 from a1), dino close to a1
--   b   completed, hash with 128 bits set, dino [0,0,1]
--   c   failed (no hash)             d  duplicate re-upload of a1's file (status duplicate, same sha)
--   g   superseded: its FIRST version carries the hash of a1, its CURRENT version carries a far hash
--   e   same hash as a1, but collection c2            f   same hash as a1, tenant img-other, collection c1
DO $$
DECLARE
  z256 text := repeat('0', 256);
  near text := repeat('0', 253) || '111';
  far text := repeat('1', 128) || repeat('0', 128);
  a1 uuid := '38000000-0000-4000-8000-0000000000a1'; a2 uuid := '38000000-0000-4000-8000-0000000000a2';
  b uuid := '38000000-0000-4000-8000-0000000000b0'; c uuid := '38000000-0000-4000-8000-0000000000c0';
  d uuid := '38000000-0000-4000-8000-0000000000d0'; g uuid := '38000000-0000-4000-8000-0000000000e0';
  e uuid := '38000000-0000-4000-8000-0000000000e1'; f uuid := '38000000-0000-4000-8000-0000000000f0';
  sha_a text := repeat('a', 64);
  m record;
  v2 uuid := gen_random_uuid();
  storage uuid;
  run2 uuid := gen_random_uuid();
  art2 uuid := gen_random_uuid();
  g_old_version uuid;
BEGIN
  SELECT * INTO m FROM pg_temp.mk_image('img-smoke', 'c1', 'k1', a1, sha_a, 'completed');
  PERFORM pg_temp.put_hash('img-smoke', 'c1', a1, m.version_id, m.artifact_id, z256);
  PERFORM pg_temp.put_vec('img-smoke', 'c1', a1, m.version_id, m.artifact_id, 'dinov2-test', '[1,0,0]');
  PERFORM pg_temp.put_vec('img-smoke', 'c1', a1, m.version_id, m.artifact_id, 'siglip2-test', '[0,1,0,0]');

  SELECT * INTO m FROM pg_temp.mk_image('img-smoke', 'c1', 'k1', a2, repeat('b', 64), 'completed');
  PERFORM pg_temp.put_hash('img-smoke', 'c1', a2, m.version_id, m.artifact_id, near);
  PERFORM pg_temp.put_vec('img-smoke', 'c1', a2, m.version_id, m.artifact_id, 'dinov2-test', '[0.99,0.1,0]');

  SELECT * INTO m FROM pg_temp.mk_image('img-smoke', 'c1', 'k1', b, repeat('c', 64), 'completed');
  PERFORM pg_temp.put_hash('img-smoke', 'c1', b, m.version_id, m.artifact_id, far);
  PERFORM pg_temp.put_vec('img-smoke', 'c1', b, m.version_id, m.artifact_id, 'dinov2-test', '[0,0,1]');

  SELECT * INTO m FROM pg_temp.mk_image('img-smoke', 'c1', 'k1', c, repeat('d', 64), 'failed');
  SELECT * INTO m FROM pg_temp.mk_image('img-smoke', 'c1', 'k1', d, sha_a, 'duplicate');

  -- g: version 1 carries the hash of a1; then version 2 becomes current and carries a far hash
  SELECT * INTO m FROM pg_temp.mk_image('img-smoke', 'c1', 'k1', g, repeat('e', 64), 'completed');
  g_old_version := m.version_id;
  PERFORM pg_temp.put_hash('img-smoke', 'c1', g, m.version_id, m.artifact_id, z256);
  PERFORM pg_temp.put_vec('img-smoke', 'c1', g, m.version_id, m.artifact_id, 'dinov2-test', '[1,0,0]');
  SELECT v.storage_object_id INTO storage FROM forensic.evidence_versions v WHERE v.version_id = g_old_version;
  INSERT INTO forensic.evidence_versions (version_id, tenant_id, collection_id, evidence_id, version_number, previous_version_id, storage_object_id, content_sha256, size_bytes, media_type, source_name, version_reason)
  VALUES (v2, 'img-smoke', 'c1', g, 2, g_old_version, storage, repeat('e', 64), 1000, 'image/jpeg', 'photo.jpg', 'smoke_replacement');
  UPDATE forensic.evidence_items SET current_version_id = v2 WHERE evidence_id = g;
  INSERT INTO forensic.processing_runs (run_id, tenant_id, collection_id, evidence_id, version_id, run_kind, pipeline_id, idempotency_key, status, input_sha256)
  VALUES (run2, 'img-smoke', 'c1', g, v2, 'ingest', 'img-smoke', 'img-smoke:' || run2::text, 'succeeded', repeat('e', 64));
  INSERT INTO forensic.derived_artifacts (artifact_id, tenant_id, collection_id, evidence_id, version_id, run_id, artifact_type, media_type, processing_status)
  VALUES (art2, 'img-smoke', 'c1', g, v2, run2, 'forensics.image-hash-observation/v1', 'application/json', 'completed');
  PERFORM pg_temp.put_hash('img-smoke', 'c1', g, v2, art2, far);

  -- another collection of the same tenant
  SELECT * INTO m FROM pg_temp.mk_image('img-smoke', 'c2', 'k9', e, repeat('1', 64), 'completed');
  PERFORM pg_temp.put_hash('img-smoke', 'c2', e, m.version_id, m.artifact_id, z256);
  PERFORM pg_temp.put_vec('img-smoke', 'c2', e, m.version_id, m.artifact_id, 'dinov2-test', '[1,0,0]');

  -- another tenant, same collection name
  PERFORM set_config('app.tenant_id', 'img-other', true);
  SELECT * INTO m FROM pg_temp.mk_image('img-other', 'c1', 'k1', f, repeat('2', 64), 'completed');
  PERFORM pg_temp.put_hash('img-other', 'c1', f, m.version_id, m.artifact_id, z256);
  PERFORM pg_temp.put_vec('img-other', 'c1', f, m.version_id, m.artifact_id, 'dinov2-test', '[1,0,0]');
  PERFORM set_config('app.tenant_id', 'img-smoke', true);
END
$$;

-- 1. the coverage line: in case a1 a2 b c g = 5 (the re-upload d is counted apart), indexed a1 a2 b g = 4, failed c = 1, re-uploads d = 1
DO $$
DECLARE s record;
BEGIN
  SELECT * INTO s FROM forensic.image_index_state('c1', 'k1');
  IF s.images_in_case <> 5 OR s.images_indexed <> 4 OR s.images_failed <> 1 OR s.images_identical_reupload <> 1 THEN
    RAISE EXCEPTION 'coverage line is %/%/%/%, expected 5/4/1/1', s.images_in_case, s.images_indexed, s.images_failed, s.images_identical_reupload;
  END IF;
  SELECT * INTO s FROM forensic.image_index_state('c1', NULL);
  IF s.images_in_case <> 5 THEN RAISE EXCEPTION 'coverage without a case filter is %, expected 5', s.images_in_case; END IF;
  SELECT * INTO s FROM forensic.image_index_state('c1', 'k-no-such-case');
  IF s.images_in_case <> 0 THEN RAISE EXCEPTION 'coverage of an unknown case is %, expected 0', s.images_in_case; END IF;
END
$$;

-- 2. the hash search: inside 10 bits of a1's hash only a1 (0 bits) and a2 (3 bits); never g's superseded hash, never the other collection or tenant
DO $$
DECLARE
  seen text;
BEGIN
  SELECT string_agg(evidence_id::text || ':' || distance, ',' ORDER BY distance, evidence_id) INTO seen
  FROM forensic.image_search_hash('c1', repeat('0', 256)::bit(256), 10, 'k1');
  IF seen IS DISTINCT FROM '38000000-0000-4000-8000-0000000000a1:0,38000000-0000-4000-8000-0000000000a2:3' THEN
    RAISE EXCEPTION 'hash search returned %', seen;
  END IF;
  SELECT string_agg(evidence_id::text, ',' ORDER BY evidence_id) INTO seen
  FROM forensic.image_search_hash('c1', repeat('0', 256)::bit(256), 10, 'k1', '38000000-0000-4000-8000-0000000000a1');
  IF seen IS DISTINCT FROM '38000000-0000-4000-8000-0000000000a2' THEN RAISE EXCEPTION 'probe exclusion failed: %', seen; END IF;
  IF (SELECT count(*) FROM forensic.image_search_hash('c1', repeat('0', 256)::bit(256), 256, 'k1')) <> 4 THEN
    RAISE EXCEPTION 'an unbounded distance should return every indexed image (4), got %', (SELECT count(*) FROM forensic.image_search_hash('c1', repeat('0', 256)::bit(256), 256, 'k1'));
  END IF;
  IF (SELECT count(*) FROM forensic.image_search_hash('c1', repeat('0', 256)::bit(256), 10, 'k-no-such-case')) <> 0 THEN
    RAISE EXCEPTION 'a search scoped to another case returned rows';
  END IF;
END
$$;

-- 3. the vector search: dino probe [1,0,0] inside 0.2 cosine distance returns a1 (0) and a2 (about 0.005), not b, not g (superseded version), not another collection or tenant; another model or dimension returns nothing
DO $$
DECLARE
  seen text;
BEGIN
  SELECT string_agg(evidence_id::text, ',' ORDER BY distance, evidence_id) INTO seen
  FROM forensic.image_search_vector('c1', 'dinov2-test', '[1,0,0]'::vector, 0.2, 'k1');
  IF seen IS DISTINCT FROM '38000000-0000-4000-8000-0000000000a1,38000000-0000-4000-8000-0000000000a2' THEN
    RAISE EXCEPTION 'vector search returned %', seen;
  END IF;
  IF (SELECT count(*) FROM forensic.image_search_vector('c1', 'dinov2-test', '[1,0,0,0]'::vector, 2, 'k1')) <> 0 THEN
    RAISE EXCEPTION 'a probe of another dimension must return nothing (and not fail)';
  END IF;
  IF (SELECT count(*) FROM forensic.image_search_vector('c1', 'no-such-model', '[1,0,0]'::vector, 2, 'k1')) <> 0 THEN
    RAISE EXCEPTION 'a model with no vectors must return nothing';
  END IF;
  IF (SELECT count(*) FROM forensic.image_search_vector('c1', 'siglip2-test', '[0,1,0,0]'::vector, 0.01, 'k1')) <> 1 THEN
    RAISE EXCEPTION 'the second model should find exactly its own row';
  END IF;
  IF (SELECT evidence_id FROM forensic.image_search_vector('c1', 'dinov2-test', '[1,0,0]'::vector, 2, 'k1', NULL, 1)) IS DISTINCT FROM '38000000-0000-4000-8000-0000000000a1' THEN
    RAISE EXCEPTION 'limit 1 should return the nearest image';
  END IF;
END
$$;

-- 4. identical files: a1 and its re-upload d share a file hash
DO $$
DECLARE
  seen text;
BEGIN
  SELECT string_agg(evidence_id::text, ',' ORDER BY evidence_id) INTO seen FROM forensic.image_exact_matches('c1', repeat('a', 64), 'k1');
  IF seen IS DISTINCT FROM '38000000-0000-4000-8000-0000000000a1,38000000-0000-4000-8000-0000000000d0' THEN
    RAISE EXCEPTION 'exact-file search returned %', seen;
  END IF;
  IF (SELECT count(*) FROM forensic.image_exact_matches('c2', repeat('a', 64))) <> 0 THEN
    RAISE EXCEPTION 'exact-file search crossed into another collection';
  END IF;
END
$$;

-- 5. tenant isolation through the setting: as img-other only f is visible and a1 is not
SELECT set_config('app.tenant_id', 'img-other', true);
DO $$
BEGIN
  IF (SELECT count(*) FROM forensic.image_search_hash('c1', repeat('0', 256)::bit(256), 256)) <> 1 THEN
    RAISE EXCEPTION 'tenant img-other should see exactly its own one image through the function';
  END IF;
  IF (SELECT images_in_case FROM forensic.image_index_state('c1')) <> 1 THEN
    RAISE EXCEPTION 'tenant img-other should count exactly its own one image';
  END IF;
END
$$;
SELECT set_config('app.tenant_id', 'img-smoke', true);

-- 6. the runtime role (when it exists, with the grants of migration 038): reads and searches, logs searches, cannot change or delete, cannot write for another tenant, and row-level security applies inside the functions
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    SET LOCAL ROLE forensic_runtime;
    PERFORM set_config('app.tenant_id', 'img-smoke', true);
    IF (SELECT count(*) FROM forensic.image_search_hash('c1', repeat('0', 256)::bit(256), 10, 'k1')) <> 2 THEN
      RAISE EXCEPTION 'the runtime role should find the two near images';
    END IF;
    IF (SELECT images_in_case FROM forensic.image_index_state('c1', 'k1')) <> 5 THEN
      RAISE EXCEPTION 'the runtime role should count five images in the case';
    END IF;
    INSERT INTO forensic.image_search_log (tenant_id, collection_id, case_id, searched_by, kind, parameters, result_count, images_in_case, images_indexed)
    VALUES ('img-smoke', 'c1', 'k1', 'smoke-user', 'copies', '{"max_distance": 10}', 2, 5, 4);
    PERFORM pg_temp.expect_error('runtime role may not update a log row', $q$UPDATE forensic.image_search_log SET result_count = 0$q$, '42501');
    PERFORM pg_temp.expect_error('runtime role may not delete a log row', $q$DELETE FROM forensic.image_search_log$q$, '42501');
    PERFORM pg_temp.expect_error('runtime role may not update a hash', $q$UPDATE forensic.image_hash SET algorithm = 'x'$q$, '42501');
    PERFORM pg_temp.expect_error('runtime role may not delete a hash', $q$DELETE FROM forensic.image_hash$q$, '42501');
    PERFORM pg_temp.expect_error('a log row for another tenant', $q$INSERT INTO forensic.image_search_log (tenant_id, collection_id, searched_by, kind) VALUES ('img-other', 'c1', 'smoke-user', 'by_words')$q$, '42501');
    RESET ROLE;
  END IF;
END
$$;

-- 7. constraints
SELECT pg_temp.expect_error('a search kind that does not exist', $q$INSERT INTO forensic.image_search_log (tenant_id, collection_id, searched_by, kind) VALUES ('img-smoke', 'c1', 'smoke-user', 'face_search')$q$, '23514');
SELECT pg_temp.expect_error('a bad probe hash', $q$INSERT INTO forensic.image_search_log (tenant_id, collection_id, searched_by, kind, probe_sha256) VALUES ('img-smoke', 'c1', 'smoke-user', 'copies', 'xyz')$q$, '23514');
SELECT pg_temp.expect_error('a hash row twice for one version', $q$
  INSERT INTO forensic.image_hash (tenant_id, collection_id, evidence_id, version_id, derived_artifact_id, chunk_ref, algorithm, pdq256)
  SELECT tenant_id, collection_id, evidence_id, version_id, derived_artifact_id, chunk_ref, algorithm, pdq256 FROM forensic.image_hash WHERE evidence_id = '38000000-0000-4000-8000-0000000000a1'$q$, '23505');

SELECT 'migration_038_smoke_passed' AS result;

ROLLBACK;
