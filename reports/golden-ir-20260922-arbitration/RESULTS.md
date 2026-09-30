# Fallback ON + clarification arbitration — 2026-09-22

62/62, `FORENSIC_IR_FALLBACK=true`, `FORENSIC_IR_ARBITRATION=true`, `--analyst-text`.
Compared against the same suite with both switches OFF.

## Answers

| verdict | before | after |
|---|---:|---:|
| CORRECT | 31 | **33** |
| WRONG | 20 | 20 |
| CLARIFIED | 6 | **5** |
| PRESENT_NOT_TOP | 3 | 3 |
| ERROR | 1 | **0** |

**IMPROVED 3 · REGRESSED 0.**

    CDR-12  CLARIFIED -> CORRECT   arbitration (route=semantic_ir_fallback)
    TXN-02  CLARIFIED -> CORRECT   fallback    (route=semantic_ir_fallback)
    CDR-08  ERROR     -> CLARIFIED (host sleep artifact in the prior run)

Plan accuracy unchanged at **25/37 = 68%** plan-exact, 0 fields outside the issued enum.
That is expected: the switches change which plan ANSWERS, not which plan is generated.

## Why zero regressions is the point

Arbitration is scoped to CLARIFICATIONS. A clarification asserts nothing, so replacing one
with a plan that passed S6, S9 SHAPE and CONSTRAINT_APPLIED cannot turn a correct answer
wrong. The measurement agrees: 3 improved, 0 regressed.

The guards still bite. CDR-11 and CDR-14 routed AMBIGUOUS_INTENT, the generator produced
plans, validation rejected them (`null filters cannot carry values`,
`filter value not supplied`) and the clarifications stood. NEG-01 — a negative test — did
the same. Coverage widened, no guard relaxed.

## The arbitration NOT built

Overriding CONFIDENT wrong answers is where the remaining 13 questions are. The
shape-mismatch signal was built and measured against all 62 FIRST:

    would fire on 3 wrong answers and 2 CORRECT ones

60% precision. A template can group its rows while the narration still states the right
scalar, so "scalar question + grouping template" does not imply a wrong answer. Trading 2
correct answers for 3 with the risk pointing that way is not a trade this product makes.
**Rejected, with the reasoning recorded in ir_fallback.go so it is not retried blind.**

## Two bugs found while wiring it

- **Arbitration ran too late.** Substituting the request after `understanding` was built
  changed nothing — capability resolution and execution both read `understanding`, so the
  original template ran and returned zero rows while the audit said `arbitrated=true`.
  Moved before `understanding`.
- **Generated plans were never scoped to their family.** `applySemanticSourceNativePlan`
  set template, plan and catalogue but not `RecordType`, so "which cell site handled the
  most calls" grouped `cdr.cell_site_id` across **12,912 rows spanning six families**
  instead of 8,642 CDR rows. The legacy binder always did this; the source-native one never
  had. Every IR-path answer was affected, fallback included. Fixed, with a test that also
  pins that an analyst's explicit record type still wins.

## Cost

With shadow AND fallback both on, an unresolved question pays for two generations
(CDR-11 167s, NEG-01 174s, CASE-04 219s). Acceptable for measurement; turn shadow off in
production, or have the fallback reuse the shadow plan.
