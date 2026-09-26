\set ON_ERROR_STOP on
\if :{?collection_id}
\else
\set collection_id 'nexusai-forensic-demo'
\endif

BEGIN READ ONLY;

-- Independent oracle for WI-LAYER-2. This reads raw preserved values directly;
-- it does not reuse the semantic-layer loader or an execution helper.
WITH values_by_side(relationship, side, record_id, join_key) AS (
    SELECT 'cdr_subscriber_msisdn', 'left', record_id,
           btrim(raw_payload->>'MSISDN')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'cdr'
    UNION ALL
    SELECT 'cdr_subscriber_msisdn', 'right', record_id,
           btrim(COALESCE(raw_payload->>'mobile_no', raw_payload->>'MSISDN',
                          raw_payload->>'account_msisdn', raw_payload->>'phone_number',
                          raw_payload->>'subscriber_number'))
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'subscriber'
    UNION ALL
    SELECT 'cdr_tower_cell_site', 'left', record_id,
           btrim(raw_payload->>'Cell_SITE_ID')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'cdr'
    UNION ALL
    SELECT 'cdr_tower_cell_site', 'right', record_id,
           btrim(raw_payload->>'Site Code')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'tower_location'
    UNION ALL
    SELECT 'cdr_tower_site_id', 'left', record_id,
           btrim(raw_payload->>'Site_Id')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'cdr'
    UNION ALL
    SELECT 'cdr_tower_site_id', 'right', record_id,
           btrim(raw_payload->>'Site Code')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'tower_location'
    UNION ALL
    SELECT 'cdr_tower_lac', 'left', record_id,
           btrim(raw_payload->>'Lac_Id')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'cdr'
    UNION ALL
    SELECT 'cdr_tower_lac', 'right', record_id,
           btrim(raw_payload->>'LAC')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'tower_location'
    UNION ALL
    SELECT 'ipdr_subscriber_msisdn', 'left', record_id,
           btrim(COALESCE(raw_payload->>'subscriber_id', raw_payload->>'msisdn'))
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'ipdr'
    UNION ALL
    SELECT 'ipdr_subscriber_msisdn', 'right', record_id,
           btrim(COALESCE(raw_payload->>'mobile_no', raw_payload->>'MSISDN',
                          raw_payload->>'account_msisdn', raw_payload->>'phone_number',
                          raw_payload->>'subscriber_number'))
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'subscriber'
    UNION ALL
    SELECT 'cdr_ipdr_msisdn', 'left', record_id,
           btrim(raw_payload->>'MSISDN')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'cdr'
    UNION ALL
    SELECT 'cdr_ipdr_msisdn', 'right', record_id,
           btrim(COALESCE(raw_payload->>'subscriber_id', raw_payload->>'msisdn'))
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'ipdr'
    UNION ALL
    SELECT 'transaction_subscriber_msisdn', 'left', record_id,
           btrim(raw_payload->>'account')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'transaction'
    UNION ALL
    SELECT 'transaction_subscriber_msisdn', 'right', record_id,
           btrim(COALESCE(raw_payload->>'mobile_no', raw_payload->>'MSISDN',
                          raw_payload->>'account_msisdn', raw_payload->>'phone_number',
                          raw_payload->>'subscriber_number'))
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'subscriber'
    UNION ALL
    SELECT 'transaction_subscriber_id', 'left', record_id,
           btrim(raw_payload->>'account')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'transaction'
    UNION ALL
    SELECT 'transaction_subscriber_id', 'right', record_id,
           btrim(COALESCE(raw_payload->>'subscriber_id', raw_payload->>'customer_id',
                          raw_payload->>'customer_no'))
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'subscriber'
    UNION ALL
    SELECT 'anpr_tower_location', 'left', record_id,
           btrim(raw_payload->>'location')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'anpr'
    UNION ALL
    SELECT 'anpr_tower_location', 'right', record_id,
           btrim(raw_payload->>'Site Location')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'tower_location'
    UNION ALL
    SELECT 'anpr_tower_coordinates', 'left', record_id,
           jsonb_build_array(raw_payload->>'latitude', raw_payload->>'longitude')::text
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'anpr'
    UNION ALL
    SELECT 'anpr_tower_coordinates', 'right', record_id,
           jsonb_build_array(raw_payload->>'Latitude WGS84',
                             raw_payload->>'Longitude WGS84')::text
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'tower_location'
), nonempty AS (
    SELECT *
      FROM values_by_side
     WHERE join_key IS NOT NULL
       AND join_key <> ''
       AND join_key <> '[null, null]'
), side_stats AS (
    SELECT relationship,
           count(*) FILTER (WHERE side = 'left') AS left_rows,
           count(DISTINCT join_key) FILTER (WHERE side = 'left') AS left_keys,
           count(*) FILTER (WHERE side = 'right') AS right_rows,
           count(DISTINCT join_key) FILTER (WHERE side = 'right') AS right_keys
      FROM nonempty
     GROUP BY relationship
), match_stats AS (
    SELECT left_side.relationship,
           count(*) AS pair_rows,
           count(DISTINCT left_side.record_id) AS matched_left_rows,
           count(DISTINCT right_side.record_id) AS matched_right_rows
      FROM nonempty left_side
      JOIN nonempty right_side
        ON right_side.relationship = left_side.relationship
       AND right_side.side = 'right'
       AND right_side.join_key = left_side.join_key
     WHERE left_side.side = 'left'
     GROUP BY left_side.relationship
), duplicate_stats AS (
    SELECT relationship, side, count(*) AS duplicate_keys
      FROM (
            SELECT relationship, side, join_key
              FROM nonempty
             GROUP BY relationship, side, join_key
            HAVING count(*) > 1
           ) duplicates
     GROUP BY relationship, side
)
SELECT side_stats.relationship,
       left_rows,
       left_keys,
       right_rows,
       right_keys,
       COALESCE(pair_rows, 0) AS pair_rows,
       COALESCE(matched_left_rows, 0) AS matched_left_rows,
       COALESCE(matched_right_rows, 0) AS matched_right_rows,
       COALESCE(left_duplicates.duplicate_keys, 0) AS left_duplicate_keys,
       COALESCE(right_duplicates.duplicate_keys, 0) AS right_duplicate_keys
  FROM side_stats
  LEFT JOIN match_stats USING (relationship)
  LEFT JOIN duplicate_stats left_duplicates
    ON left_duplicates.relationship = side_stats.relationship
   AND left_duplicates.side = 'left'
  LEFT JOIN duplicate_stats right_duplicates
    ON right_duplicates.relationship = side_stats.relationship
   AND right_duplicates.side = 'right'
 ORDER BY side_stats.relationship;

