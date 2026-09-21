# NX-B2.1 all-family query productization reconciliation V1

Date: 2026-09-03. Decision: **NX-B2.1A reconciliation complete; NX-B2.1B is the next source slice.**
NX-MMR remains closed for its accepted bounded scope. No production source correction,
deployment, evidence mutation, model inference request, migration or certification promotion occurred.
This is the first-turn reconciliation and implementation plan requested by the owner, not B2.1 completion.

Supporting artifacts in this directory:

- `reconciliation-inventory-v1.json`: all 79 executable query operations, 25 other discovery descriptors,
  nine adapter manifests, 16 agent manifests, six running agents, all 214 query variants, source hashes,
  certification levels, 20 advertised families and bounded retained coverage.
- `derived-text-probe-results-v1.json`: 14 current-artifact-driven, read-only public API diagnostics;
  eight meet the stated diagnostic contract and six expose gaps, plus two current routing diagnostics
  for recovered historical short-query text. These are not browser tests.
- `query-capability-design-v1.json`: 25 proposed query capability records, complete 20-row family gap
  matrix, semantic taxonomy, normalization/segment design and per-capability evidence dimensions.
  This is a design artifact, not a second executable registry or a new canonical certification ledger.

Private targets, source identifiers, full responses and source-test output are under the existing
Git-ignored `local-acceptance-models/nxb21-reconciliation/`. Public reports use aggregate outcomes.

## Verified Current Runtime

Five existing containers are running. API readiness and forensic API health passed. No OOM flags.

| Service | Container prefix | Image SHA-256 prefix | Restarts |
|---|---|---|---:|
| api | d62021a0a4fc | feb935b42a93 | 0 |
| forensic-records-api | ffd93bf51eca | 828e4b03e029 | 0 |
| forensic-records-worker | 40b6e628d586 | 0b22a604dbf9 | 9 |
| forensic-postgres | f66e05a3b179 | 61f891691050 | 0 |
| forensic-nats | 5c49e70d132a | e4bf19f15fd3 | 0 |

Read-only PostgreSQL counts: 63 evidence, 63 versions, 76 jobs (71 completed, five dead-letter,
zero active), 22,507 canonical rows, 827 artifacts and 61 KB assets. These agree with the accepted tuple
`63|63|76|22507|827|61`. The current acceptance case has 36 sources, 35 completed and one failed;
9,575 records comprise CDR 5,000, IPDR 2,500, logs 1,000, ANPR 750, generic nine, subscriber five,
tower five and transaction four. Family discovery counts overlap and must not be summed as evidence totals.
Per-evidence detail reads are bounded; their artifact totals are not the global database total.

Final check at 09:33:21Z again confirms VERIFIED receipt, health, all identities/restarts,
zero active jobs and unchanged tuple. Available host RAM was 4.177 GiB; this does not pass or
waive the 6 GiB build gate. SHA-256 verification found zero changes among all 428 sealed files.
The sanitized final verification is embedded in `reconciliation-inventory-v1.json`.

## Current Verified Receipt

`20260903T063719719Z`, activation verification PASS. Both current API images and all protected
identities/restarts match. The operator state retains `live_failed_cells_acceptance=PENDING` because
it predates the subsequent browser replay. The final overlay in
`reports/nexusai-nxmmr-post-activation-live-p1-acceptance-v1-20260902.md` supplies the later
PASS WITH LIMITATIONS. Preserve both records; do not rewrite the receipt or rerun its activation.
The existing 428-file source seal remains the baseline; report-only updates do not reseal it.

## Current Git / Source State

Branch `codex/forensic-hybrid-checkpoint-20260723`, HEAD
`40717b83510c08db25dc26b9d6674bf46db363ac`. Before this work: 93 modified tracked paths and
730 untracked entries; tracked diff approximately 62,609 insertions / 38,840 deletions, including
substantial pre-existing line-ending differences. This is a heavily modified accepted workspace,
not a clean branch. No stage, commit, reset, cleanup, push or PR was performed. This turn adds
diagnostic scripts/reports and updates living governance only; production paths remain unchanged.

## Authority Precedence

Latest attached NX-B2.1 owner instruction controls phase entry and permitted work. Then apply
`NEXUSAI_MASTER_DIRECTIVE.md`, `AGENTS.md`, relevant `.agents/` guidance and the STIM skill,
the sole vNext roadmap/phase ledger and the living checkpoint. Source/runtime determine implemented
behavior; dated acceptance establishes only its demonstrated scope. Latest overlays supersede older
pending P1 statements. The old current-roadmap/phase-ledger files explicitly point to the vNext pair.

## Reconciled Program Phase

NX-1, NX-UX1, NX-A1 and NX-B1 remain accepted at their recorded boundaries; NX-B2.0 remains complete.
Activate NX-B2.1 for source/query productization. B2.1A is reconciled. B2.1B–H remain unaccepted.
NX-S2, NX-M2, NX-D2, NX-Q2, NX-X2, NX-C2 and NX-ADV remain later phases, in that order.
The new source-work authority does not authorize runtime activation.

## NX-MMR Closure State

Carry forward the bounded image/video ANPR, English/Urdu/mixed OCR, English/explicit Urdu STT,
Roman Urdu derivative, timing/citations, CDR, similarity, cross-family and workspace foundation evidence.
Final video raw/group distinction, exact positive/absent, frame/time citations, new Activity and safe
historical malformed Activity reopening passed. None of these is blanket language, query, population
accuracy, TTS, security/load or product certification. New shared-search defects belong to B2.1.

## NX-B2.1 Entry Decision

**Enter source work.** Current registry and runtime have been reconciled; no P0 was observed in this
bounded inspection. Four grouped B2.1 P1 work areas below prevent baseline closure, not source entry.
Complete the shared semantic contract before broad planner/UI changes. No production fix is included
in A: the shared gap spans routing/match-mode plumbing, segment retrieval, source state and result
validation. Matching historical short/full questions were recovered; their original selected-evidence
scope is not retained in the presentation record. First-turn source implementation remains the next slice.

