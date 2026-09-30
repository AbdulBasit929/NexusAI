# WI-0 — IR-generation feasibility spike: RESULTS

**Run 2026-09-21, 07:20:31Z → 09:01:51Z.** Model
`qwen3-4b-instruct-2507-q4km-nxb21d-dev` (Q4_K_M) on live LocalAI, CPU only.
**86 live model calls.** Method and the binding decision rule were fixed in
[`PRE-REGISTRATION.md`](PRE-REGISTRATION.md) before any call was made.

Raw per-item results: `raw/arm_A_rescored.json`, `raw/arm_B_rescored.json`,
`raw/run_log.txt`. Harness: `scripts/ir_spike/`.

---

## 1. Headline

| Metric | **Arm A** (no retry) | **Arm B** (one self-correction) |
|---|---:|---:|
| **Execution-equivalent match** | **26/35 = 74.3%** | **28/35 = 80.0%** |
| Exact-IR match | 15/35 = 42.9% | 16/35 = 45.7% |
| Abstention (expressible) | 7/35 = 20.0% | 2/35 = 5.7% |
| **Confident-wrong** | **2/35 = 5.7%** | **5/35 = 14.3%** |
| **Malformed** | **0** | **0** |
| Latency p50 / p95 / max | 77.9s / 143.0s / 154.3s | 63.2s / 129.6s / 136.2s |

Scored over the **35 expressible** items. Separately: **inexpressible 3/4**
correctly abstained (both arms), **family-absent 1/1** correctly abstained (both
arms).

### Verdict under the pre-registered rule

- **Arm A — PROCEED, REINFORCED.** 74.3% sits in the 60–79% band. Neither
  override fires (confident-wrong 5.7% ≤ 10%, malformed 0).
- **Arm B — the confident-wrong override FIRES.** 80.0% would otherwise be
  PROCEED, but 14.3% confident-wrong exceeds the 10% ceiling, and §5 states the
  override applies *regardless of the percentage*.

§5 says to take the better of the two arms on execution-equivalent. That is Arm
B, and Arm B trips a safety override. **The rule therefore returns STOP AND
ESCALATE, and this document is that escalation.** The rule was not reinterpreted
to avoid that outcome. §4 below argues the override is driven by a defect in the
harness's feedback message rather than by the model, but **that is a
recommendation for the user to accept or reject, not a re-scoring.**

---

## 2. The keystone held — this is the most important result

Across **86 live calls**:

- **0 malformed outputs.** Every response was schema-valid JSON on the first
  parse, in both arms, including every retry.
- **0 plans referenced a field outside the issued enum.** Verified mechanically
  over every stored plan, not asserted.

The enum → GBNF alternation is real and it works. A hallucinated field is
**structurally impossible**, not merely unlikely. §11 of the master prompt treats
the compiler as a security control; this is the first empirical evidence that the
control actually holds. The `MALFORMED_OUTPUTS=2` result previously recorded for
Phi-4-mini was avoidable with a capability already in the tree.

**This finding is independent of the decision rule's verdict** and does not
change whichever way the escalation is resolved.

---

## 3. What the model is good and bad at

**Reliably correct (both arms):** plain counts, breakdowns by a dimension, rank
("which X the most" → group + COUNT + sort DESC + limit 1), single-literal
filters, month ranges via `BETWEEN`, and projections of specific rows.

Notably, **the model got three things right that the live product gets wrong**:

| Question | Live product (2026-09-18 baseline) | Spike |
|---|---|---|
| TXN-02 "largest transaction" | COUNT | **MAX(amount)** — correct aggregate |
| TXN-03 "account with most transactions" | ranked by amount | **COUNT per account** — correct |
| NEG-01 nonexistent number | 12,912 (whole case) | **EQ filter on the number** — correct |

NEG-01 is the D1 anchor. The compiler path produces the correct plan for it; the
live product's failure is that `plan.Filters` is never populated
(`deterministic_semantic_compiler.go:418`), not that the plan cannot be formed.

**The one systematic weakness — `field_id` omitted on non-COUNT aggregates.**
In Arm A, **6 of the 7 abstentions** were S6 rejecting exactly this: the model
chose the right aggregate (`MIN`/`MAX`/`SUM`) and the right field conceptually,
then left `field_id` empty. It is a schema-affordance problem, not a reasoning
problem — the `measures.field_id` enum includes `""` (needed for COUNT), and the
model over-applies it. **S6 caught every instance**, so none reached an analyst.

---

