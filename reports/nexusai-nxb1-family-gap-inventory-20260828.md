# NX-B1 initial family operation and readiness inventory

Status: complete before NX-B1 production implementation  
Phase: NX-B1 — all-major-family operational breadth baseline  
Boundary: source inventory only; no runtime, retained evidence, database, model,
profile, backend, deployment, staging, commit, push, or PR mutation

## Verified starting state

- NX-A1 is source-complete and remains the shared typed execution authority.
- `supportedQueryTemplates()` and the certification ledger contain 67 executable
  operations: CDR 18, IPDR 7, structured ANPR/video-ANPR 8, subscriber 6,
  tower/site 6, cross-family 21, and governed knowledge retrieval 1.
- The descriptive platform catalogue contains 66 operation descriptors. Several
  media/document/generic descriptors are not executable Ask operations; platform
  descriptor presence is therefore not counted as analyst capability.
- Existing processing and presentation paths are materially broader than the
  executable catalogue. The B1 gap is chiefly shared operation/query exposure,
  family-scoped result truth, and independent validation—not raw format admission.

## Family inventory

| Family | Ingest / processor | Presentation / citations | Executable operations and query support | Authority / readiness | Useful missing B1 capability | B1 priority and target |
| --- | --- | --- | --- | --- | --- | --- |
| CDR / communications | Operational; 13,649 retained rows reported | Typed tables/timelines, row citations, Ask and Activity | 18 operations; 3 Tier A, 15 Tier B | Deterministic; already mature | No breadth-critical gap | Preserve regression; `ALREADY_MATURE` |
| IPDR / network | Operational; 5,003 retained rows reported | Typed session results and source provenance | 7 Tier B operations | Deterministic, limited certification | Independent depth certification remains | Preserve seven-operation baseline; `PASS` for B1 breadth |
| Subscriber | Operational; 20 retained rows reported | Privacy-aware cited result cards | 6 Tier B operations | Deterministic observations; no ownership inference | No breadth-critical gap | Preserve; `ALREADY_MATURE` |
| Tower / site / geo | Operational; 10 retained rows reported | Cited reference/timeline/coordinate views | 6 Tier B operations | Deterministic supplied reference facts; no RF/device-presence inference | No breadth-critical gap | Preserve; `ALREADY_MATURE` |
| Structured ANPR | Operational; 1,508 retained rows reported | Cited sightings/timelines and Activity | 7 family operations plus retained grouped-video ANPR | Deterministic structured facts; media ANPR remains observation authority | No breadth-critical structured gap | Preserve; `ALREADY_MATURE` |
| Financial | Canonical transaction normalization exists; 8 retained rows reported | Generic source citations only | No dedicated executable operation | Common semantics are implemented but family analytics remain partial | Currency-separated exact totals/status/account-flow baseline | Add one bounded deterministic summary; remain `PARTIAL` for provider reconciliation and multi-currency depth |
| Access / security logs | Canonical access-log normalization exists; 2,000 retained rows reported | Generic source citations only | No dedicated executable operation | Deterministic normalized events; domain depth partial | Failed-event/status/action baseline with cited rows | Add one bounded deterministic failed-event operation; remain `PARTIAL` for EVTX/clock-drift/incident depth |
| Generic structured | Operational common intake and schema profiling | Generic Data/source views | Platform descriptors exist but 0 `generic.*` executable entries | Deterministic, schema-specific semantics intentionally limited | Family-scoped filter/profile/quality entry in shared catalogue | Register bounded generic filter using existing canonical executor; `PASS` baseline with explicit semantic limits |
| Documents / text | TXT/PDF/DOCX native-text paths limited by format and extraction state | Source preview and passage locators exist | Governed KB retrieval is executable; `document.search` descriptor is not | Native extraction is deterministic; OCR text is an observation | Family-scoped cited document passage search plus evidence metadata | Add metadata inventory and cited search; `PASS` for admitted native-text baseline, scanned/table depth deferred |
| KB / retrieval | Operational; 47 KB assets reported | Cited Ask results and source opening | `forensics.evidence` Tier A plus hybrid evidence-package support | Semantic retrieval; not deterministic fact authority | Measurement/reranking depth only | Preserve; `PASS` |
| General image / OCR | Image registration, metadata, OCR artifacts, comparison/similarity endpoints exist | Source preview, regions/candidate presentation and citations | No general image operation is executable through Ask | Metadata/hash are facts; OCR/similarity are observations/candidates | Image metadata inventory and family-scoped OCR search | Add metadata and OCR-search operations; keep semantic image similarity `PARTIAL` and candidate-only |
| ANPR media | Image/video ANPR processors and artifact contracts exist | Plate regions/groups with evidence/version/source-time citations | Grouped video ANPR is executable; image ANPR remains endpoint/processor capability | Model observation; never structured ANPR fact | General image Ask discovery without promoting OCR/ANPR | Covered through image inventory/OCR plus existing grouped-video operation; deeper media ANPR deferred |
| Face candidate | Detection/similarity endpoints and typed candidates exist | Candidate-only cards and citations | 0 executable Ask operations | Model-derived candidate observation; policy/calibration limited | Safe discovery of existing observations, not identity search | Initial target deferred ordinary identity/search semantics; closure review must verify whether exact-evidence candidate discovery is needed |
| Image semantic | Explicit-candidate similarity endpoint exists | Candidate ranking with citations | 0 executable Ask operations | Retrieval/candidate observation; Urdu top-1 limitation | Shared Ask adapter needs an explicit candidate-evidence contract | Defer adapter until candidate selection is part of typed Ask input; `PARTIAL` |
| Audio / ASR | Audio metadata, retained ASR and Roman-Urdu derivative artifacts exist | Playback, timestamped Urdu/Roman-Urdu transcript and citations | No audio operation is executable through Ask | Transcript/ASR is observation authority; Roman Urdu is derivative | Audio metadata inventory and family-scoped transcript search | Add both; `PASS` for admitted baseline, WER/diarization depth deferred |
| Video | Registration, metadata, bounded frames, sampled observations and grouped ANPR exist | Playback, source-time preview/timeline and Activity | Only grouped ANPR is executable; `video.timeline` descriptor is not | Sampled/model results are observations; complete-zero differs from not processed | Video metadata and bounded source-time artifact timeline | Add both; `PASS` for admitted sampled-observation baseline, event inference/tracking deferred |
| Cross-family | Stable bounded candidate correlation exists | Typed candidate result, citations, limitations | 21 case-cross operations | Candidate correlation only unless stronger certified relation exists | No B1-only expansion until family outputs are stable | Preserve regression and typed-output compatibility |