## Current Investigation Workspace

The React analyst utility retains Home / Data / Ask / Activity and one Add Data route; `/app`
remains advanced. `AnalystAsk.jsx` delegates to `AgentChat` in portal mode, binds the active case,
loads selected evidence details, and uses the compatibility forensic agent. No new selector or control
is needed. Latest browser acceptance is carried forward; this turn did not manufacture new UI proof.

## Current LocalAI / LLM / Embedding / Media Models

Live LocalAI advertises five installed models and eight backend registrations. Only
`qwen3-embedding-0.6b` was loaded at inspection; installed `qwen_qwen3-4b-instruct-2507` remains the
configured planning/synthesis baseline. Other installed IDs: `faster-whisper-small-ur`, `whisper-tiny`,
`face-detect-yunet-sface`. Installed is not the same as loaded or qualified for the requested role.

Read-only worker settings: ASR/ANPR/OCR/image embedding/face enabled; Video V3 enabled, V2 disabled;
TTS disabled. ASR uses `faster-whisper-small-ur`; image embeddings use `google/siglip-base-patch16-224`;
face uses YuNet/SFace. **Live OCR backend is `paddle`**, with the existing pinned multilingual
PP-OCRv5 adapter; the compose default `tesseract` and retained `eng+urd` setting are not proof that
Tesseract is executing. Existing FastALPR image and V3 video assets remain unchanged. No model
download, replacement, loading/unloading request, tuning or benchmark was performed.

## All Current Data Families

20 advertised data categories, 15 executable query family IDs and 32 historical maturity rows are
different namespaces. Inventory all of them without equating a category/extension to an executable parser.
The full machine matrix records input boundaries, ownership, states, schemas, operations, authority,
citations, UI/Activity, evidence, validation, certification and gaps for every advertised category.

| Current category | Actual bounded path / useful baseline | Entry limitation |
|---|---|---|
| Plain text/notes | native TXT passages; document/KB retrieval | MD/YAML advertising is not parser qualification |
| Access/security logs | structured access_log normalization; failed events | EVTX remains separate/deferred |
| CDR | dedicated adapter; identifier, contacts, time/aggregate | broader provider/query proof still needed |
| IPDR | dedicated adapter; endpoints, protocols, sessions | PCAP inventory is not IPDR extraction |
| ANPR vehicle sightings | structured rows; image observations; separate video groups | retain raw/group and observation authority |
| Subscriber/identity | dedicated adapter; lookup/validity/device associations | no ownership inference |
| Tower/location | dedicated adapter; site/reference/validity join | reference uncertainty, not handset GPS |
| Financial | transaction normalization; currency-separated summary | partial baseline; OFX/MT940 not certified |
| Generic tabular | bounded canonical filters/profile/quality | domain meaning not inferred |
| Spreadsheets/columnar | TSV/XLSX read-only and existing Parquet decoding before family routing | other advertised formats unqualified; no macros/recalculation |
| PDF/Office/email documents | native TXT/PDF/DOCX extraction | other formats, scans and layout not globally supported |
| Images/OCR | raster metadata, Paddle regions, fingerprints, SigLIP/face candidates | HEIC/raw/SVG manual review; no identity claim |
| Audio/STT | media metadata, raw ASR segments, Roman Urdu lineage | shared phrase/segment/state gaps |
| TTS artifacts | disabled generation; metadata concept only | advertised queryable state is misleading; no TTS control |
| Transcripts/subtitles | existing ASR artifact search | standalone SRT/VTT/ASS parser pending |
| Video | media inventory, bounded frames, ANPR V3 timeline | no new tuning or event understanding |
| Network/system captures | bounded PCAP/PCAPNG inventory | no case data or full session/payload query baseline |
| Databases/SQL | bounded SQLite inventory | no case data, arbitrary SQL or general dump query |
| Archives/bundles | bounded ZIP/TAR inventory | no case data or broad extraction/child-ingest baseline |
| Unknown/mixed | preserve, inspect, manual review | no simulated analytic capability |

## All Current Adapters

Nine platform adapter manifests: `cdr`, `ipdr`, `anpr`, `subscriber_identity`, `tower_location`,
`generic_tabular`, `image_media`, `audio_text`, `native_document` (all prefixed `nexusai.adapter.`).
Five dedicated Python packages exist under `ingestion/forensic_records/adapters/`: CDR, IPDR,
ANPR, subscriber and tower_location. Financial/log/generic use existing worker rules/wrappers;
media/document use shared pipelines. Discovery labels such as `access_log`, `transaction` and
pending capture/archive/subtitle paths are not additional independently versioned adapter manifests.

## All Current Agents / Tool Paths

Six running/configured agents: Communications_CDR_Analyst, Forensic_Records_Analyst,
Network_IPDR_Capture_Analyst, Subscriber_Identity_Analyst, Tower_Location_Reference_Analyst and
Vehicle_ANPR_Geospatial_Analyst. All report enabled. The 16 manifest records additionally include
Case_Intelligence_Orchestrator, Financial_Transactions_Analyst, Access_Security_Log_Analyst,
Document_OCR_Analyst, Audio_Speech_Analyst, Image_Vision_Analyst, Video_Timeline_Analyst,
Generic_Data_Quality_Analyst, Database_Archive_Unknown_Evidence_Analyst and
Evidence_Integrity_And_Provenance_Analyst. Their manifest status is preserved; do not call all 16 live agents.

Ordinary Ask uses the compatibility agent and existing direct/tool paths:
`forensic_hybrid_query`, `forensic_query_templates`, `forensic_list_evidence`,
`forensic_get_evidence`, `forensic_generate_report`; typed direct similarity paths use the existing
authorized image/face APIs. Public `/api/records/forensic/query` proxies to the authenticated sidecar
`/query/hybrid`. Public case query, inventory and citation endpoints remain shared. Domain manifests
declare records_sql/KB/inventory tool ownership; orchestration contracts do not imply separate running services.

## Operation Registry Reconciliation

