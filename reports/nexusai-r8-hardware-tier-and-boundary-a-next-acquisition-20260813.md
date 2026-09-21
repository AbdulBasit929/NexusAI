# NexusAI R8 hardware-tier and Boundary-A next-acquisition decision

Date: 2026-08-13  
Decision scope: R8.3/R8.4 evaluation governance only  
Production mutation: none  
Model acquisition: FastPlateOCR 5,263,955-byte package, isolated only  
Dataset download in this slice: none

## Verified state

- R8 remains active; R8.5 has not started.
- Boundary A and Boundary B remain blocked. Thresholds remain detector
  precision/recall >= 0.95 and OCR normalized exactness >= 0.90 with CER <=
  0.05, plus the existing calibration, abstention, provenance, resource,
  privacy and reproducibility gates.
- Production containers, roles, profiles, retained evidence, cases, databases,
  models and named volumes were not changed.
- The four acquired challenger packages remain evaluation-only. OMZ 0106,
  EasyOCR `arabic_g1` and PARSeq-tiny failed the sealed T2-V gate. OMZ 0123 was
  not converted or evaluated.

## D0 development hardware reconciliation

The new read-only inventory records Windows 11 Pro, Intel Core i7-1260P (12
physical/16 logical processors), 15.713 GiB installed RAM, Intel UHD integrated
graphics, no NVIDIA/CUDA runtime, 1,716.338 GiB free on C:, Docker Desktop
29.6.2 and a Linux Docker VM exposing 16 CPUs and 7.611 GiB RAM. Available host
RAM at inventory time was 5.31 GiB and is an observation, not a capacity claim.

This machine is `D0`: CPU-first development/evaluation. Intel UHD and WMI's
shared-memory value do not qualify a stable accelerator. DirectML is
`unqualified`; OpenVINO is supported only through the already isolated Docker
CPU evaluator until a repeatable native lane is separately proven.

## Hardware-tier architecture

| Tier | Purpose | Required behavior |
|---|---|---|
| D0 | Current CPU developer machine | Small/bounded CPU inference; fail preflight on RAM/disk; no GPU claim |
| D1 | Capable CPU/iGPU developer | Optional OpenVINO/DirectML only after measured qualification |
| P1 | NVIDIA production candidate | Pinned CUDA/TensorRT/driver matrix; benchmarked batch/concurrency and fallback |
| P2 | Higher-throughput production | Approved GPU pool, queue/backpressure/SLO and cost controls |

The API and specialist agent remain hardware neutral. They request stable roles
(`plate_detector`, `latin_plate_ocr`, `arabic_urdu_plate_ocr`) from the model
governance layer. The resolver selects an approved backend for the measured
hardware tier; it never exposes CUDA/OpenVINO/TensorRT details to the analyst.
No tier may silently substitute an unevaluated candidate.

Every future evaluation record must identify hardware profile, execution
provider, precision, device, thread count, batch size, warmup/repetition count,
peak RSS, latency distribution, package/image digests and offline
reproducibility. Historical results remain immutable and are not rewritten.

## Existing evaluation reuse and failure audit

The recent negative results are valid evidence, not discarded attempts:

- OMZ 0106 uses the documented 300x300 BGR NHWC path, class 2, fixed 0.50
  threshold and correct normalized-coordinate reconstruction. Zero T2-V true
  positives is a domain failure, not a scoring bug.
- PARSeq-tiny uses its pinned official source/weight digests, 32x128 bicubic
  input and published normalization. Raw and NFKC/uppercase/alphanumeric
  outputs are retained; no O/0 or I/1 substitution occurs.
- EasyOCR 1.7.2 uses the exact `arabic_g1` archive member, CPU mode,
  detector-disabled recognition over the same immutable crops and greedy
  decoding. It retains raw and governed normalized strings.
- Development, validation and sealed holdout partitions are unchanged.

