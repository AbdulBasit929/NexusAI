# API contract: conversations, knowledge capture, members and authority (the case layer)

**For the interface developer.** Status: **built on branch `claude/case-backend-76c2b9`, tested, not deployed, migration 034 not applied to the live database.** Every example below is a real response of the running code on a throwaway database (ids shortened where a line would be too long; nothing here is mocked). Design: [`docs/architecture/CONVERSATIONS_AND_CASE_LAYER_20261103.md`](../architecture/CONVERSATIONS_AND_CASE_LAYER_20261103.md). Requests served: BR-2 (conversations), BR-3 (knowledge capture) and the member and authority part of BR-1 in [`BACKEND_REQUESTS.md`](BACKEND_REQUESTS.md).

## 0. Conventions

* **Same door as the rest of the API.** Send the headers `apiClient.js` already sends (`X-Forensic-Tenant-ID`, `-Actor-ID`, `-Subject-ID`, `-Actor-Role`, `-Collection-ID`). The proxy at `/api` forwards every path below unchanged (it strips `/api`). A credential bound to one case (`X-Forensic-Collection-ID`) sees nothing of any other case (`404`) and `GET /conversations` without `case_id` lists only that case; a listing across cases needs a credential that is not bound to one (the multi-case question scope, request 11, is separate).
* **Who you are** is the verified subject (`X-Forensic-Subject-ID`). Today the workspace sends one shared subject, so every conversation has the same owner; per-person ownership starts working the day real identity (request 25) is wired, with no change here.
* **`case_id` is the collection id**, as in `GET /cases/{case_id}`. Ids are UUIDs (lower case) except member subjects. Times are RFC 3339 **in UTC** (`...Z` or `...+00:00`). JSON, UTF-8, request bodies at most 1 MiB, unknown fields are refused with `400` and the list of fields allowed.
* **Lists** answer `{ "items": [...], "next_cursor": "" }`; pass `next_cursor` back as `cursor` for the next page. `limit` is 30 by default, 1 to 100. A cursor is opaque; an empty `next_cursor` is the last page.
* **Errors** answer `{ "error": "<a sentence to show>", "code": "<machine code>", "request_id": "..." }`:

| Status | `code` | Meaning |
|---|---|---|
| 400 | `invalid_request` | the sentence says which field and what is allowed |
| 401 | `unauthenticated` | no valid credential |
| 403 | `forbidden` | your role or ownership does not allow it (the sentence says which) |
| 404 | `not_found` | no such object **or you may not see it**; also for a case you are not a member of |
| 409 | `duplicate`, `immutable`, `hidden`, `archived`, `last_owner`, `withheld`, `not_an_answer`, `not_finished`, `already_revoked`, `retry` | the request is understood and the state forbids it |
| 413 | `too_large` | body over 1 MiB |
| 500 | `internal` | not shown further; quote `request_id` |
| 503 | `not_installed`, `unavailable` | **until migration 034 is applied every route answers 503 `not_installed`**; the page shows its designed "not available yet" state |

* **Roles.** A case with no members is **open**: everyone signed in acts as an analyst (`you.role = "open"`). The first member makes it **controlled**; then the role decides: `owner`, `lead`, `analyst` (write), `reviewer` (review findings, give feedback), `viewer` (read, feedback), `auditor` (read everything including private conversations and the audit trail; writes nothing). A global administrator is a lead everywhere. `GET /cases/{id}/members` returns `mode` and `you` so the page can say what the person may do.
* **Nothing is deleted.** `DELETE` hides (with an optional `reason`); the audit trail keeps who and why.

## 1. The history rail and the conversations

### `GET /conversations`
Query: `case_id` (omit for every case you may see; a bound credential is limited to its case), `q` (words; every word must be in a question or an answer; Urdu script, Roman Urdu and numbers in Urdu digits are folded: `كيا` finds `کیا`), `tag`, `pinned=true|false`, `archived=true|all` (default: not archived), `mine=true`, `shared=true` (conversations of others shared with you), `include_hidden=true` (your own hidden ones; an administrator or an auditor of the case sees all), `since`, `until` (RFC 3339, on the last turn), `limit`, `cursor`.
Order: pinned first, then newest. Group by day on `last_turn_at` in the page.

