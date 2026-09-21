# NX-MMR reference-parity adapter V1 source-development closure

Status: `SOURCE_DEVELOPMENT_COMPLETE_CANDIDATE_EVALUATION_APPROVAL_REQUIRED`.

This report closes only the source-level, private, non-retained development
slice authorized for `NX-MMR-REFERENCE-PARITY-ADAPTER-V1`. The frozen candidate
has not been scored on the reserved video evaluation subset, registered in the
live processor factory, deployed, or promoted.

# Verified Starting State

- The independent human oracle remains locked at digest
  `88c6827c23fbed467f097843404ed09c947bf7f27432e5cb5fa8427be9f48567`.
- The accepted image and video benchmark history was not rewritten.
- A deterministic, model-output-independent interval partition was frozen
  before candidate inference: 11 development events and 8 reserved events.
  Overlap-connected events were kept together; therefore the nearest safe
  partition to the requested target contains 8, rather than 7, reserved
  events. Split digest:
  `a759572ba657d3385267abe06f3ae5f756708001759903b040b2d7741ad79a1b`.
- Candidate code used only the locked local source video and existing local
  model bytes. The two personal reference directories remained read-only.
- `ReferenceParityInvestigation=PASS`.

# Reference-Parity Decision Preserved

`ReferencePipelineMateriallyBetter=VIDEO` and `ImageReferenceParity=EXACT`.
Historical output remained comparison evidence, never an oracle. This slice
did not reopen the completed image holdout or alter the incumbent image path.

# Adapter Architecture

The adapter is a bounded processor implementation, not a second authority
system: bounded source decode -> deterministic frame schedule -> hash-pinned
plate detection -> optional per-frame vehicle containment -> crop-level OCR ->
short-lived temporal/spatial association -> deterministic candidate selection
-> existing ANPR observation/group contracts and coverage receipt. It is not
registered by the live worker factory.

# Existing Assets Reused

The implementation reuses the existing `forensics.anpr-observation/v1` and
`forensics.video-anpr-plate-group/v1` contracts, the registered
`video.anpr_grouped_timeline` product operation, NX-A1 observation authority,
the existing HEAVY video resource class, and already-local reference model
assets. No duplicate analyst operation was added.

# Model Hashes

| Role | File | Format/input | Backend | SHA-256 | Admission state |
|---|---|---|---|---|---|
| Dedicated plate detector | `license_plate_detector.pt` | YOLOv8 PyTorch / 640 | ultralytics 8.0.114, torch 2.5.1 CPU | `8ec3b254a6c87610f037a90957462cafa11a9c03224e33a28c6a1d1ac2ac51b0` | model license unverified; reference code MIT |
| EasyOCR detector | `craft_mlt_25k.pth` | PyTorch | EasyOCR 1.7.2, torch 2.5.1 CPU | `4a5efbfb48b4081100544e75e1e2b57f8de3d84f213004b14b85fd4b3748db17` | admission review required |
| EasyOCR recognizer | `english_g2.pth` | PyTorch | EasyOCR 1.7.2, torch 2.5.1 CPU | `e2272681d9d67a04e2dff396b6e95077bc19001f8f6d3593c307b9852e1c29e8` | admission review required |
| Optional vehicle detector | `yolov8n.pt` | YOLOv8 PyTorch / 640 | ultralytics 8.0.114, torch 2.5.1 CPU | `31e20dde3def09e2cf938c7be6fe23d9150bbbe503982af13345706515f2ef95` | admission review required |

Paths remain private and are injected by environment variable. Hash checks
occur before load, and implicit downloads are disabled.

# Video Detector

The dedicated 640-input detector is isolated to the video candidate and proved
usable with its pinned bytes. It does not replace the 384-input image detector.
`Dedicated640Detector=PASS_DEVELOPMENT_HASH_PINNED_LICENSE_ADMISSION_REQUIRED`.

# Image Detector Status

