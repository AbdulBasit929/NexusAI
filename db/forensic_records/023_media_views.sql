-- Migration 023: analyst views over what the media roles read (text in images, speech, number plates), so the Ask engine and the analyst screens can query them in one line (media activation, track D/A/P; master plan P-M).
--
-- Why. The workers already store every machine reading as a derived artifact (forensic.derived_artifacts: artifact_type, metadata.observation, citation_locator, confidence). A question such as
-- "which photo has the word TOTAL on it" or "what was said between 0:10 and 0:30 of call-17.wav" cannot be answered from the JSON by a model writing SQL without getting the paths wrong; a view
-- with named, described columns lets the query be `SELECT original_filename FROM analyst.v_ocr_regions WHERE text ILIKE '%TOTAL%'`. Nothing is interpreted here: every row is one machine
-- reading, shown as read, with its confidence and the fact that a person has to check it (needs_review).
--
-- What is added (four views; nothing existing changes; no table is touched; no data is written):
--   * analyst.v_ocr_regions            one row per text region the OCR role found in an image: the text as read, the normalised text, the script, the reader that produced it, confidence, box.
--   * analyst.v_ocr_evidence_summary   one row per image with its regions joined in reading order, the number of regions, mean and lowest confidence.
--   * analyst.v_speech_segments        one row per timed speech segment: start and end in seconds, text, requested and detected language, the model.
--   * analyst.v_plate_reads            one row per number plate reading: from an image (one sighting) or from a video (a group of sightings of the same plate with first and last second seen).
-- Plate readings and speech are model candidates: they are never an identification of a person or a vehicle owner, and every row says needs_review = true. A view for image fingerprints (near
-- duplicates) is NOT here: its method and threshold are decided by the track I measurement first, and faces are not here: identity claims are an owner decision (D33 and the face gate).
--
-- Same scope as every analyst view (tenant and collection from the transaction settings), same role (forensic_analyst_ro gets SELECT on each new view only). Additive and idempotent.
-- Apply by hand on a retained database: ops/023_media_views/preflight.sql first, verify.sql after; rollback.sql drops the four views. The Ask engine learns the views only when its catalogue is
-- rebuilt with them (a separate step with its own gate); until then the views are readable by the role and invisible to the engine.
BEGIN;
SET LOCAL lock_timeout = '10s';

CREATE OR REPLACE VIEW analyst.v_ocr_regions AS
SELECT
  d.evidence_id,
  e.original_filename,
  COALESCE((d.citation_locator ->> 'reading_order')::integer, (d.metadata -> 'observation' ->> 'reading_order')::integer) AS region_no,
  d.metadata -> 'observation' ->> 'raw_text' AS text,
  d.metadata -> 'observation' ->> 'normalized_text' AS normalized_text,
  CASE
    WHEN d.metadata -> 'observation' ->> 'script_family' ILIKE 'arabic%' THEN 'arabic'
    WHEN d.metadata -> 'observation' ->> 'script_family' ILIKE 'latin%' THEN 'latin'
    WHEN d.metadata -> 'observation' ->> 'script_family' ILIKE 'mixed%' THEN 'mixed'
    ELSE 'undetermined'
  END AS script,
  COALESCE(d.metadata -> 'observation' ->> 'selected_recognizer', d.metadata ->> 'processor') AS reader,
  d.confidence,
  (d.metadata -> 'observation' -> 'bbox' ->> 'x')::numeric AS x,
  (d.metadata -> 'observation' -> 'bbox' ->> 'y')::numeric AS y,
  (d.metadata -> 'observation' -> 'bbox' ->> 'width')::numeric AS width,
  (d.metadata -> 'observation' -> 'bbox' ->> 'height')::numeric AS height,
  COALESCE((d.metadata -> 'observation' ->> 'manual_review_required')::boolean, true) AS needs_review,
  d.created_at
