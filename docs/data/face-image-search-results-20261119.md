# Face and image search bake-off: results (started 2026-11-19)

Pre-registration: `face-image-search-preregistration-20261119.md` (gates fixed before the first measurement; amendments A1 and A2 dated 2026-11-19 before any group-level number was seen). Research: `docs/research/FACE_IMAGE_SEARCH_RESEARCH_20261119.md`. Tools: `tools/media_data/face_*.py`, `image_text_retrieval.py`, `Run-FaceMeasure.ps1`, evaluation image `nexusai-media-eval-face:20261119`. **Everything below is measured on public data on a CPU laptop in a memory-capped container without network; nothing is deployed, no role is enabled, nothing is committed.** Real-world performance on South Asian people (genuine pairs) is **not measured** (no consented set exists yet).

## 0. What was run and what was not (read this first)

**Run (all on public or synthetic data, CPU laptop, memory-capped containers without network):** detection on WIDER FACE (YuNet, BlazeFace; development threshold, sealed read once) and on FairFace; recognition of SFace and AuraFace on LFW, CFP-FP, AgeDB-30, CALFW, CPLFW; the end-to-end path with YuNet alignment on the original LFW photos; FairFace operating thresholds and false-match rates by group; OFIQ quality (conformance and a 1,200-image sample); pgvector HNSW and exact search with per-case filtering at 10 K (real vectors) and 100 K (synthetic expansion); text-to-image retrieval (English and Urdu) and near-duplicate retrieval for SigLIP v1, SigLIP 2 and DINOv2.

**NOT run, and why:** real South Asian genuine pairs (no consented set exists: the genuine-side result is a synthetic degradation proxy only); CCTV-quality faces; faces in video; the pgvector 1 M tier (memory); the pgvector iterative-scan cap above its default; the product's GGUF packaging of SFace (the bake-off measured the OpenCV ONNX file); AuraFace's centre-face end-to-end arm and SFace's flip arm; a native-speaker-reviewed Urdu caption set; an Urdu-to-English translation route in front of SigLIP; Punjabi/Saraiki speech and video (research and pre-registration only, no measurement); any training or fine-tuning. Nothing was deployed, enabled, committed or written into a case. Machine events that cost time: the host-memory watchdog killed my test containers several times (the Docker VM and the resident language model leave about 2 GB free); every long run now checkpoints and retries.

**Four amendments after pre-registration, all dated and disclosed:** A1 (operating threshold on within-group impostors, before any group number), A2 (single embedding, flip only where a target was missed), A3 (centre-face diagnostic added after R2 missed; the R2 verdict is unchanged), A4 (Urdu captions: the free machine translation was unusable, replaced by 150 captions translated by the assistant, unreviewed).

## 1. Files used (SHA-256 prefixes; full hashes in `C:/tmp/nexusai-media-data/manifest-20261119-faces.jsonl`)

