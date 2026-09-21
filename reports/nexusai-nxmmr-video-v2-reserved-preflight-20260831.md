# Freeze Verification

**Evaluation stopped before model load.** Available host RAM was 3.539 GiB,
below the unchanged 4.5-GiB HEAVY benchmark admission floor. No reserved metric
was computed and the one-time authorization remains unconsumed.

All 24 hash/policy assertions pass, including the canonical freeze digest
`212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20`,
all 12 activation source/config hashes, benchmark and development receipt,
image/video/vehicle/FastPlate model hashes, exact video, split, and selected
aggregation policy. Runtime version admission remains unverified: no inference
runtime was loaded after the RAM preflight failed.

Private receipt:
`local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/video-v2-reserved-preflight-20260831.json`.

# Reserved Evaluation Authorization

One evaluation of only the eight locked reserved events is authorized. This
attempt decoded/scored zero reserved frames, opened no reserved labels, loaded
zero models, and performed no evaluation. Reading the locked split manifest
and hashing the source video did not consume the evaluation. No image holdout
was accessed. No previous reserved result receipt was found.

`ReservedEvaluationCount=0`; `CandidateFreezeUnchanged=true`;
`PostHocTuning=false`. Do not emit `VideoANPRV2ReservedEvaluation=PASS`.

# Pre-Registered Decision Gates

The private preflight receipt records the prospective user gates before any
reserved scoring: event recall preferred >=0.75/strong >=0.85; final group
precision >=0.80; group F1 >=0.65; exact plate F1 >=0.60; practical source
boundaries without catastrophic regression; distinct raw-frame and final-group
false-positive accounting. The primary unit is the final candidate group/event.

The unchanged scorer is `benchmark_nexusai_nxmmr_anpr.py:score_video` with
`benchmark_nexusai_nxmmr_parity_adapter.py:group_frame_rows`, as used in V2
development. File hashes are recorded. No metric definition was altered.

# Reserved Event Results

Not measured. Authorized population: eight events. TP/FN/recall unavailable.

# Reserved Exact Plate Results

Not measured. Truth plate count, TP/FP/FN, precision/recall/F1 and CER unavailable.
No oracle rows were opened to fill these values.

# Reserved Group Results

Not measured. Truth group count, TP/FP/FN, precision/recall/F1 and final packet
false positives unavailable. The existing scorer counts unique normalized plate
values, not physical vehicles or individual temporal group packets; a later
receipt must distinguish these units without changing the scorer.

# Reserved Negative-Frame Results

Not measured. Raw and grouped negative-frame counts/FPR are unavailable.

# Reserved Source-Time Results

Not measured. Boundary MAE, median, p95, signed start/end errors unavailable.

# Reserved Resource Results

Preflight free physical RAM: **3.539 GiB**. Required: **4.5 GiB**, a 0.961-GiB
shortfall. Measurement was a read-only Windows host query. Model loads, analyzed
frames and OCR calls: zero. Inference wall/CPU/peak RSS and coverage receipt are
not available; preflight is not a model resource measurement. The separate
6-GiB build/deployment gate was not weakened.

# Development vs Reserved

| Metric | Development V2 | Reserved V2 | Delta |
|---|---:|---|---|
| Event recall | 1.000000 | Not run | N/A |
| Exact precision | 0.904762 | Not run | N/A |
| Exact recall | 0.791667 | Not run | N/A |
| Exact F1 | 0.844445 | Not run | N/A |
| CER | 0.178571 | Not run | N/A |
| Group precision | 0.937500 | Not run | N/A |
| Group recall | 0.882353 | Not run | N/A |
| Group F1 | 0.909091 | Not run | N/A |
| Group-selected negative-frame FPR | 0.652778 | Not run | N/A |
| Raw negative-frame FPR | Not reported by selected V2 receipt | Not run | N/A |
| Boundary MAE, seconds | 0.778033 | Not run | N/A |
| Wall, seconds | 582.528 replay total | Not run | N/A |
| Aggregate CPU, seconds | 4990.219 replay total | Not run | N/A |
| Peak RSS, MiB | 672.398 replay process | Not run | N/A |

Development resources cover the replay comparison, not a single integrated
production inference run. They cannot substitute for reserved measurements.

# Historical Reference Context

The prior historical-final full-video reference reported 18/19 events and 24/41
exact plate matches. These are existing engineering context only, not a new
evaluation and not the same population as the eight reserved events.

# Generalization Assessment

No generalization conclusion is supported yet. A pre-scoring source audit found
an important qualification to the prior development claim: development
`reocr()` passes raw BGR crops directly to FastPlate; the frozen integrated
`PlateFrameProcessor` converts to grayscale and applies binary-inverse threshold
64 before OCR. The frozen config explicitly specifies that thresholded path.
Thus development and the authorized integrated candidate have different OCR
inputs despite using the same weights and scoring definitions. Do not call a
future difference pure holdout generalization without this disclosure, and do
not silently modify the frozen candidate to make the paths agree.

The development helper also runs a policy/context comparison grid. It is not
an eligible reserved-evaluation entry point. Resume through fixed frozen
primitives and the unchanged scorer only; never run its selecting `main()` on
reserved evidence.

# Product-Utility Decision

**Not assessed because scoring did not run.** None of the four candidate
decisions is selected; this is a preflight stop, not a fifth candidate state.
There is no evidence for acceptance, rejection, or a V3 precision decision yet.

# High-FPR Interpretation

