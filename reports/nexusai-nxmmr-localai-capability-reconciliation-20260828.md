# NX-MMR LocalAI capability reconciliation

Research and inspection completed 2026-08-29. This is a read-only/source-only
reconciliation; it does not authorize model acquisition, installation, build,
deployment, reprocessing, or retained-state mutation.

## Verdict

The fork is materially capable enough to support NX-MMR, but source, live
runtime, installed models, and retained derived artifacts are four different
truth layers and must not be collapsed.

- Upstream [LocalAI v4.9.0](https://github.com/mudler/LocalAI/discussions/11628)
  was released 2026-08-20 and remained the latest official release found in
  the 2026-08-29 research pass. It adds deny-by-default authentication,
  context compression, canonical model/backend views, KNN routing, admission
  controls and additional audio/video backends.
- The local fork is branch `codex/forensic-hybrid-checkpoint-20260723`, commit
  `40717b83510c08db25dc26b9d6674bf46db363ac`, and dirty. The configured `origin`
  is the user's fork; no upstream remote is configured. No pull, fetch, merge,
  checkout, commit, push, or PR occurred.
- The running well-known document reports version
  `40717b8 (40717b83510c08db25dc26b9d6674bf46db363ac)`. That identifies the
  commit, not a proven semantic release. “Live is v4.9.0” is therefore not an
  admissible claim.
- Source contains current model/backend, audio, face, detection, diarization,
  privacy, routing, discovery, and reranking surfaces. Presence in source does
  not establish that a backend or model is installed or usable live.

## Official surface versus live probe

Official LocalAI discovery documents include `/.well-known/localai.json`,
`/api/instructions`, `/api/instructions/:name`,
`/api/models/config-metadata`, and `/v1/models/capabilities`; see the
[LocalAI repository](https://github.com/mudler/LocalAI) and its current docs.

| Surface | Live result | Disposition |
|---|---:|---|
| `/.well-known/localai.json` | 200 | usable discovery root |
| `/api/instructions` | 200 | usable instruction registry |
| `/api/instructions/config-management` | 200 | config-management instruction present |
| `/api/models/config-metadata` | 200 | config metadata present |
| `/v1/models` | 200 | five installed model IDs |
| `/v1/models/capabilities` | 404 | live capability-discovery gap |
| `/backends` | 200 | eight installed backend registrations |
| `/system` | 200 | live system endpoint available |

The live well-known flags report agents, config metadata/patch, MCP, records,
tracing and VRAM estimation enabled; P2P is disabled. Instructions include
chat, audio, images, model/config management, monitoring, MCP, agents, records,
video, face/voice recognition, PII filtering, middleware administration and
intelligent routing. These flags advertise software surfaces, not installed
role readiness.

## NexusAI consequence

The live forensic sidecar remains stale at catalog
`2026-08-23.post-bfa-runtime.1`, 67 executable operations and 68 query-corpus
entries. Current source has 79 executable operations, 214 governed variants,
and catalog `2026-08-28.nxb1.1`. None of the 12 NX-B1 operations is live.
NX-B2.1 activation is paused, and NX-MMR is the active pre-approval program.

The live forensic capability response also exposed static suggestions for
operations that had only limited, fixture, retained-artifact, or no-data
support. That violates the NX-MMR product rule. The bounded source correction
now requires `PRODUCT_CERTIFIED` before an operation can be suggestion-eligible
and filters capability suggestions through that ledger. Focused Go verification
passes. This source change is not deployed.

## Relevant v4.9.0 opportunities

| Capability | NX-MMR use | Current disposition |
|---|---|---|
| deny-by-default auth | fail-closed forensic surfaces | reconcile during future activation; no auth mutation now |
| KNN/intelligent routing | evidence/role routing candidate | use only after deterministic MIME/signature routing and sealed precision tests |
| admission and backend traces | resource and provenance evidence | valuable for future benchmark runs |
| model capability discovery | role inventory | source supports it; live `/v1/models/capabilities` is 404 |
| Qwen3-ASR via llama.cpp | ASR challenger | rejected for the Urdu primary role because the [official language list](https://github.com/QwenLM/Qwen3-ASR) omits Urdu |
| Qwen3-TTS | TTS candidate | not Urdu-eligible; the [official model card](https://huggingface.co/Qwen/Qwen3-TTS-12Hz-1.7B-CustomVoice) lists ten languages and not Urdu |
| LocateAnything-3B | open-vocabulary grounding | defer: unnecessary 3B resource cost before specialist baselines pass |
| reranker backend | grounded KB retrieval | admit Qwen3-Reranker-0.6B Q8 only behind the proposed benchmark gate |

## Gate

No runtime action follows from this report. The next authorized action, if the
operator accepts the consolidated pack, is isolated acquisition/benchmarking;
live activation remains a later, independently approved boundary with rollback
tags and at least 6 GiB free physical RAM.

