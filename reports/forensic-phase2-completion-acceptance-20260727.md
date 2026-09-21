# NexusAI Phase 2 Completion Acceptance

Date: 2026-07-27  
Host: Windows laptop, Intel Core i7-1260P, 15.71 GiB RAM, CPU-only LocalAI  
Final acceptance collection: `forensic-phase2-complete-acceptance-20260727`  
Protected comparison collection: `records-demo-verified`

## Executive result

Phase 2A and Phase 2B are complete at their documented boundary. All 20 required
evidence-family profiles have real, versioned, machine-verifiable fixture
contracts. The isolated live collection has 20 evidence items, 20 KB assets, 20
KB entries, no failed evidence/jobs, and no missing links. Eight supported
structured sources account exactly for 59 input rows: 34 unique accepted, 6 exact
duplicates, and 19 visible rejects. Twelve deeper formats remain truthfully
registered on pending/manual routes; fixture readiness is not represented as
completed OCR, STT, document extraction, video understanding, packet sessions,
archive extraction, or database querying.

The protected 9,250-row collection is unchanged at the structured-record layer.
After a verified scoped backup, its four historical KB-only entries were closed
into two content-addressed evidence objects while preserving all four source
links. It now reconciles to eight KB entries, six evidence objects, and eight KB
asset/source links, with zero missing assets or failures.

## 1. Objective

Complete every remaining Phase 2 slice: Pakistan subscriber/tower/security-log
goldens; fixtures for text, tabular/spreadsheets, documents, images, audio/STT,
TTS, transcripts, video, captures, databases, archives, and unknown data;
unchanged-model evaluation; cross-family exact queries; live isolated acceptance;
registry closure; rollback; and durable handoff documentation.

## 2. Assumptions verified

- The supplied real CDR was used only as a read-only format reference; it was not
  ingested into acceptance or committed as a fixture.
- Existing dirty-worktree changes and Docker volumes were preserved.
- The current installed model pair remained unchanged.
- `records-demo-verified` began with 9,250 accepted structured rows, four evidence
  objects, four KB asset links, and eight KB entries.
