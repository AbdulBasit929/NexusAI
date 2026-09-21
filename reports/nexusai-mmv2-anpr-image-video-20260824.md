# NexusAI MMV-2 ANPR / Image / Video maturity report

## Final P1 source closure addendum

P1-1 and P1-2 are source-complete and live-certification gated. Generic Urdu
ANPR routing preserves arbitrary ASCII plate identifiers, and exactly one
public operation, `video.anpr_grouped_timeline`, executes through the governed
case-scoped query path. It accepts evidence UUID, optional normalized plate and
optional source-second bounds; returns groups, raw/best observation locators,
confidence, processor/revision, review state and citations; and labels results
as grouped observations rather than tracking or identity.

The retained video `50057921-4f1f-4ab8-bab0-47bcdc957822` is completed with 48
media observations and zero group artifacts. Its required public response is
`ANPR processing complete, 0 plate groups detected.` Full forensic API tests,
focused agent routing, Go vet, React tests/build, focused ESLint, anti-hardcode
search and `git diff --check` pass. No deployment or retained mutation occurred.

The activation boundary is forensic API plus LocalAI/UI only. Current running
images lack matching rollback tags, so new tags plus the mandatory 6 GiB RAM
gate require explicit deployment approval. `sample.mp4` path/hash/size pass;
visual crop review and ownership/retention attestation remain pending, and its
model-derived artifact count is deliberately expressed as a formula rather
than a fabricated exact scalar. See
`mmv2-anpr-image-video-20260824/final-closure-predeployment-20260824.json`.

Date: 2026-08-24  
Scope: bounded lawful non-retained benchmarks and source changes  
Retained mutation / deployment / model download / migration: none

# Verified Starting State

Read-only truth was `51 evidence | 51 versions | 64 jobs (60 completed, 4
dead-letter, 0 active) | 22,207 canonical records | 441 artifacts | 47 KB
assets`. The worktree was already materially dirty and unrelated changes were
preserved.

# Services

LocalAI `e04dae4cbd44`, forensic API `a97e545b8356`, worker `0b2418d90cbd`,
PostgreSQL `f66e05a3b179` and NATS `5c49e70d132a` stayed running with the same
IDs. Nothing was restarted, recreated or deployed.

# Roadmap Reconciliation

The living roadmap, ledger, backlog and continuation checkpoint now identify
MMV-2 as a source-complete bounded slice with explicit retained, activation,
operation and browser gates. Historical MMV-1 files were not rewritten.

# Video ANPR Sampling Benchmark

## 5s

Six exact source-time frames, 0–25s; incomplete duration coverage; 14.570s wall,
2.172 CPU seconds, 278,084 KiB peak RSS, 52.30 MiB temporary data; two candidates
(`MW51VSU`, `EF10DZT`); cleanup PASS.

## 2s

Thirty frames, 0–58s; 35.125s wall, 9.582 CPU seconds, 278,320 KiB RSS, 262.13
MiB temporary data; the same two candidates; cleanup PASS.

## 1s

Sixty frames, 0–59s; 41.676s wall, 23.28 CPU seconds, 278,632 KiB RSS, 523.50
MiB temporary data; four candidates: `MW51VSU` 0s, `AK64DMV` 19s, `EF10DZT`
20s, `WG65ZFX` 43s; cleanup PASS.

## Adaptive if tested

Eighteen frames; 45.355s wall, 8.25 CPU seconds, 278,668 KiB RSS, 156.01 MiB;
three candidates and missed `WG65ZFX`; cleanup PASS.

## Selected Strategy

`fixed_1s_full_duration_for_anpr_only`, bounded to 60 frames by default and 120
hard maximum. Longer videos widen the interval. OCR, face and SigLIP retain a
separate 5-second cadence.

## Resource Tradeoff

One-second sampling gained two candidates over two seconds for 6.55 seconds
more wall time, but doubled temporary PNG volume. It is selected only for the
cheap ANPR role. The lawful 10.5-second negative video returned zero plates at
all four cadences.

