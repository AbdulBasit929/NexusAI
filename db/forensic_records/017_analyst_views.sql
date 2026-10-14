-- Migration 017 (DRAFT, not applied): the analyst views and the read-only analyst role (roadmap 6.8.3, 6.8.9; decisions D17 and D19).
--
-- Why. The Ask engine lets the language model write SQL for every data family and for link analysis (roadmap 6.8). The model must be able to reach only
-- reviewed, documented, already-scoped relations, never a base table. Laptop measurements (roadmap 6.8.13) showed that the model's output length is the
-- latency, so the views are THICK: they already carry the joins, the exclusions (services, hubs), the number classes and the pre-aggregates, and a question
-- becomes a short SELECT.
--
-- What it adds (nothing existing is changed):
--   * schema analyst with seven views: v_comm_events, v_contact_pairs, v_cells, v_cell_visits, v_device_pairings, v_data_sessions, v_app_usage;
--   * a NOLOGIN role forensic_analyst_ro that can read those views and nothing else, and membership of it for forensic_runtime (so that the application can
--     SET LOCAL ROLE to it inside a read-only transaction, then only the views are reachable).
--
-- Scope is STRUCTURAL. The views are not security_invoker: they read the tables as their owner, and the owner is not bound by row-level security. So every
-- view carries its own predicate, `tenant_id = current_setting('app.tenant_id', true) AND collection_id = current_setting('app.collection_id', true)`. The
-- application sets both settings (SET LOCAL) before it runs a statement; an unset setting is NULL, the predicate is then never true, and the views return no
-- rows (fail closed). The model's SQL cannot name another case because the views do not let it.
--
-- Hub numbers: a number in contact with more than N distinct subjects (N = current_setting('app.hub_threshold', true), a case setting; 20 when unset) is
-- flagged is_hub, as the planted-truth generator does. Services are flagged is_service. Both flags are in the views so that the model does not have to
-- reinvent the rule.
--
-- Withheld on purpose: raw payloads, URLs, user agents, record hashes, and the free-text fields of the base tables. Added only after a masking decision.
--
-- Role settings (statement_timeout, read-only) are NOT set on the role: `ALTER ROLE ... SET` applies at login, not at SET ROLE. The application sets them per
-- transaction (SET LOCAL statement_timeout, BEGIN READ ONLY). Requires migrations 012 and 013. Idempotent. Apply by hand on a retained database:
-- ops/017_analyst_views/preflight.sql first, verify.sql after; smoke.sql is for disposable databases only; rollback.sql removes the schema and the role.
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE SCHEMA IF NOT EXISTS analyst;
COMMENT ON SCHEMA analyst IS 'Reviewed, scoped, read-only views that the Ask engine lets the language model query. Scope comes from the settings app.tenant_id and app.collection_id.';

-- ---------------------------------------------------------------------------------------------------------------------
-- v_comm_events: one row per call, SMS or other event of a target call-detail record. Repeats and tower dumps are not here.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE OR REPLACE VIEW analyst.v_comm_events AS
SELECT
  e.event_id,
  e.evidence_id,
  e.source_row,
  e.event_type,
  e.direction,
  coalesce(e.a_norm, e.a_raw) AS src,
  -- The other party is a normalised number, or a short code, sender name or service label kept as written. Anything else (a sentinel such as -1, the
  -- subscriber itself, an unreadable value) is not a usable counterparty: dst is NULL, and the event still counts in the event totals.
  CASE WHEN e.b_norm IS NOT NULL THEN e.b_norm
       WHEN e.b_kind IN ('short_code', 'sender_id', 'service_label') THEN e.b_raw END AS dst,
  e.b_kind AS dst_kind,
  coalesce(e.b_kind IN ('person_like', 'international'), false) AS dst_is_person_like,
  e.start_utc,
  e.end_utc,
  e.duration_s,
  e.imsi,
  e.imei,
  e.cell_key,
  e.time_basis,
  e.source_tz
FROM forensic.comm_event e
WHERE e.tenant_id = current_setting('app.tenant_id', true)
  AND e.collection_id = current_setting('app.collection_id', true)
  AND e.dataset_kind = 'target_cdr'
  AND NOT ('duplicate_row' = ANY (e.quality_flags));

