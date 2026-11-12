# Free-form questions about video, speech and text in pictures (2026-11-12)

Owner's request: an analyst asks, at run time and in their own words, any question about the videos of a case, and the answer comes from the stored readings, worked out by the model's query and checked, with no
template and no keyword rule written for the question. No GPU is available: the video is read once, at upload, on the CPU (measured: 8 to 12 times the clip length for HD, about 20 times for 4K); a question
afterwards is a database query plus one short model call.

## What was wrong (measured 2026-11-12 on the live engine, five real clips)

Ten varied questions on the real-clip collection: none answered. The plate reads of a video are in the records table with no position in the video, the plate groups and the transcripts are not in it at all,
and the engine's analyst views did not include the media readings (migration 023 existed as a draft and was never applied). The engine declined, or took a file name for a search term and found nothing.

## What is built

1. **Migration 023 reworked** (`db/forensic_records/023_media_views.sql`, never applied before, so changed in place): six views over `forensic.derived_artifacts` and `forensic.evidence_items`, scoped by tenant and
   collection like every analyst view, read-only role, every row marked as a model reading that needs review:
   - `v_plate_reads`: a plate per photo, a plate GROUP per video (the frames in which the same plate was read, first and last second seen, number of frames). The frame reads of a video are not in it, so a row count
     is not multiplied by frames.
   - `v_video_plate_frames`: every plate read on a frame of a video, with the second, the frame number, the box and the confidences.
   - `v_video_files`: length, picture size and audio track of each video, from the container header kept at upload (`evidence_items.media_metadata`).
   - `v_speech_segments`: the timed transcript of audio and video.
   - `v_ocr_regions` (now with `source` and `seen_at_seconds`) and `v_ocr_evidence_summary`: text read from photos and from video frames.
2. **Engine wiring** (`ask_engine_media.go` and small edits in retrieve, caveats, coverage, run, trace, wording): the words that point at each view, a caveat for each (a reading is a candidate, never an identification),
   a coverage probe (does the case hold plate readings, speech, text, video files), the views declared untraceable with their reason, absence and premise sentences, and 14 verified examples.
3. **Nothing changes for any other question.** A case that holds no media reading never sees the media views (they are not candidates of the shortlist at all, so the scores of every other view are exactly what they were).
   A case that holds typed records as well (calls, camera plates, accounts, documents) is offered the media views only for a question that speaks of video, photos, frames, recordings, speech or text on screen.
   A case that holds media readings and no typed record is offered only the views it can speak for, so a call view brought in by a general word ("when", "long") cannot make the coverage guard decline.
   `Shortlist` keeps its old meaning (media views left out); the engine calls `ShortlistWith`.
4. **Checks done**: the views were run on the live data inside a transaction that was rolled back (plate groups 27, 1, 9, 5 and 3 per clip; frame reads 974, 49, 207, 416 and 77; durations 60.00, 19.02, 15.02, 36.97
   and 21.52 s), with the migration's own preflight and verify scripts; the throwaway-database catalogue check (4 specs, 14 new examples run as the read-only role) passes and rewrote the golden catalogue (40 views);
   the package has 1,639 passing specs including 9 new ones (shortlists in a media-only case and in a mixed case, the labelled retrieval set with the media views present, coverage, pruning, caveats, examples).

## Known limits (stated, not hidden)

- Behind a figure the answer lists no records of its own for these views: each row is itself the reading, with its file and (for a video) its second. Opening the video at the moment from an engine answer is not built;
  the plate search by typed plate keeps its openable moments.
- The readings are the models' (plates, Whisper, OCR): a plate with a letter that looks like a digit is often read as the digit; Urdu, Punjabi and Pashto speech is less reliable than English.
- A video with no readable container header has no length. A second copy of the same bytes in another collection has no `video-observation` row (a worker identity quirk, recorded 2026-11-11), which the views do not need.

## Measurement (fixed before it is run)

Bank: `tools/video_plates/video_question_bank.py`. The key is computed from `derived_artifacts` and `evidence_items` directly, not through the analyst views, on the five real clips in the private collection
`video-plates-real-clips-20261111` (the key holds plate strings and stays out of the repository). DEV set (36 questions, written first, used to tune); SEALED set (36 questions, different wording, run once after the
tuning stops). Questions: how many plates in a video, the first second, plates in a time window, the longest in view, frames with a plate, length; across videos: the most plates, plates in more than one video, total
minutes, videos with audio, the longest video, distinct plates overall; which videos have speech; which has the most text.

Gates on the SEALED set (written before any run): correct at least 85 percent of the questions; wrong answers (an answer that is not the key) at most 5 percent; declined or no plan at most 15 percent; the
median answer time at most 60 seconds on this machine. A wrong answer weighs more than a decline. The sealed result is reported whatever it is.

## Live steps (each needs the owner's word)

1. Apply migration 023 to the live database: `ops/023_media_views/preflight.sql`, the migration, `verify.sql`; rollback `ops/023_media_views/rollback.sql` drops the six views and changes nothing else.
2. Build and deploy the API image (the catalogue snapshot includes the views, so the engine must be deployed after the views exist; deploying it before makes every question that names them fail validation).
3. Restart the workspaces.
