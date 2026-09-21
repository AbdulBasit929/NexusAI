# NexusAI current roadmap

## 2026-09-09 r2 consumed resource failure — disposable lifecycle soak frozen

Final run `run-20260909T110254961Z` passed stable preload (minimum 8.733 GiB),
loaded RAM (minimum 4.201 GiB), three generic requests, and post-soak RAM
(minimum 4.177 GiB). It then created the r2 dispatch lock. The first semantic
request, `r2-sem-001`, was still in flight when available RAM reached 3.966 GiB
after 46.584 seconds. Zero semantic, router, or synthesis cases completed. The
owned model unloaded and runtime integrity passed. R2 is
`CONSUMED_DO_NOT_RERUN`; current-4B semantic quality is
`INSUFFICIENT_EVIDENCE`.

Host RAM declined from the first qualification sample of 4.160 GiB through a
last above-floor reading of 4.027 GiB to 3.966 GiB. Evaluator working set peaked
at only 22.137 MiB and stayed flat. R2 had no per-call API/backend/vmmem series,
so KV/context accumulation, API/backend growth, WSL growth, and runtime leakage
remain unproven. Exact non-inference measurement shows each semantic request
carried 66 candidates and approximately 2,778–2,800 tokens (11,111–11,197
prompt bytes; 14,798–14,884 request bytes). The evidence supports observed host
drift plus the large first-request shape as the likely trigger, compounded by
only 0.177 GiB post-soak headroom.

Disposable soak `nxb21-current4b-disposable-resource-lifecycle-soak-v1-20260909`
is frozen, statically validated, and not executed. It runs 20 generic tiny calls
in batches `1,2,4,8,5` with owned-model resets, then three non-forensic
payload-shaped calls, recording per-call host RAM, API memory, corrected backend
RSS, vmmemWSL, and latency. Runner SHA-256 is
`18f27c767982e4e078ad3280b69aa44f7af830a7d7642378e2a70e71df3c95a2`;
freeze SHA-256 is
`5e206207dc49737056481eb3f4c510d417e2540271340e075ba1dcc62a66c0c2`.
`-ValidateOnly` passes without inference. R3 is not created and is prohibited
until this soak passes. Full adjudication:
`reports/nxb21/nexusai-r2-consumed-resource-failure-and-lifecycle-diagnosis-20260909.md`.

## 2026-09-09 final current-4B stable-state resource harness frozen

The two r2 standalone attempts (`run-20260909T102228770Z` and
`run-20260909T103428085Z`) stopped before model load because the old runner
treated rising recovery observations as the formal preload window. They created
no resource or qualification lock, made no inference request, preserved runtime
integrity, and left corpus
`nxb21-english-functional-current-4b-fresh-r2-20260909` fresh and unconsumed.
They are not current-4B runtime or semantic evidence.

The runner-only correction is frozen. Diagnostic recovery samples are now
separate from the five-sample qualifying window. Recovery is observed every
five seconds for at most 120 seconds; formal preload remains 6 GiB, loaded and
post-soak floors remain 4 GiB, and a first-to-last decline greater than 0.5 GiB
rejects a nominally qualifying window. The authoritative r1 measurements are
6.720 GiB preload and 2.093 GiB loaded, yielding a 4.627 GiB observed delta.
Preferred empirical admission is 9.127 GiB with the existing 0.5 GiB margin;
the no-margin credible predicted-loaded threshold is 8.627 GiB. If neither can
be sustained, the final run stops before model load as
`CURRENT_4B_PREDICTED_RESOURCE_INSUFFICIENT` and permits no further 4B tuning
cycle.

Old runner SHA-256:
`356e45013fc22da10e8ae4d88248de240f671087d4fe482732187d0f0202b70b`.
Corrected runner SHA-256:
`8b4b6147175d3d662673f44295d21e95a59a897b4437001a156f7e7d3d789458`.
Updated freeze SHA-256:
`4b804681d71c1b4f0b6f86fef229d95dd42cef0f0bf0e177b1cd7c4deaae1ea5`.
Static state-machine, hash, tamper, gate-order, process-safety, and dispatch-lock
checks pass. The corrected `-ValidateOnly` pass reverified model/profile identity,
runtime baseline, and `LIVE_INFERENCE=false`. Corpus, oracle, evaluator, model,
profile, semantic contracts, and questions are byte-identical.

Current state: `RESOURCE_HARNESS_CORRECTION=PASS`,
`R2_CORPUS_CONSUMED=false`, `MODEL_STATE=UNLOADED`,
`NX-PFC-P1-001=FINAL_STABLE_RESOURCE_ADMISSION_PENDING`, and
`NX-PFC-P1-002=FINAL_STABLE_RESOURCE_ADMISSION_PENDING`. The operator must close
Codex/ChatGPT and other nonessential applications manually, keep Docker Desktop
and exactly one PowerShell terminal, wait 20–30 seconds, then run the corrected
standalone command exactly once. This is the final current-4B resource-admission
adjudication.

## 2026-09-09 first standalone attempt — preload gate failed, R2 unconsumed

Standalone run `run-20260909T102228770Z` failed closed at the formal preload
gate. Its five available-RAM samples were 4.304, 4.301, 5.441, 7.233, and
8.418 GiB; the minimum was 4.301 GiB against the unchanged 6 GiB floor. The
late rise does not retroactively satisfy the requirement that every sample be
at least 6 GiB, and the 9.127 GiB empirical advisory target was not met.

`RESOURCE_SMOKE=NOT_STARTED`, the resource lock was not created, the current
4B model was not loaded, transport soak and qualification were not started,
and the fresh r2 corpus remains unconsumed. Runtime integrity passed with
retained tuple `68|68|81|22511|870|68`, Activity 353, zero active jobs, and
unchanged container identities/restarts and analyst-index hash. Receipt
SHA-256:
`3bf8fde5c2465910b285651a8fe12a8f0c678b4200b667e5e86ccc622ad039eb`.

This is `PENDING_OPERATOR_MEMORY`, not current-4B runtime or semantic-quality
evidence. Because no model request occurred and the last two samples recovered
above the formal floor, one bounded manual-close retry of the unchanged frozen
runner is allowed. If that retry fails the preload gate again, stop without
another retry or tuning loop and request separate approval for smaller-model
selection. Codex/ChatGPT and other nonessential applications must again be
closed manually before that retry; the runner must not terminate them.

## 2026-09-09 current-4B standalone resource proof prepared — R2 unconsumed

The prior English corpus `nxb21-english-functional-current-4b-fresh-r1-20260909`
is consumed and must not be rerun. It is formally
`INCOMPLETE_RESOURCE_RUNTIME_FAILURE`, not a semantic-model failure. The run
admitted at 6.72 GiB, its first semantic request timed out after 60,056 ms, and
loaded-state RAM was 2.093 GiB against the frozen 4 GiB floor. Only one of 149
cases was attempted, so `CURRENT_4B_SEMANTIC_QUALITY=INSUFFICIENT_EVIDENCE_FROM_THIS_RUN`.
The model was unloaded; retained tuple `68|68|81|22511|870|68`, Activity 353,
active jobs zero, all container identities/restarts, and the accepted analyst
index remained unchanged. Immutable adjudication:
`reports/nxb21/nexusai-english-functional-model-qualification-20260909.json`.

