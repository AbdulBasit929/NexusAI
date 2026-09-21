# NexusAI R5-EVID-04 governed reprocess controls and contract publication

Date: 2026-08-10  
Status: source accepted; deployment and retained evidence mutation not performed

## Outcome

NexusAI now publishes `forensics.evidence-reprocess-plan/v1` through the
existing forensic contract catalog. The case-bound plan endpoint is read-only:
it reports the latest processing generation, terminal-job eligibility, request
requirements and the approval boundary without reserving a job, writing audit
state, changing evidence status or publishing to the queue.

Evidence Operations renders this as a compact governed recovery review. It
shows whether a future request could be considered, why, and that execution is
disabled in the analyst workspace. The actual execution endpoint remains a
separate, approval-gated operator concern.

## Verification

- `go test ./api/forensic_records -count=1`: PASS.
- LocalAI endpoint package compiled successfully; Windows denied cleanup of a
  temporary test executable after completion, without a code/test failure.
- Focused ESLint: zero errors; known repository JSX warnings only.
- Vite 8.0.16 production build: PASS, 669 transformed modules.
- Complete Case Workspace browser suite: 14/14 PASS.
- Browser coverage proves the governed review displays eligibility and submits
  no reprocess POST request.

## Boundary and next action

No Docker build/deploy, migration, upload, reprocess, cleanup, model or
configuration change, staging, commit, push or publication occurred.

Next is R5-EVID-05: generated contract refresh and R5 source-closure report.
Any live upload-to-ready or reprocess acceptance remains separately approval
gated.
