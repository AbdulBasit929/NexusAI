# NexusAI Pakistan Urdu ASR challenger acceptance

Date: 2026-08-20; runtime activation accepted 2026-08-21  
Mode: local, bounded, non-retained  
Deployment: accepted

## Decision

`PakistanUrduASRPracticalBaseline=PASS_FOR_M1`  
`WhisperTinyPakistanM1=REJECTED`  
`WhisperSmallPakistanM1=ACCEPTED_CHALLENGER`  
`AudioM1PakistanReady=YES`  
`IdentifierASRMaturity=PENDING_M2`  
`PakistanUrduASRRuntime=PASS`  
`WhisperSmallDeployment=PASS`  
`ModelDeploymentNeeded=NO`  
`RetainedDataMutated=false`

The accepted runtime uses explicit `ur` guidance for known natural Urdu.
Automatic detection mislabeled both natural clips as Hindi and emitted
Devanagari. The model is accepted as a challenger, not deployed or certified
for population-level accuracy.

## 2026-08-21 guarded runtime activation

The source and deployed paths now implement one language-safe contract:

- `ur` and `en` are validated and forwarded to
  `WhisperModel.transcribe(language=...)`;
- absent language remains `None` and preserves automatic detection;
- multipart HTTP language is copied into the LocalAI transcription request
  before gRPC;
- known-language observations retain an effective legacy `language`, record
  `requested_language`, and do not mislabel the forced value as detected;
- unknown-language observations record returned detection separately;
- video embedded-audio composition forwards the parent request language.

The verified local model was copied to the existing model volume as
`/models/faster-whisper-small` and exposed as `faster-whisper-small-ur`.
The installed backend is the official CPU faster-whisper OCI payload pinned by
image digest
`sha256:59c11b6498ad92bec79ac6d4ec90c38dae1ff7b1f219556e95ae792d349af957`,
with its adapter corrected from repository source and its isolated environment
pinned to faster-whisper 1.2.1 and CTranslate2 4.7.1. Startup resolves only the
existing local model directory; no implicit model download is allowed.

LocalAI had to be rebuilt because source inspection proved the HTTP drop was in
its Go adapter. Only LocalAI and the forensic worker were recreated. Rollback
tags `nexusai/localai-forensic:rollback-before-asr-language-20260821` and
`nexusai/forensic-records-worker:rollback-before-asr-language-20260821` preserve
the prior images; `whisper-tiny` and all named volumes remain intact.

Final protected IDs are forensic records API
`338dd6ea58350568b6a30581d5b227a862192dfd7d70fe50d26dfa51c7c13f02`,
PostgreSQL
`f66e05a3b17978c9912aa911177f22e57d69c17de87881a33f23bcba228ad361`,
and NATS
`5c49e70d132ac01b4adf86f27b1735d4d5cf6cc205d848c0881164f1f200eb04`.
The final worker ID is
`e51b3e5a325f7d8546e3f1759f230648aa34ae2d42afc2b9d9d695272ef6ecc9`;
it is healthy with `FORENSIC_ASR_MODEL=faster-whisper-small-ur`.

### Deployed non-retained results

| Sample | Requested / returned | WER / CER | Segments | Worker wall / RTF | Decision |
| --- | --- | ---: | ---: | ---: | --- |
| clear Urdu | `ur` / `ur` | 0.6000 / 0.1852 | 2, 0.00–6.12 s | 7.204 s / 1.1545 | PARTIAL, meaning preserved |
| second Urdu read | `ur` / `ur` | 0.3333 / 0.1446 | 1, 0.00–10.00 s | 4.135 s / 0.3938 | USABLE |
| clear Urdu auto control | absent / `hi` | not an M1 acceptance score | 1, 0.00–6.00 s | 7.190 s / 1.1522 | expected Devanagari control |

The direct cold explicit-Urdu clear request took 20.149 seconds; the subsequent
direct second clip took 3.244 seconds. The warm LocalAI container used about
2.091 GiB and the worker about 67.67 MiB. The composed-video worker smoke passed
in 4.376 seconds with two frames at 0/5 seconds, five unique observations, one
timestamped Urdu embedded-audio segment, parent provenance, manual-review state
and complete temporary cleanup. ANPR was intentionally optional for this lawful
non-plate video.