```json
{ "items": [ {
  "thread_id": "4f9bba0d-1533-4db9-8b88-935ade1d5576", "case_id": "orion-lahore-reference-20261030",
  "title": "Who called the most?", "display_title": "Who called the most?", "title_source": "user",
  "pinned": false, "archived": false, "visibility": "private", "shared_with": [], "owner": "alice", "is_owner": true,
  "turn_count": 2, "first_question": "Which numbers made the most calls?", "last_question": "and for March?",
  "last_headline": "3 numbers made calls: 923000000201 (14), 923000000202 (9), 923000000203 (4).", "last_outcome": "answered",
  "last_turn_at": "2026-11-03T16:37:05.299045Z", "created_at": "2026-11-03T16:37:04.735901Z", "updated_at": "2026-11-03T16:37:05.559641Z",
  "forked_from": null, "hidden": false, "changed_since": false, "tags": [] } ],
  "next_cursor": "" }
```
`display_title` is the title, or the first question shortened, or "New conversation". `changed_since` is true when a question of the conversation has a later run that differs: the rail's badge. With `q`, each item also has `"matches": [{ "turn_id", "idx", "question", "headline" }]` (at most three, newest first).

### `POST /conversations` → `201` thread
Body `{ "case_id": "...", "title": "..." }` (title optional; it stays automatic until the person names it). `404` for a case that has neither evidence nor a case record; `403` for a viewer or an auditor.

### `GET /conversations/{id}?answers=latest|all|none`
The thread (fields above) plus `active_turn_id` (the newest turn), `tags` and `turns`: every turn **without** its answer except, by default (`latest`), the newest finished one, which carries `answer`; `all` adds every stored answer (heavy); `none` none. A conversation is a **tree**: each turn has `parent_turn_id`, `kind` (`ask`, `edit`, `rerun`); the page draws the path from the root to `active_turn_id` and shows siblings. Open an older turn with `GET /turns/{id}`.

### `PATCH /conversations/{id}`
Any of `title` (text, or `null` to go back to automatic), `pinned`, `archived`, `visibility` (`private`, `selected`, `case`), `shared_with` (names; required and non-empty for `selected`; in a controlled case each must be a member). Owner or administrator only (`403`). Sharing is read-only for others: they read it, give feedback, pin findings from it, **fork** it; only the owner adds turns.

### `DELETE /conversations/{id}[?reason=...]` → `204`; `POST /conversations/{id}/restore` → thread
Hide and restore (owner or administrator). A hidden conversation leaves lists and answers `404` to everyone but its owner, an administrator and the case's auditors; its turns are kept.

## 2. Asking: turns

### `POST /conversations/{id}/turns`
Body `{ "question": "...", "client_request_id": "...", "parent_turn_id": "...", "edit_of_turn_id": "...", "language": "en" }` (only `question` is required).
* **Continue**: send `question` only (the new turn follows the newest). `parent_turn_id` answers below a particular earlier turn. **Edit and resubmit**: `edit_of_turn_id` of the question being replaced; the new turn is its sibling (same parent, `kind: "edit"`). Send one of the two, not both.
* **A retry is harmless**: the same `client_request_id` (1 to 100 characters of `A-Z a-z 0-9 . _ : -`) in the same conversation returns the turn that exists, `200` with `"existing": true`, and the engine is not asked again.
* The call runs the engine **to the end even if the tab is closed**; reopen the thread and `GET /turns/{id}` until `state` is not `running`. A turn lost with its server reads `interrupted`.
* National ID numbers typed in the question are stored masked (`#####-#######-#`); the engine still receives what was typed and withholds them.
* Only the owner of the conversation may add a question (`403`: "fork it to continue in your own conversation"); `409 hidden` / `409 archived` otherwise.

**Three encodings of one call.** Plain: no `Accept` (or `?stream=false`) answers when the engine is done (it can take minutes on the laptop), `201`:

