# M1 image search: pre-registration (2026-11-20)

Written **before** the module is built and before any fresh photo is looked at. It fixes the models, the fresh data, the transformations, the protocol and the gates for module M1 of `docs/roadmap/media-modules-reconciled-plan-20261120.md`. Every number in the results comes from the product's own code path (the worker processors and the database search), not from a separate evaluation script, so that what is measured is what ships. Amendments are added at the end, dated; a gate that is missed stays missed.

Owner's direction of 2026-11-20: no licence purchase, no long training; build each module, accept it on measurements, then deploy on the owner's word. Nothing here deploys anything.

## 1. What M1 builds (the object under test)

1. **Ingest** (worker, behind its own switch `FORENSIC_IMAGE_SEARCH_ENABLED`, default off): for every image (and later every video frame) three descriptors, computed one model at a time so the worker's memory stays bounded:
   * a 256-bit **PDQ-style perceptual hash** and the 64-bit dHash (own re-implementation of the published algorithm, labelled "PDQ-style"; thresholds calibrated here);
   * a **DINOv2 base** image vector (`facebook/dinov2-base`, CLS token, 768 numbers, L2 normalised; Apache-2.0);
   * a **SigLIP 2 base** image vector (`google/siglip2-base-patch16-224`, 768 numbers, L2 normalised; Apache-2.0).
   The vectors go to `forensic.content_embedding` (model id, hashes, dimension); the hashes to a new table `forensic.image_hash` (migration 038); the observation stored in `derived_artifacts` carries the model identity and a pointer, **not** the vector.
