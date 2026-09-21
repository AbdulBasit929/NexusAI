# NexusAI Phase 2B.1 PDF and Archive Inventory Acceptance

Date: 2026-07-27  
Deployment profile: Pakistan (`PK`, `Asia/Karachi`)  
Scope: deterministic PDF, ZIP/ZIP64, and TAR technical inventory plus warning propagation

## 1. Objective

Extend the model-free evidence-registration boundary from media headers to
bounded PDF structure and archive-safety inventory. The implementation must
preserve original evidence, execute no document actions, unpack no general
archive content, identify high-risk structures early, and expose consistent
warnings to the upload client, evidence record, and linked Knowledge Base asset.

This slice deliberately does not claim PDF text/OCR extraction, attachment
extraction, archive unpacking, recursive child registration, RAR/7z support,
compressed-TAR support, or semantic document analysis.

## 2. Assumptions and capacity verified

- The previous deployed forensic API was deterministic metadata v1.1 at
  `sha256:43489c192a7d24982ad8361f3aa84d048b2aaf9c379869c8a866ce7b11974aba`.
- LocalAI, PostgreSQL, NATS, and the records worker were already running and
  healthy. Persistent volumes were not recreated.
- Pre-build host capacity was approximately 5.2 GiB free physical memory and
  1,781 GiB free disk, sufficient for the API-only rebuild.
- The baseline collection `records-demo-verified` was treated as immutable and
  remained at 9,250 accepted records with no rejected or duplicate records.
- All acceptance files were synthetic. The user's real CDR was not ingested or
  copied into a test collection.

## 3. Files changed

- `api/forensic_records/media_metadata.go`: extractor version 1.2.0 and dispatch.
- `api/forensic_records/media_metadata_pdf.go`: bounded PDF structural inventory.
- `api/forensic_records/media_metadata_archive.go`: ZIP/ZIP64 and TAR inventory.
- `api/forensic_records/file_inventory_ginkgo_test.go`: positive, malformed,
  hostile, and resource-bound regressions.
- `api/forensic_records/main.go`: registration-warning return and merge path.
- `api/forensic_records/evidence_classification_ginkgo_test.go`: warning merge
  regression.
- `configuration/forensic_modality_evaluation_matrix.json`: PDF/archive state
  and next gates.
- `docs/content/features/forensic-intelligence-phase1.md`: operator behavior,
  bounds, limitations, and warning consistency.
- This report and `NEXUSAI_CONTINUATION.md`.

Unrelated pre-existing worktree changes were preserved.

## 4. Schema, API, and configuration impact

There was no schema migration, database backfill, new endpoint, auth/RLS change,
Compose change, or environment-variable change. The existing evidence metadata
shape now reports `media_metadata.extractor_version=1.2.0` for this extractor.

The existing upload response warning field was corrected to include warnings
created during evidence registration. No response field was added. Warnings are
deduplicated and now agree across the upload response, evidence item, and KB
asset `quality_report`.

## 5. Automated tests

Final validation after the warning propagation correction:

- `go test ./api/forensic_records -count=1`: passed in 10.320 seconds.
- `go vet ./api/forensic_records`: passed.
- `configuration/forensic_modality_evaluation_matrix.json`: parsed as valid JSON.
- `git diff --check`: no whitespace error; only existing Windows line-ending
  conversion notices were reported.

Coverage includes valid two-page PDFs; active-content, encrypted, attached-file,
missing-EOF, token-cap, and deceptive string/comment/stream PDF cases; safe ZIP;
ZIP traversal, symlink, nested archive, duplicate, ratio, and declared-size bomb
cases; safe TAR; TAR absolute-path, link, nesting, checksum, and oversized-member
cases. Tests use Ginkgo/Gomega as required by repository policy.

## 6. Live runtime and deployment

Only the forensic API was rebuilt. LocalAI and the worker were not rebuilt.

- First v1.2 API candidate: `sha256:995434ff5cd923f7d3885601ce2391ba7671fd443ef89e11802f76ef5bdf03e6`.
- Final API after warning propagation fix:
  `sha256:12a5448490e3456cf65b16ded18e70d573e08bc9f0073fd7b3fac7ba028bf701`.
