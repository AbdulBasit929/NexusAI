# NX-B2.1D Qwen3 8B selection12 attempt 1 — resource failure

Date: 2026-09-08  
Phase: `NX-B2.1D`  
Disposition: `INCOMPLETE_RESOURCE_FAILURE_DO_NOT_RERUN`  
Model-quality decision: `INSUFFICIENT_EVIDENCE`

## Authoritative outcome

The one-shot frozen selection12 gate was dispatched from standalone PowerShell.
It did not complete and cannot be used to qualify or reject Qwen3 8B on model
quality. The immutable aggregate receipt records two result entries, one actual
model call, one correct pass, and one pre-call RAM-gate failure. Its SHA-256 is
`bf45069a8c58f2020a3a1c85c56f02194ca00f7b63bda1f8c3d093cb157dc55f`;
the sidecar matches. The create-new dispatch lock points to the preserved run,
so this corpus is consumed and must not be rerun or resumed.

Run:
`local-acceptance-models/nxb21-d/qwen3-8b-selection-development-runs/run-20260908T053547136Z`

Receipt:
`local-acceptance-models/nxb21-d/qwen3-8b-selection-development-runs/run-20260908T053547136Z/selection12-development-summary-v1.json`

## Observed evidence

### Case 1 — valid model evidence

`sel12-en-resolved` made one real request and passed:

- expected and actual decision:
  `ipdr.endpoint/AGGREGATE/authorized_workspace`;
- exact target preserved: `198.51.100.241`;
- strict UTF-8 transport PASS, HTTP success, JSON parse PASS,
  `finish_reason=stop`, no retry;
- 365 prompt tokens, 16 completion tokens, 381 total tokens;
- measured request-to-response latency: 54,405 ms;
- raw model decision, deterministic facts/candidates, final plan, and
  three-layer audit all passed.

This is positive evidence for one English resolved case only. It is 1/12 and is
not enough to determine multilingual or clarification suitability.

### Case 2 — no model-quality evidence

`sel12-en-ambiguous` made no model call. The frozen per-call RAM gate observed
2.646 GiB available against the required 4 GiB floor. Bounded clean-cache
recovery preserved dirty pages and used no sync, but available RAM stayed below
the floor for the complete 180-second wait and ended at 3.567 GiB. The evaluator
therefore recorded `RAM_GATE_FAILED` and stopped before request dispatch.

Resource sampling across the run recorded 38 samples:

- available RAM range: 2.640–7.907 GiB;
- API-container memory peak: 6,048.8 MiB;
- unloaded/baseline API-container memory at the first sample: 656.9 MiB;
- post-unload API-container memory at the final sample: 860.2 MiB.

The first-run precheck had recovered from 4.002 GiB to 7.989 GiB, so initial
admission worked as designed. Loading and executing Qwen3 8B then consumed
enough memory that the 4 GiB between-call safety floor could not be restored.
This is an incompatibility between the frozen profile, the current runtime
memory envelope, and the mandatory safety floor—not a wrong model decision.

## Safety and runtime integrity

The runner's authoritative aggregate result is `DEVELOPMENT_FAILURE`. It
unloaded the owned Qwen3 8B model, and `/system` confirms that model remains
unloaded. Runtime postcheck passed with:

```text
retained_tuple=67|67|80|22510|863|66
activity_count=343
active_jobs=0
model_unloaded=true
activation=BLOCKED
D_status=OPEN
```

No qualification holdout was created or consumed, no accepted model profile was
changed, no production activation occurred, and retained evidence was not
mutated.

## Harness presentation note

The inner Ginkgo output says `SUCCESS` because its RAM-failure branch appends a
failed result and returns from the spec without raising a Ginkgo assertion. The
outer runner then correctly reads the aggregate, sees 2/12 entries and 1/12
model calls, seals `development_gate=FAIL`, returns the development-failure exit
code, and blocks advancement. The misleading inner success is a future-harness
presentation defect; it does not change the authoritative receipt or permit a
rerun of this freeze.

Any newly approved gate must make a pre-call RAM failure fail the inner test
explicitly before that new gate is frozen.

## Adjudication

```text
QWEN3_8B_SELECTION12_ATTEMPT_1=INCOMPLETE_RESOURCE_FAILURE
SELECTION12_CORPUS=CONSUMED_DO_NOT_RERUN
CASES_RECORDED=2_OF_12
MODEL_CALLS=1_OF_12
MODEL_CORRECT=1_OF_1_OBSERVED
QWEN3_8B_MODEL_QUALITY=INSUFFICIENT_EVIDENCE
QWEN3_8B_ADVANCEMENT=BLOCKED
FINAL_D_CANDIDATE=NOT_YET
FINAL_HOLDOUT=NOT_CREATED
FINAL_QUALIFICATION=NOT_STARTED
NX-B2.1D=OPEN
D_STATUS=OPEN
ACTIVATION=BLOCKED
```

## Required decision boundary

Do not rerun the existing command. Before another selection attempt, the owner
must choose a new resource/model architecture and authorize a fresh corpus and
freeze. The bounded options are:

1. run the same artifact/profile on a host or runtime with enough memory to
   preserve at least 4 GiB available while the model remains loaded;
2. authorize a new lower-memory Qwen3 8B development profile (for example a
   smaller context and explicit conservative batch setting), validate its real
   memory envelope separately, then freeze a fresh non-overlapping corpus;
3. retire this local 8B path and select a different residual-model architecture.

No option implies qualification or activation. The next step is a resource/model
architecture decision, not remediation or reuse of the consumed selection12
corpus.
