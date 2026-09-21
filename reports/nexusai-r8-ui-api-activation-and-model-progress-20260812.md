# NexusAI R8 UI/API activation and model-evaluation progress

Date: 2026-08-12  
Decision: UI/API live accepted; R8.3 accuracy and R8.4 model promotion pending

## Activation correction

The initial activation stopped with `The forensic-records-api container is
missing`. This was a false negative in the script's container discovery, not a
missing service. It failed before rollback tags, builds or service mutation.
The gate now resolves the exact `nexusai-forensic-records-api-1` and
`nexusai-api-1` containers, parses their inspection data and requires the
expected Compose project/service labels. A `-PreflightOnly` path verifies this
boundary without mutation.

Preflight passed. The guarded activation then rebuilt and recreated only
`forensic-records-api` and `api`:

- forensic API build: 21.8 seconds
- NexusAI UI/API build: 121.2 seconds
- worker rebuilt: no
- models or profiles changed: no
- named volumes preserved: yes
- health, readiness and `/app`: HTTP 200
- rollback images: preserved with stamp `20260812T092205Z`

Runtime evidence is recorded in
`reports/runtime-activation-20260812/r8-ui-api-activation-20260812T092205Z.json`.

## Manual browser acceptance

The live Evidence workspace loaded under the authorized
`nexusai-forensic-demo` case. It reported 10 registered sources, 9 ready, one
failed, zero in flight, 9,274 accepted rows and seven rejected rows. Desktop
1280x720 and mobile 390x844 checks had no document-level horizontal overflow;
the browser produced no warning or error entries. The current case contains
structured evidence only, so this run does not claim live-image candidate
overlay acceptance.

## Approved model-evaluation boundary

The operator explicitly approved model installation and relevant R8 evaluation
work. Source now contains:

- `tools/r8_ocr_eval/Dockerfile`: isolated CPU evaluator with pinned
  PaddleOCR/PaddlePaddle and Tesseract English/Urdu packages
- `tools/r8_ocr_eval/generate_fixtures.py`: deterministic, non-personal clear,
  blur, glare, night, skew, occlusion, Urdu and empty-scene fixtures
- `tools/r8_ocr_eval/run_benchmark.py`: raw/normalized output, abstention,
  latency and RSS capture for the Tesseract baseline

The approved evaluator build did not execute because the external
privileged-action service reported its usage limit until 2026-08-18 09:30
Asia/Karachi. This is not a NexusAI, Docker, disk, licensing or application
failure. No workaround was attempted. No package/model download, benchmark,
detector/OCR selection or production role assignment is claimed.

## Required next gate

When external execution is available: build the isolated evaluator; generate
and validate the synthetic manifest; measure localization precision/recall;
measure exact plate accuracy, character error rate, calibration, abstention,
P50/P95 latency, peak memory and package size for Tesseract and PaddleOCR; then
record either no promotion or an evidence-backed operator promotion. R8.5 must
not start before this gate passes.
