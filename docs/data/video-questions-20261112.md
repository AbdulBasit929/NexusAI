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

## Dev run and sealed result (2026-11-12)

**Dev set (36 questions, tuning).** First run on the live engine: 30 correct, 6 declined, 0 wrong, median 29 s. All six declines were the text-search step claiming the question before the engine saw it (Amendment 7 of
`docs/roadmap/text-intent-preregistration-20261110.md`: a first attempt, a new prompt value, was measured and reverted because it cost 12 ordinary word searches; the final fix is three server checks). After the
deploy the six were re-asked: 5 correct.

**Sealed set (36 questions in other wording, keys from the stored readings, run once on the live engine after the round 2 deploy, image `video-round2-20261112`):**

| Gate (fixed before the run) | Result | |
|---|---|---|
| correct at least 85 percent | **25 of 36 = 69.4 percent** | NOT MET |
| wrong at most 5 percent | **5 of 36 = 13.9 percent** | NOT MET |
| declined or no plan at most 15 percent | **6 of 36 = 16.7 percent** | NOT MET |
| median answer time at most 60 s | 24 s (p90 125 s, slowest 132 s) | MET |

By kind: first plate 5 of 5, frames with a plate 5 of 5, length 5 of 5, longest plate in view 6 of 6, plates in more than one video 1 of 1, videos with audio 1 of 1, longest video 1 of 1, distinct plates overall 1 of 1,
speech 1 of 1; **distinct plates of one video 0 of 5, plates in a time window 0 of 3, video with the most plates 0 of 1, video with the most text 0 of 1, total minutes 0 of 1.** No answer was right by the wrong view.

