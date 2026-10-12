-- Migration 012 (DRAFT, not applied): typed event and registration tables.
--
-- Purpose (docs/roadmap/nexusai-maturity-roadmap-20261012.md, section 4.5): give call records, internet sessions, application
-- usage, plate sightings and subscriber registrations typed columns, an explicit source time zone and time basis, number
-- classes, and lineage to the layout version that parsed each row.
--
-- Additive and idempotent: no existing table, view, policy or row is changed. Each typed row keeps its own raw source row
-- (raw_payload) so the legacy JSON copy in forensic.records can be retired after parity; record_id/record_ts are only an
-- optional soft link to that copy during the dual-write window.
--
-- Requires migration 008 (evidence_items_tenant_identity_idx and forensic.derived_artifacts) and the pg_trgm extension.
--
-- On a retained database apply it by hand (db/forensic_records/README.md and docs/data/d2-migration-runbook.md): run
-- ops/012_typed_families/preflight.sql first and verify.sql after; smoke.sql is for disposable databases only; rollback.sql
-- only if verify fails. On a fresh database the Docker entrypoint runs it in name order with the other top-level files.
BEGIN;
-- Fail fast instead of queueing behind a long transaction: a DDL statement that waits for a lock also blocks every
-- writer queued behind it. If a lock cannot be had within ten seconds the whole migration rolls back; run it again later.
SET LOCAL lock_timeout = '10s';

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ---------------------------------------------------------------------------------------------------------------------
-- Layout registry lineage. The registry itself is versioned files in the repository; this table records which version of
-- which layout produced a row, and the hash of its definition, so a later change to a layout never rewrites history.
-- Global (no tenant, no personal data), so it has no row-level security.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.layout_versions (
  layout_id text NOT NULL,
  layout_version integer NOT NULL,
  family text NOT NULL,
  dataset_kind text NOT NULL,
  operator_label text,
  definition_sha256 text NOT NULL,
  status text NOT NULL DEFAULT 'active',
  registered_at timestamptz NOT NULL DEFAULT now(),
  registered_by text NOT NULL DEFAULT '',
  notes text,
  PRIMARY KEY (layout_id, layout_version),
  CONSTRAINT layout_versions_family_chk CHECK (family IN
    ('communications', 'internet', 'app_usage', 'subscriber', 'cell_reference', 'plate', 'transaction', 'access_log', 'generic', 'other')),
  CONSTRAINT layout_versions_kind_chk CHECK (dataset_kind IN
    ('target_cdr', 'tower_dump', 'ipdr', 'app_usage', 'subscriber', 'cell_reference', 'plate_sighting', 'generic')),
  CONSTRAINT layout_versions_status_chk CHECK (status IN ('draft', 'active', 'retired')),
  CONSTRAINT layout_versions_sha_chk CHECK (definition_sha256 ~ '^[0-9a-f]{64}$'),
  CONSTRAINT layout_versions_version_chk CHECK (layout_version > 0),
  UNIQUE (layout_id, definition_sha256)
);

COMMENT ON TABLE forensic.layout_versions IS
  'One row per registered provider layout version. Rows of typed tables reference (layout_id, layout_version).';

-- A fallback entry so rows from a file with no recognised layout can still be loaded and flagged.
INSERT INTO forensic.layout_versions (layout_id, layout_version, family, dataset_kind, operator_label, definition_sha256, status, registered_by, notes)
VALUES
  ('generic-unmapped', 1, 'generic', 'generic', NULL,
   'e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855', 'active', 'migration-012',
   'Fallback for rows loaded without a recognised layout; every such row carries the quality flag unmapped_layout.')
ON CONFLICT (layout_id, layout_version) DO NOTHING;

