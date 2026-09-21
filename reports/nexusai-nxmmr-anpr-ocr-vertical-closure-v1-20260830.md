# NX-MMR ANPR/OCR vertical closure V1

Status: source/local development closure complete; live activation, reserved
Video V2 evaluation, real-world OCR certification, and product acceptance are
not performed.

# Verified Starting State

The locked independent human gold digest remains
`88c6827c23fbed467f097843404ed09c947bf7f27432e5cb5fa8427be9f48567`.
The image baseline and its already-consumed sealed holdout were not rescored.
Video Adapter V1 source/config and freeze remain byte-identical, with freeze
digest `1dcfabf17a8b66165b6c1390e65a8784eb875d3650b1b7f27d328c9ad2f0bb13`.
The reserved video partition was not opened or scored.

Read-only live inspection found all five services running; worker, LocalAI/UI,
PostgreSQL, and NATS were healthy. No service was rebuilt, restarted,
reconfigured, or recreated. The live worker image remained
`sha256:be13dbec4bd9bb62c824140df3777b898d7ad2801a0e68cd512c00dafaf4f9ef`.

# Recent Fresh-Image Failure Root Cause

The recent `DSC_1106.JPG` evidence (source digest prefix `a7c47789`, admitted
2026-08-30 07:11 UTC) was correctly classified `image/image`, routed to
`unified_media_worker`, queued, processed once, and marked completed. It
produced only the image fingerprint and technical image observation. The worker
reported `no approved local vision or ANPR role is configured`.

The exact defect is the live MODEL link: `FORENSIC_ANPR_ENABLED=false`,
`FORENSIC_OCR_ENABLED=false`, and the read-only `/models/media` mount contains
none of the configured ANPR/OCR assets. Upload, route, queue, worker execution,
and generic persistence succeeded; ANPR/OCR model execution never started.
This is `MODEL_REQUIRED`/`NOT_RUN`, not a successful zero-result inference.

`RecentImageFailureRootCause=LIVE_ANPR_OCR_ROLES_DISABLED_AND_MEDIA_MODEL_MOUNT_EMPTY`

# Fresh Image ANPR Processing Chain

`UPLOAD (PASS) → ROUTE (PASS) → QUEUE (PASS) → WORKER (PASS) → MODEL
(MODEL_REQUIRED/NOT_RUN) → RESULT (NO ANPR EXECUTION) → ARTIFACT (TECHNICAL
ONLY) → DATA (NO PLATE FINDING) → ASK (NO PLATE RESULT)`.

The repaired source path uses the existing hash-pinned FastALPR image processor,
persists structured candidate observations/crop reconstruction metadata, and
exposes explicit result states. It remains inactive until the exact model mount,
environment, image build, and worker recreation are approved.

# Image Model Readiness

Local/non-retained readiness is `READY`: code, backend, exact detector/OCR/config
bytes, resource policy, and role admission all passed. A missing-detector probe
correctly returned `MODEL_REQUIRED`. Live readiness remains `MODEL_REQUIRED`
because the roles are disabled and assets are absent from the live mount.

- detector: 7,771,218 bytes,
  `888397b96d761c89db40bc9c305838e8652660f5e282c2cadebbe8d2951a77a8`;
- OCR: 3,344,292 bytes,
  `8031afb5fdc6b4d80462c9d542f1284ebd2cfddf5dbacd62609848d7e2855f44`;
- OCR config: 1,725 bytes,
  `0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6`.

`FreshImageANPRSourceReady=true`

`ImageANPRProcessorReady=SOURCE_READY_LIVE_MODEL_REQUIRED`

# Image ANPR Local Processing Results

The exact source adapter processed two non-holdout Pakistani development images
and one generated negative in 2.157 seconds. Both positives decoded, detected a
plate, ran OCR, emitted a bounded region/crop locator, model hashes, confidences,
and `COMPLETE_RESULTS`. One prediction was exact (`BB3914`); one was a useful
but inexact candidate (`LEB15491` versus human `LEB4910`). The negative emitted
zero observations and `COMPLETE_ZERO_RESULTS`. The sealed image holdout was not
scored.

