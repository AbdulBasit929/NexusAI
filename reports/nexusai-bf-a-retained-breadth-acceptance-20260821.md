# NexusAI BF-A retained breadth acceptance

Date: 2026-08-21  
Finalized: 2026-08-23  
Scope: `default/nexusai-breadth-acceptance-20260820`  
Actor: `nexusai-breadth-acceptance-operator`  
Status: `RECOVERY_PASS_BREADTH_ACCEPTED_WITH_LIMITATIONS`

## Authorization and activation

The operator approved the exact seven-input retained run and an API-only ASR
language-admission deployment. The prior forensic API image is retained as
`nexusai/forensic-records-api:rollback-before-asr-language-admission-20260821`.
Only `nexusai-forensic-records-api-1` was recreated; LocalAI, worker,
PostgreSQL and NATS IDs were preserved. The deployed invalid-language probe
returned HTTP 400 before retention. Explicit `UR` normalized to durable `ur`.

## Registered evidence

| Input | Evidence ID | Version ID | Job ID | Frozen state | ASR language |
| --- | --- | --- | --- | --- | --- |
| `test_plate.jpg` | `343b1e62-9f24-488c-a9a0-8d79ff91ee1c` | `c5398b10-639b-4eed-bdc3-aca6a4b55a3a` | `923cba92-9d2d-4ae5-9c9f-f225ccb6d063` | `dead_letter` after 5 attempts | — |
| `wikimedia-road-cars-islamabad.jpg` | `22c74bf9-0fc1-479d-816b-311106ea707b` | `6eb10eed-a8a5-439e-9b6f-96180f684ce8` | `718b7f82-2367-41bc-8ada-5f8e5d0f7c2f` | `dead_letter` after 5 attempts | — |
| `clear-urdu.wav` | `fb025351-f86b-405f-bbf0-1b5287d7bf8b` | `d8a5e346-4f1a-42a8-b2e3-a881822a4225` | `b74b8502-1741-4e66-bde9-d0d5a520da0e` | `failed` after 4 attempts | `ur` |
| `conversational-urdu.wav` | `8c49950b-a91d-49cb-813b-3ff639aa9462` | `691e33c0-f03a-401a-85df-7c02a670b579` | `a93943d4-5f42-4530-b20e-510cb9e22e75` | `queued` | `ur` |
| `urdu-english-identifier.wav` | `88a9962d-24a0-4e3a-86be-c2a6db6e244f` | `e827d3e4-2d18-4782-b963-249356fa35af` | `c7f148c4-47ae-42ba-8f28-a87e22f0422f` | `queued` | automatic |
| `pakistan-short-video.mp4` | `0dcb49a7-bd47-4b63-b4a2-7d67e2caffb0` | `5608c3ca-48f8-4739-80de-600c882d25dd` | `cb1bb9a2-7abc-46a1-9ee3-748632caafb2` | `queued` | `ur` |
| `HP_EliteDesk_LocalAI_Remote_Setup_Guide.docx` | `2291d02e-e383-41d1-99dd-87d698a38210` | `72c2440f-59ec-495e-b32a-f45395851a01` | none | `registered`, `document_extraction_pending` | — |

The approved immutable ANPR reprocess job is
`88df3883-6c4e-44a2-ac54-704e10d3d26f` (generation 1, queued).

## P1 and safe freeze

The first derived-artifact write failed with `permission denied for table
derived_artifacts`. The runtime grant source omitted INSERT on that table even
though tenant RLS already governs it. The worker was stopped without recreation
or deletion to prevent additional retries. No derived artifacts or canonical
records were created.

Source corrections now:

1. add only `GRANT INSERT ON forensic.derived_artifacts TO forensic_runtime`;
2. copy only durable `asr_language` into admitted media processor metadata,
   leaving absent language automatic and excluding internal storage metadata.

Focused Go and Python regressions pass. The originally required grant and
worker activation were subsequently approved; their execution and the second
freeze are recorded below.

## Approved recovery and second safe freeze

The one-table INSERT grant was approved and applied. A same-tenant artifact
insert succeeded, a cross-tenant insert was denied by RLS, and the transaction
rolled back with zero probe rows. Only the worker was rebuilt/recreated, as
container `6d0bc082a8024fce806a67130adf1eb568b737c1e3d9322dd57f9b1e17a0e378`.

Processing then reached `mark_media_job_completed` and exposed a second P1:
PostgreSQL could not determine the type of parameter `$3` inside
`jsonb_build_object`. The artifact transaction therefore rolled back. Source
now casts readiness, processor ID and processor revision parameters explicitly
to `text`; focused tests pass. The worker was stopped again before later jobs
ran. This second worker deployment and the immutable reprocessing of both
dead-lettered image jobs require renewed approval.

## Accounting

Before: `jobs=19,evidence=20,artifacts=0,canonical_records=12932,kb_assets=18`  
Frozen: `jobs=26,evidence=27,artifacts=0,canonical_records=12932,kb_assets=19`

Scope: seven evidence items, seven versions, six media jobs, zero artifacts,
zero canonical records, one document KB asset. No evidence outside the approved
scope was modified. Retained inputs must not be deleted without separate
cleanup approval.

## Historical resumption gate (completed)

The renewed explicit approval authorized the worker rebuild/recreation with the
tested parameter casts, one immutable Islamabad-image reprocess job, reuse of
the existing queued plate-image generation, resumption of failed/queued jobs,
and Data/KB/Ask/citation/History reconciliation. The result follows below; no
source was re-uploaded or deleted.

