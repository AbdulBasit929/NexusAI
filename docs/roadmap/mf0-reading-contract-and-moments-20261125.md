# MF-0, part 2: the one reading contract and the moments view (design, 2026-11-25)

Programme: `docs/roadmap/master-roadmap-20261124/03_media_first_programme.md` section 2 ("Shared parts"). **Design only: nothing is built, migrated or enabled by this note.** It is grounded in what the code stores today (read 2026-11-25); each choice below says what it replaces or reuses.

## 1. What exists today (so the design reuses it)

| Family | How a reading says how far to trust it | Where |
|---|---|---|
| Speech | `reading_status` (`MACHINE_TRANSCRIPT`, `SEARCH_LEAD`, `LOW_RESOURCE_LANGUAGE`, `NOT_READ`), `needs_review` (default true), `model`, `detected_language` | `analyst.v_speech_segments`, `analyst.v_audio_files` (migration 037); the speech banner in the viewer; the Ask catalogue |
| OCR | a confidence number and flags | `analyst.v_ocr_regions` (023); the image viewer |
| Plates | a confidence and a "machine reading, check each plate by eye" note | `analyst.v_plate_reads`, `v_video_plate_frames` (023), `v_plate_sightings` (027); the viewers |
| Image search | labels: identical file, same image, same photograph, verified part, probable part, not verified, ranked | the image answers |
| Everything | `derived_artifacts` has `confidence`, `citation_locator`, `metadata`, `warnings`, `content_sha256`, `run_id`; the input hash is on `processing_runs.input_sha256` | migration 008 |

So each family has its own vocabulary and its own place on screen; none states the **measured error** next to the reading, and the model **version** and the **input hash** are not shown anywhere an analyst sees.

## 2. The contract

One shape for every machine output, stored where the output already is (no new table), shown the same way everywhere:

| Field | Meaning | Where it comes from |
|---|---|---|
| `family` | speech, ocr, plate, image_copy, image_part, image_words, video_clock, video_shot, video_motion, face (later) | the artifact type |
| `tier` | **how it may be used**: `READING` (a reading whose error is measured and stated), `LEAD` (to find, never to quote: today's `SEARCH_LEAD`), `UNMEASURED` (today's `LOW_RESOURCE_LANGUAGE` and any language or condition we have not measured), `NOT_READ` | speech maps one to one; the other families start as `READING` or `LEAD` by their registered gates |
| `method`, `version` | model or algorithm and its version, as run | `metadata.observation.model` and the pipeline version |
| `input_sha256` | the SHA-256 of the evidence version it was read from | `processing_runs.input_sha256` (always present) |
| `measured_error` | `{metric, value, set, n, measured_on, results_doc}`, for example word error 26.0% on the sealed Urdu set of 2026-11-13 | the **reading error register** (below) |
| `candidate` | always true: every machine reading is shown as "to be checked against the source"; a face, a voice or a plate is never an identification | constant |

**The reading error register** is one reviewed JSON file in the repository, `docs/data/reading-error-register.json`, with one entry per (family, method, condition) and a link to the results document the number comes from. It is not a database table: it changes only when a measurement is recorded, it is reviewed like code, and every number in it can be checked against its source document. The API reads it at start-up and attaches `measured_error` to the readings it returns (the SQL a model writes does not need to join to it). A condition we have not measured (a language, a camera, a resolution) returns `UNMEASURED` and the screen says so in words.

**Where it is shown.** (1) A small `ReadingBadge` component used by every media viewer (speech, OCR regions, plates, image parts, video) in the same place: tier, method and version, the measured error in one line, "to be checked against the source", and a link to the input file hash. (2) In every Ask answer that uses a media reading, the folded "What to keep in mind" lists the families used, their tiers and their measured error. (3) The export of a result (a clip, a transcript) carries the same block. Nothing is added to the first sentence of an answer.

**What it never says:** "identified", "authentic", "tampered", "verified person". The wording rule is a test: a search of every viewer string and answer template for those words must find none.

## 3. Moments

A **moment** is one time-coded event from any media or record: a spoken segment, a plate seen in a video frame, a plate sighting from a camera log, later a burned-in clock reading, a motion interval, an EXIF capture time. The view puts them in one list so the timeline and the cell records can be joined to them.

`analyst.v_moments` (a **view only**, no table, over the typed views that exist):

| Column | Meaning |
|---|---|
| `moment_id`, `evidence_id`, `family`, `kind` | what and from which file |
| `observed_utc` | wall-clock time when the source states one (plate sightings from a camera log, EXIF later); otherwise empty |
| `position_seconds`, `end_seconds` | the place inside the media (speech segments, video frames) |
| `time_basis` | `stated` (the source gives a time), `container` (file time), `burned_in_clock` (read from the picture), `position_only` |
| `label` | short text: the words said, the plate, the interval |
| `tier`, `needs_review` | from the contract above |
| `source_view`, `source_key` | which view and row it came from, so the viewer can open the exact place |

First sources (all exist): `v_speech_segments` (position), `v_video_plate_frames` (position), `v_plate_sightings` (stated UTC). Added by later steps: video motion intervals, shots and the **clock-offset table** (MF-1), which lets a `position_only` moment gain an estimated `observed_utc` with the offset and its error shown; photograph capture times (MF-2); calls and messages join through their own typed views. The Timeline page gets a media lane that reads this view; the viewers get "open at this moment".

**Honesty rules.** A moment whose wall-clock time is *estimated* (from a clock offset) is shown with the word "estimated" and the offset error; a moment with no usable time is shown by position only and is never placed on the wall-clock timeline.

## 4. Gates (proposed; to be pre-registered as a short file before the build, as in MF-1)

| Gate | Pass condition |
|---|---|
| RC-1 | every media family above shows the badge in its viewer: 7 of 7 families (interface test) |
| RC-2 | no stored reading lacks an input hash: 0 of the stored artifacts of those families (SQL check on a throwaway copy) |
| RC-3 | every number in the register equals the number in the results document it cites (a test that reads both) |
| RC-4 | the forbidden words (identified, authentic, tampered, verified person) occur in 0 viewer strings and answer templates |
| RC-5 | `v_moments` returns exactly the rows of its three source views for a fixture case (counts and keys), with no row from another case (role test) |
| RC-6 | an Ask answer that uses a media reading lists its families, tiers and measured error in "What to keep in mind" for a fixed set of 20 questions |

## 5. Build order and what it needs

1. The register file and the badge, speech first (the tier names already exist), then OCR and plates, then the image labels. Software only; no heavy job, no download, no database change.
2. `v_moments`: a view-only migration. **It needs a migration number**: the plan gives 043 to MF-1 "only if it needs tables", so the view can ride in 043 with the clock-offset table, or take its own number if MF-1 needs no tables. A rehearsal on a throwaway copy and the owner's word come before the live database, as for every migration.
3. The Timeline media lane and "open at this moment" in the viewers.

**Open questions for the programme owner:** the migration number for the moments view; whether `UNMEASURED` should be the word on screen or a plainer phrase such as "not measured for this language".
