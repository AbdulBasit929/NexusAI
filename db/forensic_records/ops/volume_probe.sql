-- Synthetic volume probe for migrations 012 to 015. DISPOSABLE DATABASE ONLY: it commits its rows (so sizes are real) and is run
-- by Test-MigrationsThrowaway.ps1 -VolumeRows N, never against a retained database.
--   psql -v ON_ERROR_STOP=1 -v rows=500000 -f volume_probe.sql
-- Measures: load time for comm_event and data_session with every index in place, table and index size, and the time of the
-- queries the first link analyses need (contact profile, common contacts, co-location, tower dump, IP attribution), both as the
-- owner and as the non-owner runtime role under row-level security. Numbers are for orientation on one laptop with default
-- memory settings; they are not the pre-registered performance budget.
\if :{?rows}
\else
  \set rows 500000
\endif
\timing on
\pset pager off

SELECT set_config('app.tenant_id', 'vol', false);

INSERT INTO forensic.evidence_items (
  evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file, content_type, extension,
  size_bytes, sha256, modality, detected_type, classifier_confidence, processing_route, processing_status, raw_storage_ref, metadata
) VALUES (
  '35000000-0000-4000-8000-000000000001', 'vol', 'vol-c', 'PK-SYNTHETIC-VOL', 'vol-user', 'volume.csv', 'volume.csv', 'text/csv',
  'csv', 64, repeat('c', 64), 'structured_records', 'cdr', 1, 'forensic_records_worker', 'completed',
  'immutable://vol/sha256/' || repeat('c', 64), jsonb_build_object('request_id', 'vol-request'));

INSERT INTO forensic.layout_versions (layout_id, layout_version, family, dataset_kind, operator_label, definition_sha256, registered_by)
VALUES ('vol-layout', 1, 'communications', 'target_cdr', 'synthetic', repeat('2', 64), 'volume-probe');

\echo === load comm_event (:rows rows, 8 indexes, row-level security enabled but bypassed by the owner)
INSERT INTO forensic.comm_event (
  tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, layout_version,
  event_type, direction, a_raw, a_norm, a_kind, b_raw, b_norm, b_kind, start_utc, end_utc, duration_s, source_tz, time_basis,
  imsi, imei, cell_key, raw_payload)
SELECT 'vol', 'vol-c', '35000000-0000-4000-8000-000000000001', 'file-1', '35000000-0000-4000-8000-0000000000a1', g,
       md5(g::text), md5('c' || g::text), 'vol-layout', 1,
       'voice', CASE WHEN g % 2 = 0 THEN 'incoming' ELSE 'outgoing' END,
       '0' || substr('9230000' || lpad((g % 2000)::text, 5, '0'), 3), '9230000' || lpad((g % 2000)::text, 5, '0'), 'person_like',
       '0' || substr('9230000' || lpad(((g::bigint * 7919) % 20000)::text, 5, '0'), 3), '9230000' || lpad(((g::bigint * 7919) % 20000)::text, 5, '0'), 'person_like',
       timestamptz '2026-01-01 00:00:00+00' + g * interval '15 seconds',
       timestamptz '2026-01-01 00:00:00+00' + g * interval '15 seconds' + interval '60 seconds', 60,
       'Asia/Karachi', 'declared',
       '00101' || lpad((g % 2000)::text, 10, '0'), '00' || lpad((g % 3000)::text, 12, '0') || '0',
       forensic.make_cell_key('00101', '100', (g % 5000)::text),
       jsonb_build_object('row', g, 'pad', repeat('x', 200))
FROM generate_series(1, :rows) AS g;

\echo === load data_session (rows / 2)
INSERT INTO forensic.data_session (
  tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, layout_version,
  start_utc, end_utc, nat_start_utc, nat_end_utc, source_tz, time_basis, msisdn, ue_ip, public_ip, public_port_from, public_port_to,
  server_ip, server_port, protocol_num, protocol, bytes_up, bytes_down, raw_payload)