The corrected workflow is frozen but not executed. New corpus
`nxb21-english-functional-current-4b-fresh-r2-20260909` contains 149 cases
(101 semantic, 32 router, 16 synthesis), covers all 66 workspace operations,
and passes freshness across 3,877 files with zero exact, normalized, or
prior-corpus overlap. Corpus SHA-256 is
`df0554164bd6aae54525a3329f3e570cbc37880c2d89eff7cdb58c513d7c74e5`;
oracle SHA-256 is
`80b01fa1c46535d49bfc7a74ce4a244dedb24479b997328b072d6ee75a863ed9`.
The prebuilt evaluator SHA-256 is
`462084d7a76c35dc3d295b8a43139a18e5bb040d28d80a9a6d933996a13c448a`.

The standalone runner SHA-256 is
`356e45013fc22da10e8ae4d88248de240f671087d4fe482732187d0f0202b70b`.
It performs no compilation or repository-wide scan and kills no user/system
application. It requires five preload samples >=6 GiB, then a generic
non-benchmark model-load request, five loaded samples >=4 GiB, two additional
generic transport requests, and five post-soak samples >=4 GiB before the
evaluator may create the qualification dispatch lock. The empirical advisory
preload target is 9.127 GiB, calculated as
`4.0 + (6.72 - 2.093) + 0.5`; it is not a formal threshold. Static hash,
tamper, ordering, process-safety and lock-semantics validation pass;
`-ValidateOnly` passes; the complete forensic-records package passes in
55.181 seconds. The current 4B model is unloaded, both R2 live locks are absent,
and `LIVE_INFERENCE=false`.

Current state: `NX-PFC-P1-001=STANDALONE_RESOURCE_PROOF_PENDING`,
`NX-PFC-P1-002=STANDALONE_RESOURCE_PROOF_PENDING`, `NX-PFC-P1-003=BLOCKED_BY_P1_001_002`,
`NX-PFC-P1-004=BLOCKED_BY_P1_001_003`, `NX-PFC-P1-005=SEPARATE_APPROVAL_REQUIRED`,
`NX-B2.1D=OPEN`, and Qwen3-8B remains `CLOSED_RESOURCE_INSUFFICIENT`.
The operator must save this result, close Codex/ChatGPT, browsers, IDEs and
other nonessential applications manually, keep Docker Desktop and one
PowerShell terminal running, wait 20-30 seconds, then run the one frozen
standalone command documented in
`reports/nxb21/nexusai-current4b-standalone-resource-then-english-preparation-20260909.md`.



## 2026-09-09 English-first full-registry semantic planner foundation — source validated

The product-completion reconciliation found a capability-selection mismatch:
the governed runtime registers 79 executable operations, but unresolved language
was reduced to 22 residual capability tuples and phrase-shaped candidate
filtering. Source now includes `forensics.semantic-operation-proposal/v1`.
For English questions it projects every scope-compatible non-engineering
operation with family, intent, meaning, required/optional parameters, measures,
grouping, result kind, presentation, scope, and exposure metadata. The model can
return only one server-enumerated decision; scope, identifiers, dates, filters,
limits, tools, SQL, and facts remain server-authoritative. Strict decoding,
registry/scope validation, deterministic fact binding, audit publication, and
the existing residual fallback are wired. The complete forensic-records Go
package passes.

The next request-class slice is also source validated. Only explicit or
provably recognized forensic operations take the deterministic fast path;
product help, greetings, relevant general knowledge, and unfamiliar wording
reach the configured assistant/tool router. Its policy requires governed tools
for case/evidence/processing/model-state/computed-finding claims. The complete
agents package passes. Agent-pool integration passed 58/59; the sole failure is
an unrelated repeatable Windows file-lock race in the existing
`agent_jobs_test.go` concurrent persister test.

The bounded typed-query algebra slice is source validated as well. Typed plans
now assert the registered operation's measures and grouping, lower only
executor-consumed filters and sorts, and reject mismatches or ignored controls.
Canonical/generic source rows support allowlisted payload predicates, one
allowlisted sort, bounded top-K, and provenance; frequent-contact direction and
retained-video inclusive source-time bounds are explicitly lowered. The full
forensic-records package passes (527 specs passed, 33 intentionally skipped).
Arbitrary ad-hoc projection/group/time-bucket/compare composition remains
unexposed and uncertified.

The all-family and machine-readable gap/capability matrices are
`reports/nxb21/nexusai-product-functional-all-family-gap-analysis-20260909.md`
and `reports/nxb21/product-query-capability-matrix-v1.json`. This is source
validation only: no deployment, activation, retained-data mutation, new model
gate, or product certification occurred. Read-only live observation is
`68|68|81|22511|870|68`, Activity 353, active jobs zero; the delta from the
last accepted tuple is not attributed.

The final-local Qwen3-8B selection-12 attempt is consumed and closed as
`CLOSED_RESOURCE_INSUFFICIENT`; quality remains `INSUFFICIENT_EVIDENCE`.
Its receipt SHA-256 is
`430f40db43950981d064f2e52b5cc96290f038f28c33e14771a6e34f4583f4db`.
Do not rerun it. `NX-B2.1D=OPEN`, `FINAL_D_CANDIDATE=NOT_YET`, and
activation remains blocked. Next: create a fresh representative English
qualification corpus covering both the one-decision operation contract and the
assistant/tool request classes, then independently oracle the remaining ad-hoc
projection/group/time-bucket/compare slices before any planner exposure;
approval is required before inference, build/deploy, or live acceptance.


## 2026-09-08 NX-B2.1D fresh low-memory selection12 consumed by resource failure

The one-shot gate
`nxb21d-qwen3-8b-lowmem-fresh-selection12-20260908T071710Z` dispatched its
first model request and is consumed. It must not be resumed or rerun. The run
stopped before semantic adjudication when the loaded-state RAM gate remained
below 4 GiB after the full bounded recovery window (initially 2.396 GiB,
finally 3.195 GiB). Its immutable receipt SHA-256 is
`346a519e8e83ee18ade6ffb9a3a740ed7203f5fe07d5b0c102fcc6d32330f107`.
The receipt records one dispatched model call, `MODEL_UNLOAD=PASS`,
`RUNTIME_POSTCHECK=PASS`, retained tuple `67|67|80|22510|863|66`,
Activity 344, and zero active jobs. This is
`INCOMPLETE_RESOURCE_FAILURE`; Qwen3-8B model-selection quality remains
`INSUFFICIENT_EVIDENCE`.

Only rebuildable workspace caches were removed afterward, recovering about
3.9 GiB of disk space. Governed clean-page reclamation preserved dirty pages
and raised available host RAM from 4.870 GiB to a stable 5.790 GiB while Codex
and ChatGPT remained open; `vmmemWSL` fell to about 1.426 GiB. The exact
model profile, RAM floors, protected services, retained data, consumed lock,
run evidence, and receipt were not changed.

Current state: `QWEN3_8B_MODEL_SELECTION=FAIL_INCOMPLETE_RESOURCE`,
`FRESH_SELECTION12=CONSUMED_DO_NOT_RERUN`,
`FINAL_D_CANDIDATE=NOT_YET`, `FINAL_HOLDOUT=NOT_CREATED`,
`FINAL_QUALIFICATION=NOT_STARTED`, `NX-B2.1D=OPEN`, and
`D_ACTIVATION=BLOCKED`. Stop for owner adjudication; do not create another
corpus, profile, qualification, or activation.



## 2026-09-08 NX-B2.1D fresh low-memory Qwen3 8B selection12 frozen

The sealed post-close resource recheck passed at receipt SHA-256
`f148e40e5dc6a69fa23700c9332bb14c543001625a607be5d217c51b67af917e`;
the unchanged profile sustained three loaded-state readings at 4.422, 4.413,
and 4.304 GiB after entering with 6.767 GiB available. The requested batch is
128 while LocalAI's measured effective `n_batch` remains 512.

