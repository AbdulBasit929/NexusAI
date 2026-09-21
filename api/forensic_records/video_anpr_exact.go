package main

// Exact lookup searches retained observations, not just the winning grouped
// candidate. The lateral lookup supplies context without multiplying readings.
const videoANPRExactReadingsSQL = `
SELECT raw.artifact_id, raw.evidence_id, raw.version_id,
       evidence.original_filename AS source_file,
       raw.metadata #>> '{observation,normalized_plate_text}' AS normalized_plate_text,
       raw.metadata #>> '{observation,raw_plate_text}' AS raw_plate_text,
       'raw_plate_observation' AS match_kind,
       context.artifact_id AS group_artifact_id,
       context.selected_plate AS group_selected_plate_text,
       (raw.citation_locator->>'timestamp_seconds')::double precision AS first_seen_seconds,
       (raw.citation_locator->>'timestamp_seconds')::double precision AS last_seen_seconds,
       raw.citation_locator AS best_observation_locator,
       raw.citation_locator AS citation_locator,
       raw.citation_locator->'frame_number' AS frame_number,
       raw.confidence AS best_confidence, 1 AS sightings_count,
       true AS manual_review_required,
       'nexusai://evidence/' || raw.evidence_id::text || '/artifacts/' || raw.artifact_id::text AS citation_ref
FROM forensic.derived_artifacts raw
JOIN forensic.evidence_items evidence
  ON evidence.tenant_id=raw.tenant_id AND evidence.collection_id=raw.collection_id
 AND evidence.evidence_id=raw.evidence_id AND evidence.current_version_id=raw.version_id
LEFT JOIN LATERAL (
  SELECT groups.artifact_id, groups.metadata #>> '{observation,normalized_plate_text}' AS selected_plate
  FROM forensic.derived_artifacts groups
  WHERE groups.tenant_id=raw.tenant_id AND groups.collection_id=raw.collection_id
    AND groups.evidence_id=raw.evidence_id AND groups.version_id=raw.version_id
    AND groups.artifact_type='forensics.video-anpr-plate-group/v1'
    AND groups.processing_status='completed'
    AND (groups.metadata #> '{observation,all_observation_ids}') ? raw.artifact_id::text
  ORDER BY groups.artifact_id LIMIT 1
) context ON true
WHERE raw.tenant_id=$1 AND raw.collection_id=$2 AND raw.evidence_id=$3::uuid
  AND raw.artifact_type='forensics.anpr-observation/v1' AND raw.processing_status='completed'
  AND raw.metadata #>> '{observation,normalized_plate_text}' = regexp_replace(upper($4), '[^[:alnum:]]', '', 'g')
  AND ($5::double precision IS NULL OR (raw.citation_locator->>'timestamp_seconds')::double precision >= $5)
  AND ($6::double precision IS NULL OR (raw.citation_locator->>'timestamp_seconds')::double precision <= $6)
ORDER BY first_seen_seconds, raw.artifact_id
LIMIT $7`
