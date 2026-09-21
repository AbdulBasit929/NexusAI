# NexusAI product engineering backlog

## 2026-08-31 NX-MMR next breadth — active owner

Single work item: `NX-MMR-NEXT-MULTIMODAL-BREADTH-V1`. Evidence:
`reports/nexusai-nxmmr-speech-live-closure-v1-20260831.md` supersedes the earlier
breadth report for current speech/runtime facts. No parallel phase.

| ID / severity | Evidence and impact | Disposition / acceptance |
|---|---|---|
| SPEECH-P1-WORKER / P1 | Fresh English upload has only technical observation; ASR/face/SigLIP flags false and SigLIP mount absent | SOURCE READY / LIVE GATED; sealed three-service bundle preserves ANPR/OCR/V3, builds sequentially and rechecks the 6-GiB gate before every build and recreation |
| SPEECH-P1-READY / P1 | accepted_rows=1 technical observation rendered “Machine transcript ready”; detail had zero transcripts | SOURCE FIXED; list wording neutral, detail uses transcript artifacts, banner conditional; deploy and replay separately |
| SPEECH-P1-SCOPE-QUERY / P1 | Two fresh audio questions returned unrelated document citations; evidence scope label not transported; audio intent/capability/exact/time/current-version gaps | SOURCE RESOLVED / LIVE GATED; selected scope transport, typed routing, exact/time/current-version semantics and 16/16 source matrix pass; fresh live replay pending activation |
| SPEECH-P2-DURATION / P2 | Existing Urdu player knows duration but summary did not consume deterministic duration_ms | SOURCE FIXED; unknown/boolean time stays unknown, explicit ms conversion tested; live activation pending |
| NEXT-P1-ASR-TIME / P1 | Text-only ASR fabricated clip-wide timing; null UI time became zero; same-time derivative pairing was not lineage | SOURCE FIXED; 9 ASR tests + presentation tests; deploy separately and verify source seek/unknown-time/parent-ID UI |
| NEXT-P1-DATA-PAGE / P1 | Live Audio filter showed no matches on loaded page, then three after Load more | SOURCE FIXED; bounded counts/empty copy disclose loaded scope; live acceptance after separate UI activation |
| NEXT-P1-SUGGESTIONS / P1 | Catalog checks PRODUCT_CERTIFIED; mediaQuickActions does not consult it; stored Ask exposes historical suggestions | OPEN; reconcile shared certified action gate or explicitly governed internal-demo exception; do not certify or rewrite stored answers |
| NEXT-P2-LEGACY-ACTIVITY / P2 | Stored Aug-23 audio evidence-detail answer reopens as a raw Markdown heading | Presentation follow-up; preserve stored payload, render safely with useful source navigation |
| NEXT-DEPLOY-AUTH / review gate | Ports 8080/8091 published on all interfaces; LocalAI loopback inference anonymous; forensic API rejects anonymous | Review intended exposure/auth and current upstream hardening before non-local demo; remote reachability untested |
| NEXT-DATA-ENGLISH / admission gate | Ten fixed official FLEURS validation clips acquired with frozen publisher oracles; 80 seconds; independent scoring | CLOSED for this bounded read-speech cohort; not Pakistani-English/noise/population qualification |
| NEXT-TTS-ENGLISH / admission gate | Voice and v4.9.0 linux-amd64 Piper layer pinned; known voice/runtime subtotal 97,275,960 bytes | BLOCKED; one admission packet remains: extracted installed size plus packaged transitive SBOM/license/notice closure; no download approval requested |
| NEXT-TTS-URDU / admission gate | Matcha revision/hash/size pinned; required `charactr/vocos-mel-22khz` is not publicly resolvable | BLOCKED; exact vocoder hash/license/size/22.05-kHz compatibility cannot be admitted; no alternate or download |

ANPR/OCR robustness, false positives, handwriting, cadence, thresholds and
model tuning remain deferred until breadth completion. Existing failed image
negative is not a successful zero-detection oracle. No sealed holdout reuse.

## 2026-08-24 MMV-2 final P1 source closure

- **MMV2-P1-URDU-ANPR-ROUTING — SOURCE FIXED / LIVE GATED:** generic Urdu
  ANPR intent now preserves arbitrary ASCII plate identifiers and resolves to
  the governed operation; no demo phrase or identifier drives production.
- **MMV2-P1-PUBLIC-VIDEO-ANPR — SOURCE FIXED / LIVE GATED:** exactly one
  evidence-scoped `video.anpr_grouped_timeline` operation supports optional
  plate/time filters, citations, review state and truthful complete-zero.
- **MMV2-P1-FINAL-ACTIVATION — APPROVAL REQUIRED:** rebuild/recreate only
  forensic API and LocalAI/UI. First tag the current running API and LocalAI/UI
  images for rollback and pass 6 GiB free physical RAM. Worker, PostgreSQL,
  NATS, models and retained data are unaffected/protected.
