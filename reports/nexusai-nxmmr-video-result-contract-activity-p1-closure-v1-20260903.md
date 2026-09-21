# NX-MMR video result contract and Activity P1 source closure

Work item: NX-MMR-VIDEO-RESULT-CONTRACT-AND-ACTIVITY-P1-CLOSURE-V1.
Date: 2026-09-03. **ACTIVATION VERIFIED; LIVE FAILED-CELL ACCEPTANCE PASS WITH LIMITATIONS.**
This is source closure of two demonstrated defects, not product certification or NX-B2 admission.

Final replay is recorded in the authoritative final overlay of
`reports/nexusai-nxmmr-post-activation-live-p1-acceptance-v1-20260902.md`:
four fresh Ask cells, source seeks and new/historical Activity reopens PASS;
replay-scope P0/P1=0/0, runtime/evidence/seal unchanged. The immutable operator
receipt's PENDING field reflects its earlier timestamp, not the later replay.
Older pending statements below are historical. Do not rerun activation.

## Latest: activation receipt 20260903T063719719Z VERIFIED

User operator successfully built both images and recreated api/forensic-records-api.
RAM recovered above 6 GiB at all required checkpoints; no forced bypass and no
guarded drain in the supplied log. Both activation and verification report PASS.
Independent read-only checks validate receipt/source hashes, expected live images,
zero candidate restarts, health, jobs zero, required models, protected identities
and retained tuple 63|63|76|22507|827|61. Worker restart count remains 9.
Receipt live_failed_cells_acceptance remains PENDING; do not infer functional
video/Activity acceptance from deployment success. No replay in this check.
The orphan warning refers to deliberately omitted protected services; do not
remove them. The command below is now historical/used: DO NOT RERUN IT.
Next is the bounded video/Activity failed-cell replay; all earlier source-only
and preflight-blocked sections below are historical phase evidence.

## Latest post-reboot baseline reconciliation (06:35 UTC)

User-authorized baseline-only update: unchanged container/image IDs, forensic API
restart count 0 -> 8 and worker 2 -> 9 after dependency-readiness startup retries.
API logs show NATS DNS timeout/PostgreSQL starting; worker logs show PostgreSQL
DNS unavailable. Both stabilized. Prior counters and prior seal are audited in
manifest.runtime_baseline_reconciliation; no blanket restart tolerance added.
52 operator checks PASS, including rejection of further API/worker drift.
Only manifest and operator-test bytes changed among the 428 sealed files.
Current seal SHA256: `0b11a1b6cf1b27dc890dd17ceb306cbc255a00ef3f7ba16b5e216d293fcf3127`.
Preflight: RAM-only BLOCKED (4.7279815673828125/6 GiB); other checks PASS, jobs 0,
retained counts unchanged. No build, activation, cache cleanup, model unload or
app termination performed. The user will close optional apps and run the existing
exact command below; do not restart Docker again or bypass RAM/integrity gates.
No claim that closing apps guarantees adequate RAM. Live P1 acceptance pending.
The original source-phase measurements/hash below are historical, superseded
only by this explicit baseline reconciliation; analytical code remains unchanged.

## Original source-phase runtime and receipt

Accepted activation receipt remains `20260903T050747951Z`. The used four-P1 bundle must not be resumed for this candidate.
Read-only preflight at 2026-09-03T06:01:49Z: health PASS, active jobs 0, models present, rollback/Compose/integrity PASS.
All five container IDs, immutable image IDs and restart counts match the new manifest's starting_runtime exactly.
Worker restart count 2 is preexisting; other service restart counts remain 0.
Retained tuple unchanged: `63|63|76|22507|827|61`.
RAM alone blocks activation: 1.4148445129394531 GiB available versus unchanged 6 GiB floor; disk 1638.349 GiB.
This measurement is a snapshot, not a guarantee of future availability. No cleanup, process termination, cache drop or drain was performed.

## Two remaining live P1s and root causes

1. Generic recursive enterprise flattening recursed into nested video bbox/locator maps instead of retaining observation/group rows. Subsequent public/tool projections could repeat that loss; the structured ANPR public enricher could also invent an irrelevant canonical-table citation for an empty video result.
2. Activity assumed historical optional table columns were arrays and called columns.map on null.

The STIM skill's bounded-slice, independent contract expectations and presentation gates guided the scoped correction.
No SQL, video processor, model selection, ASR/OCR/similarity algorithm, state machine, authorization, cross-family planning or database schema was changed.

