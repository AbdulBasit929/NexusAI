-- Migration 024: analyst views for "when were A and B at the same place within N minutes" and "which of those could be a meeting" (link analysis stage 3a;
-- docs/data/link-analysis-stage3-preregistration-20261022.md, analyses A5 and A7 of docs/data/case-generator-spec.md section 10).
--
-- Why. v_cell_visits (migration 019) holds one row per subscriber, cell and HOUR, so it answers "which cells did both use in the same hour" (stage 2) but not co-location
-- within minutes as the generator's A5 states it: two subscriptions are co-located when each has an event at the same cell and the two event times differ by at most W minutes
-- (exactly W counts), and co-located event pairs on one cell are merged into windows. The same question would otherwise have two different truths. These two views state the
-- rule exactly, so a question is one line and the answer equals the oracle.
--
-- RUNTIME PARAMETERS, NOT CONSTANTS. W is whatever the analyst asks (5, 15, 20, 22 minutes ...), so it is not written into the views: they read the transaction settings
-- app.colocation_minutes (W; 15 when unset) and app.contact_gap_minutes (G, for the meetings; 60 when unset), the way v_contact_pairs reads app.hub_threshold. The APPLICATION sets
-- them from the number it reads in the question (never the model, never the analyst's SQL), within 1 to 180; an unset or empty setting gives the default, and a value outside the range
-- is clamped. Every row says the window it was computed with (window_minutes), so an answer cannot be mistaken for another window.
--
-- What is added (two views; nothing existing changes; no table is touched):
--   * analyst.v_colocation_windows  one row per window of a pair of numbers at one cell: a, b (both orders present), cell_key, start_utc, end_utc, pairs (the number of co-located event
--                                   pairs in the window), window_minutes (W), and the cell site's name, position and radius when the case has them. A pair of events joins the open window
--                                   when its earlier time is within W minutes of the window's latest time; start is the earliest and end the latest event time in the window.
--   * analyst.v_meeting_candidates  the windows that are POSSIBLE meetings: the pair had a direct call or SMS with each other (either direction) no more than G minutes before the
--                                   window starts (a contact exactly G before, or at the start, counts; one after the start does not), with the number of such contacts, the latest of
--                                   them, the gap to it in minutes, the documented score 0.5 * min(pairs, 3) / 3 + 0.5 * (1 - gap / G), window_minutes (W) and allowed_gap_minutes (G).
--                                   A candidate, not a finding.
-- A pair is asked for with `WHERE a = ... AND b = ...`: the predicates reach the events through the window functions (they are the partition columns), so a question reads the two
-- numbers it names, not every pair of the case. Two numbers of different operators never share a cell key, so they have no row (cell identities are per operator).
-- No chance baseline is computed here (the generator's `lift`): the engine's caveat says so.
--
-- Same scope as every analyst view (tenant and collection from the transaction settings, through analyst.v_comm_events), same role (forensic_analyst_ro gets SELECT on each new view
-- only). Additive and idempotent. Apply by hand on a retained database: ops/024_colocation_and_meetings/preflight.sql first, verify.sql after; rollback.sql drops the two views.
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE OR REPLACE VIEW analyst.v_colocation_windows AS
SELECT
  w.a,
  w.b,
  w.cell_key,
  w.start_utc,
  w.end_utc,
  w.pairs,
  w.window_minutes,
  c.site_name,
  c.lat,
  c.lon,
  c.radius_m
FROM (
  SELECT
    g.a,
    g.b,
    g.cell_key,
    min(g.early) AS start_utc,
    max(g.late) AS end_utc,
    count(*)::bigint AS pairs,
    g.window_minutes
  FROM (
    SELECT
      o.a, o.b, o.cell_key, o.early, o.late, o.window_minutes,
      sum(CASE WHEN o.prev_end IS NULL OR o.early - o.prev_end > o.window_minutes * interval '1 minute' THEN 1 ELSE 0 END)
        OVER (PARTITION BY o.a, o.b, o.cell_key ORDER BY o.early, o.late) AS grp
    FROM (
      SELECT
        p.a, p.b, p.cell_key, p.early, p.late, p.window_minutes,
        max(p.late) OVER (PARTITION BY p.a, p.b, p.cell_key ORDER BY p.early, p.late ROWS BETWEEN UNBOUNDED PRECEDING AND 1 PRECEDING) AS prev_end
      FROM (
        SELECT
          x.src AS a,
          y.src AS b,
          x.cell_key,
          LEAST(x.start_utc, y.start_utc) AS early,
          GREATEST(x.start_utc, y.start_utc) AS late,
          pw.minutes AS window_minutes
        FROM (SELECT LEAST(180, GREATEST(1, COALESCE(NULLIF(current_setting('app.colocation_minutes', true), '')::integer, 15))) AS minutes) pw
        CROSS JOIN analyst.v_comm_events x
        JOIN analyst.v_comm_events y
          ON y.cell_key = x.cell_key
         AND y.src <> x.src
         AND y.start_utc BETWEEN x.start_utc - pw.minutes * interval '1 minute' AND x.start_utc + pw.minutes * interval '1 minute'
        WHERE x.src IS NOT NULL AND y.src IS NOT NULL AND x.cell_key IS NOT NULL
      ) p
    ) o
  ) g
  GROUP BY g.a, g.b, g.cell_key, g.grp, g.window_minutes
) w
LEFT JOIN analyst.v_cells c ON c.cell_key = w.cell_key;

CREATE OR REPLACE VIEW analyst.v_meeting_candidates AS
SELECT
  w.a,
  w.b,
  w.cell_key,
  w.start_utc,
  w.end_utc,
  w.pairs,
  k.contacts,
  k.last_contact_utc,
  (EXTRACT(epoch FROM (w.start_utc - k.last_contact_utc)) / 60.0)::float8 AS gap_minutes,
  (0.5 * LEAST(w.pairs, 3) / 3.0 + 0.5 * (1 - (EXTRACT(epoch FROM (w.start_utc - k.last_contact_utc)) / 60.0) / gp.minutes))::float8 AS score,
  w.window_minutes,
  gp.minutes AS allowed_gap_minutes,
  w.site_name,
  w.lat,
  w.lon,
  w.radius_m
FROM analyst.v_colocation_windows w
CROSS JOIN (SELECT LEAST(180, GREATEST(1, COALESCE(NULLIF(current_setting('app.contact_gap_minutes', true), '')::integer, 60))) AS minutes) gp
CROSS JOIN LATERAL (
  SELECT count(*)::bigint AS contacts, max(q.start_utc) AS last_contact_utc
  FROM (
    SELECT DISTINCT e.start_utc
    FROM analyst.v_comm_events e
    WHERE ((e.src = w.a AND e.dst = w.b) OR (e.src = w.b AND e.dst = w.a))
      AND e.start_utc BETWEEN w.start_utc - gp.minutes * interval '1 minute' AND w.start_utc
  ) q
) k
WHERE k.contacts > 0;

COMMENT ON VIEW analyst.v_colocation_windows IS
  'Windows in which two numbers of the same operator were at the same cell site within a number of minutes of each other (window_minutes; a difference of exactly that many minutes counts); event pairs on one cell are merged into a window while each pair starts within that many minutes of the window''s latest time. The number of minutes is set by the application from the question (15 when the question names none): never filter or compute on minutes yourself. Always write a = ... AND b = ... for the two numbers. Proximity is not association; no chance baseline is computed.';
COMMENT ON VIEW analyst.v_meeting_candidates IS
  'The co-location windows that could be a meeting: the two numbers had a direct call or SMS with each other no more than allowed_gap_minutes before the window started (60 when the question names none; the window length is window_minutes, 15 when none is named), both set by the application from the question: never compute on minutes yourself. A candidate, not a finding: proximity and contact are not proof of a meeting. Always write a = ... AND b = ... for the two numbers.';

DO $$
DECLARE
  d record;
BEGIN
  FOR d IN
    SELECT * FROM (VALUES
      ('v_colocation_windows', 'a', 'The first number of the pair asked about: always write a = ''...'' in the query.'),
      ('v_colocation_windows', 'b', 'The second number of the pair asked about: always write b = ''...'' in the query.'),
      ('v_colocation_windows', 'cell_key', 'The cell site where both were seen (operator code, area code and cell identity).'),
      ('v_colocation_windows', 'start_utc', 'Time of the earliest event of the window, UTC (shown on the case clock).'),
      ('v_colocation_windows', 'end_utc', 'Time of the latest event of the window, UTC (shown on the case clock).'),
      ('v_colocation_windows', 'pairs', 'Number of co-located event pairs (one event of each number, same cell, at most window_minutes apart) in the window.'),
      ('v_colocation_windows', 'window_minutes', 'The window length in minutes this row was computed with, set by the application from the question (15 when it names none).'),
      ('v_colocation_windows', 'site_name', 'Name of the cell site when the case has a reference row for it.'),
      ('v_colocation_windows', 'lat', 'Latitude of the cell site when known (the site, not the person).'),
      ('v_colocation_windows', 'lon', 'Longitude of the cell site when known (the site, not the person).'),
      ('v_colocation_windows', 'radius_m', 'Stated coverage radius of the cell site in metres when known.'),
      ('v_meeting_candidates', 'a', 'The first number of the pair asked about: always write a = ''...'' in the query.'),
      ('v_meeting_candidates', 'b', 'The second number of the pair asked about: always write b = ''...'' in the query.'),
      ('v_meeting_candidates', 'cell_key', 'The cell site of the co-location window.'),
      ('v_meeting_candidates', 'start_utc', 'Start of the co-location window, UTC (shown on the case clock).'),
      ('v_meeting_candidates', 'end_utc', 'End of the co-location window, UTC (shown on the case clock).'),
      ('v_meeting_candidates', 'pairs', 'Number of co-located event pairs in the window.'),
      ('v_meeting_candidates', 'contacts', 'Number of distinct direct calls or SMS between the two numbers in the allowed gap up to the start of the window.'),
      ('v_meeting_candidates', 'last_contact_utc', 'Time of the latest of those contacts, UTC.'),
      ('v_meeting_candidates', 'gap_minutes', 'Minutes from the latest contact to the start of the window (0 up to allowed_gap_minutes).'),
      ('v_meeting_candidates', 'score', 'Documented score from 0 to 1: half from the number of pairs (3 or more is the most), half from how short the gap is against the allowed gap. Not a probability.'),
      ('v_meeting_candidates', 'window_minutes', 'The co-location window length in minutes this row was computed with, set by the application from the question (15 when it names none).'),
      ('v_meeting_candidates', 'allowed_gap_minutes', 'The longest gap between a contact and the window start in minutes, set by the application from the question (60 when it names none).'),
      ('v_meeting_candidates', 'site_name', 'Name of the cell site when the case has a reference row for it.'),
      ('v_meeting_candidates', 'lat', 'Latitude of the cell site when known (the site, not the person).'),
      ('v_meeting_candidates', 'lon', 'Longitude of the cell site when known (the site, not the person).'),
      ('v_meeting_candidates', 'radius_m', 'Stated coverage radius of the cell site in metres when known.')
    ) AS t(view_name, column_name, description)
  LOOP
    EXECUTE format('COMMENT ON COLUMN analyst.%I.%I IS %L', d.view_name, d.column_name, d.description);
  END LOOP;
END
$$;

-- The role reads the new views and nothing else: SELECT is granted view by view; no other privilege on them.
REVOKE ALL ON analyst.v_colocation_windows, analyst.v_meeting_candidates FROM PUBLIC;
GRANT SELECT ON analyst.v_colocation_windows, analyst.v_meeting_candidates TO forensic_analyst_ro;
COMMIT;
