-- Migration 026: analyst views for movement and rhythm (link analysis stage 3c; docs/data/link-analysis-stage3-preregistration-20261022.md, analyses A10 and A11 of
-- docs/data/case-generator-spec.md section 10): where a number sleeps and works week by week, when that changes for good, trips, impossibly fast moves between cell sites, and when the
-- rhythm of use changes.
--
-- Why. "Where does he sleep", "when did he move house", "which calls are impossibly far apart" and "did his activity change" are among the commonest questions of a telecom case, and the rules
-- are subtle (weeks counted from the start of the case, an anchor that must hold, a trip that is not a move, a change point found by a statistical test). A small model cannot write them
-- reliably; the views state them exactly, as the generator's oracle does, so a question is one line and the answer equals the oracle. CELL SITES, NOT POSITIONS OF A PERSON; A CHANGE IN THE
-- PATTERN OF USE, NOT AN EXPLANATION.
--
-- RUNTIME PARAMETERS, NOT CONSTANTS. The numbers of the rules are transaction settings that the APPLICATION sets from the question (never the model, never the analyst's SQL), within bounds,
-- with the specification's value as the default when the question names none: app.anchor_min_events (3), app.anchor_hold_weeks (2), app.speed_kmh (200), app.rhythm_min_side_days (10),
-- app.rhythm_ratio_percent (150). Empty or unset gives the default; out-of-range values are clamped. Every row says the values it was computed with. The rules that stay part of the
-- definition (a week is a block of 7 days from the start of the case; the night is 22:00 to 07:00 and the day 09:00 to 17:00 on weekdays on the case clock; a change of rhythm needs |t| >= 5
-- of Welch's test) are named in the descriptions and the engine's caveats.
--
-- Times are on the case clock (the session time zone). The case covers the whole local days from the first to the last event of the case. Positions are those of the cell sites (analyst.v_cells).
--
-- What is added (seven views; nothing existing changes; no table is touched):
--   * analyst.v_weekly_anchors      per number, slot (night or day) and week, the cell with most events of the slot, when it has at least anchor_min_events events that week.
--   * analyst.v_anchor_runs         the stretches of weeks (with an anchor) that have the same anchor, with whether each holds for at least anchor_hold_weeks weeks.
--   * analyst.v_movement_anchors    the anchors of a number: the stretches that hold, consecutive stretches of one cell merged.
--   * analyst.v_anchor_changes      a move: from one anchor to a different one that then holds, at the first week of the new one, with the distance between the two cell sites.
--   * analyst.v_anchor_deviations   a stretch of fewer weeks than needed to count as a move (a trip).
--   * analyst.v_speed_alerts        two consecutive events of one number at different cell sites whose distance divided by the time between them is at least speed_kmh (none if the same second).
--   * analyst.v_rhythm_changes      per number, the best change point of the daily count of events (largest |Welch t| over the splits that leave at least rhythm_min_side_days on each side),
--                                   listed when |t| >= 5 and the larger daily mean is at least rhythm_ratio_percent / 100 times the smaller.
--
-- Same scope as every analyst view (tenant and collection from the transaction settings, through analyst.v_comm_events), same role (forensic_analyst_ro gets SELECT on each new view
-- only). Additive and idempotent. Apply by hand on a retained database: ops/026_movement_and_rhythm/preflight.sql first, verify.sql after; rollback.sql drops the seven views.
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE OR REPLACE VIEW analyst.v_weekly_anchors AS
SELECT
  q.number,
  q.slot,
  q.week,
  (b.case_start::date + q.week * 7) AS week_start,
  q.cell_key,
  q.events_at_anchor,
  q.events_in_slot,
  p.min_events,
  c.site_name,
  c.lat,
  c.lon
FROM (SELECT LEAST(100, GREATEST(1, COALESCE(NULLIF(current_setting('app.anchor_min_events', true), '')::integer, 3))) AS min_events) p
CROSS JOIN (
  SELECT date_trunc('day', min(e.start_utc AT TIME ZONE current_setting('TimeZone'))) AS case_start
  FROM analyst.v_comm_events e WHERE e.src IS NOT NULL
) b
JOIN (
  SELECT k.number, k.slot, k.week, k.cell_key, k.n AS events_at_anchor, k.total AS events_in_slot,
         row_number() OVER (PARTITION BY k.number, k.slot, k.week ORDER BY k.n DESC, k.cell_key COLLATE "C") AS rk
  FROM (
    SELECT s.number, s.slot, ((s.lt::date - bb.case_start::date) / 7) AS week, s.cell_key, count(*) AS n,
           sum(count(*)) OVER (PARTITION BY s.number, s.slot, ((s.lt::date - bb.case_start::date) / 7)) AS total
    FROM (
      SELECT e.src AS number, 'night'::text AS slot, (e.start_utc AT TIME ZONE current_setting('TimeZone')) AS lt, e.cell_key
      FROM analyst.v_comm_events e
      WHERE e.src IS NOT NULL AND e.cell_key IS NOT NULL
        AND (EXTRACT(hour FROM (e.start_utc AT TIME ZONE current_setting('TimeZone'))) >= 22 OR EXTRACT(hour FROM (e.start_utc AT TIME ZONE current_setting('TimeZone'))) < 7)
      UNION ALL
      SELECT e.src, 'day'::text, (e.start_utc AT TIME ZONE current_setting('TimeZone')), e.cell_key
      FROM analyst.v_comm_events e
      WHERE e.src IS NOT NULL AND e.cell_key IS NOT NULL
        AND EXTRACT(hour FROM (e.start_utc AT TIME ZONE current_setting('TimeZone'))) >= 9 AND EXTRACT(hour FROM (e.start_utc AT TIME ZONE current_setting('TimeZone'))) < 17
        AND EXTRACT(isodow FROM (e.start_utc AT TIME ZONE current_setting('TimeZone'))) <= 5
    ) s
    CROSS JOIN (
      SELECT date_trunc('day', min(e.start_utc AT TIME ZONE current_setting('TimeZone'))) AS case_start
      FROM analyst.v_comm_events e WHERE e.src IS NOT NULL
    ) bb
    GROUP BY s.number, s.slot, ((s.lt::date - bb.case_start::date) / 7), s.cell_key
  ) k
) q ON q.rk = 1 AND q.events_in_slot >= p.min_events
LEFT JOIN analyst.v_cells c ON c.cell_key = q.cell_key;

CREATE OR REPLACE VIEW analyst.v_anchor_runs AS
SELECT
  g.number,
  g.slot,
  g.cell_key,
  min(g.week) AS first_week,
  max(g.week) AS last_week,
  count(*)::bigint AS weeks,
  (count(*) >= LEAST(52, GREATEST(1, COALESCE(NULLIF(current_setting('app.anchor_hold_weeks', true), '')::integer, 2)))) AS holds,
  LEAST(52, GREATEST(1, COALESCE(NULLIF(current_setting('app.anchor_hold_weeks', true), '')::integer, 2))) AS hold_weeks
FROM (
  SELECT w.number, w.slot, w.week, w.cell_key,
         sum(CASE WHEN w.cell_key IS DISTINCT FROM w.previous_cell THEN 1 ELSE 0 END) OVER (PARTITION BY w.number, w.slot ORDER BY w.week) AS grp
  FROM (
    SELECT x.number, x.slot, x.week, x.cell_key, lag(x.cell_key) OVER (PARTITION BY x.number, x.slot ORDER BY x.week) AS previous_cell
    FROM analyst.v_weekly_anchors x
  ) w
) g
GROUP BY g.number, g.slot, g.grp, g.cell_key;

CREATE OR REPLACE VIEW analyst.v_movement_anchors AS
SELECT
  m.number,
  m.slot,
  m.cell_key,
  min(m.first_week) AS first_week,
  max(m.last_week) AS last_week,
  sum(m.weeks)::bigint AS weeks,
  (min(b.case_start::date) + min(m.first_week) * 7) AS first_week_start,
  min(m.hold_weeks) AS hold_weeks,
  max(c.site_name) AS site_name,
  max(c.lat) AS lat,
  max(c.lon) AS lon
FROM (
  SELECT r.number, r.slot, r.cell_key, r.first_week, r.last_week, r.weeks, r.hold_weeks,
         sum(CASE WHEN r.cell_key IS DISTINCT FROM r.previous_cell THEN 1 ELSE 0 END) OVER (PARTITION BY r.number, r.slot ORDER BY r.first_week) AS grp
  FROM (
    SELECT x.number, x.slot, x.cell_key, x.first_week, x.last_week, x.weeks, x.hold_weeks,
           lag(x.cell_key) OVER (PARTITION BY x.number, x.slot ORDER BY x.first_week) AS previous_cell
    FROM analyst.v_anchor_runs x WHERE x.holds
  ) r
) m
CROSS JOIN (
  SELECT date_trunc('day', min(e.start_utc AT TIME ZONE current_setting('TimeZone'))) AS case_start
  FROM analyst.v_comm_events e WHERE e.src IS NOT NULL
) b
LEFT JOIN analyst.v_cells c ON c.cell_key = m.cell_key
GROUP BY m.number, m.slot, m.grp, m.cell_key;

CREATE OR REPLACE VIEW analyst.v_anchor_changes AS
SELECT
  t.number,
  t.slot,
  t.from_cell_key,
  t.cell_key AS to_cell_key,
  t.first_week,
  t.first_week_start,
  CASE WHEN fc.lat IS NOT NULL AND fc.lon IS NOT NULL AND tc.lat IS NOT NULL AND tc.lon IS NOT NULL THEN
    round((2 * 6371.0088 * asin(sqrt(LEAST(1.0, power(sin(radians(tc.lat - fc.lat) / 2), 2)
      + cos(radians(fc.lat)) * cos(radians(tc.lat)) * power(sin(radians(tc.lon - fc.lon) / 2), 2)))))::numeric, 3)::float8
  END AS distance_km,
  t.hold_weeks
FROM (
  SELECT a.number, a.slot, a.cell_key, a.first_week, a.first_week_start, a.hold_weeks,
         lag(a.cell_key) OVER (PARTITION BY a.number, a.slot ORDER BY a.first_week) AS from_cell_key
  FROM analyst.v_movement_anchors a
) t
LEFT JOIN analyst.v_cells fc ON fc.cell_key = t.from_cell_key
LEFT JOIN analyst.v_cells tc ON tc.cell_key = t.cell_key
WHERE t.from_cell_key IS NOT NULL;

CREATE OR REPLACE VIEW analyst.v_anchor_deviations AS
SELECT
  r.number,
  r.slot,
  r.cell_key,
  r.first_week,
  r.last_week,
  r.weeks,
  r.hold_weeks
FROM analyst.v_anchor_runs r
WHERE NOT r.holds;

CREATE OR REPLACE VIEW analyst.v_speed_alerts AS
SELECT
  z.number,
  z.from_time_utc,
  z.to_time_utc,
  z.from_cell_key,
  z.to_cell_key,
  round(z.km::numeric, 3)::float8 AS distance_km,
  floor(z.gap_s)::bigint AS gap_s,
  CASE WHEN z.gap_s > 0 THEN round((z.km / (z.gap_s / 3600.0))::numeric, 1)::float8 END AS speed_kmh,
  z.limit_kmh
FROM (
  SELECT
    t.number, t.from_time_utc, t.to_time_utc, t.from_cell_key, t.to_cell_key, t.gap_s, t.limit_kmh,
    2 * 6371.0088 * asin(sqrt(LEAST(1.0, power(sin(radians(t.lat - t.prev_lat) / 2), 2)
      + cos(radians(t.prev_lat)) * cos(radians(t.lat)) * power(sin(radians(t.lon - t.prev_lon) / 2), 2)))) AS km
  FROM (
    SELECT
      e.src AS number,
      lag(e.start_utc) OVER w AS from_time_utc,
      e.start_utc AS to_time_utc,
      lag(e.cell_key) OVER w AS from_cell_key,
      e.cell_key AS to_cell_key,
      lag(c.lat) OVER w AS prev_lat,
      lag(c.lon) OVER w AS prev_lon,
      c.lat,
      c.lon,
      EXTRACT(epoch FROM (e.start_utc - lag(e.start_utc) OVER w)) AS gap_s,
      LEAST(2000, GREATEST(1, COALESCE(NULLIF(current_setting('app.speed_kmh', true), '')::integer, 200))) AS limit_kmh
    FROM analyst.v_comm_events e
    JOIN analyst.v_cells c ON c.cell_key = e.cell_key
    WHERE e.src IS NOT NULL AND e.cell_key IS NOT NULL AND c.lat IS NOT NULL AND c.lon IS NOT NULL
    WINDOW w AS (PARTITION BY e.src ORDER BY e.start_utc, e.event_id)
  ) t
  WHERE t.from_cell_key IS NOT NULL AND t.from_cell_key <> t.to_cell_key
) z
WHERE z.gap_s = 0 OR z.km / (z.gap_s / 3600.0) >= z.limit_kmh;

CREATE OR REPLACE VIEW analyst.v_rhythm_changes AS
SELECT
  r.number,
  r.days,
  r.first_day,
  round(r.mean_before::numeric, 3)::float8 AS mean_before,
  round(r.mean_after::numeric, 3)::float8 AS mean_after,
  CASE WHEN r.t < 'Infinity'::float8 THEN round(r.t::numeric, 2)::float8 END AS t,
  CASE WHEN r.ratio < 'Infinity'::float8 THEN round(r.ratio::numeric, 3)::float8 END AS ratio,
  CASE WHEN r.mean_after > r.mean_before THEN 'up' ELSE 'down' END AS direction,
  r.min_side_days,
  r.ratio_percent
FROM (
  SELECT
    b.number, b.n AS days, (b.first_day + b.i) AS first_day, b.ml AS mean_before, b.mr AS mean_after, b.t,
    CASE WHEN LEAST(b.ml, b.mr) > 0 THEN GREATEST(b.ml, b.mr) / LEAST(b.ml, b.mr) ELSE 'Infinity'::float8 END AS ratio,
    b.min_side_days, b.ratio_percent
  FROM (
    SELECT DISTINCT ON (s.number)
      s.number, s.n, s.first_day, s.i, s.ml, s.mr, s.min_side_days, s.ratio_percent,
      CASE WHEN s.se = 0 THEN (CASE WHEN s.ml = s.mr THEN 0::float8 ELSE 'Infinity'::float8 END) ELSE (s.mr - s.ml) / s.se END AS t
    FROM (
      SELECT
        k.number, k.n, k.first_day, k.i, k.min_side_days, k.ratio_percent,
        k.left_sum::float8 / k.i AS ml,
        k.right_sum::float8 / (k.n - k.i) AS mr,
        sqrt(((k.i * k.left_sq - k.left_sum * k.left_sum)::numeric / (k.i::numeric * (k.i - 1)))::float8 / k.i
           + ((((k.n - k.i) * k.right_sq - k.right_sum * k.right_sum)::numeric / ((k.n - k.i)::numeric * (k.n - k.i - 1)))::float8 / (k.n - k.i))) AS se
      FROM (
        SELECT
          d.number, d.n, d.first_day, d.i, d.min_side_days, d.ratio_percent,
          (sum(d.x) OVER (PARTITION BY d.number ORDER BY d.i ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) - d.x)::bigint AS left_sum,
          (sum(d.x * d.x) OVER (PARTITION BY d.number ORDER BY d.i ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) - d.x * d.x)::bigint AS left_sq,
          (sum(d.x) OVER (PARTITION BY d.number) - (sum(d.x) OVER (PARTITION BY d.number ORDER BY d.i ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) - d.x))::bigint AS right_sum,
          (sum(d.x * d.x) OVER (PARTITION BY d.number) - (sum(d.x * d.x) OVER (PARTITION BY d.number ORDER BY d.i ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) - d.x * d.x))::bigint AS right_sq
        FROM (
          SELECT sp.number, sp.n, sp.first_day, g.i, COALESCE(c.n, 0)::bigint AS x, p.min_side_days, p.ratio_percent
          FROM (SELECT LEAST(180, GREATEST(2, COALESCE(NULLIF(current_setting('app.rhythm_min_side_days', true), '')::integer, 10))) AS min_side_days,
                       LEAST(10000, GREATEST(101, COALESCE(NULLIF(current_setting('app.rhythm_ratio_percent', true), '')::integer, 150))) AS ratio_percent) p
          CROSS JOIN (
            SELECT e.src AS number, min((e.start_utc AT TIME ZONE current_setting('TimeZone'))::date) AS first_day,
                   (max((e.start_utc AT TIME ZONE current_setting('TimeZone'))::date) - min((e.start_utc AT TIME ZONE current_setting('TimeZone'))::date) + 1) AS n
            FROM analyst.v_comm_events e WHERE e.src IS NOT NULL GROUP BY e.src
          ) sp
          CROSS JOIN LATERAL generate_series(0, sp.n - 1) g(i)
          LEFT JOIN (
            SELECT e.src AS number, (e.start_utc AT TIME ZONE current_setting('TimeZone'))::date AS day, count(*) AS n
            FROM analyst.v_comm_events e WHERE e.src IS NOT NULL GROUP BY e.src, (e.start_utc AT TIME ZONE current_setting('TimeZone'))::date
          ) c ON c.number = sp.number AND c.day = sp.first_day + g.i
          WHERE sp.n >= 2 * p.min_side_days
        ) d
      ) k
      WHERE k.i >= k.min_side_days AND k.n - k.i >= k.min_side_days
    ) s
    ORDER BY s.number,
             abs(CASE WHEN s.se = 0 THEN (CASE WHEN s.ml = s.mr THEN 0::float8 ELSE 'Infinity'::float8 END) ELSE (s.mr - s.ml) / s.se END) DESC,
             s.i ASC
  ) b
) r
WHERE abs(r.t) >= 5 AND r.ratio >= r.ratio_percent / 100.0;

COMMENT ON VIEW analyst.v_weekly_anchors IS
  'Per number, slot (night: the cell site with most events between 22:00 and 07:00; day: between 09:00 and 17:00 on weekdays, on the case clock) and week (a block of 7 days counted from the start of the case, which is the first day with an event), the cell site with most events of the slot, kept when it has at least min_events events that week (ties: the lowest cell key). The minimum is set by the application from the question (3 when it names none): never compute on it yourself. CELL SITES, NOT POSITIONS OF A PERSON.';
COMMENT ON VIEW analyst.v_anchor_runs IS
  'The stretches of consecutive weeks (those that have an anchor) in which a number has the same anchor in a slot, with whether the stretch holds for at least hold_weeks weeks (set by the application from the question; 2 when it names none): never compute on it yourself. A helper for the anchors, the moves and the trips.';
COMMENT ON VIEW analyst.v_movement_anchors IS
  'The anchors of a number: where it sleeps (slot night) and where it spends weekdays (slot day), each stretch that holds for at least hold_weeks weeks, with its first and last week and the date the first week starts. Consecutive stretches of the same cell site are one anchor. hold_weeks is set by the application from the question (2 when it names none). CELL SITES, NOT POSITIONS OF A PERSON.';
COMMENT ON VIEW analyst.v_anchor_changes IS
  'A move: a number''s anchor changes from one cell site to a different one that then holds for at least hold_weeks weeks (set by the application from the question; 2 when it names none), reported at the first week of the new anchor, with the distance in kilometres between the two cell sites. One row per move per slot (night: where it sleeps, day: where it spends weekdays). CELL SITES, NOT POSITIONS OF A PERSON.';
COMMENT ON VIEW analyst.v_anchor_deviations IS
  'A trip: a stretch of fewer than hold_weeks weeks in which a number''s anchor is a different cell site, which is not reported as a move. hold_weeks is set by the application from the question (2 when it names none). CELL SITES, NOT POSITIONS OF A PERSON.';
COMMENT ON VIEW analyst.v_speed_alerts IS
  'Two consecutive events of one number at different cell sites whose distance (haversine between the sites) divided by the time between them is at least limit_kmh (set by the application from the question; 200 when it names none: never compute on it yourself); two events at different cell sites in the same second are listed with no speed. A cell site is a coverage area, so an alert may be a handover between towers, a fault or a record error: CELL SITES, NOT POSITIONS OF A PERSON.';
COMMENT ON VIEW analyst.v_rhythm_changes IS
  'For a number, the day on which its daily count of events changed for good: the daily counts (days with no event counted as zero, from its first to its last day) are split at every day that leaves at least min_side_days on each side, the split with the largest absolute Welch t is the change point, and it is listed when |t| is at least 5 and the larger daily mean is at least ratio_percent / 100 times the smaller. min_side_days and ratio_percent are set by the application from the question (10 and 150 when it names none). A CHANGE IN THE PATTERN OF USE, NOT AN EXPLANATION; a short series can show a change by chance.';

DO $$
DECLARE
  d record;
BEGIN
  FOR d IN
    SELECT * FROM (VALUES
      ('v_weekly_anchors', 'number', 'The number (the subscriber whose records these are).'),
      ('v_weekly_anchors', 'slot', 'night (22:00 to 07:00) or day (09:00 to 17:00 on weekdays), on the case clock.'),
      ('v_weekly_anchors', 'week', 'Week number counted from the start of the case in blocks of 7 days: 0 is the first week.'),
      ('v_weekly_anchors', 'week_start', 'The date on which the week starts.'),
      ('v_weekly_anchors', 'cell_key', 'The cell site with most events of the slot that week (the anchor).'),
      ('v_weekly_anchors', 'events_at_anchor', 'Events of the slot at that cell site that week.'),
      ('v_weekly_anchors', 'events_in_slot', 'Events of the slot that week at all cell sites.'),
      ('v_weekly_anchors', 'min_events', 'The fewest events of the slot a week needs to have an anchor, set by the application from the question (3 when it names none).'),
      ('v_weekly_anchors', 'site_name', 'Name of the cell site when the case has a reference row for it.'),
      ('v_weekly_anchors', 'lat', 'Latitude of the cell site when known (the site, not the person).'),
      ('v_weekly_anchors', 'lon', 'Longitude of the cell site when known (the site, not the person).'),
      ('v_anchor_runs', 'number', 'The number.'),
      ('v_anchor_runs', 'slot', 'night or day.'),
      ('v_anchor_runs', 'cell_key', 'The anchor cell site of the stretch.'),
      ('v_anchor_runs', 'first_week', 'First week of the stretch (counted from the start of the case).'),
      ('v_anchor_runs', 'last_week', 'Last week of the stretch.'),
      ('v_anchor_runs', 'weeks', 'Number of weeks with this anchor in the stretch (weeks without an anchor do not break it and do not count).'),
      ('v_anchor_runs', 'holds', 'True when the stretch has at least hold_weeks weeks.'),
      ('v_anchor_runs', 'hold_weeks', 'The weeks a stretch needs to count as an anchor, set by the application from the question (2 when it names none).'),
      ('v_movement_anchors', 'number', 'The number.'),
      ('v_movement_anchors', 'slot', 'night (where it sleeps) or day (where it spends weekdays).'),
      ('v_movement_anchors', 'cell_key', 'The anchor cell site.'),
      ('v_movement_anchors', 'first_week', 'First week of the anchor (counted from the start of the case).'),
      ('v_movement_anchors', 'last_week', 'Last week of the anchor.'),
      ('v_movement_anchors', 'weeks', 'Weeks with this anchor.'),
      ('v_movement_anchors', 'first_week_start', 'The date on which the first week of the anchor starts.'),
      ('v_movement_anchors', 'hold_weeks', 'The weeks a stretch needs to count as an anchor, set by the application from the question (2 when it names none).'),
      ('v_movement_anchors', 'site_name', 'Name of the cell site when the case has a reference row for it.'),
      ('v_movement_anchors', 'lat', 'Latitude of the cell site when known (the site, not the person).'),
      ('v_movement_anchors', 'lon', 'Longitude of the cell site when known (the site, not the person).'),
      ('v_anchor_changes', 'number', 'The number that moved.'),
      ('v_anchor_changes', 'slot', 'night (where it sleeps) or day (where it spends weekdays).'),
      ('v_anchor_changes', 'from_cell_key', 'The cell site of the anchor before the move.'),
      ('v_anchor_changes', 'to_cell_key', 'The cell site of the anchor after the move.'),
      ('v_anchor_changes', 'first_week', 'The first week of the new anchor (counted from the start of the case): when the move shows.'),
      ('v_anchor_changes', 'first_week_start', 'The date on which that week starts.'),
      ('v_anchor_changes', 'distance_km', 'Distance in kilometres between the two cell sites, empty when a site has no position.'),
      ('v_anchor_changes', 'hold_weeks', 'The weeks the new anchor must hold to count as a move, set by the application from the question (2 when it names none).'),
      ('v_anchor_deviations', 'number', 'The number.'),
      ('v_anchor_deviations', 'slot', 'night or day.'),
      ('v_anchor_deviations', 'cell_key', 'The cell site that was the anchor during the trip.'),
      ('v_anchor_deviations', 'first_week', 'First week of the trip.'),
      ('v_anchor_deviations', 'last_week', 'Last week of the trip.'),
      ('v_anchor_deviations', 'weeks', 'Weeks of the trip (fewer than hold_weeks).'),
      ('v_anchor_deviations', 'hold_weeks', 'The weeks a stretch needs to count as a move, set by the application from the question (2 when it names none).'),
      ('v_speed_alerts', 'number', 'The number.'),
      ('v_speed_alerts', 'from_time_utc', 'Time of the earlier event, UTC (shown on the case clock).'),
      ('v_speed_alerts', 'to_time_utc', 'Time of the later event, UTC (shown on the case clock).'),
      ('v_speed_alerts', 'from_cell_key', 'The cell site of the earlier event.'),
      ('v_speed_alerts', 'to_cell_key', 'The cell site of the later event.'),
      ('v_speed_alerts', 'distance_km', 'Distance in kilometres between the two cell sites.'),
      ('v_speed_alerts', 'gap_s', 'Seconds between the two events.'),
      ('v_speed_alerts', 'speed_kmh', 'Distance divided by the time, in kilometres per hour; empty when the two events are in the same second.'),
      ('v_speed_alerts', 'limit_kmh', 'The lowest speed listed, set by the application from the question (200 when it names none).'),
      ('v_rhythm_changes', 'number', 'The number whose rhythm changed.'),
      ('v_rhythm_changes', 'days', 'Days from its first to its last day with an event.'),
      ('v_rhythm_changes', 'first_day', 'The first day of the new rhythm.'),
      ('v_rhythm_changes', 'mean_before', 'Mean events a day before that day.'),
      ('v_rhythm_changes', 'mean_after', 'Mean events a day from that day on.'),
      ('v_rhythm_changes', 't', 'Welch t of the two sides (empty when the sides have no spread at all).'),
      ('v_rhythm_changes', 'ratio', 'The larger mean divided by the smaller (empty when the smaller is zero).'),
      ('v_rhythm_changes', 'direction', 'up when the daily count rose, down when it fell.'),
      ('v_rhythm_changes', 'min_side_days', 'The fewest days on each side of a change point, set by the application from the question (10 when it names none).'),
      ('v_rhythm_changes', 'ratio_percent', 'The least ratio of the two means, in percent, set by the application from the question (150 when it names none).')
    ) AS t(view_name, column_name, description)
  LOOP
    EXECUTE format('COMMENT ON COLUMN analyst.%I.%I IS %L', d.view_name, d.column_name, d.description);
  END LOOP;
END
$$;

-- The role reads the new views and nothing else: SELECT is granted view by view; no other privilege on them.
REVOKE ALL ON analyst.v_weekly_anchors, analyst.v_anchor_runs, analyst.v_movement_anchors, analyst.v_anchor_changes, analyst.v_anchor_deviations, analyst.v_speed_alerts, analyst.v_rhythm_changes FROM PUBLIC;
GRANT SELECT ON analyst.v_weekly_anchors, analyst.v_anchor_runs, analyst.v_movement_anchors, analyst.v_anchor_changes, analyst.v_anchor_deviations, analyst.v_speed_alerts, analyst.v_rhythm_changes TO forensic_analyst_ro;
COMMIT;
