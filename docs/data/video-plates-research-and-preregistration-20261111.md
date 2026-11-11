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

### DEV-V results of the first complete run (2026-11-11; development, not gates)

| Method | Positive clips read exactly | Spurious groups per positive clip | Negative clips with a group | Median time over clip length |
|---|---|---|---|---|
| B0 today's live path | 19 / 24 (79.2%) | 0.04 | 0 / 6 | 0.79x |
| B1 V3 as built (licence-clean engine, 4 frames per second) | 21 / 24 (87.5%) | 0.13 | 0 / 6 | 0.94x |
| B2 V3's scheduler with the M5 engine | 19 / 24 (79.2%) | 0.21 | 0 / 6 | 3.4x |
| V4 as built (M5 engine, per-group refinement, sharpness weight, merge) | 19 / 24 (79.2%) | 0.17 | 0 / 6 | 4.4x (max 8.9x) |

V4 as built meets neither VG1 (85%) nor the VG5 margin over B0 (a gain of 10 points); VG2 negatives and VG3 time sanity (19 of 19) are fine, VG4 median is within 6x. Reasons for the five misses of V4 (from the per-clip results): two clips with strong motion blur where the V3 engine reads the plate and the M5 engine scatters its reads over several strings (dev-01, dev-11); two plates with a letter O where the reader prints a zero (dev-05, dev-10; a confusable pair, exact equality cannot hold); one clip so blurred that no engine reads it (dev-19). Note on the set: the plates in the DEV-V clips are large (between 160 and 260 px wide at 1280 px) because the car photographs have large plates, so these clips do not test small or distant plates; the real clips do.

### Amendment 2 (2026-11-11): tuning round 1 of 3 on DEV-V

Change: each sampled frame is read by two independent engines, the M5 engine and the licence-clean default engine (YOLO plate detector with the cct-xs reader, the engine of V3). The reads of both go into the same grouping and a group selects, as before, an observed string by weighted support, so two engines that agree reinforce each other and a read that only one engine makes needs more support. Every observation keeps its own engine's model hashes. Expected cost: about 25% more time (the second engine is fast). No threshold of the gates changes. Rounds left after this one: 2.


### Round 1 result and Amendment 3: tuning round 2 of 3 on DEV-V (2026-11-11)

Round 1 (two engines, M5 and the licence-clean default engine, in one grouping; the merge of groups one edit apart that overlap in time and space) on DEV-V: **20 / 24 clips read exactly (83.3%)** against 79.2% before; spurious groups per positive clip 0.29 (gate 0.3); negatives 0 of 6 clips with a group; time sanity 20 of 20; **median time 8.5x the clip length (gate 6x), maximum 10.7x**. Remaining misses: two plates with a letter O that every engine prints as a zero (dev-05, dev-10), one plate misread as another by both engines under strong blur (dev-11: the true string was read on 14 frames by one engine, the wrong string on 83), one clip too blurred for any engine (dev-19). Those four cannot be repaired by voting, so on this set 20 of 24 is close to what an exact-match gate allows; VG1 (85%, 21 clips) is missed by one clip and VG4 is missed.

Change of round 2: the slow engine (M5, about 0.6 s per frame) reads every second BASE frame (and every refinement frame), the fast engine (about 0.2 s) reads all of them. Expected: about a third less time at a small cost in reads. Added, not a gate: **search read**, the number of positive clips whose true plate was read on at least two frames by any engine. The plate search over the case reads these frame-level reads (not the groups), so this is what a typed plate finds. Round 2 is run with B0 and B1 again so that all three report the search measure. Rounds left after this one: 1.

### Round 2 result and Amendment 4: the method is frozen (2026-11-11)

Round 2 on DEV-V (M5 engine on every second base frame, default engine on every base frame, per-group refinement, sharpness weight, merge of identical or one-edit groups that overlap):

| Method | Clips read exactly (VG1) | Search read (not a gate) | Spurious groups per positive clip (VG2) | Negative clips with a group (VG2) | Time sanity (VG3) | Median / max time over clip length (VG4) |
|---|---|---|---|---|---|---|
| B0 today's live path | 19 / 24 (79.2%) | 19 | 0.04 | 0 / 6 | 19 / 19 | 0.78x / 1.3x |
| B1 V3 (default engine) | 21 / 24 (87.5%) | 21 | 0.13 | 0 / 6 | 21 / 21 | 1.0x / 1.1x |
| **V4 frozen** | **20 / 24 (83.3%)** | 21 | 0.29 | 0 / 6 | 20 / 20 | **4.6x / 7.7x**, peak memory 598 MiB |

