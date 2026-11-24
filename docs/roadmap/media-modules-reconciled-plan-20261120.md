# Media modules: reconciled plan (2026-11-20)

Purpose: put side by side (1) what the product already does, read from the code and database today, (2) what the roadmap and the owner's decisions say, and (3) what the bake-off of 2026-11-19/20 measured, and from that fix **one build order with done-criteria per module**. Owner's direction of 2026-11-20: no licence purchase, no long model training, each module built, accepted and deployed before the next; use online-available data because no consented faces exist. Nothing here is built or deployed yet; every deploy still needs the owner's explicit word. Evidence for every number: `docs/data/face-image-search-results-20261119.md` (called "results note" below). Claims marked **(S)** are search-result summaries not yet checked on a primary page.

## 1. Reconciliation: product, roadmap, measurement

| Family | What the product has today (read from code/DB) | What the roadmap / decisions say | What was measured | Gap |
|---|---|---|---|---|
| **Faces: detection** | `LocalAIFaceProcessor` (`media_pipeline.py`) writes one `face_detection_observation` per face (box, crop artifact, detector/model hashes, `review_state: model_candidate`) through the LocalAI face backend (YuNet + SFace packaged as a GGUF, sha 9ce78d4b...). **Switch `FORENSIC_FACE_ENABLED=false` in the live worker.** Quality stored = crop size only | D34: L0 on; L1 clustering now; L2 search/compare after pre-registered evaluation and legal gate | YuNet 82.5% of faces >= 40 px, weak on small/blurred/turned; the GGUF packaging itself was **not** measured (ONNX file was) | verify the GGUF = ONNX equivalence; improve small-face recall by engineering |
| **Faces: matching** | The same processor stores a **candidate embedding inside the observation payload** (`embedding`, `embedding_status`). Migration 015's header says face embeddings are **not** stored in `content_embedding` by design (policy L0). **No governance code exists** (no `face_search` permission, no per-case authority, no search log, no kill switch: grep of `api/` finds none; they are only in the roadmap) | D34 (b); embedding route "AuraFace after counsel, or a bought InsightFace licence"; evaluation on a **consented** South Asian set before L2 | SFace >= AuraFace on 4 of 5 public sets, 13x faster; false-match differential Indian vs White ~1.0; default threshold 0.363 far too permissive; centre-face rule needed | the embedding route is now **SFace** (no purchase, AuraFace not better); the consented-set gate cannot be met: replace by a public-figure set + degradation for **development** acceptance, keep counsel/owner as the gate for L2 on real cases; build the governance first |
| **Image near-duplicates** | `image_compare.go`: dHash-64 distance <= 10 between **two named** evidence items; `image_similarity.go`: pairwise semantic similarity **inside a named set of <= 200 evidence items**, top-K <= 50, from SigLIP v1 embeddings stored in observation payloads | §5.1: PDQ + pHash clusters, within-case and opt-in cross-case "seen before" | DINOv2 base: 100%/100% sealed recall/precision; SigLIP v1/2: 99.8% recall but 95.6/96.2% precision at the same rule; PDQ and dHash-64: 100% precision but only 74% recall (they miss crops, mirrors, rotations) | no case-wide search: both endpoints need the caller to name the evidence |
| **Image semantic search** | `SigLIPImageEmbeddingProcessor` (v1 base, 224 px) writes `semantic_image_embedding_observation`; **off** (`FORENSIC_IMAGE_EMBEDDING_ENABLED=false`). `forensic.content_embedding` (migration 015, applied 2026-10-13, empty then; I did not re-check today) has `modality in (text, image)`, **no similarity index** ("exact search over a case's vectors is fast enough until then") | §5.1 item 8: SigLIP in pgvector HNSW, text-to-image and image-to-image, "candidate, review required" | SigLIP 2 +3.6 R@1 over v1 in English; both fail Urdu (15%/4% top-10); exact per-case scan is fast (22 ms for 1 K, 169 ms for 10 K rows), global HNSW with filters tops out at recall 0.86 | the vectors are not in `content_embedding`; no search endpoint over a whole case; no Urdu route; Ask cannot reach any of it |
| **Ask engine** | Media views exist for OCR, speech, plates, video, audio files (migrations 023/035/036/037). Migration 023's header **deliberately excludes near-duplicates and faces** "until measured" | exhaustive, runtime-LLM search across all sources (owner, 2026-11-19) | near-duplicate measurement now exists | Ask needs a governed vector-search route; **the Ask files belong to another session**: this plan only defines the contract |
| **OCR, plates, video plates** | live (EasyOCR role, plate engine, V4 video) | done | done earlier | none for this plan |
| **Speech Punjabi/Saraiki** | Omnilingual CTC 300M copy: Punjabi `LOW_RESOURCE_LANGUAGE`, Saraiki unsupported | tiers and bake-off pre-registered 2026-11-19 | not measured | needs the bake-off and Saraiki data |
| **Video beyond plates** | probe/proxy/older 12-frame path; faces off | §6.2 stages | not measured | pre-registered, not built |

