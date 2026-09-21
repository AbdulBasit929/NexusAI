# NexusAI current-4B standalone resource proof then fresh English qualification — prepared 2026-09-09

Status: `R2_CONSUMED_INCOMPLETE_RESOURCE_FAILURE_DISPOSABLE_LIFECYCLE_SOAK_FROZEN_PENDING`. The final resource gate loaded the current 4B model and passed generic transport, then consumed r2 and stopped during its first semantic request. R2 must not be rerun; current 4B semantic quality remains insufficient evidence.

The prior `nxb21-english-functional-current-4b-fresh-r1-20260909` corpus is formally `CONSUMED_DO_NOT_RERUN`. Its result is `INCOMPLETE_RESOURCE_RUNTIME_FAILURE`, not a semantic-model failure; current 4B semantic quality remains `INSUFFICIENT_EVIDENCE_FROM_THIS_RUN`. The immutable adjudication is in `reports/nxb21/nexusai-english-functional-model-qualification-20260909.json`.

## Frozen current model

- Model: `qwen_qwen3-4b-instruct-2507`
- Artifact: `/models/Qwen_Qwen3-4B-Instruct-2507-Q8_0.gguf`, 4,280,405,216 bytes
- Artifact SHA-256: `260b5b5b6ad73e44df81a43ea1f5c11c37007b6bac18eb3cd2016e8667c19662`
- Profile: `/models/qwen_qwen3-4b-instruct-2507.yaml`
- Profile SHA-256: `ef036e0a02cc5e8331dc4dff41c0e0cb68e6e0bda71d92b92f47f673472e666f`
- Effective runtime: context 4096, threads 8, parallel 1, batch 512, GPU layers 0

No model, artifact, quantization, profile, context, thread, batch, parallelism, or backend change was made.

## Resource proof boundary

The prior run observed 6.72 GiB before load and 2.093 GiB after load, an observed delta of 4.627 GiB. The formal gates remain 6 GiB before loading and 4 GiB while loaded. A conservative empirical advisory target is 9.127 GiB, calculated as `4.0 + (6.72 - 2.093) + 0.5`; the additional 0.5 GiB is an explicitly disclosed safety margin, not a new formal threshold.

The standalone script performs safe clean-page-cache recovery only when writeback is zero, preserves dirty pages, takes five preload samples, executes one generic non-benchmark load request, takes five loaded samples, executes two more generic transport requests, takes five post-soak samples, and only then starts the qualification evaluator. It does not compile, rebuild, run a repository-wide freshness scan, or kill Codex, browsers, editors, Explorer, Docker/WSL, or arbitrary processes. The only forced process stop is the owned evaluator if the 4 GiB floor is breached after legitimate corpus dispatch.

The generic resource/transport requests are explicitly `RESOURCE_SMOKE_QUALITY_EVIDENCE=NONE`. If preload fails, no model request or resource lock is created. If load, loaded RAM, transport, or post-soak RAM fails, the fresh semantic corpus remains unconsumed and the current 4B runtime role is adjudicated without another 4B tuning loop.

## Fresh qualification freeze

- Corpus ID: `nxb21-english-functional-current-4b-fresh-r2-20260909`
- Corpus SHA-256: `df0554164bd6aae54525a3329f3e570cbc37880c2d89eff7cdb58c513d7c74e5`
- Oracle SHA-256: `80b01fa1c46535d49bfc7a74ce4a244dedb24479b997328b072d6ee75a863ed9`
- Cases: 101 semantic + 32 router + 16 synthesis = 149
- Workspace operation coverage: 66/66
- Freshness: PASS across 3,877 files; exact overlap 0, normalized overlap 0, prior-corpus overlap 0
- Evaluator SHA-256: `462084d7a76c35dc3d295b8a43139a18e5bb040d28d80a9a6d933996a13c448a`
- Runner SHA-256: `356e45013fc22da10e8ae4d88248de240f671087d4fe482732187d0f0202b70b`
- Freeze SHA-256: `1f2029c97579c3b9590ea340a860939dbf7229f50e6d03f056df8d205b9c95e5`

Static hash, tamper, gate-order, process-safety, and dispatch-lock tests pass. The standalone `-ValidateOnly` path passes. Focused and complete `api/forensic_records` tests pass; the complete package completed in 55.181 seconds. No live inference occurred.

## Preserved runtime

Retained tuple is `68|68|81|22511|870|68`, Activity is 353, active jobs are zero, and the accepted analyst index SHA-256 is `a0f1e4288c1b9ee46b4fa3c3c94870fef46b12d7c1718c36eee2e905d14ae7de`. Qwen3-8B remains `CLOSED_RESOURCE_INSUFFICIENT`. No deployment or retained-data mutation was performed.

