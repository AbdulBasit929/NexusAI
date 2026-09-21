# NX-MMR four-P1 software-defect closure — source-only

Subsequent live outcome: user activation `20260903T050747951Z` passed, but the
six-query replay remains PARTIAL/NOT READY. OCR and similarity Ask/Activity pass;
video enterprise projection/provenance and null-column Activity crash remain.
The four-P1 overlay in `nexusai-nxmmr-post-activation-live-p1-acceptance-v1-20260902.md`
controls current live truth. This source report and original seal remain historical
proof, not a claim of complete end-to-end closure; do not rerun the used bundle.

Work item: `NX-MMR-FOUR-P1-SOFTWARE-DEFECT-CLOSURE-V1`.
Date: 2026-09-03. Boundary: CORRECT + TEST + SEAL, then STOP.

## Outcome and current truth

The four bounded source corrections are tested and separately sealed. No image
build, deployment, live replay, model acquisition, evidence rewrite, database
migration, named-volume change, product certification or NX-B2 activation was
performed. Source closure is not live acceptance: R2 remains PARTIAL/NOT READY,
with four live P1 areas pending a later authorized deployment and failed-cell
replay. The STIM skill governed scope, oracle separation, presentation and ledger
updates; no new structured-intelligence or architecture phase was opened.

## Current runtime / retained state

Accepted receipt: `20260902T154901947Z`. All five original container/image
identities are unchanged; full immutable identities are recorded in the new
manifest. Restart counts: api 0, forensic API 0, worker 2 (pre-existing),
PostgreSQL 0, NATS 0. API/worker/database/broker health checks pass; forensic API
is running and passes the operator's health checks (no Docker healthcheck is
configured for that container). Active jobs 0. Retained tuple remains
`63|63|76|22507|827|61` (evidence, versions, jobs, canonical records, artifacts,
KB assets). No Ask or Activity requests were submitted in this source phase.

## Video SQL root cause

The failure is in `currentEvidenceResultFamiliesSQL`, before the grouped-video
query: `SELECT DISTINCT artifact_type ... ORDER BY artifact_type` over
`forensic.derived_artifacts`. The live database is Timescale 2.28.3. Under the
non-superuser/non-bypass-RLS `forensic_runtime` role, the tenant policy injects a
`Result` node beneath Timescale SkipScan. Read-only execution reproduced
`unsupported subplan type for SkipScan: Result` (SQLSTATE XX000). The same query
as superuser succeeds using Unique -> SkipScan -> Index Scan. This is an
observed role/policy/planner interaction, not an ORDER BY or LEFT JOIN guess.

Equivalent `SELECT artifact_type ... GROUP BY artifact_type ORDER BY
artifact_type` succeeds with the same scope predicates and unchanged index.
The read-only live plan was Sort -> HashAggregate -> Result -> scoped scan,
returning six families in 0.377 ms. No extension, index, global/session planner
setting or database schema was changed to repair production execution.

## Video exact positive / negative / raw-group semantics / citations

Exact lookup now searches retained raw ANPR observations as well as selected
group candidates. Each raw reading retains its own plate text, timestamp,
frame locator, artifact citation and `raw_plate_observation` match kind. A
same-tenant/case/evidence/current-version lateral lookup attaches the group's
different selected candidate without relabeling it. Raw readings take priority
over a redundant display row for the same group; selected-only candidates remain
explicitly tagged. Ordering is source time then stable artifact identity, bounded
by the requested limit. Time filters and current-version joins remain enforced.

The live independent oracle showed requested raw `MM51VSU` readings at 1 and
2 seconds while the group selected `MW51VSU`. Production code contains neither
these values nor the acceptance evidence identifiers. Synthetic DB tests instead
use raw `AB12CDE`, selected `AB12CDF`, and absent `QZ99XYZ`. They execute the real
scope-discovery and video operation SQL, assert two correctly timed/cited raw
readings, retain the different selected value, return one selected-only group,
and return `no_match_for_filter` for the absent plate. No text-repair, ownership,
identity, tracking, continuous movement or population-accuracy claim is added.

## Status root cause / state machine / persistence / Activity

`executeNativeLocalChat` previously published a JSON error and then `completed`.
The executor could also publish an unbounded `error: <diagnostic>` status. Later
completion replaced the failure in the registry/history, and Activity interpreted
completed-with-no-answer as results available.

