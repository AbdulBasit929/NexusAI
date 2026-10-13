# Ingest v2: the typed shadow write

Status: **built and tested on 2026-10-13, and running in SHADOW mode on the live worker since 2026-10-13 14:25 UTC** (owner: "yes rebuild the worker image and set shadow"; section 9). The code's own default is still off; the shadow is set by a compose override file. Nothing reads the typed tables yet. Roadmap: Phase 1, section 4.4.

## 1. What it does, in plain words

Today the worker stores every structured file in the version-1 tables (`records`, `cdr_records`, `generic_records`). Ingest v2 adds a second, separate write: when a file's header is one of the 13 provider layouts in the registry, the same rows are also written to the typed tables created by migrations 012 to 015 (`comm_event` for call records and tower dumps, `data_session` for internet records, `app_usage` for application usage), with their lineage and with the time converted to UTC under the job's declared time zone.

It is a **shadow**: version 1 is not changed, nothing reads the typed tables yet, and a failure in the new code can neither fail a job nor change what version 1 stored. The point is to fill the typed tables from real uploads, check them against version 1, and only then move the questions over, family by family (roadmap section 4.8).

## 2. Settings (environment of the worker)

| Variable | Default | Meaning |
|---|---|---|
| `FORENSIC_INGEST_V2` | `off` | `off` or `shadow`. Anything else (a typo) is `off`, with a warning in the log. |
| `FORENSIC_INGEST_V2_MAX_ROWS` | `2000000` | A file with more rows is skipped for the typed write (status `skipped`, reason `too_many_rows`); version 1 is unaffected. |
| `FORENSIC_INGEST_V2_INCLUDE_DRAFT` | `true` | While every layout is `draft` (they are, until each is confirmed on a real file), `false` would recognise nothing. Set `false` before the typed rows are used for answers. |

`docker-compose.forensic-records.ingest-v2.yaml` sets `FORENSIC_INGEST_V2` to `shadow` for the worker; the base compose file does not set any of these, so a worker recreated without the override file is back to off (the safe direction).

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

