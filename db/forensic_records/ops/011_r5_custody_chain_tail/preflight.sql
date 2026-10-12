-- Read-only compatibility gate for deterministic custody-chain tail selection.
BEGIN TRANSACTION READ ONLY;

DO $$
BEGIN
  IF to_regclass('forensic.evidence_custody_events') IS NULL
     OR to_regprocedure('forensic.prepare_custody_event()') IS NULL THEN
    RAISE EXCEPTION 'migration 008 must be applied before migration 011';
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM pg_trigger
    WHERE tgrelid = 'forensic.evidence_custody_events'::regclass
      AND tgname = 'evidence_custody_events_prepare'
      AND NOT tgisinternal
  ) THEN
    RAISE EXCEPTION 'custody-event preparation trigger is missing';
  END IF;
END
$$;

ROLLBACK;
