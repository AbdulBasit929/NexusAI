-- Synthetic acceptance checks for migration 014. DISPOSABLE DATABASE ONLY; one rolled-back transaction.
-- Proves: edges are typed and explainable, inferred edges need a derivation, a person exists only when an analyst makes one,
-- no face relationship can be stored, evidence cascades with its edge, tenants are isolated, and the runtime role can record
-- analyst review but cannot rewrite what an edge is.
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

SELECT set_config('app.tenant_id', 'link-smoke', true);

INSERT INTO forensic.evidence_items (
  evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension,
  size_bytes, sha256, modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata
) VALUES
  ('33000000-0000-4000-8000-000000000001', 'link-smoke', 'link-smoke-collection', 'PK-SYNTHETIC-LINK', 'link-smoke-user',
   'synthetic.pdf', 'synthetic.pdf', 'application/pdf', 'pdf', 64, repeat('f', 64), 'document', 'pdf', 1,
   'forensic_records_worker', 'completed', 'immutable://link-smoke/sha256/' || repeat('f', 64),
   jsonb_build_object('request_id', 'link-smoke-request'));

INSERT INTO forensic.derivation (derivation_id, tenant_id, collection_id, procedure, parameters, code_version, seed, run_by)
VALUES ('33000000-0000-4000-8000-0000000000d1', 'link-smoke', 'link-smoke-collection', 'co_location.v1',
        '{"window_minutes": 10}', 'smoke-0', 42, 'smoke');

-- Entities. Phone numbers are the synthetic ones used across the repository fixtures; ids are fixed so the checks are readable.
INSERT INTO forensic.entity (entity_id, tenant_id, collection_id, entity_type, canonical) VALUES
  ('33000000-0000-4000-8000-0000000000e1', 'link-smoke', 'link-smoke-collection', 'phone', '923000000001'),
  ('33000000-0000-4000-8000-0000000000e2', 'link-smoke', 'link-smoke-collection', 'phone', '923000000002'),
  ('33000000-0000-4000-8000-0000000000e3', 'link-smoke', 'link-smoke-collection', 'imei', '350000000000000'),
  ('33000000-0000-4000-8000-0000000000e4', 'link-smoke', 'link-smoke-collection', 'evidence', '33000000-0000-4000-8000-000000000001');

INSERT INTO forensic.entity (entity_id, tenant_id, collection_id, entity_type, canonical, origin, display_label) VALUES
  ('33000000-0000-4000-8000-0000000000e5', 'link-smoke', 'link-smoke-collection', 'person',
   'person:33000000-0000-4000-8000-0000000000e5', 'analyst', 'Synthetic Person');

-- An observed edge, an inferred (symmetric) edge with its derivation, and an observed mention edge.
INSERT INTO forensic.relationship (rel_id, tenant_id, collection_id, src_entity, dst_entity, rel_type, kind, directed, evidence_count, score)
VALUES ('33000000-0000-4000-8000-0000000000f1', 'link-smoke', 'link-smoke-collection',
        '33000000-0000-4000-8000-0000000000e1', '33000000-0000-4000-8000-0000000000e2', 'called', 'observed', true, 2, 0.5);

INSERT INTO forensic.relationship (rel_id, tenant_id, collection_id, src_entity, dst_entity, rel_type, kind, directed, derivation_id)
VALUES ('33000000-0000-4000-8000-0000000000f2', 'link-smoke', 'link-smoke-collection',
        '33000000-0000-4000-8000-0000000000e1', '33000000-0000-4000-8000-0000000000e2', 'co_located', 'inferred', false,
        '33000000-0000-4000-8000-0000000000d1');

INSERT INTO forensic.relationship_evidence (rel_id, tenant_id, collection_id, source_table, source_key, observed_at) VALUES
  ('33000000-0000-4000-8000-0000000000f1', 'link-smoke', 'link-smoke-collection', 'comm_event', '1', '2026-01-05 10:00:00+00'),
  ('33000000-0000-4000-8000-0000000000f1', 'link-smoke', 'link-smoke-collection', 'comm_event', '2', '2026-01-06 10:00:00+00');

INSERT INTO forensic.entity_mention (tenant_id, collection_id, entity_id, evidence_id, source_table, source_key, role, confidence, locator)
VALUES ('link-smoke', 'link-smoke-collection', '33000000-0000-4000-8000-0000000000e1', '33000000-0000-4000-8000-000000000001',
        'evidence', 'page-3', 'mentioned', 0.82, '{"page": 3}');

INSERT INTO forensic.ip_assignment (tenant_id, collection_id, public_ip, port_from, port_to, valid_from, valid_to, msisdn, source_key, derivation_id)
VALUES ('link-smoke', 'link-smoke-collection', '203.0.113.9', 1024, 2047, '2026-01-05 11:00:00+00', '2026-01-05 11:05:00+00',
        '923000000001', 'session-1', '33000000-0000-4000-8000-0000000000d1');