- **MMV2-P1-RETAINED-POSITIVE — NOT READY FOR APPROVAL:** hash/size pass, but
  ownership/retention attestation and human crop review are still required.
  The artifact delta is model-dependent and must be reconciled after any
  separately approved normal upload; no stale scalar prediction is accepted.

## 2026-08-24 MMV-2 bounded source progress

- **MMV2-P1-SOURCE-TIMESTAMP — SOURCE FIXED:** FFmpeg source timestamps now
  come from `showinfo`; index-derived false timestamps are removed.
- **MMV2-P1-FULL-DURATION-ANPR — SOURCE FIXED:** FastALPR uses bounded 1-second
  cadence (60 default/120 hard cap); heavier frame roles remain at 5 seconds.
- **MMV2-P1-PLATE-GROUPING — SOURCE FIXED:** equal normalized OCR candidates
  group for presentation without deleting raw observations or claiming tracking.
- **MMV2-P1-ACTIVATION — OPEN, APPROVAL-GATED:** deploy the worker/UI/API source
  boundary and run live tests; do not reprocess existing retained evidence.
  Approval is now present, but the first attempt safely stopped before building:
  mandatory free RAM was 6 GiB and the bounded stop reached only 2.56 GiB.
  Rollback tags exist and the original healthy runtime is restored.
- **MMV2-P1-RETAINED-POSITIVE — OPEN, APPROVAL-GATED:** normal upload of the
  exact approved video scope only after separate authorization.
- **MMV2-P1-MEDIA-OPERATIONS — OPEN:** current artifact/detail/comparison APIs
  back several media capabilities, but the full requested ANPR/image/video Ask,
  citation and History operation set is not yet independently certified.
- **MMV2-P2-PAKISTAN-ANPR-ROBUSTNESS — MEASURED LIMITED:** 7/13 PASS, 2/13
  PARTIAL and 4/13 FAIL; no population-accuracy claim.

## 2026-08-24 MMV-1 reconciliation backlog

The authoritative machine issue list is
`reports/mmv1-real-world-validation-20260824/gap-backlog-matrix.json`.

- **MMV-P1-VIDEO-ZERO-LIMITATION — SOURCE FIXED:** frame-level zero-result
  messages no longer become contradictory video-global limitations when a
  later sampled frame contains a plate, face, or OCR result.
- **MMV-P1-AUDIO-PRESENTATION — SOURCE FIXED:** MIME/family-first audio typing,
  artifact-derived readiness, audio facts and native playback are present in
  Analyst Data. Deployment and live browser acceptance are separate gates.
- **MMV-P2-ANPR-ROBUSTNESS / VIDEO-SAMPLING — MMV-2:** the positive control
  proves the current pipeline can detect a real plate; six samples ending at 25
  seconds do not establish long-video or Pakistan population robustness.
- **MMV-P2-URDU-OCR / ASR-IDENTIFIERS / AUDIO-CORPUS — MMV-3:** current OCR CER
  is inadequate, spoken plate preservation failed, and lawful natural
  Pakistani-English/code-switch/noise coverage remains acquisition-gated.
- **MMV-P2-FACE-DIVERSITY — MMV-4; MMV-P2-DOCUMENT-URDU — MMV-5;
  MMV-P2-OPERATION-GAPS — MMV-6; MMV-P2-QUERY-CERTIFICATION — MMV-7.**
- P3 tracking, diarization, cross-modal timelines and advanced VLM work remain
  assigned to MMV-9/MMV-12 and do not block forward progress.

## 2026-08-23 post-BF-A breadth completion backlog — RECONCILED, RETAINED PROOF GATED

- **POST-BF-P1-WORKER-ACTIVATION-006 — CLOSED:** FastALPR, accepted Urdu ASR
  and the bounded face role are live in the rebuilt healthy worker. Rollback is
  preserved; PostgreSQL/NATS IDs and all `27/27/15/12932/25` retained counts
  are unchanged; no BF-A reprocess/upload or migration occurred.
- **POST-BF-P1-RETAINED-PROOF-007 — OPEN, APPROVAL-GATED:** after activation,
  run a separately authorized isolated retained matrix for ANPR, fingerprint,
  comparison, Roman Urdu and native documents. Existing BF-A evidence remains
  on review hold and may not be reprocessed implicitly.
- **POST-BF-P2-FACE-MODEL-008 — CLOSED LIMITED:** the checksum-pinned 26.1 MB
  YuNet/SFace model, synthetic reviewed fixture pack, detection-only LocalAI
  route, worker observation/crop contract and server-owned evidence-scoped
  similarity endpoint pass source and live non-retained acceptance. Similarity
  remains a candidate review signal, never identity. OCI backend signature
  verification remains an explicit supply-chain limitation.