**79 executable templates live = 79 canonical certification entries = 79 source-ledger operation IDs.**
All IDs resolve in live discovery. Live catalog version `2026-08-28.nxb1.1` has **104 descriptors**:
79 executable queries plus 25 other processing/discovery/direct-tool descriptors. The embedded platform
JSON has 66 descriptors before `reconciledForensicPlatformCatalog` merges supported templates.
Do not add these counts together or resurrect the historical 67-operation deployment gap.

The inventory records each operation's exact ID, family, description, input/scope/time contract,
executor, route, output/presentation/citation contract, tier, canonical certification and query coverage.
Presence proves registration, not live execution of every operation. Legacy descriptor `CERTIFIED`
is not canonical `PRODUCT_CERTIFIED`; some generic descriptions/time fields need capability-level
reconciliation against actual SQL. Whole-query semantic modes are not comprehensively declared today.

## Query Ledger Reconciliation

214 entries: English 163, Roman Urdu 24, Urdu 17, mixed ten. Twelve declared duplicates, zero orphan
operation references, and source-ledger status PASS for all 214. These fields concern routing,
parameters and equivalence, not fresh live certification. There are 29 operations with only catalog
coverage; 65 lack a ledger Urdu variant and 62 lack a Roman Urdu variant. All exact lists are in JSON.
Media Urdu live acceptance exists outside that ledger, so missing ledger coverage is not proof of
complete implementation absence. Conversely it is not permissible to infer missing negative/navigation proof.

The separate static family answer corpus has ten entries; live augmentation exposes 80.
Neither is the 214-entry query ledger. Variant records do not separately encode complete semantic,
positive/zero, source/version, navigation or Activity proof. Those dimensions are explicitly
NOT_RECORDED in the reconciliation, not fabricated from a PASS flag. Retain duplicate/history provenance;
classify semantic capabilities before retiring any historical example.

## Current Certification Ledger

REGISTERED 62; SOURCE_VALIDATED 12; FIXTURE_CERTIFIED one; PRODUCT_CERTIFIED four.
The four product entries are `cdr.frequent_contacts`, `cdr.temporal_activity`, `forensics.evidence`
and `forensics.evidence_package_summary`. No promotion occurred. A new phrase submode is not certified
merely because its parent operation has another accepted semantic capability. Ordinary suggestions
continue through existing PRODUCT_CERTIFIED gating.

## Current Demo Readiness

The existing 20260831 readiness matrix's 20260903 final overlay controls the accepted NX-MMR cells.
Its overall PARTIAL status remains appropriate for broader coverage and TTS. Carry forward prior
source navigation and Activity evidence by named cell. The newly reproduced B2.1 defects need their
own correction/acceptance; they do not reopen the accepted selected-video result projection.
No team-lead-demo-v4 completion or all-family live baseline is claimed here.

## Authority Hierarchy

Canonical rows/parameterized calculations decide structured facts. Raw/extracted document blocks
decide literal document occurrences. Current OCR/ASR/ANPR artifacts decide what the model recorded,
not what the real world necessarily contained. Roman Urdu is a parent-linked convenience derivative.
Retained model scores decide similarity rankings, not identity probability. KB supplies cited semantic
passages and cannot overrule exact records. LLM understanding/narrative is never an oracle.

## Current Ask Architecture

`AnalystAsk` → `AgentChat.handleSend` trims input without translating it, binds case and selected/workspace
scope, and supplies governed conversation context. Agent `buildForensicHybridQueryArgs` selects a template,
extracts typed targets/times and transcript mode. Sidecar binds authoritative current evidence version,
applies deterministic planning then narrowly gated language assistance, validates capabilities/execution,
runs registered tools, builds enterprise/shared packets, optionally synthesizes, and renders/persists results.
NX-A1 InvestigationContext, CapabilitySnapshot, typed plan, budget, ToolResult and ObservationPacket exist.
Extend their contracts; do not introduce a second planner or family calculator.

## Current Data Architecture

One Add Data → trusted format/modality routing → retained source/version and job → existing adapter/media
pipeline → canonical rows or completed derived artifacts → KB linkage and Data projections. Raw source
and artifact lineage remain authoritative. `document_pipeline.py` extracts only TXT/PDF/DOCX native text.
OCR/ASR raw text remains in artifact metadata. Detail endpoints return bounded artifact lists; search
and candidate selection must not depend on the visible Data page or arbitrary detail-page limit.

## Current Activity Architecture

Agent history persists the question, terminal state and typed presentation. `AnalystHistory` reopens
stored results; `ActivityResultTable` tolerates null/malformed historical data. Fresh Ask/Activity must
use the same validated result contract. This phase made no Ask requests and added no Activity entries;
the diagnostic endpoint executes read-only forensic queries directly. No historical answers were rewritten.

## Current Citation Contracts

Existing citations carry tenant/collection, evidence/version, artifact or row locator, source name and
optional page, region, frame, source seconds. Raw/group ANPR provenance and composed-family fairness are
accepted bounded behaviors to preserve. Probe positives returned citations owned by the selected source;
no browser navigation/seek was performed this turn. Cross-segment matches require both original
observations and their citations; a synthetic combined text blob must not replace evidentiary provenance.

## Current Query Semantics

Structured operations have typed target/filter/time/group/aggregate behaviors. Identifiers remain exact;
ANPR variant/similarity operations must require explicit distinct intent. Derived text currently uses
`TranscriptMode`: exact, time_range, source_time, otherwise source. `exact` actually means NFKC/lowercase,
non-letter/number/mark → space, whitespace collapse, then token-boundary phrase containment. It is
neither whole-observation equality nor arbitrary substring search. Each artifact is tested independently.
OCR derives exact mode from ExactTerm; documents do not. QueryLanguage is carried but is not a shared
search-representation policy. No separate typed PHRASE_CONTAINS contract currently reaches every layer.

## Exact / Phrase / Semantic Gaps

