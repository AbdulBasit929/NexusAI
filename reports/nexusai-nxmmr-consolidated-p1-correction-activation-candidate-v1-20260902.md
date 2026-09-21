# NX-MMR Consolidated P1 Correction Activation Candidate V1 — 2026-09-02

Build-policy handoff superseded by
[build-dependency repair R1](nexusai-nxmmr-p1-build-dependency-repair-r1-20260902.md).
The offline attempt failed before live mutation; required build dependencies
are now user-approved, and the bundle has new candidate tags and seal hashes.
The original build policy, hashes and validation results below are historical.

# Verified Starting State

The accepted live runtime remains healthy under activation receipt
`20260902T025246574Z`. Read-only reconciliation recorded `api`, worker,
PostgreSQL and NATS healthy, the forensic API running, restart counts zero,
active jobs zero, retained accounting `63|63|76|22507|827|61`, and no P0 or
cross-case leakage. The latest preflight measured 15.713 GiB total host RAM,
2.919 GiB free RAM and 1,640.876 GiB free disk.

# Current Live Receipt Preserved

`PreviousActivationPreserved=true`

`PreviousActivationReceipt=20260902T025246574Z`

No build, image tag, service stop, restart, recreation, migration, retained
query, evidence processing, model acquisition, cache deletion or volume change
was performed while preparing this candidate.

# Source P1 Correction Set

The consolidated source set preserves the already-tested corrections for
timing anaphora, explicit other-family intent under selected audio,
selected-video ANPR exact/no-match, bounded analytical status, selected-image
OCR exact/no-match, typed source-time citations, cross-family citation
fairness, SigLIP Ask presentation, face natural-language routing and
request-scoped NATS transport, and truthful Ask/Activity result presentation.
No Urdu ASR processing path or model role was retuned.

# Source Diff Reconciliation

The actual production ownership was reconciled from source and Dockerfiles.
The correction changes LocalAI/API/UI code and `api/forensic_records` code.
No post-activation worker production source belongs to this correction.
Therefore the technically correct deployment scope is two services:
`api` and `forensic-records-api`. The worker is protected rather than rebuilt,
as required by the continuation's minimum-correct-scope rule.

# Query / LLM Architecture

The source retains the governed flow: natural-language understanding, current
authorized scope, typed capability-aware plan, registered deterministic/model
operation, result and citation validation, then bounded answer presentation.
The LLM does not authorize scope, execute unrestricted SQL, invent identifiers,
times, scores or citations, or replace exact results with similarity.

# Timing / Citation Correction

English and Urdu same-scope timing follow-ups route to `source_time` rather
than literal pronoun search. Typed `start_seconds`/`end_seconds` locators survive
the Fact Packet/presentation path and become analyst-friendly source-time
navigation. Missing time remains `TIMING_UNAVAILABLE`.

# Selected-Video ANPR Correction

Explicit selected-video plate intent routes to retained grouped/timeline video
observations. Exact candidates keep source-time/citation semantics; absent
valid plate shapes return a normal exact no-match. One-, two- and
three-trailing-letter plate extraction is test-covered.

# OCR Exact Correction

The Data-generated exact recognized-text prompt routes to selected-image,
current-version OCR observations and preserves region citations. Absent exact
text returns no match without document/KB substitution. General semantic
retrieval remains a separate path.

# Cross-Family Citation Fairness

Composition preserves independent phone/CDR and plate/ANPR steps and reserves
one valid citation per completed authoritative step before filling the
remaining citation budget. It makes no identity, ownership or causation claim.

# SigLIP Presentation Correction

Persisted embedding similarity is presented as bounded ranked image
similarity with candidate sources, scores where allowed, review limitations and
citations. Raw vectors and the prior misleading image-metadata/no-model wording
remain excluded.

# Face Routing Correction

Natural face-candidate wording selects the bounded face-similarity operation.
Selected evidence/case scope crosses the distributed agent event, candidate
citations remain source-bound, and presentation uses similarity-only,
non-identity language with no demographics or raw vectors.

# Activity Status Correction

Terminal status is canonical and bounded. Result, exact no-match,
clarification and execution failure remain distinct, and similarity/face
results inherit their correct typed presentation rather than metadata labels.