- **POST-BF-P2-IMAGE-SEMANTIC-OCR-009 — CLOSED LIMITED:** checksum-pinned
  SigLIP image embeddings and official Tesseract `eng+urd` general OCR are live
  in the existing worker. Synthetic benchmark verdict is
  `M1_ACCEPTABLE_LIMITED`; Urdu text-image top-1 failed, general OCR remains
  review-required, and retained artifact proof is separately approval-gated.
- **POST-BF-P2-MODALITY-DRAWERS-010 — CLOSED LIMITED:** Data now renders
  authorized image/audio/video previews and contract-specific derived cards,
  including the exact `candidate visual similarity; not identity` boundary.
  Current actions come from exact persisted artifact types; unretained roles are
  not advertised. Richer timeline/table geometry remains later-family polish.

- **BF-P1-AUDIO-QUALITY-001 — RESOLVED_FOR_M1:** the installed `whisper-tiny` pipeline is
  operational, but two lawful FLEURS `ur_pk` clips failed practical Urdu
  transcription (indicative WER 1.0000/1.4583) and the synthetic mixed
  identifier fixture failed exact preservation. Evaluate a bounded, explicitly
  approved `Systran/faster-whisper-small` challenger; its published
  repository is 486 MB and its reference INT8 CPU benchmark uses 1,477 MB RAM.
  The challenger is now accepted but not deployed; identifier robustness is M2.
- **BF-P1-ASR-ACTIVATION-002 — CLOSED:** the multipart HTTP and Python backend
  boundaries now preserve governed language guidance. The pinned small model is
  live, explicit-Urdu worker and video smokes pass, automatic detection remains
  available, rollback is preserved, and retained counts are unchanged.
- **BF-P1-ASR-UPLOAD-LANGUAGE-003 — CLOSED:** BF-A
  preflight proved the sidecar upload did not persist explicit ASR language in
  job metadata. The bounded validator/admission correction and focused tests
  pass; the guarded forensic API deployment and durable language admission are
  accepted.
- **BF-P1-MEDIA-PERSISTENCE-004 — CLOSED:** retained
  BF-A processing exposed missing runtime-role INSERT permission on
  `forensic.derived_artifacts` plus the worker's omission of durable
  `asr_language` from processor metadata. The one-table grant, rolled-back RLS
  probe, worker-only activation and immutable reprocessing passed.
- **BF-P1-MEDIA-COMPLETION-TYPING-005 — CLOSED:** the
  accepted grant allowed artifact persistence to reach completion marking,
  where untyped `jsonb_build_object` string parameters failed. Explicit text
  casts and focused coverage pass; worker-only recovery and immutable image
  reprocessing completed without changing protected services.
- **BF-P1-RETAINED-E2E-001 — CLOSED WITH LIMITATIONS:** the authorized retained
  run completed six latest media generations, fifteen artifacts and seven KB
  mirrors without re-upload or deletion. Retained ANPR extraction is not
  certified because no approved local vision/ANPR role is configured.
- **BF-P2-AGENT-CITATION-PRESENTATION-002 — CLOSED LIMITED:** enterprise
  semantic evidence, fact-packet citations and stored provenance feed History;
  deployed browser replay passed. The reviewed legacy BF-A analysis truthfully
  reports `citation state not reported`, so the UI does not invent a citation.
- **BF-P2-HOME-HISTORY-OVERFLOW-003 — CLOSED:** deployed Home/Data/Ask/History
  passed at 390/820/1024/1440 with main navigation present and zero page-level
  horizontal overflow in all 16 combinations.
- **BF-P2-ANPR-ACCURACY-001 — DEFERRED:** adverse-condition Pakistan plate and
  regional OCR accuracy plus review ergonomics.
- **BF-P2-VIDEO-TRACKING-001 — DEFERRED:** tracking and cross-frame deduplication.
- **BF-P2-MEDIA-PERFORMANCE-001 — DEFERRED:** throughput, long-video budgets and
  OpenVINO Windows provider repair.
- **BF-P3-POLISH-001 — DEFERRED:** richer artifact/timeline presentation.

Governed Runtime Query Intelligence, generalized model-authored SQL and deeper
document/RAG maturity remain deferred until the breadth baseline. STIM stays closed.

## 2026-08-20 STIM-7 final acceptance and program closure — CLOSED

The complete STIM-7 source/runtime/product matrix passed with P0=0, P1=0,
P2=4, and P3=0. Evidence includes 46/46 structured goldens, forensic Go/vet
and presentation suites, 14/14 multilingual live checks, 65/65 operations with
one accepted no-result, 11/11 independent retained-data oracles, the
677-module build, 13/13 browser cases, and deployed 390/820/1024/1440
inspection without overflow or console errors. No deployment, migration,
retained ingest, worker/model/profile change, or model download occurred.

- **STIM-4-P2-INDEX-001 — OPEN, NON-BLOCKING:** the final three-source retained
  oracle completed in 205.9 ms and the 65-operation matrix completed at 898.5 ms
  p95. Correctness and bounded execution pass; representative large-case
  p50/p95 plus read-only EXPLAIN remain post-STIM reliability evidence before
  any reversible index proposal. No speculative migration is authorized.
