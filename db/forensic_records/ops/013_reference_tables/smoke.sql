-- Synthetic acceptance checks for migration 013. DISPOSABLE DATABASE ONLY; one rolled-back transaction.
-- Proves the as-of lookup rule on cell sites, the integrity rules, address-range containment and tenant isolation.
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

SELECT set_config('app.tenant_id', 'ref-smoke', true);

-- A cell that moved: two rows for the same key from the same source, a retired row for another.
INSERT INTO forensic.cell_site (tenant_id, cell_key, plmn, lac_tac, ci_eci, lat, lon, radius_m, status, valid_from, source) VALUES
  ('ref-smoke', forensic.make_cell_key('00101', '100', '200'), '00101', '100', '200', 10.0, 20.0, 1500, 'active',
   '2025-01-01 00:00:00+00', 'synthetic-list'),
  ('ref-smoke', forensic.make_cell_key('00101', '100', '200'), '00101', '100', '200', 10.5, 20.5, 900, 'active',
   '2026-01-01 00:00:00+00', 'synthetic-list'),
  ('ref-smoke', forensic.make_cell_key('00101', '100', '300'), '00101', '100', '300', 11.0, 21.0, 1000, 'retired',
   '2025-01-01 00:00:00+00', 'synthetic-list');

DO $$
DECLARE
  got record;
BEGIN
  -- As-of lookup: the row with the greatest valid_from not later than the event time.
  SELECT lat, lon INTO got
  FROM forensic.cell_site
  WHERE tenant_id = 'ref-smoke' AND cell_key = '00101:100:200' AND valid_from <= '2025-06-01 00:00:00+00'
  ORDER BY source_rank, valid_from DESC LIMIT 1;
  IF got.lat IS DISTINCT FROM 10.0 THEN RAISE EXCEPTION 'as-of lookup before the move returned %', got.lat; END IF;

  SELECT lat, lon INTO got
  FROM forensic.cell_site
  WHERE tenant_id = 'ref-smoke' AND cell_key = '00101:100:200' AND valid_from <= '2026-03-01 00:00:00+00'
  ORDER BY source_rank, valid_from DESC LIMIT 1;
  IF got.lat IS DISTINCT FROM 10.5 THEN RAISE EXCEPTION 'as-of lookup after the move returned %', got.lat; END IF;

  IF (SELECT count(*) FROM forensic.cell_site WHERE status = 'retired') <> 1 THEN
    RAISE EXCEPTION 'retired cell row was not kept';
  END IF;
END
$$;

SELECT pg_temp.expect_error('cell key that does not match its parts', $q$
  INSERT INTO forensic.cell_site (tenant_id, cell_key, plmn, lac_tac, ci_eci, source)
  VALUES ('ref-smoke', '00101:100:999', '00101', '100', '200', 'synthetic-list')$q$, '23514');
SELECT pg_temp.expect_error('latitude out of range', $q$
  INSERT INTO forensic.cell_site (tenant_id, cell_key, plmn, lac_tac, ci_eci, lat, lon, source)
  VALUES ('ref-smoke', '00101:100:201', '00101', '100', '201', 95, 20, 'synthetic-list')$q$, '23514');
SELECT pg_temp.expect_error('azimuth of 360 degrees', $q$
  INSERT INTO forensic.cell_site (tenant_id, cell_key, plmn, lac_tac, ci_eci, azimuth_deg, source)
  VALUES ('ref-smoke', '00101:100:202', '00101', '100', '202', 360, 'synthetic-list')$q$, '23514');
SELECT pg_temp.expect_error('same cell, source and start twice', $q$
  INSERT INTO forensic.cell_site (tenant_id, cell_key, plmn, lac_tac, ci_eci, source, valid_from)
  VALUES ('ref-smoke', '00101:100:200', '00101', '100', '200', 'synthetic-list', '2025-01-01 00:00:00+00')$q$, '23505');
