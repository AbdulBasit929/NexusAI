# Face and image search bake-off: pre-registration (2026-11-19)

**Written before any model was run on any of the data below.** The candidates, data, partitions, protocols and gates are fixed here; a change is made only as a dated amendment at the end (never an edit). Results go in `face-image-search-results-20261119.md`. Research behind the choices: `docs/research/FACE_IMAGE_SEARCH_RESEARCH_20261119.md`. Owner direction of 2026-11-19: research the best online options, then implement and measure (`project_media_tracks_research_directive_20261119`).

## 1. Rules

* Development use only. Model and weight licences are not a blocker for development (owner, 2026-11-17), but every licence statement and every training-data statement is recorded, and **no model is called "cleared for product use"** until counsel has given a view on training-data provenance (open question, owner).
* Real faces: only public, licensed or synthetic data in this bake-off. No consented real set of South Asian identities exists yet, so **the genuine-pair (false non-match) performance on real South Asian people is NOT MEASURED here** and is reported as such (section 7).
* A face match is a lead, never an identity. Nothing here stores or shows a name. Embeddings computed for the bake-off stay in the data folder (`C:/tmp/nexusai-media-data`, outside the repository) and are not loaded into any case.
* One heavy job at a time, in a memory-capped container without network (`--network none`), with the host watchdog (kill only the test container when host available memory is below 600 MB), as `tools/media_data/Run-OcrMeasure.ps1` does. No docker, compose or git command is run by me while a timed arm runs.
* Sealed rows are read once per model. Timings use medians (the laptop sleeps and corrupts maxima).
* Nothing is deployed, no role is enabled, no evidence is deleted, nothing is committed by this work.

## 2. Candidates (every claim marked P = read on the primary page, S = secondary)

