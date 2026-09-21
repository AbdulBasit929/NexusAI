# NexusAI Evidence-Family Platform Architecture

> **vNext sequencing overlay — 2026-08-25:** the architecture remains approved,
> but its historical family-by-family phase order is superseded by the current
> breadth-first roadmap. After NX-1, deliver the neutral standalone Investigation
> Workspace (NX-UX1), then shared typed execution contracts (NX-A1), then advance
> structured, document, image, audio, video, knowledge, and bounded cross-family
> capabilities together through NX-B1/NX-B2. One or a few demonstrable, cited,
> truthful operations per family precede deeper catalogs. The utility uses one
> Overview/Evidence/Ask/Activity flow and does not expose backend complexity or
> create family mini-applications.

Status: approved target design and implementation roadmap  
Date: 2026-07-30  
Scope: NexusAI forensic-intelligence platform, local CPU reference profile, API-first customer deployment

## 1. Outcome

NexusAI will expose one logical **Case Knowledge Fabric** while retaining the
correct physical store and processing path for each evidence family. Each
family receives an independently versioned adapter and specialist agent. A
case-level orchestrator plans cross-family work and returns one professional,
evidence-grounded response.

The product is not a collection of unrelated pages and it is not a single
general-purpose chatbot. It is a case-scoped evidence platform with:

- immutable evidence registration and lineage;
- family-specific deterministic processing;
- semantic retrieval for narrative material;
- independently configurable specialist agents;
- benchmark-gated model roles;
- one query and response contract across UI and API consumers;
- reusable headless APIs for customer applications and separate UIs.

## 2. Verified current-state findings

### 2.1 What is already valuable

- Unified evidence registration, content hashes, processing jobs, audit data,
  structured records, KB links, entity observations, and provenance fields exist.
- CDR, IPDR, ANPR, subscriber, tower/location, financial transaction, access-log,
  and generic structured classifications are executable.
- CSV, JSON/JSONL/NDJSON, TSV, Parquet, and bounded read-only XLSX paths exist at
  different maturity levels.
- Twenty family capabilities are advertised, including explicit pending states
  for documents, OCR, audio, video, captures, databases, archives, and unknown data.
- Exact structured query templates, KB retrieval, hybrid routing, and bounded
  Qwen synthesis exist.
- The deployed baseline models are `qwen_qwen3-4b-instruct-2507` for planning and
  synthesis and `qwen3-embedding-0.6b` for embeddings.

### 2.2 Structural gaps confirmed by source and live UI

1. `ingestion/forensic_records/worker.py` contains the structured-family adapter
   definitions in one module. CDR has the deepest dedicated path; most other
   structured families share generic normalization/storage machinery with
   family-specific rules. These are not yet independently packaged, versioned,
   enabled, benchmarked, or deployed.
2. Only one configured agent exists: `Forensic_Records_Analyst`. There is no
   family-specialist registry or orchestrated delegation contract.
3. `RecordsIntelligence.jsx` is 2,355 lines and mixes case selection, upload,
   seed generation, evidence review, templates, model selection, manual query
   building, results, telemetry, and administration in one component.
4. Records analysis defaults to a hardcoded `records-demo` sidecar collection.
   Upload collection, records batch, and forensic query collection are separate
   states. The Agent Chat is currently bound to
   `nexusai-structured-demo-v2-20260730`. This permits scope divergence.
5. The Records collection control is free text even though the page loads the KB
   collection list. It does not provide a validated, authoritative case selector.
6. The live Records layout overflows horizontally in the available application
   frame, clips content, and gives workflow instructions, template discovery,
   prompt chips, evidence controls, and data management competing visual weight.
7. The Agent Chat displays long record results as prose/list output, repeats
   operational metadata, and does not yet render tables, timelines, maps,
   relationship graphs, citations, and model interpretation as typed result cards.
8. The model dropdown is populated from all installed models rather than models
   compatible with the selected role and evidence family.
