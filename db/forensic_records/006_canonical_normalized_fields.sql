-- Phase 4: attach normalized structured fields to canonical records metadata.
--
-- Original evidence remains in forensic.records.raw_payload. This migration
-- adds metadata.normalized_fields so runtime canonical filters can answer
-- analyst questions over both source headers and normalized structured facts
-- such as duration_seconds, call_type, direction, location, and target fields.

UPDATE forensic.records r
SET metadata = jsonb_set(
  coalesce(r.metadata, '{}'::jsonb),
  '{normalized_fields}',
  jsonb_strip_nulls(jsonb_build_object(
    'msisdn', c.msisdn,
    'call_org_num', c.call_org_num,
    'call_dialed_num', c.call_dialed_num,
    'imsi', c.imsi,
    'imei', c.imei,
    'call_start_ts', c.call_start_ts,
    'call_end_ts', c.call_end_ts,
    'duration_seconds', c.duration_seconds,
    'direction', c.direction,
    'network_volume', c.network_volume,
    'lac_id', c.lac_id,
    'site_id', c.site_id,
    'cell_site_id', c.cell_site_id,
    'latitude', c.latitude,
    'longitude', c.longitude,
    'call_type', c.call_type,
    'location', c.location
  )),
  true
)
FROM forensic.cdr_records c
WHERE r.tenant_id = c.tenant_id
  AND r.collection_id = c.collection_id
  AND r.file_id = c.file_id
  AND r.batch_id = c.batch_id
  AND r.row_hash = c.row_hash
  AND r."timestamp" = c.call_start_ts
  AND r.record_type = 'cdr';

UPDATE forensic.records r
SET metadata = jsonb_set(
  coalesce(r.metadata, '{}'::jsonb),
  '{normalized_fields}',
  jsonb_strip_nulls(jsonb_build_object(
    'record_type', g.record_type::text,
    'observed_at', g.observed_at,
    'primary_entity', g.primary_entity,
    'secondary_entity', g.secondary_entity,
    'location', g.location,
    'latitude', g.latitude,
    'longitude', g.longitude
  )),
  true
)
FROM forensic.generic_records g
WHERE r.tenant_id = g.tenant_id
  AND r.collection_id = g.collection_id
  AND r.file_id = g.file_id
  AND r.batch_id = g.batch_id
  AND r.row_hash = g.row_hash
  AND r.record_type = g.record_type::text;

CREATE INDEX IF NOT EXISTS forensic_records_metadata_gin_idx
  ON forensic.records USING gin (metadata);

ANALYZE forensic.records;