The failure paths now publish a short canonical `error` and a generic analyst
message; full diagnostics remain in internal logs/returned errors. Empty generic
responses are rejected. Deterministic output is classified before delivery:
failed operation -> error, valid no-match/zero -> completed, clarification ->
needs_input. History and the in-memory registry reject late overwrites of all
terminal states. Activity prioritizes terminal failure over contradictory display
metadata and no longer claims success for a bare empty completed record. Ask
recognizes needs_input as a terminal request outcome. Existing retained history
was not rewritten.

Actual PostgreSQL/GORM tests create records, complete or fail them, send late
completed/processing/answer events, and reopen them through the scoped history
store. Error remains error with no answer; valid no-match remains completed;
clarification remains needs_input; empty output becomes error; another case
cannot reopen the record. These are persistence tests, not mocked callbacks.

## OCR projection / matching / citation

The writer's actual fields are `observation.raw_text` and
`observation.normalized_text`; the old SQL omitted both. OCR-specific projection
now prefers nonempty raw text, falling back to normalized text only if raw text
is empty/missing. Other transcript/document field precedence is unchanged.
Exact matching retains the existing neutral Unicode normalization; it does not
replace O/0, invent spelling corrections, or fabricate OCR.

A separate synthetic selected image stores the actual nested observation and
bbox locator shape. DB tests execute scope discovery plus `derivedTextEvidence`:
`Investigation Workspace` matches with original raw content, source filename,
artifact/version and region citation; normalized-only fallback matches; absent
`ZZ99ZZZ` and altered `Investigati0n Workspace` do not. The absent phrase exists
only in stale-version/other-case/other-tenant distractors and must not leak.

## Similarity presentation / source labels / Activity / object rendering

Scoring and ranking algorithms are unchanged. The existing selected-evidence
route now hydrates filenames for only its bounded authorized query/candidate
evidence IDs, matching current version. It does not enlarge the population or
depend on a browser catalog page. Provenance and Fact Packet citations preserve
query-observation versus candidate-observation roles and filenames.

Both image and face tables prioritize rank, similarity score, candidate filename
and query filename. Findings expose rank and score as review candidates, not
probabilities; the face title explicitly denies identity confirmation. Artifact
IDs/URIs and structured locators stay available in source navigation/technical
data rather than becoming primary table columns. Version and locator fields are
preserved in citations. Citation labels prefer filenames even when a raw label
is technical. Activity uses a safe scalar/locator formatter instead of implicit
object coercion for cells, metrics, findings and timeline values. Structured
unknown objects display a neutral placeholder, not `[object Object]`.

## Tests and independent oracles

| Check | Result | Qualification |
|---|---|---|
| Focused API suite | PASS | 44 Ginkgo specs plus selected legacy Go regressions for transcript, timing, OCR, similarity, video, canonical/CDR and scope |
| Final isolated DB rerun | PASS | 2 ordered multi-assertion specs; selected video and separate selected image; actual SQL under RLS |
| Focused agent suite | PASS | 72 Ginkgo specs plus selected legacy Go routing/status tests |
| Analysis history DB persistence | PASS | Real PostgreSQL/GORM create, terminal updates and scoped reopen |
| Analyst utility suite | PASS | 43 Node tests, no skips; includes error, no-match, citation labels and off-page source IDs |
| Changed JSX syntax | PASS | Babel parser: AgentChat and AnalystHistory; no UI build |
| Local request handler compile check | PASS | `go test -p 1 ./core/services/agentpool -run '^$'`; no tests/services run |
| Final citation/composition/Urdu check | PASS | Focused API regressions repeated after source closure |
| New operator bundle | PASS | 50 checks including separate identity/receipt root, protected restart drift rejection, RAM/job floors, sequential resume and rollback |
| Final read-only preflight | BLOCKED_RAM_ONLY | After test cleanup all non-RAM gates pass; 4.12570 GiB available, required 6; active jobs 0 |
| Git diff whitespace | PASS | Whole tracked-tree `git diff --check`; only existing line-ending warnings |
| Live browser replay | NOT RUN | Explicitly outside this phase |
| Image build / product certification | NOT RUN | Explicitly outside this phase |

