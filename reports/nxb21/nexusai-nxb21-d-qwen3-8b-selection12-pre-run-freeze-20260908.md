# NX-B2.1D Qwen3 8B model-selection development freeze

Date: 2026-09-08  
Phase: `NX-B2.1D`  
Gate: `QWEN3_8B_DEVELOPMENT_GATE=FROZEN_NOT_EXECUTED`  
Boundary: development model selection only; no live inference, qualification,
production activation, retained-data mutation, or accepted-profile replacement.

## Outcome

The exact owner-authorized Qwen artifact was acquired and independently checked
before registration. The isolated development profile is registered and remains
unloaded. The fresh standalone 12-case selection gate is frozen, source-admitted,
freshness-validated, and `-ValidateOnly` validated. Codex did not dispatch the
gate. The prior Q4 Decision8 result remains immutable at 5/8 and is not rerun.

Current authority remains:

```text
Q4_FULL_PLAN_ROLE=RETIRED
Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE
NX-B2.1D=OPEN
D_STATUS=OPEN
ACTIVATION=BLOCKED
```

## Exact model artifact and acquisition audit

| Field | Frozen value |
|---|---|
| Repository | `Qwen/Qwen3-8B-GGUF` |
| Revision | `7c41481f57cb95916b40956ab2f0b139b296d974` |
| File | `Qwen3-8B-Q4_K_M.gguf` |
| Host path | `local-acceptance-models/nxb21-d/challengers/qwen3-8b-q4km/Qwen3-8B-Q4_K_M.gguf` |
| Container path | `/models/Qwen3-8B-Q4_K_M.gguf` |
| Expected bytes | `5027783488` |
| Actual bytes | `5027783488` |
| Expected SHA-256 | `d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785` |
| Actual SHA-256, host and container | `d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785` |
| Transfer boundary | `2026-09-08T04:37:02.5970129Z` to `2026-09-08T05:07:29.0702373Z` |
| Tool | `curl.exe --location --continue-at - --fail --retry 5 --retry-all-errors` |
| Audit receipt | `local-acceptance-models/nxb21-d/challengers/qwen3-8b-q4km/acquisition-receipt-v1.json` |
| Audit receipt SHA-256 | `bd822fab9a637eec707561d9d92d6d4505fd2cfef32b156f896801a4d1f3b477` |

Mismatch policy is fail closed. No alternate artifact is permitted by this
freeze.

## Isolated LocalAI profile

Model ID: `qwen3-8b-q4km-nxb21d-selection-dev`  
Profile: `local-acceptance-models/nxb21-d/challengers/qwen3-8b-q4km/qwen3-8b-q4km-nxb21d-selection-dev.yaml`  
Profile SHA-256: `c3d7b799fdc9fb0acf39b4c694513f1eec608146dc44b1e5baf9797e40ab0afe`

Explicit settings are `llama-cpp`, CPU-only `gpu_layers: 0`,
`context_size: 4096`, `threads: 8`, `parallel: 1`, `temperature: 0`,
`top_k: 20`, `top_p: 0.95`, `min_p: 0`, `repeat_penalty: 1`, `f16: true`,
`mmap: true`, tokenizer chat template enabled, and the same stopword convention
as the accepted local Qwen profiles. `n_batch` is deliberately not overridden;
the backend default is authoritative and is recorded as `n_batch=null` /
`n_batch_source=backend_default` in the operator config. The accepted synthesis
profile was not changed.

The model configuration reload succeeded. `/v1/models` lists the isolated ID;
`/system` confirms it is not loaded. No inference occurred during registration.

## Runtime integrity baseline

The runner binds exact container IDs, image IDs, restart counts, health, zero
active jobs, retained tuple `67|67|80|22510|863|66`, and activity count `343` in
`scripts/nxb21d-qwen3-8b-selection/runtime-baseline.json` at SHA-256
`a7e7819d0f500730c880580fe76e331bfa7da22505ff09218e3c5db62c7318bc`.
The Investigation Workspace remains live on API image
`sha256:bed2eec364ca8c65151625685f79f5ab7a606a8223af88d1696504751b4a03a1`.
Direct review of `http://localhost:8080/analyst` passed with the sealed premium
shell, Ask composer, answer/citation states, clarification state, and empty
states intact. The exact served index hash remains
`780fdefe6522e65371ff092004df933a99a1daaf6b6191e453b30cde43bec963`.

The standalone runner requires at least 6 GiB available before the first call
and 4 GiB before every later call. It stops safely on a failed resource check.

## One-field residual contract and schema proof

The production contract remains `forensics.hybrid-decision/v2`. The model owns
exactly one wire field:

```json
{"decision":"<one server-issued tuple ID or CLARIFY code>"}
```

The server owns facts, candidates, authorization, scope, identifiers, literal
text, dates, limits, representation, final plan, and audit projection. The
schema requires `decision`, sets `additionalProperties:false`, and enumerates
only server-issued tuple IDs plus `CLARIFY:AMBIGUOUS_INTENT` and
`CLARIFY:INSUFFICIENT_FACTS`.

The focused production test passed through LocalAI's real
`JSONFunctionStructure` → JSON-schema grammar conversion. Every allowed enum
survived conversion and decoding. Unknown, null, empty, duplicate-key, old
five-field shape, and extra authority-field outputs were rejected. The frozen
schema bindings for all 12 cases match the live production builder exactly.

