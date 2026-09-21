# Dependency Acquisition

**Provisioning passed; the frozen candidate failed functional OCR smoke.**
The exact isolated runtime is now available, but development and reserved
scoring were correctly withheld. The failure is a crop input-contract defect,
not insufficient RAM, missing packages, or a measured recognition-accuracy loss.

`DependencyProvisioning=PASS`. Sixteen unique compatible wheels, totaling
315,352,106 bytes (315.35 MB), were acquired only from the official PyTorch CPU
index and PyPI's file host. No arbitrary mirror, model, dataset or standalone
font download was used. All installation occurred offline in one private venv.

# Downloaded Package Manifest

The complete artifact receipt, including exact filenames, Python/ABI/platform
tags, source URLs/index, byte sizes and SHA-256s, is:
`local-acceptance-models/nxmmr/private-runtime/video-v2-cp311-20260831/downloaded-artifacts.json`.

| Package | Acquired version |
|---|---|
| torch | 2.5.1+cpu |
| torchvision | 0.20.1+cpu |
| ultralytics | 8.0.114 |
| matplotlib | 3.11.1 |
| opencv-python | 5.0.0.93 |
| pandas | 3.0.5 |
| scipy | 1.17.1 |
| seaborn | 0.13.2 |
| psutil | 7.2.2 |
| contourpy | 1.3.3 |
| cycler | 0.12.1 |
| fonttools | 4.63.0 |
| kiwisolver | 1.5.1 |
| pyparsing | 3.3.2 |
| python-dateutil | 2.9.0.post0 |
| six | 1.17.0 |

NumPy, FastALPR, FastPlateOCR and open-image-models already matched the freeze
in the immutable base and were not downloaded again. The initial commentary's
count of 17 was corrected by the completed manifest: the actual count is 16.

# Package SHA256s

All 16 wheel hashes passed both acquisition verification and an independent
host recheck. Installation used `--no-index --no-deps --require-hashes` against
`overlay-requirements.lock`. Full wheel hashes:

| Package | SHA-256 |
|---|---|
| ultralytics | 6840b8b4053d72776d77f750fa5fabc1d58316526a5f90b965fea77887fa1989 |
| torch | 07d7c9e069123d5af08b0cf0013d74f680b2d8be7d9e2cf561a52c90c55d9409 |
| torchvision | a4153bcc9f6219596c65761aa00588704e91e3db91d911c251e614f6140ed047 |
| matplotlib | aee55e9041211bf84302ab55ec3965df18dd90ae19f8b58332a7feaf208bfe83 |
| opencv-python | c8de2dec111122a02e8beb28e16c31904992dfd6186560b142a92c71403c1039 |
| pandas | 2c0cf1dd9b55a22d105fc46c1b489af3bd42264fcba7c66297bf47a9a1d9c78a |
| scipy | 43af8d1f3bea642559019edfe64e9b11192a8978efbd1539d7bc2aaa23d92de4 |
| seaborn | 636f8336facf092165e27924f223d3c62ca560b1f2bb5dff7ab7fad265361987 |
| psutil | 076a2d2f923fd4821644f5ba89f059523da90dc9014e85f8e45a5774ca5bc6f9 |
| contourpy | 51e79c1f7470158e838808d4a996fa9bac72c498e93d8ebe5119bc1e6becb0db |
| cycler | 85cef7cff222d8644161529808465972e51340599459b8ac3ccbac5a854e0d30 |
| fonttools | d76ac49f929aecaf82d83250b8347e099d7aecba0f4726c1d9b6df3b8bb5fe18 |
| kiwisolver | 95a02752aa032eef4aed01cda6d9b687c669bd0396bf4519eef8bba22a286720 |
| pyparsing | 850ba148bd908d7e2411587e247a1e4f0327839c40e2e5e6d05a007ecc69911d |
| python-dateutil | a8b2bc7bffae282281c8140a97d3aa9c14da0b136dfe83f850eea9a5f7470427 |
| six | 4721f391ed90541fddacab5acf947aa0d3dc7d27b2e1e8eda2be8970586c3274 |

Manifest SHA-256:
`15c539eef07a8f50c5833688448e16784257a28175694bf79f1c96fde7670f27`.
Resolved runtime lock SHA-256:
`3880bdce39914f53537d4e231ef83348297acaeef635f889eb02104ff0c7c1b8`.

