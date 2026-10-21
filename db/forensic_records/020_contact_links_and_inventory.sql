-- Migration 020: analyst views for "how are A and B connected" without a recursive query, and for "what evidence does this case hold" (roadmap 6.8.13, S4; G10, D8).
--
-- Why. The slowest class of question is the connection question (decision D22: two numbers are connected when they are in contact, directly or through others, in either direction).
-- The engine's verified example is a bounded recursive query of about 220 tokens; the model writes about 5.5 tokens a second on the laptop, so the query alone is 40 seconds, and a
-- long query is a likelier place for a mistake (measured 2026-10-19/20: 59 to 120 s on small cases, 182 to 235 s on the real collection, the model almost all of it). A view that
-- already holds the walk lets the query be one line: `SELECT src, dst FROM v_connection_edges WHERE a = '...' AND b = '...' AND ordinary AND rank_ordinary = 1 LIMIT 200`.
-- And "how many images are in this case" is withheld today because no view describes the evidence itself (G10): v_evidence_inventory is that view.
--
-- What is added (three views; nothing existing changes; no table is touched):
--   * analyst.v_contact_links      every contact as an undirected link: a, b (both orders present), events, ordinary (the link is neither to a service nor through a number everyone contacts).
--   * analyst.v_connection_edges   for a pair (a, b): the EDGES of every path of one, two or three links between them (no recursion), with the number of links (hops), whether every
--                                  link of the path is ordinary, and the rank of the path among all paths (rank_any) and among ordinary paths (rank_ordinary): rank 1 is the shortest.
--   * analyst.v_evidence_inventory one row per evidence item of the case: file name, modality, detected type, extension, size, processing status, time.
-- A pair is asked for with `WHERE a = ... AND b = ...`: the predicates reach the links through the union and the window (they are the partition columns), so a question reads the pairs it
-- names, not every pair of the case. Paths longer than three links are not here: a question that asks for more still needs the recursive query (the verified example stays).
--
-- Same scope as every analyst view (tenant and collection from the transaction settings), same role (forensic_analyst_ro gets SELECT on each new view only). Additive and idempotent.
-- Apply by hand on a retained database: ops/020_contact_links_and_inventory/preflight.sql first, verify.sql after; rollback.sql drops the three views.
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE OR REPLACE VIEW analyst.v_contact_links AS
SELECT
  x.a,
  x.b,
  bool_or(NOT x.is_service AND NOT x.is_hub) AS ordinary,
  sum(x.events)::bigint AS events
FROM (
  SELECT p.src AS a, p.dst AS b, p.is_service, p.is_hub, p.events FROM analyst.v_contact_pairs p
  UNION ALL
  SELECT p.dst AS a, p.src AS b, p.is_service, p.is_hub, p.events FROM analyst.v_contact_pairs p
) x
WHERE x.a <> x.b
GROUP BY x.a, x.b;

-- The links are computed ONCE per query (MATERIALIZED): the base view aggregates every event of the case, and reading it in each of the six branches below took 24 s on a case of 21,000 call
-- records against 0.15 s for the recursive example. The predicates on a and b still reach the branches (they filter the scans of the materialised links, not its computation).
CREATE OR REPLACE VIEW analyst.v_connection_edges AS
WITH links AS MATERIALIZED (
  SELECT k.a, k.b, k.ordinary FROM analyst.v_contact_links k
)
SELECT
  e.a, e.b, e.src, e.dst, e.hops, e.ordinary,
  dense_rank() OVER (PARTITION BY e.a, e.b ORDER BY e.hops) AS rank_any,
  CASE WHEN e.ordinary THEN dense_rank() OVER (PARTITION BY e.a, e.b, e.ordinary ORDER BY e.hops) END AS rank_ordinary
FROM (
  SELECT DISTINCT u.a, u.b, u.src, u.dst, u.hops, u.ordinary
  FROM (
    -- one link
    SELECT l.a AS a, l.b AS b, l.a AS src, l.b AS dst, 1 AS hops, l.ordinary AS ordinary
    FROM links l
    UNION ALL
    -- two links: a - m - b
    SELECT l1.a, l2.b, l1.a, l1.b, 2, (l1.ordinary AND l2.ordinary)
    FROM links l1 JOIN links l2 ON l2.a = l1.b
    WHERE l2.b <> l1.a
    UNION ALL
    SELECT l1.a, l2.b, l2.a, l2.b, 2, (l1.ordinary AND l2.ordinary)
    FROM links l1 JOIN links l2 ON l2.a = l1.b
    WHERE l2.b <> l1.a
    UNION ALL
    -- three links: a - m - n - b
    SELECT l1.a, l3.b, l1.a, l1.b, 3, (l1.ordinary AND l2.ordinary AND l3.ordinary)
    FROM links l1 JOIN links l2 ON l2.a = l1.b JOIN links l3 ON l3.a = l2.b
    WHERE l2.b <> l1.a AND l3.b <> l2.a AND l3.b <> l1.a
    UNION ALL
    SELECT l1.a, l3.b, l2.a, l2.b, 3, (l1.ordinary AND l2.ordinary AND l3.ordinary)
    FROM links l1 JOIN links l2 ON l2.a = l1.b JOIN links l3 ON l3.a = l2.b
    WHERE l2.b <> l1.a AND l3.b <> l2.a AND l3.b <> l1.a
    UNION ALL
    SELECT l1.a, l3.b, l3.a, l3.b, 3, (l1.ordinary AND l2.ordinary AND l3.ordinary)
    FROM links l1 JOIN links l2 ON l2.a = l1.b JOIN links l3 ON l3.a = l2.b
    WHERE l2.b <> l1.a AND l3.b <> l2.a AND l3.b <> l1.a
  ) u
) e;