FROM forensic.derived_artifacts d
JOIN forensic.evidence_items e
  ON e.tenant_id = d.tenant_id AND e.collection_id = d.collection_id AND e.evidence_id = d.evidence_id
WHERE d.artifact_type = 'forensics.image-ocr-observation/v1'
  AND d.processing_status = 'completed'
  AND d.tenant_id = current_setting('app.tenant_id', true)
  AND d.collection_id = current_setting('app.collection_id', true);

CREATE OR REPLACE VIEW analyst.v_ocr_evidence_summary AS
SELECT
  r.evidence_id,
  r.original_filename,
  count(*)::bigint AS regions,
  round(avg(r.confidence), 4) AS mean_confidence,
  min(r.confidence) AS lowest_confidence,
  string_agg(r.text, E'\n' ORDER BY r.region_no NULLS LAST, r.y NULLS LAST, r.x NULLS LAST) AS text,
  bool_or(r.needs_review) AS needs_review
FROM analyst.v_ocr_regions r
GROUP BY r.evidence_id, r.original_filename;

CREATE OR REPLACE VIEW analyst.v_speech_segments AS
SELECT
  d.evidence_id,
  e.original_filename,
  COALESCE((d.citation_locator ->> 'start_seconds')::numeric, (d.metadata -> 'observation' ->> 'start_seconds')::numeric) AS start_seconds,
  COALESCE((d.citation_locator ->> 'end_seconds')::numeric, (d.metadata -> 'observation' ->> 'end_seconds')::numeric) AS end_seconds,
  d.metadata -> 'observation' ->> 'text' AS text,
  d.metadata -> 'observation' ->> 'requested_language' AS requested_language,
  d.metadata -> 'observation' ->> 'detected_language' AS detected_language,
  d.metadata -> 'observation' ->> 'model' AS model,
  COALESCE((d.metadata -> 'observation' ->> 'manual_review_required')::boolean, true) AS needs_review,
  d.created_at
FROM forensic.derived_artifacts d
JOIN forensic.evidence_items e
  ON e.tenant_id = d.tenant_id AND e.collection_id = d.collection_id AND e.evidence_id = d.evidence_id
WHERE d.artifact_type = 'forensics.audio-timestamp-segment/v1'
  AND d.processing_status = 'completed'
  AND d.tenant_id = current_setting('app.tenant_id', true)
  AND d.collection_id = current_setting('app.collection_id', true);

CREATE OR REPLACE VIEW analyst.v_plate_reads AS
SELECT
  d.evidence_id,
  e.original_filename,
  'image'::text AS source,
  d.metadata -> 'observation' ->> 'normalized_plate_text' AS plate,
  d.metadata -> 'observation' ->> 'raw_plate_text' AS plate_as_read,
  (d.metadata -> 'observation' ->> 'detection_confidence')::numeric AS detection_confidence,
  (d.metadata -> 'observation' ->> 'ocr_confidence')::numeric AS read_confidence,
  1::bigint AS sightings,
  NULL::numeric AS first_seen_seconds,
  NULL::numeric AS last_seen_seconds,
  true AS needs_review,
  d.created_at
FROM forensic.derived_artifacts d
JOIN forensic.evidence_items e
  ON e.tenant_id = d.tenant_id AND e.collection_id = d.collection_id AND e.evidence_id = d.evidence_id
WHERE d.artifact_type = 'forensics.anpr-observation/v1'
  AND d.processing_status = 'completed'
  AND d.tenant_id = current_setting('app.tenant_id', true)
  AND d.collection_id = current_setting('app.collection_id', true)
