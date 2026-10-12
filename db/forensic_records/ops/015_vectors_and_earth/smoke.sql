-- Synthetic acceptance checks for migration 015. DISPOSABLE DATABASE ONLY; one rolled-back transaction.
-- Proves: vector rows load and rank by cosine distance, a wrong dimension is refused, tenants are isolated, and a radius
-- lookup over cell sites returns exactly the sites inside the radius and uses the earth index.
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

SELECT set_config('app.tenant_id', 'vec-smoke', true);

INSERT INTO forensic.evidence_items (
  evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension,
  size_bytes, sha256, modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata
) VALUES
  ('34000000-0000-4000-8000-000000000001', 'vec-smoke', 'vec-smoke-collection', 'PK-SYNTHETIC-VEC', 'vec-smoke-user',
   'synthetic.txt', 'synthetic.txt', 'text/plain', 'txt', 16, repeat('9', 64), 'document', 'text', 1,
   'forensic_records_worker', 'completed', 'immutable://vec-smoke/sha256/' || repeat('9', 64),
   jsonb_build_object('request_id', 'vec-smoke-request'));

-- Three-dimensional synthetic vectors: chunk-a is closest to the query direction, chunk-c is opposite.
INSERT INTO forensic.content_embedding (tenant_id, collection_id, evidence_id, modality, chunk_ref, model_id, dim, embedding) VALUES
  ('vec-smoke', 'vec-smoke-collection', '34000000-0000-4000-8000-000000000001', 'text', 'chunk-a', 'synthetic-model', 3, '[1, 0, 0]'),
  ('vec-smoke', 'vec-smoke-collection', '34000000-0000-4000-8000-000000000001', 'text', 'chunk-b', 'synthetic-model', 3, '[1, 1, 0]'),
  ('vec-smoke', 'vec-smoke-collection', '34000000-0000-4000-8000-000000000001', 'text', 'chunk-c', 'synthetic-model', 3, '[-1, 0, 0]');

DO $$
DECLARE
  order_seen text;
BEGIN
  SELECT string_agg(chunk_ref, ',' ORDER BY embedding <=> '[1, 0.1, 0]'::vector) INTO order_seen
  FROM forensic.content_embedding WHERE model_id = 'synthetic-model';
  IF order_seen <> 'chunk-a,chunk-b,chunk-c' THEN
    RAISE EXCEPTION 'cosine ranking returned %', order_seen;
  END IF;
END
$$;

SELECT pg_temp.expect_error('stated dimension that does not match the vector', $q$
  INSERT INTO forensic.content_embedding (tenant_id, collection_id, evidence_id, modality, chunk_ref, model_id, dim, embedding)
  VALUES ('vec-smoke', 'vec-smoke-collection', '34000000-0000-4000-8000-000000000001', 'text', 'chunk-d', 'synthetic-model', 4, '[1, 0, 0]')$q$, '23514');
SELECT pg_temp.expect_error('face modality before the policy exists', $q$
  INSERT INTO forensic.content_embedding (tenant_id, collection_id, evidence_id, modality, chunk_ref, model_id, dim, embedding)
  VALUES ('vec-smoke', 'vec-smoke-collection', '34000000-0000-4000-8000-000000000001', 'face', 'face-1', 'synthetic-model', 3, '[1, 0, 0]')$q$, '23514');
SELECT pg_temp.expect_error('the same chunk and model twice', $q$
  INSERT INTO forensic.content_embedding (tenant_id, collection_id, evidence_id, modality, chunk_ref, model_id, dim, embedding)
  VALUES ('vec-smoke', 'vec-smoke-collection', '34000000-0000-4000-8000-000000000001', 'text', 'chunk-a', 'synthetic-model', 3, '[1, 0, 0]')$q$, '23505');
SELECT pg_temp.expect_error('evidence of another collection', $q$
  INSERT INTO forensic.content_embedding (tenant_id, collection_id, evidence_id, modality, chunk_ref, model_id, dim, embedding)
  VALUES ('vec-smoke', 'another-collection', '34000000-0000-4000-8000-000000000001', 'text', 'chunk-e', 'synthetic-model', 3, '[1, 0, 0]')$q$, '23503');

