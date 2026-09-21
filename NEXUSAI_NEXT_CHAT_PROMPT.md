# NexusAI New-Chat Master Continuation Prompt

## 2026-08-30 NX-MMR ANPR/OCR vertical controlling override

- `NX-MMR-ANPR-OCR-VERTICAL-CLOSURE-V1` is source/local complete; no live
  activation, retained reprocessing, model installation, or promotion occurred.
- The failed fresh image reached the worker but ANPR/OCR did not run because
  both roles are disabled and the live media-model mount is empty. Source/local
  image positive, negative-zero, and missing-model states pass.
- Video V2 development: event recall 1.0, normalized exact F1 0.844445, group
  F1 0.909091, group precision 0.9375, negative-frame FPR 0.652778. Vehicle
  context is required; selected candidates must be actually observed.
- Exact V2 product-source freeze:
  `212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20`.
  The 8 reserved event components have not been scored.
- Printed Paddle OCR is fixture-measured for 5 English, 5 Urdu, and 5 mixed
  samples; real-world printed labels and UI RTL/citation acceptance remain.
- Exact next action: ask for/act only on explicit authorization for one private,
  local, non-retained reserved evaluation of the exact V2 freeze, with no
  tuning, downloads, source/config changes, image holdout access, live service
  mutation, or retained-state mutation. Stop after reserved metrics.
- Do not activate yet. Exact checkpoint license admission, Paddle NOTICE,
  `configuration/nxmmr_anpr_ocr_vertical_activation_v1.json`, and live/manual
  product gates remain separate later approvals. Do not start NX-B2.1, STT/TTS,
  image-semantic, or face work.
- Authority: `NEXUSAI_CONTINUATION.md` and
  `reports/nexusai-nxmmr-anpr-ocr-vertical-closure-v1-20260830.md`.

## 2026-08-30 NX-MMR parity-adapter controlling override

- `NX-MMR-REFERENCE-PARITY-ADAPTER-V1` source development is complete and
  frozen. The 11-development/8-reserved interval partition was created before
  inference without model output; reserved candidate metrics were not computed.
- Selected candidate: fixed 4 FPS, dedicated hash-pinned 640 plate detector,
  EasyOCR, plate-only, neutral majority, support >=2, one vote per actual crop.
  Freeze digest:
  `1dcfabf17a8b66165b6c1390e65a8784eb875d3650b1b7f27d328c9ad2f0bb13`.
- Development event recall is 0.818182 versus incumbent 0.090909, but exact
  TP/FP/FN is 2/12/22 and group TP/FP/FN 1/17/16. State is `PARTIAL`; do not
  promote or activate it. Vehicle context is optional. License admission and
  precision/group quality remain P1.
- Candidate evaluation has not started. Exact next action is explicit approval
  for one local, private, non-retained run of the exact freeze on the 8 reserved
  event components, using only the metrics and resource scope in the closure
  report. Do not tune, change the freeze, rescore image holdout, deploy, mutate
  retained/Activity/database/volumes, or start NX-B2.1.
- Authority: `NEXUSAI_CONTINUATION.md` and
  `reports/nexusai-nxmmr-reference-parity-adapter-v1-20260830.md`.

## 2026-08-30 NX-MMR reference-parity controlling override

- Read-only recovery inspection found both pre-existing personal reference
  pipelines and verified that both `sample.mp4` copies are byte-identical.
- The image reference uses the same FastALPR detector/OCR model bytes and
  relevant package versions as `CURRENT_NXMMR_BASELINE_V1`. Frozen development
  and one-time holdout predictions agree 10/10 and 22/22 respectively; keep the
  current image baseline. Remaining errors are OCR capability errors.
- The historical dense video reference scores 18/19 event detection and
  24/41 final exact plate matches, versus 3/19 and 3/41 for the sparse current
  policy. The parity gap is architectural: dense frames, a dedicated 640-input
  detector, vehicle ROI and repeated OCR versus one frame/second through the
  384-input image path. The strict UK normalizer, persistent SORT identity and
  interpolation are not admissible product semantics.
- Historical output was scored only against the locked human oracle and was
  never used as truth. A full exact rerun was stopped before model load because
  host memory was below the 4.5 GiB HEAVY floor.
- Exact next action: prepare and implement only the source-level, non-retained
  `NX-MMR-REFERENCE-PARITY-ADAPTER-V1` development slice with existing local,
  hash-pinned assets and bounded ephemeral adjacent-frame aggregation. Do not
  download, fine-tune, deploy, touch retained state, rescore the completed
  image holdout or start NX-B2.1.
- This exact-next-action line is complete and superseded by the parity-adapter
  controlling override above.
- No operation is promoted. The live API/Ask/Data/citation/Activity/UI/security/
  performance/manual product-acceptance gates remain mandatory.
- Authority: `NEXUSAI_CONTINUATION.md` and
  `reports/nexusai-nxmmr-reference-pipeline-parity-20260830.md`.

## 2026-08-30 NX-MMR human-gold empirical controlling override

- The independent human pack is locked with `GroundTruthValidation=PASS`; no
  model output was used as truth. Image development and the one-time sealed
  holdout are complete with identical incumbent configuration.
- Image development detected 10/10 and recognized 5/10 exactly. Holdout
  detected 20/22 and recognized 13/22 exactly (CER 0.201439). All 32 images are
  positives and have no boxes, so specificity and IoU remain unmeasured.
- The fixed one-second `sample.mp4` policy detected 3/19 human events and
  exactly recognized 3/41 plates. Sampled-frame TP/FP/FN/TN is 3/1/31/25;
  grouping is 4/29. Reject this video policy for promotion.
- All six human speech windows are negative with no transcript, so
  `faster-whisper-small-ur` was not called and ASR scoring remains inadmissible.
- No model/operation was promoted. Live install/build/deploy, retained
  evidence/Activity/database/volume mutation and NX-B2.1 remain forbidden;
  product certification still requires live API/Ask/Data/citation/Activity/UI/
  manual acceptance.
- This exact-next-action line is superseded by the reference-parity controlling
  override above. Its evidence gaps remain open but are not the immediate
  recovery action.
- Authority: `NEXUSAI_CONTINUATION.md` and
  `reports/nexusai-nxmmr-human-gold-anpr-asr-closure-20260830.md`.

## 2026-08-18 APF-3.7 deployment controlling override

- Guarded deployment passed: API 19.3 seconds and LocalAI/UI 114 seconds;
  worker/models/profiles unchanged, volumes and rollback images preserved.
- LocalAI readiness, forensic health and Analyst HTML are HTTP 200. LocalAI,
  worker, NATS and PostgreSQL are healthy; the records API is running.
- `APF-3.7RuntimeStatus=DEPLOYED_PENDING_BROWSER_ACCEPTANCE`,
  `APF-3RuntimeStatus=DEPLOYED_PENDING_BROWSER_ACCEPTANCE`, and
  `DeploymentNeeded=NO`. Do not rebuild.
- Await the required live acceptance-deck outcomes. Only if they pass with no
  P0/P1 may the runtime statuses become accepted and APF-3 become closed.
- Do not remove orphans, mutate retained state, or start APF-4/R13.

## 2026-08-18 APF-3 final source-closure controlling override