9. The live KB contains 20 collections:

   - two empty agent-name collections;
   - fourteen acceptance/validation collections;
   - two structured-demo collections with zero KB entries but possible structured
     records outside the KB entry count;
   - `records-demo` with six KB entries;
   - `records-demo-verified` with eight KB entries.

   Collection deletion is unsafe until KB entries, structured rows, evidence,
   agents, reports, and audit references are reconciled.

## 3. Non-negotiable architecture rules

1. One selected `case_id`/`collection_id` is authoritative throughout navigation,
   upload, query, specialist delegation, export, and chat.
2. The LLM never becomes the source of exact evidence facts.
3. Each derived value retains evidence, version, processing-run, adapter, model,
   parameter, and source-locator lineage.
4. An unsupported operation returns a typed unavailable/clarification response;
   it is never simulated by a general model.
5. Format decoders and evidence-family semantics are separate. CSV and XLSX are
   formats; CDR and financial transactions are evidence families.
6. Adapters and agents are configuration-driven, versioned, observable, and
   independently testable.
7. Customer UIs use the same public application APIs as the reference NexusAI UI.
8. Destructive collection cleanup requires an export/checksum manifest, reference
   scan, explicit approval, and a recoverable retention period.

## 4. Target logical architecture

```text
Customer UI / NexusAI UI / SDK / external application
                     |
             Versioned Forensic API
                     |
       Case Query and Workflow Orchestrator
          /          |          |          \
  Adapter registry  Agent registry  Model registry  Policy registry
          |              |             |              |
  Intake -> classify -> family adapter -> derived artifacts
          |              |             |              |
     raw evidence    deterministic   vector/KB     entity/relation
       storage       SQL/Timescale     index           layer
          \______________|_____________|_______________/
                          |
            provenance, audit, jobs, metrics
```

### 4.1 Separation of concerns

| Layer | Responsibility | Must not do |
| --- | --- | --- |
| Format decoder | Safely read bytes and expose rows/pages/frames/samples | Decide investigative meaning |
| Family adapter | Validate, normalize, derive typed facts/entities/relations | Generate unsupported narrative facts |
| Deterministic tool | Execute scoped calculations and retrieve cited facts | Accept unconstrained SQL from an LLM |
| Specialist agent | Interpret intent, select allowed tools, explain bounded results | Read outside the selected case |
| Case orchestrator | Choose specialists, merge evidence, manage clarification | Replace family tools with free-form inference |
| Synthesis model | Explain returned facts and limitations | Invent counts, identities, dates, or relationships |
| UI | Present workflow, evidence, results, and state | Maintain a second independent case scope |

## 5. Adapter platform

### 5.1 Adapter contract

Every family package implements a common contract equivalent to:

```text
descriptor() -> identity, version, formats, families, operations, resource profile
detect(sample, declared hints) -> score, reasons, schema profile, warnings
validate(source, policy) -> accepted/rejected/needs-review
plan(source, case capabilities) -> ordered processing steps
extract(source) -> typed observations and raw-value locators
normalize(observations) -> canonical facts without losing raw values
derive(facts) -> entities, relations, media/text artifacts, quality signals
persist(outputs) -> idempotent references into correct physical stores
index(outputs) -> cited semantic chunks where useful
verify(run) -> accounting, hashes, counts, constraints, provenance
capabilities(case) -> available operations and reasons
```

Required adapter outputs include:

- adapter ID, semantic version, configuration hash, and code revision;
- input evidence/version ID and exact source locator;
- accepted, duplicate, rejected, warning, and derived-artifact accounting;
- typed quality and confidence measures;
- deterministic operations provided;
- emitted entities and relations with normalization version;
- model/backend identity for every model-derived artifact;
- retryability, terminal error classification, and safe reprocessing behavior.

### 5.2 Package structure

The current worker must be decomposed incrementally, preserving behavior:

