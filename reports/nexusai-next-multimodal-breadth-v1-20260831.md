# NX-MMR-NEXT-MULTIMODAL-BREADTH-V1 — 2026-08-31

Verdict: **PARTIAL — safe source progress; acquisition and live acceptance remain gated.**
This is a checkpoint, not closure of the all-modality product baseline.
The latest owner directive supersedes historical ANPR/OCR tuning and activation
next steps. The sole roadmap and phase ledger retain ownership of this work.

## Verified starting state

- Branch `codex/forensic-hybrid-checkpoint-20260723`; HEAD
  `40717b83510c08db25dc26b9d6674bf46db363ac`; extensively dirty worktree.
  Existing edits/untracked files were preserved; no staging, commit or merge.
- Live `/version` reports that same Nexus snapshot, not an independently proven
  upstream semantic version. LocalAI image
  `sha256:54683ddd2c2302632b25c2619d13254f7ef685d46f556ea3854fdfcd5463f8c1`.
  Docker showed LocalAI, worker, PostgreSQL and NATS healthy; forensic API
  running and `/healthz` successful. Worker image display was `9f8021b0fc02`;
  its exact source-to-image seal was not freshly verified.
- The existing `default/nexusai-multimodal-product-acceptance` workspace has
  32 evidence sources: 31 completed, one failed; audio 3, documents 2,
  images 17, structured 8, video 2. This is a fresh bounded inventory, not a
  full database checksum reconciliation. Historical 21-source reports are not
  current counts.
- Host RAM inventory was permission-blocked: no current free-RAM claim or
  build-admission PASS. A subsequent elevated diagnostic was blocked by the
  approval service's usage limit; no bypass was attempted.

## Existing ANPR/OCR baseline preserved

Owner-observed image ANPR, Video V3, printed English, Urdu and mixed OCR remain
the functional internal-demo baseline with their stated limitations. No model,
preprocessing, threshold, frame cadence, grouping rule or retained artifact was
changed. No sealed holdout was used. V3 remains the 384 ONNX/FastPlate,
actual-decoded-frame, 4 FPS, plate-only development-demo path; V2/Ultralytics
were not activated. No ownership, identity or continuous tracking claim is made.

Fresh browser inspection saw ready image-positive, Video V3 and printed
English/Urdu/mixed sources. The existing image-negative source is **failed**,
not a certified no-detection result. We did not rerun any retained processing.

## Existing baseline regression

Dependency-free ANPR/OCR vertical suite: **19 passed**. Video V3 suite:
**4 passed**, covering observed text/source times, empty OCR, exclusive V2/V3
configuration and demo-scoped processor identity. Shared presentation checks
remain green. These are source-contract checks, not new accuracy benchmarks or
fresh end-to-end ANPR/OCR acceptance.

## Current LocalAI upstream reconciliation

