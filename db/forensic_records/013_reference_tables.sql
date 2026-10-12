-- Migration 013 (DRAFT, not applied): reference tables used to enrich events.
--
-- Cell sites with coordinates and coverage, camera sites, handset types by TAC, and address ranges by organisation. All are
-- time-valid, append-only and tenant-scoped (a tenant may hold licensed or case-specific lists), each row names its source
-- and licence, and none is ever overwritten: a changed site is a new row with a later valid_from.
--
-- As-of lookup rule (used by the enrichment step and the query views): among rows for the same key, take the highest-priority
-- source, then the row with the greatest valid_from that is not later than the event time; treat the key as unknown when that
-- row has a valid_to earlier than the event time or status 'retired'.
--
-- Additive and idempotent. Apply by hand per db/forensic_records/README.md, with ops/013_reference_tables/ scripts.
BEGIN;
-- Fail fast instead of queueing behind a long transaction: a DDL statement that waits for a lock also blocks every
-- writer queued behind it. If a lock cannot be had within ten seconds the whole migration rolls back; run it again later.
SET LOCAL lock_timeout = '10s';

-- ---------------------------------------------------------------------------------------------------------------------
-- Cell key: one canonical text key per cell, built from the parts the layout parser produced. The area code and cell
-- identity lose leading zeros when numeric and are lower-cased otherwise, the PLMN keeps its zeros, and a missing part
-- becomes '?'. The key does not guess: hexadecimal identifiers must be converted to decimal by the layout's cell parser
-- before the key is built, and the same function is used on both sides of every join.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION forensic.norm_cell_part(part text)
RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE
AS $$
  SELECT CASE
    WHEN nullif(btrim(part), '') IS NULL THEN NULL
    WHEN btrim(part) ~ '^[0-9]+$' THEN coalesce(nullif(ltrim(btrim(part), '0'), ''), '0')
    ELSE lower(btrim(part))
  END
$$;

CREATE OR REPLACE FUNCTION forensic.make_cell_key(plmn text, lac_tac text, ci text)
RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE
AS $$
  -- The PLMN (mobile country code plus network code) is a fixed-width code, so its leading zeros are kept.
  SELECT CASE
    WHEN forensic.norm_cell_part(ci) IS NULL THEN NULL
    ELSE coalesce(nullif(lower(btrim(plmn)), ''), '?') || ':' || coalesce(forensic.norm_cell_part(lac_tac), '?') || ':' || forensic.norm_cell_part(ci)
  END
$$;

COMMENT ON FUNCTION forensic.make_cell_key(text, text, text) IS
  'Canonical cell key plmn:lac_tac:ci. NULL when the cell identity is missing; hexadecimal parts must be converted by the layout parser.';

-- ---------------------------------------------------------------------------------------------------------------------
-- cell_site: where a cell is and what it covers.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.cell_site (
  cell_site_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL DEFAULT '',
  cell_key text NOT NULL,
  operator_label text,
  plmn text,
  lac_tac text,
  ci_eci text NOT NULL,
  sector text,
  rat text,
  site_name text,
  lat double precision,
  lon double precision,
  azimuth_deg real,
  beamwidth_deg real,
  radius_m integer,
  status text NOT NULL DEFAULT 'active',
  valid_from timestamptz NOT NULL DEFAULT 'epoch',
  valid_to timestamptz,
  source text NOT NULL,
  source_rank smallint NOT NULL DEFAULT 100,
  source_license text,
  evidence_id uuid REFERENCES forensic.evidence_items (evidence_id),
  retrieved_at timestamptz,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT cell_site_key_chk CHECK (cell_key = forensic.make_cell_key(plmn, lac_tac, ci_eci)),
  CONSTRAINT cell_site_rat_chk CHECK (rat IS NULL OR rat IN ('gsm', 'umts', 'lte', 'nr', 'other')),
  CONSTRAINT cell_site_status_chk CHECK (status IN ('active', 'retired', 'planned')),
  CONSTRAINT cell_site_coord_chk CHECK (
    (lat IS NULL AND lon IS NULL)
    OR (lat IS NOT NULL AND lon IS NOT NULL AND lat BETWEEN -90 AND 90 AND lon BETWEEN -180 AND 180)),
  CONSTRAINT cell_site_geometry_chk CHECK (
    (azimuth_deg IS NULL OR (azimuth_deg >= 0 AND azimuth_deg < 360))
    AND (beamwidth_deg IS NULL OR (beamwidth_deg > 0 AND beamwidth_deg <= 360))
    AND (radius_m IS NULL OR radius_m > 0)),
  CONSTRAINT cell_site_valid_chk CHECK (valid_to IS NULL OR valid_to > valid_from),
  UNIQUE (tenant_id, collection_id, cell_key, source, valid_from)
);

CREATE INDEX IF NOT EXISTS cell_site_lookup_idx
  ON forensic.cell_site (tenant_id, cell_key, valid_from DESC);

COMMENT ON TABLE forensic.cell_site IS
  'Time-valid cell reference (location, sector, coverage radius). collection_id is empty for tenant-wide lists. Append-only: a changed site is a new row.';
COMMENT ON COLUMN forensic.cell_site.radius_m IS
  'Stated coverage radius or uncertainty. A cell is not a position: answers that use it must show this radius.';
