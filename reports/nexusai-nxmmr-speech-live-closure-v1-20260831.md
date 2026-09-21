# NX-MMR speech data acquisition and live closure V1 — 2026-08-31

Overall **PARTIAL / LIVE CLOSURE BLOCKED**. Natural-English evidence acquisition
and installed-ASR evaluation are complete. A controlled product upload exposed
disabled worker ASR and incorrect readiness copy; fresh Ask exposed scope,
routing and capability gaps. No model tuning, certification or deployment.
Machine-readable metrics and every English audio/transcript hash are in the
[sanitized receipt](nexusai-nxmmr-speech-live-closure-v1-20260831.json).

## Starting State

Continues NX-MMR-NEXT-MULTIMODAL-BREADTH-V1, not a new phase. Owner-observed
ANPR, Video V3 and English/Urdu/mixed OCR remain frozen functional baselines.
Five LocalAI aliases; installed Faster-Whisper small and YuNet/SFace; no TTS.
Initial demo: 32 sources, 31 completed, one failed. Previous timing/parent-ID
and pagination fixes were source-only. This directive authorized the small
FLEURS acquisition and representative upload/query checks, not deployment.

## FLEURS Acquisition Decision

Exactly ten English clips; no additional Urdu. First ten fixed validation rows,
not cherry-picked by inference quality. Official test-split row endpoints failed
their server-side 300,000,000-byte parquet scan ceiling before selection. The
official validation row endpoint worked; no parquet archive was downloaded.

## Dataset Revision / License

