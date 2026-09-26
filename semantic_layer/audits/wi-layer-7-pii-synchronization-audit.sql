-- WI-LAYER-7 read-only PII and Urdu/Roman-Urdu synchronization oracle.
-- Run against nexusai-multimodal-product-acceptance. The result contains only
-- counts and booleans: no transcript, identifier, plate or filename is selected.

BEGIN READ ONLY;

WITH roman AS (
  SELECT
    artifact_id,
    evidence_id,
    version_id,
    citation_locator,
    metadata #>> '{observation,raw_urdu_text}' AS raw_urdu_text,
    metadata #>> '{observation,roman_urdu_text}' AS roman_urdu_text,
    metadata #>> '{observation,parent_observation_id}' AS parent_observation_id,
    metadata #>> '{observation,start_seconds}' AS start_seconds,
    metadata #>> '{observation,end_seconds}' AS end_seconds,
    metadata #>> '{observation,identifier_preservation_pass}' AS identifier_preservation_pass,
    metadata #>> '{observation,processor}' AS processor,
    metadata #>> '{observation,processor_revision}' AS processor_revision
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-roman-urdu-segment/v1'
), timed AS (
  SELECT
    evidence_id,
    version_id,
    citation_locator,
    metadata ->> 'observation_id' AS observation_id,
    metadata #>> '{observation,text}' AS transcript_text,
    metadata #>> '{observation,start_seconds}' AS start_seconds,
    metadata #>> '{observation,end_seconds}' AS end_seconds
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-timestamp-segment/v1'
), linked AS (
  SELECT
    r.*,
    t.observation_id AS matched_parent_observation_id,
    t.transcript_text,
    t.start_seconds AS parent_start_seconds,
    t.end_seconds AS parent_end_seconds,
    t.citation_locator AS parent_citation_locator
  FROM roman r
  LEFT JOIN timed t
    ON t.evidence_id = r.evidence_id
   AND t.version_id = r.version_id
   AND t.observation_id = r.parent_observation_id
)
SELECT
  count(*) AS roman_rows,
  count(raw_urdu_text) AS raw_urdu_present,
  count(roman_urdu_text) AS roman_urdu_present,
  count(parent_observation_id) AS parent_id_present,
  count(DISTINCT parent_observation_id) AS distinct_parent_ids,
  count(matched_parent_observation_id) AS exact_parent_links,
  count(*) FILTER (WHERE raw_urdu_text IS NOT DISTINCT FROM transcript_text) AS raw_matches_parent_text,
  count(*) FILTER (
    WHERE start_seconds IS NOT DISTINCT FROM parent_start_seconds
      AND end_seconds IS NOT DISTINCT FROM parent_end_seconds
  ) AS timing_matches_parent,
  count(*) FILTER (WHERE citation_locator IS NOT DISTINCT FROM parent_citation_locator) AS locator_matches_parent,
  count(*) FILTER (WHERE identifier_preservation_pass = 'true') AS identifier_preservation_true,
  bool_and(processor = 'nexusai-deterministic-urdu-transliteration') AS one_expected_processor,
  bool_and(processor_revision = 'roman-urdu-m1-v1') AS one_expected_revision
FROM linked;

WITH roman AS (
  SELECT
    evidence_id,
    version_id,
    metadata ->> 'observation_type' AS source_modality,
    metadata #>> '{observation,parent_observation_id}' AS parent_observation_id,
    metadata #>> '{observation,raw_urdu_text}' AS raw_urdu_text,
    metadata #>> '{observation,start_seconds}' AS start_seconds,
    metadata #>> '{observation,end_seconds}' AS end_seconds,
    citation_locator
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-roman-urdu-segment/v1'
), timed AS (
  SELECT
    evidence_id,
    version_id,
    metadata ->> 'observation_id' AS observation_id,
    metadata #>> '{observation,text}' AS transcript_text,
    metadata #>> '{observation,start_seconds}' AS start_seconds,
    metadata #>> '{observation,end_seconds}' AS end_seconds,
    citation_locator
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-timestamp-segment/v1'
)
SELECT
  r.source_modality,
  count(*) AS roman_rows,
  count(*) FILTER (WHERE EXISTS (
    SELECT 1 FROM timed t
    WHERE t.evidence_id = r.evidence_id
      AND t.version_id = r.version_id
      AND t.observation_id = r.parent_observation_id
  )) AS exact_parent_links,
  count(*) FILTER (WHERE EXISTS (
    SELECT 1 FROM timed t
    WHERE t.evidence_id = r.evidence_id
      AND t.version_id = r.version_id
      AND t.transcript_text IS NOT DISTINCT FROM r.raw_urdu_text
      AND t.start_seconds IS NOT DISTINCT FROM r.start_seconds
      AND t.end_seconds IS NOT DISTINCT FROM r.end_seconds
      AND t.citation_locator IS NOT DISTINCT FROM r.citation_locator
  )) AS same_scope_text_time_locator_candidates