Reading: VG2, VG3 and VG4 are met on DEV-V; VG1 (85%, 21 clips) is missed by one clip, and the remaining four misses (two letter-O plates printed as zeros, one plate misread by both engines under strong blur, one clip too blurred for any engine) are not repairable by voting. On these synthetic clips the licence-clean default engine alone (B1) reads as many plates as V4 at a quarter of the time; the V4 advantage is expected on real footage, where the M5 engine read 7 of 7 plates of a real motorway clip correctly. That is checked separately on real clips (counts and a look at the crops) and is not a gate. Two rounds of tuning were used (round 1 two engines, round 2 engine cadence); the third is not used.

**Frozen for the sealed run:** `video_anpr_nexus_v4.py` as committed with this amendment (defaults of `from_environment`: M5 primary engine every second base frame, default engine every base frame, base cadence 4 frames per second, per-group refinement of at most 3 extra frames per group and at most half of the base frames, merge gap 1.5 s, product policy of `DEFAULT_POLICY`). SEALED-V is run once with the methods `v4s` (this configuration), B0, B1 and B2; the numbers are reported whatever they are.

### SEALED-V result (run once on 2026-11-11, frozen method of Amendment 4; reported as it came out)

| Method on the 24 positive and 6 negative sealed clips | Clips read exactly (VG1) | Search read (not a gate) | Spurious groups per positive clip (VG2) | Negative clips with a group (VG2) | Time sanity (VG3) | Median / max time over clip length (VG4) |
|---|---|---|---|---|---|---|
| B0 today's live path | 19 / 24 (79.2%) | 18 | 0.29 | 1 / 6 | 19 / 19 | 0.77x / 1.0x |
| B1 V3 (default engine) | 16 / 24 (66.7%) | 19 | 0.33 | 0 / 6 | 16 / 16 | 0.82x / 1.5x |
| B2 V3 scheduler with the M5 engine | 19 / 24 (79.2%) | 20 | 0.29 | 0 / 6 | 19 / 19 | 3.3x / 3.8x |
| **V4 frozen** | **18 / 24 (75.0%)** | **21** | **0.46** | 0 / 6 | 18 / 18 | **4.6x / 6.1x**, peak memory 598 MiB |

Gates: **VG1 (at least 85%) NOT MET** (75.0%); **VG2 NOT MET** (0.46 spurious groups per clip against at most 0.3; the negatives part is met, 0 of 6); **VG3 met**; **VG4 met** (median 4.6x, memory 598 MiB); **VG5 NOT MET** (V4 is 4.2 points below B0, a gain of 10 points was required). Per the procedure of this plan the live switch for V4 stays off until the owner decides otherwise, with these numbers in front of them.

