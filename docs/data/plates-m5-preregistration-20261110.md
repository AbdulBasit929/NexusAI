# Plates on images: own detector and reader (M5) — pre-registration, 2026-11-10

Written before any model of ours has been measured. Thresholds are fixed here; a miss is reported as a miss.

## Why

The plate models that run today are ship-blocking in the licence ledger: the YOLOv9-t detector (upstream terms unresolved, MOD-006) and the CCT-XS v2 reader (weights' terms not stated, MOD-007). The owner asked on 2026-11-10 for the replacement path of the media pack (`master-roadmap-20261103/02_media_programme.md`, section 6.1): a licence-clean detector and a reader trained on Pakistani-format plates, with grammar-constrained decoding. No consented real samples exist, so everything is trained and measured on synthetic and public data, and every number carries its tier.

## What is built

| Part | Choice | Licence position |
|---|---|---|
| Format registry | `ingestion/forensic_records/plate_format_registry_v1.json` + `plate_formats.py`: formats, series, visual classes, confusable pairs; data, versioned by date, an approximation to be reviewed by someone who knows the plates | ours |
| Reader | a small CRNN with CTC loss trained from scratch on synthetic plates (no pretrained weights), exported to ONNX, run on onnxruntime; candidates from a CTC beam; `plate_formats.choose` picks a read that fits a registered format only when it is close to the decoder's first choice and otherwise returns the first read flagged `format_unknown`; characters are never edited | ours |
| Detector | RT-DETR R18 (Apache-2.0 code in `transformers`; COCO-pretrained weights `PekingU/rtdetr_r18vd`, card licence to be read before download) fine-tuned for one class, exported to ONNX | Apache-2.0, ledger row MOD-031 updated after the card is read |
| Engine | a drop-in `predict(frame)` with the same result shape as the current FastALPR hook in `media_pipeline.py`, so the processor, the artifact contract and the readiness receipt are unchanged | ours |

## Baseline to beat (measured 2026-10-21, `results/plates/plates-dev-20261021-113415.json`, incumbent models, dev partition)

* Detection on 25 real Open Images photos (34 plate boxes): recall at IoU 0.5 = 73.5% from threshold 0.05 to 0.4 (70.6% at 0.5); image-level recall 96%; by plate width: under 40 px 10%, 40 to 100 px 100%, 100 px and over 100%; false boxes per image 0.04 to 0.2.
* Reading on 25 generated dev scenes: 20 exact (80%), character error 3.3%; among reads with confidence at least 0.9, 20 of 23 exact.

These samples are small; the sealed sets below are larger.

## Data, tiers and splits

* **T1 crops:** plates rendered from the registry (formats, series, visual classes, one-line and two-line layouts, fonts, emboss, dirt) and degraded (scale to 30 to 220 px wide, perspective, rotation, blur, noise, JPEG, partial occlusion). Training only; generated fresh each epoch from a seed.
* **T1-on-T3 scenes:** a synthetic plate warped over the plate box of a real Open Images photo (CC BY 2.0, the 501 local photos with 665 usable boxes, DAT-018); exact text and box. Several variants per photo for training.
* **T3 real plates:** the same photos with their real plates untouched (original Open Images boxes, mostly non-Pakistani plates). Used to keep the detector from learning only the look of a synthetic plate, and as the real-photo detection test. **Ledger:** DAT-018 is `evaluation-only` today; this work also trains on the development photos under their CC BY 2.0 licence with attribution (author and landing page are in `subset-test.jsonl`), and the row is updated to say so.
* **Split by photo id (SHA-256, last hex digit):** `0` to `9` train, `a` and `b` validation, `c` to `f` **sealed** (never trained on, never tuned on, run once). The earlier kit's "dev" is `0` to `b`.

## Gates (sealed split, run once, after the development numbers are fixed)

1. **Reader, gold boxes (generated sealed scenes):** exact normalised read at least 90% **and** the lower 95% Wilson bound at least 85% if the sealed count allows it (with fewer than 200 scenes the bound is reported and the gate is judged on the point estimate and stated as indicative). Calibration: of reads with confidence at least 0.9, at least 95% exact. Character error at most 3%.
2. **Detector, real sealed photos (T3):** recall at IoU 0.5 for plates at least 40 px wide at least 95%; overall recall not below the incumbent's on the same photos plus 5 points; at most 0.1 false boxes per image at the product threshold.
3. **End to end (detector then reader, generated sealed scenes):** exact at least 85%, and not below the incumbent on the same scenes.
4. **Abstention:** reads that fit no registered format are returned flagged `format_unknown`, never edited; the share is reported.
5. **Speed:** seconds per image on the laptop CPU reported; gate at most 5 s per image for the pair.

If gates 1 to 3 are met the pair replaces the blocked one behind the existing switch, after the readiness receipt and with the owner's word. If not, the incumbent stays as it is (ship-blocking, off for any shipped use) and the report says which gate failed.

## Honest limits

The plate designs are synthetic approximations from public descriptions; the registry says so and needs review. Nothing here measures real Pakistani plates in field conditions, so the result is certified as `synthetic and public data` only. Video (track voting, one-line and two-line motorcycle plates in motion) reuses these models and gets its own gate in M6.

## Compute

CPU only, one heavy job at a time. The reader (about 1 M parameters) trains inside a small container beside the running stack. The detector needs more memory than the Docker VM has spare while the model service is loaded, so its training runs with the model service paused and the owner told before and after.

## Amendment 1 (2026-11-10, before any model of ours is trained): the real Pakistani set

The owner pointed to real data already downloaded and used: the Kaggle "Pakistani License Number Plates Data" photos (`Downloads/archive`, 104 car photos and 4 plate crops, 49 MB) with a **human-verified gold of 32 images** made by the owner on 2026-08-30 (`local-acceptance-models/nxmmr/private-ground-truth/human-verification/image-labels.csv`: counts and text, no boxes, split into 10 `development` and 22 `sealed_holdout`; marked local, not to be committed or published). The incumbent models were measured on it once (`private-benchmarks/anpr-human-gold/`, processor `fastalpr-onnx-cpu`):

| Split | Images | Plates found (count proxy) | Exact normalised read | Character error |
|---|---|---|---|---|
| development | 10 | 10 of 10 | 5 of 10 (50%; Wilson 95% 24 to 76%) | 18.5% |
| sealed holdout | 22 | 20 of 22 (91%) | 13 of 22 (59.1%; Wilson 95% 39 to 77%) | 20.1% |

**Consequences.**

1. **This is the primary acceptance set** (real photographs, Pakistani plates, tier `public dataset, owner-labelled`; the licence of the photos has not been read and they are used for evaluation only, never for training). The synthetic sealed split stays as the second gate.
2. **Privacy:** plate strings from this gold are never written to a commit, a document or a report. Per-image outputs go to a local results folder outside the repository; what is committed is aggregate (counts, rates, intervals, shape classes).
3. **The sealed 22 are the incumbent's run-once set and ours.** They are used for one final run of each of our models. Tuning uses the 10 development images, the synthetic validation split and the Open Images validation photos only.
4. **Formats:** all 32 plates are one line; shapes: 3 letters + 4 digits 16, 3 + 3 10, 2 + 4 3, 2 + 3 2, 2 + 2 1; no year-style plate. The registry weights were set from this (v1, same day). The registry was not set from the sealed images' text beyond these aggregate shape counts.
5. **Gates on the real sealed 22** (added to the gates above; n is too small for the earlier "at least 90% with a lower bound of 85%", which only 22 of 22 would meet, so that stays the target for the field-certification set the owner may later supply):
   * exact normalised read at least 17 of 22 (77%) **and** strictly more images correct than the incumbent's 13, with at most one image the incumbent read correctly and we do not;
   * plates found (count proxy) at least 20 of 22;
   * no read that fits no registered format is shown as a clean read: it carries `format_unknown`.
6. The detection test has no boxes in the gold, so it is a count proxy, as the incumbent's was; box-level recall is measured on the Open Images photos.
7. Reader quality alone is also measured on the real images by cropping with the incumbent detector's boxes (evaluation scaffolding only, never shipped and never used to make training data), so a reader gain can be told apart from a detector gain.

If the real-set gates are missed while the synthetic ones are met, the result is reported as "good on synthetic, not on real" and the incumbent stays as it is.

## Amendment 2 (2026-11-10): use what is already published before training anything

The owner pointed out that this laptop has no GPU, that training and fine-tuning here is slow, and that this problem has been worked on widely. A from-scratch reader trained on the synthetic crops (1.16 M parameters, 14 epochs planned) had not finished its first 30,000-crop epoch after seven minutes, so the run was stopped (the trainer stays in `tools/media_data/` for later fine-tuning). The order of work is now: **find, fetch, measure published models; adopt the best licence-acceptable pair; fine-tune later with the data and compute that exist by then.**

**What was found (read 2026-11-10; licences are as stated by the publishers, not independently verified):**

| Candidate | What it is | Licence as stated | Open points |
|---|---|---|---|
| `autolane/rfdetr-alpr` (Hugging Face, rev 851dc6b2db27) | RF-DETR Medium fine-tuned for plates, 576 x 576, ONNX (117 MB; INT8 34 MB) | Apache-2.0 (card front matter, "same as RF-DETR") | training data and metrics not stated on the card |
| fast-plate-ocr pretrained readers (`cnn-ocr-lp` release `arg-plates`): cct-s-v2-global, global-mobile-vit-v2, and the cct-xs-v2 already in use | small readers trained on plates from 65+ countries | code MIT (the maintainer says all his repositories are MIT); no separate statement for the weights | training data not published; Pakistan coverage unknown |
| Published Pakistani work | Salma et al. 2021 (YOLOv4 + Tesseract, about 73% end to end); Yousaf and Khan 2021 PLPD dataset (about 6,000 images, access to be requested from the authors); DVLPD / VT-LPR (about 10k images, code on GitHub, no dataset link or licence seen) | not read | datasets would allow fine-tuning on real Pakistani plates once licences are read |
| Excluded | LiteALPR and Ultralytics-based detectors (AGPL-3.0) | AGPL | never ship |

**Measurement rule.** Every candidate is measured on the same sets: the 10 real development images (reader quality behind the incumbent detector's boxes, and the full pair), the Open Images validation photos with plate boxes (detector recall at IoU 0.5), and the synthetic development scenes. The best pair whose licence position is acceptable goes forward to the one sealed run on the real 22 images; a published model whose training data is not stated stays flagged `provenance unstated` in the ledger and in the product's model list until that is resolved. Gates are those above (Amendment 1). Fine-tuning on public Pakistani plate data and on the owner's own captures follows once all media families are implemented.

## Amendment 3 (2026-11-10): the pair taken to the sealed run, fixed before it

Development results on the 10 real development images (Kaggle Pakistani plate photos, owner labels; aggregate only):

| Pair | Plates found | Exact | Wilson 95% | Character error | s per image |
|---|---|---|---|---|---|
| incumbent (YOLOv9-t + cct-xs-v2) | 10 of 10 | 5 of 10 | 24 to 76% | 18.5% | 0.14 |
| incumbent detector + cct-s-v2 | 10 of 10 | 6 of 10 | 31 to 83% | 9.2% | 0.24 |
| incumbent detector + mobilevit-global | 10 of 10 | 4 of 10 | 17 to 69% | 12.3% | 0.13 |
| **RF-DETR (rev 851dc6b2db27, plain export, unit scaling) + cct-s-v2** | 10 of 10 | **7 of 10** | 40 to 89% | **6.2%** | 1.0 |
| RF-DETR (ImageNet scaling) + cct-s-v2 | 10 of 10 | 7 of 10 | 40 to 89% | 10.8% | 1.1 |

n is 10, so none of these differences is statistically established; the choice follows the lower character error and the licence position (RF-DETR Apache-2.0 tag replaces the YOLOv9-t of unresolved terms; the reader family is the one already in use).

**Configuration fixed for the sealed run, no further tuning:** detector `autolane/rfdetr-alpr` `rfdetr_alpr.onnx` (SHA-256 recorded in the fetch manifest), unit scaling, threshold 0.5, light non-maximum suppression at IoU 0.5; reader `cct_s_v2_global.onnx` with its published config; no crop padding; no registry editing of reads (the registry only flags). Run once on the 22 real sealed images; then the Open Images validation photos for box recall and the synthetic sealed scenes. Result and gates as in Amendment 1, reported as they come.

## Amendment 4 (2026-11-10): the sealed result on the real 22 images (run once, configuration of Amendment 3)

| | Incumbent (YOLOv9-t + cct-xs-v2) | New pair (RF-DETR + cct-s-v2) |
|---|---|---|
| Plates found (count proxy) | 20 of 22 (91%) | **22 of 22** (Wilson 95% 85 to 100%) |
| Exact normalised read | 13 of 22 (59.1%; 39 to 77%) | **16 of 22 (72.7%; 52 to 87%)** |
| Character error | 20.1% | **7.9%** |
| Seconds per image (CPU) | 0.10 | 1.03 |

Paired by image: both correct 13, **only the new pair correct 3, only the incumbent correct 0**, neither 6 (five of the six misses are labelled "easy", one "medium").

**Against the gates of Amendment 1:** plates found at least 20 of 22: **met** (22). Strictly more images correct than the incumbent's 13 and at most one image the incumbent read and we did not: **met** (16 against 13; 0 lost). Exact read at least 17 of 22 (77%): **NOT MET, by one image** (16 of 22). The pre-registered rule therefore leaves the incumbent as it is; the report says the new pair is better on every measure and strictly dominates it on this set, and the decision to run it as a flagged provisional default behind a switch (the incumbent being ship-blocking on licence) belongs to the owner.

**Not yet run:** box-level recall on the Open Images sealed photos, the synthetic sealed scenes, and the registry flags on real reads. The sealed 22 are now used up for this configuration; a further change needs new real images (the 72 unlabelled photos in the same folder could be labelled by the owner, or the published Pakistani datasets used once their licences are read).

**Open licence points, unchanged:** the RF-DETR weights' training data is not stated (Apache-2.0 tag); the fast-plate-ocr weights have no separate licence statement (code MIT). Both stay flagged in the ledger until closed.

## Amendment 5 (2026-11-10): wired in as a flagged, provisional engine (owner: "yes, wire it in")

* **Switch:** `FORENSIC_ANPR_ENGINE` = `fastalpr` (the processor that runs today; the code and the compose default) or `nexus` (`plate_processor_nexus.py`: RF-DETR detector + cct-s-v2 reader). `FORENSIC_ANPR_ENABLED` still decides whether the plate role runs at all. Nothing in the running stack has changed: the live worker has not been rebuilt or recreated.
* **Contract unchanged:** the new processor subclasses the existing one (observations, locators, crop provenance, review state `model_candidate`, manual review required, readiness receipt). It adds per observation `plate_format`, `plate_format_flags` (`format_unknown` when no registered format fits; the read is never edited), `plate_registry_version` and `model_provenance` ("provisional: detector training data not stated, reader weights without a separate licence statement"), and a limitation line on the result. The incumbent's `detector_version` is now an attribute, so each processor states its own.
* **Files:** code in `ingestion/forensic_records/` (`plate_processor_nexus.py`, `plate_detector_onnx.py`, `plate_formats.py` + registry, `plate_reader.py`); the three model files are staged in the lane's git-ignored `models/media/nexus-plates/` with their SHA-256 in a `PROVENANCE.json`; compose declares the paths and a detection threshold of 0.5 (the value measured).
* **Checks:** 4 unit tests of the processor pass in the worker image's own runtime, plus 6 registry and 3 reader tests; an end-to-end smoke through `configured_media_processor()` with the real models on four real photos gives readiness READY, 1.2 to 2.9 s per image on this CPU.
* **Finding to carry forward:** the detector finds several plate-like regions per photo (2 to 4 on the four smoke images, 0.69 to 0.96 confidence); usually one of them reads as a registered format and the others are flagged `format_unknown`. They may be other plates in the frame or false regions; the owner-labelled gold has one plate per image and cannot say which. All are returned as model observations for review, flagged.
* **To switch it on in the live product (not done, needs the owner's word):** rebuild the worker overlay with the four new modules and the edited `media_pipeline.py` (`tools/media_data/Build-WorkerOverlay.ps1`), mount `models/media/nexus-plates`, set `FORENSIC_ANPR_ENGINE=nexus`, recreate the worker (rollback: the previous worker tag, or set the switch back).

## Amendment 6 (2026-11-10): go-live preparation found that the detector also boxes vehicles; a correction and the final configuration

**Correction to Amendment 4.** The published detector returns boxes around whole vehicles (a car, a neighbouring car, a rickshaw) as well as around plates, at confidences up to 0.96. The "plates found" figure (22 of 22) is a count proxy that counts any detection, so a vehicle box counted as a found plate; that figure is not evidence that 22 plates were localised. The exact-read result (16 of 22) is unaffected, because an exact read needs the right text.

**Box recall on real photographs** (Open Images, CC BY 2.0, the 125 sealed photos with 167 usable plate boxes, IoU 0.5, at each detector's product threshold; evaluator `tools/media_data/plate_eval_detect.py`):

| Detector | Recall | Under 40 px | 40 to 100 px | 100 px and over | Detections per image |
|---|---|---|---|---|---|
| RF-DETR at 0.5, no rule | 79.0% | 9 of 35 | 38 of 43 | 85 of 89 | 3.2 |
| RF-DETR at 0.5 with the non-plate rule | 77.2% | 9 of 35 | 35 of 43 | 85 of 89 | 1.76 |
| incumbent YOLOv9-t at 0.75 | 70.7% | 1 of 35 | 28 of 43 | 89 of 89 | 0.94 |
| incumbent at 0.5 | 75.4% | 2 of 35 | 35 of 43 | 89 of 89 | 1.02 |

**Gate 2 (box recall):** plates 40 px and wider at least 95%: **not met** (about 93% without the rule, 91% with it; the incumbent 89% at its threshold). Overall recall at least the incumbent's plus 5 points: met against the incumbent at its product threshold (77 to 79% against 70.7%). False boxes at most 0.1 per image: **not met** as measured, but Open Images labels are not exhaustive, so the over-count (0.73 per image with the rule, 2.1 without) cannot be read as a false-plate rate.

**The non-plate rule (final form, chosen on the owner's development images and the Open Images photos, not on a sealed set).** A first rule (large and boxy) dropped a real plate in a close-up whose plate fills the frame (development: 9 of 10 found, character error 6.2% to 16.9%) and was replaced. The final rule, `drop_non_plates` in `plate_detector_onnx.py`: drop a box that is portrait-shaped (width under 1.1 times its height), and drop a large boxy box (over 5% of the image, width under twice its height) that contains a much smaller detection (at most a quarter of its area, 90% inside it). Result with the rule: development 10 of 10 found, 7 of 10 exact, character error 6.2% (identical to without); each of six real photos in the staging proof yields exactly one plate observation, all fitting a registered format.

**Disclosed second use of the sealed 22 (final configuration).** The product ships with the rule, so the final configuration was run once on the real sealed 22 to check that the rule costs no real plate: 22 found, 16 exact, character error 7.9%, paired with the incumbent unchanged (3 gained, 0 lost). This is the same result as Amendment 4; no setting was tuned on it.

**Overlay.** `nexusai-forensic-records-worker:plates-20261110c` is built on top of the running image with a fail-safe patch of the image's own (older) `media_pipeline.py` (`tools/media_data/patch_image_media_pipeline_plates.py`, `Build-PlatesOverlay.ps1`); `latest` and the running worker are unchanged. Staging proof with the overlay's own code: engine unset builds the current processor; engine `nexus` works end to end. Runbook with the rollback: `tools/media_data/PLATES_GO_LIVE.md`. **Not executed.** The live worker has the plate role off today; going live also means enabling `FORENSIC_ANPR_ENABLED`.

## Amendment 7 (2026-11-10): switched on in the live worker (owner: functional first with acceptable results, accuracy work after every data type is in)

* **What changed live:** only the worker was recreated, by `tools/dev/live-window/5-recreate-worker-plates.ps1 -Apply`, on image `nexusai-forensic-records-worker:plates-20261110d` (built on the running image `47e5e829a3d4` by `Build-PlatesOverlay.ps1`), with `docker-compose.forensic-records.plates.yaml` added to the four files of the running worker (the dry run showed the four files reproduce the running container's compose hash, `5012dc741320`). Plate role on (`FORENSIC_ANPR_ENABLED=true`), engine `nexus`, CPU providers, the three model files pinned by SHA-256 (a file that differs is refused; tested). The API, database, queue and model server were not touched. No ingest job was in flight.
* **Rollback (tested path, one command):** `.\tools\dev\live-window\5-recreate-worker-plates.ps1 -Rollback` retags `before-plates-20261110d` (= `47e5e829a3d4`, the image that ran before) as latest and recreates the worker without the plates file: the plate role is off again.
* **Live smoke, inside the recreated worker with its own configuration:** the live image processors are the plate processor and the existing OCR router; two real photos give one observation each, `nexus-plates-onnx-cpu`, ready, review required, model hashes present; one read fits a registered format, the other (labelled "partial") is flagged `format_unknown`; 0.5 to 0.6 s per image in the container. Three small test files copied into the container's `/tmp` could not be removed (permission); they disappear at the next recreate.
* **Status of the engine:** provisional and not certified: 16 of 22 exact on the owner-labelled real sealed set against 13 for the current models, the pre-registered gate of 17 missed by one image; box recall for plates 40 px and wider about 91 to 93% against the 95% gate; detector training data not stated; reader weights without a separate licence statement. Observations say so (`model_provenance`).
* **Next for plates (accuracy work, after every data type is in):** label the 72 unlabelled photos (or obtain a published Pakistani plate set once its licence is read), re-run the gates on a fresh set, fine-tune the reader on real and synthetic Pakistani crops, and tune the small-plate recall.