# Positive Video ANPR

## Non-Retained

PASS as a bounded model-observation control. Source SHA-256 is
`d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee`.

## Retained Status

APPROVAL_GATED; `sample.mp4` was not registered or uploaded.

## Plate

Review-required OCR candidates: `MW51VSU`, `AK64DMV`, `EF10DZT`, `WG65ZFX`.
They are not verified ground truth.

## Timestamp

0s, 19s, 20s and 43s exact best-effort source timestamps. The historical
MMV-1 `LN15ZZC at 25s` came from an `fps`-selected frame whose timestamp was
relabeled by output index; it is superseded as an exact-time citation and
retained only as historical unreviewed OCR output.

## BBox

Respectively `2423,1564,154,54`; `1076,1875,215,64`;
`2564,1872,193,59`; `2429,1982,197,64`.

## Confidence

Detector/OCR: `0.763425/0.993895`, `0.755904/0.999902`,
`0.780218/0.999910`, `0.750953/0.999869`.

## Citation

Every result carries video hash, version placeholder, source file, exact source
timestamp, estimated frame number with declared semantics, source-frame hash,
bbox/crop, models, processor revision, observation ID and review state.

## UI

Source now emits `forensics.video-anpr-plate-group/v1`; Analyst Data prefers
groups with count/first/last/best while raw artifacts remain available. This is
explicitly grouped OCR output, not tracking.

# Image ANPR Real-World Matrix

Thirteen honest fixtures: 7 PASS, 2 PARTIAL, 4 FAIL. Clear `LEC4800`, clear
`LEF1981`, synthetic `MN1367`, resize, bright and both no-plate negatives pass;
JPEG45 and low-light misread; angle, blur, low-contrast and obstruction fail.

# Video ANPR Operations

Plate grouping/timeline artifacts are source-backed. The complete target public
set (`find_plate`, evidence plates, time filtering, source summary) is not yet
independently executable/certified through Ask, so verdict is OPEN rather than
falsely PASS.

# ANPR Query Certification

Existing structured `anpr.sightings` remains certified for retained structured
rows. Positive-video natural-language, Roman Urdu, Urdu, follow-up and History
certification is approval/deployment gated; `ZZZ9999` was not hardcoded.

# Image Operations

`image.metadata`, OCR, scoped comparison, SHA exact duplicate, dHash near
duplicate and explicitly scoped SigLIP ranking have real backing APIs/source.
Full operation acceptance through Ask/History remains OPEN.

# Image Query Certification

Artifact-derived UI actions use current observations. The full multilingual
template matrix is not claimed complete.

# Video Operations

Timeline, ANPR, OCR, face and transcript observations compose from real sampled
frames. Public `transcript_at_time` and source-summary Ask contracts remain
OPEN.

# Video Query Certification

Source semantics and negative results pass unit/benchmark checks; live Ask and
History variants remain deployment gated.

# Exact Duplicate

PASS: identical bytes under a different filename match by SHA-256; different
bytes do not.

# Near Duplicate

PASS_OR_TRUTHFULLY_LIMITED: six transforms measured dHash distance 1–7; six
unrelated images 19–35. Threshold 10 remains unchanged because the pack is too
small for universal calibration.

# Image Comparison

PASS in focused Go tests. Output keeps SHA equality, dimensions, dHash, OCR/
ANPR overlap, face counts, compatible SigLIP cosine, citations and limitations
separate; it invents no match percentage.

# SigLIP

PASS_ACCEPTED_LIMITED on 16 images. Same-scene resize 0.9741, recompression
0.9158, low-light 0.9202, same-person blur 0.9358, different person 0.7852,
portrait/street 0.4630, vehicle-crop/street 0.4195. These are ranking signals,
not identity, duplicate or fact claims. Mean per-image latency was 1,277.5 ms.

# Face-in-Video Reuse

