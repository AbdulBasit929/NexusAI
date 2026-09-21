# NX-B2.1D Decision8 adjudication and residual-model selection

Date: 2026-09-07  
Branch: `codex/forensic-hybrid-checkpoint-20260723`  
HEAD: `40717b83510c08db25dc26b9d6674bf46db363ac`  
Phase: `NX-B2.1D`  
Decision: `Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE`

## Executive decision

The production design remains deterministic-first hybrid planning under
`forensics.hybrid-decision/v2`. The Q4_K_M model is retired from both the old
full-plan role and the bounded residual-decision role. Decision8 completed all
eight one-shot development cases, made exactly eight model calls, preserved
runtime integrity, and scored 5/8. Because the frozen acceptance threshold was
8/8 and every miss was a genuine decision error, Q4 is not a final-D
qualification candidate.

No consumed corpus may be rerun. No replacement holdout exists, no production
deployment or activation is authorized, and no runtime or retained evidence was
changed during this adjudication.

Authoritative immutable evidence:

- Run: `local-acceptance-models/nxb21-d/hybrid-decision-development-runs/run-20260906T161126132Z/`
- Summary: `decision8-development-summary-v1.json`
- Summary SHA-256: `0680fe8c706f2eda40b1352a22103a03d28f0e395f6b6835790e7d174dffce95`
- One-shot lock: `local-acceptance-models/nxb21-d/hybrid-decision-development-runs/development-dispatched.lock`
- Contract: `forensics.hybrid-decision/v2`, one mutually exclusive `decision` field
- Result: 8/8 completed, 5/8 passed; English 2/2, Urdu 1/2, Roman Urdu 1/2,
  mixed 1/2; non-English 3/6
- Runtime postcheck: PASS; evaluated model unloaded at run end

## Exact failure adjudication

| Case | Expected | Actual | Primary classification | Adjudication |
|---|---|---|---|---|
| `dec8-ur-clarify` | `CLARIFY:INSUFFICIENT_FACTS` | `CLARIFY:AMBIGUOUS_INTENT` | `MODEL_CLARIFICATION_WEAKNESS` | The Urdu request identifies one side of a comparison and explicitly says the other number is unavailable. The fact packet contains exactly `03258740692`. The model returned schema-valid JSON but chose the wrong clarification class. |
| `dec8-roman_ur-resolved` | `logs.failures/FILTER/workspace`, target `operator-z684` | `CLARIFY:AMBIGUOUS_INTENT` | `MODEL_MULTILINGUAL_INTENT_WEAKNESS` | The request positively asks for failed-login entries and explicitly negates subscriber detail. Both candidate families were admitted, and the expected tuple was reachable. The model abstained instead of resolving the Roman Urdu contrast. |
| `dec8-mixed-resolved` | `knowledge.semantic/SEMANTIC_RETRIEVAL/workspace`, target `192.0.2.186` | `CLARIFY:AMBIGUOUS_INTENT` | `MODEL_MULTILINGUAL_INTENT_WEAKNESS` | The mixed-language request positively asks what documents say and explicitly negates network totals. The expected tuple was reachable in the unchanged candidate set. The model abstained instead of resolving the contrast. |

All three responses were HTTP-successful, parseable, schema-valid one-field
decisions with `finish_reason=stop`. Their prompts, fact packets, and candidate
lists were not mutated. Source admission had independently validated every
oracle before dispatch. Therefore none is a schema, transport, truncation,
oracle, source-admission, or candidate-generation defect.

The two resolved misses are also tuple-selection failures at the symptom level,
but their primary cause is multilingual intent handling: each question contains
a positive request plus an explicit negative contrast that the model failed to
use. Adding wording-specific deterministic rules after seeing these cases would
absorb a consumed residual benchmark into source behavior, so it is rejected as
a remediation path.

## Q4 disposition

`Q4_FULL_PLAN_ROLE=RETIRED` remains unchanged. Decision8 changes the narrower
assessment from promising to
`Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE`. The decisive facts are:

- 5/8 overall against a mandatory 8/8 development threshold.
- 3/6 across Urdu, Roman Urdu, and mixed-language cases.
- Three valid responses made materially wrong decisions.
- The failure pattern is semantic, so more Q4 prompt tuning is not an
  evidence-preserving shortest path.

The Decision8 call latencies ranged from 7.090 to 20.694 seconds, with an
approximately 14.335-second median. During the run, sampled free host RAM fell
as low as 3.742 GiB and API memory reached 3562.496 MiB. These measurements are
development evidence only, not a production capacity claim.

## Installed candidate inventory

The runtime currently has only two plausible chat/planning artifacts, both the
same Qwen3 4B Instruct model family:

