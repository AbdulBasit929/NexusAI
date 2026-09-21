# Starting State

The authorized bounded correction is complete. **The OCR channel contract and
resource recording pass, but V2.1 development utility fails.** No further
inference, tuning, candidate freeze, reserved evaluation or deployment followed.

Authorization SHA-256:
`744492b44e1f73060ec87d81d9be2eab5609b7869583b1f51337ff35ddc0f343`.
The same independent human gold and development split were used. Model output
was never an oracle. Technical quality, license admission and product acceptance
remain separate decisions.

# V2 Failure Preserved

`VideoV2OriginalFreezePreserved=true`.
`VideoV2OriginalState=FUNCTIONAL_OCR_INPUT_CONTRACT_FAIL`.
The original digest remains
`212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20`.
The original V2 production module, configuration, 12 frozen source/config files,
models and failure receipts are unchanged. The old candidate was not rewritten
to pretend it had passed.

# V2.1 Change Scope

A small versioned processor wraps the exact V2 factory/frame processor with an
authoritative RGB input adapter and transform provenance. It inherits the same
detectors, thresholding, scheduling and aggregation; it is not enabled in the
live factory. A shared evaluator finalizer records resources on success/error.
Harness changes select the separate V2.1 output directory, bind source hashes
before evidence inference and explicitly refuse V2.1 reserved mode.

No plate/vehicle/OCR model or model config changed. Cadence 4 FPS, detector size
640/confidence 0.25, required vehicle containment, source-local association,
one-second temporal gap, support >=3, best OCR confidence >=0.5, weighted support
>=1.5, length 4–10, clustering, normalization and observed-string-only selection
are unchanged. No enhancement, denoising, morphology or threshold search.

# OCR Channel Contract Fix

`VideoV21ChannelAdapter=PASS`.
`thresholded_rgb_input` is called by `RGBCompatibleFastPlateOCR.read` inside the
versioned product implementation, not replicated in a benchmark helper.
HxW and HxWx1 binary uint8 images become HxWx3 through deterministic channel
replication. Already-three-channel inputs must be achromatic and binary.
Colored raw input is rejected, not silently accepted as a fallback.

# Pixel Preservation Verification

`ThresholdPixelsPreserved=true`.
Every output channel is elementwise identical to the thresholded source plane;
the value set remains {0,255}. The actual frame-processor test checks source
intensities 64 and 65 across the unchanged inverse-threshold boundary.
Achromatic channels make RGB/BGR ordering immaterial to these thresholded
pixels; the interface is explicitly RGB-compatible, not new color evidence.

# Shape / Dtype Verification

`OCRInputShape=64x128x3`; `OCRInputDtype=uint8` in synthetic smoke.
The adapter requires positive H/W, uint8, supported rank/channels and produces
C-contiguous RGB. The pinned recognizer retains its own spatial resizing to
64x128; the adapter does not resize or change intensities. Tests cover HxW,
HxWx1, HxWx3, noncontiguous input, empty arrays, zero dimensions, wrong rank,
unsupported channels, nonbinary values, invalid dtype and raw-color rejection.
Malformed input produces a clear `ParityAdapterError` before ONNX.

# Provenance Change

`GRAY_BINARY_INV_64_REPLICATED_RGB` records:
source crop -> grayscale -> binary inverse threshold 64 -> replicated-to-RGB
interface representation -> FastPlateOCR.
Observation packets explicitly say that the representation adds no color
information. Raw OCR, normalized text, source frame/crop hashes, actual source
time and model hashes remain attached. No UK or character correction, tracking,
interpolation, vehicle identity or ownership inference was introduced.

# Failure Resource Recording Fix

`FailureResourcePersistence=PASS`.
A try/finally-style context records phase, exit/error state, CPU, wall and
process/child RSS. Success, injected processing failure and a simultaneous
recording failure are tested. The original processing exception survives; a
secondary recording failure is attached as a note rather than replacing it.
Real import/smoke/development success receipts finalized. Independent host RAM
receipts remain separate and record pre/minimum/final RAM. OS termination or
unmeasurable resource fields must still not be represented as zero.