- APF-3.7 and overall APF-3 are source accepted; the running demo remains the
  healthy APF-3.4–3.6 runtime. `DeploymentNeeded=YES` and APF-3.7 runtime
  acceptance is pending the guarded activation and manual deck.
- APF-3.7 adds governed two/three-step read-only composition with typed
  dependencies/bindings, strict scope/security/citation/resource validation,
  partial-failure handling and candidate-correlation lineage. It preserves the
  single-operation fast path.
- The risk-tier ledger is 65 total: Tier A 4/4 certified/queryable, Tier B 50
  bounded/limited, Tier C 11 bounded/engineering-only. No known correctness
  defects, P0 or P1 remain; 155/155 query variants pass.
- Do not deploy implicitly, change models/profiles/worker/volumes, or mutate
  retained evidence. Use the exact guarded handoff in the final closure report
  when the operator authorizes activation.
- Authority: `reports/nexusai-apf3-final-source-closure-20260818.md`. Do not
  start APF-4/R13 without a new directive.

## 2026-08-18 APF-3.4–3.6 live-closure controlling override

- APF-3.4, APF-3.5 and APF-3.6 are live accepted. Preserve the final guarded
  marker: API 19.5 seconds, LocalAI/UI 49.5 seconds, worker/models/profiles
  unchanged, volumes and rollback images preserved.
- Deterministic queries are sub-second; the exact three-turn follow-up chain
  passes. Actual schema-bound assistance passes in 43.7 seconds with no invented
  direction/scope. English, Roman Urdu and Urdu named-month equivalents return
  identical target, UTC bounds and four temporal rows.
- KB and both hybrid operations pass; the hybrids return `records_sql + kb_rag`,
  17 deterministic rows and three KB results. The temporal UI renders all 12
  columns and aggregate contribution lineage is live.
- Operation certification is 2 certified and 63 blocked missing independent
  fixtures. Strict result remains `APF-3.7Ready=NO`; do not start APF-3.7 until
  the remaining operation-truth debt is independently certified.
- `DeploymentNeeded=NO`. Do not rebuild, change models/profiles/worker/volumes,
  or mutate retained evidence merely to continue the certification ledger.

## 2026-08-18 APF-3 final-certification controlling override

- APF-3.4 exact live follow-up acceptance passes: frequent contacts for
  `923001110001`, `Only outgoing.`, then `For 923001234567 instead.` retains
  operation and direction, replaces the target and executes without
  clarification. The temporal oracle returns 4 matched/nocturnal events, 2
  non-zero durations and 10/45/27.50 second min/max/average.
- Live KB retrieval passes. Both hybrid templates fail in the deployed executor.
  Deployed deterministic records still incur roughly 40 seconds of unnecessary
  model synthesis. The installed assisted-language model times out or returns
  invalid operation semantics; it is not role-accepted.
- Source corrections skip synthesis for deterministic records, explicitly
  dispatch governed hybrid operations, add capped aggregate contribution
  lineage and provenance, and render the full temporal calculation table.
  These corrections are tested but not deployed.
- The generated variant ledger is 155 accepted occurrences, 143 unique and 12
  duplicates; 155/155 pass routing, parameter and semantic expectations. The
  operation ledger remains 65 rows: 2 calculation/presentation passes but 0
  live citation passes, plus 63 missing independent goldens.
- Strict gate result: `APF-3.7Ready=NO`. Do not start APF-3.7 or claim complete
  analytical certification. Next run the guarded API/UI activation only with
  explicit operator approval, then replay the bounded live deck and reassess
  the assisted-language role. Do not rebuild the worker, change/download
  models, change profiles, mutate retained evidence or touch named volumes.
- Authority/report:
  `reports/nexusai-apf3-query-truth-certification-20260818.md`.

## 2026-08-17 APF-3.1-3.3 source-acceptance controlling override

- APF-3.1, APF-3.2 and APF-3.3 are source accepted. Typed understanding,
  resolved workspace-aware capability projection, and a governed one-step
  execution plan now wrap the existing deterministic/KB/hybrid executors.
- All 65 executable query operations have stable descriptor and reverse-lookup
  parity. The full platform registry has 80 descriptors because 50 historical
  platform operations are retained and 30 missing executable query descriptors
  are derived; do not collapse these complementary surfaces.
- Security and availability guards reject unknown IDs, unauthorized or widened
  scope, missing/processing data, absent runtime/model, insufficient maturity,
  invalid required parameters, arbitrary SQL/URL/tool IDs, multiple steps and
  citation disabling before execution.
- Verification is 26 APF Go events passed/0 failed/0 skipped, full forensic
  package pass, and Ginkgo 258 passed/0 failed/2 skipped. Static analysis passes;
  1,000 deterministic understanding passes took 971.0366 ms. Race testing was
  unavailable because CGO is disabled on this Windows Go environment.
- This is source acceptance only. Backend deployment and live browser acceptance
  are pending and require the existing guarded R8 UI/API gate; do not claim the
  running port 8080 contains APF-3 until the operator runs that gate.
- APF-3.4 is next but not started. Do not begin APF-3.5+, resume R8/CCPD, upload
  evidence, mutate retained data, or deploy without explicit authorization.

## 2026-08-17 APF-3 architecture controlling override

- APF-3 architecture/dependency reconciliation is complete; implementation is
  not started. The authority is
  `docs/design/nexusai-apf3-unified-query-intelligence-architecture.md`.
- NexusAI's forensic service owns authorization, workspace/data-aware capability
  resolution, planning, execution policy, validation and citations. LocalAI
  supplies bounded language/tool/retrieval/model runtime capability only.
- Verified truth: 65 executable templates, 19 family capability definitions,
  50 older platform descriptors, 16 specialist manifests and 10 model roles.
  Reconcile the 65/50 parity gap; do not add a duplicate router or registry.
- Exact next source slice is APF-3.1: typed query-understanding and execution-
  plan contracts plus a legacy deterministic-router adapter. Preserve the
  existing Ask/SSE/history APIs, SQL operations, portal UI, models and data.
- This reconciliation was documentation-only. No deployment, retained mutation,
  model download or R8/CCPD action occurred. R8 remains paused.

## 2026-08-17 APF-2 completion controlling override

- APF-2 is live/runtime accepted. Preserve evidence
  `4320772b-febe-4af2-b9e3-92cea114da0b`, its 4 canonical rows from 5 input
  rows, 1 duplicate, 0 rejected, valid custody chain and existing job/version.
- Source accounting resolves by evidence identity to the authoritative catalog;
  unknown is never rendered as zero. Unique catalog filename resolution now
  makes record citations open the exact evidence in Ask and History.
- The one authorized acceptance query is completed analysis
  `c540c875-0cd7-410d-bd6b-3f2be87b0336`: deterministic
  `cdr.temporal_activity`, 4 exact results, citations to fixture rows 5 and 2.
  History is 51 entries; Continue in Ask was navigation-only.
- Focused Playwright 10/10, scoped ESLint zero errors, 677-module build and real
  responsive dark/light preview pass. No Docker rebuild or deployment occurred.
- Exact next item is APF-3 — Unified Query Intelligence / Capability Routing.
  It is reconciled but not started. Do not resume R8 or mutate retained data.