| Role | Candidate | Licence as stated | File / version to be hashed at fetch |
|---|---|---|---|
| Detection | **YuNet 2023mar** (OpenCV Zoo) | MIT (P, LICENSE read) | `face_detection_yunet_2023mar.onnx`, already on disk |
| Detection | **BlazeFace short range** (Google MediaPipe) | Apache-2.0 (P: Google model card PDF, "LICENSED UNDER Apache License, Version 2.0"); the card lists **"any form of surveillance or identity recognition is explicitly out of scope"**, faces smaller than 20% of the image side and beyond 2 m as out of scope (P) | `blaze_face_short_range.tflite` from `storage.googleapis.com/mediapipe-models`, to be hashed |
| Recognition | **SFace** (OpenCV Zoo, MobileFaceNet + SFace loss) | Apache-2.0 (P, LICENSE read); training data not named in the zoo README; the paper (arXiv 2205.12010, P abstract) trains on CASIA-WebFace, VGGFace2 and MS-Celeb-1M and does not say which one the zoo file uses | `face_recognition_sface_2021dec.onnx` (38,696,353 bytes per the hub header, P); its SHA-256 is compared with the product value `FORENSIC_FACE_MODEL_SHA256=9ce78d4b...` read from the live worker's environment |
| Recognition | **AuraFace-v1 recogniser only** (`glintr100.onnx`, 261 MB) | Apache-2.0 (P, card + LICENSE.md); "trained on a commercial dataset ... from various sources", "may not extensively cover all ethnicities" (P). The four InsightFace-named auxiliary files in the same repository are **not used** (licence unverified). | on disk since 2026-11-16, revision folder `af6d057c9b0e`, hashed again |
| Quality | **OFIQ** (BSI, ISO/IEC 29794-5 reference) | code MIT (P: `LICENSE.md` of the repository); model files come from the ISO portal / eu-LISA `OFIQ-MODELS.zip` (422,400,019 bytes, P: HTTP header) and carry **their own licence files, read after download** (P: LICENSE.md says so). Run through the third-party Python package `ofiqpy` 0.2.2 (PyPI page P: claims 787/787 conformance with the C++ tool; **checked here by gate Q1, not assumed**) | model and package hashes recorded at fetch |
| Retrieval | SigLIP v1 base `google/siglip-base-patch16-224` (the product's, revision `7fd15f06...`), SigLIP 2 base `google/siglip2-base-patch16-224` (Apache-2.0, P: hub metadata), DINOv2 base (Apache-2.0, S) | | revisions and hashes recorded at fetch |
| Machine translation of captions | `Helsinki-NLP/opus-mt-en-ur` | Apache-2.0 (P: hub metadata) | |

Blocked for the product and **not measured here**: InsightFace `buffalo_l`/`antelopev2`, SCRFD, EdgeFace, AdaFace weights (non-commercial; see the research note). They may be added as lab-only accuracy references by amendment.

## 3. Data (licences as the primary pages state them)

| Id | Set | Source | Licence | Use |
|---|---|---|---|---|
| F1 | **WIDER FACE validation** (3,226 images, complete box annotation with blur / pose / occlusion / invalid flags) | hub `CUHK-CSE/wider_face` (`WIDER_val.zip` 362,752,168 bytes, `wider_face_split.zip`) | **cc-by-nc-nd-4.0** (P: hub metadata) | detection only; evaluation, never training; never redistributed |
| F2 | **FairFace validation, padding 1.25** (10,954 images; labels race = East Asian / Indian / Black / White / Middle Eastern / Latino_Hispanic / Southeast Asian, gender, age) | hub `HuggingFaceM4/FairFace` parquet `1.25/validation-...` (236,824,583 bytes) | **CC BY 4.0** (P: hub metadata and the authors' GitHub README) | detection rate by group; impostor false-match rate by group (no identity labels exist, so all cross-image pairs are treated as different people; a few duplicates of one person would raise every group's FMR alike) |
| R1 | **LFW, CFP-FP, AgeDB-30, CALFW, CPLFW** verification sets, 112x112 aligned, 10 folds (6,000 / 7,000 / 6,000 / 6,000 / 6,000 pairs) | hub `gaunernst/face-recognition-eval` (`*.bin`, about 70 MB each; an unofficial mirror, no card, S) | the sets are research-only releases (S); the mirror states none | recognition accuracy; **the five numbers on AuraFace's card are the reproduction target** (gate R1). The `.bin` files are pickles from an untrusted mirror: read only with a restricted unpickler (numpy array reconstruction only) inside a container without network |
| R2 | LFW original images and the standard `pairs.txt` | University of Massachusetts LFW page (S) | research use, no licence text (S) | end-to-end path with YuNet detection and alignment (gate R2) |
| V1 | **Flickr30k 1K test** (1,000 images, 5 English captions each) | hub `nlphuji/flickr_1k_test_image_text_retrieval` | Flickr images under their owners' terms, non-commercial research (S); the hub card gives no licence | text-to-image retrieval |
| V2 | Urdu captions of V1: **machine-translated** with opus-mt-en-ur, unreviewed | made here | Apache-2.0 model; the images keep V1's terms | Urdu retrieval; **the captions are not checked by a native speaker (limit stated in every result)** |
| V3 | Near-duplicate set: the Open Images photos already on disk (CC BY 2.0, DAT-018) with the 13 transformations of `tools/media_data/image_eval.py` | on disk | CC BY 2.0, attribution kept in the manifest | image-to-image retrieval |
| I1 | Face-embedding vectors for the index test: 10,954 real AuraFace embeddings from F2 for the 10 K tier; synthetic expansions of them for 100 K and 1 M (each synthetic vector = a real vector plus Gaussian noise at the within-class spread measured on R1, re-normalised; parameters printed in the result) | made here | | index recall and speed; **the larger tiers are synthetic expansions: they test the index, not real-world embedding geometry** |

**Partition rule** (as in earlier programmes): `dev` rows are those whose SHA-256 of the key (file name) ends in a hex digit in the dev set; the rest are `sealed` and are read once per model. F1: dev = last hex digit in `{0,1}` (about 12%, thresholds are chosen here), sealed = the rest. V1: dev = `{0,1,2,3}` (about 25%), sealed = the rest. R1 and F2 have no dev/sealed split (public benchmark pairs; one global threshold rule below), which is why gate R1 is a reproduction check and not a tuning set.

## 4. Protocols

**D (detection).** Score threshold of each detector is the highest-recall value with at most 0.1 false boxes per image on F1 dev; it is then applied once to F1 sealed. Matching: greedy by score, IoU >= 0.5, one detection per labelled face. A detection that matches no valid face and overlaps no `invalid` face (IoU >= 0.3) is a false box. Recall strata: width >= 40 px (the gate class), 20 to 40 px, under 20 px, heavy blur (blur = 2), atypical pose (pose = 1), heavy occlusion (occlusion = 2). Faces flagged `invalid` are ignored. On F2: a hit is a detection whose centre lies within the central quarter of the image (the faces are chip-cropped and centred); per race group. CPU time per image (median) and resident memory are recorded with 4 threads.

**R (recognition).** Cosine similarity of L2-normalised embeddings. *SFace* is run through `cv2.FaceRecognizerSF` (its own pre-processing); *AuraFace* through onnxruntime with RGB, (x - 127.5) / 127.5, 112x112, the ArcFace template. On R1: 10-fold accuracy as in InsightFace's verification code (threshold chosen on nine folds, tested on the tenth; mean and standard deviation), plus true accept rate at false accept rate 1e-3 on the pooled impostor pairs of the five sets with a 95% bootstrap interval over pairs. **Operating thresholds** are taken from F2: the cosine value at which the pooled impostor false-match rate over all cross-image pairs of all race groups equals 1e-3 and 1e-4 (about 60 million pairs, so 1e-4 is measurable). At each threshold: false-match rate within each race group (pairs inside the group), with 95% bootstrap intervals over images; false non-match rate on R1 genuine pairs per set; false-match rate on R1 impostor pairs per set. **Synthetic genuine proxy for South Asian faces (limit stated):** for every Indian and every White image of F2, a degraded copy (JPEG quality 30, 50% down-scale then back, Gaussian blur sigma 1.5, brightness x0.7) is the "second photo"; the false non-match rate of original-vs-degraded measures robustness to degradation, **not** the variation between two real photos of one person, so it is an optimistic proxy and is labelled so. **End-to-end path (R2):** LFW original images, YuNet 5-point detection, `alignCrop`, then each embedder, with the standard 6,000 pairs.

**Q (quality).** OFIQ `UnifiedQualityScore` and the component scores on F2 (Indian and White subsets, originals and degraded copies) and on a sample of R2 images; the fraction of images where OFIQ returns "failure to assess"; CPU time and memory; conformance of `ofiqpy` against the C++ expected results of the BSI conformance images (`expected_results.csv`, P: in the repository).

**I (index).** Postgres 16 with pgvector (the version in the project image, recorded) in a throw-away container, memory-capped. Table `faces(id, case_id, embedding vector(512))`, HNSW cosine index (m = 16, ef_construction = 64), queries with `WHERE case_id = X` for case sizes of 1%, 10% and 100% of the table, at 10 K, 100 K and (memory permitting) 1 M rows. Measured: recall@10 against exact search (sequential scan) at `ef_search` 40 and 200, with `hnsw.iterative_scan` off, `relaxed_order` and `strict_order`; build time; index size; p50 and p95 latency; and the latency of an **exact scan inside one case** (the way to return *all* matches above a threshold). Partitioning by case is tested at 100 K.

**V (retrieval).** Text-to-image recall@1/5/10 on V1 sealed in English and in the machine-translated Urdu (V2), for SigLIP v1 base and SigLIP 2 base; image-to-image near-duplicate retrieval on V3 with the protocol of `image_eval.py` (precision >= 99.5% on dev fixes the threshold; sealed read once) for SigLIP v1 base, SigLIP 2 base and DINOv2 base. CPU time per image and per caption (4 threads) and resident memory.

## 5. Gates (all are pre-registered; a verdict is "met" or "NOT met", never softened)

**Detection**
* **D1** YuNet: recall of faces >= 40 px wide on F1 sealed >= 80% with at most 0.15 false boxes per image realised (threshold chosen on dev for 0.1). *This is the figure of the 2026-10-21 programme; the earlier false-box result could not be trusted because Open Images labels are incomplete, which WIDER FACE does not suffer from.*
* **D2** BlazeFace is "usable for video frames" only if its recall of faces >= 40 px at the same false-box rate is within 5 percentage points of YuNet's **and** it is at least 5x faster per image. Otherwise "not adopted". Independently of the result, its card's out-of-scope statements are reported to the owner.
* **D3** On F2, detection rate for every race group >= 98% and the spread between the best and worst group <= 2 percentage points, for each detector.

**Recognition**
* **R1 (reproduction, a precondition)** My AuraFace accuracy is within 0.5 percentage point of the card on LFW and within 1.0 point on CFP-FP, AgeDB, CALFW and CPLFW (card: 99.650, 95.186, 96.100, 94.700, 90.933), and SFace's LFW accuracy is within 0.5 point of the zoo's 0.9940. If this fails, the pipeline is investigated, and until it passes every other recognition number is reported as "unreliable".
* **R2 (end-to-end path)** With YuNet detection and alignment on LFW originals, each model's accuracy is within 1.0 point of its pre-aligned accuracy, and no more than 0.5% of images lose their face.
* **R3 (AuraFace advantage)** Mean accuracy over CFP-FP, AgeDB, CALFW and CPLFW is >= SFace's + 3.0 percentage points.
* **R4 (cost)** AuraFace median embedding time <= 0.5 s per face with 4 threads, resident memory <= 1.5 GB.
* **R5 (demographic differential, impostor side)** At the FMR 1e-4 operating threshold, the false-match rate inside the Indian group is <= 3x the rate inside the White group and <= 3x the pooled rate, point estimate, with the upper 95% bound <= 5x. Reported for both models, both thresholds.
* **R6 (South Asian genuine proxy)** At the 1e-4 threshold the synthetic-proxy false non-match rate for Indian faces is no more than 2 percentage points above that of White faces. Even if met this is **not** a claim about real South Asian people.

**Quality**
* **Q1** `ofiqpy` reproduces the C++ conformance results (status equal on every image, UnifiedQualityScore equal within 1 point) on the BSI conformance images. If the images cannot be fetched, Q1 is "not verified" and every OFIQ number below is labelled so.
* **Q2** OFIQ returns a score (not "failure to assess") for >= 95% of F2 images and runs in <= 2 s per image with 4 threads.
* **Q3** OFIQ's score is informative about matching: Spearman correlation between the quality score of the degraded copy and its genuine cosine score is >= 0.3 for AuraFace (Indian and White pooled).

**Index**
* **I1** At 100 K rows, filtered queries at 10% selectivity with iterative scan on have recall@10 >= 0.95 against exact search and p95 latency <= 200 ms.
* **I2** The exact scan inside one case of 100 K faces finishes in <= 500 ms (so "all matches above a threshold" can be exact for cases of that size). If I2 is not met, completeness needs another design and the result says which.

**Retrieval**
* **V-a** SigLIP 2 base text-to-image recall@1 in English is not more than 2 points below SigLIP v1 base (no regression).
* **V-b** SigLIP 2 base Urdu (machine-translated captions) recall@10 is >= SigLIP v1 base + 20 points.
* **V-c** For near-duplicate retrieval at the 99.5%-precision threshold, the best of the three has sealed recall >= 90%; the model with the best recall is named.
* **V-d** The model proposed as the new default runs in <= 1.0 s per image with 4 threads and <= 2.5 GB resident memory.

**Decision rule.** AuraFace becomes the *development candidate for face matching (level L1)* only if R1 to R5 are met (R6 is reported alongside). Otherwise SFace stays the baseline and the result says why. SigLIP 2 or DINOv2 is *proposed* as image-embedding model only if V-a to V-d allow it. Neither is enabled or deployed by this work; adoption is the owner's word, and "cleared for product use" still waits for counsel.

## 6. Procedure and order

1. Fetch models and data (public, ungated; hashes into the manifest `C:/tmp/nexusai-media-data/manifest-20261119-faces.jsonl`; licence rows for the ledger are drafted, not edited, in the results note).
2. R1 reproduction on LFW first (both models). If it fails, stop and fix the pipeline before anything else.
3. D on F1 dev (threshold), then F1 sealed once per detector; F2 detection.
4. R on R1 sets, F2 embeddings, operating thresholds, group FMR, proxy FNMR.
5. Q (OFIQ).
6. I (index) in a separate container, one at a time with the others finished.
7. V (retrieval).

## 7. What this cannot show (stated in advance)

* No identity-labelled South Asian faces: real genuine-pair performance on South Asian people is **not measured**; the owner needs to supply consented faces or approve a request for a consented research set (for example a gated, consented video set whose terms need the owner's account).
* F2 labels are the dataset authors' perceived race categories; "Indian" there means South Asian appearance, not nationality.
* The public verification sets are mostly Western celebrity photographs; accuracy on them does not transfer to CCTV stills.
* The 1 M index tier uses synthetic expansions of real embeddings.
* Training-data provenance of every face model remains an open legal question.

## 8. Amendments

**A1, 2026-11-19, before any FairFace file was embedded or scored and before any AuraFace number existed (only SFace on LFW had been seen: 99.60% 10-fold accuracy, a reproduction check).** *The operating threshold is taken on the pooled WITHIN-GROUP impostor pairs, not on all cross-image pairs.* Reason: pairs of two different groups are much easier to tell apart than pairs inside a group, so a threshold calibrated on all pairs makes every within-group rate larger than its target and turns the "pooled rate" comparison of R5 into a comparison with a diluted number. NIST's demographic reports also use same-group impostors. Definitions as fixed from now on: the **pooled rate** of R5 is the target itself (1e-4 or 1e-3), reached by construction on the pooled within-group pairs of the seven groups; the threshold that would give the target inside the Indian group alone is also reported, so the size of the calibration shift is visible. Everything else in section 4 and the gates stand.

**A2, 2026-11-19, same time.** (a) The score thresholds used on FairFace for the detection rate (D3) are the ones chosen on WIDER dev for each detector; the faces that feed the recognisers use YuNet at score 0.6 (the OpenCV default), fixed now. (b) Embeddings are single (no flip test) for all arms; the flip test is run only on a set where a model misses its reproduction target by more than the stated tolerance, and is then reported as a separate arm. (c) The `.bin` pickles contained only numpy array reconstruction calls (restricted reader log, 2026-11-19); counts of unique images are 7,701 (LFW), 5,890 (CFP-FP), 5,296 (AgeDB-30), 6,098 (CALFW) and 6,595 (CPLFW): each unique image is embedded once.

**A3, 2026-11-20, written AFTER gate R2 was measured and missed (post hoc, disclosed; the R2 verdict below is not changed by it).** *What was seen:* the end-to-end path on the original LFW images (YuNet, highest-scoring face, alignCrop, 6,000 pairs, 0 faces lost) gave 96.62% (SFace) and 96.60% (AuraFace) against 99.60% and 99.27% on the benchmark's own aligned crops: about 3 points lower for both models, so **R2 (within 1.0 point) is NOT met**. *Why a diagnostic is added:* both models lose the same amount, which points at the shared detect-and-align stage, and LFW images often show several faces while the benchmark target is the centred one. *Diagnostic arm "R2b" (not a gate):* the same pairs with the face nearest the image centre among those with score at least 0.6 (`--select centre`), SFace first and AuraFace if time allows. It is reported next to R2 as an explanation, never instead of it. If it recovers the loss, the product rule for choosing a face in a multi-face image becomes a design item (the analyst, not a rule, must pick the face of interest in a photo of several people).

**A4, 2026-11-20, written AFTER the Marian machine translation was run and its output read, before any Urdu retrieval number existed.** *What was seen:* `opus-mt-en-ur` (protocol V2) produced unusable captions: for example "The man with pierced ears is wearing glasses and an orange hat" came out as a sentence about a man being placed on an open flower, and longer captions degenerate into repeated words; a retrieval test on them would measure the translation, not the retrieval models. *Change:* the Urdu queries for gate V-b are **150 captions translated by the assistant (Claude) from the first English caption of 150 sealed images** (images chosen by SHA-256 order among sealed captions of at most 140 characters), **not reviewed by a native speaker**; the 150 queries are searched against **all 749 sealed images**, and the same 150 images' first English captions are scored against the same gallery as a like-for-like control (gate V-b compares the two models on the Urdu queries; the English control shows how much of any gap is language and how much is the 150-caption sample). The Marian output stays in `work-faces/flickr1k/captions-ur.json` as a recorded negative result. The 1,000-image English test (V-a) is unchanged.