```text
ingestion/forensic_records/
  runtime/                 queue, lease, job, storage, audit, metrics
  formats/                 csv, json, jsonl, tsv, parquet, xlsx, document, media
  adapters/
    cdr/
    ipdr/
    anpr/
    subscriber/
    tower_location/
    financial_transactions/
    access_security_logs/
    generic_tabular/
    documents_ocr/
    audio_speech/
    image_vision/
    video/
    captures/
    databases/
    archives/
    unknown/
  registry/                descriptors, compatibility, enablement, versions
```

No big-bang rewrite is allowed. First wrap existing behavior behind the contract,
then move one accepted family at a time while holding golden results constant.

### 5.3 Evidence-family depth

| Family | Dedicated deterministic operations | Model-assisted role | Pakistan/local requirements |
| --- | --- | --- | --- |
| CDR | calls between parties, direction, durations, top contacts, cell sequence, IMEI/IMSI changes, service/USSD/GPRS classification | explain bounded patterns | `03xx`/`+92` variants, operator schemas, PK timezone, Urdu labels |
| IPDR/network sessions | IP/NAT/port/domain/session/bytes, overlap, subscriber link, DNS/session summaries | explain returned traffic patterns | IPv4/IPv6, CGNAT/provider aliases, timezone/clock drift |
| ANPR/vehicle | exact sightings, camera sequence, co-travel, route timing, plate variants | movement narrative over exact sightings | provincial plate formats, Urdu script, camera confidence |
| Subscriber/identity | ownership windows, identifier lookup, alias/conflict analysis, shared attributes | explain conflicts with confidence | CNIC controls/redaction, transliteration, SIM reuse |
| Tower/location | site/sector lookup, coordinate validation, joins, distance/time mapping | location context | LAC/TAC/CGI/ECGI/operator aliases, WGS84 and uncertainty |
| Financial | exact currency-separated flows, counterparties, reversals, duplicates, velocity, circular paths | explain bounded anomalies | PKR, IBAN, Raast/wallet/bank schemas; no cross-currency guessing |
| Access/security logs | authentication/event sequences, user/IP/path filters, rare/failed activity | incident narrative over exact events | UTC/PK clock handling, late events, redaction |
| Generic tabular | schema profile, exact filters/counts/nulls/drift, user-confirmed mapping | schema explanation and clarification | Urdu headers, encodings, leading zeroes, provider drift |
| Documents/OCR | pages, attachments, layout, tables, native text, OCR spans | cited comparison/summary | Urdu/English mixed text, forms, scans, right-to-left order |
| Audio/STT | codec/duration, transcript cues, timestamps, language, speaker turns | cited transcript summary | Urdu/English code switching, telephony noise, consent policy |
| TTS artifacts | input-text hash, voice/backend/version, duration, parent linkage | approved accessibility narration | Urdu pronunciation benchmark and voice-consent policy |
| Images/vision | dimensions/EXIF, OCR boxes, detections, hashes | grounded visual description | Urdu/English OCR, plates, low light/blur; legal face controls |
| Video | streams/duration, bounded frames, detections, transcript timeline | grounded event summary | CCTV clock drift and long-video resource bounds |
| Captures/EVTX | safe inventory, packets/events, bounded sessions, protocols | explanation over normalized events | no payload execution; defensible time normalization |
| Databases/SQL | read-only schema, tables, bounded approved queries, relationships | query clarification | WAL/encoding/corruption fixtures; no arbitrary writes |
| Archives | safe member inventory, limits, child evidence registration | bundle overview | traversal/bomb/polyglot protection and recursive lineage |
| Unknown/mixed | signature/profile, quarantine, duplicate detection, disposition | classification explanation only | preserve without guessing |

## 6. Agent platform

### 6.1 Agent topology

The product should expose agents by evidence domain rather than one agent per
extension:

- `Case_Intelligence_Orchestrator`
- `Communications_CDR_Analyst`
- `Network_IPDR_Capture_Analyst`
- `Vehicle_ANPR_Geospatial_Analyst`
- `Subscriber_Identity_Analyst`
- `Financial_Transactions_Analyst`
- `Access_Security_Log_Analyst`
- `Document_OCR_Analyst`
- `Audio_Speech_Analyst`
- `Image_Vision_Analyst`
- `Video_Timeline_Analyst`
- `Generic_Data_Quality_Analyst`
- `Evidence_Integrity_And_Provenance_Analyst`