**Causes (each read from the engine's own log, none specific to a clip):**
1. *Distinct plates of one video, 5 declined.* "How many distinct plates can be made out in X?": the engine's frame reads the word "made" as an outgoing call (a cue written for "calls made by") and requires a direction
   filter that a plate reading cannot have; the model gave up after two repairs (`OBLIGATION:DIRECTION`).
2. *Plates in a window, 3 wrong.* "from 0:10 to 0:20" was read as hours and minutes, 600 to 1200 seconds, and the query required the first second to lie inside that window; the answer said that nothing matches.
3. *Video with the most plates, 1 wrong.* The query grouped by `source` (photo or video) and not by file: "video, 45".
4. *Video with the most text, 1 wrong.* The query filtered on `needs_review = false`, a column that is always true for a machine reading, so no row matched.
5. *Total minutes, 1 declined.* The text-search step claimed it: the literal "running time of all the videos" held function words (of, the) beyond the closed set of request-describing words.

**Decision.** The sealed set is spent and is not re-run. The five causes are general and are fixed as general changes (a clock time in a question is read by code into seconds and handed to the model; the direction check
applies only to views that have a direction; the descriptions of the seconds, the source and the needs_review columns say what they are; function words are ignored by the request-words check; three examples instead of
two are shown for a media question), and proven on a fresh set, set 2 (55 questions, a third wording, new kinds of question: plates in view for more than 5 seconds, videos longer than 30 seconds, the average length,
speech segments and text regions per video), run once after the deploy and judged by the same gates. The result of set 2 is recorded here whatever it is.

## Set 2 result (2026-11-12; after round 3, image `video-round3-20261112`, migration 035 applied)

55 questions in a third wording and with new kinds of question, run once, judged by the same gates:

| Gate | Result | |
|---|---|---|
| correct at least 85 percent | **41 of 55 = 74.5 percent** | NOT MET |
| wrong at most 5 percent | **2 of 55 = 3.6 percent** (sealed run: 13.9) | MET |
| declined or no plan at most 15 percent | **11 of 55 = 20.0 percent** (sealed run: 16.7) | NOT MET |
| median answer time at most 60 s | 22 s (p90 50 s, slowest 79 s) | MET |

One answer was right by the wrong view (reported as lucky, not counted as correct). By kind: distinct plates of one video 5 of 5 (sealed run: 0 of 5), plates in a time window 3 of 3 (0 of 3), frames with a plate 5 of 5,
plates in view for more than 5 seconds 5 of 5, speech segments per video 5 of 5, text regions per video 5 of 5, longest plate in view 5 of 6, **first plate 0 of 5, length 0 of 5**, videos longer than 30 seconds 0 of 1,
frames in the whole case 0 of 1, longest video 0 of 1, speech which 0 of 1 (lucky).

**Causes (each read from the engine log; none specific to a clip):**
1. *First plate, 5 abstained.* "How many seconds into X does the first plate show up?": the count obligation of the frame fired on "how many" and wanted a COUNT; a MIN of the first second is the answer (`OBLIGATION:AGGREGATE` three times).
2. *Length, 5 (4 declined, 1 wrong).* "Tell me the running time of X in seconds": no view fitted (the file name is one token and says nothing of a video; "running time" is not a word of the view), and the one answer took the span of the plate readings for the length.
3. *Longest video, 1 declined.* The inventory view and the video view were both offered and the model asked one for a column of the other, twice.
4. *Videos longer than 30 seconds, 1 declined.* The text-search step claimed it ("longer than 30 seconds" held words and a number outside the closed set).
5. *Frames in the whole case, 1 wrong.* The query counted the reads (1,723) and not the frames that had a plate.
6. *Speech which, 1 lucky.* Answered from the audio-track flag of the video view, which says nothing of speech.

**Round 4 (the last fix round; general changes).** A question that names a media file by its name gets the words of its kind (video, recording, photo) for the shortlist; "how many" followed by a unit of time is a measure (a MIN, MAX,
AVG or SUM satisfies the count obligation); the vocabulary of the video view gains runtime and running; the inventory is not offered next to the video view; the text-search check passes over numbers and comparison words
(than, over, under, longer, shorter); migration 036 (descriptions only) says that the video view lists every video, that the audio flag is not speech and that a frame can hold several reads. The proof is a third fresh set,
set 3 (a fourth wording, new kinds of question), run once; after it the video questions are closed whatever it shows and the remaining misses are listed.

## Set 3 (stopped early, no gate verdict) and closure of the video questions (2026-11-12)

**Set 3** (51 questions, a fourth wording, new kinds of question; image `video-round4-20261112`, migrations 035 and 036 applied) was **stopped by decision after 13 questions**: 10 correct, 2 declined, 1 wrong, median 33 s.
The owner decided to close the video questions and move on; the remaining 38 questions would not have changed that, and the run held the CPU that the upload and processing of the real clips in the Orion case needed.
It is therefore reported as a partial run: it gives **no gate verdict** and is not counted as a pass or a miss. The 13 answers are recorded, including the three that were not right:
1. *Length, 2 abstained* ("How many seconds long is X?"): the count obligation of the frame fires on "how many" and the model's right answer is a plain read of a column, not an aggregate. Round 4 fixed "how many seconds into X" (a MIN) but not this form.
2. *Frames with a plate, 1 wrong* ("In X, how many frames contain a readable plate?"): the model added `read_confidence >= 0.5` for "readable" and answered 47 where the frames that had a plate number 29. A word of the question was turned into a filter nobody asked for.

**The three runs side by side (each run once, gates fixed before the first):**

| | correct (gate 85) | wrong (gate 5) | declined (gate 15) | median (gate 60 s) |
|---|---|---|---|---|
| sealed run, 36 questions, round 2 | 69.4 percent | 13.9 percent | 16.7 percent | 24 s |
| set 2, 55 questions, round 3 | 74.5 percent | **3.6 percent (met)** | 20.0 percent | 22 s (met) |
| set 3, 13 of 51 asked, round 4 | 10 of 13 | 1 of 13 | 2 of 13 | 33 s |

**Closure statement.** The gates for correct and for declined were not met on any of the runs that gave a verdict; the gate for wrong answers was met once (set 2) and the time gate every time. What can be said: after round 3 the engine
answers about three in four free-form questions about videos correctly, almost never gives a wrong answer, and declines or abstains on most of the rest; a decline is stated as one (it never invents). What is known to remain:
1. The incoming/outgoing and the "how many" checks of the frame were written for calls and transactions and still apply to the media views by word cues ("made", "how many seconds long"); each fresh wording finds one more. The fix of
   the class is to apply those checks only to the views they were built for, not to chase wordings (not done: the video questions are closed).
2. A word of the question can become a filter ("readable" became a confidence threshold).
3. Questions that name the video by a description ("the overpass clip") and not by its file name are not mapped to a file.
4. Behind an answer, the videos themselves are not yet opened at the cited second from an engine answer (built in the phase that follows: docs below).
5. The plate readings are the readers' own (O and 0, 8 and B): plates are matched exactly; the typed-plate search lists a similar reading separately.

**What is live (2026-11-12):** analyst views of migration 023 (reworked), 035 and 036 (descriptions), applied to the live database (backups of the analyst schema before 023, 035 and 036 in `C:/tmp/video-plates/`, never committed);
API image `video-round4-20261112` (rollback chain: `video-round3-20261112`, `video-round2-20261112`, `text-guards-20261112`, `media-views-20261112`, `plate-similar-20261112`); text-intent Amendment 7 (server guards, instruction unchanged).
Commits `e958399`, `88a61e4`, `32b2943`, `efdadae`, `06b3304` and this one.

**The real clips in the Orion Lahore case.** The five real clips (three HD, two 4K; the owner's own `sample.mp4` among them) were uploaded to the reference case `orion-lahore-reference-20261030` (one through the workspace's own upload on :4182,
four through the upload route), under the names `street-junction-hd`, `highway-overpass-hd`, `street-night-hd`, `highway-4k` and `traffic-control-cctv-4k`. Orion's file counts change (the verified bank's questions about the number
of files or of videos follow the database: their keys are SQL on the database, not numbers written down).