# Urdu STT Regression Protection

The protected worker remains on explicit `asr_language=ur`,
`faster-whisper-small-ur`, Urdu-script admission, Devanagari/Latin-only trusted
Urdu rejection and Roman derivation only from accepted Urdu script. The bundle
does not rebuild or reconfigure it.

# Structured Regression

Structured CDR remains deterministic authority. Cross-family planning tests
pass with separate cited families and no unrestricted SQL or ownership
inference.

# Source Tests

- Go format check: PASS.
- Focused agent status, scope transport, selected-video routing, multilingual
  timing and typed citation tests: PASS.
- One-/two-/three-trailing-letter target extraction: PASS.
- Focused forensic API timing, exact OCR, current-version authorization,
  image/face similarity, composition and citation fairness tests: PASS.
- Video ANPR complete-zero/no-match and enterprise result-state tests: PASS.
- UI Ask/Activity presentation: 13/13 PASS.
- Focused `go vet` for agents, agentpool and forensic API: PASS.
- Agentpool compile-only check: PASS.
- Changed-file ESLint: PASS.
- PowerShell parse: PASS for all bundle scripts.
- Operator bundle self-test: 30/30 PASS.

# Security / Scope Tests

Selected evidence, current version and authorized backend candidate population
are enforced in focused tests. Unauthorized capability/scope tests pass.
`CrossCaseLeakage=NONE`. The activation preflight binds to the exact accepted
five-container starting runtime and fails closed on any identity drift.

# Anti-Hardcode

The correction's production files were scanned for the live acceptance phone,
plate, English transcript and Urdu transcript values; none were present.
`NoHardcodedAnalyticalAnswers=PASS`. Tests may contain synthetic expected
values, but runtime answers remain data-derived.

# Privacy

The operator bundle contains no embedded password, bearer token, API key or
private key. Private runtime snapshots are created only during the operator run
inside an ACL-restricted receipt directory. Logs do not print container
environment values. `PrivacyScan=PASS`.

# Git Diff Check

`git diff --check` passed. Existing line-ending notices remain informational.
The repository is a large pre-existing dirty worktree; no unrelated user change
was reset, cleaned, staged or discarded.

# Candidate Service Scope

Build/recreate: `api`, `forensic-records-api`.

Exact-identity protected: `forensic-records-worker`, `forensic-postgres`,
`forensic-nats`.

This intentionally supersedes the earlier expected three-service scope because
the final correction contains no worker production change.

# Source Seal

Manifest:
`configuration/nxmmr_p1_correction_activation_v1.json`

Manifest SHA-256:
`708dd60586a8b21d3cc71b74b964ac94be60335783879ab26ae7de949c637259`

Source/operator integrity seal:
`configuration/nxmmr_p1_correction_operator_integrity_v1.json`

Seal SHA-256:
`45b3b946e8e3bd628cb38ed8c59cab2d5387256182f04976e455fbe82f8f18bd`

Any sealed-file change invalidates candidate reuse and requires review/reseal.

# Candidate Images

No candidate image has been built in Codex. The offline operator will build,
seal and receipt these tags sequentially:

- `nexusai/localai-forensic:nxmmr-shared-p1-correction-v1`
- `nexusai/forensic-records-api:nxmmr-shared-p1-correction-v1`

Each completed immutable image ID is reusable only while the exact manifest,
source seal and original image baseline still match.

# RAM Strategy

The 6-GiB floor is unchanged. The single runner waits up to 120 minutes when
RAM is the only failed preflight gate, may release Linux clean page cache only
with `drop_caches=1` after both Dirty and Writeback are zero, builds one service
at a time, and rechecks RAM around every build and recreation boundary. Builds
use `network=none` and `pull=false`; a missing cached dependency stops the run
instead of downloading it.

# Safe Service Drain Strategy

Only after both candidate images, immutable IDs, rollback Compose, exact
runtime snapshot, source seal, zero-job guard and retained counts are verified,
the runner may stop `api` and `forensic-records-api` if the post-build 6-GiB
gate remains closed. The worker, PostgreSQL and NATS stay running and retain
exact identity. Any later failure invokes the bounded two-service rollback.

