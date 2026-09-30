# CDR-14 COMPILED FAMILY SCOPE — RESULT

Measured 2026-09-27. Thresholds in `THRESHOLD_PREREGISTRATION.md`, written before the run.

**Outcome: THRESHOLD FIRED. Reverted and removed. The defect it targeted is not live in any shipped
path.**

---

## 1. What was measured

    arm A (scope off)  reproduced the shipped posture exactly     67 | 28 | 4 | 2 | 1 | 1
    arm B (scope on)                                              66 | 28 | 5 | 2 | 1 | 1
        IPDR-02  CORRECT -> WRONG   "1 access log entry matched this question."

One CORRECT lost to a confident wrong. **The switch had been made default ON** in code and compose,
on the reasoning that its off-state over-counts — so the running service was in the regressing state
at the moment the threshold fired. It was switched off and re-asserted from the container **before**
any analysis began, and the code default was then flipped to off.

## 2. Why IPDR-02 broke

*"Which domain was **accessed** most often?"* — the semantic frame labels this the access-log family
on the word "accessed". The change scoped the compiled plan to that label, `access_log`, and access
logs carry no domain: *"1 access log entry matched"*.

## 3. The diagnosis was wrong in two places, and the evidence is what corrected it

**First wrong claim.** I explained IPDR-02 as "unscoped, only IPDR rows carry a domain, so the answer
was right". Executed against the live database, the unscoped plan ranks a **`null` domain group of
10,412 non-IPDR rows first**. It is not self-scoping. Something else was scoping it.

**What actually scopes it.** `applyCanonicalQueryHints` (`query.go`) sets `RecordType` from the
**question's words** whenever it is empty and the template is `canonical_records`:

    "Which domain was accessed most often?"        -> ipdr   (from "domain")
    "How many call records are from August 2026?"  -> cdr    (from "call record")

It runs at `query.go:477`. The change set `RecordType` from the frame's family label **earlier**,
inside the compiler — so the hint saw a non-empty value and did not override it. **The change
pre-empted the existing, better scoping.** Same failure shape as the A3.3 arbitration earlier today:
a new mechanism inserted *before* a working one.

**Second wrong claim.** CDR-14's "4 records" is real, but it lives **only** on the arbitration path
(`query.go:559`), which swaps a plan in **after** the hints have already run. That path was the A3.3
deterministic arbitration — already reverted. In every shipped path, CDR-14 is scoped correctly.
Verified against the running service after the revert:

    CDR-14   "There are 2 CDR records from 2026-08-01 up to (not including) 2026-09-01 in this case."
    IPDR-02  "2,500 IPDR sessions across 4 domain values ... chat.example.test: 639 ..."

**This change fixed a defect no shipped path has, and broke one that worked.**

## 4. The instrument defect — the fifth today

`TestCompiledFamilyScopeExecutesCDR14` reproduced "4" with the switch off and "2" with it on, against
the live database, through the real executor — and it was still misleading. It called
`applyDeterministicSemanticCompiler` and `executeSourceNativePlanSQL` **directly**, skipping
`query.go:477`. It proved the fix on a path production never takes.

An offline test that bypasses part of the request pipeline can prove a fix for a defect the
pipeline does not have. The corpus run is what exposed it.

## 5. The right fix, if it is ever needed

If a compiled plan is adopted at arbitration again, **re-apply `applyCanonicalQueryHints` after
arbitration** — the one existing, question-based definition of scope. Never add a second scope
source ahead of it. That is recorded in a comment at the removal site in
`deterministic_semantic_compiler.go`.

## 6. Removed

`compiled_family_scope.go`, its unit test, its live-database test, the call site, and the compose
entry. Full package tests pass. Rebuilt from the reverted source; shipped posture re-asserted.

## 7. Lessons, all three paid for today

1. **A default-on "safety" switch is only safe once measured safe.** This one shipped the regression
   into the running service until the threshold fired.
2. **Before inserting a mechanism, find the one that already does the job.** Twice today a new
   mechanism placed *ahead* of a working one displaced it.
3. **An offline proof is only as good as the path it exercises.** Name the pipeline stages the test
   skips.

Rollback image: `nexusai-forensic-records-api:rollback-before-familyscope-20260927`.
