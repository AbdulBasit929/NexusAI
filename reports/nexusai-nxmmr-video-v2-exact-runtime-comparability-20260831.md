# Starting State

The requested exact-runtime workflow is **not blocked on RAM**. The scoped
resource correction is recorded, and measured available RAM passes it. However,
no inspected local environment satisfies the frozen runtime dependency pins.
No model inference, development replay or reserved scoring was run. This is a
runtime-admission blocker, not a measured model-quality failure.

Authority is the exact-runtime comparability directive with SHA-256
`a843de7f77bada032de623b773996495d1212bf379c8fd11cb0bd8f99af8d71d`.
The private audit receipt is
`local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/video-v2-exact-runtime-preflight-20260831.json`.

# Reserved Authorization Preserved

`ReservedEvaluationCount=0`;
`VideoANPRV2EvaluationPerformed=false`;
`CandidateFreezeUnchanged=true`;
`PostHocTuning=false`.

No reserved oracle/result rows were read, no reserved frame was scored, and no
policy selection loop was run. The one-time authorization remains unconsumed.
Only split metadata and model/video hashes were inspected; no image holdout
was accessed. The earlier preflight receipt remains intact.

# Resource-Gate Revision

The new scoped policy is
`configuration/nxmmr_video_v2_local_evaluator_policy.json`:

- class: `VIDEO_V2_LOCAL_EVALUATOR`;
- initial available physical RAM minimum: 3.0 GiB;
- one heavy evaluator at a time;
- absolute available physical RAM safety stop: 1.5 GiB;
- after exact-runtime development, derive the evaluator requirement from
  measured incremental peak plus at least 1.5 GiB headroom;
- stop/reclassify unexpectedly high usage before reserved admission.

This is registered policy, not a claim that an inference watchdog was exercised.
No inference ran. Other heavyweight classes and the future 6-GiB build/deployment
gate remain unchanged. No operator application closure is requested.

# Resource-Gate Evidence

Read-only Windows host measurements were **4.624435 GiB** initially and
**4.960541 GiB** at final inventory. Both exceed 3.0 GiB. These are point samples,
not a minimum-RAM measurement during inference. The prior 672.398-MiB peak was
an OCR replay process, not the complete frozen detector-plus-OCR runtime.

Exact evaluator incremental peak, process peak, child-process peak, CPU, wall
time and minimum host RAM remain unmeasured. No empirical post-run requirement
is fabricated. `VideoEvaluatorMinimumFreeRAMGiB=3.0`; policy allows
`ParallelHeavyModels=1`, while actual active model evaluators were zero.

# Frozen Runtime Verification

All 24 source/model/config/freeze/split/policy assertions pass, including
canonical freeze
`212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20`.
The candidate's declared runtime nevertheless differs from available runtimes:

| Runtime | Python | Torch | Torchvision | Ultralytics | NumPy | Result |
|---|---|---|---|---|---|---|
| Frozen requirement | 3.11 | 2.5.1+cpu | 0.20.1+cpu | 8.0.114 | 2.3.5 | Required |
| Existing worker image | 3.11.15 | 2.6.0+cpu | Missing | Missing | 2.3.5 | Not exact |
| Existing EasyOCR image | 3.11.15 | 2.2.2+cpu | 0.17.2+cpu | Missing | 1.26.4 | Not exact |
| Existing FastPlate image | 3.11.10 | Missing | Missing | Missing | 2.4.6 | Not exact |
| Personal vehicle venv | 3.10.11 | 2.5.1+cpu | 0.20.1+cpu | 8.0.114 | 2.2.6 | Not exact |
| Personal FastPlate venv | 3.13.14 | Missing | Missing | Missing | 2.3.5 | Not exact |

The worker image already contains matching FastALPR 0.4.0, FastPlateOCR 1.1.0,
and open-image-models 0.6.0. Its missing/wrong detector runtime cannot be
silently ignored. The bundled workspace Python is 3.12.13; Ubuntu system
Python is 3.14.4. No matching venv was found in the bounded Ubuntu search.

Existing pip HTTP-cache wheel metadata was inspected without extraction.
Torch 2.5.1+cpu and Torchvision 0.20.1+cpu are cached as
`cp310-cp310-win_amd64`; NumPy 2.3.5 is `cp313-cp313-win_amd64`.
Those compiled binaries do not satisfy a Python 3.11 environment. The matching
pure-Python Ultralytics/FastALPR/FastPlate/open-image-models wheels alone cannot
close the gap. No compatible pinned Python 3.11 Torch/Torchvision pair was
found in inspected caches. No version substitutions or installations occurred.