The current 384 FastALPR image path is unchanged. Development/reference
agreement remains 10/10 and sealed-holdout agreement 22/22. No image was
rescored and no image routing changed.

# Frame Scheduling Strategies Tested

The bounded development comparison covered fixed 2 FPS, fixed 4 FPS, and a
2 FPS coarse scan with an 8 FPS local refinement burst. The initial inference
receipts measured 66/120, 133/242, and 125/256 analyzed-frame/OCR-crop counts.
The adaptive run completed 59 actual refinement frames; its first receipt's
234 value counted scheduled refinements and is superseded by the corrected
replay receipt. Full 60 FPS / 3,600-frame OCR was not run.

# Selected Frame Policy

`FramePolicySelected=FIXED_4FPS_PLATE_ONLY_MAJORITY_SUPPORT2`.

Fixed 4 FPS with two-observation support was selected for the balanced
precision/recall tradeoff. Adaptive refinement consumed the crop ceiling,
produced more false-positive groups, and did not improve balanced product
utility. Bounds remain 65 seconds, 300 analyzed frames, 9 refinement frames per
trigger, 256 OCR crops, 128 active groups, 24 observations per group, 900
seconds, and one HEAVY model run at a time.

# Vehicle Context A/B

On the same 133 development frames, plate-only/support-2 yielded negative-frame
FPR 0.333333 and group TP/FP/FN 1/17/16. Vehicle-assisted/support-2 yielded
FPR 0.305556 and group TP/FP/FN 1/16/16. Both had event recall 0.818182 and
exact TP/FP/FN 2/12/22. Vehicle context took 186.639 seconds, used 223 OCR
crops, and peaked at 1,178.664 MiB.

# Vehicle Context Decision

`VehicleContext=OPTIONAL`. The marginal reduction of two negative sampled-frame
false positives and one group false positive does not justify a second YOLO
model, extra licensing/admission surface, and higher runtime in V1.

# Repeated OCR Design

Each actual crop can contribute at most one vote. Every observation preserves
the actual frame number and timestamp, source-frame and crop hashes, bounding
box, raw and neutral-normalized strings, OCR and detector confidence, and
processor/model provenance. Zero-result frames remain in scoring and coverage.

# Aggregation Strategies Tested

Best confidence, normalized majority, and confidence-weighted support were
replayed over the same development observations with minimum support 1, 2, and
3. Support 1 retained more detections but excessive false positives; support 3
over-pruned event recall. Majority and confidence-weighted selection tied for
the selected fixed-4-FPS/support-2 observations.

# Selected Aggregation

Neutral normalized majority with minimum support 2 was frozen. Deterministic
tie-breaking uses support, confidence-weighted support, best OCR confidence,
then lexical order. Alternatives and all supporting raw readings remain
available. `TemporalAggregation=PASS` for the governed implementation;
development product quality remains partial.

# Normalization

Normalization uppercases and removes whitespace/non-alphanumeric separators.
It performs no character substitution and never invents a character.
`UKNormalization=false`; `RawOCRPreserved=true`. Raw exact plate accuracy was
0.0 on this development slice, while neutral-normalized exact TP was 2/24,
confirming that OCR remains a material limitation rather than something to hide
with aggressive normalization.

# Ephemeral Association

Association is confined to one source, a maximum one-second gap, and local
geometry/text compatibility. It uses IoU/center-distance and bounded edit
distance, expires groups, and never persists or exposes an object identity.

# Tracking Semantics Rejected

`PersistentTracking=false`. SORT, cross-source identity, journey, ownership,
driver, and continuous-presence semantics are absent.

# Interpolation Semantics Rejected

`InterpolatedObservations=false`. Every counted observation corresponds to an
actually decoded and processed source frame.

# Source-Time Provenance

Group first/last times derive only from first/last supporting observations,
not inferred entry/exit times. Frame hashes, crop hashes, source locators, and
model hashes remain attached. `SourceTimeProvenance=PASS`.

# Coverage Receipt

