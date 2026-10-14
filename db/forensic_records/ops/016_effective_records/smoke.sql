-- Synthetic acceptance checks for migration 016. DISPOSABLE DATABASE ONLY; one rolled-back transaction.
-- Proves: per evidence item the typed rows win and its version-1 rows are not counted twice; an item without typed rows keeps its version-1 rows;
-- repeats, other dataset kinds and other record types are not returned; the typed row is presented in the semantic layer's own field names with
-- the provider's columns winning where they exist; the view has the lineage columns the builders read; tenant isolation holds for the application role.
BEGIN;

SELECT set_config('app.tenant_id', 'eff-smoke', true);

INSERT INTO forensic.evidence_items (
  evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension,
  size_bytes, sha256, modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata
)
SELECT ('36000000-0000-4000-8000-00000000000' || n)::uuid, 'eff-smoke', 'eff-smoke-collection', 'PK-SYNTHETIC-EFF', 'eff-smoke-user',
       'export-' || n || '.xlsx', 'export-' || n || '.xlsx', 'application/octet-stream', 'xlsx', 16, repeat(n::text, 64),
       'structured_records', 'cdr', 1, 'forensic_records_worker', 'completed', 'immutable://eff-smoke/sha256/' || repeat(n::text, 64),
       jsonb_build_object('request_id', 'eff-smoke-request')
FROM generate_series(1, 4) AS n;

-- Evidence 1: typed rows (two kept, one flagged repeat) AND two version-1 rows.
INSERT INTO forensic.comm_event (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, layout_version,
  dataset_kind, event_type, direction, a_norm, b_norm, start_utc, end_utc, source_tz, time_basis, imsi, imei, lat, lon, raw_payload, quality_flags)
VALUES
  ('eff-smoke', 'eff-smoke-collection', '36000000-0000-4000-8000-000000000001', 'export-1.xlsx', '37000000-0000-4000-8000-000000000001', 1,
   repeat('a', 64), repeat('a', 64), 'generic-unmapped', 1, 'target_cdr', 'voice', 'outgoing', '923000000001', '923000000002',
   '2026-03-01 05:00:00+00', '2026-03-01 05:01:00+00', 'Asia/Karachi', 'declared', '001010000000001', '00000000000001', 33.5, 73.1,
   jsonb_build_object('A-Party', '03000000001', 'Call Type', 'Outgoing Call'), '{}'),
  ('eff-smoke', 'eff-smoke-collection', '36000000-0000-4000-8000-000000000001', 'export-1.xlsx', '37000000-0000-4000-8000-000000000001', 2,
   repeat('b', 64), repeat('b', 64), 'generic-unmapped', 1, 'target_cdr', 'sms', 'incoming', '923000000001', '923000000003',
   '2026-03-02 06:00:00+00', NULL, 'Asia/Karachi', 'declared', NULL, NULL, NULL, NULL,
   jsonb_build_object('CALL_TYPE', 'CALL', 'INBOUND_OUTBOUND_IND', 'INBOUND', 'lat', '31.5', 'longitude', '74.3'), '{}'),
  ('eff-smoke', 'eff-smoke-collection', '36000000-0000-4000-8000-000000000001', 'export-1.xlsx', '37000000-0000-4000-8000-000000000001', 3,
   repeat('c', 64), repeat('b', 64), 'generic-unmapped', 1, 'target_cdr', 'sms', 'incoming', '923000000001', '923000000003',
   '2026-03-02 06:00:00+00', NULL, 'Asia/Karachi', 'declared', NULL, NULL, NULL, NULL,
   jsonb_build_object('CALL_TYPE', 'CALL'), '{duplicate_row}');

INSERT INTO forensic.records (record_id, tenant_id, collection_id, file_id, batch_id, record_type, "timestamp", primary_target, secondary_target,
  source_file, row_number, row_hash, raw_payload, evidence_id)
