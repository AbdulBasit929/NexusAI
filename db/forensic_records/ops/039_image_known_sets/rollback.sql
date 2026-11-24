-- Destructive rollback for 039_image_known_sets.sql: drops the two functions, forensic.image_known_hash AND every hash in it, forensic.image_known_set AND every set,
-- and forensic.image_known_event AND the audit trail in it.
-- The events are an audit trail and cannot be recomputed: export forensic.image_known_event first if it matters. The hashes can be re-imported only from the original lists.
-- Evidence, forensic.image_hash (038) and every search table are NOT touched. Run only from a verified backup, with ON_ERROR_STOP enabled and an explicit:
--   SET app.image_known_sets_allow_destructive_rollback = 'true';
BEGIN;

DO $$
DECLARE
  sets bigint := 0;
  hashes bigint := 0;
  events bigint := 0;
BEGIN
  IF current_setting('app.image_known_sets_allow_destructive_rollback', true) IS DISTINCT FROM 'true' THEN
    RAISE EXCEPTION 'the 039 rollback is destructive; set app.image_known_sets_allow_destructive_rollback=true explicitly'
      USING ERRCODE = '55000';
  END IF;
  IF to_regclass('forensic.image_known_set') IS NOT NULL THEN
    EXECUTE 'SELECT count(*) FROM forensic.image_known_set' INTO sets;
  END IF;
  IF to_regclass('forensic.image_known_hash') IS NOT NULL THEN
    EXECUTE 'SELECT count(*) FROM forensic.image_known_hash' INTO hashes;
  END IF;
  IF to_regclass('forensic.image_known_event') IS NOT NULL THEN
    EXECUTE 'SELECT count(*) FROM forensic.image_known_event' INTO events;
  END IF;
  RAISE NOTICE 'dropping % sets, % hashes and % audit events', sets, hashes, events;
END
$$;

DROP FUNCTION IF EXISTS forensic.image_known_state(text, text);
DROP FUNCTION IF EXISTS forensic.image_known_matches(text, integer, text, uuid[]);
DROP TABLE IF EXISTS forensic.image_known_event;
DROP TABLE IF EXISTS forensic.image_known_hash;
DROP TABLE IF EXISTS forensic.image_known_set;

COMMIT;
