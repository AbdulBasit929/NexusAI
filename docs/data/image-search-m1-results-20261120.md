# M1 image search: results (2026-11-20) - DRAFT, being filled as each gate is measured

Pre-registration with amendments A1 to A4: `docs/data/image-search-m1-preregistration-20261120.md`. Hand-off and deploy checklist: `docs/roadmap/image-search-m1-handoff-20261120.md`. Nothing is deployed or switched on. A gate that is missed stays missed; sealed parts are read once.

## 0. What was and was not run (state at the time of writing)

| Gate | State |
|---|---|
| S0 integrity of the product descriptors | **MET** (section 1) |
| T1 ingest budget | memory **MET**; time per image **not yet decided** (the first run shared the CPU with the language model; a rerun on a quiet CPU is owed) (section 2) |
| X1 no leakage, C1 coverage line, A1 audit | **MET** on a throwaway database (section 3) |
| N1, N2, N3 near-duplicate gates | **NOT MET** as registered; N4 **MET**; N1b (hash at the new operating point, fresh photos) **MET** (section 4) |
| S1, S2 photos by words | not yet run |
| L1 latency at 30,000 images | not yet run (needs a quiet CPU) |
| U0 Urdu development proxy | not yet run (first attempt stopped: the language model was saturated by another session's job) |
| U1 Urdu on native-reviewed captions | **NOT MEASURED** (needs the owner's reviewers, D10) |

## 1. Integrity (S0): MET

150 Flickr 1K photos (the ones with the assistant-translated Urdu captions), product code (`ingestion/forensic_records/image_search.py`) against the bake-off tools (`image_eval.py`, the vectors saved by `image_text_retrieval.py`):

| Descriptor | Smallest cosine over 150 photos | Share of photos with cosine >= 0.999 |
|---|---|---|
| DINOv2 base image vector (product vs bake-off `Embedder`) | 0.9999998 | 100 % |
| SigLIP 2 image vector, vision tower alone (product vs full model saved by the bake-off) | 0.9999997 | 100 % |
| SigLIP 2 text vector, text tower alone (product vs full model, first English caption) | 0.9999998 | 100 % |
| PDQ-style hash equal | - | 100 % (150 of 150) |

Result file: `work-faces/m1/results/s0.json`. Gate: >= 99 % at 0.999 and hash equal for >= 99 %. **MET.**

## 2. Ingest speed and memory (T1)

300 Flickr photos through hash + DINOv2 + SigLIP 2 one at a time, 4 threads, container memory limit 2.3 GB:

| Measure | Value | Gate | Verdict |
|---|---|---|---|
| Peak memory above the idle container | 1,049 MB (peak 1,082 MB) | <= 2,500 MB | **MET** |
| Median time per image | 1.53 s (95th percentile 1.95 s, mean 1.61 s) under CPU contention | <= 1.0 s | **not decided**: the language model was using about 800 % CPU in another session's job during the run; the bake-off figures (DINOv2 0.33 s + SigLIP 2 0.37 s + hash 0.08 s) predict about 0.8 s on a quiet machine; to be rerun |

## 3. Isolation, coverage line, audit (X1, C1, A1): MET on a throwaway database

Throwaway PostgreSQL 16 (`timescale/timescaledb:latest-pg16`) with migrations 001 to 015 and 038 (`Rehearse-038.ps1 -KeepContainer -PublishPort`), Go tests `TestImageCaseSearchDB` (`api/forensic_records/image_case_search_db_test.go`), run as the table-owner role (the worst case: row-level security does not apply to it, so isolation rests on the explicit tenant and collection predicates inside the functions). The restricted runtime role is covered by the rolled-back SQL smoke test of the migration (`ops/038_image_search/smoke.sql`).

* **X1:** a probe image belonging to another collection or another tenant returns "not found" (HTTP 404 in the handler); a search in the other tenant sees only that tenant's image; a search in the other collection sees only that collection's image; a case with no images returns no results and coverage 0.
* **C1:** a case of five images (one failed, one identical re-upload, three processed plus one more processed) reports coverage 5 in case / 4 indexed / 1 failed / 1 re-upload and the sentence "Searched 4 of 5 images in this case." (plus the failure and re-upload clauses).
* **A1:** every search wrote one `image_search_log` row (who, kind, probe, parameters, result count, coverage); no answer contained a vector, a hash value or the model's numbers.
* Tier logic checked end to end: identical file, same image (hash within the distance), same photograph (vector within the distance), a different image excluded, the probe itself excluded, uploaded probes searched by hash, vector and file hash, an unprocessed probe answered with a conflict, more-like-this ranked.

Migration 038 rehearsal on a throwaway database: preflight, apply, verify, synthetic smoke (coverage counts, hash and vector searches, superseded versions never returned, tenant and collection isolation, runtime role may read and insert but not update or delete, constraints), second apply, rollback refused without its setting, gated rollback, re-apply, verify: all passed.

## 4. Near-duplicate gates (N1 to N4)

Set (amendment A4): 600 photographs of the COCO 2014 validation Karpathy 5K test split as sources (397 with their original in the gallery, 203 without = negatives), 1,000 more as distractors, 14 transformed copies per source = 8,400 queries; gallery 1397 images. Development part 440 sources, **sealed part 160 sources (read once, 2026-11-20 17:15)**. All descriptors from the product code (`image_search.py`). Thresholds as registered: tier A (hash) the loosest Hamming distance with development precision >= 99.5 % on family R = **91 bits**; tier B (DINOv2 base) the loosest cosine distance with development precision >= 99.5 % on families R and E = **0.108**. Files: `work-faces/m1/results/{sealed-nd,m1-nd-score}.json`.

| Gate | Pass condition | Sealed result | Verdict |
|---|---|---|---|
| **N1** hash tier, family R | precision >= 99.5 % and recall >= 95 % | precision 98.84 % (6 wrong answers), recall 100.00 % | **NOT MET** (precision) |
| **N2** DINOv2 tier, families R and E | precision >= 99 % and recall >= 95 % | precision 100.00 %, recall 90.27 % | **NOT MET** (recall) |
| **N3** DINOv2 tier, every single transformation | recall >= 80 % each | all of R and 6 of 8 of E at 92 to 100 %; `social_frame` 71.57 %, `crop60_wa` 29.41 % | **NOT MET** |
| **N4** lossless control | hash distance 0 and cosine distance <= 1e-6 | 600 of 600 and 600 of 600 | **MET** |

Combined tiers (label A when the hash is within its distance, otherwise B when the vector is within its): precision 98.94 %, recall 91.40 % (families R and E, sealed). Reported only: `screenshot_ui` (a full phone screenshot with status bar, app bar and message frame around the picture) is found in 23.53 % of cases by the DINOv2 tier and 0 % by the hash.

**Sealed precision / recall per transformation** (hash at 91 bits | DINOv2 at 0.108):

| Transformation | Hash tier | DINOv2 tier |
|---|---|---|
| `jpeg75` | 99.03 % / 100.00 % | 100.00 % / 100.00 % |
| `wa_like` | 99.03 % / 100.00 % | 100.00 % / 98.04 % |
| `wa_twice` | 98.08 % / 100.00 % | 100.00 % / 93.14 % |
| `resize50` | 99.03 % / 100.00 % | 100.00 % / 97.06 % |
| `png_roundtrip` | 99.03 % / 100.00 % | 100.00 % / 100.00 % |
| `crop85` | 70.00 % / 6.86 % | 100.00 % / 92.16 % |
| `mirror` | 100.00 % / 0.00 % | 100.00 % / 100.00 % |
| `rot5` | 98.73 % / 76.47 % | 100.00 % / 95.10 % |
| `tone` | 99.03 % / 100.00 % | 100.00 % / 99.02 % |
| `caption_bar` | 100.00 % / 99.02 % | 100.00 % / 100.00 % |
| `social_frame` | 66.67 % / 1.96 % | 100.00 % / 71.57 % |
| `screenshot_tight` | 100.00 % / 100.00 % | 100.00 % / 98.04 % |
| `crop60_wa` | 0.00 % / 0.00 % | 100.00 % / 29.41 % |
| `screenshot_ui` | 100.00 % / 0.00 % | 100.00 % / 23.53 % |

### 4a. What the misses are

* **N1 was lost by the registered threshold rule, not by the hash.** The rule picks the loosest distance that still holds 99.5 % on the development part; that was 91 bits, where unrelated images begin to appear (6 wrong answers on the sealed part). On the development part the farthest true re-save was 26 bits away and the nearest unrelated gallery image 86 bits away. The supplementary curve below (post hoc, **not** used to choose anything) shows the same on the sealed part: precision is 100 % from 0 to 80 bits and recall is 100 % from 30 bits.
* **N2 and N3 are real limits of a global DINOv2 vector** on this harder family list than the 2026-11-19 bake-off used: re-saves, resizes, tone, mirror, a 5-degree turn, a caption bar and a tight screenshot are found at 92 to 100 %, but a picture framed in white (72 %) or cropped to 60 % and recompressed (29 %) usually is not, and a picture inside a full phone screenshot is found 24 % of the time. Even at the best operating point the registered target is out of reach: 93.9 % recall at 99.1 % precision (0.14).

Supplementary, post hoc: tier A (family R) on the sealed part

| Distance | Precision | Recall | Wrong answers |
|---|---|---|---|
| 0 bits | 100.00 % | 32.55 % | 0 |
| 10 bits | 100.00 % | 71.57 % | 0 |
| 20 bits | 100.00 % | 99.02 % | 0 |
| 30 bits | 100.00 % | 100.00 % | 0 |
| 40 bits | 100.00 % | 100.00 % | 0 |
| 50 bits | 100.00 % | 100.00 % | 0 |
| 60 bits | 100.00 % | 100.00 % | 0 |
| 70 bits | 100.00 % | 100.00 % | 0 |
| 80 bits | 100.00 % | 100.00 % | 0 |
| 91 bits | 98.84 % | 100.00 % | 6 |
| 100 bits | 76.12 % | 100.00 % | 160 |
| 110 bits | 63.75 % | 100.00 % | 290 |
| 120 bits | 63.75 % | 100.00 % | 290 |

Supplementary, post hoc: tier B (families R and E) on the sealed part

| Cosine distance | Precision | Recall | Wrong answers |
|---|---|---|---|
| 0.04 | 100.00 % | 64.25 % | 0 |
| 0.06 | 100.00 % | 76.85 % | 0 |
| 0.08 | 100.00 % | 84.31 % | 0 |
| 0.1 | 100.00 % | 88.69 % | 0 |
| 0.12 | 100.00 % | 91.93 % | 0 |
| 0.14 | 99.12 % | 93.89 % | 11 |
| 0.16 | 96.41 % | 95.17 % | 47 |
| 0.18 | 95.30 % | 96.30 % | 63 |
| 0.2 | 94.70 % | 96.98 % | 72 |
| 0.22 | 93.43 % | 97.59 % | 91 |
| 0.24 | 92.19 % | 97.96 % | 110 |
| 0.26 | 90.25 % | 98.42 % | 141 |
| 0.28 | 88.87 % | 98.72 % | 164 |
| 0.3 | 87.75 % | 98.87 % | 183 |
| 0.32 | 87.06 % | 98.94 % | 195 |

### 4b. Amendment A5 and gate N1b (hash tier at the new operating point): MET

After the sealed read the product's hash distance was set **from the development gap only** to **56 bits** (midway between the farthest true copy, 26, and the nearest unrelated image, 86), recorded as amendment A5 **before** any further photo was scored, and confirmed on photographs that took no part in any decision: the 1,000 SEM1000 photos with their five family R copies each (gallery = 670 originals, the rest are negatives; 5000 queries; one read; `work-faces/m1/results/n1b.json`).

| Transformation | Precision / recall at 56 bits |
|---|---|
| `jpeg75` | 100.00 % / 100.00 % |
| `wa_like` | 100.00 % / 100.00 % |
| `wa_twice` | 100.00 % / 100.00 % |
| `resize50` | 100.00 % / 100.00 % |
| `png_roundtrip` | 100.00 % / 100.00 % |

Overall precision 100.00 %, recall 100.00 % (3350 copies found, 0 wrong answers). **N1b MET** (target precision >= 99.5 %, recall >= 95 %). Two cautions: on this set the farthest true copy was 46 bits (26 on the development part), so the 56-bit operating point has about 10 bits of room on the true side and about 28 on the unrelated side (nearest wrong answer 84 bits); and the photographs are 640 px COCO images, not phone photographs at full resolution.

### 4c. Product settings that follow (they are settings, not claims beyond the measurements)

Hash distance **56 bits** ("the same image, re-saved or resized"); DINOv2 cosine distance **0.108** (the registered threshold; precision 100 % on the sealed part) labelled "the same photograph, re-compressed or lightly edited"; the limitation sentence of every copy search names the measured weaknesses (heavy crops, framed pictures, full-screen screenshots) and says that no match does not prove that no copy exists. Both can be changed in the environment without a rebuild.

## 5. Photos by words (S1, S2), latency (L1), Urdu proxy (U0)

To be filled.