## 2026-08-17 APF-2 runtime-halt controlling override

- The one approved upload of `pakistan_cdr_messy_synthetic.csv` is complete;
  do not upload it again. Preserve evidence
  `4320772b-febe-4af2-b9e3-92cea114da0b`, version
  `ad4b97c9-54eb-4657-afa7-5f06f52ba695` and completed job/run
  `605b53d8-195f-4dde-aae6-081fbef6b500`.
- Backend truth: CDR, 4 canonical rows from 5 input rows, 1 duplicate, 26
  indexed entities, KB asset, verified write-once storage and valid 3-event
  custody chain. Workspace is 11 sources, 10 Ready, 0 Processing, 0 Needs
  attention, 1 Failed and 9,278 accepted records.
- Runtime acceptance halted because modal → Open source displayed
  `0 verified rows`. Detail omits `item.accepted_rows`; the modal callback opens
  with only evidence ID and filename; drawer fallback therefore renders zero.
- No Ask query or analysis-history entry was created. Do not invent one, retry,
  reprocess, repair retained data, clean up or upload again.
- Active work remains APF-2: implement and test the bounded source-detail
  accounting correction, then resume the accepted runtime path from the
  preserved evidence through Ask/citation/History. APF-3 has not started.
- No Docker rebuild is required. R8 remains `valid_but_paused_non_blocking`.

## 2026-08-17 APF-2 controlling override

This block supersedes older APF active-work statements without reopening
accepted R0-R7 or changing preserved R8 state.

- APF-1, APF-UX-1 and APF-2 are source accepted. APF-2 remains active only for
  an explicitly approved retained-runtime upload.
- The ordinary-analyst Data journey reuses the existing workspace upload,
  evidence retention/classification/duplicate, queue, case catalog pagination,
  capability and read-only reprocess-plan contracts. No parallel backend exists.
- Add Data supports page drop and a multi-file modal, actual progress, two
  concurrent registrations, per-file partial outcomes and **Already added**.
  Central product states are Ready, Processing, Needs attention and Failed.
- Only Ready sources expose registry-derived productive Ask actions. Failed
  sources show preservation and approval requirements; do not add Retry or call
  the reprocess execution endpoint from APF-2.
- Acceptance: scoped lint zero errors; Vite production build 677 modules;
  focused Analyst Portal Playwright 9/9; real Vite/backend dark/light browser
  inspection shows 10 sources, 9 Ready and 1 Failed.
- No retained upload, Docker rebuild/deploy, schema/backend/model/worker/profile
  change, reprocess, stage, commit, push or publication occurred.
- Exact next action: ask for explicit approval before uploading
  `ingestion/forensic_records/tests/fixtures/pakistan_cdr_messy_synthetic.csv`.
  State the retained evidence/job/record/custody consequences. The existing
  deployed backend needs no rebuild for this runtime proof.
- Frontend production activation is a separate guarded deployment decision.
- R8 remains `valid_but_paused_non_blocking`; do not resume CCPD transfer,
  evaluation or promotion during APF.

## 2026-08-17 APF-UX-1 controlling override

This block supersedes the older APF active-work statements below it without
reopening accepted R0-R7 or changing preserved R8 state.

- APF-1 and APF-UX-1 are source accepted. `/analyst` is the simple ordinary-
  analyst product with Home, Data, Ask NexusAI and History.
- Home is action-oriented; Data is searchable/filterable with friendly source
  detail; Ask suggestions are backend-capability-derived; History is full-width,
  filterable, cursor-paginated, reopenable and continuable.
- Scoped lint and production build pass. The focused portal/navigation/login
  Playwright set passes 21/21. Real-backend visual QA passes at 390/820/1024/1440
  in light/dark with no overflow or console errors.
- This is source-preview acceptance. No Docker rebuild/deploy, migration,
  evidence mutation, case/collection mutation, model/profile/worker change,
  upload, reprocessing, staging, commit, push or publication occurred.
- The verified guarded deployment gate was not invoked and is not required for
  Vite review. Do not rebuild merely to inspect this UX slice.
- One work item is active: APF-2, using existing evidence admission and job
  contracts for multi-file upload, evidence-based/ambiguous classification,
  progress, failure and retry. Do not introduce a new media model.
- R8 remains `valid_but_paused_non_blocking`; preserve the stopped CCPD partial,
  T2-V, packages, receipts and rejection evidence without transfer, evaluation,
  deletion or promotion.
- Controlling design:
  `docs/design/nexusai-analyst-portal-product-architecture.md`.

## 2026-08-17 analyst-portal product rebase controlling override

This block supersedes every older active-work statement below it without
erasing accepted R0-R7 or preserved R8 evidence.

- Active Program Phase: APF — Analyst Portal Foundation, a bounded cross-cutting
  product correction; it does not renumber R0-R17.
- APF-1 is source accepted: `/analyst` has a separate responsive shell with
  Home, read-only Data, governed Ask NexusAI, and History, all using the
  authoritative workspace registry and real existing APIs.
- Scoped lint, production build, four focused Playwright flows, desktop/mobile
  inspection and a real deterministic CDR query pass. The deployed UI is not
  changed and Docker identity was not newly verified from the sandbox.
- The real query produced the expected governed history row
  `f7e12210-a7f2-42ed-a4ad-8144388ac72b`. No evidence, schema, case/collection,
  container, model, deployment, or external evaluation artifact changed.
- Current Case Workspace remains the advanced surface. R0-R7 remain accepted.
- R8 is `valid_but_paused_non_blocking`. Preserve its T2-V, model packages,
  receipts, rejection evidence and stopped CCPD partial; do not resume, delete,
  evaluate, promote, or start R8.5 during APF.
- One work item is active: APF-2, a simple upload/classification/progress/failure/
  retry experience over existing evidence and job contracts. No new media model.
- Controlling design and acceptance flow:
  `docs/design/nexusai-analyst-portal-product-architecture.md`.
- Any build/redeploy remains a separate explicit approval boundary.

## 2026-08-13 R8 feasibility-corrected controlling override

This block supersedes older R8 active-work statements below it.

- Physical T2-P tooling is preserved/source accepted; evidence is
  `deferred_environment_unavailable`, not failed. Do not ask the operator to
  resume manual capture and do not treat the empty workspace as evidence.
- Boundary A is explicitly amended without lowering thresholds: require T0,
  T1, accepted T2-V, suitable licensed real-image T3-D/T3-O, license/privacy,
  hostile-input, accuracy, calibration, abstention, resource and reproducibility
  passes. T2-P is optional/deferred only under that stronger replacement.
- T2-V 1.0.0 is generated at `C:\NexusAI-Evaluation\R8\T2V\1.0.0`: 96 images,
  56 development, 20 validation, 20 sealed holdout; manifest SHA-256
  `180889ad2dc78736cba6617ab49b3b910ddfd29e8dfb77a143fc9ce26f50e510`.
  Pack validation passes; incumbent inference is complete and all candidates
  fail unchanged development, validation and sealed-holdout gates.
- CCPD is the primary real T3-D/T3-O authorization target pending exact artifact
  integrity and privacy review. Artificial Mercosur is supplemental mixed-real,
  not sufficient alone for real OCR. No dataset/model download is approved.