## Defect found on the Orion upload: the second copy of a file lost every reading (2026-11-12)

**Found** by putting the five real clips into the Orion reference case after they had been read in a throwaway collection (the same bytes): the first questions on the Orion case answered "0 plates" and the evidence step found no reading.
In the database the Orion copy of a clip held only its speech segments; its 49 plate records (table `forensic.records`) pointed at artifacts of the OTHER collection.

**Cause.** The id of a media reading is derived from the SHA-256 of the file and the reading's position (`media_pipeline.py`), so it is the same wherever the file is, and it is the primary key of `forensic.derived_artifacts`; the
worker stores with `ON CONFLICT (artifact_id) DO NOTHING`. The second copy of the same bytes in another case therefore stored nothing of what it read (plate reads, plate groups, video summary, frame text). The speech segments survived
because their id carries the evidence and the version. The earlier hint (2026-11-11: a second copy had no `video-observation` row) was this defect seen on one artifact type. Any analyst who puts one video into two cases is affected.

**Fix** (`ingestion/forensic_records/media_artifact_scope.py`, `tools/media_data/patch_worker_artifact_scope.py`): a reading keeps its content id unless ANOTHER evidence version already holds it, and then it is stored under an id of its own
(uuid5 of "<version_id>:<content id>"); the plate record gets the same resolved id in its link and in its payload. Everything stored before, every first copy and every replay of a version is unchanged. Ten Python tests (the rule, replays,
three copies of one file, rows as tuples and as dictionaries, the patch's refusals); the repository worker and the live image's own worker take the same seven edits (the image is an older build: an overlay image, `artifact-scope-20261112`,
built by `tools/media_data/Build-ArtifactScopeOverlay.ps1`, switched by `tools/dev/live-window/7-recreate-worker-artifact-scope.ps1`; rollback tag `before-artifact-scope-20261112`; no setting changed).

**What the fix does not do.** The clips that completed during the defect (junction, overpass, highway-4k in the Orion case) keep their missing readings: a re-upload is a duplicate by hash, and a reprocess is a governed operator step (a
new linked generation, prior outputs kept). The stale plate records of those clips (pointing at the other collection) remain until then.

**Operational note.** The worker was stopped while the fix was built so that the queued clips would run on the fixed code. The queue's consumer allows one message in flight (`max_ack_pending` 1, `ack_wait` 600 s) and a video job holds a
2400 s database lease: after a worker is stopped mid-job, its job blocks the queue until the lease ends (the worker answers "busy" and retries every 30 s). The lease of the one stopped job was released by hand; the 2400 s lease is the
long pole of any worker restart that interrupts a video.
