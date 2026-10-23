# Link analysis, stage 3: the milestone run (T4), gates written first

Written 2026-10-23 **before the run**. The pre-registration (`link-analysis-stage3-preregistration-20261022.md`, section 3 and the plan row P1-A) asks for one T4 run at the end of the stage, in the background, read once (the tiers: `docs/roadmap/reconciliation-20261020.md` section 6).

## Why this run, and what it is not

Stages 3a, 3b and 3c each passed their own gates on their own cases. Since then three pieces the older stages share were changed: example retrieval (a word is weighed among the examples of the offered views), the `count` cue (the verb "count" in the sense of "take into account"), the night check (a night question of the anchor views is carried by the slot), the successor view and the parameter finders (numbers of up to five digits, two lengths after "or"), and the engine now carries 21 views and 24 examples. The run asks: **did any of that make an older stage worse, and do all five banks hold together on one case none of them has seen?** It is a regression read, not a new measurement of any analysis; no gate number of any stage is changed.

## The run

* **Case:** judgement case **110** (the second reserve; unopened until this run; 105 and 109 are spent, 111 and 112 stay in reserve; sealed 901 to 904 untouched).
* **Code:** the commit at the head of `lane-arms` when the run starts, named in the results (no change between the start and the end of the run).
* **Arm:** the engine arm (the Ask engine with the real model), one run, read once.
* **Banks, all from the generator's oracle with seed 110, counts printed before, nothing read before the run:** stage 1 (`tools/ask_eval/build_questions.py --register-case --seed 110 --bank`, about 82 questions), stage 2 (`tools/ask_eval/colocation_oracle.py --register-case --seed 110`, 24), stage 3a (`stage3a_bank.py`, 10), stage 3b and 3c (`stage3bc_bank.py`, 10 each). About 136 questions, about an hour of model time in the background.

## Gates (each stage keeps its own, as pre-registered; read with `judge.py` and `stage3_gates.py`)

| # | Gate | Applies to |
|---|---|---|
| T4-1 | **Wrong answers zero** in each bank | stages 1, 2, 3a, 3b, 3c |
| T4-2 | **At least 90 percent correct in each kind**, a decline or abstention counting against it | all five |
| T4-3 | **Controls 100 percent correct** (a question whose right answer is "none") | all five |
| T4-4 | **The caveat on every result and every probe, the value used stated on every answer that reads a rule value** | stages 3a, 3b, 3c (`stage3_gates.py` gates 4 and 5) |
| T4-5 | **Latency:** the 90th percentile at most 120 s and no answer over 300 s; the median and 90th percentile reported | all five |
| T4-6 | **No stage worse than its own record:** stage 1 was 81 of 81 on case 108, stage 2 was 72 of 72, so a stage-1 or stage-2 answer that is wrong, or more than one that is declined, is written up as a regression whatever the 90 percent says | stages 1, 2 |

## If a gate fails

Written up with its cause, **only the cause the run shows is fixed**, the fix measured once on the next fresh case (111), nothing tuned on 110 and rerun. A gate that fails twice for the same analysis leaves that analysis dark (its views stay, it is not in the engine's list, the older path answers) and the owner is told. Nothing is deployed because of this run without the owner's go.