The existing `Forensic_Records_Analyst` remains a compatibility alias until the
orchestrator is accepted.

### 6.2 Agent manifest

Each agent is declared through a versioned manifest containing:

- agent ID, family IDs, enabled operations, and tool allowlist;
- tenant/case scope source, never a second hardcoded collection;
- planner, synthesis, embedding, reranker, extraction, OCR, ASR, VLM, or TTS
  model-role references as applicable;
- limits for rows, chunks, frames, audio duration, tokens, time, and memory;
- clarification and abstention policy;
- response schema and allowed visualization types;
- required citations and provenance rules;
- benchmark suite and minimum promotion thresholds;
- fallback agent/model and rollback revision.

### 6.3 Delegation flow

```text
raw analyst question
  -> authenticate and bind tenant/case
  -> detect requested operations and evidence families
  -> inspect case capability coverage
  -> clarify if scope/identity/time/meaning is ambiguous
  -> build typed plan
  -> invoke one or more specialist toolchains
  -> validate outputs and citations
  -> cross-family correlation, if required
  -> bounded synthesis
  -> typed enterprise response
```

Specialist agents never call each other directly. The orchestrator owns the plan,
budget, case scope, and merge semantics.

## 7. Model-role architecture

### 7.1 Baseline and candidates

No single model is best for every family. The current safe defaults remain:

| Role | Current/default decision |
| --- | --- |
| Query planning and bounded explanation | Keep `qwen_qwen3-4b-instruct-2507` until a golden benchmark wins |
| Embeddings | Keep `qwen3-embedding-0.6b`; benchmark a reranker before replacing it |
| Structured exact analytics | No model; parameterized deterministic tools |
| Urdu/English OCR | First compare Tesseract `urd+eng` resource floor with PaddleOCR `arabic_PP-OCRv5_mobile_rec` |
| Document layout/tables | Benchmark Docling native text/layout/table pipeline, OCR only where required |
| Urdu/English STT | Load/latency gate with multilingual Whisper tiny; promote small only if WER gain justifies RAM/latency |
| Vision | Benchmark a small CPU-compatible VLM only after OCR/detection baselines; no model is active yet |
| TTS | Benchmark a CPU-compatible Piper/LocalAI voice first; promotion requires Urdu intelligibility and consent controls |

LocalAI documents modular backends for LLMs, STT, TTS, detection, audio,
reranking, and other roles. PaddleOCR documents an Arabic-script PP-OCRv5 mobile
recognizer covering Urdu and related languages. Docling exposes native PDF
parsing, OCR engine selection, table extraction, resource controls, and
structure-preserving outputs. These are candidates, not accepted NexusAI claims.

### 7.2 Selection and user override

The model registry ranks candidates using:

1. operation and family compatibility;
2. license and deployment policy;
3. available backend and CPU/GPU capability;
4. peak RAM/disk/load-time limits;
5. family golden-set accuracy;
6. citation/schema conformance and abstention quality;
7. warm/cold latency and throughput;
8. current case language/media characteristics.

The UI may offer **Auto (recommended)** plus compatible installed candidates.
An operator override must show the expected tradeoff, validate compatibility,
record the choice in the processing run, and retain one-click rollback. It must
not list an embedding model as a chat model or a TTS model as an OCR model.

### 7.3 Hardware-tiered role resolution

Model-role acceptance is accuracy-first and hardware-aware, not CPU-only. The
reusable policy is `configuration/nexusai_hardware_tiers.json`: D0 is the
installed CPU development tier; D1, P1 and P2 are target GPU profiles whose
performance remains `pending_hardware_validation` until real hardware is
inventoried. A model can pass accuracy for a GPU tier while being truthfully
`hardware_incompatible` on D0. Conversely, D0 speed never rescues a model that
fails its role's accuracy, calibration or abstention gate.