## 4. Why Arm B's confident-wrong rate tripled — a harness defect, not a model defect

The three abstention→confident-wrong conversions (CDR-09, ANPR-03, TXN-02) are
**one artefact with one cause**, and it is mine.

The typed diff fed back on retry was:

> `only COUNT may omit field_id; MAX needs a field_id`

The model "repaired" all three by **prepending a `COUNT` measure with an empty
`field_id`** and shifting the real aggregate to `m2`/`m3`:

```
TXN-02 "What was the largest transaction?"
  gold     measures [ {m1 MAX transaction.amount} ]
  Arm B    measures [ {m1 COUNT ""}, {m2 MAX transaction.amount} ]
```

That is a literal, faithful reading of the sentence I sent it: *the measure with
the empty `field_id` should be a COUNT*. The model complied exactly. The MAX is
present and correct in every case; a spurious COUNT was prepended.

**Conclusion: S7's feedback must state the required repair, not the violated
rule.** "Set `field_id` on measure m1 to the field being aggregated; do not add a
measure" would very likely have converted all three to correct. The research
claim that self-correction helps most at small model sizes is *not* contradicted
by this run — retry fixed IPDR-04 and TXN-01 outright — but self-correction
**amplifies whatever the feedback says**, which makes diff wording a
safety-relevant surface.

Retry outcomes, all 8: **2 fixed** (IPDR-04, TXN-01) · **3 converted an honest
abstention into a confident-wrong answer** (CDR-09, ANPR-03, TXN-02) · **3 left
unchanged** (SUB-03, TWR-02 still abstained; CDR-10 unchanged).

---

## 5. The two genuine confident-wrong answers (Arm A)

These are real model errors, not artefacts, and both matter for WI-1.

**SUB-02 — "How many subscribers are active?" → `COUNT` with no filter at all.**
This is the **D1 failure class in a new guise**, and it exposes a gap in S2 as I
implemented it: the literal extractor matches identifier *patterns* (MSISDNs,
plates, `PK-…`, `ACCT-…`), so the word **"active" never became an obligation**
and the S9 CONSTRAINT_APPLIED guard had nothing to check. **Enumerated value
literals — status values, protocol names, call types, HTTP codes — must become
obligations too**, or WI-1's guard ships with the same blind spot the defect it
is fixing has.

**CDR-10 — "Who did 923001110001 contact most frequently?"** The model bound the
subscriber filter and grouped by dialed number correctly, but omitted the
self-exclusion `NEQ`. The oracle excludes self-calls; the model read the question
literally. This was **pre-registered as a genuinely ambiguous item** (the source
data has 425 self-calls for this subscriber) and is the weakest of the failures —
arguably it should have triggered a clarification rather than either answer.

**CDR-07** (inexpressible, "who talked to the most different people") produced a
plan instead of abstaining — group by dialed number and COUNT, which silently
answers a *different* question than the one asked. It is scored separately but is
the same hazard: a distinct-count question answered with a plain count.

---

## 6. Findings that constrain the architecture regardless of the verdict

**a. Two question classes are inexpressible in `SourceNativePlanV1` at any model
quality.** No model escalation addresses either:

- **Distinct counts** (CDR-07, CDR-08, ANPR-05) — the IR allows
  `COUNT · SUM · AVG · MIN · MAX` only (`source_native_algebra.go:538`).
- **Derived measures** (CDR-15) — the CDR payload has **no duration column**,
  only `CALL_START_DT_TM` and `CALL_END_DT_TM`. "Longest call" needs
  `MAX(end − start)`; the IR has no computed expression. (SQL confirms the oracle
  1798 is exactly that difference, in seconds.)

**Both are WI-4 requirements**: the semantic layer needs **derived metrics**, not
merely field descriptors, and the IR needs a distinct-count aggregate. Until
then, the correct behaviour is abstention — which the model achieved 3/4 times.

**b. The curated layer is load-bearing where source data is messy.** SUB-02's
correct answer (6) is only reachable by unioning two source aliases — `status`
and `account_status`. The inferred-per-request catalogue cannot know they are the
same concept. (The dedicated measurement of this, Arm D, was not run — see §8.)

**c. S4 deterministic family routing: 38/40 = 95%**, lexical only, no embeddings.
Two misses, both instructive:

- **CDR-12** "which cell site handled the most calls" → `tower_location`.
  "Cell site" is both a tower entity and a CDR dimension; the disambiguator is
  what is being *counted* (calls). Reported, not tuned away.