-- ---------------------------------------------------------------------------------------------------------------------
-- comm_event: one row per call, SMS, USSD or data event from a call-record file or a tower dump.
-- a_* is the party the record is about (target CDR) or the party observed at the cell (tower dump); b_* is the other party.
-- direction is relative to a_*: incoming means b_* contacted a_*.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.comm_event (
  event_id bigint GENERATED ALWAYS AS IDENTITY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  file_id text NOT NULL,
  batch_id uuid NOT NULL,
  source_row bigint NOT NULL,
  record_id uuid,
  record_ts timestamptz,
  row_hash text NOT NULL,
  content_hash text NOT NULL,
  layout_id text NOT NULL,
  layout_version integer NOT NULL,
  dataset_kind text NOT NULL DEFAULT 'target_cdr',
  event_type text NOT NULL,
  direction text NOT NULL DEFAULT 'unknown',
  a_raw text,
  a_norm text,
  a_kind text,
  b_raw text,
  b_norm text,
  b_kind text,
  start_utc timestamptz NOT NULL,
  end_utc timestamptz,
  duration_s integer,
  source_tz text NOT NULL,
  time_basis text NOT NULL,
  imsi text,
  imei text,
  tac text,
  cell_key text,
  lac_tac text,
  cell_raw text,
  lat double precision,
  lon double precision,
  loc_method text NOT NULL DEFAULT 'none',
  loc_uncertainty_m integer,
  peer_operator text,
  volume_bytes numeric,
  raw_payload jsonb NOT NULL,
  quality_flags text[] NOT NULL DEFAULT '{}',
  ingested_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (event_id, start_utc),
  UNIQUE (tenant_id, collection_id, file_id, row_hash, start_utc),
  CONSTRAINT comm_event_layout_fk FOREIGN KEY (layout_id, layout_version) REFERENCES forensic.layout_versions (layout_id, layout_version),
  CONSTRAINT comm_event_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT comm_event_payload_chk CHECK (jsonb_typeof(raw_payload) = 'object'),
  CONSTRAINT comm_event_dataset_kind_chk CHECK (dataset_kind IN ('target_cdr', 'tower_dump')),
  CONSTRAINT comm_event_event_type_chk CHECK (event_type IN ('voice', 'sms', 'mms', 'data', 'ussd', 'other')),
  CONSTRAINT comm_event_direction_chk CHECK (direction IN ('incoming', 'outgoing', 'none', 'unknown')),
  CONSTRAINT comm_event_a_kind_chk CHECK (a_kind IS NULL OR a_kind IN
    ('person_like', 'short_code', 'sender_id', 'international', 'service_label', 'unknown')),
  CONSTRAINT comm_event_b_kind_chk CHECK (b_kind IS NULL OR b_kind IN
    ('person_like', 'short_code', 'sender_id', 'international', 'service_label', 'unknown')),
  CONSTRAINT comm_event_time_basis_chk CHECK (time_basis IN
    ('declared', 'assumed_by_profile', 'confirmed_by_analyst', 'explicit_offset')),
  CONSTRAINT comm_event_loc_method_chk CHECK (loc_method IN ('none', 'source_supplied', 'cell_reference', 'geocoded', 'analyst')),
  CONSTRAINT comm_event_duration_chk CHECK (duration_s IS NULL OR duration_s >= 0),
  CONSTRAINT comm_event_volume_chk CHECK (volume_bytes IS NULL OR volume_bytes >= 0),
  CONSTRAINT comm_event_coord_chk CHECK (
    (lat IS NULL AND lon IS NULL)
    OR (lat IS NOT NULL AND lon IS NOT NULL AND lat BETWEEN -90 AND 90 AND lon BETWEEN -180 AND 180)),
  CONSTRAINT comm_event_uncertainty_chk CHECK (loc_uncertainty_m IS NULL OR loc_uncertainty_m >= 0)
);

SELECT create_hypertable('forensic.comm_event', 'start_utc', if_not_exists => TRUE, chunk_time_interval => INTERVAL '7 days');

CREATE INDEX IF NOT EXISTS comm_event_a_time_idx
  ON forensic.comm_event (tenant_id, collection_id, a_norm, start_utc DESC) WHERE a_norm IS NOT NULL;
