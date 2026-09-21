# NexusAI program phase ledger

> **Historical ledger from 2026-08-25.** Current execution status is governed by
> `docs/roadmap/nexusai-next-generation-phase-ledger.md`. Entries below do not
> authorize a vNext phase.

Current pointer (2026-08-27): NX-1 is closed live (`OpenP0=0`, foundation
`P1=0`); NX-UX1A/B/C/D/E/F are source/browser-pass and deployment-gated.
The authoritative vNext ledger records the redesigned Home and shared analyst
system, NX-UX1G as next, and the LocalAI/UI-only future activation boundary.

## MMV-2 final P1 source closure — 2026-08-24

- P1-1 Urdu ANPR routing: `source_pass_live_certification_gated`.
- P1-2 public grouped video ANPR: `source_pass_live_certification_gated`;
  exactly one public operation is registered.
- Deployment: `required_not_authorized`; forensic API plus LocalAI/UI only,
  after current-image rollback tags and the mandatory 6 GiB RAM gate.
- Retained positive video: `not_authorized`; ownership/retention attestation
  and visual review are still required.
- Current counts: `51/51/64/0/22207/441/47`; no mutation in this phase.
- Open P0/P1/P2/P3: `0/2/3/0` until activation and live certification close
  the two source-fixed P1 gates. MMV-3 remains planned, not active.

## MMV phase ledger — 2026-08-24

- MMV-1 Real-World Validation, Gap Reconciliation & Maturity Baseline:
  `source_complete_pending_deployment_and_live_browser_acceptance`. All current
  families are inventoried; positive video ANPR passed non-retained; Urdu OCR
  and bounded audio baselines are recorded; model, operation, query, data and
  gap inventories are generated. P0=0 and open foundational P1=0. Two P1
  contradictions were corrected in source.
- MMV-2 ANPR/Image/Video: `source_slice_complete_gated_acceptance`; source-time
  sampling, positive/negative video matrices, Pakistan image robustness,
  grouping presentation, exact/dHash comparison and bounded SigLIP ranking are
  complete. Public media Ask/History operation certification, deployment,
  retained positive proof and live four-viewport browser acceptance remain.
  The first authorized narrow activation attempt stopped before building at the
  mandatory 6 GiB RAM gate (2.56 GiB maximum observed after bounded service
  stop); original containers and retained counts were restored unchanged.
- MMV-3 Pakistan Language Intelligence: `planned`; benchmark the admitted Urdu
  OCR challenger and expand lawful ASR/OCR corpora.
- MMV-4 Face/Visual: `planned`.
- MMV-5 Document: `planned`.
- MMV-6 Operation Expansion: `planned`.
- MMV-7 Query Intelligence: `planned`.
- MMV-8 Governed Dynamic Analytics: `planned`.
- MMV-9 Cross-Family Intelligence: `planned`.
- MMV-10 Model Specialization: `planned`.
- MMV-11 Complete Certification: `planned`.
- MMV-12 Advanced Capabilities: `planned`.

Gates remain explicit: retained mutation `needed_not_authorized`; deployment
`needed_for_source_changes_not_authorized`; Urdu OCR model download
`needed_for_future_challenger_not_authorized`; database migration `not_needed`.

Updated: 2026-08-24  
Authority: `NEXUSAI_MASTER_DIRECTIVE.md`

Historical pre-MMV declaration follows for traceability.

Only the two declarations above identify current work. A phase marked
`reconciling` may receive non-implementation design review without becoming
the active phase. Statuses never collapse source, deployment, runtime, or data
acceptance into one claim.

## Breadth-first multimodal ledger

- BF-0 Capability Reconciliation: `complete`.
- BF-1 ANPR MVP: `processor_runtime_accepted_retained_orchestration_limited`; opt-in FastALPR
  uses explicit local assets, preserves raw/normalized/confidence/crop
  provenance and writes review-required observations through existing stores.
- BF-2 General Image MVP: `processor_runtime_accepted_retained_technical_observation_only`; supported
  raster evidence queues, emits technical observations, supports authorized
  preview and delegates ANPR. General OCR/vision is deferred.
- BF-3 Audio MVP: `pakistan_m1_retained_e2e_accepted_with_identifier_m2`;
  deployed `whisper-tiny` is rejected for Pakistan M1. The pinned
  `faster-whisper-small-ur` worker path honors explicit `ur`, preserves absent-
  language detection, meaning, timestamps, provenance and manual review. Final
  deployed WER/CER is 0.6000/0.1852 and 0.3333/0.1446. Auto-detection still
  chooses Hindi; synthetic identifier preservation is partial and remains M2.
- BF-4 Video MVP: `composition_retained_e2e_accepted`; live
  metadata, 0/5-second frames, distinct ANPR observations, embedded audio,
  shared ASR, segment citations, provenance and temporary cleanup pass.