-- ---------------------------------------------------------------------------------------------------------------------
-- v_cells: the best current description of each cell site known to the case (the case's own list, else the tenant-wide one).
-- ---------------------------------------------------------------------------------------------------------------------
CREATE OR REPLACE VIEW analyst.v_cells AS
SELECT DISTINCT ON (c.cell_key)
  c.cell_key,
  c.site_name,
  c.operator_label AS operator,
  c.lat,
  c.lon,
  c.radius_m
FROM forensic.cell_site c
WHERE c.tenant_id = current_setting('app.tenant_id', true)
  AND (c.collection_id = '' OR c.collection_id = current_setting('app.collection_id', true))
  AND c.status = 'active'
ORDER BY c.cell_key, (c.collection_id = '') , c.source_rank, c.valid_from DESC;

-- ---------------------------------------------------------------------------------------------------------------------
-- v_contact_pairs: per subscriber (src, the file's owner) and other party (dst): counts, times, and the two flags.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE OR REPLACE VIEW analyst.v_contact_pairs AS
WITH pairs AS (
  SELECT
    v.src,
    v.dst,
    NOT bool_or(v.dst_is_person_like) AS is_service,
    count(*) AS events,
    count(*) FILTER (WHERE v.direction = 'outgoing') AS outgoing,
    count(*) FILTER (WHERE v.direction = 'incoming') AS incoming,
    count(*) FILTER (WHERE v.direction NOT IN ('outgoing', 'incoming')) AS unknown,
    sum(v.duration_s) AS total_duration_s,
    min(v.start_utc) AS first_seen,
    max(v.start_utc) AS last_seen
  FROM analyst.v_comm_events v
  WHERE v.src IS NOT NULL AND v.dst IS NOT NULL
  GROUP BY v.src, v.dst
)
SELECT
  p.src,
  p.dst,
  p.is_service,
  (NOT p.is_service
   AND count(*) FILTER (WHERE NOT p.is_service) OVER (PARTITION BY p.dst)
       > coalesce(nullif(current_setting('app.hub_threshold', true), '')::integer, 20)) AS is_hub,
  p.events,
  p.outgoing,
  p.incoming,
  p.unknown,
  p.total_duration_s,
  p.first_seen,
  p.last_seen
FROM pairs p;

-- ---------------------------------------------------------------------------------------------------------------------
-- v_cell_visits: per subscriber, cell and hour: how many events, with the cell's name, position and stated radius.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE OR REPLACE VIEW analyst.v_cell_visits AS
SELECT
  v.src,
  v.cell_key,
  date_trunc('hour', v.start_utc) AS hour_utc,
  count(*) AS events,
  c.site_name,
  c.lat,
  c.lon,
  c.radius_m
FROM analyst.v_comm_events v
LEFT JOIN analyst.v_cells c ON c.cell_key = v.cell_key
WHERE v.src IS NOT NULL AND v.cell_key IS NOT NULL
GROUP BY v.src, v.cell_key, date_trunc('hour', v.start_utc), c.site_name, c.lat, c.lon, c.radius_m;

-- ---------------------------------------------------------------------------------------------------------------------
-- v_device_pairings: which handset (IMEI) and SIM (IMSI) each number was seen with, and when.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE OR REPLACE VIEW analyst.v_device_pairings AS
SELECT
  v.src,
  v.imsi,
  v.imei,
  count(*) AS events,
  min(v.start_utc) AS first_seen,
  max(v.start_utc) AS last_seen
FROM analyst.v_comm_events v
WHERE v.src IS NOT NULL AND (v.imsi IS NOT NULL OR v.imei IS NOT NULL)
GROUP BY v.src, v.imsi, v.imei;

-- ---------------------------------------------------------------------------------------------------------------------
-- v_data_sessions: internet sessions (IPDR and similar), with the public address and port range for address-translation lookups.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE OR REPLACE VIEW analyst.v_data_sessions AS
SELECT
  s.session_id,
  s.evidence_id,
  s.msisdn AS src,
  s.imsi,
  s.imei,
  s.public_ip,
  s.public_port_from,
  s.public_port_to,
  s.ue_ip,
  s.server_ip,
  s.server_port,
  s.start_utc,
  s.end_utc,
  coalesce(s.nat_start_utc, s.start_utc) AS holds_from,
  coalesce(s.nat_end_utc, s.end_utc, s.start_utc) AS holds_to,
  s.app_category,
  s.app,
  s.host,
  s.bytes_up,
  s.bytes_down,
  s.duration_ms,
  s.cell_key
FROM forensic.data_session s
WHERE s.tenant_id = current_setting('app.tenant_id', true)
  AND s.collection_id = current_setting('app.collection_id', true)
  AND NOT ('duplicate_row' = ANY (s.quality_flags));

-- ---------------------------------------------------------------------------------------------------------------------
-- v_app_usage: application usage records per subscriber.
-- ---------------------------------------------------------------------------------------------------------------------
CREATE OR REPLACE VIEW analyst.v_app_usage AS
SELECT
  u.usage_id,
  u.evidence_id,
  u.msisdn AS src,
  u.imei,
  u.app_category,
  u.app,
  u.app_feature,
  u.start_utc,
  u.end_utc,
  u.province,
  u.city
FROM forensic.app_usage u
WHERE u.tenant_id = current_setting('app.tenant_id', true)
  AND u.collection_id = current_setting('app.collection_id', true)
  AND NOT ('duplicate_row' = ANY (u.quality_flags));

-- ---------------------------------------------------------------------------------------------------------------------
-- Descriptions: the schema catalogue (roadmap 6.8.3) is generated from these, so every view and every column has one (verify.sql checks it).
-- ---------------------------------------------------------------------------------------------------------------------
COMMENT ON VIEW analyst.v_comm_events IS 'Calls and SMS of the target call records, one row per event, repeats excluded, times in UTC. src is the subscriber whose records these are; dst is the other party.';
COMMENT ON VIEW analyst.v_cells IS 'Cell sites known to the case: name, position and the stated coverage radius. A cell is not a position of a person; show the radius with any location claim.';
COMMENT ON VIEW analyst.v_contact_pairs IS 'One row per subscriber (src) and other party (dst): event counts by direction, total duration, first and last contact. Use is_service and is_hub to leave out short codes, sender names and numbers that everyone contacts.';
COMMENT ON VIEW analyst.v_cell_visits IS 'Per subscriber, cell and hour: the number of events, with the cell name, position and radius. For where a number was and when.';
COMMENT ON VIEW analyst.v_device_pairings IS 'Which handset (IMEI) and SIM (IMSI) a number was seen with, with first and last time. For device and SIM changes.';
COMMENT ON VIEW analyst.v_data_sessions IS 'Internet sessions: the subscriber, the public address and port range (for who held an address at a time), the server, the application and the volumes.';
COMMENT ON VIEW analyst.v_app_usage IS 'Application usage records per subscriber: application, category, feature, time, province and city.';

DO $$
DECLARE
  d record;
BEGIN
  FOR d IN
    SELECT * FROM (VALUES
      ('v_comm_events', 'event_id', 'Identifier of the event row.'),
      ('v_comm_events', 'evidence_id', 'The evidence file the row came from; use it to cite the source.'),
      ('v_comm_events', 'source_row', 'Row number in the source file, counting from 0.'),
      ('v_comm_events', 'event_type', 'voice, sms, mms, data, ussd or other.'),
      ('v_comm_events', 'direction', 'outgoing, incoming, none or unknown, as the provider recorded it.'),
      ('v_comm_events', 'src', 'The subscriber number whose records these are (digits, normalised).'),
      ('v_comm_events', 'dst', 'The other party: a normalised number, or a short code, sender name or service label as written. Null when the file gives no usable counterparty (a sentinel value, the subscriber itself).'),
      ('v_comm_events', 'dst_kind', 'Class of the other party: person_like, international, short_code, sender_id, service_label or unknown.'),
      ('v_comm_events', 'dst_is_person_like', 'True when the other party is an ordinary or international number; false for short codes, sender names and services.'),
      ('v_comm_events', 'start_utc', 'Start of the event as a UTC instant. The case clock is Asia/Karachi: convert with AT TIME ZONE ''Asia/Karachi'' for local times.'),
      ('v_comm_events', 'end_utc', 'End of the event as a UTC instant, when recorded.'),
      ('v_comm_events', 'duration_s', 'Duration in seconds, when recorded.'),
      ('v_comm_events', 'imsi', 'SIM identity seen with the event, when recorded.'),
      ('v_comm_events', 'imei', 'Handset identity seen with the event, when recorded.'),
      ('v_comm_events', 'cell_key', 'Key of the serving cell, joins to v_cells.cell_key; null when the file gives no cell.'),
      ('v_comm_events', 'time_basis', 'How the source time zone is known: declared, assumed_by_profile, confirmed_by_analyst or explicit_offset.'),
      ('v_comm_events', 'source_tz', 'The time zone the source file was written in.'),
      ('v_cells', 'cell_key', 'Canonical cell key; joins to v_comm_events.cell_key and v_cell_visits.cell_key.'),
      ('v_cells', 'site_name', 'Name of the site, when known.'),
      ('v_cells', 'operator', 'Operator that runs the cell.'),
      ('v_cells', 'lat', 'Latitude of the site, degrees.'),
      ('v_cells', 'lon', 'Longitude of the site, degrees.'),
      ('v_cells', 'radius_m', 'Stated coverage radius in metres. A phone is somewhere within roughly this distance, not at the point.'),
      ('v_contact_pairs', 'src', 'The subscriber whose records these are.'),
      ('v_contact_pairs', 'dst', 'The other party.'),
      ('v_contact_pairs', 'is_service', 'True when the other party is not an ordinary or international number: a short code, a sender name, a service label.'),
      ('v_contact_pairs', 'is_hub', 'True when the other party is in contact with more than the case''s threshold of distinct subscribers (app.hub_threshold, 20 when unset): a number everyone contacts.'),
      ('v_contact_pairs', 'events', 'Number of events between the two.'),
      ('v_contact_pairs', 'outgoing', 'Events recorded as outgoing.'),
      ('v_contact_pairs', 'incoming', 'Events recorded as incoming.'),
      ('v_contact_pairs', 'unknown', 'Events whose direction is none or unknown.'),
      ('v_contact_pairs', 'total_duration_s', 'Sum of the durations in seconds over all events (null when none was recorded).'),
      ('v_contact_pairs', 'first_seen', 'Time of the first event, UTC.'),
      ('v_contact_pairs', 'last_seen', 'Time of the last event, UTC.'),
      ('v_cell_visits', 'src', 'The subscriber.'),
      ('v_cell_visits', 'cell_key', 'The cell the events were served by.'),
      ('v_cell_visits', 'hour_utc', 'The hour (UTC) the events fall in.'),
      ('v_cell_visits', 'events', 'Number of events of the subscriber on that cell in that hour.'),
      ('v_cell_visits', 'site_name', 'Name of the cell site.'),
      ('v_cell_visits', 'lat', 'Latitude of the site.'),
      ('v_cell_visits', 'lon', 'Longitude of the site.'),
      ('v_cell_visits', 'radius_m', 'Stated coverage radius in metres.'),
      ('v_device_pairings', 'src', 'The subscriber number.'),
      ('v_device_pairings', 'imsi', 'SIM identity.'),
      ('v_device_pairings', 'imei', 'Handset identity.'),
      ('v_device_pairings', 'events', 'Number of events seen with this pairing.'),
      ('v_device_pairings', 'first_seen', 'First time the pairing was seen, UTC.'),
      ('v_device_pairings', 'last_seen', 'Last time the pairing was seen, UTC.'),
      ('v_data_sessions', 'session_id', 'Identifier of the session row.'),
      ('v_data_sessions', 'evidence_id', 'The evidence file the row came from.'),
      ('v_data_sessions', 'src', 'The subscriber number of the session.'),
      ('v_data_sessions', 'imsi', 'SIM identity, when recorded.'),
      ('v_data_sessions', 'imei', 'Handset identity, when recorded.'),
      ('v_data_sessions', 'public_ip', 'The public (translated) address the session used.'),
      ('v_data_sessions', 'public_port_from', 'First port of the public port block.'),
      ('v_data_sessions', 'public_port_to', 'Last port of the public port block (inclusive).'),
      ('v_data_sessions', 'ue_ip', 'The private address of the device, when recorded.'),
      ('v_data_sessions', 'server_ip', 'The address of the server contacted.'),
      ('v_data_sessions', 'server_port', 'The port of the server contacted.'),
      ('v_data_sessions', 'start_utc', 'Session start, UTC.'),
      ('v_data_sessions', 'end_utc', 'Session end, UTC.'),
      ('v_data_sessions', 'holds_from', 'Start of the interval during which the public address and port block belonged to this subscriber (the translation start, else the session start).'),
      ('v_data_sessions', 'holds_to', 'End of that interval (the translation end, else the session end, else the start). Both ends are inclusive.'),
      ('v_data_sessions', 'app_category', 'Category of the application, as classified by the provider.'),
      ('v_data_sessions', 'app', 'Application, as classified by the provider (a vendor label, not content).'),
      ('v_data_sessions', 'host', 'Host name contacted, when recorded.'),
      ('v_data_sessions', 'bytes_up', 'Bytes sent.'),
      ('v_data_sessions', 'bytes_down', 'Bytes received.'),
      ('v_data_sessions', 'duration_ms', 'Duration in milliseconds, when recorded.'),
      ('v_data_sessions', 'cell_key', 'Serving cell, when recorded.'),
      ('v_app_usage', 'usage_id', 'Identifier of the usage row.'),
      ('v_app_usage', 'evidence_id', 'The evidence file the row came from.'),
      ('v_app_usage', 'src', 'The subscriber number.'),
      ('v_app_usage', 'imei', 'Handset identity, when recorded.'),
      ('v_app_usage', 'app_category', 'Category of the application.'),
      ('v_app_usage', 'app', 'Application name as classified by the provider.'),
      ('v_app_usage', 'app_feature', 'Feature of the application used (for example a call or a message), when recorded.'),
      ('v_app_usage', 'start_utc', 'Start of the usage record, UTC.'),
      ('v_app_usage', 'end_utc', 'End of the usage record, UTC.'),
      ('v_app_usage', 'province', 'Province, when recorded.'),
      ('v_app_usage', 'city', 'City, when recorded.')
    ) AS t(view_name, column_name, description)
  LOOP
    EXECUTE format('COMMENT ON COLUMN analyst.%I.%I IS %L', d.view_name, d.column_name, d.description);
  END LOOP;
END
$$;

-- ---------------------------------------------------------------------------------------------------------------------
-- The read-only role: it can read the analyst views and nothing else. Its privileges are granted view by view, never on the forensic schema.
-- ---------------------------------------------------------------------------------------------------------------------
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_analyst_ro') THEN
    CREATE ROLE forensic_analyst_ro NOLOGIN;
  END IF;
END
$$;
ALTER ROLE forensic_analyst_ro NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOINHERIT NOBYPASSRLS;

-- Exact, repeatable privileges: whatever the role held before is taken away first, so applying the migration again repairs a role that was given too much.
REVOKE ALL ON SCHEMA forensic FROM forensic_analyst_ro;
REVOKE ALL ON ALL TABLES IN SCHEMA forensic FROM forensic_analyst_ro;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA forensic FROM forensic_analyst_ro;
REVOKE ALL ON SCHEMA analyst FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA analyst FROM PUBLIC;
REVOKE ALL ON ALL TABLES IN SCHEMA analyst FROM forensic_analyst_ro;
GRANT USAGE ON SCHEMA analyst TO forensic_analyst_ro;
GRANT SELECT ON ALL TABLES IN SCHEMA analyst TO forensic_analyst_ro;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'forensic_runtime') THEN
    EXECUTE 'GRANT forensic_analyst_ro TO forensic_runtime';
  END IF;
END
$$;
COMMIT;