`ImageANPRLocalFunctional=PASS_WITH_OCR_LIMITATION`

# Image Data Contract

`forensics.anpr-observation/v1` now carries evidence/version identity, source
file, original-pixel bounding box, raw and formatting-only normalized text,
detector/OCR confidences, processor/model IDs and hashes, reconstructable crop
hash/transform chain, manual-review state, result state, and limitations.
Confidence is never labelled accuracy. The retained source remains authoritative.

# Image Ask Contract

The existing `anpr.sightings` operation is reused; no duplicate ANPR operation
was added. Its backend query accepts an optional validated exact `evidence_id`,
so an image-scoped question cannot leak candidates from another evidence item.
Ask and Data consume the same persisted observation contract. English, Roman
Urdu, and Urdu-script variants remain test intents only; they are not ordinary
visible suggestions.

# Video V1 Disposition

V1 remains frozen historical evidence and is not productized: development event
recall 0.818182, normalized exact F1 0.105263, exact group F1 0.057143, and
negative-frame false-positive rate 0.333333. Its reserved evaluation remains
unconsumed because development precision was already clearly inadequate.

# Historical Video Components Reused

Only measured useful components were reused: actual decoded frames, fixed 4 FPS,
the exact dedicated 640 plate detector, the exact COCO vehicle detector for a
development A/B, grayscale/binary-inverse crop preparation, and repeated OCR.
Persistent SORT identity, interpolated rows, UK positional regex, character
substitution, and vehicle identity/journey authority were excluded.

# Video V2 Architecture

`decoder → bounded fixed-4-FPS actual-frame scheduler → hash-pinned dedicated
640 plate detector → required vehicle containment → actual plate crop →
FastPlateOCR on repeated crops → short source-local clustering →
confidence-weighted selection of an observed candidate → source-time group`.

The source processor is behind `FORENSIC_VIDEO_ANPR_V2_ENABLED=false` by
default, prevents the legacy sampler from duplicating plate observations, and
allows only the separate general-OCR cadence to coexist.

# Dedicated 640 Detector

`license_plate_detector.pt` is 6,241,454 bytes with SHA-256
`8ec3b254a6c87610f037a90957462cafa11a9c03224e33a28c6a1d1ac2ac51b0`.
It is loaded at 640 pixels, CPU, confidence 0.25, with hash verification and no
download. The reference source is MIT, but the exact checkpoint's production
license admission remains a P1 gate.

# Vehicle Context A/B

On the 11-event development partition, plate-only and vehicle-contained variants
both reached event recall 1.0, normalized exact F1 0.844445, and group F1
0.909091. Plate-only negative-frame FPR was 0.680556; vehicle-contained FPR was
0.652778. Vehicle context also slightly improved mean absolute group boundary
error (0.778033 versus 0.792433 seconds).

# Selected Vehicle Context Policy

`VEHICLE_CONTEXT_REQUIRED_FOR_VIDEO`. The selected `yolov8n.pt` is 6,534,387
bytes with SHA-256
`31e20dde3def09e2cf938c7be6fe23d9150bbbe503982af13345706515f2ef95`;
only classes 2/3/5/7 are containment contexts. No vehicle ID is emitted. The
Ultralytics/checkpoint production-license decision remains open.

# Frame Policy

Fixed 4 FPS, 640 input, actual decoded source frames only, maximum 300 analyzed
frames, maximum source duration 900 seconds, no refinement/interpolation, and
partial-sampling truth in the coverage receipt. A partial zero cannot establish
that a video contains no plates.

# Repeated OCR

The development replay re-read 228 observations from 231 unique plate-only
crops and 213 from 216 unique vehicle-contained crops. OCR outputs are tied to
actual crop/frame hashes and source time. Missing OCR output remains missing;
no candidate is inferred for an unobserved frame.

# OCR Candidate Aggregation

Association is source-local with a one-second maximum gap, bounded spatial
overlap/proximity and edit distance one. The selected policy requires support
3, alphanumeric length 4–10, best OCR confidence at least 0.5, and weighted
support at least 1.5. Edit distance clusters readings only; the final string is
an OCR string actually observed on a processed frame.