Before and after retained counts are identical:
`jobs=19, evidence=20, artifacts=0, canonical_records=12932, kb_assets=18`;
active jobs are zero. `OpenP0=0`, `OpenP1=0`,
`PakistanASRLanguageForwardingSource=PASS`,
`WhisperSmallWorkerCompatibilitySource=PASS`,
`PakistanUrduASRRuntime=PASS`, `AudioProcessorRuntime=PASS`,
`AudioRetainedE2E=PENDING`, and `IdentifierASRMaturity=PENDING_M2`.

## Verified challenger

- repository: `Systran/faster-whisper-small`
- revision: `536b0662742c02347bc0e980a01041f333bce120`
- architecture: CTranslate2 conversion of multilingual OpenAI Whisper small,
  244 million parameters
- license: MIT
- Urdu support: published multilingual vocabulary includes `ur`
- model bytes: `486214370` (463.69 MiB), below the authorized 600 MiB limit
- local path: `local-acceptance-models/faster-whisper-small`
- security: local files only, offline inference, `trust_remote_code=false`
- manifest: `local-acceptance-models/CHALLENGER_MANIFEST.json`
- raw result: `local-acceptance-models/benchmark-results/faster-whisper-small.json`
- raw-result SHA-256: `a1b477be13ec01bf6f33b8561ecb66e8a2eb030ceb163f58eb220e48e089f7b7`
- evaluation: `local-acceptance-models/benchmark-results/faster-whisper-small-evaluation.json`

Required model files and SHA-256 values:

| File | Bytes | SHA-256 |
| --- | ---: | --- |
| `model.bin` | 483546902 | `3e305921506d8872816023e4c273e75d2419fb89b24da97b4fe7bce14170d671` |
| `config.json` | 2370 | `b55496ac7940a7ae47d2c01eab40edfd8701feec1229d9cce3b40014383fb828` |
| `tokenizer.json` | 2203239 | `fb7b63191e9bb045082c79fd742a3106a12c99513ab30df4a0d47fa6cb6fd0ab` |
| `vocabulary.txt` | 459861 | `34ce3fe1c5041027b3f8d42912270993f986dbc4bb34cf27f951e34a1e453913` |
| `README.md` | 1998 | `329373481008c7c38654aff8ecdcf0163c211557cc7ba8e2ef6f2f84b4f75ec8` |

`model.bin` matches the repository-linked hash. All files are non-empty and
the model loaded from the local path.

## Benchmark configuration

- host: Windows 10 build 22631, Intel64 Family 6 Model 154, 16 logical CPUs
- runtime: isolated Python 3.10.11 environment, not production dependencies
- faster-whisper 1.2.1; CTranslate2 4.7.1; PyAV 17.1.0
- device/compute: CPU / INT8, eight threads
- decode: beam size 5, `condition_on_previous_text=false`
- natural clips: forced `ur`; separate automatic-detection control
- mixed synthetic fixture: language unspecified/detected
- scoring: Unicode NFKC, casefold, punctuation/symbol-to-space, whitespace
  collapse; whitespace-token WER; space-excluding CER

## Results

| Sample | Tiny WER/CER | Small WER/CER | Tiny usability | Small usability | Tiny RTF | Small first/warm RTF | Winner |
| --- | ---: | ---: | --- | --- | ---: | ---: | --- |
| clear Urdu | 1.0000 / 1.0617 | 0.6000 / 0.1605 | UNUSABLE | PARTIAL; meaning preserved; no major omission/hallucination; timestamps usable | 1.6221 | 1.2612 / 0.8574 | SMALL |
| second-speaker read Urdu | 1.4583 / 1.3012 | 0.2917 / 0.1325 | UNUSABLE | USABLE; meaning preserved; no major omission/hallucination; timestamps usable | 1.2479 | 0.6061 / 0.4809 | SMALL |
| synthetic mixed identifier | 1.0000 / 0.8696 | 0.9231 / 0.9022 | UNUSABLE_FOR_IDENTIFIER | PARTIAL; timestamps usable | 2.9348 | 1.1107 / 1.1251 | SMALL_FOR_PHONE_AND_TIME_ONLY |

