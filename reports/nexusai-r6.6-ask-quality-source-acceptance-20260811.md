# NexusAI R6.6-A Ask quality source acceptance

Date: 2026-08-11  
Status: source accepted; deployment and live-runtime acceptance pending

## Outcome

This slice closes the screenshot-proven accuracy and presentation defects that
could be corrected without changing retained data or rebuilding the runtime.
It preserves the accepted R6 lifecycle, history, model and 65-operation
contracts while making Ask NexusAI safer and more useful for common analyst
questions.

## Corrections

- Missing-target frequent-contact questions return: “Which number or subscriber
  should I analyze for frequent contacts?” No analytical SQL runs.
- Frequent-contact SQL requires an explicit target, ranks the opposite party,
  accepts only 8–19 digit phone-like values, and excludes self and labels such
  as `INTERNET`.
- Source inventory answers lead with the exact file count. Case readiness is
  explicitly evidence-analysis readiness, not NexusAI production certification.
- The presentation contract separates executive answer, model interpretation,
  clarification and raw trace. Planner/template/route placeholders are removed
  from primary metrics; tables use operation-specific bounded columns; sources
  are readable objects rather than serialized JSON.
- The UI uses Ask NexusAI identity, compact case/specialist context, one assurance
  statement, five-row progressive disclosure and collapsed technical details.
- Retained legacy presentations are defensively normalized at render time, so
  saved system-of-record answers improve without mutation.
- The capability corpus and platform contract are versioned as R6.6 and record
  the target-required frequent-contact behavior.
- Explicit follow-ups can reuse a bounded same-case target/template/date/
  direction packet from the prior authorized answer. The packet is not retained,
  contains no findings, grants no access and is ignored for unrelated questions.
- The routing corpus verifies all 65 governed catalog examples plus 30 realistic
  natural, short and mixed English/Roman-Urdu variants.

## Verification

- `go test ./api/forensic_records` — PASS.
- `go test ./core/services/agents --ginkgo.focus='forensic presentation contract'` — PASS.
- Targeted ESLint for `AgentChat.jsx`, `api.js` and `agents.spec.js` — zero
  errors; existing warning debt remains in the large chat/API modules.
- `npm.cmd run build` — PASS, 669 modules.
- `agents.spec.js` — PASS, 17/17 Chromium scenarios.
- Post-context focused forensic Agent Chat matrix — PASS, 14/14, including the
  serialized filter-context follow-up contract.
- Interactive source UI against the live API — PASS for Ask NexusAI identity,
  compact context, retained inventory normalization, readable filenames,
  seven-column/top-five table and collapsed trace.
- Full `go test ./core/services/agents` — 96 passed, 13 failed because Windows
  does not provide the rootless Docker environment required by unrelated
  testcontainers suites. The focused changed contract passes.

## Manual acceptance pack

1. Ask `which files were ingested?`; require a direct count before detail,
   readable filenames, at most five rows initially and no placeholder metric.
2. Ask `who are the frequent contacts?`; require clarification and confirm no
   SQL/result table appears.
3. Ask `who does 923001110001 contact most?`; require only phone-like
   counterparties, excluding the target and service labels.
4. Ask `is this case ready for analysis?`; require evidence-readiness scope and
   an explicit statement that this is not platform production certification.
5. Open a retained pre-R6.6 inventory answer; require the same clean hierarchy
   without modifying the retained record.
6. Expand Technical trace; confirm raw metrics/provenance remain available only
   there. Exercise Show all / Show top 5 and keyboard focus.

## Approval boundary and next step

No deployment, migration, ingestion/reprocess, model change or retained-state
mutation occurred. R6.6-B2 must close the production-data answer matrix and full
responsive/theme/accessibility evidence. A guarded Docker rebuild and live
acceptance require explicit operator approval. R7 remains blocked.