```json
{ "turn": { "turn_id": "6ce7a207-8056-41b1-b7b0-3732006df37d", "thread_id": "4f9bba0d-...", "case_id": "orion-lahore-reference-20261030", "idx": 1,
  "parent_turn_id": null, "kind": "ask", "edit_of_turn_id": null, "rerun_of_turn_id": null, "root_turn_id": "6ce7a207-...",
  "question": "Which numbers made the most calls?", "resolved_question": null, "language": null,
  "frame": { "numbers": ["923001234567"], "dates": [], "limit": 0, "national_id_count": 0 },
  "resolution": { "method": "none" },
  "state": "completed", "outcome": "answered", "headline": "3 numbers made calls: 923000000201 (14), 923000000202 (9), 923000000203 (4).",
  "shape": "ranked", "row_count": 3, "query": "SELECT src AS label, count(*) AS value FROM v_comm_events GROUP BY 1 ORDER BY 2 DESC",
  "query_hash": "89984bdc...", "result_digest": "5d30988f...",
  "evidence_refs": [ { "evidence_id": "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "source_file": "calls.csv" } ],
  "snapshot_state": "full", "snapshot_bytes": 986, "engine_version": "round9d-20261102", "model": "test-model", "prompt_id": "p1",
  "data_version_hash": "492d3aed670ec13635b3fe63798ce96a7e9bb86f9f184fc788256d13561c277a",
  "timings": { "total_ms": 20, "queued_ms": 0, "engine_ms": 20, "model_ms": 10, "exec_ms": 5, "attempts": 1 },
  "diff_summary": null, "error": null, "created_by": "alice",
  "created_at": "2026-11-03T16:37:04.951637Z", "started_at": "2026-11-03T16:37:04.951637Z", "completed_at": "2026-11-03T16:37:05.237118Z",
  "reruns": { "count": 0, "latest_turn_id": null, "latest_changed": null },
  "live_tokens": true,
  "answer": { "...": "the engine's response exactly as POST /query/hybrid returns it" } },
  "diff": null, "existing": false }
```
**`answer` is the same object the page already presents** (`policy`, `answer`, `enterprise.data_grid`, `enterprise.evidence`, `enterprise.reading`...): feed it to the existing presentation code. **`live_tokens`** is `true` on the call that ran the turn; on a stored answer it is a hint, not a promise: the `reading` and per-line evidence tokens inside live in the engine's memory and are gone after a restart. If their endpoints answer `404`/`410`, offer **Run again**. A failed engine call is **a recorded turn with `state: "failed"` and `error`** (still `201`), so the history shows it.

`Accept: application/x-ndjson` (or `?stream=ndjson`): one JSON object per line, flushed as it happens. `Accept: text/event-stream` (or `?stream=sse`): the same objects as `event: <name>` / `data: <json>`. Events, in order (`at_ms` is milliseconds since the call began and never goes backwards):

| `event` | Fields | Meaning |
|---|---|---|
| `phase` `accepted` | `turn_id`, `thread_id`, `label` "Question received" | the turn exists and is in the history |
| `phase` `context` | `label` | the earlier questions are being read into this one (only when the follow-up role is on and there is history) |
| `phase` `queued` | `position` | waiting for the language model (answers one request at a time); position in the queue |
| `phase` `engine` | `label` "Answering" | the engine has the question |
| `phase` `engine.<name>` | `label` | a phase the engine itself reports (**not yet: the engine session adds them; until then you see `engine` and the heartbeat**). Planned names: `understanding`, `writing_query`, `checking`, `running`, `evidence`, `composing` |
| `heartbeat` | `elapsed_ms` | every 2 s while the engine works: show a running clock; it is not a progress percentage and never will be |
| `phase` `saving` | `label` | the answer is being stored |
| `turn` | `turn` (as above, with `answer`), `diff` (null unless a re-run) | the answer; **the last event on success is `done`** |
| `error` | `code` (`engine_failed`), `message`, `turn` (the failed turn) | the engine could not answer; `done` follows |
| `done` | | end of stream |

Only true phases are sent; the layer never invents progress. An error **before** the first event (validation, permission) is an ordinary JSON error with its status; after it, an `error` event.