# Exact Runtime Reuse

The same private Linux x86_64 CPython 3.11.15 evaluator was reused, with no
installation or acquisition. Torch 2.5.1+cpu, Torchvision 0.20.1+cpu,
Ultralytics 8.0.114, NumPy 2.3.5, FastALPR 0.4.0, FastPlateOCR 1.1.0 and
open-image-models 0.6.0 match. All 77 effective distributions and the interpreter
hash match the admitted fingerprint; the 64-distribution top-level lock is
unchanged. Its SHA-256 is
`3880bdce39914f53537d4e231ef83348297acaeef635f889eb02104ff0c7c1b8`.
Every evaluator container was network-disabled and used the existing immutable
read-only image. `pip check` remains the admitted PASS; no dependencies changed.

# Synthetic Smoke

`SyntheticSmoke=PASS`.
Both real detectors and the pinned FastPlate model loaded. An actual zero-frame
detector call completed; deterministic synthetic boxes then forced the actual
PlateFrameProcessor crop/threshold/RGB-adapter/recognizer path. FastPlate
received contiguous 64x128x3 uint8 without an ONNX shape error. Box injection was
synthetic smoke only, never evidence inference. OCR text correctness was not
evaluated and no synthetic output became an oracle. Model/config hashes are
recorded in `v21-development/smoke.json`.

Smoke wall was 58.906740 seconds (62.168654 including container startup), CPU
18.216461 seconds, process peak RSS 484.976563 MiB, and minimum host available
RAM 4.986732 GiB. Fontconfig cache warnings and the pinned loader's pickle
warning were preserved; neither caused execution failure or a download.

# Development Run

One run only: the same 11 human events, 24 plate occurrences and 17 unique
normalized truth strings. The unchanged scheduler analyzed 133 actual frames
from the locked development intervals, with 222 OCR calls, 215 nonempty raw
observations, 11 temporal packets and zero failed frames. The source is 60 s /
60 FPS / 3,600 frames; effective frame coverage is 3.6944%, explicitly partial.
No selected timestamp entered a reserved exclusion interval. No image holdout.

Gold digest:
`88c6827c23fbed467f097843404ed09c947bf7f27432e5cb5fa8427be9f48567`.
Split digest:
`a759572ba657d3385267abe06f3ae5f756708001759903b040b2d7741ad79a1b`.
Development receipt SHA-256:
`b36c7b93e3431be6854b96b3accaa1527e3ffd708fe29402b882fa9643355d31`.

# Development Event Metrics

Event TP/FN: **8/3**; event recall **0.727273** (Wilson 95% 0.434355–0.902539).
This registered metric means at least one group-selected prediction inside the
human event, not raw detector localization accuracy. Count-proxy TP/FP/FN is
10/0/14, precision 1.0, recall 0.416667, F1 0.588236. No boxed IoU oracle exists.

# Development Exact Metrics

Normalized exact TP/FP/FN: **9/2/15**.
Precision **0.818182**, recall **0.375000**, F1 **0.514286**;
CER **0.559524**. Neutral normalization only; raw-format exact accuracy is 0.
The existing scorer and rounding conventions were preserved.

# Development Group Metrics

Unique normalized-string group TP/FP/FN: **8/2/9**.
Precision **0.800000**, recall **0.470588**, F1 **0.592592**.
There are 10 predicted unique strings and 11 temporal packets. Neither count
represents physical vehicles or established identity.

# FPR Stage Metrics

| Stage | False negative-window frames / denominator | Rate |
|---|---:|---:|
| RawDetectorNegativeFrameFPR | 61/72 | 0.847222 |
| OCRCandidateNegativeFrameFPR | 58/72 | 0.805556 |
| GroupSelectedNegativeFrameFPR | 17/72 | 0.236111 |

Raw detector means plate detections before vehicle/OCR/aggregation selection;
OCR candidates are nonempty neutral-normalized raw readings before aggregation.
These are frame-presence rates in oracle-negative windows, not per-box detector
precision. `FinalFalseGroupCount=2`; `FinalGroupPrecision=0.800000`.

