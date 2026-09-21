BEGIN TRANSACTION READ ONLY;
SELECT set_config('app.tenant_id', 'default', true);

-- Representative aggregate lineage for the highest-ranked outgoing contact.
WITH scoped AS (
  SELECT CASE
           WHEN regexp_replace(coalesce(r.call_dialed_num, ''), '\D', '', 'g') = regexp_replace('923001110001', '\D', '', 'g')
             THEN coalesce(nullif(r.call_org_num, ''), nullif(r.msisdn, ''))
           ELSE r.call_dialed_num
         END AS result_key,
         r.batch_id, r.source_file, r.row_number, r.row_hash
  FROM forensic.cdr_records r
  WHERE r.tenant_id = 'default' AND r.collection_id = 'nexusai-forensic-demo'
    AND r.direction = 'OUTGOING'
    AND (
      r.msisdn = '923001110001' OR r.call_org_num = '923001110001' OR r.call_dialed_num = '923001110001'
      OR regexp_replace(coalesce(r.msisdn, ''), '\D', '', 'g') = '923001110001'
      OR regexp_replace(coalesce(r.call_org_num, ''), '\D', '', 'g') = '923001110001'
      OR regexp_replace(coalesce(r.call_dialed_num, ''), '\D', '', 'g') = '923001110001'
    )
), top_contact AS (
  SELECT result_key
  FROM scoped
  WHERE result_key ~ '^[0-9]{8,19}$' AND result_key <> '923001110001'
  GROUP BY result_key
  ORDER BY count(*) DESC, result_key
  LIMIT 1
), grouped AS (
  SELECT s.result_key,
         coalesce(j.evidence_id::text, '') AS evidence_id,
         coalesce(e.current_version_id::text, '') AS version_id,
         s.source_file,
         count(*)::bigint AS contribution_count,
         min(s.row_number)::bigint AS first_row,
         max(s.row_number)::bigint AS last_row,
         encode(digest(concat_ws('|', count(*)::text, min(s.row_hash), max(s.row_hash), min(s.row_number)::text, max(s.row_number)::text), 'sha256'), 'hex') AS row_group_digest
  FROM scoped s
  JOIN top_contact t USING (result_key)
  LEFT JOIN forensic.records_ingest_jobs j
    ON j.tenant_id = 'default' AND j.collection_id = 'nexusai-forensic-demo' AND j.id = s.batch_id
  LEFT JOIN forensic.evidence_items e
    ON e.tenant_id = 'default' AND e.collection_id = 'nexusai-forensic-demo' AND e.evidence_id = j.evidence_id
  GROUP BY s.result_key, j.evidence_id, e.current_version_id, s.source_file
)
SELECT 'frequent_contacts' AS operation, result_key, evidence_id, version_id, source_file,
       contribution_count, first_row, last_row, length(row_group_digest) AS digest_length
FROM grouped
ORDER BY contribution_count DESC, source_file;

-- Representative aggregate lineage for the exact APF-3 temporal acceptance query.
WITH grouped AS (
  SELECT coalesce(j.evidence_id::text, '') AS evidence_id,
         coalesce(e.current_version_id::text, '') AS version_id,
         r.source_file,
         count(*)::bigint AS contribution_count,
         min(r.row_number)::bigint AS first_row,
         max(r.row_number)::bigint AS last_row,
         encode(digest(concat_ws('|', count(*)::text, min(r.row_hash), max(r.row_hash), min(r.row_number)::text, max(r.row_number)::text), 'sha256'), 'hex') AS row_group_digest
  FROM forensic.cdr_records r
  LEFT JOIN forensic.records_ingest_jobs j
    ON j.tenant_id = 'default' AND j.collection_id = 'nexusai-forensic-demo' AND j.id = r.batch_id
  LEFT JOIN forensic.evidence_items e
    ON e.tenant_id = 'default' AND e.collection_id = 'nexusai-forensic-demo' AND e.evidence_id = j.evidence_id
  WHERE r.tenant_id = 'default' AND r.collection_id = 'nexusai-forensic-demo'
    AND (
      r.msisdn = '923001234567' OR r.call_org_num = '923001234567' OR r.call_dialed_num = '923001234567'
      OR regexp_replace(coalesce(r.msisdn, ''), '\D', '', 'g') = '923001234567'
      OR regexp_replace(coalesce(r.call_org_num, ''), '\D', '', 'g') = '923001234567'
      OR regexp_replace(coalesce(r.call_dialed_num, ''), '\D', '', 'g') = '923001234567'
    )
    AND r.call_start_ts >= '2026-07-10T00:00:00Z'::timestamptz
    AND r.call_start_ts < '2026-07-11T00:00:00Z'::timestamptz
  GROUP BY j.evidence_id, e.current_version_id, r.source_file
)
SELECT 'temporal_activity' AS operation, '' AS result_key, evidence_id, version_id, source_file,
       contribution_count, first_row, last_row, length(row_group_digest) AS digest_length
FROM grouped
ORDER BY contribution_count DESC, source_file;

ROLLBACK;