CREATE INDEX IF NOT EXISTS comm_event_b_time_idx
  ON forensic.comm_event (tenant_id, collection_id, b_norm, start_utc DESC) WHERE b_norm IS NOT NULL;
CREATE INDEX IF NOT EXISTS comm_event_cell_time_idx
  ON forensic.comm_event (tenant_id, collection_id, cell_key, start_utc DESC) WHERE cell_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS comm_event_imei_time_idx
  ON forensic.comm_event (tenant_id, collection_id, imei, start_utc DESC) WHERE imei IS NOT NULL;
CREATE INDEX IF NOT EXISTS comm_event_imsi_time_idx
  ON forensic.comm_event (tenant_id, collection_id, imsi, start_utc DESC) WHERE imsi IS NOT NULL;
CREATE INDEX IF NOT EXISTS comm_event_evidence_idx
  ON forensic.comm_event (tenant_id, collection_id, evidence_id, source_row);
CREATE INDEX IF NOT EXISTS comm_event_content_hash_idx
  ON forensic.comm_event (tenant_id, collection_id, content_hash);

COMMENT ON TABLE forensic.comm_event IS
  'Typed call, SMS, USSD and data events (CDR and tower dumps). Each row keeps its raw source row in raw_payload.';
COMMENT ON COLUMN forensic.comm_event.a_norm IS 'Canonical E.164-style digits only when unambiguous; otherwise NULL and a_raw/a_kind describe it.';
COMMENT ON COLUMN forensic.comm_event.a_kind IS 'Number class. Only person_like numbers belong in person-to-person relationship analysis.';
COMMENT ON COLUMN forensic.comm_event.time_basis IS
  'How the source time zone is known: declared by the layout, assumed by the country profile, confirmed by an analyst, or an explicit offset in the value.';
COMMENT ON COLUMN forensic.comm_event.cell_key IS 'Canonical cell key from forensic.make_cell_key(plmn, lac_tac, ci); NULL when the file gives no cell.';
COMMENT ON COLUMN forensic.comm_event.row_hash IS
  'sha256 over file identity, source row number and canonical content: identical rows at different positions of one file are both kept.';
COMMENT ON COLUMN forensic.comm_event.content_hash IS
  'sha256 over the canonical content alone (no file identity or row number), for cross-file duplicate reporting; duplicates are kept with lineage, never dropped.';
COMMENT ON COLUMN forensic.comm_event.raw_payload IS
  'The source row as read (JSON), kept with the layout version that parsed it, so the typed row never depends on the legacy forensic.records copy.';
COMMENT ON COLUMN forensic.comm_event.record_id IS
  'Optional soft link to forensic.records(record_id, "timestamp") during the dual-write window. Deliberately not a foreign key: the legacy copy is to be retired, and the link is NULL for rows loaded only through the typed path.';
COMMENT ON COLUMN forensic.comm_event.quality_flags IS 'Parser and validator flags, for example unknown_direction_code, foreign_block_suspected, unmapped_layout.';

