# Image search beyond near-duplicates: measured results (2026-11-23)

Context: the owner's test showed that the first version proved near-duplicates and one phrase, not real use. This note records what was measured while building region search, window (tile) search, the duplicate groups, the words cut-off and the Ask routing. Plan and sources: `docs/roadmap/image-search-real-world-plan-20261123.md`. How to test by hand: `docs/qa/image-search-manual-test-guide-20261123.md`. All figures are from the live Lahore case (97 pictures) unless stated. Nothing here is a pre-registered gate: they are development measurements, and every threshold is a setting.

## 1. Region search (SIFT + RANSAC, `ingestion/forensic_records/image_region.py`)

`tools/media_data/image_region_eval.py` cuts regions out of 24 of the case's own pictures (seed 11) in nine ways and attaches each one (216 probes), against the live case.

**Settings chosen on measurements** (`tools/media_data/region_param_probe.py`, offline, the same 216 probes against the true picture and against 1,080 pairs of unrelated pictures):

| SIFT contrast / features (probe, candidate) | Found at 12+ agreeing points | Found at 8+ | Tiny details | Chance matches: largest, at 8+, at 12+ |
|---|---|---|---|---|
| 0.040 / 1000, 1500 (SIFT default) | 197 | 200 | 12 of 24 | 6, 0, 0 |
| 0.025 / 1000, 1500 (first version) | 200 | 203 | 13 | 6, 0, 0 |
| 0.012 / 2000, 2500 | 204 | 206 | 16 | 6, 0, 0 |
| **0.006 / 3000, 3000 (adopted)** | **208 (96%)** | **211** | **18** | **6, 0, 0** |

**Live result at the adopted settings:**

| Variant | Came back as "contains it" | Verified | Mean overlap of the box |
|---|---|---|---|
| small detail (5-12%) | 24/24 | 24 | 1.00 |
| noisy and compressed (JPEG 30, noise) | 23/24 | 23 | 1.00 |
| rotated 8-25 degrees | 23/24 | 23 | 0.58 (against the unrotated rectangle) |
| mirrored | 22/24 | 22 | 1.00 |
| low resolution | 21/24 | 21 | 0.99 |
| perspective-warped | 21/24 | 21 | 0.77 |
| tiny detail (1.5-4%) | 20/24 | 19 | 0.99 |
| annotated screenshot | 20/24 | 20 | 0.70 |
| medium detail (18-32%) | 19/24 | 19 | 1.00 |
| **All 216** | **193 (89%)** | | |

The picture the part was cut from came back in **215 of 216**: 193 as "contains it", 17 more as a whole-picture copy (large parts; the region pass skips pictures already listed as copies), 5 more as "a part looks like it" (the window route), 1 missed. A first live run, on the first settings and counting only "contains it", gave 185 of 216 (86%); the difference is the settings (+8) and counting the copies and parts that were correct answers.

* False alarms: 0 on 14 probes from 7 pictures that are not in the case. Two further probes came from `contact.jpg`, a contact sheet of the case's own pictures: the search found case pictures inside it (126 agreeing points), which is correct.
* **Window route ("a part looks like it") level, 0.25** (largest DINOv2 distance of a window to the attached picture): at 0.15 / 0.20 / 0.25 / 0.30 the source came back as a part in 1 / 3 / 4 / 5 probes and 1 / 2 / 11 / 28 other pictures were listed over the 216 probes. 0.25 is the knee. The other pictures it lists are candidates (a different photograph with a similar-looking part), which the ground truth here counts as false because it knows only the source.
* 12 agreeing points = verified, 8 to 11 = probable (`FORENSIC_IMAGE_REGION_VERIFIED_MIN`, `_PROBABLE_MIN`). A calibration on real negatives from casework is still owed.
* Time: first search after a service restart about 45 s (all 97 pictures are read once; features kept in memory, 128 MB cap); later searches 1 to 5 s.

## 2. Photos by words: the cut-off (`tools/media_data/image_words_calibration.py`)

16 queries (airplane, laptop, dog, bus, truck, motorcycle, backpack, clock, boat, kitchen, cat, train, bicycle, people eating food, cell phone, person holding an umbrella) against the 63 in-case pictures that have a COCO caption written by a person (a picture is relevant when its caption names the thing; pictures without a caption are not counted).

