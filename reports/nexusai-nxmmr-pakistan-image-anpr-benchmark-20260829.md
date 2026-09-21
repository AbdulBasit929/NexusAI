# NX-MMR Pakistan image ANPR benchmark

Status: `COMPLETE_PRIVATE_HUMAN_GOLD_LIMITED` (2026-08-30).

## Verdict

The incumbent FastALPR stack has useful single-image plate-presence recall but
does not establish production-grade Pakistan ANPR. It detected 10/10
development plates and 20/22 sealed-holdout plates. Exact normalized plate
recognition was 5/10 development and 13/22 sealed holdout. The holdout was run
once, after development, with the same fixed configuration; no holdout tuning
or rerun occurred.

The locked independent-human oracle passed validation with digest
`88c6827c23fbed467f097843404ed09c947bf7f27432e5cb5fa8427be9f48567`.
All 32 active images are positives with one plate each. Consequently, image
specificity and negative-image false-positive rate are not estimable, and the
absence of human boxes prevents IoU localization scoring.

## Frozen incumbent

- processor: `fastalpr-onnx-cpu`, revision `nexusai-bf1-v1`;
- detector: YOLOv9-t 384 ONNX, SHA-256
  `888397b96d761c89db40bc9c305838e8652660f5e282c2cadebbe8d2951a77a8`;
- OCR: CCT-XS-v2 global ONNX, SHA-256
  `8031afb5fdc6b4d80462c9d542f1284ebd2cfddf5dbacd62609848d7e2855f44`;
- OCR configuration SHA-256
  `0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6`;
- detection threshold: `0.75`;
- providers: `OpenVINOExecutionProvider,CPUExecutionProvider`;
- execution: isolated, network-disabled, one model worker, four CPU and 2 GiB
  container limits; no live install or service mutation.

## Development metrics

| Metric | Result |
|---|---:|
| images / positive images / truth plates | 10 / 10 / 10 |
| predicted detections | 10 |
| presence TP / FP / FN / TN | 10 / 0 / 0 / 0 |
| presence recall (95% Wilson) | 1.000000 (0.722467–1.000000) |
| presence specificity | not estimable; no negatives |
| count-proxy detection precision / recall / F1 | 1.000000 / 1.000000 / 1.000000 |
| normalized exact TP / FP / FN | 5 / 5 / 5 |
| normalized exact precision / recall / F1 | 0.500000 / 0.500000 / 0.500000 |
| exact accuracy per truth plate (95% Wilson) | 0.500000 (0.236593–0.763407) |
| raw exact accuracy | 0.500000 |
| normalized CER | 0.184615 (12/65 characters) |
| latency mean / p50 / p95 / max, seconds | 0.134048 / 0.103527 / 0.267849 / 0.297259 |
| run wall / process CPU, seconds | 1.669032 / 4.092599 |
| process peak RSS | 279.691 MiB |

## Sealed-holdout metrics

| Metric | Result |
|---|---:|
| images / positive images / truth plates | 22 / 22 / 22 |
| predicted detections | 20 |
| presence TP / FP / FN / TN | 20 / 0 / 2 / 0 |
| presence recall (95% Wilson) | 0.909091 (0.721851–0.974705) |
| presence specificity | not estimable; no negatives |
| count-proxy detection precision / recall / F1 | 1.000000 / 0.909091 / 0.952381 |
| normalized exact TP / FP / FN | 13 / 7 / 9 |
| normalized exact precision / recall / F1 | 0.650000 / 0.590909 / 0.619048 |
| exact accuracy per truth plate (95% Wilson) | 0.590909 (0.387348–0.767442) |
| raw exact accuracy | 0.590909 |
| normalized CER | 0.201439 (28/139 characters) |
| latency mean / p50 / p95 / max, seconds | 0.095364 / 0.077689 / 0.176464 / 0.291457 |
| run wall / process CPU, seconds | 2.728524 / 6.442250 |
| process peak RSS | 255.539 MiB |

Detection precision is explicitly a count proxy: each image has one truth
plate, there are no negative images, and the oracle has no boxes.

## Redacted error analysis

- Development produced five exact results and five OCR errors, with no detector
  miss. Non-exact outputs had one, two, or three character edits; three of five
  were longer than the reference. Mean OCR confidence was 0.957254 on errors
  versus 0.960343 on exact results, so development confidence did not separate
  correctness.
- Holdout produced 13 exact results, seven OCR errors and two complete detector
  misses. The seven OCR errors required two or three edits; the two misses
  account for one four-character and one seven-character reference. Mean OCR
  confidence was 0.890109 on errors versus 0.995512 on exact results.
- The independently marked partial/hard image in each split was recognized
  exactly. The sample is too small to infer that hard cases are generally safe.
- The current OCR configuration lists many plate regions but not Pakistan.
  Together with 50.0% and 59.1% exact accuracy, this is a concrete domain/model
  gap rather than evidence for Pakistan-wide robustness.

Private row-level truth and predictions remain only in the ignored local
benchmark receipts. No plate string is published here.

## Certification consequence

This is limited real-world evidence for the incumbent, not a certification
promotion. `PRODUCT_CERTIFIED` remains prohibited. No operation-ledger or
suggestion status changed. Negative/hard-negative images, human boxes, broader
Pakistan plate formats and conditions, and an untouched future holdout are
required before a representative real-world certification decision.

