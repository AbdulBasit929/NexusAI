# Backend requests — UI (Codex) to backend (Claude)

The UI adds a request here when a page needs data or a feature the API doesn't provide. Claude builds
it in `api/**`, measures it, and marks it delivered with the endpoint and an evidence folder. The UI
never mocks a requested endpoint in shipped views; the page shows its honest designed state until
delivery.

Each request states: the **endpoint and method**, the **exact response fields and types**, the
**page** it serves, **empty and error behaviour**, and **privacy notes**.

| # | Request | Page | Status |
|---|---|---|---|
| 1 | Timeline events across families for a case (time, family, summary, openable locator; paginated; filter by family, identifier, date) | Timeline | open |
| 2 | Connections for one identifier (typed relationships across families, verified links only) | Connections | open |
| 3 | Processing history per case (jobs per day by status) | Dashboard | open |
| 4 | Image overlay data for an evidence item (OCR regions and plate-read boxes with confidence, privacy-aware) | Evidence detail | open |
| 5 | Server-side question history (sessions) | Investigate, Activity | open |
| 6 | Result presentation hints: display labels for result columns (today "M1"), suggested chart type | Investigate | **labels delivered 2026-09-29** (`data_grid.columns[].header`, `reports/column-labels-20260929`); chart hint open |
| 7 | `source_truth_state` on transcript and image-text citations (today absent, so the UI can only say "Source record") | Investigate | **delivered 2026-09-29**: `provenance[].source_truth_state` and `fact_packet.citations[].source_truth_state` (`derived_model_observation`, `derived_native_text`, `derived_text_representation`; absent on structured records), `reports/a1-1-a2-20260929` |
| 8 | Complete case evidence census by modality, family and processing state | Dashboard, Overview, Evidence | open |
| 9 | Authorized collection directory with summary and pagination | Dashboard, Cases | open |
| 10 | Authorized workspace and case activity/audit history | Dashboard, Activity | open |
| 11 | Multi-case question scope for a top-level Investigate page: `scope` of selected case IDs or all permitted cases, resolved by the server; results grouped by case with per-case state, and `searched_completely` per case (team lead request, 2026-09-29) | Investigate (global) | open, planned as roadmap Phase 2 in `docs/work/RUNTIME_QUERY_PLAN_20260929.md`. Until per-analyst permissions exist, "all" means every case in the workspace |

| 12 | Workspace summary: totals and per-case governed summaries in one request, replacing one `/collections/status` call per configured case | Dashboard, sidebar badges, Cases | open (proposed 2026-09-30) |
| 13 | Needs-attention feed across cases: failed sources, retained-copy gaps and stalled jobs, each with a reason label and an openable locator, complete and countable | Dashboard, Activity | open (proposed 2026-09-30) |

Rows 1–6 were anticipated in `CODEX_UI_REDESIGN_PROMPT_20260929.md` §8, and row 7 came from the
2026-09-28 demo check (`CODEX_UI_DEFECTS_20260928.md`). Codex: add detail under a heading per request,
and add new rows as you find them.

## 3. Processing history per case

- **Endpoint:** `GET /collections/processing-history`
- **Scope and filters:** required `tenant_id`, required `collection_id`, required `from`, required `to`, optional `bucket=day|hour`; normal forensic authorization headers remain binding.
- **Response:** `{ collection_id: string, from: RFC3339, to: RFC3339, bucket: string, complete: boolean, buckets: [{ start: RFC3339, end: RFC3339, jobs_total: integer, completed: integer, failed: integer, processing: integer, accepted_rows: integer, duplicate_rows: integer, rejected_rows: integer }] }`.
- **Semantics:** every bucket must cover the requested interval without sampling. `complete=false` must include a machine-readable limitation and the UI will label the chart partial.
- **Empty/error:** a resolved interval with no jobs returns `200` and `buckets: []`; forbidden remains `403`; an unavailable history service must not return zero-shaped buckets.
- **Privacy:** aggregate counts only. No evidence text, identifier values, internal worker/model names, or fabricated duration percentage.

This supports the Dashboard's processing/failure trend and the Overview's ingestion-history view. The current bounded `recent_jobs` array cannot support either claim.

## 8. Complete case evidence census

- **Endpoint:** `GET /evidence/summary`
- **Scope:** required `tenant_id` and `collection_id`; optional evidence-family and processing-state filters must use the same vocabulary as `GET /evidence`.
- **Response:** `{ collection_id: string, evidence_total: integer, size_bytes_total: integer|null, by_modality: [{ id: string, label: string|null, evidence_count: integer, size_bytes: integer|null }], by_family: [{ id: string, label: string|null, evidence_count: integer, accepted_rows: integer|null }], by_processing_state: [{ id: string, label: string, evidence_count: integer }] }`.
- **Semantics:** totals are collection-wide and unsampled. Curated labels are returned by the service; an uncurated `id` remains unchanged. `null` size or row counts mean unavailable and must not become zero.
- **Empty/error:** a resolved empty collection returns `200` with zero total and empty arrays; `403` and service errors remain distinct.
- **Privacy:** aggregates only; never emit source content, unmasked identifiers, or model-derived labels.