The earlier selected development value 47/72 = 0.652778 came from
`score_video(events, group_frame_rows(...))`. It is therefore **after group
selection**, not raw detector FPR. The prior report's raw-frame interpretation
was inaccurate. A future receipt must preserve this metric for scoring parity
and separately report raw observations/detections, final candidate false
positives, and group precision. No new inference is needed to establish this
source-level finding; no suppression rate is asserted here.

# Product Semantics Verification

Frozen source remains unchanged: source-local clustering, observed-string
selection, raw OCR preservation, actual-frame timestamps, no persistent
tracking/interpolation/vehicle identity/ownership inference, and partial-sampling
warnings remain present. Empirical verification of reserved output semantics
and live analyst presentation is pending, not newly passed.

# Certification Consequence

No promotion. `PRODUCT_CERTIFIED` and new real-world certification are not
supported by this preflight. The development-only evidence and all later
product-acceptance gates remain in force.

# Image ANPR Status

`FreshImageANPRSourceReady=true`;
`ImageANPRLocalFunctional=PASS_WITH_OCR_LIMITATION`;
`ImageANPRStatus=SOURCE_READY_LIVE_MODEL_REQUIRED`.
These are preserved prior statuses, not new tests. Image holdout untouched;
the settled disabled-role/empty-live-model explanation was not reopened.

# Urdu/English/Mixed OCR Status

Preserved: `PrintedEnglishOCR=FIXTURE_MEASURED_PASS`;
`PrintedUrduOCR=FIXTURE_MEASURED_WITH_LIMITATION`;
`MixedEnglishUrduOCR=FIXTURE_MEASURED_WITH_LIMITATION`;
`IdentifierPreservation=PARTIAL_14_OF_15_FIXTURES`.
Both handwritten English and Urdu remain `BENCHMARK_DATA_REQUIRED`.

# License/NOTICE Review

The requested post-evaluation review has not started because evaluation was
not admitted. Existing admission status remains **BLOCKED**, not newly resolved:
the exact `license_plate_detector.pt` and `yolov8n.pt` checkpoint rights,
Ultralytics production obligations, Paddle runtime/model NOTICE packaging, and
FastALPR/FastPlate asset-specific attribution/admission still require evidence.
Surrounding repository licenses are not proof of model rights. No commercial
permission, redistribution permission, exact NOTICE sufficiency or legal
admissibility is newly asserted. No downloads occurred.

# Remaining Production Gates

Reserved evaluation; production license/NOTICE admission; exact live
processor/model readiness; fresh processing; API/Ask/Data/citations/Activity;
responsive UI/manual acceptance; security/scope. Deployment resource admission
must be assessed separately from this benchmark.

# Activation Readiness

Not eligible for an activation approval yet. Image/OCR source readiness is
preserved independently; no live readiness or license admission is implied.

# Exact Activation Scope If Eligible

The unchanged proposed manifest is
`configuration/nxmmr_anpr_ocr_vertical_activation_v1.json`: exact local assets,
Image ANPR, Video V2, Paddle English/Urdu/mixed OCR, and only worker/API
build/recreation after separate approval. It is not authorized for execution.
Protected LocalAI, PostgreSQL, NATS, retained state and volumes remain untouched.

# V3 Need If Any

Undetermined. No reserved failure was measured. The preprocessing discrepancy
is a source-level comparability issue, not evidence of a reserved precision
failure. No V3 implementation or tuning is authorized or performed.

# Files Changed

Only this report, the private preflight/decision-rule receipt, and the new
continuation ledger entry. No candidate, config, scorer, model, activation
manifest or production source changed. The pre-existing dirty worktree remains
intact.

# Tests

24 hash/policy assertions passed; no earlier reserved result receipt found;
private receipt is Git-ignored. All five live container IDs match the prior
manifest before and after preflight. Receipt-to-report assertions, unconsumed
one-time boundary checks, all 31 required report sections, no reserved IDs/raw
result fields in the report, new-file whitespace, and `git diff --check` pass.
Rechecking the 12 frozen source/config hashes proves no new production hardcode
or other source edit was introduced; no gold-label scan was needed. Existing
line-ending warnings are informational. No model inference, model-dependent
test, unrelated build, installation or live acceptance test was run.

# Runtime Impact

`RuntimeMutated=false`; `LiveModelsChanged=false`;
`DeploymentPerformed=false`; `NXB2ActivationPerformed=false`.
No applications/services were stopped to reclaim RAM.

# Retained Impact

`RetainedStateMutated=false`; `ActivityMutated=false`;
`DatabaseMigration=false`; `VolumesChanged=false`.
No retained processing or DB access was performed. Container identity checks
do not independently prove all database contents; these markers describe this
attempt's non-mutating actions.

# Open P0

No new P0 demonstrated. No empirical reserved-output safety claim is made.

# Open Vertical P1

RAM admission; frozen runtime-version verification; fixed-candidate evaluator
entry point; development/product OCR-input comparability; correct raw-versus-
grouped FPR labeling; exact checkpoint licensing/NOTICE; remaining product gates.

# NX-MMR Status

Private reserved evaluation is authorized but not started. Source/local
image/OCR progress is preserved. No certification promotion; NX-B2.1 paused.

# Exact Next Action

Operator restores at least 4.5 GiB free physical RAM, then resumes this same
unconsumed evaluation authorization. Recheck memory and exact frozen runtime
before model load. Evaluate only the unchanged frozen candidate, disclose the
preprocessing mismatch, use the unchanged development scorer, and consume the
eight-event evaluation once. Do not change thresholds/preprocessing or use the
development policy-selection loop. Then perform the requested license/NOTICE
review and choose the evidence-supported candidate decision. Stop before any
activation.
