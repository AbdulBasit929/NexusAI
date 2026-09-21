# NexusAI Forensic Intelligence — Plain-Language Team-Lead Brief

Updated: 2026-07-29

## One-minute summary

NexusAI has been changed from a basic LocalAI/Knowledge Base setup into the
foundation of a local forensic-intelligence platform. It can now accept many
different evidence types through one upload path, preserve each source with a
hash and evidence identity, route supported structured records to exact database
processing, route documents to Knowledge Base search, and safely register media,
archives, databases, and captures without pretending unsupported analysis has
already happened.

The system is running locally on the target Windows laptop, using CPU-only local
models. Phase 2 is complete. The verified structured baseline contains 9,250
records, and its historical registry now reconciles to eight KB entries, six
evidence objects, and eight source links after a verified scoped backup. A
separate 20-source acceptance proves every declared evidence family,
Pakistan-oriented structured handling, and bounded technical inventory for
images, audio, video, documents, archives, SQLite, and captures. Rollback images
and the pre-reconciliation database dump are retained.

This completes the Phase 2 foundation, not the whole production forensic suite.
Phase 3 source development now passes isolated acceptance across four connected
slices: normalized evidence/version/custody schema, authenticated scope and
non-owner RLS, scoped content-addressed retention, and the durable queue
lifecycle. The queue now has atomic outbox registration, file-backed JetStream
persistence, database leases, commit-before-ack, bounded retry, DLQ and immutable
idempotent reprocessing. None of Phase 3 has been applied to the retained
database or deployed. A higher-memory/CI LocalAI wrapper check plus backup,
migration, runtime grant, secret, persistent-NATS and deployment approvals
remain. OCR, STT, full video, deep sessions, archive extraction, database
querying and production operations remain ordered later work.

Phase 4 structured-data v1 is now source-complete. The application reports truthful collection
capabilities across all 20 evidence families, blocks unaccepted processing before
SQL/KB/model execution, streams messy TSV, and safely parses read-only XLSX. XLSX
rows retain worksheet/row/cell provenance, raw values, formulas and cached values;
macros, external links and formula recalculation never run. Mixed workbooks now
classify each sheet independently into CDR, ANPR, transaction or another uniquely
matched structured family while preserving ambiguous/unrelated sheets generically.
The first analyst-first UI simplification also hides advanced, evidence and batch
controls until requested. This source is tested but has not been rebuilt or deployed.

## What has been completed

### 1. Safe continuation and recovery

- Reconciled the project after moving to the new laptop.
- Preserved all earlier source changes and data.
- Created a living checkpoint, test ledger, decision log, reports, and rollback
  points.
- Did not stage, commit, push, or publish anything to GitHub.

In simple words: development can continue without losing the earlier work, and
each deployment can be reversed locally if a new slice fails.

### 2. One evidence-registration boundary

- The normal Knowledge Base upload can register structured records, documents,
  spreadsheets, images, audio, video, transcripts, TTS outputs, captures,
  databases, archives, and unknown binary files.
- Every registered item receives evidence/version IDs, SHA-256, collection/case,
  source reference, classification, route, status, warnings, and audit lineage.
- Only formats understood by the records worker are queued. Unsupported binary
  evidence is preserved but not incorrectly parsed as CSV.

In simple words: the system accepts everything safely, but only performs an
operation when that operation has a tested adapter.

### 3. Structured forensic records and exact analytics

- Typed handling exists for CDR, IPDR, ANPR, subscriber, tower/location,
  financial transaction, access log, and generic records.
- Pakistan ANPR, PKR transaction, subscriber/identity, tower/sector, and
  access/security-log adapters now have versioned messy synthetic goldens with
  exact accepted/rejected/duplicate accounting; source rows remain unchanged
  while derived search, confidence, time, money, identity, radio-site, and
  network-event fields are stored separately.
- Exact counts, filters, timelines, and structured queries use PostgreSQL rather
  than asking an AI model to guess.
- The protected baseline contains:

| Record family | Verified rows |
| --- | ---: |
| CDR | 5,000 |
| IPDR | 2,500 |
| ANPR | 750 |
| Access log | 1,000 |
| Total | 9,250 |

### 4. Pakistan alignment

- Added `PK` and `Asia/Karachi` evidence defaults.
- Naïve Pakistan timestamps are interpreted in the declared timezone and stored
  canonically in UTC while raw values remain preserved.