1. Legacy exact conflates punctuation-normalized token phrase lookup with other notions of exactness.
2. Unknown/missing mode defaults to source and returns passages without applying the literal term.
3. Per-artifact matching misses phrases crossing valid consecutive segments.
4. Selected-evidence ASR readiness logic also runs for whole-workspace requests, overriding matches to NOT_RUN.
5. SQL selects at most 500 newest artifacts before matching; an exhausted candidate budget can later
   masquerade as no-match. This is a source-demonstrated scale risk, not the cause proven in these small probes.
6. LLM proposal parameters have target(s), dates and direction but no literal text/match mode/source-time
   submode. Rejecting invented identifiers is valuable, but the present schema cannot govern general text intent.
7. Broad extension-based availability reports TTS queryable from ordinary audio files despite disabled TTS.

## Urdu Partial-Transcript Search Reproduction

**Late read-only Activity reconciliation found a matching historical incident.** At 05:58:42Z and
06:02:01Z, two short quoted Urdu requests (two words) ended needs_input with empty operation IDs and
no retrieval, after 60,074/60,054 ms. The explicit transcript variant was shaped
`find this transcript phrase "<two copied Urdu words>"`. At 06:02:38Z, a 22-word full-transcript
request completed through `audio.transcript_search` in 330 ms. Both quoted short and full strings
occur in a retained raw segment. Hashes/times/outcomes are in `retained_history_reconciliation`.

Two fresh no-synthesis API diagnostics recover the mechanism: the historical short question without
a supplied operation selects no operation and returns no evidence (62 ms); the same literal with the
typed transcript operation/current selected source returns its artifact (172 ms). Thus the recovered
short-query failure is **before retrieval**, not whole-value equality or segment splitting. The
historical ~60s duration is consistent with the language-assistance timeout budget, but the retained
presentation does not prove that exact timeout cause or preserve the original selected-evidence scope.

All probe terms were selected from retained current artifacts, not hardcoded production answers.
One latest Urdu transcript: full text and a four-word middle phrase both returned the source. A literal
inside-word slice returned NO_EXACT_MATCH under legacy token-boundary semantics. Independently selected
Urdu and English four-word phrases crossing two touching segments returned NO_EXACT_MATCH and no citations.

Independent read-only PostgreSQL checks confirmed each cross-segment pair exists once, both belong to
the current source version and same run, and the literal phrase occurs in their concatenated raw text.
API diagnostics took roughly 0.1–0.2 seconds each; exact timings are in JSON. The full original browser
scope cannot be reconstructed, so a complete UI replay of the owner's incident is **not claimed**.
Historical short-question routing and the separate shared Urdu/English cross-segment failure are
both demonstrated with current evidence.

## Urdu Partial-Search Root Cause

Primary recovered incident mechanism: the agent and sidecar family recognizers do not classify the
explicit `find this transcript phrase` form; the agent extracts a quoted exact term only after a
transcript template is selected. That dependency loses the literal request when routing fails. The
historical result records no operation/no retrieval, while a typed request for the same short literal
succeeds. Parse the typed literal intent before family execution, and validate it end to end; do not
add a special case for these two Urdu words.

Additional shared matching mechanism: `governedDerivedTextEvidence` loops over rows independently and
`containsExactNormalizedTranscript` pads both normalized values with spaces. Thus adjacent-segment
and inside-word slices cannot match even when raw source text contains the requested characters.
This is not whole-transcript equality and needs no Urdu-specific string patch. An additional routing
gap exists: `forensicTranscriptQuery` recognizes a bounded quote/pattern set and otherwise returns source;
bare pasted text without selected audio can enter the generic planning/retrieval path. Historical input
and fresh typed/unclassified comparisons distinguish this routing gap from the later matching gaps.
No claim of an Urdu letter-variant defect is justified yet.

## Cross-Segment Search Gap

The current SQL orders by creation time/artifact ID, not source sequence; matching never joins adjacent
text. Same-run/current-version Urdu and English pairs with touching observed times were independently
verified. Future output must retain all matched segment IDs, original text, individual locators and the
observed combined interval. Never join unrelated cases, sources, versions, processing runs or raw/derivative
representations. Missing/ambiguous timing must remain explicit; do not infer unobserved continuity.

## OCR Partial-Search Gap

The sampled short OCR phrase and absent phrase both behaved correctly through explicit exact mode.
Raw OCR projection includes `observation.raw_text`, with normalized fallback, as accepted in NX-MMR.
The shared token-boundary/punctuation semantics still need an explicit phrase/whole-value distinction.
Do not label all Urdu/mixed OCR variants newly tested: this turn sampled one current OCR source.
Region/crop locator integrity remains mandatory; no unrelated document fallback on literal no-match.

## Shared Derived-Text Search Design

Introduce a compatible typed search semantic in existing request/plan/tool/result contracts:
EXACT_VALUE, EXACT_PHRASE, PHRASE_CONTAINS, TOKEN_SEARCH and existing source/time modes. Keep the legacy
`exact` alias documented until callers migrate. Whole-value exact must not silently become substring.
Phrase-contains must test a conservative normalized literal substring, without translation/embeddings.

Search-only V1 proposal: NFC and Unicode whitespace/line-break collapse with raw span mapping; preserve
punctuation, Arabic letters, diacritics and joiners unless an independently tested mapping is explicitly
admitted. Do not reuse lossy identifier normalization or silently change historical raw display. Existing
NFKC/punctuation behavior stays limited to its legacy alias. No Urdu-specific variant map is proposed.

Search consecutive segments by tenant/case/evidence/current-version/run/representation. Initial bounded
window: two to four segments, verified ordinal order or touching valid times. Keep every contributing
observation/citation and per-segment timing; reject ambiguous adjacency. Apply source/time filters before
matching. Raw Urdu search precedes derivatives; Roman Urdu results retain parent provenance. For long
collections, paginate a bounded authorized candidate scan or return explicit incomplete search; never
claim exhaustive no-match after truncation. Reuse in-memory normalization first: no migration/reprocessing.

## Query Capability Registry Design