- BF-5/6 Integration and Acceptance: `retained_e2e_accepted_with_recorded_limitations`;
  six latest media generations completed, fifteen artifacts and seven KB
  mirrors are retained, and both historical dead-letter rows are preserved.
- Post-BF-A source slice: `source_accepted_pending_activation`. Deterministic
  dHash/exact/near comparison, scoped image comparison, identifier-safe Roman
  Urdu, native TXT/PDF/DOCX passages, semantic-evidence History citations,
  responsive Home/Data UX and the 97-operation maturity registry pass focused
  source tests. The authoritative guide/matrix is
  `docs/demo/nexusai-breadth-multimodal-demo-guide.md`.
- Open P0/P1/P2/P3: `0/2/3/0`. The two P1 gates are narrow worker activation
  and separately authorized retained proof. P2 covers face-model/fixture
  approval, general OCR/semantic image search, and rich modality drawers.
  Previously recorded citation-presentation and Home-overflow P2s are repaired
  in source but remain runtime/browser verification gates.
- Database migration: `not_needed`; new ANPR model: `not_needed`; face model:
  `approval_required`; worker deployment: `required_not_authorized`; retained
  mutation: `required_for_new_acceptance_not_authorized`; cleanup:
  `not_authorized`.

## STIM phase ledger

- STIM-0: `source_baseline_complete`. Repository skill and trigger hook,
  source-to-answer audit, family/format inventory, maturity matrix, triaged issue
  register, and grounded-narrative benchmark corpus are recorded.
- STIM-1: `source_accepted`. CDR adapter 1.4, explicit date-order/timezone
  states, fail-safe ambiguity, independent time goldens, and three-layout CDR
  canonical equivalence close both foundational time P1s.
- STIM-2: `source_accepted`. The shared registry, family profiles, dynamic
  attributes, quality/readiness, provenance and bounded Data UI cover CDR,
  IPDR, subscriber, tower, structured ANPR and generic infrastructure; financial
  and access remain truthfully partial, and INPR/INPRS remain undefined.
- STIM-3: `closed_source_and_runtime_accepted`. The reconciled
  66-operation registry has five certified/queryable, 50 explicitly limited,
  and 11 engineering-only operations. Structured scope is 65 after excluding
  the KB-only evidence operation: four certified, 50 limited, and 11
  engineering-only. Silently uncertified ordinary-user exposure is zero. Exact
  retained source/file/evidence/version membership now passes at runtime and
  rejects a tampered version with stable HTTP 400 semantics. Pakistan-equivalent
  raw phone forms now reach Go canonicalization and the seven-event oracle. The
  final source correction canonicalizes provider `VOICE`/`CALL` service tokens
  before duplicate/conflict comparison.
- STIM-4: `closed_source_and_runtime_accepted`. The bounded two-to-eight
  source CDR comparison and supported typed/time-valid relationship primitives
  pass independent goldens, anti-correlation, exact scope, citation,
  presentation, and performance checks. Broad public cross-family operations
  remain explicitly limited. The runtime correction replaces three repeated
  normalized-JSON scans with one bounded indexed candidate query and a single
  non-CDR parity fallback; the 5,000-row compiler measured 63.3869 ms. Its optional
  family-filter enum/text parameter boundary now passes. The final source
  correction text-normalizes both candidate branches and join after runtime
  exposed SQLSTATE 42804 at their enum/varchar UNION.
- STIM-5: `closed_source_and_runtime_accepted`. Multilingual deterministic query
  normalization, scoped follow-ups, clarification, `forensics.fact-packet/v1`,
  schema-bound `forensics.narrative/v1`, strict claim/reference validation,
  deterministic fallback and unsupported-capability preflight pass source gates.
- STIM-6: `closed_source_and_runtime_accepted`. Analyst Data, Ask and History expose
  source mapping/quality/readiness, progressive tables, proof roles, explicit
  result states, relationships, citations, limitations and Urdu RTL/BiDi.
- STIM-7: `closed_source_and_runtime_accepted`. The final gate preserves all
  prior boundaries and passes 46 structured goldens, forensic Go/vet and
  presentation suites, the 14-check multilingual live deck, the 65-operation
  runtime matrix, 11 independent retained-data oracles, the 677-module UI build,
  13 browser cases, and deployed responsive inspection with zero open P0/P1.
- Open issue summary: P0=0, P1=0, P2=4, P3=0.
- Deployment/runtime/data status: STIM-0 through STIM-7 and the full Structured
  Intelligence Maturity program are closed. `DeploymentNeeded=NO`,
  `AcceptanceDataIngestNeeded=NO`, and `DatabaseMigrationNeeded=NO`.
- Runtime evidence: the previously guarded R8 UI/API activation passed with worker, models,
  profiles, volumes and rollback images preserved; the 14-check live deck,
  65-operation matrix, 11-case Analyst Portal deck, rich-answer case, Urdu
  390px case and deployed `/analyst/*` inspection all passed. STIM-7 adds the
  retained-data 11/11 oracle and a fresh 65-operation matrix at 898.5 ms p95.