None of these failures is invalidated by a material evaluator, preprocessing,
threshold or partition defect. Re-running them without a new hypothesis would
be wasteful and is prohibited.

## OMZ 0123 disposition

`vehicle-license-plate-detection-barrier-0123` is deferred without conversion.
It shares the front-facing Chinese barrier-camera domain and minimum-plate-width
assumptions of the correctly evaluated 0106 family, which produced zero recall
on T2-V. Its legacy TensorFlow-to-IR path adds conversion risk but no evidence
of the material Pakistan/general-scene gain required to justify it. It may
return only with a materially different capability hypothesis.

## Stronger candidate research and ranked matrix

| Rank | Role | Candidate | D0 | Future GPU | Decision |
|---:|---|---|---|---|---|
| 1 | Latin/alphanumeric OCR | FastPlateOCR `cct-s-v2-global` v1.1.0 | ONNX CPU/OpenVINO eligible | CUDA/TensorRT eligible | Selected next small evaluation |
| 2 | Detector | NVIDIA TAO LPDNet 2.3.1 | No | TensorRT/DeepStream | P1 research only; exact artifact/terms pending |
| 3 | Detector | OMZ 0123 | Conversion possible | No material benefit shown | Defer |
| 4 | Urdu/Arabic OCR | Paddle Arabic control | Measured CPU | Backend-dependent | Retain control; not accepted |
| 5 | Urdu/Arabic OCR | EasyOCR `arabic_g1` | Measured CPU | PyTorch CUDA possible | Reject on T2-V |

FastPlateOCR is selected because it is plate-specific, has a publisher global
model trained over a broad plate distribution, and exposes the same ONNX model
to CPU, OpenVINO, DirectML, CUDA and TensorRT providers. Exact proposed files:

- `cct_s_v2_global.onnx`: 5,262,230 bytes; SHA-256
  `384bbbd2cea3ef54761d3df70822ef3a349ee1a112aeafddbe0e3ba06bc6e47b`.
- `cct_s_v2_global_plate_config.yaml`: 1,725 bytes; SHA-256
  `0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6`.
- Total transfer: 5,263,955 bytes (about 5.02 MiB).
- Publisher revision: v1.1.0, commit
  `9ce7a5b64a939aa421c243b331d42e6bc25ffd44`.
- Project license: MIT. Asset-specific redistribution must still be confirmed
  and NOTICE/source receipts retained before any promotion.

The package was acquired after preflight and independently verified against
both publisher SHA-256 values. Its MIT license file SHA-256 is
`a953e6268c89179c4a8f0cf5a49f57bc380a60e38078fe1f6361a0e45fd00363`.
It remains an **evaluation** candidate, not an accepted model. It must use
the same T2-V crop/partition contract and unchanged OCR gates. It cannot be
promoted from T1/T2-V alone.

No stronger, explicitly Urdu-capable, plate-domain pretrained model with clear
immutable release metadata was found. Paddle Arabic remains the control despite
failure; generic vision-language models are excluded. A script-aware dual
recognizer may preserve both raw outputs and select only when exactly one is
Unicode/script plausible under the unchanged confidence policy. Both/neither
plausible means `script_uncertain` and abstention; character substitution and
digit-script conversion remain forbidden. Fine-tuning requires T3/T4 rights,
sealed splits and separate approval.

## T3 authorization and acquisition gate

CCPD2019 from immutable Zenodo record `10.5281/zenodo.15647076` is authorized
for **isolated offline evaluation only**, but has not been downloaded:

- archive: `CCPD2019.tar.xz`
- transfer: 13,164,924,944 bytes (12.26 GiB)
- publisher MD5: `0dfbca0e6fcb7cb8ea720b0eae94c735`
- mirror license: CC BY 4.0; attribution required
- location: outside Git under
  `C:\NexusAI-Evaluation\R8\T3\CCPD\ccpd-2019-zenodo-15647076`
- use: detector and OCR distribution-shift evaluation; no training,
  redistribution, demo, retained-case ingest or production mutation