SELECT 'vol', 'vol-c', '35000000-0000-4000-8000-000000000001', 'file-2', '35000000-0000-4000-8000-0000000000a2', g,
       md5('s' || g::text), md5('sc' || g::text), 'vol-layout', 1,
       timestamptz '2026-01-01 00:00:00+00' + g * interval '30 seconds', timestamptz '2026-01-01 00:00:00+00' + g * interval '30 seconds' + interval '120 seconds',
       timestamptz '2026-01-01 00:00:00+00' + g * interval '30 seconds', timestamptz '2026-01-01 00:00:00+00' + g * interval '30 seconds' + interval '120 seconds',
       'Asia/Karachi', 'declared', '9230000' || lpad((g % 2000)::text, 5, '0'),
       ('100.64.' || (g % 250) || '.' || ((g / 250) % 250 + 1))::inet, ('203.0.113.' || (g % 250 + 1))::inet,
       1024 + (g % 60) * 1000, 1024 + (g % 60) * 1000 + 999,
       ('198.51.100.' || (g % 200 + 1))::inet, 443, 6, 'tcp', g % 100000, g % 900000,
       jsonb_build_object('row', g, 'pad', repeat('y', 200))
FROM generate_series(1, :rows / 2) AS g;

ANALYZE forensic.comm_event;
ANALYZE forensic.data_session;

\echo === sizes (heap, indexes, TOAST, total) and bytes per row
SELECT 'comm_event' AS table_name, (SELECT count(*) FROM forensic.comm_event) AS rows,
       pg_size_pretty(d.table_bytes) AS heap, pg_size_pretty(d.index_bytes) AS indexes, pg_size_pretty(d.toast_bytes) AS toast,
       pg_size_pretty(d.total_bytes) AS total, (d.total_bytes / greatest(1, (SELECT count(*) FROM forensic.comm_event)))::int AS bytes_per_row
FROM hypertable_detailed_size('forensic.comm_event') d
UNION ALL
SELECT 'data_session', (SELECT count(*) FROM forensic.data_session),
       pg_size_pretty(d.table_bytes), pg_size_pretty(d.index_bytes), pg_size_pretty(d.toast_bytes), pg_size_pretty(d.total_bytes),
       (d.total_bytes / greatest(1, (SELECT count(*) FROM forensic.data_session)))::int
FROM hypertable_detailed_size('forensic.data_session') d;

\echo === index sizes summed over chunks
SELECT regexp_replace(c.relname, '^_hyper_[0-9]+_[0-9]+_chunk_', '') AS index_name, pg_size_pretty(sum(pg_relation_size(i.indexrelid))::bigint) AS size
FROM pg_index i
JOIN pg_class c ON c.oid = i.indexrelid
JOIN pg_class t ON t.oid = i.indrelid
JOIN pg_namespace n ON n.oid = t.relnamespace
WHERE n.nspname = '_timescaledb_internal' AND t.relname ~ '^_hyper_[0-9]+_[0-9]+_chunk$'
GROUP BY 1 ORDER BY sum(pg_relation_size(i.indexrelid)) DESC;

\echo === queries as the owner (each runs twice; the second time is the warm one)
\echo --- A1 contact profile of one number (top 20 counterparties)
SELECT count(*) FROM (SELECT b_norm, count(*) AS c, sum(duration_s) FROM forensic.comm_event
  WHERE tenant_id = 'vol' AND collection_id = 'vol-c' AND a_norm = '923000000007' GROUP BY b_norm ORDER BY c DESC LIMIT 20) q;
SELECT count(*) FROM (SELECT b_norm, count(*) AS c, sum(duration_s) FROM forensic.comm_event
  WHERE tenant_id = 'vol' AND collection_id = 'vol-c' AND a_norm = '923000000007' GROUP BY b_norm ORDER BY c DESC LIMIT 20) q;
\echo --- A1 contact profile, last 30 days only
SELECT count(*) FROM (SELECT b_norm, count(*) AS c FROM forensic.comm_event
  WHERE tenant_id = 'vol' AND collection_id = 'vol-c' AND a_norm = '923000000007' AND start_utc >= timestamptz '2026-02-15 00:00:00+00'
    AND start_utc < timestamptz '2026-03-17 00:00:00+00' GROUP BY b_norm ORDER BY c DESC LIMIT 20) q;