- Exact next action: stop at the post-STIM reconciliation boundary. Prepare a
  design-only proposal for Governed Runtime Query Intelligence; do not implement
  generalized runtime SQL/query generation or resume document/media phases.

Authority artifacts:
`docs/design/nexusai-structured-intelligence-maturity.md`,
`configuration/nexusai_stim_maturity_matrix.json`, and
`.agents/skills/nexusai-structured-intelligence-maturity/SKILL.md`.

## Preserved APF boundary

The final guarded APF-3.7 activation and full 2026-08-19 browser replay passed.
All 18 cases passed with zero P0/P1. Source/runtime parity, LocalAI readiness,
forensic health, Analyst HTML and security scope passed. The worker,
PostgreSQL, NATS, named volumes, retained evidence, KB data, models, profiles
and rollback images were preserved. `APF-3BrowserAcceptance=ACCEPTED`,
`APF-3.7RuntimeStatus=ACCEPTED`, `APF-3RuntimeStatus=ACCEPTED`,
`APF-3FinalStatus=CLOSED`, and `DeploymentNeeded=NO`.

APF-3.1 through APF-3.7 and overall APF-3 are source and live accepted. The exact APF-3.4 chain,
sub-second deterministic fast path, actual schema-bound assisted path,
English/Roman-Urdu/Urdu semantic equivalence, KB route, both hybrid operations,
12-column temporal presentation and bounded aggregate contribution lineage all
pass the final runtime deck. The final guarded marker records API 4.2 seconds,
LocalAI/UI 97.1 seconds, no worker/model/profile change, preserved volumes and
rollback images.

The generated query-variant ledger now contains 156 accepted occurrences, 144
unique cases and 12 duplicates; all 156 pass routing, parameter and semantic
expectations. The risk-tier operation-certification ledger has 66/66
registry/routing contracts: five certified/queryable, 50 bounded/limited and
11 bounded/engineering-only. Zero P0/P1 APF runtime
acceptance defects remain. The
individual-operation promotion debt stays in the controlled backlog and does
not masquerade as certification. Deferred P2
latency and P3 citation-label clarity do not reopen APF-3. The existing worker,
model, profile and volume state remains frozen. `NextProgram=STIM` and
`NextPhase=STIM-0`; that historical handoff has now been consumed by the STIM
phase ledger above.
Architecture authority:
`docs/design/nexusai-apf3-unified-query-intelligence-architecture.md`.

## Cross-cutting Definition of Done

- Reuse suitable existing APIs and functionality before extending them; add a
  reusable API only for a genuine capability gap in the active phase.
- Tests follow the material change and its risks; they are not a separate
  preliminary program.
- Fixtures and seed workflows are created just in time by the evidence-family
  phase that needs them. Pakistan-specific behavior follows the same rule.
- Preserve the supplied real CDR for read-only R7 validation and require
  approval before retained ingestion.
- Prefer deterministic tools, then installed approved models/backends; perform
  new model research/download only in the active model-dependent family phase.
- Provide repository-grounded PowerShell operations only after source/tests are
  ready and the build/download/deploy/data action reaches its approval gate.

Phase ownership: R3 UI/accessibility/assets; R4 case context; R5 evidence and
ingestion APIs/seeds; R6 agent/query/stream lifecycle; R7-R15 family APIs,
fixtures, regional behavior and models; R16 required administration APIs; R17
contract stabilization and continuous regression.

APF does not create or renumber a family phase. It is the latest team-lead
product-boundary correction across accepted R2-R6 foundations. APF-1 and
APF-UX-1 are source accepted on 2026-08-17: the distinct `/analyst` shell reuses
authoritative workspace, evidence, records, agent lifecycle,
presentation/citation and history contracts, while Analyst Experience V2 makes
Home action-oriented, Data and History searchable/filterable, and Ask
suggestions backend-capability-derived. Scoped lint and production build pass;
the focused portal/navigation/login set passes 21/21; real-backend browser QA
passes at 390/820/1024/1440 in light/dark with no overflow or console errors.
Deployment remains unchanged and Docker rebuild was not required. See
`docs/design/nexusai-analyst-portal-product-architecture.md`.

APF-2 runtime acceptance completed on 2026-08-17. The drawer resolves accepted
row truth by exact evidence ID from the loaded catalog, preserves authoritative
zero and renders missing accounting as unavailable. The retained source passes
4 canonical/accepted from 5 input, 1 duplicate and 0 rejected. The sole resumed
Ask run created completed deterministic analysis
`c540c875-0cd7-410d-bd6b-3f2be87b0336`, returned 4 exact results, cited and
opened the retained evidence, advanced History 50 to 51, and passed
navigation-only Continue in Ask. Playwright 10/10, scoped lint, production
build and responsive dark/light preview pass. Deployment remains unchanged.
APF-3 is next but not started; R8 remains valid and paused.