- **NEG-04** "how many emails" → `transaction`. A question whose family is absent
  must abstain; a weak lexical match produced a family anyway. **A minimum match
  threshold is required**, or D4's cross-family misrouting reappears through S4.

Family routing was held at the corpus value for scoring (PRE-REG §4), so these
two did not contaminate the IR numbers.

**d. Latency is a product problem, not a spike problem.** p50 63–78s, p95
~130–143s for a *single* IR generation, before any execution or narration. The
§6 target of p95 < 15s for the whole pipeline is unreachable on this hardware at
this model size. `POST /backend/load` pre-warming is already planned for Phase 3;
the first call of the run cost 130.7s against ~40s once warm, so pre-warming is
worth real seconds. It does not close a 130s → 15s gap.

---

## 7. Corrections made to this measurement, disclosed

1. **Filter comparison was wrong for `BETWEEN`/`IN`.** The first scorer compared
   the `value` field for every operator. The executor reads **only** `Values` for
   those two (`source_native_algebra.go:764-770`), and validation does not reject
   a populated `value` beside them. CDR-14 produced correct `BETWEEN` bounds plus
   a redundant `value` and was scored WRONG. Corrected in
   `scripts/ir_spike/rescore.py`, which re-scores the **stored plans** — no
   re-inference, both arms re-scored by identical code. It changed exactly one
   item per arm (CDR-14, False→True). Pre-correction figures were Arm A 71.4% /
   Arm B 77.1%.
2. **One prompt revision**, after the first smoke call and before any scored run,
   as PRE-REG §5 permits. The first smoke call abstained on CDR-11 claiming a
   literal filter "would require a WHERE clause". Two changes: the abstention
   instruction was narrowed to four named cases with a worked filter example, and
   `reason` became an **enum** rather than free text — the GBNF converter does not
   enforce `maxLength`, so free-text reasons are unbounded generation (~30s of
   that first 130.7s call).
3. **Expressible denominator is 35, not the 36 stated in the §3.2 amendment.**
   Arithmetic error in the amendment's prose: 40 items − 4 inexpressible − 1
   family-absent = 35. The harness computed 35 from the data throughout, so **no
   reported figure is affected** — only the prose was wrong. The original §3.1
   figure of 37 had the same error plus the missing CDR-15.

---

## 8. What was not done

- **Arms C (opaque hashed field IDs) and D (uncurated catalogue) were not run** —
  the user scoped this run to A+B. So the field-ID convention recommendation
  (PRE-REG §2.3) and the quantified business case for the curated layer remain
  **unmeasured**. §6b is supporting evidence, not the measurement.
- No production source file was modified by WI-0.

---

## 9. Recommendation, and the decision taken

The rule returned STOP AND ESCALATE on Arm B. My reading of the evidence:

1. **The 4B model is sufficient for S5**, at Arm A's configuration: 74.3%
   execution-equivalent with **5.7% confident-wrong** and zero malformed — versus
   a product baseline of **42% confident-wrong**. Arm A satisfies the rule's band
   *and* both overrides.
2. **Do not ship unconditional self-correction.** Arm B buys +5.7 points of
   accuracy by converting honest abstentions into confident answers, and in a
   forensic product that is the wrong trade. Re-run Arm B with a repair-shaped
   diff (§4) before deciding; that is one ~50-minute run and it is the single
   cheapest experiment available.
3. **Fix the diff wording, the S2 enumerated-literal gap (§5) and the S4 match
   threshold (§6c)** before Phase 3 builds on any of them.
4. **WI-4 must include derived metrics and a distinct-count path** (§6a), or a
   whole class of questions stays permanently unanswerable.

---

## 10. RESOLUTION — 2026-09-21

The escalation was put to the user with the numbers above. **Decision: accept Arm A and
drop unconditional self-correction.**

- **WI-0 verdict of record: PROCEED, REINFORCED** on Arm A's configuration — 74.3%
  execution-equivalent, 5.7% confident-wrong, 0 malformed, against a product baseline of
  42% confident-wrong. Per the 60-79% branch: keep the 4B model, widen S4 narrowing, and
  hold `qwen3-8b-q4km-nxb21d-selection-dev` (already present locally) as a fallback tier
  for low-confidence plans only — never as the default.
- **Self-correction is NOT adopted** in its measured form. It stays a candidate, gated on
  re-measuring with a repair-shaped diff (§4). Arm B's numbers stand as recorded.
- Arms C and D remain unrun; the field-ID convention and the curated-layer business case
  remain unmeasured and must not be cited as settled.
