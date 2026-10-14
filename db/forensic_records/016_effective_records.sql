-- Migration 016 (DRAFT, not applied): the "effective" record view of the call-record family.
--
-- Purpose (docs/data/typed-family-switch.md): let the question lane (and, later, the typed compiler) read the call-record family from the typed table
-- (forensic.comm_event) WITHOUT changing a line of their SQL builders. Those builders read `FROM <table> r` and use the columns of forensic.records
-- (record_id, tenant_id, collection_id, file_id, batch_id, record_type, timestamp, primary_target, secondary_target, source_file, row_number,
-- row_hash, raw_payload, metadata, ingested_at, evidence_id). This view has exactly those columns and types, so the family's source is a table name:
-- the Go side chooses between forensic.records and forensic.records_effective_cdr per family, behind a switch (FORENSIC_TYPED_FAMILIES, default empty).
--
-- What a row is. For every evidence item, the typed rows if the item has any (dataset_kind 'target_cdr'), otherwise its rows in forensic.records
-- (record_type 'cdr'). So no backfill is needed to switch, an item is never counted twice, and writing typed rows for an old item changes its
-- answers only from that moment.
--
-- For the fields a typed row OWNS (the subject and counterparty numbers, the two times and the device identifiers) the typed value is the only
-- one: the provider's own columns of that meaning, under every alias the semantic layer lists for the field (the removal list below mirrors
-- semantic_layer/cdr.yaml; a test compares them), are taken out of the payload first. Otherwise a row whose counterparty the typed ingest could not
-- resolve (unknown direction) would still show the provider's raw dialled number to the question lane.
--
-- Typed rows are presented in the shape the semantic layer already understands (semantic_layer/cdr.yaml): the normalised numbers (a_norm, else the raw value, as
-- MSISDN; b_norm, else the raw value such as a short code or a sender id, as CALL_DIALED_NUM), the event times as UTC instants (CALL_START_DT_TM, CALL_END_DT_TM), the device identifiers, and, only where the provider's
-- own column is absent, the call type, the direction and the coordinates. The provider's own columns stay in raw_payload and win over the derived
-- call type, direction and coordinates, as they did in version 1.
--
-- Rules that differ from version 1, on purpose:
--   * a row flagged duplicate_row is not returned (version 1 dropped repeats at ingest; the typed table keeps them, flagged);
--   * a row whose time could not be read or whose zone is unknown was quarantined at ingest and is not here (version 1 kept it with the ingest time);
--   * the time is the UTC instant of the event, not the instant of ingest;
--   * row_number is the source row + 1 (version 1 counts the header as row 1); a file with lines above its header numbers differently in the two.
--
-- security_invoker: the view runs with the privileges and the row-level security of the CALLER, so the tenant policies of the underlying tables apply
-- exactly as they do when the tables are read directly. Needs PostgreSQL 15 or later (the image is 16).
--
-- Additive and idempotent: no table, row or policy is changed. Requires migration 012 (forensic.comm_event) and 007 (forensic.evidence_items).
-- Apply by hand on a retained database: ops/016_effective_records/preflight.sql first, verify.sql after; smoke.sql is for disposable databases only;
-- rollback.sql drops the view (nothing else, no data).
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE OR REPLACE VIEW forensic.records_effective_cdr
WITH (security_invoker = true) AS
SELECT
  coalesce(e.record_id, md5('forensic.comm_event:' || e.event_id::text)::uuid) AS record_id,
  e.tenant_id::varchar AS tenant_id,
  e.collection_id::varchar AS collection_id,
  e.file_id::varchar AS file_id,
  e.batch_id AS batch_id,
  'cdr'::varchar AS record_type,
  e.start_utc AS "timestamp",
  coalesce(e.a_norm, e.a_raw)::varchar AS primary_target,
  coalesce(e.b_norm, e.b_raw)::varchar AS secondary_target,
  coalesce(ev.original_filename, e.file_id)::varchar AS source_file,
  (e.source_row + 1)::bigint AS row_number,
  e.row_hash::varchar AS row_hash,
  (e.raw_payload - ARRAY['MSISDN', 'CALL_DIALED_NUM', 'dialed_number', 'target_number', 'b_number',
                         'CALL_START_DT_TM', 'call_start_time', 'call_time', 'start_time', 'CALL_END_DT_TM', 'call_end_time', 'end_time',
                         'IMSI', 'IMEI']) || o.overlay AS raw_payload,
  jsonb_build_object(
    'normalized_fields', o.overlay,
    'typed_source', jsonb_build_object(
      'table', 'comm_event', 'layout_id', e.layout_id, 'layout_version', e.layout_version,
      'time_basis', e.time_basis, 'source_tz', e.source_tz, 'quality_flags', to_jsonb(e.quality_flags))) AS metadata,
  e.ingested_at AS ingested_at,
  e.evidence_id AS evidence_id