Of 11 temporal packets, 2 have a string absent from all development truth;
**3** fail the supplementary string-plus-time match. That match requires the
selected string in a human event and at least one actual supporting observation
inside that event. It does not establish vehicle identity. This supplementary
accounting does not modify the primary historical unique-string scorer.

Historical 0.652778 remains **GroupSelectedNegativeFrameFPR**, not raw detector
FPR. The reduction to 0.236111 coexists with lost true results and is not enough
to admit the candidate.

# Source-Time Metrics

Eight matched unique groups: boundary MAE **0.857438 s**, median **0.638500 s**,
p95 **2.059250 s**; mean signed first error **+0.638875 s**, last error
**−0.202000 s**. These are matched-result errors, not inferred full-source
entry/exit times. Missed groups do not enter these boundary statistics.
No gross clock/index regression is demonstrated on the matched subset, but
that subset differs from historical runs; it cannot offset failed utility.

# Resource Metrics

| Measurement | Development |
|---|---:|
| Pre-run host available RAM | 5.135277 GiB |
| Minimum host available RAM | 4.396755 GiB |
| Final host available RAM | 4.854031 GiB |
| Processing/scoring wall | 363.506403 s |
| Finalized process wall, including receipt work | 363.619466 s |
| Wall including container startup | 367.001276 s |
| Processing/scoring CPU | 1,028.834615 s |
| Finalized process CPU | 1,028.868533 s |
| OS process peak RSS | 801.968750 MiB |
| Sampled process peak RSS | 803.707031 MiB |
| Sampled child-process peak RSS | 0.000000 MiB |
| Initial process RSS | 29.023438 MiB |
| Incremental sampled process-tree peak | 774.683594 MiB |
| Host / process sampling intervals | 0.2 / 0.1 s |
| Safety stop / failed frames | No / 0 |

The measured incremental peak plus 1.5 GiB headroom is **2.256527 GiB** for this
bounded run. This is evidence, not a policy reduction: initial admission stays
3.0 GiB, one heavy evaluator, absolute available-RAM stop 1.5 GiB. The separate
6-GiB build/deployment gate is unchanged. Random-seek benchmark wall time is not
a certified streaming-worker latency or a multi-video production capacity claim.

# V1 vs V2.1

| Same development gold / scorer | V1 | V2.1 |
|---|---:|---:|
| Event recall | 0.818182 | 0.727273 |
| Exact TP/FP/FN | 2/12/22 | 9/2/15 |
| Exact F1 | 0.105263 | 0.514286 |
| CER | 0.583333 | 0.559524 |
| Group TP/FP/FN | 1/17/16 | 8/2/9 |
| Group precision | 0.055556 | 0.800000 |
| Group F1 | 0.057143 | 0.592592 |
| Group-selected negative-frame FPR | 0.333333 | 0.236111 |

V2.1 retains much better exact/group precision than V1 but lower event recall.
V1 uses EasyOCR/plate-only/majority-support-2; this is an architecture/recognizer
comparison, not evidence that channel replication alone caused improvement.
V1 boundary MAE 0.296 s is based on only one matched group, versus eight here.

# Historical Raw-BGR V2 vs V2.1

| Same development gold / scorer | Historical raw-BGR V2 | V2.1 |
|---|---:|---:|
| Event TP/FN | 11/0 | 8/3 |
| Exact TP/FP/FN | 19/2/5 | 9/2/15 |
| Exact precision / recall | 0.904762 / 0.791667 | 0.818182 / 0.375000 |
| Exact F1 | 0.844445 | 0.514286 |
| CER | 0.178571 | 0.559524 |
| Group TP/FP/FN | 15/1/2 | 8/2/9 |
| Group precision / recall | 0.937500 / 0.882353 | 0.800000 / 0.470588 |
| Group F1 | 0.909091 | 0.592592 |
| Group-selected negative-frame FPR | 0.652778 | 0.236111 |
| Boundary MAE / median / p95, seconds | 0.778033 / 0.512000 / 2.764550 | 0.857438 / 0.638500 / 2.059250 |