# Exact Runtime Development Replay

**Not run: exact runtime unavailable in inspected local environments.** The
actual frozen `PlateFrameProcessor`, threshold-64 preprocessing and selected
aggregation policy were left unchanged. Zero models loaded, zero development
frames analyzed and zero OCR calls. No older raw-BGR helper was substituted.

# Previous Development vs Frozen Runtime

| Metric | Previous raw-BGR development | Exact frozen runtime development |
|---|---:|---|
| Event recall | 1.000000 | Not measured |
| Exact precision | 0.904762 | Not measured |
| Exact recall | 0.791667 | Not measured |
| Exact F1 | 0.844445 | Not measured |
| CER | 0.178571 | Not measured |
| Group precision | 0.937500 | Not measured |
| Group recall | 0.882353 | Not measured |
| Group F1 | 0.909091 | Not measured |
| Boundary MAE / median / p95, seconds | 0.778033 / 0.512 / 2.764550 | Not measured |
| Mean signed first / last error, seconds | +0.161133 / -0.009733 | Not measured |
| Replay wall / CPU, seconds | 582.528 / 4990.219 | Not measured |
| Replay process peak, MiB | 672.398 | Not measured |

All old values are existing evidence, not new measurements. Deltas and
`RUNTIME_PARITY`/`MINOR_RUNTIME_DELTA`/`MATERIAL_RUNTIME_DELTA` classification
cannot be assigned without the exact-runtime run. Replay resource totals are
not an integrated single-candidate cost estimate.

# OCR Preprocessing Comparability

`LegacyDevelopmentOCRPreprocessing=RAW_BGR`.
`FrozenRuntimeOCRPreprocessing=GRAY_BINARY_INV_64`.

The earlier development `reocr()` passes raw BGR crops to FastPlate. The frozen
product's `PlateFrameProcessor` applies grayscale plus binary-inverse threshold
64 before calling OCR. The original development receipt does not prove this
exact integrated path. A later exact-runtime development run must become the
valid baseline before reserved results can support a generalization claim.

# Development Gate Decision

No empirical PASS or FAIL is assigned: execution has not occurred. The existing
prospective utility targets are recorded unchanged in the policy: useful event
coverage (preferred 0.75, strong 0.85), group precision 0.80, group F1 0.65,
exact F1 0.60 and no catastrophic source-time regression. Do not choose new
thresholds after seeing results. Reserved admission requires a demonstrated
development PASS, not merely successful dependency provisioning.

# FPR Metric Correction

`FPRMetricCorrection=PASS` for the documentation correction. The historical
0.652778 is **GroupSelectedNegativeFrameFPR**, calculated by the unchanged
scorer after `group_frame_rows`. It is not raw detector FPR. New supplemental
stage metrics must come from recorded detector/OCR/group output, without
rewriting the primary scorer or changing candidate filtering.

# Raw Detector FPR

Not measured for the exact runtime; zero detector calls. No surrogate supplied.

# OCR Candidate FPR

Not measured for the exact runtime; zero OCR calls.

# Group-Selected Negative-Frame FPR

Previous raw-BGR development: 47/72 = 0.652778. Exact-runtime development:
not measured. Reserved: not measured.

# Final False Group Count

Previous raw-BGR development scorer: one false unique normalized-plate group,
group precision 0.937500. Exact-runtime output is unmeasured. The scorer's
unique-plate groups must not be conflated with physical vehicles or individual
temporal output packets; report packet counts separately when available.

# Reserved Evaluation Not Consumed

No reserved inference or scoring was authorized to proceed without the
development gate. Count remains zero. None of the four post-scoring candidate
decisions is assigned, and no fifth quality state is invented.

# V2.1 Development Need

Not established. Runtime provisioning is needed; no measured preprocessing
regression has yet justified V2.1. Do not tune or alter the frozen candidate.
If the exact-runtime development test later shows material regression, preserve
the reserved set and prepare the bounded V2.1 proposal then.

# License / NOTICE Review