DO $$
BEGIN
  IF (SELECT count(*) FROM forensic.relationship) <> 2 THEN RAISE EXCEPTION 'expected 2 relationships'; END IF;
  IF (SELECT count(*) FROM forensic.relationship_evidence) <> 2 THEN RAISE EXCEPTION 'expected 2 evidence rows'; END IF;

  -- Explainable: every observed edge with evidence_count > 0 has at least that many stored evidence rows here.
  IF EXISTS (SELECT 1 FROM forensic.relationship r WHERE r.kind = 'observed' AND r.evidence_count > 0
               AND (SELECT count(*) FROM forensic.relationship_evidence e WHERE e.rel_id = r.rel_id) <> r.evidence_count) THEN
    RAISE EXCEPTION 'an observed edge is not fully explained by stored evidence';
  END IF;

  -- IP attribution lookup: who held 203.0.113.9 port 1500 at 11:02?
  IF (SELECT count(*) FROM forensic.ip_assignment
      WHERE public_ip = '203.0.113.9' AND 1500 BETWEEN port_from AND port_to
        AND '2026-01-05 11:02:00+00' BETWEEN valid_from AND valid_to) <> 1 THEN
    RAISE EXCEPTION 'IP attribution lookup did not return exactly one holder';
  END IF;
END
$$;

-- Rules.
SELECT pg_temp.expect_error('a person created by the system', $q$
  INSERT INTO forensic.entity (tenant_id, collection_id, entity_type, canonical)
  VALUES ('link-smoke', 'link-smoke-collection', 'person', 'person:x')$q$, '23514');
SELECT pg_temp.expect_error('a person keyed by a name', $q$
  INSERT INTO forensic.entity (tenant_id, collection_id, entity_type, canonical, origin)
  VALUES ('link-smoke', 'link-smoke-collection', 'person', 'Synthetic Person', 'analyst')$q$, '23514');
SELECT pg_temp.expect_error('unknown entity type', $q$
  INSERT INTO forensic.entity (tenant_id, collection_id, entity_type, canonical)
  VALUES ('link-smoke', 'link-smoke-collection', 'face', 'x')$q$, '23514');
SELECT pg_temp.expect_error('the same entity twice', $q$
  INSERT INTO forensic.entity (tenant_id, collection_id, entity_type, canonical)
  VALUES ('link-smoke', 'link-smoke-collection', 'phone', '923000000001')$q$, '23505');
SELECT pg_temp.expect_error('an edge from an entity to itself', $q$
  INSERT INTO forensic.relationship (tenant_id, collection_id, src_entity, dst_entity, rel_type, kind)
  VALUES ('link-smoke', 'link-smoke-collection', '33000000-0000-4000-8000-0000000000e1', '33000000-0000-4000-8000-0000000000e1', 'called', 'observed')$q$, '23514');
SELECT pg_temp.expect_error('a symmetric edge stored in the wrong order', $q$
  INSERT INTO forensic.relationship (tenant_id, collection_id, src_entity, dst_entity, rel_type, kind, directed, derivation_id)
  VALUES ('link-smoke', 'link-smoke-collection', '33000000-0000-4000-8000-0000000000e2', '33000000-0000-4000-8000-0000000000e1', 'shared_ip', 'inferred', false,
          '33000000-0000-4000-8000-0000000000d1')$q$, '23514');
SELECT pg_temp.expect_error('an inferred edge without a derivation', $q$
  INSERT INTO forensic.relationship (tenant_id, collection_id, src_entity, dst_entity, rel_type, kind, directed)
  VALUES ('link-smoke', 'link-smoke-collection', '33000000-0000-4000-8000-0000000000e1', '33000000-0000-4000-8000-0000000000e2', 'shared_ip', 'inferred', false)$q$, '23514');
SELECT pg_temp.expect_error('co_located claimed as observed', $q$
  INSERT INTO forensic.relationship (tenant_id, collection_id, src_entity, dst_entity, rel_type, kind, directed)
  VALUES ('link-smoke', 'link-smoke-collection', '33000000-0000-4000-8000-0000000000e1', '33000000-0000-4000-8000-0000000000e3', 'co_located', 'observed', false)$q$, '23514');
SELECT pg_temp.expect_error('a face relationship before the case-authority policy', $q$
  INSERT INTO forensic.relationship (tenant_id, collection_id, src_entity, dst_entity, rel_type, kind, derivation_id)
  VALUES ('link-smoke', 'link-smoke-collection', '33000000-0000-4000-8000-0000000000e4', '33000000-0000-4000-8000-0000000000e3', 'face_candidate', 'candidate',
          '33000000-0000-4000-8000-0000000000d1')$q$, '23514');
SELECT pg_temp.expect_error('the same typed edge twice', $q$
  INSERT INTO forensic.relationship (tenant_id, collection_id, src_entity, dst_entity, rel_type, kind)
  VALUES ('link-smoke', 'link-smoke-collection', '33000000-0000-4000-8000-0000000000e1', '33000000-0000-4000-8000-0000000000e2', 'called', 'observed')$q$, '23505');