## Enterprise row contract and raw/group semantics

Video-only projection copies top-level rows, bounded to 100. Nested crop/locator maps remain children, never rows.
Raw observation: normalized_plate_text/raw_plate_text remains the observed reading; group_selected_plate_text remains separate group context.
Group candidate: result_semantics=video_anpr_group; its retained first/last source range is not replaced by a best-observation instant.
Presentation carries plate, group candidate, match_kind, source_time, optional raw frame_number, source_file, sightings_count and manual_review_required.
Numeric source times and original raw/group/best locators remain available in retained metadata.
Group navigation aliases retained first/last times to start/end seconds; no best-frame number is attached to the group start.
Enterprise columns use existing key/header descriptors; priority_columns are explicit arrays, including non-null empty arrays.
Tool rows are additionally bounded by their plan's MaximumRows and retain model-observation authority.
Public video responses bypass structured ANPR canonical-record enrichment.

## Positive, negative and citations

Synthetic positive: two raw ABC123 readings at seconds 1 and 2, frames 60 and 120, with ABC128 separate group context.
Synthetic group: one ABC128 candidate over seconds 1–2, best observation at second 2; not substituted for raw observations.
Synthetic negative: zero rows/citations; “No exact observation of XYZ999 was found in this video.” The evidence UUID is not the display target.
Citations carry evidence ID, version ID, artifact ID, friendly source file and source locator, including frame/time where supplied.
Tool observation citations are matched to artifact + evidence + version, not every observation in the same video.
No changed retrieval or SQL logic, new identity conclusion, or manufactured observation.

## Activity and historical compatibility

Agent table arrays are normalized before retained metadata serialization.
Activity defensively validates the presentation object, table rows/columns/priority columns, optional metrics, limitations and findings.
The production ActivityResultTable renders the normalized model. Historical bbox-only video or invalid-row tables show “Result details are unavailable for this historical entry.”
Missing generic scalar columns may be derived; malformed video rows are never reconstructed into plate facts.
Original historical records are not rewritten or deleted. Status logic remains unchanged.
Ask and Activity receive the same corrected agent presentation; positive, negative and group contracts are tested after serialization/reopening.

## Tests and limits

PASS: focused api/forensic_records suite covering video, OCR, similarity, composition, transcript, canonical/CDR, scope and unrestricted-SQL rejection.
PASS: focused core/services/agents suite covering full-shape bridge, forensic presentation, four-P1 regressions and scope/status.
PASS: go vet for both packages.
PASS: 45 frontend node tests, zero failures/skips with NX_VIDEO_CONTRACT_DIR set.
PASS: 50 Windows PowerShell operator static/behavioral checks; source manifest validation covers 428 files.
ESLint on changed frontend files: zero errors, two JSX-import unused warnings (Link and ActivityResultTable). Both imports are used in JSX.
The full-shape test runs real records-to-enterprise construction, public/tool adapters, agent presentation, AnswerMetadata serialization/history decoding and server-rendered production ActivityResultTable.
Expected values are independently specified synthetic plate/time/frame/source values, not generated from the function under test.
Both typed maps and JSON-decoded row arrays are checked; row bounding is checked.
Navigation parameters verify evidence, artifact/finding and source_time. This is not a new live browser citation click or real database persistence test.
No production UI bundle build or image build was run. Database-dependent specs remain skipped without a test database; no full-repository certification is claimed.
Historical Activity null/invalid rows and bbox-only tables, ordinary scalar fallback, positive/group/negative reopening, shared presentation, STT hint/time, source navigation and status distinctions pass.
OCR, image similarity, face candidate and cross-family regression assertions pass within these focused suites; this is not fresh live requalification.

Reproduction (offline cached dependencies, sequential):
```powershell
$env:GOMAXPROCS='2'
$env:GOPROXY='off'
$env:NX_VIDEO_CONTRACT_DIR='C:/Users/sheik/Workspace/Office/Projects/NexusAI/local-acceptance-models/nxmmr/video-contract-tests'
go test -p 1 ./api/forensic_records -run 'TestForensicRecordsSynthesis|TestSharedAsk|TestDataGenerated|Test.*Transcript|Test.*Composition|TestVideo' '-ginkgo.focus=video|OCR|similarity|composition|transcript|canonical|CDR|scope|unauthorized|unrestricted SQL' '-ginkgo.no-color' -count=1
go test -p 1 ./core/services/agents -run 'TestAgents|Test.*(Transcript|Scope|Similarity|Status)' '-ginkgo.focus=video full-shape|forensic presentation|four P1|forensic.*(scope|transcript)' '-ginkgo.no-color' -count=1
go vet -p 1 ./api/forensic_records ./core/services/agents
Push-Location core/http/react-ui
node --test src/analyst/*.test.js
Pop-Location
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\test_nxmmr_video_result_operator_bundle.ps1
```