- Added a versioned Pakistan profile for phones, IMSI/IMEI, CNIC, IBAN, ANPR,
  Urdu/English scripts, provenance, and relevant official-source references.
- Profiled the supplied real CDR read-only and converted observed messy cases
  into synthetic regression fixtures. The real CDR itself was not ingested.
- Live synthetic Pakistan CDR acceptance verified aliases, 14-digit IMEI,
  negative cell sentinel, duplicate handling, and timezone conversion.
- Live structured acceptance now covers common ANPR vendor headers,
  Urdu/Latin plates, camera metadata, PKR debit/credit aliases, exact decimals,
  reversals, Pakistan IBAN checks, malformed rows and exact duplicates.

### 5. Knowledge Base plus exact records

- The query planner can use exact Records SQL, Knowledge Base retrieval, or both.
- Bounded LocalAI synthesis receives only capped structured facts and retrieved
  evidence excerpts, treats source text as untrusted, and falls back to a
  deterministic answer on timeout/failure.
- A warm live hybrid query returned structured facts plus real retrieved
  citations. A cold timeout safely fell back instead of inventing an answer.

### 6. Deterministic technical inventory already live

These adapters do not need a large AI model:

| Evidence | What is live now | What is still pending |
| --- | --- | --- |
| PNG/JPEG/GIF/BMP/TIFF/WebP | Dimensions, orientation, bounded header/EXIF flags | OCR, objects, plates, visual explanation |
| WAV/FLAC | Channels, sample rate, bit depth, duration, bounded headers | STT, diarization, speaker analysis |
| PDF | Version, pages, EOF, encryption, attachments, JavaScript/actions, forms/signatures | Text/layout extraction, OCR, attachments, semantic page citations |
| ZIP/ZIP64/TAR | Member/type/size/ratio counts, traversal, links, nesting, encryption and bomb-risk checks | Unpacking, child evidence, RAR/7z/compressed TAR |
| SQLite | Header, pages, encoding, schema objects/root pages, bounded exact table counts | Columns, foreign keys, selected rows, recovery, SQL dumps, other engines |
| PCAP/PCAPNG | Endianness, precision, sections/interfaces, packet/byte/truncation counts and time bounds | Endpoints, protocols, DNS/HTTP, flows, sessions, EVTX |
| TSV/XLSX | Streaming rows, dialect/workbook structure, exact raw values, sheet/row/cell lineage, per-sheet typed mapping with generic preservation, hidden state, formulas versus cached values, dates/errors/merges | XLS/XLSM, ODS, deeper columnar formats; formula recalculation and macros stay prohibited |

In simple words: NexusAI can safely describe the technical shape and risks of
these files today, but it does not yet claim to understand every item inside.

### 7. Testing and live proof

- The final Phase 4 regression passes 50/50 Python worker/adapter/XLSX/IPDR/
  attached-CDR tests and the complete forensic API Go package; its versioned
  goldens prove exact typed-family accounting and mixed-sheet provenance.
- The final isolated Phase 2 collection reconciles exactly across 20 sources:
  59 structured rows, 34 unique accepted records, 6 exact duplicates, 19 visible
  rejects, 8 completed jobs, 20 evidence items, 20 KB assets and 20 KB entries,
  with zero failures or missing links.
- The unchanged three-round CPU evaluation passed 18/18 chat guardrail checks,
  27/27 fail-closed deterministic routes, and three 8-vector embedding runs at
  1,024 dimensions. No candidate model was promoted.
- Each recent slice was deployed API-only and tested with isolated synthetic
  collections.
- Warnings are consistent across the immediate upload response, evidence record,
  and KB asset quality report.
- Current services are healthy; LocalAI and its two baseline models remain
  unchanged.
- No schema migration, model download, real sensitive-data ingest, or GitHub
  publication occurred. The explicitly requested historical registry
  reconciliation was applied only after a verified scoped database backup.

## The four words the team should use accurately

| Word | Meaning |
| --- | --- |
| Registered | Source is preserved, hashed, identified, classified, and auditable. |
| Inventoried | Safe deterministic metadata/structure has been extracted. |
| Parsed | A typed adapter has produced usable normalized facts/rows. |
| AI-analyzed | A versioned model produced a derived result with parameters and lineage. |

Do not say that every registered format is fully parsed or AI-analyzed. This
distinction is one of the project's main safety controls.

