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

## Result of the run (2026-10-23; code 49c4029; case 110; the engine arm; once, read once)

| Bank | Questions | Correct | Wrong | Declined or abstained | Latency (median, 90th percentile, longest) | Gates |
|---|---|---|---|---|---|---|
| Stage 3a (A5, A7) | 10 | 10 | 0 | 0 | 24.4 s, 50.5 s, 53.7 s | T4-1 to T4-5 **met** |
| Stage 3b (A9) | 10 | 10 | 0 | 0 | 26.1 s, 38.4 s, 61.5 s | **met**; controls 3 of 3; caveat on 5 of 5 results and 2 of 2 probes |
| Stage 3c (A10, A11) | 10 | 10 | 0 | 0 | 28.2 s, 40.1 s, 41.3 s | **met**; controls 3 of 3; caveat on 6 of 6 results and 2 of 2 probes |
| Stage 2 (co-location at the hour) | 24 | **22** | **2** | 0 | **73.8 s**, 97.5 s, 101.1 s (10 repairs) | **T4-1, T4-3 and T4-6 not met** |
| Stage 1 (connections, handsets, the rest) | 81 | **79** (80 after the judge fix) | **2** (1) | 0 | 27.7 s, 54.7 s, 91.3 s (9 repairs) | **T4-1 and T4-6 not met** |

**The four wrong answers, each with its cause.**
1. **Stage 2, `C020` and `C024`, a regression caused by stage 3a.** "How many different hours did A and B spend at the same cell site together?" was answered from `v_colocation_windows` (the 15-minute windows of stage 3a), counting their start times: 3 and 4 where the hours are 5 and 5. The shortlist offered that view because "same" and "together" are in its lexicon; a question about hours that names no minutes belongs to the hourly view. **Fix:** such a question loses `v_colocation_windows` from the shortlist (`askHourColocation`; a spec on two phrasings, and one that keeps a minutes question on the windows view).
2. **Stage 1, `Q003`.** "How many events are there between A and B?" was answered by counting the rows of `v_contact_pairs` (1) where the events are 76: a row of that view is one contact, not one event. The example retrieval was checked and is **not** the cause (the old and the new weighting give the same examples for this question). **Fix:** a count of rows on that view for a question that counts events, calls or messages fails an obligation that asks for the sum of the `events` column (a spec on the failing query, the summed query and a "how many numbers" question that stays a count).
3. **Stage 1, `Q038`, a defect of the judge, not of the engine.** "List every IMEI seen with A" was answered with both IMEIs and the IMSI of the SIM in the same rows; the `set` judge read the IMSI as a third handset. The answer was complete and right. **Fix:** the `set` judge no longer reads a column named `imsi` (4 unit tests); stage 1 re-judged is 80 of 81, and the table above gives both figures.

**Fix measurement (2026-10-23; code 4acbf23; unopened judgement case 111; once).** The classes the fixes touch, plus the stage whose routing changed: **stage 1 `pair_count`, `incoming_count`, `devices` and `distinct_contacts` 21 of 21; stage 2 24 of 24** (`coloc_hours` 12 of 12); **stage 3a 10 of 10** (windows asked for 10, 15, 20, 22 and 30 minutes); **0 wrong in 55**. Stage 2 is slower than the budget (median 61.1 s, 90th percentile 81.3 s, 12 repairs): the hourly questions need a repair in about half of the cases; the gate (90th percentile at most 120 s) is met and the cost is listed as the next speed item for the hourly view. Cases spent by this milestone: 110 and 111 (112 stays in reserve; sealed 901 to 904 untouched). **Nothing is deployed for this**: the fixes are in the repository (commit 4acbf23) and reach the live system with the owner's next go for an image.

## Deploy of the fixes (2026-10-26, with the owner's go)

Code only: no migration and no database change, so no backup was needed. Rollback tag `nexusai-forensic-records-api:before-engine-fixes-20261026` = `a48f2ff888fd` (the image of 3b and 3c). New image `nexusai-forensic-records-api:engine-fixes-20261026` = `latest` = **`8145101db0fc`**, built from `lane-arms` at `f7d61fc` (the engine fixes of `4acbf23` and the headline fix of `cc3a989`); cross-built (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`), `FROM` the rollback image with the binary and `semantic_layer` copied over it. Only the API container was recreated (`recreate_api_askv2.ps1`, the same settings: typed families `communications_cdr,network_ipdr`, Ask engine v2 on and first, no variable differs from before).

* `healthz` 200; **`live_smoke.py --quick`: PASS, 5 of 5** (the cell-site headline check still answers "899 positions at 5 cell sites." for a visits question, which is right: those rows are visits).
* Four questions asked of the live service on the synthetic development case 2, each against an answer computed from the generated records:

| Question | Live answer | Records say |
|---|---|---|
| How many different hours did 003 and 007 spend at the same cell site together? | 26, from `v_cell_visits` (the hourly view) | 26 (the fix: the 15-minute windows view is no longer offered for a question about hours) |
| When were 007 and 011 at the same cell within 22 minutes? | "4 times together at 1 cell site." | 4 windows at 1 site (the headline fix) |
| Which of the times 003 and 007 ... could have been a meeting ...? | "2 possible meetings at 1 cell site." | 2 (the headline fix) |
| **How many events are there between 003 and 007?** | **8**, by adding the events of 003 with 007 and of 007 with 003 | **4** |

**A new finding, not caused by the deploy and not covered by the fix.** The 4 calls between two subscribers are in **both** subscribers' exports (once as outgoing, once as incoming, the same start times), so adding the two directions counts each call twice: 8 where the records hold 4 distinct events. Before the deploy the same question was answered from the rows of the view (a different wrong number). The stage-1 `pair_count` bank pairs a subscriber with a number that is not a subscriber, so it could not show this. The fix to propose: one direction only (the first number named is the subscriber whose records are read), enforced as an obligation with the repair message "the events of the two numbers are in both exports; read one direction", measured once on the unopened case 112. **Built and measured 2026-10-26 on case 112 (Amendment 4 of the pre-registration): 19 of 19 correct, 0 wrong; deployed the same day (image `59d033208817`, rollback `before-pair-direction-20261026`), the question now answers 4.**