The family adapter and agent continue to request stable roles such as
`plate_region_detection`, `latin_plate_ocr` or `urdu_arabic_plate_ocr`. A
configuration-driven resolver selects an accepted implementation and records
model ID/revision, backend, precision and hardware tier in the processing run.
When the preferred implementation is absent, the resolver fails closed or
returns an explicitly degraded/unavailable capability state; it does not
silently substitute a materially weaker model. The business API remains
hardware-neutral, while its technical trace discloses execution details.

Every cross-hardware comparison holds model revision, evidence manifest and
normalization constant. GPU qualification measures output agreement, unchanged
accuracy/calibration, latency, RAM/VRAM, batch/concurrency saturation and
unavailable-accelerator/OOM behavior before the tier can be advertised.

## 8. Query and result architecture

### 8.1 From raw language to a typed plan

Templates remain regression-tested operations, not the primary UX. A user may
enter arbitrary natural language. The planner produces a typed query DSL:

```json
{
  "case_id": "case-pk-demo-001",
  "intent": "rank_counterparties",
  "families": ["cdr"],
  "entities": [{"type": "phone", "value": "..."}],
  "time_range": {"from": "...", "to": "...", "timezone": "Asia/Karachi"},
  "filters": [],
  "measures": ["call_count", "total_duration"],
  "group_by": ["counterparty"],
  "sort": [{"field": "call_count", "direction": "desc"}],
  "limit": 25,
  "required_tools": ["cdr.rank_counterparties"],
  "clarifications": []
}
```

Unknown fields are resolved against observed schemas and adapter aliases. Unsafe
or materially ambiguous plans stop with one focused clarification.

### 8.2 Enterprise response contract

Every UI and API receives the same typed response:

- executive answer;
- deterministic findings and computed measures;
- model interpretation in a separately labeled block;
- evidence and citation objects;
- result-table schema and rows;
- optional timeline, map, and graph specifications;
- route, tools, adapters, models, parameters, and latency trace;
- coverage, warnings, uncertainty, and unsupported operations;
- recommended follow-up questions/actions;
- export references and immutable report metadata.

## 9. API-first product surface

The customer-facing contract should be versioned under `/api/v1/forensics` while
existing routes remain compatibility adapters during migration.

| Endpoint group | Purpose |
| --- | --- |
| `/cases` | create/list/read cases and active evidence summaries |
| `/cases/{id}/evidence` | register, list, inspect, version, and reprocess evidence |
| `/cases/{id}/query` | execute a raw question or typed plan |
| `/cases/{id}/reports` | generate and retrieve immutable reports |
| `/cases/{id}/entities` and `/relations` | entity resolution and relationship exploration |
| `/adapters` | family/format capabilities, versions, status, limits |
| `/agents` | specialist manifests, supported operations, health |
| `/models` | role-compatible installed/candidate models and recommendations |
| `/jobs` | asynchronous processing state, retry/DLQ, events |
| `/webhooks` | customer delivery for completed/failed jobs |

API requirements:

- OpenAPI schemas and generated contract tests;
- authenticated tenant/case binding and policy scopes;
- `Idempotency-Key`, request IDs, pagination, bounded filters, and stable errors;
- synchronous bounded queries and asynchronous long-running media jobs;
- webhook signatures and replay protection;
- capability discovery before submission;
- backward-compatible versioning and deprecation headers;
- Python/TypeScript examples and a reference SDK only after the contract stabilizes.

This permits four deployment products from one core:

1. full NexusAI UI;
2. customer application using only APIs;
3. a family-specific service and UI, such as ANPR or CDR analysis;
4. an embedded case-analysis component using the same response schemas.

## 10. Professional UI/UX target

### 10.1 Replace Records Intelligence with Case Workspace

`/app/records` becomes a compatibility redirect or entry into:

```text
/app/cases/:caseId/overview
/app/cases/:caseId/evidence
/app/cases/:caseId/analyze
/app/cases/:caseId/relationships
/app/cases/:caseId/jobs
/app/cases/:caseId/settings
```

