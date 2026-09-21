# NexusAI R6.6-B2 exhaustive predeployment acceptance

Date: 2026-08-11  
Status: source/browser/data-execution accepted; rebuilt-runtime acceptance pending explicit approval

## Verified Current State

The accepted pre-R6.6 runtime is healthy and its governed `nexusai-forensic-demo` collection remains queryable. R6.6 source is newer than the running image. No build, deployment, migration, evidence mutation, model change, staging, commit, push, or publication occurred in this slice.

## Instructions Reconciled

The cumulative R6.6 directive, master directive, repository checkpoint, family-adapter architecture, current roadmap, phase ledger, API/auth guide, and testing guide were reconciled. The source/runtime distinction and all approval boundaries remain controlling. A stale ledger entry that placed R6.6 state in R3 was corrected.

## Current R6.6 Status

R6.6-A, R6.6-B1, and the safe predeployment portion of R6.6-B2 are source accepted. R6.6 is not complete or live accepted until the guarded rebuild and postdeployment manual/runtime pack pass. R7 remains blocked.

## Complete Operation/Template Coverage

The public catalog still contains exactly 65 unique templates and operation IDs. A new exhaustive contract test verifies every entry's name, operation ID, family, route, record families, example, calculation, output description, limitations, inputs, scope mode, and presentation mode. The operational split remains 23 orchestrator/cross-family, 16 CDR, 7 IPDR, 7 ANPR, 6 subscriber, and 6 tower operations.

The catalog now states one of `case_wide`, `case_or_target`, or `target_required`, and one of `bounded_records_table`, `cited_evidence_results`, or `executive_brief_with_sources`. Runtime clarification uses that same catalog authority, eliminating a second drifting target-required list.

## Query and Parameter Coverage

All 65 catalog examples route to their declared template. The 30 realistic English/Roman-Urdu variants remain covered. Required targets, optional date bounds, result limits, explicit direction, bounded prior-answer context, fresh targetless clarification, unsupported modality, and invalid subscriber target paths are tested. The exhaustive test exposed and corrected target-enforcement drift for operations such as `service_usage`.

## Shortcut/Suggestion Coverage

Visible shortcuts are capability-backed. The frequent-contact starter deliberately enters clarification rather than fabricating a global ranking. Catalog-only examples remain absent from the curated prompt rail. Follow-up actions preserve only the bounded target/template/date/direction packet.

## Specialist Coverage

One deterministic live Agent Chat check passed for each operational specialist: orchestrator, CDR, IPDR, ANPR, subscriber, and tower. All six completed with the governed collection visible, professional headings present, and internal fields hidden. Evidence is in `runtime-activation-20260811/r6.6-predeployment-agent-chat-regression.json`.

## Execution Correctness

The read-only full matrix executed all 65 operations against the governed predeployment runtime. Results: 65/65 correct template selection and completed request semantics; 64 returned deterministic rows; the KB evidence case truthfully returned a bounded no-result because the matrix explicitly set `max_kb_results=0`. No model was used. Evidence is in `runtime-activation-20260811/r6.6-predeployment-65-operation-matrix.json`.

## LLM/Model Validation

The installed model roles remain unchanged. Exact facts remain deterministic. Source tests now reject model-authored citation/source authority and invented identity/location attribution in addition to numeric/date/identifier restatement and family-specific prohibited inference. The expensive six-agent model matrix was not repeated against unchanged pre-R6.6 binaries; it belongs in postdeployment acceptance because the validator change is not live yet.

## UI/UX Presentation Coverage

Typed and retained answers preserve direct executive answers, useful metrics, bounded seven-column tables, five-row disclosure, readable sources, limitations, suggested next questions, and collapsed technical trace. Evidence sources carrying an `evidence_id` are now keyboard-focusable buttons that open the exact governed Evidence workspace item. Mobile retained-analysis selection now closes the history drawer instead of covering the answer.

## Citation/Provenance Coverage

Presentation keeps label, detail, source file, record type, row, timestamp, and evidence ID when present. New evidence-ID citations have an explicit source-open action. Legacy retained citations without an evidence ID remain readable but truthfully non-interactive.