- OMZ 0123/0106 and PARSeq/EasyOCR remain bounded model research candidates;
  Paddle remains the measured control/baseline. Thresholds are unchanged.
- Boundary A and B remain blocked; R8.5 is not started; production is unchanged.
- Next action is exact T3/model artifact authorization followed by explicit
  download approval and isolated same-partition T1/T2-V/T3 evaluation—not manual
  photography.

## 2026-08-13 R8 Boundary-A no-stall controlling override

This block supersedes older active-work statements below it.

- R8.4-A through R8.4-F are parallel internal work items; do not create a new
  top-level phase and do not start R8.5.
- T2 engineering is complete: run the 32-image controlled one-session plan,
  preserve 28 development/four sealed holdout images, then pass the strict
  validator. Physical assets and independent review are pending.
- T3 research is bounded. Preferred detector authorization candidate is
  Artificial Mercosur v1 after privacy/source-rights approval; CCPD remains
  privacy/integrity gated. SYNLIP is supplemental only. P-LPCD remains blocked;
  UFPR/RodoSol remain non-commercial; INDO-ALPR is deferred.
- Detector shortlist: OMZ 0123 primary, OMZ 0106 secondary, Paddle baseline.
  OCR shortlist: Paddle English/Arabic controls, PARSeq Latin challenger,
  EasyOCR `arabic_g1` Urdu challenger. None is downloaded or promoted.
- Evaluator provenance/partition contracts are ready. Thresholds are unchanged.
- Boundary A and Boundary B remain blocked. Production is unchanged.
- Immediate action: user performs the T2 capture session while engineering
  seeks explicit privacy/legal approval for Artificial Mercosur metadata/sample
  inspection. Do not wait for P-LPCD and do not download anything without the
  exact approved operator gate.

## 2026-08-12 R8 two-boundary controlling override

This block supersedes every older active-work statement below it.

- R6 and R7 are complete. R8 is active; R8.5 has not started.
- Boundary A is the engineering/pipeline gate: accepted T0-T3 evidence,
  license/privacy and hostile-input controls, unchanged model/calibration/
  resource thresholds and reproducible artifacts. Only a Boundary A pass may
  authorize non-production R8.5 source integration.
- Boundary B is the production gate: Boundary A plus authorized T4 evidence,
  live-case/overlay/citation acceptance, operational resource acceptance and
  explicit human promotion approval.
- Both boundaries are blocked. Current detector/OCR candidates remain below
  the unchanged gate. Exactly two OCR preprocessing variants and 640/1280
  detector inputs were measured offline; no acceptance threshold was changed.
- T2 manifest/validation tooling is ready. Next obtain owned/authorized physical
  images, independent ground truth, privacy review and immutable hashes.
- P-LPCD must not be downloaded until the CC BY 4.0 Zenodo versus CC BY-NC 4.0
  author-repository conflict and the 36/37-class schema discrepancy are resolved
  in writing. Other T3 candidates remain license/privacy scoped.
- No alternative model, public dataset or production role was downloaded or
  assigned. Production containers, worker, model profiles and retained data
  remain unchanged.
- Immediate action: materialize and validate the T2 controlled demo pack and
  seek publisher clarification for a bounded T3 source. Re-evaluate the
  specialized OpenVINO detector shortlist only after approval; do not start
  R8.5 before Boundary A.

Copy everything below the divider into a new Codex task opened for this same
repository.

---

## 2026-08-12 R8 independent evidence and expanded no-promotion controlling override

This block supersedes every older active-work statement below it.

- R6/R6.6 and R7 are complete at their governed boundaries. Do not reopen them
  for P2/P3 polish.
- Active Program Phase: R8 - ANPR Image and OCR Intelligence.
- Active Work Item: R8.3/R8.4 acceptance closure - T2/T3 agreement and passing specialized candidates after complete T0/T1 failure evidence.
- R8.1 is complete: the capability/reuse matrix and hostile-image/privacy
  threat model are accepted.
- R8.2 is source accepted: `forensics.image-intake/v1` records immutable image
  identity, bounded byte/signature/header/dimension/pixel/orientation decisions,
  explicit manual-review state and analyst-readable metadata without claiming
  OCR or detection.
- The shared Ask UI now provides a tested Jump to latest recovery for long
  answers; live acceptance remains part of the guarded deployment slice.
- The explicitly supplied team-lead CDR passed an ephemeral, read-only R7.8
  audit: 3,931/3,931 rows normalized, zero rejected, 297 exact duplicates kept
  visible, and complete source locators, party roles and timezone provenance.
- The protected CDR was not copied, uploaded or retained; no database/evidence
  write or model call occurred. A retained ingest is neither required nor
  authorized by this acceptance.
- R7.9 hardening and R7.10 guarded runtime/manual acceptance are live accepted:
  all 65 deterministic operations pass, six specialist paths are covered, and
  the synchronized map/timeline/evidence UI passed desktop and 390 px checks.
- Execute R8 only through the bounded slices and gates in
  `docs/design/nexusai-r8-anpr-image-ocr-intelligence-contract.md`.
- R8.3 platform source is accepted: typed original-pixel candidates, bounded
  deterministic NMS, exact lossless crop/hash lineage, one-to-one localization
  metrics and the read-only responsive overlay are implemented.
- The guarded R8 UI/API activation is live accepted: exact container identity,
  rollback images, health/ready/UI checks and desktop/390px Evidence checks pass.
  The worker, models, profiles and named volumes were unchanged. The active case
  has no image evidence, so the deployed candidate overlay is not yet accepted
  against a live image.
- One authoritative T0-T4 registry and just-in-time evidence program now exist.
  The reproducible 29-fixture T0/T1 pack covers 14/14 classes and safely excludes
  hostile admission files from model code. Expanded offline evaluation rejects
  the text-detector adapter (precision 0.6071) and all OCR candidates; Paddle
  Arabic is strongest at 0.875 exact/0.06863 CER but still fails. T2 capture
  protocol and T3 research are ready; no public dataset was downloaded and T4
  remains pending. No production model role was assigned.
- Immediate action: obtain explicit approval for a bounded T3 benchmark download
  and, if desired, the documented gate refinement that permits R8.5 source-only
  after T0-T3 acceptance while preserving T4 for production/runtime/closure.
  Otherwise continue candidate tuning/research without starting R8.5.
- Preserve exact evidence/derived-evidence lineage, human-review separation,
  tenant/case controls and deterministic authority. Image fixtures, protected
  media, model/backend downloads, deployments, migrations, retained-data
  changes, staging, commit, push and publication remain approval-gated.

## 2026-08-12 R7 activation controlling override

This block supersedes every older active-work statement below it.

- R6/R6.6 is complete and live accepted. Do not reopen it for P2/P3 polish.
- Active Program Phase: R7 — Pakistan CDR and Tower Intelligence Deepening.
- Active Work Item: R7.6 — uncertainty-aware telecom map and timeline presentation.
- R7.1 is source accepted: `tower_cdr_join` now exposes exact candidate counts,
  timestamp-eligible counts and explicit matched, overlapping-ambiguous,
  no-reference and outside-validity outcomes; focused forensic Go tests pass.
- Use `docs/roadmap/nexusai-product-engineering-backlog.md` as the only product
  engineering backlog. P0/P1 may interrupt; P2/P3 must not derail R7.