Reading, without any change of method (the sealed set is spent): (1) the six V4 misses are one leading character lost by a crop cut too tight (JXD573 read as XD573), four systematic confusions (T read as I, 0 as 8, W as 71, and the same under heavy blur, where the stray groups of VG2 come from: 8 of the 11 stray groups are in clips with 9 px of motion blur) and one plate read on 17 frames but split over three one-edit neighbours; (2) the search measure (a typed plate is found when it was read on at least two frames) is the best of the four for V4 (21 of 24) but the differences between the methods are inside the spread between the two sets: B1 read 87.5% of the DEV-V clips and 66.7% of the SEALED-V clips. With 24 positive clips per set, the synthetic sets cannot separate these methods; the sets use large, sharp plates (the plates of the DEV-V and SEALED-V clips are 160 to 260 px wide). (3) With the weighted confusion cost of the plate search (a commonly confused pair costs 1; similar readings within cost 2 are listed apart as "similar reading, check by eye"), the plate search would have found the true plate, exactly or as a similar reading, in 22 of 24 sealed clips for B0, B1 and V4 alike (DEV-V: 22, 23 and 23 of 24). What distinguishes the methods is real footage, which is where V4 read all 7 distinct plates of a real motorway clip correctly and B0 confirmed none; the next step is a real-footage benchmark (public stock clips of fixed cameras and the owner's `sample.mp4`), checked crop by crop, not a gate.

### Real-footage benchmark and Amendment 5: production configuration (2026-11-11, after the sealed run; not a gate)

Real clips (public stock footage of fixed cameras and the owner's own `sample.mp4`; no ground truth; every read of the groups was checked by eye against its crop). Plates confirmed on at least 3 frames, frozen V4 of Amendment 4 (`v4s`), with today's path (`b0`) and V3 (`b1`):

| Clip | b0 | b1 | v4s | v4s with native tiles (`v4t`, primary engine only) |
|---|---|---|---|---|
| Highway seen from an overpass, 1080p, 15 s | 0 | 1 | 9 (9 of 9 read exactly) | – |
| Street junction, 1080p, 19 s | 0 | 1 | 1 | – |
| Street at night, 1080p, 37 s | 1 | 5 | 1 | – |
| Highway, 4K, 21.5 s (distant plates) | 0 | 0 | 1 | 7 (7 of 7 read exactly) |
| Owner's `sample.mp4`, 4K, first 20 s | 4 | 12 | 15 (13 exact, 1 O read as 0, 1 plate cut by the frame edge) | 14 |

Time over clip length: b0 0.9 to 2.2x, b1 1.3 to 6.1x, v4s 4.3 to 12.9x (HD 4.3 to 6.5x, 4K 10 to 13x), v4t on 4K 50 to 58x. Tiles help where plates are distant (1 to 7 plates) and add nothing where they are near (14 against 15), at a cost of four to five times the time.

The night clip showed a flaw that the synthetic clips could not: 416 reads of which 279 were distinct strings (glare), 128 junk clusters filled the cap on active groups, and later real plates were dropped (with the cap lifted the same reads give 8 groups against 3, the old processor's 5). Changes, made after the sealed run and therefore **not covered by it**, checked on DEV-V (regression) and on the real clips only, which are not independent of the diagnosis: (1) `maximum_active_groups` 128 to 1024; (2) `minimum_weighted_support` 1.0 to 0.6 (the weights of V4 are a product of factors, so the floor of 1.0 dropped triples of moderately confident reads that V3's weights kept); (3) the production default of `from_environment` wraps the primary engine in `HighResolutionPlateEngine` (tiles of 1280 px on frames of 2200 px and more, on every second call of the primary engine, i.e. one second in two base frames at 4 frames per second; plate-shape filter, width over height at least 1.5, on all frames). The method of the sealed run is the method of Amendment 4; this configuration is a new one, and the pass/fail verdict of the sealed run does not carry over to it. Its evidence is the real-footage benchmark above and, as a regression check only, DEV-V.

### Production configuration results on the real clips (2026-11-11; not a gate; plates confirmed on at least 3 frames)

`VideoANPRNexusV4Processor.from_environment()` as built into the worker overlay image (two engines, per-group refinement, merge, group cap 1024, support floor 0.6, tiles on frames of 2200 px and more on every second call of the primary engine):

| Clip | Frozen V4 of the sealed run (`v4s`) | Production configuration (`v4p`) | Time over clip length (`v4p`) |
|---|---|---|---|
| Highway from an overpass, 1080p, 15 s | 9 | 9 | 11.6x (the machine was also building images and running tests) |
| Street junction, 1080p, 19 s | 1 | 1 | 7.4x |
| Street at night, 1080p, 37 s | **1** | **5** | 9.2x |
| Highway, 4K, 21.5 s (distant plates) | 1 (7 with tiles on every call, 58x) | 3 | 25x |
| Owner's `sample.mp4`, 4K, first 20 s | 15 | 15 | 19.8x |

The night clip is repaired by the larger group cap and the lower support floor (5 plates, the old processor's 5). Tiles on every second call of the primary engine (one second in two base frames at 4 frames per second) find 3 of the 7 distant plates of the 4K highway clip where tiles on every call found 7, at less than half the time (25x against 58x); near plates need no tiles (`sample.mp4` 15 against 14 with). The tile cadence is the setting `FORENSIC_VIDEO_ANPR_V4_TILE_EVERY` (default 2; 1 reads distant plates better at about twice the time): a trade-off between recall of distant plates and time, and the default favours time. A 60 s clip of 4K video takes about 20 to 25 minutes on this CPU; the wall-clock budget of the job (1800 s) stops a longer one with a stated partial result.

Integration (staging proof, offline, on the built overlay image): the worker's own pipeline processed a 10 s synthetic video with the switch on: 87 frame reads in the stored contract (`forensics.anpr-observation/v1`, with `timestamp_seconds`, `frame_number` and the bounding box), one plate group (`forensics.video-anpr-plate-group/v1`, 0.0 to 9.96 s, 86 sightings) and `result_states.video_anpr = COMPLETE_RESULTS`; with the switch off the pipeline behaves as before (frames read as images). The proof found one incompatibility (the image's older default-engine class takes its threshold from the environment), fixed before the image was built.

### Decision to go live (2026-11-11)

The owner said "Go live" after being told that the synthetic gates VG1, VG2 and VG5 were not met on the sealed set, that the strong evidence is on real clips (9 of 9, 7 of 7, 13 of 15 plates read exactly, checked crop by crop) and that the production configuration post-dates the sealed run. Applied the same day: API image `video-search-20261111` (video moments in plate answers), worker overlay with `FORENSIC_VIDEO_ANPR_V4_ENABLED=true`, second engine on, wall-clock budget 1800 s, job lease 2400 s. No gate was changed; the misses stay on the record.

Live check (throwaway collection `video-plates-live-test-20261111`, one 10 s synthetic clip with one known plate, uploaded through the normal upload route): job completed; 87 frame reads with `timestamp_seconds` 0 to 9.96; one `video_anpr_plate_group_observation` (RLX902, 86 sightings, 0.0 to 9.96 s); "Which videos show the plate RLX902?" named the clip and its moments (0:00 to 0:09) with three openable places; an absent plate (ABC123) was declined with the "the reader may have misread it" wording; a plate one character away (O for 0) listed the clip as a similar reading to check by eye; the question took 40 s cold (model load) and 7 to 9 s after. The workspace on :4182 opened the video at the cited moment (6.96 s) with the plate region. A real-footage check through the live worker (the owner's own clips) is still owed.

### Real clips through the live worker (2026-11-11, after going live; counts only, no plate text recorded)

The five real clips were uploaded through the normal upload route to a private throwaway collection (`video-plates-real-clips-20261111`, local only) and read by the live worker (overlay 4dcc14ec5901, one job at a time). Plate groups are the stored `video-anpr-plate-group/v1` rows, each with at least 3 sightings.

| Clip | Length | Frame reads stored | Plate groups | Benchmark of the same configuration (`v4p`) | Time over clip length |
|---|---|---|---|---|---|
| Street junction, 1080p | 19 s | 49 | 1 | 1 | 7.8x |
| Highway from an overpass, 1080p | 15 s | 207 | 9 | 9 | 11.7x |
| Street at night, 1080p | 37 s | 416 | 5 | 5 | 10.3x |
| Highway, 4K (Roboflow example) | 21.5 s | 77 | 3 | 3 | 20.8x |
| Owner's `sample.mp4`, 4K | 60 s | 974 | 27 | 15 in the first 20 s (no whole-clip figure) | 20.5x |

The four clips with a benchmark give the same counts as the tested code, so the live worker reproduces it. No job reached the wall-clock budget (1800 s), no error was logged, the worker's memory peaked near 1.9 GB (the machine had 1 to 2 GB free at the time). One question per clip, with that clip's best-supported plate typed, found the clip with its moments (for example 0:00 to 0:04 for `sample.mp4`, 0:25 to 0:35 for the night clip) and three openable places.

Two findings:
1. **Wording fault (fixed in the code, not yet deployed at the time of writing).** Every answer added "1 other file shows a similar reading" and named the same video: a few frames of a real clip misread the plate by one character, and those frame reads were listed as a similar reading of a second file. A file with an exact reading is now left out of the similar list (one new test); a video that has only the misreading is still listed.
2. **Same file in two collections.** `sample.mp4` is byte-identical to `video-v3.mp4` in the multimodal acceptance collection. The new copy has all its frame reads and plate groups but no `video-observation/v1` and no `audio-observation/v1` row: the single copy of each belongs to the older collection (consistent with a deterministic artifact identity, not confirmed in the code). The plate search is unaffected; the container details of the second copy are not stored.
