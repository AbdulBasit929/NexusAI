# NexusAI Phase 7.1 Runtime Hardening and Platform Audit

Date: 2026-08-04  
Environment: Windows 11, Docker Desktop/WSL2, CPU-only, approximately 16 GiB RAM  
Scope: operator activation reconciliation, live Records and Agent Chat acceptance,
Pakistan subscriber-query hardening, Knowledge Base collection governance, build
context control, privacy regression closure, full platform status, and next-phase
sequencing.

## Executive verdict

The operator rebuild succeeded and activated the five-specialist Phase 7.1
runtime. The runtime is healthy and the Subscriber Identity specialist is
operational. The governed `records-demo-verified` case still contains zero
subscriber rows, so the slice is **runtime-activated with a verified no-data
path, but not data-accepted**. No real or synthetic subscriber data was inserted
in this audit.

Live acceptance uncovered two deployment-quality defects and one privacy gap:

1. creating a forensic specialist also created agent-name Knowledge Base
   collections instead of using only its configured governed case;
2. the root Docker context transferred 1.25 GB because local Codex caches were
   not ignored;
3. generic cross-family correlation could return a full CNIC even though the
   subscriber-specific operations masked it.

All three corrections and the governed English, Roman Urdu, and Urdu routing
delta are now deployed. The post-refresh gate confirmed that the root Docker
context fell from 1.25 GB to 128.95 MB, the collection catalog remained at 28,
and the raw Roman Urdu query now clarifies rather than returning unrelated rows.
Phase 7.1 remains data-pending only because the governed case contains no
subscriber rows.

The later safe source-only preparation added a clearly synthetic populated
fixture and closed raw/typed target echo plus generic/canonical export privacy
gaps. Those changes pass focused and full forensic regressions but are not
deployed and no fixture was uploaded. See
`reports/nexusai-phase7.1-populated-subscriber-source-readiness-20260804.md`.

## Operator activation evidence

The supplied PowerShell transcript records:

- `Phase6Activation=PASS`;
- API build: 13.9 seconds;
- worker build: 5 seconds;
- LocalAI build: 735.1 seconds, first attempt;
- free physical RAM at build gate: 6.06 GiB;
- rollback images and named volumes preserved;
- five profiles applied to `records-demo-verified` / `default`;
- approved chat model: `qwen_qwen3-4b-instruct-2507`;
- subscriber profile created as version `1.1.0`;
- live agent count: five.

The later hardening activation transcript records:

- `Phase6Activation=PASS`;
- API build: 19.8 seconds;
- worker build: 4.1 seconds;
- LocalAI build: 375.4 seconds, first attempt;
- free physical RAM at build gate: 6.77 GiB;
- root LocalAI context: 128.95 MB;
- rollback images preserved and named volumes preserved.

Read-only live reconciliation confirmed:

| Surface | Result |
| --- | --- |
| LocalAI | HTTP 200 |
| Forensic API | HTTP 200 |
| Worker metrics | HTTP 200 |
| NATS monitoring | HTTP 200 |
| Installed models | Qwen 4B chat; Qwen 0.6B embedding |
| Specialist agents | five active |
| Adapter catalog | five adapters |
| Operation catalog | 42 operations |
| Governed accepted rows | 9,250 |
| Subscriber rows | 0 |

The 9,250 accepted rows remain CDR 5,000, IPDR 2,500, access/security 1,000,
and ANPR 750. A registered family or active specialist does not imply populated
data.

## Live analyst acceptance

### Records workspace

`show subscriber status summary` returned
`forensics.enterprise-response/v1` with:

- status `no_results`;
- deterministic route `records_sql`;
- operation `subscriber.status_summary`;
- adapter `nexusai.adapter.subscriber_identity`;
- zero model roles and zero model latency;
- two deterministic findings;
- one canonical-table source reference;
- explicit CNIC/name, ownership, and current-control limitations.

The UI rendered a compact `Subscriber Status Summary` result with a bounded
negative answer, exact-row/source/finding/latency metrics, export audit control,
recommended next checks, and collapsed evidence cautions. At a 1280 px viewport
the document width was 1270 px, so page overflow was zero. Browser diagnostics
were empty.