Exact F1 drops 0.330159 and group F1 drops 0.316499. The old integrated V2
failed before scoring; this run fixes that execution defect but does not match
the historical helper's recognition utility. The old helper re-read 216
vehicle-contained crops, versus 222 integrated OCR calls here, so the full
metric difference is **not a controlled crop-identical preprocessing A/B**.
No claim attributes every loss solely to thresholding or the OCR model.

The previously recorded development-only historical final reference has event
recall 0.909091, exact TP/FP/FN 16/9/8 (F1 0.653061), CER 0.166667 and group
TP/FP/FN 12/15/5 (precision 0.444444, recall 0.705882, F1 0.545454). It uses
historical tracking/finalization/interpolated-output semantics and much denser
sampling (1,094 negative frames versus 72), so its FPR/time metrics are not
direct stage-equivalent comparisons. Only its existing development summary was
read; no historical reserved output was projected or inspected.

# Development Gate Decision

| Pre-registered gate | Required | Measured | Decision |
|---|---:|---:|---|
| Event recall | >=0.75; strong >=0.85 | 0.727273 | FAIL |
| Final group precision | >=0.80 | 0.800000 | PASS |
| Final group F1 | >=0.65 | 0.592592 | FAIL |
| Exact plate F1 | >=0.60 | 0.514286 | FAIL |

`VideoV21Development=FAIL`; `VideoANPRV21Development=FAIL`.
Numerical equality with raw-BGR was not required. The actual registered utility
thresholds fail; no post-hoc gate was added or relaxed. Resource admission and
functional smoke pass but cannot rescue recognition utility.

# V2.1 Failure Class

**Pre-aggregation exact-reading deficits plus aggregation recall loss.**
All 11 events contain some raw OCR; three lose all group-selected output.
Of 24 truth occurrences, 9 are selected exactly, 10 have no exact raw reading
inside their event, and 5 have an exact raw reading but no selected exact result
inside their event. The latter may reflect insufficient/weak/competing support;
this audit did not change support thresholds or rerun alternative policies.
The former cannot be separated conclusively into localization/crop/OCR causes
without box-level evidence or a separately authorized crop-identical comparison.
There are also two final false unique strings and three string/time-false packets.

The contract defect is fixed, but the candidate is not development-admissible.
This is not a dependency, RAM or model-load failure and does not establish that
the overall video architecture lacks utility.

# Reserved Set Preserved

`ReservedEvaluationCount=0`; `ReservedEvaluationPerformed=false`.
No reserved label projection, candidate prediction, metric, error inspection
or visualization was performed. The shared oracle exporter lexically discarded
non-development payloads without CSV-decoding/returning them. Exactly 11
independent-human rows were supplied to inference. Schedule and completed-frame
audits independently confirm zero reserved-exclusion timestamps.

`VideoV21Frozen=false`; `VideoV21FreezeDigest=NONE`.
No V2.1 candidate configuration/freeze was created because development failed.
The pre-evidence input manifest is only an execution-integrity lock, not an
accepted candidate freeze. Original V2 remains unchanged and reserved use zero.

# Next Bounded Development Need

Propose a separately authorized, separately versioned **one-candidate raw-BGR
development parity run** in this exact runtime, keeping the same detector,
vehicle containment, cadence, crops, aggregation and normalization policies.
Compare against these saved V2.1 results; do not rerun/tune V2.1. This would
explicitly authorize a changed preprocessing candidate, not silently restore
raw BGR under the completed V2.1 approval. Historical raw-BGR results motivate
the proposal but do not guarantee its integrated result. No new models/datasets,
threshold search or reserved evaluation is needed for that bounded question.

This proposal is **not implemented or authorized**. If the intended thresholded
policy must remain mandatory, retain the failed status and request a different
bounded recognition investigation instead. Do not lower utility gates.

# Image ANPR Status

`FreshImageANPRSourceReady=true`;
`ImageANPRLocalFunctional=PASS_WITH_OCR_LIMITATION`;
`ImageANPRStatus=SOURCE_READY_LIVE_MODEL_REQUIRED`.
No image processing/tuning, image holdout access or live activation.