- The live blank-content/scroll defect did not reproduce and is P2 monitoring.
- Reuse existing CDR/subscriber/tower adapters, forensic API, specialists,
  deterministic authority, provenance and approved Qwen explanation role.
- Protected real CDR remains read-only/un-ingested. Deployment, migration,
  retained-data changes, model/backend downloads, staging, commit, push and
  publication require explicit approval.
- R7.2 is source accepted: CDR adapter 1.2 adds non-breaking raw/canonical
  Pakistan party roles, timezone provenance, service/sentinel classification
  and synthetic goldens while preserving legacy CDR hashes/output.
- R7.3 is source accepted: subscriber adapter 1.2 separates subscriber,
  SIM/ICCID/IMSI, device/IMEI and service/provider observations with explicit
  validity and no ownership inference; full ingestion and forensic Go suites
  pass.
- R7.4 is source accepted: tower adapter 1.2 preserves provider/site/sector
  history, validity basis, coordinate/datum/uncertainty provenance and explicit
  overlap review without RF-presence inference.
- R7.5 is source accepted: realistic tower-history, ICCID/service and CDR
  service/device language routes deterministically; missing inputs clarify and
  no-result responses stay bounded negatives; full ingestion and Go suites pass.
- Immediate action: implement synchronized telecom tables, timeline and an
  uncertainty-aware map with visible ambiguous/unmatched state and evidence
  drawer using the accepted NexusAI presentation architecture.

## 2026-08-11 R6.6-B2 predeployment controlling override

This block supersedes every older active-work statement below it.

- Active Program Phase: R6 corrective quality extension.
- Active Work Item: R6.6-B2-LIVE — guarded rebuild and rebuilt-runtime acceptance.
- R6.6-A is source accepted: targetless frequent contacts clarify before SQL;
  service labels/self are excluded; inventory/readiness answers are direct and
  scoped; primary presentation is analyst-facing; technical diagnostics are
  collapsed; retained old payloads are normalized; Ask NexusAI owns the surface;
  and tables use progressive disclosure.
- Evidence: forensic API PASS, focused presentation PASS, targeted ESLint zero
  errors, Vite build PASS, Agent Chat Playwright 17/17, interactive retained-
  inventory visual smoke PASS.
- R6.6-B1 is source accepted: bounded same-case filter context supports explicit
  follow-ups such as “Only outgoing”; fresh targetless questions still clarify.
  Routing tests cover all 65 catalog examples plus 30 realistic variants.
- The Docker runtime has not been rebuilt. Keep source-preview evidence and
  deployed-runtime claims separate.
- Do not begin R7. Do not deploy, migrate, ingest/reprocess evidence, change
  models, stage, commit, push or publish without separate approval.
- R6.6-B2 safe predeployment acceptance is complete: 65/65 operations selected
  and completed correctly, six/six specialist paths passed, the 669-module
  build and Agent Chat 17/17 passed, and the full responsive light/dark browser
  matrix has zero overflow and console errors.
- Immediate objective: obtain explicit approval for the parameterized guarded
  rebuild, then run the rebuilt-runtime/model/manual acceptance pack. Preserve
  all 65 deterministic operations and exact-fact authority. Do not begin R7.

## 2026-08-10 R5 closure and R6 activation override

This block supersedes every older active-work statement below it. R5 is complete
and live-runtime accepted: the corrected guarded refresh passed, all five
services are healthy, migration 011 remains verified with zero retained rows
rewritten, and the retained three-event same-second custody chain reports zero
broken links and `chain_valid: true`. Evidence Operations passes responsive live
acceptance with a clean console.

- Active Program Phase: R6 — Ask NexusAI and Specialist Orchestration Experience.
- R6.1 is source accepted: handoff is enabled only for operational agents and
  specialist continuation returns to Ask NexusAI with the governed case and
  analyst question preserved. The focused production-preview matrix passes 25/25.
- R6.2-A is audit accepted. Distributed status/stream events lack request IDs;
  cancellation exists internally but has no chat route; timeout and reconnect
  behavior are inconsistent; retry is not idempotent.
- R6.2-B is source accepted: local/distributed lifecycle events carry request
  identity and the browser rejects mismatched request/case live state. Focused
  Go checks, the 669-module build and browser matrix 26/26 pass.
- R6.2-C is source accepted: authorized request/case-bound cancellation and the
  Stop action pass the 27/27 production-preview matrix.
- R6.2-D is source accepted: all execution modes share the governed deadline;
  correlated `timed_out` remains distinct from `cancelled`; no automatic retry
  occurs; and the combined production-preview matrix passes 28/28.
- R6.2-E is source accepted: bounded scoped status lookup reconciles the current
  request after SSE reconnect without replay or retry; deterministic combined
  production-preview acceptance passes 29/29.
- R6.2-F is source accepted: analyst-initiated retry has `retry_of` lineage,
  terminal-state eligibility and atomic duplicate-submit protection, with no
  automatic retry or new retained schema.
- R6.3-A is source accepted: additive scoped retention, correlated local and
  distributed lifecycle persistence, five authorized APIs, server-authoritative
  saved history, explicit reversible browser import, generated Swagger/docs and
  responsive 16/16 browser acceptance are complete. Hidden reasoning is not
  retained. No deployment, schema activation or retained import occurred.
- Active Work Item: R6.3-A-LIVE — run the guarded activation script, verify the
  GET-only `live_accepted` marker with zero mutating gate requests, then begin
  R6.3-B typed streaming and execution-authority presentation.
- Preserve the existing 65 deterministic templates, six specialist agents,
  approved models, RAG/tool paths, case isolation and professional presentation.
- Do not deploy, migrate, ingest/reprocess retained evidence, change models,
  stage, commit, push or publish without separate approval.


## 2026-08-07 R4-ARCH-04 acceptance override

This is the controlling continuation block and supersedes every older active-
work statement later in this file.

R4-ARCH-04 is source accepted. Forensic Agent Chat consumes provider-validated
case/collection identity, sends both IDs explicitly, isolates conversation and
pending-event state by case and rejects late prior-case SSE/HTTP activity.
Generic Agent Chat remains compatible. Focused lint has zero errors, the Vite
production build transforms 664 modules and the protected browser matrix passes
53/53. No runtime, data, model, deployment, staging or publication state changed.

- Active Program Phase: R4 — Case-Centered Workspace Architecture.
- Active Work Item: R4-ARCH-05 — Administrative transitions and reports.
- Immediate action: distinguish collection administration from explicit
  authorized case transitions and close the case-scoped report adapter/test
  boundary. Do not generate retained reports, deploy or mutate data/runtime.

## 2026-08-07 R4-ARCH-03 acceptance override

This is the controlling continuation block and supersedes every older active-
work statement later in this file.

R4-ARCH-03 is source accepted. Embedded Records Intelligence is locked to the
shared provider case for query, upload, list, evidence, delete and export
behavior; alternate collection authority is removed in embedded mode; stale
prior-case results are discarded. Focused lint has zero errors, the Vite
production build transforms 664 modules, and the protected browser matrix
passes 46/46. No runtime, data, model, deployment, staging or publication state
changed.

