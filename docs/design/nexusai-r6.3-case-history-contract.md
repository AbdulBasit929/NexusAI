# NexusAI R6.3 Case Query History and Saved Analysis Contract

Status: R6.3-A source complete; guarded deployment acceptance remains pending.

## Authority boundary

The React agent workspace may still contain pre-R6.3 browser conversations,
but the authoritative history and saved-analysis surfaces now use the retained
server contract. Browser records remain explicitly labelled local, are never
merged automatically, and can enter retained history only after a user confirms
a bounded, versioned import.

The browser-local layer is transitional. It is not the enterprise system of
record and must never be returned by public forensic APIs as authoritative case
history before import.

## Retained contract

The retained contract is versioned as `forensics.case-analysis-history/v1` and
scoped by tenant, authenticated user, case, agent, and request. An entry
preserves:

- immutable analysis and request message IDs;
- matching case and collection IDs under the current v1 boundary;
- query text, terminal lifecycle status, timestamps, and retry lineage;
- final answer sections, citations, deterministic authority, and model role;
- saved state, title, saver identity, and saved timestamp;
- source and contract metadata needed to reproduce or audit the answer.

List and detail reads are authorized through the canonical case registry.
Save/unsave mutations are idempotent and auditable. Opaque cursor pagination is
ordered by `(created_at DESC, id DESC)`. Final answers and typed answer metadata
are retained; hidden reasoning, live tool streams, credentials, and SSE payload
history are deliberately excluded.

## Storage and lifecycle

`agent_analysis_history` is an additive PostgreSQL table migrated by the agent
store. Its unique request boundary is tenant, user, agent, case and message ID.
The current v1 contract requires `case_id == collection_id`. A request row is
created before dispatch, updated by correlated lifecycle events, and completed
with the final agent answer. The API-side event bridge performs the same update
for distributed workers, so worker database access is not required.

Runtime and imported records carry different source values. Browser imports use
deterministic, case-scoped message IDs, making repeated import non-duplicating.
Each import gets an `import_id`; rollback removes only unsaved,
non-legal-hold rows from that import. Saved or held rows are reported and
retained. Runtime records can never be removed through import rollback.

Retention expiry and legal-hold mutation are intentionally not automated in
R6.3-A. Until governance supplies an approved duration and hold-management
workflow, records remain retained and `legal_hold` is fail-safe. Database
backup, restore, and privileged retention administration remain operator
responsibilities, not browser actions.

## Public API

- `GET /api/agents/{name}/history` — authorized list and saved-only views.
- `GET /api/agents/{name}/history/{analysis_id}` — authorized detail.
- `PUT /api/agents/{name}/history/{analysis_id}/saved` — idempotent save state.
- `POST /api/agents/{name}/history/import` — explicit browser-local import.
- `DELETE /api/agents/{name}/history/imports/{import_id}` — bounded import rollback.

Every operation requires an authenticated user, a forensic-enabled agent and a
canonical authorized case with matching collection identity. Import is limited
to 5 MiB, 500 conversations and 5,000 messages.

## Migration and rollback

Deployment requires an explicit PostgreSQL connection through
`LOCALAI_AGENT_POOL_DATABASE_URL`. The schema change is additive and preserves
all existing named volumes. The guarded activation gate first checks generated
Swagger and configuration, then uses the proven sequential rebuild/health gate
and performs GET-only live acceptance. It sends zero history mutations.

Application rollback restores the preserved images while keeping the additive
table and retained rows. Dropping the table is not an application rollback and
is never automated. A database/schema rollback requires a separately approved,
verified backup and retention/legal review.

## Slice status

1. R6.3-A1: versioned and truthfully labelled browser-local foundation.
2. R6.3-A2: retained schema, tenancy indexes and governance boundary.
3. R6.3-A3: authorized list/detail/save contracts and generated API surface.
4. R6.3-A4: server-authoritative UI and explicit reversible browser import.
5. R6.3-A5: source acceptance and operator-run guarded live acceptance.

Slices A1-A5 are source complete. R6.3-A becomes live complete only after the
operator runs the guarded rebuild and the generated acceptance marker passes.
