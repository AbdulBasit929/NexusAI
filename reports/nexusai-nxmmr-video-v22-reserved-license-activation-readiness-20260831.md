# Verified Starting State

Authorization SHA-256:
`47be79708176cbb2caa1a7d9db0154e6f0ea0c0ad004b8c5d5273aa24f831626`.
The named candidate and local freeze both identify
`NX-MMR-VIDEO-ANPR-PRODUCT-BASELINE-V2.2`, `RAW_RGB`, canonical digest
`b0caae6d646e9d188ec5bc387585ac43aa7f1e22504d5b2d741699bab8d9f394`.
Before consumption, `ReservedEvaluationCount=0`; no prior V2.2 reserved receipt
or marker existed.

Historical evidence remains unchanged: original V2 is
`FUNCTIONAL_OCR_INPUT_CONTRACT_FAIL`; V2.1 is
`FUNCTIONAL_PASS_DEVELOPMENT_UTILITY_FAIL`; V2.2 development remains the frozen
RAW_RGB selection with event recall 1.0, exact F1 0.844445, CER 0.178571, group
precision 0.9375 and group F1 0.909091. Those are development metrics only.

# V2.2 Freeze Verification

`VideoV22FreezeVerified=true`. Before any reserved labels were opened, the
following matched: canonical digest, freeze/config file hashes, all 17 frozen
sources, five preregistered harness sources, scorer/helper hashes, CPython
3.11.15 interpreter binding, immutable base image, 64-entry runtime lock, frozen
77-entry effective-distribution map, four model/config hashes, development
projection, split/source-video digests, RAW_RGB input contract, frame/vehicle/
aggregation/normalization/resource policies. Ten no-model contract tests passed.

# Reserved Evaluation Consumption

`VideoV22ReservedEvaluation=FAIL` and `ReservedEvaluationCount=1`.
The immutable start receipt was written, then exactly eight allowlisted reserved
rows were lexically projected. Development rows were discarded before CSV
decoding. The authorization is consumed and the no-repeat guard is closed.

The evaluator stopped at runtime fingerprint verification before model loading:
the frozen map contains 77 entries, while an early `importlib.metadata` scan
exposed 64 top-level distributions. The 13 frozen-only names are setuptools
vendored distributions (`autocommand`, `backports.tarfile`,
`importlib-metadata`, `inflect`, four `jaraco.*` packages, `more-itertools`,
`platformdirs`, `tomli`, `typeguard`, `zipp`). A no-model diagnostic found no
version mismatch among the 64 common entries. Earlier import admission exposed
the extra 13 only after imports registered vendored metadata finders.

This is a preregistered harness fingerprint-order failure. It is not evidence of
a candidate/model quality failure. The harness was not repaired or repeated.

# Reserved Population

The locked partition contains eight events, 17 truth plate occurrences and 14
unique normalized truth strings. These counts came from the already-consumed
human projection; no prediction was used as truth. Model load count, decoded
reserved frames, scored frames and OCR calls are all **0**.

# Reserved Event Metrics

**UNAVAILABLE_NOT_SCORED.** Event TP, FN, recall and confidence interval do not
exist. Reporting development values here would be a false reserved claim.

# Reserved Exact Plate Metrics

**UNAVAILABLE_NOT_SCORED.** Exact TP/FP/FN, precision, recall, F1,
truth-occurrence accuracy and confidence interval do not exist.

# Reserved CER

`VideoV22ReservedCER=UNAVAILABLE_NOT_SCORED`.

# Reserved Group Metrics

**UNAVAILABLE_NOT_SCORED.** Group TP/FP/FN, precision, recall, F1, predicted
unique strings and temporal packet count do not exist.

# Reserved FPR Stage Metrics

`RawDetectorNegativeFrameFPR=UNAVAILABLE`;
`OCRCandidateNegativeFrameFPR=UNAVAILABLE`;
`GroupSelectedNegativeFrameFPR=UNAVAILABLE`.
There are no decoded negative frames and therefore no honest denominator.

# Final False Groups

`FinalFalseGroupCount=UNAVAILABLE_NOT_SCORED` and
`FinalFalseTemporalPacketCount=UNAVAILABLE_NOT_SCORED`.

# Reserved Source-Time Metrics

Matched groups, boundary MAE/median/p95, and signed first/last observation error
are **UNAVAILABLE_NOT_SCORED**. No sampled source observation exists.