```
{"event":"phase","phase":"accepted","label":"Question received","turn_id":"f75d3a72-...","thread_id":"4f9bba0d-...","at_ms":123}
{"event":"phase","phase":"engine","label":"Answering","turn_id":"f75d3a72-...","thread_id":"4f9bba0d-...","at_ms":128}
{"event":"heartbeat","turn_id":"f75d3a72-...","thread_id":"4f9bba0d-...","elapsed_ms":40,"at_ms":169}
{"event":"phase","phase":"saving","label":"Saving the answer","turn_id":"f75d3a72-...","thread_id":"4f9bba0d-...","at_ms":259}
{"event":"turn","turn_id":"f75d3a72-...","thread_id":"4f9bba0d-...","at_ms":292,"turn":{ "...": "the turn, with answer" }}
{"event":"done","turn_id":"f75d3a72-...","thread_id":"4f9bba0d-...","at_ms":292}
```
Browser: `fetch` with `response.body.getReader()` and split on `\n` (works through the `/api` proxy); `EventSource` cannot POST, so use `fetch` for SSE too.

**What the Ask page shows from a turn**: *Interpreted as*: `resolved_question` when not null (the follow-up role, off until measured), else `question`; `frame` is what the engine read (numbers, plates, dates, limit, cues); `resolution.method` is `none`, `model`, `rejected` (a proposed rewrite was refused because it added a value), `failed` or `reused` (a re-run). *Outcome*: `outcome` is `answered`, `abstained`, `clarification`, `withheld`, `unsupported`, `refused`, `failed` or `unknown`: design each as a result, not an error.

