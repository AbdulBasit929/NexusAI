# NexusAI R5-EVID-02 custody and citation source acceptance

Date: 2026-08-07  
Status: source accepted; deployment and retained evidence mutation not performed

## Outcome

The existing case-scoped evidence detail contract now publishes the retained
control plane instead of maintaining a second provenance model. Every query is
bound to tenant, authorized collection and evidence identity and executes in the
existing read-only evidence-detail transaction.

The response adds canonical processing runs, append-only processing events,
derived artifacts, stable `nexusai://evidence/{evidence_id}/artifacts/{artifact_id}`
citation references, exact JSON citation locators, append-only custody events and
a whole-chain integrity summary using recorded previous/event SHA-256 links.

Evidence Operations presents these facts through a compact artifact selector,
exact citation inspector, canonical pipeline ledger, and custody chronology.
Valid chains receive a verified assurance; absent history remains an explicit
unavailable state and is never inferred.

## Verification

- Focused provenance/accounting/privacy Go contracts: PASS.
- Focused ESLint: zero errors; known repository JSX false-positive warnings only.
- Vite 8.0.16 production build: PASS, 669 transformed modules.
- Focused Case Workspace suite: 12/12.
- Custody/citation contract: 1/1, including 1440 and 390 overflow checks.
- Protected production-preview matrix: 62/62 with two workers.
- Interactive live-data preview: catalog and shell render cleanly; the currently
  deployed backend truthfully returns Resource not found for the undeployed R5
  detail adapter, so no live custody history was claimed.

## Boundary and next work

R5 is not complete. Generated Swagger refresh, scalable server-backed catalog
pagination, bounded concurrent bulk registration, exact per-file outcomes,
queue observability, governed retry/reprocessing and approved upload-to-ready
runtime acceptance remain. R5-EVID-03 owns the next source slice.

No Docker build/deploy, migration, upload/reprocess, data cleanup, model or
configuration change, staging, commit, push or publication occurred.