One new residual-only development gate is frozen as
`nxb21d-qwen3-8b-lowmem-fresh-selection12-20260908T071710Z`. Its corpus
SHA-256 is `a56f9b8690f5debc6034a682496ffff004bfe89ac1971d5908bcfeba0244512c`;
it contains exactly three English, Urdu, Roman Urdu, and mixed cases, with one
resolved, one ambiguous, and one insufficient-facts oracle per language. All 12
are production-source-admitted model calls with frozen fact packets, candidate
sets, decisions, and final typed outcomes. Freshness passed across 2,020
historical files with zero question, target-value, or normalized-literal
overlap. Schema conversion, strict decoding/rejection, runner syntax, source
tamper checks, one-shot dispatch semantics, and `-ValidateOnly` passed. The
model remained unloaded and no live inference ran.

Current state: `LOWMEM_RESOURCE_RECHECK=PASS`,
`QWEN3_8B_MODEL_SELECTION_GATE=FROZEN_NOT_EXECUTED`,
`FINAL_D_CANDIDATE=NOT_YET`, `FINAL_HOLDOUT=NOT_CREATED`,
`FINAL_QUALIFICATION=NOT_STARTED`, `NX-B2.1D=OPEN`, and
`D_ACTIVATION=BLOCKED`.

After manually closing nonessential applications while leaving Docker and
protected services running, execute exactly:

`powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\nxb21d-qwen3-8b-lowmem-selection\fresh-selection12\run_fresh_selection12_development.ps1`



## 2026-09-08 NX-B2.1D post-close Qwen3 8B resource-only recheck prepared

The owner authorized one standalone post-close resource admission recheck of the
unchanged `qwen3-8b-q4km-nxb21d-selection-lowmem-dev` profile. The exact
profile remains SHA-256
`672f8e2c29f162cdc74a8a8dfe7129955436c771db454b0a3ebb20a3bf9b560a`;
the pinned 5,027,783,488-byte artifact remains
`d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785`.
The resource-only script is
`scripts/nxb21d-qwen3-8b-lowmem-selection/run_post_close_resource_recheck.ps1`
at SHA-256
`b66fa7d98d246d7b379db84406428ad27862026814c17c1d1ef7d1b0be2a2f14`.
Its `-ValidateOnly` path passed with the model unloaded, retained tuple
`67|67|80|22510|863|66`, Activity count 344, and zero active jobs; it loaded
no model and performed no inference. The live path uses tokenizer-only loading,
records the requested batch 128 and actual LocalAI effective `n_batch`, waits
within the unchanged 180-second settling ceiling, requires three consecutive
loaded readings at or above the frozen 4 GiB floor, unloads only the owned
development model, verifies all container identities and retained/runtime state,
and seals one immutable receipt plus SHA sidecar. The prior effective
`n_batch` observation remains 512; no claim that 128 is active is permitted.
No fresh corpus, qualification, or activation exists. After closing Codex,
Chrome, IDEs, and other nonessential applications while leaving Docker Desktop
running, execute exactly:

`powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\nxb21d-qwen3-8b-lowmem-selection\run_post_close_resource_recheck.ps1`


## 2026-09-08 NX-B2.1D Qwen3 8B selection12 attempt 1 stopped on RAM floor

The one-shot Qwen3 8B selection gate is consumed and must not be rerun. Its
immutable receipt SHA-256 is
`bf45069a8c58f2020a3a1c85c56f02194ca00f7b63bda1f8c3d093cb157dc55f`.
It contains two result entries: `sel12-en-resolved` made the only model call
and passed exactly in 54,405 ms; `sel12-en-ambiguous` made no model call
because available RAM remained below the frozen 4 GiB between-call floor after
bounded recovery. Resource samples measured 2.640–7.907 GiB available RAM and a
6,048.8 MiB API-container peak. The model was unloaded; runtime postcheck,
retained tuple `67|67|80|22510|863|66`, activity count 343, and zero active
jobs passed. This is `INCOMPLETE_RESOURCE_FAILURE`, not a model-quality
failure: Qwen3 8B quality is `INSUFFICIENT_EVIDENCE` from 1/12 model calls.
The inner Ginkgo SUCCESS is a harness-presentation defect caused by returning
after recording the RAM failure; the outer runner and sealed receipt correctly
failed the gate. `FINAL_D_CANDIDATE=NOT_YET`,
`FINAL_HOLDOUT=NOT_CREATED`, `FINAL_QUALIFICATION=NOT_STARTED`,
`NX-B2.1D=OPEN`, and `ACTIVATION=BLOCKED`. The next action is an explicit
resource/model architecture decision before any fresh corpus or freeze.

Incident adjudication:
`reports/nxb21/nexusai-nxb21-d-qwen3-8b-selection12-attempt-1-resource-failure-20260908.md`.


## 2026-09-08 NX-B2.1D Qwen3 8B selection12 frozen; standalone execution pending

The exact pinned Qwen3 8B Q4_K_M artifact is verified and registered under a
new isolated CPU-only development profile; it remains unloaded. The fresh
12-case selection corpus has 3 cases per English/Urdu/Roman Urdu/mixed, with one
resolved, one ambiguous and one insufficient-facts oracle in each language.
Freshness PASS scanned 1,124 historical files with zero question/value overlap;
all 12 are production-path residual calls. Corpus SHA-256 is
`6376e5d18352edd6319562e9799d773bedb67f54bbedc901f06256dd53b226cd`
and freeze identity is
`e0aa163fdf62e12ee3b416bf6fa05061355d16f2ce616566578673ae926e9771`.
Schema/decoder, tamper, source-admission and `-ValidateOnly` checks pass with no
live inference. Q4 roles remain retired/insufficient; the Qwen development gate
is `FROZEN_NOT_EXECUTED`; D is OPEN, no final candidate or qualification
holdout exists, and activation is BLOCKED. Exact operator handoff:
`reports/nxb21/nexusai-nxb21-d-qwen3-8b-selection12-pre-run-freeze-20260908.md`.

## 2026-09-07 NX-B2.1D Decision8 adjudicated; replacement-model approval required

Decision8 is complete and immutable: 8/8 cases completed, 5/8 passed, eight model calls, runtime postcheck PASS, and receipt SHA-256 `0680fe8c706f2eda40b1352a22103a03d28f0e395f6b6835790e7d174dffce95`. The three failures are `dec8-ur-clarify=MODEL_CLARIFICATION_WEAKNESS`, `dec8-roman_ur-resolved=MODEL_MULTILINGUAL_INTENT_WEAKNESS`, and `dec8-mixed-resolved=MODEL_MULTILINGUAL_INTENT_WEAKNESS`; contract, transport, candidates, and admitted oracles were valid. Q4_FULL_PLAN_ROLE=RETIRED; Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE. No installed candidate is suitable for another development benchmark. The selected acquisition candidate is the exact pinned official `Qwen3-8B-Q4_K_M.gguf` artifact (5,027,783,488 bytes; expected SHA-256 `d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785`) at revision `7c41481f57cb95916b40956ab2f0b139b296d974`; download requires explicit owner approval. FINAL_DEVELOPMENT=NOT_STARTED_APPROVAL_REQUIRED; FINAL_D_CANDIDATE=NOT_YET; FINAL_HOLDOUT=NOT_CREATED; FINAL_QUALIFICATION=NOT_STARTED; D_STATUS=OPEN; ACTIVATION=BLOCKED. Do not rerun consumed corpora or include the compile-safe but incomplete Ask UI edits in a D image.

