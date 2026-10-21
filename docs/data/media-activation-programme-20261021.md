# Media activation programme (2026-10-21): data, models, decision rules, gates

Owner direction, 2026-10-21: find the best datasets available online, no manual labelling, the questions are written by me, research the models for every media role, switch each role on and make it accurate and operational. This file is the **pre-registration**: the sources, the decision rules and the gates are fixed here **before** the first measurement and are not edited afterwards. Results go in `media-activation-results-20261021.md`. It applies sections 8 and 9 of the master plan (`docs/roadmap/master-plan-20261021.md`) and supersedes its "owner captures and labels" assumption (D26) with public licensed data plus synthetic data with exact truth; real Pakistani evidence (T4) is still the only way to measure real-world accuracy and stays reported as **not measured**.

## 1. Rules (every track)

1. **One role at a time.** Rollback tag of the worker image, the readiness receipt, the switch on for the controlled collection, the measurement, kept on only if the gate passes. A role that fails its gate stays off and the result says why.
2. **Evidence tiers are reported separately** (master plan M0.2): `T0` exact fixtures, `T1` synthetic with exact truth, `T3` public licensed data, `T4` authorised real (none yet). A number on T1 or T3 is never described as a real-world accuracy.
3. **Licences:** only sources whose licence permits our internal evaluation and use: CC0, CC BY, MIT, Apache-2.0, OFL (fonts), CDLA-Permissive. CC BY-NC, CC BY-SA text kept out of the product path (evaluation only if stated), AGPL code and weights out entirely, gated or login-only data out. Every source and model gets a row in `docs/compliance/licence-ledger.csv` **before** it is downloaded. A hub licence tag is the uploader's claim: the row says `to_verify` until the card and source have been read.
4. **Data lives outside the repository** at `C:/tmp/nexusai-media-data/` with a manifest (source URL, SHA-256, bytes, licence, fetch time). A hash mismatch blocks use. Nothing is uploaded to any service. Measurements run in `nexusai-media-eval` (the live worker image plus evaluation libraries, `--network none`, data read-only), never in the live worker.
5. **Partitions:** a sample belongs to `dev` or `sealed` by the last hex digit of the SHA-256 of its id: `0` to `b` is dev (75%), `c` to `f` is sealed (25%). Decisions are made on `dev`; `sealed` is read once per role for the final report and never used to choose or tune.
6. **Model output is `model_observation`**, never a fact; no identification of a person from a face or voice; a plate string is not an owner.
7. **What "on" means:** the role runs for new uploads of its family, writes derived artifacts with model id, hash, parameters, confidence and locator, and every answer that uses them says "model observation". It does not mean the accuracy is enough to act on without review.

## 2. Sources chosen (evidence read 2026-10-21 from the hub APIs and pages; ledger rows follow)

