# Golden-IR measurement — 2026-09-22, after the fix list

62/62, `FORENSIC_IR_SHADOW=true`, `FORENSIC_IR_FALLBACK=false`, graded with `--analyst-text`.
CDR-08 re-run separately after the host slept mid-run and errored it.

## Plan accuracy

| | run 2 | pre-fix | **after** |
|---|---:|---:|---:|
| plan-exact correct | 3/19 = 16% | 15/37 = 41% | **25/37 = 68%** |
| + execution-equivalent | — | 16/37 = 43% | **26/37 = 70%** |
| generated | 9/22 | 20/62 | **37/62** |
| out-of-enum field ids | 0 | 0 | **0** |

## Failure classes: five went to zero

| outcome | pre-fix | after |
|---|---:|---:|
| generated | 20 | **37** |
| no_family (19 of which 16 are correct: media/cross-family/negative) | 19 | 19 |
| dynamic_validation_rejected (all kinds) | 13 | **4** |
| malformed proposal | 6 | **1** |
| truncated (array padding) | 3 | **0** |
| runtime_rejected (grammar 500) | 3 | **0** |
| field_id omitted (WI-0's weakness) | 1 | **0** |

Latency 38–80 s against 70–140 s: a bounded grammar stops the model generating into a void.

## What each fix bought

- **GBNF ignored `maxItems`** — arrays compiled to unbounded repetition, so the model padded
  until the budget died (599B/1233B/1696B/1791B vs a ~292B correct plan). Fixed in LocalAI
  core; truncation and grammar-500s both went to zero.
- **Literal extraction** — `"plate LHR-2026"` extracted `-2026`, reading the hyphen as a minus.
  ANPR-02 now produces `COUNT(*) WHERE anpr.plate_number = 'LHR-2026'`. It needed this fix
  AND the grammar fix together.
- **Curated numerics compared as TEXT** — the layer declares NUMBER, the SQL builder switches
  on INTEGER/DECIMAL, so `MAX(transaction.amount)` was lexicographic: 9,200 beat 75,000.
  **This was the "instability across builds" recorded as the reason the fallback ships OFF.**
- **`COUNT_DISTINCT` + derived metrics** — both declared in the layer since WI-4 and
  inexpressible in the plan. `MAX(cdr.call_duration_seconds)` now generates; there is no
  duration column in the data, so that question was unanswerable at any model quality.
- **Rejected plans recorded** — a rejection reason can now be attributed to a plan.

## Fallback decision

Only 4 questions route where the fallback fires:

| id | if FORENSIC_IR_FALLBACK=true |
|---|---|
| TXN-02 | **CLARIFIED -> correct answer** (`MAX(transaction.amount)`, now numeric) |
| CDR-11, CDR-14, NEG-01 | no plan -> clarification stands |

**+1 correct, 0 confident-wrong.** NEG-01 is the proof the guards hold: the generator
proposed something on a negative test and validation discarded it.

## The real opportunity is arbitration, not coverage

**14 questions** (up from 10) delivered a wrong or clarified answer while the generated plan
was CORRECT: ACC-02, ACC-03, ANPR-02, ANPR-04, ANPR-06, CDR-04, CDR-05, CDR-12, CDR-16,
IPDR-04, IPDR-05, TWR-01, TXN-01, TXN-02.

Only TXN-02 is fallback-eligible. The other 13 route through
`semantic_deterministic_registered` or the ladder, which answer **confidently and wrongly**
and never hand off. Enabling the fallback gains 1; letting a re-verified IR plan arbitrate
against a confidently-wrong deterministic answer is worth up to 14.

## Known gaps, unchanged

- 3 genuine `no_family` (CDR-07, CDR-10, ANPR-03) — system clarifies rather than guesses.
  A layer-vocabulary resolver was built, measured, and REJECTED: it fixed these 3 and
  misrouted 6 negative/media questions.
- SUB-03's plan gold demands `subscriber.cnic`/`full_name`, which the privacy guard
  withholds by design. The gold is wrong, not the system.
- SUB-02 now *may* filter `status = ACTIVE` (no longer rejected) but the model omits it —
  scores `missing_filter`. Possibly my prompt converse overcorrecting.
- NEG-03 still returns no audit at all: one early-return path never reaches the shadow hook.