SELECT pg_temp.expect_error('valid_to before valid_from', $q$
  INSERT INTO forensic.cell_site (tenant_id, cell_key, plmn, lac_tac, ci_eci, source, valid_from, valid_to)
  VALUES ('ref-smoke', '00101:100:203', '00101', '100', '203', 'synthetic-list', '2026-01-01 00:00:00+00', '2025-01-01 00:00:00+00')$q$, '23514');
SELECT pg_temp.expect_error('TAC with seven digits', $q$
  INSERT INTO forensic.device_tac (tenant_id, tac, source) VALUES ('ref-smoke', '1234567', 'synthetic-list')$q$, '23514');
SELECT pg_temp.expect_error('lower-case country code', $q$
  INSERT INTO forensic.ip_range_reference (tenant_id, network, country_code, source) VALUES ('ref-smoke', '203.0.113.0/24', 'pk', 'synthetic-list')$q$, '23514');
SELECT pg_temp.expect_error('camera with a heading of 400 degrees', $q$
  INSERT INTO forensic.camera_site (tenant_id, camera_id, heading_deg, source) VALUES ('ref-smoke', 'camera-x', 400, 'synthetic-list')$q$, '23514');

-- Address-range containment, most specific range first. Documentation ranges only.
INSERT INTO forensic.ip_range_reference (tenant_id, network, org_name, asn, country_code, category, source) VALUES
  ('ref-smoke', '203.0.113.0/24', 'Synthetic Wide Org', 64496, 'PK', 'isp', 'synthetic-list'),
  ('ref-smoke', '203.0.113.128/25', 'Synthetic Narrow Org', 64497, 'PK', 'cdn', 'synthetic-list'),
  ('ref-smoke', '2001:db8::/32', 'Synthetic V6 Org', 64498, 'PK', 'cloud', 'synthetic-list');

DO $$
DECLARE
  org text;
BEGIN
  SELECT org_name INTO org FROM forensic.ip_range_reference
  WHERE tenant_id = 'ref-smoke' AND network >>= '203.0.113.200'::inet ORDER BY masklen(network) DESC LIMIT 1;
  IF org IS DISTINCT FROM 'Synthetic Narrow Org' THEN RAISE EXCEPTION 'most specific range lookup returned %', org; END IF;

  SELECT org_name INTO org FROM forensic.ip_range_reference
  WHERE tenant_id = 'ref-smoke' AND network >>= '203.0.113.5'::inet ORDER BY masklen(network) DESC LIMIT 1;
  IF org IS DISTINCT FROM 'Synthetic Wide Org' THEN RAISE EXCEPTION 'wide range lookup returned %', org; END IF;

  SELECT org_name INTO org FROM forensic.ip_range_reference
  WHERE tenant_id = 'ref-smoke' AND network >>= '2001:db8::10'::inet ORDER BY masklen(network) DESC LIMIT 1;
  IF org IS DISTINCT FROM 'Synthetic V6 Org' THEN RAISE EXCEPTION 'IPv6 range lookup returned %', org; END IF;
END
$$;

-- Tenant isolation for a non-owner role without BYPASSRLS.
CREATE ROLE ref_smoke_rls NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS;
GRANT USAGE ON SCHEMA forensic TO ref_smoke_rls;
GRANT SELECT, INSERT ON forensic.cell_site, forensic.camera_site, forensic.device_tac, forensic.ip_range_reference TO ref_smoke_rls;

SET LOCAL ROLE ref_smoke_rls;
SELECT set_config('app.tenant_id', 'ref-smoke-other', true);

DO $$
BEGIN
  IF (SELECT count(*) FROM forensic.cell_site) <> 0 OR (SELECT count(*) FROM forensic.ip_range_reference) <> 0 THEN
    RAISE EXCEPTION 'another tenant can see reference rows';
  END IF;
END
$$;

SELECT pg_temp.expect_error('cross-tenant reference insert', $q$
  INSERT INTO forensic.device_tac (tenant_id, tac, source) VALUES ('ref-smoke', '35000000', 'synthetic-list')$q$, '42501');

RESET ROLE;

SELECT 'migration_013_smoke_passed' AS result;

ROLLBACK;