# Platform / Wheel Compatibility

Verified before acquisition: Linux, x86_64, CPython 3.11.15, implementation
`cpython`, cache tag `cpython-311`. Supported wheel tags are recorded in
`base-platform-fingerprint.json`. Every wheel's embedded tag intersects that
supported set. Torch/Torchvision are `cp311-cp311-linux_x86_64`; other compiled
wheels have compatible manylinux/abi3 tags. No cached CPython 3.10 Windows wheel
was used. Runtime platform is Linux/WSL2 x86_64 with glibc 2.41.

# Isolated Runtime Location

One private environment:
`C:\Users\sheik\Workspace\Office\Projects\NexusAI\local-acceptance-models\nxmmr\private-runtime\video-v2-cp311-20260831\venv`.

It is a Linux venv, usable as `/evaluator/venv/bin/python` in the existing base
image, not a Windows interpreter. Base image:
`sha256:be13dbec4bd9bb62c824140df3777b898d7ad2801a0e68cd512c00dafaf4f9ef`.
The base was read-only; its Torch 2.6.0 files were explicitly not uninstalled.
The venv shadows those with the exact 2.5.1+cpu version. Personal venvs,
host/global Python and running application environments were not modified.

The private wheelhouse, lock and venv are retained for reproducibility of the
bounded follow-up. They are not live model placement or retained evidence.

# Exact Runtime Fingerprint

| Frozen component | Admitted value |
|---|---|
| Python | 3.11.15 |
| Torch | 2.5.1+cpu |
| Torchvision | 0.20.1+cpu |
| Ultralytics | 8.0.114 |
| NumPy | 2.3.5 |
| FastALPR | 0.4.0 |
| FastPlateOCR | 1.1.0 |
| open-image-models | 0.6.0 |

The full 64-distribution resolved lock was verified before imports. The
post-import fingerprint records 77 visible effective distributions, module
origins and interpreter hash. The 13 additional entries are existing vendored
components of setuptools 79.0.1 exposed during imports, not extra acquisitions
or substitutions. The independent top-level packaging/typing-extensions/wheel
versions retain precedence over their vendored copies.

Fingerprint:
`local-acceptance-models/nxmmr/private-benchmarks/anpr-ocr-vertical-v1/exact-runtime/imports.json`.
Interpreter SHA-256:
`92d7e40ec50be176cb1b790c7568b7e08cd862137b5aa69f1413ba1967886b79`.

The video evaluator does not provision or claim readiness of unrelated worker
roles such as PaddleOCR. Their production dependency/admission gates remain
separate. The frozen video runtime fields and all actual evaluator dependencies
are accounted for in the lock, base image and effective fingerprint.

# Import / pip-check Results

`pip check`: exit 0, no broken requirements. All required imports succeeded;
all frozen package versions and the full top-level lock matched. Torch is a CPU
build, CUDA unavailable, and a CPU tensor allocation passed.

`ExactPython311Runtime=PASS`; `Torch251CPU=PASS`;
`Torchvision0201CPU=PASS`; `Ultralytics80114=PASS`.

Fontconfig printed unwritable-cache-directory warnings in the read-only
container. They did not fail imports, and no font was downloaded. They are not
the cause of the subsequent ONNX input-shape failure.

# Model-Load Smoke Test

**FAIL at the frozen OCR interface.** The exact plate detector, vehicle detector
and FastPlate ONNX model loaded from the existing hash-pinned assets. The
synthetic frame completed the actual `PlateFrameProcessor` call. A supplemental
synthetic 2-D crop then exercised the same `HashPinnedFastPlateOCR.read`
interface that receives the product's prepared crops, and ONNX rejected it:

```text
input dimension 1: got 1, expected 64
input dimension 2: got 64, expected 128
input dimension 3: got 128, expected 3
```

This was not a human-oracle test. The pinned config explicitly requires
`img_height: 64`, `img_width: 128`, `image_color_mode: rgb`. The frozen processor
emits a 2-D grayscale/binary-inverse crop and passes it directly to the
recognizer. Installed FastPlateOCR documents that in-memory arrays must already
match the config's color mode. The synthetic failure and source trace establish
that this boundary does not satisfy the RGB model input contract; a matched
product crop would encounter the same incompatibility.

