# Ingest v2: the typed shadow write

Status: **built and tested on 2026-10-13. Not deployed.** The switch is off by default, the worker image has not been rebuilt, and nothing in the live stack has changed. Roadmap: Phase 1, section 4.4.

## 1. What it does, in plain words

Today the worker stores every structured file in the version-1 tables (`records`, `cdr_records`, `generic_records`). Ingest v2 adds a second, separate write: when a file's header is one of the 13 provider layouts in the registry, the same rows are also written to the typed tables created by migrations 012 to 015 (`comm_event` for call records and tower dumps, `data_session` for internet records, `app_usage` for application usage), with their lineage and with the time converted to UTC under the job's declared time zone.

It is a **shadow**: version 1 is not changed, nothing reads the typed tables yet, and a failure in the new code can neither fail a job nor change what version 1 stored. The point is to fill the typed tables from real uploads, check them against version 1, and only then move the questions over, family by family (roadmap section 4.8).

## 2. Settings (environment of the worker)

| Variable | Default | Meaning |
|---|---|---|
| `FORENSIC_INGEST_V2` | `off` | `off` or `shadow`. Anything else (a typo) is `off`, with a warning in the log. |
| `FORENSIC_INGEST_V2_MAX_ROWS` | `2000000` | A file with more rows is skipped for the typed write (status `skipped`, reason `too_many_rows`); version 1 is unaffected. |
| `FORENSIC_INGEST_V2_INCLUDE_DRAFT` | `true` | While every layout is `draft` (they are, until each is confirmed on a real file), `false` would recognise nothing. Set `false` before the typed rows are used for answers. |

The compose files do not set any of these, so a rebuilt image behaves exactly as before until the owner sets `FORENSIC_INGEST_V2=shadow`.

## 3. Where it sits

`worker.process_job` calls `run_typed_ingest_shadow(conn, cur, job, path, profile)` after version 1 has finished and audited, and before `conn.commit()`. That function returns at once when the mode is `off` (the module is not even imported). In `shadow` it calls `typed_ingest.run_shadow`, which works inside **a savepoint of the job's own transaction**: an error rolls back only the typed rows, is logged, and comes back as a summary with status `failed`, the error type and, for a database error, the constraint name (never the message, which can quote a row). The audit row is written in a savepoint of its own. Code: `ingestion/forensic_records/typed_ingest.py`, `worker.py` (`run_typed_ingest_shadow`), the shared readers `layout_registry/readers.py`.

## 4. What is written, and the rules it keeps

- **Readers.** The same `readers.py` the vault tools use (xlsx, SpreadsheetML 2003, delimited text; the archive and size limits are the same), so a file is read identically when it is profiled, confirmed and ingested. Other containers are reported as `unsupported_container` and skipped.
- **Layout.** `Registry.find_header` on the first rows of each sheet. `match` is used; `ambiguous` and `none` are reported and write nothing. A layout whose family has no typed table is skipped.
- **Time.** The job's time policy (`source_timezone`, how it is known, `source_date_order`) becomes the registry's policy. **Without a declared or confirmed zone no UTC time exists, none is guessed, and every such row is quarantined with the reason `source_timezone_unknown`.**
- **Rows.** `interpret_rows` decides each row: accepted, quarantined (with a reason) or skipped (blank). Accepted rows get lineage (`tenant_id`, `collection_id`, `evidence_id`, `file_id`, `batch_id` = the job id, `layout_id`, `layout_version`, `source_row`), `content_hash` (sha-256 of the canonical content), `row_hash` (sha-256 of `file_id|source_row|content`), `quality_flags` and the raw row in `raw_payload`. The formulas are the ones of the reference loader `tools/case_generator/load_case.py`, and a test compares the hashes the two produce.
- **Repeats.** A row whose content repeats inside the file is **kept** and flagged `duplicate_row`.
- **Idempotent.** Rows go through a temporary stage table (`COPY`) and `INSERT ... ON CONFLICT (tenant_id, collection_id, file_id, row_hash, start_utc) DO NOTHING`, in batches of 5,000. A second run of the same file inserts nothing and reports every row as `already_present`.
- **No evidence id, no write.** A job without a valid `evidence_id` (the foreign key of the typed tables) or batch id is skipped and reported.

## 5. The quality summary

`typed_ingest.run_shadow` returns, and the worker writes to `records_audit_log` under the action **`records.ingest.typed_shadow`**, a summary with no cell values: the mode, the policy, and per sheet the layout (id, version, status), header row and match score, the table, `rows_read`, `loaded`, `inserted`, `already_present`, `duplicates_in_file`, `quarantined` and `skipped` counts by reason with the first 20 row numbers of each reason, flag counts, the first and last local start time, and `reconciles` (rows read = loaded + quarantined + skipped, and loaded = inserted + already present). A reconciliation that is false is a defect to investigate, never to ignore.

