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
| 11 | Multi-case question scope for a top-level Investigate page: `scope` of selected case IDs or all permitted cases, resolved by the server; results grouped by case with per-case state, and `searched_completely` per case (team lead request, 2026-09-29) | Investigate (global) | open, planned as roadmap Phase 2 in `docs/work/RUNTIME_QUERY_PLAN_20260929.md`. Until per-analyst permissions exist, "all" means every case in the workspace. **UI stopgap shipped 2026-09-30:** `/investigate` asks every case with evidence (three at a time, `lib/acrossCases.js`) and shows one verified result per case, best first, with coverage. It cannot reason across cases (no joined answer, no cross-case ranking), costs one request per case, and shows per-case failures rather than `searched_completely`; this request replaces it |

| 12 | Workspace summary: totals and per-case governed summaries in one request, replacing one `/collections/status` call per configured case | Dashboard, sidebar badges, Cases | open (proposed 2026-09-30) |
| 13 | Needs-attention feed across cases: failed sources, retained-copy gaps and stalled jobs, each with a reason label and an openable locator, complete and countable | Dashboard, Activity | open (proposed 2026-09-30) |
| 14 | Reprocess a failed source (retry). **Corrected: the endpoint already exists**; only `retryable` and attempt counts on attention items remain open | Dashboard, Evidence detail | mostly delivered; small addition open |
| 19 | Complete, bucketed case activity history and top entities (counterparties, locations, plates, domains) for the Dashboard hero | Dashboard, Overview | open (proposed 2026-09-30); until then the hero uses `template=activity_by_day` and labels a 100-row cap as partial |
| 15 | Live workspace events stream, so the Dashboard updates without polling and can say what just finished | Dashboard, Activity | open (proposed 2026-09-30) |
| 16 | Case data-quality and coverage summary per record family (kept, duplicate, rejected, missing critical fields, first and last event time) | Dashboard, Overview | open (proposed 2026-09-30) |
| 17 | Report history: list, open and download generated case reports | Dashboard, Overview | open (proposed 2026-09-30) |
| 18 | Case display name and open/on-hold/closed status, stored per collection | Dashboard, Cases, sidebar | open (proposed 2026-09-30, needs owner sign-off) |
| 20 | Intraday timeline: `activity_by_hour` (or `bucket=hour\|day\|week`) per record family with time basis, so the timeline can zoom from days to hours and show a day-of-week by hour-of-day pattern. Extends row 1 | Timeline | open (proposed 2026-10-02) |
| 21 | Case timeline annotations: pin a day or event, note, tag, star, with author and time; shared across analysts on the case; list, create, update, delete. Today pins and notes live in the browser only (Timesketch-style stars, tags and comments) | Timeline, Investigate | open (proposed 2026-10-02) |
| 22 | Event clusters and co-occurrence: events from different families within a stated window (for example a call and a plate read within five minutes), each cluster with its member events and locators. Extends row 1 | Timeline | open (proposed 2026-10-02) |
| 23 | Case record: a stored case with display name, status, `created_by` (subject), `created_at` and owner, created when the first evidence is accepted. Today a case is only a `collection_id` string on evidence rows | Cases, Dashboard, sidebar, "My cases" | open (proposed 2026-10-02; extends 18) |
| 24 | Case membership and listing: members with roles (owner, editor, viewer); list cases with `mine=true` or `member=<subject>`; the server enforces access | Cases, Dashboard, "My cases" | open (proposed 2026-10-02) |
| 25 | Real identity end to end: the signed-in person's subject forwarded by the gateway instead of the constants `investigation-workspace` and `nexusai-local-operator`; the API ignores a form-supplied `user_id` whenever auth is on; `GET /whoami` returns subject, display name and role | Everywhere | open (proposed 2026-10-02) |
| 26 | Expose provenance that is already stored: `registered_by` and `registered_at` on evidence list and detail (from `evidence_items.user_id` and `evidence_versions.created_by`), and a read endpoint for custody events with `actor_type` and `actor_id`, for the evidence Lineage tab and Activity | Evidence, Activity | open (proposed 2026-10-02) |

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

## 14. Reprocess a failed source: CORRECTION 2026-09-30