VALUES
  ('38000000-0000-4000-8000-000000000011', 'eff-smoke', 'eff-smoke-collection', 'export-1.xlsx', '37000000-0000-4000-8000-000000000001', 'cdr',
   '2026-10-13 14:00:00+00', '923000000001', '03000000002', 'export-1.xlsx', 2, repeat('1', 64), '{"MSISDN": "v1-one"}', '36000000-0000-4000-8000-000000000001'),
  ('38000000-0000-4000-8000-000000000012', 'eff-smoke', 'eff-smoke-collection', 'export-1.xlsx', '37000000-0000-4000-8000-000000000001', 'cdr',
   '2026-10-13 14:00:00+00', '923000000001', '03000000003', 'export-1.xlsx', 3, repeat('2', 64), '{"MSISDN": "v1-two"}', '36000000-0000-4000-8000-000000000001'),
-- Evidence 2: version-1 rows only (kept).
  ('38000000-0000-4000-8000-000000000021', 'eff-smoke', 'eff-smoke-collection', 'export-2.xlsx', '37000000-0000-4000-8000-000000000002', 'cdr',
   '2026-02-01 10:00:00+00', '923000000009', '923000000008', 'export-2.xlsx', 2, repeat('3', 64), '{"MSISDN": "923000000009"}', '36000000-0000-4000-8000-000000000002'),
  ('38000000-0000-4000-8000-000000000022', 'eff-smoke', 'eff-smoke-collection', 'export-2.xlsx', '37000000-0000-4000-8000-000000000002', 'cdr',
   '2026-02-02 10:00:00+00', '923000000009', '923000000007', 'export-2.xlsx', 3, repeat('4', 64), '{"MSISDN": "923000000009"}', '36000000-0000-4000-8000-000000000002'),
-- Evidence 4: a version-1 row of ANOTHER record type (never returned by the call-record view).
  ('38000000-0000-4000-8000-000000000041', 'eff-smoke', 'eff-smoke-collection', 'export-4.xlsx', '37000000-0000-4000-8000-000000000004', 'generic',
   '2026-02-01 10:00:00+00', NULL, NULL, 'export-4.xlsx', 2, repeat('5', 64), '{}', '36000000-0000-4000-8000-000000000004');

-- Evidence 3: typed rows only, one call record and one tower-dump row (another dataset kind: never returned).
INSERT INTO forensic.comm_event (tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, layout_version,
  dataset_kind, event_type, direction, a_norm, b_norm, start_utc, end_utc, source_tz, time_basis, raw_payload, quality_flags)
VALUES
  ('eff-smoke', 'eff-smoke-collection', '36000000-0000-4000-8000-000000000003', 'export-3.xlsx', '37000000-0000-4000-8000-000000000003', 1,
   repeat('d', 64), repeat('d', 64), 'generic-unmapped', 1, 'target_cdr', 'data', 'unknown', '923000000005', NULL,
   '2026-04-01 00:00:00+00', NULL, 'Asia/Karachi', 'confirmed_by_analyst',
   jsonb_build_object('CALL_DIALED_NUM', 'RAW-LEAK-1', 'target_number', 'RAW-LEAK-2', 'call_time', 'RAW-LEAK-3', 'IMSI', 'RAW-LEAK-4', 'call_org_num', 'KEPT-RAW'), '{}'),
  ('eff-smoke', 'eff-smoke-collection', '36000000-0000-4000-8000-000000000003', 'export-3.xlsx', '37000000-0000-4000-8000-000000000003', 2,
   repeat('e', 64), repeat('e', 64), 'generic-unmapped', 1, 'tower_dump', 'voice', 'outgoing', '923000000005', '923000000006',
   '2026-04-01 01:00:00+00', NULL, 'Asia/Karachi', 'confirmed_by_analyst', '{}', '{}');

DO $$
DECLARE
  n bigint;
  r record;
  runtime_exists boolean := EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime');