FROM roman r
GROUP BY r.source_modality
ORDER BY r.source_modality;

WITH roman AS (
  SELECT
    metadata #>> '{observation,raw_urdu_text}' AS raw_urdu_text,
    metadata #>> '{observation,roman_urdu_text}' AS roman_urdu_text
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-roman-urdu-segment/v1'
)
SELECT
  count(*) AS roman_rows,
  count(*) FILTER (WHERE raw_urdu_text ~ '[0-9]') AS raw_with_ascii_digits,
  count(*) FILTER (WHERE raw_urdu_text ~ '[٠١٢٣٤٥٦٧٨٩]') AS raw_with_arabic_indic_digits,
  count(*) FILTER (WHERE raw_urdu_text ~ '[۰۱۲۳۴۵۶۷۸۹]') AS raw_with_eastern_arabic_digits,
  count(*) FILTER (WHERE roman_urdu_text ~ '[0-9]') AS roman_with_ascii_digits,
  count(*) FILTER (WHERE roman_urdu_text ~ '[٠١٢٣٤٥٦٧٨٩]') AS roman_with_arabic_indic_digits,
  count(*) FILTER (WHERE roman_urdu_text ~ '[۰۱۲۳۴۵۶۷۸۹]') AS roman_with_eastern_arabic_digits
FROM roman;

WITH fields AS (
  SELECT 'anpr_model_observation.plate_text' AS field_id,
         metadata #>> '{observation,normalized_plate_text}' AS value
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.anpr-observation/v1'
  UNION ALL
  SELECT 'video_anpr_plate_group.plate_text',
         metadata #>> '{observation,normalized_plate_text}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.video-anpr-plate-group/v1'
  UNION ALL
  SELECT 'image_ocr_observation.raw_text', metadata #>> '{observation,raw_text}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-ocr-observation/v1'
  UNION ALL
  SELECT 'image_ocr_observation.normalized_text', metadata #>> '{observation,normalized_text}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.image-ocr-observation/v1'
  UNION ALL
  SELECT 'audio_timestamp_segment.text', metadata #>> '{observation,text}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-timestamp-segment/v1'
  UNION ALL
  SELECT 'audio_roman_urdu_segment.raw_urdu_text', metadata #>> '{observation,raw_urdu_text}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-roman-urdu-segment/v1'
  UNION ALL
  SELECT 'audio_roman_urdu_segment.roman_urdu_text', metadata #>> '{observation,roman_urdu_text}'
  FROM forensic.derived_artifacts
  WHERE collection_id = 'nexusai-multimodal-product-acceptance'
    AND artifact_type = 'forensics.audio-roman-urdu-segment/v1'
), normalized AS (
  SELECT
    field_id,
    value,
    translate(value, '٠١٢٣٤٥٦٧٨٩۰۱۲۳۴۵۶۷۸۹', '01234567890123456789') AS ascii_digits
  FROM fields
)
SELECT
  field_id,
  count(*) AS rows,
  count(value) AS populated_rows,
  count(*) FILTER (
    WHERE ascii_digits ~* '(^|[^0-9])(\+?92|0)?3[0-9]{2}[- .()]?[0-9]{7}([^0-9]|$)'
  ) AS phone_like_rows,
  count(*) FILTER (
    WHERE ascii_digits ~ '(^|[^0-9])[0-9]{5}-?[0-9]{7}-?[0-9]([^0-9]|$)'
  ) AS cnic_like_rows,
  count(*) FILTER (
    WHERE value ~* '(^|[^[:alnum:]_.+-])[[:alnum:]_.+-]+@[[:alnum:].-]+\.[[:alpha:]]{2,}([^[:alnum:]_.+-]|$)'
  ) AS email_like_rows,
  count(*) FILTER (
    WHERE ascii_digits ~ '(^|[^0-9])([0-9]{1,3}\.){3}[0-9]{1,3}([^0-9]|$)'
  ) AS ipv4_like_rows,
  count(*) FILTER (
    WHERE value ~* '(^|[^[:alnum:]])PK[0-9]{2}[[:alnum:]]{20}([^[:alnum:]]|$)'
  ) AS pakistan_iban_like_rows,
  count(*) FILTER (WHERE value ~ '[٠١٢٣٤٥٦٧٨٩۰۱۲۳۴۵۶۷۸۹]') AS non_ascii_digit_rows
FROM normalized
GROUP BY field_id
ORDER BY field_id;

SELECT
  count(*) AS evidence_files,
  count(original_filename) AS filenames_present,
  bool_and(original_filename IS NOT NULL AND btrim(original_filename) <> '') AS every_filename_nonempty
FROM forensic.evidence_items
WHERE collection_id = 'nexusai-multimodal-product-acceptance';

COMMIT;