-- ---------------------------------------------------------------------------------------------------------------------
-- data_session: one row per internet session or flow record (IPDR, UFDR), with both address spaces and port ranges.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.data_session (
  session_id bigint GENERATED ALWAYS AS IDENTITY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  file_id text NOT NULL,
  batch_id uuid NOT NULL,
  source_row bigint NOT NULL,
  record_id uuid,
  record_ts timestamptz,
  row_hash text NOT NULL,
  content_hash text NOT NULL,
  layout_id text NOT NULL,
  layout_version integer NOT NULL,
  start_utc timestamptz NOT NULL,
  end_utc timestamptz,
  nat_start_utc timestamptz,
  nat_end_utc timestamptz,
  source_tz text NOT NULL,
  time_basis text NOT NULL,
  msisdn text,
  imsi text,
  imei text,
  tac text,
  ue_ip inet,
  ue_port integer,
  public_ip inet,
  public_port_from integer,
  public_port_to integer,
  server_ip inet,
  server_port integer,
  protocol_num smallint,
  protocol text,
  apn text,
  rat text,
  cell_key text,
  lac_tac text,
  cell_raw text,
  gateway_ip inet,
  serving_node_ip inet,
  app_category text,
  app text,
  app_feature text,
  host text,
  url text,
  user_agent text,
  bytes_up bigint,
  bytes_down bigint,
  packets_up bigint,
  packets_down bigint,
  duration_ms bigint,
  session_ref text,
  query_criteria text,
  raw_payload jsonb NOT NULL,
  quality_flags text[] NOT NULL DEFAULT '{}',
  ingested_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (session_id, start_utc),
  UNIQUE (tenant_id, collection_id, file_id, row_hash, start_utc),
  CONSTRAINT data_session_layout_fk FOREIGN KEY (layout_id, layout_version) REFERENCES forensic.layout_versions (layout_id, layout_version),
  CONSTRAINT data_session_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT data_session_payload_chk CHECK (jsonb_typeof(raw_payload) = 'object'),
  CONSTRAINT data_session_time_basis_chk CHECK (time_basis IN
    ('declared', 'assumed_by_profile', 'confirmed_by_analyst', 'explicit_offset')),
  CONSTRAINT data_session_ports_chk CHECK (
    (ue_port IS NULL OR ue_port BETWEEN 0 AND 65535)
    AND (public_port_from IS NULL OR public_port_from BETWEEN 0 AND 65535)
    AND (public_port_to IS NULL OR public_port_to BETWEEN 0 AND 65535)
    AND (server_port IS NULL OR server_port BETWEEN 0 AND 65535)
    AND (public_port_to IS NULL OR public_port_from IS NULL OR public_port_to >= public_port_from)),
  CONSTRAINT data_session_protocol_chk CHECK (protocol_num IS NULL OR protocol_num BETWEEN 0 AND 255),
  CONSTRAINT data_session_volume_chk CHECK (
    (bytes_up IS NULL OR bytes_up >= 0) AND (bytes_down IS NULL OR bytes_down >= 0)
    AND (packets_up IS NULL OR packets_up >= 0) AND (packets_down IS NULL OR packets_down >= 0)
    AND (duration_ms IS NULL OR duration_ms >= 0))
);

SELECT create_hypertable('forensic.data_session', 'start_utc', if_not_exists => TRUE, chunk_time_interval => INTERVAL '7 days');

CREATE INDEX IF NOT EXISTS data_session_msisdn_time_idx
  ON forensic.data_session (tenant_id, collection_id, msisdn, start_utc DESC) WHERE msisdn IS NOT NULL;
CREATE INDEX IF NOT EXISTS data_session_public_ip_time_idx
  ON forensic.data_session (tenant_id, collection_id, public_ip, start_utc DESC) WHERE public_ip IS NOT NULL;
CREATE INDEX IF NOT EXISTS data_session_ue_ip_time_idx
  ON forensic.data_session (tenant_id, collection_id, ue_ip, start_utc DESC) WHERE ue_ip IS NOT NULL;
CREATE INDEX IF NOT EXISTS data_session_server_ip_time_idx
  ON forensic.data_session (tenant_id, collection_id, server_ip, start_utc DESC) WHERE server_ip IS NOT NULL;
CREATE INDEX IF NOT EXISTS data_session_app_time_idx
  ON forensic.data_session (tenant_id, collection_id, app, start_utc DESC) WHERE app IS NOT NULL;
CREATE INDEX IF NOT EXISTS data_session_imei_time_idx
  ON forensic.data_session (tenant_id, collection_id, imei, start_utc DESC) WHERE imei IS NOT NULL;
CREATE INDEX IF NOT EXISTS data_session_evidence_idx
  ON forensic.data_session (tenant_id, collection_id, evidence_id, source_row);
