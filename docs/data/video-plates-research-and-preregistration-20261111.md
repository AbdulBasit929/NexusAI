# Plates in video: research, decision and pre-registration (2026-11-11)

Owner, 2026-11-11: "proceed with video plates ... the best implementation, productivity, usage and functionality; research what is available online (repositories, models, data) and implement the best, relevant and appropriate". This file records what was found, what was decided and the gates that are fixed **before any video number exists**. A miss is reported as a miss.

## 1. What exists in the repository today

| Piece | State |
|---|---|
| `video_anpr_parity_adapter.py` | Governed building blocks: frame scheduler (coarse cadence plus refinement windows), spatiotemporal grouping of reads (IoU or centre distance, 1 s gap, edit distance at most 1), selection of a string that was **actually read on a frame**, coverage receipts, contracts `forensics.anpr-observation/v1`, `forensics.video-anpr-plate-group/v1`, `forensics.video-anpr-coverage-receipt/v1`. Rule kept since the V2 acceptance: no persistent vehicle identity, no journey, no interpolated observation, no character substitution. |
| `video_anpr_product_v2*.py` | YOLO vehicle + plate weights (`yolov8n.pt`, `license_plate_detector.pt`: Ultralytics weights, AGPL-3.0 terms) and an older reader; the weights are **not staged** here and the switch is off. |
| `video_anpr_onnx_v3.py` | Licence-clean plate-only processor: fixed 4 frames per second, no vehicle model, no tracking, reads each frame through an "engine" (`engine.predict(frame)`). Switch `FORENSIC_VIDEO_ANPR_V3_ENABLED` is **off** in the live worker. Its provenance hashes are constants for its own models, so it cannot simply be given another engine. |
| The live worker today | With `FORENSIC_ANPR_ENABLED=true` and the Nexus plate engine, a video falls to the generic path: one frame per second, at most 60 frames, each read as an image, then `_video_anpr_plate_groups`. |
| Go side | `video_anpr.go` answers a grouped timeline for **one video UUID**. There is no question "which videos show this plate" across the case (the photo search, `plate_image_search.go`, reads image readings only). |

## 2. Research (2026-11-11; sources are links, claims below are what those pages say)

