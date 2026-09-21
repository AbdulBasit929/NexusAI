# NX-MMR product-demo activation and multimodal progression v1

Date: 2026-08-31  
Authority: private internal development/team-lead demonstration  
Outcome: **source-ready; live activation safely blocked by the 6 GiB RAM gate**  
Product certification: **false**

# Verified Starting State

The protected runtime is unchanged. Final identities were worker
`296e0881d891`, forensic API `b5c13f96e67f`, LocalAI `321ba681774b`, NATS
`5c49e70d132a`, and PostgreSQL `f66e05a3b179`; all applicable health checks were
healthy. The worker still has Image ANPR and printed OCR disabled, has no Video
V3 enable flag, and retains Video V2 disabled by absence/default. PostgreSQL has
61 completed and four dead-letter jobs, with zero active jobs.

Free workspace disk was 1,648.197 GiB. Available RAM was 4.790 GiB at activation
preflight and 5.031 GiB at final verification, below the mandatory 6 GiB floor.
No service, database, volume, Activity record, retained evidence, or live model
directory was mutated.

# Product-First Sequencing Correction

The authorized order was followed: close license/technical admission, select a
license-clean video candidate using development evidence only, then attempt the
bounded demo activation preflight. Because activation did not pass, fresh live
Data/Ask/citation/Activity/UI acceptance and later STT/TTS breadth were not
misrepresented as complete.

# License / Demo Admission

Engineering disposition is `ADMITTED_WITH_NOTICE` for a private internal demo,
not legal advice and not external redistribution/customer-delivery approval.
The notice bundle is in `docs/legal/nxmmr-demo-third-party-notices.md`.

## Image Detector

