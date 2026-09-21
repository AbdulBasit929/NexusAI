# NexusAI Pakistan Data Readiness Checkpoint

Date: 2026-07-24  
Scope: private profiling of one real-world CDR export, Pakistan localization, synthetic regression coverage, and neutral collection naming  
Real-data handling: read-only local analysis; no upload, database ingest, repository copy, external transfer, or raw-value disclosure

## Executive result

The supplied CDR is structurally compatible with the CDR adapter, but it proves
that carrier evidence cannot be validated with one clean-schema assumption. The
implementation now has a versioned Pakistan profile, a source-timezone contract,
and a synthetic fixture derived from observed shapes rather than real values.

The live test collections were also migrated to operational names:

| Previous name | Current name | KB entries | Forensic evidence |
| --- | --- | ---: | ---: |
| development audit collection | `forensic-ingress-audit-20260724` | 39 | 27 |
| development acceptance collection | `forensic-phase2a-acceptance-20260724` | 21 | 21 |

Every copied file was verified by SHA-256 before the old collection was retired.
All writable forensic tables, KB source-entry references, and collection metadata
were reconciled transactionally. The database has zero references to the previous
development names.

## Private CDR structural profile

Source SHA-256:
`3eee8cb613d2dee9a5ee10600ec361ad2537c0b95d5601fe8d5c8d135a39c8f5`

Only aggregate structure is recorded here:

| Measure | Result |
| --- | ---: |
| Bytes | 828,259 |
| Columns | 16 |
| Data rows | 3,931 |
| Exact duplicate rows | 297 |
| Malformed-width rows | 0 |
| Timestamp parse failures | 0 |
| Negative durations | 0 |
| Zero-duration rows | 3,751 |
| Positive-duration rows | 180 |
| Rows with blank tower/location coordinates | 46 |
| Parseable coordinate pairs | 3,885 |
| Coordinate pairs inside coarse Pakistan screening bounds | 3,885 |
| Negative cell-ID sentinel candidates | 45 |

Identifier observations:

- MSISDN values use a 12-digit Pakistan international form without `+`.
- IMSIs are 15 digits and carry MCC 410.
- IMEIs are represented as 14 digits, so a strict 15-digit-only rule would lose
  valid provider evidence.
- Originator and dialed fields mix full numbers, short codes, USSD, provider or
  service labels, blanks, and other provider-defined tokens.
- LAC and site identifiers are exported with spreadsheet-style `.0` suffixes.
- Directions include voice directions and a data category; call types include
  CALL, SMS, GPRS, and VOLTE.

These are format observations, not subscriber findings. No real identifier,
location, date range, contact relationship, or content value is reproduced.

## Implemented engineering changes

1. Added `configuration/forensic_country_profiles/pakistan.json` as a versioned,
   cited deployment profile covering Pakistan numbering, IMSI/IMEI, CNIC, PK
   IBAN, province-specific ANPR, local languages/scripts, security, and each
   evidence family.
2. Added upload metadata fields `jurisdiction` and `source_timezone`.
3. Set Pakistan deployment defaults to `PK` and `Asia/Karachi` in Compose.
4. Updated CDR and generic timestamp parsing so an explicit timestamp offset
   wins; a naïve timestamp uses the declared source timezone and is converted to
   UTC for canonical storage.
5. Added the Python `tzdata` runtime dependency so IANA timezone behavior is
   consistent on slim Linux images and Windows development systems.
6. Added a synthetic UTF-8 CDR golden containing Urdu text, quoted commas,
   service labels, USSD, a short code, a 14-digit IMEI, blank tower fields, a
   negative cell sentinel, and an exact duplicate.
7. Kept raw records immutable while retaining existing safe normalization of
   spreadsheet-generated numeric suffixes.

Focused verification after the final changes:

- Python worker and adapter suites: 17 tests passed in 0.128 seconds.
- Go forensic API package: passed; reported package execution time 15.793 seconds
  after the workspace cache was populated.
- LocalAI forwarding package: two bounded five-minute Windows cold-compile
  attempts produced no compiler/test failure but did not finish before timeout.
  The generated protobuf package is present, so this is not the earlier
  missing-protobuf setup failure. The source-level forwarding regression includes
  both `jurisdiction` and `source_timezone`; run it in CI or a less constrained
  build environment before merge.
- Compose configuration: syntactically valid with Pakistan defaults.
- Live services: LocalAI healthy and forensic API `healthz` returned `status=ok`
  after the collection migration.

## Pakistan authority baseline

The profile uses authoritative sources first and records a verification date:

- ITU-T E.164 and national numbering-plan resources for telephone structure.
- ITU-T E.212 for international mobile subscriber identity structure.
- NADRA for CNIC identity-document context.
- State Bank of Pakistan IBAN guidance for the 24-character PK IBAN structure
  and MOD 97-10 validation.
- Provincial excise sources for plate examples and series. ANPR is deliberately
  province-, vehicle-class-, series-, and time-specific rather than one regex.
- Pakistan Code and PTA/NTCERT material for traffic-data, telecom security,
  chain-of-custody, protection, localization, and disposal controls.
- The MoITT personal-data bill is marked as a draft reference, not represented
  as enacted law.

The source registry is reviewed every 90 days and on any regulatory, numbering,
plate-series, provider-export, language, or failed-golden trigger. Provider
documents and lawfully authorized local samples remain necessary: no online list
can enumerate every historic, proprietary, or malformed export.

