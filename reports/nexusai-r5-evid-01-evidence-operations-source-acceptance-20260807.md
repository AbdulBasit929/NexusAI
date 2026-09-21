# NexusAI R5-EVID-01 evidence operations source acceptance

Date: 2026-08-07  
Status: source accepted; deployment and retained ingestion not performed

## Outcome

R5 has begun with a bounded case-scoped Evidence Operations slice. The Case
Workspace now presents a clean master/detail evidence desk with truthful
catalog state, filters, compact operational metrics, integrity metadata, exact
accepted/duplicate/rejected row accounting, immutable processing runs, lineage,
linked artifacts and actionable processing notes.

The new `GET /api/v1/forensics/cases/:case_id/evidence/:evidence_id` adapter
keeps the URL case authoritative, validates its mapped collection, forwards
trusted case/collection scope and fails closed when that authority is
unavailable. The browser client consumes this case-scoped contract and rejects
late detail responses after case or selection changes.

Multi-file intake is staged locally and reports per-file failures, but this
acceptance did not upload, reprocess or mutate retained evidence. The interface
explicitly states that custody history is unavailable rather than fabricating a
timeline.

## Verification

- Focused ESLint: zero errors.
- Vite 8.0.16 production build: PASS, 669 transformed modules.
- Focused endpoint fail-closed Go contract: PASS.
- Auth route-feature package: PASS.
- Focused Case Workspace production-preview suite: 12/12.
- R5 evidence inspection contract: 1/1.
- Protected production-preview matrix: 62/62 with two workers.
- Interactive 1440 and 390 inspection: zero page-level overflow and zero browser
  console errors; the mobile summary was refined to a compact 2-by-2 layout.

## Remaining R5 boundary

R5 is not complete. Append-only custody events, stable citations that open exact
outputs, generated Swagger refresh, approved live upload-to-ready acceptance,
large-catalog pagination, bulk-ingestion controls, governed reprocessing and
deployment/runtime acceptance remain. The `swag` generator is not installed in
this workspace, so source annotations and product documentation were updated
while generated Swagger artifacts remain explicitly pending.

No Docker build/deploy, database migration, evidence upload/reprocess, model or
configuration change, cleanup, staging, commit, push or publication occurred.
