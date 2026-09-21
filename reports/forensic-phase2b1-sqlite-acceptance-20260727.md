# NexusAI Phase 2B.1 SQLite Inventory Acceptance

Date: 2026-07-27  
Deployment profile: Pakistan (`PK`, `Asia/Karachi`)  
Scope: deterministic, raw read-only SQLite technical/schema inventory

## 1. Objective

Add a bounded SQLite format 3 inventory at evidence registration without adding
a database driver, executing SQL, loading extensions, following journal/WAL
sidecars, or reading evidence data values. The slice must validate the physical
header and schema B-tree, retain bounded schema identity, count rows only through
bounded B-tree traversal, preserve warnings across all registration surfaces,
and keep the full query/normalization route explicitly pending.

## 2. Assumptions and capacity verified

- The prior deployed forensic API was v1.2 at
  `sha256:12a5448490e3456cf65b16ded18e70d573e08bc9f0073fd7b3fac7ba028bf701`.
- LocalAI, PostgreSQL, NATS, and the unchanged records worker were healthy.
- Pre-build host capacity was 4.30 GiB free physical memory and 1,780.79 GiB
  free disk.
- The API image is built with `CGO_ENABLED=0`, so the implementation uses no
  SQLite engine or new dependency.
- `records-demo-verified` was treated as immutable and rechecked after deployment.
- All live acceptance data was synthetic; the user's real CDR was not accessed
  or ingested.

## 3. Files changed

- `api/forensic_records/media_metadata.go`: extractor version 1.3.0 and SQLite
  dispatch for `.sqlite`, `.sqlite3`, and `.db`.
- `api/forensic_records/media_metadata_sqlite.go`: bounded raw-file inventory.
- `api/forensic_records/sqlite_inventory_ginkgo_test.go`: valid, WAL, active
  schema, corruption, cycle, mismatch, SQL-dump abstention, and name-bound tests.
- `api/forensic_records/file_inventory_ginkgo_test.go`: current extractor version.
- `configuration/forensic_modality_evaluation_matrix.json`: database support
  advanced from planned to foundation with remaining gates stated explicitly.
- `docs/content/features/forensic-intelligence-phase1.md`: behavior, limits,
  warnings, and non-capabilities.
- This report and `NEXUSAI_CONTINUATION.md`.

Unrelated dirty-worktree changes were preserved.

## 4. Schema, API, and configuration impact

No PostgreSQL schema migration, backfill, endpoint, auth/RLS, Compose, or
environment change occurred. The existing `media_metadata` JSON now reports
extractor `1.3.0` and SQLite inventory fields. Classification remains
`database_adapter_pending`, `queue_records=false`, so database evidence never
reaches NATS or the records worker.

No SQL-dump execution was added. `.sql` remains registered but receives no
automatic deterministic database parsing.

## 5. Automated tests

Final source validation:

- `go test ./api/forensic_records -count=1`: passed; package time 29.579 seconds.
- 69 Ginkgo specs are present in the package after adding seven SQLite cases.
- `go vet ./api/forensic_records`: passed.
- The modality evaluation matrix parsed as valid JSON.

The Windows default Go cache first returned an ACL error. A repository-local
cache avoided that host problem. Its first cold compilation reached the
120-second command ceiling without a compiler/test failure; the resumed run
passed, and the final cached validation above passed. Temporary cache contents
were removed after validation.

SQLite regressions cover:

- header, page-size/count, encoding, schema-version, application/user-version,
  object, ordinary-table, and `WITHOUT ROWID` table counts;
- no SQL, extension, sidecar, data-value, or schema-SQL use;
- WAL omission warning;
- trigger and virtual-table definition abstention;
- table and schema B-tree cycles;
- corrupt magic and impossible cell-pointer arrays;
- header/file page-count disagreement;
- `.sql` dump abstention; and
- 512-byte schema object-name enforcement.

## 6. Live runtime and deployment

Only the forensic API was rebuilt and recreated.

- Docker build/recreate wall time: 47.6 seconds.
- Go compile step: 33.6 seconds.
- Deployed API image:
  `sha256:9dc1ec79b34aa69913230f2d76e7e4b89c1c1795a7031cad146f381eb8032772`.
- Image/container creation: approximately 2026-07-27 05:12:53/05:12:57 UTC.
- API restart count: 0.
- Worker was not rebuilt and remained running.
- LocalAI remained healthy and unchanged at
  `sha256:0731ab05fa080fae9bc90ddbcc26f74fcb554389ee82aa0282e5373a27144c9b`.
- PostgreSQL and NATS remained healthy.
- API logs contain the startup line and exactly the two expected non-record
  database registrations, with no error line.

## 7. Dataset and evidence counts

Collection: `forensic-sqlite-inventory-acceptance-20260727`  
Case: `PK-SYNTHETIC-SQLITE-20260727`

| Measure | Result |
| --- | ---: |
| Synthetic SQLite source files | 2 |
| Source bytes | 32,768 |
| Evidence items | 2 |
| KB assets | 2 |
| LocalAI KB entries | 2 |
| Records-worker jobs | 0, expected |
| Failed evidence/jobs/errors | 0 |
| Warning-bearing evidence | 1 WAL-mode source |

Both files contain four schema objects: two tables, one index, and one view.
The `calls` rowid table has exactly three rows and the `accounts` `WITHOUT ROWID`
table has exactly two. Both carry `user_version=7`, application ID 1314412361,
Pakistan jurisdiction/timezone lineage, and distinct SHA-256 hashes.