The persistent case header contains one searchable selector, tenant/security
classification, ingestion health, evidence count, queryable families, and the
current processing state. Selection is URL-backed and shared by all pages and
agents.

### 10.2 Page responsibilities

- **Overview:** concise case KPIs, time coverage, family coverage, warnings, and
  recent activity.
- **Evidence:** professional table/card browser, filters, upload, provenance,
  processing status, and side-panel detail.
- **Analyze:** the main natural-language workspace with capability-aware prompt
  suggestions and typed result cards.
- **Relationships:** entity graph, timeline, map, and cross-family evidence links.
- **Jobs:** queued/running/retry/DLQ/reprocessing visibility.
- **Settings:** adapter/model policies, retention, integrations, and advanced tools.

Seed generators, raw template discovery, telemetry, and manual query builders are
development/admin tools and must not occupy the default analyst workflow.

### 10.3 Agent Chat

- reads active case from shared case context and permits authorized case change;
- shows selected specialist(s) and model role without overwhelming the answer;
- renders deterministic tables, charts, citations, timelines, maps, and graphs as
  components rather than serializing them into long prose;
- collapses execution trace and raw JSON under “How this was computed”;
- shows model interpretation only when synthesis succeeded;
- provides export, open evidence, save query, and follow-up actions;
- never advertises “bounded model synthesis” as completed when a fallback occurred.

## 11. Collection governance and safe cleanup

### 11.1 Immediate classification

| Class | Current collections | Action |
| --- | --- | --- |
| Candidate pilot | `records-demo-verified` | Preserve; likely canonical pilot after full reference scan |
| Legacy/demo | `records-demo`, both `nexusai-structured-demo*` | Reconcile structured rows, KB entries, agent bindings, and reports before merge/archive |
| Empty accidental | `Forensic_Records_Analyst`, `forensic_records_analyst` | Mark cleanup candidates; verify no agent/runtime references |
| Acceptance/system | fourteen `forensic-*acceptance*`, validation/audit/goldens collections | Hide under system/test namespace; retain for reproducibility until retention approval |

### 11.2 Required cleanup workflow

1. Add collection metadata: purpose, environment, owner, case status, visibility,
   created/last-used timestamps, retention class, and aliases.
2. Generate a read-only manifest covering KB entries, vectors, evidence items,
   structured rows, jobs, agents, reports, sources, and audit events.
3. Select one canonical demo case and create an alias/migration map.
4. Hide test/system collections from analyst selectors by default.
5. Export manifests and checksums before any merge or removal.
6. Use archive/soft-delete with a retention window.
7. Require explicit human approval before permanent deletion.

## 12. Revised phase plan

### Phase 5A — Contracts and registries

- Introduce adapter, agent, model-role, operation, and response contracts.
- Wrap existing CDR and generic worker behavior without changing outputs.
- Create registry discovery APIs and exact contract tests.
- Acceptance: old goldens remain byte/accounting equivalent; no retained data change.

### Phase 5B — Case/collection truth and governance

- Implement case metadata and one validated active-case selector.
- Bind Records, Agent Chat, upload, query, evidence, and reports to one case scope.
- Produce the collection reconciliation/cleanup manifest and hide system collections.
- Acceptance: no request can silently use a different collection; zero deletions.

### Phase 5C — Case Workspace UI foundation

- Split the monolith into Overview, Evidence, Analyze, Relationships, Jobs, and Settings.
- Add responsive layouts and typed response components.
- Preserve advanced controls behind explicit roles/settings.
- Acceptance: desktop/tablet/mobile visual tests; no horizontal overflow; WCAG-oriented
  keyboard/focus checks; existing analyst workflows still pass.

### Phase 6 — Structured family adapters and specialist agents

Deliver bounded vertical slices in this order:

1. CDR communications;
2. IPDR/network sessions;
3. ANPR and geospatial;
4. subscriber identity and tower reference;
5. financial transactions;
6. access/security logs;
7. generic tabular/schema drift.