- **STIM-7-P2-CAPABILITY-AVAILABILITY-001 — OPEN, NON-BLOCKING:** registration
  and current indexed readiness remain distinct. Execution is fail-safe; the
  label refinement moves to post-STIM product reliability.
- **STIM-5-P2-LLM-001 — OPEN, NON-BLOCKING:** deterministic Fact Packets and
  validator/fallback make system factual delivery independent of raw model
  quality. Challenger evaluation remains approval-only; no download is
  authorized.
- **STIM-0-P2-DOMAIN-001 — OPEN, NON-BLOCKING:** INPR/INPRS remain
  `NeedsDomainDefinition` and must not be implemented or advertised without an
  owner-supplied semantic contract and authorized schema.

The immediate post-STIM priority is a design-only Governed Runtime Query
Intelligence architecture. It must use typed allowlisted primitives, certified
operations, exact scope, budgets, provenance, authorization, independent
oracles and stable failure semantics. General model-authored SQL is not
authorized. Documents/RAG and media work do not begin under this handoff.

## 2026-08-20 STIM-5/STIM-6 source/runtime acceptance — superseded closed checkpoint

Machine-readable authority:
`configuration/nexusai_stim_maturity_matrix.json`. Current summary is P0=0,
P1=0, P2=4, P3=0. The matrix records evidence, analyst/correctness/security
impact, active-phase relevance, decision, reason, target phase, acceptance
condition, and status for every issue.

- **STIM-5-P2-LATENCY-001 — CLOSED:** unsupported requests are assessed
  before language assistance and return through `capability_guard` with zero DB,
  KB and model execution. The live deck confirmed the under-five-second policy.
- **STIM-5-P2-LLM-001 — OPEN, NON-BLOCKING:** Fact Packet, strict narrative,
  claim/reference validator and deterministic fallback are source accepted.
  Direct installed-Qwen probes produced schema-valid but factually rejected
  answers and about 21--45 second latency. The model therefore remains optional
  and validator-protected; accepted factual delivery never depends on its raw
  output. Challenger evaluation is deferred and no download is authorized.
- **STIM-6-P2-PROVENANCE-PRESENTATION-002 / STIM-6-P2-UI-001 /
  STIM-6-P3-CITATION-001 — CLOSED:** proof roles now distinguish
  representative evidence from aggregate lineage; server-side column loss is
  removed; progressive UI controls retain all governed columns; citations are
  explicitly labeled.
- **STIM-5/STIM-6 runtime boundary — CLOSED:** guarded activation, the 14-check
  live deck, 65-operation matrix, 11-case Analyst Portal deck, rich-answer and
  Urdu-mobile cases, and deployed `/analyst/*` inspection passed with zero
  P0/P1. No migration, retained ingest, worker/model/profile change, or model
  download occurred. STIM-7 was separately gated and not started at this
  historical checkpoint; the closure section above supersedes it.

## 2026-08-19 STIM-3/STIM-4 runtime correction — CLOSED

- **STIM-3-P1-SOURCE-MEMBERSHIP-001 — SOURCE CORRECTED:** exact authorized
  source/file/family/evidence/version membership now runs before analytical SQL;
  invalid membership returns the non-enumerating `invalid_source_membership`
  contract and execution-count tests prove the analytical query is not called.
- **STIM-4-P1-CROSS-FAMILY-PERFORMANCE-001 — SOURCE CORRECTED:** one bounded
  `record_entities` candidate query plus one non-CDR parity expansion replaces
  three repeated whole-scope JSON expansions. Typed validity and anti-
  correlation goldens remain green; 5,000-row compilation measured 63.3869 ms.
  Both P1s remain open until corrected-runtime replay passes.

- **STIM-1-P1-TIME-001 / STIM-1-P1-TIME-002 — CLOSED:**
  `forensics.time-policy/v1` rejects unresolved slash-date ambiguity and unknown
  timezone for naive timestamps; intake has no implicit Pakistan timezone.
- **STIM-1-P2-PROFILE-001 / STIM-2-P2-DYNAMIC-001 — CLOSED:** one governed
  registry and versioned mapping/dynamic/quality/provenance contracts extend
  through the prioritized families and existing JSON persistence.
- **STIM-3-P2-CERT-001 — CLOSED WITH EXPLICIT DISPOSITION:** the derived
  registry has 66 operations: five certified/queryable, 50 explicitly limited,
  and 11 engineering-only. Structured scope is 65 after excluding the KB-only
  evidence operation: four certified, 50 limited, 11 engineering-only.
  Silently uncertified ordinary-user exposure is zero. Limited operations may
  only be promoted by independent family packs.