# False Positive Filtering

Vehicle containment, support, confidence, length, short temporal lifetime,
spatial consistency, crop de-duplication, and observed-candidate selection
reduced exact group FP from 17 in V1 to 1 in V2. There is no Pakistan-province
regex and no O→0/I→1/B→8 correction. The remaining negative-frame FPR of
0.652778 is high and must be shown as a limitation: repeated noisy frame
detections persist, although temporal evidence gates prevent most from becoming
final groups.

# Video Development Metrics

On 11 development events / 24 truth plates / 17 truth groups:

- event TP/FN 11/0; recall 1.000000 (Wilson 95% 0.741167–1.0);
- normalized exact TP/FP/FN 19/2/5; precision 0.904762, recall 0.791667,
  F1 0.844445, CER 0.178571;
- count-proxy TP/FP/FN 20/0/4; F1 0.909091;
- group TP/FP/FN 15/1/2; precision 0.937500, recall 0.882353,
  F1 0.909091;
- 47 false-positive negative frames among 72; FPR 0.652778;
- matched group boundary error: mean absolute 0.778033 s, median 0.512 s,
  p95 2.764550 s; mean signed first +0.161133 s, last −0.009733 s.

`VideoANPRV2=PASS`

`VideoANPRV2DevelopmentEventRecall=1.000000`

`VideoANPRV2ExactF1=0.844445`

`VideoANPRV2GroupF1=0.909091`

# Video Resource Metrics

The deliberately simple random-seek development replay took 582.528 seconds
wall, 4,990.219 seconds aggregate CPU, and 672.398 MiB peak RSS. This is not a
streaming-production latency estimate. Vehicle-context FastPlate OCR averaged
15.586 ms/crop (p95 54.720 ms); plate-only averaged 47.079 ms (p95 124.559 ms).
Future live acceptance must measure the integrated streaming worker.

# Video V2 Freeze

Development selection is frozen against the source/config/receipt hashes. The
earlier pre-integration freeze is preserved as provenance but superseded by the
product-source freeze. V1 remains independently unchanged. Neither V2 reserved
metrics nor V1 reserved metrics were computed.

`VideoANPRV2Frozen=true`

# Video V2 Freeze Digest

Authoritative product-source freeze:
`212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20`.

# Reserved Evaluation Status

`VideoANPRV2EvaluationPerformed=false`

The one-time reserved V2 evaluation is now admissible only for the exact freeze
above, with no tuning or source/configuration changes after viewing results.

# Urdu OCR Architecture

A hash-pinned `PP-OCRv5_mobile_det` detects text regions. Each actual crop is
read by `en_PP-OCRv5_mobile_rec` and
`arabic_PP-OCRv5_mobile_rec`; the higher-confidence non-empty observed reading
is retained with both candidate records, raw Unicode, normalized whitespace,
region polygon/bbox, logical reading order, script/direction, identifiers,
model tree hashes, and review warning. Paddle documents the Arabic multilingual
model as covering Urdu and English, among other languages:
https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/algorithm/PP-OCRv5/PP-OCRv5_multi_languages.en.md

# PaddleOCR Model Inventory

- detector `PP-OCRv5_mobile_det`, tree SHA-256
  `1771a1ca1af2b1625228d2349849be1ccba507af13753547969c1de1702bdd74`;
- English recognizer, tree SHA-256
  `9580601f0a93d061d89cd89676be65353a8e81ac760063f5c22788f4fe5673a9`;
- Arabic multilingual recognizer, tree SHA-256
  `3ee6f2267929b0fae36a6c63565f70f506ced8e133e9e2ed626a89c131d4cd1f`;
- runtime `paddleocr==3.7.0`, `paddlepaddle==3.3.1`, CPU, four threads.

Official recognition module/model information is recorded at
https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/module_usage/text_recognition.en.md.

# Paddle Text Detector Availability

