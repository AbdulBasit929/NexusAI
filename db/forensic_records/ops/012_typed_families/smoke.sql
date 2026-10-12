-- Synthetic acceptance checks for migration 012. DISPOSABLE DATABASE ONLY: run it after 012 and the verify script.
-- Everything happens inside one transaction that is rolled back, but it still must never be pointed at a retained
-- registry (it inserts synthetic evidence rows, which fire the Phase 3 synchronisation triggers).
--
-- Proves: valid rows load into every typed table; every integrity rule rejects what it should; the plate search key
-- works; tenant isolation holds for a non-owner role; the runtime role cannot update, delete or read identity data.
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

SELECT set_config('app.tenant_id', 'typed-smoke', true);

-- Two synthetic tenants, one evidence item each (legacy style, as in the 008 legacy fixture).
INSERT INTO forensic.evidence_items (
  evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension,
  size_bytes, sha256, modality, detected_type, classifier_confidence, processing_route, processing_status,
  raw_storage_ref, metadata
) VALUES
  ('32000000-0000-4000-8000-000000000001', 'typed-smoke', 'typed-smoke-collection', 'PK-SYNTHETIC-TYPED', 'typed-smoke-user',
   'synthetic-a.csv', 'synthetic-a.csv', 'text/csv', 'csv', 64, repeat('d', 64), 'structured_records', 'cdr', 1,
   'forensic_records_worker', 'completed', 'immutable://typed-smoke/sha256/' || repeat('d', 64),
   jsonb_build_object('request_id', 'typed-smoke-request')),
  ('32000000-0000-4000-8000-000000000002', 'typed-smoke-other', 'typed-smoke-other-collection', 'PK-SYNTHETIC-TYPED', 'typed-smoke-user',
   'synthetic-b.csv', 'synthetic-b.csv', 'text/csv', 'csv', 64, repeat('e', 64), 'structured_records', 'cdr', 1,
   'forensic_records_worker', 'completed', 'immutable://typed-smoke-other/sha256/' || repeat('e', 64),
   jsonb_build_object('request_id', 'typed-smoke-other-request'));

INSERT INTO forensic.layout_versions (layout_id, layout_version, family, dataset_kind, operator_label, definition_sha256, registered_by)
VALUES ('smoke-layout', 1, 'communications', 'target_cdr', 'synthetic', repeat('1', 64), 'smoke');

-- Valid rows, one per typed table. Numbers are the synthetic ones used across the repository fixtures; addresses are
-- from the documentation ranges (RFC 5737, RFC 3849).
INSERT INTO forensic.comm_event (
  tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, layout_version,
  event_type, direction, a_raw, a_norm, a_kind, b_raw, b_norm, b_kind, start_utc, end_utc, duration_s, source_tz, time_basis,
  cell_key, raw_payload
) VALUES (
  'typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a', '32000000-0000-4000-8000-0000000000a1', 2,
  'h-event-1', 'c-event-1', 'smoke-layout', 1, 'voice', 'outgoing', '03000000001', '923000000001', 'person_like',
  '03000000002', '923000000002', 'person_like', '2026-01-05 10:00:00+00', '2026-01-05 10:01:30+00', 90, 'Asia/Karachi',
  'declared', '001-01:100:200', '{"synthetic": true}'
);

INSERT INTO forensic.data_session (
  tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, layout_version,
  start_utc, end_utc, source_tz, time_basis, msisdn, ue_ip, public_ip, public_port_from, public_port_to, server_ip, server_port,
  protocol_num, protocol, bytes_up, bytes_down, raw_payload
) VALUES (
  'typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a', '32000000-0000-4000-8000-0000000000a1', 3,
  'h-session-1', 'c-session-1', 'smoke-layout', 1, '2026-01-05 11:00:00+00', '2026-01-05 11:05:00+00', 'Asia/Karachi', 'declared',
  '923000000001', '100.64.0.7', '203.0.113.9', 1024, 2047, '198.51.100.20', 443, 6, 'tcp', 1200, 56000, '{"synthetic": true}'
), (
  'typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a', '32000000-0000-4000-8000-0000000000a1', 4,
  'h-session-2', 'c-session-2', 'smoke-layout', 1, '2026-01-05 11:10:00+00', NULL, 'Asia/Karachi', 'declared',
  '923000000001', NULL, NULL, NULL, NULL, '2001:db8::10', 443, 17, 'udp', 0, 10, '{"synthetic": true}'
);

