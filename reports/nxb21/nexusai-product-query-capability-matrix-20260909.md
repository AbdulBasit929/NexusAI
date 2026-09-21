# NexusAI Human-Readable Query Capability Matrix — 2026-09-09

## 2026-09-11 NX-B2.1D post-proof source remediation

Final source checks: full forensic package PASS (53.556 s), full agents PASS
(50.747 s), proxy timeout PASS, Ask presentation 8/8 PASS. Grouping passes
governed plan validation, executor dispatch and presentation helpers with the
100-group output ceiling. SQL/API live acceptance remains pending.

45_CASE_PROOF=CONSUMED_FAILED; RESOURCE=PASS; current 4B KEEP as the English
functional runtime baseline. English model effectiveness and strict certification
remain unproven/pending; NX-B2.1D remains OPEN. No live inference, deployment,
service restart, model/profile change, RAM-gate change or new proof in this pass.

PLANNER=GENERIC_REMEDIATION: the selection prompt now separates operation meaning
from execution requirements. The model still emits only an issued enum; the
server binds scope/facts and records READY, SERVER_BINDABLE or USER_FACT_REQUIRED, rechecking after
final request binding. All 66 workspace descriptors are audited in
`reports/nxb21/semantic-binding-descriptor-audit-v1.json`; no question-based candidate
pruning or benchmark phrase patches. This repairs a contract defect; it does not
prove the cause of every historical model rejection or improved model behavior.

SYNTHESIS_TIMEOUT=SOURCE_DEFECT: removed the internal eight-second cap. Both
synthesis paths default to configured 120 seconds, clamp to 180 seconds and honor
earlier parent deadlines/cancellation. The agent and proxy share a 405-second
transport ceiling (180 planning + 30 execution + 180 synthesis + 15 delivery).
Individual model calls and frozen runtime guards are unchanged. Tests cover an
actual five-second cancellation, valid output after eight seconds, parent cancel,
malformed responses, invented values/foreign references and exact fallback retention.

GENERAL_HELP=GROUNDING_REMEDIATION: non-deterministic forensic assistant turns
receive question-independent adapter/template discovery with authenticated scope,
a five-second deadline, bounded response bytes and bounded model context. Catalog
status is preserved; registry presence cannot establish runtime availability,
licensing or successful processing. Domain policy distinguishes subscription,
SIM/profile, equipment and phone identifiers; IPDR fields vary by provider;
similarity is not calibrated probability and universal thresholds are forbidden.
Model compliance requires fresh evidence and is not certified by source tests.

PROJECT retains its existing seven-field source-validated implementation.
AD_HOC_GROUP now has a bounded canonical count implementation for record_type or
source_file: at most 1000 complete input rows and 100 groups, explicit null bucket,
count-descending deterministic ties, exact row lineage, scope/type checks and no
SQL or expressions from the caller. Overflow fails closed. Projection, custom
sort and pagination cannot be silently combined. Typed lowering, agent argument,
catalog input and executor are wired; live SQL/API acceptance remains pending.
Source-native grouping, general TIME_BUCKET/COMPARE/MULTI_STEP remain OPEN.
Document literal/source/version correctness remains source tested; broad OCR/RAG,
all-family acceptance and deployed Ask interaction qualification remain OPEN.

Historical consumed runner, corpus, freeze, evaluator binary and receipts are
preserved. Original frozen API/agent source is retained under
`local-acceptance-models/nxb21-small-english-product-proof-v1/consumed-source-snapshot`.
Current source intentionally differs from that historical freeze; do not rebuild
or rerun the consumed package. Final source checks and exact next actions are in
`reports/nxb21/english-functional-remediation-source-20260910.json`.


## Authority and reading rules

This is the human projection of all 79 executable operation rows. The complete machine fields live in `api/forensic_records/contracts/operation-certification-v1.json`; `reports/nxb21/product-query-capability-matrix-v1.json` defines the join to the live query-template and capability projections.

Unless a row-specific contract says otherwise, optional calendar bounds use inclusive `date_from` and exclusive `date_to`; sorting/ranking is deterministic, bounded, and operation-defined; scope is the already-authorized tenant/case/collection plus validated parameters; and every factual row or aggregate must remain traceable through source-row/aggregate lineage citations. Full calculation, null/sentinel/duplicate, tie, result-limit, citation, limitation, oracle, and pass fields remain unabridged in the machine source. Ellipses below are display abbreviations only.