2. **Search** (API, exact scan inside one case): *same image, re-saved* (hash tier), *same photograph, edited or cropped* (DINOv2 tier), *photos by words* (SigLIP 2 text tower, query translated to English by the language model first when it is not English), *more like this image* (ranked SigLIP 2 / DINOv2 neighbours), each with a **coverage line** (images indexed of images in the case) and a search log row.
3. **Not in M1**: cross-case "seen before" search (needs the D41 visibility rules; designed in the plan, built with M2's governance), face search, video frames beyond the same descriptors.

## 2. Fresh data (none of it was used for any earlier decision)

All from COCO 2017 validation (`images.cocodataset.org`; photographs from Flickr under Creative Commons licences, the per-photo licence id is kept in the derived manifest; captions CC BY 4.0). Evaluation on this machine only; nothing is redistributed. SHA-256 of both downloaded archives is written to `manifest-20261120-image-search.jsonl`.

| Set | Selection (fixed now) | Use |
|---|---|---|
| **ND600** | the first 600 COCO val photos ordered by `sha256("m1nd" + image id)` | sources of the near-duplicate test |
| **D1000** | the next 1,000 in the same order | distractors (always in the gallery) |
| **SEM1000** | the first 1,000 photos, ordered by `sha256("m1sem" + id)`, that are not in ND600 or D1000; their first 5 captions by annotation id | text-to-image test (5,000 queries) |
| **FLK200** | the 200 Flickr 1K test photos that the bake-off used for the Urdu caption check | integrity check S0 and the Urdu development proxy |

Partition: **dev** if the last hex digit of `sha256(image id)` is 0 to b, **sealed** if c to f (as in the earlier near-duplicate test). Thresholds are chosen on dev only; the sealed part is scored once per gate.

## 3. Transformations (fixed list; the code builds exactly these, on the photo as decoded)

*Re-save family R* (what happens when the same file or image travels): `jpeg75` (JPEG quality 75 re-save), `wa_like` (longest side x0.8, JPEG quality 65, metadata stripped; **an approximation of messenger recompression, not real WhatsApp output**), `wa_twice` (`wa_like` applied twice: x0.9, quality 60), `resize50` (50 % Lanczos, JPEG 90), `png_roundtrip` (lossless control: distance must be 0).

*Edit family E* (what an analyst calls "the same photograph, edited"): `crop85` (centre 85 % of each side), `mirror`, `rot5` (5 degrees, black corners), `tone` (brightness x1.25, contrast x0.8), `caption_bar` (black bar with white text over the bottom), `social_frame` (white border of 8 % each side plus a caption strip below, then `wa_like`), `screenshot_tight` (picture shown at 1080 px width, 2 % cut from every edge, JPEG 80: a screenshot cropped to the picture), `crop60_wa` (centre 60 % then `wa_like`).

*Reported only, no gate*: `screenshot_ui` (the picture inside a 1080 x 2340 phone screenshot with status bar, app bar and a message frame). Global descriptors are expected to do poorly when most of the pixels are interface; the result is reported and the help text of the feature says to crop the picture out of a screenshot first.

## 4. Protocol

* **Near-duplicate:** the gallery holds D1000 plus the originals of two thirds of ND600 (rule: `sha256("g" + id)` mod 3 != 0); every transformed copy of every ND600 photo is a query. A query whose original is in the gallery is a *positive* and must return that original; a query whose original is absent is a *negative* and must return nothing. Nearest gallery item within the threshold = an answer. Precision = correct answers / all answers; recall = positives answered correctly / positives. Hash tier distance = Hamming on the 256 PDQ bits; DINOv2 tier distance = cosine distance.
* **Threshold rule:** per tier, the loosest threshold whose **dev** precision is at least 99.5 %. Tier A is tuned on family R; tier B on R and E together.
* **Text-to-image:** the 1,000 SEM1000 photos are the gallery, the 5,000 captions are the queries; R@1, R@5, R@10 by caption, 95 % interval by bootstrap over photos. The same set is scored with SigLIP v1 (the model the product carries today) for the like-for-like comparison.
* **Integrity (S0):** the worker's processors are run on FLK200 and compared with the embeddings produced by the bake-off tools (`image_text_retrieval.py`, `image_eval.py`) from the same files.
* **Speed and memory (T1):** 300 mixed photos through the whole ingest in one container (4 threads, memory limit 3 GB), wall clock per image and peak memory.
* **Latency (L1):** a throwaway Postgres 16 + pgvector 0.8.1 with the real migrations; 30,000 images x two vector models and the hash table; 50 random queries per search type; times measured at the SQL function and at the HTTP handler.
* **Isolation (X1):** two collections in one tenant and a second tenant, with identical photographs in both; searches issued from each as the runtime role.

## 5. Gates

| ID | Gate | Pass condition (sealed unless stated) |
|---|---|---|
| **N1** | Hash tier, family R | precision >= 99.5 % and recall >= 95 % |
| **N2** | DINOv2 tier, families R and E | precision >= 99 % and recall >= 95 % |
| **N3** | DINOv2 tier, each single transformation of R and E | recall >= 80 % for every one (the table is shown whatever the result) |
| **N4** | `png_roundtrip` control | hash distance 0 and cosine distance <= 1e-6 for every photo |
| **S0** | Integrity | for >= 99 % of FLK200 photos the worker's DINOv2 and SigLIP 2 vectors have cosine >= 0.999 with the bake-off vectors, and the PDQ bits are equal for >= 99 % |
| **S1** | Photos by words | SigLIP 2 R@10 on SEM1000 sealed >= 95.2 % (= the 97.2 % of the Flickr bake-off minus the 2 points the plan allows; COCO captions are shorter and its scenes more cluttered, so a miss here is read together with S0) |
| **S2** | Swap justified | SigLIP 2 R@1 exceeds SigLIP v1 R@1 on SEM1000 by >= 1.0 point |
| **U0** | Urdu route, development proxy | reported only: Urdu captions (assistant-translated, **unreviewed**) -> language-model translation -> SigLIP 2 on FLK200; ratio of R@10 to the English figure on the same photos. **The real Urdu gate U1 (ratio >= 80 % on native-reviewed captions, D10) stays NOT MEASURED until the owner's reviewers supply captions.** |
| **T1** | Ingest budget | median <= 1.0 s per image for hash + DINOv2 + SigLIP 2 on this laptop, peak memory <= 2.5 GB above the idle container |
| **L1** | Search latency at 30 K images | p95 <= 1.0 s at the HTTP handler for each of: photos by words, same-photograph search, more-like-this, same-image (hash) search |
| **X1** | No leakage | zero rows from another collection or tenant in any search, including a probe whose evidence id belongs to another case (expected: not found), and a forged collection parameter (expected: refused or empty) |
| **C1** | Coverage line | every response states images indexed and images in the case; with 20 of 25 images indexed it says 20 of 25 |
| **A1** | Audit | every search leaves one `image_search_log` row with who, when, kind, probe hash or query text hash, case, parameters, number of results; and no response carries a raw vector |

Combined behaviour of the two tiers (tier A label first, tier B label otherwise) is reported with its own precision and recall on R and E together; it is not a separate gate.

## 6. Will not be claimed

Real WhatsApp or screenshot output (simulated); phone photographs at full resolution (COCO is 640 px); photographs of the same scene taken a moment apart (COCO has almost none: "same photograph" means same image content, and a burst of similar shots may come back as matches that the analyst must tell apart; stated in the feature); video frames; non-photographic images (documents, screenshots of text); **completeness for photos-by-words**: a description has no sharp edge, so that search returns the most likely matches with scores and the number above a stated score, never "all occurrences" (completeness is claimed only for the hash and DINOv2 tiers, against a threshold); Urdu quality; behaviour at one million images (pgvector tests of 2026-11-19 cover per-case exact scan up to about 30 K).

## 7. Amendments

A1 to A3 were written before any fresh photo was described by a model or scored (2026-11-20, while the archive was still downloading); A4 likewise.

* **A1 (scope).** The 64-bit dHash is **not** stored in M1: it was a bake-off comparator (74.4 % recall, no better than the 256-bit hash), the existing comparison endpoint keeps its own fingerprint, and a second hash adds a column and a threshold nobody uses. Section 1.1 should be read without it.
* **A2 (protocol details that section 4 left open).**
  1. The set called FLK200 is the **150** Flickr 1K test photos that carry the 150 assistant-translated Urdu captions of the bake-off (Amendment A4 of the face and image-search pre-registration); S0 and U0 use those 150.
  2. File formats of the transformed copies are fixed by the code (`make_transforms` in `tools/media_data/image_search_m1.py`): JPEG for `jpeg75`, `wa_like`, `wa_twice`, `resize50`, `social_frame`, `screenshot_tight`, `crop60_wa`; lossless PNG for `png_roundtrip`, `crop85`, `mirror`, `rot5`, `tone`, `caption_bar`, `screenshot_ui`.
  3. Photos by words, "sealed": the queries are the captions of the SEM1000 photos whose partition digit is c to f; the gallery is **all 1,000** SEM1000 photos (so the gallery size matches the 1K protocol of the Flickr bake-off); the interval is a bootstrap over photos. S2 compares SigLIP 2 and SigLIP v1 on exactly the same captions.
  4. The partition digit is the last hex digit of `sha256(str(image id))`; gallery membership is `sha256("g" + str(image id))` modulo 3 is not 0.
  5. Tier thresholds are chosen on dev only: tier A = the loosest Hamming distance whose dev precision over family R queries is at least 99.5 %; tier B = the loosest cosine distance (grid step 0.002) whose dev precision over families R and E is at least 99.5 %. The "combined" row labels a query as tier A when its nearest hash is within the tier A distance, otherwise as tier B when its nearest vector is within the tier B distance.
  6. The sealed part of each gate is written once to `work/m1/results/sealed-<gate>.json`; the script refuses a second read unless `--reread --reason` is given, and the reason is recorded in the result.
* **A3 (descriptor source).** SigLIP 2 base is loaded with the SigLIP classes (the fixed-resolution checkpoint is a SigLIP-architecture model in transformers 4.49, the version in the worker image); the vision tower alone is used for images and the text tower alone for phrases. Checked 2026-11-20 against the full model on one photo and one phrase: image cosine 0.9999998, text cosine 1.0000001, 0.37 s per image. S0 re-checks this on 150 photos.
* **A4 (data source, written 2026-11-20 before any photo was selected, described or scored).** The original host of COCO 2017 (`images.cocodataset.org`) delivered about 12 MB per minute with dropped connections and DNS failures (224 MB of 815 MB in 18 minutes). The fresh photographs and captions are therefore taken from the **COCO 2014 validation Karpathy 5K test split**, as published by nlphuji on Hugging Face (`nlphuji/mscoco_2014_5k_test_image_text_retrieval`: 5,000 photographs and at least five captions each; the same publisher and layout as the Flickr 1K set of the bake-off). Everything in sections 2 to 5 is unchanged except: the image key is the COCO id (`cocoid`) of the 2014 file instead of the 2017 id; the ordering keys are `sha256("m1nd" + id)` and `sha256("m1sem" + id)` as written; the copy carries **no per-photo licence id** (the photographs are Flickr images under Creative Commons licences and the captions are CC BY 4.0, per the COCO paper; evaluation use on this machine only, nothing redistributed). None of these 5,000 photographs was used in any earlier decision of this programme.
* **A5 (written 2026-11-20 after the sealed near-duplicate read, before any further photo was scored; the verdicts of N1, N2, N3 stand as registered).** The sealed read gave: N1 NOT MET (hash tier at the registered 91 bits: recall 100 %, precision 98.84 %, 6 wrong answers), N2 NOT MET (DINOv2 tier at 0.108: precision 100 %, recall 90.27 %), N3 NOT MET (60 % centre crop + recompression 29.4 %, framed social copy 71.6 %), N4 MET (600 of 600). The cause of the N1 miss is the registered threshold rule, which takes the *loosest* distance that still holds 99.5 % on the development part (91 bits, where unrelated images start to appear), not a fault of the hash. From the **development part only** the gap between true copies and the nearest unrelated gallery image was measured (`dev_gap_analysis.py`): family R true copies are at most 26 bits away (99.9th percentile 24), the nearest unrelated image is at least 86 bits away. A **new operating rule for the product, set from that development gap and not from the sealed curve: the midpoint between the farthest true copy and the nearest unrelated image = 56 bits.** It is a post-hoc choice and is therefore **confirmed on photographs that took no part in any decision**: gate **N1b** = at 56 bits, on the 1,000 SEM1000 photos with their five family R copies each (gallery = the originals of the two thirds with `sha256("g"+id)` mod 3 not 0, the rest are negatives), precision >= 99.5 % and recall >= 95 %; the hash only, one read. The DINOv2 threshold stays at the registered 0.108 (precision 100 % on the sealed part). The product's labels follow the measurement: the DINOv2 tier is described as "re-compressed or lightly edited" (the measured weaknesses are heavy crops, framed images and full phone screenshots) and the limitation sentences name them. Improvements that could lift N2 and N3 (trimming uniform borders before the descriptors, local-feature verification for crops) are a separate version with its own pre-registration and its own fresh test.