INSERT INTO forensic.app_usage (
  tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, layout_version,
  start_utc, source_tz, time_basis, msisdn, app_category, app, province, city, raw_payload
) VALUES (
  'typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a', '32000000-0000-4000-8000-0000000000a1', 5,
  'h-app-1', 'c-app-1', 'smoke-layout', 1, '2026-01-05 12:00:00+00', 'Asia/Karachi', 'assumed_by_profile', '923000000001',
  'messaging', 'synthetic-app', 'Synthetic Province', 'Synthetic City', '{"synthetic": true}'
);

INSERT INTO forensic.plate_sighting (
  tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, layout_version,
  source_kind, plate_raw, plate_norm, plate_script, observed_utc, time_basis, camera_id, lat, lon, loc_method, confidence,
  review_state
) VALUES (
  'typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a', '32000000-0000-4000-8000-0000000000a1', 6,
  'h-plate-1', 'c-plate-1', 'smoke-layout', 1, 'csv', 'SYN-1234', forensic.normalize_plate('SYN-1234'), 'latin',
  '2026-01-05 13:00:00+00', 'declared', 'camera-1', 33.0, 73.0, 'source_supplied', 0.9, 'unreviewed'
);

INSERT INTO forensic.subscriber_identity (identity_ref, tenant_id, collection_id, evidence_id, source_row, full_name, cnic)
VALUES ('32000000-0000-4000-8000-0000000000b1', 'typed-smoke', 'typed-smoke-collection',
        '32000000-0000-4000-8000-000000000001', 7, 'Synthetic Person', '0000000000000');

INSERT INTO forensic.subscriber_registration (
  tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, layout_id, layout_version, msisdn,
  status, time_basis, identity_ref
) VALUES (
  'typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a', '32000000-0000-4000-8000-0000000000a1', 7,
  'h-sub-1', 'smoke-layout', 1, '923000000001', 'active', 'declared', '32000000-0000-4000-8000-0000000000b1'
);

DO $$
DECLARE
  n bigint;
BEGIN
  SELECT count(*) INTO n FROM forensic.comm_event;
  IF n <> 1 THEN RAISE EXCEPTION 'expected 1 comm_event, found %', n; END IF;
  SELECT count(*) INTO n FROM forensic.data_session;
  IF n <> 2 THEN RAISE EXCEPTION 'expected 2 data_session rows, found %', n; END IF;
  SELECT count(*) INTO n FROM forensic.app_usage;
  IF n <> 1 THEN RAISE EXCEPTION 'expected 1 app_usage, found %', n; END IF;
  SELECT count(*) INTO n FROM forensic.plate_sighting WHERE plate_norm = 'SYN1234';
  IF n <> 1 THEN RAISE EXCEPTION 'expected 1 normalised plate sighting, found %', n; END IF;
  SELECT count(*) INTO n FROM forensic.subscriber_registration;
  IF n <> 1 THEN RAISE EXCEPTION 'expected 1 subscriber_registration, found %', n; END IF;
END
$$;

-- Integrity rules: each of these must be rejected with the stated SQLSTATE.
-- A common prefix for event inserts keeps the statements short.
SELECT pg_temp.expect_error('duplicate row in the same file and time', $q$
  INSERT INTO forensic.comm_event (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, event_type, start_utc, source_tz, time_basis, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 2, 'h-event-1', 'c-event-1', 'smoke-layout', 1, 'voice',
    '2026-01-05 10:00:00+00', 'Asia/Karachi', 'declared', '{}')$q$, '23505');

