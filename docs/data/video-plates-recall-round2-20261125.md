# Video plates, round 2: why plates are missed and how to read more of them (plan and diagnosis protocol, 2026-11-25)

**Status 2026-11-25: plan and diagnosis written first; then cut down to a quick fix at the owner's word (Amendment 1 and section 8 below hold what was done and measured).** Programme step MF-7 of `docs/roadmap/master-roadmap-20261124/03_media_first_programme.md`, brought forward by the owner's word of 2026-11-25: "the current video ANPR misses many plates in videos as well, so fix and improve it accordingly". It continues `docs/data/video-plates-research-and-preregistration-20261111.md` (V4, live since 2026-11-11). Rules of every media programme apply: public or synthetic data and the owner's own clips, plate text of real footage never in the repository or in a message, one heavy job at a time in a memory-capped container without network, gates written before the sealed run, nothing deployed or enabled without the owner's word.

## 1. What we know (read from the code and the stored readings, 2026-11-25; not yet measured)

**The stored readings** of the real clips (counts only; `scratchpad/video_reads_profile.sql`, read-only). A "string" is one distinct normalised reading; a group needs the same string read on at least 3 frames:

| Clip | Reads (frames x boxes) | Distinct strings | Read 3+ times | Read twice | Read once | Plate groups kept |
|---|---|---|---|---|---|---|
| Owner's 4K traffic clip, 60 s | 1085 | 376 | 51 | 33 | 292 | 29 |
| Highway from an overpass, HD, 15 s | 207 | 55 | 12 | 2 | 41 | 9 |
| Street at night, HD, 37 s | 416 | 279 | 14 | 11 | 254 | 5 |
| Highway, 4K, 21.5 s (distant plates) | 77 | 50 | 5 | 2 | 43 | 3 |
| Street junction, HD, 19 s | 49 | 23 | 2 | 1 | 20 | 1 |

Most distinct strings are read once. Some are glare or non-plates; some are real plates whose readings differ from frame to frame (a one-character slip on each frame gives a different string each time, so no string reaches three). **How many of the 292 singletons of the 4K clip are real plates is not known. Nobody has counted the plates a person can read in these clips**: the benchmark of 2026-11-11 counted the plates the processor found and checked each found plate by eye. So the recall of visible plates has never been measured.

**Limits that apply before a single frame is read** (code, with the live compose settings):

| # | Limit | Where | Effect |
|---|---|---|---|
| L1 | A video longer than `MAX_DURATION_SECONDS` = 900 s (15 min) raises "source metadata violates configured bounds" | `video_anpr_nexus_v4.py` run_video; compose `...video.yaml` | a CCTV export of 30 or 60 minutes gets no plate reading from V4 (what the pipeline does with the error is to be checked) |
| L2 | Base frames are scheduled at 4 per second and the schedule **stops** at `maximum_analyzed_frames` = 1200 | `video_anpr_parity_adapter.schedule_base_frames` (`break`) | only the first 300 s of a longer video are looked at; the rest is not read |
| L3 | Wall-clock budget 1800 s for the whole job | compose `MAX_WALL_SECONDS` | at the measured 8 to 12 times clip length (HD) the job reads about 150 to 225 s of video; at 20 times (4K) about 90 s; then it stops and says so in the coverage receipt |
| L4 | The plate detector (RF-DETR) squashes the whole frame to a square of 576 px | `plate_detector_onnx.OnnxPlateDetector._prepare` | a plate 80 px wide in a 1080p frame is about 24 px wide to the detector; distant plates vanish |
| L5 | Native-resolution tiles only on frames of 2200 px and more, and only on every second call of the primary engine | `HighResolutionPlateEngine`, `FORENSIC_VIDEO_ANPR_V4_TILE_MIN_PX`, `TILE_EVERY` | 1080p and 720p footage never gets tiles; on 4K a clip of distant plates gave 3 of 7 with tiles every second call and 7 of 7 with tiles every call (at 2.3 times the time) |
| L6 | The primary reader sees every second base frame (2 per second) | `engine_every=(2, 1)` | a vehicle that crosses the plate-legible zone in about one second gives the primary reader 2 frames |
| L7 | A plate becomes a group only when one string is read on at least 3 frames with enough weight and a best reader confidence of at least 0.5 | `DEFAULT_POLICY` | a plate read on 2 frames, or read differently on each, is dropped as a group (the frame reads stay stored but are not searched) |