APF-2 is source accepted on 2026-08-17. The ordinary-analyst Data journey now
reuses the existing workspace-scoped collection upload, content-addressed
retention, evidence classifier, duplicate lookup, transactional queue, case
evidence pagination, capability registry and read-only reprocess plan. The UI
provides a multi-file modal and page drop target, two-concurrent registration,
actual upload progress, per-file partial outcomes, central Ready/Processing/
Needs attention/Failed mapping, Ready-only capability actions and truthful
approval-gated failure recovery with no execution control. Scoped lint has zero
errors; the 677-module build and focused portal Playwright 9/9 pass. Real
browser QA against 10 sources shows 9 Ready and 1 Failed and passes dark/light
modal and recovery inspection without retaining an upload. Deployment and
runtime data remain unchanged. The sole remaining APF-2 gate is an explicitly
approved upload of
`ingestion/forensic_records/tests/fixtures/pakistan_cdr_messy_synthetic.csv`;
that action will create retained evidence/job/record/custody state. A Docker
rebuild is not required to exercise the already deployed backend.

APF-2 retained-runtime acceptance began under explicit approval on 2026-08-17
and halted at the first unexpected behavior. One synthetic CDR fixture was
retained as evidence `4320772b-febe-4af2-b9e3-92cea114da0b`, version
`ad4b97c9-54eb-4657-afa7-5f06f52ba695`, completed job/run
`605b53d8-195f-4dde-aae6-081fbef6b500`, 4 canonical rows, 1 duplicate, one KB
asset and a valid 3-event custody chain. Source/Ready/accepted-record counts
advanced 10/9/9,274 to 11/10/9,278; Failed remains 1 and Processing/Needs
attention remain 0. The Add Data ledger became Ready and the Data catalog is
correct, but the source drawer displayed `0 verified rows` because detail omits
`item.accepted_rows` and the modal open callback carries no accepted-row value.
No Ask query/history entry followed, and no retry, reprocess, repair or cleanup
occurred. Runtime status is `blocked_on_ui_accounting_truth`; retained data is
preserved. Fix the bounded UI defect and resume against this evidence without a
second upload. APF-3 remains pending.

## R0

- phase_id: R0
- phase_name: Authoritative Reconciliation
- program_status: complete
- source_status: source_accepted
- deployment_status: not_applicable
- runtime_acceptance: read_only_state_verified
- data_acceptance: governed_inventory_verified
- historical_capabilities_reused: checkpoint ledger, dated acceptance reports, guarded runtime inventory
- completed_deliverables: 2026-08-06 reconciliation report; capability JSON; risk register; team-lead brief; directive/roadmap correction
- missing_deliverables: none for the bounded 2026-08-06 reconciliation
- entry_criteria: repository, checkpoint, live service/model/case state, tests, and reports inspectable without mutation
- exit_criteria: live state documented; existing work protected; unsupported next-plan assumptions identified; one bounded next objective selected
- approval_requirements: none for read-only/source governance work
- current_blockers: none
- next_action: maintain reconciliation evidence as later phases change truth

## R1

- phase_id: R1
- phase_name: Complete LocalAI Reverse-Engineering Audit
- program_status: source_accepted
- source_status: source_accepted
- deployment_status: not_applicable
- runtime_acceptance: not_applicable
- data_acceptance: not_applicable
- historical_capabilities_reused: prior repository inventories, route tests, backend matrix, forensic architecture
- completed_deliverables: six required architecture documents; 371-route plus 10 app registration inventory; 64-backend platform/disposition inventory; R1 source-acceptance report
- missing_deliverables: none at the R1 material architecture boundary; endpoint tests and dependency-license promotion checks belong to owning implementation phases
- entry_criteria: R0 complete and current source available
- exit_criteria: every major capability has a NexusAI disposition; no product plan relies on undocumented platform assumptions; compatibility boundaries are explicit
- approval_requirements: source inspection and documentation pre-authorized; runtime probing remains read-only
- current_blockers: none
- next_action: preserve the inventories and re-audit affected rows when upstream/source changes

## R2

- phase_id: R2
- phase_name: NexusAI Product and Brand Architecture
- program_status: source_accepted
- source_status: source_accepted
- deployment_status: not_started
- runtime_acceptance: not_started
- data_acceptance: not_applicable
- historical_capabilities_reused: dynamic branding provider, accepted Case Workspace patterns, responsive baselines
- completed_deliverables: accepted product definition, code-native/default asset set, tagline placement, role labels, navigation/evidence/administration terminology, existing branding/report fallback contract, app-shell/state/token contracts and visible LocalAI disposition rules
- missing_deliverables: none at the R2 source-contract boundary; asset rendering and per-route migration belong to R3
- entry_criteria: R1 source audit accepted
- exit_criteria: product identity and tokens established; LocalAI product-facing identity mapped; app-shell architecture source-approved; incremental migration is executable
- approval_requirements: source design accepted; deployment remains separately approval gated
- current_blockers: none
- next_action: preserve the accepted contract and reconcile it only when product evidence changes