The exact detector already exists in the isolated
`nexusai_r8_paddlex_model_cache` and ran network-disabled in the accepted fixture
benchmark. No new model download is required. Its production destination is
not populated and its Apache-2.0 packaging/NOTICE gate remains open. Official
mobile detector configuration:
https://github.com/PaddlePaddle/PaddleOCR/blob/main/configs/det/PP-OCRv5/PP-OCRv5_mobile_det.yml.

`PaddleTextDetector=AVAILABLE`

# Tesseract Baseline

The earlier identical eight-fixture comparison remains the resource floor:
English exact 50%, CER 0.143, mean 120.1 ms; Urdu exact 0%, CER 0.214, mean
172.2 ms; peak RAM 19.75 MiB. Both negatives passed. Tesseract remains a small
rollback/fallback baseline, not the selected multilingual primary.

# Urdu OCR Results

Five generated, non-retained printed Urdu fixtures with text fixed before
inference produced CER 0.118012, WER 0.258065, line exact 20%, identifier exact
80%, and mean latency 726.967 ms. All result regions were RTL. One date was
recognized in changed component order, so identifier preservation is limited.
This is fixture evidence only; 5–10 independent real-world human-labeled Urdu
images are still required.

`PrintedUrduOCR=FIXTURE_MEASURED_WITH_LIMITATION`

# English OCR Results

Five generated printed English fixtures produced CER 0, WER 0, line exact
100%, identifier exact 100%, mean latency 877.483 ms, and LTR regions.

`PrintedEnglishOCR=FIXTURE_MEASURED_PASS`

# Mixed Urdu/English Results

Five generated mixed-script fixtures produced CER 0.143791, WER 0.318182,
line exact 0%, identifier exact 100%, and mean latency 609.728 ms. Both scripts
were returned, but imperfect text/order blocks a stronger claim.

`MixedEnglishUrduOCR=FIXTURE_MEASURED_WITH_LIMITATION`

# RTL / LTR Validation

English regions were emitted LTR, Urdu regions RTL, and mixed regions with
per-region LTR/RTL/mixed direction while preserving Unicode logical order.
Browser rendering remains a post-activation manual gate.

`RTLValidation=SOURCE_AND_FIXTURE_PASS_MANUAL_UI_PENDING`

# Identifier Preservation

English and mixed identifiers were exact in 10/10 fixtures; Urdu identifiers
were exact in 4/5. No code reverses or substitutes identifiers. The single Urdu
date-order recognition error is preserved as model output rather than silently
corrected.

`IdentifierPreservation=PARTIAL_14_OF_15_FIXTURES`

# OCR Result Contract

`forensics.image-ocr-observation/v1` carries source evidence/version, source
file, pixel polygon/bbox, region reading order, raw and normalized text,
script/direction, exact identifier tokens, detector/recognition confidence,
selected and alternate recognizers, model hashes, crop hash/reconstruction
state, manual-review warning, result state, and limitations. Handwriting is
explicitly uncertified.

# OCR Ask Contract

The existing `image.ocr_search` operation is reused. Its derived-text query is
now exactly scoped by evidence ID as well as tenant/case and accepted OCR
contracts. Exact/normalized text retrieval is deterministic; semantic retrieval
authority is unchanged. Ask returns analyst prose plus source-region citations,
not raw JSON or model vectors.

# Auto-Routing

For images with both roles enabled, a contextual processor uses plate detection
as the cheap applicability gate: an observed plate runs the ANPR path; no plate
falls back to general OCR; an explicitly governed request can ask for both.
Video V2 owns plate sampling/grouping and general OCR may run at its independent
bounded cadence. Ordinary analysts select no model manually. Live route plans
and roles remain unactivated.

`AutoRouting=SOURCE_READY_LIVE_UNACTIVATED`

# Processor Readiness

The shared fail-closed contract evaluates code, role enabled/admitted, worker
health, resource gate, backend, local asset existence, and hashes. States are
`READY`, `MODEL_REQUIRED`, `UNAVAILABLE`, or `NOT_RUN`; inference results are
then separately mapped to execution states. Activation preflight must verify
every tree/file before submitting any job.

# Result-State Truth