## Recommended 90-second update to give the team lead

> We now have a running local forensic-intelligence foundation on top of
> LocalAI. A single governed upload path registers all important evidence types
> with hashes, evidence/version IDs, Pakistan jurisdiction and timezone,
> Knowledge Base links, warnings, and audit lineage. Supported structured data
> such as CDR, IPDR, ANPR, transactions, subscriber/tower and access logs can be
> processed deterministically, while unsupported formats are preserved safely
> instead of being misparsed.
>
> The protected baseline has 9,250 verified rows, and the backed-up registry
> reconciliation preserves all eight KB links. We have live synthetic Pakistan
> CDR, ANPR, PKR transaction, subscriber, tower, and security-log acceptance and
> deterministic technical inventory for common
> image/audio headers, PDF risk structure, ZIP/TAR safety, SQLite schema/table
> counts, PCAP/PCAPNG packet-capture structure, MP3 frame structure, and
> MP4/MOV track/container structure. Exact facts come from code
> and SQL; LocalAI is used only for bounded, evidence-grounded explanation with
> a deterministic fallback.
>
> Phase 4 source now also exposes a truthful 20-family capability map, supports
> messy TSV, and parses read-only XLSX with worksheet/row/cell lineage. Leading
> zeros, exact PKR decimals, Urdu, hidden data, formulas, cached values, dates,
> errors and merged ranges are preserved. Formulas/macros are never run and
> external links are never followed.
>
> Phase 2 is complete: all 20 evidence families have executable fixture contracts,
> the live 20-source collection has no failures or missing links, and the unchanged
> baseline passed all 18 chat and 27 deterministic route checks. We are not
> claiming the full platform is finished. OCR, speech recognition,
> deep documents, full network sessions, video understanding, entity graphs,
> final tenant deployment, analyst UI and production release gates are the next
> ordered work. Phase 3 source now stores uploads by scoped content hash with
> create-only publication, a read-only receipt, full size/hash re-verification,
> corruption quarantine, and a read-only worker boundary. It also saves evidence
> and queue jobs atomically, persists work in JetStream, commits before ack,
> retries temporary failures, dead-letters terminal failures and creates linked
> reprocessing generations without erasing history. Disposable database and NATS
> restart acceptance passed. This is not yet deployed and is not claimed as
> regulatory WORM. No real sensitive file has been ingested, no new
> model was downloaded, and nothing has been pushed to GitHub yet. Every live
> slice has tests, an isolated acceptance collection, and a rollback image.

## Upcoming phases in easy words

### Immediate next gate — activate the completed Phase 3 source safely

The first Phase 3 schema slice is now a validated migration candidate. It
normalizes evidence versions, storage objects, source links, processing runs and
events, typed derived artifacts, and hash-chained custody events. Disposable
tests proved old-row backfill, new registration, append-only enforcement,
repeat-apply stability, all-or-nothing failure and gated rollback. The running
registry passed a read-only compatibility preflight for 110 evidence, 104 linked
KB assets and 41 linked jobs; no Phase 3 schema was applied.

The authenticated-scope source slice is also complete. LocalAI now checks exact
collection ownership, distinguishes the acting identity from the subject user,
and forwards bearer-authenticated tenant/collection/case context. The sidecar
rejects missing credentials, cross-tenant or payload scope changes, and
non-administrator repair. A disposable `NOBYPASSRLS` role proved same-tenant
visibility/write and cross-tenant rejection across all seven Phase 3 tables.

The content-addressed-retention source slice is now complete too. Raw uploads use
a tenant/collection scope hash plus content SHA-256, atomic create-only publish,
read-only receipt, full post-retain verification and integrity quarantine. The
worker independently validates the URI, canonical path, scope, receipt, size and
hash from a read-only mount. Concurrent duplicate, traversal, symlink, tamper and
corruption tests pass. The full migration/storage contract also passes in a
disposable TimescaleDB container.

The durable queue slice is now implemented. The upload and its outbox job commit
together; pending work is stored on disk in JetStream; database leases serialize
workers; database results commit before synchronous acknowledgement; temporary
failures retry with a cap; poison/exhausted jobs reach a bounded DLQ; and
reprocessing creates a new linked generation without deleting earlier outputs.
A disposable database proved migrations 001-009 and a disposable NATS restart
proved deduplication, persistence and acknowledgement recovery.