**The backend already has this.** `POST /evidence/{evidence_id}/reprocess` and `GET /evidence/{evidence_id}/reprocess-plan` exist (`api/forensic_records/reprocess.go`). The first version of this request said they were missing; that was wrong. The request body needs `tenant_id`, `collection_id`, a `reason` of 3 to 1000 characters, and an `Idempotency-Key` (8 to 128 safe characters, header or body); `max_attempts` defaults to 5 (1 to 10). The UI can build Retry on it now: a small confirm dialog that asks for the reason, sends a generated idempotency key, and shows the result.

**Bulk retry by cause (new ask, 2026-09-30):** the Dashboard now groups failures by cause, so "retry all sources that failed for this cause" is the natural action. Ask: `POST /collections/reprocess` with `{ tenant_id, collection_id, evidence_ids: [..] | cause_label, reason, idempotency_key }`, returning a job per source and the same refusal reasons as the single call, capped at a stated batch size. It must not retry sources whose retained copy is missing.

**What is still worth asking for (small):** `GET /workspace/attention` items (row 13) should carry `retryable: boolean`, `attempts` and `max_attempts`, so the UI shows Retry only where it will work, without calling `reprocess-plan` per row. Until row 13 lands, the UI can call `reprocess-plan` for the selected failed source when its Retry dialog opens.

## 19. Case activity history (complete, bucketed) and top entities

- **Why:** the Dashboard hero should show what is *in* the evidence, not only the pipeline. Today the only whole-case aggregate is `POST /query/hybrid` with `template=activity_by_day`, which works with no target and returns `records.activity_by_day: [{ activity_date, record_type, event_count }]` across every record family, but a template call is capped at 100 rows (`maxHybridLimit`) and ordered oldest first, so a case with more than 100 day-and-family buckets is silently truncated. The UI labels that as partial; it needs a proper endpoint.
- **Endpoint:** `GET /collections/activity?tenant_id=&collection_id=&bucket=day|week|month&from=&to=`.
- **Response:** `{ collection_id, bucket, from, to, complete: boolean, buckets: [{ start: RFC3339, end: RFC3339, by_family: [{ record_type, events }], events_total }], families: [{ record_type, label|null, events_total }], first_event_at, last_event_at }`. The server picks the bucket when omitted so the response stays a bounded size, and `complete=false` states why.
- **Top entities** (for the "key entities" bubbles): `GET /collections/entities?tenant_id=&collection_id=&kind=counterparty|location|plate|domain|ip&limit=`, `{ kind, complete, items: [{ value_label, events, first_seen_at, last_seen_at, locator }] }`. Today `frequent_contacts` and `top_locations` need a target and refuse an empty one. Entity values are returned exactly as the existing masked routes return them; a kind the case has no data for returns `items: []`.
- **Also for the hero:** `GET /collections/activity/rhythm?...` returning a weekday-by-hour matrix `{ cells: [{ weekday: 0-6, hour: 0-23, events }], timezone, complete }` for a heatmap of when a case is active (today only CDR hourly counts exist through `activity_by_hour`, with no weekday), and `key_moments`: the busiest days with their record family mix, so the chart can annotate them. Both are aggregates over the same canonical records as `activity_by_day`.
- **Privacy:** aggregates and the same masking as the existing evidence and query routes; identity numbers are never returned unmasked.
- **Page:** Dashboard hero and Overview.

## 15. Live workspace events

- **Endpoint:** `GET /workspace/events?tenant_id=...&since=<event_id>` as Server-Sent Events (fall back to long-poll returning the same JSON). Heartbeat comment every 25 s.
- **Event:** `{ event_id, occurred_at, type, collection_id, evidence_id|null, source_file|null, status|null }` with `type` in `evidence.registered | processing.started | processing.completed | processing.failed | report.generated`. Ordered, resumable by `since`; a gap returns `410` so the UI reloads the summary instead of guessing.
- **Authorization:** only collections the viewer may see; the stream ends with `403` if authorization changes.
- **Why:** the Dashboard currently polls every 15 s while anything is processing and cannot say what just changed. With events it can update the cards in place, toast "call-log-march.csv finished processing", and drop polling entirely.
- **Privacy:** same masking as the evidence list; no content, only labels the service already returns.

## 16. Case data-quality and coverage summary