## R3

- phase_id: R3
- phase_name: NexusAI App Shell and Deep White-Label Foundation
- program_status: complete
- source_status: source_accepted
- deployment_status: deployed_accepted
- runtime_acceptance: live_runtime_accepted
- data_acceptance: not_applicable
- historical_capabilities_reused: React router, providers, branding endpoint, authentication shell
- completed_deliverables: R3-UI-01 identity/loading/navigation foundation; R3-UI-02 source implementation with desktop/tablet route and enforced-role context, compact mobile route/case context, URL-bound case identifier, keyboard skip target and no new API; R3-UI-03 shared empty/error/forbidden/partial/unavailable presentation across shell, governed redirect, Knowledge, Talk and Home states with visible administration terminology contextualized to NexusAI
- missing_deliverables: none; R3 shell source, browser and live container acceptance are complete
- entry_criteria: R2 source acceptance and approved asset/config contract
- exit_criteria: NexusAI shell passes login/loading/error/main and 390/820/1024/1440 acceptance without breaking compatibility
- approval_requirements: later shell changes and redeployment require their own bounded approval
- current_blockers: none
- next_action: preserve the accepted shell contract; re-run its focused browser matrix when shell source changes

## R3.1

- phase_id: R3.1
- phase_name: NexusAI Elite Visual System and UI/UX Transformation
- program_status: complete
- source_status: source_accepted
- deployment_status: deployed_accepted
- runtime_acceptance: live_runtime_accepted
- data_acceptance: not_applicable
- historical_capabilities_reused: accepted R3 shell, routes, branding provider, themes, shared primitives and 27-contract browser suite
- completed_deliverables: mandatory current-UI audit; shared Intelligence Command System tokens and presentation layer; shell, navigation, Home, tables, forms, authentication/state and primary-route refinements; code-native Settings previews; targeted lint; 663-module production build; 34/34 production-browser contracts; responsive/light-dark/keyboard/branding/asset/console acceptance and user-visible demonstration
- missing_deliverables: none; source, production-browser, guarded deployment and live runtime acceptance are complete
- entry_criteria: accepted R3 source/browser baseline preserved
- exit_criteria: all 20 R3.1 visual, responsive, accessibility, branding, build and browser criteria pass without functional regression
- approval_requirements: later rebuild/redeployment, downloads, staging, commit, push and publication remain separate
- current_blockers: none
- next_action: preserve the accepted visual-system and live deployment contract while R4 proceeds

## R4

- phase_id: R4
- phase_name: Case-Centered Workspace Architecture
- program_status: complete
- source_status: R4_source_accepted_through_ARCH_06_plus_skip_focus_correction
- deployment_status: deployed_accepted
- runtime_acceptance: live_runtime_accepted
- data_acceptance: historical_case_data_preserved
- historical_capabilities_reused: governed case APIs, Case Workspace, deterministic operations, Agent Chat baseline
- completed_deliverables: R4-PRE-01 truthful initialization fix; R4-ARCH-01 case-context inventory and accepted contract; R4-ARCH-02 shared provider plus App Shell, Records redirect and Case Workspace identity migration with 35/35 protected browser acceptance; R4-ARCH-03 provider-bound embedded Records Intelligence with exact query/upload/list/evidence/delete/export scope, stale-response rejection and 46/46 protected browser acceptance; R4-ARCH-04 provider-bound forensic Agent Chat with explicit case/collection payloads, case-isolated conversations, stale SSE/HTTP rejection, generic-chat compatibility and 53/53 protected browser acceptance; R4-ARCH-05 explicit provider-authorized collection-to-case transitions, administrative collection boundary, case-scoped non-retained Reports module, dual-ID mismatch rejection, 664-module build, focused Go scope acceptance and live responsive report preview; R4-ARCH-06 reusable elite command-center chrome, complete Overview/Ask/Evidence/Relationships/Timeline/Media/Reports/Admin composition, truthful future-capability boundaries, compatibility redirects, 667-module build and protected Chrome acceptance 61/61
- deployment_progress: second guarded refresh PASS with one LocalAI attempt, rollback images and volumes preserved; corrected deployed asset index-BY8o0UmF.js; live health and isolated skip-focus checks pass; protected deployed matrix 61/61
- missing_deliverables: none at the bounded R4 architecture and runtime boundary
- entry_criteria: R3 shell accepted; R4-PRE-01 is live accepted through the guarded combined deployment
- exit_criteria: consistent case across routes; no stale/default/cross-case state; modular UI; accepted operations preserved
- approval_requirements: later R4 source changes or redeployment require separate approval
- current_blockers: none
- next_action: preserve the accepted case contract and 61-contract deployed baseline while R5 proceeds

## R5