Full adjudication and exact next action: `reports/nxb21/nexusai-nxb21-d-decision8-adjudication-and-model-selection-20260907.md`.

## 2026-09-06 NX-B2.1D one-field residual-only 8-case development frozen

Current preparation state: RESIDUAL_V2_ONE_FIELD_DECISION=SOURCE_VALIDATED; RESIDUAL_ONLY_8CASE_GATE=FROZEN_NOT_EXECUTED. Fresh gate has 8 expected residual calls, 2/language, 4 resolved across network/CDR/logs/knowledge and 4 clarification (2 ambiguous, 2 insufficient). Corpus SHA `7bf616e0c146128dcf4fd9e4d470512ae49f48d3e8feb7497f1ba1ccdda9fbe3`. Freeze `nxb21d-q4-hybrid-decision-dev-r1`; identity `2467e8a85b0517fcfd5f7580825ddc55611db20ba4bdec073692c6d6facea4e3`. Freshness PASS against 590 historical files, zero question/value overlap. Full API PASS (527 passed, 31 skipped; 36.994 s); focused, schema conversion/enum, rejection, receipt, transport, guards and ValidateOnly PASS. No live inference. HYBRID_ATTEMPT_1=INCOMPLETE_FAILURE_RETIRED; RESIDUAL_V1_CONTRACT=RETIRED; old 32-case lock and evidence preserved. Q4_FULL_PLAN_ROLE=RETIRED; Q4_HYBRID_ROLE=HYBRID_PROMISING; FINAL_REPLACEMENT_QUALIFICATION_CANDIDATE=NOT_YET; REPLACEMENT_HOLDOUT=NOT_CREATED; D_STATUS=OPEN; ACTIVATION=BLOCKED.

Full handoff: `reports/nxb21/nexusai-nxb21-d-decision8-development-freeze-handoff-20260906.md`. New development freeze receipt: `reports/nxb21/d-decision8-development-freeze-v1.json`. Only the new standalone command is prepared; no gate was dispatched in Codex.


## 2026-09-06 NX-B2.1D first hybrid live attempt incomplete; residual and receipt source corrections

Supersedes the frozen-not-executed status below. HYBRID_DEVELOPMENT_ATTEMPT_1=INCOMPLETE_FAILURE; CASES_COMPLETED=5; DETERMINISTIC_PASS=4_OF_4; FIRST_RESIDUAL_RESULT=MALFORMED_RESIDUAL; RUNTIME_INTEGRITY=PASS. Current receipt and sidecar: `38684c2f1e9b1cb128ded3693c1778498480034eec560daf34c0e37fa06f36be`. Receipt post-hash rewrite reproduced and fixed in source. Residual schema allowed contradictory state fields; replaced with exclusive decision enum and server-derived audit state. Full API source tests PASS (42.242 s); receipt immutability tests PASS. All original run evidence and dispatch lock preserved. CURRENT_32CASE_CORPUS=RETIRED_AFTER_DISPATCH; no rerun/resume permitted. Q4_FULL_PLAN_ROLE=RETIRED; Q4_HYBRID_ROLE=HYBRID_PROMISING; DETERMINISTIC_FIRST_HYBRID=SOURCE_VALIDATED. Fresh 8-case residual gate recommended but NOT_CREATED; no new inference performed here. FINAL_REPLACEMENT_QUALIFICATION_CANDIDATE=NOT_YET; REPLACEMENT_HOLDOUT=NOT_CREATED; D_STATUS=OPEN; ACTIVATION=BLOCKED.

Incident evidence and exact next action: `reports/nxb21/nexusai-nxb21-d-hybrid-attempt-1-incident-20260906.md`; lossless diagnosis: `reports/nxb21/d-hybrid-attempt-1-diagnosis.json`. Historical freeze remains immutable and no longer matches corrected source.


## 2026-09-06 NX-B2.1D deterministic-first hybrid source validated; development frozen

Current status supersedes the earlier interface-development checkpoint below. Q4_FULL_PLAN_ROLE=RETIRED; Q4_HYBRID_ROLE=HYBRID_PROMISING; DETERMINISTIC_FIRST_HYBRID=SOURCE_VALIDATED; HYBRID_DEVELOPMENT_GATE=FROZEN_NOT_EXECUTED. The unchanged fresh corpus has 32 cases (8/language; 26 deterministic, 6 residual), SHA256 `caec70a2bbd5023358afffe24c00a51805b7539f5eb1c77c0e827325194407c8`. Freeze `nxb21d-q4-deterministic-first-hybrid-dev-r1`, identity `85566bcfcf0681fed5df649d442d25b975106e03e9d44476f88e85776547bad2`. Full API source tests PASS (526 Ginkgo passed, 29 skipped; 40.333 s); oracle admission, relevant routing, transport and guards PASS. DB/Testcontainers-dependent agent checks remain environment-blocked. No inference, replacement holdout, activation or runtime mutation. FINAL_REPLACEMENT_QUALIFICATION_CANDIDATE=NOT_YET; REPLACEMENT_HOLDOUT=NOT_CREATED; D_STATUS=OPEN; ACTIVATION=BLOCKED.

Handoff: [reports/nxb21/nexusai-nxb21-d-hybrid-source-freeze-handoff-v1-20260906.md](reports/nxb21/nexusai-nxb21-d-hybrid-source-freeze-handoff-v1-20260906.md); immutable development freeze receipt: `reports/nxb21/d-hybrid-development-freeze-v1.json`. Acceptance is frozen before operator execution. Existing evidence and consumed holdouts remain preserved.


> **Superseded for current execution on 2026-08-25.** The authoritative vNext
> program is `docs/roadmap/nexusai-next-generation-roadmap.md`; its controlling
> status is `docs/roadmap/nexusai-next-generation-phase-ledger.md`. The material
> below is retained as historical STIM/MMV evidence.

**Current vNext pointer (2026-08-27):** NX-1 is closed live with P0=0 and
foundation P1=0. NX-UX1 is active; NX-UX1A/B/C/D/E/F through the Home and
shared analyst-system slice are source/browser-pass, NX-UX1G is next, and
LocalAI/UI activation is not approved or performed.
See the two authoritative vNext documents above for the exact gate.

## 2026-08-24 MMV-2 final P1 source closure

**Active Work Item:** Urdu ANPR routing and the single governed grouped-video
ANPR operation are source-complete; narrow API plus LocalAI/UI activation and
live certification remain approval-gated.

The current retained zero-video oracle is healthy and complete, with zero
group artifacts. Source now reports that state truthfully rather than as an
unavailable capability. Full forensic tests, focused agent/UI tests, Go vet,
ESLint and the React production build pass. No worker source changed in this
closure slice, so the smallest deployment set is forensic API plus LocalAI/UI.
New rollback tags for the current running images and the 6 GiB physical-RAM
gate are mandatory before activation. Positive retained `sample.mp4` proof is
a later, separate ownership/review/mutation gate. MMV-3 has not started.

## 2026-08-24 MMV-2 ANPR/Image/Video bounded source progress

**Active Work Item:** MMV-2 source slice complete; retained positive proof,
deployment, media Ask/History operation certification and live browser
acceptance remain gated.

Narrow worker plus LocalAI/UI activation is authorized, but the 2026-08-24
preflight failed closed before any build: the mandatory 6 GiB free-RAM gate
reached only 2.56 GiB after a reversible service stop. Rollback tags are
preserved and the exact original healthy containers/counts were restored.