Next, run the focused LocalAI wrapper package on CI or higher-memory hardware,
then take and verify a fresh scoped backup and request separate approval for
migrations 008/009, runtime grants, persistent NATS storage, service secrets and
targeted deployment. Infrastructure WORM/object lock and legal-retention policy
are also deployment decisions, not claims made by this local filesystem contract.

Success means evidence cannot be silently overwritten, lost, or mixed across
tenants, and every transition or derivative is attributable and reversible.

### Phase 3 — Evidence control plane and chain of custody

Source complete: normalized evidence versions, processing runs, derived
artifacts, append-only events, immutable storage rules, authenticated tenant/case
scope, permissions, retry, acknowledgement, dead-letter and explicit reprocessing
behavior. Production activation remains gated as described above.

Success means evidence cannot be silently overwritten or lost, and every action
is attributable and reversible.

### Phase 4 — Deeper structured-data adapters

TSV, bounded read-only XLSX, conservative per-sheet typed mapping, and exact
cross-family correlation are complete in source. Next harden multiple
CDR/IPDR/ANPR/subscriber/tower/finance/log provider profiles and family-specific
queries, followed by other spreadsheet/columnar formats based on fixture value.

Success means 100% row accounting and exact answers without silent column,
currency, precision, timestamp or duplicate loss.

### Phase 5 — Documents and OCR

Extract native PDF/Office/email text, tables, attachments, reading order and
page/region coordinates. Only then benchmark one small CPU OCR candidate on
Pakistan Urdu/English and scanned-document fixtures.

Success means useful text with page/box citations and measurable OCR/table
accuracy; partial failures never discard the source.

### Phase 6 — Derived media/artifact foundation

Normalize artifacts such as OCR pages, thumbnails, transcripts, audio clips and
sampled frames with parent evidence/version, adapter/model revision, parameters,
time/box locators and retention rules.

Success means every derivative traces back to exact source bytes and location.

### Phase 7 — Audio, STT, diarization and TTS

Benchmark a small CPU speech model for noisy Urdu-English audio, store timestamped
transcripts and optional speaker turns, and keep TTS as a derived accessibility
artifact rather than new evidence.

Success means measured WER/timestamp/diarization performance and cited audio
time ranges.

### Phase 8 — Images, OCR, ANPR, objects and policy-gated faces

Add OCR boxes, plate/object detections, crops, confidence and detector versions.
Face capability stays disabled until legal, policy, permission and false-positive
controls exist.

Success means measured precision/recall and no identity claim from an
uncorroborated model detection.

### Phase 9 — Video timelines

Inventory streams, sample bounded frames/scenes, link audio/STT, track candidate
objects over time, and answer questions from stored detections/transcripts first.

Success means time-aligned, cited events without analyzing every frame blindly.

### Phase 10 — Deep captures, databases and archives

Build separately bounded PCAP sessions/protocol derivatives, EVTX events, SQLite
columns/foreign keys/selected rows, and safe archive child registration. The
technical inventories already delivered are the safety foundation for this work.

Success means no payload execution or path escape and full child/packet/row
provenance.

### Phase 11 — Entity and relationship graph

Resolve phones, people, IMEI/IMSI, IP/domain, accounts, vehicles, plates,
locations, files and organizations across sources. Store confidence, conflicts,
time validity and supporting evidence instead of declaring uncertain matches as
facts.

Success means defensible correlations and zero cross-case leakage.

### Phase 12 — Analyst UI, observability and production hardening

UI simplification runs continuously from Phase 4 onward; the first
progressive-disclosure slice is source-complete. The release phase finishes the
separate Analyst/Evidence/Ingestion/Administration workspaces, case evidence
inventory, progress/version/artifact views, retries/DLQ, quality warnings,
accessibility, metrics, professional reports, permissions, backup/restore,
load/security tests and operational documentation.

Success means analysts can see what happened, what failed, why an answer exists,
and how to recover safely.

### Phase 13 — Model candidate promotion

Compare only one model/backend change at a time against the unchanged fixed gold
set. Record license, source, version, quantization, CPU/RAM/disk, accuracy,
unsupported claims, latency and rollback before approval.

Success means a candidate measurably improves its declared task without breaking
accuracy, safety, provenance or laptop resource limits.

## Current risks to state openly

- Authenticated scope and non-owner RLS pass in source/disposable tests but are
  not yet enabled in the retained services or database.
