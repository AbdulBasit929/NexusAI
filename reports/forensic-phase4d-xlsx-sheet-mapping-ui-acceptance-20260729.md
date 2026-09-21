# Phase 4D Per-Sheet XLSX Mapping and UI Simplification Acceptance

Date: 2026-07-29  
State: source accepted; not rebuilt, deployed, migrated, staged, committed, or pushed

1. **Objective:** classify mixed XLSX workbooks safely at worksheet granularity,
   preserve every source row and mapping decision, and make the Records
   Intelligence landing workflow easier for an analyst to understand.
2. **Assumptions verified:** workbook-wide union headers are not a safe typed
   schema; exact facts must remain deterministic; a non-matching worksheet must
   not be dropped or forced through an unrelated adapter; advanced/admin controls
   do not need equal visual priority with the analyst's question.
3. **Files changed:** the forensic upload API and test, XLSX reader, ingestion
   worker and tests, a versioned Pakistan-oriented golden, capability contract,
   React Records Intelligence page, worker/API/feature documentation, this
   report, team-lead brief, and living continuation checkpoint.
4. **Schema/API/config:** no database schema, migration, endpoint, or active
   runtime configuration changed. Upload job metadata now distinguishes
   `record_type_mode=auto|explicit`; the worker records a
   `per_sheet_schema_v1` mapping policy and row-level sheet mapping provenance.
5. **Tests:** combined Python worker/adapter/XLSX regression passed 43/43 in
   0.677 s; focused XLSX regression passed 10/10 in 0.826 s; final complete
   forensic Go API package passed in 31.249 s after the capability/evaluation
   matrix identifier was synchronized. Python compilation, golden JSON, and
   `git diff --check` passed. Page-scoped ESLint returned zero errors and four
   warnings; the production React build passed with Vite 8.0.16 in 2.52 s.
6. **Live runtime:** no service or image was rebuilt/recreated/restarted, no
   migration or retained write ran, and no queue/evidence/model operation ran.
   The built static UI was rendered locally without an API backend solely for
   source-layout inspection; the temporary preview server was stopped.
7. **Dataset/evidence:** a synthetic/de-identified Pakistan-oriented mixed
   workbook contract contains Operator CDR, ANPR Export, Wallet Transactions,
   and Case Notes sheets. It tests `+92` phone values, an ICT plate, PKR exact
   decimals, Urdu text, and source row 2. No supplied real file was modified or
   ingested.
8. **Models:** the configured Qwen chat/embedding baseline is unchanged. No
   model was called, searched, downloaded, installed, benchmarked, or promoted.
9. **Accuracy/retrieval:** automatic mapping agrees 4/4 with the golden: CDR,
   ANPR, transaction, and generic. Exact normalized values and worksheet/source
   row locators agree. An explicit transaction request types only the matching
   sheet and preserves the other three as generic review-required evidence.
10. **Latency/memory:** no deployed p50/p95 or memory claim. XLSX parsing remains
    bounded by existing ZIP/XML/sheet/row/cell limits and streams rows. The local
    production UI build completed in 2.52 s; that is a build diagnostic, not a
    user-performance benchmark.
11. **Failures/fallbacks:** an initial test tried to mutate a frozen `IngestJob`
    and was corrected to use `dataclasses.replace`; a four-decimal transaction
    fixture was corrected to the adapter's exact two-decimal money contract.
    Full-repository React lint remains blocked by seven unrelated existing errors
    and 571 warnings; no lint gate or threshold was weakened.
12. **Security/provenance:** mapping is based on each sheet's bounded headers;
    ambiguity falls back to generic review. Normalized rows retain evidence,
    source file, sheet index/name/state, header row/fingerprint, source row,
    raw fields, decision, matching adapters, and review state. Formula execution,
    macro execution, and external-link traversal remain prohibited.
13. **Git:** the pre-existing dirty worktree remains unstaged, uncommitted, and
    unpushed. No branch, commit, push, pull request, or GitHub action occurred.
14. **Rollback:** all changes are source-only. No retained database, queue,
    model, service, or image state needs rollback.
15. **Next action/approval:** Phase 4E should deepen real-format but
    synthetic/de-identified provider profiles for every structured family and
    split the UI into clearer analyst/evidence/ingestion/admin workspaces. Before
    deployment, perform a read-only activation audit, obtain separate approval
    for backup and migrations 008/009, then request a bounded API/worker/LocalAI
    rebuild with captured logs, health checks, smoke tests, timeout, and rollback.
    Model downloads, sensitive-data ingest, and Git publication remain separate
    approval gates.

## Delivered ingestion behavior

- `auto`: independently classify every non-empty worksheet.
- exactly one typed adapter match: normalize through that adapter.
- zero or multiple typed matches: preserve as generic and record why.
- explicit typed request: validate each sheet; preserve non-matches generically
  with `review_required=true` rather than dropping the sheet.
- mixed workbook summary: remain generic while row-level canonical records retain
  their actual family classification and exact source locators.

## Delivered UI behavior

- visible workflow choices: **Ask and analyze**, **Review evidence**, and
  **Manage data**;
- default query surface: collection, natural-language question, and Analyze;
- advanced template/model/limit/runtime-field controls hidden until requested;
- evidence/capability and batch-management areas closed until requested;
- static render verified the intended progressive disclosure; live API behavior
  remains an activation-time test after the approved rebuild.

## Boundary retained

This slice does not claim coverage of every Pakistan or international provider,
ODS/legacy XLS/columnar adapters, probabilistic schema matching, deployed UI
usability metrics, OCR, STT, TTS, vision, video understanding, or model quality.
Each of those needs its own versioned fixtures, metrics, resource budget,
provenance contract, fail-closed path, and rollback gate.