The selected run reports 60 FPS, 60 seconds, 3,600 source frames, 133 decoded
and detector-analyzed frames, 0 refinements, 242 OCR crops, 3.6944% frame
coverage, 0 failed frames, partial sampling, and explicit limitations. Complete
zero, partial zero, processor failure, unavailable model, unsupported input,
and resource-stop states remain distinct.

# Development Benchmark

The frozen candidate was scored only on 11 development events containing 24
plate occurrences and 17 groups:

| Metric | Result |
|---|---:|
| Event detection recall | 0.818182 (9/11) |
| Count-proxy TP/FP/FN; F1 | 11/0/13; 0.628571 |
| Exact plate TP/FP/FN | 2/12/22 |
| Exact precision/recall/F1 | 0.142857 / 0.083333 / 0.105263 |
| Neutral-normalized CER | 0.583333 |
| Negative frames / FP frames / FPR | 72 / 24 / 0.333333 |
| Exact group TP/FP/FN | 1/17/16 |
| Exact group precision/recall/F1 | 0.055556 / 0.058824 / 0.057143 |
| Boundary mean/median/p95 absolute error | 0.296 / 0.296 / 0.4049 s |

No reserved-candidate metric was computed.

# Current Baseline Comparison

On the same development partition, the incumbent analyzed 33 frames and had
event recall 0.090909, exact TP/FP/FN 1/0/23, exact F1 0.080001, CER 0.958333,
negative-frame FPR 0.058824, and group TP/FP/FN 2/0/15 (F1 0.210526). The
candidate adds +0.727273 event recall, +1 exact TP, +0.025262 exact F1, and
improves CER by 0.375000, but introduces 12 exact false positives and has worse
group F1. This is a meaningful recovery, not an acceptance result.

# Historical Reference Comparison

The development-only historical final reference had event recall 0.909091,
exact TP/FP/FN 16/9/8 (F1 0.653061), CER 0.166667, and group TP/FP/FN 12/15/5
(F1 0.545454). Historical raw output had high event recall but 126 exact and
205 group false positives. The candidate is safer than copying raw historical
semantics, but remains far below the useful historical final OCR/group quality.

# False Positive Analysis

Support 2 reduced the selected fixed-4-FPS result from the support-1/high-volume
regime to 12 exact and 17 group false positives, but the remaining 0.333333
negative-frame FPR is still too high. Repeated short/non-plate readings,
unstable one-character alternatives, and geometry-local fragments dominate.
These are model/aggregation limitations, not evidence of a confirmed plate.

# Miss Analysis

Two of 11 development events remained undetected after aggregation and 22/24
plate values were not exactly recovered. The detector supplies substantially
more candidate evidence than the incumbent, but EasyOCR readings are unstable;
support filtering also trades away weak/single-frame correct observations.

# Resource Measurements

The authoritative selected run used 180.568560 seconds wall time, 184.828125
seconds CPU, and 1,154.211 MiB peak process RSS. Host free RAM was 5.585 GiB
before model load and 4.713 GiB after. The run remained inside the 4.5 GiB HEAVY
admission gate and one-heavy-run concurrency bound.

# Selected Candidate

`fixed-4fps-plate-only-majority-support2`, adapter version
`1.0.0-development`: 4 FPS, no adaptive refinement, no vehicle model, neutral
normalized majority, support >=2, and one vote per actual crop.
`ParityAdapterV1=PARTIAL`.

# Candidate Freeze Digest

`DevelopmentFreeze=PASS`.
`CandidateFreezeDigest=1dcfabf17a8b66165b6c1390e65a8784eb875d3650b1b7f27d328c9ad2f0bb13`.

- Freeze digest: `1dcfabf17a8b66165b6c1390e65a8784eb875d3650b1b7f27d328c9ad2f0bb13`
- Frozen adapter source SHA-256: `e856439183d48a500cd9290326550de16c88fdc1100b09fb2e6e985c244b1413`
- Frozen configuration SHA-256: `4e1dee33a0cb117d7af8f5c94bf34f61560e2336bc065cb72ad7ed760ecb5297`
- Authoritative development receipt SHA-256: `bac845ea2a85d70475628c0086eb3067ce0610b82ed9034a9d8006e694ae20ba`