- **STIM-4-P2-INDEX-001:** the selected-source CDR query is exact-scoped,
  read-only, and bounded to 5,000 observations, but no live `EXPLAIN` or
  representative retained-case p50/p95 was authorized. Measure after guarded
  activation before proposing any database index; do not add a speculative
  migration.
- **STIM-4-P2-STALE-ELIGIBILITY-METADATA:** regenerate the retained eligibility
  snapshot after approved normal-path acceptance ingest; do not mutate it now.
- **STIM-6-P2-PROVENANCE-PRESENTATION-002:** underlying corrected results retain
  locators; improve partial/representative aggregate presentation later.
- **STIM-7-P2-CAPABILITY-AVAILABILITY-001:** distinguish registered capability
  from current indexed-record readiness without fabricating availability.
- **STIM-5-P2-LATENCY-001 / STIM-5-P2-LLM-001:** the accepted capability guard
  and bounded synthesis cases observed about 60 and 47.6 seconds respectively;
  the future Fact Packet contract is not yet implemented or benchmarked.
- **STIM-6-P2-UI-001 / STIM-6-P3-CITATION-001:** answer tables cap selected
  columns at seven and representative versus complete aggregate proof can be
  labeled more explicitly.
- **STIM-0-P2-DOMAIN-001:** INPR/INPRS are `NeedsDomainDefinition`; do not invent
  or advertise semantics without an owner-supplied expansion, contract, and
  authorized schema.

No item authorizes retained-data mutation, database migration, model/profile
change, or reopening APF-3. STIM-3/STIM-4 require one separately authorized,
guarded API-only activation before runtime acceptance; source acceptance alone
does not authorize execution.

## 2026-08-19 APF-3 browser correctness blockers — CLOSED

- **type:** `runtime_acceptance_correctness`
- **severity:** P1; closed by final activation and 18/18 replay
- **affected phase/module:** Analyst Ask deterministic targeting, IPDR routing,
  and bounded-composition delegation
- **finding:** the fresh post-deployment 18-case browser replay passed 14 cases
  and failed Cases 7, 10, 16 and 17. Remaining defects were deterministic typo
  routing, absent IPDR aggregate provenance, and legacy bounded-composition
  dependency/presentation semantics. No P0 was observed.
- **closure evidence:** the corrected source was activated through the guarded
  API/UI gate; source/runtime parity, health and preservation passed. The fresh
  browser replay passed 18/18 with zero P0/P1. Corrected Cases 7, 10, 16 and 17
  passed with deterministic routing, representative retained-row provenance,
  and truthful bounded dependency/no-result presentation.
- **prohibited workaround:** manual target repair, skipping failed cases,
  claiming source tests as runtime acceptance, or starting STIM before APF-3.
- **owner/next slice:** closed; do not reopen for deferred performance or polish.

## 2026-08-19 APF-3 capability-guard latency — DEFERRED

- **type:** `deterministic_guard_latency`
- **severity:** P2; does not block APF-3 closure
- **affected phase/module:** unsupported/missing-input capability guard
- **finding:** browser Case 18 returned the correct unsupported audio response
  without SQL, KB or model execution, but took approximately 60 seconds.
- **required closure:** profile the pre-execution assistance/timeout path in the
  owning reliability phase without weakening fail-closed behavior.
- **owner/next slice:** R17 reliability hardening after APF-3 closure.

## 2026-08-19 APF-3 bounded synthesis latency — DEFERRED

- **type:** `grounded_model_latency`
- **severity:** P2; does not reopen APF-3
- **affected phase/module:** evidence-package bounded synthesis
- **finding:** browser Case 15 was semantically correct but recorded 47,616 ms
  LLM latency and approximately 49.3 seconds UI-observed latency.
- **owner/next slice:** STIM model/query-performance assessment.

## 2026-08-19 IPDR representative-citation labeling — DEFERRED

- **type:** `provenance_presentation_clarity`
- **severity:** P3; does not reopen APF-3
- **affected phase/module:** IPDR endpoint aggregate presentation
- **finding:** Case 10 provided evidence ID, source-file SHA-256, source row and
  row hash for each representative citation. The raw `version_id` field was
  null and the UI did not explicitly label the citation as representative
  rather than complete aggregate-contribution lineage.
- **owner/next slice:** owning IPDR/UI hardening phase; preserve the distinction
  if aggregate contribution-lineage certification is later expanded.

## 2026-08-18 APF-3 bounded operation-certification debt — OPEN

- **type:** `independent_operation_truth_expansion`
- **severity:** P2 for Tier B, P3 for Tier C; non-blocking to APF-3 source closure
- **affected phase/module:** forensic operation certification and future family packs
- **finding:** 65 executable operations are registered and security-gated. Tier
  A is 4/4 fully certified; 50 Tier-B and 11 Tier-C operations lack complete
  independent fixture/oracle/result/presentation/citation evidence. No known
  correctness defect is recorded.