- phase_id: R5
- phase_name: Evidence and Ingestion Product Experience
- program_status: complete
- source_status: source_accepted
- deployment_status: deployed_accepted
- runtime_acceptance: live_runtime_accepted
- data_acceptance: controlled_synthetic_intake_reconciled
- historical_capabilities_reused: evidence control plane, custody, queue, adapters, row accounting
- completed_deliverables: R5-EVID-01 case-scoped evidence detail API and authorization mapping; elite responsive Evidence Operations workspace; backend-authoritative catalog/detail inspection; exact accepted/duplicate/rejected row accounting; integrity, lineage, linked assets and actionable notes; safe local staging; R5-EVID-02 tenant/collection/evidence-bound canonical processing runs/events, derived artifacts with stable nexusai citation references, hash-chain custody integrity summary and append-only event history, interactive artifact locator, pipeline ledger, verified/unavailable custody UI; R5-EVID-03 exact look-ahead catalog pagination, status recent-jobs contract, server-backed catalog navigation, two-concurrent-worker registration and per-file durable outcome ledger; R5-EVID-04 case-bound read-only reprocess-plan API, published workflow contract, approval/eligibility inspection and no-execution Evidence Operations control; R5-EVID-05 pinned generated Swagger refresh, public operator documentation, guarded combined R5 activation and read-only runtime-contract gate; focused Go PASS; 669-module production build PASS; Case Workspace browser suite 14/14
- missing_deliverables: none at the bounded R5 product and runtime boundary
- entry_criteria: R4 workspace contract accepted
- exit_criteria: truthful upload-to-ready state, exact accounting, actionable errors, working citations
- approval_requirements: source slice approval under roadmap; ingestion/data mutation separately approved
- current_blockers: none
- next_action: preserve the accepted R5 runtime while R6 proceeds; reprocess execution and further retained ingestion remain separately approval-gated

## R6

- phase_id: R6
- phase_name: Ask NexusAI and Specialist Orchestration Experience
- program_status: complete
- source_status: complete
- deployment_status: R6.6_deployed
- runtime_acceptance: live_accepted
- data_acceptance: not_applicable
- historical_capabilities_reused: deterministic operation matrix, specialist agents, cited professional response presentation
- completed_deliverables: prior R6 completion set; R6.6-A target-required frequent contacts; numeric counterparty filtering and self-exclusion; direct source inventory and scoped readiness answers; clarification presentation; analyst-facing metric/table/provenance shaping; retained-payload compatibility normalization; compact Ask NexusAI identity/context; progressive table disclosure; R6.6 corpus/platform contract update; bounded same-case request-scoped follow-up filter context; explicit incoming/outgoing frequent-contact filter; all 65 catalog examples plus 30 realistic natural/mixed-language routing checks; focused API/presentation/UI/browser acceptance
- missing_deliverables: none at the approved R6/R6.6 boundary; non-blocking ideas are in the central backlog
- entry_criteria: R5 accepted; exact-fact authority and case-state contracts remain green
- exit_criteria: no stuck/stale/case-mismatched requests; deterministic fallback and accessible sources pass
- approval_requirements: later retained-data import remains analyst-confirmed in product; future rebuilds remain operator-gated
- current_blockers: none
- next_action: preserve the accepted R6.6 runtime and proceed through R7; reopen only for P0/P1 maintenance

## R7

- phase_id: R7
- phase_name: Pakistan CDR and Tower Intelligence Deepening
- program_status: complete
- source_status: R7.1_through_R7.7_and_R7.9_source_accepted
- deployment_status: R7.10_deployed_accepted
- runtime_acceptance: live_accepted
- data_acceptance: protected_read_only_accepted_no_retained_ingest
- historical_capabilities_reused: CDR/IPDR/subscriber/tower adapters and operations
- completed_deliverables: bounded phase-entry reconciliation; R7 telecom contract; explicit time-valid join outcomes; CDR adapter 1.2 Pakistan party/time/service provenance; subscriber adapter 1.2 role-separated subscriber/SIM/device/service semantics; tower adapter 1.2 provider/site/sector history, validity/datum/uncertainty provenance and overlap review; governed telecom language, clarification and bounded no-result acceptance; synthetic goldens; synchronized uncertainty-aware map/timeline/evidence UI; six specialist profiles; 65-operation deterministic and six-family model-assisted acceptance; result-bound, scope and locator hardening; guarded full deployment plus API bridge activation; responsive live acceptance
- missing_deliverables: none at the R7 governed boundary
- entry_criteria: R6 product workflow accepted
- exit_criteria: fixture agreement, citations, visible unmatched joins, no RF-presence or invented-route claims
- approval_requirements: source and benchmark work; deployment/data changes separately approved
- current_blockers: none
- next_action: preserve R7 contracts and proceed to R8; any future protected CDR retained ingest remains independently authorized

## R8

