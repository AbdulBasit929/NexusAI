-- Phase 2: backfill legacy structured rows into the canonical source-of-truth
-- forensic.records table used by the canonical query engine.
--
-- This migration is additive and idempotent. It does not modify legacy records;
-- it only mirrors rows that are not already present in forensic.records.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE INDEX IF NOT EXISTS forensic_records_source_file_idx
  ON forensic.records (tenant_id, collection_id, source_file, "timestamp" DESC);

CREATE INDEX IF NOT EXISTS forensic_records_batch_idx
  ON forensic.records (tenant_id, collection_id, batch_id, row_number);

CREATE INDEX IF NOT EXISTS forensic_records_primary_target_idx
  ON forensic.records (tenant_id, collection_id, primary_target, "timestamp" DESC);

INSERT INTO forensic.records (
  tenant_id,
  collection_id,
  file_id,
  batch_id,
  record_type,
  "timestamp",
  primary_target,
  secondary_target,
  source_file,
  row_number,
  row_hash,
  raw_payload,
  metadata,
  ingested_at
)
SELECT
  tenant_id,
  collection_id,
  file_id,
  batch_id,
  'cdr',
  call_start_ts,
  coalesce(nullif(msisdn, ''), nullif(call_org_num, '')),
  nullif(call_dialed_num, ''),
  source_file,
  row_number,
  row_hash,
  raw_record,
  jsonb_build_object(
    'dynamic_store', 'forensic.records',
    'legacy_store', 'forensic.cdr_records',
    'backfill_migration', '005_backfill_canonical_records',
    'source_fields', jsonb_build_object(
      'timestamp', 'call_start_ts',
      'primary_target', 'msisdn',
      'primary_target_fallback', 'call_org_num',
      'secondary_target', 'call_dialed_num',
      'record_type', 'cdr'
    )
  ),
  ingested_at
FROM forensic.cdr_records
ON CONFLICT DO NOTHING;

INSERT INTO forensic.records (
  tenant_id,
  collection_id,
  file_id,
  batch_id,
  record_type,
  "timestamp",
  primary_target,
  secondary_target,
  source_file,
  row_number,
  row_hash,
  raw_payload,
  metadata,
  ingested_at
)
SELECT
  tenant_id,
  collection_id,
  file_id,
  batch_id,
  record_type::text,
  coalesce(observed_at, ingested_at),
  nullif(primary_entity, ''),
  nullif(secondary_entity, ''),
  source_file,
  row_number,
  row_hash,
  raw_record,
  jsonb_build_object(
    'dynamic_store', 'forensic.records',
    'legacy_store', 'forensic.generic_records',
    'backfill_migration', '005_backfill_canonical_records',
    'source_fields', jsonb_build_object(
      'timestamp', 'observed_at',
      'timestamp_fallback', 'ingested_at',
      'primary_target', 'primary_entity',
      'secondary_target', 'secondary_entity',
      'record_type', 'record_type'
    )
  ),
  ingested_at
FROM forensic.generic_records
ON CONFLICT DO NOTHING;

ANALYZE forensic.records;