### Clear Urdu

- truth: `فلپائنی فوٹوگرافروں کے ساتھ چھےاغوا شدگان کو جلد رہا کر دیا گیا تھا، جِن میں بچّے اور بُوڑھے شامل تھے۔`
- tiny: `Pani photo graphro isa 6th, Aguashudgantho jal raha kar diya giyatath jinn me bach chur buresh haumil te.`
- small: `فیل پائنی فورٹو گرافروں کے ساتھ چھے اگوا شدگان کو جل رہا کر دیا گیا تھا چھن میں بچوں اور بڑے شاہ ملتے`
- segments: 0.00–4.40 s; 4.40–5.96 s
- first/warm: 7.870/5.350 s
- auto control: detected `hi` at 0.84527, WER/CER 1.0500/1.0123

### Second-speaker Urdu read speech

- truth: `مجھے معلوم نہیں آیا آپ نے محسوس کیا ہے یا نہیں، اس ملک میں سنٹرل امریکا سے آئی زیادہ تر اشیاء ڈیوٹی فری ہیں۔`
- tiny: `Maja Malung is not my mother, I am a guest of my mother, I am a guest of this month. I am a Central Ambita. I am a Asia-Data asya, a beauty for you.`
- small: `مجھے معلوم نہیں آیا آپ نے محسوس کیا یا نہیں اس ملک میں سینٹر لمڈیکہ سے آئی زیادہ ترشیا دیوٹی فری ہے`
- segment: 0.00–10.00 s
- first/warm: 6.364/5.049 s
- auto control: detected `hi` at 0.96743, WER/CER 1.0833/1.0723

### Synthetic identifier

- truth: `گاڑی کا نمبر ایم این ایک تین چھ سات ہے۔ Call zero three zero zero one two three four five six seven at ten thirty five.`
- tiny: `Kali Kanam Bere Meneek, Dean Khek Khek, Kall Niel of 3 Niel of 1-2-3-4-5-6-7-10-35.`
- small: `Dariq a number they may make being chicks hothead. Call 03001234567 at 1035.`
- segments: 0.00–2.78 s; 2.78–7.66 s
- first/warm: 8.640/8.752 s
- detected language: `en`, probability 0.71823
- identifier preservation: PARTIAL; phone `03001234567` YES, time `1035` YES,
  plate `MN1367` NO

This fixture is synthetic pipeline evidence and is not evidence of natural
Pakistan speech quality.

## Performance and retained-data boundary

Model load was 2.193 seconds. Peak process RSS was 962875392 bytes (918.27
MiB), a 912355328-byte delta. The three warm RTFs were 0.8574, 0.4809 and
1.1251. This is operationally acceptable for a bounded CPU-first M1, with the
identifier clip slightly slower than real time and M2 optimization still open.

Before and after retained counts were identical:

`jobs=19, evidence=20, artifacts=0, canonical_records=12932, kb_assets=18`

No evidence registration, ingest job, retained artifact, canonical record, KB
asset, migration, service restart, production dependency change, model
replacement, training, stage, commit, push, or retained-pack upload occurred.

## Historical pre-activation compatibility gate — closed 2026-08-21

This section preserves the 2026-08-20 diagnosis. The activation record above
supersedes its deployment-pending language. Runtime tracing later proved that
the multipart HTTP adapter also dropped the form language before gRPC; both
boundaries are now corrected and tested.

The HTTP and gRPC path already transports `language`: the OpenAI endpoint puts
it in `core/backend.TranscriptionRequest`, and `backend/backend.proto` defines
`TranscriptRequest.language`. The current `backend/python/faster-whisper/backend.py`
does not forward `request.language` to `WhisperModel.transcribe`. Because both
natural clips failed automatic language detection, a safe deployment requires
a focused backend correction plus regression coverage before activation.