# Reserved Resource Metrics

| Measurement | Failed pre-inference attempt |
|---|---:|
| Pre-run host available RAM | 4.803860 GiB |
| Minimum host available RAM | 4.767071 GiB |
| Final host available RAM | 4.822475 GiB |
| Process peak RSS | 29.496094 MiB |
| Sampled process peak RSS | 29.484375 MiB |
| Child-process peak RSS | 0 MiB |
| Incremental process-tree peak | 0.726563 MiB |
| Evaluator wall / CPU | 0.987217 / 0.162570 s |
| Total wall including projection/container | 3.916446 s |
| Frames / OCR calls / failed frames | 0 / 0 / 0 |
| Safety stop / container exit | No / 1 |

These are failure-boundary resources, not inference or live-throughput evidence.

# Development vs Reserved

| Metric | Development V2.2 | Reserved V2.2 | Delta |
|---|---:|---:|---:|
| Event recall | 1.000000 | unavailable | unavailable |
| Exact precision | 0.904762 | unavailable | unavailable |
| Exact recall | 0.791667 | unavailable | unavailable |
| Exact F1 | 0.844445 | unavailable | unavailable |
| CER | 0.178571 | unavailable | unavailable |
| Group precision | 0.937500 | unavailable | unavailable |
| Group recall | 0.882353 | unavailable | unavailable |
| Group F1 | 0.909091 | unavailable | unavailable |
| Final false groups | 1 | unavailable | unavailable |
| Raw detector negative-frame FPR | 0.847222 | unavailable | unavailable |
| OCR-candidate negative-frame FPR | 0.819444 | unavailable | unavailable |
| Group-selected negative-frame FPR | 0.652778 | unavailable | unavailable |
| Boundary MAE | 0.786367 s | unavailable | unavailable |
| Inference peak RSS / wall | 803.886719 MiB / 364.998796 s | unavailable | unavailable |

# Generalization Assessment

`INSUFFICIENT_SAMPLE_FOR_STRONG_GENERALIZATION_CLAIM_AND_NO_RESERVED_SCORE`.
No stable/degraded quality classification is possible. Even a successful eight-
event result from one video would not establish universal Pakistani ANPR.

# Technical Candidate Decision

`VideoV22TechnicalDecision=NOT_ASSESSABLE_NO_SCORE`. None of the three permitted
post-score outcomes can be selected honestly because their prerequisite—reserved
scoring—did not occur. V2.2 is neither accepted nor rejected on generalization.
Its canonical state is
`RESERVED_AUTHORIZATION_CONSUMED_TECHNICALLY_UNASSESSED`.

# Product Limitation Assessment

V2.2 remains a useful development candidate, but it cannot enter the live
integration module. It lacks independent reserved evidence and separately uses
license-sensitive YOLO assets. If a future candidate is evaluated, it requires a
new independent population; the consumed eight-event holdout cannot be reused.

# Post-Holdout Tuning Confirmation

`PostHoldoutTuning=false`. No preprocessing, threshold, model, cadence,
association, aggregation, normalization or source changed after label access.
No V2.3 or challenger was created.

# Image ANPR Status

`FreshImageANPRSourceReady=true`;
`ImageANPRLocalFunctional=PASS_WITH_OCR_LIMITATION`;
`ImageANPRStatus=SOURCE_READY_LIVE_MODEL_REQUIRED`. Image development was not
reopened. Output remains a candidate plate observation with bbox/crop, raw and
normalized OCR, confidences, source locator and review limitation—never owner or
confirmed vehicle identity.

# Printed English OCR Status

`PrintedEnglishOCR=FIXTURE_MEASURED_PASS`.

# Printed Urdu OCR Status

`PrintedUrduOCR=FIXTURE_MEASURED_WITH_LIMITATION`.

# Mixed English/Urdu OCR Status

`MixedEnglishUrduOCR=FIXTURE_MEASURED_WITH_LIMITATION` and
`IdentifierPreservation=PARTIAL_14_OF_15_FIXTURES`. Handwriting remains
`BENCHMARK_DATA_REQUIRED`. Fixture evidence is functional/demo readiness, not
real-world certification.

# License / NOTICE Matrix

This is engineering evidence, not legal advice.