COMMENT ON COLUMN forensic.cell_site.source_rank IS 'Lower is preferred when several sources describe the same cell at the same time.';

-- ---------------------------------------------------------------------------------------------------------------------
-- camera_site: where a plate-reading camera stands (the camera registry).
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.camera_site (
  camera_site_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  collection_id text NOT NULL DEFAULT '',
  camera_id text NOT NULL,
  label text,
  site_name text,
  lat double precision,
  lon double precision,
  heading_deg real,
  status text NOT NULL DEFAULT 'active',
  valid_from timestamptz NOT NULL DEFAULT 'epoch',
  valid_to timestamptz,
  source text NOT NULL,
  source_license text,
  evidence_id uuid REFERENCES forensic.evidence_items (evidence_id),
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT camera_site_id_chk CHECK (camera_id <> ''),
  CONSTRAINT camera_site_status_chk CHECK (status IN ('active', 'retired', 'planned')),
  CONSTRAINT camera_site_coord_chk CHECK (
    (lat IS NULL AND lon IS NULL)
    OR (lat IS NOT NULL AND lon IS NOT NULL AND lat BETWEEN -90 AND 90 AND lon BETWEEN -180 AND 180)),
  CONSTRAINT camera_site_heading_chk CHECK (heading_deg IS NULL OR (heading_deg >= 0 AND heading_deg < 360)),
  CONSTRAINT camera_site_valid_chk CHECK (valid_to IS NULL OR valid_to > valid_from),
  UNIQUE (tenant_id, collection_id, camera_id, source, valid_from)
);

CREATE INDEX IF NOT EXISTS camera_site_lookup_idx
  ON forensic.camera_site (tenant_id, camera_id, valid_from DESC);

COMMENT ON TABLE forensic.camera_site IS
  'Time-valid camera registry. Gives plate sightings a place when the source file carries none (loc_method camera_registry).';

-- ---------------------------------------------------------------------------------------------------------------------
-- device_tac: handset make and model by Type Allocation Code (the first eight digits of an IMEI).
-- Licence matters: the full GSMA database is licensed; load only a list whose terms allow it, and record them in source_license.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.device_tac (
  tenant_id text NOT NULL DEFAULT 'default',
  tac text NOT NULL,
  source text NOT NULL,
  make text,
  model text,
  marketing_name text,
  device_type text,
  source_license text,
  retrieved_at timestamptz,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, tac, source),
  CONSTRAINT device_tac_tac_chk CHECK (tac ~ '^[0-9]{8}$')
);

COMMENT ON TABLE forensic.device_tac IS
  'Handset make and model per TAC and source. A statement from it is "the TAC is listed as make/model per source", never a fact about the individual device.';

-- ---------------------------------------------------------------------------------------------------------------------
-- ip_range_reference: address ranges by organisation, autonomous system and country (open registries).
-- ---------------------------------------------------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS forensic.ip_range_reference (
  range_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  tenant_id text NOT NULL DEFAULT 'default',
  network cidr NOT NULL,
  org_name text,
  asn bigint,
  country_code text,
  category text,
  source text NOT NULL,
  source_license text,
  valid_from timestamptz NOT NULL DEFAULT 'epoch',
  valid_to timestamptz,
  retrieved_at timestamptz,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT ip_range_reference_country_chk CHECK (country_code IS NULL OR country_code ~ '^[A-Z]{2}$'),
  CONSTRAINT ip_range_reference_asn_chk CHECK (asn IS NULL OR asn BETWEEN 0 AND 4294967295),
  CONSTRAINT ip_range_reference_category_chk CHECK (category IS NULL OR category IN
    ('isp', 'mobile', 'cdn', 'cloud', 'hosting', 'enterprise', 'education', 'government', 'other')),
  CONSTRAINT ip_range_reference_valid_chk CHECK (valid_to IS NULL OR valid_to > valid_from),
  UNIQUE (tenant_id, network, source, valid_from)
);

CREATE INDEX IF NOT EXISTS ip_range_reference_network_idx
  ON forensic.ip_range_reference USING gist (network inet_ops);

COMMENT ON TABLE forensic.ip_range_reference IS
  'Address ranges by organisation, AS number and country. Lookup form: WHERE network >>= <address>, most specific range first.';

-- ---------------------------------------------------------------------------------------------------------------------
-- Row-level security and privileges.
-- ---------------------------------------------------------------------------------------------------------------------
DO $$
DECLARE
  t text;
BEGIN
  FOREACH t IN ARRAY ARRAY['cell_site', 'camera_site', 'device_tac', 'ip_range_reference']
  LOOP
    EXECUTE format('ALTER TABLE forensic.%I ENABLE ROW LEVEL SECURITY', t);
    IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = 'forensic' AND tablename = t AND policyname = 'tenant_isolation_' || t) THEN
      EXECUTE format(
        'CREATE POLICY %I ON forensic.%I USING (tenant_id = current_setting(''app.tenant_id'', true)) WITH CHECK (tenant_id = current_setting(''app.tenant_id'', true))',
        'tenant_isolation_' || t, t);
    END IF;
  END LOOP;

  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    GRANT SELECT, INSERT ON forensic.cell_site, forensic.camera_site, forensic.device_tac, forensic.ip_range_reference
      TO forensic_runtime;
  END IF;
END
$$;

COMMIT;