## Implementation Changes

- Unified target-required catalog and clarification authority.
- Added catalog scope and presentation classifications.
- Added exhaustive 65-operation metadata/routing/input acceptance.
- Strengthened model synthesis rejection for citation and attribution claims.
- Added exact Evidence workspace navigation from citations.
- Added direct evidence-ID inspection on Evidence workspace entry.
- Closed mobile history drawers after selection/new-chat/conversation actions.
- Made matrix scripts accept non-historical report locations.
- Parameterized the proven guarded R6.4 activation script for an R6.6 contract marker without changing its historical defaults.

## Automated Tests

- `go test ./api/forensic_records`: PASS, 234 specs (232 passed, 2 skipped).
- Focused 65-operation contract: PASS, 2/2.
- Focused forensic presentation: PASS.
- Agent-pool compile check: PASS.
- Targeted ESLint: zero errors; existing warning debt remains.
- Vite production build: PASS, 669 modules, 9.06 seconds on the final run.
- Agent Chat Playwright: PASS, 17/17.
- Focused Evidence citation deep-link Playwright: PASS, 1/1.
- PowerShell syntax: PASS for all four changed gate/matrix scripts.

## Responsive/Theme/Accessibility

The in-app source browser verified retained result visibility, focusable table and trace controls, progressive disclosure, zero page overflow, and zero console warnings/errors. Eight screenshots cover 390, 820, 1024, and 1440 in light and dark under `runtime-activation-20260811/r6.6-responsive/`. The 390 workflow additionally proves the retained history drawer auto-closes while preserving the answer.

## Performance

The 65 direct operations averaged 338.1 ms. Minimum was 17.4 ms; p95 was 231.3 ms. `cross_family_correlation` was the single 17,121.6 ms outlier. Six specialist Agent Chat paths completed in 5.8-9.3 seconds. No timeout occurred; the cross-family outlier remains a documented optimization candidate rather than a hidden pass.

## Remaining Defects

- R6.6 binaries and UI assets are not deployed.
- Postdeployment targetless clarification, bounded follow-up context, exact evidence-source opening, and model rejection/fallback still require live proof.
- The cross-family latency outlier should be watched during the live pack.

No critical source-level routing, answer-shape, citation, responsive, theme, or mobile-history defect remains open from this acceptance pass.

## Runtime Status

Five containers are running. LocalAI, the forensic worker, NATS, and PostgreSQL carry healthy container status; the forensic records API is running without a Docker health label. LocalAI `/readyz` returns 200. The LocalAI container used approximately 4.84 GiB during the snapshot; the remaining four services used approximately 370 MiB combined. This is the prior accepted R6 runtime, not R6.6.

## Manual UI Test Pack

After the approved rebuild: verify inventory direct answer and source opening; fresh targetless frequent-contact clarification with no table; exact-target and `Only outgoing` follow-up; readiness scope; unsupported modality; no-result; model explanation and policy fallback; retained legacy answer; mobile drawer; technical trace; and 390/820/1024/1440 light/dark overflow/console checks.

## Team-Lead Demonstration

Show the 65/65 matrix first, then Ask NexusAI inventory, evidence-source open, targetless clarification, a valid frequent-contact ranking, `Only outgoing`, one specialist result, one unsupported request, and the 390 mobile retained answer. Close with the deterministic/model authority boundary and the runtime marker.

## Files Changed

Primary R6.6-B2 changes are in `api/forensic_records/query.go`, the new `r6_6_operation_matrix_ginkgo_test.go`, Agent Chat, Case Workspace, Evidence Workspace, Agent Chat E2E, three smoke scripts, and the guarded activation script. The cumulative worktree contains extensive earlier NexusAI changes that remain user-owned and unstaged.

## Roadmap / Ledger Update

R6 remains `corrective_extension_active`. Safe R6.6-B2 predeployment acceptance is complete. Deployment/runtime acceptance is pending. R7 remains blocked.

## Next Action

Obtain explicit approval to run the parameterized rollback-safe guarded rebuild. Then run the 65-operation, six-specialist, model, manual UI, responsive, health, resource, and retained-data-preservation checks against the rebuilt runtime.