### Subscriber Agent Chat

An SSE-backed case-scoped chat against `Subscriber_Identity_Analyst` completed
in the same second. It called `forensic_hybrid_query` with
`subscriber_status_summary`, returned the same bounded no-data finding, exposed
the deterministic operating metrics and limitations, and reached `completed`.
No LLM was invoked.

### Live multilingual regression closure

The earlier deployed raw Roman Urdu question `subscriber ki maloomat dikhao` did not
select a subscriber operation and returned 20 unrelated generic structured
rows. This is unacceptable because a raw-language miss must clarify rather
than silently execute an unrelated calculation. After the hardening refresh,
the same question returns `needs_input`, selects
`subscriber.identity_lookup`, and executes no SQL or model. An exact Roman Urdu
lookup for `0300-1234567` normalizes to `03001234567` and returns a bounded
`no_results` response with `subscriber.identity_lookup` and zero model roles.

The Urdu aggregate `فعال اور معطل سبسکرائبرز کی حیثیت دکھائیں`, sent as
explicit UTF-8 bytes, selects `subscriber.status_summary` and returns the
governed bounded no-data contract with zero model use. Windows PowerShell 5.1
callers must not rely on an implicitly encoded string request body for Urdu;
tests must send UTF-8 bytes or use a client with an explicit UTF-8 charset.

## Source hardening deployed and verified

### 1. Knowledge Base collection governance

Native forensic agent create, update, and import paths now resolve their
knowledge collection as follows:

- forensic agent with `forensic_collection_id`: use that governed case;
- generic LocalAGI agent: retain the historical agent-name collection;
- unnormalized forensic fallback: retain the agent name until validation.

This prevents future forensic profiles from creating duplicate collections
while preserving generic-agent compatibility. The two subscriber collections
already created by the live bootstrap were not deleted. They remain
manifest-backed cleanup candidates pending explicit operator approval.

### 2. Root build-context control

`.dockerignore` now excludes `.cache`, `.codex-cache`, `.codex-tmp`, `.codex`,
browser reports, coverage, and other host-only artifacts. `.gitignore` also
keeps the local cache directories out of status noise.

Measured local contributors behind the prior 1.25 GB transfer included about
696.4 MiB in `.codex-tmp` and 368.8 MiB in `.codex-cache`. These paths are not
runtime inputs. The Dockerfile's intentional `.git` version-stamping behavior
is preserved.

### 3. CNIC privacy across family boundaries

Cross-family relation aggregation now retains the raw CNIC only as an internal
grouping key and emits the same default display policy as the subscriber
adapter: nine asterisks plus the last four digits. This avoids both disclosure
and masked-value grouping collisions. A regression test verifies that a
hyphenated full CNIC cannot appear in the returned relation.

### 4. Pakistan multilingual query corpus

A 24-question deterministic golden pack covers English, Roman Urdu, and Urdu
for:

- exact subscriber identity observation lookup;
- activation/deactivation and validity timelines;
- explicit SIM/IMSI/IMEI/device links;
- active/inactive/suspended status summaries;
- conflicting supplied-attribute audits;
- subscriber-reference/IMSI/IMEI reuse candidates.

Target-specific questions either extract an exact MSISDN, subscriber reference,
IMSI, or IMEI or return clarification. Aggregate questions remain target
optional. CNIC and names are never accepted as default lookup targets.

## Pakistan operational contract

The country profile and subscriber slice use the following evidence rules:

- preserve raw phone tokens and separately normalize Pakistan canonical/E.164
  forms; do not infer the operator from a mobile prefix because portability and
  historical allocation make that unsafe;
- validate CNIC shape as 13 digits while retaining only masked CNIC in default
  analyst output;
- validate IMSI length and MCC 410 independently from ownership assertions;
- validate IMEI length and check-digit state separately and surface failures as
  review flags;
- preserve Urdu/Arabic-script names without inventing transliterations;
- use `Asia/Karachi` as the configured local timezone and preserve explicit
  offsets when present;