## Files changed in this slice

Production:
- api/forensic_records/query.go
- api/forensic_records/video_result_presentation.go (new)
- api/forensic_records/cases.go
- api/forensic_records/nxa1_execution_foundation.go
- core/services/agents/forensic_presentation.go
- core/http/react-ui/src/analyst/analystActivityPresentation.js
- core/http/react-ui/src/analyst/ActivityResultTable.js (new)
- core/http/react-ui/src/analyst/AnalystHistory.jsx

Tests:
- api/forensic_records/video_result_contract_test.go (new)
- core/services/agents/video_result_contract_test.go (new)
- core/http/react-ui/src/analyst/ActivityResultTable.test.js (new)

Operator:
- configuration/nxmmr_video_result_activation_v1.json
- configuration/nxmmr_video_result_operator_integrity_v1.json
- scripts/nxmmr_video_result_operator_common.ps1
- scripts/preflight_nxmmr_video_result_activation.ps1
- scripts/run_nxmmr_video_result_activation.ps1
- scripts/activate_nxmmr_video_result.ps1
- scripts/verify_nxmmr_video_result_activation.ps1
- scripts/rollback_nxmmr_video_result_activation.ps1
- scripts/test_nxmmr_video_result_operator_bundle.ps1

Handoff: this report, NEXUSAI_CONTINUATION.md, sole roadmap/phase ledger, maturity/readiness matrices and demo runbook.
Existing unrelated dirty worktree changes are preserved; full diff versus HEAD also includes earlier work.
Git diff --check PASS (existing CRLF conversion warnings, no whitespace errors).

## Source seal and minimal service scope

Seal: configuration/nxmmr_video_result_operator_integrity_v1.json.
SHA256: `0d580a6746425fc23bc0f512189de02c4fe4726a191adcec22b39b943f77956a`.
428 sealed files. Prior used seal/receipts unchanged; new candidate manifest and namespace.
Only api and forensic-records-api may later build/recreate. Worker, PostgreSQL and NATS must preserve exact identity, image and restart count.
New private receipt root: local-acceptance-models/nxmmr/private-activation-video-result.
Candidate image IDs are intentionally null until the operator builds and seals them.
Sequential/resumable builds; RAM checkpoints; guarded drain only after both candidates are built/sealed; immutable rollback.
No runtime pull/model acquisition; required Dockerfile build-dependency networking is retained from the proven bundle.
**Fully air-gapped build readiness is not proven**; no claim of OfflineActivationBundleReady=true. No downloads were performed in this phase.

## Exact future operator command

Run only when separately authorized to deploy, from the repository:
```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_nxmmr_video_result_activation.ps1 -WaitForRamMinutes 120
```
Review the plan and explicit confirmation prompt. This is a guarded command, not a guarantee that 6 GiB will become available.
It must fail closed for RAM or any other unmet gate; do not lower thresholds or use an older activation.
After VERIFIED, replay only video positive/absent plate, relevant group/time/frame/citation cells and historical/new Activity reopening.
Keep source success separate from later activation verification and live acceptance.

## Privacy, impact and open gates

Synthetic-only test artifacts are in ignored local acceptance storage; no copied private evidence, credentials or analytical answers in source.
No hardcoded user plate/UUID/source answers and no self-oracle; no new SQL or cross-case retrieval path.
No deployment, restart, image build, cache clearing, history deletion, migration, volume change, retained evidence/history write, reprocessing, model download, product certification or NX-B2 activation.
Open P0: 0 observed in this bounded scope. Open live P1: 2 until separately activated/replayed. Source defects corrected under passing focused tests.
Deferred P2s (off-page names, Target/Current evidence labels, unrelated AND-3 date parsing) unchanged.
**Exact next action: stop this source phase; separately authorize/run the new guarded activation, then bounded failed-cell replay. NX-B2 remains paused.**