-- Radius lookup over cell sites: origin at (10, 20); one site about 1 km north, one about 5 km north, one at the origin.
INSERT INTO forensic.cell_site (tenant_id, cell_key, plmn, lac_tac, ci_eci, lat, lon, source) VALUES
  ('vec-smoke', forensic.make_cell_key('00101', '1', '1'), '00101', '1', '1', 10.0, 20.0, 'synthetic-list'),
  ('vec-smoke', forensic.make_cell_key('00101', '1', '2'), '00101', '1', '2', 10.008983, 20.0, 'synthetic-list'),
  ('vec-smoke', forensic.make_cell_key('00101', '1', '3'), '00101', '1', '3', 10.044915, 20.0, 'synthetic-list');

DO $$
DECLARE
  keys text;
BEGIN
  SELECT string_agg(ci_eci, ',' ORDER BY ci_eci) INTO keys
  FROM forensic.cell_site
  WHERE earth_box(ll_to_earth(10.0, 20.0), 2000) @> ll_to_earth(lat, lon)
    AND earth_distance(ll_to_earth(10.0, 20.0), ll_to_earth(lat, lon)) <= 2000;
  IF keys IS DISTINCT FROM '1,2' THEN RAISE EXCEPTION 'a 2 km radius returned cells %, expected 1,2', keys; END IF;

  SELECT string_agg(ci_eci, ',' ORDER BY ci_eci) INTO keys
  FROM forensic.cell_site
  WHERE earth_box(ll_to_earth(10.0, 20.0), 6000) @> ll_to_earth(lat, lon)
    AND earth_distance(ll_to_earth(10.0, 20.0), ll_to_earth(lat, lon)) <= 6000;
  IF keys IS DISTINCT FROM '1,2,3' THEN RAISE EXCEPTION 'a 6 km radius returned cells %, expected 1,2,3', keys; END IF;
END
$$;

-- The planner can use the earth index (forced here because the table is tiny).
SET LOCAL enable_seqscan = off;
DO $$
DECLARE
  plan_line text;
  used boolean := false;
BEGIN
  FOR plan_line IN EXECUTE
    'EXPLAIN SELECT 1 FROM forensic.cell_site WHERE earth_box(ll_to_earth(10.0, 20.0), 2000) @> ll_to_earth(lat, lon)'
  LOOP
    IF plan_line LIKE '%cell_site_earth_idx%' THEN used := true; END IF;
  END LOOP;
  IF NOT used THEN RAISE EXCEPTION 'the earth index is not used for a radius lookup'; END IF;
END
$$;
SET LOCAL enable_seqscan = on;

-- Tenant isolation for a non-owner role without BYPASSRLS.
CREATE ROLE vec_smoke_rls NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
GRANT USAGE ON SCHEMA forensic TO vec_smoke_rls;
GRANT SELECT, INSERT ON forensic.content_embedding TO vec_smoke_rls;

SET LOCAL ROLE vec_smoke_rls;
SELECT set_config('app.tenant_id', 'vec-smoke-other', true);
DO $$
BEGIN
  IF (SELECT count(*) FROM forensic.content_embedding) <> 0 THEN
    RAISE EXCEPTION 'another tenant can see embeddings';
  END IF;
END
$$;
SELECT pg_temp.expect_error('cross-tenant embedding insert', $q$
  INSERT INTO forensic.content_embedding (tenant_id, collection_id, evidence_id, modality, chunk_ref, model_id, dim, embedding)
  VALUES ('vec-smoke', 'vec-smoke-collection', '34000000-0000-4000-8000-000000000001', 'text', 'chunk-x', 'synthetic-model', 3, '[1, 0, 0]')$q$, '42501');
RESET ROLE;

SELECT 'migration_015_smoke_passed' AS result;

ROLLBACK;