## 2. What changes from the earlier plan (and why)

1. **Embedding route for faces = SFace** (already in the product, Apache-2.0 label, training data unresolved); AuraFace and a bought licence are dropped. The lab ceiling run with non-commercial models is dropped (it only helps a buy/train decision).
2. **The consented South Asian set cannot exist now.** For development acceptance use: (a) a public-figure set built from openly licensed photographs (Wikidata for the identities, Wikimedia Commons for >= 2 photos per person, licence and attribution recorded per file, kept on this machine, not redistributed); (b) the same photos under CCTV-style degradation; (c) FairFace for false-match rates by group. This is labelled "public-figure development set" in every result and does **not** replace counsel's view or the owner's gate for real casework. Gated consented sets (RFW, Casual Conversations; **(S)**, my recollection) stay a parallel request.
3. **Face embeddings leave the observation payload** for a dedicated per-case table (below), so that access can be controlled, logged and killed; today's payload copy would travel with every export of the observation.
4. **Search is exhaustive and per case**: exact scan inside the case (22 ms per 1 K faces, about 30 K faces per 500 ms), per-case partitions beyond that; a global HNSW index is only for an explicit cross-case "seen anywhere" lead with its measured recall shown.
5. **Quality is a warning band, not a refusal:** with SFace, the false non-match rate for probes of OFIQ unified score 0 to 9 is 9.6% at 1e-4 (260 of 599 degraded probes), against 0 to 2.5% for scores 10 and above (`work-faces/results/ofiq-bins-sface.json`). No sharp cut-off exists in this synthetic proxy; show "low quality" below 10, recalibrate on CCTV-like probes.

## 3. Design rules from forensic practice (S: summaries of FISWG/SWGDE/NIST material, to verify at the primary documents before they are written into policy)

* A face search result is a **lead for trained human review, never an identification**; a second reviewer should verify, ideally blind to the first decision; model policies cap what is passed on (one example returns at most two images as leads).
* NIST separates **investigation mode** (threshold zero, a fixed-length ranked candidate list, e.g. 50) from **identification mode** (a high threshold that limits the false-positive identification rate, e.g. 0.003). The product should offer both, named: **ranked list (K up to 50)** and **above-threshold list (1e-4 default, 1e-3 "wider")**, each with the stated completeness and the count just below the threshold.
* NIST warns that low error rates come from good cooperative images; probe quality must be shown and results labelled accordingly.
* Every search logged: who, when, probe hash, case, scope, reason, number of results; no embedding export; per-case enablement with a recorded authority; kill switch.

## 4. Build order, one module at a time (each: pre-register gates, build behind a switch, accept on fresh data, show in UI and Ask, deploy on the owner's word with a rollback, then the next)

Effort is my rough size in working days at this session's pace (slowed by the memory squeeze); it is not a promise.

### M0. Foundations (about 2 days; needed by M1 and M2)
* **Measurement window:** unload the language model during heavy runs (owner's command) or run in a quiet period; the runners already checkpoint and wait for headroom.
* **Vector storage:** use `forensic.content_embedding` for image vectors (model id, dimension, hashes); create no global index at first (exact per-case scan); add per-case partitions when a case passes about 30 K vectors. New migration **038** (face tables below) with preflight/verify/rollback and a rehearsal on a restored copy, as for 037.
* **Face tables (migration 038):** `forensic.face_embedding` (case, evidence, box, SFace 128-d vector, OFIQ score, model hash, crop ref) with the same row-level security as other forensic tables; `forensic.face_search_log`; per-case `face_matching_enabled` with authority reference. All **off** by default.

### M1. Image search (about 6 days)
Analyst functions: **find near-duplicates / the same photograph** (two labelled tiers: a PDQ hash match = "same image, re-saved", 81 ms per image, no false matches in the test; a DINOv2 base match at cosine distance <= 0.205 = "same photograph, edited or cropped", 100% recall and precision on the sealed test), **find photos by words** (SigLIP 2; Urdu or other languages translated to English by the language model first), **more like this image**, **seen before in other cases** (opt-in, D41). Every result lists all matches above the threshold with score, thumbnail and citation, plus an explicit completeness line.
* Ingest: DINOv2 base (0.33 s/image) + SigLIP 2 (0.29 s/image) in the worker overlay, one at a time, models released after each batch (about 0.6 s per image, so 10,000 images take about 1.7 hours in the background; worker memory +1.8 GB while loaded).
* API: one governed endpoint for case-wide search (replaces the "name your evidence ids" limit of today); Ask contract: a table-valued search function the Ask engine may call and cite (to hand to the Ask session).
* **Gates (to pre-register before building):** near-duplicate recall >= 95% at precision >= 99% on a **fresh** photo set that includes WhatsApp-style re-compressions and screenshots; English semantic R@10 on a fresh set not more than 2 points below today's 97.2% (sealed Flickr); Urdu via translation R@10 at least 80% of the English figure on a native-reviewed caption set (the owner's reviewers, D10); time per image within the budget above; search latency <= 1 s at 30 K images; zero cross-case leakage in a role test.
* Dependencies: nothing from the owner except the deploy word; Urdu review set (D10) for the last gate.