[Google FLEURS](https://huggingface.co/datasets/google/fleurs), revision
`70bb2e84b976b7e960aa89f1c648e09c59f894dd`, official card CC-BY-4.0.
Private attribution receipt names Google FLEURS, its paper, revision, license
link, and internal validation purpose. No broader redistribution/legal claim.
Publisher `raw_transcription` is the sole accuracy oracle, never model output.

## English Sample Manifest

`en_us`, validation rows 0–9; IDs respectively
1548, 1620, 1510, 1578, 1652, 1531, 1528, 1595, 1559, 1633.
Durations: 6.54, 16.38, 7.92, 4.14, 4.92, 4.44, 6.84, 10.02, 9.32, 9.48 seconds.
Total 80 seconds. Publisher WAV assets are unmodified mono 16-kHz float WAV.
Private filenames: `english/fleurs-en_us-validation-row-00.wav` through `09`.
Raw transcripts, official card/metadata/row receipts and results remain under
Git-ignored `local-acceptance-models/nxmmr/private-benchmarks/speech-breadth-v1/fleurs`.

## Urdu Sample Manifest

Separate existing two-clip `ur_pk` cohort, not merged with a new Urdu sample:
test row 1 (`clear-urdu.wav`, 6.24 seconds) and validation row 4
(`conversational-urdu.wav`, 10.50 seconds). Despite the filename, this is read
speech, not spontaneous-conversation validation. Reused existing publisher
ground-truth sidecars and immutable acquisition provenance; no new Urdu download.
Existing PCM16 conversions remain distinguished from publisher float originals.

## Download Size / Sample Hashes

Audio: **5,120,580 bytes**. Successful frozen card/API/row metadata receipts:
716,257 bytes. Accounted pack payload: **5,836,837 bytes**. Full dataset: false.
Retry/research metadata responses and protocol overhead were not network-metered;
the total all-network wire-byte figure is unavailable, not this pack subtotal.
All ten full audio SHA-256 and transcript SHA-256 values are in the sanitized JSON.
Existing Urdu audio hashes:

- clear: `c499e06e26b8cbf37386b88bba26f37045439a775e964bdad8634376435a056e`
- second: `f016076b153c2ed6be21fa5f7a4bbc22b3e391d40080b9fa3de3b8a2dd517550`

## Selection Pre-Registration

Selection digest:
`629e08abcb4c4eefcc280cb1d8a2bce9e7c8bafa40b99198ad3c7aa9e0bf778e`.
Twelve-clip evaluation preregistration digest:
`1c4683e3d36f3575d55165a9498253eec8c4c231752cfe499aaa14e167cf4b04`.
Frozen before inference; no substitutions, favorable reselection, tuning or
post-hoc accuracy threshold. Hashes, duration bounds and license rechecked.

## English ASR Results

### WER / CER

WER **0.1079545455** (19 edits / 176 words); CER **0.0322939866**
(29 edits / 898 non-space characters). Normalized exact clip rate 4/10.
NFKC, casefold, punctuation/symbols to spaces, collapsed whitespace; CER omits
spaces. Full-table independent edit-distance recomputation matches every clip
and both aggregate denominators. This is not an estimate of population accuracy.

### Failures / Timing

0/10 failures; 10 nonempty transcripts; 12 backend-reported segments across all
ten clips. Every segment had valid reported time. Alignment accuracy and word
timing were not independently labeled. Current source adapter was used against
the installed endpoint; this does not prove that adapter is deployed in worker.

### Resources / Error Categories

29.603 seconds total request wall time; 2.434–3.675 seconds per clip, warm backend.
127.18 process CPU-seconds; sampled peak RSS 1,145,840 KiB (~1.093 GiB).
Read-only 50-ms `/proc` sampling; process CPU can exceed one core. Lifetime high
water is separately recorded, not confused with per-request sampled peak.
Word alignment review: ten replacement blocks, one insertion block, one deletion
block (blocks are not edit counts). No digit-bearing publisher reference clips;
identifier/number fidelity is therefore not qualified. No accent/noise/telephony,
name-specific or code-switch benchmark and no model adjustment.

## Urdu ASR Results

### WER / CER

WER **0.4545454545** (20/44); CER **0.1646341463** (27/164).
Normalized exact clip rate 0/2. The substantially different word and character
rates warrant script/tokenization review, not an assumption that all errors are
only spacing. Raw Urdu remains model-produced, review-required transcription.

### Failures / Timing

0/2 failures; two nonempty transcripts; three segments, all with backend times.
The two reused clips are a small historical read-speech cohort, not new balanced
Urdu validation. Raw source/derivative lineage was checked separately in retention.

### Resources / Error Categories

7.989 seconds request wall time; 3.937–4.053 seconds per clip; 34.02 process
CPU-seconds; sampled peak RSS 1,146,936 KiB (~1.094 GiB).
Ten word replacement blocks and one deletion block. No digit-bearing references;
number, named-entity and code-switch fidelity unqualified. No tuning.

## English vs Urdu Comparison

| Cohort | Clips / seconds | WER | CER | Failures | Wall / CPU seconds | Peak RSS KiB | Demo / certification limit |
|---|---|---|---|---|---|---|---|
| New English validation | 10 / 80 | 10.80% | 3.23% | 0 | 29.60 / 127.18 | 1,145,840 | Functional live backend; product path blocked; tiny read-speech sample |
| Existing Urdu | 2 / 16.74 | 45.45% | 16.46% | 0 | 7.99 / 34.02 | 1,146,936 | Functional live backend with substantial text errors; product Ask blocked |

No language ranking: different sample sizes and clips. No cold-load resource claim.

## STT Demo Readiness

ASR backend: FUNCTIONAL_LIVE_WITH_LIMITATION for these named cohorts. Complete
Investigation Workspace STT demo: **BLOCKED_LIVE_INTEGRATION** for both languages,
not LIVE_DEMO_READY. Dataset acquisition is no longer the English blocker.

## English Live UI Acceptance

One fresh normal Add Data item, no batch flood or duplicate-renaming:
`22a595a9-bd51-4da0-8cde-dd892b55842e`, source hash
`048169439c0328b758349af74aeb54a82a7a476e309709eb791dced06db25d48`.
Version `bdec5ca3-7e92-4838-b321-8bd5db1093d9`.
Registration, audio classification, queued → processing → completed custody
chain and one technical observation PASS. **Zero transcript segments**.
Worker `FORENSIC_ASR_ENABLED=false`; retained warning explicitly says ASR not
configured. The UI incorrectly inferred transcript readiness from accepted_rows=1.
Source now uses actual transcript artifacts in detail and neutral availability
copy on list cards; unconditional “transcript available” is removed. Not deployed.
Native playback reached 6.54 seconds with ended=true and no media error.

## Urdu Live UI Acceptance / Roman Urdu

Reused existing `2097c6df-c0ad-4886-b0e7-ea4f4856a220`; no extra Urdu upload
while worker ASR is disabled. Data showed two raw Urdu and two derived Roman
segments. Raw nodes use dir=auto and resolve RTL; primary raw text is preserved.
Both derivatives match exact parent observation IDs, raw text and source times
(0–4.42 and 4.42–6.12). Native playback ended at 6.24 seconds without error.
Old absence of timing-status metadata means new unknown-time handling is not
live-tested. Citation-to-time seek is unproven; displayed “Source reference”
is not an executed seek. Duration display fallback is corrected in source only.

## Data / Ask / Citation / Activity

Data/audio playback PASS for these sources; STT readiness copy FAIL live.
Five new Ask attempts were retained, all terminal; none passed requested STT
semantics. Exact questions and answers remain only in private history receipts.

| Attempt | Actual outcome |
|---|---|
| English “what was said”, explicit fresh evidence UUID | Wrong generic evidence route; three unrelated document citations |
| Urdu-source generated transcript phrase action | Wrong generic evidence route; unrelated document citations |
| Urdu-script actual stored term | Transcript route recognized but capability unavailable; no operation |
| Roman Urdu absent exact phrase | Capability unavailable, not an evaluated NO MATCH |
| Current-audio 2–5-second question | Clarification after ~60 seconds; no SQL/retrieval/synthesis executed |

Activity recorded five analyses and fresh English result reopen PASS. A completed
history job does not imply a successful answer. No “transcribed audio” activity
claim for the fresh file; it only underwent technical inspection. No fresh valid
STT answer citation or citation/time-seek PASS. No full responsive matrix.

## Query Routing / No-Match

Open P1, not closed by good WER. Source trace: AnalystAsk labels evidence scope,
but AgentChat.handleSend and agentsApi.chat transmit only case/collection/context,
not portalScope.evidenceId; the chat handler has no explicit evidence field.
Direct argument construction only extracts evidence IDs for grouped video.
“Find the transcript phrase” is absent from the current transcript route aliases.
The derived-text query scores tokens rather than enforcing an exact normalized
phrase, does not apply start/end seconds, and lacks a current-version join.
Do not broaden capabilities or enable NX-B2 to hide these failures. Next bounded
source slice must wire typed scope end to end, separate transcript retrieval,
exact phrase and time-overlap semantics, fail closed for absent/unavailable
transcripts, validate current-version citations, then test LLM routing/presentation.
No new production SQL, route shortcut or hardcoded FLEURS answer was added here.

## Current LocalAI Audio Reconciliation

Fresh official [v4.9.0 release](https://github.com/mudler/LocalAI/releases/tag/v4.9.0)
is latest on the checked endpoint, published 2026-08-20; commit
`f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e`. No fork merge/update.

| Upstream capability | Local fork/runtime | Reuse / gap / action |
|---|---|---|
| `/v1/audio/transcriptions`, faster-whisper, language/verbose timing options | Installed small alias works; worker disabled | Reuse model; separately activate bounded worker integration |
| `/tts`, `/v1/audio/speech`, CPU Piper | Source backend/gallery exists; no runtime voice/backend | Finish exact admission, then request download separately |
| Model/backend gallery management | Existing customized integration | Reuse; avoid whole-fork replacement |
| v4.9.0 deny-by-default route authentication | Local source retains protected-prefix isAPIPath; anonymous loopback inference works | Focused security port/backport review before non-local demo; no bypass claim tested |
| New TTS engines including Qwen3-TTS support | Not installed or qualified here | Out of this one-candidate admission scope |

Official [STT documentation](https://localai.io/docs/features/audio-to-text/) and
[TTS documentation](https://localai.io/docs/features/text-to-audio/) establish
protocol/backend support, not evidence truth or successful local activation.
Ports 8080/8091 remain published beyond loopback; remote reachability not tested.

## TTS Acquisition Pack

### English

Candidate: `rhasspy/piper-voices`, `en_US-ljspeech-medium`, revision
`217ddc79818708b078d0d14a8fae9608b9d77141`. Repository card MIT; voice card
attributes public-domain LJSpeech, single English female voice, 22.05 kHz.
No cloning/reference audio is proposed.

- ONNX: 63,531,379 bytes, SHA-256 `6f52a751e2349abe7a76735eb09dc1875298c77ea2342ffd2fef79ff81b87f22`.
- JSON: 4,972 bytes; MODEL_CARD: 517 bytes. Required voice subtotal 63,536,868.
- LocalAI v4.9.0 Piper linux/amd64 manifest:
  `sha256:373aa02b18b8deecff7b8962c12fedbe3eb1602e7e9655ef918d3d793fd94ef3`.
- OCI layer: 33,737,429 bytes,
  `sha256:61ac738ab55f574d2f942853a66196fe509bd0b7cac652ccb163ade6f63535ff`.
  Config 1,181 bytes, `sha256:1082ffbb45a61f769ceaa1db0f088404d555b7902114617ee76d9db5f330f4e6`;
  platform manifest 482 bytes. Voice + layer subtotal 97,274,297 bytes; with
  config and platform manifest 97,275,960, excluding notice/index overhead.
- go-piper `e10ca041a885d4a8f3871d52924b47792d5e5aa0` MIT; Piper
  `0987603ebd2a93c3c14289f3914cd9145a7dddb5` MIT; piper-phonemize
  `fccd4f335aa68ac0b72600822f34d84363daa2bf` MIT; declared espeak submodule
  `8593723f10cfd9befd50de447f14bf0a9d2a14a4` GPL-3.0.
- Packaging includes phonemizer/ONNX runtime, espeak data, glibc loader and
  C/C++ runtime libraries. Phonemizer CMake defaults to ONNX Runtime 1.14.1
  and fetches an espeak master archive: the submodule pin alone does not prove
  the packaged dependency version. OCI label MIT is not the complete license chain.
- No separate vocoder for this Piper ONNX. Integration: existing LocalAI
  Piper TTS → disclosed derived artifact with input hash/provenance; no new UI.
- Installed extracted bytes, resolved library SBOM/licenses/notices and CPU/RAM
  measurement remain unknown. Provisional capacity budget 256 MiB disk and
  0.5–1 GiB RAM is a planning estimate, not an admission PASS.
- Rollback proposal: isolate voice/backend directory and config; preserve
  current image/config hashes; disable/remove only newly admitted assets after
  reference review. No removal now. NOTICE must include voice card, repository
  MIT notices and all linked/runtime GPL/LGPL obligations after review.

Evidence: [pinned voice card](https://huggingface.co/rhasspy/piper-voices/blob/217ddc79818708b078d0d14a8fae9608b9d77141/en/en_US/ljspeech/medium/MODEL_CARD),
[Piper packaging](https://github.com/mudler/LocalAI/blob/f7ad3f70eb5d8a0ddf80e08557f0d7df28cf032e/backend/go/piper/package.sh),
[phonemizer dependency declarations](https://github.com/rhasspy/piper-phonemize/blob/fccd4f335aa68ac0b72600822f34d84363daa2bf/CMakeLists.txt).

### Urdu

[AhsanTalal/urdu-matcha-tts](https://huggingface.co/AhsanTalal/urdu-matcha-tts),
revision `d2201d251bf13bc461e7ef88ba889aa780246ee4`; card Apache-2.0.
Acoustic `matcha_tts.onnx`: 75,289,695 bytes,
`5d0fc22d9dac9b985e0b3a6e6e867b697997d73e790a65f4485c4ca4e231d88e`.
vocab.json 4,423; symbols.txt 1,379; config.yaml 546; README.md 5,989 bytes.
Acoustic/config/card subtotal 75,302,032 bytes. Training checkpoint/examples
unnecessary and excluded. This subtotal is **not** total runnable TTS size.
Publisher example requires a separate 22.05-kHz Vocos model; its documented
`charactr/vocos-mel-22khz` API returned 401. No alternate vocoder was silently
substituted. Exact vocoder pin, weight hash/license/size and mel compatibility
are unresolved; package versions/hashes for onnxruntime, numpy, soundfile,
torch, torchaudio and vocos are unpinned. Code-license closure and full NOTICE,
installed disk/CPU/RAM and total runnable bytes remain blocked.
Integration would require a bounded specialist adapter behind existing TTS
artifact handling, preserving raw Urdu and rejecting unsupported characters;
out-of-vocabulary characters currently map to spaces in publisher example.
Rollback: isolated candidate environment/config only; no retained artifact deletion.

## TTS Approval Readiness

English=false; Urdu=false. **No download approval requested with unknown
dependencies/licenses/installed size.** No TTS model/backend/dependency downloaded.

## Image Similarity Progress

Existing seven-file SigLIP host cache remains available and previously hash-checked.
Current worker binds only repository `models/media`; configured SigLIP directory
does not exist inside it. torch/transformers/PIL/sentencepiece are installed;
missing items are the mounted seven-file asset and enabled role, not a new model.
Embedding role is disabled. No fresh worker load/embedding PASS.
Fresh three-candidate API ranking over existing current-version embeddings PASS:
pose 0.966838, blur 0.935789, other synthetic portrait 0.785178; 768-D pin and
three source citations retained. These are semantic visual similarities, not
object/person identity or probabilities. Data reference image/candidate panel
inspected. No fresh natural-language image-similarity Ask/citation navigation PASS.

## Face Candidate Progress

Installed YuNet/SFace direct detection: one synthetic portrait → one face in
2.563 seconds; synthetic street negative → zero in 0.448 seconds. No inference
result retained as evidence. Fresh scoped comparison of stored 128-D embeddings:
blur 0.877459, pose 0.841914, other synthetic portrait 0.309298, all cited.
Data displayed the retained face box, crop and candidate-only wording. Worker
face role is disabled; no fresh worker ingestion or face Ask PASS. No identity,
enrollment or demographic claim. Did not expand UI ranking to every loaded image;
the API check deliberately named exactly three synthetic candidates.

## Demo Workspace / Files Changed / Tests

33 sources now: 32 completed, one pre-existing failure; audio count 4. “Completed”
includes technical inspection and does not imply transcript availability.
Exactly one English source/version/job and five Ask histories added; no Urdu
duplicates, bulk uploads, reprocessing, database migration or volume changes.

New acquisition/evaluation/resource scripts, independent scorer, read-only live
receipt verifier and two-fixture face probe; three presentation source files
and two focused test files; this report/JSON; existing master/checkpoint/roadmap/
ledger/backlog/maturity pointer/demo/readiness artifacts updated.

- Nine acquisition/license/scorer tests, nine ASR timing tests, four Video V3
  tests: 22 PASS; separate 19 ANPR/OCR baseline self-tests PASS. Acquisition
  tests and immutable file validation also PASS under optimized Python;
  integrity checks cannot be disabled by removal of assertion statements.
- Four analyst test files: 31 named checks PASS (Data/Media/Ask/Activity),
  confirmed by a fresh command naming only those four existing test files.
- Focused Go NXB1/derived-text/face/image routing, scope and citation regression
  command PASS. Existing tests do not prove the missing live STT semantics.
- Independent WER/CER recomputation, two retained derivative parent/time checks,
  ten downloaded file hash checks, Python compile PASS.
- ESLint zero errors, nine existing JSX unused-symbol warnings.
- Privacy scan: 24 distinct private reference/hypothesis/retained text strings
  absent from all 23 touched public/source files. Private assets are Git ignored.
  All ten public manifest rows equal the private acquisition hashes/IDs/sizes.
  JSON parse, production sample-specific constant scan, conflict-marker/new-file
  whitespace checks and `git diff --check` PASS (tracked diff only for Git;
  existing line-ending warnings retained). No transcript/audio payload published.
- No full build, dependency install, coverage baseline change or hook bypass.

## Manual UI Results

Normal Add Data registration, source reopen, English/Urdu playback completion,
raw Urdu RTL, derivative display, fresh Activity entries/result reopen and
existing face/image candidate Data panels observed. Five STT questions failed
semantic acceptance as above. New source fixes were not deployed. No claim of
whole-UI, citation-seek, responsive or LLM presentation acceptance.

## Downloads Performed / Total Download Bytes

Only ten authorized speech WAV assets and bounded official metadata/license
research. Dataset payload accounting 5,836,837 bytes; audio subset 5,120,580.
No full corpus, TTS/ASR/image/face weights, backend image layer or dependency pack.
Research/retry wire bytes unmetered and explicitly unavailable.

## Runtime Impact / Retained Impact / Product Certification Impact

Existing services unchanged; direct ASR/face inference can warm resident memory.
Read-only sampling/ranking and normal one-file intake/query activity only.
RetainedStateMutated=true for the explicit one-source/five-query checks, not
for benchmark results. No service recreation, model profile change, migration,
volume change, deployment, Git staging/commit/push or cleanup. Certification
unchanged: 62 REGISTERED / 12 SOURCE_VALIDATED / 1 FIXTURE_CERTIFIED /
0 REAL_WORLD_CERTIFIED / 4 PRODUCT_CERTIFIED. NX-B2 remains paused.

## Open P0 / Open P1 / Exact Next Action

No new cross-tenant isolation, destructive mutation or arbitrary-execution P0
reproduced. This is not a security audit. Open P1: disabled speech/image/face
worker integration; false transcript readiness (source-fixed); evidence scope
not transported; transcript routing/capability/exact/time/current-version gaps;
source timing and pagination deployment; uncertified media quick-action exposure.

Next source work: one bounded typed transcript scope/retrieval/exact/time slice
with independent multilingual/current-version/absent-transcript tests. Then
request approval for exact worker + API + LocalAI/UI deployment targets,
rollback preservation, unchanged 6-GiB physical-RAM gate, and reprocessing only
the named English source as a new immutable generation. Preserve ANPR/OCR/V3
settings; do not run old broad activation or reprocessing scripts. This request
does not yet authorize that deployment or reprocessing.

```text
FleursBoundedAcquisition=PASS
FleursFullDatasetDownloaded=false
EnglishSpeechClips=10
UrduSpeechClips=2
EnglishSTTWER=0.1079545455
EnglishSTTCER=0.0322939866
UrduSTTWER=0.4545454545
UrduSTTCER=0.1646341463
EnglishSTTDemoReadiness=BLOCKED_LIVE_INTEGRATION
UrduSTTDemoReadiness=BLOCKED_LIVE_INTEGRATION
RomanUrdu=DERIVED_PARENT_AND_TIME_VERIFIED_REVIEW_REQUIRED
STTData=PARTIAL
STTAsk=FAIL_SEMANTIC_ACCEPTANCE
STTCitations=BLOCKED_VALID_ANSWER_AND_TIME_SEEK
STTActivity=STORED_AND_REOPENED_NOT_SEMANTIC_PASS
TTSEnglishAcquisitionReady=false
TTSUrduAcquisitionReady=false
ImageSimilarity=STORED_RANKING_PASS_FRESH_WORKER_BLOCKED
FaceCandidateComparison=STORED_RANKING_PASS_DIRECT_DETECT_PASS_WORKER_BLOCKED
NoSelfOracle=PASS
NoHardcodedAnalyticalAnswers=PASS
NoUnrestrictedSQL=PASS
ProductCertificationPerformed=false
RetainedStateMutated=true
DatabaseMigration=false
VolumesChanged=false
NXB2ActivationPerformed=false
```

The last three safety PASS markers describe this slice's added code/data flow,
not an exhaustive audit of the repository or every historic answer.