CREATE INDEX IF NOT EXISTS data_session_content_hash_idx
  ON forensic.data_session (tenant_id, collection_id, content_hash);
CREATE INDEX IF NOT EXISTS data_session_host_trgm_idx
  ON forensic.data_session USING gin (host gin_trgm_ops) WHERE host IS NOT NULL;

COMMENT ON TABLE forensic.data_session IS
  'Typed internet session (IPDR, UFDR) records with private and public addresses, port ranges and two clocks (flow and NAT).';
COMMENT ON COLUMN forensic.data_session.ue_ip IS 'Subscriber-side (private) address: the UE IP or private source address of the file.';
COMMENT ON COLUMN forensic.data_session.public_ip IS 'Address after network address translation; with public_port_from/to and the NAT times it attributes traffic to a subscriber.';
COMMENT ON COLUMN forensic.data_session.query_criteria IS 'How the export was queried (for example by number or by NAT source IP), from the report metadata when present.';

-- ---------------------------------------------------------------------------------------------------------------------
-- app_usage: application-level usage exports (category, application, feature, province and city, handset model text).
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.app_usage (
  usage_id bigint GENERATED ALWAYS AS IDENTITY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  file_id text NOT NULL,
  batch_id uuid NOT NULL,
  source_row bigint NOT NULL,
  record_id uuid,
  record_ts timestamptz,
  row_hash text NOT NULL,
  content_hash text NOT NULL,
  layout_id text NOT NULL,
  layout_version integer NOT NULL,
  start_utc timestamptz NOT NULL,
  end_utc timestamptz,
  source_tz text NOT NULL,
  time_basis text NOT NULL,
  msisdn text,
  imsi text,
  imei text,
  tac text,
  terminal text,
  apn text,
  app_category text,
  app text,
  app_feature text,
  province text,
  city text,
  raw_payload jsonb NOT NULL,
  quality_flags text[] NOT NULL DEFAULT '{}',
  ingested_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (usage_id, start_utc),
  UNIQUE (tenant_id, collection_id, file_id, row_hash, start_utc),
  CONSTRAINT app_usage_layout_fk FOREIGN KEY (layout_id, layout_version) REFERENCES forensic.layout_versions (layout_id, layout_version),
  CONSTRAINT app_usage_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT app_usage_payload_chk CHECK (jsonb_typeof(raw_payload) = 'object'),
  CONSTRAINT app_usage_time_basis_chk CHECK (time_basis IN
    ('declared', 'assumed_by_profile', 'confirmed_by_analyst', 'explicit_offset'))
);

SELECT create_hypertable('forensic.app_usage', 'start_utc', if_not_exists => TRUE, chunk_time_interval => INTERVAL '7 days');

CREATE INDEX IF NOT EXISTS app_usage_msisdn_time_idx
  ON forensic.app_usage (tenant_id, collection_id, msisdn, start_utc DESC) WHERE msisdn IS NOT NULL;
CREATE INDEX IF NOT EXISTS app_usage_app_time_idx
  ON forensic.app_usage (tenant_id, collection_id, app, start_utc DESC) WHERE app IS NOT NULL;
CREATE INDEX IF NOT EXISTS app_usage_imei_time_idx
  ON forensic.app_usage (tenant_id, collection_id, imei, start_utc DESC) WHERE imei IS NOT NULL;
CREATE INDEX IF NOT EXISTS app_usage_evidence_idx
  ON forensic.app_usage (tenant_id, collection_id, evidence_id, source_row);

COMMENT ON TABLE forensic.app_usage IS 'Typed application-usage rows (no IP addresses or byte counts; location only as province and city).';