Use a thin machine-readable semantic layer referencing existing operation IDs/submodes. It must record
intent, authority, entity/language/scope/time/composition, positive/zero behavior, citation/Activity/UI
contract, performance policy, proof and limitations. The attached 25 entries are **design only**.
Direct image/face tools reference their existing discovery descriptor; they do not inflate the 79-query count.
Implement validation against the canonical operation registry, not another copied allowlist. Resolve current
availability from authorized evidence and role state, not file extension or static certification language.

## Query Certification Framework

Certify intent + semantic + operation/submode + authority + representative paraphrase family. The attached
matrix reserves separate contract/planner/executor/DB-artifact/result-validator, live positive/zero/scope/
current-version/citation/navigation/Activity/UI, multilingual, performance, security and anti-hardcode proof.
Every applicable dimension needs a result plus immutable evidence reference; N/A needs a reason. A
diagnostic PASS is not a capability PASS. Formal product promotion remains under the existing canonical
gate with source seal, runtime receipt and review; ProductCertificationPerformed remains false.

## LLM Planner Design

Reuse installed Qwen and `query_language_assistance.go`. Current assistance runs only when both request
and deterministic plan lack a template and a synthesis model is supplied; proposal decoding rejects unknown
fields, allows registered operations, preserves scope and prevents identifier invention. Existing schema
does not express text mode, literal span, representation or general source-time intent. Extend that schema
and validator with bounded extracted spans that must occur in the user's input; never let the model rewrite
exact identifiers. Keep deterministic fast paths and follow-up context; compare typed plans across unseen
English/Urdu/Roman/mixed paraphrases. Do not add one regex per sentence or consume later NX-Q2 depth.

## Deterministic Fast Paths

Explicit phone/IP/plate/UUID, quoted literal phrase, typed source-time interval and selected evidence
should bind directly to registered capabilities. Current explicit new targets override anaphoric context.
Selected evidence is a scope, not permission to reinterpret every question as a full transcript request.
Unknown literal semantics should clarify/fail explicitly instead of returning all text. Explicit workspace
scope must clear selected IDs and use workspace readiness; no stale context may narrow or broaden it silently.

## Result Validation Design

Extend existing plan/result/packet validation to enforce semantic and representation, current-version/run,
requested target, time, row ownership, citation ownership, scope, complete/incomplete population and count
consistency. Nonempty rows cannot have NOT_RUN; absent literal cannot yield COMPLETE_RESULTS. A cross-segment
match must own all required citations before truncation. Exact negatives require completed/exhaustive search.
No LLM repair of facts; rejected result becomes a typed failure/clarification with no fabricated answer.

## Answer Synthesis Policy

Exact positive/negative/count headlines may be deterministic. Semantic/composed explanation uses only
validated bounded facts, observations, counts, citations and material limitations. Existing factual
validator/fallback remains authoritative. No raw database, arbitrary SQL/shell/URL, embeddings or unbounded
source text enters model authority. A retrieved ASR/OCR observation proves recorded model output, not speech
or optical recognition accuracy. No unsupported identity, owner, intent, guilt or causal merge.

## Ask UI Productization

Preserve Answer → Key findings → Sources. Existing portal-mode renderer already suppresses technical
labels and supports typed findings; current operation/rank/score cards must remain truthful. B2.1G should
use one shared disclosure model in Ask and Activity, not bespoke calculators. Keep short fact-backed
headlines, useful columns only, visible human-readable sources and no fake unavailable controls.
Four accepted widths (390/820/1024/1440), keyboard focus and RTL/dir=auto remain required future gates.

## Evidence & Calculation Disclosure

Existing Ask has technical details, proof fields and result details; it lacks a consistent analyst
Evidence & calculation contract. Add server-derived scope, rows considered, filters, grouping, aggregation,
ordering, cap, exclusions, returned/total counts, latency and source references when actually measured.
Do not infer scanned rows from returned rows. Model observations show raw text, match representation,
region/time/frame and correctly labeled scores/review state. Keep embeddings hidden. Material limitations
can collapse, but should remain noticeable; omit empty boxes. Technical IDs/traces remain hidden by default.

## Citation UX

Source buttons remain visible outside disclosures, with source names and useful locators. Reuse current
evidence source routing for row/page/region/time/frame navigation. On a cross-segment phrase, provide both
source locators and seek to the first observed segment while exposing the full observed range. Do not
attach the best video frame to a different group start. Verify both Ask and reopened Activity source clicks.

## Activity Consistency

Persist the same validated semantic, scope, answer state, findings, citations and material limitations
shown in Ask. Reopen without re-execution or model inference. Continue safe historical null-table handling;
new semantics must be optional/versioned for old entries. Do not rewrite historical bad records to claim
acceptance. B2.1 probes did not exercise history persistence; this remains a required later browser gate.

## Family Gap Matrix

The complete machine matrix has 20 rows and explicitly distinguishes advertised formats, executable
ownership, current observation coverage and pending certification. High-priority shared gaps:

| ID | Severity | Evidence | Acceptance owner |
|---|---|---|---|
| NXB21-P1-001 | P1 | historical quoted partial transcript requests lose operation selection; typed literal succeeds; mode/default and token/substring gaps also proven | B2.1B semantic request→result contract |
| NXB21-P1-002 | P1 | current Urdu/English consecutive-segment phrases miss; independent SQL oracle positive | B2.1B segment search/citations |
| NXB21-P1-003 | P1 | workspace transcript query has three matching rows/citations but NOT_RUN state | B2.1B scope-aware result state |
| NXB21-P1-004 | P1 | TTS family reports queryable with seven audio sources despite disabled TTS; format OR matching | B2.1C readiness projection |
| NXB21-P2-001 | P2 | 29 catalog-only operations; missing semantic/negative/navigation dimensions | B2.1C/E/F coverage work |
| NXB21-P2-002 | P2 | existing off-page Data filename/face Target polish and disclosure inconsistency | B2.1G |

Recovered historical copy/paste behavior links primarily to P1-001; P1-002 is a separate shared
gap. Original browser selection is unrecorded. These are four grouped B2.1 work areas, not a
repository-wide defect count.