- **control:** Tier B is limited to explicit requests and cannot be suggested;
  Tier C is engineering-only and not queryable. A bounded operation cannot be
  labeled certified, and a known defect cannot be exposed as queryable.
- **required closure:** add genuinely independent family fixtures/oracles and
  promote individual operations only after routing, parameters, calculation,
  citations, presentation and security all pass.
- **prohibited workaround:** using implementation output as its own oracle,
  advertising 65/65 certification, or widening exposure to hide debt.
- **owner/next slice:** owning evidence-family phases; do not reopen APF-3.

## 2026-08-17 APF-3 catalog reconciliation blocker — CLOSED

- **type:** `foundational_contract_parity`
- **severity:** P1 bounded APF-3.1 blocker, closed in source 2026-08-17
- **affected phase/module:** APF-3 query intelligence and capability resolution
- **finding:** executable source publishes 65 query templates while the older
  platform catalog publishes 50 operation descriptors. The capability endpoint
  separately defines 19 evidence families; these are complementary dimensions,
  not interchangeable operation authorities.
- **required closure:** one derived capability projection and a parity test that
  maps every executable operation to an executor, input/result/presentation and
  citation contract, while rejecting orphan descriptors.
- **prohibited workaround:** adding a second portal router/registry, dropping
  accepted operations, or allowing a model to invent operation IDs.
- **closure evidence:** one derived projection covers all 65 executable
  operations; every operation has a stable descriptor and reverse lookup; 50
  historical platform descriptors remain, producing 80 total registry entries;
  focused parity and security regression passes.
- **owner/next slice:** closed; superseded by the accepted APF-3.7 projection.

## 2026-08-13 R8 hardware/candidate/T3 acquisition update

- **completed:** D0/D1/P1/P2 governance, measured D0 inventory, fair-failure
  audit, OMZ 0123 deferral, exact small-candidate and T3 acquisition contracts,
  privacy policy and hostile-archive gate.
- **selected next:** FastPlateOCR `cct-s-v2-global` v1.1.0; its 5,263,955-byte
  evaluation-only package is acquired and checksum verified, not measured or
  promoted. Its pinned offline CPU evaluator and preflight pass; image build and
  inference wait for the active CCPD transfer to finish to avoid resource and
  bandwidth contention.
- **authorized but not started:** CCPD2019, 13,164,924,944 bytes, isolated
  offline T3 only. The first curl attempt stopped after retry-resetting to a
  preserved 28.35-MiB partial; the corrected range-validated Python downloader
  is ready to resume it with long retry and visible progress.
- **diagnostic real-image progress:** five official CCPD repository images at a
  pinned verified commit are acquired with publisher blob hashes and local
  SHA-256 receipts. They support privacy-minimized compatibility/failure testing
  only because publisher ground truth is absent; they cannot close Boundary A.
- **FastPlate decision:** 5/5 real-pilot inputs decode to nonempty valid-charset
  results with no retained strings/crops, proving compatibility only. T2-V
  development exactness/CER is 0.7949/0.1566; validation and sealed holdout are
  0.7857/0.1683. Resource/calibration gates pass, but correctness/abstention do
  not; retain as efficient reference, `blocked_no_promotion`.
- **active blocker:** no detector or Latin/Urdu OCR passes unchanged gates; no
  accepted real T3 results or Pakistan T4 operational agreement.
- **prohibited:** OMZ 0123 conversion without new evidence, threshold reduction,
  holdout tuning, unmanifested transfer, R8.5 start or production assignment.

## 2026-08-13 R8 replacement-model evidence update

- **completed:** checksum/license acquisition for OMZ 0106/0123, PARSeq-tiny and
  EasyOCR `arabic_g1`; offline development/validation/sealed-holdout bundles.
- **measured:** OMZ 0106 has zero T2-V true positives; EasyOCR validation and
  holdout exactness is 0.0714; PARSeq is strongest at 0.7857 exactness / 0.2079
  CER. All remain rejected under unchanged thresholds.
- **active blocker:** an integrity/privacy-cleared real T3-D/T3-O artifact plus
  detector and Latin/Urdu OCR candidates that pass every gate.
- **prohibited:** threshold reduction, holdout tuning, production role
  assignment, R8.5 start or unmanifested multi-GB dataset transfer.

Last triaged: 2026-08-12  
Authority: `NEXUSAI_MASTER_DIRECTIVE.md` and the active R0-R17 roadmap  
Rule: one backlog, phase-owned work, no active-phase interruption except P0/P1

## NX-BL-006 — Multi-tier evidence readiness and operational agreement

- **type:** `data_governance_program`
- **severity:** P2 cross-cutting; phase exit may elevate its T4 dependency
- **affected phase/module:** R8-R17 evidence-family acceptance
- **discovered_in_phase:** R8
- **description:** Maintain T0 deterministic, T1 synthetic, T2-V controlled
  virtual, optional T2-P physical, T3 licensed-public and T4
  authorized-operational registry and build packs just
  in time for each evidence phase.
