-- Migration 028 (DRAFT, not applied): analyst views over subscriber registrations and the device reference, docs/data/subscriber-device-family-20261028.md (P1-B, second and third family).
--
-- Why. "Who was this number registered to, and when", "did the number stay active after its registration ended", "what make and model of handset did it use" are ordinary questions of a
-- telecom inquiry, and today a call-to-subscriber join matches nothing because no registration is typed. The tables exist since migrations 012 and 013 (forensic.subscriber_registration, filled by the
-- production typed ingest from the layout subscriber-s1 and later layouts; forensic.device_tac, filled by the reference loader); the Ask engine reaches data only through reviewed analyst views
-- (migration 017), so this adds three.
--
-- What it adds:
--   * analyst.v_subscriber_registrations      one row per registration of a number, per source file: the SIM (IMSI, ICCID), the status, the operator and service type, the validity (activation to
--                                              deactivation), and `conflicts` when another source file says something else for an overlapping period.
--   * analyst.v_number_registration_activity  per number: how many registrations and sources, the span they cover, and how many events of the number (calls and messages in the case's call records)
--                                              fall OUTSIDE every registration period and after the last registration ended (the thick view for "active after deactivation").
--   * analyst.v_handsets                      per number and handset (IMEI, IMSI) from the call records, with the TAC (the first eight digits of the IMEI) and the make and model the device
--                                              reference lists for it (the thick view for "what phone did this number use").
-- It also lets a registration row carry NO time basis (a registration may have no dates at all; time_basis stays what the typed writer knows, NULL when there is no date to read a zone for).
--
-- Registrations are facts of a source, not of the world. A row says "this number was registered per this source as this SIM with this status in this period"; it does not say who holds or uses the
-- number, and two sources may differ (both are kept; `conflicts` marks them). Identity (name, national ID number, address) is NOT in any view and is not stored: the typed writer drops those columns
-- and the runtime role has no privilege on forensic.subscriber_identity (migration 012) until the masking design is approved. A TAC list is community-collected: a model is "listed as", never a fact
-- about the individual handset.
--
-- Scope is STRUCTURAL, as in migration 017: each view takes `tenant_id = current_setting('app.tenant_id', true)` and, for case data, `collection_id = current_setting('app.collection_id', true)`; an unset
-- setting is NULL and the view returns no rows (fail closed). The device reference is tenant-wide (one list for every case of the tenant). Withheld on purpose: raw payloads, file hashes, row hashes.
--
-- Requires migrations 012, 013 and 017. Idempotent. Apply by hand on a retained database: ops/028_subscriber_device_views/preflight.sql first, verify.sql after; rollback.sql drops the three views.
BEGIN;
SET LOCAL lock_timeout = '10s';

-- A registration without any date has no zone to know: time_basis may be NULL (the writer sets it only when there is a time to read). The values stay those of migration 012.
ALTER TABLE forensic.subscriber_registration ALTER COLUMN time_basis DROP NOT NULL;
ALTER TABLE forensic.subscriber_registration DROP CONSTRAINT IF EXISTS subscriber_registration_time_basis_chk;
ALTER TABLE forensic.subscriber_registration ADD CONSTRAINT subscriber_registration_time_basis_chk
  CHECK (time_basis IS NULL OR time_basis IN ('declared', 'assumed_by_profile', 'confirmed_by_analyst', 'explicit_offset'));

CREATE OR REPLACE VIEW analyst.v_subscriber_registrations AS
SELECT
  r.registration_id,
  r.evidence_id,
  r.file_id AS source_file,
  r.source_row,
  r.msisdn,
  r.imsi,
  r.iccid,
  r.subscriber_ref,
  r.status,
  r.operator_label,
  r.service_type,
  r.valid_from,
  r.valid_to,
  r.time_basis,
  r.source_tz,
  -- another source file states something else (a different SIM or status) for a period that overlaps this one; a missing start or end is an open end
  EXISTS (
    SELECT 1 FROM forensic.subscriber_registration o
    WHERE o.tenant_id = r.tenant_id AND o.collection_id = r.collection_id AND o.msisdn = r.msisdn AND o.file_id <> r.file_id
      AND NOT ('duplicate_row' = ANY (o.quality_flags))
      AND coalesce(o.valid_from, '-infinity'::timestamptz) < coalesce(r.valid_to, 'infinity'::timestamptz)
      AND coalesce(r.valid_from, '-infinity'::timestamptz) < coalesce(o.valid_to, 'infinity'::timestamptz)
      AND (o.imsi IS DISTINCT FROM r.imsi OR o.status IS DISTINCT FROM r.status)
  ) AS conflicts
FROM forensic.subscriber_registration r
WHERE r.tenant_id = current_setting('app.tenant_id', true)
  AND r.collection_id = current_setting('app.collection_id', true)
  AND NOT ('duplicate_row' = ANY (r.quality_flags));

CREATE OR REPLACE VIEW analyst.v_number_registration_activity AS
SELECT
  n.msisdn,
  n.registrations,
  n.sources,
  n.first_valid_from,
  n.last_valid_to,
  n.conflicts,
  coalesce(a.events, 0) AS events,
  coalesce(a.events_outside, 0) AS events_outside_registration,
  coalesce(a.events_after_end, 0) AS events_after_last_end,
  a.first_event_utc,
  a.last_event_utc
FROM (
  SELECT
    r.msisdn,
    count(*) AS registrations,
    count(DISTINCT r.source_file) AS sources,
    min(r.valid_from) AS first_valid_from,
    -- an open end on any registration means the number was never ended: no last end
    CASE WHEN bool_or(r.valid_to IS NULL) THEN NULL ELSE max(r.valid_to) END AS last_valid_to,
    bool_or(r.conflicts) AS conflicts
  FROM analyst.v_subscriber_registrations r
  GROUP BY r.msisdn
) n
LEFT JOIN LATERAL (
  SELECT
    count(*) AS events,
    count(*) FILTER (WHERE NOT EXISTS (
      SELECT 1 FROM analyst.v_subscriber_registrations x
      WHERE x.msisdn = n.msisdn
        AND (x.valid_from IS NULL OR x.valid_from <= e.start_utc)
        AND (x.valid_to IS NULL OR e.start_utc < x.valid_to)
    )) AS events_outside,
    count(*) FILTER (WHERE n.last_valid_to IS NOT NULL AND e.start_utc >= n.last_valid_to) AS events_after_end,
    min(e.start_utc) AS first_event_utc,
    max(e.start_utc) AS last_event_utc
  FROM analyst.v_comm_events e
  WHERE e.src = n.msisdn
) a ON true;

CREATE OR REPLACE VIEW analyst.v_handsets AS
SELECT
  p.src,
  p.imei,
  p.imsi,
  p.events,
  p.first_seen,
  p.last_seen,
  CASE WHEN p.imei ~ '^[0-9]{14,16}$' THEN left(p.imei, 8) END AS tac,
  t.make,
  t.model,
  t.source AS listed_by,
  (t.tac IS NOT NULL) AS tac_listed
FROM analyst.v_device_pairings p
LEFT JOIN LATERAL (
  SELECT d.tac, d.make, d.model, d.source
  FROM forensic.device_tac d
  WHERE d.tenant_id = current_setting('app.tenant_id', true)
    AND p.imei ~ '^[0-9]{14,16}$'
    AND d.tac = left(p.imei, 8)
  ORDER BY d.source
  LIMIT 1
) t ON true
WHERE p.imei IS NOT NULL;

COMMENT ON VIEW analyst.v_subscriber_registrations IS
  'Registrations of numbers per source file: this number was registered, per this source, as this SIM (IMSI, ICCID) with this status from activation to deactivation. A statement is "registered per source", never who holds or uses the number; two sources can disagree (both rows are kept, conflicts marks them). No names, national ID numbers or addresses are held.';
COMMENT ON VIEW analyst.v_number_registration_activity IS
  'Per number: its registrations and sources, the span they cover, and how many of its events in the case''s call records fall outside every registration period and after the last registration ended. For whether a number was active when it was not registered. Outside a registration period is a pattern in the rows, not a finding.';
COMMENT ON VIEW analyst.v_handsets IS
  'Per number and handset (IMEI, IMSI) seen in the call records, with the TAC (first eight digits of the IMEI) and the make and model the device reference lists for it. A TAC is listed as a make and model per source: it is not a fact about the individual handset. For what phone a number used and which numbers use a model.';

DO $$
DECLARE
  d record;
BEGIN
  FOR d IN
    SELECT * FROM (VALUES
      ('v_subscriber_registrations', 'registration_id', 'Identifier of the registration row.'),
      ('v_subscriber_registrations', 'evidence_id', 'The evidence file the registration came from.'),
      ('v_subscriber_registrations', 'source_file', 'The file the registration came from (the source it is registered per).'),
      ('v_subscriber_registrations', 'source_row', 'The row of that file.'),
      ('v_subscriber_registrations', 'msisdn', 'The registered number, normalised: 92 and ten digits for a Pakistani mobile number. Always compare this column, with the number written the same way.'),
      ('v_subscriber_registrations', 'imsi', 'The IMSI of the SIM the number was registered on, when the source has one.'),
      ('v_subscriber_registrations', 'iccid', 'The SIM serial (ICCID) when the source has one.'),
      ('v_subscriber_registrations', 'subscriber_ref', 'The source''s own reference for the subscriber or account, when it has one (not a name).'),
      ('v_subscriber_registrations', 'status', 'The registration status in plain words: active, suspended, deactivated, or unknown when the source''s word is not one of those.'),
      ('v_subscriber_registrations', 'operator_label', 'The operator the source names, when it names one.'),
      ('v_subscriber_registrations', 'service_type', 'The service type the source names (prepaid or postpaid, in lower case), when it names one.'),
      ('v_subscriber_registrations', 'valid_from', 'When the registration began (the activation), UTC; empty when the source has no activation date (an open start).'),
      ('v_subscriber_registrations', 'valid_to', 'When the registration ended (the deactivation), UTC; empty when it has not ended or the source has no date (an open end).'),
      ('v_subscriber_registrations', 'time_basis', 'How the zone of the dates is known: declared, assumed_by_profile, confirmed_by_analyst or explicit_offset; empty when the row has no date.'),
      ('v_subscriber_registrations', 'source_tz', 'The time zone the source''s dates were read in.'),
      ('v_subscriber_registrations', 'conflicts', 'True when another source file states a different SIM or status for the same number in an overlapping period. Both rows are kept; say that the sources differ.'),
      ('v_number_registration_activity', 'msisdn', 'The number, normalised (92 and ten digits).'),
      ('v_number_registration_activity', 'registrations', 'How many registration rows the number has (all sources).'),
      ('v_number_registration_activity', 'sources', 'How many different source files registered the number.'),
      ('v_number_registration_activity', 'first_valid_from', 'The earliest registration start of the number, UTC; empty when no registration has a start date.'),
      ('v_number_registration_activity', 'last_valid_to', 'The latest registration end of the number, UTC; empty when any registration of the number is open-ended (the number was never ended).'),
      ('v_number_registration_activity', 'conflicts', 'True when any registration of the number conflicts with another source.'),
      ('v_number_registration_activity', 'events', 'How many events (calls and messages) of the number are in the case''s call records.'),
      ('v_number_registration_activity', 'events_outside_registration', 'How many of those events fall outside every registration period of the number (a registration with no start or end is open on that side).'),
      ('v_number_registration_activity', 'events_after_last_end', 'How many of those events are at or after the end of the last registration; zero when a registration of the number is open-ended.'),
      ('v_number_registration_activity', 'first_event_utc', 'Time of the number''s first event in the call records, UTC.'),
      ('v_number_registration_activity', 'last_event_utc', 'Time of the number''s last event in the call records, UTC.'),
      ('v_handsets', 'src', 'The subscriber number the handset was seen with.'),
      ('v_handsets', 'imei', 'The handset identity (IMEI) as the call records have it.'),
      ('v_handsets', 'imsi', 'The SIM identity (IMSI) seen with that handset and number.'),
      ('v_handsets', 'events', 'How many events the number made with this handset and SIM.'),
      ('v_handsets', 'first_seen', 'Time of the first event with this handset, UTC.'),
      ('v_handsets', 'last_seen', 'Time of the last event with this handset, UTC.'),
      ('v_handsets', 'tac', 'The Type Allocation Code: the first eight digits of the IMEI; empty when the IMEI is not 14 to 16 digits.'),
      ('v_handsets', 'make', 'The make the device reference lists for the TAC; empty when the TAC is not listed.'),
      ('v_handsets', 'model', 'The model (with its variant and year when the list has them) the device reference lists for the TAC; empty when the TAC is not listed. Listed as, not a fact about this handset.'),
      ('v_handsets', 'listed_by', 'The source of the reference list that names the model.'),
      ('v_handsets', 'tac_listed', 'True when the device reference lists the TAC; false means the model is unknown, not that the handset is unusual.')
    ) AS t(view_name, column_name, description)
  LOOP
    EXECUTE format('COMMENT ON COLUMN analyst.%I.%I IS %L', d.view_name, d.column_name, d.description);
  END LOOP;
END
$$;

-- The role reads the new views and nothing else: SELECT is granted view by view; no other privilege on them.
REVOKE ALL ON analyst.v_subscriber_registrations, analyst.v_number_registration_activity, analyst.v_handsets FROM PUBLIC;
GRANT SELECT ON analyst.v_subscriber_registrations, analyst.v_number_registration_activity, analyst.v_handsets TO forensic_analyst_ro;
COMMIT;