Earlier attempt/replay receipts are retained privately as superseded audit
history. They are not the candidate freeze authority.

# Certification State

No operation, processor, model, query, or suggestion was promoted.
`OperationPromotionPerformed=false` and
`VisibleSuggestionsProductCertifiedOnly=PASS`. The existing live API, Ask,
Data, citation, Activity, UI, security, performance, and manual product gates
remain mandatory before any `PRODUCT_CERTIFIED` decision.

# Auto-Routing Consequences

No live or source-default routing was activated. The existing route planner
already represents video as HEAVY and unresolved media processors as
`MODEL_REQUIRED`; the frozen configuration supplies candidate role, model,
hash, resource, and readiness information for a future shadow-registration
slice. The live factory does not expose this candidate.

# Query/Ask Consequences

No operation or query template was added. The existing
`video.anpr_grouped_timeline` remains the intended owner, but cannot advertise
this candidate as certified or available. No Ask request was submitted.

# Data/UI Consequences

No React/UI change was needed. The structured groups already preserve candidate
text, alternatives, support, first/last actual times, best observation/crop,
provenance, and coverage required for a later governed timeline/drill-down.

# Candidate Evaluation Plan

After explicit authorization, evaluate exactly the frozen candidate once on
the 8 reserved event/interval components from the locked split. Compare it with
accepted incumbent and historical-final predictions restricted to the same
reserved scope. Compute event recall; exact plate precision, recall and F1;
CER; group precision, recall and F1; negative-frame FPR; boundary
mean/median/p95 error; wall/CPU/peak RAM; frames analyzed; and OCR calls.

# Exact Evaluation Scope

- Candidate/version: `NX-MMR-REFERENCE-PARITY-ADAPTER-V1` /
  `1.0.0-development` at the freeze digest above.
- Input: the locked local `sample.mp4`, restricted to the frozen reserved
  intervals and guard coverage; no image holdout and no external evidence.
- Reserved count: 8 overlap-safe event components; private IDs remain only in
  the ignored split receipt.
- Models: the three selected plate/EasyOCR hashes above; vehicle model excluded.
- Resource class: HEAVY, minimum 4.5 GiB free host RAM, one run at a time.
- Expected duration/peak: approximately 181 seconds and 1.2 GiB process RSS,
  with actual receipt required.
- Execution: local, network-disabled, private, non-retained; no Activity,
  database, volume, runtime, or live-model mutation.
- Rollback/removal: no runtime rollback is required. Private evaluation
  receipts can be removed from the ignored benchmark directory if separately
  requested; the source candidate remains reviewable and unregistered.

# Exact Approval Required

Authorize one and only one reserved-video evaluation of freeze digest
`1dcfabf17a8b66165b6c1390e65a8784eb875d3650b1b7f27d328c9ad2f0bb13`
under the exact scope above. Authorization must not include tuning, another
candidate, image-holdout rescoring, deployment, route activation, model/data
download, retained processing, or certification promotion.
`CandidateEvaluationPerformed=false`.

# Fine-Tuning Decision

`FineTuningRequired=UNDECIDED`. Fine-tuning was not performed and is not yet
justified without reserved/generalization evidence and a separately admitted
dataset.

# New Model Decision

`NewANPRModelRequired=false` for the immediate candidate-evaluation boundary.
The locked local assets are sufficient to evaluate the frozen candidate. Any
future replacement model decision remains evidence- and license-gated.

# New Dataset Decision

`NewANPRDatasetRequired=false` for the immediate candidate evaluation. A later
representative independent multi-video, negative-frame, boxed Pakistan plate
pack remains necessary for product certification, but acquisition is not
authorized here.

# Files Changed

