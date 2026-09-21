# NX-MMR workload-aware resource gate revision

Status: `SOURCE_GOVERNANCE_RECONCILED` on 2026-08-29.

## 2026-08-31 workload-specific Video V2 override

The later explicit exact-runtime comparability directive supersedes the 4.5-GiB
HEAVY floor **only** for `VIDEO_V2_LOCAL_EVALUATOR`: initial admission requires
3.0 GiB available physical RAM, one heavy evaluator at a time, and an absolute
stop at 1.5 GiB available physical RAM. Record host minimum available RAM,
process/child peaks, incremental peak, CPU and wall time during exact-runtime
development; derive the empirical requirement from measured incremental peak
plus at least 1.5 GiB headroom. Unexpectedly high usage requires stopping and
reclassification before reserved admission. The old 672.398-MiB OCR replay peak
is context, not an exact integrated-runtime measurement.

This override is recorded in
`configuration/nxmmr_video_v2_local_evaluator_policy.json`; all other workload
floors and future build/deployment/live-startup policies remain unchanged.
Do not ask the operator to close Codex/Chrome for the superseded evaluator
floor. No download, live installation, build, recreation or activation is
authorized by this resource revision.

## Authorization boundary

The product owner approved the NX-MMR A-D empirical slice and superseded the
fixed 6 GiB threshold for isolated benchmarks only. This revision does not
authorize a build, deployment, live model installation, service restart,
retained processing, database/KB/Activity mutation, named-volume change, or
public-dataset acquisition. The existing 6 GiB build/deployment gate remains
unchanged.

## Workload classes

| Class | Minimum free physical RAM before run | Admission |
|---|---:|---|
| LIGHT | 3.0 GiB / 3,221,225,472 bytes | allowed when freshly measured |
| MEDIUM | 3.5 GiB / 3,758,096,384 bytes | allowed when freshly measured |
| HEAVY | 4.5 GiB / 4,831,838,208 bytes | allowed when freshly measured |
| VERY_HEAVY | not defined | not admitted by default |

Only one heavy model may be active. Once a representative measurement exists,
the pre-run requirement becomes the greater of the class floor and expected
incremental peak RAM plus 1.5 GiB (1,610,612,736 bytes) headroom, unless a run
receipt records a narrower evidence-based justification.

## Execution controls

- Remeasure free physical RAM immediately before each admitted run.
- Use sequential adaptive batches of 5, then 10, then 20 samples.
- Stream or bound video frames; never decode the full video into memory.
- Do not concurrently load Qwen reranker, PaddleOCR, or another heavyweight
  evaluator.
- Cleanly stop only the isolated evaluator between roles; do not stop Chrome,
  Codex, LocalAI/UI, forensic API/worker, PostgreSQL, NATS, or their volumes.
- Record RAM before, process/evaluator peak, host minimum free RAM, CPU,
  duration, batch and outcome in every run receipt.

## Emergency stop

Stop the relevant run when free physical RAM approaches 1.0 GiB, severe paging
appears, an evaluator is killed, Docker becomes unstable, or two resource
failures occur. Hash/size/license/notice mismatch, missing independent ground
truth, P0 integrity/security, scope leakage, or retained mutation also stops
the affected slice.

## Reconciliation result

`configuration/nexusai_multimodal_benchmark_manifest_v1.json`, the benchmark
plan, current inventory, consolidated approval pack, manual UI guide, Master
Directive, continuation checkpoint, roadmap, phase ledger and maturity matrix
now express the same benchmark policy and preserve the separate deployment
gate.

`NXMMRApprovalRecorded=PASS`

`WorkloadAwareResourceGate=PASS`

`ParallelHeavyModels=1`

`DeploymentPerformed=false`