- Active Program Phase: R4 — Case-Centered Workspace Architecture.
- Active Work Item: R4-ARCH-04 — Bind Agent Chat to the active case.
- Immediate action: inspect and implement only the bounded Agent Chat case-
  identity/request-state slice. Do not expand into the future R6 streaming
  lifecycle and do not deploy or mutate API/backend/data/model/runtime state.

## 2026-08-06 R3.1 acceptance and R4 reactivation override

Deployment update: the user subsequently ran the existing guarded combined
gate and it passed. Live containers/readiness, the accepted R3.1 asset hashes,
the governed case, desktop/mobile layouts, Settings branding and the browser
console are accepted. Do not rerun the deployment. The R4-ARCH-01 case-context
and collection-coupling inventory is accepted. R4-ARCH-02 is source accepted
with a shared active-case provider, 664-module build and 35/35 protected browser
matrix. Proceed with R4-ARCH-03's bounded embedded Records Intelligence binding.

This block supersedes older active-phase statements below. R3.1 is complete at
the source/local-production-browser boundary: targeted lint passed, the Vite
production build transformed 663 modules, 34/34 protected plus R3.1 browser
contracts passed, all required widths/themes/keyboard/branding/hashed-asset
checks passed and the browser console was clean. Deployment was not executed.

- Active Program Phase: R4 — Case-Centered Workspace Architecture.
- Active Work Item: R4-ARCH-03 — Bind embedded Records Intelligence to the
  active case.
- Immediate action: implement the bounded R4-ARCH-03 source slice with focused
  no-cross-case tests; do not rerun deployment or mutate API/backend/data/model/runtime state.

## 2026-08-06 R2 closure and R3 activation override

This is the controlling continuation block. The older overrides and objectives
later in this file are historical context and must not be treated as current.

Read in precedence order: latest user instruction,
`NEXUSAI_MASTER_DIRECTIVE.md`, `AGENTS.md`/`.agents/`,
`docs/roadmap/nexusai-current-roadmap.md`, `NEXUSAI_CONTINUATION.md`, then this
file. Use `docs/roadmap/nexusai-phase-ledger.md` for status and
`docs/roadmap/nexusai-phase-mapping.md` for legacy references.

- Active Program Phase: R4 — Case-Centered Workspace Architecture.
- Active Work Item: R4-ARCH-01 — Reconcile and establish the single active-case
  contract.
- R0 is complete at its bounded reconciliation criteria.
- R1 is source accepted with exact 371-route plus 10 app registrations and
  64-backend inventories. R2 is source accepted.
- Historical Phase 7.3 professional answer presentation remains accepted
  history; it is not current and does not complete R6.
- `R4-PRE-01 — Truthful Case Workspace initialization` is source and
  live-runtime accepted through the guarded combined deployment.
- Agent Chat lifecycle reliability belongs to future R6, tentatively R6.2. Do
  not implement it now.

R2 decisions are closed: use the code-native NexusAI identity with the existing
branding API override, exact established tagline/context placement,
Analyst/System Administrator labels at the currently enforceable role boundary,
analyst/intelligence/system-administration navigation, the shared report asset
fallback, and replace/contextualize/retain/hide LocalAI rules.

R3-UI-01 identity/loading/navigation and R3-UI-02 unified shell context are
source implemented. The desktop/tablet shell now exposes route, enforced role
and URL-bound case identity; mobile has compact route/case context and keyboard
users have a skip target. The shared R3-UI-03 state contract is applied to 404
and governed-case redirect states.

Immediate objective: continue R3-UI-03 by migrating remaining primary analyst
route loading/empty/error/forbidden/unavailable states and completing visible
white-label dispositions. Prepare but do not execute a current-source build or
deployment without approval.

No Docker rebuild/deploy, database migration/backfill, evidence
upload/reprocess, collection cleanup, model/backend download, retained
configuration change, staging, commit, push or publication is authorized.

## 2026-08-06 authoritative override

The older Phase 7.1 objective retained later in this document is historical and
must not be executed again. The latest checkpoint and live reconciliation are
authoritative:

- Phase 6 CDR/IPDR/ANPR, Phase 7.1 populated subscriber, Phase 7.2 populated
  tower/site, and Phase 7.3 professional answer presentation are runtime
  accepted.
- Live images are LocalAI/UI `0aff21e8f09b`, forensic API `4c415a95e18d`, and
  worker `7a62791e40cf`; all five services are running.
- Exactly two approved models remain installed. Six agents are bound to the one
  governed case `nexusai-forensic-demo`.
- The case manifest has 9,272 canonical records across seven populated families,
  eight evidence items, eight KB assets, 11 KB entries, and zero deletions.
- Current source is ahead of the image: the governed selector hides four cleanup
  candidates and the Overview uses a truthful verification state while data
  loads. These changes are source-only until an approved guarded refresh.

Immediate objective: complete Phase 7.3 Agent Chat lifecycle reliability with
typed start/status/tool/complete/error/cancel/timeout events, cancel/retry,
elapsed time, deterministic-versus-model-assist state, persistence, and
reconnect/concurrency tests. Exit only when no refresh, cancel, timeout, retry,
duplicate submit, conversation switch, or concurrent request can leave an
orphaned `Working...` state. Then continue Phase 7.4 Pakistan CDR/tower
deepening. Do not rebuild, upload, backfill, clean collections, or repeat
accepted matrices without the relevant approval or source change.

You are Codex continuing NexusAI / LocalAI forensic-intelligence development in:

`C:\Users\sheik\Workspace\Office\Projects\NexusAI`

Date/timezone context: 4 August 2026, Asia/Karachi.

## Mission

Build NexusAI as an enterprise, Pakistan-ready Case Knowledge Fabric for lawful
forensic analysis. The product must combine independently versioned evidence
adapters, a specialist agent per evidence family, one coordinating analyst,
deterministic exact analytics, cited Knowledge Base retrieval, optional bounded
model explanation, chain of custody, professional case-centered UI/UX, and
reusable versioned APIs.

Maximum productivity never overrides evidential correctness. Exact facts,
counts, joins, identities, timestamps, locations, and citations must come from
hardcoded parameterized operations over preserved evidence—not model prose.
Unknown questions, missing targets, unsupported modalities, ambiguous schemas,
and conflicting observations must fail closed or request clarification.

## Mandatory reading before any action

Read completely, in this order:

1. `AGENTS.md`
2. `NEXUSAI_CONTINUATION.md`
3. `docs/design/nexusai-family-adapter-agent-api-architecture.md`
4. `reports/nexusai-phase7.1-runtime-hardening-and-platform-audit-20260804.md`
5. `reports/nexusai-phase7.1-subscriber-identity-source-acceptance-20260804.md`
6. the task-relevant `.agents/*.md` guides

Reconcile documents with the actual worktree and live runtime. The worktree is
intentionally large, dirty, unstaged, uncommitted, and unpushed. Preserve every
existing user change. Never stage, commit, push, open a PR, delete retained
state, or publish externally unless the user explicitly authorizes that action.

## Non-negotiable product standard

Every delivered capability must be:

- real-data-driven and honest about no-data states;
- tenant/case/collection scoped at every API, agent, job, report, and export;
- deterministic for exact operations and parameterized against injection;
- source-cited down to evidence/version/file/row/page/frame/time locator;
- privacy-aware, access-controlled, audited, and fail-closed;
- independently versioned by adapter, operation, prompt, and response contract;
- exposed through documented reusable APIs, not only UI buttons;
- responsive, keyboard-accessible, clear, dense without clutter, and usable by
  a working analyst under time pressure;
- covered by Pakistan-shaped goldens, negative/ambiguity/privacy tests, live
  Agent Chat and Records acceptance, and rollback evidence.

Do not claim a capability because a catalog entry, UI card, model, prompt, or
adapter skeleton exists. Distinguish **source-verified**, **runtime-activated**,
**data-accepted**, and **planned**.

## Current accepted truth

- Phase 6 CDR, IPDR, ANPR records intelligence and the case workspace are
  live-accepted.
- The latest hardening rebuild passed `Phase6Activation=PASS`; API, worker,
  and LocalAI builds took 19.8, 4.1, and 375.4 seconds on the first attempt
  with 6.77 GiB free. Rollback images and named volumes were preserved.
- The root LocalAI Docker context is now 128.95 MB rather than the prior
  1.25 GB transfer, confirming that host Codex caches are excluded.
- All health surfaces return HTTP 200. Five specialist profiles are active.
- Installed models remain exactly:
  - `qwen_qwen3-4b-instruct-2507`: optional bounded chat explanation only;
  - `qwen3-embedding-0.6b`: Knowledge Base embedding/retrieval only.
- Do not download a model or backend without explicit approval and a written
  benchmark plan. This is a CPU-only approximately 16 GiB laptop.
- Official Qwen3-ASR support does not list Urdu, so its 0.6B and 1.7B entries
  are explicitly ineligible for the Pakistan Urdu/English primary ASR role.
  Whisper remains benchmark-only; PaddleOCR PP-OCRv5 is OCR-eligible because
  its official Arabic-script model lists Urdu/Pashto/Sindhi/Balochi/English.
- Source catalog: five adapters and 42 public operations.
- Governed case `records-demo-verified`: 9,250 accepted rows—CDR 5,000, IPDR
  2,500, access/security 1,000, ANPR 750, subscriber 0.
- Subscriber identity is runtime-activated: adapter 1.1.0, six public analyst
  workflows, Specialist 1.1.0, enterprise responses, and deterministic Agent
  Chat. The professional no-data path is live-verified. Populated subscriber
  behavior is not yet data-accepted.
- Live Subscriber Agent Chat completed `subscriber.status_summary` through
  `forensic_hybrid_query` in the same second with no model.
- Live Records UI rendered a bounded no-data answer with two findings, one
  source reference, audit/export controls, no page overflow at 1280 px, and
  zero browser diagnostics.
- The collection-governance, Docker-context, multilingual-routing, and CNIC
  masking hardening described below is deployed and focused-live-accepted.

## Deployed hardening truth

1. Native forensic agent create/update/import now bind Knowledge Base use to
   `forensic_collection_id` and no longer create agent-name collections.
   Generic agents retain historical name-based collection behavior.
2. `.dockerignore` excludes `.codex-tmp`, `.codex-cache`, `.cache`, browser
   reports, coverage, and other host-only artifacts. The previous root context
   transferred 1.25 GB; about 696.4 MiB and 368.8 MiB came from the first two
   Codex paths. Do not remove `.git` because version stamping consumes it.
3. Cross-family subscriber relations mask CNIC as nine asterisks plus the last
   four digits while retaining raw identity only as an internal grouping key.
4. A 24-question English, Roman Urdu, and Urdu subscriber golden corpus drives
   both planner and deterministic Agent Chat routing. The earlier image
   incorrectly returned 20 unrelated rows for `subscriber ki maloomat dikhao`;
   the deployed runtime now selects exact subscriber lookup and asks for an
   MSISDN/reference/IMSI/IMEI when missing.
5. Focused direct-agent tests pass. Focused Phase 7.1 plus cross-family API
   tests pass 26/26 selected specs.
6. Live post-refresh checks pass: four health probes return 200, only the two
   approved models are present, five agents are active, the catalog remains 28
   collections, Roman Urdu missing-target routing clarifies, an exact Pakistan
   mobile lookup returns a bounded negative result, and explicit UTF-8 Urdu
   selects `subscriber.status_summary` with zero model use.
7. At 1280 x 720 the Analyze desk has no horizontal overflow or browser
   diagnostics. Switching from `records-demo-verified` to
   `Subscriber_Identity_Analyst` and back succeeds.

Two live collections—`Subscriber_Identity_Analyst` and
`subscriber_identity_analyst`—already exist because of the old provisioning
defect. Do not delete them automatically. Treat them as manifest-backed cleanup
candidates and request explicit operator approval before cleanup.

## Immediate objective: finish Phase 7.1 data acceptance honestly

### Gate A — hardening activation: complete

This gate is complete. Do not rebuild again merely to repeat it. Keep the
24-question source corpus and focused live subset as regression gates. When
testing Urdu from Windows PowerShell 5.1, send request JSON as explicit UTF-8
bytes; a default string body can be encoded incorrectly and create a false
routing failure. Cross-family full-CNIC privacy remains source-test accepted;
repeat it against populated synthetic subscriber data in Gate B.

### Gate B — populated subscriber acceptance

Do not claim this gate without data. Obtain explicit authorization before any
retained upload/backfill. Prefer a clearly synthetic Pakistan fixture with no
real CNIC, subscriber, or operator data.

For the authorized fixture:

1. record source hash, evidence ID, version ID, batch/job ID, accepted/rejected/
   duplicate counts, and exact cleanup/rollback procedure;
2. include 03xx, +92, and canonical phone shapes; MCC 410 IMSIs; valid and
   invalid IMEIs; 13-digit synthetic CNIC shapes; Urdu names; explicit statuses;
   open/closed validity windows; conflicts; and reuse candidates;
3. run all six operations: identity lookup, validity timeline, device/SIM
   links, status summary, conflict audit, and reuse candidates;
4. verify masked CNIC, default name omission, target-type restrictions,
   source-row citations, request/case/tenant scope, audit trace, deterministic
   counts, no inference, CSV/JSON/audit export, and pagination/limits;
5. test Records and Subscriber Specialist at 390/820/1024/1440 px, keyboard
   focus, loading/no-data/error/success states, console, first-party requests,
   and page overflow;
6. run one explicit Qwen explanation over a deterministic fact packet; compare
   every number and locator with the response objects. A bounded deterministic
   fallback is acceptable. Model prose never owns citations;
7. record peak RAM, latencies, row accounting, screenshots, and exact pass/fail
   evidence before declaring Phase 7.1 data-accepted.

## Pakistan evidence rules

- Preserve raw values and store normalized values separately.
- Normalize Pakistan phones conservatively across local 03xx, `92...`, and
  `+92...` forms. Do not infer the current operator from a prefix.
- CNIC is a sensitive evidence field. Validate its 13-digit shape separately,
  mask it by default, and never accept it as a default subscriber lookup target.
  A future full reveal requires distinct role permission, reason, audit event,
  UI warning, export policy, and acceptance gate.
- IMSI MCC 410 is a format signal, not identity or ownership proof. IMEI length
  and check-digit validity are review signals, not fraud conclusions.