- phase_id: R8
- phase_name: ANPR Image and OCR Intelligence
- program_status: valid_but_paused_non_blocking
- source_status: R8.1_complete_R8.2_source_accepted_R8.3_platform_accepted_R8.4_T2V_sealed_hardware_governed_fastplate_real_diagnostic_pass_T2V_correctness_fail_blocked_no_promotion
- deployment_status: R8.2_R8.3_UI_API_live_worker_and_models_unchanged
- runtime_acceptance: UI_API_health_and_responsive_evidence_workspace_accepted_candidate_overlay_pending_live_image
- data_acceptance: T0_T1_accepted_T2P_deferred_T2V_96_sealed_all_measured_candidates_failed_T3_CCPD2019_downloading_official_five_image_diagnostic_acquired_no_accuracy_authority_T4_pending
- historical_capabilities_reused: structured ANPR records and deterministic image-header inventory
- completed_deliverables: approved execution contract; R8.1/R8.2 intake and live UI/API; R8.3 candidate/crop/overlay contracts; amended T0/T1/T2-P/T2-V/T3/T4 registry; deterministic T0/T1 pack; two-boundary gate; preserved 32-slot T2-P tooling; deterministic 96-image perspective-aware T2-V pack with exact polygons, hashes, development/validation/sealed-holdout partitions and anti-leakage guard; bounded detector/OCR T3 research; OMZ detector and Paddle/PARSeq/EasyOCR shortlists; exact evaluator provenance; explicit no-promotion decision
- missing_deliverables: T2-V candidate threshold acceptance; approved/downloaded/audited T3-D and T3-O real benchmarks; T4 operational agreement; passing specialized detector and OCR configuration; bounded OCR pipeline; correlation; review desk; live image/overlay acceptance and final runtime acceptance
- entry_criteria: R7 accepted; model/license/resource approval is required before any new detector or OCR download
- exit_criteria: precision/recall/OCR/provenance/abstention gates pass; candidates never presented as proof
- approval_requirements: synthetic visual pack and local model/backend evaluation installation approved on 2026-08-12; production role promotion remains evidence-gated
- current_blockers: Boundary A blocked because all measured detector/OCR candidates fail unchanged sealed T2-V gates and CCPD2019 is authorized but not downloaded/evaluated; no stronger Urdu plate-domain pretrained challenger is verified; T2-P remains deferred; Boundary B additionally lacks T4/live operational approval
- next_action: preserve all artifacts, rejection evidence and the stopped CCPD partial without resuming transfer, evaluation or promotion during APF; after the shared product foundation is accepted, resume only through the unchanged Boundary A/B gates and do not start R8.5 before Boundary A

## R9

- phase_id: R9
- phase_name: General Image Forensics
- program_status: not_started
- source_status: inventory_baseline_only
- deployment_status: not_started
- runtime_acceptance: not_started
- data_acceptance: not_started
- historical_capabilities_reused: deterministic metadata and hashes
- completed_deliverables: none
- missing_deliverables: OCR, objects, perceptual hashing, embeddings, similarity, screenshots and policy-gated identity features
- entry_criteria: R8 accepted
- exit_criteria: stable citations, labeled candidates, policy enforcement, measured CPU/latency
- approval_requirements: legal/privacy/model approvals where applicable
- current_blockers: R8 and policy gates
- next_action: wait for R8

## R10

- phase_id: R10
- phase_name: Urdu/English Audio and Speech Intelligence
- program_status: not_started
- source_status: inventory_baseline_only
- deployment_status: not_started
- runtime_acceptance: not_started
- data_acceptance: not_started
- historical_capabilities_reused: audio metadata and derived-artifact lineage
- completed_deliverables: none
- missing_deliverables: ASR benchmark, VAD, timestamps, search, speaker turns/diarization, citations, disclosed TTS and audio UI
- entry_criteria: R9 accepted and approved local-model benchmark
- exit_criteria: WER/timestamp/language/provenance/failure-preservation gates pass
- approval_requirements: model download/deployment and representative-data approval
- current_blockers: no approved ASR model installed
- next_action: wait for R9

## R11

- phase_id: R11
- phase_name: Video Intelligence
- program_status: not_started
- source_status: header_inventory_only
- deployment_status: not_started
- runtime_acceptance: not_started
- data_acceptance: not_started
- historical_capabilities_reused: media metadata inventory
- completed_deliverables: none
- missing_deliverables: scenes/frames/audio/STT/detections/timeline/search/citations/video workspace
- entry_criteria: R10 accepted and bounded resource design approved
- exit_criteria: recall/alignment/linkage/persistence/resource/long-video gates pass
- approval_requirements: model and representative-video approvals
- current_blockers: R8-R10 modality dependencies
- next_action: wait for R10

## R12

- phase_id: R12
- phase_name: Financial and Access/Security Intelligence
- program_status: not_started
- source_status: historical_structured_baseline_only
- deployment_status: not_started
- runtime_acceptance: not_started_for_R12
- data_acceptance: not_started_for_R12
- historical_capabilities_reused: PKR transactions and structured security-log adapters
- completed_deliverables: none
- missing_deliverables: dedicated adapters/agents/UI, exact currency rules, cross-family correlation, privacy and exports
- entry_criteria: R11 accepted
- exit_criteria: exact accounting/currency/timestamps/citations and no unsupported fraud labels
- approval_requirements: representative-data and deployment approvals
- current_blockers: earlier phases
- next_action: wait for R11