### `GET /turns/{id}[?answer=false]`
One turn with its answer, `query` (the engine's SQL, for administrators one level down), `feedback` (`{ "mine": {...}|null, "counts": { "wrong": 1 } }`) and `pinned_as` (ids of the findings it was pinned to). The turn of a hidden conversation is `404` to whoever may not see the conversation.

### `POST /turns/{id}/rerun` (same three encodings) → `{ "turn", "diff" }`
Asks the same question again against the data as it is now, as a **new turn of the same conversation** (`kind: "rerun"`, `rerun_of_turn_id` = this turn, `root_turn_id` = the first run of the question). A follow-up is not resolved again. `409 withheld` for a question that named a national ID number. Owner only. `diff` compares the new run with the one it re-ran:

```json
{ "of": "d89db1e4-...", "against": "6ce7a207-...", "since": "2026-11-03T16:37:05.237118Z",
  "changed": true, "figures_changed": true, "evidence_changed": false, "complete": true,
  "outcome":  { "before": "answered", "after": "answered", "changed": false },
  "headline": { "before": "3 numbers made calls: ...", "after": "4 numbers made calls: ...", "changed": true },
  "metrics":  [ { "label": "Number of records", "before": "5,005", "after": "5,010", "delta": "5" } ],
  "rows": { "before_count": 3, "after_count": 4, "columns_changed": false, "reordered": false, "listed_per_class": 50,
            "added_total": 1, "added": [["923000000204", 2]], "removed_total": 0, "removed": [],
            "changed_total": 1, "changed": [ { "key": "923000000202", "cells": [ { "column": "Calls", "before": 9, "after": 10 } ] } ] },
  "evidence": { "added": [], "removed": [], "replaced": [ { "evidence_id": "...", "name": "calls.csv", "before_version": "...", "after_version": "..." } ] },
  "data_version": { "before": "492d3aed...", "after": "492d3aed...", "known": true, "changed": false,
                    "detail": { "evidence_added": [ { "evidence_id": "...", "name": "new-calls.csv" } ], "evidence_removed": [], "evidence_replaced": [], "families": [ { "family": "comm_event", "rows_before": 100, "rows_after": 120 } ], "settings_changed": ["hub_threshold"] } },
  "query": { "before_hash": "89984bdc...", "after_hash": "89984bdc...", "changed": false },
  "same_data_different_answer": true }
```
How to read it: `changed` is "show the badge". Rows are compared as sets; a table whose rows only came in another order is `reordered`, not changed; a removed and an added row with the same first cell are one `changed` row, cell by cell. `data_version.detail` exists only when the data moved and both versions are stored; it names the files and families that differ. **`same_data_different_answer: true` means the data did not move but the answer did**: say so plainly; `query.changed` then tells whether the model wrote another query. `complete: false` means a result had more than 200 rows: the digest still says whether anything differs, the lists are the first rows. `data_version.known: false` means one of the runs has no stored data version.

### `GET /turns/{id}/diff[?against={turn_id}]`
The same document between any two finished turns of one case, computed from stored data (no engine call). Default `against`: the turn it re-ran, else the run before it in the chain; `400` if there is none. `of` is the turn in the path.

### `GET /turns/{id}/ancestry` → `{ "items": [turn, ...] }`
The chain above a turn, oldest first, across the conversation it was forked from (turns you may not see are left out).

### `POST /turns/{id}/fork` → `201` thread
Body `{ "title": "..." }` optional. A new conversation owned by you whose `forked_from` is `{ "thread_id", "turn_id" }`; nothing is copied. Post the first question to it **without** `parent_turn_id`: it continues from the forked turn and the engine receives the chain as context.

### `POST /turns/{id}/feedback` → `{ "mine", "counts" }`; `GET /turns/{id}/feedback` → `{ "items": [...] }`
Body `{ "verdict": "correct|wrong|incomplete|unclear", "note": "..." }`; one verdict per person, changeable. "This answer is wrong" is `verdict: "wrong"`. The list of everyone's feedback is for the owner of the conversation and those who run the case (`403` otherwise); everyone sees `counts` on the turn.

### `POST /turns/{id}/pin` → `201` `{ "finding": {...}, "already_pinned": false }`
Body `{ "title", "note", "basis": "observed|inferred" }` all optional (title defaults to the headline). Only an **answered** turn can be pinned (`409 not_an_answer`). Pinning again returns the existing finding, `200`, `already_pinned: true`. The finding keeps `pinned_json`, a digest of the answer as it was (sentence, first 50 rows, query hash, engine, data version), and `evidence_refs`, so it stays whole if the conversation is hidden.

## 3. Findings, notes, hypotheses, tasks, annotations, saved views, tags

One shape for all. For each **plural** `findings`, `notes`, `hypotheses`, `tasks`, `annotations`, `views`:

| Call | |
|---|---|
| `GET /cases/{case_id}/{plural}` | list; `q` (words), `tag`, `limit`, `cursor`, `include_deleted=true`, plus the filters below |
| `POST /cases/{case_id}/{plural}` → `201` | create from a JSON body |
| `GET /{plural}/{id}` | one |
| `PATCH /{plural}/{id}` | change the named fields (a field that cannot change after creation is refused) |
| `DELETE /{plural}/{id}[?reason=...]` → `204` | hide, with a reason (not `authority`) |

Every object comes back as its row with `case_id` (no tenant), `created_by`, `created_at`, `updated_by`, `updated_at` and `tags` (names). Example note:

```json
{ "note_id": "b22767cd-33fc-4dc7-85e0-7a49f2d5214f", "case_id": "orion-lahore-reference-20261030", "category": "lead", "title": "Handset",
  "body": "Check the second handset of 923001234567", "pinned": false, "source_turn_id": null, "tags": ["bank", "Urgent"],
  "created_by": "alice", "created_at": "2026-11-03T16:37:06.083741+00:00", "updated_by": null, "updated_at": "2026-11-03T16:37:06.083741+00:00" }
```

| Plural | Create (`*` required) / change | Filters |
|---|---|---|
| `notes` | `body*` (1 to 20,000), `category` (`note` default, `lead`, `hypothesis`, `finding`, `question`), `title`, `pinned`; `source_turn_id` on create | `category`, `pinned`, `source_turn_id` |
| `findings` | `title*` (200), `body` (20,000), `basis` (`observed` default, `inferred`), `sort_order`, `evidence_refs` (list), `source_turn_id` on create. **Review is not a patch**: `POST /findings/{id}/review` `{ "state": "confirmed|rejected|unreviewed", "note" }`: in a controlled case the reviewer must not be the author (`403`); `unreviewed` withdraws a verdict (owner or lead). Fields `review_state`, `reviewed_by`, `reviewed_at`, `review_note`, `pinned_json`, `data_version_hash` are read-only | `review_state`, `basis`, `source_turn_id`; ordered by `sort_order` |
| `hypotheses` | `statement*` (2,000), `rationale`, `status` (`open`, `supported`, `refuted`, `parked`), `confidence` (`low`, `medium`, `high`, null), `supports`, `contradicts` (lists of finding ids) | `status`, `confidence` |
| `tasks` | `title*`, `detail`, `status` (`open`, `in_progress`, `done`, `cancelled`; `done` sets `completed_at` and `completed_by`, any other clears them), `assignee_subject`, `due_at`; `source_kind` (`turn`, `finding`, `hypothesis`, `note`, `evidence`) and `source_id` on create | `status`, `assignee` |
| `annotations` | `target_kind*` and `target_key*` on create (the timeline's pin is `timeline_day` + `2026-02-03`), `body*` (4,000), `locator` (object) | `target_kind`, `target_key` |
| `views` | `name*`, `kind*` on create (`connections`, `places`, `timeline`..., letters, digits, `_`), `params_json` (object, 256 KB), `visibility` (`private` default, `case`). Private views are seen only by their owner; only the owner changes or hides one | `kind`, `visibility` |

**Tags.** `PUT /cases/{case_id}/objects/{kind}/{id}/tags` with `{ "tags": ["Urgent", "bank"] }` makes the tags of the object exactly these (creating tags as needed; two spellings, `Urgent` and ` URGENT `, are one tag); `GET` the same path reads them. `kind` is one of `thread`, `turn`, `note`, `finding`, `hypothesis`, `task`, `view`, `evidence` (at most 20 per object). `GET /cases/{case_id}/tags` → `{ "items": [ { "tag_id", "name", "color", "uses" } ] }`; `PATCH /tags/{id}` `{ "name", "color": "#c0392b"|null }`.

**Replacing the browser's own storage.** `nexusai.viewer.questions.*` → conversations (the old local history is not imported: it would mean asking the questions again); `nexusai.casenotes.*` (`text`, `category` lead/hypothesis/finding) → `notes` (`body`, same `category`); `nexusai.viewer.timeline.pins.*` (`date`, `note`) → `annotations` with `target_kind: "timeline_day"`, `target_key: date`, `body: note`.

## 4. Members, authority and the audit trail

### `GET /cases/{case_id}/members`
```json
{ "case_id": "orion-lahore-reference-20261030", "mode": "controlled",
  "members": [ { "subject_id": "alice", "role": "owner", "display_name": "Alice Khan", "added_by": "alice", "added_at": "2026-11-03T16:37:06.367828Z", "updated_at": "2026-11-03T16:37:06.367828Z" } ],
  "you": { "subject_id": "alice", "role": "owner", "admin": false } }
```
`mode` is `open` (no members; `you.role` is `open`) or `controlled`. Every participant reads it; a non-member of a controlled case gets `404`.

### `PUT /cases/{case_id}/members/{subject}` `{ "role", "display_name" }` → member; `DELETE` → `204`
While the case is **open**, the first call must make **yourself** the `owner` (an administrator may name the owner); that turns the case controlled. After that owners, leads and administrators manage the list; only an owner or an administrator grants, changes or removes an owner; the last owner cannot be removed or demoted (`409 last_owner`). Roles: `owner`, `lead`, `analyst`, `reviewer`, `viewer`, `auditor`.

### Authority: `GET` and `POST /cases/{case_id}/authority`, `GET` and `PATCH /authority/{id}`, `POST /authority/{id}/revoke` `{ "reason" }`
`kind` (`warrant`, `court_order`, `consent`, `internal_approval`, `other`), `reference*`, `issuing_body*`, `scope`, `valid_from`, `valid_to` (dates), `notes`, `attachments` (`[{ "evidence_id" }]`). Computed `status`: `active`, `expired`, `not_yet_valid`, `revoked`. Owners, leads and an open case's users manage it; everyone reads. It **informs; it does not yet gate** any capability. A record is revoked, never deleted.

### Audit: `GET /cases/{case_id}/audit`, `GET /cases/{case_id}/audit/verify`
Owners, leads, auditors, administrators (anyone while the case is open). Filters `action` (`turn.completed`, or a prefix ending in a dot: `member.`), `object_kind`, `object_id`, `limit`, `cursor`; newest first. Each row has `seq`, `actor`, `actor_role`, `action`, `object_kind`, `object_id`, `details` (what changed: field names, verdicts, reasons; **never the text** of a note or a question), `occurred_at`, `event_sha256` and `previous_sha256`. `verify` recomputes the chain: `{ "case_id", "events": 16, "ok": true, "first_broken_seq": null }`.
Actions written: `thread.created|updated|shared|hidden|restored|forked`, `turn.started|completed|failed|feedback`, `finding.pinned|reviewed`, `note|hypothesis|task|view|annotation|finding .created|updated|deleted`, `authority.created|updated|revoked`, `member.added|role_changed|removed`, `tag.updated`, `tags.changed`.

## 5. What the Ask page needs, call by call

| The page | Calls |
|---|---|
| History rail: grouped by day, pinned first, search, badges | `GET /conversations?case_id=&q=&limit=&cursor=`; badge from `changed_since`; counts from `turn_count`; why a search matched from `matches` |
| First question | `POST /conversations` `{ case_id }` then `POST /conversations/{id}/turns` as a stream (two calls; the title is automatic from the first question until the person renames it) |
| Narrated wait | NDJSON events: `accepted` → (`queued`) → `engine` (+ `engine.*` when the engine reports) → `saving` → `turn`; a clock from `heartbeat.elapsed_ms` |
| Open a conversation | `GET /conversations/{id}` (turns, latest answer), then `GET /turns/{id}` for any other answer when it is scrolled to |
| Continue / edit and resubmit / fork | `POST .../turns` / `POST .../turns {edit_of_turn_id}` / `POST /turns/{id}/fork` then `POST .../turns` |
| Re-run with "changed since" | `POST /turns/{id}/rerun` → `diff`; the turn's own badge from `reruns.latest_changed`; later `GET /turns/{id}/diff` |
| Compare two turns | `GET /turns/{id}/diff?against=` |
| Pin to findings | `POST /turns/{id}/pin`; `pinned_as` on the turn tells the button it is done |
| Share | `PATCH /conversations/{id}` `{ visibility, shared_with }`; members from `GET /cases/{id}/members` |
| "This answer is wrong" | `POST /turns/{id}/feedback` |
| Rename, pin, archive, delete, restore | `PATCH`, `DELETE`, `POST .../restore` |
| Reproduce / exhibit | the turn's `engine_version`, `model`, `prompt_id`, `data_version_hash`, `query`, `query_hash` |
| **Export to the report builder** | **not built** (BR-4); until then the turn read gives the answer and its digest |

## 6. Limits

Question 4,000 characters; title 200; note, finding and hypothesis text 20,000; page size 30 (max 100); at most 100 names in `shared_with`; 20 tags per object. A turn runs at most **10 minutes** (queue and engine together); two turns are in the engine at once. The answer is stored whole up to 8 MiB (1 MiB compressed); over that it is stored **trimmed** (result rows cut to 200, evidence rows to 100, and `snapshot_trimmed` says so: `snapshot_state: "trimmed"`) or not at all (`"none"`: the turn can only be run again). The comparison keeps the first 200 rows of a result and a digest of all of them.

## 7. Not built

Cross-case conversations (the columns are reserved); export of a thread to the report builder (BR-4); the follow-up resolver is built, **off** and unmeasured (`FORENSIC_CONVERSATION_FOLLOWUP`); the engine's own phases (`engine.<name>`) await a call from the engine session; per-person identity (request 25) so that conversations have real owners; fuzzy Roman-Urdu spelling in search; gating a capability by an authority record.