- Scoped content-addressed retention, receipts and worker verification pass in
  source, but are undeployed. The local application contract is not regulatory
  WORM; production needs reviewed object lock/ACL, legal hold, retention and
  recovery controls.
- The queue lifecycle is verified in source/disposable NATS; retained persistent
  NATS activation, monitoring, backup and operational smoke are still gated.
- The historic four-link KB registry gap is closed; retain and protect the
  verified pre-closure database dump for rollback.
- Multipart MIME preservation passes source tests but the second full LocalAI
  rebuild stalled; deployed signature sniffing is the safe fallback.
- OCR/STT/video/session/entity capabilities are pending and must not be presented
  as operational.
- Work remains unstaged/uncommitted/unpushed until the human review and Git
  publication decision.
- Source annotations cover the forensic proxy, but the generated Swagger bundle
  predates those routes; offline regeneration needs the pinned CLI dependency or
  an approved generator container.

## Decisions needed from the team lead later

Phase 3 source design and isolated acceptance are complete. Separate approval
will be needed before:

1. Ingesting a real sensitive CDR/capture/database or other case evidence.
2. Applying migrations 008/009 or restoring/replacing registry data.
3. Downloading or activating a new OCR/STT/vision/reranker model.
4. Running another heavy LocalAI rebuild.
5. Enabling policy-sensitive face or identity capabilities.
6. Staging, committing, pushing, opening a PR, or externally publishing results.
7. Enabling the shared service key and authenticated scope in retained services.
8. Selecting and enabling production object-lock/WORM, legal-hold and retention
   policy for raw evidence.
9. Provisioning persistent NATS retention/backup and non-owner runtime database
   grants, then activating the queue lifecycle in retained services.

## Evidence and detailed reports

- `reports/forensic-phase4b-xlsx-acceptance-20260729.md` — bounded read-only
  XLSX extraction, exact-value/provenance goldens and active-content rejection.

- `reports/forensic-phase3-queue-lifecycle-completion-20260729.md` — atomic
  outbox, leases, retry/DLQ, reprocessing and disposable DB/NATS acceptance.
- `docs/design/forensic-queue-lifecycle.md` — queue state machine, failure
  recovery, security and deployment sequence.
- `reports/forensic-phase3-content-addressed-retention-20260729.md` — raw-byte
  retention, receipt, corruption/concurrency and disposable migration acceptance.
- `docs/design/forensic-content-addressed-retention.md` — storage addressing,
  trust boundary, verification lifecycle and production limits.

- `NEXUSAI_CONTINUATION.md` — authoritative living status and decision ledger.
- `reports/forensic-phase3-authenticated-scope-rls-20260728.md` — authenticated
  actor/subject scope, collection ownership, and non-owner RLS acceptance.
- `reports/forensic-pakistan-data-readiness-20260724.md` — Pakistan formats and
  detailed roadmap.
- `reports/forensic-phase2a-ingress-acceptance-20260724.md` — universal ingress.
- `reports/forensic-phase2b1-pakistan-media-acceptance-20260724.md` — Pakistan CDR
  and first media metadata.
- `reports/forensic-phase2b1-media-headers-acceptance-20260727.md` — image/audio.
- `reports/forensic-phase2b1-pdf-archive-acceptance-20260727.md` — PDF/archives.
- `reports/forensic-phase2b1-sqlite-acceptance-20260727.md` — SQLite.
- `reports/forensic-phase2b1-pcap-pcapng-acceptance-20260727.md` — captures.
- `reports/forensic-phase2-completion-acceptance-20260727.md` — final Phase 2
  fixtures, live acceptance, unchanged baseline, registry closure, and rollback.
- `reports/forensic-phase2-unchanged-baseline-20260727.json` — final unchanged
  three-round model and deterministic-route measurements.

## 2026-07-29 morning update — Phase 4 has started

### Easy version to say

"Phase 3 is complete in source and safely tested, but not deployed. I have now
started Phase 4. The system has a capability map for all 20 evidence families,
so it can say what this collection can answer, what only has Knowledge Base
context, and what still needs an adapter or model. If someone asks it to OCR,
transcribe, analyze video, unpack an archive, or query a database before that
capability is accepted, it stops before touching SQL, the KB or an AI model and
explains what is missing. I completed the first two Phase 4 format slices: TSV
files stream through the worker, and read-only XLSX preserves every sheet's rows,
hidden state, exact raw values, formula text, cached values, dates, errors and
source locators without executing workbook content. The UI shows this capability
map and lets an operator reprocess evidence without deleting history. Nothing
has been rebuilt, migrated, downloaded, staged or pushed yet."