`OpenP0=0`  
`OpenP1=0`  
`RetainedDataMutated=true_authorized_scope_only`  
`DatabaseStateChangeNeeded=NO_GRANT_ACCEPTED`  
`WorkerDeploymentNeeded=NO_COMPLETED`

## Approved media-completion recovery result

The renewed operator approval was executed exactly within the worker-only
boundary. The prior worker image is retained as
`nexusai/forensic-records-worker:rollback-before-bfa-media-completion-20260821`
at digest
`sha256:99f2858a0a78dec531eadd0373d57c67c302373fad6e156331b4be9527882808`.
Only `nexusai-forensic-records-worker-1` was rebuilt and recreated. Its current
container is `d79aa29fb1b8`, current image digest is
`sha256:a4a4c228ccc6942bd37b415a27245410377a6aed78ecc19e3a9a6e16c2094307`,
and it is healthy.

Protected service identities remained unchanged:

| Service | Container ID | State |
| --- | --- | --- |
| forensic API | `cb3d0e79d146` | running |
| LocalAI | `ba79e39824a5` | healthy |
| PostgreSQL | `f66e05a3b179` | healthy |
| NATS | `5c49e70d132a` | healthy |

No further grant, migration, model download, volume change, evidence upload,
deletion or cleanup occurred. Focused worker tests passed `2 passed, 18
deselected`; Python compilation and `git diff --check` also passed.

## Immutable job recovery

Exactly one new reprocess job was created for the dead-lettered Islamabad image:

- evidence: `22c74bf9-0fc1-479d-816b-311106ea707b`
- original job: `718b7f82-2367-41bc-8ada-5f8e5d0f7c2f`
- reprocess job: `831c9b62-4872-4302-a8e5-c2cf7f27042b`
- generation: `1`
- idempotency key: `bf-a-media-completion:islamabad-image:20260821-v1`
- final state: `completed`

The already-queued `test_plate.jpg` generation-1 job
`88df3883-6c4e-44a2-ac54-704e10d3d26f` completed. The clear-Urdu failed job
and the remaining queued audio/video jobs resumed in place and completed. No
source evidence was re-uploaded. Final scoped job accounting is six completed
latest media generations plus the two preserved historical dead-letter rows;
there are no failed or queued jobs.

## Retained result accounting

Final global counts are
`jobs=27|evidence=27|artifacts=15|canonical_records=12932|kb_assets=25`.
Final isolated-scope counts are
`jobs=8|evidence=7|artifacts=15|canonical_records=0|kb_assets=7`.

Artifact accounting is:

| Artifact contract | Count |
| --- | ---: |
| `forensics.audio-observation/v1` | 5 |
| `forensics.audio-timestamp-segment/v1` | 5 |
| `forensics.image-observation/v1` | 4 |
| `forensics.video-observation/v1` | 1 |

Both known-natural Urdu audio jobs and the Urdu video retained explicit
`asr_language=ur`. The synthetic identifier input correctly remained automatic.
The identifier transcript preserved phone `03001234567` and time `1035`, but
did not preserve plate `MN1367`; `IdentifierSpeechLimitation=PENDING_M2` remains
truthful.

All seven evidence items have retained KB mirrors. The six media evidence items
are completed. The DOCX is retained as `registered` with
`document_extraction_pending`, which is the truthful current document route.
The exact evidence/version/hash manifest is in
`reports/nexusai-bf-a-retained-evidence-checksum-manifest-20260823.md`.

## Data, Ask, citation, History and browser acceptance

The Data surface exposes all seven retained sources. The deterministic hybrid
query returned 15 scoped rows with a complete citation set spanning the seven
evidence sources and eight job generations. The retained Ask request completed
and is present in History as analysis
`2e3adfea-2f61-4d94-b021-de7a13886d05`. The agent-history presentation itself
returned an empty citation array, while the underlying hybrid response had a
complete citation contract; this is recorded as a presentation limitation and
must not be represented as end-to-end agent citation closure.

Browser inspection at 390, 820, 1024 and 1440 pixels found no console errors.
Data, Ask and History had no page-level horizontal overflow. Home has a
reproducible horizontal-overflow defect at 820, 1024 and 1440 pixels because
long retained-analysis cards expand to approximately 1,337 pixels. At 390
pixels the page does not overflow. No UI source or deployment was changed under
this worker-only authorization.

## Final verdict

`MediaCompletionRecovery=PASS`  
`RetainedProcessing=PASS`  
`DataKBHistory=PASS`  
`HybridCitationContract=PASS`  
`AgentHistoryCitationPresentation=LIMITED_EMPTY_CITATION_ARRAY`  
`BrowserAcceptance=PASS_WITH_HOME_OVERFLOW_P2`  
`ANPRRetainedExtraction=LIMITED_ROLE_NOT_CONFIGURED`  
`IdentifierSpeechLimitation=PENDING_M2`  
`BreadthVerdict=ACCEPTED_WITH_RECORDED_LIMITATIONS`  
`OpenP0=0`  
`OpenP1=0`  
`OpenP2=2`  
`RetainedDataMutated=true_authorized_scope_only`  
`WorkerDeploymentNeeded=NO`  
`DatabaseStateChangeNeeded=NO`  
`ModelDownloadNeeded=NO`  
`CleanupAuthorized=NO`

The retained image jobs prove immutable orchestration, persistence and
technical image observation, but they do not prove retained plate extraction:
both completed with the truthful limitation `no approved local vision or ANPR
role is configured`. The breadth verdict therefore accepts the bounded retained
media/data workflow with explicit limitations; it does not certify ANPR
accuracy, general vision, document extraction, agent-history citation display,
or exact spoken-plate recognition.