The authoritative source-timestamp sampler now replaces the MMV-1 index-based
labels. Bounded 1-second FastALPR cadence was selected from measured 5s/2s/1s/
adaptive matrices; heavier OCR/face/SigLIP work remains at 5 seconds. Positive
and negative videos, a 13-image Pakistan ANPR pack, exact/dHash comparison and a
16-image SigLIP pack are recorded under
`reports/mmv2-anpr-image-video-20260824/`. The old `LN15ZZC at 25s` statement is
historical non-retained output, not an exact source-time citation; current exact
source sampling produced four different review-required plate candidates.

No retained state or service changed. Next authority boundaries are the narrow
worker/UI/API deployment needed to activate source, the separately authorized
normal retained upload of the exact positive video if approved, and live
browser/Ask/History certification. MMV-3 acquisition remains out of scope.

## 2026-08-24 MMV-1 real-world maturity baseline

**Active Program Phase:** Post-Demo Multimodal Maturity Program  
**Active Work Item:** MMV-1 source-complete; retained video proof, model admission,
deployment and live browser acceptance remain separately gated

MMV-1 reconciled the accepted multimodal product with current source and live
read-only runtime truth. The unchanged deployed video path passed a non-retained
real 4K/60 positive control (`LN15ZZC` at 25 seconds); current Tesseract
`eng+urd` measured aggregate non-negative Urdu CER `0.436842` and therefore
needs an MMV-3 challenger; and two natural Urdu speakers plus one synthetic
identifier sample established a bounded ASR baseline without population-level
accuracy claims. Audio presentation and contradictory per-frame video zero
limitations are corrected in source. No retained upload/reprocess, model
download, database migration, deployment, cleanup, or commit occurred.

The machine inventories and benchmark evidence are in
`reports/mmv1-real-world-validation-20260824/`; the master report is
`reports/nexusai-mmv1-real-world-validation-20260824.md`. Live runtime has
advanced independently beyond the 21-source checkpoint to `51 evidence | 51
versions | 64 jobs (60 completed, 4 dead-letter, 0 active) | 22,207 canonical
records | 441 artifacts | 47 KB assets`; 24 evidence are now in the product
acceptance collection. MMV-1 did not cause this drift.

Rebaselined: 2026-08-24  
Authority: `NEXUSAI_MASTER_DIRECTIVE.md`  
Detailed status: `docs/roadmap/nexusai-phase-ledger.md`  
Controlled backlog: `docs/roadmap/nexusai-product-engineering-backlog.md`

Historical declaration below is preserved as the pre-MMV checkpoint.

The latest team-lead breadth directive preserves STIM-0 through STIM-7 as
closed and defers Governed Runtime Query Intelligence. BF-0 reconciled the
existing R8-R17 history. BF-1 ANPR, BF-2 Image, BF-3 Audio and BF-4 Video now
have bounded source implementations and accepted non-retained processor runtime.
The corrected worker is deployed with the pinned `faster-whisper-small-ur`;
FastALPR passed an earlier non-retained processor check but is disabled in the
active worker configuration. Composed video metadata, 0/5-second frames,
embedded audio, timestamped ASR, unique IDs, provenance and cleanup pass. The isolated retained BF-A recovery
now passes worker orchestration, persistence, audio/video processing, Data/KB,
hybrid citations and History. Retained image jobs currently emit technical
observations only because no approved local vision/ANPR role is configured;
agent-history citation presentation and Home-card overflow are recorded P2
limitations. Post-BF-A source now adds deterministic image fingerprints and a
scoped comparison API, governed Roman-Urdu derivatives, bounded native
TXT/PDF/DOCX extraction, History citation fallbacks, responsive Home/Data
improvements and explicit multimodal operation maturity. Focused source suites
pass. None of those changes is represented as deployed or retained acceptance;
worker-only activation, a separately authorized retained fixture run and fresh
browser verification remain required. Face detection/similarity is
`MODEL_APPROVAL_REQUIRED`. The single operator guide is
`docs/demo/nexusai-breadth-multimodal-demo-guide.md`. The
Pakistan Urdu M1 now passes in the deployed worker with the verified local
`Systran/faster-whisper-small` challenger. Explicit `ur` guidance materially
improved both lawful FLEURS clips; automatic detection still chooses Hindi, so
known-Urdu evidence uses the governed explicit-language policy.
The synthetic fixture preserved phone/time but not plate and remains M2. The
lawful ignored pack has been retained and processed under explicit operator
approval. Historical R8 evaluation
evidence is preserved and is not reinterpreted as certification.

APF was a bounded cross-cutting work item and is now closed without replacing
the R0-R17 sequence or reopening accepted phase boundaries. R8 remains valid,
preserved, paused and non-blocking. The separately authorized STIM-0 through
STIM-7 program is now closed with final source/runtime acceptance. The guarded API activation
and controlled synthetic acceptance-data ingest have run; the ingest accepted
seven fixtures with 21 total rows, 20 accepted rows, one duplicate row and zero
rejected rows. The first read-only closure probe exposed a UUID/text comparison
defect in the source-membership SQL boundary. The next activation proved that
repair and its stable negative response, then exposed two narrower P1s: raw
Pakistan phone forms were filtered before multi-CDR canonicalization, and the
cross-family optional family filter compared an enum to text. Their activation
proved the seven-event oracle and negative membership gate, then exposed the
last two bounded gaps: raw `VOICE`/`CALL` provider tokens in the conflict key and
an enum/varchar candidate `UNION`. The final activation and retained-data replay
now pass both corrections. STIM-5/STIM-6 and the final STIM-7 certification also
passed, closing the whole program with zero open P0/P1; stopped R8
acquisition/model research does not resume implicitly.

Repository STIM work is governed by
`.agents/skills/nexusai-structured-intelligence-maturity/SKILL.md`, the living
architecture at `docs/design/nexusai-structured-intelligence-maturity.md`, and
the machine-readable matrix at
`configuration/nexusai_stim_maturity_matrix.json`.

## Governance

Instruction precedence is:

1. latest explicit user instruction;
2. `NEXUSAI_MASTER_DIRECTIVE.md`;
3. `AGENTS.md` and task-relevant `.agents/` guides;
4. this roadmap;
5. `docs/roadmap/nexusai-phase-ledger.md`;
6. `NEXUSAI_CONTINUATION.md`;
7. `NEXUSAI_NEXT_CHAT_PROMPT.md`;
8. current architecture, design, testing, security, model and data docs;
9. current acceptance reports;
10. historical reports; and
11. upstream LocalAI documentation.

Completed phase boundaries remain frozen. P0 security/integrity defects and
bounded P1 foundational blockers may interrupt the active phase. P2/P3 issues
are recorded in the single controlled backlog and assigned to their owning
phase without reopening accepted phases.

## Program sequence

### Active STIM sequence

| Phase | Scope | Status |
| --- | --- | --- |
| STIM-0 | audit, governance, maturity matrix, issue register, benchmark corpus | source_baseline_complete |
| STIM-1 | multi-schema intake, profiling, mapping, time policy | source_accepted |
| STIM-2 | canonical and dynamic attributes | source_accepted |
| STIM-3 | analytical truth and certification | closed_source_and_runtime_accepted |
| STIM-4 | multi-source, multi-CDR, and cross-family correlation | closed_source_and_runtime_accepted |
| STIM-5 | natural-language semantics and grounded Fact Packets | closed_source_and_runtime_accepted |
| STIM-6 | analyst answer and proof presentation | closed_source_and_runtime_accepted |
| STIM-7 | performance, security, deployment, and full acceptance | closed_source_and_runtime_accepted |