- Pakistan localization is evidence-oriented and conservative. Official sources
  support retaining original traffic data and preservation orders under PECA,
  and formal event recording/evidence preservation under the PTA NTCERT telecom
  cybersecurity framework; this implementation does not claim legal compliance
  certification. Sources: [Pakistan Code PECA PDF](https://www.pakistancode.gov.pk/pdffiles/administrator6a061efe0ed5bd153fa8b79b8eb4cba7.pdf),
  [PTA NTCERT framework](https://ntcert.pta.gov.pk/sops/national_cs_framework_for_telecom_07-07-2022.pdf).

## 3. Files and artifacts changed

- Expanded `ingestion/forensic_records/worker.py` adapters for Pakistan
  subscriber/identity, tower/sector, and access/security-log variants.
- Expanded API classifier aliases and deterministic query behavior in
  `api/forensic_records/`.
- Added Pakistan v2 goldens and three new messy synthetic structured fixtures.
- Added the 12-case deterministic remaining-modality manifest at
  `tests/fixtures/forensic_modalities/phase2_remaining_goldens_v1.json`.
- Closed the 20-profile matrix in
  `configuration/forensic_modality_evaluation_matrix.json`.
- Updated the Pakistan profile to schema `1.2.0`.
- Extended `scripts/benchmark_forensic_models.py` to six chat tasks, eight
  embedding inputs, nine deterministic routes per round, matrix hashing, and
  fail-closed route scoring.
- Added the final JSON baseline, this report, product docs, team-lead brief, and
  continuation ledger updates.

## 4. Schema, API, and configuration impact

- No database migration, new dependency, endpoint, auth registry, queue subject,
  or LocalAI configuration change was required.
- Derived normalized values remain in existing JSON metadata while immutable raw
  rows and row hashes remain intact.
- `tower_activity` now queries both `cdr` and `tower_location` rows from the
  canonical store instead of silently using a CDR-only query.
- Schema profiling now treats absent/scalar JSON header metadata as zero headers,
  preventing a PostgreSQL `jsonb_array_length` failure on valid text evidence.
- Matrix schema is `2026-07-27-phase2-complete`; 20/20 fixtures are `ready`.

## 5. Tests and exact results

- Python worker/adapters: 24/24 passed in 0.669 seconds.
- Go forensic API: 97/97 Ginkgo specs plus legacy tests passed; final package
  rerun recorded in the continuation ledger.
- `go vet ./api/forensic_records`: passed.
- Python compilation: worker and benchmark runner passed.
- JSON validation: modality matrix, Pakistan profile, Pakistan v2 golden manifest,
  and remaining-modality manifest passed.
- Docker Compose `config --quiet`: passed.
- `git diff --check`: passed; only pre-existing line-ending warnings were emitted.

## 6. Live runtime checks

- LocalAI `/readyz`: HTTP 200.
- Forensic API `/healthz`: `ok`.
- NATS `/healthz`: HTTP 200.
- Worker `/metrics`: HTTP 200.
- Deployed forensic API: `sha256:2fc0425044baeda5339cc4f06b8e242ef4a5734c70254e3d58861418dcec9473`.
- Deployed worker: `sha256:b0b66c87d11c556bcdee1a184806c270c0a393b57280436778fc2b7f4222e233`.
- LocalAI stayed on `sha256:0731ab05fa080fae9bc90ddbcc26f74fcb554389ee82aa0282e5373a27144c9b`.
- Only the forensic API/worker received bounded targeted builds; LocalAI,
  PostgreSQL, NATS, and named data volumes were not recreated for Phase 2 closure.

## 7. Dataset and evidence accounting

### Final 20-source collection

| Measure | Result |
| --- | ---: |
| Evidence / KB assets / KB entries | 20 / 20 / 20 |
| Structured jobs completed / failed | 8 / 0 |
| Source rows | 59 |
| Unique accepted / duplicates / rejects | 34 / 6 / 19 |
| Evidence failures / missing links | 0 / 0 |
| Truthfully registered pending/manual items | 12 |

Structured family accounting:

| Family | Input | Accepted | Duplicate | Rejected |
| --- | ---: | ---: | ---: | ---: |
| CDR | 4 | 4 | 0 | 0 |
| IPDR | 2 | 2 | 0 | 0 |
| ANPR | 10 | 5 | 1 | 4 |
| Subscriber | 9 | 5 | 1 | 3 |
| Tower/location | 10 | 5 | 1 | 4 |
| Financial transaction | 11 | 6 | 1 | 4 |
| Access/security log | 10 | 5 | 1 | 4 |
| Generic tabular | 3 | 2 | 1 | 0 |

### Protected comparison collection

- Structured rows: 9,250 accepted, zero duplicate/rejected, four completed jobs.
- Registry after closure: eight KB entries, six evidence objects, eight source
  links, zero missing assets and zero failed evidence/jobs.
- Each historical text hash maps to one evidence object and two distinct KB links.

## 8. Models, revisions, and parameters

- Chat: `qwen_qwen3-4b-instruct-2507`, temperature 0, maximum 256 output tokens.
- Embeddings: `qwen3-embedding-0.6b`, 1,024 dimensions.
- Profile: CPU; three rounds; no model download, replacement, quantization change,
  reranker activation, or candidate promotion.
- Final report: `reports/forensic-phase2-unchanged-baseline-20260727.json`.

## 9. Accuracy, routing, retrieval, and abstention

- Fixture closure: 20/20 ready, matrix SHA-256
  `b407f8dc6d3ed0b22beb8cd1146d0f45bba3a3d0fb77f0c914c3c4484b4bd3f0`;
  the report-embedded digest matches the file.
- Structured goldens matched exact accepted/duplicate/rejected counts and
  normalized-field expectations.
- Chat guardrails: 18/18 successful; required-term agreement 1.0; lexical
  unsupported-claim rate 0.0. These are lexical guardrail signals, not a substitute
  for human forensic correctness review.
- Deterministic route agreement: 27/27, zero HTTP or template failures, score 1.0.
- Embeddings: three successful runs, eight vectors per run, 1,024 dimensions.
- Live cross-family exact queries: subscriber profile 2 rows, relationship network
  13 rows, entity timeline 5 rows, and tower activity 1 cited tower-sector row.
- Media and binary fixtures retained deterministic technical metadata while
  correctly abstaining from unsupported higher-level extraction.

## 10. Latency and memory snapshots

- Chat: average 13,663.93 ms; p50 10,672.12 ms; p95 20,274.93 ms.
- Embeddings: average 2,185.99 ms; p95 2,584.72 ms.
- Deterministic sidecar routes: average 191.71 ms; p50 from the JSON report;
  p95 1,102.20 ms.
- Final free host RAM snapshot: 1.38 GiB.
- Container memory snapshots: forensic API 13.53 MiB; worker 56.57 MiB;
  PostgreSQL 133.7 MiB; NATS 17.47 MiB; LocalAI 4.637 GiB.
- These are single-host acceptance diagnostics, not concurrency/load-test SLAs.

## 11. Failures, fallbacks, and unresolved boundaries

- Final acceptance found and fixed a CDR-only `tower_activity` implementation.
- Registry closure exposed and fixed schema-profile JSON-array assumptions; it
  had produced six HTTP 500 checks across three benchmark rounds.
- Benchmark scoring previously excluded HTTP failures from its route denominator;
  scoring now fails closed. The corrected rerun is 27/27 with zero errors.
- The initial clean Go cache compile took 266.7 seconds but passed; subsequent
  package runs used the workspace cache.
- Pending by design: OCR/text-layout extraction, STT/diarization, deep video,
  packet/session/protocol decoding, archive child extraction, database query
  adapters, and unknown-file automation.

## 12. Security, tenant isolation, and provenance

- All acceptance fixtures are synthetic; the real attached CDR was not ingested.
- Raw values, source row numbers/hashes, tenant, collection, case, evidence,
  version, source-entry, KB, rejection, and audit lineage remain preserved.
- Pakistan phone normalization only emits `+92` when unambiguous; CNIC, IMSI,
  IMEI, plate, IBAN, coordinates, timezone, currency, and log fields retain raw
  values and explicit validity/review flags rather than inferred identity facts.
- Unsupported binaries never enter the structured worker.
- Phase 3 must still bind caller tenant scope to authenticated identity, review
  RLS, finalize immutable retention, and verify JetStream ack/retry/DLQ behavior.

## 13. Git state

All work remains in the existing dirty worktree. Nothing was staged, committed,
pushed, submitted as a pull request, or published to GitHub.

## 14. Rollback and recovery points

- Pre-closure database dump:
  `.phase2-backups/records-demo-verified-registry-preclosure-20260727.dump`.
- Dump format: PostgreSQL custom; verified with `pg_restore -l`; 102,065 bytes;
  SHA-256 `8194527272738f4feeb0440217dc989d043d9506708dfd04138218b5cb2795ff`.
- API rollback:
  `nexusai-forensic-records-api:rollback-20260727-pk-structured-v1.1` at
  `sha256:39a1bc8d9af8f3bb43352ac75ac36dff26281932abbe34856d302328f39d4869`.
- Worker rollback:
  `nexusai-forensic-records-worker:rollback-20260727-pk-structured-v1.1` at
  `sha256:c2e7fe083355f2bd1a031ccd24c44883dbb9a25253a553ecc079b0703eee73c9`.
- No acceptance collection or audit history was deleted.

## 15. Next action and approval required

Proceed to Phase 3 evidence control plane design and source/test work: normalized
evidence versions and processing runs, typed derived artifacts, append-only custody
events, immutable source retention, authenticated tenant/case scope, reviewed RLS,
and idempotent JetStream acknowledgement/retry/DLQ behavior.

Separate explicit approval remains required before applying the Phase 3 database
migration, ingesting real sensitive evidence, downloading or activating a model,
running another full LocalAI build, enabling policy-sensitive identity features,
restoring/replacing database state, or staging/committing/pushing/publishing.