UNION ALL
SELECT
  d.evidence_id,
  e.original_filename,
  'video'::text AS source,
  d.metadata -> 'observation' ->> 'normalized_plate_text' AS plate,
  d.metadata -> 'observation' ->> 'selected_raw_plate_text' AS plate_as_read,
  NULL::numeric AS detection_confidence,
  d.confidence AS read_confidence,
  COALESCE((d.metadata -> 'observation' ->> 'sightings_count')::bigint, 1) AS sightings,
  COALESCE((d.citation_locator ->> 'first_seen_seconds')::numeric, (d.metadata -> 'observation' ->> 'first_seen_seconds')::numeric) AS first_seen_seconds,
  COALESCE((d.citation_locator ->> 'last_seen_seconds')::numeric, (d.metadata -> 'observation' ->> 'last_seen_seconds')::numeric) AS last_seen_seconds,
  true AS needs_review,
  d.created_at
FROM forensic.derived_artifacts d
JOIN forensic.evidence_items e
  ON e.tenant_id = d.tenant_id AND e.collection_id = d.collection_id AND e.evidence_id = d.evidence_id
WHERE d.artifact_type = 'forensics.video-anpr-plate-group/v1'
  AND d.processing_status = 'completed'
  AND d.tenant_id = current_setting('app.tenant_id', true)
  AND d.collection_id = current_setting('app.collection_id', true);

COMMENT ON VIEW analyst.v_ocr_regions IS
  'Text the OCR role read from images, one row per region of text, as read (nothing corrected). Every row is a machine reading that a person must check (needs_review). Use text ILIKE for a word; a value read from a photo is not a fact about the case until checked.';
COMMENT ON VIEW analyst.v_ocr_evidence_summary IS
  'One row per image that has OCR text: the regions joined in reading order, how many there are, mean and lowest confidence. Use it to find the images that mention a word or an identifier, or to count images with text.';
COMMENT ON VIEW analyst.v_speech_segments IS
  'Speech to text of audio and video evidence, one row per timed segment (start and end in seconds from the start of the recording). Machine transcript of unverified quality, needs_review is always true; Urdu, Punjabi and Pashto readings are less reliable than English.';
COMMENT ON VIEW analyst.v_plate_reads IS
  'Number plates a model read from images (one sighting per row) or from video (one row per plate group with first and last second seen). A model candidate that needs review: never an identification of a vehicle or its owner.';

DO $$
DECLARE
  d record;