## B1 target operation set

The first bounded implementation slice will add or activate these operations in
the existing executable catalogue and NX-A1 path:

1. `financial.transaction_summary`
2. `access.failed_events`
3. `generic.filter_records`
4. `document.metadata`
5. `document.search`
6. `image.metadata`
7. `image.ocr_search`
8. `audio.metadata`
9. `audio.transcript_search`
10. `video.metadata`
11. `video.timeline`

Closure review added one bounded operation without changing the initial-inventory
record above:

12. `face.candidate_observations` — exact retained-image scope, completed
current-version artifacts only, embedding-redacted public rows, model-
observation authority, and no identity inference. This closes safe Ask/typed-
action candidate discovery while leaving face identity/search and calibration
deferred.

Each operation will be case-scoped, bounded, stably ordered, citation-required,
and independently tested. Metadata and structured calculations remain
deterministic. OCR, ASR, face, image similarity, sampled-video and ANPR media
outputs remain observations or candidate retrieval. No new model, processor,
database schema, unrestricted SQL surface, family planner, React analytical
implementation, or retained-data operation is authorized by this inventory.

## Important limitations and deferred work

- Financial provider reconciliation, exchange-rate conversion, inferred
  ownership, fraud scoring, and provider-specific semantics are out of scope.
- Access-log incident causation, EVTX parsing, clock-drift correction, and user
  identity conclusions are out of scope.
- Scanned-document OCR quality, table/layout extraction, image semantic search
  without an explicit candidate set, face identity, diarization, video event
  inference, tracking, and continuous journey reconstruction remain deferred.
- Platform descriptors and installed processors do not count as executable Ask
  capability until the shared catalogue, readiness projection, result contract,
  citations, language routing, and independent oracle all pass.