- output: aggregate metrics/error classes/non-reversible hashes only; never
  plate strings, crops, encoded filenames or contextual thumbnails

The acquisition gate checks absolute/out-of-repository destination, free disk,
resumable transfer, exact byte count and publisher MD5 before atomic finalize,
then writes a local SHA-256 receipt. Extraction is optional and blocked until
the archive validator rejects traversal paths, links/special members and
executable content. Official partitions must be pinned before comparison.

CCPD is Chinese and cannot satisfy Pakistan T4 operational agreement. A pass
can support Boundary A distribution-shift evidence only. Artificial Mercosur
remains supplemental/privacy-gated and was not selected for transfer.

### First-transfer correction

The first operator-run attempt displayed 118 MiB, then curl internally retried,
reset the file and stopped. Read-only inspection found no active acquisition
process and a preserved 29,724,672-byte (28.35-MiB) partial. A one-byte probe of
the exact Zenodo URL returned HTTP 206, `Accept-Ranges: bytes` and the correct
`Content-Range` total. The acquisition script now uses the shared Python
downloader with up to 1,000 attempts, a 120-second read timeout, bounded retry
delay and 64-MiB progress records. Every attempt re-reads file length and may
append only after exact HTTP 206 start/total validation. An ignored or incorrect
range fails closed without truncating the partial. Resume tests cover both the
valid and hostile server behaviors.

### Small real-image diagnostic pilot

Five official `rpnet/demo` files were acquired from CCPD commit
`02aaea15137c4d2fe662e57d257c6822356e9304` (GitHub reports a verified commit).
Their total is 354,853 bytes; exact publisher Git blob identities and local
SHA-256 values are recorded outside Git. Authorized visual inspection confirms
five full scenes with one visible real plate each and varied tilt, exposure,
background clutter and wet conditions.

This is `T3-diagnostic-pilot`, not a benchmark. The repository provides no
ground-truth annotation for these demo filenames, so model correctness cannot
be scored and manual observations cannot become independent truth. The pinned
FastPlateOCR diagnostic uses reviewed in-memory regions, retains neither crops
nor recognized strings, and records only image hashes, nonempty/charset-valid
counts, confidence summaries and latency. It is useful for format, provider,
real-image and privacy-path compatibility, but grants no accuracy, tuning,
Boundary-A, T4 or promotion authority.

The diagnostic executed in a network-disabled, 2-CPU/2-GiB container. All five
images produced nonempty Latin/digit-charset-valid results; mean inference
latency was 30.097 ms. Geometric-mean confidence ranged 0.7431-0.9999 and the
lowest individual character probability was 0.4756, demonstrating why nonempty
output cannot be treated as correctness. No string or crop was retained. Result
SHA-256 is `ef001a0708043106674feacba46134d430e819e272910cde42fa2a155e428e9d`.

### FastPlateOCR governed T2-V decision

The same pinned image then ran offline on the immutable 96-image T2-V manifest
`180889ad...e510`. Results are:

| Partition | Normalized exact | CER | Brier | P95 ms | Decision |
|---|---:|---:|---:|---:|---|
| Development | 0.7949 | 0.1566 | 0.1390 | 48.735 | blocked |
| Validation | 0.7857 | 0.1683 | 0.1354 | 48.267 | blocked |
| Sealed holdout | 0.7857 | 0.1683 | 0.1429 | 40.398 | blocked |

Peak RSS was 106.484 MiB and the package was 5.020 MiB. Calibration, latency and
resource checks pass. Exactness, CER and abstention precision fail unchanged
gates on every partition; the sealed holdout also contains one incorrect
high-confidence acceptance. FastPlateOCR is therefore the best efficient Latin
reference, not accepted or promoted. No post-holdout tuning is allowed against
this sealed generation.

## CPU and GPU execution plans

### D0 now