What was **not** run: a build of `ingestion/forensic_records/Dockerfile` itself (the repository's Dockerfile has two new `COPY` lines, `typed_ingest.py` and `layout_registry/`; the deployed image was built as an overlay, section 9), and any upload through the normal path (section 7, step 3).

## 7. Rollout plan (each step needs the owner's go-ahead)

1. **Rebuild the worker image** (done 2026-10-13 as an overlay on the running image, section 9); the current image was first tagged `before-ingest-v2` as the rollback.
2. **Set `FORENSIC_INGEST_V2=shadow`** for the worker (done 2026-10-13 with a compose override; only the worker was recreated).
3. **Upload one synthetic case file per layout** through the normal path; read the audit summaries (`records.ingest.typed_shadow`); compare the typed counts with version 1 for the same file. Expect equal accepted counts, and differences only where version 1 rejects what the registry accepts or the reverse; each difference is explained or fixed.
4. **Watch** the worker log and the audit summaries for a period the owner sets; `failed` or a false `reconciles` is a stop.
5. **Not before:** each layout confirmed on a real file inside the vault (`tools/real_data/confirm_layout.py`) and promoted from `draft`; then `INCLUDE_DRAFT=false`, a typed quarantine table, and the family-by-family switch of the questions.

**Rollback.** Set `FORENSIC_INGEST_V2=off` (or recreate the worker without the override file) and the next job writes nothing new. Rows already written stay in the typed tables; they can be removed by `batch_id` or `evidence_id` as the database owner (the application role has no DELETE by design). Image rollback: section 9.

## 8. Known limits

- A database error inside a batch drops the typed write for that **whole file** (version 1 is untouched); there is no per-row retry yet. The summary names the constraint.
- Every sheet of a workbook is tried and reported on its own; a sheet that matches no layout is reported and ignored, not an error.
- Memory: the repeat check keeps 16 bytes per row of the file in memory; the 2,000,000-row default keeps that near 100 MB. Throughput on very large files is not measured.
- Draft layouts are accepted by default (see section 2). The typed tables are read by nothing yet, so this is safe in the shadow phase and must change before they are used.
- No enrichment (cell reference join, handset from TAC, number classes beyond the registry's), no entity or relationship updates, no versioning of re-uploads (roadmap 4.4 steps 8 to 10).
- Version 1's own counters, quality summary and the question API are untouched by design.

## 9. Deployed on 2026-10-13: what exactly is running

**Authority.** The owner said "yes rebuild the worker image and set shadow" in the conversation of 2026-10-13, after the rollout plan above.

**Why an overlay and not a full rebuild.** The worker image that was running (`376bbf162777`, built 2026-08-23) is seven weeks behind the repository. Its `worker.py`, `media_pipeline.py` and `roman_urdu.py` differ from the repository's; `multilingual_ocr.py`, `processor_readiness.py` and the `video_anpr_*.py` modules, which the repository's Dockerfile copies, are not in it; and `requirements.txt` has since grown packages the image does not have (the licence ledger notes torch, safetensors, paddleocr and others). A build of the repository's Dockerfile would have shipped all of that, and installed the new packages from the network, together with Ingest v2, which was not what was asked. The image that runs is therefore **the running image plus only Ingest v2**: its own `worker.py` with exactly the two Ingest v2 hunks (35 added lines, none removed), `typed_ingest.py` and the `layout_registry/` package. Recipe: `tools/worker_overlay/make_overlay.py` (reproduces the deployed files byte for byte). **A full rebuild from the repository is a separate decision** (it also deploys the seven weeks of worker and media changes and the new dependencies).

| | Image | Tags |
|---|---|---|
| Running now | `5682f75a18a3` | `nexusai-forensic-records-worker:latest`, `:ingest-v2-shadow-20261013` |
| Rollback | `376bbf162777` | `nexusai-forensic-records-worker:before-ingest-v2` (also `nexusai/forensic-records-worker:rollback-before-unified-anpr-cast-20260823`) |

**How it was started.** Only the worker container was recreated (`--no-deps --no-build --force-recreate`), with the main checkout as project directory and its `.env.forensic-runtime.local`, exactly as the worker had been created, plus `docker-compose.forensic-records.ingest-v2.yaml`, which sets `FORENSIC_INGEST_V2=shadow`. Checked before: the base compose file hashed to the same value the running worker was created from (no drift), and the rendered configuration with the override differed by that one line. The postgres, NATS and API containers were not touched. No job was running (87 completed, 5 dead-lettered, none in flight). After: healthy, subscribed to the durable JetStream consumer, no restarts, no error lines, metrics endpoint 200.

Start command (PowerShell, from anywhere):

```powershell
$main = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'; $lw = 'C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork'
docker compose --project-name nexusai --project-directory $main --env-file "$main\.env.forensic-runtime.local" `
  -f "$main\docker-compose.forensic-records.yaml" -f "$lw\docker-compose.forensic-records.ingest-v2.yaml" `
  up -d --no-deps --no-build --force-recreate forensic-records-worker
```

**Proof on the live database (rolled back).** Inside the running worker container, the worker's own hook (`run_typed_ingest_shadow`) was run on three synthetic files (a call-record workbook, an internet-record text file, an application-usage workbook from a generated case) in one transaction on the LIVE database, then rolled back. The layouts were found correctly (`call-c1-company`, `ipdr-i4-zong-txt`, `appusage-a1`); 700, 363 and 505 rows were written, every sheet reconciled, the three audit rows (`records.ingest.typed_shadow`) were written; a second run of the same files inserted nothing; the call rows carried `Asia/Karachi`, `confirmed_by_analyst` and UTC times. After the rollback the typed tables, `evidence_items` (79) and the audit log had exactly their earlier contents; the probe files were removed from the container. Real uploads will show the same audit action in `records_audit_log`.

**Rollback.**

```powershell
docker tag nexusai-forensic-records-worker:before-ingest-v2 nexusai-forensic-records-worker:latest
# then recreate the worker WITHOUT the override file (same command as above, without the second -f)
```

**What happens now.** Every structured file the worker ingests is also offered to the typed writer. Files in an unrecognised layout, media and documents are not affected. Watch `docker logs nexusai-forensic-records-worker-1` and the audit rows (`select created_at, file_id, details->'typed_ingest'->>'status', details->'typed_ingest'->>'rows_loaded', details->'typed_ingest'->>'reconciles' ...`). A `failed` status or a false `reconciles` is a stop (section 7, step 4). **Next:** step 3 of the plan, one synthetic file per layout through the normal upload path, compared with version 1 (done on 2026-10-13: section 10).

## 10. Step 3 on 2026-10-13: one synthetic file per layout through the normal upload

**Authority.** The owner said "go, upload one synthetic file per layout" after section 9.

**What was done.** 13 files of the generated case (`seed 7`: one per layout, all with identifiers from the reserved synthetic ranges) were posted to `POST /webhooks/records/upload` of the live forensic API like an analyst's upload: the runtime tenant and API key from the local runtime environment file (as the repository's own acceptance scripts do), `source_timezone=Asia/Karachi` declared as `analyst_confirmed`, record type left to auto-detection, into a **new collection of their own, `ingest-v2-shadow-synthetic-20261013`** (case `ingest-v2-shadow-synthetic-20261013`), so no existing collection was touched. 12 became ingest jobs, all `completed`; one was only registered (below). The live tables moved by exactly what the jobs report: `evidence_items` 79 to 92, `records_ingest_jobs` 92 to 104, `records` and `generic_records` +5,090 each, `cdr_records` unchanged (13,654), `comm_event` 0 to 3,580, `data_session` 0 to 1,019, `app_usage` 0 to 505, 11 audit rows `records.ingest.typed_shadow`. The synthetic collection stays in the live database (clearly labelled); it is separate from the existing cases.

| Layout | File | Version 1 (job) | Typed shadow (audit summary) |
|---|---|---|---|
| C1 `call-c1-company` | xlsx | 706 of 706 stored, **event time = ingestion time** | 709 read, **700 loaded**, 6 quarantined (3 time missing, 3 time unparsed), 3 blank |
| C2 `call-c2-strt-tm` | xlsx | 356 of 356, ingestion time | 356 read, **344 loaded**, 9 quarantined (3 missing, 3 unparsed, 3 date-order conflict), 3 blank |
| C3 `call-c3-ufone-style` | xlsx | 766 of 766, ingestion time | 766 read, **757 loaded**, 6 quarantined, 3 blank |
| C4 `call-c4-telenor-style` | xlsx | 1,304 of 1,304, ingestion time | 1,307 read, **1,298 loaded**, 6 quarantined, 3 blank |
| C5 `geofence-c5-mobilink` | xlsx | 141 of 141, ingestion time | 144 read, **135 loaded**, 6 quarantined, 3 blank |
| C6 `geofence-c6-telenor` | xlsx | 156 of 156, ingestion time | 159 read, **150 loaded**, 6 quarantined, 3 blank |
| C7 `geofence-c7-ufone` | xlsx | **0 of 73 stored, all 73 rejected** | 76 read, **67 loaded**, 6 quarantined, 3 blank |
| C8 `geofence-c8-zong` | xlsx | 135 of 135, **event time in the year 2065** | 138 read, **129 loaded**, 6 quarantined, 3 blank |
| I1 `ipdr-i1-jazz-ufdr` | csv | 501 stored of 509 (5 repeats dropped, 3 rejected), times right | 509 read, **500 loaded** (3 repeats kept and flagged), 6 quarantined, 3 blank |
| I2 `ipdr-i2-telenor-export` | csv | 520 of 528 (5 repeats, 3 rejected), times right | 528 read, **519 loaded**, 6 quarantined, 3 blank |
| I3 `ipdr-i3-ufone-xls` | .xls (SpreadsheetML 2003) | **no job: registered only** (`tabular_adapter_pending`) | not reached |
| I4 `ipdr-i4-zong-txt` | .txt (UTF-16) | job ran as a **document** (69 passages, route `native_document_worker`) | **not reached** |
| A1 `appusage-a1` | xlsx | 505 of 505, times right | 505 read, **505 loaded** |

**What was checked.** (1) The registry chose the right layout for each of the 11 files that reached the writer, and every sheet reconciled (rows read = loaded + quarantined + skipped; loaded = inserted + already present). (2) **The deployed worker wrote exactly the rows the tested code produces:** for all 11 files the content hashes in the live typed tables equal those of an offline run of `typed_ingest.py` on the same files (`ALL IDENTICAL`). (3) Where version 1 parses the time correctly (I1, I2, A1) the typed rows have **the same UTC instant on every aligned row** (497 of 497, 516 of 516, 505 of 505; version 1 numbers rows from the header, so version 1 row n is typed `source_row` n minus 1). (4) The injected defects of the generated case behave as designed: 3 rows with no time and 3 with an unparsable time quarantined per file, 3 date-order conflicts in C2, blank rows skipped, repeats flagged.

**Findings (none was fixed in this step; each needs a decision).**

1. **I4: a UTF-16 `.txt` export never reaches the typed writer.** The upload route sends text files to the document pipeline (`native_document_worker`), which returns before the hook. The Zong IPDR layout therefore cannot be ingested as records by either path today. Fix options: offer text files whose header the registry recognises to the typed writer from the document branch, or classify them as tabular at intake.
2. **I3: a `.xls` export is registered but never queued** (the intake marks it `tabular_adapter_pending`). The file is SpreadsheetML 2003, which the shared readers parse. Fix: intake must accept `.xls` as a tabular source and queue a job. This is a change in the Go API (the commit gate needs the owner's one-time `--no-verify` for any Go commit).
3. **Version 1 gives wrong event times to eight workbook layouts** (measured on synthetic files with the shape of the real exports; the real files may differ). For C1 to C6 the source holds Excel serial times that version 1 does not parse, and it stores the ingestion instant on every row (all 706 rows of C1 carry 2026-10-13 14:34:49); for C8 it stores the year 2065; for C7 it rejects every row. Any time-based question over such a file is wrong in version 1. The typed rows have the right instants. This is the concrete case for moving families to the typed tables (roadmap 4.8).
4. **Differences by design.** The typed path quarantines a row that has no usable time (version 1 keeps it, with the ingestion instant), keeps and flags repeated rows (version 1 drops them), and skips blank rows (version 1 also drops them from its total). The row counts therefore differ by the quarantined rows only, which the summary lists by reason.

**Not done.** Nothing was removed from the live database; the synthetic collection is retained. No code or image changed in this step.