- keep province-specific vehicle plate rules as versioned schema packs rather
  than one nationwide OCR regex;
- treat tower coordinates, ranges, and aliases as time-versioned supplied facts;
- never turn identifier reuse, temporal proximity, shared rows, tower joins, or
  model prose into identity, ownership, association, fraud, route, or continuous
  movement.

## Model and agent architecture audit

| Role | Current decision | Evidence boundary |
| --- | --- | --- |
| Exact facts/counts/joins/timelines | hardcoded parameterized SQL and adapters | authoritative |
| Planner/routing | deterministic typed planner first | fail closed on ambiguity |
| Narrative explanation | Qwen 4B only when explicitly requested | may explain fact packet; never owns citations |
| Knowledge retrieval | Qwen 0.6B embedding | retrieval only; not an answer model |
| OCR/vision/STT/TTS | catalog candidates only | unavailable until separate benchmark gates |

The current five agents are an orchestrator plus CDR, IPDR, ANPR/geospatial,
and subscriber specialists. Their prompts are family-specific and prohibit the
main unsupported inferences. The model is intentionally bypassed for exact
queries, which improves correctness and latency on this CPU-only laptop.

Previous live model acceptance remains applicable because neither downloaded
model nor the model policy changed: exact CDR/IPDR/ANPR probes used no model;
an explicit CDR explanation completed in 93.228 seconds and preserved
deterministic response objects as authority. Repeating the same long CPU model
benchmark would add cost without new evidence.

Current primary-source review also corrected one future-model assumption. The
official Qwen3-ASR checkpoint card lists 30 supported languages and 22 Chinese
dialects but does not list Urdu. Both Qwen3-ASR catalog entries are therefore
marked `urdu_eligible: false`; they cannot be promoted to the Pakistan
Urdu/English primary role. OpenAI Whisper remains a multilingual benchmark
candidate whose own documentation warns that quality varies materially by
language, so promotion still depends on the local Urdu/code-switch/noise WER
gate. PaddleOCR's official multilingual table explicitly includes Urdu,
Pashto, Sindhi, Balochi, and English in its Arabic-script PP-OCRv5 recognition
model, so it remains an eligible OCR candidate—but not an accepted runtime
capability until the Pakistan fixture benchmark passes.

## Full module status

| Module | Current grade | Production truth | Required next work |
| --- | --- | --- | --- |
| Case/collection governance | strong foundation | case-bound scope, selectable authorized catalog, deployed duplicate prevention | approved manifest-backed cleanup workflow |
| Knowledge Base fabric | foundation | indexed assets, semantic retrieval, source previews, per-case bindings | hybrid ranking benchmark, immutable versions, field-level ACL, collection reconciliation UX |
| Evidence/chain of custody | strong foundation | hashes, versions, evidence IDs, source locators, audit contracts | signed export/package approval and child-artifact lineage |
| Structured ingestion | strong | CSV/TSV/JSON/JSONL/XLSX canonical pipeline | additive provider packs and mapping UI |
| CDR | operational specialist | exact call/service/device/time/contact/location operations | Pakistan provider drift packs and time-aware tower joins |
| IPDR | operational specialist | endpoint/domain/protocol/volume/session/timeline operations | DNS/NAT provider packs and capture-derived sessions |
| ANPR records | operational specialist | exact sightings, variants, cameras, sequences, timing/co-observations | authorized image-to-plate OCR gate and clock calibration |
| Subscriber identity | activated, data gate pending | adapter, six operations, specialist, multilingual professional no-data path | authorized synthetic/real populated-data acceptance |
| Tower/location | adapter foundation | canonical shared queries only | Phase 7.2 dedicated time-aware slice and specialist |
| Access/security | shared operational | 1,000 rows and generic canonical/correlation tools | dedicated session/failure/incident operations and agent |
| Financial | adapter foundation | exact filters/correlation only | currency-aware flows, counterparties, aggregates, specialist |
| Vehicle ownership | planned | no ownership inference | separate authorized registry adapter and field ACL |
| Documents/OCR | contract/catalog only | safe registration and deterministic metadata | Urdu/English OCR benchmark and cited page/table artifacts |
| Images | metadata foundation | safe metadata only | detection, comparison, tamper, face-policy and provenance gates |
| STT/TTS | catalog only | unavailable, correctly guarded | Urdu/English WER/MOS, timestamps, diarization readiness, disclosure |
| Social/digital footprint | planned | no accepted capability | lawful-source connectors, identity-resolution policy, audit |
| Reports | useful synchronous foundation | deterministic analyst brief/export | immutable report versions, approvals, signatures |
| APIs | strong versioned foundation | enterprise query/case/adapter/operation contracts | OpenAPI completeness, SDK examples, service-account scopes, webhooks |
| UI/UX | analyst-capable | professional typed results and responsive case desk | task-based a11y, dense-table personalization, saved queries, comparison workspace |

