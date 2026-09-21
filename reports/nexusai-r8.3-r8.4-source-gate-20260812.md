# NexusAI R8.3 implementation and R8.4 approval gate

Date: 2026-08-12  
Disposition: R8.3 platform implementation source accepted; R8.3 accuracy
acceptance blocked by the absent approved visual pack and detector; R8.4
installed-inventory evaluation complete with approval required  
Mutation boundary: source, tests, documentation, and read-only runtime inventory
only; no evidence ingest, image retention, model/backend download, migration,
profile change, service rebuild, or deployment

## Delivered R8.3 platform contract

- `forensics.plate-region-candidate/v1` requires a candidate ID, immutable parent
  evidence/version/hash, detector and version, `license_plate` label, confidence,
  review state, and integer bounds in `original_image_pixels`.
- Candidate validation rejects display-space geometry, out-of-bounds or empty
  rectangles, unversioned detectors, invalid confidence, missing lineage, and
  unsupported review states.
- Candidate output is capped at 32 regions. Deterministic confidence-first NMS
  uses a 0.45 IoU threshold and candidate-ID tie breaking.
- `forensics.plate-region-crop/v1` reconstructs the exact original-coordinate
  region as lossless PNG and records parent identity/hash, bounds, transform
  chain, dimensions, encoding, and crop SHA-256.
- Localization benchmarking uses one-to-one IoU matching so duplicate detections
  cannot inflate recall.
- Evidence Operations displays recorded candidate artifacts in an accessible,
  responsive original-coordinate overlay. It states that candidates are not
  proof and does not expose unaudited accept/reject/correction controls.

The contract and reconstruction path are executable and tested. Model
localization accuracy is not accepted because no approved visual fixture pack
or eligible detector exists in the current runtime.

## R8.4 installed inventory and approval decision

Read-only inspection found:

| Inventory | Observed state | Eligible detector/OCR role |
| --- | --- | --- |
| `qwen_qwen3-4b-instruct-2507` | installed LocalAI text model | no |
| `qwen3-embedding-0.6b` | installed LocalAI embedding model | no |
| PaddleOCR | absent from worker and host command inventory | no |
| Tesseract/pytesseract | absent from worker and host command inventory | no |
| OpenCV | absent from worker | no |
| Ultralytics | absent from worker | no |
| Torch | absent from worker | no |

The evidence-backed shortlist remains:

1. A specialized offline plate detector that emits original-pixel boxes and
   meets the approved localization gate. Generic VLM coordinates are not an
   authoritative substitute.
2. PaddleOCR `arabic_PP-OCRv5_mobile_rec` for the Urdu/English OCR candidate.
3. Tesseract `tessdata_fast` `urd+eng` as the lightweight CPU baseline.

These are shortlist entries, not approvals. License/package notices, exact
plate accuracy, CER, calibration, abstention, latency, memory, disk footprint,
offline operation and CPU/GPU fit remain unmeasured until an operator approves
the visual pack and local installation. No download or role assignment is
allowed by this report.

## Benchmark gate

The offline runner `scripts/benchmark_r8_anpr_models.py` consumes only approved
annotations and supplied prediction manifests. It computes one-to-one
localization precision/recall, exact plate accuracy, character error rate,
abstention accuracy and latency percentiles. Running it against the accepted
taxonomy currently exits with the expected blocked state because the fixture
list is empty and no prediction manifest exists. Zero fixtures never produce a
passing accuracy score.

## Verification

- `go test ./api/forensic_records`: PASS, including candidate validation,
  deterministic NMS, candidate caps, exact repeatable crop/hash reconstruction,
  and one-to-one localization metrics.
- R8 model-approval and fixture-taxonomy JSON documents parse successfully.
- Empty-pack benchmark safety gate: PASS; expected exit code 2 and explicit
  blocked reasons.
- Focused ESLint: zero errors; pre-existing JSX false-positive unused warnings
  remain.
- React production build: PASS, 669 modules transformed.
- Agent Chat plus Case Workspace Chromium production-bundle suites: 38/38 PASS,
  including desktop and 390 px candidate-overlay acceptance with no page-level
  horizontal overflow.

## Safe rebuild disposition

A rebuild is required only to display the accepted R8.2/R8.3 UI and publish the
updated forensic API contract in the live local runtime. Only
`forensic-records-api` and LocalAI/UI changed; the worker, PostgreSQL, NATS,
named volumes, case data, profiles and models must not be rebuilt or altered.
Use the rollback-preserving commands in the handoff response. Deployment does
not make plate detection or OCR operational. The same guarded sequence is
captured in `scripts/build_deploy_nexusai_r8_ui_api_gate.ps1`; it rebuilds only
the forensic API and LocalAI/UI, tags both current images for rollback, verifies
health, and automatically restores both accepted images on failure.

## Exit decision

R8.3 contract/platform implementation is source accepted, but R8.3 accuracy
acceptance cannot truthfully close without an approved image pack and detector.
R8.4's installed-inventory evaluation and approval gate are complete; its
decision is `approval_required`, with no selected detector or OCR role. R8.5
must not begin until those two approvals are explicit and the measured R8.3/
R8.4 gates pass.