## Fresh 12-case selection corpus

Corpus: `scripts/nxb21d-qwen3-8b-selection/qwen3-8b-selection-12case-corpus-v1.json`  
Corpus SHA-256: `6376e5d18352edd6319562e9799d773bedb67f54bbedc901f06256dd53b226cd`  
Bindings SHA-256: `a741d0a0e39c0b257f29532d8e968a4766082abcd62ef0d39e5d856d4583e749`  
Freshness receipt SHA-256: `b98e290f43ded0d15cebf623b16798fe0d869d4be2a4623f75900a0d0ed19d1e`

Freshness scanned 1,124 historical JSON, JSONL, and text files and found zero
exact question overlap and zero normalized held-out value overlap. There are
three cases per language (`en`, `ur`, `roman_ur`, `mixed`), with one resolved,
one ambiguous, and one insufficient-facts oracle in every language. All twelve
are eligible residual cases and require exactly one real model call.

| Language | Resolved | Ambiguous | Insufficient |
|---|---|---|---|
| English | `sel12-en-resolved` | `sel12-en-ambiguous` | `sel12-en-insufficient` |
| Urdu | `sel12-ur-resolved` | `sel12-ur-ambiguous` | `sel12-ur-insufficient` |
| Roman Urdu | `sel12-roman-ur-resolved` | `sel12-roman-ur-ambiguous` | `sel12-roman-ur-insufficient` |
| Mixed | `sel12-mixed-resolved` | `sel12-mixed-ambiguous` | `sel12-mixed-insufficient` |

Resolved coverage spans IPDR aggregate selection, CDR time-activity aggregate,
knowledge semantic retrieval, and failed-login filtering. Clarification cases
exercise competing reachable tuples or missing required comparison facts.

## Mandatory advancement threshold

The model may advance only if one immutable run proves all of the following:

- 12/12 cases passed, 12/12 real model calls, and 12/12 raw decisions correct;
- English, Urdu, Roman Urdu, and mixed are each 3/3;
- resolved, ambiguous, and insufficient are each 4/4;
- every request and response is strict UTF-8 without BOM, HTTP-successful,
  parseable, schema-valid, and complete with `finish_reason=stop`;
- deterministic facts and candidate tuples are byte-stable across selection;
- three-layer audit and final-plan checks pass for every case;
- runtime/container/restart/health/active-job/retained-state checks pass before
  and after, and the runner unloads only its owned model.

Anything less is `DEVELOPMENT_FAIL_DO_NOT_ADVANCE`. A 12/12 pass would make the
model eligible to become the replacement qualification candidate, but would
not close D, create or consume an independent holdout, or authorize activation.

## Runner, evidence, and freeze identity

Runner: `scripts/nxb21d-qwen3-8b-selection/run_nxb21d_qwen3_8b_selection12_development.ps1`  
Operator config: `scripts/nxb21d-qwen3-8b-selection/selection12_config.json`  
Operator config SHA-256: `ae400506ea64bdd8434b950bd236fb00d22561a9ceedbc44fcb991960a98cd0b`  
Freeze ID: `nxb21d-qwen3-8b-selection12-20260908T051432Z`  
Freeze identity: `e0aa163fdf62e12ee3b416bf6fa05061355d16f2ce616566578673ae926e9771`  
Completion budget: 512 tokens.

The runner uses a create-new one-shot dispatch lock. For every case it stores
the exact request bytes, raw response bytes, strict decoded response, request
and response hashes, latency, retry count (fixed at zero), completion metadata,
decision validation, deterministic facts/candidates/final plan, and the
three-layer audit. It stops on resource, runtime, transport, parse, schema,
completion, or final-plan failure, then attempts owned-model unload and runtime
postcheck. The aggregate receipt is sealed once with a SHA-256 sidecar.

`test_selection12_framework.ps1` passed PowerShell syntax checks, source-tamper
and identity-tamper rejection, frozen corpus validation, and the runner's
`-ValidateOnly` path. `live_inference=false` throughout preparation.

## Exact standalone command

Run once, in a fresh standalone PowerShell from the repository root:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\nxb21d-qwen3-8b-selection\run_nxb21d_qwen3_8b_selection12_development.ps1
```

Do not run the historical Decision8 command. After the standalone command
finishes, preserve the run directory and reopen Codex with its terminal output
and sealed `selection12-development-summary-v1.json` receipt. Do not retry an
integrity, transport, schema, completion, or runtime failure; do not create a
qualification holdout and do not activate.

```text
PREMIUM_UI=LIVE_VERIFIED
Q4_FULL_PLAN_ROLE=RETIRED
Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE
QWEN3_8B_ARTIFACT=VERIFIED
QWEN3_8B_PROFILE=FROZEN
QWEN3_8B_DEVELOPMENT_GATE=FROZEN_NOT_EXECUTED
QWEN3_8B_DEVELOPMENT_RESULT=NOT_RUN
FINAL_D_CANDIDATE=NOT_YET
FINAL_HOLDOUT=NOT_CREATED
FINAL_QUALIFICATION=NOT_STARTED
NX-B2.1D=OPEN
D_STATUS=OPEN
ACTIVATION=BLOCKED
EXACT_NEXT_ACTION=Run the frozen standalone Qwen3 8B selection12 command once, preserve its sealed receipt, and return to Codex for adjudication; do not rerun Decision8, create a qualification holdout, or activate.
```
