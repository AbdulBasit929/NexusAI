-- Migration 025: analyst views for "which numbers went silent" and "which number may have taken over from a silent one" (link analysis stage 3b;
-- docs/data/link-analysis-stage3-preregistration-20261022.md, analysis A9 of docs/data/case-generator-spec.md section 10).
--
-- Why. A number that stops and a new number that starts a little later with the same contacts, places and perhaps the same handset is one of the commonest leads in a telecom case, and
-- the rule has many parts (silence, a start window, contact similarity, shared places, a shared handset, a score). A small model cannot write it reliably; the views state it exactly,
-- as the generator's A9 does, so a question is one line and the answer equals the oracle. BEHAVIOURAL SIMILARITY, NOT IDENTITY.
--
-- RUNTIME PARAMETERS, NOT CONSTANTS. Every number of the rule is a transaction setting the APPLICATION sets from the question (never the model, never the analyst's SQL), within bounds,
-- with the specification's value as the default when the question names none: app.silence_days (7), app.successor_window_days (14), app.successor_overlap_days (2),
-- app.successor_history_days (30), app.successor_new_after_days (7), app.successor_score_percent (35). Empty or unset gives the default; out-of-range values are clamped. Every row says
-- the values it was computed with.
--
-- Definitions (all times on the case clock, the session time zone; the case covers the whole local days from the first to the last event of the case):
--   * a number is SILENT when its last event is at least silence_days before the end of the case;
--   * a CANDIDATE successor is another number whose first event falls from successor_overlap_days before to successor_window_days after the silent number's last event, and that was not
--     already in use in the first successor_new_after_days of the case;
--   * features: the cosine of the two contact-count vectors (person-like counterparties, the silent number's last successor_history_days against the candidate's first successor_history_days,
--     the two numbers themselves left out), the Jaccard index of the cells each used at least twice in those windows, and whether they share a 14-digit IMEI at any time;
--   * score = 0.6 * cosine + 0.2 * jaccard + 0.2 * (shared handset), rounded to four decimals; a candidate is listed when score >= successor_score_percent / 100.
--
-- What is added (two views; nothing existing changes; no table is touched):
--   * analyst.v_silent_numbers        one row per silent number: when it started and stopped, its events, the days of silence at the end of the case, and the silence rule used.
--   * analyst.v_successor_candidates  one row per (silent number, candidate): old_number, new_number, when each started or stopped, the score and its three features, and the rules used.
-- Ask with `WHERE old_number = ...` (who took over from it) or `WHERE new_number = ...` (which silent number it may have replaced); the predicates reach the numbers through the joins.
--
-- Same scope as every analyst view (tenant and collection from the transaction settings, through analyst.v_comm_events), same role (forensic_analyst_ro gets SELECT on each new view
-- only). Additive and idempotent. Apply by hand on a retained database: ops/025_successor_candidates/preflight.sql first, verify.sql after; rollback.sql drops the two views.
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE OR REPLACE VIEW analyst.v_silent_numbers AS
SELECT
  n.number,
  n.first_event_utc,
  n.last_event_utc,
  n.events,
  (EXTRACT(epoch FROM (b.case_end - n.last_lt)) / 86400.0)::float8 AS days_silent,
  p.silence_days
FROM (SELECT LEAST(365, GREATEST(1, COALESCE(NULLIF(current_setting('app.silence_days', true), '')::integer, 7))) AS silence_days) p
CROSS JOIN (
  SELECT date_trunc('day', min(e.start_utc AT TIME ZONE current_setting('TimeZone'))) AS case_start,
         date_trunc('day', max(e.start_utc AT TIME ZONE current_setting('TimeZone'))) + interval '1 day' AS case_end
  FROM analyst.v_comm_events e WHERE e.src IS NOT NULL
) b
JOIN (
  SELECT e.src AS number, min(e.start_utc) AS first_event_utc, max(e.start_utc) AS last_event_utc,
         max(e.start_utc AT TIME ZONE current_setting('TimeZone')) AS last_lt, count(*)::bigint AS events
  FROM analyst.v_comm_events e WHERE e.src IS NOT NULL GROUP BY e.src
) n ON b.case_end - n.last_lt >= p.silence_days * interval '1 day';

-- One pass over the case's events. The features below look at the events of each (silent number, candidate) pair; read through analyst.v_comm_events itself, every look is a new pass over
-- the typed tables, which is invisible on a small case and took minutes on the retained database (found by the first live question after the deploy). The events are read ONCE, with the
-- case-clock time and the 14-digit handset prefix worked out once, and every feature reads that (the handset test reads the few distinct handsets of each number: a join of every pair of
-- their events took 15 seconds on 4,000 events).
CREATE OR REPLACE VIEW analyst.v_successor_candidates AS
WITH ev AS MATERIALIZED (
  SELECT e.src, e.dst, e.dst_is_person_like, e.cell_key, NULLIF(left(e.imei, 14), '') AS imei14, e.start_utc,
         (e.start_utc AT TIME ZONE current_setting('TimeZone')) AS lt
  FROM analyst.v_comm_events e
  WHERE e.src IS NOT NULL
),
-- the handsets each number used: a few rows, so that "do these two share a handset" is a join of a few rows and not of every pair of their events
dev AS MATERIALIZED (
  SELECT DISTINCT ev.src, ev.imei14 FROM ev WHERE ev.imei14 IS NOT NULL
)
SELECT
  r.old_number,
  r.new_number,
  r.old_last_event_utc,
  r.new_first_event_utc,
  r.score,
  r.contact_cosine,
  r.shared_contacts,
  r.cell_jaccard,
  r.shared_device,
  r.silence_days,
  r.window_days,
  r.score_percent
FROM (
  SELECT
    s.number AS old_number,
    c.number AS new_number,
    s.last_event_utc AS old_last_event_utc,
    c.first_event_utc AS new_first_event_utc,
    round((0.6 * f.cosine + 0.2 * f.jaccard + CASE WHEN f.shared_device THEN 0.2 ELSE 0.0 END)::numeric, 4)::float8 AS score,
    round(f.cosine::numeric, 4)::float8 AS contact_cosine,
    f.shared_contacts,
    round(f.jaccard::numeric, 4)::float8 AS cell_jaccard,
    f.shared_device,
    p.silence_days,
    p.window_days,
    p.score_percent
  FROM (
    SELECT
      LEAST(365, GREATEST(1, COALESCE(NULLIF(current_setting('app.silence_days', true), '')::integer, 7))) AS silence_days,
      LEAST(365, GREATEST(0, COALESCE(NULLIF(current_setting('app.successor_window_days', true), '')::integer, 14))) AS window_days,
      LEAST(90, GREATEST(0, COALESCE(NULLIF(current_setting('app.successor_overlap_days', true), '')::integer, 2))) AS overlap_days,
      LEAST(365, GREATEST(1, COALESCE(NULLIF(current_setting('app.successor_history_days', true), '')::integer, 30))) AS history_days,
      LEAST(365, GREATEST(0, COALESCE(NULLIF(current_setting('app.successor_new_after_days', true), '')::integer, 7))) AS new_after_days,
      LEAST(100, GREATEST(0, COALESCE(NULLIF(current_setting('app.successor_score_percent', true), '')::integer, 35))) AS score_percent
  ) p
  CROSS JOIN (
    SELECT date_trunc('day', min(ev.lt)) AS case_start, date_trunc('day', max(ev.lt)) + interval '1 day' AS case_end
    FROM ev
  ) b
  JOIN (
    SELECT ev.src AS number, min(ev.start_utc) AS first_event_utc, max(ev.start_utc) AS last_event_utc, min(ev.lt) AS first_lt, max(ev.lt) AS last_lt
    FROM ev GROUP BY ev.src
  ) s ON b.case_end - s.last_lt >= p.silence_days * interval '1 day'
  JOIN (
    SELECT ev.src AS number, min(ev.start_utc) AS first_event_utc, min(ev.lt) AS first_lt
    FROM ev GROUP BY ev.src
  ) c ON c.number <> s.number
     AND c.first_lt - b.case_start >= p.new_after_days * interval '1 day'
     AND c.first_lt BETWEEN s.last_lt - p.overlap_days * interval '1 day' AND s.last_lt + p.window_days * interval '1 day'
  CROSS JOIN LATERAL (
    SELECT
      COALESCE(ct.cosine, 0)::float8 AS cosine,
      COALESCE(ct.shared_contacts, 0)::bigint AS shared_contacts,
      COALESCE(cj.jaccard, 0)::float8 AS jaccard,
      EXISTS (
        SELECT 1
        FROM dev ex
        JOIN dev ey ON ey.imei14 = ex.imei14
        WHERE ex.src = s.number AND ey.src = c.number
      ) AS shared_device
    FROM (SELECT 1) one
    LEFT JOIN LATERAL (
      SELECT
        CASE WHEN t.nx > 0 AND t.ny > 0 THEN t.dot / (sqrt(t.nx) * sqrt(t.ny)) ELSE 0 END AS cosine,
        t.shared AS shared_contacts
      FROM (
        SELECT
          COALESCE(sum(x.cnt * y.cnt), 0)::float8 AS dot,
          COALESCE(sum(x.cnt * x.cnt), 0)::float8 AS nx,
          COALESCE(sum(y.cnt * y.cnt), 0)::float8 AS ny,
          count(*) FILTER (WHERE x.cnt IS NOT NULL AND y.cnt IS NOT NULL) AS shared
        FROM (
          SELECT e.dst, count(*) AS cnt
          FROM ev e
          WHERE e.src = s.number AND e.dst_is_person_like AND e.dst IS NOT NULL AND e.dst <> s.number AND e.dst <> c.number
            AND e.lt > s.last_lt - p.history_days * interval '1 day'
            AND e.lt <= s.last_lt
          GROUP BY e.dst
        ) x
        FULL JOIN (
          SELECT e.dst, count(*) AS cnt
          FROM ev e
          WHERE e.src = c.number AND e.dst_is_person_like AND e.dst IS NOT NULL AND e.dst <> s.number AND e.dst <> c.number
            AND e.lt >= c.first_lt
            AND e.lt < c.first_lt + p.history_days * interval '1 day'
          GROUP BY e.dst
        ) y ON x.dst = y.dst
      ) t
    ) ct ON true
    LEFT JOIN LATERAL (
      SELECT CASE WHEN t.u > 0 THEN t.i::float8 / t.u ELSE 0 END AS jaccard
      FROM (
        SELECT count(*) FILTER (WHERE x.cell_key IS NOT NULL AND y.cell_key IS NOT NULL) AS i, count(*) AS u
        FROM (
          SELECT e.cell_key
          FROM ev e
          WHERE e.src = s.number AND e.cell_key IS NOT NULL
            AND e.lt > s.last_lt - p.history_days * interval '1 day'
            AND e.lt <= s.last_lt
          GROUP BY e.cell_key HAVING count(*) >= 2
        ) x
        FULL JOIN (
          SELECT e.cell_key
          FROM ev e
          WHERE e.src = c.number AND e.cell_key IS NOT NULL
            AND e.lt >= c.first_lt
            AND e.lt < c.first_lt + p.history_days * interval '1 day'
          GROUP BY e.cell_key HAVING count(*) >= 2
        ) y ON x.cell_key = y.cell_key
      ) t
    ) cj ON true
  ) f
) r
WHERE r.score >= r.score_percent / 100.0;

COMMENT ON VIEW analyst.v_silent_numbers IS
  'Numbers that went silent: the last event of the number is at least silence_days before the end of the case (the whole local days from the first to the last event of the case). The number of days is set by the application from the question (7 when it names none): never compute on it yourself. One row per silent number.';
COMMENT ON VIEW analyst.v_successor_candidates IS
  'For a number that went silent (old_number) the numbers that may have taken over from it (new_number): a number whose first event is from a few days before to window_days after the old number''s last event and that was not in use at the start of the case, with a documented score from three features: contact similarity, shared places and a shared handset. BEHAVIOURAL SIMILARITY, NOT IDENTITY. The silence, the window and the lowest score are set by the application from the question (7 days, 14 days and 35 percent when it names none): never compute on them yourself. Ask with old_number = ... or new_number = ....';

DO $$
DECLARE
  d record;
BEGIN
  FOR d IN
    SELECT * FROM (VALUES
      ('v_silent_numbers', 'number', 'The number that went silent (the subscriber whose records these are).'),
      ('v_silent_numbers', 'first_event_utc', 'Time of the first event of the number, UTC (shown on the case clock).'),
      ('v_silent_numbers', 'last_event_utc', 'Time of the last event of the number, UTC (shown on the case clock).'),
      ('v_silent_numbers', 'events', 'Number of events of the number in the case.'),
      ('v_silent_numbers', 'days_silent', 'Days from the last event of the number to the end of the case.'),
      ('v_silent_numbers', 'silence_days', 'The days of silence this row was computed with, set by the application from the question (7 when it names none).'),
      ('v_successor_candidates', 'old_number', 'The number that went silent. Ask with old_number = ... to see who may have taken over from it.'),
      ('v_successor_candidates', 'new_number', 'A number that started about when the old number stopped and may have taken over from it. Ask with new_number = ... to see which silent number it may have replaced.'),
      ('v_successor_candidates', 'old_last_event_utc', 'Time of the last event of the old number, UTC (shown on the case clock).'),
      ('v_successor_candidates', 'new_first_event_utc', 'Time of the first event of the new number, UTC (shown on the case clock).'),
      ('v_successor_candidates', 'score', 'Documented similarity score from 0 to 1: 0.6 times the contact similarity, 0.2 times the shared places, 0.2 when they share a handset. Not a probability and not identity.'),
      ('v_successor_candidates', 'contact_cosine', 'Similarity from 0 to 1 of the people the old number contacted in its last days and the new number contacted in its first days (the two numbers themselves left out).'),
      ('v_successor_candidates', 'shared_contacts', 'Number of people contacted by both.'),
      ('v_successor_candidates', 'cell_jaccard', 'Share from 0 to 1 of the cell sites that both used at least twice in those periods.'),
      ('v_successor_candidates', 'shared_device', 'True when the two numbers were used with the same handset (the first 14 digits of the IMEI) at any time.'),
      ('v_successor_candidates', 'silence_days', 'The days of silence this row was computed with, set by the application from the question (7 when it names none).'),
      ('v_successor_candidates', 'window_days', 'The days after the old number''s last event within which the new number must have started, set by the application from the question (14 when it names none).'),
      ('v_successor_candidates', 'score_percent', 'The lowest score listed, in percent, set by the application from the question (35 when it names none).')
    ) AS t(view_name, column_name, description)
  LOOP
    EXECUTE format('COMMENT ON COLUMN analyst.%I.%I IS %L', d.view_name, d.column_name, d.description);
  END LOOP;
END
$$;

-- The role reads the new views and nothing else: SELECT is granted view by view; no other privilege on them.
REVOKE ALL ON analyst.v_silent_numbers, analyst.v_successor_candidates FROM PUBLIC;
GRANT SELECT ON analyst.v_silent_numbers, analyst.v_successor_candidates TO forensic_analyst_ro;
COMMIT;