- **evidence:** R8 has reproducible T0/T1, preserved source-accepted T2-P
  tooling with evidence deferred because the environment is unavailable, and a
  separate 96-image T2-V pack with exact polygons/hashes plus development,
  validation and sealed-holdout partitions. T2-V validation passes; incumbent
  inference is complete and all candidates fail unchanged gates. CCPD is the primary real T3-D/T3-O authorization target;
  Artificial Mercosur is mixed-real supplemental, P-LPCD remains conflicted and
  SYNLIP remains synthetic-only. Candidate thresholds still fail Boundary A and
  T4 remains unavailable.
- **business impact:** Removes dependence on ad-hoc team samples for engineering
  and demos while preserving truthful production claims.
- **correctness impact:** Per-tier metrics and hash/version checks prevent easy
  synthetic results from masking distribution shift.
- **security impact:** Hostile fixtures never enter model lanes; T3/T4 remain
  privacy, license and authorization controlled.
- **data-integrity impact:** Manifests and expected labels are immutable inputs;
  demo evidence follows normal production logic.
- **UX impact:** Enables repeatable family workspace, Ask, citation and failure
  demonstrations as each phase matures.
- **dependency impact:** Does not reorder phases. T4 is non-blocking for safe
  source engineering unless the active exit gate explicitly requires it.
- **recommended phase:** just in time in R8-R17; immutable consolidation in R17
- **recommended priority:** P2 program, current R8 exit dependency
- **status:** R8_T0_T1_complete_T2P_tooling_preserved_environment_unavailable_T2V_generated_validated_sealed_not_inference_accepted_T3_real_not_approved_boundary_A_blocked_T4_pending_boundary_B_blocked

## Priority model

- **P0:** stop immediately for security, tenant/case isolation, evidence
  corruption, destructive execution, or silent forensic falsification.
- **P1:** bounded correction required because the active phase cannot safely or
  correctly proceed.
- **P2:** important and scheduled into the owning phase without interrupting the
  active roadmap.
- **P3:** useful enhancement retained for later evaluation.

## NX-BL-001 — Ask NexusAI intermittent blank-content or scroll failure

- **type:** `ux_defect`
- **severity:** P2
- **affected phase/module:** shared Ask NexusAI / Agent Chat UI
- **discovered_in_phase:** R6; re-triaged at R7 entry
- **description:** A previously reported interaction could leave Ask NexusAI
  content blank or apparently inaccessible after navigation or scrolling.
- **evidence:** On 2026-08-12 the accepted live runtime loaded the governed case,
  retained history, active conversation, large result tables, provenance,
  suggestions and analyst input. No blank primary surface was reproduced.
  Existing app-shell tests cover authorization loading instead of a blank
  screen. R8.2 adds stable chat viewport sizing, contained overscroll, a stable
  scrollbar gutter and an explicit keyboard-focusable Jump to latest recovery;
  the focused long-answer Chromium scenario passes.
- **business impact:** A reliable recurrence would damage confidence in the
  primary analyst workflow.
- **correctness impact:** None observed; presentation availability only.
- **security impact:** None observed.
- **data-integrity impact:** None observed.
- **UX impact:** Potentially high when reproduced, currently intermittent or
  unconfirmed.
- **dependency impact:** Does not block deterministic R7 backend/API work.
- **recommended phase:** bounded shared-UI maintenance when reproducible;
  otherwise R17 hardening
- **recommended priority:** P2; promote to P1 only with a repeatable primary-flow
  failure
- **status:** source_corrected_pending_live_acceptance

## NX-BL-002 — Retained-history progressive loading

- **type:** `performance_issue`
- **severity:** P2
- **affected phase/module:** Ask NexusAI retained case history
- **discovered_in_phase:** R6
- **description:** Replace the current eager retained-history list with bounded
  server pagination or incremental loading while preserving case scope.
- **evidence:** The live R7-entry inspection showed a long retained-history list,
  but it remained functional and did not block chat use.
- **business impact:** Improves large-case navigation and perceived performance.
- **correctness impact:** Pagination must preserve stable ordering and no
  cross-case leakage.
- **security impact:** Scope checks must remain server-authoritative.
- **data-integrity impact:** Read-only.
- **UX impact:** Medium for long-running cases.
- **dependency impact:** Not required for R7 telecom correctness.
- **recommended phase:** R16 enterprise usability
- **recommended priority:** P2
- **status:** deferred

## NX-BL-003 — Governed open-world analytical planning

- **type:** `architecture_improvement`
- **severity:** P3
- **affected phase/module:** query planner and cross-family orchestration
- **discovered_in_phase:** R6 governance reconciliation
- **description:** Progress from governed operation selection toward a constrained
  analytical DSL that composes authorized family operations. Never permit an LLM
  to issue arbitrary SQL.