Each slice includes adapter package, manifest, operations, specialist agent,
Pakistan-shaped synthetic goldens, authorized real-format audit, API, UI cards,
metrics, documentation, and rollback.

Implementation status at 2026-07-31: slices 1 and 2 are deployed and live-
accepted. Slice 3 is implemented as `nexusai.adapter.anpr` 1.1.0 with an
operational `Vehicle_ANPR_Geospatial_Analyst`, seven deterministic governed
query operations, source goldens/benchmark, typed API responses, and a dedicated
Records Intelligence command surface. Its retained-data semantics deliberately
limit co-travel to same-camera temporal proximity and route timing to consecutive
observations plus optional straight-line distance; association and road-route
claims remain out of scope. Runtime activation is controlled by the same
rollback-preserving 6 GiB rebuild gate.

Implementation status at 2026-08-04: Phase 6 is live-accepted. The subscriber
half of structured slice 4 is populated-runtime accepted as
`nexusai.adapter.subscriber_identity` 1.1.0 with six deterministic public
workflows, an operational privacy-aware specialist, exact fixture accounting,
privacy/citation/export/model-fallback/Agent Chat/responsive-UI acceptance, and
an isolated authorized synthetic collection. Tower/site intelligence is now a
separately bounded Phase 7.2 adapter/API/profile/runtime slice with six public
operations. It preserves time-aware supplied references and explicit unmatched
CDR joins; neither subscriber nor Tower acceptance implies RF coverage, device
presence, continuous location, or ownership.

### Phase 7 — Documents and OCR

- Native PDF/Office/email extraction first; OCR only for scanned/image regions.
- Benchmark Urdu/English OCR candidates and Docling resource profiles.
- Store page/block/table/bounding-box citations and extraction confidence.

### Phase 8 — Audio, STT, transcripts, and TTS artifacts

- Deterministic audio inspection, bounded conversion, multilingual ASR benchmark,
  cue/timestamp storage, transcript indexing, and speaker processing.
- Add TTS as a governed derived artifact, not evidence truth.

### Phase 9 — Images and video

- OCR, EXIF, detections, small-VLM benchmark, frame sampling, audio extraction,
  and timeline results with coordinate/time citations.
- Face/biometric functions remain policy-gated.

### Phase 10 — Captures, databases, archives, and unknown evidence

- Safe bounded parsers, child evidence, read-only operations, and specialist tools.
- Enforce bomb/traversal/execution/corruption controls.

### Phase 11 — Cross-family entity and relationship intelligence

- Versioned normalization, aliases, temporal ownership, confidence, graph/timeline/map.
- Orchestrated multi-agent plans with deterministic merge semantics.

### Phase 12 — API productization and customer integration

- Freeze v1 OpenAPI, async jobs/webhooks, policy scopes, contract SDK examples,
  family-specific deployment profiles, and embeddable UI packages.

### Phase 13 — Enterprise hardening and promotion

- Multi-tenant isolation, audit/retention, backup/restore, observability, load/fault
  testing, benchmark dashboards, security review, deployment/rollback runbooks,
  and release acceptance.

## 13. Definition of done for every family slice

A family is not “complete” until all of these are true:

1. real-format and adversarial fixtures exist;
2. classification and schema/profile metrics meet a recorded threshold;
3. exact row/artifact accounting reconciles;
4. family operations match goldens exactly;
5. entities/relations and locators are verified;
6. model outputs meet accuracy/citation/abstention thresholds;
7. cold/warm latency, throughput, peak RAM, and disk are measured;
8. API schema and authorization tests pass;
9. UI workflow and accessibility tests pass;
10. failures/retries/reprocessing are visible and auditable;
11. documentation, demo, rollback, and team-lead brief are updated;
12. retained deployment is separately approved and live-smoked.

## 14. Immediate next action