BEGIN
  FOR d IN
    SELECT * FROM (VALUES
      ('v_ocr_regions', 'evidence_id', 'Identifier of the image evidence item.'),
      ('v_ocr_regions', 'original_filename', 'File name of the image as it was added.'),
      ('v_ocr_regions', 'region_no', 'Reading order of the region on the image: 1 is read first.'),
      ('v_ocr_regions', 'text', 'The text of the region exactly as the model read it, in the language of the image; digits and letters are not corrected.'),
      ('v_ocr_regions', 'normalized_text', 'The same text after Unicode normalisation and whitespace clean-up; use it for matching, show text to the analyst.'),
      ('v_ocr_regions', 'script', 'latin, arabic (Urdu, Pashto, Arabic), mixed (both in one region) or undetermined when the region is only digits or symbols.'),
      ('v_ocr_regions', 'reader', 'Which reader produced the text (the name of the recogniser or of the OCR engine); readings from different readers are not equally reliable.'),
      ('v_ocr_regions', 'confidence', 'The reader''s own confidence from 0 to 1; a high value is not a proof that the text is right.'),
      ('v_ocr_regions', 'x', 'Left edge of the region in pixels of the original image.'),
      ('v_ocr_regions', 'y', 'Top edge of the region in pixels of the original image.'),
      ('v_ocr_regions', 'width', 'Width of the region in pixels of the original image.'),
      ('v_ocr_regions', 'height', 'Height of the region in pixels of the original image.'),
      ('v_ocr_regions', 'needs_review', 'True: a person must check the reading before it is relied on (always true for a machine reading).'),
      ('v_ocr_regions', 'created_at', 'When the reading was stored, UTC.'),
      ('v_ocr_evidence_summary', 'evidence_id', 'Identifier of the image evidence item.'),
      ('v_ocr_evidence_summary', 'original_filename', 'File name of the image as it was added.'),
      ('v_ocr_evidence_summary', 'regions', 'Number of text regions read on the image.'),
      ('v_ocr_evidence_summary', 'mean_confidence', 'Mean of the readers'' confidence over the regions, 0 to 1.'),
      ('v_ocr_evidence_summary', 'lowest_confidence', 'The lowest confidence among the regions; a low value marks an image worth checking first.'),
      ('v_ocr_evidence_summary', 'text', 'All the regions joined in reading order, one per line, as read.'),
      ('v_ocr_evidence_summary', 'needs_review', 'True: a person must check the reading before it is relied on.'),
      ('v_speech_segments', 'evidence_id', 'Identifier of the audio or video evidence item.'),
      ('v_speech_segments', 'original_filename', 'File name of the recording as it was added.'),
      ('v_speech_segments', 'start_seconds', 'Start of the segment in seconds from the start of the recording.'),
      ('v_speech_segments', 'end_seconds', 'End of the segment in seconds from the start of the recording.'),
      ('v_speech_segments', 'text', 'What the model heard in the segment, as written by the model; not a verified transcript.'),
      ('v_speech_segments', 'requested_language', 'The language the processing was told to expect (ur, en, pa, ps) or empty when it was left to the model.'),
      ('v_speech_segments', 'detected_language', 'The language the model reports it heard, when it was asked to detect it.'),
      ('v_speech_segments', 'model', 'The speech model that produced the segment.'),
      ('v_speech_segments', 'needs_review', 'True: a person must listen and check before the text is relied on (always true).'),
      ('v_speech_segments', 'created_at', 'When the segment was stored, UTC.'),
      ('v_plate_reads', 'evidence_id', 'Identifier of the image or video evidence item.'),
      ('v_plate_reads', 'original_filename', 'File name of the image or video as it was added.'),
      ('v_plate_reads', 'source', 'image for one sighting in a photo, video for a group of sightings of the same plate in a video.'),
      ('v_plate_reads', 'plate', 'The plate text after normalisation (capital letters and digits only); use it for matching.'),
      ('v_plate_reads', 'plate_as_read', 'The plate text exactly as the reader gave it, before normalisation.'),
      ('v_plate_reads', 'detection_confidence', 'How sure the detector is that the box is a plate, 0 to 1 (empty for a video group).'),
      ('v_plate_reads', 'read_confidence', 'The reader''s confidence in the characters, 0 to 1; a letter-digit confusion such as O and 0 can remain at high confidence.'),
      ('v_plate_reads', 'sightings', 'Number of frames or images in which the plate was seen (1 for a photo).'),
      ('v_plate_reads', 'first_seen_seconds', 'For a video, the second the plate was first seen; empty for a photo.'),
      ('v_plate_reads', 'last_seen_seconds', 'For a video, the second the plate was last seen; empty for a photo.'),
      ('v_plate_reads', 'needs_review', 'True: a model candidate that a person must check (always true); never an identification of a vehicle owner.'),
      ('v_plate_reads', 'created_at', 'When the reading was stored, UTC.')
    ) AS t(view_name, column_name, description)
  LOOP
    EXECUTE format('COMMENT ON COLUMN analyst.%I.%I IS %L', d.view_name, d.column_name, d.description);
  END LOOP;
END
$$;

-- The role reads the new views and nothing else: SELECT is granted view by view; no other privilege on them.
REVOKE ALL ON analyst.v_ocr_regions, analyst.v_ocr_evidence_summary, analyst.v_speech_segments, analyst.v_plate_reads FROM PUBLIC;
GRANT SELECT ON analyst.v_ocr_regions, analyst.v_ocr_evidence_summary, analyst.v_speech_segments, analyst.v_plate_reads TO forensic_analyst_ro;
COMMIT;
