-- Run after the legacy fixture, migration 008, native smoke, and a second
-- application of migration 008. Any count drift means reapplication is unsafe.

DO $$
DECLARE
  actual bigint;
BEGIN
  SELECT count(*) INTO actual FROM forensic.evidence_storage_objects
  WHERE tenant_id = 'phase3-legacy';
  IF actual <> 1 THEN RAISE EXCEPTION 'legacy storage count drifted: %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_versions
  WHERE tenant_id = 'phase3-legacy';
  IF actual <> 1 THEN RAISE EXCEPTION 'legacy version count drifted: %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_source_links
  WHERE tenant_id = 'phase3-legacy';
  IF actual <> 3 THEN RAISE EXCEPTION 'legacy source-link count drifted: %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.processing_runs
  WHERE tenant_id = 'phase3-legacy';
  IF actual <> 1 THEN RAISE EXCEPTION 'legacy run count drifted: %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.processing_events
  WHERE tenant_id = 'phase3-legacy';
  IF actual <> 1 THEN RAISE EXCEPTION 'legacy processing-event count drifted: %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_custody_events
  WHERE tenant_id = 'phase3-legacy';
  IF actual <> 1 THEN RAISE EXCEPTION 'legacy custody-event count drifted: %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_source_links
  WHERE tenant_id = 'phase3-smoke';
  IF actual <> 2 THEN RAISE EXCEPTION 'native source-link count drifted: %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.processing_events
  WHERE tenant_id = 'phase3-smoke';
  IF actual <> 3 THEN RAISE EXCEPTION 'native processing-event count drifted: %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_custody_events
  WHERE tenant_id = 'phase3-smoke';
  IF actual <> 3 THEN RAISE EXCEPTION 'native custody-event count drifted: %', actual; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_items
  WHERE tenant_id = 'phase3-smoke'
    AND metadata->>'phase3_native_registration' = 'true';
  IF actual <> 1 THEN RAISE EXCEPTION 'native registration marker missing'; END IF;

  SELECT count(*) INTO actual FROM forensic.evidence_items
  WHERE tenant_id = 'phase3-legacy'
    AND metadata ? 'phase3_native_registration';
  IF actual <> 0 THEN RAISE EXCEPTION 'legacy registration was marked native'; END IF;
END $$;

SELECT 'phase3_control_plane_idempotency_passed' AS result;