-- ---------------------------------------------------------------------------------------------------------------------
-- plate_sighting: one row per plate sighting from a CSV, an image, a video track or a manual entry.
-- Model readings and ingested sightings share the table, told apart by source_kind and review_state, so a model's reading
-- is never reported as a camera sighting (the provenance rule of the multimodal case).
-- ---------------------------------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION forensic.normalize_plate(plate text)
RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE
AS $$
  -- NFKC (full-width forms to ASCII), Arabic-Indic and Persian digits to ASCII digits, dash, dot and middle-dot style
  -- separators removed, upper case. U& escapes keep this file ASCII.
  SELECT upper(regexp_replace(
           translate(normalize(plate, NFKC),
             U&'\0660\0661\0662\0663\0664\0665\0666\0667\0668\0669\06F0\06F1\06F2\06F3\06F4\06F5\06F6\06F7\06F8\06F9\00B7\2022\2010\2011\2012\2013\2014\2015\2212',
             '01234567890123456789'),
           '[\s\-_./]+', '', 'g'))
$$;

COMMENT ON FUNCTION forensic.normalize_plate(text) IS
  'Search key for a plate: Unicode NFKC, Arabic-Indic and Persian digits to ASCII, upper case, presentation separators removed. The raw token stays in plate_raw.';

CREATE TABLE IF NOT EXISTS forensic.plate_sighting (
  sighting_id bigint GENERATED ALWAYS AS IDENTITY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  file_id text NOT NULL,
  batch_id uuid NOT NULL,
  source_row bigint NOT NULL,
  record_id uuid,
  record_ts timestamptz,
  derived_artifact_id uuid,
  row_hash text NOT NULL,
  content_hash text NOT NULL,
  layout_id text NOT NULL,
  layout_version integer NOT NULL,
  source_kind text NOT NULL,
  plate_raw text NOT NULL,
  plate_norm text NOT NULL,
  plate_script text,
  province text,
  series text,
  rule_version text,
  observed_utc timestamptz NOT NULL,
  time_basis text NOT NULL,
  source_tz text,
  camera_id text,
  camera_label text,
  lane text,
  lat double precision,
  lon double precision,
  loc_method text NOT NULL DEFAULT 'none',
  confidence real,
  detector_confidence real,
  ocr_confidence real,
  bbox jsonb,
  crop_hash text,
  frame_number integer,
  media_offset_ms bigint,
  track_id text,
  model_id text,
  model_sha256 text,
  review_state text NOT NULL DEFAULT 'unreviewed',
  raw_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  quality_flags text[] NOT NULL DEFAULT '{}',
  ingested_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (sighting_id, observed_utc),
  UNIQUE (tenant_id, collection_id, file_id, row_hash, observed_utc),
  CONSTRAINT plate_sighting_layout_fk FOREIGN KEY (layout_id, layout_version) REFERENCES forensic.layout_versions (layout_id, layout_version),
  CONSTRAINT plate_sighting_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT plate_sighting_artifact_fk FOREIGN KEY (tenant_id, collection_id, derived_artifact_id)
    REFERENCES forensic.derived_artifacts (tenant_id, collection_id, artifact_id),
  CONSTRAINT plate_sighting_payload_chk CHECK (jsonb_typeof(raw_payload) = 'object'),
  CONSTRAINT plate_sighting_source_kind_chk CHECK (source_kind IN ('csv', 'image', 'video', 'manual')),
  CONSTRAINT plate_sighting_time_basis_chk CHECK (time_basis IN
    ('declared', 'assumed_by_profile', 'confirmed_by_analyst', 'explicit_offset', 'ingest_time')),
  CONSTRAINT plate_sighting_script_chk CHECK (plate_script IS NULL OR plate_script IN ('latin', 'arabic', 'mixed', 'other')),
  CONSTRAINT plate_sighting_review_chk CHECK (review_state IN ('unreviewed', 'confirmed', 'rejected')),
  CONSTRAINT plate_sighting_loc_method_chk CHECK (loc_method IN ('none', 'source_supplied', 'camera_registry', 'geocoded', 'analyst')),
  CONSTRAINT plate_sighting_confidence_chk CHECK (
    (confidence IS NULL OR confidence BETWEEN 0 AND 1)
    AND (detector_confidence IS NULL OR detector_confidence BETWEEN 0 AND 1)
    AND (ocr_confidence IS NULL OR ocr_confidence BETWEEN 0 AND 1)),
  CONSTRAINT plate_sighting_coord_chk CHECK (
    (lat IS NULL AND lon IS NULL)
    OR (lat IS NOT NULL AND lon IS NOT NULL AND lat BETWEEN -90 AND 90 AND lon BETWEEN -180 AND 180)),
  CONSTRAINT plate_sighting_norm_chk CHECK (plate_norm <> '')
);