## Verification

- focused native collection selection plus direct-agent routing: PASS;
- agent-pool package compile: PASS (no tests selected);
- Phase 7.1 plus cross-family API focus: PASS, 26 specs selected, 26 passed;
- hardening rebuild: PASS, 19.8/4.1/375.4 seconds, one LocalAI attempt,
  6.77 GiB free, 128.95 MB root context, rollback/volumes preserved;
- post-refresh health/models/agents: PASS, four HTTP 200 probes, two approved
  models, five active specialists;
- collection governance: PASS, catalog remained at 28 and no new
  specialist-name collection was created;
- live Roman Urdu clarification/exact-target routing: PASS;
- live explicit-UTF-8 Urdu status routing: PASS;
- live Records no-data query: PASS;
- live Subscriber Agent Chat no-data query: PASS;
- live collection switch and return to the canonical case: PASS;
- live browser overflow at 1280 x 720: PASS;
- live browser diagnostics: zero entries;
- no data, collection, entry, evidence, model, image, volume, or rollback
  artifact deletion;
- no upload, reprocess, migration, staging, commit, push, or publication.

## Required next gate

1. Do not repeat the expensive rebuild; hardening activation is accepted.
2. With explicit authorization, deploy the source-only populated privacy fixes
   and ingest the exact hashed synthetic Pakistan subscriber pack
   and run all six operations, privacy checks, row accounting, exports, Agent
   Chat, and one bounded explanation.
3. Verify the populated cross-family path never exposes full CNIC/name values
   in API, UI, export, logs, or model prompts.
4. Only then mark Phase 7.1 fully data-accepted and begin Phase 7.2.

## Phase sequence after Phase 7.1

1. **Phase 7.2 Tower/Site Intelligence:** time-versioned cell/site reference,
   Pakistan provider packs, coordinate/range validation, explicit CDR joins,
   table/map citations, and RF non-inference policy.
2. **Phase 7.3 Pakistan CDR Deepening:** provider-specific schemas, multi-SIM
   and device-change evidence, tower joins, larger Urdu/Roman Urdu corpus,
   calibrated graph/timeline exports.
3. **Phase 7.4 ANPR Image Recognition:** image evidence child artifacts,
   Urdu/English/province plate OCR benchmarks, confidence/variant review,
   camera-clock calibration, and linkage to existing ANPR records.
4. **Phase 8 Image Forensics:** metadata, object detection, similarity,
   tamper indicators, policy-controlled face candidates, and visual evidence
   correlation with immutable bounding-box/source provenance.
5. **Phase 9 Speech:** Urdu/English STT benchmark, timestamps, noise/accent
   fixtures, diarization readiness, transcript KB indexing, then clearly
   disclosed TTS for reports/alerts. Synthetic voice must never be confused
   with evidence.
6. **Phase 10 Supporting Families:** vehicle registry, financial, access/log,
   social/digital-footprint, generic mapping, and advanced case/report workflow
   as separate accepted slices.

Each phase must ship adapter, operations, specialist prompt, API contract,
Pakistan goldens, deterministic tests, live UI/Agent Chat acceptance, audit
evidence, rollback, and an updated continuation ledger. No phase may claim a
capability because a model or catalog entry merely exists.