# Resume Strategy

Before building each service, the operator searches prior receipts for
completed image IDs bound to the identical manifest, source seal and original
images. Valid completed images are reused; invalid or drifted tags are rejected.
If RAM times out before recreation, the live runtime remains unchanged.

# Rollback Strategy

Before mutation, the bundle captures container IDs, image IDs, environment,
mounts, ports, commands, healthchecks, networks, restart policy, retained
counts and role flags. Rollback recreates only the two touched services from
their original immutable image IDs and verifies topology parity, health, zero
jobs, retained counts and protected identities.

# Operator Bundle Tests

The six operator scripts parse successfully. Thirty static assertions prove
the exact two-service scope, protected services, 6-GiB and zero-job fail-closed
gates, sequential/offline/no-pull builds, source-bound resume, guarded drain
ordering, no destructive cleanup/app termination, mode-1-only clean-cache
policy, bounded recreation, verification and rollback.

The installed Docker Compose also accepted the generated two-service
`build.network: none` specification through read-only `compose config`; no
build was started.

The live read-only preflight passed integrity, disk, Docker, health, jobs,
models, rollback topology, Compose and retained-count gates. It blocked only on
RAM: measured 2.919 GiB, required 6 GiB.

# Current Runtime Impact

`DeploymentPerformed=false`

`RuntimeMutated=false`

The accepted containers and images are unchanged.

# Retained Impact

`DatabaseMigration=false`

`VolumesChanged=false`

`RetainedStateMutated=false`

No evidence was uploaded, reprocessed, deleted or rewritten. No Activity entry
was created.

# Exact User PowerShell Command

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
powershell.exe -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_nxmmr_p1_correction_activation.ps1 -WaitForRamMinutes 120
```

# What User Must Close

Close Codex/ChatGPT, Chrome or other browsers, VS Code/editors and optional
background applications. The script reports high-memory processes but never
kills them.

# What Must Remain Running

Keep Docker Desktop and the standalone PowerShell terminal running. Do not run
uploads, Ask queries or evidence processing during activation.

# Open P0

None observed.

# Open P1

Source P1: zero for this correction candidate.

Live P1: remains open until a new `P1_CORRECTION_ACTIVATION_VERIFICATION=PASS`
receipt and the failed-cells-only browser acceptance prove the corrected
runtime.

# EXACT NEXT ACTION

Save work, close Codex/ChatGPT/browser/editor processes, leave Docker Desktop
running, open standalone PowerShell, run the single command above, type `YES`
only after its preflight reaches PASS, and wait for either
`P1_CORRECTION_ACTIVATION_VERIFICATION=PASS` or a reported rollback outcome.
Do not reopen apps or interrupt the terminal during builds/drain/recreation.

## Proven Source Markers

```text
PreviousActivationPreserved=true
PreviousActivationReceipt=20260902T025246574Z
TimingAnaphoraSourceFix=PASS
ExplicitCrossFamilyIntentIsolationSourceFix=PASS
SelectedVideoANPRSourceFix=PASS
ANPRNoMatchSourceFix=PASS
BoundedStatusSourceFix=PASS
OCRExactSourceFix=PASS
OCRNoMatchSourceFix=PASS
TranscriptTypedLocatorSourceFix=PASS
CrossFamilyCitationFairnessSourceFix=PASS
SigLIPAskPresentationSourceFix=PASS
SigLIPActivitySourceFix=PASS
FaceIntentRoutingSourceFix=PASS
FaceScopeTransportSourceFix=PASS
FaceCitationSourceFix=PASS
FaceActivitySourceFix=PASS
UrduSTTRegression=PASS
EnglishSTTRegression=PASS
StructuredCDRRegression=PASS
NoHardcodedAnalyticalAnswers=PASS
NoSelfOracle=PASS
NoUnrestrictedSQL=PASS
CrossCaseLeakage=NONE
SourceSeal=PASS
OfflineActivationBundleReady=true
DeploymentPerformed=false
DatabaseMigration=false
VolumesChanged=false
RetainedStateMutated=false
ProductCertificationPerformed=false
NXB2ActivationPerformed=false
```
