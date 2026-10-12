-- Read-only compatibility checks. Run immediately before migration 008.

BEGIN TRANSACTION READ ONLY;

DO $$
DECLARE
  invalid_count bigint;
BEGIN
  SELECT count(*) INTO invalid_count
  FROM forensic.evidence_items
  WHERE sha256 !~ '^[0-9a-f]{64}$';
  IF invalid_count <> 0 THEN
    RAISE EXCEPTION '% evidence item(s) have an invalid lowercase SHA-256', invalid_count;
  END IF;

  SELECT count(*) INTO invalid_count
  FROM forensic.evidence_items
  WHERE size_bytes IS NOT NULL AND size_bytes < 0;
  IF invalid_count <> 0 THEN
    RAISE EXCEPTION '% evidence item(s) have a negative byte size', invalid_count;
  END IF;

  SELECT count(*) INTO invalid_count
  FROM forensic.evidence_items
  WHERE metadata ? 'storage_layout_version'
    AND metadata->>'storage_layout_version' <> 'sha256-scope-v1';
  IF invalid_count <> 0 THEN
    RAISE EXCEPTION '% evidence item(s) declare an unsupported storage layout', invalid_count;
  END IF;

  SELECT count(*) INTO invalid_count
  FROM forensic.evidence_items
  WHERE metadata->>'storage_layout_version' = 'sha256-scope-v1'
    AND (
      size_bytes IS NULL
      OR raw_storage_ref !~ '^forensic-spool://sha256-scope-v1/[0-9a-f]{64}/[0-9a-f]{64}$'
      OR split_part(raw_storage_ref, '/', 5) <> sha256
      OR metadata->>'storage_scope_key' <> split_part(raw_storage_ref, '/', 4)
      OR metadata->>'storage_scope_key' <> encode(digest(convert_to(
        octet_length(tenant_id)::text || ':' || tenant_id ||
        octet_length(collection_id)::text || ':' || collection_id,
        'UTF8'
      ), 'sha256'), 'hex')
      OR metadata->>'storage_receipt_uri' <> 'forensic-spool-receipt://sha256-scope-v1/' ||
        split_part(raw_storage_ref, '/', 4) || '/' || sha256
      OR metadata->>'storage_integrity_state' <> 'verified'
      OR metadata->>'storage_write_once' <> 'true'
      OR metadata->>'storage_verification_method' <> 'sha256-full-read-after-retain'
    );
  IF invalid_count <> 0 THEN
    RAISE EXCEPTION '% content-addressed evidence item(s) have inconsistent storage identity metadata', invalid_count;
  END IF;

  SELECT count(*) INTO invalid_count
  FROM forensic.records_ingest_jobs
  WHERE evidence_id IS NOT NULL
    AND sha256 !~ '^[0-9a-f]{64}$';
  IF invalid_count <> 0 THEN
    RAISE EXCEPTION '% linked ingest job(s) have an invalid lowercase SHA-256', invalid_count;
  END IF;

  SELECT count(*) INTO invalid_count
  FROM forensic.records_ingest_jobs jobs
  JOIN forensic.evidence_items evidence ON evidence.evidence_id = jobs.evidence_id
  WHERE jobs.evidence_id IS NOT NULL
    AND (jobs.tenant_id, jobs.collection_id) IS DISTINCT FROM
        (evidence.tenant_id, evidence.collection_id);
  IF invalid_count <> 0 THEN
    RAISE EXCEPTION '% ingest job(s) point across tenant/collection scope', invalid_count;
  END IF;

  SELECT count(*) INTO invalid_count
  FROM forensic.kb_collection_assets assets
  JOIN forensic.evidence_items evidence ON evidence.evidence_id = assets.evidence_id
  WHERE assets.evidence_id IS NOT NULL
    AND (assets.tenant_id, assets.collection_id) IS DISTINCT FROM
        (evidence.tenant_id, evidence.collection_id);
  IF invalid_count <> 0 THEN
    RAISE EXCEPTION '% KB asset(s) point across tenant/collection scope', invalid_count;
  END IF;

  SELECT count(*) INTO invalid_count
  FROM forensic.records_ingest_jobs
  WHERE evidence_id IS NOT NULL
    AND (
      (started_at IS NOT NULL AND started_at < queued_at)
      OR (completed_at IS NOT NULL AND started_at IS NOT NULL AND completed_at < started_at)
    );
  IF invalid_count <> 0 THEN
    RAISE EXCEPTION '% linked ingest job(s) have non-monotonic lifecycle times', invalid_count;
  END IF;

  SELECT count(*) INTO invalid_count
  FROM (
    SELECT normalized_version_id
    FROM (
      SELECT CASE
        WHEN coalesce(metadata->>'version_id', '') ~
             '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89aAbB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$'
        THEN (metadata->>'version_id')::uuid
        ELSE evidence_id
      END AS normalized_version_id
      FROM forensic.evidence_items
    ) versions
    GROUP BY normalized_version_id
    HAVING count(*) > 1
  ) collisions;
  IF invalid_count <> 0 THEN
    RAISE EXCEPTION '% normalized evidence version ID collision(s) detected', invalid_count;
  END IF;
END $$;

SELECT
  (SELECT count(*) FROM forensic.evidence_items) AS evidence_items,
  (SELECT count(*) FROM forensic.kb_collection_assets WHERE evidence_id IS NOT NULL) AS linked_kb_assets,
  (SELECT count(*) FROM forensic.records_ingest_jobs WHERE evidence_id IS NOT NULL) AS linked_ingest_jobs,
  'phase3_control_plane_preflight_passed' AS result;

COMMIT;