## Family Certification Order

After B: CDR/IPDR; subscriber/tower/device; financial/log/generic; documents/KB; OCR; English/Urdu/Roman
STT; image ANPR; video ANPR; image similarity; face candidates; bounded cross-family composition.
Capture/database/archive/subtitle categories receive truthful inventory/unavailable proof appropriate
to their actual baseline, not newly invented deep operations. TTS remains unavailable and independent.

## Real Runtime Certification Strategy

For each capability, inspect current authorized evidence/version, choose a target dynamically, freeze an
independent DB/artifact/source oracle, confirm a format-valid absent target, execute natural Ask, compare
the typed plan and result, open citations, reopen Activity, and store sanitized proof. Use the existing case
where lawful/current evidence is adequate. Missing family evidence is a distinct gate; do not silently
upload or reprocess. Current-version/cross-case adversarial proof must be explicit. Do not use model output
to certify its own accuracy or production helpers to compute their expected answer.

## Performance Strategy

Record planner, operation, synthesis and total latency separately, with result limits, candidate coverage
and resource use. The 14 no-synthesis diagnostics are single samples, not p95/load evidence. Proposed
initial exact-query target: p95 ≤2s on the retained baseline, subject to measurement and existing budgets;
not a new certified SLA. Resolve the 500-candidate cap's false-negative risk before claiming whole-case
exhaustiveness. Keep LLM work off explicit exact paths. No model or index acquisition is justified by A.

## Security / Scope

Public proxy checks collection access and forwards server identity; sidecar binds tenant/user/case and
current version, uses parameterized queries and tenant-policy transactions. Focused source tests cover
scope conflicts and shape validation. Positive selected probes preserve source ownership; independent
SQL corroborates current-version/run membership. No leakage was observed, but no full cross-tenant/live
security campaign was performed. Every new semantic mode must traverse the same authorization and budget
gates. Raw private source material remains ignored. No unrestricted SQL was added to product execution.

## P0

None observed in this bounded reconciliation. No new authorization bypass, destructive behavior,
corruption, fabricated evidence/citation or silent fact modification was established. This is not a
global P0=0 certification. Stop any affected path immediately if one is found during B2.1.

## P1

Four B2.1 grouped work areas remain open as recorded above. Six of 14 diagnostic contracts expose gaps:
workspace result-state contradiction, inside-word semantics, absent unclassified audio, Urdu segment,
English segment, and absent document. Some failures are mismatches against the newly requested phrase
contract rather than regressions of legacy exact-token behavior. No production fix or closure is claimed.

## P2/P3

Keep prior off-page filenames, Target/source labels and spacing polish deferred to G. Catalogue taxonomy,
query-proof completeness and unknown format coverage must be explicit in C/E/F. No new P3 blocker found.
TTS acquisition and model accuracy depth remain independently deferred, without Read Aloud controls.

## Required Source Changes

First change existing `derived_text_query.go` and typed request/plan/tool contracts, then corresponding
`forensic_direct.go`/`records_tools.go` bridge and result presentation plumbing. Do not fork the engine.
Likely contract owners: `query.go`, `nxa1_execution_foundation.go`, `query_intelligence_capabilities.go`,
`query_language_assistance.go` and existing enterprise packet/projection code (confirm exact ownership
at implementation). C later corrects `capabilities.go` state projection and adds semantic references.
G later extends shared Ask/Activity disclosure components. Worker/source extraction changes are not
currently required; no DB schema or model change is justified by the reproduced defects.

## Required Tests

Independent synthetic Unicode/whitespace/punctuation/identifier fixtures; raw text immutability;
within-word versus token phrase semantics; current retained Urdu/English positive/absent/segment cases;
raw/derivative lineage; source/run/version/case isolation; missing/reversed/overlapping timing;
candidate cap/incomplete state; result-state consistency; whole-source versus literal routing;
LLM proposal span/semantic validation; typed bridge and serialization; cross-segment citation budgets;
shared Ask/Activity renders and later actual source navigation. DB-backed tests must execute real SQL
under the tenant role, not only SQL-string assertions. No retained migration/test writes are authorized.

Executed A validation:

- `python scripts/nxb21_reconcile.py --case nexusai-multimodal-product-acceptance`: PASS inventory/parity.
- `python scripts/nxb21_probe_derived_text.py`: 14 read-only probes, eight complete diagnostic PASS,
  six expected gap observations; full responses private. The second run corrected diagnostic evaluation
  to require all contributing artifacts, zero rows for a negative, and consistent result state.
- Read-only PostgreSQL oracle: Urdu and English pair existence/current-version/same-run/literal union PASS.
- Historical Activity inspection: three matching short/full query records recovered without modifying
  history; two fresh no-synthesis public API probes separate routing failure from successful literal retrieval.
  A history-list request lacking matching case_id/collection_id was rejected HTTP 409; the correctly
  scoped request succeeded. No new history was created.
- `go test -p 1 ./api/forensic_records -run 'Test(DerivedText|QueryTerms|MergeEvidence|QueryVariant|OperationCertification|Forensic)' -count=1 '-ginkgo.focus=Governed selected-evidence transcript queries' '-ginkgo.no-color' -v`:
  PASS; nine of 336 Ginkgo specs ran, 327 outside focus, plus selected legacy tests. Initial unquoted
  PowerShell dotted-flag invocation failed argument parsing; corrected quoted invocation passed.
- No new browser acceptance, load test or formal product certification. Existing focused tests passing
  does not close the newly reproduced missing semantic cases.

## Deployment Impact

**None in this phase.** Current production source and sealed baseline remain unchanged. When B's compatible
source corrections are complete and tested, reassess actual file ownership. Likely minimal future set is
api plus forensic-records-api; worker/database/NATS stay protected unless evidence proves a worker change
essential. Prepare a new source seal and one resumable rollback-capable operator bundle, preserve 6 GiB
available RAM and stop before activation. Do not reuse the accepted NX-MMR bundle. No deployment command
is supplied now because no deployable B2.1 source candidate exists. No model/profile/volume change.