The preserved R0-R17 sequence remains product history and future family
ownership. The closed STIM phase table above is the accepted structured and
textual maturity baseline for post-STIM reconciliation.

| Phase | Name | Program status | Immediate disposition |
| --- | --- | --- | --- |
| R0 | Authoritative Reconciliation | complete | Maintain truth as later state changes. |
| R1 | Complete LocalAI Reverse-Engineering Audit | source_accepted | Re-audit only when relevant source changes. |
| R2 | NexusAI Product and Brand Architecture | source_accepted | Preserve accepted identity, role and product contracts. |
| R3 | NexusAI App Shell and Deep White-Label Foundation | complete | Preserve accepted shell behavior. |
| R3.1 | NexusAI Elite Visual System and UI/UX Transformation | complete | Reuse the visual system in every later family. |
| R4 | Case-Centered Workspace Architecture | complete | Preserve one authoritative active-case scope. |
| R5 | Evidence and Ingestion Product Experience | complete | Preserve evidence, custody, intake and accounting contracts. |
| R6 | Ask NexusAI and Specialist Orchestration Experience | complete | Preserve R6.6 live acceptance; defer non-blocking polish. |
| R7 | Pakistan CDR and Tower Intelligence Deepening | complete | Preserve accepted structured/protected-data, specialist and live UI contracts. |
| R8 | ANPR Image and OCR Intelligence | valid_but_paused | Preserve T2-V, stopped CCPD partial, packages, receipts, and rejection evidence. Boundary A/B remain blocked; no transfer, evaluation, promotion, or R8.5 during APF. |
| R9 | General Image Forensics | not_started | Build on R8 image infrastructure. |
| R10 | Urdu/English Audio and Speech Intelligence | not_started | Requires an approved ASR benchmark. |
| R11 | Video Intelligence | not_started | Orchestrate accepted image/audio foundations. |
| R12 | Financial and Access/Security Intelligence | not_started | Reuse structured foundations; preserve exact arithmetic. |
| R13 | Document and Knowledge Intelligence | not_started | Requires extraction/OCR/retrieval gates. |
| R14 | Captures, Databases and Archives | not_started | Requires hostile-input security acceptance. |
| R15 | Entity and Relationship Intelligence | not_started | Requires cited, time-valid family relationships. |
| R16 | Enterprise Administration and Scale | not_started | Owns auth, tenant, retention, scale, backup and SLO gates. |
| R17 | Release and Continuous Regression | not_started | Owns release, upgrade, rollback and full production gates. |

The historical family-phase order remains preserved, but STIM is the current
explicitly authorized program boundary. R8 does not resume implicitly.

## Current next action

- **STIM-0:** source baseline complete. The family-by-family maturity matrix,
  issue register, living architecture, repository skill, and narrative
  benchmark corpus now exist.
- **STIM-1:** source accepted. CDR adapter 1.4 and
  `forensics.time-policy/v1` fail safely on unresolved date order and unknown
  timezone; independent goldens cover declared, confirmed, explicit-offset,
  profile-defaulted, ambiguous, and cross-midnight cases.
- **STIM-2:** source accepted. One shared schema/mapping registry, versioned
  dynamic attributes, quality/readiness, raw/canonical provenance, representative
  family coverage and the bounded Analyst Data surface are implemented without
  a parallel store or migration.
- **STIM-3:** source and runtime accepted; closed. The derived 66-operation ledger contains five
  certified/queryable, 50 explicitly limited, and 11 engineering-only
  operations. Structured scope excludes the KB-only evidence operation and is
  65 total: four certified, 50 limited, 11 engineering-only. Silently
  uncertified ordinary-user exposure is zero.
- **STIM-4:** source and runtime accepted; closed. Exact two-to-eight-source CDR comparison and
  typed, time-valid cross-family relationship primitives pass independent
  goldens, anti-correlation, provenance, presentation, scoped-SQL, and bounded
  performance tests. Broad public cross-family routes remain limited.
- **Runtime correction:** exact retained source membership now rejects before
  analytical SQL with stable `invalid_source_membership` semantics. Cross-
  family retrieval now uses one bounded `record_entities` candidate query,
  one non-CDR semantic fallback expansion, and Go compilation of coverage,
  relations and citations. The 5,000-row compiler measured 63.3869 ms. The
  multi-CDR conflict key now uses the accepted canonical service class, and both
  cross-family candidate branches normalize record type to text for their UNION.
- **Acceptance data:** seven small synthetic files plus one hand-authored oracle
  now cover the previously unexercisable multi-CDR, IPDR↔subscriber and
  distinct-source ANPR cases in retained state: 21 total rows, 20 accepted, one
  duplicate, zero rejected.
- **STIM-5/STIM-6:** source and runtime accepted; closed. Multilingual query
  semantics, bounded Fact Packets, validator/fallback, proof presentation,
  responsive tables, citations, limitations and Urdu RTL passed their source,
  runtime and browser gates.
- **STIM-7:** source and runtime accepted; closed. The final matrix passed
  46 structured goldens, Go/vet/presentation suites, 14 multilingual runtime
  checks, all 65 operations, 11 retained-data oracles, the production UI build,
  13 controlled browser cases and deployed responsive inspection.
- **Open severity:** P0=0, P1=0, P2=4, P3=0. All remaining P2 debt is
  non-blocking and recorded in the single maturity matrix.
- **Exact next phase/action:** stop at Post-STIM architecture reconciliation.
  Prepare the governed-runtime-query-intelligence design and threat/acceptance
  boundary; do not implement generalized runtime SQL/query generation.
- **Runtime boundary:** source and runtime accepted;
  `DeploymentNeeded=NO`, `AcceptanceDataIngestNeeded=NO`,
  `DatabaseMigrationNeeded=NO`. Retained acceptance evidence changed only through
  the approved normal-path ingest.

## Post-STIM reconciliation priority

Governed Runtime Query Intelligence is the immediate design priority. It must
compose certified operations or explicitly allowlisted typed analytical
primitives, preserve exact scope and provenance, enforce authorization and
cost/row/time budgets, and expose plan/parameter/proof telemetry with stable
fail-closed semantics. Unconstrained model-authored SQL is outside the accepted
boundary. Documents/RAG and all media families remain separately gated and do
not start under this handoff.

- **APF-3 final gate:** CLOSED — 18/18 browser cases passed with zero P0/P1;
  source/runtime parity, health and security scope passed.
- **Current runtime state:** `APF-3BrowserAcceptance = ACCEPTED`,
  `APF-3.7RuntimeStatus = ACCEPTED`, `APF-3RuntimeStatus = ACCEPTED`,
  `APF-3FinalStatus = CLOSED`, and `DeploymentNeeded = NO`.
- **Preservation:** worker, PostgreSQL, NATS, named volumes, retained evidence,
  KB data, models, profiles and rollback images remained preserved.
- **Accepted handoff boundary:** `NextProgram = STIM`, `NextPhase = STIM-0`.
  This historical handoff has now been consumed by the controlling STIM update.

- **Final source gate:** `APF-3SourceStatus = ACCEPTED` and
  `APF-3.7SourceStatus = ACCEPTED`. Bounded two/three-step composition is
  implemented with typed dependencies/bindings and fail-closed governance.
- **Historical runtime boundary:** APF-3.4–3.6 was live accepted before the
  APF-3.7 activation. That activation passed its infrastructure checks but
  failed the later browser correctness gate.
- **Query ledger:** 156 current occurrences resolve to 144 unique accepted
  cases and 12 duplicate occurrences. All 156 pass routing, parameters and
  semantic equivalence in the derived ledger.
