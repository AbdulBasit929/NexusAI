-- Migration 018: the "effective" record view of the internet-record (IPDR) family, the second typed family switch after migration 016.
--
-- Purpose (docs/data/typed-family-switch.md): let the question lane read the internet-record family from the typed table (forensic.data_session) WITHOUT changing a
-- line of its SQL builders, exactly as 016 does for call records. This view has exactly the columns and types of forensic.records, so the family's source is a table
-- name that the Go side chooses per family behind FORENSIC_TYPED_FAMILIES (family network_ipdr).
--
-- Why it was needed. Version 1's adapters recognise few real internet-record layouts, so most of them are stored there as `generic` rows, and a question such as
-- "How many internet sessions are there?" answered 0 while the typed table held 14,969: a confident wrong answer (found on 2026-10-14 on the real-data demo collection).
--
-- What a row is. For every evidence item, the typed rows if the item has any in forensic.data_session (repeats excluded), otherwise its rows in forensic.records
-- (record_type 'ipdr'). So no backfill is needed to switch and an item is never counted twice.
--
-- Typed rows are presented in the shape the semantic layer already understands (semantic_layer/ipdr.yaml): the subscriber as subscriber_id and msisdn, the session
-- times as UTC instants (timestamp, session_start, session_end), the private source address and port (source_ip, source_port), the server address and port
-- (destination_ip, destination_port), the volumes (bytes_up, bytes_down and bytes, their sum), the host as domain, the protocol, the session reference, the cell and
-- the URL, each only where the typed row has it. The provider's own columns stay in raw_payload (under the provider's own header names).
--
-- Differences from version 1, on purpose: a row flagged duplicate_row is not returned; the time is the UTC instant of the session; row_number is the source row + 1.
--
-- security_invoker: the view runs with the privileges and the row-level security of the CALLER, so the tenant policies of the underlying tables apply as they do when
-- the tables are read directly. Needs PostgreSQL 15 or later (the image is 16).
--
-- Additive and idempotent: no table, row or policy is changed. Requires migration 012 (forensic.data_session) and 007 (forensic.evidence_items).
-- Apply by hand on a retained database: ops/018_effective_ipdr/preflight.sql first, verify.sql after; rollback.sql drops the view (nothing else, no data).
BEGIN;
SET LOCAL lock_timeout = '10s';
CREATE OR REPLACE VIEW forensic.records_effective_ipdr
WITH (security_invoker = true) AS
SELECT
  coalesce(s.record_id, md5('forensic.data_session:' || s.session_id::text)::uuid) AS record_id,
  s.tenant_id::varchar AS tenant_id,
  s.collection_id::varchar AS collection_id,
  s.file_id::varchar AS file_id,
  s.batch_id AS batch_id,
  'ipdr'::varchar AS record_type,
  s.start_utc AS "timestamp",
  s.msisdn::varchar AS primary_target,
  coalesce(host(s.server_ip), s.host)::varchar AS secondary_target,
  coalesce(ev.original_filename, s.file_id)::varchar AS source_file,
  (s.source_row + 1)::bigint AS row_number,
  s.row_hash::varchar AS row_hash,
  s.raw_payload || o.overlay AS raw_payload,
  jsonb_build_object(
    'normalized_fields', o.overlay,
    'typed_source', jsonb_build_object(
      'table', 'data_session', 'layout_id', s.layout_id, 'layout_version', s.layout_version,
      'time_basis', s.time_basis, 'source_tz', s.source_tz, 'quality_flags', to_jsonb(s.quality_flags))) AS metadata,
  s.ingested_at AS ingested_at,
  s.evidence_id AS evidence_id
FROM forensic.data_session s
LEFT JOIN forensic.evidence_items ev
  ON ev.tenant_id = s.tenant_id AND ev.collection_id = s.collection_id AND ev.evidence_id = s.evidence_id
CROSS JOIN LATERAL (
  SELECT jsonb_strip_nulls(jsonb_build_object(
    'subscriber_id', s.msisdn,
    'msisdn', s.msisdn,
    'timestamp', to_char(s.start_utc AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"+00:00"'),
    'session_start', to_char(s.start_utc AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"+00:00"'),
    'session_end', to_char(s.end_utc AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"+00:00"'),
    'source_ip', host(s.ue_ip),
    'destination_ip', host(s.server_ip),
    'source_port', s.ue_port::text,
    'destination_port', s.server_port::text,
    'bytes_up', s.bytes_up::text,
    'bytes_down', s.bytes_down::text,
    'bytes', CASE WHEN s.bytes_up IS NULL AND s.bytes_down IS NULL THEN NULL ELSE (coalesce(s.bytes_up, 0) + coalesce(s.bytes_down, 0))::text END,
    'domain', s.host,
    'protocol', s.protocol,
    'session_id', s.session_ref,
    'cell_id', s.cell_key,
    'url', s.url,
    'source_timezone', s.source_tz
  )) AS overlay
) o
WHERE NOT ('duplicate_row' = ANY (s.quality_flags))
UNION ALL
SELECT r.record_id, r.tenant_id, r.collection_id, r.file_id, r.batch_id, r.record_type, r."timestamp", r.primary_target, r.secondary_target,
       r.source_file, r.row_number, r.row_hash, r.raw_payload, r.metadata, r.ingested_at, r.evidence_id
FROM forensic.records r
WHERE r.record_type = 'ipdr'
  AND NOT EXISTS (
    SELECT 1 FROM forensic.data_session t
    WHERE t.tenant_id = r.tenant_id AND t.collection_id = r.collection_id AND t.evidence_id = r.evidence_id);
COMMENT ON VIEW forensic.records_effective_ipdr IS
  'Internet records as the question lane reads them: per evidence item the typed rows (forensic.data_session, repeats excluded) if it has any, else its forensic.records rows. Same columns and types as forensic.records. Read only by the lane when FORENSIC_TYPED_FAMILIES lists network_ipdr.';

-- The application role reads the view like the tables beneath it (it is created after the blanket grants of runtime_role.grants.sql ran).
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    EXECUTE 'GRANT SELECT ON forensic.records_effective_ipdr TO forensic_runtime';
  END IF;
END
$$;
COMMIT;