- **evidence:** The accepted 65-operation catalog is reliable but intentionally
  bounded; broader questions will require composition as evidence families
  mature. NX-A1 now supplies the shared typed Investigation Context,
  capability/readiness snapshot, plan/step/budget validation, Tool Result,
  Observation Packet, Answer Envelope integration, direct fast path and bounded
  composition contract. Typed case-query input rejects unrestricted SQL and
  arbitrary execution fields. Broad family operations remain NX-B1 work.
- **business impact:** Expands natural analyst workflows without surrendering
  deterministic authority.
- **correctness impact:** High if implemented without typed plans and validators.
- **security impact:** Requires operation authorization, cost limits and query
  policy enforcement.
- **data-integrity impact:** Must remain read-only unless a separately authorized
  workflow explicitly permits mutation.
- **UX impact:** High future value.
- **dependency impact:** Depends on mature R7-R14 family contracts.
- **recommended phase:** NX-A1 foundation, NX-B1 operation breadth, later
  composition/cross-family and enterprise hardening phases
- **recommended priority:** P2 product architecture; foundation completed,
  breadth and deeper composition remain scheduled
- **status:** nxa1_shared_source_foundation_implemented_pending_final_regression_and_closure

## NX-BL-004 — Additional real-world natural query coverage

- **type:** `query_idea`
- **severity:** P2
- **affected phase/module:** family query corpora
- **discovered_in_phase:** R6.6
- **description:** Expand clean, short, messy, typo, Roman Urdu, Urdu, mixed-
  language, Pakistan-number and date-expression variants just in time per family.
- **evidence:** R6.6 accepts all 65 catalog examples plus 30 mixed natural
  variants. R7.5 adds exercised telecom variants for provider/sector history,
  overlap review, ICCID/service links, packet/USSD usage and SIM/device history,
  while preserving clarification and bounded no-result behavior. This remains a
  family-owned program rather than an open-world completion claim.
- **business impact:** Improves analyst success with realistic language.
- **correctness impact:** New variants must select the same governed operation
  and parameters as their goldens.
- **security impact:** No expansion of authorization or arbitrary SQL.
- **data-integrity impact:** None.
- **UX impact:** High within each evidence family.
- **dependency impact:** Family-owned and non-blocking outside that phase.
- **recommended phase:** R7-R14 with each owning family
- **recommended priority:** P2
- **status:** in_progress across later families; R7 telecom increment live accepted

## NX-BL-005 — Family-specific rich Ask NexusAI presentations

- **type:** `feature_idea`
- **severity:** P2
- **affected phase/module:** Case Workspace and Ask NexusAI presentation registry
- **discovered_in_phase:** R6
- **description:** Add maps/timelines for telecom, image/crop views for ANPR,
  synchronized transcript players for audio/video, document viewers and graph
  views as their evidence-family phases mature.
- **evidence:** R6 provides typed cards and bounded tables. R7.6 adds an
  accessible synchronized telecom table/timeline/coordinate plot and selected-
  observation evidence drawer. It carries citation IDs, source row/hash
  locators, coordinate datum/uncertainty and matched/ambiguous/unmatched join
  states; it explicitly suppresses route and RF-presence inference. Later
  families retain their phase-owned renderers.
- **business impact:** Converts family results into professional analyst
  workspaces.
- **correctness impact:** Visual elements must retain exact evidence locators and
  uncertainty.
- **security impact:** All source access remains case/tenant authorized.
- **data-integrity impact:** Derived visual state must not rewrite evidence.
- **UX impact:** High.
- **dependency impact:** Requires each family contract and result semantics.
- **recommended phase:** R7-R15, owned by the phase introducing the family
- **recommended priority:** P2
- **status:** in_progress across later families; R7 telecom increment live
  accepted, R8.2 image-admission presentation source accepted, and R8.3 adds a
  responsive read-only original-coordinate plate-candidate overlay. The first
  isolated detector/OCR comparison is measured with no production promotion;
  fixture coverage, authorized real-image agreement and OCR quality remain the
  R8-owned blockers.

## MMV2-P1-A — Positive result summarized as no-match

- **severity:** P1
- **status:** SOURCE_RESOLVED_LIVE_GATED (2026-08-25)
- **resolution:** authoritative operation rows/counts now drive positive result
  state through summary, public contract, presentation, citations and History.
- **remaining gate:** rebuild forensic API and LocalAI/UI, then live-certify
  English, Roman Urdu and Urdu retained ANPR without retained reprocessing.

## MMV2-P1-B — Completed-zero video collapsed to generic no-result

- **severity:** P1
- **status:** SOURCE_RESOLVED_LIVE_GATED (2026-08-25)
- **resolution:** public/UI contracts preserve completed-zero separately from
  filter miss, not processed, processing, failed, unavailable, unauthorized
  and invalid request.
- **remaining gate:** same two-service activation and live grouped-video
  certification. No worker rebuild, model change, migration or retained
  mutation is required.