SELECT create_hypertable('forensic.plate_sighting', 'observed_utc', if_not_exists => TRUE, chunk_time_interval => INTERVAL '7 days');

CREATE INDEX IF NOT EXISTS plate_sighting_plate_time_idx
  ON forensic.plate_sighting (tenant_id, collection_id, plate_norm, observed_utc DESC);
CREATE INDEX IF NOT EXISTS plate_sighting_plate_trgm_idx
  ON forensic.plate_sighting USING gin (plate_norm gin_trgm_ops);
CREATE INDEX IF NOT EXISTS plate_sighting_camera_time_idx
  ON forensic.plate_sighting (tenant_id, collection_id, camera_id, observed_utc DESC) WHERE camera_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS plate_sighting_artifact_idx
  ON forensic.plate_sighting (tenant_id, collection_id, derived_artifact_id) WHERE derived_artifact_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS plate_sighting_track_idx
  ON forensic.plate_sighting (tenant_id, collection_id, evidence_id, track_id) WHERE track_id IS NOT NULL;

COMMENT ON TABLE forensic.plate_sighting IS
  'Plate sightings from CSV, image, video and manual sources. source_kind and review_state keep model readings apart from camera sightings.';
COMMENT ON COLUMN forensic.plate_sighting.time_basis IS
  'ingest_time means the evidence has no time of its own (for example a frame read): such rows never match a time condition.';
COMMENT ON COLUMN forensic.plate_sighting.plate_norm IS 'Search key from forensic.normalize_plate(plate_raw). The raw token is the evidence; this is only for matching.';

-- ---------------------------------------------------------------------------------------------------------------------
-- subscriber_identity: personal identity fields (name, national ID number, address), kept apart from registration facts.
-- The runtime role gets no privilege on this table until the masking design (roadmap R9) is approved.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.subscriber_identity (
  identity_ref uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  source_row bigint NOT NULL,
  full_name text,
  cnic text,
  address_text text,
  extra jsonb NOT NULL DEFAULT '{}'::jsonb,
  ingested_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT subscriber_identity_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT subscriber_identity_extra_chk CHECK (jsonb_typeof(extra) = 'object'),
  UNIQUE (tenant_id, collection_id, identity_ref)
);

CREATE INDEX IF NOT EXISTS subscriber_identity_evidence_idx
  ON forensic.subscriber_identity (tenant_id, collection_id, evidence_id, source_row);

COMMENT ON TABLE forensic.subscriber_identity IS
  'PII isolated from registration facts. No privilege is granted to forensic_runtime: reads go through a masked projection once R9 is approved.';

