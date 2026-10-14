# The typed family switch: design, first slice, and what turning it on involves

Status: **first slice built and tested on 2026-10-14. Not applied to the live database, not deployed, switch off.** Roadmap: section 4.6, steps 3 to 5 ("backfill, switch one family's views, retire the JSON view after two weeks of parity").

## 1. What it is, in plain words

Today the question lane reads every family from `forensic.records`, where each source row is a JSON document and a field is found by looking up the provider's own column names. Ingest v2 (`docs/data/ingest-v2.md`) now also writes the same rows to typed tables (`comm_event` and the others), with normalised numbers, UTC instants and the layout that read them. The switch makes the lane read a family from those typed rows instead, **one family at a time, behind a variable that is empty by default**, and only for evidence items that have typed rows; every other item keeps being served from version 1. Nothing is deleted: version 1 stays as the fallback and as the rollback.

The first family is **call records** (`communications_cdr`, typed table `comm_event`, `dataset_kind = target_cdr`). Reasons: it is the family with the most questions, it is where version 1's times are wrong for eight workbook layouts (`ingest-v2.md` section 10), and its typed rows are already written and verified.

## 2. Facts that shaped the design (measured in the code and on the live system, 2026-10-14)

1. **The lane's "views" are not database objects.** They are CTEs that Go code (`governed_sql_catalog.go`) prepends to the model's query, built from the curated semantic layer (`semantic_layer/*.yaml`) with the same expression builders the typed compiler uses (`source_native_sql_executor.go`): `FROM <table> r WHERE r.tenant_id=$1 AND ...`, with each field pulled out of `r.raw_payload` by the provider column names the layer lists as `source_names`. A family's table is chosen in a handful of places (the layer's `semanticSourceTables`), and the curated catalogue of fields does not depend on sampled rows.
2. **The typed compiler refuses a plan whose fields come from more than one table** (`sourceNativeBindingForFieldIDs`). Giving it a second table for one family would refuse plans that join that family to another. The lane has no such limit (one CTE per view), so **the first slice switches the lane only**; the compiler and the older query path (`query.go`, which reads `forensic.records` directly in dozens of places) still answer from version 1 when the lane declines.
3. **What the typed path can cover today is narrow.** A read-only audit of the 46 completed structured jobs in the live database (stored files read in the worker container, header matched against the registry, counts only): the new synthetic collection is covered completely (12 of 12 files); of the four older collections (34 files, 31,778 rows) the registry recognises 6 files (18,941 rows, 59.6 %), **all of them as `geofence-c6-telenor`, a draft tower-dump layout, not as call records**; the other 28 files (CDR TSV and acceptance files, IPDR, access logs, ANPR, transactions, subscriber and tower-reference files) match no registered layout, and four of those families have no typed writer at all. So for the existing demo cases the switch changes nothing today, which is the safe outcome; its effect shows on evidence ingested in a recognised layout.
4. **Version 1 and the typed path disagree for stated reasons** (`ingest-v2.md` section 10): version 1 stores the ingest instant, or a wrong year, as the event time for eight workbook layouts and keeps rows without a time; the typed path stores the real UTC instant and quarantines such rows; version 1 drops repeated rows, the typed path keeps and flags them.

## 3. Design

**One database view per family**, `forensic.records_effective_cdr` (migration `016_effective_records.sql`), with **exactly the columns and types of `forensic.records`**, so no builder of the lane's SQL changes: the family's table name is the only difference. Per evidence item it returns the typed rows if the item has any, otherwise its version-1 rows, so no backfill is needed to switch, no item is counted twice, and writing typed rows for an old item changes its answers only from that moment.

- **Presentation of a typed row.** The normalised numbers and the UTC event times are written into the payload under the semantic layer's own names (`MSISDN`, `CALL_DIALED_NUM`, `CALL_START_DT_TM`, `CALL_END_DT_TM`, `IMSI`, `IMEI`); the provider's own columns of those meanings, under every alias the layer lists, are first removed, so that **a counterparty the typed ingest could not resolve stays empty instead of showing the provider's raw dialled number** (found by the parity test, section 5). Call type, direction and coordinates come from the typed row only where the provider's column is absent. `timestamp` is the UTC event time; `record_id`, `source_file`, `row_number` and the lineage columns are filled so that citations work.
- **Rules that differ from version 1, on purpose:** repeats flagged `duplicate_row` are not returned; a row without a usable time was quarantined at ingest and is not here; `row_number` is the source row + 1 (version 1 counts the header as row 1; a file with lines above its header numbers differently).
- **Security.** `security_invoker`: the view runs with the caller's privileges and row-level security, so tenant isolation is exactly that of the tables beneath it. The application role gets `SELECT` only.
- **Go (`api/forensic_records/typed_families.go`).** `FORENSIC_TYPED_FAMILIES` (comma-separated family names, default empty). For a listed family the lane's view reads the effective view instead of `forensic.records`. Safety: the view's existence is checked (re-checked every 30 s), so setting the variable before the migration is applied leaves the lane on `forensic.records` with one warning in the log instead of failing every call-record question; derived (model-observation) views are never switched; the answer's provenance carries the `table` it read.

## 4. What was built