# Urdu/English/Mixed OCR Status

`PrintedEnglishOCR=FIXTURE_MEASURED_PASS`;
`PrintedUrduOCR=FIXTURE_MEASURED_WITH_LIMITATION`;
`MixedEnglishUrduOCR=FIXTURE_MEASURED_WITH_LIMITATION`;
`IdentifierPreservation=PARTIAL_14_OF_15_FIXTURES`.
Separate Paddle/general OCR source and models are unchanged; no new OCR evidence
run occurred. License/NOTICE review remains deferred until a technically valid
candidate passes separately authorized reserved evaluation.

# Files Changed

- Added `ingestion/forensic_records/video_anpr_product_v21.py`.
- Added `scripts/nxmmr_evaluator_resources.py`.
- Added `scripts/test_nxmmr_video_v21_contracts.py`.
- Added `scripts/audit_nxmmr_video_v21_development.py`.
- Updated the exact evaluator, host runner and development projection scripts.
- Updated this report and `NEXUSAI_CONTINUATION.md`.
- Added private input-lock, projection, runtime, smoke, development and audit receipts.

Old production/config/freeze files remain byte-identical. No general refactor,
API/Go/UI interface change or new live registration occurred.

# Tests

60 tests passed: 12 new offline channel/resource contracts; 10 evaluator tests;
14 parity/aggregation/source-time tests; 4 primary scorer tests; 8 parity
benchmark tests; 12 existing vertical contracts. Seven changed/new Python
modules compile. All 12 original source/config hashes, five local model hashes,
the full runtime fingerprint and pre-evidence input manifest pass verification.
Saved-output recomputation matches both primary scores and FPR stage receipts
exactly; no second inference was used. Private artifacts are Git-ignored.

The host audit utility initially hit Python 3.10's missing `hashlib.file_digest`;
its receipt hashing was corrected to portable SHA-256 and the audit completed.
This changed only the post-run audit tool, not candidate/scorer/runtime/input
manifest or predictions. Whitespace and source compilation checks pass. No Go
or frontend build was needed because no public interface changed.

# Anti-Hardcode

Production source contains no benchmark filename, event ID, gold value, source
time or expected metric. All 24 development plate values were checked against
the new production module; zero matches. No reserved label was read for this
scan. Synthetic test identifiers are confined to tests/evaluator smoke. Actual
observations preserve raw OCR without character repair or invented candidates.

# Runtime Impact

`RuntimeMutated=false`; `LiveModelsChanged=false`; `ModelDownloads=0`;
`PostHocTuning=false`; `DeploymentPerformed=false`; `NXB2ActivationPerformed=false`.
Only the authorized private evaluator ran. All temporary containers exited;
all five protected live IDs match the starting baseline. No Docker build/pull,
dependency install, environment edit, model placement or role enablement.

# Retained Impact

`RetainedStateMutated=false`; `ActivityMutated=false`;
`DatabaseMigration=false`; `VolumesChanged=false`.
No retained evidence processing, Ask/API mutation, database operation or named
volume change. No staging/commit/push/PR. Existing dirty worktree changes remain
preserved. Scratch artifacts are private benchmark records, not retained evidence.

# Open P0

`OpenP0=0` for this bounded slice: no new scope, integrity or retained-state
failure demonstrated. This is not a completed production security assessment.

# Open Vertical P1

Development recognition/aggregation utility; broader independent multi-video
and boxed localization evidence; a passing new candidate and separately approved
reserved evaluation; exact checkpoint license/NOTICE admission; and all live
API/Ask/Data/citation/Activity/UI/manual/security/performance acceptance gates.
No integration acceptance or `PRODUCT_CERTIFIED` promotion is claimed.

# Exact Next Action

Request explicit approval for the single raw-BGR development parity candidate
described above, or retain the thresholded candidate's FAIL state. **Do not
evaluate reserved, tune, deploy or start NX-B2.1 under this completed approval.**