## In-depth next phases

### Phase 2B.1 — deterministic metadata foundation

Build bounded, model-free extractors before downloading OCR/STT/vision models:

- images: MIME, dimensions, EXIF, cryptographic/perceptual hashes, orientation;
- audio/TTS: container, codec, duration, sample rate, channels, loudness/noise;
- video: container, streams, duration, frame rate, keyframe inventory;
- PDF/Office: page/sheet/slide counts, embedded-object inventory, macro flags;
- archives: safe member inventory, member hashes, size/ratio limits, traversal
  blocking, no executable extraction;
- SQLite/databases: read-only schema/table/row-count inventory and type samples;
- PCAP/PCAPNG: capture metadata and deterministic flow/session summaries.

Acceptance: fixed synthetic and legally shareable goldens, 100% source hashes and
provenance, explicit partial failures, no silent member loss, bounded CPU/RAM,
and rollback to registration-only routing.

### Phase 2B.2 — Pakistan structured-data adapters

Expand CDR, IPDR, ANPR, subscriber, tower, financial, and log goldens across
CSV/TSV/JSON/JSONL/Parquet/spreadsheet exports, encodings, quoting, reordered and
unknown columns, provider sentinels, late events, duplicates, and clock drift.

- CDR/IPDR: short/service identifiers, IMSI MNC ambiguity, 14/15/16-digit device
  identifiers, GPRS/VoLTE/data semantics, cell/tower uncertainty, NAT and IPv6.
- ANPR: Punjab, Sindh, KP, Balochistan, ICT, GB, and AJK series; Urdu/English;
  motorcycles, commercial/government/diplomatic, premium and legacy plates.
- Subscriber: CNIC and phone canonicalization under restricted-field controls,
  history/effective dates, duplicates and conflicting provider records.
- Financial: PKR/paisa, PK IBAN checksum, masked accounts, reversals, chargebacks,
  value versus posting time, double-entry reconciliation, and fraud indicators.
- Logs: syslog, web, Windows/Linux events, firewall/VPN/DNS/DHCP/cloud exports,
  clock source, IPv4/IPv6, user/device/service identities and parser confidence.

Acceptance: row preservation equals 100% when accepted plus explicitly rejected
rows are counted; schema-mapping goldens pass; no raw-field loss; duplicate and
timezone results match deterministic expectations.

### Phase 2C — Urdu/English document and image intelligence

Introduce OCR only after deterministic metadata gates pass. Benchmark printed
Urdu, English, mixed script, Roman Urdu, low-resolution scans, skew, stamps,
tables, handwriting abstention, and province-specific plates. Preserve page/image
coordinates and create redacted derivatives without replacing source evidence.

Acceptance: per-language character/word error rates, plate exact-match and
province accuracy, confidence calibration, manual-review thresholds, citation to
page/region, model revision, and CPU/RAM/latency measurements.

### Phase 2D — audio, STT, diarization, and TTS

Benchmark Urdu, Pakistani English, Roman Urdu/code-switching, Punjabi, Sindhi,
Pashto, Balochi, and Saraiki coverage as datasets become lawfully available.
Separate transcription, diarization, language ID, translation, and TTS so each
derivative has its own evidence ID, model revision, timestamps, and parent hash.

Acceptance: WER by language/noise/channel, diarization error, timestamp p95,
named-entity preservation, hallucination/abstention checks, and audible human
review for TTS. No model becomes default from an English-only benchmark.

### Phase 2E — entity resolution and temporal graph

Create typed, uncertainty-aware entities for people, phones, IMSI/IMEI, accounts,
vehicles, IPs/domains, towers, places, cameras, files and cases. Joins must retain
source, effective interval, confidence, contradiction, and analyst decisions.

Acceptance: precision/recall by link type, zero unsupported deterministic claims,
reversible merges, conflicting-source display, and exact timeline reconstruction.

### Phase 2F — retrieval, reasoning, reports, and analyst UI

Use SQL/typed stores for exact counts and graph facts; use semantic retrieval for
narrative evidence. Reports must cite evidence IDs and source locations. The React
UI will expose ingest status, schema mapping, Pakistan profile/version, timezone
assumptions, duplicates, rejections, OCR/STT confidence, lineage and audit.

Acceptance: exact-answer agreement, retrieval recall@5, citation correctness,
unsupported-claim rate, abstention accuracy, tenant isolation, accessibility,
and redaction/export controls.

### Phase 2G — scale, security, and release

Run volume/concurrency benchmarks, fault injection, backup/restore, tenant escape
tests, malicious archive/document tests, model-supply-chain verification, audit
retention, data-localization deployment checks, and disaster recovery. Promote in
small reversible slices with immutable source evidence and previous image/config
rollback points.

## Team-lead handoff

Current decision: proceed with Phase 2B.1 deterministic metadata extractors while
continuing Pakistan structured goldens in parallel at fixture level. Do not import
the attached real CDR until the owner explicitly authorizes that sensitive live
database write and supplies case/retention/access metadata. Do not start model
downloads or a bulk database backfill without their separate approval gates.

Git state: changes remain unstaged and uncommitted; nothing was pushed or
published. Production comparison collection `records-demo-verified` remains at
9,250 structured rows. Rollback images are retained. The live rename affected
only the two isolated development collections and their matching forensic
references.