| File | What |
|---|---|
| `db/forensic_records/016_effective_records.sql` | the view (DRAFT, not applied) |
| `db/forensic_records/ops/016_effective_records/` | `preflight.sql`, `verify.sql` (columns equal `forensic.records`, `security_invoker`, grants), `rollback.sql` (drops the view, nothing else), `smoke.sql` (disposable databases only) |
| `api/forensic_records/typed_families.go` and `governed_sql_lane.go` (one line) | the lane-only switch |
| `tools/case_generator/test_effective_cdr.py` | 8 tests on a throwaway PostgreSQL with a generated case written by the production typed ingest |
| `api/forensic_records/typed_families_ginkgo_test.go` | 11 specs (no database) |
| `api/forensic_records/typed_families_db_ginkgo_test.go`, `tools/case_generator/check_lane_on_typed_view.py` | 5 specs that run the lane's own Go-generated CTE against the view on a throwaway database, **connected as the application role**, and compare what it reads with the typed table |

## 5. Evidence

- **The ops scripts** (preflight, verify, smoke) pass on a database built like a fresh one; the migration applies twice and rolls back and forward; verify fails when the view lacks `security_invoker` or a column.
- **Parity with the key.** For every subject of the generated case, the contact profile read through the view the way the lane reads it (counterparties, event counts, first and last time) equals the generator's answer (A1). **This test found a defect in the first version of the view**: for a call whose direction is unknown, the typed ingest leaves the counterparty empty (it is flagged), but the provider's own dialled-number column was still in the payload and the lane would have read it as a resolved counterparty (one extra row per affected counterparty). The view now removes the provider's columns of every field a typed row owns; a test compares the removal list with `semantic_layer/cdr.yaml`.
- **The lane's own code.** 5 specs: the row count equals the non-repeated target call records of the typed table; a subject's counterparties, counts and first and last instants equal the typed table's, through the builders' guarded casts; the event-time column and the derived duration metric (`timestamp_diff_seconds`) equal the typed table's; provenance columns are present and unique; with the switch off the same lane reads `forensic.records` and sees none of the typed rows.
- **Isolation.** The application role sees its tenant's rows and nobody else's, without the setting nothing.
- **Deliberate faults:** 10 in the migration (payload keys not removed, repeats returned, version-1 rows counted beside typed ones, owner's privileges, tower dumps returned, row number, time of ingest, derived call type overriding the provider's, other record types falling through, raw values unused), 7 in the Go switch (variable ignored, missing view not noticed, database asked every time, derived views switched, unlisted family switched, switch never applied, wrong view name): all caught. Two of the Go faults were first missed (an unlisted family; the derived guard) and two specs were added.
- **Not run:** the migration on the live database; the lane with the variable on in the running stack; the question-factory sets and the corpus (the gates below).

## 6. What turning it on involves (each step needs the owner's go-ahead)

1. **Apply migration 016** to the live database: `ops/016_effective_records/preflight.sql`, the migration, `verify.sql` (additive, one view, no data touched; same procedure as section 11 of the D2 runbook; the rollback drops the view).
2. **Set `FORENSIC_TYPED_FAMILIES=communications_cdr` on the API** and recreate only the API container **through `evaluation/question_factory/Run-Arms.ps1`'s helpers** (never a bare `compose up`; `ingest-v2.md` section 12). With nothing but version-1 evidence in the case this changes no answer; with typed evidence (the synthetic collection, new uploads) it does.
3. **Measure before keeping it.** The lane's gates must hold or the family stays on the old view (roadmap 4.6 step 4): re-run the family's question-factory set and the corpus on a case that has typed call records (the synthetic collection can be extended to the factory's question shapes), compare arms with `Compare-Arms`, and record the result in a pre-registration like runs 10 to 17. Expected differences are the ones listed in section 3; anything else is a defect.
4. **Two weeks of parity** before retiring the JSON view of the family (step 5), as the roadmap says.
5. **Rollback** at any point: unset the variable and recreate the API container (the lane returns to `forensic.records`); the view can then be dropped with `rollback.sql`.

## 7. What is not done, and open decisions

- **Other families** (IPDR, access log, ANPR, transactions, subscribers, towers) are not switchable: the registry has layouts only for the telecom provider shapes, and only `comm_event`, `data_session` and `app_usage` have writers. IPDR (`data_session`) is the natural second family once its view is written; the canonical-header files of the demo cases need layouts first.
- **The typed compiler and the older query path** still read version 1 (section 2, point 2). Switching them needs a decision about plans that join families.
- **The three demo CDR files match a draft tower-dump layout** (`geofence-c6-telenor`, score 0.94 on 15 of 16 columns). If they were ever backfilled they would become `tower_dump` rows and stay out of the call-record view, which is right only if they really are tower dumps. Whether they are is a question for whoever confirms layouts on real files (the registry's drafts are unconfirmed).
- **Line numbers.** A file with lines above its header numbers its rows differently in version 1 and in the typed path; recording the header row in the typed row would remove the difference.
- **Draft layouts.** All 13 are drafts; `FORENSIC_INGEST_V2_INCLUDE_DRAFT` is true. Before the typed rows decide answers on real data, each layout should be confirmed on a real file in the vault and promoted.