The split-cadence composition preserves timestamp/frame/bbox/crop context for
the existing face processor and makes no tracking/identity claim. A live
positive retained-video proof was not run.

# OCR-in-Video

Actual sampled-frame locators are forwarded to non-empty OCR observations;
Urdu quality work remains MMV-3.

# Data UI

Source supports preview/player, bbox overlays, crops, typed states, plate groups,
timeline, frame observations, OCR, faces, transcripts, artifacts and citations.

# Ask

Suggestions remain derived from current artifacts. Complete media operation
resolution is OPEN and is not represented as certified.

# Citations

Source contracts PASS; retained/live query citation certification remains gated.

# History

Existing retained History behavior is preserved. New MMV-2 media-operation
reopen proof was not run because no deployment/retained mutation was authorized.

# Audio Presentation Activation

SOURCE_READY, DEPLOYMENT_GATED. It was not bundled into a worker/API deployment.

# Browser Acceptance

## 390

Not run live: deployment gated.

## 820

Not run live: deployment gated.

## 1024

Not run live: deployment gated.

## 1440

Not run live: deployment gated.

# Performance Matrix

Video figures are in the sampling sections; image ANPR latency is roughly
0.03–0.11s/fixture; dHash pack completed in 0.82s; SigLIP mean was 1.278s/image.

# Operation Acceptance Matrix

Existing image compare/similarity source tests PASS; new grouped video
presentation PASS; full requested media operation set OPEN.

# Query Template Matrix

Structured ANPR templates remain accepted; new media English/Roman-Urdu/Urdu,
missing-parameter, follow-up and negative variants remain OPEN.

# Security

No unrestricted SQL, tracking, identity, inferred plate corrections, new model,
database change, retained mutation or deployment. Scope and review boundaries
remain explicit.

# Retained Counts

Before and after: `51 / 51 / 64 (60 completed, 4 dead-letter, 0 active) /
22,207 / 441 / 47 KB`.

# P0

0.

# P1

0 open foundational source defects; activation, retained proof, media-operation
certification and browser acceptance are explicit gates.

# P2

Pakistan ANPR failures and broader calibration/corpus breadth remain.

# P3

Tracking, identity, diarization and generalized VLM remain out of scope.

# ModelDownloadNeeded

NO.

# DatabaseMigrationNeeded

NO.

# DeploymentNeeded

YES, separately authorized narrow worker/UI/API activation.

# RetainedMutationNeeded

YES only for optional retained positive-video proof; not authorized.

# Exact Approval Needed

Approve only if desired: narrow deployment of the MMV-2 sampler/group/UI plus
the already-prepared audio UI; separately approve one normal upload of the exact
`sample.mp4` hash to the default multimodal acceptance collection/case, with no
reprocess, migration, cleanup or model download.

# Files Changed

See the MMV-2 report directory, benchmark/smoke scripts, media pipeline/tests,
Analyst media presentation/tests, and the four living roadmap/checkpoint files.
No files were staged or committed.

# Team-Lead Demo Status

SOURCE DEMO READY; live/retained demonstration is gated.

# MMV-2 Verdict

SOURCE_SLICE_COMPLETE_ACCEPTANCE_GATED. Sampling, non-retained positive/negative
video, image robustness, grouping, exact/near comparison and SigLIP evidence
are complete. The full exit contract is not closed because public media
operations/templates, live History/browser proof, deployment and retained
positive proof are not authorized or certified.

The subsequently authorized narrow activation attempted its guarded preflight
and stopped before building because the 6 GiB free-RAM requirement reached only
2.56 GiB. Rollback tags were created, no image/container was replaced, the exact
original runtime was restored healthy, and retained counts remained unchanged.
Live operation/query/History/browser claims therefore remain unexecuted rather
than being inferred from the old deployment.

## 2026-08-24 Narrow Activation and Live Certification