- Final API image creation time: 2026-07-27 04:49:41 UTC.
- Second API-only build and deploy: 25.8 seconds total, including a 19.4-second
  compile.
- LocalAI remained unchanged and healthy at `sha256:0731ab05...`.
- Worker remained unchanged at `sha256:faeb6559...`.
- API and worker restart counts were zero after deployment; PostgreSQL and NATS
  were healthy.

## 7. Dataset and evidence counts

Final clean collection `forensic-document-archive-acceptance-20260727-v2`:

| Measure | Result |
| --- | ---: |
| Synthetic source objects | 5 |
| Evidence items | 5 |
| KB assets | 5 |
| LocalAI KB entries | 5 |
| Documents | 2 |
| Containers | 3 |
| Total source bytes | 6,201 |
| Failed evidence/jobs/errors | 0 |
| Records-worker jobs | 0, as expected for inventory-only routes |

The sources were a safe PDF, an active/attachment PDF, a safe ZIP, a hostile
ZIP, and a safe TAR. Two evidence items carried warnings. There were five actual
warning strings: two on the active PDF and three on the hostile ZIP.

The earlier diagnostic collection
`forensic-document-archive-acceptance-20260727` is intentionally preserved. A
PowerShell fixture-generator error produced a malformed 90-byte PDF; PDFium
correctly rejected its KB mirror. The diagnostic collection has four evidence
items, three KB assets, no worker jobs or job failures, and a LocalAI KB count of
four. That KB/evidence-asset count difference is recorded as diagnostic state,
not presented as acceptance success and not deleted or concealed.

The isolated warning propagation collection
`forensic-warning-propagation-acceptance-20260727` contains one hostile ZIP, one
evidence item, one KB asset, and one KB entry.

## 8. Models

No model was called, downloaded, changed, or benchmarked. The visible LocalAI
model IDs remained:

- `qwen3-embedding-0.6b`
- `qwen_qwen3-4b-instruct-2507`

The implementation is deterministic Go parsing and does not depend on either
model.

## 9. Accuracy and agreement

- 5/5 clean sources agreed with their expected technical inventory fields.
- 5/5 clean sources had a KB mirror, evidence item, and linked KB asset.
- 5/5 expected security-warning strings appeared exactly: active content and an
  embedded attachment on the risky PDF; unsafe path, nested archive, and high
  expansion ratio on the hostile ZIP.
- In the isolated propagation check, all three hostile-ZIP warnings matched
  across the immediate upload response, persisted evidence warnings, and linked
  KB asset `quality_report.warnings`.
- The safe PDF reported PDF 1.7, two declared and two visible pages, reliable
  page count, complete inventory, and zero warnings.
- The active PDF reported two pages, one embedded-file object, JavaScript and
  OpenAction indicators, complete inventory, and the two expected warnings.
- The safe ZIP reported three members, 28 declared expanded bytes, safe status,
  and zero warnings.
- The hostile ZIP reported an unsafe path, nested archive, maximum ratio about
  963.76:1, unsafe status, and three expected warnings.
- The safe TAR reported three members, valid header checksums, two terminal zero
  blocks, safe status, and zero warnings.

These are deterministic fixture agreements, not OCR, document-understanding, or
malware-detection accuracy claims.

## 10. Latency and memory snapshots

- API health request: 3.83 ms.
- Five-item clean evidence catalog request: 6.63 ms.
- Post-run free physical memory: 3.77 GiB.
- Forensic API: 5.215 MiB.
- Records worker: 47.02 MiB.
- PostgreSQL: 54.52 MiB.
- NATS: 8.367 MiB.
- LocalAI: 671.8 MiB.

These are single local snapshots, not percentile benchmarks or capacity limits.

## 11. Failures, fallbacks, and unresolved limits

Two issues were found and handled transparently:

1. The first PowerShell PDF generator used numeric ordered-dictionary keys
   incorrectly, creating a malformed 90-byte file. The source and failed mirror
   state were retained. A corrected generator produced the clean v2 collection.
