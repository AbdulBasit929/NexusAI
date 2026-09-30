# Golden-IR measurement — 2026-09-22 (final)

62/62 questions, `FORENSIC_IR_SHADOW=true`, `FORENSIC_IR_FALLBACK=false`.
Plans scored against `scripts/ir_spike/gold_plans.json` by `scripts/golden_ir_score.py`.

## Headline

| | |
|---|---:|
| plan-exact correct | **15/37 = 41%** |
| + execution-equivalent | **16/37 = 43%** |
| out-of-enum field ids | **0** — the enum keystone holds, and is now *verifiable* |
| unmeasured (instrument gap) | 1 |

Run 2, before the four fixes, scored **3/19 = 16%** with **5 wrong plans**. Final has
**1 wrong plan**. The failure mode moved from "picks the wrong field" to "produces no
plan", which is a different and more tractable problem.

## Shadow outcomes, all 62

| outcome | n | whose defect |
|---|---:|---|
| generated | 20 | — |
| no_family (generator never invoked) | 19 | 16 correct (media/cross-family/negative), 3 known gaps |
| filter value not supplied | 9 | **mixed — see below** |
| malformed proposal | 6 | model |
| truncated (array padding) | 3 | schema |
| null filter carried a value | 2 | model |
| BETWEEN arity | 1 | model |
| field_id omitted (WI-0's weakness) | 1 | model |
| absent | 1 | instrument |

`only COUNT may omit field_id` — which WI-0 recorded as the cause of **6 of 7** abstentions
— occurred **once in 62**. Much of what WI-0 attributed to the model was the model coping
with a degraded enum. IPDR-01 shows it directly: same question, same model, `COUNT(fld_b4d5a2…)`
when starved of curated fields, the exact gold plan when given them.

## The `filter value not supplied` class is mostly OURS

`semanticSourceNativeLiteralValues` extracts only quoted strings and bare numbers:

    "plate LHR-2026 seen?"         -> ['-2026']     <- plate unusable, hyphen read as minus
    "How many subscribers active?" -> []            <- 'active' invisible

`subscriber.status` **declares** `ACTIVE` with synonym `active`, and the layer has a
purpose-built accessor `ValueLiterals(family)` — **never called anywhere in production**.
So the generator produced correct filters and we rejected them.

## Truncation is array padding, not budget

    CDR-16  599B     ANPR-04 1791B     ACC-03 1696B      valid plan ~292B

Every truncation is 2–6x a complete plan. Raising `max_tokens` would be the wrong fix.

## Fallback decision

The fallback only fires where the compiler could not resolve. In this run that is 4 questions:

| id | if FORENSIC_IR_FALLBACK=true |
|---|---|
| TXN-02 | **CLARIFIED -> correct answer** (`MAX(transaction.amount)`) |
| CDR-11, CDR-14, NEG-01 | no plan -> clarification stands |

**+1 correct, 0 confident-wrong.** The risk that shipped it OFF does not materialise —
S6/S9/CONSTRAINT_APPLIED discard the bad plans, as NEG-01 demonstrates.

## The larger finding: arbitration, not coverage

10 questions where the DELIVERED answer was wrong or clarified while the generated plan
was correct — ACC-02, ANPR-06, CDR-02, CDR-04, CDR-05, IPDR-04, SUB-01, TWR-01, TXN-01,
TXN-02. Only TXN-02 is fallback-eligible; the other 9 route through
`semantic_deterministic_registered` or the ladder, which answer **confidently and wrongly**
and therefore never hand off.

Enabling the fallback gains 1. Letting a re-verified IR plan arbitrate against a
confidently-wrong deterministic answer is worth up to 10.

## Caveat on answer accuracy

The answer verdicts in `run_log.txt` were graded WITHOUT `--analyst-text`, so they are not
comparable to WI-6's 31C/20W. CDR-02 and SUB-01 appear as regressions purely for that
reason; their responses are byte-identical to the earlier run. The plan measurement above
is unaffected.