This enables interactive ranked evidence-family/modality bars on Dashboard and Overview with drill-through to the matching Evidence filters. The bounded `recent_evidence` sample is not a census and will not be used as one.

## 9. Authorized collection directory

- **Endpoint:** `GET /collections`
- **Filters:** cursor pagination, search by exact/substring collection ID, and optional readiness state. Authorization determines the visible collection set before filtering.
- **Response:** `{ items: [{ collection_id: string, created_at: RFC3339|null, latest_activity_at: RFC3339|null, summary: <same governed summary shape as /collections/status> }], next_cursor: string|null, total: integer|null }`.
- **Semantics:** `total=null` is allowed when the service cannot cheaply provide an exact authorized total. Do not add owner, assignment, title, classification, priority or display name unless each gains an authoritative source.
- **Empty/error:** `200` with `items: []` is the only directory empty state; forbidden and unavailable remain explicit.
- **Privacy:** the directory must be authorization-filtered server-side and must not reveal collection IDs through counts, cursors or search timing outside the viewer's scope.

This removes the Dashboard and Cases dependency on served `caseIds` and the current one-request-per-configured-case fan-out.

## 10. Authorized activity and custody history

- **Endpoint:** `GET /activity`
- **Filters:** required authorized scope (`tenant_id`, optional `collection_id`), cursor, time interval, event type and actor only when the viewer may see actor identity.
- **Response:** `{ items: [{ event_id: string, occurred_at: RFC3339, event_type: string, event_label: string, collection_id: string|null, evidence_id: string|null, actor_label: string|null, outcome: string, openable_locator: object|null }], next_cursor: string|null }`.
- **Semantics:** event labels are curated by the service. Evidence lifecycle, custody, query, export and access events remain distinguishable; absence of actor identity is `null`, never “System”.
- **Empty/error:** a completed search with no events returns `200` with an empty list; partial history must carry an explicit completeness field and reason; `403` remains forbidden.
- **Privacy:** authorization and server-side CNIC masking apply before projection. Do not expose internal agents, models, operations, SQL, templates, token counts or backend identifiers.

This enables a truthful activity volume view with an accessible event table and case drill-through. Browser-local questions and bounded recent evidence remain labelled working records until this endpoint exists.

## 12. Workspace summary

- **Endpoint:** `GET /workspace/summary`
- **Scope:** required `tenant_id`; the authorized set of collections is resolved by the server (today, every collection in the workspace, per row 11's note).
- **Response:** `{ collections_total: integer, totals: { evidence_total, evidence_completed, evidence_in_flight, evidence_failed, completed_jobs_missing_kb_asset, accepted_rows, duplicate_rows, rejected_rows }, items: [{ collection_id: string, latest_activity_at: RFC3339|null, summary: <same governed summary shape as /collections/status>, record_families: [{ record_type, accepted_rows }] , status: "ok"|"forbidden"|"unavailable" }], complete: boolean }`.
- **Semantics:** `totals` sums only collections whose `status` is `ok`; the count of the others is derivable from `items`. `complete=false` names why (for example a collection timed out). A collection that could not report is never zero-shaped.
- **Why:** the Dashboard, the sidebar badges and the Cases page currently make one request per configured case on every load and refresh. One request is faster, atomic (all figures from one moment) and removes the `caseIds` configuration dependency together with row 9.
- **Privacy:** aggregates only; no evidence text, identifiers or model and worker names.

## 13. Needs-attention feed

- **Endpoint:** `GET /workspace/attention`
- **Filters:** required `tenant_id`; optional `collection_id`, `kind`, cursor pagination.
- **Response:** `{ counts: { failed: integer, retained_copy_missing: integer, stalled: integer }, items: [{ collection_id: string, evidence_id: string|null, source_file: string|null, kind: "failed"|"retained_copy_missing"|"stalled", reason_label: string|null, occurred_at: RFC3339|null, attempts: integer|null, max_attempts: integer|null, openable_locator: object|null }], next_cursor: string|null, complete: boolean }`.
- **Semantics:** unsampled and countable; `counts` are collection-wide and equal the number of items across all pages. `reason_label` is a curated analyst-safe sentence (never a stack trace, model, template or SQL). `stalled` means a job has been processing longer than the service's own threshold, which the response states.
- **Why:** the Dashboard's needs-review list can only name the most recent items each case reports (`recent_evidence`, `recent_jobs`), so it discloses that it may be incomplete. This makes it a complete, paginated queue with exact counts and one-click review.
- **Empty/error:** `200` with zero counts and no items is the only "nothing needs review" state; `403` and service errors stay distinct.
- **Privacy:** server-side masking applies before projection; file names are shown only as the service returned them.