Quarantined rows are **counted, with sample row numbers, not stored row by row** in this phase: version 1's reject table (`records_ingest_errors`) is read by the question API, so it is left alone, and a typed quarantine table belongs to the switch to `on` (a later migration).

## 6. How it was tested (2026-10-13)

| Level | What | Result |
|---|---|---|
| Unit, no database | `ingestion/forensic_records/tests/test_typed_ingest.py`: 38 tests (mode, policy, reading, counts, flags, hashes, idempotence, refusals, savepoint and failure containment, COPY text, SQL builders, the worker hook) with a fake sink that keeps the table's unique key | pass |
| Breakage | 20 deliberate faults (no `ON CONFLICT`, a typo turning the write on, no zone check, no repeat flag, hash without the source row, uncounted rows, no savepoint, uncontained failure, `\N` text read as NULL, NUL kept, wrong time basis, no id validation, header read as data, off-by-one limit, wrong file id, ambiguous accepted, hook letting a failure through, hook not behind the switch, hook never called) | all 20 caught (one was first missed, the off-switch of the hook; the test was tightened) |
| Throwaway PostgreSQL, SQL | `tools/case_generator/test_ingest_v2.py`: a generated case through the production module, then the same SQL checks as the reference loader. 7 tests: all checks pass, the registry picks the layout the key names for every export, **the row hashes equal the reference loader's**, lineage and layout foreign keys, a second run writes nothing, a job with no zone writes no row, deliberate damage is noticed | pass |
| Whole default case | `test_ingest_v2.py --case DIR` (12 subjects, 120 people, 90 days, 36 files): 12,432 rows (6,992 `comm_event`, 4,569 `data_session`, 871 `app_usage`) | **174 of 174 SQL checks pass** |
| Worker image, real driver | `tools/case_generator/check_psycopg_sink.py DIR` runs `psycopg_probe.py` inside the current worker image (psycopg 3.3.2) with the new code mounted flat as the Dockerfile lays out `/app`, on the throwaway database, **as `forensic_runtime`** (not the owner, no RLS bypass, INSERT-only): 12,432 rows written, 174 checks + 6 more pass; a second run writes nothing; the worker's own hook writes its audit row; a foreign-key refusal and a row-level-security refusal are each contained (named, connection still usable, the job's rows intact) | pass (180 checks) |

The earlier suites still pass after the readers moved out of `profile_structure.py`: real-data tools 32, registry 54, licence ledger 13, case generator 106, case set 8, reference loader 12, worker 22, PowerShell vault checks 13.

What was **not** run: a rebuild of the worker image (the Dockerfile has two new `COPY` lines, `typed_ingest.py` and `layout_registry/`, that a build would exercise; the mounted flat layout is the same file set), and any live upload.

## 7. Rollout plan (each step needs the owner's go-ahead)

1. **Rebuild the worker image** with the two new `COPY` lines; tag the current image first as the rollback (`before-ingest-v2`). Behaviour is unchanged while the switch is off.
2. **Set `FORENSIC_INGEST_V2=shadow`** for the worker (compose runtime override, like the lane settings). Recreate the worker only.
3. **Upload one synthetic case file per layout** through the normal path; read the audit summaries (`records.ingest.typed_shadow`); compare the typed counts with version 1 for the same file. Expect equal accepted counts, and differences only where version 1 rejects what the registry accepts or the reverse; each difference is explained or fixed.
4. **Watch** the worker log and the audit summaries for a period the owner sets; `failed` or a false `reconciles` is a stop.
5. **Not before:** each layout confirmed on a real file inside the vault (`tools/real_data/confirm_layout.py`) and promoted from `draft`; then `INCLUDE_DRAFT=false`, a typed quarantine table, and the family-by-family switch of the questions.

**Rollback.** Set `FORENSIC_INGEST_V2=off` (or remove it) and recreate the worker: the next job writes nothing new. Rows already written stay in the typed tables; they can be removed by `batch_id` or `evidence_id` as the database owner (the application role has no DELETE by design). Image rollback: the tag from step 1.

## 8. Known limits

- A database error inside a batch drops the typed write for that **whole file** (version 1 is untouched); there is no per-row retry yet. The summary names the constraint.
- Every sheet of a workbook is tried and reported on its own; a sheet that matches no layout is reported and ignored, not an error.
- Memory: the repeat check keeps 16 bytes per row of the file in memory; the 2,000,000-row default keeps that near 100 MB. Throughput on very large files is not measured.
- Draft layouts are accepted by default (see section 2). The typed tables are read by nothing yet, so this is safe in the shadow phase and must change before they are used.
- No enrichment (cell reference join, handset from TAC, number classes beyond the registry's), no entity or relationship updates, no versioning of re-uploads (roadmap 4.4 steps 8 to 10).
- Version 1's own counters, quality summary and the question API are untouched by design.
