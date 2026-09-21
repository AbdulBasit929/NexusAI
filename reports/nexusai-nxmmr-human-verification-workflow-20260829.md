# NX-MMR human-in-the-loop verification workflow

Status: `COMPLETE_LOCKED_HUMAN_GOLD` (2026-08-30).

## Bounded result

The local verification utility is benchmark tooling, not product UI. It binds
only to `127.0.0.1`, serves media through a source-manifest allowlist, verifies
the prepared image/video hashes at startup, and writes only under the ignored
`local-acceptance-models/nxmmr/private-ground-truth/human-verification`
directory. It contains no ANPR, OCR, ASR, LLM, LocalAI, product database,
Activity, retained-service, or external-upload integration.

Run from the repository root:

```powershell
.\scripts\run_nexusai_nxmmr_human_verification.ps1
```

The launcher uses the approved local image directory and `sample.mp4` defaults,
starts a random-port loopback server, and opens the browser. An explicit
`-ImageRoot` or `-Video` override is available, but the helper still verifies
the media against the sealed source manifest.

## Human workload

- Images: exactly 10 deterministic, metadata-diverse development images, then
  the existing 22-image sealed holdout. The other 76 images do not require
  review in this slice.
- Independence: every active image requires an explicit no-output-seen answer.
  Any yes answer is recorded, excluded, and replaced by the next fixed
  hash-ranked development sample. It is never silently claimed independent.
- Video: watch the original 60.010-second source once and mark plate-visible
  start/end events or explicit negative intervals. Rows are written
  automatically; the reviewer does not edit the old 30-row coarse CSV.
- Speech: browser Web Audio uses 100 ms signal-energy windows only. The prepared
  audio decoded as digital silence in the real browser check, so the honest
  fallback is six fixed approximately 10-second audit windows. The reviewer
  marks speech yes/no and, only when heard, enters raw verbatim text, language,
  and unintelligible status. No ASR text is shown. Urdu must remain Urdu script;
  Roman Urdu is not raw oracle truth.

## Development and holdout discipline

Development labels must be complete and explicitly frozen before the holdout
opens. The helper preserves the original split digest
`16b950bbf79780a227239d790a9e1a7d9192555db4958d99278e2110b8206d2d`.
If an exposure replacement is required, the original ID, reason, reviewer,
replacement ID, and timestamp are preserved in a separate exclusion CSV and
the final gold digest reflects the replacement. No candidate result influences
selection or labels.

The holdout is labeled and locked before any comparison. Candidate comparison
is deliberately absent from gold-label mode. Development data may later be
used for integration checks; the frozen holdout is used once after the
pipeline is frozen. Any rerun must be reported with reason and cannot silently
replace the first-run score.

## Automatic and human fields

Automatic fields are sample/event/segment ID, relative source, byte size,
SHA-256, image dimensions, split, privacy, independent-human label source,
model-output-seen disposition, reviewer timestamp, completion state, progress,
exposure replacement receipt, VAD method/parameters, validation errors, and
the final locked gold digest.

Human fields are reviewer identity; per-image exposure declaration, plate
presence/count/exact text/readability and optional difficulty/notes; video
plate interval bounds/count/text/readability, explicit negatives and full-watch
attestation; and per-audio-window speech presence/raw verbatim transcript/
language/unintelligible/notes.

## Validation and certification integrity

Finish fails closed until there are 10 complete frozen development labels, 22
complete holdout labels, independent/no-output-seen declarations for all 32
active images, a full-video attestation plus at least one reviewed interval,
and all six speech audit windows reviewed. Successful Finish atomically locks
the state and writes `GroundTruthValidation=PASS` plus a canonical gold digest.
The workflow completed with `GroundTruthValidation=PASS`, no validation errors,
and gold digest
`88c6827c23fbed467f097843404ed09c947bf7f27432e5cb5fa8427be9f48567`.
That unlocked only the already-authorized private, non-retained benchmark.

This small private gold set can later support explicitly limited
`REAL_WORLD_CERTIFIED` evidence. It cannot by itself establish general
Pakistan-wide performance. `PRODUCT_CERTIFIED` still requires separately
approved live/API/Ask/citation/Activity/error-state/UI and operational
acceptance. The five certification levels remain `REGISTERED`,
`SOURCE_VALIDATED`, `FIXTURE_CERTIFIED`, `REAL_WORLD_CERTIFIED`, and
`PRODUCT_CERTIFIED`.

## Follow-up boundaries

After local scores exist, propose one publisher-labeled public dataset with
license, provenance, label schema, Pakistan/domain relevance, download size,
and exact admission value. Do not download it before approval. Query-template
and operation certification rules remain unchanged. Evidence routing remains
shadow-only. An opt-in source test used an actual private JPG, `sample.mp4`, the
prepared M4A, an actual local PDF and an actual Pakistan CDR CSV. The resulting
families/roles were image → ANPR/OCR, video → ANPR/ASR/bounded frames/OCR,
audio → ASR, document → OCR, and structured → structured profile. All plans
retained `SHADOW_ONLY`; absent model roles remained `MODEL_REQUIRED` rather
than becoming zero findings. The route probe now distinguishes the M4A
`ftypM4A ` audio brand from video MP4.

Later product manual QA should use one guided checklist covering upload/route,
positive/zero/no-match/unavailable/failure states, citation/source navigation,
Ask/Activity reopen, responsive breakpoints, keyboard, theme, Urdu/RTL and
performance. That checklist is a separate `PRODUCT_CERTIFIED` gate.

## Verification evidence

- New workflow unit tests: 4/4 pass.
- Python compile and JavaScript syntax checks: pass.
- Real source binding: 108/108 image paths and hashes plus video size/hash pass.
- Loopback API: 10 development + 22 holdout; `GOLD_LABEL_ONLY`.
- HTTP video range: 206 with exact 32-byte range and correct total size.
- Browser: original image renders; video and audio both report 60.010 seconds;
  layout and independence controls render; signal-energy probe returns the
  explicit digital-silence audit fallback.
- Shadow routing: two focused specs pass, including all five actual local
  families; the complete NX-MMR route-plan focus and Go vet pass.
- Full forensic API regression: 324 specs (322 passed, two skipped), pass in
  72.950 seconds.
- Earlier Pillow-based pack test is not runnable in the current host Python
  because Pillow is absent; the new helper uses only the Python standard
  library and its own tests pass.

No runtime, model, service, database, Activity, evidence, volume, deployment,
NX-B2.1, commit, push or PR mutation occurred.

```text
VerificationToolImplemented=PASS
InitialHumanImageReviewCount=32
DevelopmentReviewCount=10
SealedHoldoutReviewCount=22
HoldoutIndependence=PASS_HUMAN_ATTESTED_MODEL_OUTPUT_UNSEEN
GroundTruthValidation=PASS
NoInferenceDuringLabeling=PASS
NoSelfOracle=PASS
RuntimeMutated=false
RetainedStateMutated=false
ActivityMutated=false
DatabaseMigration=false
DeploymentPerformed=false
NXB2ActivationPerformed=false
```