The isolated DB used the already-local immutable Timescale image
`sha256:61f891691050da6032023c01ea885730eeeba06b7c17b403e7d0b9c49c37dfe9`,
database `nxmmr_p1_test`, loopback port 55439, a 384 MiB container limit and tmpfs
storage. Tests verify the exact database name before fixture writes. GORM tests
use per-test random schemas and cleanup. No production migrations were run.
The disposable container was stopped and removed after validation; its
synthetic data was discarded, not retained evidence. No images or named volumes
were removed. Tests can recreate these synthetic fixtures; the discarded tmpfs
is not recoverable.

Final isolated RLS plan: Sort -> HashAggregate -> Result (tenant one-time filter)
-> scoped Seq Scan; 803 matching synthetic artifacts, eight families, execution
0.355 ms, planning 0.142 ms. The preserved evidence index is available; no planner
forcing is used. The fixtures' expected values are declared independently of
the answer builder, use different plates/IDs from live evidence, and are checked
against actual SQL output. Existing shared regressions remain independent of the
new positive/negative fixtures. This is bounded contract proof, not certification.

Urdu/English timing anaphora, selected-audio intent isolation, structured/CDR
query construction, cross-source correlation/provenance, image result limits,
face safety and catalog-independent citation labeling remain covered by the
focused suites. Prior live passes are preserved as historical live proof, not
rerun or relabeled as new live proof.

## Environment limitations and corrected test attempts

Initial PowerShell invocations split an unquoted `-ginkgo.focus` argument; quoted
arguments ran successfully. Sandbox access to the existing Go cache required
approved escalation. A presentation fixture initially used Go slices rather
than the real JSON-decoded array shape; it was corrected to the wire shape.
Optional nil citation fields were removed to retain the existing CDR display
contract. The lifecycle assertion now requires a canonical public error while
still asserting that the returned internal error retains its diagnostic. No
coverage gate was lowered, no failing test was skipped to manufacture a pass.
An unavailable esbuild import was replaced by the already-installed Babel parser;
no package download was performed. No full frontend build or browser rendering
claim is made in this source-only phase.

## Security / scope / anti-hardcode / privacy

All changed analytical SQL remains parameterized and tenant/case/current-version
bounded under RLS. No unrestricted SQL endpoint was added. No raw embeddings,
private keys, face identity conclusions, investigator evidence or diagnostic SQL
is added to primary analyst presentation. Acceptance values occur only in
regression inputs/reporting, not analytical decisions. User changes and the dirty
worktree were preserved; no reset, checkout, staging or commit was performed.

## Files changed

Production: `api/forensic_records/{derived_text_query.go,video_anpr.go,
video_anpr_exact.go,similarity_query.go,query.go,stim_fact_packet.go}`;
`core/services/agents/{analysis_history.go,lifecycle.go,executor.go,
forensic_presentation.go}`; `core/services/agentpool/agent_pool.go`;
`core/http/react-ui/src/pages/AgentChat.jsx`;
`core/http/react-ui/src/analyst/{AnalystHistory.jsx,analystAskPresentation.js,
analystActivityPresentation.js}`.

Tests/support: `api/forensic_records/four_p1_db_test.go`,
`core/services/agents/{four_p1_test.go,lifecycle_test.go}`,
`core/services/testutil/testdb.go`, `analystActivityPresentation.test.js`.
Seven new four-P1 operator scripts, a new activation manifest and integrity seal;
the old R2 operator files/receipts were not repurposed. The continuation,
single roadmap/ledger, maturity/readiness artifacts and demo runbook were updated
without promoting live readiness.

## New source seal and activation bundle

415 source/config/test/operator files are individually SHA-256 sealed in
`configuration/nxmmr_four_p1_operator_integrity_v1.json`.
Seal-file SHA-256:
`2d20b3cce2ebc581d884f0430cff1069861dc36884019d622f1cff6969d75340`.
Independent .NET SHA-256 recomputation matched all 415 entries (not just the
operator self-test). New manifest and both updated readiness/maturity JSON files
parse successfully. Final `git diff --check` exited zero.
New namespace `NX-MMR-FOUR-P1-SOFTWARE-DEFECT-CLOSURE-V1`; new private receipt
root `local-acceptance-models/nxmmr/private-activation-four-p1`.
The manifest captures exact current container/image/restart identities and
retained counts. Candidate images have NOT been built; their immutable IDs are
intentionally unset until a later source-bound build-set receipt captures them.