The smallest honest deployment boundary is therefore LocalAI ASR backend/model
activation plus worker ASR-model configuration. API, UI, PostgreSQL, NATS,
evidence, KB, and named data volumes stay protected.

## Historical deployment proposal — superseded by accepted execution

The following is an operator proposal, not authorization. It assumes the
backend language-forwarding correction has first been reviewed and its focused
test passes. Replace no values silently.

```powershell
$ErrorActionPreference = 'Stop'
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

# Readiness, resources, immutable inputs.
docker info | Out-Null
Get-CimInstance Win32_OperatingSystem | Select-Object FreePhysicalMemory,TotalVisibleMemorySize
Get-FileHash -Algorithm SHA256 '.\local-acceptance-models\faster-whisper-small\model.bin'
if ((Get-FileHash -Algorithm SHA256 '.\local-acceptance-models\faster-whisper-small\model.bin').Hash.ToLowerInvariant() -ne '3e305921506d8872816023e4c273e75d2419fb89b24da97b4fe7bce14170d671') { throw 'Model hash mismatch' }

# Capture rollback/protected identities and require an empty queue.
$Compose = @('--env-file','.env.forensic-runtime.local','-f','docker-compose.forensic-records.yaml','-f','docker-compose.forensic-records.runtime.yaml')
$Before = docker ps --format '{{json .}}'
$WorkerBefore = docker compose @Compose ps -q forensic-records-worker
$ProtectedBefore = @('forensic-postgres','forensic-nats','forensic-records-api') | ForEach-Object { "$_=" + (docker compose @Compose ps -q $_) }
$ActiveJobs = docker compose @Compose exec -T forensic-postgres psql -U localrecall -d localrecall -Atc "SELECT count(*) FROM forensic.records_ingest_jobs WHERE status IN ('queued','running');"
if ($LASTEXITCODE -ne 0 -or [int]$ActiveJobs -ne 0) { throw "Queue preflight failed: $ActiveJobs active" }

# Build/install the reviewed faster-whisper backend correction and copy the
# verified CTranslate2 model into the existing LocalAI model volume. Use the
# repository's pinned image/tag and backend packaging flow; do not pull latest.
# Configure a new LocalAI alias (for example whisper-small-ur) pointing only to
# that verified path. Verify a direct multipart request with language=ur before
# changing the worker.

# Set FORENSIC_ASR_MODEL=whisper-small-ur in the protected runtime env, then:
docker compose @Compose up -d --no-deps --force-recreate forensic-records-worker

# Health, queue and protection checks.
docker compose @Compose ps forensic-records-worker
docker compose @Compose logs --tail 100 forensic-records-worker
$ProtectedAfter = @('forensic-postgres','forensic-nats','forensic-records-api') | ForEach-Object { "$_=" + (docker compose @Compose ps -q $_) }
if ((Compare-Object $ProtectedBefore $ProtectedAfter)) { throw 'Protected container changed' }

# Bounded non-retained Urdu smoke must preserve Urdu script, timestamps, model
# identity, manual-review state and the accepted quality/performance envelope.
# Only after that smoke passes may the approval-gated retained breadth run begin.
'PakistanUrduASRDeployment=PASS ProtectedContainersPreserved=true RetainedDataMutated=false'
```

Rollback: restore the previous environment file, keep `whisper-tiny` and its
model volume untouched, recreate only `forensic-records-worker`, and remove no
model/backend assets. If LocalAI itself had to be recreated for the new backend,
restore its captured prior image/config and verify all pre-existing model IDs.

The proposal intentionally does not invent a ready-to-run backend install/build
command: the current backend drops the required language parameter, so issuing
an activation command before that reviewed source correction would be unsafe.

## Current next action and phase

Exact next action: obtain operator truth/license/ownership review and explicit
approval for the isolated seven-input retained breadth E2E. Do not upload the
retained pack implicitly. Basic Documents M1 is next after retained breadth
closure; identifier robustness remains M2.