No source, model/config, threshold, cadence, normalization or aggregation was
changed to make the smoke pass. No repeat smoke or evidence run was attempted.
The frozen Ultralytics loader also emitted the expected `torch.load` pickle
warning; only the explicitly authorized hash-pinned local checkpoints were used.

# Freeze Verification

Canonical digest remains
`212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20`.
The frozen source/config hashes, selected policy, benchmark source and original
development receipt hashes, plus model assets, all pass. Both admitted runtime
phases repeated source/config/model verification before model loading.

`CandidateFreezeUnchanged=true`; `PostHocTuning=false`.
Only separate provisioning/evaluator tools, private receipts, scoped resource
policy and continuation/report documentation changed.

# Resource Admission

Initial policy remains 3.0 GiB available RAM, one heavy evaluator at a time,
absolute stop at 1.5 GiB. Host monitoring sampled every 0.2 seconds; the
evaluator sampled process/child RSS every 0.1 seconds. No safety stop occurred.
The build/deployment gate remains 6 GiB. No operator application was closed.

# Exact-Runtime Development Replay

**NOT_RUN.** Mandatory functional smoke failed before development admission.
No development oracle projection was performed and no development video frame
was processed. No old raw-BGR replay was substituted. Consequently there are
no valid exact-runtime event/exact/CER/group/source-time accuracy metrics yet.

# OCR Preprocessing Comparison

Earlier development helper: `RAW_BGR` three-channel crop supplied to FastPlate.
Exact frozen product: `GRAY_BINARY_INV_64` two-dimensional crop supplied directly.

The issue is stronger than an unknown statistical preprocessing delta: the
array shape/color contract is incompatible. Old development event recall 1.0,
exact F1 0.844445, group precision 0.9375 and group F1 0.909091 remain historical
raw-BGR results, not measurements of the frozen thresholded processor. No new
accuracy deltas or generalization claims are made.

# Development Comparability Decision

`MATERIAL_RUNTIME_DELTA` denotes the demonstrated **functional execution
failure**, not a calculated accuracy drop.

`FrozenRuntimeDevelopmentGate=FAIL` because its mandatory functional prerequisite
failed. The empirical development scorer was **not run**. The pre-registered
numerical gates remain unchanged and unassessed: event recall 0.75/0.85,
group precision 0.80, group F1 0.65, exact F1 0.60 and practical source-time
quality without catastrophic regression.

No reserved candidate acceptance/rejection/V3 decision is assigned. This does
not establish that the model architecture lacks utility; it establishes a
bounded product adapter input-contract defect.

# Runtime Resource Measurements

| Measurement | Import admission | Synthetic model smoke |
|---|---:|---:|
| Pre-run available host RAM, GiB | 5.467899 | 5.332481 |
| Minimum available host RAM, GiB | 5.253571 | 5.011143 |
| In-process wall, seconds | 62.509504 | Not finalized on exception |
| Wall including container startup, seconds | 67.473803 | 59.062876 |
| Aggregate process CPU, seconds | 10.644209 | Not finalized on exception |
| OS-reported peak RSS, MiB | 351.140625 | Not finalized on exception |
| Sampled peak RSS, MiB | 353.441406 | Not finalized on exception |
| Sampled child peak RSS, MiB | 0.250000 | Not finalized on exception |
| Safety stop | No | No |

The failed smoke exited before its in-process resource snapshot was finalized;
the independent host resource receipt survived. Do not substitute import RSS
for model-loaded RSS. No retry was performed just to fill missing measurements.
The exact integrated evaluator's empirical memory requirement is still
unmeasured; it must be derived from the corrected development run's incremental
peak plus at least 1.5 GiB headroom. Import-only measurements do not justify
lowering any policy.

# FPR Stage Metrics

No development or reserved FPR was measured. The earlier 47/72 = 0.652778 is
preserved as `GroupSelectedNegativeFrameFPR`, not raw detector FPR.
`RawDetectorNegativeFrameFPR`, `OCRCandidateNegativeFrameFPR`, exact-runtime
`GroupSelectedNegativeFrameFPR`, `FinalFalseGroupCount` and `FinalGroupPrecision`
are unavailable for this failed functional gate, not zero.

The added evaluator records the three stage outputs separately and reuses the
unchanged primary scorer. Tests verify that the three rates can differ, zero
denominators remain unavailable, and unique normalized-plate groups are not
conflated with temporal output packet counts.