Future mutable services: **api and forensic-records-api only**. Protected:
forensic-records-worker, forensic-postgres, forensic-nats. No worker rebuild,
model-role change, migration or named-volume change. Source/config drift rejects
the bundle; previously used R2 build receipts cannot be resumed in this identity.

## RAM / offline operator plan

The 6 GiB available-RAM gate is unchanged. Disk cleanup does not itself create
RAM, and no evidence/data deletion or arbitrary process termination was used.
The later operator waits with a bounded timeout, builds sequentially, preserves
resumable source-bound image receipts and checks RAM at admission/build/recreate
boundaries. Only clean Linux page-cache mode 1 is available after dirty/writeback
reach zero. A two-service drain is permitted only after both candidates and the
rollback are complete. Failure before mutation leaves R2 running.

“Offline operator” means the standalone terminal workflow with optional apps
closed, not a claim that building is air-gapped. Existing policy permits only
approved Dockerfile build dependencies over the default build network; there is
no forced base refresh, model download or runtime package acquisition. Runtime
recreation is no-build/no-pull. Sufficient RAM cannot be guaranteed; admission
fails closed instead of bypassing the floor.

## Rollback plan

The later activation captures a private exact runtime topology and immutable
original image IDs before any service mutation. Rollback restores only the two
mutable services from its integrity-checked compose receipt, verifies environment,
mount/port/network parity, protected identities/restarts, worker roles, health and
retained counts. The old R2 receipt is the baseline, not a script to rerun.

## Exact future operator command

For a later separately authorized activation only; it was NOT executed now:

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_nxmmr_four_p1_activation.ps1 -WaitForRamMinutes 120
```

The command asks for `YES` only after admission checks. Do not invoke the old
`run_nxmmr_p1_correction_activation.ps1`. After successful future activation,
replay only the failed video positive/negative/status, OCR positive/negative,
and image/face Ask/Activity presentation cells. If those pass, stop P1 work.

## Open P0 / P1 / exact next action

No new P0 identified. Four targeted source defects corrected/tested; **four live
P1 areas remain** until deployment/replay. No blanket platform certification.
STOP at the sealed source candidate. Next action requires separate activation
authorization and the unchanged RAM admission floor, not another source phase.

## Proven markers (source proof unless explicitly runtime)

```text
CurrentR2RuntimePreserved=true
CurrentR2Receipt=20260902T154901947Z
VideoSkipScanRootCause=DISTINCT_artifact_type_with_RLS_Result_under_Timescale_SkipScan
VideoQueryDBBackedPositive=PASS
VideoQueryDBBackedNegative=PASS
VideoRawAlternativeSemantics=PASS
VideoNoMatchSemantics=PASS
TerminalStatusStateMachine=PASS
FailureStatusPersistence=PASS
NoMatchStatusPersistence=PASS
ActivityFailurePresentation=PASS
OCRProjectionFix=PASS
OCRDBBackedPositive=PASS
OCRDBBackedNegative=PASS
OCRCitation=PASS
ImageSimilarityPresentation=PASS
ImageSimilarityRankVisible=PASS
ImageSimilarityScoreVisible=PASS
ImageSimilarityActivity=PASS
FaceSimilarityPresentation=PASS
FaceSimilarityRankVisible=PASS
FaceSimilarityScoreVisible=PASS
FaceSimilarityActivity=PASS
FriendlyCitationPresentation=PASS
NoObjectObjectPresentation=PASS
UrduTimingRegression=PASS
EnglishTimingRegression=PASS
AudioIntentIsolationRegression=PASS
CrossFamilyCitationRegression=PASS
StructuredCDRRegression=PASS
CrossCaseLeakage=NONE_IN_TESTED_SCOPES
NoHardcodedAnalyticalAnswers=PASS
NoSelfOracle=PASS
NoUnrestrictedSQL=PASS
SourceSeal=PASS
OfflineActivationBundleReady=true
DeploymentPerformed=false
DatabaseMigration=false
VolumesChanged=false
RetainedStateMutated=false
ProductCertificationPerformed=false
NXB2ActivationPerformed=false
```