- `configuration/nxmmr_reference_parity_adapter_v1.json`
- `ingestion/forensic_records/video_anpr_parity_adapter.py`
- `ingestion/forensic_records/tests/test_video_anpr_parity_adapter.py`
- `scripts/benchmark_nexusai_nxmmr_parity_adapter.py`
- `scripts/test_benchmark_nexusai_nxmmr_parity_adapter.py`
- `scripts/test_video_anpr_parity_adapter_unittest.py`
- this report and the living continuation/next-approval records.

Private split, development, replay, vehicle A/B, and freeze receipts remain
under the Git-ignored NX-MMR benchmark directory.

# Tests

Focused deterministic tests cover scheduler/refinement bounds, repeated and
conflicting OCR, deterministic ties, confidence outliers, raw/normalized
preservation, adjacent/large-gap behavior, duplicate frames, invalid/no OCR,
multiple plates, source time, bounded groups, coverage states, missing models,
no interpolation, and no persistent IDs.

- 33 dependency-free adapter/benchmark/human-verification Python tests passed
  in 3.008 seconds.
- `py_compile` passed for the adapter, benchmark, and focused test modules;
  frozen JSON configuration parsing passed.
- `go test ./api/forensic_records` passed, covering the existing route-plan,
  operation-contract, NX-A1/NX-B1 and unrestricted-SQL rejection gates.
- `go vet ./api/forensic_records` passed.
- Freeze/source/config/receipt integrity, reserved-untouched, private-receipt
  ignore, 51-label privacy, trailing-whitespace, and scoped diff checks passed.
- `pytest` is not installed in either the host Python or the read-only reference
  environment. It was not installed; the equivalent dependency-free closure
  suite passed instead.

# Git Diff Check

The final scoped `git diff --check` passed, and untracked slice files passed an
explicit trailing-whitespace scan. The dirty worktree is preserved; no stage,
commit, push, reset, clean, or unrelated-file restoration is part of this
slice.

# Anti-Hardcode Scan

Production candidate/configuration code contains no benchmark filename, gold
plate strings, human event times, historical CSV values, or expected benchmark
counts. A 51-label exact privacy scan across the candidate/configuration,
benchmark harness, report and ledgers found zero leaks. Private identifiers and
raw observations stay in ignored receipts.

# Runtime Impact

`RuntimeMutated=false`; `LiveModelsChanged=false`; `DeploymentPerformed=false`.
No service was installed, built, deployed, restarted, or recreated. The final
read-only audit found the same five pre-slice container IDs: worker
`296e0881d891`, records API `b5c13f96e67f`, LocalAI/API `321ba681774b`, NATS
`5c49e70d132a`, and PostgreSQL `f66e05a3b179`.

# Retained Impact

`RetainedStateMutated=false`. The adapter processed only private benchmark
evidence and is not registered in the retained ingestion path.

# Activity Impact

`ActivityMutated=false`. No Ask/API request or persisted execution history was
created.

# Database/Volume Impact

`DatabaseMigration=false`; `VolumesChanged=false`. No SQL or volume write was
performed.

# Open P0

`OpenP0=0` for this bounded source-development slice.

# Open NX-MMR P1

`OpenNXMMRP1=2`:

1. development exact/group precision remains below product-useful quality;
2. detector/OCR checkpoint licensing and formal model admission remain open
   before any registration or activation.

Reserved candidate evaluation is a separate approval gate, not a completed
quality claim.

# NX-MMR Status

Reference parity investigation is complete. Adapter V1 source development is
complete and frozen. Candidate evaluation is not started and requires explicit
authorization. NX-MMR remains open; NX-B2.1 remains paused.
`NXB2ActivationPerformed=false`.

# Exact Next Action

Obtain explicit authorization for the one-time, local, non-retained reserved
video evaluation of the exact frozen digest and scope stated above. Until then,
do not evaluate the reserved subset, tune or change the candidate, deploy,
register routing, promote certification, acquire models/data, process retained
evidence, mutate Activity/database/volumes, or start NX-B2.1.