The operator subsequently completed the guarded narrow activation after the
mandatory RAM gate passed. The activation transcript is
`reports/mmv2-anpr-image-video-20260824/manual-narrow-activation-20260824-142418.log`
(SHA-256 `c0ce43a437a430d6162769e86db6ee2f833dea578ce4d543b0b443bbbf5fec03`).
Only LocalAI and the forensic worker were recreated. PostgreSQL, NATS and the
forensic API retained their exact container IDs. All required health endpoints
return HTTP 200 and the worker has zero restarts.

Live current-data certification passes case-scoped face/image similarity,
image comparison, and ranged image/audio/video source delivery. Retained ANPR
plate `MN1367` routes through `anpr.sightings` in English, natural English and
Roman Urdu with one exact row and one source citation. A typed absent plate
returns bounded `no_results`. The positive citation binds evidence
`a0ff7b18-...`, version `3dc6b0f6-...`, row number/hash/timestamp and source
filename. Urdu-script ANPR phrasing still returns `needs_input`; this is an open
foundational P1 rather than a claimed pass.

The retained video exposes 44 artifacts, including 40 source-timestamped OCR
observations at its retained 0/5-second sampling points, but zero retained ANPR
groups. Its detail and playback operations pass. The grouped video ANPR
timeline remains absent as an independently executable public operation, and
no retained reprocessing or `sample.mp4` upload was performed. This is the
second open foundational P1 for the requested public operation exit contract.

History save/reopen passes for analysis
`95a61f9d-a03b-4d96-8802-16f8fe987d6e`, including its cited source in the
live Analyst Portal. Responsive acceptance passes at 390, 820, 1024 and 1440
pixels with no horizontal overflow; mobile navigation is present at 390 and
desktop navigation at the other widths. Audio presentation passes with the
waveform row icon, visible native controls, a loaded 6.24-second WAV, successful
playback, and two timestamped Urdu plus two Roman-Urdu derivative cards.

Final retained counts remain exactly `51|51|64|0|22207|441|47`. No evidence,
version, job, canonical record, artifact or KB count changed; no model changed
and no database migration occurred. The detailed evidence is recorded in
`reports/mmv2-anpr-image-video-20260824/post-activation-certification-20260824.json`.

Final markers are `MMV2Activation=PASS`, `PublicMediaOperations=PARTIAL`,
`MediaQueryCertification=PARTIAL`, `CitationCertification=PASS`,
`HistoryCertification=PASS`, `BrowserAcceptance=PASS`,
`AudioPresentation=PASS`, `OpenP0=0`, and `OpenFoundationalP1=2`.

# Exact Next Phase

Remain in MMV-2: obtain narrow activation approval, deploy without retained
reprocessing, certify current-data media operations/queries/citations/History,
run four-viewport browser acceptance, then request retained-video authorization
only if the non-retained evidence is insufficient. Do not start MMV-3.

## 2026-08-25 Final Enterprise Presentation Source Closure

Both remaining P1s are fixed and source-tested without deployment. The shared
enterprise boundary emits typed result/processing state, authoritative row
count and operation identity. Positive rows override stale zero metadata;
completed video processing with zero groups is `complete_zero_results`; a
valid target miss is `no_match_for_filter`. Remaining processor, failure,
availability, authorization and invalid-request states stay separate.

The public case adapter retains grouped-video tool identity, and the agent
presentation/Analyst Portal History preserve the state. Focused History
acceptance passes completed-zero at 390/820/1024/1440 and no-match reopen in
Chromium. Existing positive citations remain unchanged.

Full forensic API tests, focused agent tests, Go vet, scoped UI lint, React
production build and focused Playwright pass. Full agent tests are 105 pass/13
Windows-testcontainer setup failures; full UI lint retains six unrelated
pre-existing errors. No runtime mutation occurred; counts remain
`51|51|64|0|22207|441|47`.

Source-open P1=0; live-open P1=2 until narrow activation/certification. Rebuild
only forensic API plus LocalAI/UI. Worker, PostgreSQL, NATS, models, volumes,
schema and retained data are protected. The 6 GiB RAM gate remains. Do not
begin MMV-3.