| Rule | Precision | Recall | F1 |
|---|---|---|---|
| similarity >= 0.88 x the best | 0.97 | 0.71 | 0.82 |
| similarity >= 0.80 x the best | 0.86 | 0.77 | 0.81 |
| similarity >= 0.75 x the best | 0.78 | 0.79 | 0.78 |
| absolute similarity >= 0.09 | 0.92 | 0.73 | 0.81 |
| absolute similarity >= 0.10 | 1.00 | 0.48 | 0.65 |

Product setting: **best matches** at 0.88 x the best, **also possible** from 0.75 x, nothing below. Caption mentions miss pictures that contain the object without naming it, so precision is a lower bound.

**With the window vectors (all 97 pictures):** a picture can also score as its best of 34 windows. `tools/media_data/image_words_penalty_calibration.py` reads each picture's whole-picture and best-window similarity from the database for the same 16 queries and sweeps a penalty subtracted from the window score:

| Window penalty | Precision at 0.88 x best | Recall | F1 |
|---|---|---|---|
| 0 (windows compete freely) | 0.69 | 0.71 | 0.70 |
| 0.02 | 0.74 | 0.67 | 0.70 |
| 0.03 | 0.83 | 0.71 | 0.76 |
| 0.04 | 0.92 | 0.71 | 0.80 |
| **0.06 or more (windows never win)** | **0.97** | **0.71** | **0.82** |

The best of 34 noisy window scores is biased upward, so unrelated pictures climb. The ranked list therefore uses the whole picture (penalty 0.06 by default). Windows get their own group, **"in a part of a picture (not verified)"**: up to 6 pictures outside the ranked tiers whose best window is at least 0.92 x the best window of the case, outlined. The ground truth (captions) cannot reward a window that finds a real but uncaptioned object, so this group is unmeasured: it is a labelled candidate list.

**What a score cannot do:** the best similarity for things that are NOT in the case (giraffe 0.121, surfboard 0.094, helicopter 0.091, elephant 0.084, snowman 0.075, submarine 0.077, cow 0.058) overlaps the best similarity for things that are (0.085 to 0.134). A standardised score (how far the best picture stands out from the rest) does not separate them either (present: z 1.65 to 3.87). A whole-picture vector always has a nearest neighbour, so a firm "nothing here" needs a verification step (a vision-language model asked yes or no about the top candidates): planned, not built.

## 3. Ask routing (`tools/media_data/image_intent_routing_check.py`)

The model reads each of 29 sentences with the real prompt and schema: 14 descriptions of photographs (English, Roman Urdu, Urdu), 13 that are not (plates, written words, records, people, counts, a document), 1 named-file copies, 1 named-file look-alikes. **28 of 29 as expected.** The one miss ("which photos show the plate SYNT9150" read as copies of a named image with no file name) is rejected by the server (no file name in the message) and goes on to plate search.

Root cause of the first failures: the engine's text-intent step read "photos of an airplane" as a plate called AIRPLANE and "a baby watching something on a laptop" as words written in images. The image step now runs first for sentences about photographs.

## 4. Duplicate groups (`api/forensic_records/image_duplicate_groups.go`)

Live: 7 groups, 18 pictures, 11 that could be set aside, 0.5 s: laptop, motorcycle, bus, market (original, re-saved, cropped), truck and backpack (original, re-saved), and one pair of real plate photographs (`real-photo-PKPLATE-0074`, `-0064`, cosine distance 0.091). The truck and backpack crops are not linked (beyond 0.108).

## 5. The machine (what to know before reading any latency)

On this laptop the Docker VM has 7.6 GB. The language model (about 3 GB), the worker with its OCR role (2.3 GB) and the search service (up to 1.8 GB while indexing windows) do not fit together. Measured: with the search service resident at about 1 GB the model needed 99 to 124 s for a 30-token answer; with it stopped, 6 to 12 s (memory pressure fell from 63% to 5%). Fixes made: the service frees its heap after releasing its models (idle release 45 s, `malloc_trim`, two malloc arenas) and gives its model lock back every few windows. Remaining rule: do not run window indexing and interactive Ask at the same time here. A machine with more memory or a GPU removes the constraint.

Window indexing cost: about 14 s of compute per picture (34 windows x 2 models), 30 to 90 s per picture end to end including the worker's own readings.