## Operator execution

### Consumed r2 final run

Run `run-20260909T110254961Z` passed stable preload, loaded RAM, three generic
requests, and post-soak RAM, then created the r2 dispatch lock. No qualification
case completed: the first request for `r2-sem-001` was still in flight when
host RAM fell from 4.160 to 3.966 GiB over 46.584 seconds. The owned evaluator
was stopped, the model unloaded, and runtime integrity passed. R2 is
`CONSUMED_DO_NOT_RERUN`; semantic quality is `INSUFFICIENT_EVIDENCE`.

The next executable is no longer this report's r2 runner. The only authorized
next live action is the frozen disposable, non-semantic lifecycle soak described
in `reports/nxb21/nexusai-r2-consumed-resource-failure-and-lifecycle-diagnosis-20260909.md`.

### Final resource-harness correction

The previous runner SHA-256
`356e45013fc22da10e8ae4d88248de240f671087d4fe482732187d0f0202b70b`
incorrectly included the rising Windows/WSL recovery sequence in the formal
minimum. The corrected runner separates a bounded diagnostic recovery phase
from a new qualifying window. Its SHA-256 is
`8b4b6147175d3d662673f44295d21e95a59a897b4437001a156f7e7d3d789458`;
the updated freeze SHA-256 is
`4b804681d71c1b4f0b6f86fef229d95dd42cef0f0bf0e177b1cd7c4deaae1ea5`.

The authoritative r1 receipt records 6.720 GiB before load and 2.093 GiB after
the first request, so the measured delta is 4.627 GiB. Formal floors remain 6
GiB preload and 4 GiB loaded. The preferred empirical admission remains 9.127
GiB (`4.0 + 4.627 + 0.5`); the no-margin credible predicted-loaded threshold
is 8.627 GiB (`max(6.0, 4.0 + 4.627)`). These are empirical predictions, not
guarantees; the subsequent loaded and post-soak gates still require five
consecutive readings at or above 4 GiB.

After one safe cache reclaim, the runner observes diagnostic RAM every five
seconds for up to 120 seconds. It may begin the separate five-sample formal
window early only after a five-reading stable recovery window reaches 9.127
GiB. Otherwise it waits to the deadline and permits a formal window at 8.627
GiB only if the best stable recovery window predicts at least 4 GiB after the
measured load delta. Every formal sample must meet its chosen threshold, and a
window declining by more than 0.5 GiB from first to last fails. A failed final
admission stops before model load as
`CURRENT_4B_PREDICTED_RESOURCE_INSUFFICIENT`; no retry loop is permitted.

PowerShell parse, frozen hashes, tamper rejection, gate ordering, process-kill
prohibition, semantic artifact hashes, model/profile identity, runtime baseline,
and resource-state-machine synthetic tests pass. `-ValidateOnly` passes with
`RESOURCE_SMOKE=NOT_STARTED`, `QUALIFICATION_CORPUS=UNCONSUMED`, and
`LIVE_INFERENCE=false`.

### Superseded transient-window attempts

Run `run-20260909T102228770Z` stopped at the preload gate with samples
4.304, 4.301, 5.441, 7.233, and 8.418 GiB. The 4.301 GiB minimum failed the
unchanged 6 GiB formal floor, even though the final two samples later recovered
above it. The 9.127 GiB empirical advisory target was not met.

Resource smoke never started, neither dispatch lock was created, the current
4B model was never loaded, transport and qualification did not start, and the
r2 corpus remains unconsumed. Runtime integrity passed. Receipt SHA-256 is
`3bf8fde5c2465910b285651a8fe12a8f0c678b4200b667e5e86ccc622ad039eb`;
resource observations SHA-256 is
`d3b02d5dbedf8304eb0c45537586e1fa55e11131d761e2251f9f765f9ee81e8e`.

Run `run-20260909T103428085Z` similarly observed 4.400, 4.399, 5.288, 6.846,
and 8.308 GiB. Its receipt SHA-256 is
`85bb6e70df449dfe39431e6f28d115cbea9421c25f36421ce5342881928cae3b`.
Both sequences are now classified as diagnostic memory recovery, not formal
stable windows. Neither attempt provides current-4B runtime or semantic-quality
evidence.

Save this result, close Codex/ChatGPT, Chrome and other browsers, VS Code/IDEs, Teams/Slack/Office/Phone Link, and other nonessential applications. Keep Docker Desktop and exactly one PowerShell terminal running, wait 20–30 seconds, then run the corrected frozen command once from the repository root. This is the final current-4B resource-admission run. Do not kill Windows system processes, Memory Compression, Defender, Docker, or `vmmemWSL`.