- **Risk-tier certification:** the executable registry is 66/66. Five
  operations are certified/queryable, 50 are bounded/limited, and 11 are
  bounded/engineering-only. Promotion debt remains visible at the individual
  operation level; no limited operation is mislabeled as certified. Case 18 capability-guard latency and
  Case 15 bounded-synthesis latency are deferred P2; representative-citation
  labeling/version-field clarity is deferred P3. None reopens APF-3.
- **Final deployment:** the 2026-08-19 guarded activation passed with API 4.2
  seconds and LocalAI/UI 97.1 seconds, followed by the accepted full replay.

- **APF-3.1-3.3 source accepted:** NexusAI's forensic service remains the
  orchestration policy owner and LocalAI remains a bounded runtime helper. One
  derived projection resolves all 66 executable query operations; the broader
  platform registry retains 50 historical descriptors and derives 30 missing
  query descriptors plus the STIM-4 comparison for 81 total. Existing Ask, SSE, typed response, citation
  and History APIs are preserved.
  See `docs/design/nexusai-apf3-unified-query-intelligence-architecture.md`.
- APF-3.1-3.7 deployment/runtime acceptance is complete and overall APF-3 is
  closed. Do not reopen it for deferred latency, polish or new capability work.

- **APF-2 runtime acceptance complete:** authoritative catalog accounting now
  resolves by evidence ID; the retained source shows 4 verified records, 5
  input, 1 duplicate and 0 rejected, while unavailable counts remain unknown.
  The single authorized Ask run created completed deterministic analysis
  `c540c875-0cd7-410d-bd6b-3f2be87b0336`, returned 4 exact CDR results, cited
  and reopened the retained source, advanced History 50 to 51, and passed
  navigation-only Continue in Ask. Focused Playwright is 10/10 and the
  677-module build plus responsive dark/light preview pass. No rebuild/deploy,
  reprocess, retry, second upload or other retained mutation occurred.
- The bounded APF-3 architecture/dependency reconciliation described here is
  complete; production implementation remains not started.

- **APF-UX-1 source acceptance:** Home, Data, Ask and History now implement the
  approved Analyst Experience V2. Suggested questions are capability-derived,
  source and history workflows are filterable/reopenable, and real-data visual
  QA passed at 390/820/1024/1440 in both themes with no overflow or console
  errors. The 21-test focused suite, scoped lint and production build pass.
  Docker was not rebuilt; this source slice is reviewed through Vite.
- **APF-1 source acceptance:** the separate `/analyst` shell now provides Home,
  read-only Data, governed Ask NexusAI and History over the authoritative
  workspace registry and real backend contracts. Focused lint, build, four
  Playwright flows, desktop/mobile inspection and a real deterministic CDR query
  pass. The running deployment is unchanged.
- **APF-2 source acceptance:** the permanent Data journey now reuses existing
  evidence admission, classification, custody, queue, catalog pagination,
  capability and read-only reprocess-plan contracts. Multi-file intake is
  bounded to two concurrent registrations and reports per-file progress,
  duplicates and partial failure. Source actions are capability-derived and
  available only when the source is Ready. Failed sources explain preservation
  and approval-gated immutable reprocess; no fake Retry exists. Scoped lint has
  zero errors, the 677-module build passes, focused portal Playwright is 9/9,
  and real dark/light browser QA passes without a retained upload.
- **APF-2 retained-runtime progress:** the single approved synthetic upload is
  complete and preserved as evidence `4320772b-febe-4af2-b9e3-92cea114da0b`,
  version `ad4b97c9-54eb-4657-afa7-5f06f52ba695` and completed job/run
  `605b53d8-195f-4dde-aae6-081fbef6b500`. It produced 4 canonical CDR rows,
  one duplicate, a KB asset and a valid 3-event custody chain. Counts are now
  11 sources, 10 Ready, 1 Failed and 9,278 accepted records.
- **APF-2 runtime blocker:** the modal-to-source-detail path displays
  `0 verified rows` because the detail item omits `accepted_rows` and the modal
  callback supplies no catalog accounting. The authoritative value is 4. The
  run stopped before Ask/History under the approved failure rule. Fix and test
  this bounded UI truth defect, then resume from the preserved evidence without
  another upload. APF-3 does not start until APF-2 passes. No Docker rebuild is
  required.
- **Architecture contract:** follow
  `docs/design/nexusai-analyst-portal-product-architecture.md` for the two-surface
  product boundary, reuse matrices, family contract and manual acceptance flow.

- **R8.4-A–F feasibility correction:** physical T2-P tooling remains source
  accepted while its evidence is `deferred_environment_unavailable`. A separate
  96-image T2-V pack is generated, validated and sealed (56 development, 20
  validation, 20 holdout), but no candidate has passed it. Boundary A explicitly
  requires T2-V plus suitable licensed real-image T3-D/T3-O and all unchanged
  gates. CCPD is the primary real T3 authorization target; Artificial Mercosur
  is supplemental mixed-real, and neither is approved for download.

- **R8 two-boundary gate:** Boundary A separates production-disabled pipeline
  eligibility from Boundary B production promotion. T2-P tooling is preserved
  but optional/deferred for Boundary A only when accepted T2-V and real T3 both
  exist. P-LPCD is not approved for
  download because its official records conflict on CC BY versus CC BY-NC and
  on 36 versus 37 classes. Offline OCR preprocessing did not improve accuracy;
  detector 640-long-side improved precision to 0.85 but still fails the
  unchanged 0.95 precision/0.95 recall gate. Resolve exact T3 artifact/privacy
  and model-acquisition gates before R8.5; preserve T4 for Boundary B.

- **R6 closure:** the guarded R6.6 rebuild and live acceptance passed with all
  catalog/corpus checks, zero mutating gate requests and preserved rollback
  assets.
- **R7.1 source acceptance:** the exact time-valid tower join now exposes total
  candidates, timestamp-eligible candidates and matched, overlapping-ambiguous,
  no-reference or outside-validity outcomes. Aggregate metrics are part of the
  existing enterprise response.
- **R7.2 source acceptance:** CDR adapter 1.2 preserves legacy output while
  adding raw/canonical Pakistan party roles, service/sentinel classification,
  timezone provenance and exercised synthetic goldens.
- **R7.3 source acceptance:** subscriber adapter 1.2 separates subscriber,
  SIM/ICCID/IMSI, device/IMEI and service/provider roles with explicit validity
  and observation-only association semantics.
- **R7.4 source acceptance:** tower adapter 1.2 preserves provider/site/sector
  history, validity basis, datum/uncertainty provenance and explicit overlap
  review without claiming RF presence or movement.
- **R7.5 source acceptance:** governed telecom natural-language variants,
  missing-input clarification and bounded no-result behavior pass while the
  65-operation public catalog remains stable.
- **R7.6/R7.7 acceptance:** synchronized telecom table/timeline/map/evidence
  presentation and all six version 1.2 specialist integrations are accepted.
- **R7.9/R7.10 acceptance:** bounds, scope, source-locator checks and the guarded
  deployed desktop/mobile workflow pass; rollback images and volumes remain
  preserved.
- **R7.8/R7 closure:** the supplied protected CDR passed privacy-safe read-only
  normalization, provenance, classification and performance acceptance with no
  retained ingest. R7 is complete.
- **R8.1 acceptance:** reusable evidence, ANPR, model-governance, API and UI
  boundaries plus the hostile-image/privacy threat model are reconciled.