### Technical proof

- New collection-authorized capability endpoint covers exactly 20 matrix families.
- Dynamic states distinguish queryable, semantic-only, registered-pending,
  no-data and manual-review coverage.
- Hybrid query guard blocks unaccepted derived processing before DB/KB/LLM work.
- Streaming TSV adapter accepts UTF-8/UTF-16/CP1252, detected delimiters,
  duplicate/blank headers, overflow columns and exact source strings.
- Bounded XLSX adapter passes versioned PKR/Urdu multisheet goldens and rejects
  macros, DTD/entities, traversal members and malformed shared-string references.
- Records Intelligence displays family states and supports idempotent, immutable
  evidence reprocessing with attempts and generations.
- The final 50/50 combined Python tests pass; full forensic Go package passes; focused
  capability contracts pass; React lint has zero errors from Phase 4A.
- Model research is recorded, but no model was downloaded: first OCR evaluation
  is Tesseract Urdu/English versus PaddleOCR PP-OCRv5 Arabic/Urdu; first STT
  evaluation is multilingual Whisper tiny then small. Promotion still requires
  fixed Pakistan goldens, license review and RAM/latency/accuracy acceptance.

### What comes next

1. Expand exact CDR/IPDR/ANPR/subscriber/tower/finance/log correlations and query
   goldens over messy schema/timezone/duplicate cases.
2. Per-sheet typed mapping is source-complete; now add multi-provider schema
   drift goldens, then evaluate ODS and deeper columnar formats only against
   versioned fixtures and bounded resource limits.
3. Validate the LocalAI wrapper on CI/high-memory hardware.
4. Only after separate approval: back up, apply migrations 008/009, rebuild and
   activate the Phase 3 runtime.
5. Only after separate model-download approval: benchmark one OCR or STT
   candidate at a time; reject or promote from measurements, never from a demo.

Detailed evidence:

- `reports/forensic-phase4a-capability-tsv-acceptance-20260729.md`
- `reports/forensic-model-candidate-shortlist-20260729.md`

## 2026-07-29 later update — Phase 4C exact correlation

### Easy version to say

"I completed the next Phase 4 source slice. An analyst can now give the system a
phone number, vehicle plate, account, IP, device ID, identity, cell/site, or
other canonical identifier and ask it to correlate that target across CDR,
IPDR, ANPR, subscriber, tower, financial, access-log, and generic records. The
answer is based on exact database facts, shows which record families matched,
and cites the source file and row for the matches and related entities. It does
not guess from partial substrings, and it openly says when a displayed result or
relationship scan was capped. This is tested source code only: I did not rebuild,
migrate, deploy, download a model, or publish anything to GitHub."

### Technical proof

- New deterministic template: `cross_family_correlation`.
- Canonical source: `forensic.records`, scoped by tenant and collection, with
  optional date and record-family filters.
- Eight supported canonical families: CDR, IPDR, ANPR, subscriber,
  tower/location, transaction, access log, and generic.
- Exact case-normalized matching; phone digit normalization requires at least
  eight digits; mixed letter/digit IDs support bounded alphanumeric compaction;
  SQL substring matching is prohibited.
- Source locators carry available evidence/version/record IDs, filename, row,
  row hash, timestamp, and XLSX sheet/row coordinates.
- Expansion is bounded to eight targets, capped displayed matches, 1,000 scanned
  related occurrences, and five citations per related entity; truncation is
  explicit.
- Versioned synthetic Pakistan-oriented goldens pass 4/4 planner cases, 3/3
  relation aggregates, and 4/4 citation preservation.
- Six focused Phase 4C specs pass; the final full forensic API package passes in
  11.309 seconds; the focused agent-tools suite also passes.

### Next engineering slice

Phase 4D added conservative per-sheet typed XLSX mapping and the first UI
simplification. Phase 4E completes the v1 matrix with privacy-safe real CSV
auditing, typed IPDR, and multi-provider messy-data goldens for schema
drift, aliases, timezone ambiguity, late records, duplicates, money/currency
separation, and explicit rejects. Family-specific finance/log analytics then
build on the exact correlation foundation. Activation, model downloads,
sensitive-data ingest, and Git publication remain separate approval gates.