SELECT pg_temp.expect_error('unknown direction value', $q$
  INSERT INTO forensic.comm_event (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, event_type, direction, start_utc, source_tz, time_basis, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-1', 'c-bad-1', 'smoke-layout', 1, 'voice', 'sideways',
    '2026-01-05 10:00:00+00', 'Asia/Karachi', 'declared', '{}')$q$, '23514');

SELECT pg_temp.expect_error('negative duration', $q$
  INSERT INTO forensic.comm_event (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, event_type, duration_s, start_utc, source_tz, time_basis, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-2', 'c-bad-2', 'smoke-layout', 1, 'voice', -1,
    '2026-01-05 10:00:00+00', 'Asia/Karachi', 'declared', '{}')$q$, '23514');

SELECT pg_temp.expect_error('latitude without longitude', $q$
  INSERT INTO forensic.comm_event (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, event_type, lat, start_utc, source_tz, time_basis, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-3', 'c-bad-3', 'smoke-layout', 1, 'voice', 33.0,
    '2026-01-05 10:00:00+00', 'Asia/Karachi', 'declared', '{}')$q$, '23514');

SELECT pg_temp.expect_error('time basis missing from the allowed set', $q$
  INSERT INTO forensic.comm_event (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, event_type, start_utc, source_tz, time_basis, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-4', 'c-bad-4', 'smoke-layout', 1, 'voice',
    '2026-01-05 10:00:00+00', 'Asia/Karachi', 'guess', '{}')$q$, '23514');

SELECT pg_temp.expect_error('raw payload that is not an object', $q$
  INSERT INTO forensic.comm_event (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, event_type, start_utc, source_tz, time_basis, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-5', 'c-bad-5', 'smoke-layout', 1, 'voice',
    '2026-01-05 10:00:00+00', 'Asia/Karachi', 'declared', '[]')$q$, '23514');

SELECT pg_temp.expect_error('unregistered layout version', $q$
  INSERT INTO forensic.comm_event (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, event_type, start_utc, source_tz, time_basis, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-6', 'c-bad-6', 'smoke-layout', 99, 'voice',
    '2026-01-05 10:00:00+00', 'Asia/Karachi', 'declared', '{}')$q$, '23503');

SELECT pg_temp.expect_error('evidence of another tenant', $q$
  INSERT INTO forensic.comm_event (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, event_type, start_utc, source_tz, time_basis, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000002', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-7', 'c-bad-7', 'smoke-layout', 1, 'voice',
    '2026-01-05 10:00:00+00', 'Asia/Karachi', 'declared', '{}')$q$, '23503');

SELECT pg_temp.expect_error('port out of range', $q$
  INSERT INTO forensic.data_session (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, start_utc, source_tz, time_basis, ue_port, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-8', 'c-bad-8', 'smoke-layout', 1, '2026-01-05 11:00:00+00',
    'Asia/Karachi', 'declared', 70000, '{}')$q$, '23514');

SELECT pg_temp.expect_error('public port range reversed', $q$
  INSERT INTO forensic.data_session (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, start_utc, source_tz, time_basis, public_port_from, public_port_to, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-9', 'c-bad-9', 'smoke-layout', 1, '2026-01-05 11:00:00+00',
    'Asia/Karachi', 'declared', 2000, 1000, '{}')$q$, '23514');

SELECT pg_temp.expect_error('IP address that is not an address', $q$
  INSERT INTO forensic.data_session (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, start_utc, source_tz, time_basis, ue_ip, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-10', 'c-bad-10', 'smoke-layout', 1, '2026-01-05 11:00:00+00',
    'Asia/Karachi', 'declared', '999.1.1.1', '{}')$q$, '22P02');

SELECT pg_temp.expect_error('plate confidence above one', $q$
  INSERT INTO forensic.plate_sighting (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, source_kind, plate_raw, plate_norm, observed_utc, time_basis, confidence)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-11', 'c-bad-11', 'smoke-layout', 1, 'csv', 'X', 'X',
    '2026-01-05 13:00:00+00', 'declared', 1.5)$q$, '23514');