- Preserve Urdu, Roman Urdu, English, Punjabi, Sindhi, Pashto, Balochi, and
  Saraiki source text. Never invent a transliteration.
- Use `Asia/Karachi` for configured local interpretation and preserve explicit
  offsets. Never silently fill a missing timestamp zone.
- Vehicle plates vary by province, issuance period, typography, and source OCR.
  Use versioned provincial schema packs and preserve raw/normalized/OCR values.
- Tower/site aliases, coordinates, azimuth, range, and technology are supplied,
  time-versioned facts. A tower observation is not precise GPS, home, route, or
  continuous presence.
- Correlation means cited co-observation under a declared key/time rule; it is
  not identity, ownership, association, causation, intent, guilt, or fraud.

## Architecture of the Knowledge Base Fabric

Implement each family through the same contract chain:

`evidence registration -> immutable version/hash -> safe format decoder ->
family adapter -> canonical records/entities -> versioned deterministic
operations -> cited KB artifacts -> specialist agent -> enterprise response ->
Records/Relationships/Reports UI -> versioned external API`

The Knowledge Base must provide:

- hybrid dense/sparse retrieval with benchmarked multilingual embeddings;
- immutable document/chunk versions and content/source hashes;
- cross-entity linking with explicit match method and confidence;
- automated enrichment only as versioned derived artifacts;
- per-tenant/case/collection/field access control;
- complete audit trail for read, reveal, query, export, model use, and mutation;
- source-page/row/frame/time citations;
- retention/legal-hold and approved disposition controls;
- high-performance retrieval with measured latency/recall and no-answer tests;
- reusable REST/OpenAPI contracts, service-account scopes, idempotency,
  pagination, async jobs/events, and SDK examples.

## Agent/model operating model

- Orchestrator determines case scope and delegates only to registered family
  operations.
- Each family specialist has a versioned prompt, operation allowlist, evidence
  boundary, prohibited inferences, clarification order, and response schema.
- Deterministic tools execute before any explanation model.
- The Qwen 4B model may explain bounded result packets only on explicit
  explain/interpret/summarize/brief intent. It may not invent or rewrite facts,
  locators, identities, relationships, or locations.
- The embedding model is never selectable as a chat answer model.
- OCR, vision, ASR, speaker, face, and TTS candidates remain unavailable until
  separate dataset, resource, quality, privacy, and live acceptance gates pass.

## Next phase sequence after Phase 7.1

### Phase 7.2 — Tower/Site Intelligence

Implement first because Pakistan CDR location analysis depends on it:

- provider/versioned schema packs and time-valid alias/reference tables;
- exact site/cell/LAC/TAC/sector lookup and explicit CDR/subscriber joins;
- coordinate, azimuth, range, and impossible/conflicting-coordinate review;
- straight-line distance clearly separated from route/RF coverage;
- dedicated Tower/Location specialist, typed map/table result, citations,
  exports, Pakistan provider/timezone drift fixtures, raw-language corpus,
  API/agent/UI/a11y/performance/rollback/live gates.

### Phase 7.3 — Pakistan CDR Deepening

- real provider header/schema packs without vendor lock-in;
- calls/SMS/data/USSD/VoLTE classification and duration semantics;
- IMSI/IMEI changes, multi-SIM/device observations, explicit tower joins;
- chronological reconstruction, contact/network graphs with declared edges,
  location uncertainty, Urdu/Roman Urdu corpus, large-file performance,
  provider-clock drift, export/report gates.

### Phase 7.4 — ANPR Image Recognition

- image evidence child artifacts and immutable metadata;
- province-aware Urdu/English plate detection/OCR benchmark;
- raw OCR, normalized plate, confidence, bounding boxes, frame/camera/time
  provenance, human review for low confidence;
- camera-clock calibration and linkage to existing ANPR records;
- no owner/driver/occupant/association/route inference.

### Phase 8 — Image Forensics

- EXIF/container/hash analysis, object detection, similarity, duplicate/near-
  duplicate search, tamper indicators, and visual evidence correlation;
- face detection/embedding/recognition only behind explicit legal/policy,
  threshold, false-match, demographic-bias, human-review, and audit gates;
- specialist models create derived artifacts, never silent source replacement.

### Phase 9 — Urdu/English Speech and TTS

- benchmark Whisper/Qwen ASR candidates on consented synthetic/public
  Pakistan-accent, Urdu-English code-switching, noise, channel, and call-quality
  fixtures using WER/CER, timestamps, memory, and latency;
- preserve audio hashes and transcript time spans; diarization/speaker identity
  remain separate readiness gates;
- index accepted transcripts into KB with time citations;
- add TTS only for clearly labeled synthesized reports/alerts, with voice and
  provenance disclosure. Never present synthetic speech as evidence.

### Phase 10 — Supporting families

Vehicle registry/ownership, financial transactions, access/security, social and
digital footprint, generic mapping, and advanced case/report workflows each
ship as separate bounded slices with authorization, Pakistan schemas, agents,
APIs, tests, and live gates.

## UI/UX definition of done

- Case first: one authoritative active case across every surface.
- Ask-first workflow with optional expert controls, not a development console.
- Results lead with status, plain-language answer, exact scope, KPIs, compact
  table/timeline/graph/map only when appropriate, citations, limitations, audit
  trace, export, and executable next checks.
- No giant empty panels, uncontrolled chip walls, duplicate controls, raw JSON
  by default, unexplained jargon, or silent collection switching.
- Every state—loading, partial, no-data, clarification, unauthorized,
  unsupported, failed, and successful—is deliberate and actionable.
- WCAG-oriented keyboard/focus/contrast/labels, 390/820/1024/1440 responsive
  acceptance, no page overflow, no broken assets, and clean diagnostics.

## Testing requirements

For every slice, test:

- adapter detection/normalization/validation and schema drift;
- raw-value preservation and deterministic hashes/idempotency;
- parameterized operation correctness, pagination, date boundaries, timezones,
  duplicates, nulls, conflicts, and large-input limits;
- English/Roman Urdu/Urdu natural-language routing, ambiguity, missing target,
  unknown query, and capability guard;
- privacy/non-inference and cross-family leakage regressions;
- enterprise response, citations, execution trace, audit events, and exports;
- direct specialist chat, optional model explanation/fallback, and role guard;
- UI task flow, keyboard/a11y, responsive layout, console/network, and visual
  output clarity;
- resource use, rollback, retained-state safety, and exact row accounting.

Use focused tests first. Ask before a long build or benchmark. Never lower a
coverage, memory, timeout, privacy, or acceptance gate merely to make it green.

## Required handoff behavior

Lead with verified outcomes, not promises. Explicitly list what is live,
source-only, data-pending, blocked by authorization, and planned. Update
`NEXUSAI_CONTINUATION.md` and the current phase report after every meaningful
slice. Give the operator exact PowerShell commands only when a rebuild/live gate
is actually required. Do not repeat already accepted expensive gates unless
related source changed.

Proceed with the immediate Phase 7.1 populated-data acceptance objective. If
retained synthetic data or cleanup authorization is unavailable, complete every
safe source/read-only gate, report the exact boundary, and do not falsely mark
the slice complete.

---