CREATE OR REPLACE VIEW analyst.v_evidence_inventory AS
SELECT
  i.evidence_id,
  i.original_filename,
  i.modality,
  i.detected_type,
  i.extension,
  i.size_bytes,
  i.processing_status,
  i.created_at
FROM forensic.evidence_items i
WHERE i.tenant_id = current_setting('app.tenant_id', true)
  AND i.collection_id = current_setting('app.collection_id', true);

COMMENT ON VIEW analyst.v_contact_links IS 'Every contact of the case as an undirected link between two numbers (both orders are present), with the number of events and whether the link is ordinary: neither to a service nor through a number everyone contacts. The building block of v_connection_edges.';
COMMENT ON VIEW analyst.v_connection_edges IS 'How two numbers are connected, through calls and messages in either direction, within three links: ask for a pair with a = first number AND b = second number. Each row is one link (src to dst) of a path from a to b; hops is how many links the path has (1 direct, 2 through one shared contact, 3 through two numbers); ordinary is true when no link of the path goes to a service or through a number everyone contacts; rank_any = 1 marks the shortest paths, rank_ordinary = 1 the shortest ordinary paths. Use for connected, connection, path, link, chain, route, shared contact, how are A and B related, within N hops (up to 3).';
COMMENT ON VIEW analyst.v_evidence_inventory IS 'The evidence files of the case, one row each: name, modality (image, audio, video, document, structured_records), detected type, extension, size, processing status and time added. For how many files, images, documents or recordings the case holds, and which failed.';

DO $$
DECLARE
  d record;
BEGIN
  FOR d IN
    SELECT * FROM (VALUES
      ('v_contact_links', 'a', 'One of the two numbers of the link; each link is present in both orders.'),
      ('v_contact_links', 'b', 'The other number of the link.'),
      ('v_contact_links', 'ordinary', 'True when the link comes from an ordinary contact: neither to a service nor to a number everyone contacts.'),
      ('v_contact_links', 'events', 'Number of events between the two numbers, both directions together.'),
      ('v_connection_edges', 'a', 'The first number of the pair asked about: always write a = ''...'' in the query.'),
      ('v_connection_edges', 'b', 'The second number of the pair asked about: always write b = ''...'' in the query.'),
      ('v_connection_edges', 'src', 'One end of a link of a path from a to b.'),
      ('v_connection_edges', 'dst', 'The other end of that link.'),
      ('v_connection_edges', 'hops', 'Number of links of the path the row belongs to: 1 direct contact, 2 through one shared contact, 3 through two numbers.'),
      ('v_connection_edges', 'ordinary', 'True when no link of the path goes to a service or through a number everyone contacts; use it when the question leaves services and hubs out.'),
      ('v_connection_edges', 'rank_any', '1 for the shortest paths between a and b, 2 for the next length, and so on.'),
      ('v_connection_edges', 'rank_ordinary', '1 for the shortest ORDINARY paths (null for a path that is not ordinary); use rank_ordinary = 1 with ordinary when services and hubs are left out.'),
      ('v_evidence_inventory', 'evidence_id', 'Identifier of the evidence item.'),
      ('v_evidence_inventory', 'original_filename', 'File name as it was added.'),
      ('v_evidence_inventory', 'modality', 'image, audio, video, document or structured_records.'),
      ('v_evidence_inventory', 'detected_type', 'What the file was recognised as: cdr, ipdr, anpr, subscriber, transaction, access_log, tower_location, pdf, image, audio, video, document or generic.'),
      ('v_evidence_inventory', 'extension', 'File extension.'),
      ('v_evidence_inventory', 'size_bytes', 'Size of the file in bytes.'),
      ('v_evidence_inventory', 'processing_status', 'completed, failed or still processing.'),
      ('v_evidence_inventory', 'created_at', 'When the item was added to the case, UTC.')
    ) AS t(view_name, column_name, description)
  LOOP
    EXECUTE format('COMMENT ON COLUMN analyst.%I.%I IS %L', d.view_name, d.column_name, d.description);
  END LOOP;
END
$$;

-- The role reads the new views and nothing else: SELECT is granted view by view; no other privilege on them.
REVOKE ALL ON analyst.v_contact_links, analyst.v_connection_edges, analyst.v_evidence_inventory FROM PUBLIC;
GRANT SELECT ON analyst.v_contact_links, analyst.v_connection_edges, analyst.v_evidence_inventory TO forensic_analyst_ro;
COMMIT;