\echo --- A2 common contacts of two numbers
SELECT count(*) FROM (
  SELECT b_norm FROM forensic.comm_event WHERE tenant_id = 'vol' AND collection_id = 'vol-c' AND a_norm = '923000000007'
  INTERSECT
  SELECT b_norm FROM forensic.comm_event WHERE tenant_id = 'vol' AND collection_id = 'vol-c' AND a_norm = '923000000011') q;
\echo --- A5 co-location: events of two numbers in the same cell within ten minutes
SELECT count(*) FROM forensic.comm_event x JOIN forensic.comm_event y
  ON y.tenant_id = x.tenant_id AND y.collection_id = x.collection_id AND y.cell_key = x.cell_key
 AND y.start_utc BETWEEN x.start_utc - interval '10 minutes' AND x.start_utc + interval '10 minutes'
WHERE x.tenant_id = 'vol' AND x.collection_id = 'vol-c' AND x.a_norm = '923000000007' AND y.a_norm = '923000000011';
\echo --- A6 tower dump: distinct numbers seen in one cell during a day
SELECT count(DISTINCT a_norm) FROM forensic.comm_event WHERE tenant_id = 'vol' AND collection_id = 'vol-c'
  AND cell_key = '00101:100:42' AND start_utc >= timestamptz '2026-02-01 00:00:00+00' AND start_utc < timestamptz '2026-02-02 00:00:00+00';
\echo --- A13 IP attribution: who held a public address and port at a time
SELECT count(*) FROM forensic.data_session WHERE tenant_id = 'vol' AND collection_id = 'vol-c' AND public_ip = '203.0.113.7'
  AND 1500 BETWEEN public_port_from AND public_port_to
  AND timestamptz '2026-01-20 12:00:00+00' BETWEEN nat_start_utc AND nat_end_utc;
\echo --- plain count (reference point)
SELECT count(*) FROM forensic.comm_event WHERE tenant_id = 'vol' AND collection_id = 'vol-c';

\echo === the same contact profile as the non-owner runtime role under row-level security
DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN CREATE ROLE forensic_runtime LOGIN; GRANT SELECT ON forensic.comm_event TO forensic_runtime; GRANT USAGE ON SCHEMA forensic TO forensic_runtime; END IF; END $$;
SET ROLE forensic_runtime;
SELECT count(*) FROM (SELECT b_norm, count(*) AS c, sum(duration_s) FROM forensic.comm_event
  WHERE tenant_id = 'vol' AND collection_id = 'vol-c' AND a_norm = '923000000007' GROUP BY b_norm ORDER BY c DESC LIMIT 20) q;
SELECT count(*) FROM (SELECT b_norm, count(*) AS c, sum(duration_s) FROM forensic.comm_event
  WHERE tenant_id = 'vol' AND collection_id = 'vol-c' AND a_norm = '923000000007' GROUP BY b_norm ORDER BY c DESC LIMIT 20) q;
RESET ROLE;

\echo === idempotent re-load of the first 1000 rows (must insert nothing)
INSERT INTO forensic.comm_event (
  tenant_id, collection_id, evidence_id, file_id, batch_id, source_row, row_hash, content_hash, layout_id, layout_version,
  event_type, start_utc, source_tz, time_basis, raw_payload)
SELECT 'vol', 'vol-c', '35000000-0000-4000-8000-000000000001', 'file-1', '35000000-0000-4000-8000-0000000000a1', g,
       md5(g::text), md5('c' || g::text), 'vol-layout', 1, 'voice',
       timestamptz '2026-01-01 00:00:00+00' + g * interval '15 seconds', 'Asia/Karachi', 'declared', '{}'
FROM generate_series(1, 1000) AS g
ON CONFLICT (tenant_id, collection_id, file_id, row_hash, start_utc) DO NOTHING;
SELECT CASE WHEN count(*) = :rows THEN 'reload_inserted_nothing' ELSE 'RELOAD_DUPLICATED_ROWS' END AS reload_check FROM forensic.comm_event;