## R13

- phase_id: R13
- phase_name: Document and Knowledge Intelligence
- program_status: not_started
- source_status: safe_inventory_and_KB_baseline_only
- deployment_status: not_started
- runtime_acceptance: not_started
- data_acceptance: not_started
- historical_capabilities_reused: PDF inventory, KB/RAG, evidence lineage
- completed_deliverables: none
- missing_deliverables: native extraction, OCR fallback, tables/attachments/layout, page-region citations, viewer and report integration
- entry_criteria: R12 accepted and extraction/OCR benchmark approved
- exit_criteria: extraction/OCR/retrieval/citation gates pass with partial-failure and source preservation
- approval_requirements: model/backend and data approvals
- current_blockers: no approved OCR/document stack
- next_action: wait for R12

## R14

- phase_id: R14
- phase_name: Captures, Databases and Archives
- program_status: not_started
- source_status: safe_inventory_baseline_only
- deployment_status: not_started
- runtime_acceptance: not_started
- data_acceptance: not_started
- historical_capabilities_reused: PCAP/PDF/archive/SQLite bounded inventories and hostile-input fixtures
- completed_deliverables: none
- missing_deliverables: deep PCAP/EVTX/database/archive adapters, child lineage and bounded query UI
- entry_criteria: R13 accepted
- exit_criteria: no execution/traversal/decompression escape; exact inventory; truthful preservation/rejection
- approval_requirements: security review and representative-data approval
- current_blockers: R13 incomplete
- next_action: wait for R13

## R15

- phase_id: R15
- phase_name: Entity and Relationship Intelligence
- program_status: not_started
- source_status: historical_entity_baseline_only
- deployment_status: not_started
- runtime_acceptance: not_started_for_R15
- data_acceptance: not_started_for_R15
- historical_capabilities_reused: canonical entities and exact cited correlations
- completed_deliverables: none
- missing_deliverables: resolution/aliases/conflicts/time validity/evidence graph and relationship workspace
- entry_criteria: R14 accepted
- exit_criteria: no model-only identity; every relation cited; contradictions visible; zero tenant/case leakage
- approval_requirements: privacy/security/data approvals
- current_blockers: family depth and UI prerequisites
- next_action: wait for R14

## R16

- phase_id: R16
- phase_name: Enterprise Administration and Scale
- program_status: not_started
- source_status: upstream_and_historical_foundation_only
- deployment_status: not_started
- runtime_acceptance: local_profile_not_enterprise_accepted
- data_acceptance: not_applicable
- historical_capabilities_reused: LocalAI auth/API keys/quotas/OIDC, forensic RLS/NATS/storage, distributed primitives
- completed_deliverables: none
- missing_deliverables: tenant/role policy, retention/legal hold, secure distributed workers, backup/restore, monitoring/SLO/load/security/accessibility
- entry_criteria: R15 accepted and enterprise deployment profile approved
- exit_criteria: isolation, recovery, concurrency, scale, audit, accessibility and operations gates pass
- approval_requirements: security architecture and infrastructure approvals
- current_blockers: no enterprise deployment acceptance
- next_action: wait for R15

## R17

- phase_id: R17
- phase_name: Release and Continuous Regression
- program_status: not_started
- source_status: historical_regression_assets_only
- deployment_status: not_started
- runtime_acceptance: not_started
- data_acceptance: not_started
- historical_capabilities_reused: phase reports, goldens, smoke scripts, rollback tags and CI
- completed_deliverables: none
- missing_deliverables: unified fixed gold sets, model/adapter/UI/security/performance regression, upgrade/upstream merge/customer artifacts and support docs
- entry_criteria: R16 accepted and release candidate scoped
- exit_criteria: truthful release inventory, rollback and upgrade proven, all production gates pass
- approval_requirements: release, publication, staging, commit and push require explicit approval
- current_blockers: R0-R16 program not complete
- next_action: maintain reusable regression evidence until release-candidate entry

## 2026-08-25 MMV-2 final enterprise-semantics ledger

- `MMV2-P1-A-POSITIVE-SUMMARY`: SOURCE_ACCEPTED. Authoritative one-row and
  multi-row results override stale zero/no-match metadata.
- `MMV2-P1-B-COMPLETE-ZERO`: SOURCE_ACCEPTED. Public response, agent
  presentation and History preserve `complete_zero_results`, completed
  processing state, zero rows and operation identity.
- `MMV2-LIVE-CLOSURE`: DEPLOYMENT_APPROVAL_GATED. Required services are only
  forensic API and LocalAI/UI. Source-open P1=0; live-open P1=2. No retained
  mutation or MMV-3 work is authorized.