| File | Bytes | SHA-256 (first 16) | Licence as read |
|---|---|---|---|
| `face_detection_yunet_2023mar.onnx` | | `8f2383e4dd3cfbb4` (equals the ledger row MOD-008) | MIT |
| `face_recognition_sface_2021dec.onnx` (OpenCV Zoo) | 38,696,353 | `0ba9fbfa01b5270c` (the product's configured value `9ce78d4b...` is a different file: a GGUF packaging) | Apache-2.0 |
| `glintr100.onnx` (AuraFace-v1 recogniser only) | | `a7933ea5330113b0` | Apache-2.0 (card) |
| `blaze_face_short_range.tflite` | 229,746 | `b4578f35940bf5a1` | Apache-2.0 (Google card); surveillance and identity recognition out of scope |
| `lfw.bin`, `cfp_fp.bin`, `agedb_30.bin`, `calfw.bin`, `cplfw.bin` (unofficial mirror) | 65.1 / 76.5 / 76.5 / 74.2 / 62.6 MB | `2cdf0242947ad828`, `a8754ddf9729a014`, `1af672c0ea829b95`, `f643466e87586567`, `2d18b80537ef3dce` | research benchmarks; the mirror states none |
| FairFace validation 1.25 (parquet) | 236,824,583 | `ce47863c011f355e` | CC BY 4.0 |
| WIDER FACE val + annotations | 362,752,168 / 3,591,642 | `f9efbd09f28c5d2d`, `c7561e4f5e7a118c` | cc-by-nc-nd-4.0 (hub), evaluation only |
| LFW originals + `pairs.txt` (scikit-learn project's figshare mirror) | 180,566,744 / 155,335 | `055f7d9c632d7370`, `ea42330c62c92989` | research use, no licence text |
| Flickr30k 1K test images + captions | 140,829,215 / 1,125,896 | `b0fa9970cc1680a9`, `a427bd870759196b` | Flickr owners' terms, non-commercial research |

(Counts after unpacking: LFW 6,000 pairs with 7,701 unique images; CFP-FP 7,000 / 5,890; AgeDB-30 6,000 / 5,296; CALFW 6,000 / 6,098; CPLFW 6,000 / 6,595; WIDER val 3,226 images with 39,708 labelled faces of which 585 flagged invalid; FairFace validation 10,954 images: East Asian 1,550, Indian 1,516, Black 1,556, White 2,085, Middle Eastern 1,209, Latino 1,623, Southeast Asian 1,415.)

## 2. Recognition on the public verification sets (gates R1, R3)

All five sets, both models, 112x112 pre-aligned pairs from the mirror, cosine similarity, 10-fold accuracy as in InsightFace's verification code (threshold chosen on nine folds), **single embedding per image, no flip test**. True accept rate at false accept rate 1e-3 uses only the 3,000 to 3,500 impostor pairs of each set (the intervals are wide; read it as a coarse figure).

| Set (pairs) | SFace accuracy (std) | AuraFace accuracy (std) | AuraFace card | AuraFace minus card | SFace TAR at FAR 1e-3 (95%) | AuraFace TAR at FAR 1e-3 (95%) | Accuracy at the fixed cosine 0.363: SFace / AuraFace |
|---|---|---|---|---|---|---|---|
| LFW (6,000) | **99.60%** (0.30) | 99.27% (0.41) | 99.650 | -0.38 | 99.33% (99.03 to 99.60) | 98.37% (96.83 to 99.10) | 99.57% / 98.02% |
| CFP-FP (7,000) | **96.96%** (0.73) | 94.36% (1.19) | 95.186 | -0.83 | 86.71% (83.44 to 89.47) | 73.26% (68.13 to 79.87) | 88.41% / 76.50% |
| AgeDB-30 (6,000) | 94.92% (1.15) | **95.15%** (1.16) | 96.100 | -0.95 | 78.83% (74.05 to 82.92) | 70.20% (67.75 to 80.62) | 89.97% / 80.60% |
| CALFW (6,000) | **94.82%** (0.80) | 93.85% (0.98) | 94.700 | -0.85 | 83.13% (78.69 to 86.23) | 80.97% (76.72 to 84.72) | 93.70% / 88.13% |
| CPLFW (6,000) | **90.62%** (1.33) | 88.97% (1.58) | 90.933 | **-1.97** | 71.57% (67.60 to 75.20) | 62.63% (50.86 to 65.08) | 87.77% / 76.37% |
| Mean of the four hard sets | **94.33%** | 93.08% | | | | | |

* **Gate R1 (reproduction): MET for SFace on LFW** (99.60% against the zoo's 99.40%, tolerance 0.5). **For AuraFace: met on LFW (-0.38, tolerance 0.5), CFP-FP (-0.83), AgeDB-30 (-0.95) and CALFW (-0.85) (tolerance 1.0), NOT MET on CPLFW (-1.97).** The shortfall is one-sided and present on every set (0.4 to 2.0 points below the card), which fits a card number obtained with the flip test (embedding of the image plus its mirror) and not a broken pipeline; under Amendment A2(b) the flip arm was run on CPLFW only (section 2a). **Every AuraFace number in this note is "reproduced to within 0.4 to 0.95 points on four of five sets and 1.3 points on the hardest (CPLFW, with the flip test)"**.
* **Gate R3 (AuraFace at least 3 points better than SFace on the four hard sets): NOT MET, and the sign is reversed.** AuraFace averages **93.08%** against SFace's **94.33%** (-1.25 points); SFace is ahead on CFP-FP (+2.6), CALFW (+1.0) and CPLFW (+1.7), AuraFace ahead only on AgeDB-30 (+0.2, inside the standard deviation). At the fixed cosine 0.363 the gap is larger because AuraFace's scores sit lower (its mean best thresholds are 0.17 to 0.25, SFace's 0.21 to 0.35), which again shows the threshold must be calibrated per model.
* Both models are licence-clean on paper and trained on undisclosed data (SFace: the paper trains on CASIA-WebFace, VGGFace2 and MS-Celeb-1M and does not say which one the zoo file uses; AuraFace: "a commercial dataset"). The public sets are mostly Western celebrity photographs; none of this is a statement about South Asian genuine pairs, which are **not measured**.

### 2a. Flip-test arm (Amendment A2b), AuraFace on CPLFW only

Embedding of the image plus embedding of its mirror image (what some published numbers use), single run: **89.67%** (std 1.59; TAR at FAR 1e-3 62.4%; accuracy at cosine 0.363 80.6%) against 88.97% without the flip and the card's 90.93%. The flip recovers 0.70 of the 1.97-point gap and **leaves 1.27 points: gate R1 stays NOT MET for AuraFace on CPLFW**. I do not know why the last 1.3 points are missing (a different build of the set on the mirror, the card's own preprocessing, or the model file); none of the explanations was tested. It does not change the decision: SFace stays ahead of AuraFace on this pipeline on three of four hard sets, and the flip test (which costs twice the compute) was not run for SFace, so the comparison of record is the single-embedding one in the table above.

## 3. Detection (gates D1, D2, D3)

WIDER FACE validation, 3,226 images: development half (430 images, last hex digit of the file-name hash 0 or 1) fixes each detector's score threshold at the highest recall with at most 0.1 false boxes per image; the sealed half (2,796 images, 39,708 labelled faces in total, 585 flagged invalid and ignored) was read once per detector at that threshold.

| Detector | Threshold chosen on dev | False boxes per image on sealed | Recall, faces >= 40 px wide | 20 to 40 px | under 20 px | heavy blur (>= 20 px) | atypical pose (>= 20 px) | heavy occlusion (>= 20 px) | Median ms per image (contended run) |
|---|---|---|---|---|---|---|---|---|---|
| **YuNet 2023mar** | 0.80 (dev: 0.086 false boxes, recall 79.4%) | 0.106 | **82.5%** (5,138 of 6,225) | 66.2% | 14.9% | 48.8% | 30.4% | 26.6% | 64 |
| **BlazeFace short range** | 0.60 (dev: 0.095 false boxes, recall 16.0%) | 0.117 | **15.9%** (989 of 6,225) | 0.0% | 0.0% | 0.1% | 4.8% | 1.1% | 10 |

* **Gate D1 (YuNet >= 80% at <= 0.15 false boxes per image on sealed): MET** (82.5% at 0.106). It is met for clear faces; on small (under 40 px), blurred, turned or occluded faces YuNet misses most of what WIDER labels, which is the CCTV-still case.
* **Gate D2 (BlazeFace usable for video frames: within 5 points of YuNet and >= 5x faster): NOT met.** It is faster (about 6x in the contended run; 3.7 ms against 25 ms per 0.9-megapixel image in the 4-thread timing run) but finds 16% of the faces YuNet finds, as its model card predicts (faces smaller than 20% of the image side are out of scope; it also lists surveillance and identity recognition as out of scope). Not adopted. Whether its decoding is correct for the faces it is built for is checked by the FairFace detection rate below (a high rate there means the decoding is right).
* **Gate D3 (detection rate by group on FairFace): MET** for both detectors (at least 99.48% in every group, spread 0.40 and 0.45 points): see the start of section 4.

## 4. Operating thresholds and false-match rate by group (gates R5, R6; gate D3 is stated at the start of this section)

FairFace validation, padding 1.25: 10,954 images, 10,946 aligned by YuNet (score >= 0.6; 8 lost: 3 East Asian, 2 White, 1 Middle Eastern, 2 Southeast Asian), 8,765,253 pairs of different images inside the same group (Amendment A1: the operating threshold is taken on these pooled within-group pairs). No identity labels exist, so every pair of different images counts as two different people; a few photos of one person would raise all groups alike. 95% intervals resample images (200 draws).

**Detection by group (gate D3): MET for both detectors.** A face found near the centre in at least 99.5% of images in every group; spread between best and worst group 0.40 points (YuNet at 0.80) and 0.45 points (BlazeFace at 0.60), limit 2. Overall 99.65% and 99.79%. Indian: 99.80% and 100%. This also shows BlazeFace's decoding is correct (it finds almost every large centred face), so its 16% on WIDER is its stated range, not a bug. FairFace faces are large chips, so this says nothing about small or blurred faces.

**Operating thresholds** (cosine similarity at which the pooled within-group false-match rate is the target):

| Model | Target 1e-3 | Target 1e-4 | Threshold that would give 1e-4 inside the Indian group alone | Inside the White group alone |
|---|---|---|---|---|
| SFace | 0.472 | 0.669 | 0.602 | 0.615 |
| AuraFace | 0.432 | 0.624 | 0.544 | 0.549 |

One cosine threshold must not be shared between the models (their scales differ).

**False-match rate inside each group at the pooled 1e-4 threshold** (pairs inside the group):

| Group | SFace | AuraFace |
|---|---|---|
| East Asian | 1.27e-4 | 1.66e-4 |
| **Indian** | **6.97e-5** (95%: 4.4e-5 to 9.9e-5) | **6.79e-5** (4.4e-5 to 9.9e-5) |
| Black | 9.09e-5 | 7.52e-5 |
| **White** | **7.01e-5** (3.9e-5 to 1.2e-4) | **6.64e-5** (4.0e-5 to 1.1e-4) |
| Middle Eastern | 2.13e-4 | 2.11e-4 |
| Latino/Hispanic | 1.10e-4 | 9.50e-5 |
| Southeast Asian | 8.22e-5 | 8.52e-5 |
| Indian / White | 0.99 (95%: 0.48 to 1.91) | 1.02 (0.54 to 1.89) |
| Indian / pooled target | 0.70 | 0.68 |

At the pooled 1e-3 threshold the Indian / White ratio is 2.66 (95%: 1.88 to 3.93) for SFace and 2.19 (1.47 to 2.97) for AuraFace; Indian / pooled target 1.10 and 0.48.

* **Gate R5 (Indian rate <= 3x White and <= 3x the pooled target at 1e-4, upper 95% bound of the ratio <= 5): MET for both models** (ratios 0.99 and 1.02, upper bounds 1.91 and 1.89; Indian / pooled 0.70 and 0.68). At the looser 1e-3 threshold the point ratios (2.66 and 2.19) are still under 3, and the upper bounds (3.93 and 2.97) under 5. **Indian faces are not the hardest group for either model**: East Asian and Middle Eastern pairs have the highest rates (up to 2.1x the target), White the lowest. At 1e-3 AuraFace's Southeast Asian (2.2e-3) and East Asian (2.8e-3) rates are twice to three times the target.
* **Gate R6 (synthetic genuine proxy: the Indian rate of falsely rejecting a degraded copy of the same photo <= White + 2 points, at the 1e-4 threshold): MET for both models** (SFace Indian 4.42% vs White 5.57%; AuraFace Indian 7.06% vs White 10.18%; 1,515 and 2,082 pairs). **This is not a statement about two different real photos of one person** (the degraded copy is the same face, brightness x0.7, blur sigma 1.5, half resolution, JPEG quality 30); it is an optimistic proxy and the real South Asian genuine-pair error is **not measured**. A side finding on the same proxy: at equal false-match rate AuraFace rejects more degraded copies than SFace (7 to 10% against 4 to 6%).
* **The product's configured default threshold is too permissive.** The LocalAI gallery entry configures SFace verification at cosine 0.363. On FairFace, pairs of different people reach 0.363 with probability **0.94% pooled (about 1 in 107), 1.6% inside the Indian group (about 1 in 62), 1.6% Black, 1.2% Southeast Asian, 0.96% East Asian, 0.39% White (about 1 in 256)**: about 100 times the 1e-4 operating point and 4.1 times higher for Indian than for White faces. (AuraFace at 0.363: 0.43% pooled, 0.27% Indian, 0.07% White.) FairFace images are web photographs of mixed quality, which may differ from case images. If matching is ever enabled, the threshold must come from a rate chosen by the owner (the research note's open question), not from the default.

## 5. End-to-end path with YuNet alignment (gate R2) and cost (gate R4)

**Cost, gate R4: MET.** One face at a time, 4 threads (`face_timing.py`, medians of 100 calls after 5 warm-ups): SFace 20.9 ms (95th percentile 24.9 ms), peak resident memory 171 MB; **AuraFace 273.8 ms (95th percentile 355.6 ms), 847 MB** (limits 500 ms and 1.5 GB). YuNet 24.9 ms and BlazeFace 3.7 ms per image of 0.91 megapixels on average. The sampling of the laptop was shared with other sessions' work, so these are medians of a noisy machine. AuraFace is about 13 times the cost of SFace per face.

**Gate R2 (LFW originals through YuNet detection and alignment, within 1.0 point of the pre-aligned accuracy and at most 0.5% faces lost): NOT MET.** All 7,701 faces were found (0% lost) and the 6,000 standard pairs scored, but accuracy fell by about 3 points for both models:

| Model | Pre-aligned benchmark crops | End-to-end, highest-scoring face (registered rule) | Change | End-to-end, face nearest the image centre (diagnostic R2b, Amendment A3, post hoc) |
|---|---|---|---|---|
| SFace | 99.60% | **96.62%** (std 0.89) | -2.98 | **99.33%** (std 0.38), -0.27 |
| AuraFace | 99.27% | **96.60%** (std 0.61) | -2.67 | not run (SFace only, to spare the machine) |

The cause is the choice of face, not the alignment: LFW photographs often contain several faces and the highest-scoring one is not always the labelled person; choosing the centred face brings SFace back to within 0.3 points of the benchmark. For the product this means a photograph with several faces needs the analyst (or an explicit rule shown to the analyst) to say which face is the one searched; an automatic "best-scoring face" costs about 3 points of pair accuracy on a set where the target is centred.

## 6. Face quality, OFIQ (gates Q1 to Q3)

OFIQ v1.1.0 through the third-party package `ofiqpy` 0.2.2 in an isolated environment, with the BSI model files (`OFIQ-MODELS.zip`, 422,400,019 bytes, from the ISO portal) and the canonical config (`ofiq_config.jaxn` sha256 `e1172867...`, equal to the value `ofiqpy` requires; it verified all twelve model files before assessing). **Licences read after the download** (files in `work-faces/results/ofiq-model-licences/`): the OFIQ code and its own models are MIT (BSI); the components come from MIT projects (SSD face detector, ADNet landmarks, FaceExtraction occlusion, BiSeNet face parsing, 3DDFA_V2 head pose, Efficient-Expression-Neutrality) and **Apache-2.0** projects (MagFace for the unified quality score, HSEMotion for expression, whose final classification layer was zeroed "such that the model files cannot be used for emotion estimation"). The conformance images carry the rule that they **shall only be used for reproducing conformance tests and shall not be modified or republished**; they were used unmodified for that and nothing else.

* **Gate Q1 (the package reproduces the C++ results on the BSI conformance images): MET.** 28 of 28 images assessed with status success; **all 784 component scores (28 images x 28 components) equal the C++ expected scalars exactly** (largest difference 0); the unified quality score is within 1 point on 28 of 28.
* **Gate Q2 (a score for at least 95% of FairFace images, at most 2 s per image with 4 threads): MET.** Sample of 300 Indian and 300 White faces chosen by SHA-256 order, each as the original and as the degraded copy (1,200 images): 1,188 assessed fully, 10 partly, 2 failures to assess, so **1,198 of 1,200 (99.8%) have a usable status**. Median **1.72 s** per image (95th percentile 2.24 s) with 4 threads while another job was running on the machine.
* **Gate Q3 (quality predicts matching: Spearman correlation between the quality of the degraded copy and its genuine cosine score with AuraFace, at least 0.3): MET.** **0.542** (599 pairs, p about 5e-47). OFIQ also sees the degradation: mean unified score 31.2 for the originals and 15.4 for the degraded copies; the degraded copy scores lower in 95.2% of pairs. (Both ends of this test use the same synthetic degradation; it shows OFIQ's score moves with what hurts matching, not that it predicts real casework misses.)

Meaning for the product: a quality score next to every face result is justified by these numbers (a low OFIQ score warns that the match score is unreliable). It is a warning figure, not a threshold: no OFIQ cut-off for "refuse to search" has been calibrated here.

## 7. Index: pgvector HNSW with per-case filtering (gates I1, I2)

A throw-away Postgres 16.14 from the product's database image with **pgvector 0.8.1** (the version installed there; the README read earlier describes 0.8.7), on a docker network with no route out, 2.2 GB memory cap; the live database was not touched. Table of 512-dimension AuraFace vectors with a case id (100 cases of about 1% each); HNSW cosine index, m = 16, ef_construction = 64; 100 probe vectors per setting (noisy variants of stored vectors); ground truth = exact search with index scans off; recall@10 counts a short answer as misses. **10 K tier: the 10,000 real FairFace embeddings. 100 K tier: SYNTHETIC expansion** (each vector = a real one plus Gaussian noise of relative size 0.7, so clusters of near-identical "same person" vectors exist): it tests the index, not real-world embedding geometry. **The 1 M tier was NOT run**: the throw-away container would need about 5 GB and the Docker VM has about 2 GB free beside the resident language model.

**Build and size:** 10 K rows: index built in 12 s, 27 MB. 100 K rows: 82 s, 273 MB (partitioned by case, 100 per-case indexes: 34 s).

**Recall@10 and latency (median / 95th percentile, ms), 100 K tier:**

| Filter | Exact scan (index off) | HNSW ef 40, iterative off | ef 40, relaxed order | ef 40, strict order | ef 200, off | ef 200, relaxed | ef 200, strict | Partitioned by case, per-partition HNSW, ef 40 |
|---|---|---|---|---|---|---|---|---|
| 1% (one case, about 1,000 rows) | 22 / 28 | recall **0.04** (0.4 rows returned), 1.5 / 1.8 | 0.80, 14 / 19 | 0.67, 14 / 22 | 1.00, 17 / 20 | 1.00, 20 / 25 | 1.00, 17 / 22 | **1.00**, 16 / 19 |
| 10% (ten cases, about 10,000 rows) | 169 / 208 | **0.40** (4.0 rows returned), 2.0 / 3.4 | 0.81, 4.0 / 6.6 | 0.72, 3.5 / 5.7 | 0.86 (10 rows), 4.8 / 6.8 | **0.86**, 4.7 / 5.5 | 0.86, 6.3 / 9.0 | **1.00**, 147 / 170 |
| 100% (no filter, 100,000 rows) | **1,499** / 1,672 | 0.99, 1.9 / 2.7 | 0.99, 1.6 / 2.0 | 0.99, 1.6 / 2.0 | 0.997, 4.6 / 6.0 | 0.997, 4.2 / 4.8 | 0.997, 4.2 / 4.7 | |

At 10 K rows (real vectors) every setting reached recall 1.00 and 3 to 20 ms for the filtered cases, 145 ms for an unfiltered exact scan. In the 1% rows with ef 200 the planner most likely chose the case index with an exact sort (recall exactly 1.00, ef-independent); I did not check the plans.

* **What the table shows:** pgvector's documented behaviour is reproduced: **without iterative scan a filter that keeps 10% of rows returns only about 4 of 10 rows (recall 0.40), and a 1% filter returns 0.4 rows (recall 0.04)**, because the filter is applied after the index scan. Iterative scan fixes the count (10 rows) but not the quality: best recall at 10% selectivity is **0.86**.
* **Gate I1 (100 K rows, 10% filter, iterative scan on: recall@10 >= 0.95 and p95 <= 200 ms): NOT MET.** Latency is fine (5 to 9 ms) but recall tops out at 0.857 (ef_search 200); the cause is probably the default cap of 20,000 index tuples per iterative scan (`hnsw.max_scan_tuples`), which I did **not** test above its default: a follow-up arm with a larger cap is the obvious next measurement and is not claimed here.
* **Gate I2 (an exact scan inside one case of 100 K faces in <= 500 ms): NOT MET as worded; MET for the case sizes actually tested.** Exact search inside one case of about 1,000 faces took 22 ms (95th percentile 32 ms); inside ten cases (about 10,000 faces) 169 ms; over the whole 100,000-face table 1,499 ms. Cost is about linear, so **an exact search stays under 500 ms up to roughly 30,000 faces per case** on this CPU; a single case of 100,000 faces takes about 1.5 s, which breaks the gate as written.
* **Partitioning by case (list partitions, one HNSW index each) gave recall 1.00 at both selectivities** (16 ms for one case, 147 ms for ten cases at 100 K rows) and builds faster than one global index; it is the design that gives exact-quality answers for a per-case search without scanning the whole table.
* **Meaning for "return ALL matches above a threshold":** an HNSW index cannot promise completeness (best filtered recall 0.86; unfiltered 0.99 to 0.997). The product's per-case face search should therefore be an **exact scan inside the case** for cases up to about 30,000 faces, and a per-case partition with an exact or high-ef index search beyond that, with the stated completeness; the global HNSW index is for the cross-case "have we seen this face anywhere" lead, with its measured recall printed beside the answer.

## 8. Image retrieval (gates V-a to V-d)

Flickr30k 1K test (1,000 images, five English captions each). Partition by the SHA-256 of the file name: development 251 images (last hex digit 0 to 3), **sealed 749 images, 3,745 captions, scored once**. Text-to-image recall is measured inside the gallery of the partition. Both models: the product's SigLIP v1 base (`google/siglip-base-patch16-224`, revision `7fd15f06...`) and SigLIP 2 base (`google/siglip2-base-patch16-224`, Apache-2.0 per its hub metadata), lower-cased captions, 64-token padding, 4 threads, fp32.

| Sealed half | SigLIP v1 base | SigLIP 2 base |
|---|---|---|
| English text-to-image R@1 / R@5 / R@10 (3,745 captions) | 77.06 / 93.30 / 96.37 | **80.61 / 95.27 / 97.22** |
| English image-to-text R@1 (749 images) | 91.59 | **93.32** |
| Development half, text-to-image R@1 / R@10 | 86.69 / 98.96 | 89.08 / 99.52 |
| **Urdu** text-to-image R@1 / R@5 / R@10 (150 hand-translated captions against all 749 images) | **0.67 / 2.00 / 4.00** | **3.33 / 10.00 / 15.33** |
| English control on the same 150 images (first caption, same gallery) R@1 / R@10 | 88.67 / 98.67 | 90.67 / 98.67 |
| Median time per image / per caption, 4 threads | 0.286 s / 73 ms | 0.290 s / 72 ms |
| Peak resident memory | 1,319 MB | 1,795 MB |

* **Gate V-a (SigLIP 2 English R@1 not more than 2 points below v1): MET** (+3.55). SigLIP 2 is better than v1 on every English measure.
* **Gate V-b (SigLIP 2 Urdu R@10 at least 20 points above v1): NOT MET** (15.33 against 4.00: +11.33). **Both models are close to useless for Urdu text queries** (chance for R@10 among 749 images is about 1.3%; the same images with English captions score 98.7%). Caveats stated in Amendment A4: the 150 Urdu captions were translated by the assistant and are not reviewed by a native speaker, and the free machine-translation model (`opus-mt-en-ur`) produced unusable text and was dropped. A translation error cannot explain a gap of 80+ points, but a native-speaker set (the owner's reviewers) would be the proper test. **For the product this means Urdu questions must go through the language model (translated to English) before a SigLIP-family search, or a genuinely multilingual retrieval model must be found and measured; this route was not measured here.**
* **Gate V-d (new default model at most 1.0 s per image and 2.5 GB with 4 threads): MET for SigLIP 2** (0.29 s, 1.8 GB). It costs about 0.5 GB more memory than v1 at the same speed.
* **Gate V-c (near-duplicate retrieval at the 99.5%-precision threshold: the best of the three has sealed recall >= 90%; name it): MET; the best is DINOv2 base.** Protocol of `image_eval.py` (unchanged): 200 Open Images photos (CC BY 2.0) each with 13 transformations (JPEG 30 and 70, resizes, brightness, contrast, grey, crops, rotations, mirror, overlay, watermark); two thirds of the originals form the gallery, one third do not (negatives that must return nothing); the cosine threshold is the loosest one with precision >= 99.5% on the development photos, then applied once to the sealed photos (about 504 positive queries, a small set).

| Model | Threshold (dev) | Development precision / recall | **Sealed precision / recall** | False matches on sealed | ms per image (6 threads, shared machine) |
|---|---|---|---|---|---|
| SigLIP v1 base | 0.215 | 99.6% / 99.9% | **95.6% / 99.8%** | 23 | 283 |
| SigLIP 2 base | 0.210 | 99.7% / 100.0% | **96.2% / 99.8%** | 20 | 292 |
| **DINOv2 base** | 0.205 | 99.5% / 99.7% | **100.0% / 100.0%** | 0 | 333 |

Perceptual hashes on the same photos and protocol (run afterwards, 2026-11-20, same 200 photos; sealed read once per method):

| Method | Threshold (dev) | Sealed precision / recall | Recall on mild changes (JPEG, resize, brightness, overlay, watermark) | Recall on strong changes (crops, rotation, mirror) | ms per image |
|---|---|---|---|---|---|
| PDQ-style 256-bit hash | Hamming 91 | **100.0% / 74.2%** | 100.0% | 48.4% | 81 |
| dHash-64 (the API's `image_compare.go` method) | Hamming 10 | **100.0% / 74.4%** | 99.6% | 49.2% | 221 |

The hashes never produced a false match and are almost perfect on re-compression, resizing, brightness, overlays and watermarks, but miss most cropped (80% crop: 2 to 7% found), mirrored (0 to 2%) and rotated-by-10-degrees (6 to 15%) copies. DINOv2 base finds all of those. Design consequence: a cheap hash for "the same file re-saved" and DINOv2 for "the same photograph, edited", reported as two different labels.

All three find the transformed copy of a gallery photo in 99.7% or more of cases. The difference is in the threshold: the two SigLIP models fall below the 99.5% precision floor on the sealed half (95.6% and 96.2%: photos with no original in the gallery got matched), while DINOv2's threshold carries over (no false match). DINOv2 base is therefore the model for "the same photograph" and "find the near-duplicates of this image"; SigLIP is for meaning ("find photos of ..."). With 504 positive queries and a small number of negative photos in the sealed part, treat the precision difference as strong evidence on this set, not as a proven rate.

## 9. Draft ledger rows (not edited into `docs/compliance/licence-ledger.csv`; the owner or the ledger's keeper adds them)

Columns as in the ledger: id, component, kind, role, version, usage, licence, evidence tier, evidence, restrictions, status, ship blocking, condition, config ref, checked on. Tier B = a primary page or hub metadata read, not the weights' own licence file; none of these is "cleared for product use" (training-data provenance is open for every face model).

| Id | Component | Role | Licence as read | Evidence (2026-11-19) | Restrictions / condition | Status |
|---|---|---|---|---|---|---|
| MOD-050 | BlazeFace short range (MediaPipe `blaze_face_short_range.tflite`, sha256 `b4578f35...`) | detector candidate for prominent faces | Apache-2.0 | Google model card PDF read ("LICENSED UNDER Apache License, Version 2.0") | card lists surveillance and identity recognition as out of scope, faces under 20% of the image side as out of scope; measured here (section 3) | evaluation-only; ship-blocking until the owner and counsel accept the out-of-scope clause |
| MOD-051 | OFIQ (BSI, ISO/IEC 29794-5 reference), v1.1.0 | face image quality | code MIT (`LICENSE.md` read); model files `OFIQ-MODELS.zip` carry their own licence files (listed in the results once downloaded) | repository LICENSE.md read | model licence files must be read before any use beyond evaluation | candidate; evaluation-only |
| MOD-052 | `ofiqpy` 0.2.2 (third-party Python reimplementation of OFIQ) | runs OFIQ without the C++ build | PyPI licence field empty (to read from the sdist) | PyPI page read; conformance claim checked by gate Q1, not assumed | third party; not the BSI reference | evaluation-only |
| MOD-053 | `Helsinki-NLP/opus-mt-en-ur` | machine translation of English captions into Urdu for the retrieval test | Apache-2.0 (hub metadata) | hub API read | translations are unreviewed machine output | evaluation-only |
| MOD-054 | SigLIP 2 base `google/siglip2-base-patch16-224` | image/text embedding candidate | Apache-2.0 (hub metadata) | hub API read | none found | candidate |
| MOD-055 | Omnilingual ASR CTC v2 and 1B checkpoints, LLM variants | Punjabi/Saraiki speech candidates (not fetched) | Apache-2.0 (README and hub tag) | README and `facebook/omniASR-CTC-1B` tag read | the v2 files are served from `dl.fbaipublicfiles.com`, not the hub; LLM variants do not fit the CPU gate | candidate |
| MOD-056 | `facebook/w2v-bert-2.0` | base for a CTC fine-tune (not fetched) | MIT (hub metadata) | hub API read | no Saraiki in its pre-training list | candidate |
| DAT-025 | FairFace validation (1.25 padding) | detection rate and impostor false-match rate by group | CC BY 4.0 | authors' README and hub metadata read | perceived race labels; no identities | evaluation-only |
| DAT-026 | WIDER FACE validation | face detection scoring | cc-by-nc-nd-4.0 (hub metadata) | hub API read | non-commercial, no derivatives: never redistributed, never trained on | evaluation-only |
| DAT-027 | LFW, CFP-FP, AgeDB-30, CALFW, CPLFW verification sets (unofficial mirror `gaunernst/face-recognition-eval`) and LFW originals (scikit-learn figshare mirror) | recognition accuracy | research releases; the mirrors state no licence | files hashed; read through a restricted unpickler | research use only | evaluation-only |
| DAT-028 | Flickr30k 1K test (images + captions) | image-text retrieval | Flickr owners' terms (non-commercial research) | hub listing read | the hub card gives no licence | evaluation-only |
| DAT-029 | Common Voice Saraiki 24.0 (not fetched) | Saraiki speech | CC0 (dataset listing, S) | search summary only; page not rendered | account and terms click by the owner; not to be re-hosted | candidate |
| DAT-030 | MirasAI Saraiki 10 hours TTS dataset (not fetched) | Saraiki speech | CC BY-NC-SA 4.0 (S) | search summary only | non-commercial, share-alike | candidate, development only |

## 10. Verdicts, decision and what the owner must do

| Gate | Statement | Result | Verdict |
|---|---|---|---|
| D1 | YuNet recall of faces >= 40 px >= 80% at <= 0.15 false boxes per image (WIDER sealed) | 82.5% at 0.106 | **MET** |
| D2 | BlazeFace within 5 points of YuNet and >= 5x faster | 15.9% vs 82.5%; about 6x faster | **NOT MET** |
| D3 | Detection rate by race group >= 98%, spread <= 2 points (FairFace) | >= 99.48%, spread 0.40 (YuNet) and 0.45 (BlazeFace) | **MET** |
| R1 | Reproduce published accuracies (0.5 on LFW, 1.0 on the others) | SFace LFW +0.20 met. AuraFace: LFW -0.38, CFP-FP -0.83, AgeDB -0.95, CALFW -0.85 met; CPLFW -1.97 (-1.27 with the flip test) not met | **PARTLY MET** (AuraFace NOT MET on CPLFW) |
| R2 | End-to-end YuNet path within 1.0 point of pre-aligned, <= 0.5% faces lost | SFace 96.62% (-2.98), AuraFace 96.60% (-2.67); 0% lost; centre-face diagnostic SFace 99.33% | **NOT MET** |
| R3 | AuraFace >= SFace + 3.0 points on the four hard sets | -1.25 (93.08% vs 94.33%) | **NOT MET** |
| R4 | AuraFace <= 0.5 s per face and <= 1.5 GB with 4 threads | 274 ms, 847 MB | **MET** |
| R5 | Indian false-match rate <= 3x White and <= 3x pooled at 1e-4 (upper bound <= 5) | ratios 0.99 and 1.02; upper bounds 1.91 and 1.89 | **MET** (both models) |
| R6 | Synthetic-proxy false non-match, Indian <= White + 2 points | Indian lower than White for both models | **MET, proxy only** |
| Q1 | `ofiqpy` reproduces the C++ conformance results | 784 of 784 component scores identical | **MET** |
| Q2 | Score for >= 95% of FairFace images, <= 2 s per image | 99.8%, median 1.72 s | **MET** |
| Q3 | Spearman quality vs genuine cosine >= 0.3 | 0.542 | **MET** |
| I1 | 100 K rows, 10% filter, iterative scan: recall@10 >= 0.95 and p95 <= 200 ms | best recall 0.857, p95 5 to 9 ms | **NOT MET** |
| I2 | Exact scan inside one case of 100 K faces <= 500 ms | 1,499 ms for 100 K rows; 22 ms for a 1 K-face case; 169 ms for 10 K faces | **NOT MET as worded** (met up to about 30 K faces per case) |
| V-a | SigLIP 2 English R@1 not more than 2 points below v1 | +3.55 | **MET** |
| V-b | SigLIP 2 Urdu R@10 >= v1 + 20 points | +11.33 (15.3% vs 4.0%) | **NOT MET** |
| V-c | Best near-duplicate model sealed recall >= 90% | DINOv2 base 100.0% (precision 100.0%) | **MET** |
| V-d | New default <= 1.0 s per image and <= 2.5 GB | SigLIP 2: 0.29 s, 1.8 GB | **MET** |

**Decision by the pre-registered rules**
* **Face matching:** AuraFace does not become the development candidate (R1 partly, R2 and R3 not met, and it is 13 times slower than SFace for no accuracy gain on this pipeline). **SFace and YuNet stay the baseline.** No model is "cleared for product use": training-data provenance is an open question for counsel.
* **Image embeddings:** the rule asks for V-a to V-d together, and V-b is not met, so SigLIP 2 is not proposed on the rule alone. What the numbers support, for the owner's decision: SigLIP 2 is better than v1 for English meaning search at the same speed (+0.5 GB), **neither handles Urdu**, and **DINOv2 base is the better model for near-duplicate and "same photograph" search**.

**Findings that change the design**
1. The configured SFace threshold (cosine 0.363) lets about 1 in 107 pairs of different people through (1 in 62 among Indian faces): too permissive for any lead list. The owner must choose a false-match rate; at 1 in 10,000 the SFace threshold is cosine 0.669, at 1 in 1,000 it is 0.472.
2. A photograph with several faces needs an explicit choice of the face searched; the automatic best-scoring face costs about 3 points of pair accuracy.
3. Per-case exact search (or per-case partitions) gives complete answers; a global HNSW index does not (recall 0.86 with filters) and should only produce a labelled cross-case lead.
4. An OFIQ quality figure is a valid warning beside every result (it tracks matching reliability); no refusal cut-off was calibrated.
5. Urdu text queries need translation to English by the language model before a SigLIP-family search, or a different retrieval model; not measured here.

**What the owner must do:** choose the accepted false-match rate; ask counsel about training-data provenance of the face models; supply consented faces (or approve a request for a consented research set) because the real South Asian genuine-pair error is still not measured; provide native-speaker Urdu reviewers for the caption set; say whether any of the proposed builds (a quality figure, a calibrated threshold, per-case exact search, face-choice step) should start behind a switch.