* **Current practice is tracking-by-detection plus per-track fusion.** A 2026 surveillance paper links plate reads by IoU overlap and settles the text by confidence-weighted majority vote once a track has at least three reads ([arXiv 2607.03131](https://arxiv.org/pdf/2607.03131)). The ICPR 2026 low-resolution plate competition reports that strong entries exploited the five-frame track by voting, logit aggregation, ensembling or a temporal transformer ([competition page](https://icpr26lrlpr.github.io), [summary paper arXiv 2604.22506](https://arxiv.org/abs/2604.22506)). A super-resolution benchmark votes per character position over consecutive frames ([arXiv 2505.06393](https://arxiv.org/pdf/2505.06393)). Sharpest-frame selection appears in a 2026 challenge entry ([arXiv 2607.08896](https://arxiv.org/pdf/2607.08896)) and in patent filings. None of the sources I found reports a before-and-after gain of voting over single-frame reading on a shared benchmark, so the gain is **measured here, not assumed**.
* **Heavier methods exist and are parked.** Optical-flow alignment and restoration (MF-LPR², [review](https://www.themoonlight.io/review/mf-lpr2-multi-frame-license-plate-image-restoration-and-recognition-using-optical-flow)), HAT or RVRT super-resolution and PARSeq plus CLIP4STR ensembles need a GPU or are several times slower; this laptop has none. They are an accuracy step for later, after the video path is functional.
* **A counterpoint:** one plate per vehicle from a single well-chosen frame can be efficient ([visual-rhythm paper review](https://www.themoonlight.io/en/review/efficient-license-plate-recognition-in-videos-using-visual-rhythm-and-accumulative-line-analysis)). It supports choosing the best frames of a short interval instead of reading every frame.
* **Open-source pipelines.** [anpr-pipeline](https://pypi.org/project/anpr-pipeline/) (MIT): fast-alpr detector and reader, a simple IoU tracker, per-position confidence-weighted vote over the top reads of a track, a minimum number of frames before a result is confirmed. [fast-alpr](https://pypi.org/project/fast-alpr/) itself has no tracker. [Roboflow trackers](https://trackers.roboflow.com/latest/) (Apache-2.0: SORT, ByteTrack, OC-SORT). Ultralytics and BoxMOT trackers are AGPL and are not used.
* **Test data that can be used.** No video set with readable plates is both free of strings and downloadable without a request: UFPR-ALPR is academic-only and given by e-mail from a university account ([licence](https://www.github.com/raysonlaroca/ufpr-alpr-dataset)); LRLPR-26 needs a signed agreement and registration; RLPR (dash-cam plate sequences, [Mendeley](https://data.mendeley.com/datasets/4rs5wpvckz/2)) is CC BY-NC-ND 4.0 with pseudonymised Korean plates; SLICE traffic video is CC BY 4.0 but has no plate labels; the Bangla plate video set ([arXiv 2108.08339](https://arxiv.org/abs/2108.08339)) does not say where it is hosted; Pexels and Pixabay licences are not clear for model testing and the clips show real plates and people ([Pixabay FAQ](https://pixabay.com/service/faq/)). No Pakistani plate video set was found.

## 3. Decision

1. **Keep the governed skeleton** (scheduler, grouping, observed-string selection, coverage receipts, contracts, no tracking identity). It already encodes the product rules and the earlier acceptance. Tracks are not exposed: a "group" is a short interval in one video in which reads of the same plate agree.
2. **A new processor, `video_anpr_nexus_v4.py`**, leaves the frozen V2 and V3 files untouched and changes four things: (a) the frame engine is the measured M5 stack (RF-DETR plate detector with the non-plate rule plus the global reader), with its **own** pinned hashes in every observation; (b) the scheduler is V3's base cadence plus the existing refinement windows around frames that read a plate, so the sharpest, largest views of a passing plate are read more than once; (c) each read gets a quality weight from detector and reader confidence, crop size and **sharpness** (variance of the Laplacian of the crop), and a group selects the observed string with the highest weighted support; (d) the result carries the same contracts and the flags `persistent_tracking: false`, `interpolated_observations: false`.
3. **No invented strings.** Per-character voting that produces a plate nobody read on any frame is **not** used for the answer (the product rule). It is measured as an ablation and reported, not shipped.
4. **Search across the case** (after the processor is accepted): the plate search reads video groups too and answers with the video, the time interval and an openable citation that seeks the player.

## 4. Test data (built here, exact truth; no real person's plate)

Videos are made from real car photographs of the public Kaggle set that are **not** in the owner-labelled gold set. The plate in the photo is replaced by a **synthetic plate string** (`plates_synth`, same method as the Orion demonstration photos), so the truth is exact and no real plate is used. A virtual camera moves over the photo: the vehicle approaches (the plate grows from about 25 to 40 px wide to 5 times that), with seeded camera shake, motion blur, sensor noise, an occlusion in a quarter of the clips, an exposure ramp in a quarter, a quarter at 854x480, then H.264 at CRF 28 to 34.

| Set | Positive clips | Negative clips | Use |
|---|---|---|---|
| DEV-V | 24 | 6 | choose and tune; at most three design rounds, each recorded here |
| SEALED-V | 24 | 6 | run once on the frozen method; the photos and plates are different from DEV-V |

Negatives: three with the plate area painted flat (a blank plate) and three that never show the plate. Per clip the key holds the plate string, the plate box per frame and the interval in which the plate is at least 50 px wide. The key and the builder are committed; the videos are not (size). The owner's local `sample.mp4` (authorised, non-retained, no independent ground truth, privacy review not done) is **not a gate**: if it is run, only counts and times are reported and no frame is saved.

## 5. Methods compared on the same clips

| Id | What it is |
|---|---|
| B0 | Today's live path, reproduced: 1 frame per second, at most 60 frames, M5 engine per frame, the generic grouping of `media_pipeline.py` |
| B1 | V3 as built (licence-clean fast-alpr default engine, 4 frames per second) |
| B2 | V3's scheduler and policy with the M5 engine (isolates the engine) |
| V4 | The method of section 3 |
| ablations (reported, not gated) | V4 without refinement; V4 without the sharpness weight; per-character vote over a group (not shippable) |

## 6. Gates (fixed now)

| Gate | Threshold |
|---|---|
| VG1 clip read | at least 85% of positive clips have a group whose selected plate equals the truth exactly |
| VG2 spurious reads | at most 0.3 groups per positive clip with support 2 or more whose plate is more than one edit away from the truth; in the 6 negative clips, at most 1 clip with any group of support 2 or more |
| VG3 time sanity | for the exact reads, the group's first observation lies in [legible start minus 1 s, legible end plus 1 s] in at least 90% |
| VG4 cost | median wall time at most 6 times the clip duration, one worker on this CPU; peak memory below 1.5 GB |
| VG5 gain | on SEALED-V, V4's clip-read rate beats B0's by at least 10 points |

Procedure: B0, B1, B2 and V4 run on DEV-V; design changes to V4 only from DEV-V, at most three rounds, each appended below; the frozen V4 runs once on SEALED-V together with B0 to B2. If a gate is missed the live switch stays off and the result is reported as it is. Changes to the instruction of the question step or to the Ask engine are out of scope here.

## Amendments

### Amendment 1 (2026-11-11, before any DEV-V measurement; no gate changed)

First runs of the processor on a real 15 s fixed-camera clip (public stock footage of a highway seen from an overpass, 1080p, dense traffic; no ground truth, plates checked by eye; nothing here is a gate).

| Method on that clip | Groups (support at least 3) | Distinct plates | Frames analysed | Time |
|---|---|---|---|---|
| Today's live path (1 frame per second) | 15 strings, none with 3 reads | not confirmed | 15 | 1.0x real time |
| V4 with refinement windows around every read (as first designed) | 23 | not counted | 343 of 450 | 25x |
| V4 with a per-group refinement budget (now the default) | 8 | 7, all read correctly by eye (one plate split into two groups) | 97 | 12x |
| V4 per-group plus native-resolution tiles | 12 | 9 (two extra plates, both real, found only by the tiles; two plates split into two groups) | 97 | 25x |

Changes made because of this, all before the first DEV-V run: (1) the refinement windows of V3 analysed 76% of all frames in dense traffic; replaced by a per-group budget (at most 3 extra frames per cluster of reads, unconfirmed and larger plates first, extra frames at most half of the base frames); the windows stay available as a mode for comparison. (2) The per-frame hash is the SHA-256 of the decoded pixels, not of a PNG encoding (saves about 0.1 s per 1080p frame; recorded in each observation as `frame_hash_method`). (3) Groups of the same selected text whose intervals touch within 1.5 s are merged (a plate that moves far between two sampled frames was split into two groups). (4) `HighResolutionPlateEngine` (native tiles on large frames and a plate-shape filter, width over height at least 1.5) exists as an option and is measured on real clips; it is not part of the default V4 until DEV-V shows its effect. The per-frame cost grows with the number of plates in the frame (each plate is one read); VG4 stays as written and the measured factor is reported.