| Asset | Code license | Weight/model license | Commercial/redistribution evidence | Admission |
|---|---|---|---|---|
| `license_plate_detector.pt` | Surrounding repo MIT | No checkpoint-specific statement; YOLOv8-trained | Dataset v4 says CC BY 4.0; Ultralytics model obligations remain | LEGAL_REVIEW_REQUIRED |
| `yolov8n.pt` + Ultralytics 8.0.114 | AGPL-3.0 or commercial terms | Ultralytics says trained models default AGPL-3.0 | Proprietary/internal/SaaS route requires applicable commercial license per vendor guidance | LEGAL_REVIEW_REQUIRED |
| FastALPR 0.4.0 | MIT | delegates models | MIT notice | ADMITTED_WITH_NOTICE |
| FastPlateOCR 1.1.0 | MIT | separate CCT checkpoint | MIT notice for code | ADMITTED_WITH_NOTICE |
| CCT-XS-v2 ONNX/config | MIT repository | no separate checkpoint statement found | confirm exact release-asset rights | CONDITIONAL |
| open-image-models 0.6.0 | MIT | separate YOLOv9 checkpoint | MIT notice for code | ADMITTED_WITH_NOTICE |
| Image YOLOv9 ONNX | MIT repository | no separate checkpoint statement found | confirm training/checkpoint rights | CONDITIONAL |
| PaddleOCR 3.7.0 | Apache-2.0 | separate official checkpoints | license/changed-file/NOTICE obligations | ADMITTED_WITH_NOTICE |
| PaddlePaddle 3.3.1 | Apache-2.0 | n/a | Apache-2.0 obligations | ADMITTED_WITH_NOTICE |
| Three PP-OCRv5 model directories | Apache project | cache has no LICENSE/NOTICE | capture official URL/version and confirm checkpoint grant | CONDITIONAL |
| Tesseract + `tessdata_fast` eng/urd | Apache-2.0; Leptonica BSD | `tessdata_fast` states all data Apache-2.0 | package license notices | ADMITTED_WITH_NOTICE |

