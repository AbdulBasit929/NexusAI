# Forensic Records Ingestion Worker

Python 3.11 worker for Phase 1/2 Knowledge Base structured-record ingestion.

Key properties:

- Durable JetStream explicit-ack consumer backed by a file work queue.
- Fails startup unless Phase 3 queue migration 009 is present.
- Claims a PostgreSQL worker lease with row locking and increments attempts only
  after a successful claim.
- Validates content-addressed scope, receipt, size, read-only state and full
  SHA-256 before ingest.
- Uses bounded source-text profiling for delimited data so identifiers, money
  precision and leading zeros are not coerced during schema discovery; Polars is
  retained for Parquet.
- Routes files through typed adapters: CDR, IPDR, ANPR, subscriber,
  tower/location, transaction, access log, and generic delimited/JSON/Parquet/XLSX.
- Resolves adapters and dispatches row normalization through the versioned
  platform registry in `forensic_contracts.py`. Phase 6.1 extracts CDR aliases,
  validation, canonical construction, verification, capabilities, and its
  manifest into `adapters/cdr/`; Phase 6.2 does the same for IPDR/network
  sessions in `adapters/ipdr/`; and Phase 6.3 promotes ANPR/vehicle observations
  through `adapters/anpr/`. The worker injects its accepted timezone,
  scalar-parsing, and row-hash hooks so persistence, accounting, and normalized
  output remain unchanged. Generic and the remaining structured families stay
  on their accepted compatibility paths until their bounded slices are promoted.
- Streams CSV and TSV rows into PostgreSQL through `COPY`, including UTF-8,
  UTF-16 and CP1252 fallback, detected delimiters, duplicate/blank header
  disambiguation and explicit overflow columns.
- Reads `.xlsx` OOXML directly with bounded ZIP/XML limits. It inventories all
  sheets, streams rows with sheet/row provenance, retains hidden-state and
  per-cell raw/style/formula/error metadata, distinguishes cached formula values
  from formula text, respects the 1900/1904 date systems, and inventories merged
  ranges. It never recalculates formulas, follows external links, or executes
  macros; macro-bearing, encrypted, unsafe-path and DTD/entity inputs fail closed.
- Classifies each non-empty XLSX sheet independently. In `auto` mode, a sheet is
  promoted to CDR, IPDR, ANPR, subscriber, tower/location, transaction, or
  access-log only when exactly one typed adapter matches its own headers;
  ambiguous and unrelated sheets remain generic with review metadata. An
  explicit typed request validates each sheet independently and preserves
  non-matching sheets generically instead of dropping or coercing them.
- Inserts CDR into a TimescaleDB hypertable.
- Inserts non-CDR structured files into `forensic.generic_records`.
- Indexes normalized entities in `forensic.record_entities` for cross-record correlation.
- Materializes `forensic.kb_active_metadata`.
- Materializes `forensic.kb_collection_assets` so KB remains the analyst-facing feature.
- Exposes Prometheus metrics on port `9109`.

The worker commits database results before `AckSync`. A lost acknowledgement is
safe: redelivery observes the completed job and only acknowledges. Transient
errors store a bounded retry time and use delayed NAK; permanent input failures
or exhausted attempts publish an idempotent diagnostic envelope to
`FORENSIC_RECORDS_DLQ` before terminating the source message. Earlier results
are immutable; explicit reprocessing creates a linked new job. Full lifecycle
and failure contracts are documented in
`docs/design/forensic-queue-lifecycle.md`.

Validate the attached sample profile without starting services:

```powershell
python ingestion/forensic_records/worker.py validate-sample "C:\Users\W S Mughal\Downloads\923461678183.csv"
```

Run the standard-library test:

```powershell
python -m unittest ingestion.forensic_records.tests.test_attached_cdr_sample
```

Run the worker, adapter, XLSX mapping, and Phase 5A contract regression set:

```powershell
python -m unittest ingestion.forensic_records.tests.test_worker ingestion.forensic_records.tests.test_phase2_adapters ingestion.forensic_records.tests.test_phase4_xlsx ingestion.forensic_records.tests.test_phase5a_contracts
```

Run the dedicated Phase 6.1 CDR manifest, Pakistan-golden, and adversarial
verification tests:

```powershell
python -m unittest ingestion.forensic_records.tests.test_phase6_cdr_adapter
```

Run the dedicated Phase 6.2 IPDR manifest, IPv4/IPv6/NAT/timezone golden, and
adversarial verification tests plus the read-only normalization benchmark:

```powershell
python -m unittest ingestion.forensic_records.tests.test_phase6_ipdr_adapter
python scripts/benchmark_phase6_ipdr.py --iterations 2000
```

Run the dedicated Phase 6.3 ANPR manifest, Pakistan plate/camera/timezone,
confidence, crop-lineage, duplicate-accounting, and adversarial verification
tests plus its read-only normalization benchmark:

```powershell
python -m unittest ingestion.forensic_records.tests.test_phase6_anpr_adapter
python scripts/benchmark_phase6_anpr.py --iterations 2000
```

The ANPR adapter preserves source plate text and evidence locators while adding
exact search keys, validated UTC observation time, explicitly supplied camera,
location, coordinates and confidence, and crop SHA-256 when present. It does not
infer ownership, occupants, road routes, OCR results, clock correction, or a
province that the source did not supply.

The unified media worker also has bounded, opt-in retained processors:

- images emit technical metadata and deterministic `dhash-64-ffmpeg-area-v1`
  fingerprints; FastALPR adds cited plate observations only when explicitly
  enabled with readable local model paths;
- Urdu ASR segments retain raw Urdu as authoritative and may add an
  identifier-safe `ur-Latn` Roman-Urdu derivative;
- video-embedded audio preserves the underlying timestamp/Roman-Urdu contract
  and parent evidence lineage; and
- TXT, DOCX and text-bearing PDF evidence emits bounded native-text passages
  with section, paragraph or page locators. Scanned PDFs are reported as
  native-text unavailable and never trigger implicit OCR.

These processors do not download models, rewrite sources, infer biometric
identity, perform general image OCR, or bypass tenant/case authorization.

Audit a real structured source without uploading it or printing raw rows and
identifiers:

```powershell
python ingestion/forensic_records/worker.py audit-source "C:\path\to\calls.csv" --record-type auto --source-timezone Asia/Karachi --jurisdiction PK
```

The audit uses the production profiler and normalizers and reports source hash,
format/dialect, adapter decision, total/accepted/rejected/duplicate/overflow row
accounting, rejection categories, UTC time range, and privacy assertions. It does
not write to PostgreSQL, NATS, the Knowledge Base, or the source file.
