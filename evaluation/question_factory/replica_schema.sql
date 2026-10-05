-- A small stand-in for the canonical store, so the question factory and the compiler can be developed without the laptop.
-- Same columns and meaning as db/forensic_records/004_dynamic_records.sql (forensic.records), without TimescaleDB, row-level security or indexes.
-- raw_payload holds the source row keyed by its ORIGINAL column headers; the semantic layer's `source_names` are the aliases for those headers.
CREATE SCHEMA IF NOT EXISTS forensic;
DROP TABLE IF EXISTS forensic.records;
CREATE TABLE forensic.records (
  record_id        uuid        NOT NULL DEFAULT gen_random_uuid(),
  tenant_id        varchar     NOT NULL DEFAULT 'default',
  collection_id    varchar     NOT NULL,
  file_id          varchar     NOT NULL,
  batch_id         uuid        NOT NULL DEFAULT gen_random_uuid(),
  record_type      varchar     NOT NULL DEFAULT 'generic',
  "timestamp"      timestamptz NOT NULL DEFAULT now(),
  primary_target   varchar,
  secondary_target varchar,
  source_file      varchar     NOT NULL,
  row_number       bigint      NOT NULL,
  row_hash         varchar     NOT NULL,
  raw_payload      jsonb       NOT NULL,
  metadata         jsonb       NOT NULL DEFAULT '{}'::jsonb,
  ingested_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (record_id, "timestamp")
);
CREATE INDEX replica_records_lookup ON forensic.records (tenant_id, collection_id, record_type);