Primary evidence: the [Ultralytics licensing page](https://www.ultralytics.com/license),
[FastALPR MIT license](https://github.com/ankandrew/fast-alpr/blob/master/LICENSE),
[FastPlateOCR MIT license](https://github.com/ankandrew/fast-plate-ocr/blob/master/LICENSE),
[open-image-models MIT license](https://github.com/ankandrew/open-image-models/blob/master/LICENSE),
[PaddleOCR Apache-2.0 license](https://github.com/PaddlePaddle/PaddleOCR/blob/main/LICENSE),
[PaddlePaddle Apache-2.0 license](https://github.com/PaddlePaddle/Paddle/blob/develop/LICENSE),
[Tesseract license](https://github.com/tesseract-ocr/tesseract/blob/main/LICENSE),
and [tessdata_fast](https://github.com/tesseract-ocr/tessdata_fast).

# Video Plate Detector Admission

`license_plate_detector.pt` SHA-256 remains
`8ec3b254a6c87610f037a90957462cafa11a9c03224e33a28c6a1d1ac2ac51b0`.
The local snapshot has no `.git`, so its exact source revision is unrecoverable.
The upstream repository includes the checkpoint and an MIT license, but that
does not settle checkpoint rights. Its README says the detector was trained
with YOLOv8 on [Roboflow v4](https://universe.roboflow.com/roboflow-universe-projects/license-plate-recognition-rxg4e/dataset/4),
which identifies CC BY 4.0. Because Ultralytics states trained models default to
AGPL-3.0, admission is `LEGAL_REVIEW_REQUIRED`.

# Ultralytics / YOLO Admission

Ultralytics 8.0.114 package metadata is AGPL-3.0. The official current licensing
page says AGPL is appropriate when the full project is open-sourced under its
requirements, while private/internal/proprietary/commercial/SaaS use without
that route requires an Enterprise license. No enterprise agreement or approved
AGPL compliance posture is recorded. `yolov8n.pt` and the YOLOv8-derived plate
checkpoint are therefore `LEGAL_REVIEW_REQUIRED`, not production-admitted.

# FastALPR / FastPlate Admission

The three Python libraries are MIT and admitted with retained notices. The
image detector and CCT OCR release weights remain conditional because the model
zoo/release evidence inspected did not contain a checkpoint-specific grant.
Code license is not being substituted for model license.

# PaddleOCR Admission

PaddleOCR and PaddlePaddle code are Apache-2.0 and admitted with license/NOTICE
packaging. The three existing official-model cache directories match their tree
hashes and contain identifying `inference.yml`, but no LICENSE or NOTICE.
Checkpoint packaging remains conditional until the official source URL/version
receipt and applicability of Apache-2.0 to the exact payloads are recorded.

# Remaining License Ambiguity

`VideoLicenseAdmission=LEGAL_REVIEW_REQUIRED`;
`ImageANPRLicenseAdmission=CONDITIONAL`;
`PaddleOCRLicenseAdmission=CONDITIONAL`. Exact unresolved items are checkpoint
rights for both ankandrew release assets, the Ultralytics commercial/AGPL path,
the YOLOv8-derived plate checkpoint, and the three PP-OCRv5 payload receipts.

# Modular Activation Plan

The prepared manifest is
`configuration/nxmmr_anpr_ocr_activation_readiness_v22.json`:

- Module A: Image ANPR, source-ready but license-conditional.
- Module B: Video V2.2, disabled; technical evidence and licenses blocked.
- Module C: printed English/Urdu/mixed OCR, technically source-ready but official
  checkpoint license/source receipts conditional.

Only the worker and forensic API may later be built/recreated with `--no-deps`.
LocalAI/API, PostgreSQL and NATS must not be recreated. The future gate remains
6 GiB free RAM, 12 GiB disk, zero jobs, healthy protected services, exact hashes,
license/NOTICE closure and explicit activation authority.

# Combined Activation Option

`CombinedActivationReady=false`. The package is prepared, but Video has no
reserved technical score and its YOLO licensing is unresolved; Image/OCR model
payload rights are also conditional.

# Partial Activation Option

`PartialActivationReady=false` today. Image ANPR + printed OCR is the preferred
independent path after its checkpoint rights close. Its plan keeps
`FORENSIC_VIDEO_ANPR_V2_ENABLED=false`; Video does not block its technical work.

# Exact Model Placement Plan

| Module | Source -> destination | Identity |
|---|---|---|
| Image | local image YOLOv9 ONNX -> `models/media/open-image-models/yolo-v9-t-384-license-plate-end2end/...onnx` | 7,771,218 bytes; `888397...77a8` |
| Image/shared OCR | local CCT ONNX/YAML -> `models/media/fast-plate-ocr/cct-xs-v2-global-model/` | 3,344,292 + 1,725 bytes; frozen hashes |
| Video (disabled) | personal `.pt` files -> `models/media/video-anpr-v2/` | 6,241,454 + 6,534,387 bytes; frozen hashes |
| Printed OCR | three cache trees -> `models/media/paddleocr/<model>` | 4,923,617 / 7,993,991 / 8,146,716 bytes; frozen tree hashes |
| Fallback | eng/urd traineddata -> revisioned tessdata directory | 4,113,088 / 1,398,718 bytes; frozen hashes |

Targets are planned root:root, files `0444`, directories `0555`, exposed to the
non-root worker through the existing `/models/media:ro` bind. Runtime downloads
remain forbidden.

# Exact Environment / Role Changes

Partial future activation sets image ANPR and Paddle OCR flags/paths in the
manifest and leaves Video false. Combined future activation is not admissible.
ASR, face and embedding roles stay false. Ordinary analysts choose no model;
contextual routing remains authoritative.

# Rollback Plan

The exact current worker/API container and image identities are preserved in the
manifest. A future rollback restores those images, recreates only worker and
forensic API with `--no-deps`, restores all three role flags to false, leaves
retained evidence/Activity/PostgreSQL/NATS/named volumes/LocalAI/UI untouched,
and quarantines new assets only after destination revalidation.

# Image Fresh Acceptance Plan

After a separately approved activation: fresh positive Add Data -> PROCESSING ->
COMPLETE_RESULTS with bbox/crop/candidate/confidence/Data/Ask/region citation;
fresh true negative -> COMPLETE_ZERO_RESULTS; missing asset -> MODEL_REQUIRED
before job submission. The earlier retained failed car image is not reprocessed
without separate bounded authority.

# Video Fresh Acceptance Plan

Not executable now. If a future independently evaluated/licensed Video candidate
is admitted, validate classification, readiness, actual-frame groups, source
times, timeline/seek, Data, Ask, citation, coverage and limitations. Never expose
raw frame noise as confirmed identity.

# Urdu/Mixed OCR Acceptance Plan

Use one fresh printed Urdu and one mixed image plus an English control. Verify
regions, RTL/LTR logical text, numbers/dates/identifiers, reading order, Data,
Ask and source-region citations. Handwriting is excluded. A small independent
real-world pack remains required for `REAL_WORLD_CERTIFIED`.

# Data Readiness

`DataReadiness=SOURCE_READY_LIVE_UNACTIVATED`. Data and Ask must consume the same
governed ANPR/OCR/group contracts; React remains presentation-only.

# Ask Readiness

`AskReadiness=SOURCE_READY_LIVE_UNACTIVATED`. Existing scoped typed operations
remain; unrestricted SQL and hardcoded answers are prohibited.

# Citation Readiness

`CitationReadiness=SOURCE_READY_MANUAL_CLICK_SEEK_PENDING`. Image region, OCR
polygon/bbox, and actual video frame/time locators are source-ready.

# Activity Acceptance Requirement

Fresh authorized runs must produce truthful Activity transitions without
mutating historical/retained evidence. This remains a live acceptance gate.

# UI / Manual Acceptance Requirement

Fresh panels, RTL rendering, video seek, responsive layout, error/result states,
security scope and manual review remain mandatory before product certification.

# Certification Consequences

`ProductCertificationPerformed=false`; no module is `PRODUCT_CERTIFIED`.
`VisibleSuggestionsProductCertifiedOnly=PASS`; `NoSelfOracle=PASS`;
`NoUnrestrictedSQL=PASS`.

# What Can Proceed Independently

License/source-receipt closure and the prepared partial Image ANPR + printed OCR
module can proceed without Video. No ANPR preprocessing research is needed.

# Next Multimodal Breadth

After the partial vertical reaches live manual acceptance: English STT, Urdu STT,
English TTS, Urdu TTS, image similarity/search, then face candidate comparison.

# Files Changed

Five one-time harness/test files, private registration/consumption/projection/
failure receipts and closure, license matrix, proposed notices, modular
activation-readiness manifest, this report and continuation checkpoint. Frozen
candidate sources/configs/receipts were not changed.

# Tests

Ten no-model reserved-harness contracts passed after one pre-registration test-
expectation correction. Canonical/hash/policy preflight passed. The actual
one-time run correctly failed closed at its immutable fingerprint assertion.
No heavy frontend build ran.

# Anti-Hardcode

Candidate predictions/metrics are absent; no answer or reserved label is embedded
in product code/config/report. The evaluator uses the unchanged scorer and
allowlisted oracle projection. Harness hashes were registered before label access.

# Privacy

The reserved CSV and receipts remain under Git-ignored private benchmark storage.
No plate strings appear in logs, public configuration, notices or this report.
Only aggregate truth counts are reported. Image holdout and retained evidence
were not accessed.

# Runtime Impact

`RuntimeMutated=false`; `LiveModelsChanged=false`; `DeploymentPerformed=false`.
No install, download, build, restart or service recreation occurred.

# Retained Impact

`RetainedStateMutated=false`; `ActivityMutated=false`;
`DatabaseMigration=false`; `VolumesChanged=false`;
`NXB2ActivationPerformed=false`.

# Open P0

`OpenP0=3`: no reserved technical decision for Video; no approved Ultralytics/
plate-checkpoint production path; conditional Image/Paddle checkpoint rights.

# Open P1

Capture official release/source receipts and exact notices; later run fresh-only
Image/OCR live API/Data/Ask/citation/Activity/UI/manual acceptance; obtain a
small independent real-world Urdu/mixed pack after functional activation.

# EXACT NEXT REAL APPROVAL BOUNDARY

Do **not** authorize another run on this consumed reserved holdout. First close
the conditional Image/FastPlate/Paddle checkpoint rights with counsel or explicit
maintainer/vendor evidence. Then the next consequential choice is:

**APPROVE PARTIAL IMAGE ANPR + PRINTED OCR ACTIVATION** using exactly
`configuration/nxmmr_anpr_ocr_activation_readiness_v22.json`, with Video disabled,
the 6-GiB/12-GiB/zero-job gate, exact notices, rollback tags, and recreation of
only the worker and forensic API.

Video requires a separately governed independent evaluation population before
any combined activation decision; it may not reuse or tune against this holdout.