`NOT_RUN`, `PROCESSING`, `COMPLETE_ZERO_RESULTS`, `COMPLETE_RESULTS`, `FAILED`,
`UNAVAILABLE`, and `MODEL_REQUIRED` remain distinct. The video query path now
reads the explicit ANPR result state and no longer converts a generic completed
media job into a false zero-plate claim.

# Citation Readiness

Image ANPR cites original-pixel plate bounds/crop identity; OCR cites polygon,
bbox and reading order; Video V2 cites actual frame number/hash and source time
for observations plus first/last time for groups. No interpolated row becomes a
citation. Contract/unit tests pass; live click/seek/manual certification is
pending.

# Data UI Readiness

The existing Evidence Workspace already presents retained image previews,
bounded overlays/crops, candidate OCR, confidence labels, citation references,
and video time locators. No React analytical logic was added. Backend contracts
are source-ready; fresh panels, RTL rendering, video seek, responsive layout,
and error states remain post-activation manual acceptance gates.

# Ask Readiness

Existing typed operations `anpr.sightings`, `image.ocr_search`, and
`video.anpr_grouped_timeline` are reused with case/evidence scope. Focused and
full forensic API suites pass. These operations are not promoted; live API,
agent synthesis, citations, History, Activity, UI, and manual product acceptance
remain required.

`VisibleSuggestionsProductCertifiedOnly=PASS`

`NoSelfOracle=PASS`

`NoUnrestrictedSQL=PASS`

# Manual UI Acceptance Plan

The exact fresh-only plan is
`docs/demo/nexusai-nxmmr-anpr-ocr-vertical-manual-acceptance.md`. It covers
positive/negative image ANPR, `sample.mp4` timeline/seek/Ask, printed Urdu and
mixed OCR, English/Roman Urdu/Urdu questions, identifier order, citations,
result states, four viewport widths, console errors, resources, health,
Activity, retained invariants, and rollback triggers.

# Model Assets Required For Live Runtime

The exact sources, destinations, sizes/tree hashes, environment variables, and
license gates are in
`configuration/nxmmr_anpr_ocr_vertical_activation_v1.json`. They comprise five
ANPR files (image detector, shared FastPlate OCR/config, video plate detector,
video vehicle detector) and three Paddle model directories. Tesseract `eng/urd`
is an optional rollback floor. No new model/dataset download is proposed.

# Consolidated Activation Delta

One future activation copies only the hash-pinned assets into the existing
read-only host bind source `models/media`, enables image ANPR, Paddle OCR and
Video V2, builds/recreates only `forensic-records-worker` and
`forensic-records-api` with `--no-deps`, and runs fresh-only acceptance.
LocalAI/UI, PostgreSQL, NATS, named volumes, retained evidence, and existing
Activity are protected. Minimum gates are 6 GiB free RAM, 12 GiB free disk,
zero active jobs, exact hashes, healthy services, license/NOTICE closure, and a
passing one-time V2 reserved evaluation.

# Exact Activation Approval Required

Activation is **not yet requested** because two prerequisites remain: the exact
frozen V2 one-time reserved evaluation, and production license/NOTICE admission
for the two historical `.pt` checkpoints plus Paddle packaging. After those
close, the approval wording is: “Approve execution of
`configuration/nxmmr_anpr_ocr_vertical_activation_v1.json` exactly, including
model-file placement, dependency-capable builds, rollback tags, and recreation
of only the worker and forensic API.”

# Fine-Tuning Decision

`DEFER`. Current baselines are useful enough for source integration; measured
errors should inform a later separately governed training decision. No tuning
used sealed image or reserved video evidence.

# New ANPR Model Decision

`NO_NEW_MODEL`. Reuse the proven image FastALPR components and historical
hash-pinned video detectors. Resolve production licensing and validate the
frozen V2 on reserved events before activation.

# New OCR Model Decision

`NO_NEW_DOWNLOAD`. Select the already-cached Paddle detector plus English and
Arabic-multilingual recognizers for printed baseline use; retain Tesseract as
the low-memory floor. Require small independent real-world printed packs before
`REAL_WORLD_CERTIFIED`; handwriting remains out of scope.

