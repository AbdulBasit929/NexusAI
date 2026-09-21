# NX-MMR human-gold ANPR/ASR empirical closure

## Outcome

`GroundTruthValidation=PASS` was independently established before inference.
The incumbent image ANPR development split, one-time sealed holdout and bounded
`sample.mp4` pass are complete. ASR was correctly not run because all six human
speech windows are negative and contain no transcript evidence.

No model output was used as an oracle. No additional artifact was downloaded,
installed or deployed. No retained evidence, Activity, database, named volume,
live model, profile, backend or service was mutated. NX-B2.1 was not started.

## Decision summary

| Slice | Key empirical result | Decision |
|---|---|---|
| image development | detection 10/10; normalized exact 5/10; CER 0.184615 | limited development evidence |
| image sealed holdout | detection 20/22; normalized exact 13/22; CER 0.201439 | limited evidence; no promotion |
| video events | detected 3/19; exact 3/41; CER 0.926829 | reject current video policy for promotion |
| video grouping | exact groups 4/29; F1 0.242424; mean absolute boundary error 1.178250 s | insufficient |
| video temporal gaps | 1 FP among 26 negative sampled frames; 31 misses among 34 positive sampled frames | insufficient |
| ASR | 6/6 windows human-labeled no speech; no transcript | scoring inadmissible; no model call |

Full metric definitions, confidence intervals and redacted error analysis are
in the image, video and ASR reports dated 2026-08-29 and completed on
2026-08-30.

## Reproducibility and private receipts

The evaluator and fail-closed runner are
`scripts/benchmark_nexusai_nxmmr_anpr.py` and
`scripts/run_nexusai_nxmmr_anpr_benchmark.ps1`. The runner pins model hashes,
the 0.75 threshold, oracle/video hashes, split order, network isolation, read-
only inputs, one worker, and refusal to overwrite a prior receipt.

Private per-sample receipts remain in the Git-ignored
`local-acceptance-models/nxmmr/private-benchmarks/anpr-human-gold` directory:

| Receipt | SHA-256 |
|---|---|
| image development | `d497d2766fb123aa9696fa0ed5f5bb9f1fc027b5e06a66b54450d831ad78a8c7` |
| image sealed holdout | `9bed54cb292d49b996d326ca49163f1d762daf03acf7ee6ef3d0c3bc95735500` |
| sample.mp4 video | `6ff952885bde7939fc0afddfd3126459483578e8775d48060b314e1be4212201` |

No raw plate value appears in this report or any committed report.

## Resource receipt

- Host free physical RAM was 4.156 GiB before and 3.683 GiB after the bounded
  run, above the 3.5 GiB MEDIUM floor and far above the approximately 1 GiB
  emergency stop.
- Container limit: 2 GiB memory, four CPU, 512 processes; network disabled.
- Development: 1.669032 s wall, 4.092599 s process CPU, 279.691 MiB peak RSS.
- Holdout: 2.728524 s wall, 6.442250 s process CPU, 255.539 MiB peak RSS.
- Video: 63.878912 s end-to-end wall, 13.380812 s process CPU, 273.211 MiB
  peak RSS; model inference itself totaled 3.930030 s across 60 frames.
- The five live NexusAI container IDs/names were identical before and after.

## Verification

- Four ANPR metric tests and four human-workflow tests pass.
- Evaluator Python compilation, PowerShell runner parsing and benchmark-manifest
  JSON parsing pass.
- Published image/video headline metrics were re-read from the private receipts
  and match this report.
- An automated comparison of every private human plate string against the
  changed public reports and continuation ledger found zero identifier leaks.
- The older ground-truth preparation test remains host-environment blocked
  because Pillow is not installed; this benchmark added no dependency and did
  not alter that known limitation.

## Certification consequence

The operation-certification ledger remains 62 `REGISTERED`, 12
`SOURCE_VALIDATED`, one `FIXTURE_CERTIFIED`, zero newly real-world-certified
operations and four existing `PRODUCT_CERTIFIED` operations. Ordinary visible
suggestions remain restricted to exact existing `PRODUCT_CERTIFIED` mappings.
No image, video or ASR operation is promoted by this benchmark.

Before any future `PRODUCT_CERTIFIED` decision, the unchanged live API, Ask,
Data, citation/source-navigation, Activity, UI/error-state, security,
performance and manual product-acceptance gates must all pass on the selected
and separately approved model/policy.

## Remaining gaps and exact next action

Image negatives/boxes/domain breadth, video development-versus-sealed sources
with track truth, and positive human-transcribed Urdu/Pakistani speech are all
missing. The later authorized reference-parity investigation supersedes this
report's original immediate next action: follow
`reports/nexusai-nxmmr-reference-pipeline-parity-20260830.md` and the updated
approval-boundary record. Do not retune or rescore the completed holdout,
download a challenger, activate a service or start NX-B2.1.