Detailed evidence:

- `reports/forensic-phase4c-cross-family-correlation-acceptance-20260729.md`

## 2026-07-29 latest update — Phase 4D mixed workbooks and simpler UI

### Easy version to say

> A messy Excel file no longer has to be treated as one giant table. NexusAI now
> checks every worksheet separately. A CDR sheet can become CDR records, an ANPR
> sheet can become vehicle sightings, a wallet sheet can become exact PKR
> transactions, and unrelated notes are still preserved instead of being lost.
> If a sheet is ambiguous, the system marks it for review rather than guessing.
> We also simplified the Records screen so the default task is just choose a case,
> ask a question, and analyze; advanced, evidence and data-management controls
> open only when needed. The source tests pass, but this version is not yet rebuilt
> into the running containers.

### Technical proof

- Upload jobs retain whether `record_type` was automatic or explicitly chosen.
- The worker records `per_sheet_schema_v1`, header fingerprints, matching
  adapters, mapping decision, review state, sheet identity, header row and exact
  source row on normalized records.
- Auto promotion requires exactly one typed adapter match. Zero or multiple
  matches remain generic. Explicit typed uploads validate each sheet and preserve
  non-matches generically instead of dropping them.
- Synthetic/de-identified Pakistan-oriented golden agreement is 4/4 across CDR,
  ANPR, PKR wallet transaction and Urdu/English case notes.
- Python combined regression: 43/43 passed. The final forensic Go package passed
  in 31.249 s after the capability/evaluation-matrix identifier was synchronized.
  Python compile, golden JSON, whitespace and production UI build passed.
- Static UI inspection verified the intended progressive disclosure. It was not
  a deployed API test, and the temporary preview service was stopped.

### When the rebuild will happen

The rebuild is intentionally after the source acceptance gate and before any
claim that the UI or Phase 3/4 runtime is live. The order is:

1. Run the remaining read-only activation audit, including the focused LocalAI
   wrapper check on CI/higher-memory capacity because this laptop previously
   exceeded the bounded linker window.
2. Take and hash a fresh scoped backup; verify migration 009 preflight.
3. Ask separately for approval to apply migrations 008/009 and provision the
   non-owner runtime role, required sidecar secret, and persistent NATS volume.
4. Ask for a bounded rebuild/recreate of forensic API, worker and LocalAI UI/proxy,
   with visible logs, a time limit, health checks, smoke tests and named rollback
   images. Do not repeat an unbounded 45-minute silent build.
5. Smoke success, retry, DLQ, crash/redelivery, tenant isolation, reprocessing,
   mixed XLSX mapping and the simplified UI before admitting real evidence.

### Practical depth from here

Phase 4E completes the first repeated vertical contract for every structured
family. Continuing compatibility work adds real-world provider layouts converted
to synthetic/de-identified goldens; aliases and schema drift; encodings; Pakistan
and declared foreign timezones; exact duplicates and near-duplicates; rejects;
canonical mapping; source locators; deterministic queries; UI results; latency,
RAM and row-accounting gates. The order is CDR/IPDR, ANPR, subscriber/identity,
tower/location, financial transaction, access/security logs, then generic
delimited/JSON/Parquet/XLSX and prioritized ODS/columnar formats.

The modality phases then implement the same complete vertical contract rather
than only registering a file:

- documents: native text/layout/tables first, then OCR and page/box citations;
- audio: metadata, preprocessing, STT, timestamps, diarization, language/noise
  tests, transcript indexing and TTS artifact lineage;
- images: metadata, OCR, plate/document text, object detection and bounding-box
  citations; face processing only with explicit policy/authorization;
- video: streams/keyframes/scenes/audio transcript, timeline citations and
  cross-modal search;
- captures/databases/archives: bounded protocol/session parsing, safe read-only
  schema/row adapters, archive child evidence and anti-bomb controls;
- intelligence: canonical entity resolution, confidence/alias policy, temporal
  relationship graph and exact cited path explanations;
- UI/operations: split Analyst, Evidence, Ingestion and Administration
  workspaces; accessibility, keyboard, responsive, empty/error/loading states,
  audit/lineage/model views, load/security/recovery and analyst acceptance tests.

