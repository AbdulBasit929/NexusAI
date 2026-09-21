# NexusAI Phase 5 Case Governance and Workspace Completion

Date: 2026-07-30  
Status: source-complete and tested; not deployed in this phase  
Safety: zero retained-data, model, collection, container, or Git publication changes

## Outcome

Phase 5A, 5B, and the Phase 5C UI foundation now form one bounded vertical
slice:

1. versioned adapter, operation, specialist-agent, model-role, query-plan, and
   response contracts;
2. a read-only collection-governance and reconciliation surface;
3. one URL-authoritative case scope across Case Workspace, evidence upload,
   deterministic/hybrid query, reports, and forensic Agent Chat;
4. a six-route responsive Case Workspace with analyst-first and advanced
   settings surfaces;
5. compatibility preservation for the legacy hybrid query and Records UI flows.

The attached architecture document was compared with
`docs/design/nexusai-family-adapter-agent-api-architecture.md`; all 589 lines
were identical. The repository design remains the approved target.

## Phase 5A completion

- Shared catalog: `api/forensic_records/contracts/forensic-platform-v1.json`.
- Read-only discovery:
  - `GET /api/v1/forensics/adapters`
  - `GET /api/v1/forensics/operations`
  - `GET /api/v1/forensics/agents`
  - `GET /api/v1/forensics/contracts`
- Go types separate deterministic findings, citations, inference, successful
  model interpretation, unsupported operations, tables, visualizations, and
  execution trace.
- The new case query route calls the accepted legacy hybrid handler internally;
  `/query/hybrid` output is not changed.
- Raw queries and accepted `forensics.query-plan/v1` plans are supported. Typed
  plans map registered query operations to current deterministic templates.
  Detection/normalization requests return typed unsupported results, and plans
  with clarifications return `needs_input` without executing tools.
- Model interpretation is omitted when synthesis falls back.

## Phase 5B completion

### Case APIs

LocalAI exposes the records-feature-protected public surface:

- `GET /api/v1/forensics/cases`
- `GET /api/v1/forensics/cases/{case_id}`
- `GET /api/v1/forensics/cases/{case_id}/manifest`
- `GET /api/v1/forensics/cases/{case_id}/evidence`
- `POST /api/v1/forensics/cases/{case_id}/query`
- `POST /api/v1/forensics/cases/{case_id}/reports`

The URL case is authoritative. A different `case_id` or `collection_id` in a
body returns a conflict. The authenticated sidecar additionally binds both
`X-Forensic-Collection-ID` and `X-Forensic-Case-ID`; a header mismatch is
forbidden before a query runs.

### Agent Chat scope

- Chat requests accept `case_id` and `collection_id` and require equality.
- LocalAI verifies collection access, copies the stored agent configuration,
  injects the request-bound collection into the copy, and leaves retained agent
  configuration untouched.
- Both deterministic direct routes and model-selected forensic tools receive
  that copied scope in local and distributed execution modes.
- Legacy forensic chat without a body scope resolves and returns the configured
  scope explicitly; it cannot execute with an unnamed collection.
- The React forensic chat defaults to the governed case, carries it in the URL,
  displays a selector, sends it in every chat request, and stores conversation
  histories under case-specific keys.

### Governance and manifest

- `records-demo-verified` is the governed local default when authorized.
- Legacy demos remain visible with reconciliation warnings.
- acceptance/validation/audit/golden collections and the two accidental
  analyst-named collections are hidden from analyst selectors.
- Admins may explicitly request system collections.
- The sidecar manifest runs in a read-only PostgreSQL transaction and counts KB
  asset links, evidence, canonical/legacy rows, entities, jobs, families, and
  audit events.
- LocalAI augments exact KB-entry, source, local-batch, and agent-binding counts.
- Vector cardinality remains explicit `external`; reports remain explicit
  `not_persisted`. Unknown values are never synthesized.
- Every manifest declares `read_only: true` and `deletion_count: 0`.

The live 20-collection snapshot and disposition are recorded in
`reports/forensic-phase5-collection-reconciliation-manifest-20260730.md`.

## Phase 5C completion

`/app/records` is now a compatibility entry that selects the governed default
and opens `/app/cases/:caseId/analyze`. The six routed pages are:

- Overview: evidence/row/KB/family KPIs, warnings, family coverage, manifest
  summary.
- Evidence: URL-bound upload, search/status filters, professional evidence
  table, provenance identifiers.
- Analyze: the existing analyst workbench, bound to the URL case, now using the
  v1 typed query response.
- Relationships: deterministic relationship/timeline/correlation/geospatial
  workflow launchers and a no-fabricated-edge empty state.
- Jobs: read-only queued/running/completed/failed/dead-letter accounting.
- Settings: case metadata, retention posture, full manifest, and advanced
  destructive controls visibly disabled pending policy/approval.

Responsive CSS uses bounded grids, `min-width: 0`, page-level overflow clipping,
scrollable tables, mobile navigation icons, focusable table wrappers, labeled
inputs, status text, and semantic headings/navigation. Advanced Records controls
remain behind explicit expanders/drawers.

## Verification

| Check | Result |
| --- | --- |
| `go test ./api/forensic_records -count=1` | PASS, 27.706 s |
| Phase 5 focused sidecar specs after typed-plan addition | PASS, 10.838 s |
| LocalAI Phase 5 governance specs | 6/6 PASS; package runner PASS when the correct internal suite is selected |
| Agent-pool compile check | PASS |
| LocalAI route compile check | PASS |
| `go vet` on sidecar, agent pool, LocalAI endpoints, routes | PASS, 57 s |
| ESLint on touched UI and E2E files | 0 errors; existing JSX/react-hook warnings remain |
| React production build | PASS, 658 modules, 1.67 s final run |
| Playwright desktop 1440×1000 | PASS, no page-level horizontal overflow |
| Playwright tablet 820×1000 | PASS, no page-level horizontal overflow |
| Playwright mobile 390×844 | PASS, no page-level horizontal overflow |
| Playwright keyboard/focus and case-bound evidence | PASS |
| Preserved Records Intelligence E2E flows | 3/3 PASS |
| Combined Playwright result | 7/7 PASS, 22.6 s, installed system Chrome (no download) |
| `git diff --check` on Phase 5 files | PASS; only line-ending conversion notices on existing tracked Go files |

The package-level LocalAI command can report a Windows `unlinkat ... Access is
denied` after printing successful test results because the host scanner holds
the temporary test executable. Running only `TestLocalAIInternalEndpoints`
avoids the repository's two-Ginkgo-suite collision and passed. This is a host
cleanup issue, not a compile or spec failure.

## Live read-only observations

- LocalAI `/readyz`: HTTP 200.
- forensic API `/healthz`: HTTP 200.
- NATS monitoring `/healthz`: HTTP 200.
- 20 collections remain present.
- `records-demo-verified`: 8 KB entries, 8 KB assets, 6 evidence items, 9,250
  accepted rows, 4 jobs, 0 failed jobs.
- `records-demo`: 6 KB entries, 2 KB assets, 8 evidence items, 10,000 accepted
  rows, 8 jobs, 6 failed jobs.
- Stored forensic agent binding remains
  `nexusai-structured-demo-v2-20260730`; source UI/request scope now overrides
  it with the governed case without mutating runtime state.
- No source changes were deployed, so new `/api/v1/forensics/cases` routes are
  source-tested rather than claimed live.

## Security and provenance impact

- Tenant/user collection access is verified before all LocalAI v1 case routes.
- The sidecar rejects collection and case header/body disagreement.
- Case-bound agent config is copied per request, avoiding shared mutable state.
- Exact SQL facts and model interpretation remain separately labeled.
- Model fallbacks cannot be advertised as successful interpretation.
- Manifest and cleanup behavior is read-only and zero-deletion.
- No raw evidence, full table, model, secret, or database state was copied into
  the source tree.

## Deferred approval gates

Phase 5 source completion does not authorize:

- deployment/rebuild/restart of the running stack;
- changing the stored agent collection;
- database migration or report-registry persistence;
- model download or model-role promotion;
- vector backend enumeration changes;
- collection merge, alias materialization, archive, soft-delete, or permanent
  deletion;
- staging, commit, push, pull request, or external publication.

The next bounded product slice is Phase 6.1, Communications/CDR, after an
operator separately approves any deployment and retained configuration change.