The STIM-0 through STIM-7 structured-intelligence maturity program is now
closed with final source/runtime acceptance. Before resuming the historical
family roadmap, reconcile this target architecture with a design-only
**Governed Runtime Query Intelligence** proposal. The proposal must compose
certified family operations or explicitly allowlisted typed primitives, bind
every plan to tenant/case/collection/source/evidence/version scope, enforce
cost and row budgets, and emit inspectable plan, parameter, citation, and
failure telemetry. General model-authored SQL is not authorized. This is the
immediate post-STIM priority, not an implementation approval.

Documents/RAG and media families retain their existing separate acceptance
boundaries. Document retrieval cannot become an uncited computational oracle,
and ANPR image, general image, audio, or video work does not begin under the
post-STIM reconciliation handoff.

Phase 7.1 populated subscriber acceptance is complete. Phase 7.2 now has a
bounded Tower/Site adapter/API/profile/runtime slice with exact reference
lookup, timeline, coordinate audit, status, alias-conflict, and time-aware CDR
join operations. When at least 6 GiB free RAM is available, perform one guarded
full LocalAI refresh and repeat only the new Tower natural-language direct-route
and UI smoke; the compatible live specialist/API path is already accepted.

Then deepen Pakistan CDR analysis against exact time-aware tower references,
keeping unmatched identifiers explicit and prohibiting RF/device-presence
inference. The subsequent priority is ANPR image/plate recognition, general
image forensics, Urdu/English STT plus disclosed TTS, then separately accepted
supporting vehicle, financial, access/security, social/digital-footprint, and
report/case slices. This priority overlay supersedes the older generic modality
ordering while preserving every contract and definition-of-done requirement in
this document.

## 15. Team-lead briefing

> We audited the current platform rather than adding another isolated feature.
> The foundation is sound—evidence registration, structured storage, KB retrieval,
> provenance, and hybrid queries already work—but the family logic, agent logic,
> collection scope, and UI are too centralized. We now have an approved design in
> which every evidence domain has a versioned adapter and specialist agent, while
> one case orchestrator answers questions across them. Exact facts continue to
> come from deterministic tools; models are selected per role through measured
> benchmarks and only explain bounded evidence. The same versioned APIs will power
> NexusAI, customer applications, and optional family-specific UIs. The first
> implementation fixes one authoritative case/Knowledge Base scope and extracts
> adapter/agent contracts without deleting data or rewriting the whole system.

## 16. Primary capability references

- LocalAI backend catalog: <https://localai.io/backends/>
- PaddleOCR PP-OCRv5 multilingual models, including the Arabic-script model:
  <https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/algorithm/PP-OCRv5/PP-OCRv5_multi_languages.en.md>
- Docling model catalog and processing roles:
  <https://docling-project.github.io/docling/usage/model_catalog/>
- Docling pipeline options and resource controls:
  <https://docling-project.github.io/docling/reference/pipeline_options/>
- OpenAI Whisper multilingual model family baseline:
  <https://huggingface.co/openai/whisper-small>
- Tesseract fast CPU language models:
  <https://github.com/tesseract-ocr/tessdata_fast>

## 17. NX-A1 shared execution contract integration — 2026-08-28

Family adapters and specialists now have one source-level integration target:
the NX-A1 shared execution foundation. An operation is implemented and
registered once, projected with certification/readiness/authorization metadata,
validated as a typed plan step, and returned as a Tool Result into the existing
enterprise response. UI actions and natural-language Ask select that same
operation contract. Specialists remain executors; the case orchestrator retains
scope, permission, budget, dependency and merge authority.

Model-derived family output enters `forensics.observation-packet/v1` and cannot
be placed in deterministic facts. Calendar and media source time have separate
scope domains. Cross-family output remains a cited candidate correlation unless
a certified deterministic relation proves a stronger state. The extension
workflow and current ceilings are documented in
`docs/design/nexusai-nxa1-shared-query-agent-tool-execution-foundation.md`.

This source slice changes no family maturity, operation count, model role,
adapter runtime, database schema, or retained evidence. NX-B1 may now add
all-major-family baseline operations without inventing a second planner or UI
execution path.