- **R8.2 source acceptance:** bounded immutable image admission now records
  signature, dimensions, orientation, color/pixel policy and explicit review
  state; the Evidence desk presents that truth without OCR/detection claims.
- **R8.3 platform source acceptance:** typed original-pixel candidates,
  deterministic bounded NMS, exact lossless crop/hash lineage, localization
  metrics and a responsive read-only overlay are implemented.
- **R8 evidence and expanded gate:** one T0-T4 registry and just-in-time program
  now govern data readiness. The deterministic 29-fixture T0/T1 pack covers
  14/14 classes; its offline run rejects the text detector at 0.6071 precision
  and every OCR candidate below accuracy/CER thresholds. T2 protocol and T3
  research are ready without downloads; T4 remains pending. No production role
  is assigned and R8.5 has not started.

## R7 sequence

| Slice | Outcome | Current state |
| --- | --- | --- |
| R7.1 | Capability reconciliation and explicit time-valid join outcomes | source accepted |
| R7.2 | Pakistan CDR normalization contract and synthetic goldens | source accepted |
| R7.3 | Subscriber/device/service semantics | source accepted |
| R7.4 | Provider/sector history and deep tower validity | source accepted |
| R7.5 | Telecom operations and realistic query variants | source accepted |
| R7.6 | Uncertainty-aware map/timeline family UI | source and live accepted |
| R7.7 | Specialist and Ask NexusAI rich integration | source and live accepted |
| R7.8 | Protected real-CDR validation | read-only accepted |
| R7.9 | Performance, security and provenance acceptance | source accepted |
| R7.10 | Guarded runtime, manual test pack and team-lead closure | live accepted |

The complete R8.1-R8.10 execution map is maintained in
`docs/design/nexusai-r8-anpr-image-ocr-intelligence-contract.md`.

## Approval boundary

Source-only documentation, bounded active-phase code, focused tests, lint and
local preview may proceed. Docker rebuild/deploy, database migration/backfill,
evidence upload/reprocess, collection cleanup, model/backend download, retained
configuration change, staging, commit, push and publication require explicit
approval.

## 2026-08-25 MMV-2 enterprise presentation closure gate

- Final two MMV-2 P1 source fixes are accepted: authoritative positive rows
  remain positive through summary/UI/History, and completed-zero video ANPR is
  distinct from filter miss, unprocessed, failed or unavailable.
- Exact activation scope is `forensic-records-api` plus LocalAI/UI `api` only.
  Worker, PostgreSQL, NATS, models, volumes, retained evidence and schema are
  protected. Deployment remains approval-gated behind 6 GiB free physical RAM.
- MMV-2 remains active until live query, citation, History and four-viewport
  certification passes. MMV-3 must not start automatically.

## 2026-08-28 NX-A1 closure override

The later NX-1, NX-UX1 and live-activation checkpoint supersedes the historical
MMV-2 active marker above. NX-UX1 is closed and live; NX-A1 is source-complete.
NX-A1 extends the accepted APF-3 query/capability/plan/execution/Fact Packet/
enterprise-response architecture and passed its bounded source certification.
No runtime deployment or retained-state mutation occurred. The exact next
phase is NX-B1, not started; MMV-3 and NX-B2 remain out of scope.
+## 2026-09-05 NX-B2.1D schema-bound development v2 frozen

Development attempt 1 stopped after four model calls and is
`INVALID_INCOMPLETE`; its v1 corpus must not be resumed. All four responses
finished normally and parsed, proving the 512-token budget correction for the
observed sample, but the permissive schema allowed zero-valued unused media-time
fields and non-text `text_query` data that the production validator rejected.

The production response schema now binds model-supplied parameters to
deterministic values present in the request. Fresh development freeze
`nxb21d-q4-production-interface-schema-bound-dev-r2` has identity SHA-256
`ae5ff08e090a4c23b10f6dca2d1e6a8c4549ab13059fc51dcf34d70926b0c4db`
and fresh corpus SHA-256
`3eb69ae6284e38cfc4e3f4c68514d0ad2ef5ed5081908faf389fc4a60122dbb0`.
Source tests, the 24-case deterministic oracle, and 70 framework assertions
pass. The v2 runner seals an early failure receipt after four consecutive model
failures and unloads Q4. D remains OPEN, qualification is not frozen, and
activation is BLOCKED until this development gate passes and is reviewed.

## 2026-09-08 NX-B2.1D final controlled local Qwen3-8B attempt prepared

The last authorized local Qwen3-8B development gate is frozen but unexecuted as
`nxb21d-qwen3-8b-final-local-selection12-20260908T100500Z`. It preserves the
exact 5,027,783,488-byte artifact
`d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785`,
the exact low-memory profile
`672f8e2c29f162cdc74a8a8dfe7129955436c771db454b0a3ebb20a3bf9b560a`,
and `forensics.hybrid-decision/v2`. The completely new 12-case corpus is
`1fdd47b3112e910c9a9850a0cdca16db2240a12e8a9209676b0e60035d4f2d6d`;
freshness passed across 2,035 files with zero question, target-value, or
normalized-literal overlap, and production source admission passed 12/12.

The operator runner uses the sealed prebuilt evaluator
`130ffac7fcfeb01c57b713cf171dc27d4b9f54a0fde5a7b69796a1dc9875588d`.
It creates no consumption marker until immediately before the first actual
model request, requires three consecutive >=6 GiB preload samples at five-second
intervals, preserves the >=4 GiB pre-inference loaded-state floor, records
initial and settled loaded RAM separately, and preserves backend/container/host
memory observations. The prebuilt no-inference Ginkgo admission path measured
0.621 seconds; compilation is absent from the live gate. The prior consumed
failure remains a resource-admission failure with insufficient model-quality
evidence and immutable receipt SHA
`346a519e8e83ee18ade6ffb9a3a740ed7203f5fe07d5b0c102fcc6d32330f107`.

Current state: `LOCAL_QWEN3_8B_FINAL_ATTEMPT=PREPARED_NOT_EXECUTED`,
`QWEN3_8B_MODEL_QUALITY=INSUFFICIENT_EVIDENCE`,
`FINAL_HOLDOUT=NOT_CREATED`, `FINAL_QUALIFICATION=NOT_STARTED`,
`NX-B2.1D=OPEN`, and `D_ACTIVATION=BLOCKED`. Execute only after closing
Codex/ChatGPT, Chrome, VS Code/IDE, and other nonessential applications while
keeping Docker Desktop running:

`powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\nxb21d-qwen3-8b-lowmem-selection\final-local-selection12\run_final_local_selection12_development.ps1`

## 2026-09-17 frontend source-truth correction

The bounded active-React-UI correction is source verified. Add Data now uses a
minimal automatic flow with server capability truth, unknown-time preservation,
real cancellation and retry; Evidence has a visible responsive sticky toolbar;
one authoritative modality resolver covers audio and the other supported
classes; and History separates stored-result reopen from non-executing prompt
reuse. Acceptance is 49 focused unit tests, 31 Analyst Portal browser tests,
20 Case Workspace browser tests, a 689-module production build, and zero new
focused lint warnings.

The fresh UI-only candidate is
`reports/nxb21/nxb21d-source-truth-ui-activation-20260917/`. Its preflight
passed, but the authorized activation stopped before mutation because twelve
RAM samples remained 4.57–4.836 GiB against the unchanged 6 GiB gate. No
backend/query/semantic source, model, database, retained evidence or service
state changed. Exact next action is to free RAM and rerun
`scripts/activate_nexusai_source_truth_ui_20260917.ps1` without recovery
switches. This does not change the independent NX-B2.1D query/model status.