# Reserved Evaluation Preserved

`ReservedEvaluationCount=0`; `VideoANPRV2EvaluationPerformed=false`.
No reserved labels/results were opened, projected or scored. Only the shared
CSV header was inspected during harness preparation. The projection utility
was tested with synthetic text only and was never run on the real oracle.
No `.consumed` marker, development score receipt or reserved receipt exists.

# Exact V2.1 Need

Request **one development-only OCR input-contract correction**:

1. Preserve the current failed freeze and all evidence receipts.
2. Add the smallest explicit shape/channel adaptation at the FastPlate boundary:
   represent the existing thresholded 2-D pixels in the RGB model's three-channel
   format, without changing pixel values/threshold 64, weights, model config,
   cadence, vehicle containment, confidence/support, clustering or normalization.
3. Test that real frozen preprocessing reaches the recognizer with the expected
   shape; retain independent tests for raw OCR/provenance and observed selection.
4. Make evaluator failure-resource persistence reliable so exceptions retain
   process/child RSS and CPU, not only the host monitor receipt.
5. Run synthetic smoke and development-only comparability in this locked runtime.
   If functionally valid but quality still regresses, report it; do not tune.
6. Produce a new V2.1 freeze and return for its reserved-evaluation admission.
   Do not represent the old digest's authorization as permission for an altered
   candidate.

This correction is **proposed, not implemented**. It is not a V3 precision sweep,
a model search, or authorization to restore raw BGR preprocessing by stealth.

# License / NOTICE Review

Deferred: technical accuracy scoring did not occur, and the directive orders
this review after candidate scoring. No new production-license admission is
claimed. Exact checkpoint rights, FastALPR/FastPlate attribution and Paddle
runtime/model NOTICE gates remain open. Technical provisioning success is not
production permission or certification.

# Runtime Impact

`RuntimeMutated=false`; `LiveModelsChanged=false`;
`DeploymentPerformed=false`; `NXB2ActivationPerformed=false`.

These markers refer to the protected live runtime. One explicitly authorized
private evaluator was provisioned. Temporary acquisition/install/admission/
smoke and metadata containers exited and were removed. No Docker image was
built/pulled; all five protected live container IDs match the baseline. No
running worker/API, personal venv, live role or `models/media` file changed.

# Retained Impact

`ModelDownloads=0`; `RetainedStateMutated=false`; `ActivityMutated=false`;
`DatabaseMigration=false`; `VolumesChanged=false`.
No retained evidence processing, database access/migration or protected volume
mutation occurred. All private runtime/benchmark artifacts remain Git-ignored.
No stage/commit/push/PR operation occurred.

Image status stays `SOURCE_READY_LIVE_MODEL_REQUIRED`, with prior local
`PASS_WITH_OCR_LIMITATION` preserved. Printed English remains
`FIXTURE_MEASURED_PASS`; printed Urdu and mixed English/Urdu remain
`FIXTURE_MEASURED_WITH_LIMITATION`; identifier preservation remains
`PARTIAL_14_OF_15_FIXTURES`. Handwriting remains `BENCHMARK_DATA_REQUIRED`.
No image holdout access, OCR research, promotion or NX-B2.1 activation.

# Open P0

`OpenP0=0`: no new security/integrity/scope/retained-state failure demonstrated.
The pre-live OCR contract defect is a blocking vertical P1. Product-output
safety remains unaccepted, not certified by these smoke checks.

# Open Vertical P1

Frozen OCR input contract; missing failure-path process resource finalization;
exact corrected development comparability; subsequent reserved evaluation;
checkpoint/NOTICE admission; all live fresh processing/API/Ask/Data/citation/
Activity/responsive UI/manual/security acceptance gates.

# Exact Next Action

**Approve one V2.1 development-only OCR input-contract correction as scoped
above, preserving the reserved set and all frozen model/policy values.**
Reuse this admitted isolated runtime. Do not deploy, reopen image/OCR model
development, tune thresholds, or consume reserved authorization yet.

Changes: five new provisioning/evaluation/projection/monitor/test tools,
scoped resource-policy authorization/status, private runtime/receipts, this
report and continuation ledger. Ten non-oracle unit tests, package hash checks,
frozen-source/model integrity checks and whitespace checks pass. The functional
smoke failure is explicitly retained; no failed gate was bypassed.