| Model/profile | Artifact | Size | Evidence-based disposition |
|---|---|---:|---|
| `qwen3-4b-instruct-2507-q4km-nxb21d-dev` | `Qwen_Qwen3-4B-Instruct-2507-Q4_K_M.gguf` | 2,497,280,736 bytes | Decision8 5/8; insufficient for residual role |
| `qwen_qwen3-4b-instruct-2507` | `Qwen_Qwen3-4B-Instruct-2507-Q8_0.gguf` | 4,280,405,216 bytes | Existing full-plan run is invalid for intrinsic comparison: 0/81 schema-valid under a mismatched/retired interface, severe memory pressure, and 42.814-second median latency; the user has retired this path |

The remaining registered models are embedding, face, and speech models and are
not planner candidates. There is no installed candidate that is both
independently promising and admissible for another development benchmark.

At the read-only 2026-09-07 inspection, five services were running; API,
worker, NATS, and PostgreSQL were healthy, while the forensic-records API was
running without a healthcheck. The observed retained tuple was
`65|65|78|22508|843|62`, activity count was 328, and active jobs were zero.
Those values are observations only and were not normalized or modified.

## Selected acquisition candidate

The shortest technically credible replacement route is the official
`Qwen/Qwen3-8B-GGUF` release, pinned to the last clean upstream revision before
the repository's later Q4 filename/content change:

- Repository: `Qwen/Qwen3-8B-GGUF`
- Revision: `7c41481f57cb95916b40956ab2f0b139b296d974`
- File: `Qwen3-8B-Q4_K_M.gguf`
- Exact size: 5,027,783,488 bytes
- Expected SHA-256: `d98cdcbd03e17ce47681435b5150e34c1417f50b5c0019dd560e4882c5745785`
- License: Apache-2.0
- Upstream evidence: [pinned tree](https://huggingface.co/Qwen/Qwen3-8B-GGUF/tree/7c41481f57cb95916b40956ab2f0b139b296d974), [exact file](https://huggingface.co/Qwen/Qwen3-8B-GGUF/blob/7c41481f57cb95916b40956ab2f0b139b296d974/Qwen3-8B-Q4_K_M.gguf), [Qwen3 model card](https://huggingface.co/Qwen/Qwen3-8B), and [official Qwen3 announcement](https://qwenlm.github.io/blog/qwen3/)

This is a candidate, not an approved model. Its larger dense 8B capacity and
official multilingual coverage make it a more credible residual selector than
another quantization of the failed 4B model. CPU execution is plausible but
tight on this 15.7-GiB host: a low-memory, single-heavy-model session is
required, with the current Q8 model unloaded and at least about 7 GiB free
before loading so the existing 4-GiB loaded-runtime safety floor has a chance to
hold. A rough 20-45 second per-case CPU latency is planning guidance, not a
measurement or guarantee.

No download has been authorized or performed. If the owner approves the exact
artifact, it must land only in ignored local acceptance storage, be byte-size
and SHA-256 verified before use, and remain isolated from production profiles.

## Shortest route to final D

1. Obtain explicit approval for the exact pinned 5,027,783,488-byte artifact.
2. Download to ignored acceptance storage and verify the exact SHA-256; stop on
   any mismatch.
3. Create an isolated LocalAI profile for Qwen3-8B Q4_K_M using the existing
   one-field Decision8 contract. Do not deploy it to production.
4. Freeze a fresh, non-overlapping 12-case model-selection development corpus:
   three per language, balanced between resolved decisions and clarification,
   with all deterministic facts, candidates, and oracles source-admitted before
   dispatch.
5. Run once under the memory gates. A candidate must complete 12/12, produce
   valid one-field decisions, pass all semantic/oracle checks, preserve runtime
   integrity, and unload cleanly. Any failure retires that corpus and blocks
   qualification.
6. Only after a 12/12 result, nominate the 8B profile as
   `FINAL_D_CANDIDATE`, create and freeze a new independent qualification
   holdout, and request separate authorization to execute it.
7. Activation remains a later, separately authorized action after qualification
   and product/runtime acceptance. It is not implied by model selection.

## UI interruption disposition

The unfinished Ask-page edits are `INCOMPLETE_BUT_COMPILE_SAFE`: the focused
lint run produced zero errors (warnings only), and the React production build
completed successfully. The edits are not required for D's residual-model
decision and must be excluded from any D frozen production image or deployment
manifest. They remain user-owned worktree changes.

## Gate state

`FINAL_DEVELOPMENT=NOT_STARTED_APPROVAL_REQUIRED`  
`FINAL_D_CANDIDATE=NOT_YET`  
`FINAL_HOLDOUT=NOT_CREATED`  
`FINAL_QUALIFICATION=NOT_STARTED`  
`ACTIVATION=BLOCKED`  
`NX-B2.1D=OPEN`

Exact next action: request owner approval to download only the pinned
`Qwen3-8B-Q4_K_M.gguf` artifact above into ignored local acceptance storage and
prepare its fresh 12-case model-selection development benchmark. Do not create
a qualification holdout, deploy, activate, or rerun any consumed corpus.