Models are selected only after a fixed golden exists. Each candidate must beat
the deterministic/current baseline on the relevant accuracy, citation,
abstention, latency, RAM, throughput and security gates. Download, installation,
benchmark on sensitive data and promotion each remain explicit approvals; model
popularity alone is not an acceptance criterion.

Detailed evidence:

- `reports/forensic-phase4d-xlsx-sheet-mapping-ui-acceptance-20260729.md`

## 2026-07-29 final Phase 4 update — CSV proof and demo readiness

### Easy version to say

> CSV is fully included; Excel support did not replace it. I ran the supplied
> 923461678183 CSV read-only through the production CDR profiler and normalizer.
> It detected CDR automatically and accounted for all 3,931 rows: 3,931 accepted,
> zero rejected, 297 exact duplicates and 3,634 unique normalized rows. The audit
> did not upload the file or print raw rows or identifiers. I also completed the
> missing IPDR depth for IPv4/IPv6, NAT, ports, bytes, durations, domains and
> timezones, and added one safe demo command covering every typed structured
> family. Phase 4 structured-data v1 is now source-complete and regression green;
> deployment still requires the controlled migration and rebuild gate.

### Final source evidence

- Python worker/adapter/XLSX/IPDR/attached-CDR regression: 50/50 passed.
- Complete forensic Go API package: passed in 30.520 seconds.
- React production UI build: passed in 2.70 seconds.
- Safe demonstration: passed in 15 seconds.
- Real CSV full audit: 2.94 seconds after removing repeated timezone/header work;
  the same measured path previously needed approximately 35–55 seconds.
- Seven typed families have one versioned accounting matrix: CDR, IPDR, ANPR,
  transaction, subscriber, tower/location and access/security. Generic, TSV and
  per-sheet XLSX retain their own accepted goldens.
- No container rebuild, migration, database/queue write, model operation, real
  evidence upload or GitHub action occurred.

### Tomorrow’s demonstration

Follow `reports/nexusai-team-lead-demo-20260730.md`. The default command proves
the supplied CDR and synthetic family matrix without changing runtime state. If
the new images are not activated before the meeting, explicitly say that the
UI/Phase 3/4 runtime remains source-tested and use the offline demonstration;
do not present the older deployed UI as the new source UI.

### Next gate

Run activation readiness and the higher-memory LocalAI wrapper validation, then
request separate approval for a fresh backup, migrations 008/009, non-owner role,
sidecar authentication, persistent NATS, and a bounded API/worker/LocalAI rebuild
with visible logs and rollback tags. After live smoke, Phase 5 moves to native
documents and OCR; provider-specific Phase 4 aliases remain additive regression
packs rather than reopening the stable v1 contract.

Detailed evidence:

- `reports/forensic-phase4e-csv-ipdr-demo-acceptance-20260729.md`
- `reports/nexusai-team-lead-demo-20260730.md`

## 2026-07-29 Phase 3 retained database completion

### Easy version to say

> I completed the database foundation for crash-safe evidence processing. I made
> and fully verified separate recovery backups, activated the evidence version,
> source, processing and custody control plane, preserved seven old failed jobs as
> dead-letter history instead of deleting or calling them successful, and then
> activated the durable queue lifecycle. The final database has 41 stable jobs:
> 34 succeeded and 7 preserved dead-letter failures, with zero active blockers or
> worker leases. Both official migration verifiers pass. The API and worker are
> intentionally still stopped until the separate security/build/live-smoke gate.

### Technical proof

- Migration 008 and 009 both committed transactionally and passed their official
  retained verification.
- Exact control-plane totals: 110 evidence/storage/version rows, 214 source links,
  41 runs, 48 processing events, 116 custody events and zero derived artifacts.
- Queue totals: 41/41 stable message IDs; 34 completed/succeeded; 7 dead-letter;
  zero queued/running/failed blockers; zero leases; three constraints and one
  history-protection trigger.
- Recovery preserved original IDs, diagnostics, attempts and timestamps; appended
  seven audit rows, seven processing transitions and six custody corrections.
- Three independent recovery points exist: pre-008, post-008/pre-recovery and
  post-009, all round-trip hash/catalog/full-read verified.
- No API/worker/LocalAI rebuild, model change, sensitive ingestion or GitHub action
  occurred.

Detailed evidence:

- `reports/forensic-phase3-retained-queue-activation-completion-20260729.md`