`HandwrittenEnglishOCR=BENCHMARK_DATA_REQUIRED`

`HandwrittenUrduOCR=BENCHMARK_DATA_REQUIRED`

# Files Changed

Source changes are bounded to the shared readiness contract, image/video/OCR
processors and tests, worker Docker/Compose dependency/config wiring, scoped
ANPR/OCR query semantics and tests, local benchmark/verifier scripts, V2 config
and freeze, activation manifest, manual guide, this report, and continuation
records. Private fixtures/receipts remain under the Git-ignored
`local-acceptance-models` tree. Frozen V1 source/config were not changed.

# Tests

- dependency-free NX-MMR ANPR/OCR self-tests: 12 passed;
- Python compilation: passed for all new/modified processors and benchmark tools;
- focused Go operation/query/source-time tests: passed (`3.787s` final run);
- `go vet ./api/forensic_records`: passed;
- full `go test ./api/forensic_records -count=1 -timeout=180s`: passed
  (`121.689s`);
- JSON parse and freeze-digest verification: passed;
- activation source-hash manifest and V1 source/config preservation: passed;
- base forensic Docker Compose render: passed; the runtime overlay render was
  intentionally not supplied its required API secret during this source-only run;
- `pytest` was not available in the host Python, so no dependency was installed;
  the dependency-free runner covers the new unit contracts.

# Regression

Image pipeline parity remains exact and the sealed image result is untouched.
Video V1 source/config hashes and freeze digest remain unchanged. Full forensic
API regression passes. No frontend source changed. Live/manual product
regression remains pending because deployment was explicitly forbidden.

# Git Diff Check

`git diff --check` passes for tracked changes and the explicit trailing-whitespace
scan passes for the new source/document set. Existing Windows line-ending
conversion warnings remain informational. The worktree was already materially
dirty; no user file was reset, cleaned, staged, committed, or pushed.

# Anti-Hardcode Scan

Production source contains no benchmark UUID, expected fixture answer, fixed
source time, known `sample.mp4` plate, or known local development plate. Test
fixtures and benchmark tools contain their own declared oracles only.

`ProductionHardcodeScan=PASS`

# Runtime Impact

None. No live environment, image, service, container, backend, model role, or
mount was changed.

`RuntimeMutated=false`

`LiveModelsChanged=false`

`DeploymentPerformed=false`

`NXB2ActivationPerformed=false`

# Retained Impact

None. The recent failed image was inspected read-only and was not reprocessed.

`RetainedStateMutated=false`

# Activity Impact

None.

`ActivityMutated=false`

# Database Impact

No write and no schema migration.

`DatabaseMigration=false`

# Volume Impact

No named or bind-mounted model volume was changed.

`VolumesChanged=false`

# Open P0

None at the source/local boundary.

`OpenP0=0`

# Open Vertical P1

1. Authorize and run the one-time exact frozen V2 reserved evaluation without
   tuning.
2. Close exact production licensing/NOTICE for both video `.pt` checkpoints and
   Paddle runtime/model packaging.
3. Address or explicitly accept the high V2 negative-frame FPR while preserving
   the strong group precision result.
4. Obtain 5–10 independent real-world printed samples per English, Urdu, and
   mixed stratum and score region/order/identifier quality.
5. Approve and execute the consolidated two-service/model activation, then pass
   live API/Ask/Data/citation/Activity/UI/manual acceptance.

# NX-MMR Status

`SOURCE_LOCAL_VERTICAL_COMPLETE_AWAITING_RESERVED_EVALUATION_LICENSE_AND_ACTIVATION`.
No capability is promoted directly to `PRODUCT_CERTIFIED`; the existing
certification ladder and all live gates remain intact. NX-B2.1, STT/TTS,
image-semantic, and face work remain paused.

# Exact Next Action

Approve one one-time, private, local, non-retained evaluation of **only** the
exact Video V2 freeze
`212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20`
on the already-locked eight reserved event components. The run must use no
network, no tuning, no source/config/model change, no image holdout, no retained
evidence, and must stop after reporting reserved event/exact/group/time-error/
false-positive/resource metrics. Activation remains a later separate approval.