The exact 384 ONNX detector is hash-pinned to
`888397b96d761c89db40bc9c305838e8652660f5e282c2cadebbe8d2951a77a8`.
The publisher exposes that exact model through its project hub, and the
open-image-models project is MIT licensed: [model hub source](https://github.com/ankandrew/open-image-models/blob/main/open_image_models/detection/core/hub.py),
[license](https://github.com/ankandrew/open-image-models/blob/main/LICENSE).

## FastPlate OCR

The OCR ONNX and configuration are hash-pinned to `8031afb5...e2855f44` and
`0335c74a...23b2d0a6`. The project release publishes the model family and the
project is MIT licensed: [FastPlate v1.1.0](https://github.com/ankandrew/fast-plate-ocr/releases/tag/v1.1.0),
[license](https://github.com/ankandrew/fast-plate-ocr/blob/master/LICENSE).

## Paddle OCR

The detector, English recognizer, and Arabic/Urdu recognizer trees are locally
present and hash-pinned. PaddleOCR documents the official OCR and detection
models and uses Apache-2.0: [OCR model documentation](https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/pipeline_usage/OCR.md),
[detector documentation](https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/module_usage/text_detection.en.md),
[license](https://github.com/PaddlePaddle/PaddleOCR/blob/main/LICENSE).

## Ultralytics Status

`BLOCKED_NOT_ENABLED`. No applicable enterprise/R&D agreement or approved AGPL
posture is documented. Video V3 neither imports nor requires Ultralytics, and
the direct worker dependency was removed. The prior `.pt` V2 assets remain
ineligible for live activation.

# Image ANPR

## Live Model Readiness

`PASS_WITH_OCR_LIMITATION`. The existing non-holdout local run processed two
positive Pakistani development images and one generated negative in 2.157 s.
Both positives produced bounded candidate observations; one of two OCR readings
was exact. The negative produced `COMPLETE_ZERO_RESULTS`. The sealed image
holdout was not rescored.

## Activation

`SOURCE_READY_LIVE_BLOCKED_RAM_GATE`. Exact asset destinations and flags are
frozen in `configuration/nxmmr_demo_activation_v1.json`; no files were copied
into the live model mount and no worker rebuild/recreate occurred.

## Fresh Processing

Not run against live services because the activation gate failed.

## Data

Source contract and typed result-state support pass; fresh live display is pending.

## Ask

Evidence-scoped query routing exists; fresh live execution is pending.

## Citation

The `forensics.anpr-observation/v1` contract carries evidence/version identity,
original-pixel bounds, crop identity, model hashes, confidence, and
review-required state. Fresh live citation opening is pending.

## Activity

Not exercised; Activity was not mutated.

## UI Acceptance

Not run. No live or manual pass is claimed.

# Video ANPR

## Selected License-Admissible Architecture

`NX-MMR-VIDEO-ANPR-ONNX-DEMO-V3`: the existing 384 ONNX plate detector plus
FastPlate `DefaultOCR`, actual source frames at 4 FPS, plate-only processing,
minimum support three, no vehicle detector, no persistent tracking, and no
interpolation. Selected strings must have been observed. All results remain
review-required candidate observations.

## Existing 384 ONNX Development Result

The preregistered development-only run covered 11 human events, 24 truth plate
occurrences, and 17 truth groups. It analyzed 133 frames and made 210 OCR calls
with zero failed frames.

| Measure | Result |
|---|---:|
| Event detection recall | 1.000000 |
| Exact TP / FP / FN | 14 / 4 / 10 |
| Exact precision / recall / F1 | 0.777778 / 0.583333 / 0.666667 |
| Normalized CER | 0.315476 |
| Group TP / FP / FN | 13 / 2 / 4 |
| Group precision / recall / F1 | 0.866667 / 0.764706 / 0.812500 |
| Raw detector negative-frame FPR | 0.875000 |
| OCR-candidate negative-frame FPR | 0.805556 |
| Group-selected negative-frame FPR | 0.555556 |
| Boundary MAE / median / p95 | 0.894385 / 0.584500 / 2.676750 s |

All four preregistered development utility gates passed. An independent
receipt-only recomputation matched both scorer and stage metrics exactly. Error
decomposition found 14 exact selected occurrences, eight misses with no exact
raw OCR observation, and two exact raw observations not selected by grouping.
No development event lacked raw or selected OCR.

The run used 198.186 wall seconds, 486.113 CPU seconds, and 550.203 MiB peak RSS.
The result receipt SHA-256 is
`3db78b26ff8ae6c1cba395c7b1731f7c17c6960ac0498ca43ae5c4730b4bf14b`.

## 640 Need If Any

None for this bounded internal-demo decision. The 384 candidate passed all
preregistered gates, so no 640 model was acquired or downloaded.

## Activation

`SOURCE_READY_LIVE_BLOCKED_RAM_GATE`. The worker-only build/recreate was not run.

## sample.mp4 Processing

Development projection only. The consumed V2.2 reserved evaluation was not
rerun, reused, or tuned against; the reserved count remains one and consumed
reserved rows accessed by V3 remain zero. A fresh live copy was not ingested.

## Candidate Groups

Fifteen temporal packets were produced. Two were false groups under the human
plate-and-actual-source-time definition. High detector/OCR candidate noise is
therefore intentionally hidden behind grouped, review-required observations.

## Timeline / Seek

Typed group/source-time and exact observation locator contracts are present.
Fresh live timeline seeking remains pending.

## Ask

Evidence-scoped group/timeline query support is present. Fresh live Ask execution
remains pending.

## Citation

The group contract retains source evidence/version identity, first/last source
time, support observations, and exact frame locators. Fresh live citation opening
remains pending.

## Activity

Not exercised; Activity was not mutated.

## UI Acceptance

Not run at 390/820/1024/1440 widths because live activation did not pass.

## Certification Limitation

`DEVELOPMENT_EVIDENCE_ONLY_NOT_PRODUCT_CERTIFIED`. One video and 11 development
events do not establish general Pakistani or real-world accuracy. Outputs cannot
assert identity, ownership, or continuous vehicle presence.

# English OCR

Five generated printed fixtures measured CER 0, WER 0, line exact 100%,
identifier exact 100%, and 877.483 ms mean latency: `FIXTURE_MEASURED_PASS`.

# Urdu OCR

Five generated printed fixtures measured CER 0.118012, WER 0.258065, line exact
20%, identifier exact 80%, and 726.967 ms mean latency:
`FIXTURE_MEASURED_WITH_LIMITATION`. Independent real-world Urdu evidence remains
required.

# Mixed English/Urdu OCR

Five generated mixed fixtures measured CER 0.143791, WER 0.318182, line exact
0%, identifier exact 100%, and 609.728 ms mean latency:
`FIXTURE_MEASURED_WITH_LIMITATION`.

## Live Processing

Not run because the RAM activation gate failed.

## RTL/LTR

Source and fixture validation pass: English LTR, Urdu RTL, and per-region mixed
direction with logical Unicode order. Manual browser rendering is pending.

## Identifiers

English and mixed identifiers were exact in 10/10 fixtures; Urdu identifiers
were exact in 4/5. One Urdu date changed component order.

## Data

Typed region/result states exist; fresh live Data presentation is pending.

## Ask

English/Urdu/Roman-Urdu/mixed evidence-scoped query paths exist; fresh live Ask
execution is pending.

## Citation

Region polygon, reading order, script/direction, confidence, and source identity
are preserved; fresh live citation opening is pending.

## Activity

Not exercised; Activity was not mutated.

## UI Acceptance

Not run. Handwriting is outside this demo.

# Demo Workspace Status

| Capability | Current status |
|---|---|
| Image ANPR | Source-ready; activation blocked by RAM |
| Video ANPR V3 | Development utility pass; activation blocked by RAM |
| English printed OCR | Fixture pass; activation blocked by RAM |
| Urdu printed OCR | Fixture pass with limitation; activation blocked by RAM |
| Mixed printed OCR | Fixture pass with limitation; activation blocked by RAM |
| Data/Ask/citations | Source contracts pass; fresh live acceptance pending |
| Activity/manual responsive UI | Not run; no pass claimed |

# Team-Lead Demo Runbook

The exact guarded sequence is documented in
`docs/demo/nexusai-team-lead-multimodal-demo-v3.md`. It requires a new preflight,
worker-only `--no-deps` activation, fresh controlled evidence, zero-result cases,
source-time seeking, citations, Activity, reopen continuity, keyboard checks,
RTL rendering, and four responsive widths.

# Product Certification Status

`ProductCertification=false`. No operation is promoted directly to
`PRODUCT_CERTIFIED`; live API/Ask/Data/citation/Activity/UI and human product
acceptance gates remain mandatory.

# New Independent Video Evaluation Deferred

No new sealed evaluation was created or run. The consumed V2.2 holdout remains
sealed/consumed and is never to be rerun, reused, or tuned against. Any future
independent video evaluation requires a new human-labeled, independently locked
pack and a new preregistration.

# Next Multimodal Breadth

## English STT

Deferred until this vertical passes live UI acceptance. No ASR scoring was run
in this phase.

## Urdu STT

Deferred under the same gate. The existing speech oracle must independently
provide sufficient transcript evidence; model output may never serve as oracle.

## TTS

English then Urdu TTS follows STT acceptance; not started.

## Image Similarity

Next after STT/TTS; not started.

## Face

Safe candidate comparison follows image similarity; no identity claim and not started.

## Documents/Structured

Remain later breadth items; NX-B2.1 was not started.

# Files Changed

- Added the plate-only Video V3 processor, its focused tests, preregistered
  development evaluator, and independent receipt auditor.
- Wired an explicit mutually exclusive V3 factory flag, configuration knobs, and
  worker image source copy.
- Removed the direct Ultralytics worker dependency.
- Added V3 candidate, selection, demo admission, activation, third-party notice,
  and team-lead runbook artifacts.
- Updated `NEXUSAI_CONTINUATION.md` with the bounded outcome.

The worktree contained extensive pre-existing user changes; they were preserved.

# Tests

- Video V3 unit tests: 4/4 pass.
- Network-disabled synthetic exact-model smoke: pass; Ultralytics import false;
  OpenVINO and CPU execution providers active.
- Python compile check for V3/media/evaluator: pass.
- Analyst media presentation Node tests: 10/10 pass.
- Focused forensic API Go tests: pass in 17.129 s.
- Four new JSON governance/configuration files: parse pass.
- Registered source hashes: unchanged after evaluation.
- Independent scorer/stage recomputation: identical.
- Candidate/source/config truth-literal scan: zero leaks. The older continuation
  history still contains previously documented sample outputs; none were added to
  candidate behavior.

# Manual Browser Results

`NOT_RUN_BLOCKED_BY_ACTIVATION_RAM_GATE`. No screenshots or viewport pass is
claimed.

# Runtime Impact

Zero live runtime impact. No asset copy, image build, container recreate, model
download, service deployment, or configuration mutation occurred.

# Retained Impact

Zero. No retained evidence was processed or bulk reprocessed; no database,
volume, or Activity mutation occurred.

# Open P0

One: restore at least 6 GiB available RAM while keeping 12 GiB disk, zero active
jobs, healthy protected services, exact asset hashes, and the notice gate. The
latest measurement was 5.031 GiB, so activation must remain stopped.

# Open P1

- Worker-only live activation and rollback verification.
- Fresh Data/Ask/citation/Activity/manual UI acceptance at four viewports.
- Independent real-world Urdu/mixed printed-image pack.
- A future new independent video pack, if stronger than demo evidence is needed.
- Separate legal/packaging review before external distribution or delivery.

# Exact Next REAL Boundary

Restore available RAM to at least 6 GiB without force-closing user applications,
then rerun the complete preflight. Only if every gate passes: copy the exact
hash-pinned assets, build and recreate **only** `forensic-records-worker` with
`--no-deps`, keep V2/Ultralytics disabled, enable V3/Image ANPR/Paddle OCR, and
run the fresh controlled Image ANPR, `sample.mp4`, English, Urdu, mixed, negative,
Data, Ask, citation, Activity, reopen, and 390/820/1024/1440 UI acceptance pack.
If any gate fails, stop and report it. Do not start STT/TTS or later breadth until
that live product boundary passes.