SELECT pg_temp.expect_error('plate source kind outside the set', $q$
  INSERT INTO forensic.plate_sighting (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, source_kind, plate_raw, plate_norm, observed_utc, time_basis)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-12', 'c-bad-12', 'smoke-layout', 1, 'telepathy', 'X', 'X',
    '2026-01-05 13:00:00+00', 'declared')$q$, '23514');

SELECT pg_temp.expect_error('registration that ends before it starts', $q$
  INSERT INTO forensic.subscriber_registration (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash,
    layout_id, layout_version, msisdn, time_basis, valid_from, valid_to)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-bad-13', 'smoke-layout', 1, '923000000001', 'declared',
    '2026-02-01 00:00:00+00', '2026-01-01 00:00:00+00')$q$, '23514');

SELECT pg_temp.expect_error('identity from another tenant', $q$
  INSERT INTO forensic.subscriber_identity (tenant_id, collection_id, evidence_id, source_row, full_name)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000002', 9, 'Synthetic Person')$q$, '23503');

-- Tenant isolation for a non-owner role that has no BYPASSRLS, with the same grants the migration gives forensic_runtime.
CREATE ROLE typed_smoke_rls NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
GRANT USAGE ON SCHEMA forensic TO typed_smoke_rls;
GRANT SELECT, INSERT ON forensic.comm_event, forensic.data_session, forensic.app_usage, forensic.plate_sighting,
  forensic.subscriber_registration TO typed_smoke_rls;
GRANT SELECT ON forensic.layout_versions, forensic.evidence_items TO typed_smoke_rls;

SET LOCAL ROLE typed_smoke_rls;
SELECT set_config('app.tenant_id', 'typed-smoke-other', true);

DO $$
DECLARE
  n bigint;
BEGIN
  SELECT count(*) INTO n FROM forensic.comm_event;
  IF n <> 0 THEN RAISE EXCEPTION 'another tenant can see % comm_event rows', n; END IF;
  SELECT count(*) INTO n FROM forensic.data_session;
  IF n <> 0 THEN RAISE EXCEPTION 'another tenant can see % data_session rows', n; END IF;
  SELECT count(*) INTO n FROM forensic.plate_sighting;
  IF n <> 0 THEN RAISE EXCEPTION 'another tenant can see % plate_sighting rows', n; END IF;
  SELECT count(*) INTO n FROM forensic.subscriber_registration;
  IF n <> 0 THEN RAISE EXCEPTION 'another tenant can see % subscriber_registration rows', n; END IF;
END
$$;

SELECT pg_temp.expect_error('cross-tenant insert under row-level security', $q$
  INSERT INTO forensic.app_usage (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash,
    layout_id, layout_version, start_utc, source_tz, time_basis, raw_payload)
  VALUES ('typed-smoke', 'typed-smoke-collection', '32000000-0000-4000-8000-000000000001', 'file-a',
    '32000000-0000-4000-8000-0000000000a1', 9, 'h-rls-1', 'c-rls-1', 'smoke-layout', 1, '2026-01-05 12:00:00+00',
    'Asia/Karachi', 'declared', '{}')$q$, '42501');

RESET ROLE;

-- The runtime role itself, when it exists in this disposable database.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    SET LOCAL ROLE forensic_runtime;
    PERFORM set_config('app.tenant_id', 'typed-smoke', true);

    PERFORM pg_temp.expect_error('runtime role cannot read identity data', 'SELECT count(*) FROM forensic.subscriber_identity', '42501');
    PERFORM pg_temp.expect_error('runtime role cannot update events', 'UPDATE forensic.comm_event SET duration_s = 1', '42501');
    PERFORM pg_temp.expect_error('runtime role cannot delete events', 'DELETE FROM forensic.comm_event', '42501');

    IF (SELECT count(*) FROM forensic.comm_event) <> 1 THEN
      RAISE EXCEPTION 'runtime role cannot read its own tenant events';
    END IF;
    RESET ROLE;
  END IF;
END
$$;

SELECT 'migration_012_smoke_passed' AS result;

ROLLBACK;