| Family | Operation ID | Question type | Required fields | Optional fields | Grouping | Exposure | Certification | Principal limitation |
|---|---|---|---|---|---|---|---|---|
| case_cross_family | `forensics.collection_overview` | aggregate | - | date_from, date_to, limit | - | limited | bounded_uncertified / REGISTERED | Bounded by ingested case coverage |
| communications_cdr | `cdr.frequent_contacts` | rank | target | date_from, date_to, limit | counterparty | queryable | certified / PRODUCT_CERTIFIED | Bounded by ingested case coverage |
| communications_cdr | `cdr.call_type_breakdown` | aggregate | - | target, date_from, date_to, limit | call type, direction | limited | bounded_uncertified / REGISTERED | Bounded by ingested case coverage |
| communications_cdr | `cdr.service_usage` | aggregate | target | date_from, date_to, limit | explicit service class, direction | limited | bounded_uncertified / REGISTERED | Bounded by ingested case coverage |
| communications_cdr | `cdr.device_identity_changes` | lookup | target | date_from, date_to, limit | originating subscriber, IMEI, IMSI | limited | bounded_uncertified / REGISTERED | Observations do not prove ownership |
| communications_cdr | `cdr.multi_source_comparison` | compare | target, source_set | date_from, date_to, limit | source, counterparty, typed identifier, event signature | queryable | certified / FIXTURE_CERTIFIED | Shared observations do not prove identity, ownership, presence, or association |
| network_ipdr | `ipdr.endpoint_summary` | summarize | - | target, date_from, date_to, limit | source/destination IP, NAT IP, ports, protocol | limited | bounded_uncertified / REGISTERED | Bounded by ingested case coverage |
| network_ipdr | `ipdr.domain_summary` | summarize | - | target, date_from, date_to, limit | explicit normalized domain | limited | bounded_uncertified / REGISTERED | Domain observation does not establish ownership, content, or maliciousness |
| network_ipdr | `ipdr.protocol_breakdown` | aggregate | - | target, date_from, date_to, limit | explicit protocol | limited | bounded_uncertified / REGISTERED | Bounded by ingested case coverage |
| network_ipdr | `ipdr.session_volume` | lookup | - | target, date_from, date_to, limit | hour | limited | bounded_uncertified / REGISTERED | Bounded by ingested case coverage |
| network_ipdr | `ipdr.subscriber_sessions` | lookup | target | date_from, date_to, limit | - | limited | bounded_uncertified / REGISTERED | Bounded by ingested case coverage |
| network_ipdr | `ipdr.concurrent_sessions` | lookup | target | date_from, date_to, limit | - | limited | bounded_uncertified / REGISTERED | Explicit overlap is not attribution |
| network_ipdr | `ipdr.timeline` | timeline | target | date_from, date_to, limit | session start | limited | bounded_uncertified / REGISTERED | Bounded by ingested case coverage |
| communications_cdr | `cdr.temporal_activity` | timeline | - | target, date_from, date_to, limit | day, hour | queryable | certified / PRODUCT_CERTIFIED | Bounded by ingested case coverage |
| communications_cdr | `forensics.top_locations` | rank | - | date_from, date_to, limit | location/cell | limited | bounded_uncertified / REGISTERED | Cell observation is not continuous presence |
| communications_cdr | `cdr.geospatial_movement` | lookup | target | date_from, date_to, limit | cell site, supplied location | limited | bounded_uncertified / REGISTERED | Cell observations do not prove continuous device location or person presence |
| anpr_vehicles | `anpr.sightings` | lookup | - | target, date_from, date_to, limit | - | limited | bounded_uncertified / REGISTERED | Image-derived observations require review |
| anpr_vehicles | `anpr.camera_sequence` | lookup | target | date_from, date_to, limit | observation time, camera | limited | bounded_uncertified / REGISTERED | Sequence does not establish driver or route |
| anpr_vehicles | `anpr.camera_activity` | aggregate | - | target, date_from, date_to, limit | camera ID, supplied location | limited | bounded_uncertified / REGISTERED | Bounded by observed sightings |
| anpr_vehicles | `anpr.co_travel` | correlate | target | date_from, date_to, limit | other plate, camera | limited | bounded_uncertified / REGISTERED | Temporal proximity is not proof of association or shared travel |
| anpr_vehicles | `anpr.route_timing` | lookup | target | date_from, date_to, limit | consecutive observation pair | limited | bounded_uncertified / REGISTERED | No road route, speed, continuous movement, driver, or occupant is inferred |
| anpr_vehicles | `anpr.plate_variants` | compare | target | date_from, date_to, limit | normalized search key, raw plate text | limited | bounded_uncertified / REGISTERED | OCR variants remain observations |
| anpr_vehicles | `anpr.timeline` | timeline | target | date_from, date_to, limit | observation timestamp | limited | bounded_uncertified / REGISTERED | Bounded by ingested observations |
| anpr_vehicles | `video.anpr_grouped_timeline` | timeline | target, evidence_id | date_from, date_to, limit, plate, start_seconds, end_seconds | equal normalized model observation | limited | bounded_uncertified / REGISTERED | Equal OCR output is not tracking or object identity |
| subscriber_identity | `subscriber.identity_lookup` | lookup | target | date_from, date_to, limit | - | limited | bounded_uncertified / REGISTERED | Identifier match is an evidence observation, not proof of a person |
| subscriber_identity | `subscriber.validity_timeline` | timeline | target | date_from, date_to, limit | source row | limited | bounded_uncertified / REGISTERED | Missing validity bounds do not prove uninterrupted service |
| subscriber_identity | `subscriber.device_links` | relationship | target | date_from, date_to, limit | source row | limited | bounded_uncertified / REGISTERED | Row co-observation does not prove ownership or current control |
| subscriber_identity | `subscriber.status_summary` | summarize | - | target, date_from, date_to, limit | explicit status, review state | limited | bounded_uncertified / REGISTERED | Source-declared status only |
| subscriber_identity | `subscriber.conflict_audit` | compare | - | target, date_from, date_to, limit | stable subscriber reference or MSISDN | limited | bounded_uncertified / REGISTERED | Conflicts require human review |
| subscriber_identity | `subscriber.reuse_candidates` | lookup | - | target, date_from, date_to, limit | identifier kind/value | limited | bounded_uncertified / REGISTERED | Candidate reuse does not prove reassignment, SIM swap, or sharing |
| tower_location | `tower.site_lookup` | lookup | target | date_from, date_to, limit | - | limited | bounded_uncertified / REGISTERED | Reference coordinates do not prove RF coverage or presence |
| tower_location | `tower.reference_timeline` | timeline | target | date_from, date_to, limit | provider/site/sector history key | limited | bounded_uncertified / REGISTERED | Missing validity remains unknown; recency is not historical validity proof |
| tower_location | `tower.coordinate_audit` | inspect | - | target, date_from, date_to, limit | - | limited | bounded_uncertified / REGISTERED | Supplied coordinates/uncertainty are not measured handset location |
| tower_location | `tower.status_summary` | summarize | - | date_from, date_to, limit | status, technology, review state | limited | bounded_uncertified / REGISTERED | Supplied status is not verified live availability |
| tower_location | `tower.alias_conflicts` | compare | - | target, date_from, date_to, limit | provider/site/sector history key | limited | bounded_uncertified / REGISTERED | Conflicts are not automatically resolved |
| tower_location | `tower.cdr_join` | lookup | target | date_from, date_to, limit | - | limited | bounded_uncertified / REGISTERED | Exact reference join supplies context only, not presence |
| financial_transactions | `financial.transaction_summary` | summarize | - | target, date_from, date_to, limit | currency, status, amount role | limited | bounded_uncertified / SOURCE_VALIDATED | No FX conversion, cross-currency total, ownership, or intent inference |
| access_security_logs | `access.failed_events` | lookup | - | target, date_from, date_to, limit | source event | limited | bounded_uncertified / SOURCE_VALIDATED | Failure is not proof of compromise, malicious intent, or actor identity |
| generic_tabular | `generic.filter_records` | lookup | - | target, date_from, date_to, limit | - | limited | bounded_uncertified / SOURCE_VALIDATED | Generic rows retain source meaning; no family semantics are invented |
| document_intelligence | `document.metadata` | lookup | - | target, date_from, date_to, limit | - | limited | bounded_uncertified / SOURCE_VALIDATED | Supported native formats and completed processing only |
| document_intelligence | `document.search` | retrieve | - | target/text query | - | limited | bounded_uncertified / SOURCE_VALIDATED | Retrieved passage is context, not a deterministic analytical fact |
| image_intelligence | `image.metadata` | lookup | - | target, date_from, date_to, limit | - | limited | bounded_uncertified / SOURCE_VALIDATED | Supported decoders and completed processing only |
| image_intelligence | `image.ocr_search` | retrieve | - | target/text query | - | limited | bounded_uncertified / SOURCE_VALIDATED | OCR is a review-required model observation |
| face_intelligence | `face.candidate_observations` | lookup | evidence_id | limit | - | limited | bounded_uncertified / SOURCE_VALIDATED | Candidates never establish identity |
| audio_intelligence | `audio.metadata` | lookup | - | target, date_from, date_to, limit | - | limited | bounded_uncertified / SOURCE_VALIDATED | Supported codecs and completed processing only |
| audio_intelligence | `audio.transcript_search` | retrieve | - | target/text query | source time/representation | limited | bounded_uncertified / SOURCE_VALIDATED | ASR/Roman Urdu are observations, not verbatim fact; identifier speech is limited |
| video_intelligence | `video.metadata` | lookup | - | target, date_from, date_to, limit | - | limited | bounded_uncertified / SOURCE_VALIDATED | Supported codecs and completed processing only |
| video_intelligence | `video.timeline` | timeline | evidence_id | start_seconds, end_seconds, limit | source time, observation contract | limited | bounded_uncertified / SOURCE_VALIDATED | Sampled/incomplete; model observations require review |
| case_cross_family | `forensics.entity_activity` | aggregate | - | date_from, date_to, limit | entity/family | limited | bounded_uncertified / REGISTERED | Observation counts do not prove identity |
| case_cross_family | `forensics.relationship_network` | relationship | - | date_from, date_to, limit | source-row co-observation | limited | bounded_uncertified / REGISTERED | Co-observation does not prove relationship |
| case_cross_family | `forensics.cross_family_correlation` | correlate | target | date_from, date_to, limit | record family, exact normalized entity | limited | bounded_uncertified / REGISTERED | Co-observation does not establish identity, ownership, or causation |
| communications_cdr | `cdr.timeline` | timeline | target | date_from, date_to, limit | event timestamp, record family | limited | bounded_uncertified / REGISTERED | Bounded by ingested case coverage |
| case_cross_family | `forensics.source_records` | inspect | - | date_from, date_to, limit | - | limited | bounded_uncertified / REGISTERED | Capped audit sample, not exhaustive unless labeled |
| case_cross_family | `forensics.canonical_records` | inspect | - | date/time, target, record/source/batch, typed payload/field filters, sort, page | source rows | limited | bounded_uncertified / REGISTERED | Only allowlisted parameterized predicates; no arbitrary SQL |
| case_cross_family | `forensics.schema_profile` | inspect | - | date_from, date_to, limit | - | engineering_only | bounded_uncertified / REGISTERED | Not model-exposed |
| case_cross_family | `forensics.data_quality` | inspect | - | date_from, date_to, limit | family/source/error type | limited | bounded_uncertified / REGISTERED | Reports observed ingest quality, not unseen source quality |
| knowledge_evidence | `forensics.evidence` | retrieve | - | query, result limit | retrieval result | queryable | certified / PRODUCT_CERTIFIED | Evidence retrieval does not convert passages into deterministic facts |
| communications_cdr | `forensics.shortest_call` | rank | - | date_from, date_to, limit | - | limited | bounded_uncertified / REGISTERED | Non-zero audited duration rows only |
| communications_cdr | `forensics.longest_call` | rank | - | date_from, date_to, limit | - | limited | bounded_uncertified / REGISTERED | Non-zero audited duration rows only |
| communications_cdr | `cdr.duration_extremes` | rank | - | target, date_from, date_to, limit | extrema/aggregate | limited | bounded_uncertified / REGISTERED | Non-zero audited duration semantics |
| case_cross_family | `forensics.first_seen_last_seen` | timeline | - | date_from, date_to, limit | entity | engineering_only | bounded_uncertified / REGISTERED | Not model-exposed |
| case_cross_family | `forensics.activity_by_day` | timeline | - | date_from, date_to, limit | day/family | engineering_only | bounded_uncertified / REGISTERED | Not model-exposed |
| communications_cdr | `forensics.activity_by_hour` | timeline | - | date_from, date_to, limit | hour | limited | bounded_uncertified / REGISTERED | Bounded by ingested CDR coverage |
| communications_cdr | `forensics.night_activity` | timeline | - | date_from, date_to, limit | night window | limited | bounded_uncertified / REGISTERED | Rule window is not suspicious intent |
| communications_cdr | `forensics.repeated_location_visits` | lookup | - | date_from, date_to, limit | location/cell | limited | bounded_uncertified / REGISTERED | Cell observation is not continuous presence |
| case_cross_family | `forensics.co_travel_or_co_presence` | correlate | - | date_from, date_to, limit | shared source row | engineering_only | bounded_uncertified / REGISTERED | Not model-exposed; co-observation is not co-travel proof |
| communications_cdr | `forensics.subscriber_profile` | lookup | - | date_from, date_to, limit | identifier roles | limited | bounded_uncertified / REGISTERED | Profile is observed identifiers, not ownership |
| communications_cdr | `forensics.imei_imsi_usage` | aggregate | - | date_from, date_to, limit | IMEI/IMSI | limited | bounded_uncertified / REGISTERED | Usage observation does not establish owner/operator |
| communications_cdr | `cdr.tower_activity` | aggregate | target | date_from, date_to, limit | cell site, tower/site, supplied location | limited | bounded_uncertified / REGISTERED | Tower activity is not precise or continuous location |
| case_cross_family | `forensics.suspicious_patterns` | aggregate | - | date_from, date_to, limit | rules | engineering_only | bounded_uncertified / REGISTERED | Not model-exposed; rules do not prove suspicious intent |
| case_cross_family | `forensics.anomaly_summary` | summarize | - | date_from, date_to, limit | rules | engineering_only | bounded_uncertified / REGISTERED | Not model-exposed; anomaly is not guilt/intent |
| case_cross_family | `forensics.cross_dataset_entity_summary` | summarize | - | date_from, date_to, limit | family/entity | engineering_only | bounded_uncertified / REGISTERED | Not model-exposed |
| case_cross_family | `forensics.source_file_audit` | inspect | - | date_from, date_to, limit | source | engineering_only | bounded_uncertified / REGISTERED | Not model-exposed |
| case_cross_family | `forensics.duplicate_upload_audit` | inspect | - | date_from, date_to, limit | content identity | engineering_only | bounded_uncertified / REGISTERED | Not model-exposed; duplicate upload is not duplicate event |
| case_cross_family | `forensics.case_readiness` | readiness | - | date_from, date_to, limit | readiness component | limited | bounded_uncertified / REGISTERED | Readiness covers observed operational components only |
| case_cross_family | `forensics.evidence_package_summary` | summarize | - | date_from, date_to, limit | package component | queryable | certified / PRODUCT_CERTIFIED | Bounded by ingested and retrievable evidence |
| case_cross_family | `forensics.executive_case_brief` | summarize | - | date_from, date_to, limit | deterministic component | limited | bounded_uncertified / REGISTERED | Narrative must remain grounded in returned facts |
| case_cross_family | `forensics.court_ready_source_summary` | summarize | - | date_from, date_to, limit | source/custody component | engineering_only | bounded_uncertified / REGISTERED | Not model-exposed; does not establish legal admissibility |
| case_cross_family | `forensics.limitations_and_data_quality` | inspect | - | date_from, date_to, limit | limitation/quality component | engineering_only | bounded_uncertified / REGISTERED | Not model-exposed |

## Reconciled totals

- 79 executable/certification rows: 5 certified, 74 bounded-uncertified.
- Exposure: 5 queryable, 63 limited, 11 engineering-only.
- The English semantic planner sees every scope-compatible non-engineering row (66 in workspace scope); evidence-required rows are projected only for an authorized selected-evidence request.
- A model choice never changes the row’s required/optional inputs or any authority rule. The server binds facts and later validation requests clarification when required inputs are absent.