L1 to L3 decide how much of a long video is read at all; L4 to L7 decide how many of the plates in the read part are found. They need different fixes and different tests.

## 2. Hypotheses to test (each has a measurement in section 3)

* **H1** Most plates a person can read in these clips are never detected on any frame (L4, L5).
* **H2** Plates are detected but read wrongly on most frames (small, blurred crops), so no string reaches 3 reads (L7).
* **H3** Plates are read correctly on 1 or 2 frames only (fast vehicles, L6) and dropped (L7).
* **H4** On clips longer than 5 minutes most of the video is never read (L1 to L3).
* **H5** The two engines add recall mainly for small plates; a cascade (a fast detector on tiles, the accurate reader only on candidates) would cost less than tiles with RF-DETR.

## 3. Protocol A: diagnosis (exploratory, no gates)

**A1. Census of what a person can read.** Per real clip (the five above), the **oracle pass**: sample 4 frames per second over the first 20 s (all of the clip for the HD clips); run the fast plate detector (YOLO v9-t 384, licence-clean, already in the worker) on overlapping native tiles at two scales, the accurate detector (RF-DETR) on the whole frame and on candidate crops, both readers on every kept box; link boxes over consecutive frames into candidates; write one contact sheet per clip of the best crop of each candidate. **I look at every sheet** and label each candidate: not a plate / plate not readable by eye / plate readable (and what it says, kept in a private file outside the repository, never printed). The result is a **census**: the number of plates a person can read, and the frames on which each is readable. The census is a lower bound (a plate that neither detector sees and I cannot see is not counted) and it is by one reader (me); this is stated wherever the figures are used.

**A2. Where each census plate is lost.** Against the stored production reads of the same clips (frame, box, text; private query), each census plate is put in one class: not detected on any frame by production; detected but never read correctly; read correctly on 1 to 2 frames; read correctly on 3 or more frames but not kept as a group; kept as a group with the right text. The classes say which of H1 to H3 is true and how large each is.

**A3. Coverage of long videos (H4).** A synthetic 20-minute and a 60-minute video (known plates at known times, built from the existing clip generator by joining clips) through the live configuration on a throwaway copy; what is read, what the receipt says, what the pipeline does at 900 s. No heavy repeat: one run each, memory-capped.

**A4. A hard synthetic set with exact truth.** The existing DEV-V and SEALED-V sets use plates of 160 to 260 px and cannot separate methods. New sets, built like them and with the same partition rule, where the plates are 24 to 120 px wide, with motion blur, night and glare, compression, 720p, 1080p and 4K, several vehicles per frame, vehicles that cross in one second, and South Asian plate formats; exact truth per plate (string, times). Used for tuning (dev) and for the gates (sealed, once).

## 4. Candidates (to be measured on dev only; gates fixed as an amendment before any sealed run)

* **C1 Tiles on all frames of 1280 px and more**, at the native scale of plates, with a cheaper schedule than "every call".
* **C2 Cascade:** the fast detector on tiles every base frame (about 30 ms per tile), the accurate detector and readers only on candidate crops widened around each box; this is where the CPU time goes today.
* **C3 Motion-gated regions:** tiles only where something moves (the motion index of MF-1) plus a full sweep every few seconds for stopped vehicles; also skips empty minutes of a long recording.
* **C4 Adaptive cadence:** more frames per second while plates are being read, fewer in empty stretches; both engines on every frame read.
* **C5 Low-support readings kept and shown as leads:** a plate read on 1 or 2 frames, or a set of one-edit variants at one place, is stored as a "lead" group with its tier (reading contract of MF-0), found by a typed search and labelled "seen on 1 frame: check"; it is never promoted to a confirmed plate. No character substitution, no identity, as before.
* **C6 Long videos:** no hard rejection at 15 minutes; the job works in chunks inside its lease, resumes, skips empty time by the motion index, and always says in words how much of the video was read and what was not.
* **C7 Crop preparation for the reader:** the best of a few frames' crops (sharpness and size) upscaled before reading; evaluated per crop on the census plates.