2. Live acceptance found that evidence-registration warnings were persisted but
   not returned by the outer upload response. The return value and merge path
   were corrected, regression-tested, rebuilt, and verified in an isolated live
   collection.

Current technical limits are explicit:

- PDF processing is a bounded lexical structural inventory, not a conforming
  rendering parser or text extractor. Page reliability is only asserted under
  the conservative reconciliation conditions recorded in metadata.
- ZIP validation covers end records and central-directory declarations; general
  member bodies are not decompressed. Only bounded Store/Deflate symlink targets
  are inspected.
- TAR validation covers headers, checksums, declared ranges, and terminal blocks.
  PAX/GNU extensions are skipped and marked incomplete; regular bodies are not
  read.
- RAR, 7z, compressed TAR, recursive child registration, attachment extraction,
  SQLite, PCAP/PCAPNG, video, and MP3/MP4 inventories remain pending.
- Explicit jurisdiction/timezone forwarding by LocalAI remains source-only in
  the current worktree; the deployed API's verified Pakistan defaults remain the
  runtime fallback.

## 12. Security, tenant isolation, and provenance

The PDF lexer skips comments, literal strings, hex strings, and stream bodies,
never executes actions, and caps scanning at the first 64 MiB plus the final
128 KiB and 500,000 structural tokens. It flags encryption, embedded files,
JavaScript/actions, forms/XFA, signatures, and object streams.

Archive inventory caps ZIP central-directory processing at 64 MiB and 10,000
members and marks review requirements above 2 GiB per member, 10 GiB aggregate,
or 100:1 ratio. It flags traversal/absolute paths, links, nested archives,
encryption, duplicates, non-UTF-8 names, unsupported compression, and special
files. TAR scanning uses constant-size 512-byte headers.

No member-name list or PDF text is persisted by this extractor. Existing tenant,
collection, case, evidence/version, source hash, raw storage, KB entry, audit,
jurisdiction, timezone, warning, and extractor-version lineage remains intact.
No cross-tenant query or write path was introduced.

## 13. Git state

The worktree remains dirty, unstaged, uncommitted, and unpushed. No branch,
commit, pull request, GitHub issue, or external publication was created. Existing
user changes were not discarded or overwritten.

## 14. Rollback and recovery

- Previous v1.1 API rollback tag:
  `nexusai-forensic-records-api:rollback-20260727-media-v1.1`, preserving
  `sha256:43489c192a7d24982ad8361f3aa84d048b2aaf9c379869c8a866ce7b11974aba`.
- Pre-warning-fix v1.2 rollback tag:
  `nexusai-forensic-records-api:rollback-20260727-file-inventory-pre-warning-fix`,
  preserving `sha256:995434ff5cd923f7d3885601ce2391ba7671fd443ef89e11802f76ef5bdf03e6`.
- Current deployable API image:
  `sha256:12a5448490e3456cf65b16ded18e70d573e08bc9f0073fd7b3fac7ba028bf701`.

Persistent volumes and all acceptance/diagnostic evidence remain intact. A
rollback requires only selecting the intended API image and recreating that
service; no data reversal is required because this phase made no schema change.

## 15. Next action and approval gates

The next bounded Phase 2B.1 implementation slice should be read-only SQLite
technical/schema inventory, followed by PCAP/PCAPNG inventory and then MP3/MP4
and video container headers. Each slice should add adversarial bounds, synthetic
Pakistan-oriented golden fixtures, full package tests, an API-only deploy, and a
clean isolated live acceptance collection before its support state is advanced.

In parallel but not conflated with binary inventories, expand Pakistan goldens
for ANPR, financial transactions, subscriber/tower records, access/application
logs, bilingual Urdu/English text, and provider/vendor variants. Every adapter
must preserve raw values, explicit timezone and jurisdiction, source hashes,
reject/quarantine reasons, and exact deterministic analytics.

Separate explicit approval remains required before model downloads or changes,
real sensitive-data ingest, database migrations/backfills, a heavy LocalAI
rebuild, destructive cleanup, Git staging/commit/push, or external publication.