Fresh official review confirms the v4.9.0 announcement dated 2026-08-20:
deny-by-default route classification, backend/model lifecycle consolidation,
Qwen3 TTS via llama.cpp and resource-admission changes. The live snapshot is
not proven equivalent; `releases/latest` metadata could not be fetched, so
latest-head/release completeness is **PARTIAL**. No full upstream merge.
[Official release announcement](https://github.com/mudler/LocalAI/discussions/11628).

| Capability | Decision in this workspace |
|---|---|
| Whisper/Faster-Whisper transcription | Reuse installed CPU backend and explicit language selection |
| Speech synthesis API and gallery | Reuse protocol after per-voice/backend admission; gallery presence is not installation |
| Text embeddings / KB retrieval | Keep installed Qwen3 embedding and bounded cited retrieval; no replacement |
| Face detection/embedding | Reuse YuNet/SFace primitive behind Nexus scope and candidate-only semantics |
| Image embedding | Reuse existing matched SigLIP image/text space, not the text-only embedding API |
| OCR and decoded video | Preserve accepted Nexus processors, cadence and provenance |
| Agents / tools / MCP | Reuse transport/execution; Nexus owns scope, allowlists, observations and certification |
| Generated-media/auth routes | Review selectively before deployment; do not assume upstream hardening exists locally |

Official protocol references: [features](https://localai.io/docs/features/index.print.html),
[backend reference](https://localai.io/docs/reference/index.print.html),
[TTS](https://localai.io/docs/features/text-to-audio/).

## Current LocalAI fork reconciliation

The local backend index contains newer capabilities, but the live installation
has only the five model aliases below and four backend families (eight alias/
backend registrations). Source has 79 executable operations/214 query variants;
the last separately verified live catalog had 67/68. That catalog count was
not freshly recertified. No model role, profile, API catalog, agent or schema
was changed by this slice.

Ports 8080/8091 are published on all interfaces. Anonymous loopback LocalAI
model/transcription requests succeeded; forensic API rejected anonymous access.
Remote reachability and deployment auth intent were not tested. Treat this as
a deployment-hardening review gate before non-local demonstration, not proof
of a newly reproduced tenant bypass.

## STT runtime / model inventory

Installed aliases: `whisper-tiny`, `faster-whisper-small-ur`,
`face-detect-yunet-sface`, `qwen3-embedding-0.6b`,
`qwen_qwen3-4b-instruct-2507`. Installed backend families: CPU Whisper,
Faster-Whisper, face-detect and llama.cpp. No TTS backend/model.

Reuse Systran/faster-whisper-small, revision
`536b0662742c02347bc0e980a01041f333bce120`, CPU INT8 (historical installation
receipt: MIT, model/config/tokenizer/vocabulary/README total 486,214,370 bytes).
No redownload. Explicit `ur` avoids the previously measured Hindi-script
auto-detection issue. No new diarization, speaker identity or language claim.

## English STT

- Processing: fresh direct, non-retained inference on the existing 4.967-second
  synthetic English fixture succeeded in 10.111 seconds using the installed
  Faster-Whisper alias and `language=en`.
- Transcript: returned text includes number-word errors (`Nero` versus `zero`);
  identifiers are not considered verified.
- Timing: backend supplied one segment, 0–4.64 seconds. This is model timing,
  not independently checked word alignment.
- Data / Ask / Citation / Activity: shared source path exists; no natural-English
  retained end-to-end run was performed. Do not substitute the silent video or
  synthetic speech for natural positive evidence.
- Demo readiness: **FUNCTIONAL_LOCAL**, synthetic plumbing only.
- Certification limitation: natural English/Pakistani English remains data-gated.

## Urdu STT

- Processing: fresh direct inference on `clear-urdu.wav`, 6.24 seconds,
  `language=ur`, 3.783-second wall time, succeeded.
- Transcript: raw Urdu retained as a model observation. Against the existing
  independent publisher transcript, the repository evaluator measured WER
  **0.6000** (12/20 word edits), CER **0.1852** (15/81 character edits).
  Normalization is NFKC/casefold, punctuation/symbol separation and whitespace
  collapse; CER excludes spaces. One clip is not population certification.
- Timing: returned intervals 0–4.42 and 4.42–6.12 seconds; human timing oracle
  not supplied. No fabricated word-level accuracy.
- Urdu / Roman Urdu: three retained audio sources completed, with five raw
  transcript and five derivative segments in the current inventory. Raw Urdu
  is primary, `ur-Latn` secondary; identifiers remain exact where present in
  the raw transcript. Transliteration does not repair ASR mistakes.
- Data: fresh browser check displayed two Urdu paragraphs, two Roman Urdu
  derivatives and segment starts 0:00/0:04.4 for the clear clip. Duration was
  shown as not reported; browser audio playback was not verified.
- Ask / Citation / Activity: existing source-scoped phrase action is visible;
  stored audio Activity can reopen. Its old evidence-detail answer renders raw
  Markdown as a heading, so that legacy result is not presentation-accepted.
  No fresh query was submitted. Citation locators are top-level artifact fields,
  not `metadata.citation_locator`; the earlier null projection was not evidence
  that stored citations were missing.
- Demo readiness: **INTEGRATION_READY**, not a fully accepted live speech demo.
- Certification limitation: accuracy, native listening review, actual playback,
  fresh positive/zero Ask and source-seek acceptance remain open.

## Positive speech evidence

Existing local manifest: `local-acceptance-inputs/ACQUISITION_MANIFEST.json`.
Google FLEURS `ur_pk` test row 1 and validation row 4 have publisher transcripts
and CC-BY-4.0 attribution. They are **read speech**, including the misleadingly
named `conversational-urdu.wav`, not spontaneous conversation.

| Local clip | Seconds | Bytes | SHA-256 |
|---|---:|---:|---|
| clear-urdu.wav | 6.24 | 199758 | c499e06e26b8cbf37386b88bba26f37045439a775e964bdad8634376435a056e |
| conversational-urdu.wav | 10.5 | 336078 | f016076b153c2ed6be21fa5f7a4bbc22b3e391d40080b9fa3de3b8a2dd517550 |
| urdu-english-identifier.wav | 7.779 | 248992 | 606ffc7e281940b1b175fff98169a83a6a6ab7744ffd49f05849ac94318fd314 |

The third is synthetic mixed speech; the English-only fixture is synthetic
too. They test routing/identifiers, not natural mixed-language recognition.
No new dataset, private-folder sweep or holdout acquisition occurred.

## TTS capability inventory / English TTS / Urdu TTS

Both TTS baselines are **BLOCKED**: no installed voice/backend, no generated
audio and no governed Read aloud control. TTS remains output/accessibility,
never evidence truth. Do not add a fake control or generate imitation voices.

English candidate: Piper `en_US-ljspeech-medium`, single-speaker US English.
The [voice card](https://huggingface.co/rhasspy/piper-voices/blame/main/en/en_US/ljspeech/medium/MODEL_CARD)
states LJ Speech public-domain training data and training from scratch.
That is not approval of a particular Piper engine binary or dependency license.
The local backend index uses a mutable `latest-piper` OCI tag: resolve a digest,
compressed/unpacked size and full dependency notice chain before acquisition.

Candidate voice revision: `217ddc79818708b078d0d14a8fae9608b9d77141` in
`rhasspy/piper-voices`, directory `en/en_US/ljspeech/medium`:

| File | Bytes | SHA-256 |
|---|---:|---|
| en_US-ljspeech-medium.onnx | 63531379 | 6f52a751e2349abe7a76735eb09dc1875298c77ea2342ffd2fef79ff81b87f22 |
| en_US-ljspeech-medium.onnx.json | 4972 | 141d612cc0a95ed7efc1ca936b845c2364967f2e9217c5dbfcf69fc4d6c65860 |
| MODEL_CARD | 517 | Must seal before approval |

Voice-file subtotal: 63,536,868 bytes; **not** total installation size.
CPU peak RAM and latency are unmeasured. No permissive-license conclusion is
inferred from the repository-wide MIT badge alone.

## Urdu TTS model decision

One focused candidate, **AhsanTalal/urdu-matcha-tts**, not admitted. Publisher
declares Apache-2.0, Urdu grapheme input, single speaker/style, ONNX CPU support,
80-band mel output at 22.05 kHz and a separate Vocos dependency. It does not
claim code-switching or voice cloning. Its example silently substitutes spaces
for unknown characters; a future adapter must reject/report unsupported input,
especially identifiers. [Publisher model card](https://huggingface.co/AhsanTalal/urdu-matcha-tts).

Exact revision, ONNX/config/vocabulary sizes/hashes and the referenced
`charactr/vocos-mel-22khz` dependency could not be verified. Do not silently use
a different sample-rate vocoder or download the training checkpoint. No
downloadable approval pack can honestly be called complete yet. Supertonic's
published language list does not establish Urdu support; no Hindi/Arabic voice
is substituted for Urdu. CPU memory/quality remain unmeasured.

## Image similarity

Existing asset: matched SigLIP image/text embedding space, 768 dimensions,
revision `7fd15f0689c79d79e38b1c2e2e2370a7bf2761ed`. Fresh host inspection
found all seven files, 815,871,927 bytes; weight hash freshly matches
`2c63cb7d1f2e95ba501893cbb8faeb4ea9a3af295498d35097126228659c2af8`.
No acquisition is needed. A present host cache is not proof of current worker
mount/readiness. Reuse retained vectors for explicit bounded candidate ranking;
historical live acceptance proves five-candidate search and comparison.

Data/source comparison APIs and citation contracts exist. General semantic
image search is not yet an ordinary certified Ask operation. Fresh ranking and
source navigation were not replayed. **INTEGRATION_READY** for retained-path
replay; no identity/probability/scene-fact interpretation.

## Face candidate comparison

Installed YuNet/SFace model; detection and 128-dimensional comparison/search
have historical synthetic positive, negative and three-candidate proof.
Reuse existing scoped endpoints; no model search, biometric enrollment or
population identification. Candidate scores are not match probabilities or
identity; raw vectors must remain out of analyst presentation. Current complete
comparison/authorization/zero-result replay remains pending.
Data/Ask/citations are bounded by existing source/operation contracts.
**INTEGRATION_READY**, not biometric certification.

## Documents / KB

Reuse native TXT/PDF/DOCX extraction, existing Qwen embeddings and cited KB/
derived-text retrieval. Historical acceptance recorded 114 native passages;
current Data visibly lists the retained DOCX with 112 passages. No fresh full
retrieval run or scanned-PDF/layout/table qualification. **INTEGRATION_READY**.

## Structured families / cross-evidence operations

Keep CDR, IPDR, subscriber, tower, structured ANPR, financial, access-log and
generic paths. Historical 65-operation runtime matrix: 64 answered, one
accepted zero; not rerun here. Four operations remain PRODUCT_CERTIFIED in the
current source matrix. Broader source operations are not silently promoted.

Cross-evidence uses NX-A1 typed tools, separate Fact/Observation packets,
tenant/case/evidence/version scope and citations. No unrestricted SQL,
LLM-generated relation truth or same-identifier-to-identity leap. No new
cross-evidence operation or end-to-end acceptance: **SOURCE_READY** only.

## Auto routing

Reuse existing media classification and configured processors. Historical
NX-MMR route plan passed 16/16 shadow cases; broad live auto-routing activation
was not performed. Missing models remain unavailable, not zero findings.

## Data UX / Ask UX / Activity / citation integrity

The STIM skill's source-truth/presentation rules caused two bounded corrections:

1. **NEXT-P1-ASR-TIME:** text-only ASR no longer invents 0-to-clip-duration
   timing; invalid/non-finite/boolean/reversed times cannot become valid
   locators. Text/raw values survive with a partial/unavailable warning.
   Roman Urdu pairs by parent observation, never equal/missing timestamps.
   Missing UI time reads `Time unavailable`, not `0:00`; real zero survives.
2. **NEXT-P1-DATA-PAGE:** filters disclose loaded-page scope and ask the analyst
   to load more before concluding that no sources match. Live reproduction:
   audio filter showed zero on the first page, then three after Load more.

Both are **source-fixed, deployment/UI acceptance pending**. The ASR processor
revision is `nexusai-next-breadth-asr-timing-v1`; retained artifacts are unchanged.
No new API/schema
or control was introduced. Existing derived-text queries already preserve
artifact/evidence/version and top-level locators. Legacy raw-Markdown Activity
rendering is a recorded presentation gap, not a new successful answer.

The catalog validator enforces PRODUCT_CERTIFIED suggestions, but media quick
actions are built locally without consulting that certification. Fresh browser
inspection also found suggestions in stored historical Ask answers. Therefore
`VisibleSuggestionsProductCertifiedOnly=PASS` is **not** asserted for the
whole live UI. A bounded gate/explicit internal-demo exception reconciliation
is required before ordinary-user acceptance; do not rewrite stored history.

## Demo workspace / team-lead sequence / readiness matrix

Use the existing Multimodal Product Acceptance workspace. No new case, upload,
seed, cleanup or retained reprocessing was performed. Sequence and operator
checks are in `docs/demo/nexusai-team-lead-multimodal-demo-v3.md`.
The dated readiness Markdown and JSON distinguish owner baseline, fresh direct
inference, historical acceptance, source tests and unrun live checks. TTS is
shown as unavailable, not demonstrated with a mock.

## Certification matrix / existing ANPR/OCR backlog

Source counts freshly reconciled: REGISTERED 62, SOURCE_VALIDATED 12,
FIXTURE_CERTIFIED 1, REAL_WORLD_CERTIFIED 0, PRODUCT_CERTIFIED 4. No changes.
ANPR robustness, OCR accuracy, handwriting, V2/V3 optimization and threshold
tuning remain deferred until breadth is complete. No self-oracle or sealed
holdout reuse; fixture tests do not promote real-world certification.

## Models / downloads required / exact approval boundary

No new acquisition is authorized by this checkpoint. The next data choice is
three lawful natural English clips (clean/read, conversational/Pakistani
English, identifier-bearing), each at most 30 seconds, with independently
written transcripts and permission for local non-retained evaluation. Owner
recordings avoid a public-corpus download; any public corpus needs a separate
exact license/revision/file manifest first.

The TTS acquisition pack is **not ready to execute**. Resolve the English
backend digest/licenses/total bytes and Urdu model/vocoder revision/hash/size/
compatibility first, then request one explicit isolated acquisition approval.
No mutable-tag install, automatic dependency/model pull, service/profile change
or retained generation is covered. Do not request blanket download permission.

Later activation must separately cover this worker/UI source delta, preserve
rollback and protected services, pass the unchanged 6 GiB build/deployment gate,
then run positive/zero speech, playback/seek, source citation, Activity and
responsive checks. Do not run historical broad activation scripts.

## Files changed / tests / manual UI / diff

Production: `media_pipeline.py`, `roman_urdu.py`, `AnalystData.jsx`,
`analystMediaPresentation.js`, `analystDataPresentation.js`. Tests: new
`test_asr_timing_integrity.py`, extended two analyst presentation tests.
Documentation: this report, dated Markdown/JSON readiness matrix, demo v3,
master/checkpoint, sole roadmap/phase ledger/backlog, maturity-matrix pointer,
and forensic feature documentation. No certification values changed.

Validation commands and outcomes are finalized in the readiness JSON. Existing
host Python lacks pytest; no dependency was installed and no full pytest PASS
is claimed. Dependency-free unit tests were used. ESLint reports no errors and
nine existing JSX unused-symbol warnings. An optional standalone JSX transform
could not run because esbuild is absent; ESLint parsing passed and no package
was installed. No long/full build or deployment.

Final checks: 9 ASR + 4 V3 + 19 ANPR/OCR + 35 analyst presentation checks =
67 passing checks. Readiness JSON parses, all 16 capability IDs/states are
unique/valid, modality counts sum to 32 and certification counts to 79.
All 19 changed paths exist with no merge-conflict markers; current checkpoint
headings occur once. `git diff --check` passes. Most target files were already
untracked, so Git's tracked-diff check alone does not cover their contents;
the focused tests and explicit path/content checks provide that additional
validation. No staging or commit was performed.

Read-only live UI: neutral Home/Data/Ask/Activity, selected scope, evidence
counts, three audio sources, Urdu/derivative display and stored Activity reopen
verified. New source changes are not live. No fresh Ask, full responsive matrix,
listening assessment or playback/seek PASS. Browser workflow followed the
Browser skill. These limitations remain explicit in the matrix.

## Runtime impact / retained impact / open P0 / open P1 / next action

Only two bounded direct transcription calls, read-only API/Docker inventory and
browser navigation were performed. Inference may warm backend memory/logs.
No deployment, service recreation, asset/config/profile mutation, migration,
volume change, retained evidence mutation or new query-history write.

No P0 incident was reproduced by these bounded checks; this is not a security
certification. P1 timing and pagination fixes require activation. Suggestion
eligibility coverage requires source/UI reconciliation; legacy Activity
presentation and current worker asset mount need follow-up. Natural English
data, TTS acquisition and complete live demo remain blocked/unaccepted.

Exact next action: obtain the bounded natural-English evidence choice and finish
the pinned TTS admission manifest; review the source delta without deploying.
NX-B2.1 remains paused and product certification is unchanged.