## NX-B2.1 Detailed Roadmap

| Slice | Concrete output | Exit evidence / dependency |
|---|---|---|
| A — reconciliation | this report, source/live inventory, current-artifact diagnostics, historical partial-query trace | complete for inspection/design; original browser selected-evidence scope is unrecorded |
| B — shared semantics | typed literal semantics, conservative search-only normalization, ordered segment matches, workspace state correctness | independent unit/DB/artifact and bridge/result/citation proof; no false positives/negatives in bounded matrix |
| C — capabilities | thin semantic references to registered operations; truthful readiness and input boundaries | registry parity, no duplicate execution catalogue, TTS/unrun/zero distinctions |
| D — dynamic planner | existing model's bounded literal/intent/entity/follow-up schema extension | typed plan parity on unseen English/Urdu/Roman/mixed queries; exact tokens and scope preserved |
| E — certification framework | evidence matrix, runtime-derived target/oracle manifest and case schema | independent oracle and proof requirements executable; no automatic product promotion |
| F — family certification | ordered practical positive/absent/scope/current-version/query baseline per family | individual capability proof, truthful missing data; separate activation/retained gates where necessary |
| G — Ask productization | shared answer/findings/sources and optional calculation/evidence/limitations/technical disclosures | no React analytical calculations; Ask/Activity parity, RTL/accessibility/four widths |
| H — live demo acceptance | one current-data-driven team-lead-demo-v4 | all applicable family exits, sources/navigation/Activity/performance; canonical product gate only when truly met |

Later roadmap remains NX-S2 → NX-M2 → NX-D2 → NX-Q2 → NX-X2 → NX-C2 → NX-ADV. B2.1 builds
trustworthy common semantics and practical natural access; it does not absorb generalized Q2 reasoning
or deep model/family qualification.

## Exact First Implementation Slice

**NX-B2.1B-1: shared derived-text semantic and result-state contract.** First extract quoted literal
intent independently of prior family-template selection so natural partial requests cannot lose their
operation and term. Add explicit phrase-contains
without changing exact identifiers or the documented legacy token alias. Wire it through agent, public
request, typed plan and operation. Match within current raw OCR/document/ASR text, then bounded same-run
touching transcript segments with all citations. Treat source retrieval as explicit intent, reject ignored
literal filters, and apply selected ASR readiness only to selected scope. Carry match mode, representation,
raw spans, coverage/completeness and source/time provenance into validated results. First replay the
private 14-case oracle through non-retained tests; expand only for the stated scope/timing/cap risks.
No schema migration, worker tuning, model acquisition or production activation is part of this slice.

## Exact Next Action

Implement B2.1B-1 source/tests under the already granted authority, using this inventory and private
artifact fixtures. Replay the recovered short/full history questions and both selected/workspace scope
in the new source tests; original browser selection remains unrecorded. Fix routing and shared matching
without claiming full UI closure before acceptance. Complete source/test/validation work before any deployment
approval request. Preserve current receipt and all existing retained state.

## First-pass markers and interpretation

```text
CurrentRuntimeVerified=true
CurrentUsedActivationNotRerun=true
NXMMRBoundedClosureReconciled=true
NXB21EntryReconciled=true
OperationRegistryCount=79
DiscoveryDescriptorCount=104
QueryLedgerCount=214
AllFamilyInventoryComplete=true
AuthorityHierarchyReconciled=true
LLMPlannerReconciled=true
QuerySemanticsReconciled=true
UrduPartialSearchDefectReproduced=true
UrduPartialSearchReproductionScope=historical_short_query_routing+current_typed_comparison+cross_segment_API_probes
OriginalUserAskIncidentUniquelyReproduced=false
HistoricalPartialQueryRoutingFailureReproduced=true
UrduPartialSearchRootCause=pre_retrieval_template_selection_loses_quoted_literal;additional_per_artifact_token_boundary_gaps
SharedDerivedTextSearchGap=mode_contract+cross_segment+workspace_state+candidate_completeness
QueryCapabilityRegistryDesigned=true
QueryCertificationFrameworkDesigned=true
AskUXHierarchyReconciled=true
EvidenceCalculationDisclosureDesigned=true
CrossCaseLeakage=NONE
CrossCaseLeakageMeaning=none_observed_in_bounded_inspection;full_live_security_not_performed
NoUnrestrictedSQL=PASS
NoHardcodedAnalyticalAnswers=PASS
NoSelfOracle=PASS
DeploymentPerformed=false
DatabaseMigration=false
VolumesChanged=false
ProductCertificationPerformed=false
NXB2ActivationPerformed=false
```

Inventory completion means all current registry/category entries were accounted for, including explicit
unavailable/pending cases. It does not mean all operations, formats, languages or family baselines passed.


## 2026-09-05 NX-B2.1D remediation checkpoint

The pre-remediation Q4 development candidate remains a historical 31/32 FAIL;
receipt SHA-256
`bf73696f2fc89ee02d26027818ee9d0b838926d1cbaf67a7276829a9b1321132`.
A new explicit source-bound candidate,
`nxb21d-q4-atomic-identifier-authority-r1`, is REFROZEN with identity SHA-256
`1ccc03ab44a25cb20ec28053b898c2f975b6373829303e8265833164a89e2ccc`.
Its deterministic authority preserves one exact `image.plate` source span as
one final typed entity, records any model mismatch, rejects ambiguous/no-span
inputs, and leaves phrase planning unchanged.

The fresh multilingual 16-case development corpus is FROZEN_NOT_EXECUTED with
SHA-256
`937f44c241dce1abc6f0ca1781b865b8dc0d49c89ed804411c1ac58e6a329cec`.
Source gates pass. D remains OPEN; independent qualification is not frozen;
E is not accepted; activation is BLOCKED. The consumed Q8 holdout and completed
32-case development corpus must never be rerun as qualification evidence.


## 2026-09-05 NX-B2.1D remediation development decision