### M2. Faces v1 (about 12 days; starts after M1 is deployed or in parallel only on the design)
Analyst functions: faces detected and cropped at ingest with a quality figure; **within-case face groups** ("may show the same face", strict threshold 1e-4, analyst merges/splits); **search a face**: the analyst picks the face in a photo (a required step when several faces are found) or uploads a probe, chooses ranked list or above-threshold mode, gives a reason, and gets all results with score, quality, count just below the threshold, and the "lead, not identity" label; second-reviewer confirmation; audit log; kill switch.
* Detection: YuNet; first try upscaling/tiling for small faces on the public sets (pre-registered); if it does not help, the feature says it works on clear faces.
* Matching: SFace 128-d, thresholds from the owner's rate (1e-4 cosine 0.669; 1e-3 cosine 0.472, re-checked on the new set); OFIQ on the face crop with margin (to be verified on crops, since OFIQ expects a portrait).
* **Gates (pre-registered on the public-figure set, sealed part read once):** genuine-pair true accept rate at the 1e-4 threshold >= the pre-registered floor after the CCTV degradation; false-match rate on impostor pairs <= 2x the target; group purity >= 95% (pairwise precision) at the strict threshold; search completeness 100% against an exact oracle; latency <= 1 s at 30 K faces; detection recall of degraded faces >= 40 px at the pre-registered floor; role/authority/log/kill-switch tests pass; no embedding in any export.
* **Not allowed without the owner and counsel:** enabling search on a real case (L2). Building and accepting on public data is allowed now.

### M3. Video beyond plates (about 12 days; U1 first)
U1 overview (motion index, shots, clock OCR, clip export, same-footage), then detection and tracking (RF-DETR/RT-DETR + ByteTrack, Apache/MIT), then appearance search on crops, then integrity indicators; faces in video only after M2's gates. Gates are the ones already pre-registered in `video-evidence-preregistration-20261119.md`.

### M4. Punjabi/Saraiki speech (about 4 days plus data)
Run the pre-registered Omnilingual v2/1B comparison on the unread Punjabi half (needs memory headroom: the language model unloaded), apply the speech lab's tiers, add the telephone-band simulation; Saraiki once audio exists (Common Voice click or team recordings). No fine-tuning unless the owner approves a run.

## 5. Sequence

M0, then M1 (deploy), then M2 (deploy behind its switch, off), with M4's Punjabi bake-off running in the background whenever memory allows, then M3. The Ask hand-off for M1 and M2 is written as soon as each contract is fixed.

## 6. Decisions needed from the owner (minimal)

1. Default false-match rate for face leads: recommend **1e-4 default, 1e-3 "wider search"**, both named development thresholds.
2. OK to store face embeddings in a dedicated per-case table (migration 038, rehearsed first) and keep them out of observation payloads.
3. OK to build the public-figure development set from openly licensed photographs, and to tell counsel it exists.
4. Reviewers for Urdu captions (D10), and the Common Voice click for Saraiki.
5. A deploy word per module, when its gates are met.

## 7. Risks and limits

Public-figure photographs are not CCTV: degradation approximates it but a real consented or authorised CCTV sample remains the only true measure (**not measured**). SFace's training data is unresolved (counsel). The pgvector version in the product is 0.8.1. The memory squeeze on this laptop slows every measurement; a GPU host (D36) would remove it. The Ask engine integration depends on the other session.
