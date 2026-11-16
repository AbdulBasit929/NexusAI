-- Migration 037: the audio files of a case as an analyst view, and the language of every speech reading (additive).
--
-- Why. Measured 2026-11-13 on ten public recordings put through the interface with no language chosen (docs/data/speech-lab-results-20261112.md, section 10; bank questions VB-SPEE-001 to 031: 5 correct,
-- 4 wrong, 22 declined). The reading was good; the analyst views could not say what an analyst asks about a recording:
--   * LANGUAGE. v_speech_segments.detected_language was filled for English and for the Pashto route only. A Whisper reading of Urdu carries the language in `language` (and, through the worker's
--     Hindi-as-Urdu step, in `requested_language` although nobody requested it). "Which recordings are in Urdu?" had no column to filter on and was declined, and "how many are in English?" counted all ten.
--   * LENGTH. The length of an audio file is stored (evidence_items.media_metadata, read from the file's own header at upload) but no view exposed it: v_video_files covers videos only. "How long is ...",
--     "longer than 9 seconds" and "all together" were declined or answered from the wrong column (the end of the last segment is not the length: Whisper ended a 6.0 s clip at 7.0 s).
--   * SOUNDTRACKS. v_speech_segments mixes recordings with the soundtracks of videos, so "the speech segments of the audio recordings" counted 39 (12 and 27).
--
-- What it does (a view and three appended columns; no table, row or privilege other than SELECT on the new view changes):
--   1. analyst.v_audio_files: one row per audio file: length in seconds (the container's own figure), sample rate, channels, the language of the recording (the language most of its segments carry, with
--      its plain name), how it was read (reading_status: MACHINE_TRANSCRIPT, SEARCH_LEAD for Pashto, LOW_RESOURCE_LANGUAGE, or NOT_READ when it has no segment yet), the number of segments, size and state.
--   2. analyst.v_speech_segments gains four columns at the end: language (the code the model or the route reports, from detected_language, else language, else requested_language), language_name (Pashto,
--      Urdu, English, Punjabi, ... in words), reading_status and source_kind ('audio' for a recording, 'video' for the soundtrack of a video). The existing columns keep their meaning and position.
--   3. Descriptions for the model (it reads them from the database's own comments).
--
-- The Ask engine learns the view only when its catalogue is rebuilt with it (ask_engine_catalogue.golden.json, tools/case_generator/check_ask_catalogue.py --write-golden). Rollback:
-- ops/037_audio_files_and_speech_language/rollback.sql (drops v_audio_files and restores v_speech_segments as it was). Idempotent. Apply by hand on a retained database: ops/.../preflight.sql first,
-- verify.sql after, and only after a backup and a rehearsal on a restored copy.
BEGIN;
SET LOCAL lock_timeout = '10s';

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
  d.created_at,
  lower(NULLIF(btrim(COALESCE(NULLIF(d.metadata -> 'observation' ->> 'detected_language', ''), NULLIF(d.metadata -> 'observation' ->> 'language', ''), NULLIF(d.metadata -> 'observation' ->> 'requested_language', ''))), '')) AS language,
  CASE lower(NULLIF(btrim(COALESCE(NULLIF(d.metadata -> 'observation' ->> 'detected_language', ''), NULLIF(d.metadata -> 'observation' ->> 'language', ''), NULLIF(d.metadata -> 'observation' ->> 'requested_language', ''))), ''))
    WHEN 'ps' THEN 'Pashto' WHEN 'pus' THEN 'Pashto' WHEN 'pbt' THEN 'Pashto' WHEN 'pbu' THEN 'Pashto' WHEN 'pst' THEN 'Pashto'
    WHEN 'ur' THEN 'Urdu' WHEN 'en' THEN 'English' WHEN 'pa' THEN 'Punjabi' WHEN 'pnb' THEN 'Punjabi' WHEN 'fa' THEN 'Persian' WHEN 'prs' THEN 'Persian'
    WHEN 'ar' THEN 'Arabic' WHEN 'hi' THEN 'Hindi' WHEN 'sd' THEN 'Sindhi' WHEN 'skr' THEN 'Saraiki' WHEN 'bal' THEN 'Balochi' WHEN 'zh' THEN 'Chinese' WHEN 'ru' THEN 'Russian'
    ELSE lower(NULLIF(btrim(COALESCE(NULLIF(d.metadata -> 'observation' ->> 'detected_language', ''), NULLIF(d.metadata -> 'observation' ->> 'language', ''), NULLIF(d.metadata -> 'observation' ->> 'requested_language', ''))), ''))
  END AS language_name,
  COALESCE(NULLIF(d.metadata -> 'observation' ->> 'reliability_state', ''), 'MACHINE_TRANSCRIPT') AS reading_status,
  e.modality AS source_kind
FROM forensic.derived_artifacts d
JOIN forensic.evidence_items e
  ON e.tenant_id = d.tenant_id AND e.collection_id = d.collection_id AND e.evidence_id = d.evidence_id
WHERE d.artifact_type = 'forensics.audio-timestamp-segment/v1'
  AND d.processing_status = 'completed'
  AND d.tenant_id = current_setting('app.tenant_id', true)
  AND d.collection_id = current_setting('app.collection_id', true);

CREATE OR REPLACE VIEW analyst.v_audio_files AS
SELECT
  e.evidence_id,
  e.original_filename,
  round(((e.media_metadata ->> 'duration_ms')::numeric) / 1000, 2) AS duration_seconds,
  (e.media_metadata ->> 'sample_rate_hz')::integer AS sample_rate_hz,
  (e.media_metadata ->> 'channels')::integer AS channels,
  s.language,
  s.language_name,
  CASE
    WHEN s.segments IS NULL THEN 'NOT_READ'
    WHEN s.search_lead THEN 'SEARCH_LEAD'
    WHEN s.low_resource THEN 'LOW_RESOURCE_LANGUAGE'
    ELSE 'MACHINE_TRANSCRIPT'
  END AS reading_status,
  COALESCE(s.segments, 0) AS segments,
  e.size_bytes,
  e.processing_status,
  e.created_at,
  true AS needs_review
FROM forensic.evidence_items e
LEFT JOIN LATERAL (
  SELECT
    count(*) AS segments,
    mode() WITHIN GROUP (ORDER BY g.language) AS language,
    mode() WITHIN GROUP (ORDER BY g.language_name) AS language_name,
    bool_or(g.reading_status = 'SEARCH_LEAD') AS search_lead,
    bool_or(g.reading_status = 'LOW_RESOURCE_LANGUAGE') AS low_resource
  FROM analyst.v_speech_segments g
  WHERE g.evidence_id = e.evidence_id
  HAVING count(*) > 0
) s ON true
WHERE e.modality = 'audio'
  AND e.tenant_id = current_setting('app.tenant_id', true)
  AND e.collection_id = current_setting('app.collection_id', true);

COMMENT ON VIEW analyst.v_speech_segments IS
  'What was said, one row per timed segment of the speech to text of audio recordings (source_kind audio) and of the soundtracks of videos (source_kind video); start and end are seconds from the start of the file. A machine transcript of unverified quality (needs_review is always true); Pashto is a search lead and Urdu and Punjabi are less reliable than English. For the recordings themselves (how many, how long, which language) use v_audio_files; to separate recordings from video soundtracks filter source_kind. The language of a segment is in language and language_name, not in requested_language.';
COMMENT ON COLUMN analyst.v_speech_segments.requested_language IS
  'The language the processing was told to expect (ur, en, pa, ps) or empty when it was left to the model. It is not the language that was spoken: use language or language_name for that.';
COMMENT ON COLUMN analyst.v_speech_segments.detected_language IS
  'The language the model or the language route reported for the segment, when it reported one; empty for older readings. Use language, which falls back to the other fields.';
COMMENT ON COLUMN analyst.v_speech_segments.language IS
  'The language of the segment as a short code (en English, ur Urdu, ps Pashto, pa Punjabi, fa Persian, ...), from what the model or the route reported; empty when none was reported.';
COMMENT ON COLUMN analyst.v_speech_segments.language_name IS
  'The language of the segment in words (English, Urdu, Pashto, Punjabi, ...): filter on this for a question that names a language, for example language_name = ''Pashto''.';
COMMENT ON COLUMN analyst.v_speech_segments.reading_status IS
  'How far the reading can be relied on: MACHINE_TRANSCRIPT (a transcript to be checked by listening), SEARCH_LEAD (Pashto: good enough to find a recording by a spoken word, not to read as a transcript) or LOW_RESOURCE_LANGUAGE (a language the speech model reads poorly).';
COMMENT ON COLUMN analyst.v_speech_segments.source_kind IS
  'audio for a segment of an audio recording, video for a segment of the soundtrack of a video. Filter source_kind = ''audio'' for questions about recordings only.';

COMMENT ON VIEW analyst.v_audio_files IS
  'One row per audio file (recording) of the case: its length in seconds (running time or duration), the language spoken, how it was read and how many speech segments it has. Use it, and not v_evidence_inventory, for anything about the recordings themselves: how many there are, which is the longest or shortest, how long they are in total or longer than some seconds, which are in Pashto, Urdu or English. Every recording needs a person''s check, so a question about which recordings need manual review or checking is answered by listing every row; reading_status says how far a reading can be relied on and is not a filter for that. For what was said use v_speech_segments (source_kind audio); the soundtrack of a video is not here (see v_video_files.has_audio). A reading is a machine candidate: a person must listen before relying on it.';
DO $$
DECLARE
  d record;
BEGIN
  FOR d IN
    SELECT * FROM (VALUES
      ('v_audio_files', 'evidence_id', 'Identifier of the audio evidence item.'),
      ('v_audio_files', 'original_filename', 'File name of the recording as it was added, exactly as written (for example voice-note-03.wav).'),
      ('v_audio_files', 'duration_seconds', 'Length of the recording in seconds (its running time or duration), from the audio file''s own header; empty when the header could not be read. Use this, not the end of the last speech segment, for how long a recording is.'),
      ('v_audio_files', 'sample_rate_hz', 'Samples per second of the audio, from the file''s header (16000 for telephone-quality speech, 44100 for music).'),
      ('v_audio_files', 'channels', 'Number of audio channels: 1 mono, 2 stereo.'),
      ('v_audio_files', 'language', 'The language spoken as a short code (en English, ur Urdu, ps Pashto, pa Punjabi, ...): the one most of the recording''s segments carry; empty when the recording has not been read.'),
      ('v_audio_files', 'language_name', 'The language spoken in words (English, Urdu, Pashto, Punjabi, ...): filter on this for a question that names a language, for example language_name = ''Pashto''; empty when the recording has not been read.'),
      ('v_audio_files', 'reading_status', 'How the recording was read: MACHINE_TRANSCRIPT (a transcript to be checked by listening), SEARCH_LEAD (Pashto: good enough to find the recording by a spoken word, not to read as a transcript), LOW_RESOURCE_LANGUAGE (a language read poorly) or NOT_READ (no speech segment yet). Every reading needs a person''s check.'),
      ('v_audio_files', 'segments', 'Number of timed speech segments the recording has.'),
      ('v_audio_files', 'size_bytes', 'Size of the file in bytes.'),
      ('v_audio_files', 'processing_status', 'Where the file is in processing (for example completed or failed).'),
      ('v_audio_files', 'created_at', 'When the file was added, UTC.'),
      ('v_audio_files', 'needs_review', 'True for every row: a person must listen and check before the language or the text of a recording is relied on. A question about which recordings need manual review is answered by listing every row; do not filter on reading_status for it.')
    ) AS t(view_name, column_name, description)
  LOOP
    EXECUTE format('COMMENT ON COLUMN analyst.%I.%I IS %L', d.view_name, d.column_name, d.description);
  END LOOP;
END
$$;

-- The role reads the new view and nothing else new: SELECT, no other privilege; v_speech_segments keeps the grants it had.
REVOKE ALL ON analyst.v_audio_files FROM PUBLIC;
GRANT SELECT ON analyst.v_audio_files TO forensic_analyst_ro;
COMMIT;