Deferred in accordance with the explicit ordering: only after candidate
scoring/decision. Existing checkpoint rights/NOTICE admission blockers remain
open; no new commercial, redistribution or licensing conclusion is asserted.
No downloads occurred. Runtime dependency availability is separate from model
license admission and from technical candidate quality.

# Image ANPR Status

Preserved prior evidence: `FreshImageANPRSourceReady=true`;
`ImageANPRLocalFunctional=PASS_WITH_OCR_LIMITATION`;
`ImageANPRStatus=SOURCE_READY_LIVE_MODEL_REQUIRED`.
The settled fresh-image failure remains
`LIVE_ANPR_OCR_ROLES_DISABLED_AND_MEDIA_MODEL_MOUNT_EMPTY`; not re-investigated.

# English/Urdu/Mixed OCR Status

Preserved: `PrintedEnglishOCR=FIXTURE_MEASURED_PASS`;
`PrintedUrduOCR=FIXTURE_MEASURED_WITH_LIMITATION`;
`MixedEnglishUrduOCR=FIXTURE_MEASURED_WITH_LIMITATION`;
`IdentifierPreservation=PARTIAL_14_OF_15_FIXTURES`.
Handwriting remains `BENCHMARK_DATA_REQUIRED`. No OCR research or model search.

# Activation Readiness

Not eligible. No new technical acceptance, license admission or product
certification. All live model/readiness, fresh processing, API/Ask/Data/citation/
Activity/UI/manual/security gates remain pending. The existing activation
manifest and its 6-GiB deployment gate are unchanged. NX-B2.1 stays paused.

# Exact Next Action

Provide an already-prepared runtime matching the frozen pins, or authorize
provisioning a private Python 3.11 evaluator with pinned dependency downloads.
The smallest identified route is to reuse the existing Python 3.11 worker image
as a read-only base and provide an isolated dependency overlay containing the
exact Torch 2.5.1+cpu, Torchvision 0.20.1+cpu and Ultralytics 8.0.114 stack plus
required dependencies. This would need separately authorized compatible wheel
acquisition, not a model download or a Docker build. No dependency plan is
claimed install-verified yet.

After that approval/provision: verify all pins/imports, record pre-run memory,
run exact frozen DEVELOPMENT only with the 1.5-GiB safety stop and complete
resource measurements, and decide comparability. Only a PASS allows the same
unchanged candidate to consume the reserved evaluation once. Then score,
decide candidate utility and review licenses. Stop before live activation.

# Files Changed

- New scoped evaluator policy JSON (not frozen candidate config).
- New private exact-runtime preflight receipt.
- This report.
- Resource-governance report: scoped override added; other gates unchanged.
- Continuation ledger: new active blocker and exact next action.

Frozen production source, models, scorer, split, original receipts and activation
manifest were not edited. Pre-existing user changes remain intact.

# Tests

24 freeze/hash/policy assertions passed. Package metadata checks cover the three
existing relevant images, both personal venvs, bundled Python, Ubuntu and local
wheel caches. All five protected live container IDs are unchanged after the
inventory. No model-dependent tests, unrelated builds or live acceptance ran.
Receipt/policy/report consistency, all 27 required non-reserved report sections,
unchanged 6-GiB deployment gate, unconsumed reserved boundary, privacy scan for
reserved IDs, new-file whitespace, and `git diff --check` pass. The private
receipt is Git-ignored. Metadata inspection is not an inference test.

# Runtime Impact

`RuntimeMutated=false` refers to the protected live runtime. Three temporary
metadata-only inventory containers ran with network disabled, read-only root,
256-MiB memory cap, 64-PID limit and no model/evidence mounts; all exited and
were automatically removed. No image build/pull, installation, live role change,
service stop or recreation occurred. A read-only Ubuntu environment inventory
was also performed. `DeploymentPerformed=false`; `NXB2ActivationPerformed=false`.

# Retained Impact

`RetainedStateMutated=false`; `ActivityMutated=false`;
`DatabaseMigration=false`; `VolumesChanged=false`.
No retained evidence, DB or named volume was accessed by the inventory runs.

# Open P0

No new P0 demonstrated. Reserved-output safety remains untested; no promotion.

# Open Vertical P1

Exact frozen runtime availability; exact-runtime development comparability and
measured memory requirement; subsequent one-time reserved evaluation; checkpoint
license/NOTICE admission; live product acceptance. Missing runtime is not a
measured architecture rejection or a V2.1/V3 precision failure.