The clean source is evidence
`053af46c-c14f-4265-837a-bc45626ff8f2`, SHA-256
`f905f77d3c61de37979aacd99daec962bf5f276318ac61f84c67899529587473`.
The WAL-header source is evidence
`3ca9201e-e61c-4a17-88cd-f81e35a7bbb6`, SHA-256
`48982a6acfb9be60ad29da7be116d5964bacd7b4ff2ea7ce0ca751c517b67ba5`.

The local temporary generator copies were removed after upload and
reconciliation. Immutable spooled evidence and KB entries remain intact.

## 8. Models

No model was called, downloaded, changed, or benchmarked. Visible IDs remain:

- `qwen_qwen3-4b-instruct-2507`
- `qwen3-embedding-0.6b`

## 9. Accuracy and agreement

- 2/2 sources: format, 4,096-byte page size, four actual/declared pages,
  UTF-8, schema format 4, user/application versions, and four schema objects
  matched the independent Python SQLite engine baseline.
- 4/4 schema objects matched type, name, table association, and root page.
- 4/4 table row-count checks matched across the two sources: `calls=3` and
  `accounts=2` in each.
- 2/2 sources reported complete bounded inventory and complete table counts.
- 2/2 sources had matching evidence, KB asset, and KB entry lineage.
- The clean source had zero warnings.
- The WAL source returned the one expected omission warning immediately and the
  same warning persisted on the evidence item and KB asset quality report.
- No column, foreign-key, selected-row, SQL execution, data extraction, recovery,
  or semantic accuracy is claimed.

## 10. Latency and memory snapshots

- API health request: 400.55 ms.
- Two-item evidence catalog request: 134.92 ms.
- API: 8.168 MiB.
- Worker: 47.02 MiB.
- PostgreSQL: 52.4 MiB.
- NATS: 8.805 MiB.
- LocalAI: 667.2 MiB.
- Post-run free physical memory: 3.34 GiB.

These are single local snapshots, not p50/p95 benchmarks or capacity limits.

## 11. Failures, fallbacks, and unresolved limits

- The Windows default Go cache ACL blocked the first test command. The documented
  workspace-local cache fallback passed; no source behavior was changed to hide
  the environment issue.
- A first read-only detail request used ambiguous PowerShell `$id?` expansion and
  sent `=default` as the path. `${id}` corrected the client URL. The bad read
  caused no evidence/database mutation.
- A single uploaded main database cannot include separate uncheckpointed WAL
  frames. WAL mode therefore always warns.
- Header and bounded schema/table B-tree inventory is implemented; column types,
  foreign keys, selected rows, deleted-record recovery, freelist traversal,
  integrity checking, SQL dumps, and other database engines remain pending.
- Schema object names are retained because they are the inventory identity;
  schema SQL text and evidence data values are not retained.

## 12. Security, tenant isolation, and provenance

The parser uses only `os.File.ReadAt` over the registered immutable source. It
never imports a SQLite driver, interprets SQL, calls a database engine, loads an
extension, follows an attachment, or opens `-wal`, `-journal`, or `-shm` paths.

Limits are 4,096 distinct pages, 64 MiB read, 2,048 schema objects, 1 MiB per
schema record, 8 MiB aggregate schema payload, 512 bytes per retained name,
B-tree depth 64, and 256 row-counted tables. Page cycles, out-of-range pages,
invalid payload fractions, malformed records/encodings, impossible sizes, and
unsupported format versions fail closed while evidence registration preserves
the source and warning.

Existing tenant, collection, case, evidence/version, hash, raw storage, KB entry,
jurisdiction, timezone, audit, warning, and extractor-version lineage remains
intact. No cross-tenant path was introduced.

## 13. Git state

The worktree remains dirty, unstaged, uncommitted, and unpushed. No branch,
commit, pull request, GitHub issue, or external publication was created. No user
change was reset or discarded.

## 14. Rollback and recovery

The pre-SQLite v1.2 API is preserved as
`nexusai-forensic-records-api:rollback-20260727-file-inventory-v1.2`, resolving
to `sha256:12a5448490e3456cf65b16ded18e70d573e08bc9f0073fd7b3fac7ba028bf701`.

Current v1.3 is
`sha256:9dc1ec79b34aa69913230f2d76e7e4b89c1c1795a7031cad146f381eb8032772`.
No schema rollback or data reversal is required because this slice adds only
registration-time JSON metadata. Persistent volumes and acceptance evidence are
unchanged by image rollback.

## 15. Next action and approval gates

Continue Phase 2B.1 with bounded PCAP and PCAPNG capture metadata: magic/endian
validation, link types/interfaces, packet counts and timestamp bounds, captured
versus original lengths, truncation, malformed blocks, and resource caps without
payload protocol interpretation or session reconstruction. EVTX remains a
separate later adapter.

After capture inventory, continue MP3/MP4/video container headers and expand
Pakistan golden fixtures for ANPR, financial transactions, subscriber/tower
data, access/application logs, and bilingual Urdu/English evidence.

Separate approval remains required before model changes/downloads, real sensitive
data ingest, database migrations/backfills, heavy LocalAI rebuild, destructive
cleanup of persistent evidence, Git staging/commit/push, or publication.