BEGIN
  -- Rows: evidence 1 -> 2 typed (the repeat is excluded, its version-1 rows are NOT added); evidence 2 -> 2 version-1; evidence 3 -> 1 (tower dump excluded).
  SELECT count(*) INTO n FROM forensic.records_effective_cdr WHERE tenant_id = 'eff-smoke';
  IF n <> 5 THEN RAISE EXCEPTION 'smoke: expected 5 effective rows, got %', n; END IF;

  SELECT count(*) INTO n FROM forensic.records_effective_cdr WHERE evidence_id = '36000000-0000-4000-8000-000000000001' AND metadata ? 'typed_source';
  IF n <> 2 THEN RAISE EXCEPTION 'smoke: evidence 1 should be served by 2 typed rows, got %', n; END IF;
  SELECT count(*) INTO n FROM forensic.records_effective_cdr WHERE record_id IN ('38000000-0000-4000-8000-000000000011', '38000000-0000-4000-8000-000000000012');
  IF n <> 0 THEN RAISE EXCEPTION 'smoke: the version-1 rows of an item that has typed rows were counted as well (%)', n; END IF;

  SELECT count(*) INTO n FROM forensic.records_effective_cdr WHERE evidence_id = '36000000-0000-4000-8000-000000000002' AND NOT (metadata ? 'typed_source');
  IF n <> 2 THEN RAISE EXCEPTION 'smoke: evidence 2 should keep its 2 version-1 rows, got %', n; END IF;
  SELECT count(*) INTO n FROM forensic.records_effective_cdr WHERE record_id = '38000000-0000-4000-8000-000000000021' AND raw_payload = '{"MSISDN": "923000000009"}'::jsonb
    AND primary_target = '923000000009' AND "timestamp" = '2026-02-01 10:00:00+00' AND row_number = 2;
  IF n <> 1 THEN RAISE EXCEPTION 'smoke: a version-1 row must come through unchanged'; END IF;

  IF EXISTS (SELECT 1 FROM forensic.records_effective_cdr WHERE evidence_id = '36000000-0000-4000-8000-000000000004') THEN
    RAISE EXCEPTION 'smoke: a version-1 row of another record type was returned';
  END IF;

  -- The first typed row, in the semantic layer's names.
  SELECT * INTO r FROM forensic.records_effective_cdr WHERE evidence_id = '36000000-0000-4000-8000-000000000001' AND row_hash = repeat('a', 64);
  IF r.record_type <> 'cdr' OR r."timestamp" <> '2026-03-01 05:00:00+00' OR r.primary_target <> '923000000001' OR r.secondary_target <> '923000000002'
     OR r.source_file <> 'export-1.xlsx' OR r.row_number <> 2 OR r.record_id IS NULL OR r.batch_id <> '37000000-0000-4000-8000-000000000001' THEN
    RAISE EXCEPTION 'smoke: the typed row has the wrong top-level columns: %', to_jsonb(r);
  END IF;
  IF r.raw_payload->>'MSISDN' <> '923000000001' OR r.raw_payload->>'CALL_DIALED_NUM' <> '923000000002'
     OR r.raw_payload->>'CALL_START_DT_TM' <> '2026-03-01T05:00:00+00:00' OR r.raw_payload->>'CALL_END_DT_TM' <> '2026-03-01T05:01:00+00:00'
     OR r.raw_payload->>'IMSI' <> '001010000000001' OR r.raw_payload->>'IMEI' <> '00000000000001' THEN
    RAISE EXCEPTION 'smoke: the typed row is not presented in the semantic layer''s names: %', r.raw_payload;
  END IF;
  IF r.raw_payload->>'CALL_TYPE' <> 'VOICE' OR r.raw_payload->>'INBOUND_OUTBOUND_IND' <> 'OUTGOING' OR r.raw_payload->>'lat' <> '33.5' OR r.raw_payload->>'longitude' <> '73.1' THEN
    RAISE EXCEPTION 'smoke: the derived call type, direction and coordinates are missing where the provider had none: %', r.raw_payload;
  END IF;
  IF r.raw_payload->>'A-Party' <> '03000000001' OR r.raw_payload->>'Call Type' <> 'Outgoing Call' THEN
    RAISE EXCEPTION 'smoke: the provider''s own columns must stay in the payload';
  END IF;
  IF r.metadata->'typed_source'->>'layout_id' <> 'generic-unmapped' OR r.metadata->'normalized_fields'->>'MSISDN' <> '923000000001'
     OR r.metadata->'typed_source'->>'time_basis' <> 'declared' THEN
    RAISE EXCEPTION 'smoke: the metadata does not carry the typed lineage: %', r.metadata;
  END IF;

  -- The provider's own call type, direction and coordinates win; a missing end time leaves no empty key.
  SELECT * INTO r FROM forensic.records_effective_cdr WHERE evidence_id = '36000000-0000-4000-8000-000000000001' AND row_hash = repeat('b', 64);
  IF r.raw_payload->>'CALL_TYPE' <> 'CALL' OR r.raw_payload->>'INBOUND_OUTBOUND_IND' <> 'INBOUND' OR r.raw_payload->>'lat' <> '31.5' OR r.raw_payload->>'longitude' <> '74.3' THEN
    RAISE EXCEPTION 'smoke: the provider''s own values must win over the derived ones: %', r.raw_payload;
  END IF;
  IF r.raw_payload ? 'CALL_END_DT_TM' OR r.raw_payload ? 'IMSI' THEN
    RAISE EXCEPTION 'smoke: an absent value must be an absent key, not a null: %', r.raw_payload;
  END IF;

  -- An item with typed rows only.
  SELECT * INTO r FROM forensic.records_effective_cdr WHERE evidence_id = '36000000-0000-4000-8000-000000000003';
  IF r.raw_payload->>'CALL_TYPE' <> 'GPRS' OR r.raw_payload ? 'INBOUND_OUTBOUND_IND' OR r.raw_payload ? 'CALL_DIALED_NUM' THEN
    RAISE EXCEPTION 'smoke: data sessions map to GPRS and an unknown direction or counterparty stays absent: %', r.raw_payload;
  END IF;
  IF r.raw_payload ? 'target_number' OR r.raw_payload ? 'call_time' OR r.raw_payload ? 'IMSI' OR r.raw_payload::text LIKE '%RAW-LEAK%' THEN
    RAISE EXCEPTION 'smoke: the provider''s own value of a field the typed row owns leaked through (a counterparty, time or identifier the typed ingest left empty): %', r.raw_payload;
  END IF;
  IF r.raw_payload->>'call_org_num' <> 'KEPT-RAW' THEN
    RAISE EXCEPTION 'smoke: a provider column the typed row does not own must stay: %', r.raw_payload;
  END IF;

  -- Tenant isolation as the application role: the view runs with the caller's privileges and row-level security.
  IF runtime_exists THEN
    SET LOCAL ROLE forensic_runtime;
    PERFORM set_config('app.tenant_id', 'eff-smoke', true);
    SELECT count(*) INTO n FROM forensic.records_effective_cdr;
    IF n <> 5 THEN RAISE EXCEPTION 'smoke: the application role should see 5 rows of its own tenant, saw %', n; END IF;
    PERFORM set_config('app.tenant_id', 'another-tenant', true);
    SELECT count(*) INTO n FROM forensic.records_effective_cdr;
    IF n <> 0 THEN RAISE EXCEPTION 'smoke: another tenant must see no row, saw %', n; END IF;
    PERFORM set_config('app.tenant_id', '', true);
    SELECT count(*) INTO n FROM forensic.records_effective_cdr;
    IF n <> 0 THEN RAISE EXCEPTION 'smoke: no tenant setting must see no row, saw %', n; END IF;
    RESET ROLE;
  END IF;
END
$$;

SELECT 'migration_016_smoke_passed' AS result;

ROLLBACK;
