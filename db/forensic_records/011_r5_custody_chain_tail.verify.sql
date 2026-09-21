BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  definition text;
BEGIN
  SELECT pg_get_functiondef('forensic.prepare_custody_event()'::regprocedure)
    INTO definition;
  IF definition NOT LIKE '%child.previous_event_sha256 = event.event_sha256%' THEN
    RAISE EXCEPTION 'migration 011 custody tail selection is not active';
  END IF;
END
$$;

ROLLBACK;
