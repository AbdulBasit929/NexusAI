-- Rollback for 037_audio_files_and_speech_language.sql: drops analyst.v_audio_files and restores analyst.v_speech_segments as migration 023 and the descriptions of migration 035 left it
-- (the four appended columns cannot be removed from a view, so it is dropped and created again; the grants are given again). No table or row was changed by the migration.
-- An Ask engine whose catalogue lists v_audio_files, or the new columns, must be rolled back with it, or the validator rejects questions that name them.
BEGIN;
DROP VIEW IF EXISTS analyst.v_audio_files;
DROP VIEW IF EXISTS analyst.v_speech_segments;
CREATE VIEW analyst.v_speech_segments AS
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
COMMENT ON VIEW analyst.v_speech_segments IS
  'Speech to text of audio and video evidence, one row per timed segment (start and end in seconds from the start of the recording). Machine transcript of unverified quality, needs_review is always true; Urdu, Punjabi and Pashto readings are less reliable than English.';
COMMENT ON COLUMN analyst.v_speech_segments.evidence_id IS 'Identifier of the audio or video evidence item.';
COMMENT ON COLUMN analyst.v_speech_segments.original_filename IS 'File name of the recording as it was added.';
COMMENT ON COLUMN analyst.v_speech_segments.start_seconds IS 'Start of the segment in seconds from the start of the recording (a time written 1:30 in a question is 90 seconds, minutes and seconds).';
COMMENT ON COLUMN analyst.v_speech_segments.end_seconds IS 'End of the segment in seconds from the start of the recording (a time written 1:30 in a question is 90 seconds, minutes and seconds).';
COMMENT ON COLUMN analyst.v_speech_segments.text IS 'What the model heard in the segment, as written by the model; not a verified transcript.';
COMMENT ON COLUMN analyst.v_speech_segments.requested_language IS 'The language the processing was told to expect (ur, en, pa, ps) or empty when it was left to the model.';
COMMENT ON COLUMN analyst.v_speech_segments.detected_language IS 'The language the model reports it heard, when it was asked to detect it.';
COMMENT ON COLUMN analyst.v_speech_segments.model IS 'The speech model that produced the segment.';
COMMENT ON COLUMN analyst.v_speech_segments.needs_review IS 'True for every row: a person must listen and check before the text is relied on. Do not filter on it: needs_review = false matches no row.';
COMMENT ON COLUMN analyst.v_speech_segments.created_at IS 'When the segment was stored, UTC.';
REVOKE ALL ON analyst.v_speech_segments FROM PUBLIC;
GRANT SELECT ON analyst.v_speech_segments TO forensic_analyst_ro;
COMMIT;