-- An exact site-code collision is not enough. Both apparent Cell_SITE_ID/Site
-- Code matches disagree on LAC, so the site-code edge is rejected.
WITH cdr AS (
    SELECT record_id,
           btrim(raw_payload->>'Cell_SITE_ID') AS cell_site,
           btrim(raw_payload->>'Lac_Id') AS lac
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'cdr'
), tower AS (
    SELECT record_id,
           btrim(raw_payload->>'Site Code') AS site_code,
           btrim(raw_payload->>'LAC') AS lac
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'tower_location'
)
SELECT count(*) AS site_match_pairs,
       count(*) FILTER (WHERE cdr.lac = tower.lac) AS consistent_lac_pairs,
       count(*) FILTER (WHERE cdr.lac IS DISTINCT FROM tower.lac) AS conflicting_lac_pairs
  FROM cdr
  JOIN tower ON cdr.cell_site = tower.site_code;

-- Verify that the zero subscriber joins are not merely +92/03 formatting drift.
WITH raw_phone(relationship, side, record_id, value) AS (
    SELECT 'cdr_subscriber_msisdn', 'left', record_id, raw_payload->>'MSISDN'
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'cdr'
    UNION ALL
    SELECT 'cdr_subscriber_msisdn', 'right', record_id,
           COALESCE(raw_payload->>'mobile_no', raw_payload->>'MSISDN',
                    raw_payload->>'account_msisdn', raw_payload->>'phone_number',
                    raw_payload->>'subscriber_number')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'subscriber'
    UNION ALL
    SELECT 'ipdr_subscriber_msisdn', 'left', record_id,
           COALESCE(raw_payload->>'subscriber_id', raw_payload->>'msisdn')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'ipdr'
    UNION ALL
    SELECT 'ipdr_subscriber_msisdn', 'right', record_id,
           COALESCE(raw_payload->>'mobile_no', raw_payload->>'MSISDN',
                    raw_payload->>'account_msisdn', raw_payload->>'phone_number',
                    raw_payload->>'subscriber_number')
      FROM forensic.records
     WHERE collection_id = :'collection_id' AND record_type = 'subscriber'
), digits AS (
    SELECT relationship, side, record_id,
           regexp_replace(COALESCE(value, ''), '[^0-9]', '', 'g') AS value
      FROM raw_phone
), normalized AS (
    SELECT relationship, side, record_id,
           CASE
             WHEN length(value) = 12 AND left(value, 2) = '92'
               THEN '0' || substr(value, 3)
             WHEN length(value) = 10 AND left(value, 1) = '3'
               THEN '0' || value
             ELSE value
           END AS join_key
      FROM digits
     WHERE value <> ''
)
SELECT left_side.relationship,
       count(*) AS normalized_pair_rows,
       count(DISTINCT left_side.record_id) AS matched_left_rows,
       count(DISTINCT right_side.record_id) AS matched_right_rows
  FROM normalized left_side
  JOIN normalized right_side
    ON right_side.relationship = left_side.relationship
   AND right_side.side = 'right'
   AND right_side.join_key = left_side.join_key
 WHERE left_side.side = 'left'
 GROUP BY left_side.relationship
UNION ALL
SELECT relationship, 0, 0, 0
  FROM (VALUES ('cdr_subscriber_msisdn'), ('ipdr_subscriber_msisdn')) expected(relationship)
 WHERE NOT EXISTS (
       SELECT 1
         FROM normalized left_side
         JOIN normalized right_side
           ON right_side.relationship = left_side.relationship
          AND right_side.side = 'right'
          AND right_side.join_key = left_side.join_key
        WHERE left_side.side = 'left'
          AND left_side.relationship = expected.relationship
 )
 ORDER BY relationship;

COMMIT;