FROM forensic.comm_event e
LEFT JOIN forensic.evidence_items ev
  ON ev.tenant_id = e.tenant_id AND ev.collection_id = e.collection_id AND ev.evidence_id = e.evidence_id
CROSS JOIN LATERAL (
  SELECT jsonb_strip_nulls(jsonb_build_object(
    'MSISDN', coalesce(e.a_norm, e.a_raw),
    'CALL_DIALED_NUM', coalesce(e.b_norm, e.b_raw),
    'CALL_START_DT_TM', to_char(e.start_utc AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"+00:00"'),
    'CALL_END_DT_TM', to_char(e.end_utc AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"+00:00"'),
    'IMSI', e.imsi,
    'IMEI', e.imei,
    'CALL_TYPE', CASE WHEN nullif(e.raw_payload->>'CALL_TYPE', '') IS NOT NULL THEN NULL
                      ELSE CASE e.event_type WHEN 'voice' THEN 'VOICE' WHEN 'sms' THEN 'SMS' WHEN 'mms' THEN 'MMS'
                                             WHEN 'data' THEN 'GPRS' WHEN 'ussd' THEN 'USSD' ELSE NULL END END,
    'INBOUND_OUTBOUND_IND', CASE WHEN nullif(e.raw_payload->>'INBOUND_OUTBOUND_IND', '') IS NOT NULL THEN NULL
                                 ELSE CASE e.direction WHEN 'incoming' THEN 'INCOMING' WHEN 'outgoing' THEN 'OUTGOING' ELSE NULL END END,
    'lat', CASE WHEN nullif(e.raw_payload->>'lat', '') IS NOT NULL THEN NULL ELSE e.lat::text END,
    'longitude', CASE WHEN nullif(e.raw_payload->>'longitude', '') IS NOT NULL THEN NULL ELSE e.lon::text END
  )) AS overlay
) o
WHERE e.dataset_kind = 'target_cdr'
  AND NOT ('duplicate_row' = ANY (e.quality_flags))
UNION ALL
SELECT r.record_id, r.tenant_id, r.collection_id, r.file_id, r.batch_id, r.record_type, r."timestamp", r.primary_target, r.secondary_target,
       r.source_file, r.row_number, r.row_hash, r.raw_payload, r.metadata, r.ingested_at, r.evidence_id
FROM forensic.records r
WHERE r.record_type = 'cdr'
  AND NOT EXISTS (
    SELECT 1 FROM forensic.comm_event t
    WHERE t.tenant_id = r.tenant_id AND t.collection_id = r.collection_id AND t.evidence_id = r.evidence_id
      AND t.dataset_kind = 'target_cdr');

COMMENT ON VIEW forensic.records_effective_cdr IS
  'Call records as the question lane and the typed compiler read them: per evidence item the typed rows (forensic.comm_event, dataset_kind target_cdr, repeats excluded) if it has any, else its forensic.records rows. Same columns and types as forensic.records. security_invoker: the caller''s row-level security applies.';

-- The application role reads the view like the tables beneath it (it is created after the blanket grants of runtime_role.grants.sql ran).
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    EXECUTE 'GRANT SELECT ON forensic.records_effective_cdr TO forensic_runtime';
  END IF;
END
$$;

COMMIT;
