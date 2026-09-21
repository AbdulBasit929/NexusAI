# NexusAI R5 live intake and custody correction progress

Date: 2026-08-10  
Status: superseded by final R5 runtime acceptance; corrected rebuild passed

## Accepted deployment evidence

The operator-run combined build and R5 gate passed:

- API build: 19.0 seconds.
- Worker build: 5.8 seconds.
- LocalAI/UI build: 417.3 seconds in one attempt.
- Free RAM before build: 7.37 GiB.
- Rollback images and named volumes preserved.
- LocalAI `/readyz`: 200.
- forensic API `/healthz`: 200.
- `forensics.evidence-reprocess-plan/v1`: live and read-only.
- approval required: true; execution permitted: false.
- mutating rebuild-gate requests: zero.

## Controlled live intake

The user explicitly proceeded to the previously disclosed retained synthetic
intake gate. Baseline was eight completed evidence items and 9,272 accepted
rows in `nexusai-forensic-demo`.

The live Evidence Operations UI staged two files concurrently:

- `r5_live_intake_acceptance_20260810.csv` registered but reached a truthful
  permanent-input/dead-letter state because its intentionally non-contract CDR
  columns were missing required adapter fields.
- `r5_live_intake_empty_20260810.txt` failed independently before forensic
  registration with the actionable empty-document error. The UI preserved the
  successful and failed per-file outcomes and reported `1 registered, 1 failed`.

A corrected, contract-shaped fixture
`r5_live_intake_ready_20260810.csv` then completed with exactly two total rows,
two accepted rows, zero row errors, one immutable processing run and three
processing events. Case accepted rows reconciled exactly from 9,272 to 9,274;
in-flight jobs returned to zero. Its evidence ID is
`85a4e63e-9409-442c-a056-068d4c065f54` and SHA-256 is
`dd17bd681c04c8d1b55d392cce5003d101f94c2b40976257f6e1f2809ddab7aa`.

Retained changes are limited to the two registered synthetic evidence sources,
their immutable versions/jobs/events/custody/audit/KB links and two accepted
canonical CDR rows. The zero-byte file created no evidence item. No reprocess,
deletion, cleanup, model/configuration change, migration, staging, commit or
push occurred.

## Defects exposed and bounded corrections

1. A collection upload can return HTTP 200 while forensic forwarding reports
   `evidence_status: failed`. Evidence Operations now treats backend-declared
   `failed` or `skipped` as a per-file failure instead of a successful result.
   Browser coverage now exercises the HTTP-200 failure contract.
2. Three valid custody events recorded in the same second exposed that API
   verification incorrectly inferred chain order from timestamp plus random
   UUID. The API now walks recorded predecessor hashes, checks one root,
   reachability, missing predecessors, duplicate hashes and forks, and presents
   events in chain depth order.
3. Fresh-schema trigger logic now selects the unreferenced chain tail. Additive
   migration 011 applies the same function-only correction to the retained
   database. It rewrites zero retained rows.

## Source verification

- `go test ./api/forensic_records -count=1`: PASS in 8.662 seconds.
- React production build: PASS, 669 modules.
- Case Workspace production-browser suite: PASS, 14/14 in 17.2 seconds.
- Partial-failure regression now covers an HTTP-200 backend-declared failure.
- Migration and activation scripts parse cleanly; diff/format checks pass.

## Retained migration acceptance

The explicitly approved guarded migration 011 completed at
`2026-08-10T05:47:03.7869041Z`. It changed only the custody-event trigger
function and rewrote zero retained rows. The script created and validated the
custom-format rollback archive
`.phase2-backups/r5-pre-migration-011-20260810T054652Z.dump` (5,707,090 bytes,
SHA-256 `030208fbeed598d82ec91dca73a7522c81821a6396a49387222ef66040c2998b`).
Preflight, apply and verification transactions passed; API and worker health
returned 200 after writer restart. The machine-readable acceptance marker is
`reports/runtime-activation-20260810/r5-custody-migration-011.json`.

## Final deployment closure

The corrected API/UI refresh subsequently passed. Live detail for evidence
`85a4e63e-9409-442c-a056-068d4c065f54` now reports three custody events, zero
broken links and `chain_valid: true`. Focused responsive browser acceptance is
clean. R5 is complete; final evidence is recorded in
`reports/nexusai-r5-final-runtime-acceptance-20260810.md`.