SELECT pg_temp.expect_error('an edge to an entity of another tenant', $q$
  INSERT INTO forensic.relationship (tenant_id, collection_id, src_entity, dst_entity, rel_type, kind)
  VALUES ('link-smoke-other', 'link-smoke-collection', '33000000-0000-4000-8000-0000000000e1', '33000000-0000-4000-8000-0000000000e2', 'called', 'observed')$q$, '23503');
SELECT pg_temp.expect_error('evidence for a source table that does not exist', $q$
  INSERT INTO forensic.relationship_evidence (rel_id, tenant_id, collection_id, source_table, source_key)
  VALUES ('33000000-0000-4000-8000-0000000000f1', 'link-smoke', 'link-smoke-collection', 'telepathy', '1')$q$, '23514');
SELECT pg_temp.expect_error('port range reversed in an assignment', $q$
  INSERT INTO forensic.ip_assignment (tenant_id, collection_id, public_ip, port_from, port_to, valid_from, valid_to, msisdn, source_key, derivation_id)
  VALUES ('link-smoke', 'link-smoke-collection', '203.0.113.9', 2000, 1000, '2026-01-05 11:00:00+00', '2026-01-05 11:05:00+00', '923000000001', 's2',
          '33000000-0000-4000-8000-0000000000d1')$q$, '23514');
SELECT pg_temp.expect_error('an assignment with no holder', $q$
  INSERT INTO forensic.ip_assignment (tenant_id, collection_id, public_ip, valid_from, valid_to, source_key, derivation_id)
  VALUES ('link-smoke', 'link-smoke-collection', '203.0.113.9', '2026-01-05 11:00:00+00', '2026-01-05 11:05:00+00', 's3',
          '33000000-0000-4000-8000-0000000000d1')$q$, '23514');

-- Evidence goes with its edge; merging keeps the merged entity.
DELETE FROM forensic.relationship WHERE rel_id = '33000000-0000-4000-8000-0000000000f1';
DO $$
BEGIN
  IF (SELECT count(*) FROM forensic.relationship_evidence) <> 0 THEN
    RAISE EXCEPTION 'evidence rows survived the deletion of their relationship';
  END IF;
END
$$;

UPDATE forensic.entity SET merged_into = '33000000-0000-4000-8000-0000000000e1' WHERE entity_id = '33000000-0000-4000-8000-0000000000e2';
DO $$
BEGIN
  IF (SELECT count(*) FROM forensic.entity WHERE entity_id = '33000000-0000-4000-8000-0000000000e2') <> 1 THEN
    RAISE EXCEPTION 'a merged entity was removed instead of kept';
  END IF;
END
$$;

-- Tenant isolation and the runtime role.
CREATE ROLE link_smoke_rls NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
GRANT USAGE ON SCHEMA forensic TO link_smoke_rls;
GRANT SELECT, INSERT ON forensic.derivation, forensic.entity, forensic.entity_mention, forensic.relationship,
  forensic.relationship_evidence, forensic.ip_assignment TO link_smoke_rls;

SET LOCAL ROLE link_smoke_rls;
SELECT set_config('app.tenant_id', 'link-smoke-other', true);
DO $$
BEGIN
  IF (SELECT count(*) FROM forensic.entity) <> 0 OR (SELECT count(*) FROM forensic.relationship) <> 0
     OR (SELECT count(*) FROM forensic.ip_assignment) <> 0 THEN
    RAISE EXCEPTION 'another tenant can see link-analysis rows';
  END IF;
END
$$;
SELECT pg_temp.expect_error('cross-tenant entity insert under row-level security', $q$
  INSERT INTO forensic.entity (tenant_id, collection_id, entity_type, canonical)
  VALUES ('link-smoke', 'link-smoke-collection', 'phone', '923000000009')$q$, '42501');
RESET ROLE;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    SET LOCAL ROLE forensic_runtime;
    PERFORM set_config('app.tenant_id', 'link-smoke', true);

    PERFORM pg_temp.expect_error('runtime role cannot change what an entity is', $q$
      UPDATE forensic.entity SET canonical = 'x' WHERE entity_id = '33000000-0000-4000-8000-0000000000e1'$q$, '42501');
    PERFORM pg_temp.expect_error('runtime role cannot retype an edge', $q$
      UPDATE forensic.relationship SET rel_type = 'texted' WHERE rel_id = '33000000-0000-4000-8000-0000000000f2'$q$, '42501');
    PERFORM pg_temp.expect_error('runtime role cannot delete an edge', $q$
      DELETE FROM forensic.relationship WHERE rel_id = '33000000-0000-4000-8000-0000000000f2'$q$, '42501');

    UPDATE forensic.relationship SET review_state = 'confirmed', reviewed_by = 'smoke-analyst', reviewed_at = now()
      WHERE rel_id = '33000000-0000-4000-8000-0000000000f2';
    IF (SELECT review_state FROM forensic.relationship WHERE rel_id = '33000000-0000-4000-8000-0000000000f2') <> 'confirmed' THEN
      RAISE EXCEPTION 'runtime role could not record an analyst review';
    END IF;
    RESET ROLE;
  END IF;
END
$$;

SELECT 'migration_014_smoke_passed' AS result;

ROLLBACK;