Research marks, as for every media step: primary sources for each of these (plate detectors for small objects, SAHI-style sliced inference, ByteTrack-style association of boxes, multi-frame plate reading, licence of every model) before any model is chosen; a licence that is not clear is a ledger row, not a download.

### 4a. Research marks so far (2026-11-25; from search results, the papers were not read in full and their numbers are theirs, not ours)

* **Sliced inference (SAHI)**, Akyon et al., [arXiv 2202.06934](https://arxiv.org/abs/2202.06934): overlapping slices, each run through the detector and merged, no retraining; the price is many more forward passes, which is why C1 needs a gate (C3) and why a cheap detector on slices with the accurate one only on candidates (C2) is the shape to test. The cascade for plates specifically was not found in a source; it is our design to measure.
* **Multi-frame plate reading**, MF-LPR^2, [arXiv 2508.14797](https://arxiv.org/abs/2508.14797): aligns neighbouring frames with optical flow and refines the flow; the authors report 86.44% on their own real-world set against 14.04% for the best single-frame reader. **UFPR-SR-Plates**, [arXiv 2505.06393](https://arxiv.org/abs/2505.06393): 10,000 tracks of low-resolution plate frames with high-resolution pairs; fusing five frames raised recognition from 29.9% to 42.3%. Both say that blurred, small plates are recovered by combining frames, not by a better single-frame reader.
* **Governance question this raises (the owner's to decide, not assumed):** a fused or super-resolved crop contains pixels that no frame contained. A reading taken from one would break the rule kept since V2 ("the selected string is one actually read on a frame"). If used at all it would be stored as a separate, explicitly labelled lead (tier LEAD in the reading contract of MF-0, the enhanced crop kept next to the original frames), never as a confirmed reading. C7 is therefore limited to choosing the best of several real crops until the owner decides.
* Licences of any model taken from these papers are not read yet; a licence that is unclear becomes a ledger row, not a download.

## 5. What this cannot show (stated in advance)

The census is the plates a person can read in five clips by one reader and is a lower bound; synthetic plates are easier than real ones in ways the generator does not draw (dirt, damage, lighting on the metal); the clips are daytime motorway and one night street; faces are not touched; real CCTV from the owner's own sites is not available here; figures are on this laptop's CPU. Speed claims are for 4 threads in a memory-capped container.

## 6. Order of work

1. A1 and A2 on the owner's 4K clip first (the likeliest source of the complaint), then the HD clips. 2. A3 (cheap). 3. A4 sets. 4. Amendment: gates and the chosen candidate list, written before any candidate runs on sealed data. 5. Dev tuning (at most three rounds), sealed once. 6. Overlay image behind a switch (`FORENSIC_VIDEO_ANPR_V5`, default off), staging proof, the owner's word to deploy.

## 7. Amendments

### Amendment 1 (2026-11-25): the owner asked for a quick fix, so the programme was cut down

The owner's word the same day: fix and improve it, do not spend long on it or on testing, then go back to the main plan. So sections 3 and 4 were **not** run as a pre-registered programme: no hard synthetic set (A4), no long-video run (A3), no gates, no sealed run. What was done is the census (A1, A2) on three of the five real clips, a fix of the causes that the code reading and the census made certain (C1 partly, C4 as a new whole-video schedule, C5 as a sparse-sampling rule, C6 partly), and one confirming run per clip. The figures below are development evidence on real clips that are not independent of the diagnosis; they are not a gate result and must not be quoted as one. C2 (cascade), C3 (motion-gated regions, which the video overview will supply), C7 (crop preparation) and multi-frame fusion were not done.

## 8. Results (2026-11-25; counts only, no plate text of real footage; the census files with the plate text are private and outside the repository)

**Census, where production loses plates** (a plate "a person can read" is one the person labelling the contact sheets could read on its best crop; the census covers 20 s of the owner's 4K clip, all of the HD overpass and the junction clip; the night clip was run but not labelled, and the 4K highway clip and the rest of the owner's clip were not run):

| Clip | Readable plates | Found by production (stored reads of the live run) | Where the rest were lost |
|---|---|---|---|
| Owner's 4K traffic clip, first 20 s | 16 (seen as 36 candidates; 33 more were plates not readable, many of them white bars with no text at all: the clip looks anonymised there) | 14 (13 exactly right, 1 one character off) | 1 plate hidden by a flashing blue light (boxed, read wrongly); 1 blurred plate seen on 2 frames at the end (never detected) |
| HD highway from an overpass | 11 | 9, all exact | both losses are plates under 80 px wide near the start of the clip: one read correctly on only 1 to 2 frames (no group), one never detected |
| Street junction, HD | 1 (16 more were plates too small or blurred to read, 24 to 60 px wide) | 1 | none |

Reading: **for plates of 80 px or more the stored production reading found every one; losses are small plates (under 80 px), plates visible for under a second, and obstructed plates.** The stored reads of the 4K clip show that most distinct strings are single reads (292 of 376), which is glare and non-plates and unreadable small plates; none of the 292 that were labelled turned out to be a readable plate the production run had missed (the 16 census plates account for the 51 strings read three or more times, the 33 read twice and a few of the singles). The recall of readable near plates is therefore high, and the larger loss for an analyst is **coverage of long videos**, below.

**Limits read from the code (L1 to L3, now changed) and what the fix does:**

| Limit | Before | Now (video_anpr_nexus_v4.py 4.1.0) |
|---|---|---|
| L1 length | a video over 900 s was refused (no plate reading) | limit 14,400 s (the compose default changes from 900) |
| L2 frames | the schedule stopped at 1,200 frames, i.e. after the first 300 s of a long video | `progressive_schedule`: the whole video is read coarse to fine (every 8th base frame first, then the frames between), up to 6,000 frames; the coarse pass always fits, so a cut thins a finer level, never the coverage |
| L3 time | the 1800 s budget was spent on the first minutes at full density | the same budget is spent on the whole video, sparse first; the result states how much was read |
| L5 tiles | tiles only on frames of 2200 px and more | tiles also on 1080p frames (two native-resolution tiles beside the whole frame); the 4K cadence is unchanged |
| L7 support | a plate needed 3 reads of one string to become a group | when the frames actually read give under 2 per second of video, 2 reads are enough and the grouping gap widens to 2.5 sampling intervals; the group's payload and the result's limitations say the reading is sparse |

The result's `result_states` still say COMPLETE_RESULTS; the coverage is stated in the stored metadata (`coverage_note`, `effective_frames_per_second`, `support_required`) and in the limitations. **The API and the interface do not show coverage anywhere**, so an analyst does not see "read 3 of 10 minutes" yet; that is a follow-up (a line in the video viewer and in answers about a video).

**Confirming runs with the new settings** (the `v4p` configuration of `from_environment()` with the new defaults, the 1800 s budget, one container at a time; scored against the census):

| Clip | Readable plates found, before | now | Time over clip length, before | now |
|---|---|---|---|---|
| Owner's 4K clip, first 20 s | 14 of 16 (13 exact) | 14 of 16 (13 exact) | 19.8x | 21.3x |
| HD overpass | 9 of 11 | 9 of 11 (the small 58 px plate that was missed is found; a plate seen for about half a second, found by chance before, is missed) | 11.7x | 12.2x |
| Junction | 1 of 1 | 1 of 1 | 7.8x | 9.1x |

**Per-frame recall on these clips did not change.** What changed is how much of a long video is read, the 15-minute refusal, and the rule for sparse sampling. These were checked by 16 unit tests (the schedule spans a ten-minute video within 100 frames where the old one stopped at 25 s; the 2-read rule applies only when sparse; HD tiles; the length limit), not by an end-to-end run of a long video, which was not done.

**Deployment (nothing is deployed):** overlay image `nexusai-forensic-records-worker:video-v41-20261125` (built on the running `latest`, replacing only `video_anpr_nexus_v4.py`; imports without network). `tools/dev/live-window/12-recreate-worker-video-v41.ps1` (dry run passed: with the length limit forced back to 900 s the nine compose files reproduce the running worker's configuration exactly, so the limit is the only configuration change; `-Apply` and `-Rollback` need the owner's word). Rollback tag created at `-Apply`: `before-video-v41-20261125`.