| Role | Source | Licence (stated) | Use | Tier | Size |
|---|---|---|---|---|---|
| A | `google/fleurs`, test splits `ur_pk`, `pa_in`, `ps_af`, `en_us` (read speech with transcripts) | CC BY 4.0, ungated | WER, speed, script, hallucination loops | T3 | a few hundred MB per language |
| A | Whisper family: `openai/whisper-large-v3-turbo` (MIT), `Systran/faster-whisper-{small,medium,large-v3}` (MIT), `deepdml/faster-whisper-large-v3-turbo-ct2` (MIT), `kingabzpro/whisper-large-v3-turbo-urdu` (Apache-2.0), `awaaz-se-alfaaz/whisper-turbo-v3-urdu` (Apache-2.0 LoRA), `BuzzASR/urdu` (MIT) | as stated | bake-off | | 0.5 to 3 GB each |
| A | `speechbrain/lang-id-voxlingua107-ecapa` (Apache-2.0); Silero VAD (MIT, GitHub) | as stated | language id and voice activity | | 86 MB |
| D/O | `PuristanLabs1/urdu-ocr-1M` (MIT, synthetic Nastaliq and Naskh lines), `Ali000007/Urdu-Ocr-Dataset` (CC BY 4.0, synthetic Nastaliq pages), `oddadmix/qaari-0.1-ocr-urdu-news-dataset-small` (MIT) | as stated | Urdu OCR error | T1 (rendered text, scan-like augmentation) | validation splits only |
| D | `naver-clova-ix/cord-v2` (CC BY 4.0, real receipt scans with text) | CC BY 4.0 | English printed OCR on real scans | T3 | test split |
| D | our own generated pages and identifier fixtures (CNIC pattern, IBAN PK with valid checksum, phone, plate, dates) rendered with OFL fonts | ours | identifier exactness, tables | T1 | generated |
| D/O | `PaddlePaddle/arabic_PP-OCRv5_mobile_rec`, `PP-OCRv5_server_det` (Apache-2.0), `PaddleOCR-VL` (Apache-2.0), `ibm-granite/granite-docling-258M` (Apache-2.0), Tesseract `tessdata_fast` and `tessdata_best` (Apache-2.0) | as stated | engine bake-off | | 8 MB to 2 GB |
| I | Open Images V7 test split photos (images CC BY 2.0, annotations CC BY 4.0), 300 photos | CC BY | near-duplicate set built by transformations; EXIF fixtures built by us | T1 on T3 images | about 60 MB |
| I | `facebook/dinov2-small` (Apache-2.0), `google/siglip-base-patch16-224` (Apache-2.0, in the worker), PDQ (ThreatExchange, BSD per its repository; the directory's licence file read before use) | as stated | similarity and hashing | | |
| P, F | Open Images V7 validation and test: **724 + 2,065 images with plate boxes, 3,124 + 9,292 with face boxes**, all images CC BY 2.0 | CC BY | detection recall and false positives (no plate text in this set) | T3 | subsets of 300 to 500 images |
| P | our controlled plate pack (96 images, exact truth, Pakistani formats) and a larger generated pack with exact truth and degradations | ours | exact read, confusions | T1 | generated |
| P | `justjuu/rtdetr-v2-license-plate-detection` (Apache-2.0), `autolane/rfdetr-alpr` (Apache-2.0), RF-DETR Nano to Large (Apache-2.0) | stated; training-data provenance **to verify** | detector candidates | | 170 to 430 MB |
| V | videos composed from the plate pack and Open Images vehicle photos with known motion and exact per-frame truth | ours | track reading, event recall | T1 | generated |
| F | `opencv/face_detection_yunet` (in the worker); `fal/AuraFace-v1` (Apache-2.0 card, kept out unless D5 moves) | as stated | L0 detection only | | |
| X | hostile-input files generated by us (zip bombs, traversal names, polyglots, malicious SQLite, oversized images) | ours | admission safety | T0 | generated |

**Known holes, stated plainly.** No licence-clean public set has Pakistani plates with text, Punjabi in Shahmukhi, Pashto speech with a trusted test transcript at the scale needed, real Urdu scans with line labels, or consented South Asian faces. Those numbers stay `not measured`; the controlled and synthetic sets measure the machinery, not Pakistani field accuracy. The 2021 PLPD paper's dataset and the UTRSet datasets are non-commercial or on request and are not used.

## 3. Decision rules and gates (fixed now)

### D: documents and printed OCR
- **Rule:** per script, choose the engine with the lowest median character error on `dev` among engines that run in at most 8 s per page image on the laptop CPU; ties go to the simpler engine with an Apache-2.0 or MIT licence that runs in the worker's runtime.
- **Candidates:** Tesseract `fast` (current), Tesseract `best`, PaddleOCR PP-OCRv5 (English and Arabic recognisers), granite-docling for page structure.
- **Gate to switch on:** English clean generated pages: character error at most 3% and identifier exact match at least 95%; CORD real receipts: reported (no floor on first measurement, the floor is set to the measured value minus 5 points after the first run and then frozen); Urdu: on only if line character error is at most 15% on the Naskh `dev` lines and at most 25% on Nastaliq; if Urdu fails, the English path goes on and Urdu regions stay unprocessed with the state `UNSUPPORTED_SCRIPT_QUALITY` and the failing number in the results. Every region carries page and box; low confidence is flagged, never dropped.

### I: image forensics
- **Set:** 300 Open Images test photos; per photo the transformations JPEG quality 30 and 70, resize 50% and 75%, centre crop 90% and 80%, rotate 3 and 10 degrees, mirror, brightness and contrast, grayscale, text overlay, a 5% watermark; negatives are all other photos (44,850 pairs).
- **Rule:** for the label "near-duplicate" take the method and threshold with the highest recall at precision at least 99.5% on `dev` negatives; for "similar" the embedding cosine at which precision is 95%. Candidates: PDQ 256-bit Hamming, the worker's current perceptual hash, DINOv2-small, SigLIP-base.
- **Gate:** near-duplicate recall at least 90% on the mild group (JPEG, resize, brightness, grayscale) at precision at least 99.5%; recall on strong transformations reported; EXIF and GPS extracted exactly on 30 built fixtures; embedding at most 2 s per image on CPU; no identity claim anywhere. The role is "candidate, review required".

### A: audio and speech
- **Rule:** lowest word error on `dev` per language with the written normalisation (Unicode NFKC, punctuation stripped, Arabic-Indic digits mapped, whitespace collapsed), among models with a real-time factor at most 1.5 on the laptop CPU with int8; ties to the smaller model. Candidates: faster-whisper small (current), medium, large-v3-turbo, the Urdu fine-tunes if they can be run through CTranslate2 or the transformers path without leaving the licence set.
- **Gate:** Urdu word error at least 20% relatively below the current small model on the same `dev` clips; English no worse than current; correct script at least 99% of segments; repetition loops at most 1% of clips; every segment with start and end. Punjabi (Gurmukhi in FLEURS, not Shahmukhi) and Pashto are measured and reported; they go on only with their number shown in the interface and the state `LOW_RESOURCE_LANGUAGE`.

### P: plates on images
- **Rule:** detection by recall at IoU 0.5 on Open Images plate `dev` images, then false boxes per image, then latency; reading by exact match on the generated pack with its exact truth, then by confusion class (O and 0, M and W). The licence outcome decides before the numbers: a detector or reader whose weights licence cannot be established is out of the product path, whatever its score (master plan D31).
- **Gate:** detection recall at least 80% at no more than 0.3 false boxes per image on Open Images `dev`; exact read at least 90% on the generated pack's clean partition with calibrated confidence (confidence at least 0.9 means accuracy at least 95%); always "model candidate, review required"; never linked to an owner. If the current YOLOv9-t and CCT-XS weights cannot be licensed, they run only as an internal baseline and the role ships on the replacement.

### V: video
- **Rule:** per-track exact read on generated videos with per-frame truth, then event recall, then real-time factor; no unbounded model call.
- **Gate:** track-level exact read at least 85% on the generated clean partition; false plates per hour reported; every claim with a frame and box; resource caps unchanged.

### F: faces (L0 only; decision D5)
- **Gate:** detection recall at least 80% for faces at least 40 pixels wide at no more than 0.1 false boxes per image on Open Images face `dev` images; no embedding stored; the similarity route stays off; no recognition claim.

### X: captures, databases, archives
- **Gate:** every hostile file is rejected or inventoried inside the limits with no execution, no path traversal and no unbounded resource use; exact inventory on valid fixtures.

## 4. Questions (written by me against the controlled truth)

At least 20 questions per family, each with its expected answer computed from the generated truth (counts, which file, which time, which language, which identifier), plus the declines that must stay declines (a plate string as owner, a person from a face or voice, a model output as fact). Format and location: `tools/media_data/questions/` (JSON, one file per family, generated by a script from the manifest and truth so they change with the data). They are run through the Ask engine once the media views (migration 023) exist; until then the retrieval path and the derived-artifact specialists answer them.

## 5. Order

D, I (CPU and deterministic), then A (CPU baseline with the real-time rule), P and F (detection on Open Images), V (generated video), X (hostile suite), then the media views and the question bank. Each track: ledger rows, data fetch with manifest, measurement on `dev`, one read of `sealed`, switch on or not, results recorded.
