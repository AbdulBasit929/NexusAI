-- Migration 035: plainer descriptions of the seconds columns of the media views (comments only; no view, table or row changes).
--
-- Why. Measured 2026-11-12 on the sealed run of the free-form video questions (docs/data/video-questions-20261112.md): "List the plates that show up from 0:10 to 0:20" was answered from a query that
-- required both the first and the last second of a plate to lie inside the window, so a plate that was already in view at 0:10, or stayed on after 0:20, was missed and the answer said that nothing matches.
-- What a window holds is a matter of overlap, and the model reads its documentation from these comments (the catalogue is built from the database's own comments), so the comments say it.
--
-- Scope. 11 COMMENT ON COLUMN statements on views of migration 023 (the seconds columns, the source column and the needs_review columns). The pinned catalogue (api/forensic_records/ask_engine_catalogue.golden.json) carries the same texts and is regenerated with
-- tools/case_generator/check_ask_catalogue.py --write-golden. Rollback restores the earlier texts (ops/035_media_view_descriptions/rollback.sql). Idempotent.
-- Also measured on the same run: a clock time such as 0:10 was read as hours and minutes (the run now reads it into seconds by code and says so in the question's prompt), a count per video grouped by
-- the kind of evidence and not by the file, and a filter needs_review = false (the column is true for every machine reading) that matched no row.
BEGIN;
SET LOCAL lock_timeout = '10s';
COMMENT ON COLUMN analyst.v_plate_reads.first_seen_seconds IS
  'For a video, the second the plate was first seen; empty for a photo. Seconds count from the start of the video: a time written 1:30 in a question is 90 seconds (minutes and seconds, not hours and minutes). To ask which plates were in view during a window from second A to second B, compare the stay with the window: first_seen_seconds <= B AND last_seen_seconds >= A. Requiring both ends inside the window would miss a plate that was already in view when the window began or stayed on after it ended.';
COMMENT ON COLUMN analyst.v_plate_reads.last_seen_seconds IS
  'For a video, the second the plate was last seen; empty for a photo. How long a plate stayed in view is last_seen_seconds minus first_seen_seconds. For a window, see first_seen_seconds.';
COMMENT ON COLUMN analyst.v_plate_reads.source IS
  'image for one sighting in a photo, video for a group of the sightings of the same plate in one video. It says which kind of evidence the row is, not which file: to count or compare per video, group by original_filename.';
COMMENT ON COLUMN analyst.v_plate_reads.needs_review IS
  'True for every row: a model candidate that a person must check, never an identification of a vehicle owner. Do not filter on it: needs_review = false matches no row.';
COMMENT ON COLUMN analyst.v_video_plate_frames.seen_at_seconds IS
  'The second of the video the frame was read at, counted from the start (a time written 1:30 in a question is 90 seconds). The reads of a window from second A to second B are those with seen_at_seconds BETWEEN A AND B.';
COMMENT ON COLUMN analyst.v_video_plate_frames.needs_review IS
  'True for every row: a model candidate that a person must check. Do not filter on it: needs_review = false matches no row.';
COMMENT ON COLUMN analyst.v_ocr_regions.needs_review IS
  'True for every row: a person must check the reading before it is relied on. Do not filter on it: needs_review = false matches no row.';
COMMENT ON COLUMN analyst.v_ocr_evidence_summary.needs_review IS
  'True for every row: a person must check the reading before it is relied on. Do not filter on it: needs_review = false matches no row.';
COMMENT ON COLUMN analyst.v_speech_segments.needs_review IS
  'True for every row: a person must listen and check before the text is relied on. Do not filter on it: needs_review = false matches no row.';
COMMENT ON COLUMN analyst.v_speech_segments.start_seconds IS
  'Start of the segment in seconds from the start of the recording (a time written 1:30 in a question is 90 seconds, minutes and seconds).';
COMMENT ON COLUMN analyst.v_speech_segments.end_seconds IS
  'End of the segment in seconds from the start of the recording (a time written 1:30 in a question is 90 seconds, minutes and seconds).';
COMMIT;
