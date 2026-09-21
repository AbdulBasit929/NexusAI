# NX-MMR multimodal benchmark plan

The executable contract is
`configuration/nexusai_multimodal_benchmark_manifest_v1.json`. This report
explains the acceptance method. The 2026-08-29 owner directive authorizes the
bounded empirical run described here; it does not authorize deployment,
retained processing, public-dataset acquisition, or any unlisted artifact.

## Benchmark law

1. Record baseline before tuning.
2. Keep fixture, controlled, public-real and authorized-operational evidence
   as separate strata.
3. Never use a candidate's output as ground truth.
4. Run at most one heavy model at a time. Admission is workload-aware: LIGHT
   requires at least 3.0 GiB free, MEDIUM 3.5 GiB, and HEAVY 4.5 GiB.
   VERY_HEAVY is not admitted by default. After measurement, require expected
   incremental peak plus at least 1.5 GiB headroom without falling below the
   class floor unless a receipt gives an explicit evidence-based justification.
5. Record artifact URL, immutable revision, byte size, hash, license, runtime,
   latency, peak memory and failure behavior.
6. Preserve complete-zero, no-match, unavailable, processing and failed states.
7. Report per-sample results and aggregate confidence intervals where sample
   counts permit; do not convert a small benchmark into population accuracy.
8. Promotion requires representative real-world evidence and integrated
   product acceptance, not only better fixture metrics.

## Slices

| Slice | Families | Required questions | Core measures |
|---|---|---|---|
| MMR-1 | structured, documents, KB | Are exact facts exact, retrieval grounded, and no-answer truthful? | exact-match, citation precision/recall, abstention, p50/p95, peak RAM |
| MMR-2 | image, ANPR, OCR, face candidates | Are regions localized, text preserved, negatives quiet, and face claims bounded? | detector precision/recall, full-plate match, CER, false positives, calibration, p95 |
| MMR-3 | audio, video | Is Urdu/English speech and source time preserved without inventing identifiers? | WER/CER, identifier recall, timestamp error, complete-zero truth, p95 |
| MMR-4 | routing, operations, UI | Does each upload/query reach a supported role and present certified suggestions only? | route precision, unsupported abstention, suggestion gate, citation navigation, four viewports |

## Evidence strata and sealed splits

- T0/T1 repository fixtures: development and regression; may yield at most
  `FIXTURE_CERTIFIED`.
- Local user-controlled evidence: hold out at least 20% after independent
  labeling; no tuning against the holdout. It remains outside Git.
- Retained product evidence: read-only replay first. Any new processing or
  retained upload is a separate approval.
- Public datasets: admit only after immutable artifact, exact bytes/hash,
  license, privacy, redistribution, and purpose review.
- Operational evidence: never inferred from a Downloads folder; requires
  explicit case authority and custody controls.

For the 108 local plate JPGs, first construct an independent manifest with
image hash, plate-present/absent, visible plate string or unreadable label,
province/template, blur/angle/occlusion, privacy disposition and reviewer. The
model receives the image only after that label is sealed. For `sample.mp4`, the
independent manifest records source-time ranges, plate-visible ranges, readable
text, speech transcript segments and negative intervals.

## Promotion thresholds

Thresholds are gates, not promises:

- deterministic operations: 100% golden exactness; zero cross-scope leakage;
- routing: at least 0.98 precision for auto-execution, otherwise manual review;
- no-answer/negative controls: zero invented factual positives;
- ANPR/OCR/ASR: candidate must improve the declared incumbent metric without
  regressing safety, timestamps, provenance, p95 latency or peak memory beyond
  the approved envelope;
- face: only candidate comparison/detection, never identity, with all false
  matches surfaced for review;
- UI: Home/Data/Ask/Activity, positive/zero/no-match/failure, citations,
  Activity reopen, Urdu/LTR and 390/820/1024/1440 pass without console errors.

Exact numeric ANPR/OCR/ASR promotion thresholds are frozen only after the
independent real-world manifest is sealed; setting them after seeing challenger
outputs would invalidate the holdout.

## Outputs of an approved run

Each run produces a versioned machine-readable receipt, per-sample matrix,
aggregate report, resource trace, license/notice inventory, error log, routing
trace, and promotion decision. Raw sensitive evidence and model-extracted plate,
face or voice identifiers are excluded from repository reports.

## Stop and rollback

Stop on hash/license mismatch, missing ground truth, P0 integrity/security,
failure of the applicable workload-aware RAM admission rule, free RAM
approaching 1 GiB, severe paging, an evaluator being killed, Docker instability,
two resource failures, scope leakage, or attempted retained mutation. Use
adaptive sequential batches of 5, then 10, then 20 and stream/bound video
frames. Isolated cache acquisition rolls back by deleting only the new
versioned cache directory after its path and receipts are verified; live
containers, named volumes and databases are not in scope. The distinct 6 GiB
build/deployment gate remains unchanged and deployment is not authorized.