1. Reuse the acquired, checksum-verified FastPlateOCR 5.02 MiB package.
2. Build a network-disabled ONNX Runtime CPU evaluator pinned by digest.
3. Run identical T2-V development/validation/sealed-holdout crops; record raw
   and normalized output, abstention, calibration, P50/P95/P99 and peak RSS.
4. Only if T2-V passes, run CCPD after its separate local acquisition, safety
   validation and sealed-partition materialization.

### P1/P2 later

Qualify NVIDIA hardware, driver, CUDA, TensorRT and model-engine revisions as
one immutable profile. Evaluate TAO LPDNet and chosen OCR on identical evidence,
including cold/warm latency, throughput, concurrency, VRAM/RAM, thermal
stability, queue pressure and CPU fallback. GPU gains never change correctness
thresholds and cannot bypass license/privacy/T4 gates.

## Implemented governance and verification

- Added versioned D0/D1/P1/P2 hardware and candidate/tier contracts.
- Added read-only hardware inventory and the measured D0 receipt.
- Added exact CCPD acquisition/privacy contract and resumable preflight-first
  PowerShell workflow.
- Added hostile-archive validation and tests; extraction remains separate.
- Extended model, approval, artifact and dataset registries.
- Added hardware-neutral role resolution to the approved family architecture.
- Added the pinned FastPlateOCR CPU evaluator and guarded PowerShell runner.
  It enforces publisher preprocessing, CPUExecutionProvider/two-thread limits,
  network-disabled execution, immutable T2-V scoring, character-level and
  aggregate confidence provenance, latency/RSS/image digest capture, and
  before/after production-container identity checks. Three unit tests and the
  read-only model/T2-V/Docker preflight pass. Build and inference deliberately
  wait until the active CCPD transfer completes to avoid bandwidth contention.
- Hardware/T3 validation, 10 focused unit tests and PowerShell parsing pass.
  FastPlate acquisition passes at 5,263,955 bytes with both publisher hashes;
  CCPD preflight passes at 13,164,924,944 bytes and its transfer did not start.

## Exact next commands

Run from the repository in Windows PowerShell. The small pair can also be run
by Codex after explicit transfer approval. The 12.26 GiB T3 transfer is better
run locally so it can resume across UI/tool timeouts.

```powershell
Set-Location C:\Users\sheik\Workspace\Office\Projects\NexusAI

# Small model preflight: no network write and no production mutation.
powershell -ExecutionPolicy Bypass -File .\scripts\acquire_nexusai_r8_evaluation_artifacts.ps1 `
  -Candidate fast-plate-ocr-cct-s-v2-global-v1.1.0 `
  -PreflightOnly

# Small model acquisition: about 5.02 MiB into the isolated artifact cache.
powershell -ExecutionPolicy Bypass -File .\scripts\acquire_nexusai_r8_evaluation_artifacts.ps1 `
  -Candidate fast-plate-ocr-cct-s-v2-global-v1.1.0

# Large T3 preflight: no download.
powershell -ExecutionPolicy Bypass -File .\scripts\acquire_nexusai_r8_t3_ccpd.ps1

# Large T3 acquisition: resumes the preserved partial; do not add -Extract yet.
powershell -ExecutionPolicy Bypass -File .\scripts\acquire_nexusai_r8_t3_ccpd.ps1 `
  -ExecuteDownload
```

Expected markers are `R8ArtifactAcquisitionPreflight=PASS`,
`R8ArtifactAcquisition=PASS`, `R8T3AcquisitionPreflight=PASS` and
`R8T3Acquisition=PASS`. Keep any `.partial` file: it is resumable. Do not use
`-Extract` until the download PASS transcript is reviewed. Rollback is simply
non-assignment: neither package is referenced by production. File deletion is
a separate destructive operator decision.

## Decision

Proceed next with the isolated FastPlateOCR CPU evaluator. Run the CCPD
transfer locally only when the long transfer can remain open. Stop before any
other model or dataset download. R8.5 and production promotion remain blocked.