-- ---------------------------------------------------------------------------------------------------------------------
-- subscriber_registration: valid-time registration facts, without personal identity fields. Conflicting rows are kept,
-- never merged: a statement is always "registered per source X at time T", never identity or ownership.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.subscriber_registration (
  registration_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL,
  evidence_id uuid NOT NULL,
  file_id text NOT NULL,
  batch_id uuid NOT NULL,
  source_row bigint NOT NULL,
  record_id uuid,
  record_ts timestamptz,
  row_hash text NOT NULL,
  layout_id text NOT NULL,
  layout_version integer NOT NULL,
  msisdn text NOT NULL,
  msisdn_raw text,
  subscriber_ref text,
  imsi text,
  iccid text,
  status text,
  operator_label text,
  service_type text,
  activated_at timestamptz,
  deactivated_at timestamptz,
  valid_from timestamptz,
  valid_to timestamptz,
  time_basis text NOT NULL,
  source_tz text,
  identity_ref uuid,
  raw_payload_redacted jsonb NOT NULL DEFAULT '{}'::jsonb,
  quality_flags text[] NOT NULL DEFAULT '{}',
  ingested_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, collection_id, file_id, row_hash),
  CONSTRAINT subscriber_registration_layout_fk FOREIGN KEY (layout_id, layout_version) REFERENCES forensic.layout_versions (layout_id, layout_version),
  CONSTRAINT subscriber_registration_evidence_fk FOREIGN KEY (tenant_id, collection_id, evidence_id)
    REFERENCES forensic.evidence_items (tenant_id, collection_id, evidence_id),
  CONSTRAINT subscriber_registration_identity_fk FOREIGN KEY (tenant_id, collection_id, identity_ref)
    REFERENCES forensic.subscriber_identity (tenant_id, collection_id, identity_ref),
  CONSTRAINT subscriber_registration_payload_chk CHECK (jsonb_typeof(raw_payload_redacted) = 'object'),
  CONSTRAINT subscriber_registration_time_basis_chk CHECK (time_basis IN
    ('declared', 'assumed_by_profile', 'confirmed_by_analyst', 'explicit_offset')),
  CONSTRAINT subscriber_registration_valid_chk CHECK (valid_from IS NULL OR valid_to IS NULL OR valid_to >= valid_from)
);

CREATE INDEX IF NOT EXISTS subscriber_registration_msisdn_idx
  ON forensic.subscriber_registration (tenant_id, collection_id, msisdn, valid_from);
CREATE INDEX IF NOT EXISTS subscriber_registration_imsi_idx
  ON forensic.subscriber_registration (tenant_id, collection_id, imsi) WHERE imsi IS NOT NULL;
CREATE INDEX IF NOT EXISTS subscriber_registration_identity_idx
  ON forensic.subscriber_registration (identity_ref) WHERE identity_ref IS NOT NULL;

COMMENT ON TABLE forensic.subscriber_registration IS
  'Who a number was registered to, and when, per source. A statement is always "registered per source X at time T", never identity or ownership.';
COMMENT ON COLUMN forensic.subscriber_registration.raw_payload_redacted IS
  'The source row with identity columns (name, national ID number, address) removed; those live in forensic.subscriber_identity.';

-- ---------------------------------------------------------------------------------------------------------------------
-- Row-level security: the same tenant policy as every other forensic table.
-- ---------------------------------------------------------------------------------------------------------------------
DO $$
DECLARE
  t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['comm_event', 'data_session', 'app_usage', 'plate_sighting', 'subscriber_registration', 'subscriber_identity']
  LOOP
    EXECUTE format('ALTER TABLE forensic.%I ENABLE ROW LEVEL SECURITY', t);
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = t AND policyname = 'tenant_isolation_' || t) THEN
      EXECUTE format(
        'CREATE POLICY %I ON forensic.%I USING (tenant_id = current_setting(''app.tenant_id'', true)) WITH CHECK (tenant_id = current_setting(''app.tenant_id'', true))',
        'tenant_isolation_' || t, t);
    END IF;
  END LOOP;
END
$$;

-- ---------------------------------------------------------------------------------------------------------------------
-- Privileges for the runtime role, if it exists. Event rows are immutable: INSERT and SELECT only, never UPDATE or DELETE.
-- ---------------------------------------------------------------------------------------------------------------------
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    GRANT SELECT ON forensic.layout_versions TO forensic_runtime;
    GRANT INSERT ON forensic.layout_versions TO forensic_runtime;
    GRANT SELECT, INSERT ON forensic.comm_event, forensic.data_session, forensic.app_usage, forensic.plate_sighting,
      forensic.subscriber_registration TO forensic_runtime;
    REVOKE ALL ON forensic.subscriber_identity FROM forensic_runtime;
  END IF;
END
$$;

COMMIT;
