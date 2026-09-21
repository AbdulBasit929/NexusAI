# NexusAI APF-2 unified Data intake source acceptance

Date: 2026-08-17  
Program status: APF-2 live/runtime accepted  
Deployment status: unchanged  
R8 status: `valid_but_paused_non_blocking`

## Retained-runtime acceptance closure

The halted attempt was resumed without another upload. Source detail now
resolves authoritative catalog accounting by exact evidence identity and shows
4 verified records, 5 input rows, 1 duplicate and 0 rejected. A genuine zero
remains displayable; missing accounting renders `Not available`. Regression
coverage also proves source A cannot borrow source B accounting, failed-source
truth remains intact, and filtered/paginated opening uses evidence identity.

The one authorized Ask query was `Show temporal CDR activity for 923001234567
on 2026-07-10.` It produced completed analysis
`c540c875-0cd7-410d-bd6b-3f2be87b0336`, deterministic authority
`cdr.temporal_activity`, 4 exact results and citations to
`pakistan_cdr_messy_synthetic.csv` rows 5 and 2. Unique filename-to-catalog
resolution opened the exact retained evidence. History advanced 50 to 51,
reopened the answer and citations, and Continue in Ask populated the follow-up
without submitting it.

Final matrix: upload PASS; registration PASS; classification PASS; processing
PASS; canonicalization PASS; duplicate accounting PASS; custody PASS;
capability readiness PASS; Data catalog PASS; source-detail accounting PASS;
Ask routing PASS; deterministic result PASS; citation PASS; source opening PASS;
History PASS. Focused Playwright 10/10, scoped ESLint zero errors, 677-module
build, and real 390/820/1024/1440 dark/light checks pass without overflow or
console errors. No deployment or backend source change was required.

## Verified current state

The source preview at `http://127.0.0.1:3000/analyst/data` resolves the existing
`nexusai-forensic-demo` workspace and reports 10 governed sources: 9 Ready and
1 Failed. The existing upload backend, evidence control plane, queue, catalog,
capability registry and reprocess-plan contract are live. No retained upload was
performed during this acceptance.

## Existing capability audit and reuse matrix

| Need | Reused contract | Result |
| --- | --- | --- |
| Workspace upload | `/api/agents/collections/:name/upload` | reused unchanged |
| Retention and custody | content-addressed evidence admission | reused unchanged |
| Duplicate identity | scope-local SHA-256 lookup | reused unchanged |
| Classification | retained-byte sniffing, bounded inspection and classifier | reused unchanged |
| Processing | transactional outbox and queue worker | reused unchanged |
| Catalog and detail | case evidence APIs with `limit`, `offset`, `has_next` | reused unchanged |
| Product actions | forensic capability registry | reused unchanged |
| Failure recovery | case-bound read-only reprocess plan | reused unchanged; no execution |
| Upload progress | browser `XMLHttpRequest.upload` events | small shared frontend extension |

No backend, API route, database schema, migration, queue, storage, model, agent
profile or worker change was required.

## Implemented product behavior

- Add Data is available from Home and Data and supports a page drop target.
- The accessible modal stages one or many files and keeps server classification
  authoritative.
- At most two registrations execute concurrently.
- Each file reports Waiting, Uploading with percent, Registering, Processing,
  Ready, Needs attention, Failed or Already added.
- A failed file does not hide successful or duplicate outcomes in the same batch.
- The catalog uses one central status mapper and separates Failed from Needs
  attention.
- Subsequent sources load through server pagination.
- Source detail shows the original filename, useful facts and technical detail
  without exposing the raw storage reference.
- Only Ready sources expose productive actions, and those actions come from the
  live capability registry.
- Failed sources show preservation, eligibility and required approval from the
  read-only recovery plan. No retry or reprocess execution control is exposed.

## Security, privacy and provenance

The server remains authoritative for authentication, workspace ownership,
bytes, MIME/type inference, final classification, duplicate identity,
retention, queue admission and custody. Browser checks are only usability
preflight. Opening the modal or inspecting the UI does not retain local content.
Raw storage paths are not displayed. A runtime upload will create retained
evidence, job, record and custody state and therefore requires explicit approval.

## Acceptance evidence

- scoped ESLint: zero errors and six warnings (three JSX-import false positives
  plus three pre-existing catch-variable warnings in `api.js`);
- Vite production build: pass, 677 modules;
- `e2e/analyst-portal.spec.js`: 9/9 pass in Chromium;
- responsive assertions: 390, 820, 1024 and 1440 px pass;
- keyboard account access, theme switch and clean console assertions pass;
- real browser: Data catalog, Add Data dark/light layouts and failed recovery
  plan pass;
- retained uploads: zero.

## Runtime and deployment decision

The proposed safe fixture is
`ingestion/forensic_records/tests/fixtures/pakistan_cdr_messy_synthetic.csv`.
Do not upload it until the user explicitly approves the retained mutation. The
existing deployed backend can execute the runtime check without a Docker
rebuild. The frontend preview is sufficient for source/manual review; production
frontend activation remains a separate guarded build/redeploy decision.

DeploymentNeeded: NO  
Frontend-only preview sufficient: YES  
Backend/runtime changes: none in source; one retained synthetic upload pending approval  
Verified deployment gate: not invoked; existing backend intake is already live  
Reason: APF-2 is a frontend composition over deployed contracts  
Exact operator action: continue reviewing `http://127.0.0.1:3000/analyst/data`; explicitly approve the named synthetic fixture before any retained runtime upload

## Retained-runtime attempt — halted

The user approved exactly one upload of the named synthetic fixture. Preflight
confirmed the 1,020-byte repository test fixture, SHA-256
`cf6792d8339f3a3cd79c2845eef2642d8fef480cc237062df30a20d6e17561e1`,
the `nexusai-forensic-demo` case/collection and no existing hash/name match.

The upload and backend processing passed:

- evidence: `4320772b-febe-4af2-b9e3-92cea114da0b`;
- version: `ad4b97c9-54eb-4657-afa7-5f06f52ba695`;
- job/run: `605b53d8-195f-4dde-aae6-081fbef6b500`, completed/succeeded on
  attempt 1 of 5;
- classification: CDR, structured records, `cdr` adapter;
- rows: 5 total, 4 canonical/inserted, 1 duplicate, 0 rejected;
- indexed entities: 26;
- KB asset: `8d772259-6d03-4fa8-923a-1f9c153d331d`;
- retention: verified SHA-256 full-read-after-retain, write-once;
- custody: 3 events, zero broken links, valid chain.

Before → after: sources 10 → 11; Ready 9 → 10; Processing 0 → 0;
Needs attention 0 → 0; Failed 1 → 1; accepted records 9,274 → 9,278;
CDR indexed records 5,004 → 5,008; registered evidence 10 → 11;
normalized evidence 9 → 10.

The acceptance then halted. The Add Data ledger and catalog correctly reported
Ready, but modal **Open source** rendered `0 verified rows`; authoritative
accounting is 4. The detail response lacks `item.accepted_rows`, the modal
callback supplies only evidence ID and filename, and the drawer falls back to
zero. Per the approved failure rule, Ask/citation/History validation did not
run. History remains at 50 entries, no acceptance analysis was created, and no
retry, reprocess, retained-data repair, cleanup or second upload occurred.

Runtime status: `blocked_on_ui_accounting_truth`. Fix and test this bounded UI
defect, then resume from the preserved evidence without another upload. APF-2
is not closed and APF-3 has not started.