- **Endpoint:** `GET /collections/health?tenant_id=&collection_id=`.
- **Response:** `{ collection_id, generated_at, families: [{ record_type, label|null, accepted_rows, duplicate_rows, rejected_rows, first_event_at: RFC3339|null, last_event_at: RFC3339|null, missing_critical_fields: [{ field_label, missing_rows }] , rejected_sample_locator: object|null }], readiness: { ready: boolean, reasons: [ string ] } }`.
- **Semantics:** unsampled aggregates over the canonical records. `first_event_at` and `last_event_at` are the true earliest and latest event times per family (null when a family has no timestamp). `readiness` reuses the existing deterministic case-readiness computation and its reasons, so the Dashboard can say "ready for analysis" or why not. `missing_critical_fields` uses curated field labels.
- **Why:** "can I trust the evidence" is the second question of the Dashboard. Today the UI has only totals. Coverage dates tell an investigator at a glance what period the data spans, and the rejected sample locator lets them open the rows that were dropped.
- **Empty and error:** a case with no records returns `200` with `families: []`. `403` and service errors stay distinct.
- **Privacy:** aggregates and labels only; the sample locator opens rows through the normal masked evidence route.

## 17. Report history

- **Endpoints:** `GET /reports?tenant_id=&collection_id=&cursor=` and `GET /reports/{report_id}` (the Markdown body). `POST /reports/generate` (exists) additionally returns and stores `report_id`.
- **Response:** `{ items: [{ report_id, collection_id, title, target_label|null, created_at, size_bytes, generated_from: { collection_id, evidence_count, generated_at } }], next_cursor }`. The stored body is exactly what was generated (deterministic), with its inputs listed.
- **Why:** the deterministic report generator exists but its output is not kept, so an analyst cannot find a brief they made yesterday. A "Recent reports" list and a one-click "Generate case brief" make the last step of the workflow (reporting) visible on the Dashboard.
- **Audit:** generation is a custody event (row 10). **Privacy:** reports are masked at generation; listing shows titles only.

## 18. Case display name and status

- **Endpoints:** `GET /collections/{id}/meta` and `PATCH /collections/{id}/meta` with `{ display_name: string|null, status: "open"|"on_hold"|"closed" }`; `GET /collections` (row 9) includes both.
- **Why:** collection IDs such as `nexusai-multimodal-product-acceptance` are long and unfriendly everywhere the UI shows a case. A display name and a simple lifecycle status make the sidebar, Dashboard and Cases readable and let closed cases drop out of "needs review".
- **Constraint:** no owner, assignment, priority or classification is requested; those wait for an authoritative source. `display_name` is user-entered text and is shown escaped with `dir="auto"`. Needs owner sign-off before it is built.
- **Audit:** changes are custody events (row 10).

## 23 to 26. Ownership, membership and real identity

Why: "my own cases" and any per-person view depend on facts the system does not yet hold. Findings are in
`MODEL_AND_ARCHITECTURE_RESEARCH_20261002.md` (Pass 1).

- **23 Case record.** `GET /cases`, `GET /cases/{case_id}`, `PATCH /cases/{case_id}` (display name, status). Fields:
  `{ case_id, display_name|null, status: open|on_hold|closed, created_by: subject|null, created_at, owner: subject|null }`.
  `created_by` is set from the authenticated subject when the first evidence is accepted. Existing cases are backfilled with
  `created_by: null` unless the database shows exactly one real subject for the case; unknown stays unknown, never guessed.
- **24 Membership.** `GET|PUT|DELETE /cases/{case_id}/members` with `{ subject_id, role: owner|editor|viewer }`;
  `GET /cases?mine=true` returns cases where the caller is a member. Access to a case, its evidence and its answers is decided by
  membership on the server, never by the client.
- **25 Identity.** The gateway sets `X-Forensic-Subject-ID` from the sign-in (LocalAI users, `core/http/auth`), the API
  rejects a request-supplied `user_id` when authentication is required, and `GET /whoami` returns
  `{ subject_id, display_name, role }`. The workspace stops sending constants.
- **26 Provenance read.** Add `registered_by` and `registered_at` to the evidence list and detail responses; add
  `GET /evidence/{id}/custody` returning `{ events: [{ at, event_type, actor_type, actor_id, reason }] }` in chain order.
- **Empty and error behaviour.** A case with no known creator returns `created_by: null`; the UI says "not recorded".
  Forbidden stays `403`; an unavailable identity service must not be shown as an anonymous person.
- **Privacy.** Subject ids are identifiers; show display names only to members of the same case.