The frozen remediation run completed 16/16 requests with healthy byte-safe
transport and unchanged runtime integrity. Receipt SHA-256:
`83ac55c7cd9627f0d37e21f33c556245bc6e7e62f35fb267d66f95b6872c23f3`.

Final typed plans passed 13/16. Exact `image.plate` behavior passed 8/8,
including two transparent deterministic reconciliations. Phrase controls
passed 5/8 and exposed one Urdu codepoint substitution plus two transcript
literal omissions/false clarifications. The candidate is DEVELOPMENT_FAIL and
does not advance. Preserve the run; do not rerun it or freeze a qualification
holdout from it. D is OPEN and activation is BLOCKED.

## 2026-09-05 phrase-literal remediation refreeze

The three immutable phrase failures from the atomic run are model content
defects: one Urdu literal contained Cyrillic codepoint substitutions and two
transcript proposals omitted their literal and requested clarification. Raw
bytes and strict UTF-8 agreed, so transport is not the cause. Atomic exact
identifier behavior remains 8/8 and unchanged.

Candidate `nxb21d-q4-phrase-literal-authority-r1` is separately refrozen with
identity SHA-256
`0129bd4ab0d660573726c744f6ba9874a9a15b9343a87cc5b37a264dcadf675e`.
The shared typed boundary authorizes exactly one explicit quoted phrase span
when document/transcript family context agrees. It preserves raw and final
proposals plus Unicode diagnostics, rejects multiple spans or family
contradictions, and does not extract unquoted substrings.

The fresh 20-case corpus SHA-256 is
`7c273c3b45fd3103234f098cd38bce84348c8cfff27b243f6c6987a5dd1555e6`.
It is **FROZEN_NOT_EXECUTED**. Source validation passes 35 focused planner
specs, the full forensic-records package, 133 phrase-framework assertions,
corpus validation and 14 prior byte-safe checks. D remains OPEN, qualification
is NOT_FROZEN, E is not accepted, and activation is BLOCKED.

## 2026-09-05 phrase development result and explicit-scope refreeze

The phrase candidate run is immutable at 19/20 with receipt SHA-256
`02a7981aa4d4918255ef056e0008250baa7fd265e72db5e10cf510da934123a0`.
All 16 phrase literals and four exact plate controls pass. The sole miss kept
the Roman-Urdu phrase exactly but selected one source instead of workspace for
`tamam transcript files`; transport, schema and runtime integrity pass.

Candidate `nxb21d-q4-explicit-scope-authority-r1`, identity SHA-256
`ba17c5c688eaa567ce2d006d8a0a80948a4e01e1b3c62f0765c84aeaa4d9d23a`,
centralizes bounded scope phrases outside quoted evidence. Contradictory scope
fails closed, implicit scope remains request-bound, and case authorization
cannot be widened. Its fresh 16-case corpus SHA-256 is
`e034bb3683f23c1aeb7488232a7337c56d5e9eb7cb69f9f280698af3412b1b50`.
It is FROZEN_NOT_EXECUTED. D remains OPEN and activation remains BLOCKED.

## 2026-09-05 scope-run startup preflight correction

Two operator attempts, `run-20260905T075458031Z` and
`run-20260905T075506272Z`, stopped before case 1 while the preserved LocalAI
container was still moving through its startup health interval. Both receipts
record zero executed cases at SHA-256 `b70774be...d4f7f8` and
`af4f7dff...84f44a`; the 16 fresh questions therefore remain unconsumed.

The runner now waits up to 180 seconds for that same container to become
healthy and does not restart or recreate it. Framework validation passes 122
assertions and the corpus hash remains unchanged. D is OPEN and activation is
BLOCKED pending the single standalone development execution and review.

The next attempt `run-20260905T080101493Z` passed container health and stopped
at the preserved 6 GiB preload RAM floor before case 1, also leaving the corpus
unconsumed. Receipt SHA-256 is `a6eea5c4...c9fc5a3`. The runner now performs
safe clean-page-cache recovery and waits up to 180 seconds for the same 6 GiB
floor. The updated framework passes 125 assertions.

The following run `run-20260905T080508522Z` consumed only case 1, which passed
both model and final oracles, then stopped at the 4 GiB loaded-model RAM gate
before case 2. Partial receipt SHA-256 is `d61e6ffb...5a7fa33`. The runner now
verifies and preserves completed prefix evidence, skips case 1, and applies the
bounded safe RAM wait before each remaining case. The resumable framework
passes 135 assertions. Cases 2 through 16 remain unconsumed.

`run-20260905T081329300Z` then aborted before inference because recursive
resume discovery descended into a case evidence directory. Its zero-case
receipt SHA-256 is `d5f4c307...b86e62`. Discovery is now restricted to the
one-level run/aggregate layout, while exact evidence paths and hashes remain
verified separately. Framework validation passes 137 assertions.

## 2026-09-05 scope development pass and independent qualification freeze

The resumed scope-development run `run-20260905T081643297Z` passed 16/16 raw
model and 16/16 final typed plans with zero corrections and intact runtime.
Receipt SHA-256 is
`921229a78719a5594271b4ede2fd5aadabc6f3a52d50c3893e4dd5f5badfafe1`.

The separate qualification candidate `nxb21d-q4-final-qualification-r1` is
frozen at identity SHA-256
`11c7ce2a5c0ed1d6bb83a8d2a9b26274dbcedf6c96b014a4c7e0404160c26371`.
Its new 168-case corpus SHA-256 is
`aaa8517983ea4523e235c4b3de31e5562bfaeada21ef24a7b23065a567d31d13`.
All 22 capabilities and four language groups are represented; corpus freshness,
168/168 deterministic source admissibility and 537 qualification/activation-framework
assertions pass. A preparation-only baseline run passed with receipt SHA-256
`01d28017aab8755147cd848dc4c1546c9e12f59b68a5810a1f024246406592cd`.
No qualification inference has run. The Q4-specific activation/rollback path is
sealed but gated on a reviewed suitable receipt. D remains OPEN and activation
is BLOCKED.
